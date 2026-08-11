// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package openconfig

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/apistatus"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

var _ provider.BGPPeerProvider = (*Provider)(nil)

// bgpPeerGroupName is the default peer-group name used for all neighbors.
// SRLinux requires a peer-group on every neighbor via OpenConfig.
const bgpPeerGroupName = "NETOP-DEFAULT"

func (p *Provider) EnsureBGPPeer(ctx context.Context, req *provider.EnsureBGPPeerRequest) error {
	spec := req.BGPPeer.Spec

	peerAS, err := asnToUint32(spec.ASNumber)
	if err != nil {
		return err
	}

	ni := DefaultNetworkInstance
	if req.VRF != nil {
		ni = req.VRF.Spec.Name
	}

	pg := &BGPPeerGroup{
		NetworkInstance: ni,
		PeerGroupName:   bgpPeerGroupName,
		Config: &BGPPeerGroupConfig{
			PeerGroupName: bgpPeerGroupName,
		},
	}

	neighbor := &BGPNeighbor{
		NetworkInstance: ni,
		NeighborAddress: spec.Address,
		Config: &BGPNeighborConfig{
			NeighborAddress: spec.Address,
			PeerAS:          peerAS,
			PeerGroup:       bgpPeerGroupName,
			Enabled:         spec.AdminState != v1alpha1.AdminStateDown,
			Description:     spec.Description,
		},
	}

	if spec.LocalAS != nil {
		localAS, err := asnToUint32(spec.LocalAS.ASNumber)
		if err != nil {
			return err
		}
		neighbor.Config.LocalAS = localAS
		// Note: PrependLocalAS / PrependGlobalAS not supported via OC local-as leaf.
		if spec.LocalAS.PrependLocalAS != nil && !*spec.LocalAS.PrependLocalAS {
			return apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{
				Field:       "spec.localAS.prependLocalAS",
				Description: "openconfig provider does not support disabling local-AS prepend on SRLinux",
			})
		}
		if spec.LocalAS.PrependGlobalAS != nil && !*spec.LocalAS.PrependGlobalAS {
			return apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{
				Field:       "spec.localAS.prependGlobalAS",
				Description: "openconfig provider does not support disabling global-AS prepend on SRLinux",
			})
		}
	}

	if req.SourceInterface != "" {
		neighbor.Transport = &BGPNeighborTransport{
			Config: &BGPNeighborTransportConfig{
				LocalAddress: req.SourceInterface,
			},
		}
	}

	if spec.AddressFamilies != nil {
		neighbor.AfiSafis = &BGPNeighborAfiSafis{}
		if af := spec.AddressFamilies.Ipv4Unicast; af != nil {
			safi := &BGPNeighborAfiSafi{
				AfiSafiName: BGPAfiSafiTypeIPv4Unicast,
				Config: &BGPNeighborAfiSafiConfig{
					AfiSafiName: BGPAfiSafiTypeIPv4Unicast,
					Enabled:     af.Enabled,
				},
			}
			if policy, ok := req.InboundRoutingPolicies[v1alpha1.BGPAddressFamilyIpv4Unicast]; ok {
				safi.ApplyPolicy = &BGPNeighborApplyPolicy{
					Config: &BGPNeighborApplyPolicyConfig{
						ImportPolicy: []string{policy},
					},
				}
			}
			if policy, ok := req.OutboundRoutingPolicies[v1alpha1.BGPAddressFamilyIpv4Unicast]; ok {
				if safi.ApplyPolicy == nil {
					safi.ApplyPolicy = &BGPNeighborApplyPolicy{Config: &BGPNeighborApplyPolicyConfig{}}
				}
				safi.ApplyPolicy.Config.ExportPolicy = []string{policy}
			}
			neighbor.AfiSafis.AfiSafi.Set(safi)
		}
		if af := spec.AddressFamilies.Ipv6Unicast; af != nil {
			safi := &BGPNeighborAfiSafi{
				AfiSafiName: BGPAfiSafiTypeIPv6Unicast,
				Config: &BGPNeighborAfiSafiConfig{
					AfiSafiName: BGPAfiSafiTypeIPv6Unicast,
					Enabled:     af.Enabled,
				},
			}
			if policy, ok := req.InboundRoutingPolicies[v1alpha1.BGPAddressFamilyIpv6Unicast]; ok {
				safi.ApplyPolicy = &BGPNeighborApplyPolicy{
					Config: &BGPNeighborApplyPolicyConfig{
						ImportPolicy: []string{policy},
					},
				}
			}
			if policy, ok := req.OutboundRoutingPolicies[v1alpha1.BGPAddressFamilyIpv6Unicast]; ok {
				if safi.ApplyPolicy == nil {
					safi.ApplyPolicy = &BGPNeighborApplyPolicy{Config: &BGPNeighborApplyPolicyConfig{}}
				}
				safi.ApplyPolicy.Config.ExportPolicy = []string{policy}
			}
			neighbor.AfiSafis.AfiSafi.Set(safi)
		}
		if af := spec.AddressFamilies.L2vpnEvpn; af != nil && af.Enabled {
			return apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{
				Field:       "spec.addressFamilies.l2vpnEvpn",
				Description: "openconfig provider does not support L2VPN EVPN on SRLinux",
			})
		}
	}

	return p.client.Update(ctx, pg, neighbor)
}

func (p *Provider) DeleteBGPPeer(ctx context.Context, req *provider.DeleteBGPPeerRequest) error {
	ni := DefaultNetworkInstance
	if req.VRF != nil {
		ni = req.VRF.Spec.Name
	}
	neighbor := &BGPNeighbor{
		NetworkInstance: ni,
		NeighborAddress: req.BGPPeer.Spec.Address,
	}
	return p.client.Delete(ctx, neighbor)
}

func (p *Provider) GetPeerStatus(ctx context.Context, req *provider.BGPPeerStatusRequest) (provider.BGPPeerStatus, error) {
	ni := DefaultNetworkInstance
	if req.VRF != nil {
		ni = req.VRF.Spec.Name
	}

	state := &BGPNeighborState{
		NetworkInstance: ni,
		NeighborAddress: req.BGPPeer.Spec.Address,
	}
	if err := p.client.GetState(ctx, state); err != nil {
		return provider.BGPPeerStatus{}, err
	}

	return provider.BGPPeerStatus{
		SessionState:        toBGPSessionState(state.SessionState),
		LastEstablishedTime: state.LastEstablished,
	}, nil
}

func toBGPSessionState(s string) v1alpha1.BGPPeerSessionState {
	switch BGPSessionState(strings.ToUpper(s)) {
	case BGPSessionStateIdle:
		return v1alpha1.BGPPeerSessionStateIdle
	case BGPSessionStateConnect:
		return v1alpha1.BGPPeerSessionStateConnect
	case BGPSessionStateActive:
		return v1alpha1.BGPPeerSessionStateActive
	case BGPSessionStateOpenSent:
		return v1alpha1.BGPPeerSessionStateOpenSent
	case BGPSessionStateOpenConfirm:
		return v1alpha1.BGPPeerSessionStateOpenConfirm
	case BGPSessionStateEstablished:
		return v1alpha1.BGPPeerSessionStateEstablished
	default:
		return v1alpha1.BGPPeerSessionStateUnknown
	}
}

// BGPSessionState represents the OpenConfig BGP session state.
type BGPSessionState string

const (
	BGPSessionStateIdle        BGPSessionState = "IDLE"
	BGPSessionStateConnect     BGPSessionState = "CONNECT"
	BGPSessionStateActive      BGPSessionState = "ACTIVE"
	BGPSessionStateOpenSent    BGPSessionState = "OPENSENT"
	BGPSessionStateOpenConfirm BGPSessionState = "OPENCONFIRM"
	BGPSessionStateEstablished BGPSessionState = "ESTABLISHED"
)

// Compile-time assertions.
var (
	_ gnmiext.DataElement = (*BGPPeerGroup)(nil)
	_ gnmiext.DataElement = (*BGPNeighbor)(nil)
	_ gnmiext.DataElement = (*BGPNeighborState)(nil)
)

// BGPPeerGroup targets a peer-group entry.
type BGPPeerGroup struct {
	NetworkInstance string              `json:"-"`
	PeerGroupName   string              `json:"-"`
	Config          *BGPPeerGroupConfig `json:"config,omitempty"`
}

func (pg *BGPPeerGroup) XPath() string {
	return fmt.Sprintf("openconfig-network-instance:network-instances/network-instance[name=%s]/protocols/protocol[identifier=openconfig-policy-types:BGP][name=BGP]/bgp/peer-groups/peer-group[peer-group-name=%s]", pg.NetworkInstance, pg.PeerGroupName)
}

// BGPPeerGroupConfig holds peer-group config.
type BGPPeerGroupConfig struct {
	PeerGroupName string `json:"peer-group-name"`
}

// BGPNeighbor targets a neighbor entry.
type BGPNeighbor struct {
	NetworkInstance string                `json:"-"`
	NeighborAddress string                `json:"-"`
	Config          *BGPNeighborConfig    `json:"config,omitempty"`
	Transport       *BGPNeighborTransport `json:"transport,omitempty"`
	AfiSafis        *BGPNeighborAfiSafis  `json:"afi-safis,omitempty"`
}

func (n *BGPNeighbor) XPath() string {
	return fmt.Sprintf("openconfig-network-instance:network-instances/network-instance[name=%s]/protocols/protocol[identifier=openconfig-policy-types:BGP][name=BGP]/bgp/neighbors/neighbor[neighbor-address=%s]", n.NetworkInstance, n.NeighborAddress)
}

// BGPNeighborConfig holds neighbor config.
type BGPNeighborConfig struct {
	NeighborAddress string `json:"neighbor-address"`
	PeerAS          uint32 `json:"peer-as"`
	PeerGroup       string `json:"peer-group"`
	Enabled         bool   `json:"enabled"`
	Description     string `json:"description,omitempty"`
	LocalAS         uint32 `json:"local-as,omitempty"`
}

// BGPNeighborTransport holds transport config.
type BGPNeighborTransport struct {
	Config *BGPNeighborTransportConfig `json:"config,omitempty"`
}

// BGPNeighborTransportConfig holds transport/config.
type BGPNeighborTransportConfig struct {
	LocalAddress string `json:"local-address,omitempty"`
}

// BGPNeighborAfiSafis holds the per-neighbor afi-safis.
type BGPNeighborAfiSafis struct {
	AfiSafi gnmiext.List[string, *BGPNeighborAfiSafi] `json:"afi-safi,omitempty"`
}

// BGPNeighborAfiSafi represents a per-neighbor afi-safi.
type BGPNeighborAfiSafi struct {
	AfiSafiName BGPAfiSafiType            `json:"afi-safi-name"`
	Config      *BGPNeighborAfiSafiConfig `json:"config,omitempty"`
	ApplyPolicy *BGPNeighborApplyPolicy   `json:"apply-policy,omitempty"`
}

func (a *BGPNeighborAfiSafi) Key() string { return string(a.AfiSafiName) }

// BGPNeighborAfiSafiConfig holds per-neighbor afi-safi config.
type BGPNeighborAfiSafiConfig struct {
	AfiSafiName BGPAfiSafiType `json:"afi-safi-name"`
	Enabled     bool           `json:"enabled"`
}

// BGPNeighborApplyPolicy holds apply-policy for a neighbor afi-safi.
type BGPNeighborApplyPolicy struct {
	Config *BGPNeighborApplyPolicyConfig `json:"config,omitempty"`
}

// BGPNeighborApplyPolicyConfig holds apply-policy config.
type BGPNeighborApplyPolicyConfig struct {
	ImportPolicy []string `json:"import-policy,omitempty"`
	ExportPolicy []string `json:"export-policy,omitempty"`
}

// BGPNeighborState reads neighbor state for session-state.
type BGPNeighborState struct {
	NetworkInstance string    `json:"-"`
	NeighborAddress string    `json:"-"`
	SessionState    string    `json:"session-state,omitempty"`
	LastEstablished time.Time `json:"last-established,omitzero"`
}

func (s *BGPNeighborState) XPath() string {
	return fmt.Sprintf("openconfig-network-instance:network-instances/network-instance[name=%s]/protocols/protocol[identifier=openconfig-policy-types:BGP][name=BGP]/bgp/neighbors/neighbor[neighbor-address=%s]/state", s.NetworkInstance, s.NeighborAddress)
}
