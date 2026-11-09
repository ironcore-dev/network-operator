// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package srlinux

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/apistatus"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

var _ provider.DNSProvider = (*Provider)(nil)

func (p *Provider) EnsureDNS(ctx context.Context, req *provider.EnsureDNSRequest) error {
	spec := req.DNS.Spec
	if err := validateDNSSpec(spec); err != nil {
		return err
	}

	// Nokia SR Linux configures the DNS source as an IP address, while the API provides
	// a source interface name. Resolve it to the interface's first IPv4 address.
	var sourceAddress string
	if spec.SourceInterfaceName != "" {
		addr, err := p.interfaceIPAddr(ctx, spec.SourceInterfaceName)
		if err != nil {
			return err
		}
		sourceAddress = addr
	}

	// Group servers by VRF. Each unique VRF maps to one dns-instance on Nokia SRL.
	instances := make(map[string]*DNSInstance)
	for _, s := range spec.Servers {
		inst, ok := instances[s.VrfName]
		if !ok {
			inst = &DNSInstance{
				VrfName:         s.VrfName,
				NetworkInstance: s.VrfName,
				SourceAddress:   sourceAddress,
				SearchList:      []string{spec.Domain},
			}
			instances[s.VrfName] = inst
		}
		inst.ServerList = append(inst.ServerList, s.Address)
	}

	elements := make([]gnmiext.DataElement, 0, len(instances))
	for _, inst := range instances {
		elements = append(elements, inst)
	}
	return p.client.Update(ctx, elements...)
}

func (p *Provider) DeleteDNS(ctx context.Context) error {
	return p.client.Delete(ctx, &DNSInstanceList{})
}

func validateDNSSpec(spec v1alpha1.DNSSpec) error {
	var violations []apistatus.FieldViolation
	if spec.AdminState == v1alpha1.AdminStateDown {
		violations = append(violations, apistatus.FieldViolation{
			Field:       "spec.adminState",
			Description: "adminState Down is not supported by the Nokia SR Linux DNS model",
		})
	}
	for _, s := range spec.Servers {
		if s.VrfName == "" {
			violations = append(violations, apistatus.FieldViolation{
				Field:       fmt.Sprintf("spec.servers[%s].vrfName", s.Address),
				Description: "vrfName is required by the Nokia SR Linux DNS model — network-instance is mandatory",
			})
		}
	}
	if len(violations) > 0 {
		return apistatus.NewUnsupportedFieldError(violations...)
	}
	return nil
}

// Compile-time assertions.
var (
	_ gnmiext.DataElement = (*DNSInstance)(nil)
	_ gnmiext.DataElement = (*DNSInstanceList)(nil)
)

// DNSInstance represents a single Nokia SRL dns-instance list entry, keyed by VRF name.
type DNSInstance struct {
	VrfName         string   `json:"-"`
	NetworkInstance string   `json:"network-instance"`
	SourceAddress   string   `json:"source-address,omitempty"`
	SearchList      []string `json:"search-list,omitempty"`
	ServerList      []string `json:"server-list,omitempty"`
}

func (d *DNSInstance) XPath() string {
	return fmt.Sprintf("system/dns-instance[name=%s]", d.VrfName)
}

// DNSInstanceList targets the entire dns-instance list for deletion.
type DNSInstanceList struct{}

func (*DNSInstanceList) XPath() string {
	return "system/dns-instance"
}

// interfaceIPAddr retrieves the first IPv4 address from the state of the named
// interface's first subinterface, returning the bare IP without the prefix length.
func (p *Provider) interfaceIPAddr(ctx context.Context, name string) (string, error) {
	addrs := &InterfaceIPv4Addrs{IfName: name}
	if err := p.client.GetState(ctx, addrs); err != nil {
		if errors.Is(err, gnmiext.ErrNil) {
			return "", apistatus.NewFailedPreconditionError(fmt.Sprintf("interface %q has no IPv4 address configured", name))
		}
		return "", fmt.Errorf("failed to get IPv4 address for interface %q: %w", name, err)
	}
	for _, a := range addrs.Address {
		ip, _, _ := strings.Cut(a.IPPrefix, "/")
		return ip, nil
	}
	return "", apistatus.NewFailedPreconditionError(fmt.Sprintf("interface %q has no IPv4 address configured", name))
}

var _ gnmiext.DataElement = (*InterfaceIPv4Addrs)(nil)

// InterfaceIPv4Addrs targets the IPv4 address state of an interface's first subinterface.
type InterfaceIPv4Addrs struct {
	IfName  string          `json:"-"`
	Address []InterfaceAddr `json:"address,omitempty"`
}

func (a *InterfaceIPv4Addrs) XPath() string {
	return fmt.Sprintf("interface[name=%s]/subinterface[index=0]/ipv4/address", a.IfName)
}

// InterfaceAddr is a single IPv4 address entry; ip-prefix carries the address and length.
type InterfaceAddr struct {
	IPPrefix string `json:"ip-prefix"`
}
