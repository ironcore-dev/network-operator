// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import "github.com/ironcore-dev/network-operator/internal/transport/gnmiext"

var (
	_ gnmiext.DataElement = (*HMMFwdEvpn)(nil)
	_ gnmiext.Defaultable = (*HMMFwdEvpn)(nil)
)

const (
	hmmFwdEvpnDefaultMoves           uint16 = 5
	hmmFwdEvpnDefaultDuration        uint16 = 180
	hmmFwdEvpnDefaultRecoveryTimeout uint16 = 30
	hmmFwdEvpnDefaultRecoveryCount   uint16 = 5
)

// HMMFwdEvpn represents the L2RIB duplicate host IP address detection settings
// ("l2rib dup-host-ip-addr-detection").
// Writes System/hmm-items/fwdinst-items/evpn-items. Must use only PATCH operations
// so that FabricFwd's data at the parent path is not clobbered.
type HMMFwdEvpn struct {
	AdminSt    AdminSt `json:"adminSt"`
	DdHmmoves  uint16  `json:"ddHmmoves"`
	DdHnsecs   uint16  `json:"ddHnsecs"`
	Dhrtimeout uint16  `json:"dhrtimeout"`
	Dhrcount   uint16  `json:"dhrcount"`
}

func (*HMMFwdEvpn) XPath() string {
	return "System/hmm-items/fwdinst-items/evpn-items"
}

func (h *HMMFwdEvpn) Default() {
	h.AdminSt = AdminStEnabled
	h.DdHmmoves = hmmFwdEvpnDefaultMoves
	h.DdHnsecs = hmmFwdEvpnDefaultDuration
	h.Dhrtimeout = hmmFwdEvpnDefaultRecoveryTimeout
	h.Dhrcount = hmmFwdEvpnDefaultRecoveryCount
}
