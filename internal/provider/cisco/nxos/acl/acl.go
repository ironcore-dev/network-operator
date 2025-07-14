// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package acl

import (
	"fmt"
	"net/netip"

	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

var _ gnmiext.DeviceConf = (*ACL)(nil)

// ACL represents an access control list.
type ACL struct {
	// The items of the access control list.
	Items []*Item
}

type Item struct {
	// The name of the access list.
	Name string
	// A list of rules to apply.
	Rules []*Rule
}

type Rule struct {
	// The sequence number of the rule.
	Seq uint32
	// The action type of ACL Rule. Either 'permit' or 'deny'.
	Action Action
	// Source IP address or network. Use 'any' to match any source.
	Source netip.Prefix
	// Destination IP address or network. Use 'any' to match any target.
	Destination netip.Prefix
}

//go:generate go run golang.org/x/tools/cmd/stringer@v0.35.0 -type=Action
type Action uint8

const (
	Permit Action = iota
	Deny
)

// returns a single update forcing the removal of existing ACLs on the target device. Differential updates
// are currently not applicable to ACLs; the entire tree must be replaced.
func (a *ACL) ToYGOT(_ gnmiext.Client) ([]gnmiext.Update, error) {
	items := &nxos.Cisco_NX_OSDevice_System_AclItems_Ipv4Items_NameItems{}
	items.PopulateDefaults()
	for _, i := range a.Items {
		aclList := items.GetOrCreateACLList(i.Name)
		aclList.PopulateDefaults()
		aclListItems := aclList.GetOrCreateSeqItems()
		aclListItems.PopulateDefaults()

		for _, r := range i.Rules {
			aceList := aclListItems.GetOrCreateACEList(r.Seq)
			aceList.PopulateDefaults()
			// seq
			aceList.SeqNum = ygot.Uint32(r.Seq)
			// action
			var action nxos.E_Cisco_NX_OSDevice_Acl_ActionType
			switch r.Action {
			case Permit:
				action = nxos.Cisco_NX_OSDevice_Acl_ActionType_permit
			case Deny:
				action = nxos.Cisco_NX_OSDevice_Acl_ActionType_deny
			default:
				return nil, fmt.Errorf("acl: invalid action type: %s", r.Action)
			}
			aceList.Action = action
			aceList.SrcPrefix = ygot.String(r.Source.Addr().String())
			aceList.SrcPrefixLength = ygot.Uint8(uint8(r.Source.Bits())) //nolint:gosec
			aceList.DstPrefix = ygot.String(r.Destination.Addr().String())
			aceList.DstPrefixLength = ygot.Uint8(uint8(r.Destination.Bits())) //nolint:gosec
		}
	}
	return []gnmiext.Update{
		gnmiext.ReplacingUpdate{
			XPath: "System/acl-items/ipv4-items/name-items",
			Value: items,
		},
	}, nil
}

func (v *ACL) Reset(_ gnmiext.Client) ([]gnmiext.Update, error) {
	items := &nxos.Cisco_NX_OSDevice_System_AclItems_Ipv4Items_NameItems{}
	items.PopulateDefaults()

	return []gnmiext.Update{
		gnmiext.ReplacingUpdate{
			XPath: "System/acl-items/ipv4-items/name-items",
			Value: items,
		},
	}, nil
}
