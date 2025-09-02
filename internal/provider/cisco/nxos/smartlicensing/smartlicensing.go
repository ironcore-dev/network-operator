// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package smartlicensing

import (
	"context"
	"fmt"

	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

var (
	_ gnmiext.DeviceConf = (*Licensing)(nil)
	_ gnmiext.DeviceConf = (*CallHome)(nil)
)

type Licensing struct {
	// Smart Transport URL
	URL string
	// URL for transport mode CSLU.
	CSLU string
	// Transport mode for smart licensing. Default is off
	Transport TransportMode
	// The VRF to use to send smart licensing notifications.
	Vrf string
}

//go:generate go run golang.org/x/tools/cmd/stringer@v0.35.0 -type=TransportMode

type TransportMode uint8

const (
	TransportOff TransportMode = iota
	TransportSmart
	TransportCSLU
	TransportCallhome
)

func (l *Licensing) ToYGOT(_ context.Context, _ gnmiext.Client) ([]gnmiext.Update, error) {
	var mode nxos.E_Cisco_NX_OSDevice_LicensemanagerTransportmodeType
	switch l.Transport {
	case TransportCallhome:
		mode = nxos.Cisco_NX_OSDevice_LicensemanagerTransportmodeType_transportCallhome
	case TransportCSLU:
		mode = nxos.Cisco_NX_OSDevice_LicensemanagerTransportmodeType_transportCslu
	case TransportOff:
		mode = nxos.Cisco_NX_OSDevice_LicensemanagerTransportmodeType_transportOff
	case TransportSmart:
		mode = nxos.Cisco_NX_OSDevice_LicensemanagerTransportmodeType_transportSmart
	default:
		return nil, fmt.Errorf("smartlicensing: invalid transport mode %s", l.Transport)
	}

	return []gnmiext.Update{
		gnmiext.EditingUpdate{
			XPath: "System/licensemanager-items/inst-items/smartlicensing-items",
			Value: &nxos.Cisco_NX_OSDevice_System_LicensemanagerItems_InstItems_SmartlicensingItems{
				TransportMode:          mode,
				TransportcsluurlItems:  &nxos.Cisco_NX_OSDevice_System_LicensemanagerItems_InstItems_SmartlicensingItems_TransportcsluurlItems{Url: ygot.String(l.CSLU)},
				TransportsmarturlItems: &nxos.Cisco_NX_OSDevice_System_LicensemanagerItems_InstItems_SmartlicensingItems_TransportsmarturlItems{Url: ygot.String(l.URL)},
				VrfItems:               &nxos.Cisco_NX_OSDevice_System_LicensemanagerItems_InstItems_SmartlicensingItems_VrfItems{Vrfname: ygot.String(l.Vrf)},
			},
			IgnorePaths: []string{
				"/usage-items",
			},
		},
	}, nil
}

func (v *Licensing) Reset(_ context.Context, _ gnmiext.Client) ([]gnmiext.Update, error) {
	x := &nxos.Cisco_NX_OSDevice_System_LicensemanagerItems_InstItems_SmartlicensingItems{}
	x.PopulateDefaults()
	return []gnmiext.Update{
		gnmiext.EditingUpdate{
			XPath: "System/licensemanager-items/inst-items/smartlicensing-items",
			Value: x,
		},
	}, nil
}

type CallHome struct {
	Enable   bool
	Email    string
	Vrf      string
	Profiles []*Profile
}

type Profile struct {
	URL string
	Seq uint32
}

func (c *CallHome) ToYGOT(_ context.Context, _ gnmiext.Client) ([]gnmiext.Update, error) {
	if !c.Enable {
		return []gnmiext.Update{
			gnmiext.EditingUpdate{
				XPath: "System/callhome-items/inst-items",
				Value: &nxos.Cisco_NX_OSDevice_System_CallhomeItems_InstItems{
					CallhomeEnable: nxos.Cisco_NX_OSDevice_Callhome_Boolean_disabled,
				},
			},
		}, nil
	}

	items := &nxos.Cisco_NX_OSDevice_System_CallhomeItems_InstItems_DestprofItems_PredefprofileItems_PredefinedProfileList_PdprofhttpItems{
		PredefProfHttpList: make(map[uint32]*nxos.Cisco_NX_OSDevice_System_CallhomeItems_InstItems_DestprofItems_PredefprofileItems_PredefinedProfileList_PdprofhttpItems_PredefProfHttpList, len(c.Profiles)),
	}
	for _, p := range c.Profiles {
		err := items.AppendPredefProfHttpList(&nxos.Cisco_NX_OSDevice_System_CallhomeItems_InstItems_DestprofItems_PredefprofileItems_PredefinedProfileList_PdprofhttpItems_PredefProfHttpList{
			Http:   ygot.String(p.URL),
			SeqNum: ygot.Uint32(p.Seq),
		})
		if err != nil {
			return nil, err
		}
	}

	return []gnmiext.Update{
		// Make sure the email is configured before enabling SNMP, otherwise the transaction might fail.
		gnmiext.EditingUpdate{
			XPath: "System/callhome-items/inst-items",
			Value: &nxos.Cisco_NX_OSDevice_System_CallhomeItems_InstItems{
				EmailId: ygot.String(c.Email),
			},
			IgnorePaths: []string{
				"/alertgroup-items",
				"/callhomeEnable",
				"/contractId",
				"/customerId",
				"/destprof-items",
				"/dupMsgThrottle",
				"/periodInvNotifInterval",
				"/periodInvNotifTimeOfDayHour",
				"/periodInvNotifTimeOfDayMinute",
				"/periodicInvNotif",
				"/phoneContact",
				"/siteId",
				"/sourceinterface-items",
				"/streetAddress",
				"/switchPri",
				"/transport-items",
			},
		},
		gnmiext.EditingUpdate{
			XPath: "System/callhome-items/inst-items",
			Value: &nxos.Cisco_NX_OSDevice_System_CallhomeItems_InstItems{
				EmailId:        ygot.String(c.Email),
				CallhomeEnable: nxos.Cisco_NX_OSDevice_Callhome_Boolean_enabled,
				TransportItems: &nxos.Cisco_NX_OSDevice_System_CallhomeItems_InstItems_TransportItems{HttpUseVrf: ygot.String(c.Vrf)},
				DestprofItems: &nxos.Cisco_NX_OSDevice_System_CallhomeItems_InstItems_DestprofItems{
					PredefprofileItems: &nxos.Cisco_NX_OSDevice_System_CallhomeItems_InstItems_DestprofItems_PredefprofileItems{
						PredefinedProfileList: map[nxos.E_Cisco_NX_OSDevice_Callhome_PredefProfileName]*nxos.Cisco_NX_OSDevice_System_CallhomeItems_InstItems_DestprofItems_PredefprofileItems_PredefinedProfileList{
							nxos.Cisco_NX_OSDevice_Callhome_PredefProfileName_predef_tac_prof: {
								AlertGrpCiscoTac1:    nxos.Cisco_NX_OSDevice_Callhome_Boolean_enabled,
								Format:               nxos.Cisco_NX_OSDevice_Callhome_Format_xml,
								MessageLevel:         ygot.Uint32(0),
								MessageSize:          ygot.Uint32(5_000_000),
								PredefProfile:        nxos.Cisco_NX_OSDevice_Callhome_PredefProfileName_predef_tac_prof,
								TransportMethodHttp:  nxos.Cisco_NX_OSDevice_Callhome_Boolean_enabled,
								TransportMethodEmail: nxos.Cisco_NX_OSDevice_Callhome_Boolean_disabled,
								PdprofhttpItems:      items,
							},
						},
					},
				},
			},
			IgnorePaths: []string{
				"/dupMsgThrottle",
				"/emailId",
				"/periodInvNotifInterval",
				"/periodInvNotifTimeOfDayHour",
				"/periodInvNotifTimeOfDayMinute",
				"/periodicInvNotif",
				"/switchPri",
				"/transport-items/httpProxyEnable",
				"/transport-items/proxyServerPort",
				"destprof-items/predefprofile-items/PredefinedProfile-list[predefProfile=full_txt]",
				"destprof-items/predefprofile-items/PredefinedProfile-list[predefProfile=short_txt]",
			},
		},
	}, nil
}

func (v *CallHome) Reset(_ context.Context, _ gnmiext.Client) ([]gnmiext.Update, error) {
	x := &nxos.Cisco_NX_OSDevice_System_CallhomeItems_InstItems{}
	x.PopulateDefaults()
	return []gnmiext.Update{
		gnmiext.EditingUpdate{
			XPath: "System/callhome-items/inst-items",
			Value: x,
		},
	}, nil
}
