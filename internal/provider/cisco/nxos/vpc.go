// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"bytes"
	"encoding"
	"fmt"
	"net/netip"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

var (
	_ gnmiext.DataElement = (*VPCDomain)(nil)
	_ gnmiext.DataElement = (*VPCIf)(nil)
)

// VPCDomain represents the domain of a virtual Port Channel (vPC)
type VPCDomain struct {
	AdminSt                 AdminSt `json:"adminSt"`
	AutoRecovery            AdminSt `json:"autoRecovery"`
	AutoRecoveryReloadDelay int16   `json:"autoRecoveryIntvl"`
	DelayRestoreSVI         int16   `json:"delayRestoreSVI"`
	DelayRestoreVPC         int16   `json:"delayRestoreVPC"`
	FastConvergence         AdminSt `json:"fastConvergence"`
	ID                      int16   `json:"id"`
	L3PeerRouter            AdminSt `json:"l3PeerRouter"`
	PeerGateway             AdminSt `json:"peerGw"`
	PeerSwitch              AdminSt `json:"peerSwitch"`
	RolePrio                int32   `json:"rolePrio"`
	SysPrio                 int32   `json:"sysPrio"`
	KeepAliveItems          struct {
		DestIP        Prefix `json:"destIp"`
		SrcIP         Prefix `json:"srcIp"`
		VRF           string `json:"vrf"`
		PeerLinkItems struct {
			AdminSt AdminSt `json:"adminSt"`
			ID      string  `json:"id"`
		} `json:"peerlink-items,omitzero"`
	} `json:"keepalive-items,omitzero"`
}

func (*VPCDomain) XPath() string {
	return "System/vpc-items/inst-items/dom-items"
}

// VPCDomainOper represents the operational status of a vPC domain
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

// VPCDomainRole represents the role of a vPC peer.
type VPCDomainRole string

const (
	vpcRoleElectionNotDone             VPCDomainRole = "election-not-done"
	vpcRolePrimary                     VPCDomainRole = "cfg-master-oper-master"
	vpcRolePrimaryOperationalSecondary VPCDomainRole = "cfg-master-oper-slave"
	vpcRoleSecondary                   VPCDomainRole = "cfg-slave-oper-slave"
	vpcRoleSecondaryOperationalPrimary VPCDomainRole = "cfg-slave-oper-master"
)

// parsePeerUptime parses the peerUpTime string returned by the device.
// It assumes time is formatted as "(<seconds>) seconds"; e.g., "(3600) seconds".
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

// VPCIf represents a vPC member interface
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

var (
	_ encoding.TextMarshaler   = Prefix{}
	_ encoding.TextUnmarshaler = (*Prefix)(nil)
)

type Prefix netip.Prefix

// MarshalText implements the [encoding.TextMarshaler] interface.
func (p Prefix) MarshalText() ([]byte, error) {
	return netip.Prefix(p).MarshalText()
}

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
func (p *Prefix) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		*p = Prefix{}
		return nil
	}
	i := bytes.IndexByte(text, '/')
	if i < 0 {
		addr, err := netip.ParseAddr(string(text))
		if err != nil {
			return fmt.Errorf("failed to parse address: %w", err)
		}
		if addr.Is4() {
			*p = Prefix(netip.PrefixFrom(addr, 32))
			return nil
		}
		if addr.Is6() {
			*p = Prefix(netip.PrefixFrom(addr, 128))
			return nil
		}
		return fmt.Errorf("failed to infer prefix length from address: %s", addr.String())
	}
	np, err := netip.ParsePrefix(string(text))
	if err != nil {
		return fmt.Errorf("failed to parse prefix: %w", err)
	}
	*p = Prefix(np)
	return nil
}
