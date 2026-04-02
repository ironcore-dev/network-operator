// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"net/netip"

	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

var _ gnmiext.Configurable = (*DHCPRelayConfig)(nil)

// DHCPRelayConfig represents the complete DHCP relay configuration tree.
type DHCPRelayConfig struct {
	RelayIfList gnmiext.List[string, *DHCPRelay] `json:"RelayIf-list,omitzero"`
}

func (*DHCPRelayConfig) XPath() string {
	return "System/dhcp-items/inst-items/relayif-items"
}

// DHCPRelay represents the DHCP Relay configuration for a single interface.
type DHCPRelay struct {
	ID        string `json:"id"`
	AddrItems struct {
		AddrList gnmiext.List[netip.Addr, *DHCPRelayServer] `json:"RelayAddr-list,omitzero"`
	} `json:"addr-items"`
}

func (d *DHCPRelay) Key() string {
	return d.ID
}

func (*DHCPRelay) IsListItem() {}

type DHCPRelayServer struct {
	Address netip.Addr `json:"address"`
	Vrf     string     `json:"vrf,omitempty"`
}

func (d *DHCPRelayServer) Key() netip.Addr {
	return d.Address
}
