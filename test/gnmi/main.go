package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
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
		s.State.RLock()
		defer s.State.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if len(s.State.Buf) == 0 {
			w.Write([]byte("{}"))
			return
		}
		var buf bytes.Buffer
		if err := json.Compact(&buf, s.State.Buf); err != nil {
			log.Printf("Failed to compact JSON: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal Server Error"))
			return
		}
		w.Write(buf.Bytes())
	case http.MethodDelete:
		s.State.Lock()
		defer s.State.Unlock()
		s.State.Buf = nil
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// State represents a JSON body that can be manipulated using [sjson] syntax.
type State struct {
	sync.RWMutex

	Buf []byte
}

func (s State) Get(path *gpb.Path) []byte {
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
		return []byte("null")
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

func main() {
	// Parse command line flags
	port := flag.Int("port", 9339, "The gRPC server port")
	httpPort := flag.Int("http-port", 8000, "The HTTP server port")
	flag.Parse()

	// Create a listener on the specified port
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("Failed to listen on port %d: %v", *port, err)
	}

	// Create a TLS certificate for gRPC server
	// This is a self-signed certificate for testing purposes.
	cert, err := gtls.NewCert()
	if err != nil {
		log.Fatalf("Failed to create TLS certificate: %v", err)
	}

	// Create a new gRPC server with TLS
	grpcServer := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
	})))

	// Create our server implementation
	server := &Server{State: &State{}}

	// Register the GNMIService with our server implementation
	gpb.RegisterGNMIServer(grpcServer, server)

	// Enable reflection for easier testing with tools like grpcurl
	reflection.Register(grpcServer)

	// Setup HTTP server
	http.HandleFunc("/v1/state", server.handleState)
	httpServer := &http.Server{Addr: fmt.Sprintf(":%d", *httpPort)}

	// Start HTTP server in a goroutine
	go func() {
		log.Printf("Starting HTTP server on port %d", *httpPort)
		log.Printf("HTTP endpoint available at: /v1/state")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to serve HTTP server: %v", err)
		}
	}()

	log.Printf("Starting gRPC server on port %d", *port)
	log.Printf("Server is ready to accept connections...")
	log.Printf("Use --port flag to specify a different gRPC port (default: 9339)")
	log.Printf("Use --http-port flag to specify a different HTTP port (default: 8000)")
	log.Printf("Available services: GNMI")

	// Start serving
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve gRPC server: %v", err)
	}
}
