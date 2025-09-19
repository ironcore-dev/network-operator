// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
//
// Package vpc provides functionality to configure Virtual Port Channels (vPC) on Cisco NX-OS devices.
//
// See https://www.cisco.com/c/en/us/td/docs/dcn/nx-os/nexus9000/104x/configuration/interfaces/cisco-nexus-9000-series-nx-os-interfaces-configuration-guide-release-104x/m_configuring_vpcs_9x.html
package vpc

import (
	"context"
	"errors"
	"fmt"
	"net/netip"

	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/iface"
)

// VPC represents a Virtual Port Channel (vPC). New instances must be created using the [NewVPC] function .
type VPC struct {
	domainID            uint16
	peerLinkPortChannel string
	keepaliveDstIP      *netip.Addr
	keepaliveSrcIP      *netip.Addr
	keepaliveVRF        string
	members             map[string]memberInfo
	peerSwitch          bool
	peerGateway         bool
}

type memberInfo struct {
	VPCID uint16
}

type Option func(*VPC) error

// NewVPC creates a new VPC instance with the given domain ID and options. The domain ID must be between 1 and 1000.
func NewVPC(domainID int, opts ...Option) (*VPC, error) {
	if domainID < 1 || domainID > 1000 {
		return nil, fmt.Errorf("vpc: domain ID must be between 1 and 1000")
	}
	v := &VPC{
		domainID: uint16(domainID),
		members:  make(map[string]memberInfo),
	}
	for _, opt := range opts {
		if err := opt(v); err != nil {
			return nil, err
		}
	}
	// Check that peerLinkPortChannel is not included in members
	if v.peerLinkPortChannel != "" {
		if _, exists := v.members[v.peerLinkPortChannel]; exists {
			return nil, errors.New("vpc: peer-link in port-channel is included in the vPC members list")
		}
	}
	return v, nil
}

type PeerLinkConfig struct {
	// PortChannel to be used as the vPC peer-link. It must be configured as a trunk port-channel interface.
	PortChannel string
	// KeepAliveDstIP must be a valid IPv4 or IPv6 address to use as destination for the peer-keepalive link.
	KeepAliveDstIP string
	// KeepAliveSrcIP must be a valid IP address for the peer-keepalive link source. Must use the same version as the destination IP.
	// If nil, the device will use its default.
	KeepAliveSrcIP *string
	// KeepAliveVRF is the VRF to be used for the peer-keepalive link. If nil, the device will use the default VRF.
	KeepAliveVRF *string
}

// WithPeerLink configures the vPC peer-link and the peer-keepalive configuration.
func WithPeerLink(cfg PeerLinkConfig) Option {
	return func(v *VPC) error {
		errs := []error{}
		shortName, err := iface.ShortNamePortChannel(cfg.PortChannel)
		if err != nil {
			errs = append(errs, fmt.Errorf("vpc: cannot add peer-link to port-channel: %w", err))
		} else {
			v.peerLinkPortChannel = shortName
		}

		dst, err := netip.ParseAddr(cfg.KeepAliveDstIP)
		switch {
		case err != nil:
			errs = append(errs, fmt.Errorf("vpc: cannot configure peer-link destination IP address: %w", err))
		case !dst.IsValid():
			errs = append(errs, errors.New("vpc: cannot configure peer-link destination IP address: not a valid IPv4 or IPv6 address"))
		default:
			v.keepaliveDstIP = &dst
		}

		if cfg.KeepAliveSrcIP != nil && v.keepaliveDstIP != nil && v.keepaliveDstIP.IsValid() {
			src, err := netip.ParseAddr(*cfg.KeepAliveSrcIP)
			switch {
			case err != nil:
				errs = append(errs, fmt.Errorf("vpc: cannot configure peer-link source IP address: %w", err))
			case !src.IsValid():
				errs = append(errs, errors.New("vpc: cannot configure peer-link source IP address: not a valid IPv4 or IPv6 address"))
			default:
				if src.Is4() != v.keepaliveDstIP.Is4() || src.Is6() != v.keepaliveDstIP.Is6() {
					errs = append(errs, errors.New("vpc: peer-link source IP address must be the same IP version as the destination address"))
					break
				}
				v.keepaliveSrcIP = &src
			}
		}

		if cfg.KeepAliveVRF != nil {
			if *cfg.KeepAliveVRF == "" {
				errs = append(errs, errors.New("vpc: peer-link VRF cannot be empty"))
			} else {
				v.keepaliveVRF = *cfg.KeepAliveVRF
			}
		}
		return errors.Join(errs...)
	}
}

type Member struct {
	PortChannel string
	VPCID       uint16
}

// WithMembers configures the members of this vPC. The list must contain at least one member. Each port-channel
// in the provided list must have a valid name and vPC ID (between 1 and 4096). Duplicate vPC IDs are not allowed.
func WithMembers(members []Member) Option {
	return func(v *VPC) error {
		if len(members) == 0 {
			return errors.New("vpc: members list cannot be empty")
		}

		v.members = make(map[string]memberInfo, len(members))
		seenVPCIDs := make(map[uint16]struct{}, len(members))

		errs := []error{}
		for _, pc := range members {
			shortName, err := iface.ShortNamePortChannel(pc.PortChannel)
			if err != nil {
				errs = append(errs, fmt.Errorf("vpc: invalid port-channel name: %w", err))
				continue
			}

			if _, exists := v.members[shortName]; exists {
				errs = append(errs, fmt.Errorf("vpc: port-channel %q is already a member", shortName))
				continue
			}

			if pc.VPCID < 1 || pc.VPCID > 4096 {
				errs = append(errs, errors.New("vpc: member vPC ID must be between 1 and 4096"))
				continue
			}

			if _, exists := seenVPCIDs[pc.VPCID]; exists {
				errs = append(errs, fmt.Errorf("vpc: member vPC ID %d is used more than once", pc.VPCID))
				continue
			}
			seenVPCIDs[pc.VPCID] = struct{}{}

			v.members[shortName] = memberInfo{VPCID: pc.VPCID}
		}
		if len(errs) > 0 {
			v.members = nil
		}
		return errors.Join(errs...)
	}
}

// EnablePeerSwitchFeature enables the peer-switch feature on the vPC. See [Cisco vPC] for details.
func EnablePeerSwitchFeature() Option {
	return func(v *VPC) error {
		v.peerSwitch = true
		return nil
	}
}

// EnablePeerGatewayFeature enables the peer-gateway feature on the vPC. See [Cisco vPC] for details.
func EnablePeerGatewayFeature() Option {
	return func(v *VPC) error {
		v.peerGateway = true
		return nil
	}
}

var _ gnmiext.DeviceConf = (*VPC)(nil)

// ToYGOT enables the vPC feature and configures the vPC domain, peer-link, and member port-channels.
// It gets config from remote file to validate that the peer-link port-channel is set and configured as an L2 trunk.
// If validation succeeds it will also enables the vPC feature
func (v *VPC) ToYGOT(ctx context.Context, c gnmiext.Client) ([]gnmiext.Update, error) {
	val := &nxos.Cisco_NX_OSDevice_System_VpcItems_InstItems_DomItems{
		Id:      ygot.Uint16(v.domainID),
		AdminSt: nxos.Cisco_NX_OSDevice_Nw_AdminSt_enabled,
	}

	if v.peerLinkPortChannel != "" {
		// Check if the port-channel exists and is configured as an L2 trunk
		pc := &nxos.Cisco_NX_OSDevice_System_IntfItems_AggrItems_AggrIfList{}
		if err := c.Get(ctx, "System/intf-items/aggr-items/AggrIf-list[id="+v.peerLinkPortChannel+"]", pc); err != nil {
			return nil, fmt.Errorf("vpc: cannot get port-channel %q from remote switch: %w", v.peerLinkPortChannel, err)
		}
		if pc.Layer != nxos.Cisco_NX_OSDevice_L1_Layer_AggrIfLayer_Layer2 || pc.Mode != nxos.Cisco_NX_OSDevice_L1_Mode_trunk {
			return nil, errors.New("vpc: peer-link port-channel must be configured as an L2 trunk")
		}
		val.GetOrCreateKeepaliveItems().GetOrCreatePeerlinkItems().Id = ygot.String(v.peerLinkPortChannel)

		// Keepalive link configuration
		if v.keepaliveDstIP != nil {
			val.GetOrCreateKeepaliveItems().DestIp = ygot.String(v.keepaliveDstIP.String())
			if v.keepaliveSrcIP != nil {
				val.GetOrCreateKeepaliveItems().SrcIp = ygot.String(v.keepaliveSrcIP.String())
			}
			if v.keepaliveVRF != "" {
				val.GetOrCreateKeepaliveItems().Vrf = ygot.String(v.keepaliveVRF)
			}
		}
	}

	for member, info := range v.members {
		isConfigured, err := iface.Exists(ctx, c, member)
		if err != nil {
			return nil, fmt.Errorf("vpc: cannot get port-channel %q from remote switch: %w", member, err)
		}
		if !isConfigured {
			return nil, fmt.Errorf("vpc: member port-channel %q does not exist on the device", member)
		}
		val.GetOrCreateIfItems().GetOrCreateIfList(info.VPCID).GetOrCreateRsvpcConfItems().TDn = ygot.String("/System/intf-items/aggr-items/AggrIf-list[id='" + member + "']")
	}

	if v.peerSwitch {
		val.PeerSwitch = nxos.Cisco_NX_OSDevice_Nw_AdminSt_enabled
	}

	if v.peerGateway {
		val.PeerGw = nxos.Cisco_NX_OSDevice_Nw_AdminSt_enabled
	}

	return []gnmiext.Update{
		gnmiext.EditingUpdate{
			XPath: "System/fm-items/vpc-items",
			Value: &nxos.Cisco_NX_OSDevice_System_FmItems_VpcItems{
				AdminSt: nxos.Cisco_NX_OSDevice_Fm_AdminState_enabled,
			},
		},
		gnmiext.ReplacingUpdate{
			XPath: "System/vpc-items/inst-items/dom-items",
			Value: val,
		},
	}, nil
}

func (v *VPC) Reset(_ context.Context, _ gnmiext.Client) ([]gnmiext.Update, error) {
	return []gnmiext.Update{
		gnmiext.DeletingUpdate{
			XPath: "System/vpc-items/inst-items/dom-items",
		},
	}, nil
}
