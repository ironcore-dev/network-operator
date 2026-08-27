// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"go/types"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

// matrix is the fully-derived model the renderer consumes.
type matrix struct {
	// kinds are the API kinds (interface X in XProvider), sorted.
	kinds []string
	// columns are the concrete matrix columns, in providerColumns order.
	columns []column
	// support is keyed [kind][columnName] -> cell.
	support map[string]map[string]cell
}

// column is one rendered matrix column.
type column struct {
	// name is the provider's registered name and the column header.
	name string
}

// cell is one matrix entry for a (kind, column) pair.
type cell struct {
	// implemented is false when the provider type does not satisfy the interface;
	// it renders as N/A.
	implemented bool
	// unsupportedFields are the spec field paths the EnsureX method rejects with
	// NewUnsupportedFieldError. Empty means full support; non-empty means partial.
	unsupportedFields []string
}

// build performs the single go/packages load and derives the matrix.
func build() (*matrix, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedTypes | packages.NeedTypesInfo |
			packages.NeedSyntax | packages.NeedDeps | packages.NeedImports,
	}
	// Load the provider root (for interfaces) and every provider impl package.
	patterns := make([]string, 0, 1+len(providerColumns))
	patterns = append(patterns, providerModulePath)
	for _, pc := range providerColumns {
		patterns = append(patterns, pc.pkgPath)
	}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, fmt.Errorf("loading packages: %w", err)
	}
	if packages.PrintErrors(pkgs) > 0 {
		return nil, errors.New("packages loaded with errors")
	}

	byPath := make(map[string]*packages.Package, len(pkgs))
	for _, p := range pkgs {
		byPath[p.PkgPath] = p
	}
	root := byPath[providerModulePath]
	if root == nil {
		return nil, fmt.Errorf("provider root package %q not loaded", providerModulePath)
	}

	ifaces := discoverInterfaces(root)
	if len(ifaces) == 0 {
		return nil, fmt.Errorf("no XProvider interfaces found in %s", providerModulePath)
	}

	m := &matrix{support: make(map[string]map[string]cell)}
	for kind := range ifaces {
		m.kinds = append(m.kinds, kind)
	}
	sort.Strings(m.kinds)

	for _, pc := range providerColumns {
		m.columns = append(m.columns, column{name: pc.registeredName})
		pkg := byPath[pc.pkgPath]
		if pkg == nil {
			return nil, fmt.Errorf("provider package %q not loaded", pc.pkgPath)
		}
		providerType := lookupProviderType(pkg)
		if providerType == nil {
			return nil, fmt.Errorf("no Provider type in %q", pc.pkgPath)
		}
		violations := collectViolations(pkg, m.kinds)

		for _, kind := range m.kinds {
			implemented := types.Implements(providerType, ifaces[kind]) ||
				types.Implements(types.NewPointer(providerType), ifaces[kind])
			if m.support[kind] == nil {
				m.support[kind] = make(map[string]cell)
			}
			m.support[kind][pc.registeredName] = deriveCell(implemented, violations[kind])
		}
	}
	return m, nil
}

// discoverInterfaces returns kind -> *types.Interface for every exported
// XProvider interface in the provider root package that declares an EnsureX
// method — the marker that distinguishes resource-configuration interfaces from
// operational ones (Maintenance, Provisioning, ConfigBackup, etc.).
func discoverInterfaces(root *packages.Package) map[string]*types.Interface {
	out := make(map[string]*types.Interface)
	scope := root.Types.Scope()
	for _, name := range scope.Names() {
		if !strings.HasSuffix(name, "Provider") || name == "Provider" {
			continue
		}
		obj := scope.Lookup(name)
		tn, ok := obj.(*types.TypeName)
		if !ok {
			continue
		}
		iface, ok := tn.Type().Underlying().(*types.Interface)
		if !ok {
			continue
		}
		kind := strings.TrimSuffix(name, "Provider")
		if !hasMethod(iface, "Ensure"+kind) {
			continue
		}
		out[kind] = iface
	}
	return out
}

// hasMethod reports whether iface declares a method with the given name.
func hasMethod(iface *types.Interface, name string) bool {
	for m := range iface.Methods() {
		if m.Name() == name {
			return true
		}
	}
	return false
}

// lookupProviderType returns the concrete named "Provider" type of a package.
func lookupProviderType(pkg *packages.Package) *types.Named {
	obj := pkg.Types.Scope().Lookup("Provider")
	if obj == nil {
		return nil
	}
	tn, ok := obj.(*types.TypeName)
	if !ok {
		return nil
	}
	named, ok := tn.Type().(*types.Named)
	if !ok {
		return nil
	}
	return named
}

// deriveCell folds the implemented floor and the collected unsupported field
// paths into a single cell: unimplemented is N/A, implemented with rejected
// fields is partial, implemented with none is full.
func deriveCell(implemented bool, unsupported map[string]bool) cell {
	c := cell{implemented: implemented}
	if !implemented {
		return c
	}
	for field := range unsupported {
		c.unsupportedFields = append(c.unsupportedFields, field)
	}
	sort.Strings(c.unsupportedFields)
	return c
}
