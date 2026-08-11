// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package openconfig

import (
	"context"
	"fmt"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/apistatus"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

var _ provider.ACLProvider = (*Provider)(nil)

func (p *Provider) EnsureACL(ctx context.Context, req *provider.ACLRequest) error {
	spec := req.ACL.Spec

	aclType := ACLTypeIPv4
	if len(spec.Entries) > 0 && spec.Entries[0].SourceAddress.Is6() {
		aclType = ACLTypeIPv6
	}

	acl := &ACLSet{
		Name:    spec.Name,
		ACLType: aclType,
		Config: &ACLSetConfig{
			Name: spec.Name,
			Type: aclType,
		},
		Entries: &ACLEntries{},
	}

	for i, entry := range spec.Entries {
		if entry.SourceAddress.Is6() != entry.DestinationAddress.Is6() {
			return apistatus.NewInvalidArgumentError(apistatus.FieldViolation{
				Field:       fmt.Sprintf("spec.entries[%d]", i),
				Description: "source and destination addresses must use the same IP version",
			})
		}
		if (aclType == ACLTypeIPv6) != entry.SourceAddress.Is6() {
			return apistatus.NewInvalidArgumentError(apistatus.FieldViolation{
				Field:       fmt.Sprintf("spec.entries[%d].sourceAddress", i),
				Description: "all ACL entries must use the same IP version",
			})
		}

		e := &ACLEntry{
			SequenceID: uint32(entry.Sequence), //nolint:gosec
			Config: &ACLEntryConfig{
				SequenceID:  uint32(entry.Sequence), //nolint:gosec
				Description: entry.Description,
			},
			Actions: &ACLEntryActions{
				Config: &ACLEntryActionsConfig{
					ForwardingAction: toACLForwardingAction(entry.Action),
				},
			},
		}

		if aclType == ACLTypeIPv4 {
			e.IPv4 = &ACLIPv4{
				Config: &ACLIPv4Config{
					SourceAddress:      entry.SourceAddress.String(),
					DestinationAddress: entry.DestinationAddress.String(),
					Protocol:           toACLProtocol(entry.Protocol),
				},
			}
		} else {
			e.IPv6 = &ACLIPv6{
				Config: &ACLIPv6Config{
					SourceAddress:      entry.SourceAddress.String(),
					DestinationAddress: entry.DestinationAddress.String(),
					Protocol:           toACLProtocol(entry.Protocol),
				},
			}
		}

		acl.Entries.Entry.Set(e)
	}

	return p.client.Update(ctx, acl)
}

func (p *Provider) DeleteACL(ctx context.Context, req *provider.ACLRequest) error {
	aclType := ACLTypeIPv4
	if len(req.ACL.Spec.Entries) > 0 && req.ACL.Spec.Entries[0].SourceAddress.Is6() {
		aclType = ACLTypeIPv6
	}
	return p.client.Delete(ctx, &ACLSet{Name: req.ACL.Spec.Name, ACLType: aclType})
}

func toACLForwardingAction(a v1alpha1.ACLAction) ACLForwardingAction {
	switch a {
	case v1alpha1.ActionPermit:
		return ACLForwardingActionAccept
	case v1alpha1.ActionDeny:
		return ACLForwardingActionDrop
	default:
		return ACLForwardingActionDrop
	}
}

func toACLProtocol(proto v1alpha1.Protocol) ACLProtocol {
	switch proto {
	case v1alpha1.ProtocolICMP:
		return ACLProtocolICMP
	case v1alpha1.ProtocolTCP:
		return ACLProtocolTCP
	case v1alpha1.ProtocolUDP:
		return ACLProtocolUDP
	case v1alpha1.ProtocolOSPF:
		return ACLProtocolOSPF
	case v1alpha1.ProtocolPIM:
		return ACLProtocolPIM
	default:
		return "" // IP (any)
	}
}

// ACLType represents the OpenConfig ACL set type identity.
type ACLType string

const (
	ACLTypeIPv4 ACLType = "openconfig-acl:ACL_IPV4"
	ACLTypeIPv6 ACLType = "openconfig-acl:ACL_IPV6"
)

// ACLForwardingAction represents the OpenConfig ACL forwarding action.
type ACLForwardingAction string

const (
	ACLForwardingActionAccept ACLForwardingAction = "openconfig-acl:ACCEPT"
	ACLForwardingActionDrop   ACLForwardingAction = "openconfig-acl:DROP"
)

// ACLProtocol represents an IP protocol number used in ACL entries.
type ACLProtocol string

const (
	ACLProtocolICMP ACLProtocol = "1"
	ACLProtocolTCP  ACLProtocol = "6"
	ACLProtocolUDP  ACLProtocol = "17"
	ACLProtocolOSPF ACLProtocol = "89"
	ACLProtocolPIM  ACLProtocol = "103"
)

// Compile-time assertion.
var _ gnmiext.DataElement = (*ACLSet)(nil)

// ACLSet represents an OC acl-set list entry.
type ACLSet struct {
	Name    string        `json:"-"`
	ACLType ACLType       `json:"-"`
	Config  *ACLSetConfig `json:"config,omitempty"`
	Entries *ACLEntries   `json:"acl-entries,omitempty"`
}

func (a *ACLSet) XPath() string {
	return fmt.Sprintf("openconfig-acl:acl/acl-sets/acl-set[name=%s][type=%s]", a.Name, a.ACLType)
}

// ACLSetConfig holds the acl-set config.
type ACLSetConfig struct {
	Name string  `json:"name"`
	Type ACLType `json:"type"`
}

// ACLEntries holds the acl-entry list.
type ACLEntries struct {
	Entry gnmiext.List[uint32, *ACLEntry] `json:"acl-entry,omitempty"`
}

// ACLEntry represents a single ACL entry.
type ACLEntry struct {
	SequenceID uint32           `json:"sequence-id"`
	Config     *ACLEntryConfig  `json:"config,omitempty"`
	IPv4       *ACLIPv4         `json:"ipv4,omitempty"`
	IPv6       *ACLIPv6         `json:"ipv6,omitempty"`
	Actions    *ACLEntryActions `json:"actions,omitempty"`
}

func (e *ACLEntry) Key() uint32 { return e.SequenceID }

// ACLEntryConfig holds the entry config.
type ACLEntryConfig struct {
	SequenceID  uint32 `json:"sequence-id"`
	Description string `json:"description,omitempty"`
}

// ACLIPv4 holds the IPv4 match criteria.
type ACLIPv4 struct {
	Config *ACLIPv4Config `json:"config,omitempty"`
}

// ACLIPv4Config holds IPv4 match config.
type ACLIPv4Config struct {
	SourceAddress      string      `json:"source-address,omitempty"`
	DestinationAddress string      `json:"destination-address,omitempty"`
	Protocol           ACLProtocol `json:"protocol,omitempty"`
}

// ACLIPv6 holds the IPv6 match criteria.
type ACLIPv6 struct {
	Config *ACLIPv6Config `json:"config,omitempty"`
}

// ACLIPv6Config holds IPv6 match config.
type ACLIPv6Config struct {
	SourceAddress      string      `json:"source-address,omitempty"`
	DestinationAddress string      `json:"destination-address,omitempty"`
	Protocol           ACLProtocol `json:"protocol,omitempty"`
}

// ACLEntryActions holds the actions for an ACL entry.
type ACLEntryActions struct {
	Config *ACLEntryActionsConfig `json:"config,omitempty"`
}

// ACLEntryActionsConfig holds the actions config.
type ACLEntryActionsConfig struct {
	ForwardingAction ACLForwardingAction `json:"forwarding-action"`
}
