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
		Items: []*Item{
			{
				Name: "ACL-SNMP-VTY",
				Rules: []*Rule{
					{
						Seq:         10,
						Action:      Permit,
						Source:      netip.MustParsePrefix("10.0.0.0/8"),
						Destination: netip.MustParsePrefix("0.0.0.0/0"),
					},
				},
			},
		},
	}

	got, err := a.ToYGOT(&gnmiext.ClientMock{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Errorf("expected 1 key, got %d", len(got))
	}

	update, ok := got[0].(gnmiext.ReplacingUpdate)
	if !ok {
		t.Errorf("expected value to be of type ReplacingUpdate")
	}

	if update.XPath != "System/acl-items/ipv4-items/name-items" {
		t.Errorf("expected key 'System/acl-items/ipv4-items/name-items' to be present")
	}

	items, ok := update.Value.(*nxos.Cisco_NX_OSDevice_System_AclItems_Ipv4Items_NameItems)
	if !ok {
		t.Errorf("expected value to be of type *nxos.Cisco_NX_OSDevice_System_AclItems_Ipv4Items_NameItems")
	}

	if len(items.ACLList) != 1 {
		t.Errorf("expected 1 ACL, got %d", len(items.ACLList))
	}

	if len(items.ACLList["ACL-SNMP-VTY"].SeqItems.ACEList) != 1 {
		t.Errorf("expected 1 rule, got %d", len(items.ACLList["ACL-SNMP-VTY"].SeqItems.ACEList))
	}

	if items.ACLList["ACL-SNMP-VTY"].SeqItems.ACEList[10].Action != nxos.Cisco_NX_OSDevice_Acl_ActionType_permit {
		t.Errorf("expected action to be 'permit', got %v", items.ACLList["ACL-SNMP-VTY"].SeqItems.ACEList[10].Action)
	}

	if *items.ACLList["ACL-SNMP-VTY"].SeqItems.ACEList[10].SrcPrefix != "10.0.0.0" {
		t.Errorf("expected source prefix to be '10.0.0.0', got %v", *items.ACLList["ACL-SNMP-VTY"].SeqItems.ACEList[10].SrcPrefix)
	}

	if *items.ACLList["ACL-SNMP-VTY"].SeqItems.ACEList[10].SrcPrefixLength != 8 {
		t.Errorf("expected source mask to be '8', got %d", *items.ACLList["ACL-SNMP-VTY"].SeqItems.ACEList[10].SrcPrefixLength)
	}

	if *items.ACLList["ACL-SNMP-VTY"].SeqItems.ACEList[10].DstPrefix != "0.0.0.0" {
		t.Errorf("expected source prefix to be '0.0.0.0', got %v", *items.ACLList["ACL-SNMP-VTY"].SeqItems.ACEList[10].DstPrefix)
	}

	if *items.ACLList["ACL-SNMP-VTY"].SeqItems.ACEList[10].DstPrefixLength != 0 {
		t.Errorf("expected source mask to be '0', got %d", *items.ACLList["ACL-SNMP-VTY"].SeqItems.ACEList[10].DstPrefixLength)
	}
}
