// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package gnmiext

import (
	"bytes"
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
		configs []DataElement
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
						Elem: []*gpb.PathElem{
							{Name: "openconfig-system:system"},
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
											Elem: []*gpb.PathElem{
												{Name: "openconfig-system:system"},
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
			configs: []DataElement{new(Hostname)},
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
						Elem: []*gpb.PathElem{
							{Name: "openconfig-system:system"},
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
											Elem: []*gpb.PathElem{
												{Name: "openconfig-system:system"},
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
											Elem: []*gpb.PathElem{
												{Name: "openconfig-system:system"},
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
			configs: []DataElement{new(Hostname), new(Hostname)},
			wantErr: false,
		},
		{
			name:    "Empty list",
			configs: []DataElement{},
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
						Elem: []*gpb.PathElem{
							{Name: "openconfig-system:system"},
							{Name: "config"},
							{Name: "hostname"},
						},
					}) {
						t.Errorf("Unexpected path in GetRequest: %v", req.Path[0])
					}
					return nil, errors.New("get rpc failed")
				},
			},
			configs: []DataElement{new(Hostname)},
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
						Elem: []*gpb.PathElem{
							{Name: "openconfig-system:system"},
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
			configs: []DataElement{new(Hostname)},
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
						Elem: []*gpb.PathElem{
							{Name: "openconfig-system:system"},
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
			configs: []DataElement{new(Hostname)},
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
						Elem: []*gpb.PathElem{
							{Name: "openconfig-system:system"},
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
											Elem: []*gpb.PathElem{
												{Name: "openconfig-system:system"},
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
			configs: []DataElement{new(Hostname)},
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
						Elem: []*gpb.PathElem{
							{Name: "openconfig-system:system"},
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
											Elem: []*gpb.PathElem{
												{Name: "openconfig-system:system"},
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
											Elem: []*gpb.PathElem{
												{Name: "openconfig-system:system"},
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
			configs: []DataElement{new(Hostname)},
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
						Elem: []*gpb.PathElem{
							{Name: "openconfig-system:system"},
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
											Elem: []*gpb.PathElem{
												{Name: "openconfig-system:system"},
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
			configs: []DataElement{new(Hostname)},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &client{
				encoding: gpb.Encoding_JSON,
				gnmi:     gpb.NewGNMIClient(test.conn),
			}

			err := client.GetConfig(t.Context(), test.configs...)
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
		states  []DataElement
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
						Elem: []*gpb.PathElem{
							{Name: "openconfig-system:system"},
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
											Elem: []*gpb.PathElem{
												{Name: "openconfig-system:system"},
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
			states:  []DataElement{new(HostnameState)},
			wantErr: false,
		},
		{
			name:    "Empty list",
			states:  []DataElement{},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &client{
				encoding: gpb.Encoding_JSON,
				gnmi:     gpb.NewGNMIClient(test.conn),
			}

			err := client.GetState(t.Context(), test.states...)
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
		updates []DataElement
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
						Elem: []*gpb.PathElem{
							{Name: "openconfig-system:system"},
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
											Elem: []*gpb.PathElem{
												{Name: "openconfig-system:system"},
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
							Elem: []*gpb.PathElem{
								{Name: "openconfig-system:system"},
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
			updates: []DataElement{new(Hostname("new-hostname"))},
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
						Elem: []*gpb.PathElem{
							{Name: "openconfig-system:system"},
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
											Elem: []*gpb.PathElem{
												{Name: "openconfig-system:system"},
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
			updates: []DataElement{new(Hostname("test-hostname"))},
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
						Elem: []*gpb.PathElem{
							{Name: "openconfig-system:system"},
							{Name: "config"},
							{Name: "hostname"},
						},
					}) {
						t.Errorf("Unexpected path in GetRequest: %v", req.Path[0])
					}
					return nil, errors.New("get rpc failed")
				},
			},
			updates: []DataElement{new(Hostname("test-hostname"))},
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
						Elem: []*gpb.PathElem{
							{Name: "openconfig-system:system"},
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
											Elem: []*gpb.PathElem{
												{Name: "openconfig-system:system"},
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
							Elem: []*gpb.PathElem{
								{Name: "openconfig-system:system"},
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
			updates: []DataElement{new(Hostname("new-hostname"))},
			wantErr: true,
		},
		{
			name:    "Empty list",
			updates: []DataElement{},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &client{
				encoding: gpb.Encoding_JSON,
				gnmi:     gpb.NewGNMIClient(test.conn),
			}

			err := client.Update(t.Context(), test.updates...)
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
		patches []DataElement
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
						Elem: []*gpb.PathElem{
							{Name: "openconfig-system:system"},
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
											Elem: []*gpb.PathElem{
												{Name: "openconfig-system:system"},
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
							Elem: []*gpb.PathElem{
								{Name: "openconfig-system:system"},
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
			patches: []DataElement{new(Hostname("new-hostname"))},
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
						Elem: []*gpb.PathElem{
							{Name: "openconfig-system:system"},
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
											Elem: []*gpb.PathElem{
												{Name: "openconfig-system:system"},
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
			patches: []DataElement{new(Hostname("test-hostname"))},
			wantErr: false,
		},
		{
			name:    "Empty list",
			patches: []DataElement{},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &client{
				encoding: gpb.Encoding_JSON_IETF,
				gnmi:     gpb.NewGNMIClient(test.conn),
			}

			err := client.Patch(t.Context(), test.patches...)
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
		deletes []DataElement
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
						Elem: []*gpb.PathElem{
							{Name: "openconfig-system:system"},
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
			deletes: []DataElement{new(Hostname)},
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
							Elem: []*gpb.PathElem{
								{Name: "openconfig-system:system"},
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
			deletes: []DataElement{new(DefaultableHostname)},
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
						Elem: []*gpb.PathElem{
							{Name: "openconfig-system:system"},
							{Name: "config"},
							{Name: "hostname"},
						},
					}) {
						t.Errorf("Unexpected Delete operation: %v", req.Delete[0])
					}
					return nil, errors.New("set rpc failed")
				},
			},
			deletes: []DataElement{new(Hostname)},
			wantErr: true,
		},
		{
			name:    "Empty list",
			deletes: []DataElement{},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &client{
				encoding: gpb.Encoding_JSON,
				gnmi:     gpb.NewGNMIClient(test.conn),
			}

			err := client.Delete(t.Context(), test.deletes...)
			if (err != nil) != test.wantErr {
				t.Errorf("Delete() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestClient_Do(t *testing.T) {
	tests := []struct {
		name    string
		conn    grpc.ClientConnInterface
		builder *SetBuilder
		wantErr bool
	}{
		{
			name: "Mixed operations in single request",
			conn: &MockClientConn{
				GetFunc: func(ctx context.Context, req *gpb.GetRequest) (*gpb.GetResponse, error) {
					return &gpb.GetResponse{
						Notification: []*gpb.Notification{
							{
								Update: []*gpb.Update{
									{
										Path: req.Path[0],
										Val: &gpb.TypedValue{
											Value: &gpb.TypedValue_JsonVal{
												JsonVal: []byte(`"old-value"`),
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
						t.Errorf("Expected 1 Replace, got %d", len(req.Replace))
					}
					if len(req.Update) != 1 {
						t.Errorf("Expected 1 Update, got %d", len(req.Update))
					}
					if len(req.Delete) != 1 {
						t.Errorf("Expected 1 Delete, got %d", len(req.Delete))
					}
					return &gpb.SetResponse{
						Timestamp: time.Now().UnixNano(),
					}, nil
				},
			},
			builder: new(SetBuilder).
				Update(new(Hostname("update-host"))).
				Patch(new(Hostname("patch-host"))).
				Delete(new(Hostname)),
			wantErr: false,
		},
		{
			name: "All unchanged skips Set RPC",
			conn: &MockClientConn{
				GetFunc: func(ctx context.Context, req *gpb.GetRequest) (*gpb.GetResponse, error) {
					return &gpb.GetResponse{
						Notification: []*gpb.Notification{
							{
								Update: []*gpb.Update{
									{
										Path: req.Path[0],
										Val: &gpb.TypedValue{
											Value: &gpb.TypedValue_JsonVal{
												JsonVal: []byte(`"same"`),
											},
										},
									},
								},
							},
						},
					}, nil
				},
				SetFunc: func(ctx context.Context, req *gpb.SetRequest) (*gpb.SetResponse, error) {
					t.Error("Set RPC should not be called when config is unchanged")
					return nil, nil //nolint:nilnil
				},
			},
			builder: new(SetBuilder).
				Update(new(Hostname("same"))),
			wantErr: false,
		},
		{
			name: "Limit splits into multiple RPCs",
			conn: func() *MockClientConn {
				calls := 0
				return &MockClientConn{
					GetFunc: func(ctx context.Context, req *gpb.GetRequest) (*gpb.GetResponse, error) {
						return nil, status.Error(codes.NotFound, "not found")
					},
					SetFunc: func(ctx context.Context, req *gpb.SetRequest) (*gpb.SetResponse, error) {
						calls++
						total := len(req.Replace) + len(req.Update) + len(req.Delete)
						if total > 2 {
							t.Errorf("Set call %d: expected at most 2 operations, got %d", calls, total)
						}
						return &gpb.SetResponse{
							Timestamp: time.Now().UnixNano(),
						}, nil
					},
				}
			}(),
			builder: new(SetBuilder).
				Limit(2).
				Update(new(Hostname("a"))).
				Update(new(Hostname("b"))).
				Update(new(Hostname("c"))),
			wantErr: false,
		},
		{
			name:    "Empty builder",
			builder: new(SetBuilder),
			wantErr: false,
		},
		{
			name: "Set RPC error",
			conn: &MockClientConn{
				GetFunc: func(ctx context.Context, req *gpb.GetRequest) (*gpb.GetResponse, error) {
					return nil, status.Error(codes.NotFound, "not found")
				},
				SetFunc: func(ctx context.Context, req *gpb.SetRequest) (*gpb.SetResponse, error) {
					return nil, errors.New("set rpc failed")
				},
			},
			builder: new(SetBuilder).
				Update(new(Hostname("new"))),
			wantErr: true,
		},
		{
			name: "Defaultable delete becomes replace",
			conn: &MockClientConn{
				SetFunc: func(ctx context.Context, req *gpb.SetRequest) (*gpb.SetResponse, error) {
					if len(req.Replace) != 1 {
						t.Errorf("Expected 1 Replace for Defaultable, got %d", len(req.Replace))
					}
					if len(req.Delete) != 0 {
						t.Errorf("Expected 0 Delete for Defaultable, got %d", len(req.Delete))
					}
					return &gpb.SetResponse{
						Timestamp: time.Now().UnixNano(),
					}, nil
				},
			},
			builder: new(SetBuilder).
				Delete(new(DefaultableHostname)),
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &client{
				encoding: gpb.Encoding_JSON,
				gnmi:     gpb.NewGNMIClient(test.conn),
			}

			err := client.Do(t.Context(), test.builder)
			if (err != nil) != test.wantErr {
				t.Errorf("Do() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestSetBuilder_Split(t *testing.T) {
	a := new(Hostname("a"))
	b := new(Hostname("b"))
	c := new(DefaultableHostname("c"))
	d := new(Hostname("d"))

	builder := new(SetBuilder).Limit(5).
		Update(a).
		Patch(b).
		Delete(c).
		Update(d)

	match, rest := builder.Split(func(el DataElement) bool {
		_, ok := el.(*DefaultableHostname)
		return ok
	})

	// match should contain only the DefaultableHostname
	if len(match.ops) != 1 {
		t.Fatalf("Split() match: got %d ops, want 1", len(match.ops))
	}
	if match.ops[0].el != c {
		t.Errorf("Split() match[0]: got %v, want %v", match.ops[0].el, c)
	}
	if match.ops[0].mode != del {
		t.Errorf("Split() match[0].mode: got %v, want del", match.ops[0].mode)
	}

	// rest should contain the other three
	if len(rest.ops) != 3 {
		t.Fatalf("Split() rest: got %d ops, want 3", len(rest.ops))
	}
	if rest.ops[0].el != a || rest.ops[0].mode != replace {
		t.Errorf("Split() rest[0]: got %v/%v, want a/replace", rest.ops[0].el, rest.ops[0].mode)
	}
	if rest.ops[1].el != b || rest.ops[1].mode != update {
		t.Errorf("Split() rest[1]: got %v/%v, want b/update", rest.ops[1].el, rest.ops[1].mode)
	}
	if rest.ops[2].el != d || rest.ops[2].mode != replace {
		t.Errorf("Split() rest[2]: got %v/%v, want d/replace", rest.ops[2].el, rest.ops[2].mode)
	}

	// Both inherit the limit
	if match.limit != 5 {
		t.Errorf("Split() match.limit: got %d, want 5", match.limit)
	}
	if rest.limit != 5 {
		t.Errorf("Split() rest.limit: got %d, want 5", rest.limit)
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
			xpath: "openconfig-system:system/config/hostname",
			want: &gpb.Path{
				Elem: []*gpb.PathElem{
					{Name: "openconfig-system:system"},
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
			if !test.wantErr && !bytes.Equal(got, test.want) {
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

var _ DataElement = (*Hostname)(nil)

func (*Hostname) XPath() string { return "openconfig-system:system/config/hostname" }

// -- State --

type HostnameState string

var _ DataElement = (*HostnameState)(nil)

func (*HostnameState) XPath() string { return "openconfig-system:system/state/hostname" }

// -- Defaultable --

type DefaultableHostname string

var (
	_ DataElement = (*DefaultableHostname)(nil)
	_ Defaultable = (*DefaultableHostname)(nil)
)

func (*DefaultableHostname) XPath() string { return "openconfig-system:system/config/hostname" }
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
