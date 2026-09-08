// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"

	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/ironcore-dev/network-operator/kubectl-net/pkg/cmd"
)

var version = "dev"

func main() {
	flags := pflag.NewFlagSet("kubectl-net", pflag.ExitOnError)
	pflag.CommandLine = flags

	root := cmd.NewCmdNet(genericiooptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}, version)
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
