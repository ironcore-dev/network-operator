// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"math/big"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

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

func TestIPAddressPool_IsExhausted(t *testing.T) {
	tests := []struct {
		name string
		pool IPAddressPool
		want bool
	}{
		{
			name: "empty pool - exhausted",
			pool: IPAddressPool{
				Spec:   IPAddressPoolSpec{Prefixes: []corev1alpha1.IPPrefix{}},
				Status: IPAddressPoolStatus{Allocated: 0},
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
				Status: IPAddressPoolStatus{Allocated: 0},
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
				Status: IPAddressPoolStatus{Allocated: 2},
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
				Status: IPAddressPoolStatus{Allocated: 4},
			},
			want: true,
		},
		{
			name: "over-allocated - exhausted",
			pool: IPAddressPool{
				Spec: IPAddressPoolSpec{
					Prefixes: []corev1alpha1.IPPrefix{
						corev1alpha1.MustParsePrefix("192.168.1.0/30"),
					},
				},
				Status: IPAddressPoolStatus{Allocated: 10},
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

func TestIPAddressPool_Allocate(t *testing.T) {
	tests := []struct {
		name     string
		pool     IPAddressPool
		existing []client.Object
		wantVal  string
		wantName string
		wantErr  bool
	}{
		{
			name: "empty pool allocates first address",
			pool: IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pool", Namespace: "default"},
				Spec: IPAddressPoolSpec{
					Prefixes: []corev1alpha1.IPPrefix{
						corev1alpha1.MustParsePrefix("10.0.0.0/30"),
					},
				},
			},
			existing: nil,
			wantVal:  "10.0.0.0",
			wantName: "test-pool-10-0-0-0",
		},
		{
			name: "skips already allocated address",
			pool: IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pool", Namespace: "default"},
				Spec: IPAddressPoolSpec{
					Prefixes: []corev1alpha1.IPPrefix{
						corev1alpha1.MustParsePrefix("10.0.0.0/30"),
					},
				},
			},
			existing: []client.Object{
				&IPAddress{Spec: IPAddressSpec{Address: corev1alpha1.MustParseAddr("10.0.0.0")}},
			},
			wantVal:  "10.0.0.1",
			wantName: "test-pool-10-0-0-1",
		},
		{
			name: "all allocated returns error",
			pool: IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pool", Namespace: "default"},
				Spec: IPAddressPoolSpec{
					Prefixes: []corev1alpha1.IPPrefix{
						corev1alpha1.MustParsePrefix("10.0.0.0/31"),
					},
				},
			},
			existing: []client.Object{
				&IPAddress{Spec: IPAddressSpec{Address: corev1alpha1.MustParseAddr("10.0.0.0")}},
				&IPAddress{Spec: IPAddressSpec{Address: corev1alpha1.MustParseAddr("10.0.0.1")}},
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
