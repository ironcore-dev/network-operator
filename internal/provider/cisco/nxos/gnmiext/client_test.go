// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package gnmiext

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
)

func newMockGNMIClient() *GNMIClientMock {
	return &GNMIClientMock{
		CapabilitiesFunc: func(ctx context.Context, req *gpb.CapabilityRequest, opts ...grpc.CallOption) (*gpb.CapabilityResponse, error) {
			return &gpb.CapabilityResponse{
				SupportedEncodings: []gpb.Encoding{gpb.Encoding_JSON},
				SupportedModels: []*gpb.ModelData{
					{Name: "Cisco-NX-OS-device", Version: "2024-03-26"},
				},
			}, nil
		},
	}
}

func Test_NewClient(t *testing.T) {
	cc := newMockGNMIClient()
	c, err := NewClient(t.Context(), cc, false)
	if err != nil {
		t.Fatalf("unexpected error: got %v, nil", err)
	}
	if c == nil {
		t.Fatal("unexpected nil client")
	}
}

func Test_NewClient_Err(t *testing.T) {
	cc := newMockGNMIClient()
	cc.CapabilitiesFunc = func(_ context.Context, _ *gpb.CapabilityRequest, _ ...grpc.CallOption) (*gpb.CapabilityResponse, error) {
		return nil, status.Error(codes.Unavailable, "unavailable")
	}

	c, err := NewClient(t.Context(), cc, false)
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("unexpected error: got %v, want status.Err with code unavailable", err)
	}
	if !errors.Is(err, ErrDeviceUnavailable) {
		t.Fatalf("unexpected error: got %v, want %v", err, ErrDeviceUnavailable)
	}
	if c != nil {
		t.Fatal("expected nil client")
	}
}

func Test_NewClient_Encoding(t *testing.T) {
	cc := newMockGNMIClient()
	cc.CapabilitiesFunc = func(_ context.Context, _ *gpb.CapabilityRequest, _ ...grpc.CallOption) (*gpb.CapabilityResponse, error) {
		return &gpb.CapabilityResponse{
			SupportedEncodings: []gpb.Encoding{gpb.Encoding_PROTO},
			SupportedModels: []*gpb.ModelData{
				{
					Name:    "Cisco-NX-OS-device",
					Version: "2024-03-26",
				},
			},
		}, nil
	}

	c, err := NewClient(t.Context(), cc, false)
	if !errors.Is(err, ErrUnsupportedEncoding) {
		t.Fatalf("unexpected error: got %v, want %v", err, ErrUnsupportedEncoding)
	}
	if c != nil {
		t.Fatal("expected nil client")
	}
}

func Test_NewClient_Device(t *testing.T) {
	cc := newMockGNMIClient()
	cc.CapabilitiesFunc = func(_ context.Context, _ *gpb.CapabilityRequest, _ ...grpc.CallOption) (*gpb.CapabilityResponse, error) {
		return &gpb.CapabilityResponse{
			SupportedEncodings: []gpb.Encoding{gpb.Encoding_JSON},
			SupportedModels: []*gpb.ModelData{
				{
					Name:    "Arista-device",
					Version: "2024-03-26",
				},
			},
		}, nil
	}

	c, err := NewClient(t.Context(), cc, false)
	if !errors.Is(err, ErrUnsupportedDevice) {
		t.Fatalf("unexpected error: got %v, want %v", err, ErrUnsupportedDevice)
	}
	if c != nil {
		t.Fatal("expected nil client")
	}
}

func Test_NewClient_Version(t *testing.T) {
	cc := &GNMIClientMock{
		CapabilitiesFunc: func(_ context.Context, _ *gpb.CapabilityRequest, _ ...grpc.CallOption) (*gpb.CapabilityResponse, error) {
			return &gpb.CapabilityResponse{
				SupportedEncodings: []gpb.Encoding{gpb.Encoding_JSON},
				SupportedModels: []*gpb.ModelData{
					{
						Name:    "Cisco-NX-OS-device",
						Version: "2024-04-26",
					},
				},
			}, nil
		},
	}

	c, err := NewClient(t.Context(), cc, false)
	if !errors.Is(err, ErrUnsupportedDevice) {
		t.Fatalf("unexpected error: got %v, want %v", err, ErrUnsupportedDevice)
	}
	if c != nil {
		t.Fatal("expected nil client")
	}
}

func TestClient_Exists(t *testing.T) {
	ctx := context.Background()
	path := "System/time-items/srcIf-items"

	t.Run("exists with value", func(t *testing.T) {
		cc := &GNMIClientMock{
			GetFunc: func(_ context.Context, in *gpb.GetRequest, _ ...grpc.CallOption) (*gpb.GetResponse, error) {
				return &gpb.GetResponse{
					Notification: []*gpb.Notification{
						{
							Update: []*gpb.Update{
								{
									Path: &gpb.Path{Elem: []*gpb.PathElem{
										{Name: "System"},
										{Name: "time-items"},
										{Name: "srcIf-items"},
									}},
									Val: &gpb.TypedValue{
										Value: &gpb.TypedValue_JsonVal{
											JsonVal: []byte(`{"srcIf":"mgmt0"}`),
										},
									},
								},
							},
						},
					},
				}, nil
			},
		}
		c := &client{c: cc}
		found, err := c.Exists(ctx, path)
		if !found || err != nil {
			t.Fatalf("expected true, nil; got %v, %v", found, err)
		}
	})

	t.Run("exists but empty value", func(t *testing.T) {
		cc := &GNMIClientMock{
			GetFunc: func(_ context.Context, in *gpb.GetRequest, _ ...grpc.CallOption) (*gpb.GetResponse, error) {
				return &gpb.GetResponse{
					Notification: []*gpb.Notification{
						{
							Update: []*gpb.Update{
								{
									Path: &gpb.Path{Elem: []*gpb.PathElem{
										{Name: "System"},
										{Name: "time-items"},
										{Name: "srcIf-items"},
									}},
									Val: &gpb.TypedValue{
										Value: &gpb.TypedValue_JsonVal{
											JsonVal: []byte{},
										},
									},
								},
							},
						},
					},
				}, nil
			},
		}
		c := &client{c: cc}
		found, err := c.Exists(ctx, path)
		if found || err != nil {
			t.Fatalf("expected false, nil; got %v, %v", found, err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		cc := &GNMIClientMock{
			GetFunc: func(_ context.Context, in *gpb.GetRequest, _ ...grpc.CallOption) (*gpb.GetResponse, error) {
				return &gpb.GetResponse{
					Notification: []*gpb.Notification{
						{
							Update: []*gpb.Update{},
						},
					},
				}, nil
			},
		}
		c := &client{c: cc}
		found, err := c.Exists(ctx, path)
		if found || err != nil {
			t.Fatalf("expected false, nil; got %v, %v", found, err)
		}
	})
}

func Test_Get(t *testing.T) {
	cc := &GNMIClientMock{
		GetFunc: func(_ context.Context, in *gpb.GetRequest, _ ...grpc.CallOption) (*gpb.GetResponse, error) {
			if in.Type != gpb.GetRequest_CONFIG {
				t.Fatalf("unexpected type: %v", in.Type)
			}
			if in.Encoding != gpb.Encoding_JSON {
				t.Fatalf("unexpected encoding: %v", in.Encoding)
			}
			return &gpb.GetResponse{
				Notification: []*gpb.Notification{
					{
						Update: []*gpb.Update{
							{
								Path: &gpb.Path{Elem: []*gpb.PathElem{
									{Name: "System"},
									{Name: "time-items"},
									{Name: "srcIf-items"},
								}},
								Val: &gpb.TypedValue{
									Value: &gpb.TypedValue_JsonVal{
										JsonVal: []byte(`{"srcIf":"mgmt0"}`),
									},
								},
							},
						},
					},
				},
			}, nil
		},
	}

	var got nxos.Cisco_NX_OSDevice_System_TimeItems_SrcIfItems
	if err := (&client{c: cc}).Get(t.Context(), "System/time-items/srcIf-items", &got); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.SrcIf == nil {
		t.Fatal("unexpected nil srcIf")
	}
	if *got.SrcIf != "mgmt0" {
		t.Fatalf("unexpected srcIf: got '%v', want 'mgmt0'", *got.SrcIf)
	}
}

func Test_Get_ListElement(t *testing.T) {
	cc := &GNMIClientMock{
		GetFunc: func(_ context.Context, in *gpb.GetRequest, _ ...grpc.CallOption) (*gpb.GetResponse, error) {
			if in.Type != gpb.GetRequest_CONFIG {
				t.Fatalf("unexpected type: %v", in.Type)
			}
			if in.Encoding != gpb.Encoding_JSON {
				t.Fatalf("unexpected encoding: %v", in.Encoding)
			}
			return &gpb.GetResponse{
				Notification: []*gpb.Notification{
					{
						Update: []*gpb.Update{
							{
								Path: &gpb.Path{Elem: []*gpb.PathElem{
									{Name: "System"},
									{Name: "intf-items"},
									{Name: "phys-items"},
									{Name: "PhysIf-list", Key: map[string]string{"id": "eth1/1"}},
								}},
								Val: &gpb.TypedValue{
									Value: &gpb.TypedValue_JsonVal{
										JsonVal: []byte(`[{"id":"eth1/1"}]`),
									},
								},
							},
						},
					},
				},
			}, nil
		},
	}

	var got nxos.Cisco_NX_OSDevice_System_IntfItems_PhysItems_PhysIfList
	if err := (&client{c: cc}).Get(t.Context(), "System/intf-items/phys-items/PhysIf-list[id=eth1/1]", &got); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Id == nil {
		t.Fatal("unexpected nil id")
	}
	if *got.Id != "eth1/1" {
		t.Fatalf("unexpected id: got '%v', want 'eth1/1'", *got.Id)
	}
}

func Test_Get_Err(t *testing.T) {
	cc := &GNMIClientMock{
		GetFunc: func(_ context.Context, in *gpb.GetRequest, _ ...grpc.CallOption) (*gpb.GetResponse, error) {
			if in.Type != gpb.GetRequest_CONFIG {
				t.Fatalf("unexpected type: %v", in.Type)
			}
			if in.Encoding != gpb.Encoding_JSON {
				t.Fatalf("unexpected encoding: %v", in.Encoding)
			}
			return nil, status.Error(codes.Unavailable, "unavailable")
		},
	}

	var got nxos.Cisco_NX_OSDevice_System_TimeItems_SrcIfItems
	err := (&client{c: cc}).Get(t.Context(), "System/time-items/srcIf-items", &got)
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("unexpected error: got %v, want status.Err with code unavailable", err)
	}
	if got.SrcIf != nil {
		t.Fatalf("unexpected srcIf: got '%v', want nil", *got.SrcIf)
	}
}

func Test_Get_NotFound(t *testing.T) {
	cc := &GNMIClientMock{
		GetFunc: func(_ context.Context, in *gpb.GetRequest, _ ...grpc.CallOption) (*gpb.GetResponse, error) {
			if in.Type != gpb.GetRequest_CONFIG {
				t.Fatalf("unexpected type: %v", in.Type)
			}
			if in.Encoding != gpb.Encoding_JSON {
				t.Fatalf("unexpected encoding: %v", in.Encoding)
			}
			return &gpb.GetResponse{
				Notification: []*gpb.Notification{
					{
						Update: []*gpb.Update{},
					},
				},
			}, nil
		},
	}

	var got nxos.Cisco_NX_OSDevice_System_TimeItems_SrcIfItems
	err := (&client{c: cc}).Get(t.Context(), "System/time-items/srcIf-items", &got)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("unexpected error: got %v, want %v", err, ErrNotFound)
	}
	if got.SrcIf != nil {
		t.Fatalf("unexpected srcIf: got '%v', want nil", *got.SrcIf)
	}
}

func Test_Get_EmptyVal(t *testing.T) {
	cc := &GNMIClientMock{
		GetFunc: func(_ context.Context, in *gpb.GetRequest, _ ...grpc.CallOption) (*gpb.GetResponse, error) {
			if in.Type != gpb.GetRequest_CONFIG {
				t.Fatalf("unexpected type: %v", in.Type)
			}
			if in.Encoding != gpb.Encoding_JSON {
				t.Fatalf("unexpected encoding: %v", in.Encoding)
			}
			return &gpb.GetResponse{
				Notification: []*gpb.Notification{
					{
						Update: []*gpb.Update{
							{
								Path: &gpb.Path{Elem: []*gpb.PathElem{
									{Name: "System"},
									{Name: "time-items"},
									{Name: "srcIf-items"},
								}},
								Val: &gpb.TypedValue{
									Value: &gpb.TypedValue_JsonVal{
										JsonVal: nil,
									},
								},
							},
						},
					},
				},
			}, nil
		},
	}

	var got nxos.Cisco_NX_OSDevice_System_TimeItems_SrcIfItems
	err := (&client{c: cc}).Get(t.Context(), "System/time-items/srcIf-items", &got)
	if !errors.Is(err, ErrNil) {
		t.Fatalf("unexpected error: got %v, want %v", err, ErrNil)
	}
	if got.SrcIf != nil {
		t.Fatalf("unexpected srcIf: got '%v', want nil", *got.SrcIf)
	}
}

func Test_Get_Encoding(t *testing.T) {
	cc := &GNMIClientMock{
		GetFunc: func(_ context.Context, in *gpb.GetRequest, _ ...grpc.CallOption) (*gpb.GetResponse, error) {
			if in.Type != gpb.GetRequest_CONFIG {
				t.Fatalf("unexpected type: %v", in.Type)
			}
			if in.Encoding != gpb.Encoding_JSON {
				t.Fatalf("unexpected encoding: %v", in.Encoding)
			}
			return &gpb.GetResponse{
				Notification: []*gpb.Notification{
					{
						Update: []*gpb.Update{
							{
								Path: &gpb.Path{Elem: []*gpb.PathElem{
									{Name: "System"},
									{Name: "time-items"},
									{Name: "srcIf-items"},
									{Name: "srcIf"},
								}},
								Val: &gpb.TypedValue{
									Value: &gpb.TypedValue_StringVal{
										StringVal: "mgmt0",
									},
								},
							},
						},
					},
				},
			}, nil
		},
	}

	if err := (&client{c: cc}).Get(t.Context(), "System/time-items/srcIf-items/srcIf", nil); err == nil {
		t.Fatal("expected error")
	}
}

func Test_Get_IgnoreExtraFields(t *testing.T) {
	cc := &GNMIClientMock{
		GetFunc: func(_ context.Context, in *gpb.GetRequest, _ ...grpc.CallOption) (*gpb.GetResponse, error) {
			if in.Type != gpb.GetRequest_CONFIG {
				t.Fatalf("unexpected type: %v", in.Type)
			}
			if in.Encoding != gpb.Encoding_JSON {
				t.Fatalf("unexpected encoding: %v", in.Encoding)
			}
			return &gpb.GetResponse{
				Notification: []*gpb.Notification{
					{
						Update: []*gpb.Update{
							{
								Path: &gpb.Path{Elem: []*gpb.PathElem{
									{Name: "System"},
									{Name: "time-items"},
									{Name: "srcIf-items"},
								}},
								Val: &gpb.TypedValue{
									Value: &gpb.TypedValue_JsonVal{
										JsonVal: []byte(`{"srcIf":"mgmt0","extraField":"should be ignored"}`),
									},
								},
							},
						},
					},
				},
			}, nil
		},
	}

	var got nxos.Cisco_NX_OSDevice_System_TimeItems_SrcIfItems
	if err := (&client{c: cc}).Get(t.Context(), "System/time-items/srcIf-items", &got); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.SrcIf == nil {
		t.Fatal("unexpected nil srcIf")
	}
	if *got.SrcIf != "mgmt0" {
		t.Fatalf("unexpected srcIf: got '%v', want 'mgmt0'", *got.SrcIf)
	}
}

type DummyYgot struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

func (*DummyYgot) IsYANGGoStruct() {}

var _ ygot.GoStruct = (*DummyYgot)(nil)

func TestClient_Get_WithStdJSONUnmarshal_NonRFC7951(t *testing.T) {
	cc := &GNMIClientMock{
		GetFunc: func(_ context.Context, in *gpb.GetRequest, _ ...grpc.CallOption) (*gpb.GetResponse, error) {
			if in.Type != gpb.GetRequest_CONFIG {
				t.Fatalf("unexpected type: %v", in.Type)
			}
			if in.Encoding != gpb.Encoding_JSON {
				t.Fatalf("unexpected encoding: %v", in.Encoding)
			}
			return &gpb.GetResponse{
				Notification: []*gpb.Notification{
					{
						Update: []*gpb.Update{
							{
								Path: &gpb.Path{Elem: []*gpb.PathElem{{Name: "System/time-items/srcIf-items/srcIf"}}},
								Val: func() *gpb.TypedValue {
									val, _ := json.Marshal(map[string]interface{}{
										"name":  "eth1/1",
										"value": 42,
									})
									return &gpb.TypedValue{Value: &gpb.TypedValue_JsonVal{JsonVal: val}}
								}(),
							},
						},
					},
				},
			}, nil
		},
	}

	var got DummyYgot
	if err := (&client{c: cc}).Get(t.Context(), "System/time-items/srcIf-items/srcIf", &got); err == nil {
		t.Fatal("unexpected error: test uses an RFC7951 compliant JSON")
	}
	if err := (&client{c: cc}).Get(t.Context(), "System/time-items/srcIf-items/srcIf", &got, WithStdJSONUnmarshal()); err == nil {
		t.Fatal("failed to parse non RFC7951 compliant JSON")
	}
}

func Test_Set(t *testing.T) {
	cc := &GNMIClientMock{
		SetFunc: func(ctx context.Context, in *gpb.SetRequest, opts ...grpc.CallOption) (*gpb.SetResponse, error) {
			if len(in.Delete) == 0 && len(in.Update) == 0 {
				t.Fatal("unexpected empty diff")
			}
			return &gpb.SetResponse{}, nil
		},
		CapabilitiesFunc: func(ctx context.Context, req *gpb.CapabilityRequest, opts ...grpc.CallOption) (*gpb.CapabilityResponse, error) {
			// Return a mock response or error as needed
			return &gpb.CapabilityResponse{
				SupportedEncodings: []gpb.Encoding{gpb.Encoding_JSON},
				SupportedModels: []*gpb.ModelData{
					{Name: "Cisco-NX-OS-device", Version: "2024-03-26"},
				},
			}, nil
		},
	}

	n := &gpb.Notification{
		Update: []*gpb.Update{
			{
				Path: &gpb.Path{Elem: []*gpb.PathElem{
					{Name: "System"},
					{Name: "time-items"},
					{Name: "srcIf-items"},
					{Name: "srcIf"},
				}},
				Val: &gpb.TypedValue{
					Value: &gpb.TypedValue_JsonVal{
						JsonVal: []byte(`{"srcIf":"mgmt0"}`),
					},
				},
			},
		},
	}

	c, err := NewClient(t.Context(), cc, false)
	if _, ok := c.(*client); !ok {
		t.Fatalf("expected type *client, got %T", c)
	}
	if err != nil {
		t.Fatalf("unexpected error: got %v, nil", err)
	}
	err = c.Set(t.Context(), n)
	if err != nil {
		t.Fatalf("unexpected error: got %v, want nil", err)
	}
}

func Test_Set_Err(t *testing.T) {
	cc := &GNMIClientMock{
		SetFunc: func(ctx context.Context, in *gpb.SetRequest, opts ...grpc.CallOption) (*gpb.SetResponse, error) {
			return nil, status.Error(codes.Unavailable, "unavailable")
		},
		CapabilitiesFunc: func(ctx context.Context, req *gpb.CapabilityRequest, opts ...grpc.CallOption) (*gpb.CapabilityResponse, error) {
			// Return a mock response or error as needed
			return &gpb.CapabilityResponse{
				SupportedEncodings: []gpb.Encoding{gpb.Encoding_JSON},
				SupportedModels: []*gpb.ModelData{
					{Name: "Cisco-NX-OS-device", Version: "2024-03-26"},
				},
			}, nil
		},
	}

	n := &gpb.Notification{
		Update: []*gpb.Update{
			{
				Path: &gpb.Path{Elem: []*gpb.PathElem{
					{Name: "System"},
					{Name: "time-items"},
					{Name: "srcIf-items"},
					{Name: "srcIf"},
				}},
				Val: &gpb.TypedValue{
					Value: &gpb.TypedValue_JsonVal{
						JsonVal: []byte(`{"srcIf":"mgmt0"}`),
					},
				},
			},
		},
	}
	c, err := NewClient(t.Context(), cc, false)
	if _, ok := c.(*client); !ok {
		t.Fatalf("expected type *client, got %T", c)
	}
	if err != nil {
		t.Fatalf("unexpected error: got %v, nil", err)
	}
	err = c.Set(t.Context(), n)
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("unexpected error: got %v, want status.Err with code unavailable", err)
	}
}

func Test_Set_DryRun(t *testing.T) {
	cc := &GNMIClientMock{
		SetFunc: func(ctx context.Context, in *gpb.SetRequest, opts ...grpc.CallOption) (*gpb.SetResponse, error) {
			t.Fatal("unexpected call")
			return nil, nil
		},
	}

	n := &gpb.Notification{
		Update: []*gpb.Update{
			{
				Path: &gpb.Path{Elem: []*gpb.PathElem{
					{Name: "System"},
					{Name: "time-items"},
					{Name: "srcIf-items"},
					{Name: "srcIf"},
				}},
				Val: &gpb.TypedValue{
					Value: &gpb.TypedValue_JsonVal{
						JsonVal: []byte(`{"srcIf":"mgmt0"}`),
					},
				},
			},
		},
	}

	if err := (&client{c: cc, dryRun: true}).Set(t.Context(), n); err != nil {
		t.Fatalf("unexpected error: got %v, want nil", err)
	}
}

func Test_Set_NoOp(t *testing.T) {
	cc := &GNMIClientMock{
		SetFunc: func(ctx context.Context, in *gpb.SetRequest, opts ...grpc.CallOption) (*gpb.SetResponse, error) {
			t.Fatal("unexpected call")
			return nil, nil
		},
	}

	if err := (&client{c: cc, dryRun: true}).Set(t.Context(), &gpb.Notification{Update: []*gpb.Update{}}); err != nil {
		t.Fatalf("unexpected error: got %v, want nil", err)
	}
}

var _ DeviceConf = (*Dummy)(nil)

type Dummy struct{ srcIf string }

func (d *Dummy) ToYGOT(_ context.Context, _ Client) ([]Update, error) {
	return []Update{
		EditingUpdate{
			XPath: "System/time-items/srcIf-items",
			Value: &nxos.Cisco_NX_OSDevice_System_TimeItems_SrcIfItems{SrcIf: ygot.String(d.srcIf)},
		},
	}, nil
}

func (*Dummy) Reset(_ context.Context, _ Client) ([]Update, error) {
	return nil, errors.New("not implemented")
}

type DummyWithError struct{}

func (*DummyWithError) ToYGOT(_ context.Context, _ Client) ([]Update, error) {
	return nil, errors.New("YGOT error")
}

func (*DummyWithError) Reset(_ context.Context, _ Client) ([]Update, error) {
	return nil, errors.New("not implemented")
}

func Test_Update(t *testing.T) {
	cc := newMockGNMIClient()
	cc.GetFunc = func(_ context.Context, in *gpb.GetRequest, _ ...grpc.CallOption) (*gpb.GetResponse, error) {
		if in.Type != gpb.GetRequest_CONFIG {
			t.Fatalf("unexpected type: %v", in.Type)
		}
		if in.Encoding != gpb.Encoding_JSON {
			t.Fatalf("unexpected encoding: %v", in.Encoding)
		}
		return &gpb.GetResponse{
			Notification: []*gpb.Notification{
				{
					Update: []*gpb.Update{
						{
							Path: &gpb.Path{Elem: []*gpb.PathElem{
								{Name: "System"},
								{Name: "time-items"},
								{Name: "srcIf-items"},
							}},
							Val: &gpb.TypedValue{
								Value: &gpb.TypedValue_JsonVal{
									JsonVal: []byte(`{"srcIf":"mgmt0"}`),
								},
							},
						},
					},
				},
			},
		}, nil
	}
	cc.SetFunc = func(ctx context.Context, in *gpb.SetRequest, opts ...grpc.CallOption) (*gpb.SetResponse, error) {
		if len(in.Delete) == 0 && len(in.Update) == 0 {
			t.Fatal("unexpected empty diff")
		}
		return &gpb.SetResponse{}, nil
	}

	c, err := NewClient(t.Context(), cc, false)
	if _, ok := c.(*client); !ok {
		t.Fatalf("expected type *client, got %T", c)
	}
	if err != nil {
		t.Fatalf("unexpected error: got %v, nil", err)
	}

	d := &Dummy{srcIf: "mgmt1"}
	err = c.Update(t.Context(), d)
	if err != nil {
		t.Fatalf("unexpected error: got %v, want nil", err)
	}
}

func Test_Update_ToYGOTError(t *testing.T) {
	cc := newMockGNMIClient()
	cc.GetFunc = func(_ context.Context, in *gpb.GetRequest, _ ...grpc.CallOption) (*gpb.GetResponse, error) {
		return &gpb.GetResponse{
			Notification: []*gpb.Notification{
				{
					Update: []*gpb.Update{},
				},
			},
		}, nil
	}
	cc.SetFunc = func(ctx context.Context, in *gpb.SetRequest, opts ...grpc.CallOption) (*gpb.SetResponse, error) {
		return &gpb.SetResponse{}, nil
	}

	d := &DummyWithError{}
	err := (&client{c: cc}).Update(t.Context(), d)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

type DummyWithValidationError struct{}

func (*DummyWithValidationError) ToYGOT(_ context.Context, _ Client) ([]Update, error) {
	return []Update{
		EditingUpdate{
			XPath: "",
			Value: &nxos.Cisco_NX_OSDevice_System_Ipv4Items_InstItems_DomItems_DomList_IfItems_IfList_AddrItems_AddrList{
				Addr: ygot.String("not-an-ip-address"),
			},
		},
	}, nil
}

func (*DummyWithValidationError) Reset(_ context.Context, _ Client) ([]Update, error) {
	return nil, errors.New("not implemented")
}

func Test_Update_ValidationFails(t *testing.T) {
	cc := newMockGNMIClient()
	cc.GetFunc = func(_ context.Context, in *gpb.GetRequest, _ ...grpc.CallOption) (*gpb.GetResponse, error) {
		return &gpb.GetResponse{
			Notification: []*gpb.Notification{
				{
					Update: []*gpb.Update{},
				},
			},
		}, nil
	}
	cc.SetFunc = func(ctx context.Context, in *gpb.SetRequest, opts ...grpc.CallOption) (*gpb.SetResponse, error) {
		return &gpb.SetResponse{}, nil
	}

	d := &DummyWithValidationError{}
	err := (&client{c: cc}).Update(t.Context(), d)
	if err == nil || !strings.Contains(err.Error(), `"not-an-ip-address" does not match regular expression pattern`) {
		t.Fatalf("unexpected error: got %v, want validation error containing 'does not match regular expression pattern'", err)
	}
}

func Test_Diff(t *testing.T) {
	a := &nxos.Cisco_NX_OSDevice_System_TimeItems_SrcIfItems{SrcIf: ygot.String("mgmt0")}
	b := &nxos.Cisco_NX_OSDevice_System_TimeItems_SrcIfItems{SrcIf: ygot.String("mgmt1")}
	diff, err := diff("System/time-items/srcIf-items", a, b)
	if err != nil {
		t.Fatalf("unexpected error: got %v, want nil", err)
	}

	want := &gpb.Notification{
		Update: []*gpb.Update{
			{
				Path: &gpb.Path{Elem: []*gpb.PathElem{
					{Name: "System"},
					{Name: "time-items"},
					{Name: "srcIf-items"},
					{Name: "srcIf"},
				}},
				Val: &gpb.TypedValue{
					Value: &gpb.TypedValue_JsonVal{
						JsonVal: []byte(`"mgmt1"`),
					},
				},
			},
		},
	}
	if !proto.Equal(diff, want) {
		t.Fatalf("unexpected diff: got %v, want %v", diff, want)
	}
}

func Test_Diff_Empty(t *testing.T) {
	a := &nxos.Cisco_NX_OSDevice_System_TimeItems_SrcIfItems{SrcIf: ygot.String("mgmt0")}
	b := &nxos.Cisco_NX_OSDevice_System_TimeItems_SrcIfItems{SrcIf: ygot.String("mgmt0")}
	diff, err := diff("System/time-items/srcIf-items", a, b)
	if err != nil {
		t.Fatalf("unexpected error: got %v, want nil", err)
	}

	if !proto.Equal(diff, &gpb.Notification{}) {
		t.Fatalf("unexpected diff: got %v, want empty notification", diff)
	}
}

func Test_Diff_Partial(t *testing.T) {
	a := &nxos.Cisco_NX_OSDevice_System_TimeItems{Logging: nxos.Cisco_NX_OSDevice_Datetime_AdminState_enabled, LoggingLevel: nxos.Cisco_NX_OSDevice_Datetime_LoggingLevel_critical}
	b := &nxos.Cisco_NX_OSDevice_System_TimeItems{Logging: nxos.Cisco_NX_OSDevice_Datetime_AdminState_disabled}
	diff, err := diff("System/time-items", a, b, ygot.MustStringToPath("/loggingLevel"))
	if err != nil {
		t.Fatalf("unexpected error: got %v, want nil", err)
	}

	// The diff should only contain the fields that explicitly defined in both states.
	want := &gpb.Notification{
		Update: []*gpb.Update{
			{
				Path: &gpb.Path{
					Elem: []*gpb.PathElem{
						{Name: "System"},
						{Name: "time-items"},
						{Name: "logging"},
					},
				},
				Val: &gpb.TypedValue{
					Value: &gpb.TypedValue_JsonVal{JsonVal: []byte(`"disabled"`)},
				},
			},
		},
	}

	if !proto.Equal(diff, want) {
		t.Fatalf("unexpected diff: got %v, want %v", diff, want)
	}
}

func Test_Diff_List(t *testing.T) {
	a := &nxos.Cisco_NX_OSDevice_System_TimeItems_ProvItems{
		NtpProviderList: map[string]*nxos.Cisco_NX_OSDevice_System_TimeItems_ProvItems_NtpProviderList{
			"147.204.9.202": {
				Name:      ygot.String("147.204.9.202"),
				Preferred: ygot.Bool(true),
				ProvT:     nxos.Cisco_NX_OSDevice_Datetime_ProvT_server,
				Vrf:       ygot.String("CC-MGMT"),
			},
			"147.204.9.203": {
				Name:      ygot.String("147.204.9.203"),
				Preferred: ygot.Bool(true),
				ProvT:     nxos.Cisco_NX_OSDevice_Datetime_ProvT_server,
				Vrf:       ygot.String("CC-MGMT"),
			},
		},
	}
	b := &nxos.Cisco_NX_OSDevice_System_TimeItems_ProvItems{
		NtpProviderList: map[string]*nxos.Cisco_NX_OSDevice_System_TimeItems_ProvItems_NtpProviderList{
			"147.204.9.202": {
				Name:      ygot.String("147.204.9.202"),
				Preferred: ygot.Bool(true),
				ProvT:     nxos.Cisco_NX_OSDevice_Datetime_ProvT_server,
				Vrf:       ygot.String("CC-MGMT"),
			},
		},
	}
	diff, err := diff("System/time-items/prov-items", a, b)
	if err != nil {
		t.Fatalf("unexpected error: got %v, want nil", err)
	}

	want := &gpb.Notification{
		Delete: []*gpb.Path{
			{
				Elem: []*gpb.PathElem{
					{Name: "System"},
					{Name: "time-items"},
					{Name: "prov-items"},
					{Name: "NtpProvider-list", Key: map[string]string{"name": "147.204.9.203"}},
				},
			},
		},
	}

	if !proto.Equal(diff, want) {
		t.Fatalf("unexpected diff: got %v, want %v", diff, want)
	}
}

func Test_Diff_List_Nested(t *testing.T) {
	a := &nxos.Cisco_NX_OSDevice_System_TimeItems{
		ProvItems: &nxos.Cisco_NX_OSDevice_System_TimeItems_ProvItems{
			NtpProviderList: map[string]*nxos.Cisco_NX_OSDevice_System_TimeItems_ProvItems_NtpProviderList{
				"147.204.9.202": {
					Name:      ygot.String("147.204.9.202"),
					Preferred: ygot.Bool(true),
					ProvT:     nxos.Cisco_NX_OSDevice_Datetime_ProvT_server,
					Vrf:       ygot.String("CC-MGMT"),
				},
				"147.204.9.203": {
					Name:      ygot.String("147.204.9.203"),
					Preferred: ygot.Bool(true),
					ProvT:     nxos.Cisco_NX_OSDevice_Datetime_ProvT_server,
					Vrf:       ygot.String("CC-MGMT"),
				},
			},
		},
	}
	b := &nxos.Cisco_NX_OSDevice_System_TimeItems{
		ProvItems: &nxos.Cisco_NX_OSDevice_System_TimeItems_ProvItems{
			NtpProviderList: map[string]*nxos.Cisco_NX_OSDevice_System_TimeItems_ProvItems_NtpProviderList{
				"147.204.9.202": {
					Name:      ygot.String("147.204.9.202"),
					Preferred: ygot.Bool(true),
					ProvT:     nxos.Cisco_NX_OSDevice_Datetime_ProvT_server,
					Vrf:       ygot.String("CC-MGMT"),
				},
			},
		},
	}
	diff, err := diff("System/time-items", a, b)
	if err != nil {
		t.Fatalf("unexpected error: got %v, want nil", err)
	}

	want := &gpb.Notification{
		Delete: []*gpb.Path{
			{
				Elem: []*gpb.PathElem{
					{Name: "System"},
					{Name: "time-items"},
					{Name: "prov-items"},
					{Name: "NtpProvider-list", Key: map[string]string{"name": "147.204.9.203"}},
				},
			},
		},
	}

	if !proto.Equal(diff, want) {
		t.Fatalf("unexpected diff: got %v, want %v", diff, want)
	}
}

func Test_Diff_List_Nested_List(t *testing.T) {
	a := &nxos.Cisco_NX_OSDevice_System_DnsItems{
		ProfItems: &nxos.Cisco_NX_OSDevice_System_DnsItems_ProfItems{
			ProfList: map[string]*nxos.Cisco_NX_OSDevice_System_DnsItems_ProfItems_ProfList{
				"default": {
					Name: ygot.String("default"),
					VrfItems: &nxos.Cisco_NX_OSDevice_System_DnsItems_ProfItems_ProfList_VrfItems{
						VrfList: map[string]*nxos.Cisco_NX_OSDevice_System_DnsItems_ProfItems_ProfList_VrfItems_VrfList{
							"CC-MGMT": {
								Name: ygot.String("CC-MGMT"),
								ProvItems: &nxos.Cisco_NX_OSDevice_System_DnsItems_ProfItems_ProfList_VrfItems_VrfList_ProvItems{
									ProviderList: map[string]*nxos.Cisco_NX_OSDevice_System_DnsItems_ProfItems_ProfList_VrfItems_VrfList_ProvItems_ProviderList{
										"127.0.0.1": {
											Addr:  ygot.String("127.0.0.1"),
											SrcIf: ygot.String("mgmt0"),
										},
										"127.0.0.2": {
											Addr:  ygot.String("127.0.0.2"),
											SrcIf: ygot.String("mgmt0"),
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	b := &nxos.Cisco_NX_OSDevice_System_DnsItems{
		ProfItems: &nxos.Cisco_NX_OSDevice_System_DnsItems_ProfItems{
			ProfList: map[string]*nxos.Cisco_NX_OSDevice_System_DnsItems_ProfItems_ProfList{
				"default": {
					Name: ygot.String("default"),
					VrfItems: &nxos.Cisco_NX_OSDevice_System_DnsItems_ProfItems_ProfList_VrfItems{
						VrfList: map[string]*nxos.Cisco_NX_OSDevice_System_DnsItems_ProfItems_ProfList_VrfItems_VrfList{
							"CC-MGMT": {
								Name: ygot.String("CC-MGMT"),
								ProvItems: &nxos.Cisco_NX_OSDevice_System_DnsItems_ProfItems_ProfList_VrfItems_VrfList_ProvItems{
									ProviderList: map[string]*nxos.Cisco_NX_OSDevice_System_DnsItems_ProfItems_ProfList_VrfItems_VrfList_ProvItems_ProviderList{
										"127.0.0.1": {
											Addr:  ygot.String("127.0.0.1"),
											SrcIf: ygot.String("mgmt0"),
										},
										"127.0.0.3": {
											Addr:  ygot.String("127.0.0.3"),
											SrcIf: ygot.String("mgmt0"),
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	diff, err := diff("System/dns-items", a, b)
	if err != nil {
		t.Fatalf("unexpected error: got %v, want nil", err)
	}

	want := &gpb.Notification{
		Delete: []*gpb.Path{
			{
				Elem: []*gpb.PathElem{
					{Name: "System"},
					{Name: "dns-items"},
					{Name: "prof-items"},
					{Name: "Prof-list", Key: map[string]string{"name": "default"}},
					{Name: "vrf-items"},
					{Name: "Vrf-list", Key: map[string]string{"name": "CC-MGMT"}},
					{Name: "prov-items"},
					{Name: "Provider-list", Key: map[string]string{"addr": "127.0.0.2"}},
				},
			},
		},
		Update: []*gpb.Update{
			{
				Path: &gpb.Path{
					Elem: []*gpb.PathElem{
						{Name: "System"},
						{Name: "dns-items"},
						{Name: "prof-items"},
						{Name: "Prof-list", Key: map[string]string{"name": "default"}},
						{Name: "vrf-items"},
						{Name: "Vrf-list", Key: map[string]string{"name": "CC-MGMT"}},
						{Name: "prov-items"},
						{Name: "Provider-list", Key: map[string]string{"addr": "127.0.0.3"}},
						{Name: "srcIf"},
					},
				},
				Val: &gpb.TypedValue{
					Value: &gpb.TypedValue_JsonVal{JsonVal: []byte("\"mgmt0\"")},
				},
			},
		},
	}

	if !proto.Equal(diff, want) {
		t.Fatalf("unexpected diff: got \n%v, want \n%v", diff, want)
	}
}

func Test_Diff_Leaf(t *testing.T) {
	a := &nxos.Cisco_NX_OSDevice_System_TimeItems_ProvItems{
		NtpProviderList: map[string]*nxos.Cisco_NX_OSDevice_System_TimeItems_ProvItems_NtpProviderList{
			"147.204.9.202": {
				Name:      ygot.String("147.204.9.202"),
				Preferred: ygot.Bool(true),
				ProvT:     nxos.Cisco_NX_OSDevice_Datetime_ProvT_server,
				Vrf:       ygot.String("CC-MGMT"),
			},
		},
	}
	b := &nxos.Cisco_NX_OSDevice_System_TimeItems_ProvItems{
		NtpProviderList: map[string]*nxos.Cisco_NX_OSDevice_System_TimeItems_ProvItems_NtpProviderList{
			"147.204.9.202": {
				Name:      ygot.String("147.204.9.202"),
				Preferred: ygot.Bool(false),
				ProvT:     nxos.Cisco_NX_OSDevice_Datetime_ProvT_server,
				Vrf:       ygot.String("CC-MGMT"),
			},
		},
	}
	diff, err := diff("System/time-items/prov-items", a, b)
	if err != nil {
		t.Fatalf("unexpected error: got %v, want nil", err)
	}

	want := &gpb.Notification{
		Update: []*gpb.Update{
			{
				Path: &gpb.Path{
					Elem: []*gpb.PathElem{
						{Name: "System"},
						{Name: "time-items"},
						{Name: "prov-items"},
						{Name: "NtpProvider-list", Key: map[string]string{"name": "147.204.9.202"}},
						{Name: "preferred"},
					},
				},
				Val: &gpb.TypedValue{
					Value: &gpb.TypedValue_JsonVal{JsonVal: []byte("false")},
				},
			},
		},
	}

	if !proto.Equal(diff, want) {
		t.Fatalf("unexpected diff: got %v, want %v", diff, want)
	}
}

func Test_valueToJSON(t *testing.T) {
	tests := []struct {
		name    string
		in, out *gpb.TypedValue
	}{
		{
			name: "string",
			in:   &gpb.TypedValue{Value: &gpb.TypedValue_StringVal{StringVal: "foo"}},
			out:  &gpb.TypedValue{Value: &gpb.TypedValue_JsonVal{JsonVal: []byte(`"foo"`)}},
		},
		{
			name: "int",
			in:   &gpb.TypedValue{Value: &gpb.TypedValue_IntVal{IntVal: 42}},
			out:  &gpb.TypedValue{Value: &gpb.TypedValue_JsonVal{JsonVal: []byte(`42`)}},
		},
		{
			name: "unint",
			in:   &gpb.TypedValue{Value: &gpb.TypedValue_UintVal{UintVal: 42}},
			out:  &gpb.TypedValue{Value: &gpb.TypedValue_JsonVal{JsonVal: []byte(`42`)}},
		},
		{
			name: "bool",
			in:   &gpb.TypedValue{Value: &gpb.TypedValue_BoolVal{BoolVal: true}},
			out:  &gpb.TypedValue{Value: &gpb.TypedValue_JsonVal{JsonVal: []byte(`true`)}},
		},
		{
			name: "bytes",
			in:   &gpb.TypedValue{Value: &gpb.TypedValue_BytesVal{BytesVal: []byte{1, 2, 3}}},
			out:  &gpb.TypedValue{Value: &gpb.TypedValue_JsonVal{JsonVal: []byte(`"AQID"`)}},
		},
		{
			name: "float",
			in:   &gpb.TypedValue{Value: &gpb.TypedValue_FloatVal{FloatVal: 0.5}},
			out:  &gpb.TypedValue{Value: &gpb.TypedValue_JsonVal{JsonVal: []byte(`0.5`)}},
		},
		{
			name: "double",
			in:   &gpb.TypedValue{Value: &gpb.TypedValue_DoubleVal{DoubleVal: 0.5}},
			out:  &gpb.TypedValue{Value: &gpb.TypedValue_JsonVal{JsonVal: []byte(`0.5`)}},
		},
		{
			name: "json",
			in:   &gpb.TypedValue{Value: &gpb.TypedValue_JsonVal{JsonVal: []byte(`{"foo":"bar"}`)}},
			out:  &gpb.TypedValue{Value: &gpb.TypedValue_JsonVal{JsonVal: []byte(`{"foo":"bar"}`)}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			out, err := valueToJSON(test.in)
			if err != nil {
				t.Fatalf("unexpected error: got %v, want nil", err)
			}
			if !proto.Equal(out, test.out) {
				t.Fatalf("unexpected output: got %v, want %v", out, test.out)
			}
		})
	}
}

func Test_valueToJSON_Err(t *testing.T) {
	tests := []struct {
		name string
		in   *gpb.TypedValue
	}{
		{
			name: "decimal",
			in:   &gpb.TypedValue{Value: &gpb.TypedValue_DecimalVal{DecimalVal: &gpb.Decimal64{Digits: 42, Precision: 2}}}, //nolint:staticcheck
		},
		{
			name: "leaf-list",
			in:   &gpb.TypedValue{Value: &gpb.TypedValue_LeaflistVal{LeaflistVal: &gpb.ScalarArray{Element: []*gpb.TypedValue{}}}},
		},
		{
			name: "any",
			in:   &gpb.TypedValue{Value: &gpb.TypedValue_AnyVal{AnyVal: &anypb.Any{}}},
		},
		{
			name: "json-ietf",
			in:   &gpb.TypedValue{Value: &gpb.TypedValue_JsonIetfVal{JsonIetfVal: []byte(`{"foo":"bar"}`)}},
		},
		{
			name: "ascii",
			in:   &gpb.TypedValue{Value: &gpb.TypedValue_AsciiVal{AsciiVal: "foo"}},
		},
		{
			name: "proto",
			in:   &gpb.TypedValue{Value: &gpb.TypedValue_ProtoBytes{ProtoBytes: []byte{1, 2, 3}}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			out, err := valueToJSON(test.in)
			if err == nil {
				t.Fatalf("unexpected error: got %v, nil", err)
			}
			if out != nil {
				t.Fatalf("unexpected output: got %v, want nil", out)
			}
		})
	}
}

func Test_Set_SplitSetRequestsIntoChunks(t *testing.T) {
	// Define test cases with different combinations of maxPathsPerRequest and number of updates
	testCases := []struct {
		name               string
		maxPathsPerRequest int
		numUpdates         int
		expectedCalls      int
	}{
		{"BatchSize20_Updates45", 20, 45, 3},   // batch size 20, updates 45 -> 3 calls
		{"BatchSize10_Updates25", 10, 25, 3},   // batch size 10, updates 25 -> 3 calls
		{"BatchSize15_Updates30", 15, 30, 2},   // batch size 15, updates 30 -> 2 calls
		{"BatchSize50_Updates100", 50, 100, 2}, // batch size 50, updates 100 -> 2 calls
		{"BatchSize5_Updates12", 5, 12, 3},     // batch size 5, updates 12 -> 3 calls
		{"BatchSize1_Updates3", 1, 3, 3},       // batch size 1, updates 3 -> 3 calls
		{"BatchSize10_Updates3", 10, 3, 1},     // batch size 10, updates 3 -> 1 call
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var setCallCount int

			// Mock GNMIClient
			cc := newMockGNMIClient()
			cc.SetFunc = func(ctx context.Context, in *gpb.SetRequest, opts ...grpc.CallOption) (*gpb.SetResponse, error) {
				setCallCount++
				return &gpb.SetResponse{}, nil
			}

			// Create a client with the mocked GNMIClient
			client := &client{
				c:                  cc,
				maxPathsPerRequest: tc.maxPathsPerRequest, // Use the batch size from the test case
			}

			// Create a mock diff with the specified number of updates
			mockDiff := &gpb.Notification{
				Update: make([]*gpb.Update, tc.numUpdates), // Use the number of updates from the test case
			}

			// Call the Set method
			err := client.Set(t.Context(), mockDiff)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify that Set was called the expected number of times
			if setCallCount != tc.expectedCalls {
				t.Fatalf("expected Set to be called %d times, but got %d", tc.expectedCalls, setCallCount)
			}
		})
	}
}
