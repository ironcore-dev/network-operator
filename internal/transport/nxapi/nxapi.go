// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/ironcore-dev/network-operator/internal/deviceutil"
)

// The RoundTripFunc type is an adapter to allow the use of
// ordinary functions as [http.RoundTripper]. If f is a function
// with the appropriate signature, RoundTripFunc(f) is a
// [http.RoundTripper] that calls f.
type RoundTripFunc func(*http.Request) (*http.Response, error)

// RoundTrip returns f(r).
func (f RoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

// Client sends JSON-RPC requests to a Cisco NX-OS device via NX-API.
// Use [NewClient] to construct one.
type Client struct {
	client *http.Client
	url    url.URL
}

// Option configures a [Client].
type Option func(*Client) error

// WithPort overrides the port in the connection address.
// This is useful when NX-API is reachable on a different
// port (e.g. 8443) than the default (80/443).
func WithPort(port string) Option {
	return func(c *Client) error {
		host, _, err := net.SplitHostPort(c.url.Host)
		if err != nil {
			return fmt.Errorf("nxapi: failed to parse address: %w", err)
		}
		c.url.Host = net.JoinHostPort(host, port)
		return nil
	}
}

}

// NewClient creates a new [Client] for the given connection.
// If the connection has a TLS configuration set, HTTPS is used; otherwise HTTP.
func NewClient(conn *deviceutil.Connection, timeout time.Duration, opts ...Option) (*Client, error) {
	proto := "http"
	if conn.TLS != nil {
		proto = "https"
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if conn.TLS != nil {
		transport.TLSClientConfig = conn.TLS
	}
	c := &Client{
		client: &http.Client{
			Transport: RoundTripFunc(func(r *http.Request) (*http.Response, error) {
				r.Header.Set("Content-Type", "application/json-rpc")
				r.Header.Set("Cache-Control", "no-cache")
				r.SetBasicAuth(conn.Username, conn.Password)
				return transport.RoundTrip(r)
			}),
			Timeout: timeout,
		},
		url: url.URL{
			Scheme: proto,
			Host:   conn.Address,
			Path:   "/ins",
		},
	}
	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, err
		}
	}
	return c, nil
}

// Do sends a Request to the device and returns one [json.RawMessage] per
// command, in the same order as the request. If any command fails, Do returns
// an [RPCErrors] containing one [RPCError] per failed command; transport and
// HTTP errors are returned directly.
func (c *Client) Do(ctx context.Context, r Request) ([]json.RawMessage, error) {
	b, err := r.Encode()
	if err != nil {
		return nil, fmt.Errorf("nxapi: failed to encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url.String(), bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("nxapi: failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nxapi: failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("nxapi: failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Try to extract JSON-RPC errors from the body, but fall back to a
		// plain HTTPError if the response is not valid JSON-RPC (e.g. a 401
		// from an nginx reverse proxy).
		if res, err := decode(body); err == nil {
			var errs RPCErrors
			for _, r := range res {
				if r.Error != nil {
					errs = append(errs, r.Error)
				}
			}
			if len(errs) > 0 {
				return nil, errs
			}
		}
		return nil, &HTTPError{Code: resp.StatusCode, Body: body}
	}

	res, err := decode(body)
	if err != nil {
		return nil, fmt.Errorf("nxapi: failed to decode response: %w", err)
	}

	msg := make([]json.RawMessage, len(res))
	for i, r := range res {
		msg[i] = r.Body.Data
	}

	return msg, nil
}

// Request is an ordered list of commands to send in a single JSON-RPC batch.
type Request []cmd

// NewRequest creates a Request from plain CLI command strings.
func NewRequest(cmds ...string) Request {
	r := make(Request, len(cmds))
	for i, c := range cmds {
		r[i] = cmd{
			Jsonrpc: "2.0",
			// Other possible values are "cli_ascii" and "cli_array".
			// For now, we only support "cli" which is the default.
			Method: "cli",
			Params: params{
				Cmd: c,
				// Static NX-API version.
				Version: 1,
			},
			ID: i + 1,
		}
	}
	return r
}

// WithRollback sets the error action on each command in the request,
// controlling what NX-OS does if that individual command fails.
func (r Request) WithRollback(a ErrorAction) Request {
	for i := range r {
		r[i].Rollback = a
	}
	return r
}

// Encode serialises the request to JSON.
func (r Request) Encode() ([]byte, error) {
	return json.Marshal(r)
}

// cmd represents a single JSON-RPC command within a [Request].
type cmd struct {
	Jsonrpc  string      `json:"jsonrpc"`
	Method   string      `json:"method"`
	Params   params      `json:"params"`
	ID       int         `json:"id"`
	Rollback ErrorAction `json:"rollback,omitempty"`
}

// params holds the NX-API specific parameters for a [cmd].
type params struct {
	Cmd     string `json:"cmd"`
	Version int    `json:"version"`
}

// ErrorAction controls how NX-OS responds when an individual command fails.
type ErrorAction string

const (
	Stop     ErrorAction = "stop-on-error"
	Continue ErrorAction = "continue-on-error"
	Rollback ErrorAction = "rollback-on-error"
)

// res is the shared JSON-RPC response envelope.
type res struct {
	Error *RPCError `json:"error"`
	Body  struct {
		Data json.RawMessage `json:"body"`
	} `json:"result"`
}

// decode attempts to parse the response body as either a single JSON-RPC response
// or a batch of responses, returning a slice of [res] values in either case.
func decode(body []byte) ([]res, error) {
	if len(body) > 0 && body[0] == '{' {
		var r res
		if err := json.Unmarshal(body, &r); err != nil {
			return nil, err
		}
		return []res{r}, nil
	}
	var r []res
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, err
	}
	return r, nil
}

// RPCError represents a single JSON-RPC error returned by NX-OS.
type RPCError struct {
	// Code is the JSON-RPC error code.
	Code int `json:"code"`
	// Message is the human-readable error description.
	Message string `json:"message"`
	// Data contains additional error context as raw JSON, if present.
	Data json.RawMessage `json:"data"`
}

func (e *RPCError) Error() string {
	var detail struct {
		Msg string `json:"msg"`
	}
	if json.Unmarshal(e.Data, &detail) == nil && detail.Msg != "" {
		return fmt.Sprintf("nxapi: RPC error %d: %s: %s", e.Code, e.Message, detail.Msg)
	}
	return fmt.Sprintf("nxapi: RPC error %d: %s", e.Code, e.Message)
}

// RPCErrors is a slice of [RPCError] values returned when one or more commands
// in a request fail. It implements the error interface.
type RPCErrors []*RPCError

// Error implements the built-in error interface.
func (e RPCErrors) Error() string {
	errs := make([]error, len(e))
	for i, err := range e {
		errs[i] = err
	}
	return errors.Join(errs...).Error()
}

// Unwrap returns the individual RPC errors for use with errors.Is and errors.As.
func (e RPCErrors) Unwrap() []error {
	errs := make([]error, len(e))
	for i, err := range e {
		errs[i] = err
	}
	return errs
}

// HTTPError is returned when the NX-API endpoint responds with a non-2xx
// status and the body cannot be parsed as a JSON-RPC error.
type HTTPError struct {
	// Code is the HTTP status code.
	Code int
	// Body contains the raw response body.
	Body []byte
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("nxapi: non-2xx status code: %d - %s", e.Code, string(e.Body))
}

// IsTransportError reports whether err is a network-level transport error
// (connection reset, timeout, EOF) as opposed to a logical error returned
// by the NX-API endpoint (RPCError, HTTPError). This is useful for callers
// that issue disruptive commands (e.g. reboot) where the device going down
// mid-request is expected.
func IsTransportError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr)
}
