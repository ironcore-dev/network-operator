// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package nxos

import (
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"
)

var (
	_ gnmiext.Configurable = (*VPCDomain)(nil)
)

// VPCDomain represents a virtual Port Channel (vPC)
type VPCDomain struct {
	AdminSt      AdminSt `json:"adminSt,omitempty"`
	AutoRecovery AdminSt `json:"autoRecovery,omitempty"`
	// AutoRecoveryReloadDelay is the time to wait before assuming peer dead and restoring vpcs
	AutoRecoveryReloadDelay uint32 `json:"autoRecoveryIntvl,omitempty"`
	// DelayRestoreSVI is the delay in bringing up the interface-vlan
	DelayRestoreSVI uint16 `json:"delayRestoreSVI,omitempty"`
	// DelayRestoreVPC is the delay in bringing up the vPC links/interfaces of various instances after restoring the peer-link
	DelayRestoreVPC uint16  `json:"delayRestoreVPC,omitempty"`
	FastConvergence AdminSt `json:"fastConvergence,omitempty"`
	Id              uint16  `json:"id"`
	L3PeerRouter    AdminSt `json:"l3PeerRouter,omitempty"`
	PeerGateway     AdminSt `json:"peerGw,omitempty"`
	PeerSwitch      AdminSt `json:"peerSwitch,omitempty"`
	RolePrio        uint16  `json:"rolePrio,omitempty"`
	SysPrio         uint16  `json:"sysPrio,omitempty"`
	KeepAliveItems  struct {
		DestIP string `json:"destIp,omitempty"`
		SrcIP  string `json:"srcIp,omitempty"`
		VRF    string `json:"vrf,omitempty"`
	} `json:"keepalive-items,omitzero"`
}

func (v *VPCDomain) XPath() string {
	return "System/vpc-items/inst-items/dom-items"
}

var _ gnmiext.Configurable = (*VPCIf)(nil)

type VPCIf struct {
	ID             int `json:"id"`
	RsvpcConfItems struct {
		TDn string `json:"tDn"`
	} `json:"rsvpcConf-items"`
}

func (*VPCIf) IsListItem() {}

func (v *VPCIf) XPath() string {
	return "System/vpc-items/inst-items/dom-items/if-items/If-list[id=" + strconv.Itoa(v.ID) + "]"
}

func (v *VPCIf) SetPortChannel(name string) {
	v.RsvpcConfItems.TDn = "/System/intf-items/aggr-items/AggrIf-list[id='" + name + "']"
}

type VPCIfItems struct {
	IfList []*VPCIf `json:"If-list"`
}

func (*VPCIfItems) XPath() string {
	return "System/vpc-items/inst-items/dom-items/if-items"
}

func (v *VPCIfItems) GetListItemByInterface(name string) *VPCIf {
	for _, item := range v.IfList {
		if item.RsvpcConfItems.TDn == "/System/intf-items/aggr-items/AggrIf-list[id='"+name+"']" {
			return item
		}
	}
	return nil
}

// VPCOper represents the operational status of a vPC domain
type VPCDomainOper struct {
	KeepAliveItems struct {
		OperSt     string `json:"operSt,omitempty"`
		PeerUpTime string `json:"peerUpTime,omitempty"`
	} `json:"keepalive-items,omitzero"`
	Role VPCDomainRole `json:"summOperRole,omitempty"`
}

func (*VPCDomainOper) XPath() string {
	return "System/vpc-items/inst-items/dom-items"
}

// VPCRole represents the role of a vPC peer. The value `election-not-done`
// will be ignored and mapped to `unknown` role.
type VPCDomainRole string

const (
	vpcRolePrimary                     VPCDomainRole = "cfg-master-oper-master"
	vpcRolePrimaryOperationalSecondary VPCDomainRole = "cfg-master-oper-slave"
	vpcRoleSecondary                   VPCDomainRole = "cfg-slave-oper-slave"
	vpcRoleSecondaryOperationalPrimary VPCDomainRole = "cfg-slave-oper-master"
)

// parsePeerUptime parses the peerUpTime string returned by the device
// Assumes the format is "(<seconds>) seconds", e.g., "(3600) seconds".
// Ignores trailing information, i.e., milliseconds.
func parsePeerUptime(s string) (time.Duration, error) {
	re := regexp.MustCompile(`^\((\d+)\)\s*seconds`)
	m := re.FindStringSubmatch(s)
	if len(m) != 2 {
		return 0, fmt.Errorf("invalid peerUpTime format: %s", s)
	}
	seconds, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		return 0, err
	}
	return time.Duration(seconds) * time.Second, nil
}
