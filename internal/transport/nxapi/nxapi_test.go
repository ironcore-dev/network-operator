// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxapi

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/ironcore-dev/network-operator/internal/deviceutil"
)

func TestUri(t *testing.T) {
	tests := []struct {
		desc      string
		conn      *deviceutil.Connection
		wantProto string
	}{
		{
			desc:      "no TLS uses http",
			conn:      &deviceutil.Connection{Address: "10.0.0.1:80"},
			wantProto: "http",
		},
		{
			desc:      "with TLS uses https",
			conn:      &deviceutil.Connection{Address: "10.0.0.1:443", TLS: &tls.Config{MinVersion: tls.VersionTLS12}},
			wantProto: "https",
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			c, err := NewClient(test.conn, 0)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if c.url.Scheme != test.wantProto {
				t.Errorf("scheme = %q, want %q", c.url.Scheme, test.wantProto)
			}
			if c.url.Host != test.conn.Address {
				t.Errorf("host = %q, want %q", c.url.Host, test.conn.Address)
			}
			if c.url.Path != "/ins" {
				t.Errorf("path = %q, want %q", c.url.Path, "/ins")
			}
		})
	}
}

func TestEncode(t *testing.T) {
	tests := []struct {
		desc string
		cmds []string
		want string
	}{
		{
			desc: "single show command",
			cmds: []string{"show crypto ca certificates"},
			want: `
[
  {
    "jsonrpc": "2.0",
    "method": "cli",
    "params": {
      "cmd": "show crypto ca certificates",
      "version": 1
    },
    "id": 1
  }
]`,
		},
		{
			desc: "multiple conf commands",
			cmds: []string{"crypto ca trustpoint mytrustpoint", "crypto ca import mytrustpoint pkcs12 bootflash:server.pfx cisco123"},
			want: `
[
  {
    "jsonrpc": "2.0",
    "method": "cli",
    "params": {
      "cmd": "crypto ca trustpoint mytrustpoint",
      "version": 1
    },
    "id": 1
  },
  {
    "jsonrpc": "2.0",
    "method": "cli",
    "params": {
      "cmd": "crypto ca import mytrustpoint pkcs12 bootflash:server.pfx cisco123",
      "version": 1
    },
    "id": 2
  }
]`,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			r := NewRequest(test.cmds...)
			b, err := r.Encode()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			var got bytes.Buffer
			if err := json.Compact(&got, b); err != nil {
				t.Fatalf("json.Compact error: %v", err)
			}
			var want bytes.Buffer
			if err := json.Compact(&want, []byte(test.want)); err != nil {
				t.Fatalf("json.Compact error: %v", err)
			}
			if got.String() != want.String() {
				t.Errorf("Encode() = %s, want %s", got.String(), want.String())
			}
		})
	}
}

func TestDecode(t *testing.T) {
	tests := []struct {
		desc    string
		body    string
		wantLen int
		wantErr bool
	}{
		{
			desc:    "single object normalized to slice",
			body:    `{"jsonrpc":"2.0","result":{"body":{"key":"val"}},"id":1}`,
			wantLen: 1,
		},
		{
			desc:    "array decoded as-is",
			body:    `[{"jsonrpc":"2.0","result":{"body":{"a":1}},"id":1},{"jsonrpc":"2.0","result":{"body":{"b":2}},"id":2}]`,
			wantLen: 2,
		},
		{
			desc:    "invalid JSON returns error",
			body:    `not json`,
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			got, err := decode([]byte(test.body))
			if test.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != test.wantLen {
				t.Fatalf("len = %d, want %d", len(got), test.wantLen)
			}
		})
	}
}

func TestDo(t *testing.T) {
	tests := []struct {
		desc           string
		statusCode     int
		serverResponse string
		wantResultLen  int
		wantRPCErrors  int
		wantHTTPError  bool
	}{
		{
			desc:           "2xx single command",
			statusCode:     http.StatusOK,
			serverResponse: `{"jsonrpc":"2.0","result":{"body":{"version":"9.3(5)"}},"id":1}`,
			wantResultLen:  1,
		},
		{
			desc:       "2xx multiple commands",
			statusCode: http.StatusOK,
			serverResponse: `[
				{"jsonrpc":"2.0","result":{"body":{"a":1}},"id":1},
				{"jsonrpc":"2.0","result":{"body":{"b":2}},"id":2}
			]`,
			wantResultLen: 2,
		},
		{
			desc:           "2xx null result for config command",
			statusCode:     http.StatusOK,
			serverResponse: `{"jsonrpc":"2.0","result":null,"id":1}`,
			wantResultLen:  1,
		},
		{
			desc:           "non-2xx single RPC error",
			statusCode:     http.StatusBadRequest,
			serverResponse: `{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid params","data":{"msg":"bad\n"}},"id":1}`,
			wantRPCErrors:  1,
		},
		{
			desc:       "non-2xx multiple RPC errors",
			statusCode: http.StatusBadRequest,
			serverResponse: `[
				{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid params"},"id":1},
				{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid command"},"id":2}
			]`,
			wantRPCErrors: 2,
		},
		{
			desc:       "non-2xx with no error fields falls back to HTTPError",
			statusCode: http.StatusBadRequest,
			serverResponse: `[
				{"jsonrpc":"2.0","result":{"body":{}},"id":1}
			]`,
			wantHTTPError: true,
		},
		{
			desc:       "401 HTML from nginx proxy returns HTTPError",
			statusCode: http.StatusUnauthorized,
			serverResponse: `<html>
<head><title>401 Authorization Required</title></head>
<body>
<center><h1>401 Authorization Required</h1></center>
<hr><center>nginx/1.25.4</center>
</body>
</html>`,
			wantHTTPError: true,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if ct := r.Header.Get("Content-Type"); ct != "application/json-rpc" {
					t.Errorf("Content-Type = %q, want %q", ct, "application/json-rpc")
				}
				if cc := r.Header.Get("Cache-Control"); cc != "no-cache" {
					t.Errorf("Cache-Control = %q, want %q", cc, "no-cache")
				}
				user, pass, ok := r.BasicAuth()
				if !ok {
					t.Error("Basic auth header not set")
				} else if user != "admin" || pass != "secret" {
					t.Errorf("BasicAuth = (%q, %q), want (\"admin\", \"secret\")", user, pass)
				}
				w.Header().Set("Content-Type", "application/json-rpc")
				w.WriteHeader(test.statusCode)
				fmt.Fprint(w, test.serverResponse)
			}))
			defer srv.Close()

			conn := &deviceutil.Connection{
				Address:  srv.Listener.Addr().String(),
				Username: "admin",
				Password: "secret",
			}
			c, err := NewClient(conn, 0)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			results, err := c.Do(t.Context(), NewRequest("show version"))
			if test.wantRPCErrors > 0 || test.wantHTTPError {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if test.wantRPCErrors > 0 {
					var rpcErrs RPCErrors
					if !errors.As(err, &rpcErrs) {
						t.Fatalf("expected RPCErrors, got %T: %v", err, err)
					}
					if len(rpcErrs) != test.wantRPCErrors {
						t.Errorf("len(RPCErrors) = %d, want %d", len(rpcErrs), test.wantRPCErrors)
					}
				}
				if test.wantHTTPError {
					var httpErr *HTTPError
					if !errors.As(err, &httpErr) {
						t.Fatalf("expected *HTTPError, got %T: %v", err, err)
					}
					if httpErr.Code != test.statusCode {
						t.Errorf("HTTPError.Code = %d, want %d", httpErr.Code, test.statusCode)
					}
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(results) != test.wantResultLen {
				t.Fatalf("len(results) = %d, want %d", len(results), test.wantResultLen)
			}
		})
	}
}

func TestIsTransportError(t *testing.T) {
	tests := []struct {
		desc string
		err  error
		want bool
	}{
		{
			desc: "nil is not a transport error",
			err:  nil,
			want: false,
		},
		{
			desc: "RPCError is not a transport error",
			err:  &RPCError{Code: -32602, Message: "Invalid params"},
			want: false,
		},
		{
			desc: "RPCErrors is not a transport error",
			err:  RPCErrors{&RPCError{Code: -32602, Message: "Invalid params"}},
			want: false,
		},
		{
			desc: "HTTPError is not a transport error",
			err:  &HTTPError{Code: 401, Body: []byte("unauthorized")},
			want: false,
		},
		{
			desc: "generic error is not a transport error",
			err:  errors.New("some logic error"),
			want: false,
		},
		{
			desc: "io.EOF is a transport error",
			err:  io.EOF,
			want: true,
		},
		{
			desc: "io.ErrUnexpectedEOF is a transport error",
			err:  io.ErrUnexpectedEOF,
			want: true,
		},
		{
			desc: "wrapped io.EOF is a transport error",
			err:  fmt.Errorf("request failed: %w", io.EOF),
			want: true,
		},
		{
			desc: "net.Error is a transport error",
			err:  &netError{msg: "i/o timeout"},
			want: true,
		},
		{
			desc: "url.Error wrapping net.Error is a transport error",
			err:  &url.Error{Op: "Post", URL: "http://x/ins", Err: &netError{msg: "i/o timeout"}},
			want: true,
		},
		{
			desc: "wrapped net.Error is a transport error",
			err:  fmt.Errorf("read tcp: %w", &netError{msg: "connection reset by peer"}),
			want: true,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			got := IsTransportError(test.err)
			if got != test.want {
				t.Errorf("IsTransportError(%v) = %t, want %t", test.err, got, test.want)
			}
		})
	}
}

// netError is a mock net.Error for testing.
type netError struct{ msg string }

func (e *netError) Error() string   { return e.msg }
func (e *netError) Timeout() bool   { return false }
func (e *netError) Temporary() bool { return false }
