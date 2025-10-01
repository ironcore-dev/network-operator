// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iosxr

import (
	"bytes"
	"context"
	"encoding/json"
	"net/netip"
	"os"
	"strings"
	"testing"

	"github.com/tidwall/gjson"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"
)

type TestCase struct {
	name string
	val  gnmiext.Configurable
}

var tests []TestCase

func Register(name string, val gnmiext.Configurable) {
	tests = append(tests, TestCase{
		name: name,
		val:  val,
	})
}

func removeRootElement(xpath string) string {
	parts := strings.Split(xpath, "/")
	if len(parts) == 1 {
		return xpath
	}
	return strings.Join(parts[1:], "/")
}

func Test_Payload(t *testing.T) {
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			b, err := json.Marshal(test.val)
			if err != nil {
				t.Errorf("json.Marshal() error = %v", err)
				return
			}

			file := "testdata/" + test.name + ".json"
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("os.ReadFile(%q) error = %v", file, err)
			}

			var buf bytes.Buffer
			if err := json.Compact(&buf, data); err != nil {
				t.Errorf("json.Compact() error = %v", err)
				return
			}

			xpath := removeRootElement(test.val.XPath())
			path, err := gnmiext.StringToStructuredPath(xpath)
			if err != nil {
				t.Errorf("StringToStructuredPath(%q) error = %v", xpath, err)
				return
			}

			var sb strings.Builder
			for _, elem := range path.GetElem() {
				if elem.GetName() == "" {
					continue
				}
				if sb.Len() > 0 {
					sb.WriteByte('|')
				}
				sb.WriteString(elem.GetName())
			}

			res := gjson.GetBytes(buf.Bytes(), sb.String())
			if want := []byte(res.Raw); !bytes.Equal(want, b) {
				t.Errorf("payload mismatch:\nwant: %s\ngot:  %s", want, b)
			}
		})
	}
}

// MockClient provides a mock implementation of gnmiext.Client for testing.
type MockClient struct {
	// Function fields for mocking different methods
	GetConfigFunc func(ctx context.Context, conf ...gnmiext.Configurable) error
	PatchFunc     func(ctx context.Context, conf ...gnmiext.Configurable) error
	UpdateFunc    func(ctx context.Context, conf ...gnmiext.Configurable) error
	DeleteFunc    func(ctx context.Context, conf ...gnmiext.Configurable) error
	GetStateFunc  func(ctx context.Context, conf ...gnmiext.Configurable) error
}

// Implement the methods that Provider uses
func (m *MockClient) GetConfig(ctx context.Context, conf ...gnmiext.Configurable) error {
	if m.GetConfigFunc != nil {
		return m.GetConfigFunc(ctx, conf...)
	}
	return nil
}

func (m *MockClient) Patch(ctx context.Context, conf ...gnmiext.Configurable) error {
	return nil
}

func (m *MockClient) Update(ctx context.Context, conf ...gnmiext.Configurable) error {
	return nil
}

func (m *MockClient) Delete(ctx context.Context, conf ...gnmiext.Configurable) error {
	return nil
}

func (m *MockClient) GetState(ctx context.Context, conf ...gnmiext.Configurable) error {
	if m.GetStateFunc != nil {
		return m.GetStateFunc(ctx, conf...)
	}
	return nil
}

func Test_EnsureInterface(t *testing.T) {
	m := &MockClient{}

	p := &Provider{
		client: m,
		conn:   nil,
	}

	ctx := context.Background()

	var name = "TwentyFiveGigE0/0/0/14"
	var prefix netip.Prefix

	prefix, err := netip.ParsePrefix("192.168.1.0/24")

	if err != nil {
		t.Fatalf("Failed to parse prefix: %v", err)
	}

	ipv4 := v1alpha1.InterfaceIPv4{
		Addresses: []v1alpha1.IPPrefix{
			{
				Prefix: prefix,
			},
		},
	}

	req := &provider.EnsureInterfaceRequest{
		Interface: &v1alpha1.Interface{
			Spec: v1alpha1.InterfaceSpec{
				Name:        name,
				IPv4:        &ipv4,
				Description: "i572056-test-2",
				AdminState:  "UP",
				Type:        "Physical",
				MTU:         9600,
			},
		},
	}

	err = p.EnsureInterface(ctx, req)
	if err != nil {
		t.Fatalf("EnsureInterface() error = %v", err)
	}
}

func Test_GetState(t *testing.T) {
	m := &MockClient{
		GetStateFunc: func(ctx context.Context, conf ...gnmiext.Configurable) error {
			conf[0].(*PhysIfState).State = "im-state-up"
			return nil
		},
	}

	p := &Provider{
		client: m,
		conn:   nil,
	}

	ctx := context.Background()

	var name = "TwentyFiveGigE0/0/0/14"

	req := &provider.InterfaceRequest{
		Interface: &v1alpha1.Interface{
			Spec: v1alpha1.InterfaceSpec{
				Name: name,
			},
		},
	}

	status, err := p.GetInterfaceStatus(ctx, req)
	if err != nil {
		t.Fatalf("EnsureInterface() error = %v", err)
	}

	if status.OperStatus != true {
		t.Fatalf("GetInterfaceStatus() expected OperStatus=true, got false")
	}
}
