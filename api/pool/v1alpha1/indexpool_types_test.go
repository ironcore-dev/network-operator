// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	corev1alpha1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

func TestIndexPool_Total(t *testing.T) {
	tests := []struct {
		name string
		pool IndexPool
		want uint64
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

func TestIndexPool_Allocated(t *testing.T) {
	tests := []struct {
		name string
		pool IndexPool
		want int
	}{
		{
			name: "no allocations",
			pool: IndexPool{
				Status: IndexPoolStatus{
					Allocations: []IndexAllocation{},
				},
			},
			want: 0,
		},
		{
			name: "single allocation",
			pool: IndexPool{
				Status: IndexPoolStatus{
					Allocations: []IndexAllocation{
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "claim-1"},
							ClaimUID: types.UID("uid-1"),
							Index:    1,
						},
					},
				},
			},
			want: 1,
		},
		{
			name: "multiple allocations",
			pool: IndexPool{
				Status: IndexPoolStatus{
					Allocations: []IndexAllocation{
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "claim-1"},
							ClaimUID: types.UID("uid-1"),
							Index:    1,
						},
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "claim-2"},
							ClaimUID: types.UID("uid-2"),
							Index:    2,
						},
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "claim-3"},
							ClaimUID: types.UID("uid-3"),
							Index:    3,
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

func TestIndexPool_IsExhausted(t *testing.T) {
	tests := []struct {
		name string
		pool IndexPool
		want bool
	}{
		{
			name: "empty pool - exhausted",
			pool: IndexPool{
				Spec: IndexPoolSpec{
					Ranges: []corev1alpha1.IndexRange{},
				},
				Status: IndexPoolStatus{
					Allocations: []IndexAllocation{},
				},
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
				Status: IndexPoolStatus{
					Allocations: []IndexAllocation{},
				},
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
				Status: IndexPoolStatus{
					Allocations: []IndexAllocation{
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "claim-1"},
							ClaimUID: types.UID("uid-1"),
							Index:    1,
						},
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "claim-2"},
							ClaimUID: types.UID("uid-2"),
							Index:    2,
						},
					},
				},
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
				Status: IndexPoolStatus{
					Allocations: []IndexAllocation{
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "claim-1"},
							ClaimUID: types.UID("uid-1"),
							Index:    1,
						},
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "claim-2"},
							ClaimUID: types.UID("uid-2"),
							Index:    2,
						},
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "claim-3"},
							ClaimUID: types.UID("uid-3"),
							Index:    3,
						},
					},
				},
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
				Status: IndexPoolStatus{
					Allocations: []IndexAllocation{
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "claim-1"},
							ClaimUID: types.UID("uid-1"),
							Index:    1,
						},
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "claim-2"},
							ClaimUID: types.UID("uid-2"),
							Index:    2,
						},
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "claim-3"},
							ClaimUID: types.UID("uid-3"),
							Index:    3,
						},
					},
				},
			},
			want: true,
		},
		{
			name: "multiple ranges - partially allocated",
			pool: IndexPool{
				Spec: IndexPoolSpec{
					Ranges: []corev1alpha1.IndexRange{
						corev1alpha1.MustParseIndexRange("1..10"),
						corev1alpha1.MustParseIndexRange("20..30"),
					},
				},
				Status: IndexPoolStatus{
					Allocations: []IndexAllocation{
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "claim-1"},
							ClaimUID: types.UID("uid-1"),
							Index:    1,
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

func TestIndexPool_FindAllocation(t *testing.T) {
	tests := []struct {
		name  string
		pool  IndexPool
		claim Claim
		want  *ClaimAllocation
	}{
		{
			name: "empty allocations returns nil",
			pool: IndexPool{
				Status: IndexPoolStatus{
					Allocations: []IndexAllocation{},
				},
			},
			claim: Claim{ObjectMeta: metav1.ObjectMeta{Name: "c1", UID: "uid1"}},
			want:  nil,
		},
		{
			name: "matching claim returns allocation",
			pool: IndexPool{
				Status: IndexPoolStatus{
					Allocations: []IndexAllocation{
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "c1"},
							ClaimUID: types.UID("uid1"),
							Index:    5,
						},
					},
				},
			},
			claim: Claim{ObjectMeta: metav1.ObjectMeta{Name: "c1", UID: "uid1"}},
			want:  &ClaimAllocation{Index: new(uint64(5)), Value: "5"},
		},
		{
			name: "different claim name returns nil",
			pool: IndexPool{
				Status: IndexPoolStatus{
					Allocations: []IndexAllocation{
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "c1"},
							ClaimUID: types.UID("uid1"),
							Index:    5,
						},
					},
				},
			},
			claim: Claim{ObjectMeta: metav1.ObjectMeta{Name: "other", UID: "uid1"}},
			want:  nil,
		},
		{
			name: "different claim UID returns nil",
			pool: IndexPool{
				Status: IndexPoolStatus{
					Allocations: []IndexAllocation{
						{
							ClaimRef: corev1alpha1.LocalObjectReference{Name: "c1"},
							ClaimUID: types.UID("uid1"),
							Index:    5,
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

func TestIndexPool_Allocate(t *testing.T) {
	tests := []struct {
		name      string
		pool      IndexPool
		claim     Claim
		wantErr   bool
		checkFunc func(t *testing.T, pool *IndexPool, alloc *ClaimAllocation)
	}{
		{
			name: "empty pool range allocates first index",
			pool: IndexPool{
				Spec: IndexPoolSpec{
					Ranges: []corev1alpha1.IndexRange{
						corev1alpha1.MustParseIndexRange("1..3"),
					},
				},
			},
			claim: Claim{ObjectMeta: metav1.ObjectMeta{Name: "test-claim", UID: "test-uid"}},
			checkFunc: func(t *testing.T, pool *IndexPool, alloc *ClaimAllocation) {
				if alloc.Index == nil {
					t.Fatal("Index is nil, want non-nil")
				}
				if *alloc.Index != 1 {
					t.Errorf("Index = %v, want 1", *alloc.Index)
				}
				if alloc.Value != "1" {
					t.Errorf("Value = %q, want %q", alloc.Value, "1")
				}
				recorded := pool.Status.Allocations[len(pool.Status.Allocations)-1]
				if recorded.ClaimRef.Name != "test-claim" {
					t.Errorf("ClaimRef.Name = %q, want %q", recorded.ClaimRef.Name, "test-claim")
				}
				if recorded.ClaimUID != "test-uid" {
					t.Errorf("ClaimUID = %q, want %q", recorded.ClaimUID, "test-uid")
				}
			},
		},
		{
			name: "one already allocated allocates next index",
			pool: IndexPool{
				Spec: IndexPoolSpec{
					Ranges: []corev1alpha1.IndexRange{
						corev1alpha1.MustParseIndexRange("1..3"),
					},
				},
				Status: IndexPoolStatus{
					Allocations: []IndexAllocation{
						{ClaimRef: corev1alpha1.LocalObjectReference{Name: "existing"}, ClaimUID: "existing-uid", Index: 1},
					},
				},
			},
			claim: Claim{ObjectMeta: metav1.ObjectMeta{Name: "test-claim", UID: "test-uid"}},
			checkFunc: func(t *testing.T, pool *IndexPool, alloc *ClaimAllocation) {
				if alloc.Index == nil {
					t.Fatal("Index is nil, want non-nil")
				}
				if *alloc.Index != 2 {
					t.Errorf("Index = %v, want 2", *alloc.Index)
				}
				recorded := pool.Status.Allocations[len(pool.Status.Allocations)-1]
				if recorded.ClaimRef.Name != "test-claim" {
					t.Errorf("ClaimRef.Name = %q, want %q", recorded.ClaimRef.Name, "test-claim")
				}
				if recorded.ClaimUID != "test-uid" {
					t.Errorf("ClaimUID = %q, want %q", recorded.ClaimUID, "test-uid")
				}
			},
		},
		{
			name: "all allocated returns error",
			pool: IndexPool{
				Spec: IndexPoolSpec{
					Ranges: []corev1alpha1.IndexRange{
						corev1alpha1.MustParseIndexRange("1..2"),
					},
				},
				Status: IndexPoolStatus{
					Allocations: []IndexAllocation{
						{ClaimRef: corev1alpha1.LocalObjectReference{Name: "c1"}, ClaimUID: "uid1", Index: 1},
						{ClaimRef: corev1alpha1.LocalObjectReference{Name: "c2"}, ClaimUID: "uid2", Index: 2},
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
