// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	"github.com/openconfig/gnoi/factory_reset"
	"github.com/openconfig/gnoi/system"

	"github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"
)

const Manufacturer = "Cisco"

var (
	_ gnmiext.Configurable = (*SystemJumboMTU)(nil)
	_ gnmiext.Defaultable  = (*SystemJumboMTU)(nil)
	_ gnmiext.Configurable = (*Model)(nil)
	_ gnmiext.Configurable = (*SerialNumber)(nil)
	_ gnmiext.Configurable = (*FirmwareVersion)(nil)
)

// SystemJumboMTU represents the jumbo MTU size configured on the system.
type SystemJumboMTU int16

func (s *SystemJumboMTU) XPath() string {
	return "System/ethpm-items/inst-items/systemJumboMtu"
}

func (s *SystemJumboMTU) Default() {
	*s = 9216
}

// Model is the chassis model of the device, e.g. "N9K-C9336C-FX2".
type Model string

func (*Model) XPath() string {
	return "System/ch-items/model"
}

// SerialNumber is the serial number of the device, e.g. "9VT9OHZBC3H".
// This value should typically match the serial number under "System/serial".
type SerialNumber string

func (*SerialNumber) XPath() string {
	return "System/ch-items/ser"
}

// FirmwareVersion is the firmware version of the device, e.g. "10.4(3)".
type FirmwareVersion string

func (*FirmwareVersion) XPath() string {
	return "System/showversion-items/nxosVersion"
}

var _ gnmiext.Configurable = (*BootPOAP)(nil)

type BootPOAP string

func (*BootPOAP) XPath() string {
	return "/System/boot-items/poap"
}

func Reboot(ctx context.Context, conn *grpc.ClientConn) error {
	request := system.RebootRequest{
		Method: system.RebootMethod_COLD,
		// Message is not supported on NXOS
		// Delay is not supported on NXOS
		Force: true, // only Force true is supported
	}
	c := system.NewSystemClient(conn)
	_, err := c.Reboot(ctx, &request, grpc.WaitForReady(true))
	return err
}

func ResetToFactoryDefaults(ctx context.Context, conn *grpc.ClientConn) error {
	request := factory_reset.StartRequest{
		// True not supported on NXOS, NXOS makes sure running OS is preserved
		FactoryOs:   false,
		ZeroFill:    true,
		RetainCerts: false,
	}
	c := factory_reset.NewFactoryResetClient(conn)
	response, err := c.Start(ctx, &request, grpc.WaitForReady(true))
	if err != nil {
		return err
	}
	resetError := response.GetResetError()
	if resetError != nil {
		return fmt.Errorf("factory reset failed: %s", resetError.String())
	}
	return nil
}
