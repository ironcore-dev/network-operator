// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package nxos

type RTAction string

const (
	RTNone   RTAction = "none"
	RTImport RTAction = "import"
	RTExport RTAction = "export"
	RTBoth   RTAction = "both"
)

type RouteTarget struct {
	// Addr is the VPN-IPv4 field of the route target (the VPN-IPv4 address)
	Addr VPNIPv4Address
	// AddressFamilyIPv4 indicates if the route target should be applied to IPv4 addresses
	AddressFamilyIPv4 bool
	// AddressFamilyIPv6 indicates if the route target should be applied to IPv6 addresses
	AddressFamilyIPv6 bool
	// Action is the Action to be taken for this route target
	Action RTAction
	// AddEVPN indicates if this route target should be added to the EVPN context
	AddEVPN bool
}

type RTOption func(*RouteTarget) error

// NewRouteTarget creates a new RouteTarget with the given address and options. By default
// it will not apply to any address family and will not add EVPN. Default action is RTNone.
// `addr“ must be constructed with `NewVPNIPv4Address`.
func NewRouteTarget(addr VPNIPv4Address, opts ...RTOption) (*RouteTarget, error) {
	rt := &RouteTarget{
		Addr:   addr,
		Action: RTNone,
	}
	for _, opt := range opts {
		if err := opt(rt); err != nil {
			return nil, err
		}
	}
	return rt, nil
}

func WithAction(action RTAction) RTOption {
	return func(rt *RouteTarget) error {
		rt.Action = action
		return nil
	}
}

// WithAddressFamilyIPv4Unicast sets enables this route target for ipv4 unicast addresses
func WithAddressFamilyIPv4Unicast(enabled bool) RTOption {
	return func(rt *RouteTarget) error {
		rt.AddressFamilyIPv4 = enabled
		return nil
	}
}

// WithAddressFamilyIPv6Unicast sets enables this route target for ipv6 unicast addresses
func WithAddressFamilyIPv6Unicast(enabled bool) RTOption {
	return func(rt *RouteTarget) error {
		rt.AddressFamilyIPv6 = enabled
		return nil
	}
}

// WithEVPN	sets whether this route target should be added to the EVPN context
func WithEVPN(enabled bool) RTOption {
	return func(rt *RouteTarget) error {
		rt.AddEVPN = enabled
		return nil
	}
}
