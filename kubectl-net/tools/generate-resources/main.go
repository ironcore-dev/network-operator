// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

// generate-resources walks the network-operator API directory for *_types.go
// files and extracts resource definitions from kubebuilder markers:
//
//   - +kubebuilder:resource:path=<plural>
//   - +kubebuilder:resource:singular=<singular>
//   - +kubebuilder:resource:shortName=<name>[;<name>...]
//
// It outputs a Go source file containing a ResourceDef slice with the plural
// name, aliases (singular + short names), and kind for each resource. This
// keeps the kubectl plugin's resource list in sync with the API types
// automatically.
//
// Usage:
//
//	go run ./tools/generate-resources --api-root <path> --out <file>
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

type resourceDef struct {
	Name    string
	Aliases []string
	Kind    string
	Group   string
	Version string
}

func main() {
	var apiRoot string
	var outPath string
	flag.StringVar(&apiRoot, "api-root", "", "Path to api directory")
	flag.StringVar(&outPath, "out", "", "Output file path")
	flag.Parse()

	if apiRoot == "" || outPath == "" {
		fmt.Fprintln(os.Stderr, "usage: generate-resources --api-root <path> --out <file>")
		os.Exit(2)
	}

	defs, err := loadResources(apiRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := writeOutput(outPath, defs); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// packageMeta looks for a +groupName marker in groupversion.go and doc.go
// (in that order) and returns (group, version). version is derived from the
// last path segment of dir (e.g. "v1alpha1"). Returns empty group if no
// marker is found in either file.
func packageMeta(dir string) (group, version string) {
	version = filepath.Base(dir)
	for _, name := range []string{"groupversion.go", "doc.go"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		for line := range strings.SplitSeq(string(data), "\n") {
			if matches := groupNameRe.FindStringSubmatch(line); matches != nil {
				return matches[1], version
			}
		}
	}
	return "", version
}

func loadResources(apiRoot string) ([]resourceDef, error) {
	var defs []resourceDef
	cache := map[string][2]string{}

	err := filepath.WalkDir(apiRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_types.go") {
			return nil
		}
		dir := filepath.Dir(path)
		meta, ok := cache[dir]
		if !ok {
			g, v := packageMeta(dir)
			meta = [2]string{g, v}
			cache[dir] = meta
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		defs = append(defs, extractResources(string(content), meta[0], meta[1])...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.SortFunc(defs, func(i, j resourceDef) int {
		return strings.Compare(i.Name, j.Name)
	})
	return defs, nil
}

var (
	groupNameRe     = regexp.MustCompile(`^\s*//\s*\+groupName=(\S+)`)
	resourcePathRe  = regexp.MustCompile(`^//\s*\+kubebuilder:resource:path=([^,\s]+)`)
	resourceSingRe  = regexp.MustCompile(`^//\s*\+kubebuilder:resource:singular=([^\s]+)`)
	resourceShortRe = regexp.MustCompile(`^//\s*\+kubebuilder:resource:shortName=([^\s]+)`)
	typeDefRe       = regexp.MustCompile(`^type\s+(\w+)\s+struct\b`)
)

func extractResources(content, group, version string) []resourceDef {
	var defs []resourceDef
	lines := strings.Split(content, "\n")
	for i := range lines {
		line := strings.TrimSpace(lines[i])
		m := resourcePathRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}

		var (
			path       = strings.TrimSpace(m[1])
			kind       string
			singular   string
			shortNames []string
		)

		// Parse inline singular/shortName from the same line as path.
		parseInlineAttrs(line, &singular, &shortNames)

		for j := i + 1; j < len(lines); j++ {
			l := strings.TrimSpace(lines[j])
			if match := resourceSingRe.FindStringSubmatch(l); match != nil {
				singular = strings.TrimSpace(match[1])
				continue
			}
			if match := resourceShortRe.FindStringSubmatch(l); match != nil {
				for part := range strings.SplitSeq(match[1], ";") {
					if name := strings.TrimSpace(part); name != "" {
						shortNames = append(shortNames, name)
					}
				}
				continue
			}
			if match := typeDefRe.FindStringSubmatch(l); match != nil {
				kind = strings.TrimSpace(match[1])
				break
			}
		}

		if kind == "" {
			continue
		}

		aliases := make([]string, 0, 1+len(shortNames))
		if singular != "" && singular != path {
			aliases = append(aliases, singular)
		}
		aliases = append(aliases, shortNames...)

		defs = append(defs, resourceDef{
			Name:    path,
			Aliases: aliases,
			Kind:    kind,
			Group:   group,
			Version: version,
		})
	}
	return defs
}

// parseInlineAttrs extracts singular and shortName from a line like:
// +kubebuilder:resource:path=indices,singular=index,shortName=idx
func parseInlineAttrs(line string, singular *string, shortNames *[]string) {
	idx := strings.Index(line, ":path=")
	if idx < 0 {
		return
	}
	attrs := line[idx+1:] // "path=indices,singular=index,shortName=idx"
	for kv := range strings.SplitSeq(attrs, ",") {
		k, v, _ := strings.Cut(strings.TrimSpace(kv), "=")
		switch k {
		case "singular":
			*singular = v
		case "shortName":
			for sn := range strings.SplitSeq(v, ";") {
				if name := strings.TrimSpace(sn); name != "" {
					*shortNames = append(*shortNames, name)
				}
			}
		}
	}
}

func writeOutput(outPath string, defs []resourceDef) error {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "// SPDX-"+"FileCopyrightText: %d SAP SE or an SAP affiliate company and IronCore contributors\n", time.Now().UTC().Year())
	fmt.Fprintln(&buf, "// SPDX-"+"License-Identifier: Apache-2.0")
	fmt.Fprintln(&buf, "")
	fmt.Fprintln(&buf, "// Code generated by generate-resources; DO NOT EDIT.")
	fmt.Fprintln(&buf, "")
	fmt.Fprintln(&buf, "package cmd")
	fmt.Fprintln(&buf, "")
	fmt.Fprintln(&buf, "type ResourceDef struct {")
	fmt.Fprintln(&buf, "\tName    string")
	fmt.Fprintln(&buf, "\tAliases []string")
	fmt.Fprintln(&buf, "\tKind    string")
	fmt.Fprintln(&buf, "\tGroup   string")
	fmt.Fprintln(&buf, "\tVersion string")
	fmt.Fprintln(&buf, "}")
	fmt.Fprintln(&buf, "")
	fmt.Fprintln(&buf, "var resourceDefs = []ResourceDef{")
	for _, def := range defs {
		fmt.Fprintf(&buf, "\t{Name: %q, Aliases: %#v, Kind: %q, Group: %q, Version: %q},\n", def.Name, def.Aliases, def.Kind, def.Group, def.Version)
	}
	fmt.Fprintln(&buf, "}")
	fmt.Fprintln(&buf, "")
	fmt.Fprintln(&buf, "// QualifiedName returns the resource name in \"name.group\" form understood by")
	fmt.Fprintln(&buf, "// the Kubernetes resource builder, pinning the lookup to this resource's API")
	fmt.Fprintln(&buf, "// group and avoiding clashes with same-named types from other controllers")
	fmt.Fprintln(&buf, "// (e.g. Calico BGPPeer, cert-manager Certificate).")
	fmt.Fprintln(&buf, "func (r ResourceDef) QualifiedName() string {")
	fmt.Fprintln(&buf, "\tif r.Group == \"\" {")
	fmt.Fprintln(&buf, "\t\treturn r.Name")
	fmt.Fprintln(&buf, "\t}")
	fmt.Fprintln(&buf, "\treturn r.Name + \".\" + r.Group")
	fmt.Fprintln(&buf, "}")

	if err := os.MkdirAll(filepath.Dir(outPath), fs.ModePerm); err != nil {
		return err
	}
	return os.WriteFile(outPath, buf.Bytes(), 0o644)
}
