// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"
)

// RootOptions holds the global configuration shared across all subcommands.
type RootOptions struct {
	ConfigFlags   *genericclioptions.ConfigFlags
	IOStreams     genericiooptions.IOStreams
	AllNamespaces bool
}

// NewCmdNet returns the root "kubectl net" command with all subcommands registered.
func NewCmdNet(streams genericiooptions.IOStreams) *cobra.Command {
	o := &RootOptions{
		ConfigFlags: genericclioptions.NewConfigFlags(true),
		IOStreams:   streams,
	}

	cmd := &cobra.Command{
		Use:          "net",
		Short:        "Manage network-operator resources",
		SilenceUsage: true,
		Annotations: map[string]string{
			cobra.CommandDisplayNameAnnotation: "kubectl net",
		},
	}

	cmd.SetOut(streams.Out)
	cmd.SetErr(streams.ErrOut)

	o.ConfigFlags.AddFlags(cmd.PersistentFlags())
	cmd.PersistentFlags().BoolVarP(&o.AllNamespaces, "all-namespaces", "A", false, "If present, list requested objects across all namespaces")

	cmd.CompletionOptions.DisableDefaultCmd = true
	cmd.AddCommand(
		NewCmdGet(o),
		NewCmdPause(o, true),
		NewCmdPause(o, false),
		NewCmdMove(o),
		NewCmdCompletion(cmd),
	)

	return cmd
}
