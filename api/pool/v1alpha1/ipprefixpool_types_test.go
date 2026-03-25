// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"math/big"
	"net/netip"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	corev1alpha1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

func TestIPPrefixPool_Total(t *testing.T) {
	tests := []struct {
		name string
		pool IPPrefixPool
		want *big.Int
	}{
		{
			name: "empty prefixes",
			pool: IPPrefixPool{
				Spec: IPPrefixPoolSpec{
					Prefixes: []IPPrefixPoolPrefix{},
				},
			},
			want: big.NewInt(0),
		},
		{
			name: "single IPv4 prefix /24 allocating /28",
			pool: IPPrefixPool{
				Spec: IPPrefixPoolSpec{
					Prefixes: []IPPrefixPoolPrefix{
						{
							Prefix:       corev1alpha1.MustParsePrefix("192.168.1.0/24"),
							PrefixLength: 28,
						},
					},
				},
			},
			want: big.NewInt(16),
		},
		{
			name: "single IPv4 prefix /16 allocating /24",
			pool: IPPrefixPool{
				Spec: IPPrefixPoolSpec{
					Prefixes: []IPPrefixPoolPrefix{
						{
							Prefix:       corev1alpha1.MustParsePrefix("10.0.0.0/16"),
							PrefixLength: 24,
						},
					},
				},
			},
			want: big.NewInt(256),
		},
		{
			name: "multiple IPv4 prefixes",
			pool: IPPrefixPool{
				Spec: IPPrefixPoolSpec{
					Prefixes: []IPPrefixPoolPrefix{
						{
							Prefix:       corev1alpha1.MustParsePrefix("192.168.0.0/24"),
							PrefixLength: 28,
						},
						{
							Prefix:       corev1alpha1.MustParsePrefix("10.0.0.0/24"),
							PrefixLength: 28,
						},
					},
				},
			},
			want: big.NewInt(32),
		},
		{
			name: "IPv6 prefix /48 allocating /64",
			pool: IPPrefixPool{
				Spec: IPPrefixPoolSpec{
					Prefixes: []IPPrefixPoolPrefix{
						{
							Prefix:       corev1alpha1.MustParsePrefix("2001:db8::/48"),
							PrefixLength: 64,
						},
					},
				},
			},
			want: big.NewInt(65536),
		},
		{
			name: "invalid prefix - target smaller than base",
			pool: IPPrefixPool{
				Spec: IPPrefixPoolSpec{
					Prefixes: []IPPrefixPoolPrefix{
						{
							Prefix:       corev1alpha1.MustParsePrefix("192.168.1.0/24"),
							PrefixLength: 16,
						},
					},
				},
			},
			want: big.NewInt(0),
		},
		{
			name: "invalid prefix - target too large for IPv4",
			pool: IPPrefixPool{
				Spec: IPPrefixPoolSpec{
					Prefixes: []IPPrefixPoolPrefix{
						{
							Prefix:       corev1alpha1.MustParsePrefix("192.168.1.0/24"),
							PrefixLength: 64,
						},
					},
				},
			},
			want: big.NewInt(0),
		},
		{
			name: "mixed valid and invalid prefixes",
			pool: IPPrefixPool{
				Spec: IPPrefixPoolSpec{
					Prefixes: []IPPrefixPoolPrefix{
						{
							Prefix:       corev1alpha1.MustParsePrefix("192.168.0.0/24"),
							PrefixLength: 28,
						},
						{
							Prefix:       corev1alpha1.MustParsePrefix("10.0.0.0/24"),
							PrefixLength: 16,
						},
					},
				},
			},
			want: big.NewInt(16),
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

func TestIPPrefixPool_Allocated(t *testing.T) {
	tests := []struct {
		name string
		pool IPPrefixPool
		want int
	}{
		{
			name: "no allocations",
			pool: IPPrefixPool{
				Status: IPPrefixPoolStatus{
					Allocations: []IPPrefixAllocation{},
				},
			},
			want: 0,
		},
		{
			name: "single allocation",
			pool: IPPrefixPool{
				Status: IPPrefixPoolStatus{
					Allocations: []IPPrefixAllocation{
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "claim-1"},
							ClaimUID: types.UID("uid-1"),
							Prefix:   corev1alpha1.MustParsePrefix("192.168.1.0/28"),
						},
					},
				},
			},
			want: 1,
		},
		{
			name: "multiple allocations",
			pool: IPPrefixPool{
				Status: IPPrefixPoolStatus{
					Allocations: []IPPrefixAllocation{
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "claim-1"},
							ClaimUID: types.UID("uid-1"),
							Prefix:   corev1alpha1.MustParsePrefix("192.168.1.0/28"),
						},
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "claim-2"},
							ClaimUID: types.UID("uid-2"),
							Prefix:   corev1alpha1.MustParsePrefix("192.168.1.16/28"),
						},
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "claim-3"},
							ClaimUID: types.UID("uid-3"),
							Prefix:   corev1alpha1.MustParsePrefix("192.168.1.32/28"),
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

func TestIPPrefixPool_IsExhausted(t *testing.T) {
	tests := []struct {
		name string
		pool IPPrefixPool
		want bool
	}{
		{
			name: "empty pool - exhausted",
			pool: IPPrefixPool{
				Spec: IPPrefixPoolSpec{
					Prefixes: []IPPrefixPoolPrefix{},
				},
				Status: IPPrefixPoolStatus{
					Allocations: []IPPrefixAllocation{},
				},
			},
			want: true,
		},
		{
			name: "no allocations - not exhausted",
			pool: IPPrefixPool{
				Spec: IPPrefixPoolSpec{
					Prefixes: []IPPrefixPoolPrefix{
						{
							Prefix:       corev1alpha1.MustParsePrefix("192.168.1.0/30"),
							PrefixLength: 31,
						},
					},
				},
				Status: IPPrefixPoolStatus{
					Allocations: []IPPrefixAllocation{},
				},
			},
			want: false,
		},
		{
			name: "partially allocated - not exhausted",
			pool: IPPrefixPool{
				Spec: IPPrefixPoolSpec{
					Prefixes: []IPPrefixPoolPrefix{
						{
							Prefix:       corev1alpha1.MustParsePrefix("192.168.1.0/30"),
							PrefixLength: 31,
						},
					},
				},
				Status: IPPrefixPoolStatus{
					Allocations: []IPPrefixAllocation{
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "claim-1"},
							ClaimUID: types.UID("uid-1"),
							Prefix:   corev1alpha1.MustParsePrefix("192.168.1.0/31"),
						},
					},
				},
			},
			want: false,
		},
		{
			name: "fully allocated - exhausted",
			pool: IPPrefixPool{
				Spec: IPPrefixPoolSpec{
					Prefixes: []IPPrefixPoolPrefix{
						{
							Prefix:       corev1alpha1.MustParsePrefix("192.168.1.0/30"),
							PrefixLength: 31,
						},
					},
				},
				Status: IPPrefixPoolStatus{
					Allocations: []IPPrefixAllocation{
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "claim-1"},
							ClaimUID: types.UID("uid-1"),
							Prefix:   corev1alpha1.MustParsePrefix("192.168.1.0/31"),
						},
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "claim-2"},
							ClaimUID: types.UID("uid-2"),
							Prefix:   corev1alpha1.MustParsePrefix("192.168.1.2/31"),
						},
					},
				},
			},
			want: true,
		},
		{
			name: "over-allocated - exhausted",
			pool: IPPrefixPool{
				Spec: IPPrefixPoolSpec{
					Prefixes: []IPPrefixPoolPrefix{
						{
							Prefix:       corev1alpha1.MustParsePrefix("192.168.1.0/31"),
							PrefixLength: 32,
						},
					},
				},
				Status: IPPrefixPoolStatus{
					Allocations: []IPPrefixAllocation{
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "claim-1"},
							ClaimUID: types.UID("uid-1"),
							Prefix:   corev1alpha1.MustParsePrefix("192.168.1.0/32"),
						},
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "claim-2"},
							ClaimUID: types.UID("uid-2"),
							Prefix:   corev1alpha1.MustParsePrefix("192.168.1.1/32"),
						},
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "claim-3"},
							ClaimUID: types.UID("uid-3"),
							Prefix:   corev1alpha1.MustParsePrefix("192.168.1.2/32"),
						},
					},
				},
			},
			want: true,
		},
		{
			name: "IPv6 pool - not exhausted",
			pool: IPPrefixPool{
				Spec: IPPrefixPoolSpec{
					Prefixes: []IPPrefixPoolPrefix{
						{
							Prefix:       corev1alpha1.MustParsePrefix("2001:db8::/62"),
							PrefixLength: 64,
						},
					},
				},
				Status: IPPrefixPoolStatus{
					Allocations: []IPPrefixAllocation{
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "claim-1"},
							ClaimUID: types.UID("uid-1"),
							Prefix:   corev1alpha1.MustParsePrefix("2001:db8::/64"),
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

func TestIPPrefixPool_FindAllocation(t *testing.T) {
	tests := []struct {
		name  string
		pool  IPPrefixPool
		claim Claim
		want  *ClaimAllocation
	}{
		{
			name: "empty allocations returns nil",
			pool: IPPrefixPool{
				Status: IPPrefixPoolStatus{
					Allocations: []IPPrefixAllocation{},
				},
			},
			claim: Claim{ObjectMeta: metav1.ObjectMeta{Name: "c1", UID: "uid1"}},
			want:  nil,
		},
		{
			name: "matching claim returns allocation",
			pool: IPPrefixPool{
				Status: IPPrefixPoolStatus{
					Allocations: []IPPrefixAllocation{
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "c1"},
							ClaimUID: types.UID("uid1"),
							Prefix:   corev1alpha1.MustParsePrefix("10.0.0.0/26"),
						},
					},
				},
			},
			claim: Claim{ObjectMeta: metav1.ObjectMeta{Name: "c1", UID: "uid1"}},
			want: &ClaimAllocation{
				Prefix: new(corev1alpha1.MustParsePrefix("10.0.0.0/26")),
				Value:  "10.0.0.0/26",
			},
		},
		{
			name: "different claim name returns nil",
			pool: IPPrefixPool{
				Status: IPPrefixPoolStatus{
					Allocations: []IPPrefixAllocation{
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "c1"},
							ClaimUID: types.UID("uid1"),
							Prefix:   corev1alpha1.MustParsePrefix("10.0.0.0/26"),
						},
					},
				},
			},
			claim: Claim{ObjectMeta: metav1.ObjectMeta{Name: "other", UID: "uid1"}},
			want:  nil,
		},
		{
			name: "different claim UID returns nil",
			pool: IPPrefixPool{
				Status: IPPrefixPoolStatus{
					Allocations: []IPPrefixAllocation{
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "c1"},
							ClaimUID: types.UID("uid1"),
							Prefix:   corev1alpha1.MustParsePrefix("10.0.0.0/26"),
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

func TestIPPrefixPool_Allocate(t *testing.T) {
	tests := []struct {
		name      string
		pool      IPPrefixPool
		claim     Claim
		wantErr   bool
		checkFunc func(t *testing.T, pool *IPPrefixPool, alloc *ClaimAllocation)
	}{
		{
			name: "base 10.0.0.0/24 prefixLength 26 allocates first subnet",
			pool: IPPrefixPool{
				Spec: IPPrefixPoolSpec{
					Prefixes: []IPPrefixPoolPrefix{
						{
							Prefix:       corev1alpha1.MustParsePrefix("10.0.0.0/24"),
							PrefixLength: 26,
						},
					},
				},
			},
			claim: Claim{ObjectMeta: metav1.ObjectMeta{Name: "test-claim", UID: "test-uid"}},
			checkFunc: func(t *testing.T, pool *IPPrefixPool, alloc *ClaimAllocation) {
				if alloc.Prefix == nil {
					t.Fatal("Prefix is nil, want non-nil")
				}
				if alloc.Prefix.String() != "10.0.0.0/26" {
					t.Errorf("Prefix = %q, want %q", alloc.Prefix.String(), "10.0.0.0/26")
				}
				if alloc.Value != "10.0.0.0/26" {
					t.Errorf("Value = %q, want %q", alloc.Value, "10.0.0.0/26")
				}
			},
		},
		{
			name: "10.0.0.0/26 already allocated allocates next subnet",
			pool: IPPrefixPool{
				Spec: IPPrefixPoolSpec{
					Prefixes: []IPPrefixPoolPrefix{
						{
							Prefix:       corev1alpha1.MustParsePrefix("10.0.0.0/24"),
							PrefixLength: 26,
						},
					},
				},
				Status: IPPrefixPoolStatus{
					Allocations: []IPPrefixAllocation{
						{ClaimRef: corev1alpha1.LocalObjectReference{Name: "existing"}, ClaimUID: "existing-uid", Prefix: corev1alpha1.MustParsePrefix("10.0.0.0/26")},
					},
				},
			},
			claim: Claim{ObjectMeta: metav1.ObjectMeta{Name: "test-claim", UID: "test-uid"}},
			checkFunc: func(t *testing.T, pool *IPPrefixPool, alloc *ClaimAllocation) {
				if alloc.Prefix == nil {
					t.Fatal("Prefix is nil, want non-nil")
				}
				if alloc.Prefix.String() != "10.0.0.64/26" {
					t.Errorf("Prefix = %q, want %q", alloc.Prefix.String(), "10.0.0.64/26")
				}
			},
		},
		{
			name: "all 4 /26 subnets of /24 allocated returns error",
			pool: IPPrefixPool{
				Spec: IPPrefixPoolSpec{
					Prefixes: []IPPrefixPoolPrefix{
						{
							Prefix:       corev1alpha1.MustParsePrefix("10.0.0.0/24"),
							PrefixLength: 26,
						},
					},
				},
				Status: IPPrefixPoolStatus{
					Allocations: []IPPrefixAllocation{
						{ClaimRef: corev1alpha1.LocalObjectReference{Name: "c1"}, ClaimUID: "uid1", Prefix: corev1alpha1.MustParsePrefix("10.0.0.0/26")},
						{ClaimRef: corev1alpha1.LocalObjectReference{Name: "c2"}, ClaimUID: "uid2", Prefix: corev1alpha1.MustParsePrefix("10.0.0.64/26")},
						{ClaimRef: corev1alpha1.LocalObjectReference{Name: "c3"}, ClaimUID: "uid3", Prefix: corev1alpha1.MustParsePrefix("10.0.0.128/26")},
						{ClaimRef: corev1alpha1.LocalObjectReference{Name: "c4"}, ClaimUID: "uid4", Prefix: corev1alpha1.MustParsePrefix("10.0.0.192/26")},
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

func TestStepAddr(t *testing.T) {
	tests := []struct {
		name string
		addr string
		n    int
		want string // empty means zero/invalid addr expected
	}{
		{
			name: "normal IPv4 advance",
			addr: "10.0.0.192",
			n:    6,
			want: "10.0.1.0",
		},
		{
			name: "IPv4 overflow",
			addr: "255.255.255.254",
			n:    1,
			want: "",
		},
		{
			name: "IPv6 overflow",
			addr: "ffff:ffff:ffff:ffff:ffff:ffff:ffff:fffe",
			n:    1,
			want: "",
		},
		{
			name: "normal IPv6 advance",
			addr: "2001:db8::1",
			n:    0,
			want: "2001:db8::2",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			addr := netip.MustParseAddr(test.addr)
			got := stepAddr(addr, test.n)
			if test.want == "" {
				if got.IsValid() {
					t.Errorf("stepAddr(%s, %d) = %s, want invalid", test.addr, test.n, got)
				}
				return
			}
			if got.String() != test.want {
				t.Errorf("stepAddr(%s, %d) = %s, want %s", test.addr, test.n, got, test.want)
			}
		})
	}
}
