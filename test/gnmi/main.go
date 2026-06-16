// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ironcore-dev/gnmi-test-server/testserver"
)

func main() {
	// Parse command line flags
	port := flag.Int("port", 9339, "The gRPC server port")
	httpPort := flag.Int("http-port", 8000, "The HTTP server port")
	nxos := flag.Bool("nxos", false, "Enable NX-OS behavior (strip DME markers)")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Build server options
	opts := []testserver.ServerOption{
		testserver.WithGRPCPort(*port),
		testserver.WithHTTPPort(*httpPort),
		testserver.WithBindAddress("0.0.0.0"),
	}
	if *nxos {
		opts = append(opts, testserver.WithNXOSBehavior())
	}

	// Start the server using the reusable NewTestServer function
	// Bind to 0.0.0.0 to accept connections from other pods in the cluster
	server, grpcAddr, httpAddr, err := testserver.NewTestServer(ctx, opts...)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	log.Printf("gRPC server listening on %s", grpcAddr)
	log.Printf("HTTP server listening on %s", httpAddr)
	log.Printf("HTTP endpoint available at: /v1/state")
	log.Printf("Use --port flag to specify a different gRPC port (default: 9339)")
	log.Printf("Use --http-port flag to specify a different HTTP port (default: 8000)")
	log.Printf("Available services: GNMI")

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")
	if err := server.Close(); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}
}
