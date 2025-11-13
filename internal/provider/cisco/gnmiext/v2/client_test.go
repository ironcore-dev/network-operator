// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package gnmiext

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"testing"
	"time"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"k8s.io/utils/ptr"
)

func TestClient_New(t *testing.T) {
	tests := []struct {
		name    string
		conn    grpc.ClientConnInterface
		wantErr bool
	}{
		{
			name: "Capabilities error",
			conn: &MockClientConn{
				CapabilitiesFunc: func(ctx context.Context, req *gpb.CapabilityRequest) (*gpb.CapabilityResponse, error) {
					return nil, errors.New("test error")
				},
			},
			wantErr: true,
		},
		{
			name: "Unsupported Encoding",
			conn: &MockClientConn{
				CapabilitiesFunc: func(ctx context.Context, req *gpb.CapabilityRequest) (*gpb.CapabilityResponse, error) {
					return &gpb.CapabilityResponse{
						SupportedEncodings: []gpb.Encoding{gpb.Encoding_ASCII},
					}, nil
				},
			},
			wantErr: true,
		},
		{
			name: "JSON Encoding",
			conn: &MockClientConn{
				CapabilitiesFunc: func(ctx context.Context, req *gpb.CapabilityRequest) (*gpb.CapabilityResponse, error) {
					return &gpb.CapabilityResponse{
						SupportedEncodings: []gpb.Encoding{gpb.Encoding_JSON},
					}, nil
				},
			},
			wantErr: false,
		},
		{
			name: "JSON_IETF Encoding",
			conn: &MockClientConn{
				CapabilitiesFunc: func(ctx context.Context, req *gpb.CapabilityRequest) (*gpb.CapabilityResponse, error) {
					return &gpb.CapabilityResponse{
						SupportedEncodings: []gpb.Encoding{gpb.Encoding_JSON_IETF},
					}, nil
				},
			},
			wantErr: false,
		},
		{
			name: "Supported Models",
			conn: &MockClientConn{
				CapabilitiesFunc: func(ctx context.Context, req *gpb.CapabilityRequest) (*gpb.CapabilityResponse, error) {
					return &gpb.CapabilityResponse{
						SupportedModels: []*gpb.ModelData{
							{
								Name:         "openconfig-system",
								Organization: "OpenConfig working group",
								Version:      "0.17.0",
							},
						},
						SupportedEncodings: []gpb.Encoding{gpb.Encoding_JSON, gpb.Encoding_JSON_IETF},
					}, nil
				},
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := New(t.Context(), test.conn)
			if (err != nil) != test.wantErr {
				t.Errorf("NewClient() error = %v, wantErr %v", err, test.wantErr)
				return
			}
			if got == nil && !test.wantErr {
				t.Errorf("NewClient() = nil, want non-nil")
			}
		})
	}
}

func TestClient_GetConfig(t *testing.T) {
	tests := []struct {
		name    string
		conn    grpc.ClientConnInterface
		conf    []Configurable
		wantErr bool
	}{
		{
			name: "Single",
			conn: &MockClientConn{
				GetFunc: func(ctx context.Context, req *gpb.GetRequest) (*gpb.GetResponse, error) {
					if req.Type != gpb.GetRequest_CONFIG {
						t.Errorf("Expected GetRequest_CONFIG, got %v", req.Type)
					}
					if req.Encoding != gpb.Encoding_JSON {
						t.Errorf("Expected Encoding_JSON, got %v", req.Encoding)
					}
					if len(req.Path) != 1 {
						t.Errorf("Expected single path in GetRequest, got %d", len(req.Path))
					}
					if !proto.Equal(req.Path[0], &gpb.Path{
						Origin: "openconfig",
						Elem: []*gpb.PathElem{
							{Name: "system"},
							{Name: "config"},
							{Name: "hostname"},
						},
					}) {
						t.Errorf("Unexpected path in GetRequest: %v", req.Path[0])
					}
					return &gpb.GetResponse{
						Notification: []*gpb.Notification{
							{
								Update: []*gpb.Update{
									{
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "system"},
												{Name: "config"},
												{Name: "hostname"},
											},
										},
										Val: &gpb.TypedValue{
											Value: &gpb.TypedValue_JsonVal{
												JsonVal: []byte(`"test-hostname"`),
											},
										},
									},
								},
							},
						},
					}, nil
				},
			},
			conf:    []Configurable{new(Hostname)},
			wantErr: false,
		},
		{
			name: "Multiple",
			conn: &MockClientConn{
				GetFunc: func(ctx context.Context, req *gpb.GetRequest) (*gpb.GetResponse, error) {
					if req.Type != gpb.GetRequest_CONFIG {
						t.Errorf("Expected GetRequest_CONFIG, got %v", req.Type)
					}
					if req.Encoding != gpb.Encoding_JSON {
						t.Errorf("Expected Encoding_JSON, got %v", req.Encoding)
					}
					if len(req.Path) != 2 {
						t.Errorf("Expected two paths in GetRequest, got %d", len(req.Path))
					}
					if !proto.Equal(req.Path[0], &gpb.Path{
						Origin: "openconfig",
						Elem: []*gpb.PathElem{
							{Name: "system"},
							{Name: "config"},
							{Name: "hostname"},
						},
					}) {
						t.Errorf("Unexpected path in GetRequest: %v", req.Path[0])
					}
					return &gpb.GetResponse{
						Notification: []*gpb.Notification{
							{
								Update: []*gpb.Update{
									{
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "system"},
												{Name: "config"},
												{Name: "hostname"},
											},
										},
										Val: &gpb.TypedValue{
											Value: &gpb.TypedValue_JsonVal{
												JsonVal: []byte(`"test-hostname"`),
											},
										},
									},
								},
							},
							{
								Update: []*gpb.Update{
									{
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "system"},
												{Name: "config"},
												{Name: "hostname"},
											},
										},
										Val: &gpb.TypedValue{
											Value: &gpb.TypedValue_JsonVal{
												JsonVal: []byte(`"test-hostname"`),
											},
										},
									},
								},
							},
						},
					}, nil
				},
			},
			conf:    []Configurable{new(Hostname), new(Hostname)},
			wantErr: false,
		},
		{
			name:    "Empty list",
			conf:    []Configurable{},
			wantErr: false,
		},
		{
			name: "Get RPC Error",
			conn: &MockClientConn{
				GetFunc: func(ctx context.Context, req *gpb.GetRequest) (*gpb.GetResponse, error) {
					if req.Type != gpb.GetRequest_CONFIG {
						t.Errorf("Expected GetRequest_CONFIG, got %v", req.Type)
					}
					if req.Encoding != gpb.Encoding_JSON {
						t.Errorf("Expected Encoding_JSON, got %v", req.Encoding)
					}
					if len(req.Path) != 1 {
						t.Errorf("Expected single path in GetRequest, got %d", len(req.Path))
					}
					if !proto.Equal(req.Path[0], &gpb.Path{
						Origin: "openconfig",
						Elem: []*gpb.PathElem{
							{Name: "system"},
							{Name: "config"},
							{Name: "hostname"},
						},
					}) {
						t.Errorf("Unexpected path in GetRequest: %v", req.Path[0])
					}
					return nil, errors.New("get rpc failed")
				},
			},
			conf:    []Configurable{new(Hostname)},
			wantErr: true,
		},
		{
			name: "Empty Notifications",
			conn: &MockClientConn{
				GetFunc: func(ctx context.Context, req *gpb.GetRequest) (*gpb.GetResponse, error) {
					if req.Type != gpb.GetRequest_CONFIG {
						t.Errorf("Expected GetRequest_CONFIG, got %v", req.Type)
					}
					if req.Encoding != gpb.Encoding_JSON {
						t.Errorf("Expected Encoding_JSON, got %v", req.Encoding)
					}
					if len(req.Path) != 1 {
						t.Errorf("Expected single path in GetRequest, got %d", len(req.Path))
					}
					if !proto.Equal(req.Path[0], &gpb.Path{
						Origin: "openconfig",
						Elem: []*gpb.PathElem{
							{Name: "system"},
							{Name: "config"},
							{Name: "hostname"},
						},
					}) {
						t.Errorf("Unexpected path in GetRequest: %v", req.Path[0])
					}
					return &gpb.GetResponse{
						Notification: []*gpb.Notification{},
					}, nil
				},
			},
			conf:    []Configurable{new(Hostname)},
			wantErr: true,
		},
		{
			name: "Empty Updates",
			conn: &MockClientConn{
				GetFunc: func(ctx context.Context, req *gpb.GetRequest) (*gpb.GetResponse, error) {
					if req.Type != gpb.GetRequest_CONFIG {
						t.Errorf("Expected GetRequest_CONFIG, got %v", req.Type)
					}
					if req.Encoding != gpb.Encoding_JSON {
						t.Errorf("Expected Encoding_JSON, got %v", req.Encoding)
					}
					if len(req.Path) != 1 {
						t.Errorf("Expected single path in GetRequest, got %d", len(req.Path))
					}
					if !proto.Equal(req.Path[0], &gpb.Path{
						Origin: "openconfig",
						Elem: []*gpb.PathElem{
							{Name: "system"},
							{Name: "config"},
							{Name: "hostname"},
						},
					}) {
						t.Errorf("Unexpected path in GetRequest: %v", req.Path[0])
					}
					return &gpb.GetResponse{
						Notification: []*gpb.Notification{
							{
								Update: []*gpb.Update{},
							},
						},
					}, nil
				},
			},
			conf:    []Configurable{new(Hostname)},
			wantErr: true,
		},
		{
			name: "Empty Value",
			conn: &MockClientConn{
				GetFunc: func(ctx context.Context, req *gpb.GetRequest) (*gpb.GetResponse, error) {
					if req.Type != gpb.GetRequest_CONFIG {
						t.Errorf("Expected GetRequest_CONFIG, got %v", req.Type)
					}
					if req.Encoding != gpb.Encoding_JSON {
						t.Errorf("Expected Encoding_JSON, got %v", req.Encoding)
					}
					if len(req.Path) != 1 {
						t.Errorf("Expected single path in GetRequest, got %d", len(req.Path))
					}
					if !proto.Equal(req.Path[0], &gpb.Path{
						Origin: "openconfig",
						Elem: []*gpb.PathElem{
							{Name: "system"},
							{Name: "config"},
							{Name: "hostname"},
						},
					}) {
						t.Errorf("Unexpected path in GetRequest: %v", req.Path[0])
					}
					return &gpb.GetResponse{
						Notification: []*gpb.Notification{
							{
								Update: []*gpb.Update{
									{
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "system"},
												{Name: "config"},
												{Name: "hostname"},
											},
										},
										Val: &gpb.TypedValue{
											Value: &gpb.TypedValue_JsonVal{
												JsonVal: []byte(""),
											},
										},
									},
								},
							},
						},
					}, nil
				},
			},
			conf:    []Configurable{new(Hostname)},
			wantErr: true,
		},
		{
			name: "Multiple Updates",
			conn: &MockClientConn{
				GetFunc: func(ctx context.Context, req *gpb.GetRequest) (*gpb.GetResponse, error) {
					if req.Type != gpb.GetRequest_CONFIG {
						t.Errorf("Expected GetRequest_CONFIG, got %v", req.Type)
					}
					if req.Encoding != gpb.Encoding_JSON {
						t.Errorf("Expected Encoding_JSON, got %v", req.Encoding)
					}
					if len(req.Path) != 1 {
						t.Errorf("Expected single path in GetRequest, got %d", len(req.Path))
					}
					if !proto.Equal(req.Path[0], &gpb.Path{
						Origin: "openconfig",
						Elem: []*gpb.PathElem{
							{Name: "system"},
							{Name: "config"},
							{Name: "hostname"},
						},
					}) {
						t.Errorf("Unexpected path in GetRequest: %v", req.Path[0])
					}
					return &gpb.GetResponse{
						Notification: []*gpb.Notification{
							{
								Update: []*gpb.Update{
									{
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "system"},
												{Name: "config"},
												{Name: "hostname"},
											},
										},
										Val: &gpb.TypedValue{
											Value: &gpb.TypedValue_JsonVal{
												JsonVal: []byte(`"test-hostname"`),
											},
										},
									},
									{
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "system"},
												{Name: "config"},
												{Name: "hostname"},
											},
										},
										Val: &gpb.TypedValue{
											Value: &gpb.TypedValue_JsonVal{
												JsonVal: []byte(`"test-hostname"`),
											},
										},
									},
								},
							},
						},
					}, nil
				},
			},
			conf:    []Configurable{new(Hostname)},
			wantErr: true,
		},
		{
			name: "Unexpected Encoding",
			conn: &MockClientConn{
				GetFunc: func(ctx context.Context, req *gpb.GetRequest) (*gpb.GetResponse, error) {
					if req.Type != gpb.GetRequest_CONFIG {
						t.Errorf("Expected GetRequest_CONFIG, got %v", req.Type)
					}
					if req.Encoding != gpb.Encoding_JSON {
						t.Errorf("Expected Encoding_JSON, got %v", req.Encoding)
					}
					if len(req.Path) != 1 {
						t.Errorf("Expected single path in GetRequest, got %d", len(req.Path))
					}
					if !proto.Equal(req.Path[0], &gpb.Path{
						Origin: "openconfig",
						Elem: []*gpb.PathElem{
							{Name: "system"},
							{Name: "config"},
							{Name: "hostname"},
						},
					}) {
						t.Errorf("Unexpected path in GetRequest: %v", req.Path[0])
					}
					return &gpb.GetResponse{
						Notification: []*gpb.Notification{
							{
								Update: []*gpb.Update{
									{
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "system"},
												{Name: "config"},
												{Name: "hostname"},
											},
										},
										Val: &gpb.TypedValue{
											Value: &gpb.TypedValue_JsonIetfVal{
												JsonIetfVal: []byte(`"test-hostname"`),
											},
										},
									},
								},
							},
						},
					}, nil
				},
			},
			conf:    []Configurable{new(Hostname)},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &client{
				encoding: gpb.Encoding_JSON,
				gnmi:     gpb.NewGNMIClient(test.conn),
			}

			err := client.GetConfig(t.Context(), test.conf...)
			if (err != nil) != test.wantErr {
				t.Errorf("GetConfig() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestClient_GetState(t *testing.T) {
	tests := []struct {
		name    string
		conn    grpc.ClientConnInterface
		conf    []Configurable
		wantErr bool
	}{
		{
			name: "Single",
			conn: &MockClientConn{
				GetFunc: func(ctx context.Context, req *gpb.GetRequest) (*gpb.GetResponse, error) {
					if req.Type != gpb.GetRequest_STATE {
						t.Errorf("Expected GetRequest_STATE, got %v", req.Type)
					}
					if req.Encoding != gpb.Encoding_JSON {
						t.Errorf("Expected Encoding_JSON, got %v", req.Encoding)
					}
					if len(req.Path) != 1 {
						t.Errorf("Expected single path in GetRequest, got %d", len(req.Path))
					}
					if !proto.Equal(req.Path[0], &gpb.Path{
						Origin: "openconfig",
						Elem: []*gpb.PathElem{
							{Name: "system"},
							{Name: "state"},
							{Name: "hostname"},
						},
					}) {
						t.Errorf("Unexpected path in GetRequest: %v", req.Path[0])
					}
					return &gpb.GetResponse{
						Notification: []*gpb.Notification{
							{
								Update: []*gpb.Update{
									{
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "system"},
												{Name: "state"},
												{Name: "hostname"},
											},
										},
										Val: &gpb.TypedValue{
											Value: &gpb.TypedValue_JsonVal{
												JsonVal: []byte(`"test-hostname"`),
											},
										},
									},
								},
							},
						},
					}, nil
				},
			},
			conf:    []Configurable{new(HostnameState)},
			wantErr: false,
		},
		{
			name:    "Empty list",
			conf:    []Configurable{},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &client{
				encoding: gpb.Encoding_JSON,
				gnmi:     gpb.NewGNMIClient(test.conn),
			}

			err := client.GetState(t.Context(), test.conf...)
			if (err != nil) != test.wantErr {
				t.Errorf("GetState() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestClient_Update(t *testing.T) {
	tests := []struct {
		name    string
		conn    grpc.ClientConnInterface
		conf    []Configurable
		wantErr bool
	}{
		{
			name: "Replace",
			conn: &MockClientConn{
				GetFunc: func(ctx context.Context, req *gpb.GetRequest) (*gpb.GetResponse, error) {
					if req.Type != gpb.GetRequest_CONFIG {
						t.Errorf("Expected GetRequest_CONFIG, got %v", req.Type)
					}
					if req.Encoding != gpb.Encoding_JSON {
						t.Errorf("Expected Encoding_JSON, got %v", req.Encoding)
					}
					if len(req.Path) != 1 {
						t.Errorf("Expected single path in GetRequest, got %d", len(req.Path))
					}
					if !proto.Equal(req.Path[0], &gpb.Path{
						Origin: "openconfig",
						Elem: []*gpb.PathElem{
							{Name: "system"},
							{Name: "config"},
							{Name: "hostname"},
						},
					}) {
						t.Errorf("Unexpected path in GetRequest: %v", req.Path[0])
					}
					return &gpb.GetResponse{
						Notification: []*gpb.Notification{
							{
								Update: []*gpb.Update{
									{
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "system"},
												{Name: "config"},
												{Name: "hostname"},
											},
										},
										Val: &gpb.TypedValue{
											Value: &gpb.TypedValue_JsonVal{
												JsonVal: []byte(`"test-hostname"`),
											},
										},
									},
								},
							},
						},
					}, nil
				},
				SetFunc: func(ctx context.Context, req *gpb.SetRequest) (*gpb.SetResponse, error) {
					if len(req.Replace) != 1 {
						t.Errorf("Expected single Replace operation, got %d", len(req.Replace))
					}
					if len(req.Update) != 0 {
						t.Errorf("Expected no Update operations, got %d", len(req.Update))
					}
					if len(req.Delete) != 0 {
						t.Errorf("Expected no Delete operations, got %d", len(req.Delete))
					}
					if !proto.Equal(req.Replace[0], &gpb.Update{
						Path: &gpb.Path{
							Origin: "openconfig",
							Elem: []*gpb.PathElem{
								{Name: "system"},
								{Name: "config"},
								{Name: "hostname"},
							},
						},
						Val: &gpb.TypedValue{
							Value: &gpb.TypedValue_JsonVal{
								JsonVal: []byte(`"new-hostname"`),
							},
						},
					}) {
						t.Errorf("Unexpected Replace operation: %v", req.Replace[0])
					}
					return &gpb.SetResponse{
						Timestamp: time.Now().UnixNano(),
					}, nil
				},
			},
			conf:    []Configurable{ptr.To(Hostname("new-hostname"))},
			wantErr: false,
		},
		{
			name: "Unchanged",
			conn: &MockClientConn{
				GetFunc: func(ctx context.Context, req *gpb.GetRequest) (*gpb.GetResponse, error) {
					if req.Type != gpb.GetRequest_CONFIG {
						t.Errorf("Expected GetRequest_CONFIG, got %v", req.Type)
					}
					if req.Encoding != gpb.Encoding_JSON {
						t.Errorf("Expected Encoding_JSON, got %v", req.Encoding)
					}
					if len(req.Path) != 1 {
						t.Errorf("Expected single path in GetRequest, got %d", len(req.Path))
					}
					if !proto.Equal(req.Path[0], &gpb.Path{
						Origin: "openconfig",
						Elem: []*gpb.PathElem{
							{Name: "system"},
							{Name: "config"},
							{Name: "hostname"},
						},
					}) {
						t.Errorf("Unexpected path in GetRequest: %v", req.Path[0])
					}
					return &gpb.GetResponse{
						Notification: []*gpb.Notification{
							{
								Update: []*gpb.Update{
									{
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "system"},
												{Name: "config"},
												{Name: "hostname"},
											},
										},
										Val: &gpb.TypedValue{
											Value: &gpb.TypedValue_JsonVal{
												JsonVal: []byte(`"test-hostname"`),
											},
										},
									},
								},
							},
						},
					}, nil
				},
			},
			conf:    []Configurable{ptr.To(Hostname("test-hostname"))},
			wantErr: false,
		},
		{
			name: "Get RPC Error",
			conn: &MockClientConn{
				GetFunc: func(ctx context.Context, req *gpb.GetRequest) (*gpb.GetResponse, error) {
					if req.Type != gpb.GetRequest_CONFIG {
						t.Errorf("Expected GetRequest_CONFIG, got %v", req.Type)
					}
					if req.Encoding != gpb.Encoding_JSON {
						t.Errorf("Expected Encoding_JSON, got %v", req.Encoding)
					}
					if len(req.Path) != 1 {
						t.Errorf("Expected single path in GetRequest, got %d", len(req.Path))
					}
					if !proto.Equal(req.Path[0], &gpb.Path{
						Origin: "openconfig",
						Elem: []*gpb.PathElem{
							{Name: "system"},
							{Name: "config"},
							{Name: "hostname"},
						},
					}) {
						t.Errorf("Unexpected path in GetRequest: %v", req.Path[0])
					}
					return nil, errors.New("get rpc failed")
				},
			},
			conf:    []Configurable{ptr.To(Hostname("test-hostname"))},
			wantErr: true,
		},
		{
			name: "Set RPC Error",
			conn: &MockClientConn{
				GetFunc: func(ctx context.Context, req *gpb.GetRequest) (*gpb.GetResponse, error) {
					if req.Type != gpb.GetRequest_CONFIG {
						t.Errorf("Expected GetRequest_CONFIG, got %v", req.Type)
					}
					if req.Encoding != gpb.Encoding_JSON {
						t.Errorf("Expected Encoding_JSON, got %v", req.Encoding)
					}
					if len(req.Path) != 1 {
						t.Errorf("Expected single path in GetRequest, got %d", len(req.Path))
					}
					if !proto.Equal(req.Path[0], &gpb.Path{
						Origin: "openconfig",
						Elem: []*gpb.PathElem{
							{Name: "system"},
							{Name: "config"},
							{Name: "hostname"},
						},
					}) {
						t.Errorf("Unexpected path in GetRequest: %v", req.Path[0])
					}
					return &gpb.GetResponse{
						Notification: []*gpb.Notification{
							{
								Update: []*gpb.Update{
									{
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "system"},
												{Name: "config"},
												{Name: "hostname"},
											},
										},
										Val: &gpb.TypedValue{
											Value: &gpb.TypedValue_JsonVal{
												JsonVal: []byte(`"test-hostname"`),
											},
										},
									},
								},
							},
						},
					}, nil
				},
				SetFunc: func(ctx context.Context, req *gpb.SetRequest) (*gpb.SetResponse, error) {
					if len(req.Replace) != 1 {
						t.Errorf("Expected single Replace operation, got %d", len(req.Replace))
					}
					if len(req.Update) != 0 {
						t.Errorf("Expected no Update operations, got %d", len(req.Update))
					}
					if len(req.Delete) != 0 {
						t.Errorf("Expected no Delete operations, got %d", len(req.Delete))
					}
					if !proto.Equal(req.Replace[0], &gpb.Update{
						Path: &gpb.Path{
							Origin: "openconfig",
							Elem: []*gpb.PathElem{
								{Name: "system"},
								{Name: "config"},
								{Name: "hostname"},
							},
						},
						Val: &gpb.TypedValue{
							Value: &gpb.TypedValue_JsonVal{
								JsonVal: []byte(`"new-hostname"`),
							},
						},
					}) {
						t.Errorf("Unexpected Replace operation: %v", req.Replace[0])
					}
					return nil, errors.New("set rpc failed")
				},
			},
			conf:    []Configurable{ptr.To(Hostname("new-hostname"))},
			wantErr: true,
		},
		{
			name:    "Empty list",
			conf:    []Configurable{},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &client{
				encoding: gpb.Encoding_JSON,
				gnmi:     gpb.NewGNMIClient(test.conn),
			}

			err := client.Update(t.Context(), test.conf...)
			if (err != nil) != test.wantErr {
				t.Errorf("Update() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestClient_Patch(t *testing.T) {
	tests := []struct {
		name    string
		conn    grpc.ClientConnInterface
		conf    []Configurable
		wantErr bool
	}{
		{
			name: "Merge",
			conn: &MockClientConn{
				GetFunc: func(ctx context.Context, req *gpb.GetRequest) (*gpb.GetResponse, error) {
					if req.Type != gpb.GetRequest_CONFIG {
						t.Errorf("Expected GetRequest_CONFIG, got %v", req.Type)
					}
					if req.Encoding != gpb.Encoding_JSON_IETF {
						t.Errorf("Expected Encoding_JSON_IETF, got %v", req.Encoding)
					}
					if len(req.Path) != 1 {
						t.Errorf("Expected single path in GetRequest, got %d", len(req.Path))
					}
					if !proto.Equal(req.Path[0], &gpb.Path{
						Origin: "openconfig",
						Elem: []*gpb.PathElem{
							{Name: "system"},
							{Name: "config"},
							{Name: "hostname"},
						},
					}) {
						t.Errorf("Unexpected path in GetRequest: %v", req.Path[0])
					}
					return &gpb.GetResponse{
						Notification: []*gpb.Notification{
							{
								Update: []*gpb.Update{
									{
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "system"},
												{Name: "config"},
												{Name: "hostname"},
											},
										},
										Val: &gpb.TypedValue{
											Value: &gpb.TypedValue_JsonIetfVal{
												JsonIetfVal: []byte(`"test-hostname"`),
											},
										},
									},
								},
							},
						},
					}, nil
				},
				SetFunc: func(ctx context.Context, req *gpb.SetRequest) (*gpb.SetResponse, error) {
					if len(req.Update) != 1 {
						t.Errorf("Expected single Update operation, got %d", len(req.Update))
					}
					if len(req.Replace) != 0 {
						t.Errorf("Expected no Replace operations, got %d", len(req.Replace))
					}
					if len(req.Delete) != 0 {
						t.Errorf("Expected no Delete operations, got %d", len(req.Delete))
					}
					if !proto.Equal(req.Update[0], &gpb.Update{
						Path: &gpb.Path{
							Origin: "openconfig",
							Elem: []*gpb.PathElem{
								{Name: "system"},
								{Name: "config"},
								{Name: "hostname"},
							},
						},
						Val: &gpb.TypedValue{
							Value: &gpb.TypedValue_JsonIetfVal{
								JsonIetfVal: []byte(`"new-hostname"`),
							},
						},
					}) {
						t.Errorf("Unexpected Update operation: %v", req.Replace[0])
					}
					return &gpb.SetResponse{
						Timestamp: time.Now().UnixNano(),
					}, nil
				},
			},
			conf:    []Configurable{ptr.To(Hostname("new-hostname"))},
			wantErr: false,
		},
		{
			name: "Unchanged",
			conn: &MockClientConn{
				GetFunc: func(ctx context.Context, req *gpb.GetRequest) (*gpb.GetResponse, error) {
					if req.Type != gpb.GetRequest_CONFIG {
						t.Errorf("Expected GetRequest_CONFIG, got %v", req.Type)
					}
					if req.Encoding != gpb.Encoding_JSON_IETF {
						t.Errorf("Expected Encoding_JSON_IETF, got %v", req.Encoding)
					}
					if len(req.Path) != 1 {
						t.Errorf("Expected single path in GetRequest, got %d", len(req.Path))
					}
					if !proto.Equal(req.Path[0], &gpb.Path{
						Origin: "openconfig",
						Elem: []*gpb.PathElem{
							{Name: "system"},
							{Name: "config"},
							{Name: "hostname"},
						},
					}) {
						t.Errorf("Unexpected path in GetRequest: %v", req.Path[0])
					}
					return &gpb.GetResponse{
						Notification: []*gpb.Notification{
							{
								Update: []*gpb.Update{
									{
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "system"},
												{Name: "config"},
												{Name: "hostname"},
											},
										},
										Val: &gpb.TypedValue{
											Value: &gpb.TypedValue_JsonIetfVal{
												JsonIetfVal: []byte(`"test-hostname"`),
											},
										},
									},
								},
							},
						},
					}, nil
				},
			},
			conf:    []Configurable{ptr.To(Hostname("test-hostname"))},
			wantErr: false,
		},
		{
			name:    "Empty list",
			conf:    []Configurable{},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &client{
				encoding: gpb.Encoding_JSON_IETF,
				gnmi:     gpb.NewGNMIClient(test.conn),
			}

			err := client.Patch(t.Context(), test.conf...)
			if (err != nil) != test.wantErr {
				t.Errorf("Update() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestClient_Delete(t *testing.T) {
	tests := []struct {
		name    string
		conn    grpc.ClientConnInterface
		conf    []Configurable
		wantErr bool
	}{
		{
			name: "Regular Delete",
			conn: &MockClientConn{
				SetFunc: func(ctx context.Context, req *gpb.SetRequest) (*gpb.SetResponse, error) {
					if len(req.Delete) == 0 {
						t.Error("Expected Delete operation for regular Configurable")
					}
					if len(req.Replace) > 0 {
						t.Error("Expected no Replace operations for regular Configurable")
					}
					if !proto.Equal(req.Delete[0], &gpb.Path{
						Origin: "openconfig",
						Elem: []*gpb.PathElem{
							{Name: "system"},
							{Name: "config"},
							{Name: "hostname"},
						},
					}) {
						t.Errorf("Unexpected Delete operation: %v", req.Delete[0])
					}
					return &gpb.SetResponse{
						Timestamp: time.Now().UnixNano(),
					}, nil
				},
			},
			conf:    []Configurable{new(Hostname)},
			wantErr: false,
		},
		{
			name: "Defaultable Replace",
			conn: &MockClientConn{
				SetFunc: func(ctx context.Context, req *gpb.SetRequest) (*gpb.SetResponse, error) {
					if len(req.Replace) == 0 {
						t.Error("Expected Replace operation for Defaultable")
					}
					if len(req.Delete) > 0 {
						t.Error("Expected no Delete operations for Defaultable")
					}
					if !proto.Equal(req.Replace[0], &gpb.Update{
						Path: &gpb.Path{
							Origin: "openconfig",
							Elem: []*gpb.PathElem{
								{Name: "system"},
								{Name: "config"},
								{Name: "hostname"},
							},
						},
						Val: &gpb.TypedValue{
							Value: &gpb.TypedValue_JsonVal{
								JsonVal: []byte(`"default-hostname"`),
							},
						},
					}) {
						t.Errorf("Unexpected Replace operation: %v", req.Replace[0])
					}
					return &gpb.SetResponse{
						Timestamp: time.Now().UnixNano(),
					}, nil
				},
			},
			conf:    []Configurable{new(DefaultableHostname)},
			wantErr: false,
		},
		{
			name: "Set RPC Error",
			conn: &MockClientConn{
				SetFunc: func(ctx context.Context, req *gpb.SetRequest) (*gpb.SetResponse, error) {
					if len(req.Delete) == 0 {
						t.Error("Expected Delete operation for regular Configurable")
					}
					if len(req.Replace) > 0 {
						t.Error("Expected no Replace operations for regular Configurable")
					}
					if !proto.Equal(req.Delete[0], &gpb.Path{
						Origin: "openconfig",
						Elem: []*gpb.PathElem{
							{Name: "system"},
							{Name: "config"},
							{Name: "hostname"},
						},
					}) {
						t.Errorf("Unexpected Delete operation: %v", req.Delete[0])
					}
					return nil, errors.New("set rpc failed")
				},
			},
			conf:    []Configurable{new(Hostname)},
			wantErr: true,
		},
		{
			name:    "Empty list",
			conf:    []Configurable{},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &client{
				encoding: gpb.Encoding_JSON,
				gnmi:     gpb.NewGNMIClient(test.conn),
			}

			err := client.Delete(context.Background(), test.conf...)
			if (err != nil) != test.wantErr {
				t.Errorf("Delete() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestStringToStructuredPath(t *testing.T) {
	tests := []struct {
		name    string
		xpath   string
		want    *gpb.Path
		wantErr bool
	}{
		{
			name:  "Simple",
			xpath: "System/name",
			want: &gpb.Path{
				Elem: []*gpb.PathElem{
					{Name: "System"},
					{Name: "name"},
				},
			},
		},
		{
			name:  "Model",
			xpath: "openconfig:system/config/hostname",
			want: &gpb.Path{
				Origin: "openconfig",
				Elem: []*gpb.PathElem{
					{Name: "system"},
					{Name: "config"},
					{Name: "hostname"},
				},
			},
		},
		{
			name:    "Invalid",
			xpath:   "[",
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := StringToStructuredPath(test.xpath)
			if (err != nil) != test.wantErr {
				t.Errorf("StringToStructuredPath() error = %v, wantErr %v", err, test.wantErr)
				return
			}
			if !proto.Equal(got, test.want) {
				t.Errorf("StringToStructuredPath() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestClient_Marshal(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		want    []byte
		wantErr bool
	}{
		{
			name:    "JSON string",
			value:   "test-hostname",
			want:    []byte(`"test-hostname"`),
			wantErr: false,
		},
		{
			name: "JSON struct",
			value: struct {
				Name string `json:"name"`
			}{Name: "test"},
			want:    []byte(`{"name":"test"}`),
			wantErr: false,
		},
		{
			name:    "Custom Marshaller",
			value:   &Interface{Name: "eth1/1"},
			want:    []byte(`{"config":{"name":"eth1/1"},"name":"eth1/1"}`),
			wantErr: false,
		},
		{
			name:    "Error",
			value:   func() {},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &client{
				capabilities: &Capabilities{
					SupportedModels: []Model{
						{Name: "openconfig-interfaces", Organization: "OpenConfig working group", Version: "2.5.0"},
					},
				},
			}

			got, err := client.Marshal(test.value)
			if (err != nil) != test.wantErr {
				t.Errorf("Marshal() error = %v, wantErr %v", err, test.wantErr)
				return
			}
			if !test.wantErr && string(got) != string(test.want) {
				t.Errorf("Marshal() = %s, want %s", got, test.want)
			}
		})
	}
}

func TestClient_Unmarshal(t *testing.T) {
	tests := []struct {
		name    string
		want    any
		value   []byte
		wantErr bool
	}{
		{
			name:    "JSON string",
			want:    "test-hostname",
			value:   []byte(`"test-hostname"`),
			wantErr: false,
		},
		{
			name: "JSON struct",
			want: struct {
				Name string `json:"name"`
			}{Name: "test"},
			value:   []byte(`{"name":"test"}`),
			wantErr: false,
		},
		{
			name:    "Custom Marshaller",
			want:    &Interface{Name: "eth1/1"},
			value:   []byte(`{"config":{"name":"eth1/1"},"name":"eth1/1"}`),
			wantErr: false,
		},
		{
			name:    "Error",
			want:    "",
			value:   []byte(`{}`),
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &client{
				capabilities: &Capabilities{
					SupportedModels: []Model{
						{Name: "openconfig-interfaces", Organization: "OpenConfig working group", Version: "2.5.0"},
					},
				},
			}

			rt := reflect.TypeOf(test.want)
			for rt.Kind() == reflect.Pointer {
				rt = rt.Elem()
			}

			got := reflect.New(rt).Interface()
			if err := client.Unmarshal(test.value, got); (err != nil) != test.wantErr {
				t.Errorf("Marshal() error = %v, wantErr %v", err, test.wantErr)
				return
			}

			if !test.wantErr {
				rv := reflect.ValueOf(test.want)
				if rv.Kind() != reflect.Pointer {
					p := reflect.New(rv.Type())
					p.Elem().Set(rv)
					rv = p
				}

				want := rv.Interface()
				if !reflect.DeepEqual(got, want) {
					t.Errorf("Unmarshal() = %v, want %v", got, want)
				}
			}
		})
	}
}

// -- Config --

type Hostname string

var _ Configurable = (*Hostname)(nil)

func (*Hostname) XPath() string { return "openconfig:system/config/hostname" }

// -- State --

type HostnameState string

var _ Configurable = (*HostnameState)(nil)

func (*HostnameState) XPath() string { return "openconfig:system/state/hostname" }

// -- Defaultable --

type DefaultableHostname string

var (
	_ Configurable = (*DefaultableHostname)(nil)
	_ Defaultable  = (*DefaultableHostname)(nil)
)

func (*DefaultableHostname) XPath() string { return "openconfig:system/config/hostname" }
func (h *DefaultableHostname) Default()    { *h = "default-hostname" }

var _ grpc.ClientConnInterface = (*MockClientConn)(nil)

// MockClientConn provides a mock implementation of [grpc.ClientConnInterface] for testing gNMI clients.
// It includes configurable mock responses for gNMI RPC methods.
type MockClientConn struct {
	// CapabilitiesFunc allows mocking of the Capabilities RPC response.
	CapabilitiesFunc func(ctx context.Context, req *gpb.CapabilityRequest) (*gpb.CapabilityResponse, error)

	// GetFunc allows mocking of the Get RPC response.
	GetFunc func(ctx context.Context, req *gpb.GetRequest) (*gpb.GetResponse, error)

	// SetFunc allows mocking of the Set RPC response.
	SetFunc func(ctx context.Context, req *gpb.SetRequest) (*gpb.SetResponse, error)
}

func (m *MockClientConn) Invoke(ctx context.Context, method string, args, reply any, opts ...grpc.CallOption) error {
	switch method {
	case "/gnmi.gNMI/Capabilities":
		if m.CapabilitiesFunc == nil {
			return status.Error(codes.Unimplemented, "Capabilities RPC not mocked")
		}
		req := args.(*gpb.CapabilityRequest)
		res, err := m.CapabilitiesFunc(ctx, req)
		if err != nil {
			return err
		}
		proto.Merge(reply.(*gpb.CapabilityResponse), res)
		return nil

	case "/gnmi.gNMI/Get":
		if m.GetFunc == nil {
			return status.Error(codes.Unimplemented, "Get RPC not mocked")
		}
		req := args.(*gpb.GetRequest)
		res, err := m.GetFunc(ctx, req)
		if err != nil {
			return err
		}
		proto.Merge(reply.(*gpb.GetResponse), res)
		return nil

	case "/gnmi.gNMI/Set":
		if m.SetFunc == nil {
			return status.Error(codes.Unimplemented, "Set RPC not mocked")
		}
		req := args.(*gpb.SetRequest)
		res, err := m.SetFunc(ctx, req)
		if err != nil {
			return err
		}
		proto.Merge(reply.(*gpb.SetResponse), res)
		return nil

	default:
		return status.Errorf(codes.Unimplemented, "method %s not mocked", method)
	}
}

func (m *MockClientConn) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return nil, status.Errorf(codes.Unimplemented, "streaming method %s not mocked", method)
}

// Interface implements the [Marshaler] interface.
// It marshals to different YANG models based on the client's capabilities.
type Interface struct {
	Name string
}

var _ Marshaler = (*Interface)(nil)

func (i *Interface) MarshalYANG(caps *Capabilities) ([]byte, error) {
	if slices.ContainsFunc(caps.SupportedModels, func(m Model) bool {
		return m.Name == "openconfig-interfaces"
	}) {
		// Openconfig Interfaces model
		return fmt.Appendf(nil, `{"config":{"name":"%s"},"name":"%s"}`, i.Name, i.Name), nil
	}
	// Cisco NX-OS Device model
	return fmt.Appendf(nil, `{"id":"%s"}`, i.Name), nil
}

func (i *Interface) UnmarshalYANG(caps *Capabilities, data []byte) error {
	if slices.ContainsFunc(caps.SupportedModels, func(m Model) bool {
		return m.Name == "openconfig-interfaces"
	}) {
		var res struct {
			Name   string `json:"name"`
			Config struct {
				Name string `json:"name"`
			} `json:"config"`
		}
		if err := json.Unmarshal(data, &res); err != nil {
			return err
		}
		i.Name = res.Config.Name
		return nil
	}
	var res struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &res); err != nil {
		return err
	}
	i.Name = res.ID
	return nil
}
