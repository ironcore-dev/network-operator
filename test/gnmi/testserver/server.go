// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package testserver

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	gtls "github.com/openconfig/gnmi/testing/fake/testing/tls"
)

var _ gpb.GNMIServer = (*Server)(nil)

// Server implements the GNMI gRPC server
type Server struct {
	gpb.UnimplementedGNMIServer
	// state is the internal state of the server, which can be manipulated via gNMI Set requests or the HTTP API.
	state *State
	// grpcServer is the gRPC server instance where gNMI clients can connect to.
	grpcServer *grpc.Server
	// grpcAddr is the address grpcServer is listening on, e.g., 127.0.0.1:9443
	grpcAddr string
	// httpServer is the HTTP server instance used to inspect and manipulate the server's internal state.
	httpServer *http.Server
	// httpAddr is the address httpServer is listening on, e.g., 127.0.0.1:8000
	httpAddr string
	// closeOnce ensures Close only runs once, even when triggered by both
	// context cancellation and an explicit caller.
	closeOnce sync.Once
}

// ServerOption configures the test server
type ServerOption func(*serverConfig)

type serverConfig struct {
	// nos indicates the network operating system flavor for the test server. This is used to enable specific behaviors, such as stripping DME markers for NXOS.
	nos         nos
	grpcPort    int
	httpPort    int
	bindAddress string
}

// nos is a type representing the network operating system flavor for the test server.
type nos string

const (
	nosNone nos = "none"
	nosNXOS nos = "nxos"
)

// dmeUnsetPropertyMarker is the value NX-OS uses to signal that a field should
// be unset rather than stored as a literal string.
const dmeUnsetPropertyMarker = "DME_UNSET_PROPERTY_MARKER"

// WithGRPCPort sets a specific gRPC port (default: 0 for random)
func WithGRPCPort(port int) ServerOption {
	return func(c *serverConfig) {
		c.grpcPort = port
	}
}

// WithHTTPPort sets a specific HTTP port (default: 0 for random)
func WithHTTPPort(port int) ServerOption {
	return func(c *serverConfig) {
		c.httpPort = port
	}
}

// WithBindAddress sets the address to bind to (default: 127.0.0.1).
func WithBindAddress(addr string) ServerOption {
	return func(c *serverConfig) {
		c.bindAddress = addr
	}
}

// WithNXOSBehavior configures the server to emulate NX-OS device behavior:
//   - Strips fields with DME_UNSET_PROPERTY_MARKER value when value is stored in the server state
//     (the marker means "unset this field", not "store this literal string")
func WithNXOSBehavior() ServerOption {
	return func(c *serverConfig) {
		c.nos = nosNXOS
	}
}

// NewTestServer starts an in-process gNMI + HTTP server.
// By default, it uses random available ports. Use WithGRPCPort/WithHTTPPort to specify ports.
// Returns the server, gRPC address, HTTP address, and any error.
func NewTestServer(ctx context.Context, opts ...ServerOption) (*Server, string, string, error) {
	cfg := &serverConfig{
		nos:         nosNone,     // No special behavior by default
		grpcPort:    0,           // Random port by default
		httpPort:    0,           // Random port by default
		bindAddress: "127.0.0.1", // Localhost by default (safe for in-process tests)
	}
	for _, opt := range opts {
		opt(cfg)
	}

	grpcListener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.bindAddress, cfg.grpcPort))
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to listen for gRPC: %w", err)
	}

	httpListener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.bindAddress, cfg.httpPort))
	if err != nil {
		grpcListener.Close()
		return nil, "", "", fmt.Errorf("failed to listen for HTTP: %w", err)
	}

	cert, err := gtls.NewCert()
	if err != nil {
		grpcListener.Close()
		httpListener.Close()
		return nil, "", "", fmt.Errorf("failed to create TLS certificate: %w", err)
	}

	grpcServer := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
	})))

	server := &Server{
		state: &State{
			nos: cfg.nos,
		},
		grpcServer: grpcServer,
		grpcAddr:   grpcListener.Addr().String(),
		httpAddr:   httpListener.Addr().String(),
	}

	gpb.RegisterGNMIServer(grpcServer, server)

	reflection.Register(grpcServer)

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/state", server.handleState)
	mux.HandleFunc("/v1/clear", server.handleClear)
	server.httpServer = &http.Server{Handler: mux}

	go func() {
		log.Printf("Starting HTTP server on %s", server.httpAddr)
		if err := server.httpServer.Serve(httpListener); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	go func() {
		log.Printf("Starting gRPC server on %s", server.grpcAddr)
		if err := grpcServer.Serve(grpcListener); err != nil {
			log.Printf("gRPC server error: %v", err)
		}
	}()

	go func() {
		<-ctx.Done()
		if err := server.Close(); err != nil {
			log.Printf("Shutdown error: %v", err)
		}
	}()

	return server, server.grpcAddr, server.httpAddr, nil
}

// GRPCAddr returns the gRPC server address
func (s *Server) GRPCAddr() string {
	return s.grpcAddr
}

// HTTPAddr returns the HTTP server address
func (s *Server) HTTPAddr() string {
	return s.httpAddr
}

// GetState returns the current JSON state
func (s *Server) GetState() ([]byte, error) {
	s.state.RLock()
	defer s.state.RUnlock()
	if len(s.state.Buf) == 0 {
		return []byte("{}"), nil
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, s.state.Buf); err != nil {
		return nil, fmt.Errorf("failed to compact JSON: %w", err)
	}
	return buf.Bytes(), nil
}

// SetState replaces the server state with the given JSON bytes.
func (s *Server) SetState(data []byte) {
	s.state.Lock()
	defer s.state.Unlock()
	s.state.Buf = data
}

// ClearState clears all accumulated state
func (s *Server) ClearState() {
	s.state.Lock()
	defer s.state.Unlock()
	s.state.Buf = nil
}

// Close gracefully shuts down the server. It is safe to call multiple times
// and from multiple goroutines; only the first call performs shutdown.
func (s *Server) Close() error {
	var closeErr error
	s.closeOnce.Do(func() {
		log.Printf("Shutting down gNMI test server")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if s.httpServer != nil {
			if err := s.httpServer.Shutdown(ctx); err != nil {
				closeErr = fmt.Errorf("HTTP shutdown: %w", err)
			}
		}
		if s.grpcServer != nil {
			s.grpcServer.GracefulStop()
		}
	})
	return closeErr
}

func (s *Server) Capabilities(_ context.Context, _ *gpb.CapabilityRequest) (*gpb.CapabilityResponse, error) {
	return &gpb.CapabilityResponse{SupportedEncodings: []gpb.Encoding{gpb.Encoding_JSON}}, nil
}

func (s *Server) Get(_ context.Context, req *gpb.GetRequest) (*gpb.GetResponse, error) {
	notifications := make([]*gpb.Notification, 0, len(req.GetPath()))
	for _, path := range req.GetPath() {
		if len(path.GetElem()) == 0 {
			return nil, status.Error(codes.InvalidArgument, "root path is not allowed")
		}
		log.Printf("Getting path: %v", path)
		val := s.state.Get(path)
		if val == nil {
			// Per gNMI spec, return NOT_FOUND for missing paths.
			// NX-OS devices have a non-compliant implementation that returns
			// an empty notification instead. Use WithNXOSBehavior() to emulate this.
			if s.state.nos == nosNXOS {
				notifications = append(notifications, &gpb.Notification{
					Timestamp: time.Now().UnixNano(),
				})
				continue
			}
			return nil, status.Errorf(codes.NotFound, "path not found: %v", path)
		}
		notifications = append(notifications, &gpb.Notification{
			Timestamp: time.Now().UnixNano(),
			Update: []*gpb.Update{
				{
					Path: path,
					Val: &gpb.TypedValue{
						Value: &gpb.TypedValue_JsonVal{
							JsonVal: val,
						},
					},
				},
			},
		})
	}
	return &gpb.GetResponse{
		Notification: notifications,
	}, nil
}

func (s *Server) Set(_ context.Context, req *gpb.SetRequest) (*gpb.SetResponse, error) {
	log.Printf("Received Set request: %v", req)
	res := make([]*gpb.UpdateResult, 0, len(req.GetDelete())+len(req.GetUpdate()))
	for _, del := range req.GetDelete() {
		log.Printf("Deleting path: %v", del)
		res = append(res, &gpb.UpdateResult{
			Timestamp: time.Now().UnixNano(),
			Path:      del,
			Op:        gpb.UpdateResult_DELETE,
		})
		s.state.Del(del)
	}
	for _, replace := range req.GetReplace() {
		log.Printf("Replacing path: %v with value: %q", replace.GetPath(), replace.GetVal().GetJsonVal())
		res = append(res, &gpb.UpdateResult{
			Timestamp: time.Now().UnixNano(),
			Path:      replace.Path,
			Op:        gpb.UpdateResult_REPLACE,
		})
		// Delete the existing value at the path and set the new value.
		s.state.Del(replace.GetPath())
		s.state.Set(replace.GetPath(), replace.GetVal().GetJsonVal())
	}
	for _, update := range req.GetUpdate() {
		log.Printf("Updating path: %v with value: %q", update.GetPath(), update.GetVal().GetJsonVal())
		res = append(res, &gpb.UpdateResult{
			Timestamp: time.Now().UnixNano(),
			Path:      update.Path,
			Op:        gpb.UpdateResult_UPDATE,
		})
		// The value will automatically be merged into the existing state.
		s.state.Set(update.GetPath(), update.GetVal().GetJsonVal())
	}
	// TODO: Handle UnionReplace
	return &gpb.SetResponse{
		Response:  res,
		Timestamp: time.Now().UnixNano(),
	}, nil
}

func (s *Server) Subscribe(stream grpc.BidiStreamingServer[gpb.SubscribeRequest, gpb.SubscribeResponse]) error {
	req, err := stream.Recv()
	switch {
	case err == io.EOF:
		return nil
	case err != nil:
		return err
	case req.GetSubscribe() == nil:
		return status.Errorf(codes.InvalidArgument, "the subscribe request must contain a subscription definition")
	}

	switch req.GetRequest().(type) {
	case *gpb.SubscribeRequest_Poll:
		return status.Errorf(codes.InvalidArgument, "invalid request type: %T", req.GetRequest())
	case *gpb.SubscribeRequest_Subscribe:
	}

	switch mode := req.GetSubscribe().GetMode(); mode {
	case gpb.SubscriptionList_ONCE:
		log.Printf("Received Subscribe request with ONCE mode")

		paths := make([]*gpb.Path, 0, len(req.GetSubscribe().GetSubscription()))
		for _, r := range req.GetSubscribe().GetSubscription() {
			paths = append(paths, r.GetPath())
		}

		res, err := s.Get(stream.Context(), &gpb.GetRequest{
			Prefix:    req.GetSubscribe().GetPrefix(),
			Path:      paths,
			Encoding:  req.GetSubscribe().GetEncoding(),
			UseModels: req.GetSubscribe().GetUseModels(),
			Extension: req.GetExtension(),
		})
		if err != nil {
			return err
		}

		for _, notification := range res.GetNotification() {
			if err := stream.Send(&gpb.SubscribeResponse{
				Response: &gpb.SubscribeResponse_Update{
					Update: notification,
				},
			}); err != nil {
				return status.Errorf(codes.Internal, "failed to send response: %v", err)
			}
		}

	case gpb.SubscriptionList_STREAM:
		return status.Errorf(codes.Unimplemented, "subscribe method Stream not implemented")
	case gpb.SubscriptionList_POLL:
		return status.Errorf(codes.Unimplemented, "subscribe method Poll not implemented")
	default:
		return status.Errorf(codes.InvalidArgument, "unknown subscribe request mode: %v", mode)
	}

	return nil
}

// handleState handles HTTP requests to the /v1/state endpoint:
//   - GET: returns the current state as JSON
//   - POST: merges the JSON request body into the root of the state
//     (shallow: top-level keys overwrite, nested objects are replaced);
//     an empty body is a no-op, invalid JSON returns 400
//   - DELETE: clears all state
//
// If set, the X-HTTP-Method-Override header overrides the request method
// (case-insensitive), so clients that can only send GET/POST can still invoke
// DELETE. Any other method returns 405.
func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	method := r.Method
	if override := r.Header.Get("X-HTTP-Method-Override"); override != "" {
		method = strings.ToUpper(override)
	}
	switch method {
	case http.MethodGet:
		state, err := s.GetState()
		if err != nil {
			log.Printf("Failed to get state: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal Server Error"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(state)
	case http.MethodPost:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("Failed to read body: %v", err)
			http.Error(w, "Failed to read body", http.StatusBadRequest)
			return
		}
		if len(body) == 0 {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if !gjson.ValidBytes(body) {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		s.state.Set(&gpb.Path{}, body)
		log.Printf("Merged state from JSON")
		w.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		s.ClearState()
		w.WriteHeader(http.StatusNoContent)
	default:
		w.Header().Set("Allow", "GET, POST, DELETE")
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// handleClear handles POST /v1/clear to clear all state.
func (s *Server) handleClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	s.ClearState()
	w.WriteHeader(http.StatusNoContent)
}

// mergeJSON merges src JSON into dst JSON at the root level.
// Keys in src overwrite keys in dst. The merge is shallow: nested objects are
// replaced wholesale, not deep-merged. If src is not a JSON object (e.g. an
// array or scalar), dst is discarded and src is returned as-is.
func mergeJSON(dst, src []byte) []byte {
	srcParsed := gjson.ParseBytes(src)
	if !srcParsed.IsObject() {
		return src
	}
	result := dst
	srcParsed.ForEach(func(key, value gjson.Result) bool {
		result, _ = sjson.SetRawBytes(result, key.String(), []byte(value.Raw))
		return true
	})
	return result
}

// State represents a JSON body that can be manipulated using [sjson] syntax.
type State struct {
	sync.RWMutex

	Buf []byte
	// nos defines network-operating-system-specific behavior, e.g. strip DME markers.
	nos nos
}

// stripMarkerFields removes fields with the DME marker value from JSON recursively.
// It is a no-op when the server is not configured to emulate NX-OS behavior.
func (s *State) stripMarkerFields(data []byte) []byte {
	if s.nos != nosNXOS {
		return data
	}
	return s.stripMarkersRecursive(data)
}

// stripMarkersRecursive walks the JSON structure and removes marker fields.
func (s *State) stripMarkersRecursive(data []byte) []byte {
	parsed := gjson.ParseBytes(data)
	if !parsed.IsObject() && !parsed.IsArray() {
		return data
	}

	if parsed.IsArray() {
		var results []string
		parsed.ForEach(func(_, value gjson.Result) bool {
			processed := s.stripMarkersRecursive([]byte(value.Raw))
			results = append(results, string(processed))
			return true
		})
		return []byte("[" + strings.Join(results, ",") + "]")
	}

	var toDelete []string
	parsed.ForEach(func(key, value gjson.Result) bool {
		keyStr := key.String()
		if value.Type == gjson.String && value.String() == dmeUnsetPropertyMarker {
			toDelete = append(toDelete, keyStr)
		} else if value.IsObject() || value.IsArray() {
			processed := s.stripMarkersRecursive([]byte(value.Raw))
			data, _ = sjson.SetRawBytes(data, keyStr, processed)
		}
		return true
	})
	for _, key := range toDelete {
		data, _ = sjson.DeleteBytes(data, key)
	}
	return data
}

func (s *State) Get(path *gpb.Path) []byte {
	s.RLock()
	defer s.RUnlock()
	var sb strings.Builder
	for _, elem := range path.GetElem() {
		if elem.GetName() == "" {
			continue
		}
		if sb.Len() > 0 {
			sb.WriteByte('|')
		}
		sb.WriteString(elem.GetName())
		if len(elem.GetKey()) == 0 {
			continue
		}
		for k, v := range elem.GetKey() {
			sb.WriteByte('|')
			sb.WriteString(`#(`)
			sb.WriteString(k)
			sb.WriteString(`=="`)
			sb.WriteString(v)
			sb.WriteString(`")`)
		}
	}
	res := gjson.GetBytes(s.Buf, sb.String())

	// For NX-OS, return empty bytes when the path is missing or resolves to an empty array
	if s.nos == nosNXOS && (!res.Exists() || (res.IsArray() && len(res.Array()) == 0)) {
		return []byte{}
	}

	return []byte(res.Raw)
}

func (s *State) Set(path *gpb.Path, raw []byte) {
	s.Lock()
	defer s.Unlock()

	raw = s.stripMarkerFields(raw)

	if len(path.GetElem()) == 0 {
		s.Buf = mergeJSON(s.Buf, raw)
		return
	}

	var sb strings.Builder
	for _, elem := range path.GetElem() {
		if elem.GetName() == "" {
			continue
		}
		if sb.Len() > 0 {
			sb.WriteByte('.')
		}
		sb.WriteString(elem.GetName())
		if len(elem.GetKey()) == 0 {
			continue
		}
		var idx int
		gjson.GetBytes(s.Buf, sb.String()).ForEach(func(_, r gjson.Result) bool {
			for k, v := range elem.GetKey() {
				if r.Get(k).String() != v {
					idx++
					return true
				}
			}
			return false
		})
		sb.WriteByte('.')
		sb.WriteString(strconv.Itoa(idx))
		for k, v := range elem.GetKey() {
			s.Buf, _ = sjson.SetBytes(s.Buf, sb.String()+"."+k, parseKeyValue(v)) //nolint:errcheck
		}
	}
	s.Buf, _ = sjson.SetRawBytes(s.Buf, sb.String(), raw) //nolint:errcheck
	for k, v := range path.GetElem()[len(path.GetElem())-1].GetKey() {
		s.Buf, _ = sjson.SetBytes(s.Buf, sb.String()+"."+k, parseKeyValue(v)) //nolint:errcheck
	}
}

// parseKeyValue converts a gNMI path key value to the appropriate Go type.
// Numbers are converted to int64, "true"/"false" to bool, otherwise string.
func parseKeyValue(v string) any {
	if i, err := strconv.ParseInt(v, 10, 64); err == nil {
		return i
	}
	if b, err := strconv.ParseBool(v); err == nil {
		return b
	}
	return v
}

func (s *State) Del(path *gpb.Path) {
	s.Lock()
	defer s.Unlock()
	var sb strings.Builder
	for _, elem := range path.GetElem() {
		if elem.GetName() == "" {
			continue
		}
		if sb.Len() > 0 {
			sb.WriteByte('.')
		}
		sb.WriteString(elem.GetName())
		if len(elem.GetKey()) == 0 {
			continue
		}
		var (
			idx   int
			found bool
		)
		gjson.GetBytes(s.Buf, sb.String()).ForEach(func(_, r gjson.Result) bool {
			for k, v := range elem.GetKey() {
				if r.Get(k).String() != v {
					idx++
					return true
				}
			}
			found = true
			return false
		})
		if !found {
			return
		}
		sb.WriteByte('.')
		sb.WriteString(strconv.Itoa(idx))
	}

	s.Buf, _ = sjson.DeleteBytes(s.Buf, sb.String()) //nolint:errcheck
}
