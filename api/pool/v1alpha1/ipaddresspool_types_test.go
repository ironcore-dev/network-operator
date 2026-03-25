// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"math/big"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	corev1alpha1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

func TestIPAddressPool_Total(t *testing.T) {
	tests := []struct {
		name string
		pool IPAddressPool
		want *big.Int
	}{
		{
			name: "empty prefixes",
			pool: IPAddressPool{
				Spec: IPAddressPoolSpec{
					Prefixes: []corev1alpha1.IPPrefix{},
				},
			},
			want: big.NewInt(0),
		},
		{
			name: "single IPv4 /24 prefix",
			pool: IPAddressPool{
				Spec: IPAddressPoolSpec{
					Prefixes: []corev1alpha1.IPPrefix{
						corev1alpha1.MustParsePrefix("192.168.1.0/24"),
					},
				},
			},
			want: big.NewInt(256),
		},
		{
			name: "single IPv4 /32 prefix",
			pool: IPAddressPool{
				Spec: IPAddressPoolSpec{
					Prefixes: []corev1alpha1.IPPrefix{
						corev1alpha1.MustParsePrefix("192.168.1.1/32"),
					},
				},
			},
			want: big.NewInt(1),
		},
		{
			name: "multiple IPv4 prefixes",
			pool: IPAddressPool{
				Spec: IPAddressPoolSpec{
					Prefixes: []corev1alpha1.IPPrefix{
						corev1alpha1.MustParsePrefix("192.168.1.0/24"),
						corev1alpha1.MustParsePrefix("10.0.0.0/24"),
						corev1alpha1.MustParsePrefix("172.16.0.0/28"),
					},
				},
			},
			want: big.NewInt(528),
		},
		{
			name: "single IPv6 /64 prefix",
			pool: IPAddressPool{
				Spec: IPAddressPoolSpec{
					Prefixes: []corev1alpha1.IPPrefix{
						corev1alpha1.MustParsePrefix("2001:db8::/64"),
					},
				},
			},
			want: new(big.Int).Lsh(big.NewInt(1), 64),
		},
		{
			name: "single IPv6 /128 prefix",
			pool: IPAddressPool{
				Spec: IPAddressPoolSpec{
					Prefixes: []corev1alpha1.IPPrefix{
						corev1alpha1.MustParsePrefix("2001:db8::1/128"),
					},
				},
			},
			want: big.NewInt(1),
		},
		{
			name: "mixed IPv4 and IPv6 prefixes",
			pool: IPAddressPool{
				Spec: IPAddressPoolSpec{
					Prefixes: []corev1alpha1.IPPrefix{
						corev1alpha1.MustParsePrefix("192.168.1.0/24"),
						corev1alpha1.MustParsePrefix("2001:db8::/126"),
					},
				},
			},
			want: big.NewInt(260),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.pool.Total(); got.Cmp(test.want) != 0 {
				t.Errorf("Total() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestIPAddressPool_Allocated(t *testing.T) {
	tests := []struct {
		name string
		pool IPAddressPool
		want int
	}{
		{
			name: "no allocations",
			pool: IPAddressPool{
				Status: IPAddressPoolStatus{
					Allocations: []IPAddressAllocation{},
				},
			},
			want: 0,
		},
		{
			name: "single allocation",
			pool: IPAddressPool{
				Status: IPAddressPoolStatus{
					Allocations: []IPAddressAllocation{
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "claim-1"},
							ClaimUID: types.UID("uid-1"),
							Address:  "192.168.1.1",
						},
					},
				},
			},
			want: 1,
		},
		{
			name: "multiple allocations",
			pool: IPAddressPool{
				Status: IPAddressPoolStatus{
					Allocations: []IPAddressAllocation{
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "claim-1"},
							ClaimUID: types.UID("uid-1"),
							Address:  "192.168.1.1",
						},
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "claim-2"},
							ClaimUID: types.UID("uid-2"),
							Address:  "192.168.1.2",
						},
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "claim-3"},
							ClaimUID: types.UID("uid-3"),
							Address:  "192.168.1.3",
						},
					},
				},
			},
			want: 3,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.pool.Allocated(); got != test.want {
				t.Errorf("Allocated() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestIPAddressPool_IsExhausted(t *testing.T) {
	tests := []struct {
		name string
		pool IPAddressPool
		want bool
	}{
		{
			name: "empty pool - exhausted",
			pool: IPAddressPool{
				Spec: IPAddressPoolSpec{
					Prefixes: []corev1alpha1.IPPrefix{},
				},
				Status: IPAddressPoolStatus{
					Allocations: []IPAddressAllocation{},
				},
			},
			want: true,
		},
		{
			name: "no allocations - not exhausted",
			pool: IPAddressPool{
				Spec: IPAddressPoolSpec{
					Prefixes: []corev1alpha1.IPPrefix{
						corev1alpha1.MustParsePrefix("192.168.1.0/30"),
					},
				},
				Status: IPAddressPoolStatus{
					Allocations: []IPAddressAllocation{},
				},
			},
			want: false,
		},
		{
			name: "partially allocated - not exhausted",
			pool: IPAddressPool{
				Spec: IPAddressPoolSpec{
					Prefixes: []corev1alpha1.IPPrefix{
						corev1alpha1.MustParsePrefix("192.168.1.0/30"),
					},
				},
				Status: IPAddressPoolStatus{
					Allocations: []IPAddressAllocation{
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "claim-1"},
							ClaimUID: types.UID("uid-1"),
							Address:  "192.168.1.1",
						},
					},
				},
			},
			want: false,
		},
		{
			name: "fully allocated - exhausted",
			pool: IPAddressPool{
				Spec: IPAddressPoolSpec{
					Prefixes: []corev1alpha1.IPPrefix{
						corev1alpha1.MustParsePrefix("192.168.1.0/30"),
					},
				},
				Status: IPAddressPoolStatus{
					Allocations: []IPAddressAllocation{
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "claim-1"},
							ClaimUID: types.UID("uid-1"),
							Address:  "192.168.1.0",
						},
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "claim-2"},
							ClaimUID: types.UID("uid-2"),
							Address:  "192.168.1.1",
						},
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "claim-3"},
							ClaimUID: types.UID("uid-3"),
							Address:  "192.168.1.2",
						},
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "claim-4"},
							ClaimUID: types.UID("uid-4"),
							Address:  "192.168.1.3",
						},
					},
				},
			},
			want: true,
		},
		{
			name: "over-allocated - exhausted",
			pool: IPAddressPool{
				Spec: IPAddressPoolSpec{
					Prefixes: []corev1alpha1.IPPrefix{
						corev1alpha1.MustParsePrefix("192.168.1.0/31"),
					},
				},
				Status: IPAddressPoolStatus{
					Allocations: []IPAddressAllocation{
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "claim-1"},
							ClaimUID: types.UID("uid-1"),
							Address:  "192.168.1.0",
						},
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "claim-2"},
							ClaimUID: types.UID("uid-2"),
							Address:  "192.168.1.1",
						},
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "claim-3"},
							ClaimUID: types.UID("uid-3"),
							Address:  "192.168.1.2",
						},
					},
				},
			},
			want: true,
		},
		{
			name: "IPv6 pool - not exhausted",
			pool: IPAddressPool{
				Spec: IPAddressPoolSpec{
					Prefixes: []corev1alpha1.IPPrefix{
						corev1alpha1.MustParsePrefix("2001:db8::/126"),
					},
				},
				Status: IPAddressPoolStatus{
					Allocations: []IPAddressAllocation{
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "claim-1"},
							ClaimUID: types.UID("uid-1"),
							Address:  "2001:db8::1",
						},
					},
				},
			},
			want: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.pool.IsExhausted(); got != test.want {
				t.Errorf("IsExhausted() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestIPAddressPool_FindAllocation(t *testing.T) {
	tests := []struct {
		name  string
		pool  IPAddressPool
		claim Claim
		want  *ClaimAllocation
	}{
		{
			name: "empty allocations returns nil",
			pool: IPAddressPool{
				Status: IPAddressPoolStatus{
					Allocations: []IPAddressAllocation{},
				},
			},
			claim: Claim{ObjectMeta: metav1.ObjectMeta{Name: "c1", UID: "uid1"}},
			want:  nil,
		},
		{
			name: "matching claim returns allocation",
			pool: IPAddressPool{
				Status: IPAddressPoolStatus{
					Allocations: []IPAddressAllocation{
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "c1"},
							ClaimUID: types.UID("uid1"),
							Address:  "10.0.0.1",
						},
					},
				},
			},
			claim: Claim{ObjectMeta: metav1.ObjectMeta{Name: "c1", UID: "uid1"}},
			want:  &ClaimAllocation{IPAddress: new("10.0.0.1"), Value: "10.0.0.1"},
		},
		{
			name: "different claim name returns nil",
			pool: IPAddressPool{
				Status: IPAddressPoolStatus{
					Allocations: []IPAddressAllocation{
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "c1"},
							ClaimUID: types.UID("uid1"),
							Address:  "10.0.0.1",
						},
					},
				},
			},
			claim: Claim{ObjectMeta: metav1.ObjectMeta{Name: "other", UID: "uid1"}},
			want:  nil,
		},
		{
			name: "different claim UID returns nil",
			pool: IPAddressPool{
				Status: IPAddressPoolStatus{
					Allocations: []IPAddressAllocation{
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "c1"},
							ClaimUID: types.UID("uid1"),
							Address:  "10.0.0.1",
						},
					},
				},
			},
			claim: Claim{ObjectMeta: metav1.ObjectMeta{Name: "c1", UID: "other-uid"}},
			want:  nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.pool.FindAllocation(&test.claim)
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("FindAllocation() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestIPAddressPool_Allocate(t *testing.T) {
	tests := []struct {
		name      string
		pool      IPAddressPool
		claim     Claim
		wantErr   bool
		checkFunc func(t *testing.T, pool *IPAddressPool, alloc *ClaimAllocation)
	}{
		{
			name: "prefix 10.0.0.0/30 allocates first address",
			pool: IPAddressPool{
				Spec: IPAddressPoolSpec{
					Prefixes: []corev1alpha1.IPPrefix{
						corev1alpha1.MustParsePrefix("10.0.0.0/30"),
					},
				},
			},
			claim: Claim{ObjectMeta: metav1.ObjectMeta{Name: "test-claim", UID: "test-uid"}},
			checkFunc: func(t *testing.T, pool *IPAddressPool, alloc *ClaimAllocation) {
				if alloc.IPAddress == nil {
					t.Fatal("IPAddress is nil, want non-nil")
				}
				if *alloc.IPAddress != "10.0.0.0" {
					t.Errorf("IPAddress = %q, want %q", *alloc.IPAddress, "10.0.0.0")
				}
				if alloc.Value != "10.0.0.0" {
					t.Errorf("Value = %q, want %q", alloc.Value, "10.0.0.0")
				}
			},
		},
		{
			name: "10.0.0.0 already allocated allocates next address",
			pool: IPAddressPool{
				Spec: IPAddressPoolSpec{
					Prefixes: []corev1alpha1.IPPrefix{
						corev1alpha1.MustParsePrefix("10.0.0.0/30"),
					},
				},
				Status: IPAddressPoolStatus{
					Allocations: []IPAddressAllocation{
						{ClaimRef: corev1alpha1.LocalObjectReference{Name: "existing"}, ClaimUID: "existing-uid", Address: "10.0.0.0"},
					},
				},
			},
			claim: Claim{ObjectMeta: metav1.ObjectMeta{Name: "test-claim", UID: "test-uid"}},
			checkFunc: func(t *testing.T, pool *IPAddressPool, alloc *ClaimAllocation) {
				if alloc.IPAddress == nil {
					t.Fatal("IPAddress is nil, want non-nil")
				}
				if *alloc.IPAddress != "10.0.0.1" {
					t.Errorf("IPAddress = %q, want %q", *alloc.IPAddress, "10.0.0.1")
				}
			},
		},
		{
			name: "all 4 addresses of /30 allocated returns error",
			pool: IPAddressPool{
				Spec: IPAddressPoolSpec{
					Prefixes: []corev1alpha1.IPPrefix{
						corev1alpha1.MustParsePrefix("10.0.0.0/30"),
					},
				},
				Status: IPAddressPoolStatus{
					Allocations: []IPAddressAllocation{
						{ClaimRef: corev1alpha1.LocalObjectReference{Name: "c1"}, ClaimUID: "uid1", Address: "10.0.0.0"},
						{ClaimRef: corev1alpha1.LocalObjectReference{Name: "c2"}, ClaimUID: "uid2", Address: "10.0.0.1"},
						{ClaimRef: corev1alpha1.LocalObjectReference{Name: "c3"}, ClaimUID: "uid3", Address: "10.0.0.2"},
						{ClaimRef: corev1alpha1.LocalObjectReference{Name: "c4"}, ClaimUID: "uid4", Address: "10.0.0.3"},
					},
				},
			},
			claim:   Claim{ObjectMeta: metav1.ObjectMeta{Name: "test-claim", UID: "test-uid"}},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			alloc, err := test.pool.Allocate(&test.claim)
			if test.wantErr {
				if err == nil {
					t.Fatal("Allocate() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Allocate() unexpected error: %v", err)
			}
			if test.checkFunc != nil {
				test.checkFunc(t, &test.pool, alloc)
			}
		})
	}
}
