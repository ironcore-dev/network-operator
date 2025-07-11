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
	Source string
	// Destination IP address or network. Use 'any' to match any target.
	Destination string
}

//go:generate go tool stringer -type=Action
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
			// src
			src, err := parse(r.Source)
			if err != nil {
				return nil, err
			}
			aceList.SrcPrefix = ygot.String(src.Addr().String())
			aceList.SrcPrefixLength = ygot.Uint8(uint8(src.Bits())) //nolint:gosec
			// dst
			if r.Destination == "" {
				r.Destination = "any"
			}
			dst, err := parse(r.Destination)
			if err != nil {
				return nil, err
			}
			aceList.DstPrefix = ygot.String(dst.Addr().String())
			aceList.DstPrefixLength = ygot.Uint8(uint8(dst.Bits())) //nolint:gosec
		}
	}
	return []gnmiext.Update{
		gnmiext.ReplacingUpdate{
			XPath: "System/acl-items/ipv4-items/name-items",
			Value: items,
		},
	}, nil
}

// parse a CIDR string and returns it's [netip.Prefix].
// If the CIDR is "any", it returns '0.0.0.0/0' as the default.
func parse(cidr string) (p netip.Prefix, err error) {
	p = netip.PrefixFrom(netip.AddrFrom4([4]byte{0, 0, 0, 0}), 0)
	if cidr != "any" {
		p, err = netip.ParsePrefix(cidr)
		if err == nil && !p.IsValid() {
			err = fmt.Errorf("acl: invalid network CIDR: %s", cidr)
		}
	}
	return
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
