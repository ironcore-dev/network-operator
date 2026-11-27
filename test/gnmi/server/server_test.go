// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"testing"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/tidwall/gjson"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestSetUpdatePreservesChildConfiguration(t *testing.T) {
	for _, replace := range []bool{false, true} {
		name := "update"
		if replace {
			name = "replace"
		}
		t.Run(name, func(t *testing.T) {
			s := &Server{state: &State{}}
			s.state.SetBuf([]byte(`{"System":{"pim-items":{"adminSt":"disabled","inst-items":{"adminSt":"disabled","rp-members":["10.0.0.1","10.0.0.2"]}}}}`))
			u := &gpb.Update{
				Path: &gpb.Path{Elem: []*gpb.PathElem{{Name: "System"}, {Name: "pim-items"}}},
				Val:  &gpb.TypedValue{Value: &gpb.TypedValue_JsonVal{JsonVal: []byte(`{"adminSt":"enabled","inst-items":{"adminSt":"enabled"}}`)}},
			}
			req := &gpb.SetRequest{}
			if replace {
				req.Replace = []*gpb.Update{u}
			} else {
				req.Update = []*gpb.Update{u}
			}
			if _, err := s.Set(t.Context(), req); err != nil {
				t.Fatal(err)
			}
			state := s.state.GetBuf()
			if got := gjson.GetBytes(state, "System.pim-items.inst-items.adminSt").String(); got != "enabled" {
				t.Fatalf("expected enabled PIM, got %q", got)
			}
			members := gjson.GetBytes(state, "System.pim-items.inst-items.rp-members")
			if replace {
				if members.Exists() {
					t.Fatal("replace retained omitted child configuration")
				}
			} else if members.Raw != `["10.0.0.1","10.0.0.2"]` {
				t.Fatalf("update did not preserve RP members: %s", members.Raw)
			}
		})
	}
}

func TestSetInvalidValueRollsBack(t *testing.T) {
	for _, replace := range []bool{false, true} {
		name := "update"
		if replace {
			name = "replace"
		}
		t.Run(name, func(t *testing.T) {
			s := &Server{state: &State{}}
			const initial = `{"a":{"b":false}}`
			s.state.SetBuf([]byte(initial))
			invalid := &gpb.Update{
				Path: &gpb.Path{Elem: []*gpb.PathElem{{Name: "a"}}},
				Val:  &gpb.TypedValue{Value: &gpb.TypedValue_JsonVal{JsonVal: []byte(`{"b":`)}},
			}
			req := &gpb.SetRequest{
				Delete: []*gpb.Path{{Elem: []*gpb.PathElem{{Name: "a"}, {Name: "b"}}}},
				Update: []*gpb.Update{invalid},
			}
			if replace {
				req.Update = nil
				req.Replace = []*gpb.Update{invalid}
			}
			if _, err := s.Set(t.Context(), req); status.Code(err) != codes.InvalidArgument {
				t.Fatalf("expected InvalidArgument, got %v", err)
			}
			if got := string(s.state.GetBuf()); got != initial {
				t.Fatalf("failed Set changed state: %s", got)
			}
		})
	}
}
