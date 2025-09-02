// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"errors"

	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

var _ gnmiext.DeviceConf = (*GRPC)(nil)

type GRPC struct {
	// Enable the gRPC agent on the switch. The default is false.
	Enable bool
	// The port number for the grpc server. The range of port-id is from 1024 to 65535. 50051 is the default.
	Port uint32
	// Enable the gRPC agent to accept incoming (dial-in) RPC requests from a given VRF.
	// If left empty, the management VRF processes incoming RPC requests when the gRPC feature is enabled.
	Vrf string
	// The certificate trustpoint ID.
	Trustpoint string
	// GNMI configuration.
	GNMI *GNMI `json:"gnmi,omitempty"`
}

type GNMI struct {
	// The maximum number of concurrent gNMI calls that can be made to the gRPC server on the switch for each VRF.
	// Configure a limit from 1 through 16. The default limit is 8.
	MaxConcurrentCall uint16
	// Configure the keepalive timeout for inactive or unauthorized connections.
	// The gRPC agent periodically sends an empty response to the client. If this response fails to deliver to the client, then the connection stops.
	// The default interval value is 600 seconds. You can configure to change the keepalive interval with the interval range of 600-86400 seconds.
	KeepAliveTimeout uint32
	// Configure the minimum sample interval for the gNMI telemetry stream.
	// Once per stream sample interval, the switch sends the current values for all specified paths. The supported sample interval range is from 1 through 604,800 second.
	// The default sample interval is 10 seconds.
	MinSampleInterval uint32
}

// ToYGOT converts the GRPC configuration to a slice of gNMI updates.
func (g *GRPC) ToYGOT(_ context.Context, _ gnmiext.Client) ([]gnmiext.Update, error) {
	if !g.Enable {
		return []gnmiext.Update{
			gnmiext.EditingUpdate{
				XPath: "System/fm-items/grpc-items",
				Value: &nxos.Cisco_NX_OSDevice_System_FmItems_GrpcItems{AdminSt: nxos.Cisco_NX_OSDevice_Fm_AdminState_disabled},
			},
		}, nil
	}

	if g.Port == 0 {
		g.Port = 50051
	}

	if g.GNMI == nil {
		g.GNMI = &GNMI{}
	}

	if g.GNMI.MaxConcurrentCall == 0 {
		g.GNMI.MaxConcurrentCall = 8
	}

	if g.GNMI.KeepAliveTimeout == 0 {
		g.GNMI.KeepAliveTimeout = 600
	}

	if g.GNMI.MinSampleInterval == 0 {
		g.GNMI.MinSampleInterval = 10
	}

	var vrf *string
	if g.Vrf != "" {
		vrf = ygot.String(g.Vrf)
	}

	return []gnmiext.Update{
		gnmiext.EditingUpdate{
			XPath: "System/fm-items/grpc-items",
			Value: &nxos.Cisco_NX_OSDevice_System_FmItems_GrpcItems{AdminSt: nxos.Cisco_NX_OSDevice_Fm_AdminState_enabled},
		},
		gnmiext.EditingUpdate{
			XPath: "System/grpc-items",
			Value: &nxos.Cisco_NX_OSDevice_System_GrpcItems{
				Cert:   ygot.String(g.Trustpoint),
				Port:   ygot.Uint32(g.Port),
				UseVrf: vrf,
				GnmiItems: &nxos.Cisco_NX_OSDevice_System_GrpcItems_GnmiItems{
					MaxCalls:          ygot.Uint16(g.GNMI.MaxConcurrentCall),
					KeepAliveTimeout:  ygot.Uint32(g.GNMI.KeepAliveTimeout),
					MinSampleInterval: ygot.Uint32(g.GNMI.MinSampleInterval),
				},
			},
		},
	}, nil
}

// returns an empty update and an error indicating that the reset is not implemented
func (v *GRPC) Reset(_ context.Context, _ gnmiext.Client) ([]gnmiext.Update, error) {
	return []gnmiext.Update{}, errors.New("grpc: reset not implemented as it effectively disables management over gNMI")
}
