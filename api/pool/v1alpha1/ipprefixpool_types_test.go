// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"math/big"
	"net/netip"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
					Prefixes:               []corev1alpha1.IPPrefix{},
					AllocationPrefixLength: 28,
				},
			},
			want: big.NewInt(0),
		},
		{
			name: "single IPv4 prefix /24 allocating /28",
			pool: IPPrefixPool{
				Spec: IPPrefixPoolSpec{
					Prefixes:               []corev1alpha1.IPPrefix{corev1alpha1.MustParsePrefix("192.168.1.0/24")},
					AllocationPrefixLength: 28,
				},
			},
			want: big.NewInt(16),
		},
		{
			name: "single IPv4 prefix /16 allocating /24",
			pool: IPPrefixPool{
				Spec: IPPrefixPoolSpec{
					Prefixes:               []corev1alpha1.IPPrefix{corev1alpha1.MustParsePrefix("10.0.0.0/16")},
					AllocationPrefixLength: 24,
				},
			},
			want: big.NewInt(256),
		},
		{
			name: "multiple IPv4 prefixes",
			pool: IPPrefixPool{
				Spec: IPPrefixPoolSpec{
					Prefixes: []corev1alpha1.IPPrefix{
						corev1alpha1.MustParsePrefix("192.168.0.0/24"),
						corev1alpha1.MustParsePrefix("10.0.0.0/24"),
					},
					AllocationPrefixLength: 28,
				},
			},
			want: big.NewInt(32),
		},
		{
			name: "IPv6 prefix /48 allocating /64",
			pool: IPPrefixPool{
				Spec: IPPrefixPoolSpec{
					Prefixes:               []corev1alpha1.IPPrefix{corev1alpha1.MustParsePrefix("2001:db8::/48")},
					AllocationPrefixLength: 64,
				},
			},
			want: big.NewInt(65536),
		},
		{
			name: "invalid prefix - target smaller than base",
			pool: IPPrefixPool{
				Spec: IPPrefixPoolSpec{
					Prefixes:               []corev1alpha1.IPPrefix{corev1alpha1.MustParsePrefix("192.168.1.0/24")},
					AllocationPrefixLength: 16,
				},
			},
			want: big.NewInt(0),
		},
		{
			name: "mixed valid and invalid base prefixes",
			pool: IPPrefixPool{
				Spec: IPPrefixPoolSpec{
					Prefixes: []corev1alpha1.IPPrefix{
						corev1alpha1.MustParsePrefix("192.168.0.0/24"),
						corev1alpha1.MustParsePrefix("10.0.0.0/30"), // /30 base with /28 target is invalid (28 < 30)
					},
					AllocationPrefixLength: 28,
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

func TestIPPrefixPool_IsExhausted(t *testing.T) {
	tests := []struct {
		name string
		pool IPPrefixPool
		want bool
	}{
		{
			name: "empty pool - exhausted",
			pool: IPPrefixPool{
				Spec:   IPPrefixPoolSpec{Prefixes: []corev1alpha1.IPPrefix{}, AllocationPrefixLength: 31},
				Status: IPPrefixPoolStatus{Allocated: 0},
			},
			want: true,
		},
		{
			name: "no allocations - not exhausted",
			pool: IPPrefixPool{
				Spec: IPPrefixPoolSpec{
					Prefixes:               []corev1alpha1.IPPrefix{corev1alpha1.MustParsePrefix("192.168.1.0/30")},
					AllocationPrefixLength: 31,
				},
				Status: IPPrefixPoolStatus{Allocated: 0},
			},
			want: false,
		},
		{
			name: "partially allocated - not exhausted",
			pool: IPPrefixPool{
				Spec: IPPrefixPoolSpec{
					Prefixes:               []corev1alpha1.IPPrefix{corev1alpha1.MustParsePrefix("192.168.1.0/30")},
					AllocationPrefixLength: 31,
				},
				Status: IPPrefixPoolStatus{Allocated: 1},
			},
			want: false,
		},
		{
			name: "fully allocated - exhausted",
			pool: IPPrefixPool{
				Spec: IPPrefixPoolSpec{
					Prefixes:               []corev1alpha1.IPPrefix{corev1alpha1.MustParsePrefix("192.168.1.0/30")},
					AllocationPrefixLength: 31,
				},
				Status: IPPrefixPoolStatus{Allocated: 2},
			},
			want: true,
		},
		{
			name: "over-allocated - exhausted",
			pool: IPPrefixPool{
				Spec: IPPrefixPoolSpec{
					Prefixes:               []corev1alpha1.IPPrefix{corev1alpha1.MustParsePrefix("192.168.1.0/31")},
					AllocationPrefixLength: 32,
				},
				Status: IPPrefixPoolStatus{Allocated: 5},
			},
			want: true,
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

func TestIPPrefixPool_Allocate(t *testing.T) {
	tests := []struct {
		name     string
		pool     IPPrefixPool
		existing []client.Object
		wantVal  string
		wantName string
		wantErr  bool
	}{
		{
			name: "allocates first subnet",
			pool: IPPrefixPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pool", Namespace: "default"},
				Spec: IPPrefixPoolSpec{
					Prefixes:               []corev1alpha1.IPPrefix{corev1alpha1.MustParsePrefix("10.0.0.0/24")},
					AllocationPrefixLength: 26,
				},
			},
			existing: nil,
			wantVal:  "10.0.0.0/26",
			wantName: "test-pool-10-0-0-0-26",
		},
		{
			name: "skips already allocated subnet",
			pool: IPPrefixPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pool", Namespace: "default"},
				Spec: IPPrefixPoolSpec{
					Prefixes:               []corev1alpha1.IPPrefix{corev1alpha1.MustParsePrefix("10.0.0.0/24")},
					AllocationPrefixLength: 26,
				},
			},
			existing: []client.Object{
				&IPPrefix{Spec: IPPrefixSpec{Prefix: corev1alpha1.MustParsePrefix("10.0.0.0/26")}},
			},
			wantVal:  "10.0.0.64/26",
			wantName: "test-pool-10-0-0-64-26",
		},
		{
			name: "all allocated returns error",
			pool: IPPrefixPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pool", Namespace: "default"},
				Spec: IPPrefixPoolSpec{
					Prefixes:               []corev1alpha1.IPPrefix{corev1alpha1.MustParsePrefix("10.0.0.0/24")},
					AllocationPrefixLength: 26,
				},
			},
			existing: []client.Object{
				&IPPrefix{Spec: IPPrefixSpec{Prefix: corev1alpha1.MustParsePrefix("10.0.0.0/26")}},
				&IPPrefix{Spec: IPPrefixSpec{Prefix: corev1alpha1.MustParsePrefix("10.0.0.64/26")}},
				&IPPrefix{Spec: IPPrefixSpec{Prefix: corev1alpha1.MustParsePrefix("10.0.0.128/26")}},
				&IPPrefix{Spec: IPPrefixSpec{Prefix: corev1alpha1.MustParsePrefix("10.0.0.192/26")}},
			},
			wantErr: true,
		},
	}

	claim := &Claim{ObjectMeta: metav1.ObjectMeta{Name: "test-claim", UID: "test-uid"}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			allocs := make([]Allocation, len(test.existing))
			for i, obj := range test.existing {
				allocs[i] = obj.(Allocation)
			}

			result, err := test.pool.Allocate(claim, allocs)
			if test.wantErr {
				if err == nil {
					t.Fatal("Allocate() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Allocate() unexpected error: %v", err)
			}
			if result.Value() != test.wantVal {
				t.Errorf("Value = %q, want %q", result.Value(), test.wantVal)
			}
			if result.GetName() != test.wantName {
				t.Errorf("Name = %q, want %q", result.GetName(), test.wantName)
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
