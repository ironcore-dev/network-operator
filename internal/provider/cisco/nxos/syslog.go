// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"github.com/ironcore-dev/network-operator/api/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"
)

var (
	_ gnmiext.Configurable = (*SyslogOrigin)(nil)
	_ gnmiext.Configurable = (*SyslogSrcIf)(nil)
	_ gnmiext.Configurable = (*SyslogHistory)(nil)
	_ gnmiext.Configurable = (*SyslogRemoteItems)(nil)
	_ gnmiext.Configurable = (*SyslogFacilityItems)(nil)
)

type SyslogOrigin struct {
	Idtype  string `json:"idtype"`
	Idvalue string `json:"idvalue,omitempty"`
}

func (*SyslogOrigin) XPath() string {
	return "System/syslog-items/originid-items"
}

type SyslogSrcIf struct {
	AdminSt AdminSt `json:"adminState"`
	IfName  string  `json:"ifName"`
}

func (*SyslogSrcIf) XPath() string {
	return "System/syslog-items/source-items"
}

type SyslogHistory struct {
	Level SeverityLevel `json:"level"`
	Size  uint32        `json:"size"`
}

func (*SyslogHistory) XPath() string {
	return "System/syslog-items/logginghistory-items"
}

type SyslogRemoteItems struct {
	RemoteDestList []*SyslogRemote `json:"RemoteDest-list"`
}

func (*SyslogRemoteItems) XPath() string {
	return "System/syslog-items/rdst-items"
}

type SyslogRemote struct {
	ForwardingFacility string        `json:"forwardingFacility"`
	Host               string        `json:"host"`
	Port               int32         `json:"port"`
	Severity           SeverityLevel `json:"severity"`
	Transport          Transport     `json:"transport"`
	VrfName            string        `json:"vrfName"`
}

type SyslogFacilityItems struct {
	FacilityList []*SyslogFacility `json:"Facility-list"`
}

func (*SyslogFacilityItems) XPath() string {
	return "System/logging-items/loglevel-items/facility-items"
}

type SyslogFacility struct {
	FacilityName  string        `json:"facilityName"`
	SeverityLevel SeverityLevel `json:"severityLevel"`
}

func (s *SyslogFacility) IsListItem() {}

func (s *SyslogFacility) XPath() string {
	return "System/logging-items/loglevel-items/facility-items[facilityName=" + s.FacilityName + "]"
}

type Transport string

const (
	TransportUDP Transport = "udp"
	TransportTCP Transport = "tcp"
)

type SeverityLevel string

const (
	Emergency     SeverityLevel = "emergencies"
	Alert         SeverityLevel = "alerts"
	Critical      SeverityLevel = "critical"
	Error         SeverityLevel = "errors"
	Warning       SeverityLevel = "warnings"
	Notice        SeverityLevel = "notifications"
	Informational SeverityLevel = "information"
	Debug         SeverityLevel = "debugging"
)

func SeverityLevelFrom(v v1alpha1.Severity) SeverityLevel {
	switch v {
	case v1alpha1.SeverityEmergency:
		return Emergency
	case v1alpha1.SeverityAlert:
		return Alert
	case v1alpha1.SeverityCritical:
		return Critical
	case v1alpha1.SeverityError:
		return Error
	case v1alpha1.SeverityWarning:
		return Warning
	case v1alpha1.SeverityNotice:
		return Notice
	case v1alpha1.SeverityInfo:
		return Informational
	case v1alpha1.SeverityDebug:
		return Debug
	default:
		return Informational
	}
}
