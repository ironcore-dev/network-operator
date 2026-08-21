// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"google.golang.org/grpc"

	factoryresetpb "github.com/openconfig/gnoi/factory_reset"
	systempb "github.com/openconfig/gnoi/system"

	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

const Manufacturer = "Cisco"

var (
	_ gnmiext.DataElement = (*SystemJumboMTU)(nil)
	_ gnmiext.Defaultable = (*SystemJumboMTU)(nil)
	_ gnmiext.DataElement = (*Model)(nil)
	_ gnmiext.DataElement = (*SerialNumber)(nil)
	_ gnmiext.DataElement = (*FirmwareVersion)(nil)
)

// SystemJumboMTU represents the jumbo MTU size configured on the system.
type SystemJumboMTU int16

func (s *SystemJumboMTU) XPath() string {
	return "System/ethpm-items/inst-items/systemJumboMtu"
}

func (s *SystemJumboMTU) Default() {
	*s = 9216
}

// Hostname is the configured hostname of the device.
type Hostname string

func (*Hostname) XPath() string {
	return "System/name"
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

type BootTime UnixTime

func (*BootTime) XPath() string {
	return "System/procsys-items/bootTime"
}

func (t *BootTime) UnmarshalJSON(b []byte) error {
	return (*UnixTime)(t).UnmarshalJSON(b)
}

// UnixTime is a wrapper around time.Time that marshals/unmarshals to/from a Unix timestamp in seconds.
type UnixTime struct {
	time.Time `json:"-"`
}

// UnmarshalJSON implements json.Unmarshaler.
func (t *UnixTime) UnmarshalJSON(b []byte) error {
	var unix int64
	if err := json.Unmarshal(b, &unix); err != nil {
		var str string
		if err := json.Unmarshal(b, &str); err != nil {
			return fmt.Errorf("failed to unmarshal UnixTime: %w", err)
		}
		unix, err = strconv.ParseInt(str, 10, 64)
		if err != nil {
			return fmt.Errorf("failed to parse UnixTime string: %w", err)
		}
	}
	t.Time = time.Unix(unix, 0)
	return nil
}

func Reboot(ctx context.Context, conn *grpc.ClientConn) error {
	req := &systempb.RebootRequest{
		Method:  systempb.RebootMethod_COLD,
		Delay:   0,  // Unsupported on NX-OS, must be 0
		Message: "", // Unsupported on NX-OS, must be empty
		Force:   true,
	}
	c := systempb.NewSystemClient(conn)
	_, err := c.Reboot(ctx, req, grpc.WaitForReady(true))
	return err
}

func FactoryReset(ctx context.Context, conn *grpc.ClientConn) error {
	req := &factoryresetpb.StartRequest{
		FactoryOs:   false, // NX-OS does not support factory OS reset, it always ensures the running OS is preserved
		ZeroFill:    true,
		RetainCerts: false,
	}
	c := factoryresetpb.NewFactoryResetClient(conn)
	res, err := c.Start(ctx, req, grpc.WaitForReady(true))
	if err != nil {
		return err
	}
	if resetErr := res.GetResetError(); resetErr != nil {
		return fmt.Errorf("factory reset failed: %s", resetErr.String())
	}
	return nil
}
