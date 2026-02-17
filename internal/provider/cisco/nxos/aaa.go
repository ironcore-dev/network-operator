// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	nxv1alpha1 "github.com/ironcore-dev/network-operator/api/cisco/nx/v1alpha1"
	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	gnmiext "github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"
)

var (
	_ gnmiext.Configurable = (*TACACSFeature)(nil)
	_ gnmiext.Configurable = (*TacacsPlusProvider)(nil)
	_ gnmiext.Configurable = (*TacacsPlusProviderGroup)(nil)
	_ gnmiext.Configurable = (*AAADefaultAuth)(nil)
	_ gnmiext.Configurable = (*AAAConsoleAuth)(nil)
	_ gnmiext.Configurable = (*AAADefaultAuthor)(nil)
	_ gnmiext.Configurable = (*AAADefaultAcc)(nil)
)

// TACACSFeature enables/disables the TACACS+ feature on NX-OS.
type TACACSFeature string

func (*TACACSFeature) XPath() string {
	return "System/fm-items/tacacsplus-items/adminSt"
}

const (
	TACACSFeatureEnabled  TACACSFeature = "enabled"
	TACACSFeatureDisabled TACACSFeature = "disabled"
)

// AAA configuration constants
const (
	AAARealmTacacs = "tacacs"
	AAARealmLocal  = "local"
	AAARealmNone   = "none"
	AAAValueYes    = "yes"
	AAAValueNo     = "no"
)

// TacacsPlusProvider represents a TACACS+ server host configuration.
type TacacsPlusProvider struct {
	Name         string `json:"name"`
	Port         int32  `json:"port,omitempty"`
	Key          string `json:"key,omitempty"`
	KeyEnc       string `json:"keyEnc,omitempty"`
	Timeout      int32  `json:"timeout,omitempty"`
	Retries      int32  `json:"retries,omitempty"`
	AuthProtocol string `json:"authProtocol,omitempty"`
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

type TacacsPlusProviderRef struct {
	Name string `json:"name"`
}

func (r *TacacsPlusProviderRef) Key() string { return r.Name }

// AAADefaultAuth represents AAA default authentication configuration.
type AAADefaultAuth struct {
	Realm         string `json:"realm,omitempty"`
	ProviderGroup string `json:"providerGroup,omitempty"`
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
	ProviderGroup string `json:"providerGroup,omitempty"`
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
	ProviderGroup    string `json:"providerGroup,omitempty"`
	LocalRbac        bool   `json:"localRbac,omitempty"`
	AuthorMethodNone bool   `json:"authorMethodNone,omitempty"`
}

func (*AAADefaultAuthor) IsListItem() {}

func (a *AAADefaultAuthor) XPath() string {
	return "System/userext-items/authrealm-items/defaultauthor-items/DefaultAuthor-list[cmdType=" + a.CmdType + "]"
}

// AAADefaultAcc represents AAA default accounting configuration.
type AAADefaultAcc struct {
	Name          string `json:"name,omitempty"`
	Realm         string `json:"realm,omitempty"`
	ProviderGroup string `json:"providerGroup,omitempty"`
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

// MapNXOSRealm maps an NX-OS method type string to NX-OS realm.
func MapNXOSRealm(methodType string) string {
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

// MapNXOSLocal checks if local is in an NX-OS method list.
func MapNXOSLocal(methods []nxv1alpha1.NXOSMethod) string {
	for _, m := range methods {
		if m.Type == "Local" {
			return AAAValueYes
		}
	}
	return AAAValueNo
}

// MapNXOSFallback determines fallback setting from an NX-OS method list.
func MapNXOSFallback(methods []nxv1alpha1.NXOSMethod) string {
	if len(methods) > 1 {
		return AAAValueYes
	}
	return AAAValueNo
}
