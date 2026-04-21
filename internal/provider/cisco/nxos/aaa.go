// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	nxv1alpha1 "github.com/ironcore-dev/network-operator/api/cisco/nx/v1alpha1"
	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

var (
	_ gnmiext.DataElement = (*TacacsPlusProvider)(nil)
	_ gnmiext.DataElement = (*TacacsPlusProviderGroup)(nil)
	_ gnmiext.DataElement = (*RadiusProvider)(nil)
	_ gnmiext.DataElement = (*RadiusProviderGroup)(nil)
	_ gnmiext.DataElement = (*AAADefaultAuth)(nil)
	_ gnmiext.DataElement = (*AAAConsoleAuth)(nil)
	_ gnmiext.DataElement = (*AAADefaultAuthor)(nil)
	_ gnmiext.DataElement = (*AAADefaultAcc)(nil)
)

// AAA configuration constants
const (
	AAARealmTacacs = "tacacs"
	AAARealmRadius = "radius"
	AAARealmLocal  = "local"
	AAARealmNone   = "none"
	AAAValueYes    = "yes"
	AAAValueNo     = "no"
)

// TacacsPlusProvider represents a TACACS+ server host configuration.
type TacacsPlusProvider struct {
	Name    string `json:"name"`
	Port    int32  `json:"port"`
	Key     string `json:"key,omitempty"`
	KeyEnc  string `json:"keyEnc"`
	Timeout int32  `json:"timeout"`
}

func (*TacacsPlusProvider) IsListItem() {}

func (p *TacacsPlusProvider) XPath() string {
	return "System/userext-items/tacacsext-items/tacacsplusprovider-items/TacacsPlusProvider-list[name=" + p.Name + "]"
}

// TacacsPlusProviderGroup represents a TACACS+ server group configuration.
type TacacsPlusProviderGroup struct {
	Name             string                          `json:"name"`
	Vrf              string                          `json:"vrf,omitempty"`
	SrcIf            string                          `json:"srcIf,omitempty"`
	Deadtime         int32                           `json:"deadtime,omitempty"`
	ProviderRefItems TacacsPlusProviderGroupRefItems `json:"providerref-items,omitzero"`
}

func (*TacacsPlusProviderGroup) IsListItem() {}

func (g *TacacsPlusProviderGroup) XPath() string {
	return "System/userext-items/tacacsext-items/tacacsplusprovidergroup-items/TacacsPlusProviderGroup-list[name=" + g.Name + "]"
}

type TacacsPlusProviderGroupRefItems struct {
	ProviderRefList gnmiext.List[string, *TacacsPlusProviderRef] `json:"ProviderRef-list,omitzero"`
}

// TacacsPlusProviderItems is the container for all TACACS+ server configurations on the device.
// Used for reading the current state — not for writing.
type TacacsPlusProviderItems struct {
	ProviderList []TacacsPlusProvider `json:"TacacsPlusProvider-list,omitzero"`
}

func (*TacacsPlusProviderItems) XPath() string {
	return "System/userext-items/tacacsext-items/tacacsplusprovider-items"
}

// TacacsPlusProviderGroupItems is the container for all TACACS+ server group configurations on the device.
// Used for reading the current state — not for writing.
type TacacsPlusProviderGroupItems struct {
	GroupList []TacacsPlusProviderGroup `json:"TacacsPlusProviderGroup-list,omitzero"`
}

func (*TacacsPlusProviderGroupItems) XPath() string {
	return "System/userext-items/tacacsext-items/tacacsplusprovidergroup-items"
}

type TacacsPlusProviderRef struct {
	Name string `json:"name"`
}

func (r *TacacsPlusProviderRef) Key() string { return r.Name }

// RadiusProvider represents a RADIUS server host configuration.
type RadiusProvider struct {
	Name     string `json:"name"`
	AuthPort int32  `json:"authPort"`
	AcctPort int32  `json:"acctPort"`
	Key      string `json:"key,omitempty"`
	KeyEnc   string `json:"keyEnc"`
	Timeout  int32  `json:"timeout"`
}

func (*RadiusProvider) IsListItem() {}

func (p *RadiusProvider) XPath() string {
	return "System/userext-items/radiusext-items/radiusprovider-items/RadiusProvider-list[name=" + p.Name + "]"
}

// RadiusProviderGroup represents a RADIUS server group configuration.
type RadiusProviderGroup struct {
	Name             string                      `json:"name"`
	Vrf              string                      `json:"vrf,omitempty"`
	SrcIf            string                      `json:"srcIf,omitempty"`
	Deadtime         int32                       `json:"deadtime,omitempty"`
	ProviderRefItems RadiusProviderGroupRefItems `json:"providerref-items,omitzero"`
}

func (*RadiusProviderGroup) IsListItem() {}

func (g *RadiusProviderGroup) XPath() string {
	return "System/userext-items/radiusext-items/radiusprovidergroup-items/RadiusProviderGroup-list[name=" + g.Name + "]"
}

type RadiusProviderGroupRefItems struct {
	ProviderRefList gnmiext.List[string, *RadiusProviderRef] `json:"ProviderRef-list,omitzero"`
}

// RadiusProviderItems is the container for all RADIUS server configurations on the device.
// Used for reading the current state — not for writing.
type RadiusProviderItems struct {
	ProviderList []RadiusProvider `json:"RadiusProvider-list,omitzero"`
}

func (*RadiusProviderItems) XPath() string {
	return "System/userext-items/radiusext-items/radiusprovider-items"
}

// RadiusProviderGroupItems is the container for all RADIUS server group configurations on the device.
// Used for reading the current state — not for writing.
type RadiusProviderGroupItems struct {
	GroupList []RadiusProviderGroup `json:"RadiusProviderGroup-list,omitzero"`
}

func (*RadiusProviderGroupItems) XPath() string {
	return "System/userext-items/radiusext-items/radiusprovidergroup-items"
}

type RadiusProviderRef struct {
	Name string `json:"name"`
}

func (r *RadiusProviderRef) Key() string { return r.Name }

// AAADefaultAuth represents AAA default authentication configuration.
type AAADefaultAuth struct {
	Realm         string `json:"realm,omitempty"`
	ProviderGroup string `json:"providerGroup"`
	Fallback      string `json:"fallback,omitempty"`
	Local         string `json:"local,omitempty"`
	None          string `json:"none,omitempty"`
	ErrEn         bool   `json:"errEn,omitempty"`
	AuthProtocol  string `json:"authProtocol,omitempty"`
}

func (*AAADefaultAuth) XPath() string {
	return "System/userext-items/authrealm-items/defaultauth-items"
}

// AAAConsoleAuth represents AAA console authentication configuration.
type AAAConsoleAuth struct {
	Realm         string `json:"realm,omitempty"`
	ProviderGroup string `json:"providerGroup"`
	Fallback      string `json:"fallback,omitempty"`
	Local         string `json:"local,omitempty"`
	None          string `json:"none,omitempty"`
	ErrEn         bool   `json:"errEn,omitempty"`
	AuthProtocol  string `json:"authProtocol,omitempty"`
}

func (*AAAConsoleAuth) XPath() string {
	return "System/userext-items/authrealm-items/consoleauth-items"
}

// AAADefaultAuthor represents AAA default authorization configuration for config commands.
// Note: "name" and "realm" are read-only operational fields on NX-OS and must not be sent.
type AAADefaultAuthor struct {
	CmdType          string `json:"cmdType"`
	ProviderGroup    string `json:"providerGroup"`
	LocalRbac        bool   `json:"localRbac,omitempty"`
	AuthorMethodNone bool   `json:"authorMethodNone,omitempty"`
}

func (*AAADefaultAuthor) IsListItem() {}

func (a *AAADefaultAuthor) XPath() string {
	return "System/userext-items/authrealm-items/defaultauthor-items/DefaultAuthor-list[cmdType=" + a.CmdType + "]"
}

// AAADefaultAcc represents AAA default accounting configuration.
// Note: "name" is a read-only operational field on NX-OS and must not be sent.
type AAADefaultAcc struct {
	Realm         string `json:"realm,omitempty"`
	ProviderGroup string `json:"providerGroup"`
	LocalRbac     bool   `json:"localRbac,omitempty"`
	AccMethodNone bool   `json:"accMethodNone,omitempty"`
}

func (*AAADefaultAcc) XPath() string {
	return "System/userext-items/authrealm-items/defaultacc-items"
}

// MapKeyEncryption maps the Cisco-specific key encryption type to NX-OS type.
func MapKeyEncryption(enc nxv1alpha1.TACACSKeyEncryption) string {
	switch enc {
	case nxv1alpha1.TACACSKeyEncryptionType6:
		return "6"
	case nxv1alpha1.TACACSKeyEncryptionType7:
		return "7"
	case nxv1alpha1.TACACSKeyEncryptionClear:
		return "0"
	default:
		return "7"
	}
}

// MapRADIUSKeyEncryption maps the Cisco-specific RADIUS key encryption type to NX-OS type.
func MapRADIUSKeyEncryption(enc nxv1alpha1.RADIUSKeyEncryption) string {
	switch enc {
	case nxv1alpha1.RADIUSKeyEncryptionType6:
		return "6"
	case nxv1alpha1.RADIUSKeyEncryptionType7:
		return "7"
	case nxv1alpha1.RADIUSKeyEncryptionClear:
		return "0"
	default:
		return "7"
	}
}

// groupTypeByName returns the AAAServerGroupType for the given group name,
// defaulting to TACACS if not found.
func groupTypeByName(name string, groups []v1alpha1.AAAServerGroup) v1alpha1.AAAServerGroupType {
	for _, g := range groups {
		if g.Name == name {
			return g.Type
		}
	}
	return v1alpha1.AAAServerGroupTypeTACACS
}

// MapRealmFromGroup returns the NX-OS realm string for the given group name,
// resolving TACACS vs RADIUS from the server group list.
func MapRealmFromGroup(groupName string, groups []v1alpha1.AAAServerGroup) string {
	switch groupTypeByName(groupName, groups) {
	case v1alpha1.AAAServerGroupTypeRADIUS:
		return AAARealmRadius
	default:
		return AAARealmTacacs
	}
}

// MapRealmFromMethodType maps the API method type to NX-OS realm.
func MapRealmFromMethodType(method v1alpha1.AAAMethodType) string {
	switch method {
	case v1alpha1.AAAMethodTypeGroup:
		return AAARealmTacacs
	case v1alpha1.AAAMethodTypeLocal:
		return AAARealmLocal
	case v1alpha1.AAAMethodTypeNone:
		return AAARealmNone
	default:
		return AAARealmLocal
	}
}

// MapLocalFromMethodList checks if local is in the method list.
func MapLocalFromMethodList(methods []v1alpha1.AAAMethod) string {
	for _, m := range methods {
		if m.Type == v1alpha1.AAAMethodTypeLocal {
			return AAAValueYes
		}
	}
	return AAAValueNo
}

// MapFallbackFromMethodList determines fallback setting from method list.
func MapFallbackFromMethodList(methods []v1alpha1.AAAMethod) string {
	// If there's more than one method, enable fallback
	if len(methods) > 1 {
		return AAAValueYes
	}
	return AAAValueNo
}

// MapRealm maps a method type string to an NX-OS realm.
func MapRealm(methodType string) string {
	switch methodType {
	case "Group":
		return AAARealmTacacs
	case "Local":
		return AAARealmLocal
	case "None":
		return AAARealmNone
	default:
		return AAARealmLocal
	}
}

// MapLocal checks if local is in a method list.
func MapLocal(methods []v1alpha1.AAAMethod) string {
	for _, m := range methods {
		if m.Type == v1alpha1.AAAMethodTypeLocal {
			return AAAValueYes
		}
	}
	return AAAValueNo
}

// MapFallback determines fallback setting from a method list.
func MapFallback(methods []v1alpha1.AAAMethod) string {
	if len(methods) > 1 {
		return AAAValueYes
	}
	return AAAValueNo
}
