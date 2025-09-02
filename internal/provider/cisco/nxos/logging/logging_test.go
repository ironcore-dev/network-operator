// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package logging

import (
	"reflect"
	"testing"

	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

// tests for the ToYGOT method, no special cases are considered, like empty values
func TestToYGOT(t *testing.T) {
	loggingConfig := &Logging{
		Enable:   true,
		OriginID: "vrouter",
		SrcIf:    "mgmt0",
		History: History{
			Severity: Warning,
			Size:     389,
		},
		Servers: []*SyslogServer{
			{
				Host:  "147.204.192.125",
				Port:  514,
				Proto: UDP,
				Vrf:   "management",
				Level: Debug,
			},
			{
				Host:  "147.204.192.118",
				Port:  1514,
				Proto: TCP,
				Vrf:   "management",
				Level: Debug,
			},
		},
		DefaultSeverity: Warning,
	}

	got, err := loggingConfig.ToYGOT(t.Context(), &gnmiext.ClientMock{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tests := []struct {
		name     string
		xpath    string
		expected ygot.GoStruct
	}{
		{
			name:  "SourceItems",
			xpath: "System/syslog-items/source-items",
			expected: &nxos.Cisco_NX_OSDevice_System_SyslogItems_SourceItems{
				AdminState: nxos.Cisco_NX_OSDevice_Mon_AdminState_enabled,
				IfName:     ygot.String(loggingConfig.SrcIf),
			},
		},
		{
			name:  "OriginIDItems",
			xpath: "System/syslog-items/originid-items",
			expected: &nxos.Cisco_NX_OSDevice_System_SyslogItems_OriginidItems{
				Idvalue: ygot.String(loggingConfig.OriginID),
			},
		},
		{
			name:  "LoggingHistoryItems",
			xpath: "System/syslog-items/logginghistory-items",
			expected: &nxos.Cisco_NX_OSDevice_System_SyslogItems_LogginghistoryItems{
				Level: nxos.E_Cisco_NX_OSDevice_Syslog_Severity(Warning),
				Size:  ygot.Uint32(389),
			},
		},
		{
			name:  "RemoteSyslogServersFull",
			xpath: "System/syslog-items/rdst-items",
			expected: &nxos.Cisco_NX_OSDevice_System_SyslogItems_RdstItems{
				RemoteDestList: map[string]*nxos.Cisco_NX_OSDevice_System_SyslogItems_RdstItems_RemoteDestList{
					"147.204.192.125": {
						Severity:  nxos.E_Cisco_NX_OSDevice_Syslog_Severity(Debug),
						VrfName:   ygot.String("management"),
						Host:      ygot.String("147.204.192.125"),
						Port:      ygot.Uint32(514),
						Transport: nxos.Cisco_NX_OSDevice_Mon_Transport_udp,
					},
					"147.204.192.118": {
						Severity:  nxos.E_Cisco_NX_OSDevice_Syslog_Severity(Debug),
						VrfName:   ygot.String("management"),
						Host:      ygot.String("147.204.192.118"),
						Port:      ygot.Uint32(1514),
						Transport: nxos.Cisco_NX_OSDevice_Mon_Transport_tcp,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var found bool
			for _, update := range got {
				u, ok := update.(gnmiext.EditingUpdate)
				if ok {
					if u.XPath == tt.xpath {
						found = true
						if !reflect.DeepEqual(u.Value, tt.expected) {
							t.Errorf("for XPath %s, expected %+v, got %+v", tt.xpath, tt.expected, u.Value)
						}
						break
					}
				} else {
					u, ok := update.(gnmiext.ReplacingUpdate)
					if ok {
						if u.XPath == tt.xpath {
							found = true
							if !reflect.DeepEqual(u.Value, tt.expected) {
								t.Errorf("for XPath %s, expected %+v, got %+v", tt.xpath, tt.expected, u.Value)
							}
							break
						}
					}
				}
			}
			if !found {
				t.Errorf("XPath %s not found in updates", tt.xpath)
			}
		})
	}
}

func TestToYGOT_Facilities(t *testing.T) {
	l := &Logging{
		Enable:          true,
		DefaultSeverity: Warning,
		Facilities: []*Facility{
			{
				Name:     "aaa",
				Severity: Emergency,
			},
		},
	}

	got, err := l.ToYGOT(t.Context(), &gnmiext.ClientMock{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedFacilities := make(map[string]nxos.E_Cisco_NX_OSDevice_Syslog_Severity)
	for _, f := range AvailableFacilities {
		expectedFacilities[f] = nxos.E_Cisco_NX_OSDevice_Syslog_Severity(l.DefaultSeverity)
	}
	expectedFacilities["aaa"] = nxos.E_Cisco_NX_OSDevice_Syslog_Severity(Emergency)

	var loggingItems *nxos.Cisco_NX_OSDevice_System_LoggingItems
	for _, update := range got {
		u, ok := update.(gnmiext.EditingUpdate)
		if !ok {
			t.Errorf("expected value to be of type EditingUpdate")
		}
		if u.XPath == "System/logging-items" {
			var ok bool
			loggingItems, ok = u.Value.(*nxos.Cisco_NX_OSDevice_System_LoggingItems)
			if !ok {
				t.Fatalf("expected value to be of type *nxos.Cisco_NX_OSDevice_System_LoggingItems")
			}
			break
		}
	}
	if loggingItems == nil {
		t.Fatalf("expected key 'System/logging-items' to be present")
		return
	}

	if loggingItems.LoglevelItems == nil || loggingItems.LoglevelItems.FacilityItems == nil {
		t.Fatalf("expected FacilityItems to be present")
		return
	}

	for facilityName, expectedSeverity := range expectedFacilities {
		facility, ok := loggingItems.LoglevelItems.FacilityItems.FacilityList[facilityName]
		if !ok {
			t.Errorf("expected facility '%s' to be present", facilityName)
			continue
		}
		if facility.SeverityLevel != expectedSeverity {
			t.Errorf("expected severity level for facility '%s' to be '%v', got '%v'", facilityName, expectedSeverity, facility.SeverityLevel)
		}
	}
}
