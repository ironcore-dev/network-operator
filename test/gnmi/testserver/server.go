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

	State *State

	grpcServer *grpc.Server
	httpServer *http.Server
	grpcAddr   string
	httpAddr   string
}

// ServerOption configures the test server
type ServerOption func(*serverConfig)

type serverConfig struct {
	grpcPort        int
	httpPort        int
	bindAddress     string
	stripDMEMarkers bool
	dmeMarkerValue  string
}

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
// Use "0.0.0.0" to listen on all interfaces (required for container/pod deployments).
func WithBindAddress(addr string) ServerOption {
	return func(c *serverConfig) {
		c.bindAddress = addr
	}
}

// WithNXOSBehavior configures the server to emulate NX-OS device behavior:
//   - Strips fields with DME_UNSET_PROPERTY_MARKER value when storing (the marker
//     means "unset this field", not "store this literal string")
//   - Returns empty TypedValue for non-existent paths (instead of NOT_FOUND error)
func WithNXOSBehavior() ServerOption {
	return func(c *serverConfig) {
		c.stripDMEMarkers = true
		c.dmeMarkerValue = "DME_UNSET_PROPERTY_MARKER"
	}
}

// NewTestServer starts an in-process gNMI + HTTP server.
// By default, it uses random available ports. Use WithGRPCPort/WithHTTPPort to specify ports.
// Returns the server, gRPC address, HTTP address, and any error.
func NewTestServer(ctx context.Context, opts ...ServerOption) (*Server, string, string, error) {
	cfg := &serverConfig{
		grpcPort:    0,           // Random port by default
		httpPort:    0,           // Random port by default
		bindAddress: "127.0.0.1", // Localhost by default (safe for in-process tests)
	}
	for _, opt := range opts {
		opt(cfg)
	}

	// Create a listener on the specified port
	grpcLis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.bindAddress, cfg.grpcPort))
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to listen for gRPC: %w", err)
	}

	httpLis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.bindAddress, cfg.httpPort))
	if err != nil {
		grpcLis.Close()
		return nil, "", "", fmt.Errorf("failed to listen for HTTP: %w", err)
	}

	// Create a TLS certificate for gRPC server
	cert, err := gtls.NewCert()
	if err != nil {
		grpcLis.Close()
		httpLis.Close()
		return nil, "", "", fmt.Errorf("failed to create TLS certificate: %w", err)
	}

	// Create a new gRPC server with TLS
	grpcServer := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
	})))

	// Create our server implementation
	server := &Server{
		State: &State{
			stripDMEMarkers: cfg.stripDMEMarkers,
			dmeMarkerValue:  cfg.dmeMarkerValue,
		},
		grpcServer: grpcServer,
		grpcAddr:   grpcLis.Addr().String(),
		httpAddr:   httpLis.Addr().String(),
	}

	// Register the GNMIService with our server implementation
	gpb.RegisterGNMIServer(grpcServer, server)

	// Enable reflection for easier testing
	reflection.Register(grpcServer)

	// Setup HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/state", server.handleState)
	server.httpServer = &http.Server{Handler: mux}

	// Start HTTP server in a goroutine
	go func() {
		log.Printf("Starting HTTP server on %s", server.httpAddr)
		if err := server.httpServer.Serve(httpLis); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Start gRPC server in a goroutine
	go func() {
		log.Printf("Starting gRPC server on %s", server.grpcAddr)
		if err := grpcServer.Serve(grpcLis); err != nil {
			log.Printf("gRPC server error: %v", err)
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
	s.State.RLock()
	defer s.State.RUnlock()
	if len(s.State.Buf) == 0 {
		return []byte("{}"), nil
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, s.State.Buf); err != nil {
		return nil, fmt.Errorf("failed to compact JSON: %w", err)
	}
	return buf.Bytes(), nil
}

// ClearState clears all accumulated state
func (s *Server) ClearState() {
	s.State.Lock()
	defer s.State.Unlock()
	s.State.Buf = nil
}

// Close gracefully shuts down the server
func (s *Server) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var errs []error
	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("HTTP shutdown: %w", err))
		}
	}
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
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
		notifications = append(notifications, &gpb.Notification{
			Timestamp: time.Now().UnixNano(),
			Update: []*gpb.Update{
				{
					Path: path,
					Val: &gpb.TypedValue{
						Value: &gpb.TypedValue_JsonVal{
							JsonVal: s.State.Get(path),
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
		s.State.Del(del)
	}
	for _, replace := range req.GetReplace() {
		log.Printf("Replacing path: %v with value: %q", replace.GetPath(), replace.GetVal().GetJsonVal())
		res = append(res, &gpb.UpdateResult{
			Timestamp: time.Now().UnixNano(),
			Path:      replace.Path,
			Op:        gpb.UpdateResult_REPLACE,
		})
		// Delete the existing value at the path and set the new value.
		s.State.Del(replace.GetPath())
		s.State.Set(replace.GetPath(), replace.GetVal().GetJsonVal())
	}
	for _, update := range req.GetUpdate() {
		log.Printf("Updating path: %v with value: %q", update.GetPath(), update.GetVal().GetJsonVal())
		res = append(res, &gpb.UpdateResult{
			Timestamp: time.Now().UnixNano(),
			Path:      update.Path,
			Op:        gpb.UpdateResult_UPDATE,
		})
		// The value will automatically be merged into the existing state.
		s.State.Set(update.GetPath(), update.GetVal().GetJsonVal())
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

// handleState handles HTTP requests to the /v1/state endpoint
func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
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
	case http.MethodDelete:
		s.ClearState()
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// State represents a JSON body that can be manipulated using [sjson] syntax.
type State struct {
	sync.RWMutex

	Buf []byte

	// NX-OS behavior options
	stripDMEMarkers bool
	dmeMarkerValue  string
}

// stripMarkerFields removes fields with the DME marker value from JSON recursively.
// This emulates NX-OS behavior where these markers mean "unset this field"
// rather than "store this literal string".
func (s *State) stripMarkerFields(data []byte) []byte {
	if !s.stripDMEMarkers || s.dmeMarkerValue == "" {
		return data
	}
	return s.stripMarkersRecursive(data)
}

// stripMarkersRecursive walks the JSON structure and removes marker fields at all levels.
func (s *State) stripMarkersRecursive(data []byte) []byte {
	parsed := gjson.ParseBytes(data)
	if !parsed.IsObject() && !parsed.IsArray() {
		return data
	}

	if parsed.IsArray() {
		// Process each array element
		var results []string
		parsed.ForEach(func(_, value gjson.Result) bool {
			processed := s.stripMarkersRecursive([]byte(value.Raw))
			results = append(results, string(processed))
			return true
		})
		return []byte("[" + strings.Join(results, ",") + "]")
	}

	// It's an object - process fields
	var toDelete []string
	parsed.ForEach(func(key, value gjson.Result) bool {
		keyStr := key.String()
		if value.Type == gjson.String && value.String() == s.dmeMarkerValue {
			toDelete = append(toDelete, keyStr)
		} else if value.IsObject() || value.IsArray() {
			// Recurse into nested structures
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
			sb.WriteString(`")#`)
		}
	}
	res := gjson.GetBytes(s.Buf, sb.String())
	if !res.Exists() || (res.IsArray() && len(res.Array()) == 0) {
		// Return empty bytes for non-existent paths. This triggers gnmiext's
		// ErrNil handling (len(b) == 0), matching real NX-OS behavior which
		// returns empty TypedValue for paths that don't exist yet.
		return []byte{}
	}
	return []byte(res.Raw)
}

func (s *State) Set(path *gpb.Path, raw []byte) {
	s.Lock()
	defer s.Unlock()

	// Strip DME marker fields if NX-OS behavior is enabled
	raw = s.stripMarkerFields(raw)

	elems := path.GetElem()
	var sb strings.Builder

	for i, elem := range elems {
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

		// Find existing array index or append
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

		// Inject keys into this list element if it's not the final element
		// (for the final element, keys go into raw below)
		if i < len(elems)-1 {
			currentPath := sb.String()
			current := gjson.GetBytes(s.Buf, currentPath)
			if !current.Exists() || current.Raw == "null" {
				// Create the element with its keys
				keyObj := make(map[string]string)
				for k, v := range elem.GetKey() {
					keyObj[k] = v
				}
				keyJSON, _ := json.Marshal(keyObj)
				s.Buf, _ = sjson.SetRawBytes(s.Buf, currentPath, keyJSON)
			} else {
				// Element exists, ensure keys are set
				for k, v := range elem.GetKey() {
					if !gjson.GetBytes(s.Buf, currentPath+"."+k).Exists() {
						s.Buf, _ = sjson.SetBytes(s.Buf, currentPath+"."+k, v)
					}
				}
			}
		}
	}

	// For the final element, inject its keys (from the last keyed element) into raw
	lastElem := elems[len(elems)-1]
	for k, v := range lastElem.GetKey() {
		if !gjson.GetBytes(raw, k).Exists() {
			raw, _ = sjson.SetBytes(raw, k, v)
		}
	}

	s.Buf, _ = sjson.SetRawBytes(s.Buf, sb.String(), raw) //nolint:errcheck
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
