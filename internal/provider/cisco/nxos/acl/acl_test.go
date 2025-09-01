// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package acl

import (
	"net/netip"
	"testing"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

func Test_ACL(t *testing.T) {
	a := &ACL{
		Name: "ACL-SNMP-VTY",
		Rules: []*Rule{
			{
				Seq:         10,
				Action:      Permit,
				Protocol:    IP,
				Description: "Allow internal access",
				Source:      netip.MustParsePrefix("10.0.0.0/8"),
				Destination: netip.MustParsePrefix("0.0.0.0/0"),
			},
		},
	}

	got, err := a.ToYGOT(&gnmiext.ClientMock{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Errorf("expected 1 update, got %d", len(got))
	}

	update, ok := got[0].(gnmiext.ReplacingUpdate)
	if !ok {
		t.Errorf("expected value to be of type ReplacingUpdate")
	}

	expectedXPath := "System/acl-items/ipv4-items/name-items/ACL-list[name=ACL-SNMP-VTY]"
	if update.XPath != expectedXPath {
		t.Errorf("expected XPath '%s', got '%s'", expectedXPath, update.XPath)
	}

	aclList, ok := update.Value.(*nxos.Cisco_NX_OSDevice_System_AclItems_Ipv4Items_NameItems_ACLList)
	if !ok {
		t.Errorf("expected value to be of type *nxos.Cisco_NX_OSDevice_System_AclItems_Ipv4Items_NameItems_ACLList")
	}

	if *aclList.Name != "ACL-SNMP-VTY" {
		t.Errorf("expected ACL name to be 'ACL-SNMP-VTY', got %v", *aclList.Name)
	}

	if len(aclList.SeqItems.ACEList) != 1 {
		t.Errorf("expected 1 rule, got %d", len(aclList.SeqItems.ACEList))
	}

	rule := aclList.SeqItems.ACEList[10]
	if rule.Action != nxos.Cisco_NX_OSDevice_Acl_ActionType_permit {
		t.Errorf("expected action to be 'permit', got %v", rule.Action)
	}

	if *rule.SrcPrefix != "10.0.0.0" {
		t.Errorf("expected source prefix to be '10.0.0.0', got %v", *rule.SrcPrefix)
	}

	if *rule.SrcPrefixLength != 8 {
		t.Errorf("expected source prefix length to be '8', got %d", *rule.SrcPrefixLength)
	}

	if *rule.DstPrefix != "0.0.0.0" {
		t.Errorf("expected destination prefix to be '0.0.0.0', got %v", *rule.DstPrefix)
	}

	if *rule.DstPrefixLength != 0 {
		t.Errorf("expected destination prefix length to be '0', got %d", *rule.DstPrefixLength)
	}

	if *rule.Protocol != uint8(IP) {
		t.Errorf("expected protocol to be '%d' (IP), got %d", uint8(IP), *rule.Protocol)
	}

	if *rule.Remark != "Allow internal access" {
		t.Errorf("expected remark to be 'Allow internal access', got %v", *rule.Remark)
	}
}

func Test_ACL_IPv6(t *testing.T) {
	a := &ACL{
		Name: "ACL-IPv6-TEST",
		Rules: []*Rule{
			{
				Seq:         20,
				Action:      Deny,
				Protocol:    TCP,
				Description: "Block IPv6 traffic",
				Source:      netip.MustParsePrefix("2001:db8::/32"),
				Destination: netip.MustParsePrefix("::/0"),
			},
		},
	}

	got, err := a.ToYGOT(&gnmiext.ClientMock{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Errorf("expected 1 update, got %d", len(got))
	}

	update, ok := got[0].(gnmiext.ReplacingUpdate)
	if !ok {
		t.Errorf("expected value to be of type ReplacingUpdate")
	}

	expectedXPath := "System/acl-items/ipv6-items/name-items/ACL-list[name=ACL-IPv6-TEST]"
	if update.XPath != expectedXPath {
		t.Errorf("expected XPath '%s', got '%s'", expectedXPath, update.XPath)
	}
}

func Test_ACL_Validation_MixedIPVersions(t *testing.T) {
	a := &ACL{
		Name: "ACL-MIXED",
		Rules: []*Rule{
			{
				Seq:         10,
				Action:      Permit,
				Protocol:    IP,
				Source:      netip.MustParsePrefix("10.0.0.0/8"),
				Destination: netip.MustParsePrefix("2001:db8::/32"), // IPv6 destination with IPv4 source
			},
		},
	}

	_, err := a.ToYGOT(&gnmiext.ClientMock{})
	if err == nil {
		t.Errorf("expected validation error for mixed IP versions, got nil")
	}
}

func Test_ACL_EmptyDescription(t *testing.T) {
	a := &ACL{
		Name: "ACL-NO-DESC",
		Rules: []*Rule{
			{
				Seq:         10,
				Action:      Permit,
				Protocol:    UDP,
				Description: "", // Empty description
				Source:      netip.MustParsePrefix("192.168.1.0/24"),
				Destination: netip.MustParsePrefix("0.0.0.0/0"),
			},
		},
	}

	got, err := a.ToYGOT(&gnmiext.ClientMock{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	update := got[0].(gnmiext.ReplacingUpdate)
	aclList := update.Value.(*nxos.Cisco_NX_OSDevice_System_AclItems_Ipv4Items_NameItems_ACLList)
	rule := aclList.SeqItems.ACEList[10]

	// Remark should not be set when description is empty
	if rule.Remark != nil {
		t.Errorf("expected remark to be nil for empty description, got %v", *rule.Remark)
	}

	if *rule.Protocol != uint8(UDP) {
		t.Errorf("expected protocol to be '%d' (UDP), got %d", uint8(UDP), *rule.Protocol)
	}
}
