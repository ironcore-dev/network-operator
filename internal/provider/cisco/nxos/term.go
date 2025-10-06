// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"fmt"

	"github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"
)

var (
	_ gnmiext.Configurable = (*Console)(nil)
	_ gnmiext.Defaultable  = (*Console)(nil)
	_ gnmiext.Configurable = (*VTY)(nil)
	_ gnmiext.Defaultable  = (*VTY)(nil)
	_ gnmiext.Configurable = (*VTYAccessClass)(nil)
)

// Console represents the primary terminal line configuration.
type Console struct {
	// Maximum time allowed for a command to execute in minutes.
	// Leave unspecified (zero) to disable.
	Timeout int `json:"timeout"`
}

func (*Console) XPath() string {
	return "System/terml-items/ln-items/cons-items/execTmeout-items"
}

func (c *Console) Default() {
	c.Timeout = 30
}

func (c *Console) Validate() error {
	if c.Timeout < 0 || c.Timeout > 525600 {
		return fmt.Errorf("console: invalid exec-timeout %d: must be between 0 and 525600", c.Timeout)
	}
	return nil
}

// VTY represents the virtual terminal line configuration.
type VTY struct {
	ExecTmeoutItems struct {
		// Maximum time allowed for a command to execute in minutes.
		// Leave unspecified (zero) to disable.
		Timeout int `json:"timeout"`
	} `json:"execTmeout-items"`
	SsLmtItems struct {
		// Maximum number of concurrent vsh sessions.
		SesLmt int `json:"sesLmt"`
	} `json:"ssLmt-items"`
}

func (*VTY) XPath() string {
	return "System/terml-items/ln-items/vty-items"
}

func (v *VTY) Default() {
	v.SsLmtItems.SesLmt = 32
	v.ExecTmeoutItems.Timeout = 30
}

func (v *VTY) Validate() error {
	if v.ExecTmeoutItems.Timeout < 0 || v.ExecTmeoutItems.Timeout > 525600 {
		return fmt.Errorf("vty: invalid exec-timeout %d: must be between 0 and 525600", v.ExecTmeoutItems.Timeout)
	}
	if v.SsLmtItems.SesLmt < 1 || v.SsLmtItems.SesLmt > 64 {
		return fmt.Errorf("vty: invalid session-limit %d: must be between 1 and 64", v.SsLmtItems.SesLmt)
	}
	return nil
}

// VTYAccessClass represents the access control list applied to packets.
type VTYAccessClass struct {
	// IPv4 access control list to be applied for packets.
	Name string `json:"name"`
}

func (v *VTYAccessClass) XPath() string {
	return "System/acl-items/ipv4-items/policy-items/ingress-items/vty-items/acl-items"
}
