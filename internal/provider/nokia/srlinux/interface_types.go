// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package srlinux

import (
	"encoding/json"
	"fmt"

	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

// Compile-time assertions for the native SR Linux interface payload types.
var (
	_ gnmiext.DataElement = (*ifaceConfig)(nil)
	_ gnmiext.DataElement = (*subinterface)(nil)
	_ gnmiext.DataElement = (*vlanTagging)(nil)
	_ gnmiext.DataElement = (*bfdSubinterface)(nil)
	_ gnmiext.DataElement = (*ifaceOperState)(nil)
)

// SR Linux leaf values. Admin-state defaults to "enable" on the device, so it is
// always written explicitly to keep the Get-and-Check diff stable (an omitted
// value would read back as the platform default and trigger a spurious Set).
const (
	adminEnable  = "enable"
	adminDisable = "disable"

	// subIfTypeRouted is the module-qualified YANG identity SR Linux returns for a
	// routed subinterface. The value must be stored qualified or the Get-and-Check
	// diff would never match the device's read-back value.
	subIfTypeRouted = "srl_nokia-interfaces:routed"

	lagTypeLACP   = "lacp"
	lagTypeStatic = "static"

	lacpModeActive  = "ACTIVE"
	lacpModePassive = "PASSIVE"

	fieldSpecType = "spec.type"
)

// ifaceConfig holds the interface-level leaves of an SR Linux interface. It is
// merged (gNMI update), never replaced, so it does not disturb subinterfaces that
// belong to separate Subinterface resources. Augmented leaves (lag) carry their
// module-qualified JSON names because that is how the device returns them; a bare
// name would fail to unmarshal and break idempotency.
type ifaceConfig struct {
	Name        string          `json:"-"`
	AdminState  string          `json:"admin-state"`
	Description string          `json:"description,omitempty"`
	MTU         uint16          `json:"mtu,omitempty"`
	Ethernet    *ethernetConfig `json:"ethernet,omitempty"`
	LAG         *lagConfig      `json:"srl_nokia-interfaces-lag:lag,omitempty"`
}

func (i *ifaceConfig) XPath() string {
	return fmt.Sprintf("srl_nokia-interfaces:interface[name=%s]", i.Name)
}

// ethernetConfig holds the interface-level ethernet container. Only aggregate-id
// is managed: it enslaves a physical port to a LAG.
type ethernetConfig struct {
	AggregateID string `json:"aggregate-id,omitempty"`
}

// lagConfig maps the srl_nokia-interfaces-lag:lag container of an aggregate interface.
type lagConfig struct {
	LagType string      `json:"lag-type,omitempty"`
	LACP    *lacpConfig `json:"srl_nokia-lacp:lacp,omitempty"`
}

// lacpConfig maps the srl_nokia-lacp:lacp container. admin-key and system-id-mac
// carry the per-LAG multi-chassis identity.
type lacpConfig struct {
	LACPMode    string `json:"lacp-mode,omitempty"`
	AdminKey    uint16 `json:"admin-key,omitempty"`
	SystemIDMAC string `json:"system-id-mac,omitempty"`
}

// vlanTagging writes only the parent interface's vlan-tagging leaf (needed for a
// tagged subinterface) without disturbing the rest of its configuration.
type vlanTagging struct {
	Name  string `json:"-"`
	Value bool   `json:"-"`
}

func (v *vlanTagging) XPath() string {
	return fmt.Sprintf("srl_nokia-interfaces:interface[name=%s]/srl_nokia-interfaces-vlans:vlan-tagging", v.Name)
}

func (v vlanTagging) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.Value)
}

func (v *vlanTagging) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, &v.Value)
}

// subinterface maps a subinterface list entry. It is used both nested inside
// ifaceConfig (ParentName empty, XPath unused) and as a standalone element
// addressed by its own path (ParentName set) so a tagged subinterface can be
// written without replacing its parent interface.
type subinterface struct {
	ParentName string   `json:"-"`
	Index      uint32   `json:"-"`
	AdminState string   `json:"admin-state"`
	Type       string   `json:"type,omitempty"`
	IPMTU      uint16   `json:"ip-mtu,omitempty"`
	VLAN       *subVLAN `json:"srl_nokia-interfaces-vlans:vlan,omitempty"`
	IPv4       *subIPv4 `json:"ipv4,omitempty"`
	IPv6       *subIPv6 `json:"ipv6,omitempty"`
}

func (s *subinterface) XPath() string {
	return fmt.Sprintf("srl_nokia-interfaces:interface[name=%s]/subinterface[index=%d]", s.ParentName, s.Index)
}

// MarshalJSON injects the list key (index) into the payload. The field itself is
// json:"-" so it is preserved during the Get-and-Check comparison: SR Linux omits
// list keys from GetConfig responses, so a serialized key would be zeroed on read
// and produce a false diff (a write on every reconcile) for any non-zero index.
func (s *subinterface) MarshalJSON() ([]byte, error) {
	type alias subinterface
	b, err := json.Marshal((*alias)(s))
	if err != nil {
		return nil, err
	}
	return injectKey(b, "index", s.Index)
}

// subVLAN maps the subinterface vlan encapsulation container.
type subVLAN struct {
	Encap *vlanEncap `json:"encap,omitempty"`
}

type vlanEncap struct {
	SingleTagged *singleTagged `json:"single-tagged,omitempty"`
	DoubleTagged *doubleTagged `json:"double-tagged,omitempty"`
}

type singleTagged struct {
	VLANID uint16 `json:"vlan-id"`
}

type doubleTagged struct {
	OuterVLANID uint16 `json:"outer-vlan-id"`
	InnerVLANID uint16 `json:"inner-vlan-id"`
}

// subIPv4 maps the subinterface ipv4 container. SR Linux has no primary/secondary
// distinction: addresses are a plain list of ip-prefix (CIDR) strings.
type subIPv4 struct {
	AdminState string                           `json:"admin-state"`
	Address    gnmiext.List[string, *ipAddress] `json:"address,omitempty"`
	Unnumbered *unnumbered                      `json:"unnumbered,omitempty"`
}

type subIPv6 struct {
	AdminState string                           `json:"admin-state"`
	Address    gnmiext.List[string, *ipAddress] `json:"address,omitempty"`
}

// ipAddress is a single ipv4/ipv6 address entry keyed by its CIDR ip-prefix.
type ipAddress struct {
	IPPrefix string `json:"ip-prefix"`
}

func (a *ipAddress) Key() string { return a.IPPrefix }

// unnumbered maps the ipv4 unnumbered container; interface references the borrowed
// source (e.g. "system0.0").
type unnumbered struct {
	AdminState string `json:"admin-state"`
	Interface  string `json:"interface,omitempty"`
}

// bfdSubinterface maps an entry of the top-level srl_nokia-bfd:bfd/subinterface
// list. Its id is "<interface>.<index>". Intervals are expressed in microseconds.
type bfdSubinterface struct {
	ID                   string `json:"-"`
	AdminState           string `json:"admin-state"`
	DesiredMinTxInterval uint32 `json:"desired-minimum-transmit-interval,omitempty"`
	RequiredMinRx        uint32 `json:"required-minimum-receive,omitempty"`
	DetectionMultiplier  uint8  `json:"detection-multiplier,omitempty"`
}

func (b *bfdSubinterface) XPath() string {
	return fmt.Sprintf("srl_nokia-bfd:bfd/subinterface[id=%s]", b.ID)
}

// MarshalJSON injects the list key (id) into the payload; see subinterface.MarshalJSON.
func (b *bfdSubinterface) MarshalJSON() ([]byte, error) {
	type alias bfdSubinterface
	raw, err := json.Marshal((*alias)(b))
	if err != nil {
		return nil, err
	}
	return injectKey(raw, "id", b.ID)
}

// injectKey adds a YANG list key leaf into an already-marshaled JSON object. SR
// Linux requires the key in a Set body but omits it from GetConfig responses, so
// the key is kept out of the struct's json tags (preserved across the idempotency
// comparison) and spliced back in only for writes.
func injectKey(b []byte, key string, val any) ([]byte, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(val)
	if err != nil {
		return nil, err
	}
	m[key] = raw
	return json.Marshal(m)
}

// ifaceOperState reads the operational state of an interface. SR Linux reports
// oper-state as a direct leaf with the lowercase values "up"/"down".
type ifaceOperState struct {
	Name      string `json:"-"`
	OperState string `json:"oper-state,omitempty"`
}

func (s *ifaceOperState) XPath() string {
	return fmt.Sprintf("srl_nokia-interfaces:interface[name=%s]", s.Name)
}

const operStateUp = "up"
