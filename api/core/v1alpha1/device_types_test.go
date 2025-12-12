// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDevice_GetActiveProvisioning(t *testing.T) {
	now := metav1.Now()
	oneHourAgo := metav1.NewTime(now.Add(-1 * time.Hour))
	twoHoursAgo := metav1.NewTime(now.Add(-2 * time.Hour))

	tests := []struct {
		name   string
		device *Device
		want   *ProvisioningInfo
	}{
		{
			name: "no provisioning entries",
			device: &Device{
				Status: DeviceStatus{
					Provisioning: []ProvisioningInfo{},
				},
			},
			want: nil,
		},
		{
			name: "single active provisioning entry",
			device: &Device{
				Status: DeviceStatus{
					Provisioning: []ProvisioningInfo{
						{
							StartTime: now,
							Token:     "token1",
						},
					},
				},
			},
			want: &ProvisioningInfo{
				StartTime: now,
				Token:     "token1",
			},
		},
		{
			name: "single completed provisioning entry",
			device: &Device{
				Status: DeviceStatus{
					Provisioning: []ProvisioningInfo{
						{
							StartTime: twoHoursAgo,
							EndTime:   oneHourAgo,
							Token:     "token1",
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "multiple completed provisioning entries",
			device: &Device{
				Status: DeviceStatus{
					Provisioning: []ProvisioningInfo{
						{
							StartTime: metav1.NewTime(now.Add(-3 * time.Hour)),
							EndTime:   twoHoursAgo,
							Token:     "token1",
						},
						{
							StartTime: twoHoursAgo,
							EndTime:   oneHourAgo,
							Token:     "token2",
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "active provisioning",
			device: &Device{
				Status: DeviceStatus{
					Provisioning: []ProvisioningInfo{
						{
							StartTime: now,
							Token:     "token-active",
						},
						{
							StartTime: twoHoursAgo,
							EndTime:   oneHourAgo,
							Token:     "token-completed",
						},
					},
				},
			},
			want: &ProvisioningInfo{
				StartTime: now,
				Token:     "token-active",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.device.GetActiveProvisioning()
			if tt.want == nil {
				if got != nil {
					t.Errorf("GetActiveProvisioning() = %v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Fatalf("GetActiveProvisioning() = nil, want non-nil")
			}

			if got.Token != tt.want.Token {
				t.Errorf("GetActiveProvisioning() token = %v, want %v", got.Token, tt.want.Token)
			}
			if !got.StartTime.Equal(&tt.want.StartTime) {
				t.Errorf("GetActiveProvisioning() startTime = %v, want %v", got.StartTime, tt.want.StartTime)
			}
			if !got.EndTime.IsZero() {
				t.Errorf("GetActiveProvisioning() endTime = %v, want zero", got.EndTime)
			}
		})
	}
}

func TestDevice_CreateProvisioningEntry(t *testing.T) {
	now := metav1.Now()
	oneHourAgo := metav1.NewTime(now.Add(-1 * time.Hour))
	twoHoursAgo := metav1.NewTime(now.Add(-2 * time.Hour))

	tests := []struct {
		name            string
		device          *Device
		wantErr         bool
		expectedEntries int
	}{
		{
			name: "successful creation in provisioning phase with no existing entries",
			device: &Device{
				Status: DeviceStatus{
					Phase:        DevicePhaseProvisioning,
					Provisioning: []ProvisioningInfo{},
				},
			},
			wantErr:         false,
			expectedEntries: 1,
		},
		{
			name: "successful creation with completed provisioning entries",
			device: &Device{
				Status: DeviceStatus{
					Phase: DevicePhaseProvisioning,
					Provisioning: []ProvisioningInfo{
						{
							StartTime: twoHoursAgo,
							EndTime:   oneHourAgo,
							Token:     "old-token",
						},
					},
				},
			},
			wantErr:         false,
			expectedEntries: 2,
		},
		{
			name: "error when device is in pending phase",
			device: &Device{
				Status: DeviceStatus{
					Phase:        DevicePhasePending,
					Provisioning: []ProvisioningInfo{},
				},
			},
			expectedEntries: 0,
			wantErr:         true,
		},
		{
			name: "error when active provisioning already exists",
			device: &Device{
				Status: DeviceStatus{
					Phase: DevicePhaseProvisioning,
					Provisioning: []ProvisioningInfo{
						{
							StartTime: now,
							Token:     "active-token",
						},
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, err := tt.device.CreateProvisioningEntry()
			if (err != nil) != tt.wantErr {
				t.Fatalf("CreateProvisioningEntry() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if entry == nil {
				t.Fatal("expected non-nil entry")
			}
			if len(tt.device.Status.Provisioning) != tt.expectedEntries {
				t.Errorf("expected %d provisioning entries, got %d", tt.expectedEntries, len(tt.device.Status.Provisioning))
			}
			if entry.Token == "" {
				t.Error("expected non-empty token")
			}
			if len(entry.Token) != 64 { // 32 bytes hex encoded = 64 chars
				t.Errorf("expected token length 64, got %d", len(entry.Token))
			}
			if entry.StartTime.IsZero() {
				t.Error("expected non-zero start time")
			}
			if !entry.EndTime.IsZero() {
				t.Error("expected zero end time")
			}
		})
	}
}
