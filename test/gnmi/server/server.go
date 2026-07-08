// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
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

// Server implements the GNMI gRPC server for testing purposes.
// It maintains an internal state that can be manipulated via gNMI requests or by modifying the internal state directly.
type Server struct {
	gpb.UnimplementedGNMIServer
	// state is the internal state of the server.
	state *State
	// grpcServer is the gRPC server instance where gNMI clients can connect to.
	grpcServer *grpc.Server
	// grpcAddr is the address grpcServer is listening on, e.g., 127.0.0.1:9443
	grpcAddr string
	// closeOnce ensures Close only runs once, even when triggered by both
	// context cancellation and an explicit caller.
	closeOnce sync.Once
}

// NewTestServer starts an in-process gNMI server on a random available port.
func NewTestServer(ctx context.Context) (*Server, error) {
	lc := &net.ListenConfig{}
	grpcLis, err := lc.Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("failed to listen for gRPC: %w", err)
	}

	cert, err := gtls.NewCert()
	if err != nil {
		grpcLis.Close()
		return nil, fmt.Errorf("failed to create TLS certificate: %w", err)
	}

	grpcServer := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
	})))

	server := &Server{
		state:      &State{},
		grpcServer: grpcServer,
		grpcAddr:   grpcLis.Addr().String(),
	}

	gpb.RegisterGNMIServer(grpcServer, server)

	reflection.Register(grpcServer)

	go func() {
		log.Printf("Starting gRPC server on %s", server.grpcAddr)
		if err := grpcServer.Serve(grpcLis); err != nil {
			log.Printf("gRPC server error: %v", err)
		}
	}()

	go func() { //nolint:gosec // G118: ctx is already done, must use Background for shutdown timeout
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Close(shutdownCtx); err != nil { //nolint:contextcheck // shutdownCtx is correctly derived from Background
			log.Printf("Shutdown error: %v", err)
		}
	}()

	return server, nil
}

// GRPCAddr returns the gRPC server address
func (s *Server) GRPCAddr() string {
	return s.grpcAddr
}

// State returns the internal state of the server
// Callers must use State's methods (Get, Set, Del) which handle locking.
func (s *Server) State() *State {
	return s.state
}

// Close gracefully shuts down the server. It is safe to call multiple times
// and from multiple goroutines; only the first call performs shutdown.
func (s *Server) Close(ctx context.Context) error {
	var closeErr error
	s.closeOnce.Do(func() {
		log.Printf("Shutting down gNMI test server")

		if s.grpcServer != nil {
			s.grpcServer.GracefulStop()
		}
	})
	return closeErr
}

// Capabilities returns the capabilities of the gNMI server
func (s *Server) Capabilities(_ context.Context, _ *gpb.CapabilityRequest) (*gpb.CapabilityResponse, error) {
	return &gpb.CapabilityResponse{SupportedEncodings: []gpb.Encoding{gpb.Encoding_JSON}}, nil
}

// Get returns the current state of the server for the requested path
func (s *Server) Get(_ context.Context, req *gpb.GetRequest) (*gpb.GetResponse, error) {
	notifications := make([]*gpb.Notification, 0, len(req.GetPath()))
	for _, path := range req.GetPath() {
		if len(path.GetElem()) == 0 {
			return nil, status.Error(codes.InvalidArgument, "root path is not allowed")
		}
		log.Printf("Getting path: %v", path)
		val := s.state.Get(path)
		if val == nil {
			notifications = append(notifications, &gpb.Notification{
				Timestamp: time.Now().UnixNano(),
			})
			continue
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

// Set updates the state of the server for the requested path
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
			Path:      replace.GetPath(),
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
			Path:      update.GetPath(),
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

// Subscribe handles gNMI subscription requests.
func (s *Server) Subscribe(stream grpc.BidiStreamingServer[gpb.SubscribeRequest, gpb.SubscribeResponse]) error {
	req, err := stream.Recv()
	switch {
	case errors.Is(err, io.EOF):
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

// State represents a JSON body that can be manipulated using [sjson] syntax.
type State struct {
	sync.RWMutex

	Buf []byte
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
	if !res.Exists() || (res.IsArray() && len(res.Array()) == 0) {
		return nil
	}
	return []byte(res.Raw)
}

func (s *State) Set(path *gpb.Path, raw []byte) {
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
			s.Buf, _ = sjson.SetBytes(s.Buf, sb.String()+"."+k, v) //nolint:errcheck
		}
	}
	s.Buf, _ = sjson.SetRawBytes(s.Buf, sb.String(), raw) //nolint:errcheck
	for k, v := range path.GetElem()[len(path.GetElem())-1].GetKey() {
		s.Buf, _ = sjson.SetBytes(s.Buf, sb.String()+"."+k, v) //nolint:errcheck
	}
}

// Del deletes the value at the specified path from the state.
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
