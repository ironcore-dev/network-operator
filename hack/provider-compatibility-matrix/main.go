// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
)

const (
	providerModulePath = "github.com/ironcore-dev/network-operator/internal/provider"
	outputPath         = "docs/provider-compatibility.md"
	// apiRefPath is the generated API reference; resource names link into it when a
	// matching heading exists.
	apiRefPath = "docs/api-reference/index.md"
)

// kindToAnchor overrides the default lowercased-kind → anchor mapping for kinds
// whose provider interface name does not match the CRD type name used in the
// API reference headings.
var kindToAnchor = map[string]string{
	"ACL": "accesscontrollist",
	"NVE": "networkvirtualizationedge",
}

type providerColumn struct {
	// registeredName is the string a Device CR's spec.provider carries; it is
	// also the column header.
	registeredName string
	// pkgPath is the Go package implementing the provider.
	pkgPath string
}

// providerColumns enumerates the providers rendered in the matrix, one column
// each.
var providerColumns = []providerColumn{
	{registeredName: "openconfig", pkgPath: providerModulePath + "/openconfig"},
	{registeredName: "cisco-nxos-gnmi", pkgPath: providerModulePath + "/cisco/nxos"},
	{registeredName: "cisco-iosxr-gnmi", pkgPath: providerModulePath + "/cisco/iosxr"},
}

func main() {
	m, err := build()
	if err != nil {
		fmt.Fprintln(os.Stderr, "provider-compatibility-matrix:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(outputPath, []byte(m.render()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "provider-compatibility-matrix:", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "provider-compatibility-matrix: wrote", outputPath)
}
