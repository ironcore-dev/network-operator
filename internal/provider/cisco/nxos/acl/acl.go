// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package acl

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"

	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

var _ gnmiext.DeviceConf = (*ACL)(nil)

// ACL represents an IPv4 or IPv6 access control list, depending on the rules it contains.
// It can only contain either IPv4 or IPv6 rules, never both.
type ACL struct {
	// The name of the access list.
	// This name must be unique across IPv4 and IPv6 access lists.
	Name string
	// A list of rules to apply.
	Rules []*Rule
}

func (a *ACL) XPath() string {
	if len(a.Rules) > 0 && a.Rules[0].Source.Addr().Is6() {
		return "System/acl-items/ipv6-items/name-items/ACL-list[name=" + a.Name + "]"
	}
	return "System/acl-items/ipv4-items/name-items/ACL-list[name=" + a.Name + "]"
}

func (a *ACL) Validate() error {
	errs := make([]error, 0, len(a.Rules))
	for _, r := range a.Rules {
		if err := r.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

type Rule struct {
	// The sequence number of the rule.
	Seq uint32
	// The action type of ACL Rule. Either 'permit' or 'deny'.
	Action Action
	// The protocol of the ACL Rule. Defaults to 'ip' if not specified.
	Protocol Protocol
	// The description of the ACL Rule.
	Description string
	// Source IP address or network. Use 'any' to match any source.
	Source netip.Prefix
	// Destination IP address or network. Use 'any' to match any target.
	Destination netip.Prefix
}

func (r Rule) Validate() error {
	if r.Source.Addr().Is4() != r.Destination.Addr().Is4() {
		return errors.New("acl: rule contains both ipv4 and ipv6 rules")
	}
	return nil
}

//go:generate go run golang.org/x/tools/cmd/stringer@v0.35.0 -type=Action
type Action uint8

const (
	Permit Action = iota + 1
	Deny
)

type Protocol uint8

const (
	IP   Protocol = 0
	ICMP Protocol = 1
	TCP  Protocol = 6
	UDP  Protocol = 17
	OSPF Protocol = 89
	PIM  Protocol = 103
)

func ProtocolFrom(s string) Protocol {
	switch strings.ToLower(s) {
	case "ip":
		return IP
	case "icmp":
		return ICMP
	case "tcp":
		return TCP
	case "udp":
		return UDP
	case "ospf":
		return OSPF
	case "pim":
		return PIM
	default:
		return 0 // unknown protocol - default to 0 == "ip"
	}
}

// ToYGOT returns a single update forcing the replacement of an ACL configuration.
func (a *ACL) ToYGOT(_ context.Context, _ gnmiext.Client) ([]gnmiext.Update, error) {
	if err := a.Validate(); err != nil {
		return nil, err
	}
	list := &nxos.Cisco_NX_OSDevice_System_AclItems_Ipv4Items_NameItems_ACLList{}
	list.PopulateDefaults()
	list.Name = ygot.String(a.Name)
	for _, r := range a.Rules {
		entry := list.GetOrCreateSeqItems().GetOrCreateACEList(r.Seq)
		entry.PopulateDefaults()
		var action nxos.E_Cisco_NX_OSDevice_Acl_ActionType
		switch r.Action {
		case Permit:
			action = nxos.Cisco_NX_OSDevice_Acl_ActionType_permit
		case Deny:
			action = nxos.Cisco_NX_OSDevice_Acl_ActionType_deny
		default:
			return nil, fmt.Errorf("acl: invalid action type: %s", r.Action)
		}
		entry.Action = action
		entry.Protocol = ygot.Uint8(uint8(r.Protocol))
		entry.SrcPrefix = ygot.String(r.Source.Addr().String())
		entry.SrcPrefixLength = ygot.Uint8(uint8(r.Source.Bits())) //nolint:gosec
		entry.DstPrefix = ygot.String(r.Destination.Addr().String())
		entry.DstPrefixLength = ygot.Uint8(uint8(r.Destination.Bits())) //nolint:gosec
		if r.Description != "" {
			entry.Remark = ygot.String(r.Description)
		}
	}
	return []gnmiext.Update{
		gnmiext.ReplacingUpdate{
			XPath: a.XPath(),
			Value: list,
		},
	}, nil
}

// Reset returns a single update deleting the YANG entry of the ACL.
func (a *ACL) Reset(_ context.Context, _ gnmiext.Client) ([]gnmiext.Update, error) {
	if err := a.Validate(); err != nil {
		return nil, err
	}
	return []gnmiext.Update{
		gnmiext.DeletingUpdate{
			XPath: a.XPath(),
		},
	}, nil
}
