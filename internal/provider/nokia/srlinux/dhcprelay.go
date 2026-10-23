// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package srlinux

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/ironcore-dev/network-operator/internal/provider"
)

const (
	// dhcpRelayDefaultSubinterfaceIndex is the subinterface a relay attaches to
	// when the interface name carries no ".<index>" suffix.
	dhcpRelayDefaultSubinterfaceIndex = 0
)

var _ provider.DHCPRelayProvider = (*Provider)(nil)

// DeleteDHCPRelay implements [provider.DHCPRelayProvider].
func (p *Provider) DeleteDHCPRelay(ctx context.Context, req *provider.DeleteDHCPRelayRequest) error {
	// The deprecated InterfaceRefs path provides no resolved interface in the
	// delete request. SRLinux models the relay per subinterface with no single
	// root to remove, so there is nothing to delete here.
	if len(req.DHCPRelay.Spec.InterfaceRefs) > 0 { //nolint:staticcheck
		return nil
	}
	name, index, err := splitSubinterface(req.Interface.Spec.Name)
	if err != nil {
		return err
	}
	return p.client.Delete(ctx, &DHCPRelay{Interface: name, SubinterfaceIndex: index})
}

// EnsureDHCPRelay implements [provider.DHCPRelayProvider].
func (p *Provider) EnsureDHCPRelay(ctx context.Context, req *provider.DHCPRelayRequest) error {
	var networkInstance string
	if req.VRF != nil {
		networkInstance = req.VRF.Spec.Name
	}
	if len(req.DHCPRelay.Spec.InterfaceRefs) > 0 { //nolint:staticcheck
		return nil
	}
	name, index, err := splitSubinterface(req.Interface.Spec.Name)
	if err != nil {
		return err
	}
	dhcprelay := &DHCPRelay{
		Interface:         name,
		SubinterfaceIndex: index,
		AdminState:        AdminStEnable,
		Server:            req.DHCPRelay.Spec.Servers,
		NetworkInstance:   networkInstance,
	}
	return p.client.Update(ctx, dhcprelay)
}

// splitSubinterface splits an interface name into its base name and
// subinterface index. SRLinux names a subinterface "<interface>.<index>";
// without a suffix the relay attaches to the base subinterface.
func splitSubinterface(name string) (string, int, error) {
	base, suffix, found := strings.CutLast(name, ".")
	if !found {
		return name, dhcpRelayDefaultSubinterfaceIndex, nil
	}
	index, err := strconv.Atoi(suffix)
	if err != nil {
		return "", 0, fmt.Errorf("invalid subinterface index in interface name %q: %w", name, err)
	}
	return base, index, nil
}

type DHCPRelay struct {
	Interface         string   `json:"-"`
	SubinterfaceIndex int      `json:"-"`
	AdminState        AdminSt  `json:"admin-state"`
	Server            []string `json:"server"`
	NetworkInstance   string   `json:"network-instance,omitempty"`
}

func (d *DHCPRelay) XPath() string {
	return fmt.Sprintf("srl_nokia-interfaces:interface[name=%s]/subinterface[index=%d]/ipv4/srl_nokia-interfaces-ip-dhcp-relay:dhcp-relay", d.Interface, d.SubinterfaceIndex)
}
