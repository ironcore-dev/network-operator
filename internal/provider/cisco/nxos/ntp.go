// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import "github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"

var (
	_ gnmiext.Configurable = (*NTP)(nil)
	_ gnmiext.Defaultable  = (*NTP)(nil)
)

// NTP represents the NTP configuration on a NX-OS device.
type NTP struct {
	AdminSt   AdminSt `json:"adminSt"`
	Logging   AdminSt `json:"logging"`
	ProvItems struct {
		NtpProviderList []*NTPProvider `json:"NtpProvider-list,omitzero"`
	} `json:"prov-items,omitzero"`
	SrcIfItems struct {
		SrcIf string `json:"srcIf,omitempty"`
	} `json:"srcIf-items,omitzero"`
}

func (*NTP) XPath() string {
	return "System/time-items"
}

func (n *NTP) Default() {
	n.AdminSt = AdminStDisabled
}

type NTPProvider struct {
	KeyID     int      `json:"keyId"`
	MaxPoll   int      `json:"maxPoll"`
	MinPoll   int      `json:"minPoll"`
	Name      string   `json:"name"`
	Preferred bool     `json:"preferred"`
	ProvT     ProvType `json:"provT"`
	Vrf       string   `json:"vrf,omitempty"`
}

type ProvType string

const (
	ProvTypePeer   ProvType = "peer"
	ProvTypeServer ProvType = "server"
)
