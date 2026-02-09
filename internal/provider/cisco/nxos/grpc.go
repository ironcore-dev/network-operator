// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"errors"
	"fmt"

	"github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"
)

var (
	_ gnmiext.Configurable = (*GRPC)(nil)
	_ gnmiext.Defaultable  = (*GRPC)(nil)
	_ gnmiext.Configurable = (*GNMI)(nil)
	_ gnmiext.Defaultable  = (*GNMI)(nil)
)

// GRPC represents the gRPC configuration on a NX-OS device.
type GRPC struct {
	Cert           Option[string] `json:"cert,omitzero"`
	CertClientRoot string         `json:"certClientRoot,omitempty"`
	Port           int32          `json:"port"`
	UseVrf         string         `json:"useVrf,omitempty"`
}

func (*GRPC) XPath() string {
	return "System/grpc-items"
}

func (g *GRPC) Default() {
	g.Port = 50051
}

func (g *GRPC) Validate() error {
	if g.Port < 1024 || g.Port > 65535 {
		return fmt.Errorf("grpc: invalid port %d: must be between 1024 and 65535", g.Port)
	}
	if g.UseVrf == ManagementVRFName {
		return errors.New("grpc: cannot use vrf 'management'")
	}
	return nil
}

// GNMI represents the gNMI configuration on a NX-OS device.
type GNMI struct {
	// The keepalive timeout in seconds for inactive or unauthorized connections.
	// The gRPC agent periodically sends an empty response to the client. If this response fails to deliver to the client, then the connection stops.
	// The default interval value is 600 seconds. You can configure to change the keepalive interval with the interval range of 600-86400 seconds.
	KeepAliveTimeout int `json:"keepAliveTimeout"`
	// The maximum number of concurrent gNMI calls that can be made to the gRPC server on the switch for each VRF.
	// Configure a limit from 1 through 16. The default limit is 8.
	MaxCalls int8 `json:"maxCalls"`
}

func (*GNMI) XPath() string {
	return "System/grpc-items/gnmi-items"
}

func (g *GNMI) Default() {
	g.KeepAliveTimeout = 600
	g.MaxCalls = 8
}

func (g *GNMI) Validate() error {
	if g.KeepAliveTimeout < 600 || g.KeepAliveTimeout > 86400 {
		return fmt.Errorf("gnmi: invalid keepAliveTimeout %d: must be between 600 and 86400", g.KeepAliveTimeout)
	}
	if g.MaxCalls < 1 || g.MaxCalls > 16 {
		return fmt.Errorf("gnmi: invalid maxCalls %d: must be between 1 and 16", g.MaxCalls)
	}
	return nil
}
