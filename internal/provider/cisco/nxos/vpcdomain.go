// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package nxos

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"
)

var (
	_ gnmiext.Configurable = (*VPCDomain)(nil)
	_ gnmiext.Configurable = (*VPCPeerLinkIf)(nil)
)

// VPCDomain represents a virtual Port Channel (vPC)
type VPCDomain struct {
	AdminSt      AdminSt         `json:"adminSt"`
	AutoRecovery Option[AdminSt] `json:"autoRecovery"`
	// AutoRecoveryReloadDelay is the time to wait before assuming peer dead and restoring vpcs
	AutoRecoveryReloadDelay uint32 `json:"autoRecoveryIntvl,omitempty"`
	// DelayRestoreSVI is the delay in bringing up the interface-vlan
	DelayRestoreSVI Option[uint16] `json:"delayRestoreSVI"`
	// DelayRestoreVPC is the delay in bringing up the vPC links/interfaces of various instances after restoring the peer-link
	DelayRestoreVPC Option[uint16]  `json:"delayRestoreVPC"`
	FastConvergence Option[AdminSt] `json:"fastConvergence"`
	Id              uint16          `json:"id"`
	L3PeerRouter    AdminSt         `json:"l3PeerRouter,omitempty"`
	PeerGateway     AdminSt         `json:"peerGw,omitempty"`
	PeerSwitch      AdminSt         `json:"peerSwitch,omitempty"`
	RolePrio        Option[uint16]  `json:"rolePrio"`
	SysPrio         Option[uint16]  `json:"sysPrio"`
	KeepAliveItems  struct {
		DestIP        string `json:"destIp,omitempty"`
		SrcIP         string `json:"srcIp,omitempty"`
		VRF           string `json:"vrf,omitempty"`
		PeerLinkItems struct {
			AdminSt AdminSt `json:"adminSt"`
			Id      string  `json:"id"`
		} `json:"peerlink-items"`
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
	PeerStQual string        `json:"peerStQual,omitempty"`
	Role       VPCDomainRole `json:"summOperRole,omitempty"`
}

func (*VPCDomainOper) XPath() string {
	return "System/vpc-items/inst-items/dom-items"
}

type VPCPeerLinkIf struct {
	Id string `json:"id"`
}

func (*VPCPeerLinkIf) XPath() string {
	return "System/vpc-items/inst-items/dom-items/keepalive-items/peerlink-items"
}

// VPCRole represents the role of a vPC peer.
type VPCDomainRole string

const (
	vpcRoleElectionNotDone             VPCDomainRole = "election-not-done"
	vpcRolePrimary                     VPCDomainRole = "cfg-master-oper-master"
	vpcRolePrimaryOperationalSecondary VPCDomainRole = "cfg-master-oper-slave"
	vpcRoleSecondary                   VPCDomainRole = "cfg-slave-oper-slave"
	vpcRoleSecondaryOperationalPrimary VPCDomainRole = "cfg-slave-oper-master"
)

// parsePeerUptime parses the peerUpTime string returned by the device
// Assumes the format is "(<seconds>) seconds", e.g., "(3600) seconds".
// Ignores trailing information, i.e., milliseconds.
func parsePeerUptime(s string) (*time.Duration, error) {
	re := regexp.MustCompile(`^\((\d+)\)\s*seconds`)
	m := re.FindStringSubmatch(s)
	if len(m) != 2 {
		return nil, fmt.Errorf("invalid peerUpTime format: %s", s)
	}
	seconds, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		return nil, err
	}
	d := time.Duration(seconds) * time.Second
	return &d, nil
}

func peerIsAlive(operSt string) bool {
	st := strings.Split(operSt, ",")
	if len(st) != 2 {
		return false
	}
	slices.Sort(st)
	if st[0] == "operational" && st[1] == "peer-was-alive" {
		return true
	}
	return false
}
