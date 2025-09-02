// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"fmt"

	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

var _ gnmiext.DeviceConf = (*NXAPI)(nil)

type NXAPI struct {
	// Enable the NX-API agent on the switch. The default is false.
	Enable bool
	// Certificate configuration for the NX-API server.
	Cert NXAPICert
}

type NXAPICert interface{ isNXAPICert() }

type Certificate struct {
	// HTTPS certificate filename.
	CertFile string
	// HTTPS key file name
	KeyFile string
	// The passphrase for decrypting the encrypted private key.
	// Leave empty, if no passphrase is required.
	Passphrase string
}

func (Certificate) isNXAPICert() {}

type Trustpoint struct {
	// The certificate trustpoint ID.
	ID string
}

func (Trustpoint) isNXAPICert() {}

func (n *NXAPI) ToYGOT(_ context.Context, _ gnmiext.Client) ([]gnmiext.Update, error) {
	if !n.Enable {
		return []gnmiext.Update{
			gnmiext.EditingUpdate{
				XPath: "System/fm-items/nxapi-items",
				Value: &nxos.Cisco_NX_OSDevice_System_FmItems_NxapiItems{AdminSt: nxos.Cisco_NX_OSDevice_Fm_AdminState_disabled},
			},
		}, nil
	}

	updates := []gnmiext.Update{
		gnmiext.EditingUpdate{
			XPath: "System/fm-items/nxapi-items",
			Value: &nxos.Cisco_NX_OSDevice_System_FmItems_NxapiItems{AdminSt: nxos.Cisco_NX_OSDevice_Fm_AdminState_enabled},
		},
	}

	if n.Cert != nil {
		nxapi := &nxos.Cisco_NX_OSDevice_System_NxapiItems{}
		if n.Cert != nil {
			switch v := n.Cert.(type) {
			case Trustpoint:
				nxapi.Trustpoint = ygot.String(v.ID)
			case Certificate:
				nxapi.CertFile = ygot.String(v.CertFile)
				nxapi.KeyFile = ygot.String(v.KeyFile)
				nxapi.CertEnable = ygot.Bool(true)
				if v.Passphrase != "" {
					nxapi.EncrKeyPassphrase = ygot.String(v.Passphrase)
				}
			default:
				return nil, fmt.Errorf("nxapi: unknown certificate type %T", v)
			}
		}

		updates = append(updates, gnmiext.EditingUpdate{
			XPath: "System/nxapi-items",
			Value: nxapi,
			IgnorePaths: []string{
				"/clientCertAuth",
				"/httpPort",
				"/httpsPort",
				"/idleTimeout",
				"/sslCiphersWeak",
				"/sslProtocols",
				"/sudi",
				"/useVrf",
			},
		})
	}

	return updates, nil
}

// disables nxapi, resets config to defaults
func (v *NXAPI) Reset(_ context.Context, _ gnmiext.Client) ([]gnmiext.Update, error) {
	nxapi := &nxos.Cisco_NX_OSDevice_System_NxapiItems{}
	nxapi.PopulateDefaults()
	return []gnmiext.Update{
		gnmiext.EditingUpdate{
			XPath: "System/fm-items/nxapi-items",
			Value: &nxos.Cisco_NX_OSDevice_System_FmItems_NxapiItems{AdminSt: nxos.Cisco_NX_OSDevice_Fm_AdminState_disabled},
		},
		gnmiext.EditingUpdate{
			XPath: "System/nxapi-items",
			Value: nxapi,
		},
	}, nil
}
