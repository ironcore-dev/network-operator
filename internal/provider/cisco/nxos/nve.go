// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"encoding/json"
	"strconv"

	"github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"
)

var _ gnmiext.Configurable = (*NVE)(nil)
var _ gnmiext.Configurable = (*NVEInfraVLANs)(nil)
var _ gnmiext.Configurable = (*FabricFwd)(nil)

// NVE represents the Network Virtualization Edge interface (nve1).
// Note: NXOS only supports a single NVE interface with epId=1.
type NVE struct {
	AdminSt          AdminSt        `json:"adminSt"`
	AdvertiseVmac    bool           `json:"advertiseVmac"`
	SourceInterface  string         `json:"sourceInterface,omitempty"`
	AnycastInterface Option[string] `json:"anycastIntf"`
	HoldDownTime     uint16         `json:"holdDownTime"`
	HostReach        HostReachType  `json:"hostReach"`
	McastGroupL2     Option[string] `json:"mcastGroupL2"`
	McastGroupL3     Option[string] `json:"mcastGroupL3"`
	SuppressARP      bool           `json:"suppressARP"`
}

type HostReachType string

const (
	HostReachFloodAndLearn HostReachType = "Flood_and_learn"
	HostReachBGP           HostReachType = "bgp"
	HostReachController    HostReachType = "controller"
	HostReachOpenFlow      HostReachType = "openflow"
	HostReachOpenFlowIR    HostReachType = "openflowIR"
)

func (*NVE) IsListItem() {}

func (n *NVE) XPath() string {
	return "System/eps-items/epId-items/Ep-list[epId=1]"
}

var (
	_ json.Marshaler   = (*NVE)(nil)
	_ json.Unmarshaler = (*NVE)(nil)
)

// MarshalJSON marshals the NVE struct to JSON, adding the fixed epId field with value 1.
func (n NVE) MarshalJSON() ([]byte, error) {
	type Copy NVE
	cpy := Copy(n)
	return json.Marshal(struct {
		EpId int32 `json:"epId"`
		Copy
	}{
		Copy: cpy,
		EpId: 1,
	})
}

// UnmarshalJSON unmarshals JSON data into the NVE struct, ignoring the epId field, which is always 1.
func (n *NVE) UnmarshalJSON(b []byte) error {
	type Copy NVE
	var aux struct {
		EpId *int32 `json:"epId,omitempty"`
		Copy
	}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	*n = NVE(aux.Copy)
	return nil
}

type VNI struct {
	AssociateVrfFlag bool           `json:"associateVrfFlag"`
	McastGroup       Option[string] `json:"mcastGroup"`
	Vni              int32          `json:"vni"`
}

func (*VNI) IsListItem() {}

func (v *VNI) XPath() string {
	return "System/eps-items/epId-items/Ep-list[epId=1]/nws-items/vni-items/Nw-list[vni=" + strconv.FormatInt(int64(v.Vni), 10) + "]"
}

type VNIOperItems struct {
	Vni   int32  `json:"vni"`
	State OperSt `json:"state"`
}

func (v *VNIOperItems) XPath() string {
	return "System/eps-items/epId-items/Ep-list[epId=1]/nws-items/opervni-items/OperNw-list[vni=" + strconv.FormatInt(int64(v.Vni), 10) + "]"
}

type VNIState string

const (
	VNIStateUp   VNIState = "Up"
	VNIStateDown VNIState = "Down"
)

type NVEInfraVLANs struct {
	InfraVLANList []*NVEInfraVLAN `json:"InfraVlan-list,omitempty"`
}

func (n *NVEInfraVLANs) XPath() string {
	return "System/pltfm-items/nve-items/NVE-list[id=1]/infravlan-items"
}

type NVEInfraVLAN struct {
	ID uint32 `json:"id"`
}

func (*NVEInfraVLAN) IsListItem() {}

// NVEOper represents the operational state of the NVE interface.
// Note: NXOS also returns the Operational status of the associated interfaces,
// but those are not included here.
type NVEOper struct {
	OperSt OperSt `json:"operState"`
}

func (n *NVEOper) XPath() string {
	return "System/eps-items/epId-items/Ep-list[epId=1]"
}

func (*NVEOper) IsListItem() {}

// FabricFwd represents the fabric forwarding settings required for NVE operation.
// Should use only PATCH operations: `FabricFwdIf` also modifies this model.
type FabricFwd struct {
	// AdminSt defines the administrative state of fabric forwarding
	AdminSt string `json:"adminSt"`
	// Address defines the anycast gateway MAC address
	Address string `json:"amac"`
}

func (*FabricFwd) XPath() string {
	return "System/hmm-items/fwdinst-items"
}

func (*FabricFwd) IsListItem() {}
