// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/ironcore-dev/network-operator/internal/deviceutil"
	"github.com/ironcore-dev/network-operator/internal/transport/nxapi"
)

func TestReprovision(t *testing.T) {
	t.Run("success with connection drop on reload", func(t *testing.T) {
		var requests [][]string
		called := 0

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var cmds []struct {
				Params struct {
					Cmd string `json:"cmd"`
				} `json:"params"`
			}
			if err := json.NewDecoder(r.Body).Decode(&cmds); err != nil {
				t.Fatalf("failed to decode request: %v", err)
			}

			batch := make([]string, len(cmds))
			for i, c := range cmds {
				batch[i] = c.Params.Cmd
			}
			requests = append(requests, batch)
			called++

			if called == 1 {
				w.Header().Set("Content-Type", "application/json-rpc")
				fmt.Fprint(w, `[
					{"jsonrpc":"2.0","result":null,"id":1},
					{"jsonrpc":"2.0","result":null,"id":2}
				]`)
				return
			}

			// Simulate device going down by closing connection abruptly.
			hijacker, ok := w.(http.Hijacker)
			if !ok {
				t.Fatal("server does not support hijacking")
			}
			conn, _, err := hijacker.Hijack()
			if err != nil {
				t.Fatalf("hijack failed: %v", err)
			}
			conn.Close()
		}))
		defer srv.Close()

		_, port, _ := net.SplitHostPort(srv.Listener.Addr().String()) //nolint:errcheck // httptest address is always host:port
		conn := &deviceutil.Connection{Address: srv.Listener.Addr().String(), Username: "admin", Password: "secret"}
		client, err := nxapi.NewClient(conn, 0, nxapi.WithPort(port))
		if err != nil {
			t.Fatalf("failed to create nxapi client: %v", err)
		}

		p := &Provider{nxapi: client}
		if err := p.Reprovision(t.Context(), conn); err != nil {
			t.Fatalf("Reprovision returned unexpected error: %v", err)
		}

		if len(requests) != 2 {
			t.Fatalf("expected 2 NXAPI requests, got %d", len(requests))
		}
		if !slices.Equal(requests[0], []string{"boot poap enable", "copy running-config startup-config"}) {
			t.Errorf("prep batch = %v", requests[0])
		}
		if !slices.Equal(requests[1], []string{"reload"}) {
			t.Errorf("reload request = %v", requests[1])
		}
	})

	t.Run("prep batch RPC error fails", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json-rpc")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `[{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid command"},"id":1}]`)
		}))
		defer srv.Close()

		_, port, _ := net.SplitHostPort(srv.Listener.Addr().String()) //nolint:errcheck // httptest address is always host:port
		conn := &deviceutil.Connection{Address: srv.Listener.Addr().String(), Username: "admin", Password: "secret"}
		client, err := nxapi.NewClient(conn, 0, nxapi.WithPort(port))
		if err != nil {
			t.Fatalf("failed to create nxapi client: %v", err)
		}

		p := &Provider{nxapi: client}
		if err := p.Reprovision(t.Context(), conn); err == nil {
			t.Fatal("expected error from prep batch, got nil")
		}
	})

	t.Run("reload RPC error fails", func(t *testing.T) {
		called := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called++
			w.Header().Set("Content-Type", "application/json-rpc")
			if called == 1 {
				fmt.Fprint(w, `[
					{"jsonrpc":"2.0","result":null,"id":1},
					{"jsonrpc":"2.0","result":null,"id":2}
				]`)
				return
			}
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `[{"jsonrpc":"2.0","error":{"code":-32602,"message":"Permission denied"},"id":1}]`)
		}))
		defer srv.Close()

		_, port, _ := net.SplitHostPort(srv.Listener.Addr().String()) //nolint:errcheck // httptest address is always host:port
		conn := &deviceutil.Connection{Address: srv.Listener.Addr().String(), Username: "admin", Password: "secret"}
		client, err := nxapi.NewClient(conn, 0, nxapi.WithPort(port))
		if err != nil {
			t.Fatalf("failed to create nxapi client: %v", err)
		}

		p := &Provider{nxapi: client}
		if err := p.Reprovision(t.Context(), conn); err == nil {
			t.Fatal("expected error from reload RPC failure, got nil")
		}
	})
}
