// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

// nxapi is a minimal CLI tool for sending NX-API JSON-RPC commands to a
// Cisco NX-OS device and printing the results.
//
// Usage:
//
//	go run ./hack/nxapi [flags] <cmd> [<cmd> ...]
//
// Example:
//
//	go run ./hack/nxapi -address 10.0.0.1:443 "show version" "show interface brief"
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/ironcore-dev/network-operator/internal/deviceutil"
	"github.com/ironcore-dev/network-operator/internal/transport/nxapi"
)

var fs = flag.NewFlagSet("nxapi", flag.ContinueOnError)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: nxapi [flags] <cmd> [<cmd> ...]\n\n")
	fmt.Fprintf(os.Stderr, "Sends NX-API JSON-RPC commands to a Cisco NX-OS device.\n\n")
	fmt.Fprintf(os.Stderr, "Flags:\n")
	fs.PrintDefaults()
	fmt.Fprintf(os.Stderr, "\nExample:\n")
	fmt.Fprintf(os.Stderr, "  nxapi -address 10.0.0.1:8080 \"show version\" \"show interface brief\"\n")
}

func main() {
	fs.Usage = usage

	address := fs.String("address", "localhost:8080", "device address (host:port)")
	username := fs.String("username", "admin", "NX-API username")
	password := fs.String("password", "admin", "NX-API password")

	if err := fs.Parse(os.Args[1:]); err != nil {
		// flag.ContinueOnError: -h/--help prints usage and returns ErrHelp;
		// other parse errors are already printed by the FlagSet.
		return
	}

	cmds := fs.Args()
	if len(cmds) == 0 {
		fs.Usage()
		os.Exit(1)
	}

	conn := &deviceutil.Connection{
		Address:  *address,
		Username: *username,
		Password: *password,
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt /* == syscall.SIGINT */, syscall.SIGTERM)
	defer cancel()

	c, err := nxapi.NewClient(conn, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return
	}

	res, err := c.Do(ctx, nxapi.NewRequest(cmds...))
	if err != nil {
		if errs, ok := errors.AsType[nxapi.RPCErrors](err); ok {
			for _, e := range errs {
				fmt.Fprintf(os.Stderr, "RPC error %d: %s\n", e.Code, e.Message)
				if len(e.Data) > 0 {
					fmt.Fprintf(os.Stderr, "  data: %s\n", e.Data)
				}
			}
			return
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return
	}

	for i, r := range res {
		fmt.Printf("=== [%d] %s ===\n", i+1, cmds[i])
		var pretty any
		if err := json.Unmarshal(r, &pretty); err != nil {
			fmt.Println(string(r))
			continue
		}
		out, err := json.MarshalIndent(pretty, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: failed to pretty-print JSON: %v\n", err)
			fmt.Println(string(r))
			continue
		}
		fmt.Println(string(out))
	}
}
