// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"strconv"

	"github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"
)

var (
	_ gnmiext.Configurable = (*SNMPSysInfo)(nil)
	_ gnmiext.Configurable = (*SNMPSrcIf)(nil)
	_ gnmiext.Configurable = (*SNMPUser)(nil)
	_ gnmiext.Configurable = (*SNMPHostItems)(nil)
	_ gnmiext.Configurable = (*SNMPHost)(nil)
	_ gnmiext.Configurable = (*SNMPCommunityItems)(nil)
	_ gnmiext.Configurable = (*SNMPCommunity)(nil)
)

// SNMPSysInfo represents the SNMP system information configuration on a NX-OS device.
type SNMPSysInfo struct {
	SysContact  Option[string] `json:"sysContact"`
	SysLocation Option[string] `json:"sysLocation"`
}

func (*SNMPSysInfo) XPath() string {
	return "System/snmp-items/inst-items/sysinfo-items"
}

// SNMPSrcIf represents the SNMP source interface configuration on a NX-OS device.
type SNMPSrcIf struct {
	Type   MessageType    `json:"-"`
	Ifname Option[string] `json:"ifname"`
}

func (s *SNMPSrcIf) XPath() string {
	return "System/snmp-items/inst-items/globals-items/srcInterface" + string(s.Type) + "-items"
}

// SNMPUser represents an SNMP local user configuration on a NX-OS device.
type SNMPUser struct {
	Username    string         `json:"userName"`
	Ipv4AclName Option[string] `json:"ipv4AclName"`
}

func (*SNMPUser) IsListItem() {}

func (s *SNMPUser) XPath() string {
	return "System/snmp-items/inst-items/lclUser-items/LocalUser-list[userName=" + s.Username + "]"
}

// SNMPHostItems represents the SNMP host configuration on a NX-OS device.
type SNMPHostItems struct {
	HostList gnmiext.List[SNMPHostKey, *SNMPHost] `json:"Host-list,omitzero"`
}

func (*SNMPHostItems) XPath() string {
	return "System/snmp-items/inst-items/host-items"
}

type SNMPHostKey struct {
	HostName  string
	UDPPortID int
}

type SNMPHost struct {
	CommName    Option[string] `json:"commName"`
	HostName    string         `json:"hostName"`
	NotifType   string         `json:"notifType"`
	SecLevel    SecLevel       `json:"secLevel"`
	UDPPortID   int            `json:"udpPortID"`
	Version     string         `json:"version"`
	UsevrfItems struct {
		UseVrfList gnmiext.List[string, *SNMPHostVrf] `json:"UseVrf-list,omitzero"`
	} `json:"usevrf-items,omitzero"`
}

func (s *SNMPHost) Key() SNMPHostKey {
	return SNMPHostKey{HostName: s.HostName, UDPPortID: s.UDPPortID}
}

func (*SNMPHost) IsListItem() {}

func (s *SNMPHost) XPath() string {
	return "System/snmp-items/inst-items/host-items[hostName=" + s.HostName + "][udpPortID=" + strconv.Itoa(s.UDPPortID) + "]"
}

type SNMPHostVrf struct {
	Vrfname string `json:"vrfname,omitempty"`
}

func (s *SNMPHostVrf) Key() string { return s.Vrfname }

// SNMPCommunityItems represents the SNMP community configuration on a NX-OS device.
type SNMPCommunityItems struct {
	CommSecPList gnmiext.List[string, *SNMPCommunity] `json:"CommSecP-list,omitzero"`
}

func (*SNMPCommunityItems) XPath() string {
	return "System/snmp-items/inst-items/community-items"
}

type SNMPCommunity struct {
	CommAccess string `json:"commAcess"`
	GrpName    string `json:"grpName"`
	Name       string `json:"name"`
	ACLItems   struct {
		UseACLName string `json:"useAclName,omitempty"`
	} `json:"acl-items,omitzero"`
}

func (s *SNMPCommunity) Key() string { return s.Name }

func (*SNMPCommunity) IsListItem() {}

func (s *SNMPCommunity) XPath() string {
	return "System/snmp-items/inst-items/community-items/CommSecP-list[name=" + s.Name + "]"
}

type SNMPTrapsItems struct {
	AaaItems struct {
		ServerstatechangeItems *SNMPTraps `json:"serverstatechange-items,omitzero"`
	} `json:"aaa-items,omitzero"`
	BfdItems struct {
		SessiondownItems *SNMPTraps `json:"sessiondown-items,omitzero"`
		SessionupItems   *SNMPTraps `json:"sessionup-items,omitzero"`
	} `json:"bfd-items,omitzero"`
	BridgeItems struct {
		NewrootItems        *SNMPTraps `json:"newroot-items,omitzero"`
		TopologychangeItems *SNMPTraps `json:"topologychange-items,omitzero"`
	} `json:"bridge-items,omitzero"`
	CallhomeItems struct {
		EventnotifyItems  *SNMPTraps `json:"eventnotify-items,omitzero"`
		SmtpsendfailItems *SNMPTraps `json:"smtpsendfail-items,omitzero"`
	} `json:"callhome-items,omitzero"`
	CfsItems struct {
		MergefailureItems     *SNMPTraps `json:"mergefailure-items,omitzero"`
		StatechangenotifItems *SNMPTraps `json:"statechangenotif-items,omitzero"`
	} `json:"cfs-items,omitzero"`
	ConfigItems struct {
		CcmCLIRunningConfigChangedItems *SNMPTraps `json:"ccmCLIRunningConfigChanged-items,omitzero"`
	} `json:"config-items,omitzero"`
	EntityItems struct {
		CefcMIBEnableStatusNotificationItems *SNMPTraps `json:"cefcMIBEnableStatusNotification-items,omitzero"`
		EntityfanstatuschangeItems           *SNMPTraps `json:"entityfanstatuschange-items,omitzero"`
		EntitymibchangeItems                 *SNMPTraps `json:"entitymibchange-items,omitzero"`
		EntitymoduleinsertedItems            *SNMPTraps `json:"entitymoduleinserted-items,omitzero"`
		EntitymoduleremovedItems             *SNMPTraps `json:"entitymoduleremoved-items,omitzero"`
		EntitymodulestatuschangeItems        *SNMPTraps `json:"entitymodulestatuschange-items,omitzero"`
		EntitypoweroutchangeItems            *SNMPTraps `json:"entitypoweroutchange-items,omitzero"`
		EntitypowerstatuschangeItems         *SNMPTraps `json:"entitypowerstatuschange-items,omitzero"`
		EntitysensorItems                    *SNMPTraps `json:"entitysensor-items,omitzero"`
		EntityunrecognisedmoduleItems        *SNMPTraps `json:"entityunrecognisedmodule-items,omitzero"`
	} `json:"entity-items,omitzero"`
	FeaturecontrolItems struct {
		FeatureOpStatusChangeItems   *SNMPTraps `json:"FeatureOpStatusChange-items,omitzero"`
		CiscoFeatOpStatusChangeItems *SNMPTraps `json:"ciscoFeatOpStatusChange-items,omitzero"`
	} `json:"featurecontrol-items,omitzero"`
	GenericItems struct {
		ColdStartItems *SNMPTraps `json:"coldStart-items,omitzero"`
		WarmStartItems *SNMPTraps `json:"warmStart-items,omitzero"`
	} `json:"generic-items,omitzero"`
	HsrpItems struct {
		StatechangeItems *SNMPTraps `json:"statechange-items,omitzero"`
	} `json:"hsrp-items,omitzero"`
	IPItems struct {
		SLAItems *SNMPTraps `json:"sla-items,omitzero"`
	} `json:"ip-items,omitzero"`
	LicenseItems struct {
		NotifylicenseexpiryItems        *SNMPTraps `json:"notifylicenseexpiry-items,omitzero"`
		NotifylicenseexpirywarningItems *SNMPTraps `json:"notifylicenseexpirywarning-items,omitzero"`
		NotifylicensefilemissingItems   *SNMPTraps `json:"notifylicensefilemissing-items,omitzero"`
		NotifynolicenseforfeatureItems  *SNMPTraps `json:"notifynolicenseforfeature-items,omitzero"`
	} `json:"license-items,omitzero"`
	LinkItems struct {
		CieLinkDownItems            *SNMPTraps `json:"cieLinkDown-items,omitzero"`
		CieLinkUpItems              *SNMPTraps `json:"cieLinkUp-items,omitzero"`
		CiscoxcvrmonstatuschgItems  *SNMPTraps `json:"ciscoxcvrmonstatuschg-items,omitzero"`
		DelayedlinkstatechangeItems *SNMPTraps `json:"delayedlinkstatechange-items,omitzero"`
		ExtendedlinkDownItems       *SNMPTraps `json:"extendedlinkDown-items,omitzero"`
		ExtendedlinkUpItems         *SNMPTraps `json:"extendedlinkUp-items,omitzero"`
		LinkDownItems               *SNMPTraps `json:"linkDown-items,omitzero"`
		LinkUpItems                 *SNMPTraps `json:"linkUp-items,omitzero"`
	} `json:"link-items,omitzero"`
	LldpItems struct {
		LldpRemTablesChangeItems *SNMPTraps `json:"lldpRemTablesChange-items,omitzero"`
	} `json:"lldp-items,omitzero"`
	MmodeItems struct {
		CseMaintModeChangeNotifyItems  *SNMPTraps `json:"cseMaintModeChangeNotify-items,omitzero"`
		CseNormalModeChangeNotifyItems *SNMPTraps `json:"cseNormalModeChangeNotify-items,omitzero"`
	} `json:"mmode-items,omitzero"`
	MsdpItems struct {
		MsdpBackwardTransitionItems *SNMPTraps `json:"msdpBackwardTransition-items,omitzero"`
	} `json:"msdp-items,omitzero"`
	PimItems struct {
		PimNeighborLossItems *SNMPTraps `json:"pimNeighborLoss-items,omitzero"`
	} `json:"pim-items,omitzero"`
	PoeItems struct {
		ControlenableItems *SNMPTraps `json:"controlenable-items,omitzero"`
		PolicenotifyItems  *SNMPTraps `json:"policenotify-items,omitzero"`
	} `json:"poe-items,omitzero"`
	PortsecurityItems struct {
		AccesssecuremacviolationItems *SNMPTraps `json:"accesssecuremacviolation-items,omitzero"`
		TrunksecuremacviolationItems  *SNMPTraps `json:"trunksecuremacviolation-items,omitzero"`
	} `json:"portsecurity-items,omitzero"`
	RfItems struct {
		RedundancyframeworkItems *SNMPTraps `json:"redundancyframework-items,omitzero"`
	} `json:"rf-items,omitzero"`
	RmonItems struct {
		FallingAlarmItems   *SNMPTraps `json:"fallingAlarm-items,omitzero"`
		HcFallingAlarmItems *SNMPTraps `json:"hcFallingAlarm-items,omitzero"`
		HcRisingAlarmItems  *SNMPTraps `json:"hcRisingAlarm-items,omitzero"`
		RisingAlarmItems    *SNMPTraps `json:"risingAlarm-items,omitzero"`
	} `json:"rmon-items,omitzero"`
	SnmpItems struct {
		AuthenticationItems *SNMPTraps `json:"authentication-items,omitzero"`
	} `json:"snmp-items,omitzero"`
	StormcontrolItems struct {
		CpscEventRev1Items *SNMPTraps `json:"cpscEventRev1-items,omitzero"`
	} `json:"stormcontrol-items,omitzero"`
	StpxItems struct {
		InconsistencyItems     *SNMPTraps `json:"inconsistency-items,omitzero"`
		LoopinconsistencyItems *SNMPTraps `json:"loopinconsistency-items,omitzero"`
		RootinconsistencyItems *SNMPTraps `json:"rootinconsistency-items,omitzero"`
	} `json:"stpx-items,omitzero"`
	SysmgrItems struct {
		CseFailSwCoreNotifyExtendedItems *SNMPTraps `json:"cseFailSwCoreNotifyExtended-items,omitzero"`
	} `json:"sysmgr-items,omitzero"`
	SystemItems struct {
		ClockchangenotificationItems *SNMPTraps `json:"Clockchangenotification-items,omitzero"`
	} `json:"system-items,omitzero"`
	UpgradeItems struct {
		UpgradeJobStatusNotifyItems *SNMPTraps `json:"UpgradeJobStatusNotify-items,omitzero"`
	} `json:"upgrade-items,omitzero"`
	VsanItems struct {
		VsanPortMembershipChangeItems *SNMPTraps `json:"vsanPortMembershipChange-items,omitzero"`
		VsanStatusChangeItems         *SNMPTraps `json:"vsanStatusChange-items,omitzero"`
	} `json:"vsan-items,omitzero"`
	VtpItems struct {
		NotifsItems     *SNMPTraps `json:"notifs-items,omitzero"`
		VlancreateItems *SNMPTraps `json:"vlancreate-items,omitzero"`
		VlandeleteItems *SNMPTraps `json:"vlandelete-items,omitzero"`
	} `json:"vtp-items,omitzero"`
}

func (*SNMPTrapsItems) XPath() string {
	return "System/snmp-items/inst-items/traps-items"
}

type SNMPTraps struct {
	Trapstatus Trapstatus `json:"trapstatus"`
}

type MessageType string

const (
	Informs MessageType = "Informs"
	Traps   MessageType = "Traps"
)

type SecLevel string

const (
	SecLevelNoAuth SecLevel = "noauth"
	SecLevelAuth   SecLevel = "auth"
)

type Trapstatus string

const (
	TrapstatusEnable  Trapstatus = "enable"
	TrapstatusDisable Trapstatus = "disable"
)
