// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

// This package enables the configuration of Control Plane Policing (CoPP) on the device as per the following configuration sample:
//   - copp profile moderate
//
// Which corresponds to the following YANG model:
//   - /System/copp-items/profile-items
package copp

import (
	"fmt"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

var _ gnmiext.DeviceConf = (*COPP)(nil)

type COPP struct {
	Profile Profile
}

//go:generate go tool stringer -type=Profile

type Profile int64

const (
	Unspecified Profile = iota
	Unknown
	Strict
	Moderate
	Dense
	Lenient
)

func (c *COPP) ToYGOT(_ gnmiext.Client) ([]gnmiext.Update, error) {
	var prof nxos.E_Cisco_NX_OSDevice_Copp_ProfT
	switch c.Profile {
	case Unspecified:
		prof = nxos.Cisco_NX_OSDevice_Copp_ProfT_UNSET
	case Unknown:
		prof = nxos.Cisco_NX_OSDevice_Copp_ProfT_unknown
	case Strict:
		prof = nxos.Cisco_NX_OSDevice_Copp_ProfT_strict
	case Moderate:
		prof = nxos.Cisco_NX_OSDevice_Copp_ProfT_moderate
	case Dense:
		prof = nxos.Cisco_NX_OSDevice_Copp_ProfT_dense
	case Lenient:
		prof = nxos.Cisco_NX_OSDevice_Copp_ProfT_lenient
	default:
		return nil, fmt.Errorf("copp: invalid copp profile %s", c.Profile)
	}

	return []gnmiext.Update{
		gnmiext.EditingUpdate{
			XPath: "System/copp-items/profile-items",
			Value: &nxos.Cisco_NX_OSDevice_System_CoppItems_ProfileItems{Prof: prof},
		},
	}, nil
}

// sets the COPP profile to strict, which is the default profile.
func (v *COPP) Reset(_ gnmiext.Client) ([]gnmiext.Update, error) {
	x := &nxos.Cisco_NX_OSDevice_System_CoppItems_ProfileItems{}
	x.PopulateDefaults()
	x.Prof = nxos.Cisco_NX_OSDevice_Copp_ProfT_strict
	return []gnmiext.Update{
		gnmiext.EditingUpdate{
			XPath: "System/copp-items/profile-items",
			Value: x,
		},
	}, nil
}
