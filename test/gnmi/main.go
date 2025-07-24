package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

var _ gpb.GNMIServer = (*Server)(nil)

// Server implements the GNMI gRPC server
type Server struct {
	gpb.UnimplementedGNMIServer

	State *State
}

func (s *Server) Capabilities(_ context.Context, req *gpb.CapabilityRequest) (*gpb.CapabilityResponse, error) {
	log.Printf("Received Capabilities request: %v", req)
	return nil, status.Errorf(codes.Unimplemented, "method Capabilities not implemented")
}

func (s *Server) Get(_ context.Context, req *gpb.GetRequest) (*gpb.GetResponse, error) {
	notifications := make([]*gpb.Notification, 0, len(req.GetPath()))
	for _, path := range req.GetPath() {
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

func (s *Server) Subscribe(grpc.BidiStreamingServer[gpb.SubscribeRequest, gpb.SubscribeResponse]) error {
	log.Printf("Received Subscribe request")
	return status.Errorf(codes.Unimplemented, "method Subscribe not implemented")
}

// State represents a JSON body that can be manipulated using [sjson] syntax.
type State struct{ Buf []byte }

func (s State) Get(path *gpb.Path) []byte {
	res := gjson.GetBytes(s.Buf, xpath(path))
	if !res.Exists() {
		return []byte("null")
	}
	return []byte(res.Raw)
}

func (s *State) Set(path *gpb.Path, raw []byte) {
	s.Buf, _ = sjson.SetRawBytes(s.Buf, xpath(path), raw) //nolint:errcheck
}

func (s *State) Del(path *gpb.Path) {
	s.Buf, _ = sjson.DeleteBytes(s.Buf, xpath(path)) //nolint:errcheck
}

// xpath converts a GNMI Path to a JSON path interpretable by [gjson] and [sjson].
func xpath(path *gpb.Path) string {
	parts := make([]string, 0, len(path.GetElem()))
	for _, elem := range path.GetElem() {
		if elem.GetName() != "" {
			parts = append(parts, elem.GetName())
		}
		// TODO: Handle list keys
	}
	return strings.Join(parts, ".")
}

func main() {
	// Parse command line flags
	port := flag.Int("port", 9339, "The server port")
	flag.Parse()

	// Create a listener on the specified port
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("Failed to listen on port %d: %v", *port, err)
	}

	// TODO: Configure TLS options if needed
	var opts []grpc.ServerOption

	// Create a new gRPC server without TLS
	grpcServer := grpc.NewServer(opts...)

	// Create our server implementation
	server := &Server{State: &State{}}

	// Register the GNMIService with our server implementation
	gpb.RegisterGNMIServer(grpcServer, server)

	// Enable reflection for easier testing with tools like grpcurl
	reflection.Register(grpcServer)

	log.Printf("Starting gRPC server on port %d", *port)
	log.Printf("Server is ready to accept connections...")
	log.Printf("Use --port flag to specify a different port (default: 9339)")
	log.Printf("Available services: GNMI")

	// Start serving
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve gRPC server: %v", err)
	}
}
