// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package term

import (
	"testing"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

func Test_Console(t *testing.T) {
	c := &Console{Timeout: 10}

	got, err := c.ToYGOT(t.Context(), &gnmiext.ClientMock{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Errorf("expected 1 key, got %d", len(got))
	}

	update, ok := got[0].(gnmiext.EditingUpdate)
	if !ok {
		t.Errorf("expected value to be of type EditingUpdate")
	}

	if update.XPath != "System/terml-items/ln-items/cons-items/execTmeout-items" {
		t.Error("expected key 'System/terml-items/ln-items/cons-items/execTmeout-items' to be present")
	}

	i, ok := update.Value.(*nxos.Cisco_NX_OSDevice_System_TermlItems_LnItems_ConsItems_ExecTmeoutItems)
	if !ok {
		t.Error("expected value to be of type *nxos.Cisco_NX_OSDevice_System_TermlItems_LnItems_ConsItems_ExecTmeoutItems")
	}

	if i.Timeout == nil {
		t.Error("expected value for 'System/terml-items/ln-items/cons-items/execTmeout-items/timeout' to be present, got <nil>")
	}

	if *i.Timeout != 10 {
		t.Errorf("expected value for 'System/terml-items/ln-items/cons-items/execTmeout-items/timeout' to be 10, got %d", *i.Timeout)
	}
}

func Test_VTY(t *testing.T) {
	v := &VTY{ACL: "example", SessionLimit: 16, Timeout: 10}

	got, err := v.ToYGOT(t.Context(), &gnmiext.ClientMock{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 2 {
		t.Errorf("expected 2 key, got %d", len(got))
	}

	update, ok := got[0].(gnmiext.EditingUpdate)
	if !ok {
		t.Errorf("expected value to be of type EditingUpdate")
	}

	if update.XPath != "System/terml-items/ln-items/vty-items" {
		t.Error("expected key 'System/terml-items/ln-items/vty-items' to be present")
	}

	i, ok := update.Value.(*nxos.Cisco_NX_OSDevice_System_TermlItems_LnItems_VtyItems)
	if !ok {
		t.Error("expected value to be of type *nxos.Cisco_NX_OSDevice_System_TermlItems_LnItems_VtyItems")
	}

	if i.ExecTmeoutItems == nil || i.ExecTmeoutItems.Timeout == nil {
		t.Error("expected value for 'System/terml-items/ln-items/vty-items/execTmeout-items/timeout' to be present, got <nil>")
	}

	if *i.ExecTmeoutItems.Timeout != 10 {
		t.Errorf("expected value for 'System/terml-items/ln-items/vty-items/execTmeout-items/timeout' to be 10, got %d", *i.ExecTmeoutItems.Timeout)
	}

	if i.SsLmtItems == nil || i.SsLmtItems.SesLmt == nil {
		t.Error("expected value for 'System/terml-items/ln-items/vty-items/ssLmt-items/sesLmt' to be present, got <nil>")
	}

	if *i.SsLmtItems.SesLmt != 16 {
		t.Errorf("expected value for 'System/terml-items/ln-items/vty-items/ssLmt-items/sesLmt' to be 16, got %d", *i.SsLmtItems.SesLmt)
	}

	update, ok = got[1].(gnmiext.EditingUpdate)
	if !ok {
		t.Errorf("expected value to be of type EditingUpdate")
	}

	if update.XPath != "System/acl-items/ipv4-items/policy-items/ingress-items/vty-items/acl-items" {
		t.Error("expected key 'System/acl-items/ipv4-items/policy-items/ingress-items/vty-items/acl-items' to be present")
	}

	j, ok := update.Value.(*nxos.Cisco_NX_OSDevice_System_AclItems_Ipv4Items_PolicyItems_IngressItems_VtyItems_AclItems)
	if !ok {
		t.Error("expected value to be of type *nxos.Cisco_NX_OSDevice_System_AclItems_Ipv4Items_PolicyItems_IngressItems_VtyItems_AclItems")
	}

	if j.Name == nil || *j.Name != "example" {
		t.Errorf("expected value for 'System/acl-items/ipv4-items/policy-items/ingress-items/vty-items/acl-items' to be 'example', got %s", *j.Name)
	}
}
