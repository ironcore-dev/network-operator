// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iosxr

import "strconv"

// IPSLAFrequency is the probe frequency in seconds configured for IPSLA ICMP
// echo operations backing static route tracking.
const IPSLAFrequency = 5

func (o *IPSLAOperation) XPath() string {
	return "Cisco-IOS-XR-um-ipsla-cfg:ipsla/operations/operation[operation-number=" + strconv.Itoa(int(o.OperationNumber)) + "]"
}

func (s *IPSLASchedule) XPath() string {
	return "Cisco-IOS-XR-um-ipsla-cfg:ipsla/schedule/operations/operation[operation-number=" + strconv.Itoa(int(s.OperationNumber)) + "]"
}

type IPSLAOperation struct {
	OperationNumber uint32              `json:"operation-number"`
	Type            *IPSLAOperationType `json:"type,omitempty"`
}

type IPSLAOperationType struct {
	ICMP *IPSLAICMP `json:"icmp,omitempty"`
}

type IPSLAICMP struct {
	Echo *IPSLAEcho `json:"echo,omitempty"`
}

type IPSLAEcho struct {
	Destination *IPSLADestination `json:"destination,omitempty"`
	Frequency   uint32            `json:"frequency,omitempty"`
	VRF         string            `json:"vrf,omitempty"`
}

type IPSLADestination struct {
	Address IPSLAAddress `json:"address"`
}

type IPSLAAddress struct {
	IPv4Address string `json:"ipv4-address"`
}

type IPSLASchedule struct {
	OperationNumber uint32          `json:"operation-number"`
	Life            *IPSLALife      `json:"life,omitempty"`
	StartTime       *IPSLAStartTime `json:"start-time,omitempty"`
}

type IPSLALife struct {
	Forever *struct{} `json:"forever,omitempty"`
}

type IPSLAStartTime struct {
	Now *struct{} `json:"now,omitempty"`
}
