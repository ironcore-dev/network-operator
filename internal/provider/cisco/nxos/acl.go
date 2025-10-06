// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"fmt"

	"github.com/ironcore-dev/network-operator/api/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"
)

var _ gnmiext.Configurable = (*ACL)(nil)

// ACL represents an IPv4 or IPv6 access control list, depending on the rules it contains.
// It can only contain either IPv4 or IPv6 rules, never both. It's name must be unique
// across IPv4 and IPv6 access lists.
type ACL struct {
	// Name is the name of the ACL. This name must be unique across IPv4 and IPv6 access lists.
	Name string `json:"name"`
	// SeqItems contains the list of ACE entries in the ACL.
	SeqItems struct {
		ACEList []*ACLEntry `json:"ACE-list,omitzero"`
	} `json:"seq-items,omitzero"`
	// Is6 indicates whether this is an IPv6 ACL. This field is not serialized to JSON
	// and is only used internally to determine the correct XPath for the ACL.
	Is6 bool `json:"-"`
}

func (a *ACL) IsListItem() {}

func (a *ACL) XPath() string {
	if a.Is6 {
		return "System/acl-items/ipv6-items/name-items/ACL-list[name=" + a.Name + "]"
	}
	return "System/acl-items/ipv4-items/name-items/ACL-list[name=" + a.Name + "]"
}

type ACLEntry struct {
	SeqNum          int32    `json:"seqNum"`
	Action          Action   `json:"action"`
	Protocol        Protocol `json:"protocol"`
	Remark          string   `json:"remark,omitempty"`
	SrcPrefix       string   `json:"srcPrefix"`
	SrcPrefixLength int      `json:"srcPrefixLength,omitempty"`
	DstPrefix       string   `json:"dstPrefix"`
	DstPrefixLength int      `json:"dstPrefixLength,omitempty"`
}

type Action string

const (
	ActionPermit Action = "permit"
	ActionDeny   Action = "deny"
)

func ActionFrom(act v1alpha1.ACLAction) (Action, error) {
	switch act {
	case v1alpha1.ActionPermit:
		return ActionPermit, nil
	case v1alpha1.ActionDeny:
		return ActionDeny, nil
	default:
		var zero Action
		return zero, fmt.Errorf("acl: unsupported action %q", act)
	}
}

type Protocol uint8

const (
	ProtocolIP   Protocol = 0
	ProtocolICMP Protocol = 1
	ProtocolTCP  Protocol = 6
	ProtocolUDP  Protocol = 17
	ProtocolOSPF Protocol = 89
	ProtocolPIM  Protocol = 103
)

func ProtocolFrom(proto v1alpha1.Protocol) Protocol {
	switch proto {
	case v1alpha1.ProtocolIP:
		return ProtocolIP
	case v1alpha1.ProtocolICMP:
		return ProtocolICMP
	case v1alpha1.ProtocolTCP:
		return ProtocolTCP
	case v1alpha1.ProtocolUDP:
		return ProtocolUDP
	case v1alpha1.ProtocolOSPF:
		return ProtocolOSPF
	case v1alpha1.ProtocolPIM:
		return ProtocolPIM
	default:
		return 0 // unknown protocol - default to 0 == "ip"
	}
}
