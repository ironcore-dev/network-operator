// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

func TestIndexPool_Total(t *testing.T) {
	tests := []struct {
		name string
		pool IndexPool
		want int64
	}{
		{
			name: "empty ranges",
			pool: IndexPool{
				Spec: IndexPoolSpec{
					Ranges: []corev1alpha1.IndexRange{},
				},
			},
			want: 0,
		},
		{
			name: "single range",
			pool: IndexPool{
				Spec: IndexPoolSpec{
					Ranges: []corev1alpha1.IndexRange{
						corev1alpha1.MustParseIndexRange("1..10"),
					},
				},
			},
			want: 10,
		},
		{
			name: "single range with same start and end",
			pool: IndexPool{
				Spec: IndexPoolSpec{
					Ranges: []corev1alpha1.IndexRange{
						corev1alpha1.MustParseIndexRange("5..5"),
					},
				},
			},
			want: 1,
		},
		{
			name: "multiple ranges",
			pool: IndexPool{
				Spec: IndexPoolSpec{
					Ranges: []corev1alpha1.IndexRange{
						corev1alpha1.MustParseIndexRange("1..10"),
						corev1alpha1.MustParseIndexRange("20..30"),
						corev1alpha1.MustParseIndexRange("100..200"),
					},
				},
			},
			want: 122,
		},
		{
			name: "large range",
			pool: IndexPool{
				Spec: IndexPoolSpec{
					Ranges: []corev1alpha1.IndexRange{
						corev1alpha1.MustParseIndexRange("64512..65534"),
					},
				},
			},
			want: 1023,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.pool.Total(); got != test.want {
				t.Errorf("Total() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestIndexPool_IsExhausted(t *testing.T) {
	tests := []struct {
		name string
		pool IndexPool
		want bool
	}{
		{
			name: "empty pool - exhausted",
			pool: IndexPool{
				Spec:   IndexPoolSpec{Ranges: []corev1alpha1.IndexRange{}},
				Status: IndexPoolStatus{Allocated: 0},
			},
			want: true,
		},
		{
			name: "no allocations - not exhausted",
			pool: IndexPool{
				Spec: IndexPoolSpec{
					Ranges: []corev1alpha1.IndexRange{
						corev1alpha1.MustParseIndexRange("1..10"),
					},
				},
				Status: IndexPoolStatus{Allocated: 0},
			},
			want: false,
		},
		{
			name: "partially allocated - not exhausted",
			pool: IndexPool{
				Spec: IndexPoolSpec{
					Ranges: []corev1alpha1.IndexRange{
						corev1alpha1.MustParseIndexRange("1..10"),
					},
				},
				Status: IndexPoolStatus{Allocated: 2},
			},
			want: false,
		},
		{
			name: "fully allocated - exhausted",
			pool: IndexPool{
				Spec: IndexPoolSpec{
					Ranges: []corev1alpha1.IndexRange{
						corev1alpha1.MustParseIndexRange("1..3"),
					},
				},
				Status: IndexPoolStatus{Allocated: 3},
			},
			want: true,
		},
		{
			name: "over-allocated - exhausted",
			pool: IndexPool{
				Spec: IndexPoolSpec{
					Ranges: []corev1alpha1.IndexRange{
						corev1alpha1.MustParseIndexRange("1..2"),
					},
				},
				Status: IndexPoolStatus{Allocated: 5},
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

func TestIndexPool_Allocate(t *testing.T) {
	tests := []struct {
		name     string
		pool     IndexPool
		existing []client.Object
		wantVal  string
		wantName string
		wantErr  bool
	}{
		{
			name: "empty pool allocates first index",
			pool: IndexPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pool", Namespace: "default"},
				Spec: IndexPoolSpec{
					Ranges: []corev1alpha1.IndexRange{
						corev1alpha1.MustParseIndexRange("1..3"),
					},
				},
			},
			existing: nil,
			wantVal:  "1",
			wantName: "test-pool-1",
		},
		{
			name: "skips already allocated index",
			pool: IndexPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pool", Namespace: "default"},
				Spec: IndexPoolSpec{
					Ranges: []corev1alpha1.IndexRange{
						corev1alpha1.MustParseIndexRange("1..3"),
					},
				},
			},
			existing: []client.Object{
				&Index{Spec: IndexSpec{Index: 1}},
			},
			wantVal:  "2",
			wantName: "test-pool-2",
		},
		{
			name: "all allocated returns error",
			pool: IndexPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pool", Namespace: "default"},
				Spec: IndexPoolSpec{
					Ranges: []corev1alpha1.IndexRange{
						corev1alpha1.MustParseIndexRange("1..2"),
					},
				},
			},
			existing: []client.Object{
				&Index{Spec: IndexSpec{Index: 1}},
				&Index{Spec: IndexSpec{Index: 2}},
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
