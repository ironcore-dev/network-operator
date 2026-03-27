// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"

	v1alpha1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/iosxr"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos"
	"github.com/ironcore-dev/network-operator/internal/provider/openconfig"
)

// interfaceMap maps provider interface names to their reflect.Type.
// generate-provider-matrix fails if it finds a different set of Provider interfaces in the source code.
var interfaceMap = map[string]reflect.Type{
	"ACLProvider":              reflect.TypeFor[provider.ACLProvider](),
	"BannerProvider":           reflect.TypeFor[provider.BannerProvider](),
	"BGPProvider":              reflect.TypeFor[provider.BGPProvider](),
	"BGPPeerProvider":          reflect.TypeFor[provider.BGPPeerProvider](),
	"CertificateProvider":      reflect.TypeFor[provider.CertificateProvider](),
	"DNSProvider":              reflect.TypeFor[provider.DNSProvider](),
	"EVPNInstanceProvider":     reflect.TypeFor[provider.EVPNInstanceProvider](),
	"InterfaceProvider":        reflect.TypeFor[provider.InterfaceProvider](),
	"ISISProvider":             reflect.TypeFor[provider.ISISProvider](),
	"LLDPProvider":             reflect.TypeFor[provider.LLDPProvider](),
	"ManagementAccessProvider": reflect.TypeFor[provider.ManagementAccessProvider](),
	"NTPProvider":              reflect.TypeFor[provider.NTPProvider](),
	"NVEProvider":              reflect.TypeFor[provider.NVEProvider](),
	"OSPFProvider":             reflect.TypeFor[provider.OSPFProvider](),
	"PIMProvider":              reflect.TypeFor[provider.PIMProvider](),
	"PrefixSetProvider":        reflect.TypeFor[provider.PrefixSetProvider](),
	"RoutingPolicyProvider":    reflect.TypeFor[provider.RoutingPolicyProvider](),
	"SNMPProvider":             reflect.TypeFor[provider.SNMPProvider](),
	"SyslogProvider":           reflect.TypeFor[provider.SyslogProvider](),
	"UserProvider":             reflect.TypeFor[provider.UserProvider](),
	"VLANProvider":             reflect.TypeFor[provider.VLANProvider](),
	"VRFProvider":              reflect.TypeFor[provider.VRFProvider](),
}

type providerInfo struct {
	// name is the short identifier used as map key (e.g., "NXOS")
	name string
	// displayName is the human-readable name for documentation (e.g., "Cisco NX-OS")
	displayName string
	// description is shown in the provider section of the generated docs
	description string
	// apiDir is the path to the provider's API directory (e.g., "./api/cisco/nx/v1alpha1")
	// Used to scan for provider-specific config configurations.
	apiDir string
	// instance is the provider implementation to check for interface support
	instance provider.Provider
}

// providers lists all registered providers to check for interface implementations.
// When adding a new provider, add it here and the matrix will automatically detect
// which interfaces it implements.
var providers = []providerInfo{
	{
		name:        "NXOS",
		displayName: "Cisco NX-OS",
		description: "Cisco NX-OS provides comprehensive support for network configuration through the network-operator.",
		apiDir:      "api/cisco/nx/v1alpha1",
		instance:    nxos.NewProvider(),
	},
	{
		name:        "IOS-XR",
		displayName: "Cisco IOS-XR",
		description: "Cisco IOS-XR support is currently in early development.",
		apiDir:      "api/cisco/xr/v1alpha1",
		instance:    iosxr.NewProvider(),
	},
	{
		name:        "OpenConfig",
		displayName: "OpenConfig",
		description: "OpenConfig provides a vendor-neutral configuration interface using standard OpenConfig YANG models.",
		apiDir:      "", // OpenConfig has no provider-specific API configurations
		instance:    openconfig.NewProvider(),
	},
}

const (
	readmeBeginMarker = "<!-- BEGIN provider-summary -->"
	readmeEndMarker   = "<!-- END provider-summary -->"

	coreAPIDir       = "api/core/v1alpha1"
	dependencyPrefix = "Register"
	dependencySuffix = "Dependency"
)

// baseInterfaces are provider interfaces that don't map to a specific API type
// and should be excluded from the compatibility matrix.
var baseInterfaces = map[string]bool{
	"Provider":             true,
	"ProvisioningProvider": true,
	"DeviceProvider":       true,
}

func main() {
	// Verify interfaceMap has all entries by checking source code before generating
	if err := verifyInterfaceMapComplete(); err != nil {
		fmt.Fprintf(os.Stderr, "Consistency check failed: %v\n", err)
		os.Exit(1)
	}

	interfaceNames := slices.Sorted(maps.Keys(interfaceMap))

	// Check implementations for each provider
	implementations := make(map[string]map[string]bool)
	counts := make(map[string]int)
	configurations := make(map[string][]providerSpecificType)
	standaloneTypes := make(map[string][]string)

	for _, prov := range providers {
		implementations[prov.name] = make(map[string]bool)
		providerType := reflect.TypeOf(prov.instance)

		for _, ifaceName := range interfaceNames {
			interfaceType := interfaceMap[ifaceName]
			implements := providerType.Implements(interfaceType)
			implementations[prov.name][ifaceName] = implements

			if implements {
				counts[prov.name]++
			}
		}

		exts, err := scanConfigExtensions(prov.apiDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to scan config configurations for %s: %v\n", prov.name, err)
		}
		configurations[prov.name] = exts

		standalone, err := scanStandaloneTypes(prov.apiDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to scan standalone types for %s: %v\n", prov.name, err)
		}
		standaloneTypes[prov.name] = standalone
	}

	// Build mapping from Register*Dependency core types to provider interface names
	coreInfo, err := buildCoreTypeMapping()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to build core type mapping: %v\n", err)
		coreInfo = &coreTypeInfo{
			coreTypeToInterface: make(map[string]string),
			interfaceToKind:     make(map[string]string),
		}
	}

	// Generate markdown documentation
	fmt.Println("<!--")
	fmt.Println("SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors")
	// Split the SPDX identifier to prevent REUSE tool from detecting it in this source file.
	// Otherwise it parses 'Apache-2.0")' (including the quote and paren) as an invalid license.
	fmt.Println("SPDX-License" + "-Identifier: Apache-2.0")
	fmt.Println("-->")
	fmt.Println()
	fmt.Println("# Provider Compatibility Matrix")
	fmt.Println()
	fmt.Println("This document provides a detailed overview of which API types are supported by each network device provider.")
	fmt.Println()
	fmt.Println("<!-- To regenerate this matrix, run: make provider-matrix -->")
	fmt.Println()

	fmt.Println("## Compatibility Matrix")
	fmt.Println()

	fmt.Print("| Core Kind |")
	for _, prov := range providers {
		fmt.Printf(" %s |", prov.displayName)
	}
	fmt.Println()

	fmt.Print("|-----------|")
	for range providers {
		fmt.Print("--------|")
	}
	fmt.Println()

	for _, ifaceName := range interfaceNames {
		kindName := formatDisplayName(ifaceName)
		if k, ok := coreInfo.interfaceToKind[ifaceName]; ok {
			kindName = k
		}
		fmt.Printf("| `%s` |", kindName)

		for _, prov := range providers {
			if implementations[prov.name][ifaceName] {
				fmt.Print(" ✅ |")
			} else {
				fmt.Print(" — |")
			}
		}
		fmt.Println()
	}

	fmt.Println()
	fmt.Println("**Legend:**")
	fmt.Println("- ✅ Supported")
	fmt.Println("- — Not supported")
	fmt.Println()

	// Generate per-provider detail sections
	total := len(interfaceMap)
	for _, prov := range providers {
		fmt.Printf("## %s\n", prov.displayName)
		fmt.Println()
		fmt.Println(prov.description)
		fmt.Println()
		fmt.Printf("**Core API types:** %d / %d\n", counts[prov.name], total)
		if len(configurations[prov.name]) > 0 || len(standaloneTypes[prov.name]) > 0 {
			fmt.Println()
			fmt.Println("**Provider-specific types:**")
			fmt.Println()
			fmt.Println("| Kind | Category |")
			fmt.Println("|------|----------|")
			for _, ext := range configurations[prov.name] {
				displayName := ext.coreType
				if ifaceName, ok := coreInfo.coreTypeToInterface[ext.coreType]; ok {
					if k, ok := coreInfo.interfaceToKind[ifaceName]; ok {
						displayName = k
					} else {
						displayName = formatDisplayName(ifaceName)
					}
				}
				fmt.Printf("| `%s` | Extends core type `%s` |\n", ext.kind, displayName)
			}
			for _, kind := range standaloneTypes[prov.name] {
				fmt.Printf("| `%s` | Provider-exclusive |\n", kind)
			}
		}
		fmt.Println()
	}

	fmt.Println("## Contributing")
	fmt.Println()
	fmt.Println("To add support for a new API type or provider:")
	fmt.Println()
	fmt.Println("1. Define the provider interface in `internal/provider/`")
	fmt.Println("2. Implement the interface methods in your provider package")
	fmt.Println("3. Run `make provider-matrix` to regenerate this document")
	fmt.Println("4. Submit a pull request with your changes")
	fmt.Println()
	fmt.Println("The matrix is automatically generated by checking which provider interfaces")
	fmt.Println("each provider implements, ensuring accuracy and eliminating manual maintenance.")

	if err := updateReadmeSummary(counts); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to update README.md: %v\n", err)
		os.Exit(1)
	}
}

func updateReadmeSummary(counts map[string]int) error {
	data, err := os.ReadFile("README.md")
	if err != nil {
		return fmt.Errorf("reading README.md: %w", err)
	}

	content := string(data)
	beginIdx := strings.Index(content, readmeBeginMarker)
	endIdx := strings.Index(content, readmeEndMarker)

	if beginIdx == -1 || endIdx == -1 {
		return fmt.Errorf("markers %q and %q not found in README.md", readmeBeginMarker, readmeEndMarker)
	}

	if endIdx <= beginIdx {
		return errors.New("end marker appears before begin marker in README.md")
	}

	// Build the new summary table
	total := len(interfaceMap)
	var summary strings.Builder
	summary.WriteString(readmeBeginMarker + "\n")
	summary.WriteString("| Provider | Supported API Types |\n")
	summary.WriteString("|----------|---------------------|\n")
	for _, prov := range providers {
		fmt.Fprintf(&summary, "| %s | %d / %d |\n", prov.displayName, counts[prov.name], total)
	}
	summary.WriteString(readmeEndMarker)

	// Replace content between markers (inclusive)
	newContent := content[:beginIdx] + summary.String() + content[endIdx+len(readmeEndMarker):]

	if err := os.WriteFile("README.md", []byte(newContent), 0644); err != nil { //nolint:gosec // hardcoded path
		return fmt.Errorf("writing README.md: %w", err)
	}

	return nil
}

// formatDisplayName converts an interface name like "BGPPeerProvider" to "BGP Peer"
func formatDisplayName(ifaceName string) string {
	// Remove "Provider" suffix
	name := strings.TrimSuffix(ifaceName, "Provider")

	// Insert spaces before uppercase letters (except at start or consecutive uppercase)
	var result strings.Builder
	for i, r := range name {
		if i > 0 && r >= 'A' && r <= 'Z' {
			// Add space if previous char is lowercase, or if next char is lowercase (for acronyms)
			if name[i-1] >= 'a' && name[i-1] <= 'z' {
				result.WriteByte(' ')
			} else if i < len(name)-1 && name[i+1] >= 'a' && name[i+1] <= 'z' {
				result.WriteByte(' ')
			}
		}
		result.WriteRune(r)
	}
	return result.String()
}

// verifyInterfaceMapComplete checks that interfaceMap contains all *Provider interfaces
// defined in internal/provider/ and there are no stale entries.
func verifyInterfaceMapComplete() error {
	sourceInterfaces, err := scanProviderInterfaces("internal/provider")
	if err != nil {
		return fmt.Errorf("failed to discover provider interfaces: %w", err)
	}

	// Check for interfaces in source but missing from interfaceMap
	var missing []string
	for iface := range sourceInterfaces {
		if _, exists := interfaceMap[iface]; !exists {
			missing = append(missing, iface)
		}
	}

	// Check for stale entries in interfaceMap (no longer in source)
	var stale []string
	for iface := range interfaceMap {
		if !sourceInterfaces[iface] {
			stale = append(stale, iface)
		}
	}

	if len(missing) > 0 || len(stale) > 0 {
		var errMsg strings.Builder
		errMsg.WriteString("interfaceMap is out of sync with source:\n")

		if len(missing) > 0 {
			slices.Sort(missing)
			fmt.Fprintf(&errMsg, "  Missing (defined in source but not in interfaceMap): %v\n", missing)
			errMsg.WriteString("  -> Add these to interfaceMap\n")
		}

		if len(stale) > 0 {
			slices.Sort(stale)
			fmt.Fprintf(&errMsg, "  Stale (in interfaceMap but not defined in source): %v\n", stale)
			errMsg.WriteString("  -> Remove these from interfaceMap\n")
		}

		return errors.New(errMsg.String())
	}

	return nil
}

// scanProviderInterfaces parses Go source files in the provider package and returns
// all interface names matching *Provider (excluding base interfaces like Provider itself).
func scanProviderInterfaces(providerDir string) (map[string]bool, error) {
	entries, err := os.ReadDir(providerDir)
	if err != nil {
		return nil, err
	}

	fset := token.NewFileSet()
	interfaces := make(map[string]bool)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		if strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		filePath := filepath.Join(providerDir, entry.Name())
		file, err := parser.ParseFile(fset, filePath, nil, 0)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", filePath, err)
		}

		ast.Inspect(file, func(n ast.Node) bool {
			typeSpec, ok := n.(*ast.TypeSpec)
			if !ok {
				return true
			}

			_, ok = typeSpec.Type.(*ast.InterfaceType)
			if !ok {
				return true
			}

			name := typeSpec.Name.Name
			if !strings.HasSuffix(name, "Provider") {
				return true
			}

			if baseInterfaces[name] {
				return true
			}

			interfaces[name] = true

			return true
		})
	}

	return interfaces, nil
}

type providerSpecificType struct {
	kind     string
	coreType string
}

// extractKind returns the Kind name from a Register*Dependency function name.
// e.g., "RegisterAccessControlListDependency" -> "AccessControlList"
func extractKind(funcName string) string {
	return strings.TrimSuffix(strings.TrimPrefix(funcName, dependencyPrefix), dependencySuffix)
}

// coreTypeInfo holds the mapping between core API types, interface names, and Kubernetes Kind names.
type coreTypeInfo struct {
	// e.g., "NetworkVirtualizationEdge" -> "NVEProvider"
	coreTypeToInterface map[string]string
	// e.g., "NVEProvider" -> "NetworkVirtualizationEdge"
	interfaceToKind map[string]string
}

// buildCoreTypeMapping scans core API type files to build mappings between
// Register*Dependency function names, interfaceMap keys, and Kubernetes Kind names.
//
// The Register*Dependency function names contain the Kubernetes Kind name
// (e.g., RegisterAccessControlListDependency -> Kind="AccessControlList", interface="ACLProvider").
// The filename convention correlates files to interface names:
// e.g., nve_types.go -> "nve" matches "NVE" in "NVEProvider".
func buildCoreTypeMapping() (*coreTypeInfo, error) {
	entries, err := os.ReadDir(coreAPIDir)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", coreAPIDir, err)
	}

	fset := token.NewFileSet()
	info := &coreTypeInfo{
		coreTypeToInterface: make(map[string]string),
		interfaceToKind:     make(map[string]string),
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_types.go") {
			continue
		}
		if strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		// Derive interface name from filename: nve_types.go -> nve -> NVEProvider
		baseName := strings.TrimSuffix(entry.Name(), "_types.go")
		baseName = strings.ReplaceAll(baseName, "_", "")

		var matchedInterface string
		for ifaceName := range interfaceMap {
			stripped := strings.TrimSuffix(ifaceName, "Provider")
			if strings.EqualFold(baseName, stripped) {
				matchedInterface = ifaceName
				break
			}
		}

		if matchedInterface == "" {
			continue
		}

		filePath := filepath.Join(coreAPIDir, entry.Name())
		file, err := parser.ParseFile(fset, filePath, nil, 0)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", filePath, err)
		}

		var inspectErr error
		ast.Inspect(file, func(n ast.Node) bool {
			funcDecl, ok := n.(*ast.FuncDecl)
			if !ok {
				return true
			}

			name := funcDecl.Name.Name
			if strings.HasPrefix(name, dependencyPrefix) && strings.HasSuffix(name, dependencySuffix) {
				coreType := extractKind(name)
				info.coreTypeToInterface[coreType] = matchedInterface
				if existing, ok := info.interfaceToKind[matchedInterface]; ok && existing != coreType {
					inspectErr = fmt.Errorf("duplicate mapping for %s: %q and %q in %s", matchedInterface, existing, coreType, entry.Name())
					return false
				}
				info.interfaceToKind[matchedInterface] = coreType
			}

			return true
		})
		if inspectErr != nil {
			return nil, inspectErr
		}
	}

	// Verify that each derived Kind name is a real registered type.
	// This catches cases where a Register*Dependency function is named
	// differently than the actual Kubernetes Kind.
	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("adding core types to scheme: %w", err)
	}
	for iface, kind := range info.interfaceToKind {
		gvk := v1alpha1.GroupVersion.WithKind(kind)
		if _, err := scheme.New(gvk); err != nil {
			return nil, fmt.Errorf("derived Kind %q for %s is not a registered type: %w", kind, iface, err)
		}
	}

	return info, nil
}

// scanConfigExtensions parses Go source files in a provider's API directory and returns
// all config configurations (types that call Register*Dependency in their init function).
func scanConfigExtensions(apiDir string) ([]providerSpecificType, error) {
	if apiDir == "" {
		return nil, nil
	}

	entries, err := os.ReadDir(apiDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	fset := token.NewFileSet()
	var configurations []providerSpecificType

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		if strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		filePath := filepath.Join(apiDir, entry.Name())
		file, err := parser.ParseFile(fset, filePath, nil, 0)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", filePath, err)
		}

		ast.Inspect(file, func(n ast.Node) bool {
			funcDecl, ok := n.(*ast.FuncDecl)
			if !ok || funcDecl.Name.Name != "init" {
				return true
			}

			ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
				callExpr, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}

				selExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}

				funcName := selExpr.Sel.Name
				if !strings.HasPrefix(funcName, dependencyPrefix) || !strings.HasSuffix(funcName, dependencySuffix) {
					return true
				}

				coreType := extractKind(funcName)

				if len(callExpr.Args) > 0 {
					if argCall, ok := callExpr.Args[0].(*ast.CallExpr); ok {
						if argSel, ok := argCall.Fun.(*ast.SelectorExpr); ok {
							if argSel.Sel.Name == "WithKind" && len(argCall.Args) > 0 {
								if lit, ok := argCall.Args[0].(*ast.BasicLit); ok {
									kind := strings.Trim(lit.Value, `"`)
									configurations = append(configurations, providerSpecificType{
										kind:     kind,
										coreType: coreType,
									})
								}
							}
						}
					}
				}

				return true
			})

			return true
		})
	}

	slices.SortFunc(configurations, func(a, b providerSpecificType) int {
		return strings.Compare(a.coreType, b.coreType)
	})

	return configurations, nil
}

// scanStandaloneTypes parses Go source files in a provider's API directory and returns
// all standalone types (CRDs that are registered but don't extend core types via Register*Dependency).
func scanStandaloneTypes(apiDir string) ([]string, error) {
	if apiDir == "" {
		return nil, nil
	}

	entries, err := os.ReadDir(apiDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	fset := token.NewFileSet()
	var standaloneKinds []string

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_types.go") {
			continue
		}
		if strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		filePath := filepath.Join(apiDir, entry.Name())
		file, err := parser.ParseFile(fset, filePath, nil, 0)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", filePath, err)
		}

		hasRegisterDependency := false
		ast.Inspect(file, func(n ast.Node) bool {
			callExpr, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if selExpr, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
				if strings.HasPrefix(selExpr.Sel.Name, dependencyPrefix) && strings.HasSuffix(selExpr.Sel.Name, dependencySuffix) {
					hasRegisterDependency = true
					return false
				}
			}
			return true
		})

		if hasRegisterDependency {
			continue
		}

		ast.Inspect(file, func(n ast.Node) bool {
			callExpr, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			selExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			if ident, ok := selExpr.X.(*ast.Ident); ok {
				if ident.Name == "SchemeBuilder" && selExpr.Sel.Name == "Register" {
					for _, arg := range callExpr.Args {
						if unary, ok := arg.(*ast.UnaryExpr); ok {
							if composite, ok := unary.X.(*ast.CompositeLit); ok {
								if ident, ok := composite.Type.(*ast.Ident); ok {
									if !strings.HasSuffix(ident.Name, "List") {
										standaloneKinds = append(standaloneKinds, ident.Name)
									}
								}
							}
						}
					}
				}
			}
			return true
		})
	}

	slices.Sort(standaloneKinds)
	return standaloneKinds, nil
}
