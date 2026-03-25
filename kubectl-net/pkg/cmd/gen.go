// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package cmd

// generate-resources scans the network-operator API types for kubebuilder
// resource markers (+kubebuilder:resource:path=, singular=, shortName=) and
// produces resources.go with the ResourceDef slice used by all subcommands.
// Run "go generate ./pkg/cmd" to regenerate after API type changes.
//go:generate go run ../../tools/generate-resources --api-root ../../../api --out ./resources.go
