// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"encoding/json"

	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

var (
	_ gnmiext.DataElement = (*EthernetSegmentItems)(nil)
	_ gnmiext.DataElement = (*MultihomingItems)(nil)
	_ gnmiext.DataElement = (*EvpnMulticastItems)(nil)
)

// EthernetSegmentItems represents the per-interface Ethernet Segment configuration
// nested under a port-channel aggregate interface.
type EthernetSegmentItems struct {
	ID                     string              `json:"-"`
	Type                   EthernetSegmentType `json:"esType"`
	ESI                    Option[string]      `json:"esi"`
	SysMac                 Option[string]      `json:"sysMac"`
	SysMacInherit          bool                `json:"sysMacInherit"`
	LocalIdentifier        uint32              `json:"localIdentifier"`
	LocalIdentifierInherit bool                `json:"localIdentifierInherit"`
}

func (e *EthernetSegmentItems) XPath() string {
	return "System/intf-items/aggr-items/AggrIf-list[id=" + e.ID + "]/ethernetsegment-items"
}

type EthernetSegmentType string

const EthernetSegmentTypeNative EthernetSegmentType = "native"

// MultihomingItems represents the global EVPN multihoming configuration.
type MultihomingItems struct {
	EadEviRoute    bool        `json:"eadEviRoute"`
	AdminSt        AdminSt     `json:"state"`
	DfElectionMode DfElectMode `json:"dfElectionMode,omitempty"`
	DfElectionTime string      `json:"dfElectionTime,omitempty"`
}

func (*MultihomingItems) XPath() string {
	return "System/eps-items/multihoming-items"
}

type DfElectMode string

const DfElectModeModulo DfElectMode = "modulo"

// EvpnMulticastItems represents the global EVPN multicast configuration.
type EvpnMulticastItems struct {
	State AdminSt `json:"state"`
}

func (*EvpnMulticastItems) XPath() string {
	return "System/eps-items/evpnmulticast-items"
}

// EthernetSegmentResponse is the NX-API JSON response for
// "show nve ethernet-segment summary".
type EthernetSegmentResponse struct {
	Table struct {
		Row ethernetSegmentRows `json:"ROW_es"`
	} `json:"TABLE_es"`
}

type ethernetSegmentRows []EthernetSegmentRow

func (r *ethernetSegmentRows) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '{' {
		var single EthernetSegmentRow
		if err := json.Unmarshal(data, &single); err != nil {
			return err
		}
		*r = []EthernetSegmentRow{single}
		return nil
	}
	return json.Unmarshal(data, (*[]EthernetSegmentRow)(r))
}

// EthernetSegmentRow is a single row from the NX-API
// "show nve ethernet-segment summary" response.
type EthernetSegmentRow struct {
	ESI       string `json:"esi"`
	ESState   string `json:"es-state"`
	Interface string `json:"if-name"`
}
