// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package resourcelock

import (
	"context"
	"errors"
	"testing"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func init() {
	utilruntime.Must(coordinationv1.AddToScheme(scheme.Scheme))
}

func TestNewResourceLocker(t *testing.T) {
	tests := []struct {
		name          string
		namespace     string
		leaseDuration time.Duration
		renewPeriod   time.Duration
		wantErr       bool
	}{
		{
			name:          "valid configuration",
			namespace:     metav1.NamespaceDefault,
			leaseDuration: 15 * time.Second,
			renewPeriod:   5 * time.Second,
			wantErr:       false,
		},
		{
			name:          "renewPeriod equal to leaseDuration",
			namespace:     metav1.NamespaceDefault,
			leaseDuration: 10 * time.Second,
			renewPeriod:   10 * time.Second,
			wantErr:       true,
		},
		{
			name:          "renewPeriod greater than leaseDuration",
			namespace:     metav1.NamespaceDefault,
			leaseDuration: 5 * time.Second,
			renewPeriod:   10 * time.Second,
			wantErr:       true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
			rl, err := NewResourceLocker(client, test.namespace, test.leaseDuration, test.renewPeriod)

			if test.wantErr {
				if err == nil {
					t.Errorf("NewResourceLocker() expected error but got none")
					return
				}
				return
			}

			if err != nil {
				t.Errorf("NewResourceLocker() unexpected error = %v", err)
				return
			}
			if rl == nil {
				t.Errorf("NewResourceLocker() returned nil locker")
				return
			}
			if rl.namespace != test.namespace {
				t.Errorf("NewResourceLocker() namespace = %v, want %v", rl.namespace, test.namespace)
			}
			if rl.leaseDurationSeconds != int32(test.leaseDuration.Seconds()) {
				t.Errorf("NewResourceLocker() leaseDurationSeconds = %v, want %v", rl.leaseDurationSeconds, int32(test.leaseDuration.Seconds()))
			}
			if rl.renewPeriod != test.renewPeriod {
				t.Errorf("NewResourceLocker() renewPeriod = %v, want %v", rl.renewPeriod, test.renewPeriod)
			}
		})
	}
}

func TestAcquireLock_CreateNew(t *testing.T) {
	client := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
	rl, err := NewResourceLocker(client, metav1.NamespaceDefault, 15*time.Second, 5*time.Second)
	if err != nil {
		t.Fatalf("NewResourceLocker() error = %v", err)
	}

	ctx := context.Background()
	leaseName := "test-lease"
	lockerID := "locker-1"

	if err := rl.AcquireLock(ctx, leaseName, lockerID); err != nil {
		t.Errorf("AcquireLock() error = %v", err)
		return
	}

	lease := &coordinationv1.Lease{}
	key := types.NamespacedName{Namespace: metav1.NamespaceDefault, Name: leaseName}
	if err := client.Get(ctx, key, lease); err != nil {
		t.Fatalf("Get lease error = %v", err)
	}

	if lease.Spec.HolderIdentity == nil || *lease.Spec.HolderIdentity != lockerID {
		t.Errorf("Lease holder = %v, want %v", lease.Spec.HolderIdentity, lockerID)
	}

	if lease.Spec.LeaseDurationSeconds == nil || *lease.Spec.LeaseDurationSeconds != 15 {
		t.Errorf("Lease duration = %v, want %v", lease.Spec.LeaseDurationSeconds, 15)
	}
}

func TestAcquireLock_AlreadyOwned(t *testing.T) {
	lockerID := "locker-1"
	leaseName := "test-lease"

	now := metav1.NewMicroTime(time.Now())
	leaseDuration := int32(15)
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      leaseName,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       &lockerID,
			LeaseDurationSeconds: &leaseDuration,
			AcquireTime:          &now,
			RenewTime:            &now,
		},
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(lease).
		Build()

	rl, err := NewResourceLocker(client, metav1.NamespaceDefault, 15*time.Second, 5*time.Second)
	if err != nil {
		t.Fatalf("NewResourceLocker() error = %v", err)
	}

	ctx := context.Background()
	if err := rl.AcquireLock(ctx, leaseName, lockerID); err != nil {
		t.Errorf("AcquireLock() error = %v, expected success when already owned", err)
	}
}

func TestAcquireLock_HeldByAnother(t *testing.T) {
	lockerID1 := "locker-1"
	lockerID2 := "locker-2"
	leaseName := "test-lease"

	now := metav1.NewMicroTime(time.Now())
	leaseDuration := int32(15)
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      leaseName,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       &lockerID1,
			LeaseDurationSeconds: &leaseDuration,
			AcquireTime:          &now,
			RenewTime:            &now,
		},
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(lease).
		Build()

	rl, err := NewResourceLocker(client, metav1.NamespaceDefault, 15*time.Second, 5*time.Second)
	if err != nil {
		t.Fatalf("NewResourceLocker() error = %v", err)
	}

	ctx := context.Background()
	if err := rl.AcquireLock(ctx, leaseName, lockerID2); !errors.Is(err, ErrLockAlreadyHeld) {
		t.Errorf("AcquireLock() error = %v, want %v", err, ErrLockAlreadyHeld)
	}
}

func TestAcquireLock_ClaimExpired(t *testing.T) {
	lockerID1 := "locker-1"
	lockerID2 := "locker-2"
	leaseName := "test-lease"

	// Create an expired lease (renewed 20 seconds ago with 15 second duration)
	expiredTime := metav1.NewMicroTime(time.Now().Add(-20 * time.Second))
	leaseDuration := int32(15)
	existingLease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      leaseName,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       &lockerID1,
			LeaseDurationSeconds: &leaseDuration,
			AcquireTime:          &expiredTime,
			RenewTime:            &expiredTime,
		},
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(existingLease).
		Build()

	rl, err := NewResourceLocker(client, metav1.NamespaceDefault, 15*time.Second, 5*time.Second)
	if err != nil {
		t.Fatalf("NewResourceLocker() error = %v", err)
	}

	ctx := context.Background()
	if err := rl.AcquireLock(ctx, leaseName, lockerID2); err != nil {
		t.Errorf("AcquireLock() error = %v, expected to claim expired lease", err)
		return
	}

	lease := &coordinationv1.Lease{}
	key := types.NamespacedName{Namespace: metav1.NamespaceDefault, Name: leaseName}
	if err := client.Get(ctx, key, lease); err != nil {
		t.Fatalf("Get lease error = %v", err)
	}

	if lease.Spec.HolderIdentity == nil || *lease.Spec.HolderIdentity != lockerID2 {
		t.Errorf("Lease holder = %v, want %v", lease.Spec.HolderIdentity, lockerID2)
	}
}

func TestReleaseLock(t *testing.T) {
	lockerID := "locker-1"
	leaseName := "test-lease"

	now := metav1.NewMicroTime(time.Now())
	leaseDuration := int32(15)
	existingLease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      leaseName,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       &lockerID,
			LeaseDurationSeconds: &leaseDuration,
			AcquireTime:          &now,
			RenewTime:            &now,
		},
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(existingLease).
		Build()

	rl, err := NewResourceLocker(client, metav1.NamespaceDefault, 15*time.Second, 5*time.Second)
	if err != nil {
		t.Fatalf("NewResourceLocker() error = %v", err)
	}

	ctx := context.Background()
	if err := rl.ReleaseLock(ctx, leaseName, lockerID); err != nil {
		t.Errorf("ReleaseLock() error = %v", err)
		return
	}

	lease := &coordinationv1.Lease{}
	key := types.NamespacedName{Namespace: metav1.NamespaceDefault, Name: leaseName}
	if err := client.Get(ctx, key, lease); !apierrors.IsNotFound(err) {
		t.Errorf("Expected lease to be deleted, but got error = %v", err)
	}
}

func TestReleaseLock_NotOwned(t *testing.T) {
	lockerID1 := "locker-1"
	lockerID2 := "locker-2"
	leaseName := "test-lease"

	now := metav1.NewMicroTime(time.Now())
	leaseDuration := int32(15)
	existingLease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      leaseName,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       &lockerID1,
			LeaseDurationSeconds: &leaseDuration,
			AcquireTime:          &now,
			RenewTime:            &now,
		},
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(existingLease).
		Build()

	rl, err := NewResourceLocker(client, metav1.NamespaceDefault, 15*time.Second, 5*time.Second)
	if err != nil {
		t.Fatalf("NewResourceLocker() error = %v", err)
	}

	ctx := context.Background()
	if err := rl.ReleaseLock(ctx, leaseName, lockerID2); err != nil {
		t.Errorf("ReleaseLock() error = %v, expected success (noop) when not owned", err)
		return
	}

	lease := &coordinationv1.Lease{}
	key := types.NamespacedName{Namespace: metav1.NamespaceDefault, Name: leaseName}
	if err := client.Get(ctx, key, lease); err != nil {
		t.Fatalf("Get lease error = %v", err)
	}

	if lease.Spec.HolderIdentity == nil || *lease.Spec.HolderIdentity != lockerID1 {
		t.Errorf("Lease holder = %v, want %v", lease.Spec.HolderIdentity, lockerID1)
	}
}

func TestReleaseLock_NotFound(t *testing.T) {
	client := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()

	rl, err := NewResourceLocker(client, metav1.NamespaceDefault, 15*time.Second, 5*time.Second)
	if err != nil {
		t.Fatalf("NewResourceLocker() error = %v", err)
	}

	ctx := context.Background()
	if err := rl.ReleaseLock(ctx, "non-existent-lease", "locker-1"); err != nil {
		t.Errorf("ReleaseLock() error = %v, expected success (noop) when lease not found", err)
	}
}

func TestRenewLease(t *testing.T) {
	lockerID := "locker-1"
	leaseName := "test-lease"

	oldTime := metav1.NewMicroTime(time.Now().Add(-10 * time.Second))
	leaseDuration := int32(15)
	existingLease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      leaseName,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       &lockerID,
			LeaseDurationSeconds: &leaseDuration,
			AcquireTime:          &oldTime,
			RenewTime:            &oldTime,
		},
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(existingLease).
		Build()

	rl, err := NewResourceLocker(client, metav1.NamespaceDefault, 15*time.Second, 5*time.Second)
	if err != nil {
		t.Fatalf("NewResourceLocker() error = %v", err)
	}

	ctx := context.Background()
	beforeRenew := time.Now()
	if err := rl.renewLease(ctx, leaseName, lockerID); err != nil {
		t.Errorf("renewLease() error = %v", err)
		return
	}

	lease := &coordinationv1.Lease{}
	key := types.NamespacedName{Namespace: metav1.NamespaceDefault, Name: leaseName}
	if err := client.Get(ctx, key, lease); err != nil {
		t.Fatalf("Get lease error = %v", err)
	}

	if lease.Spec.RenewTime == nil {
		t.Errorf("Lease RenewTime is nil")
		return
	}

	if lease.Spec.RenewTime.Time.Before(beforeRenew) {
		t.Errorf("Lease RenewTime = %v, want after %v", lease.Spec.RenewTime.Time, beforeRenew)
	}
}

func TestRenewLease_NotOwned(t *testing.T) {
	lockerID1 := "locker-1"
	lockerID2 := "locker-2"
	leaseName := "test-lease"

	now := metav1.NewMicroTime(time.Now())
	leaseDuration := int32(15)
	existingLease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      leaseName,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       &lockerID1,
			LeaseDurationSeconds: &leaseDuration,
			AcquireTime:          &now,
			RenewTime:            &now,
		},
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(existingLease).
		Build()

	rl, err := NewResourceLocker(client, metav1.NamespaceDefault, 15*time.Second, 5*time.Second)
	if err != nil {
		t.Fatalf("NewResourceLocker() error = %v", err)
	}

	ctx := context.Background()
	if err := rl.renewLease(ctx, leaseName, lockerID2); err == nil {
		t.Errorf("renewLease() expected error when not owner, got nil")
	}
}

func TestReleaseLock_CancelsRenewalGoroutine(t *testing.T) {
	leaseName := "test-lease"
	lockerID := "test-locker"

	leaseDuration := int32(15)
	existingLease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      leaseName,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       &lockerID,
			LeaseDurationSeconds: &leaseDuration,
		},
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(existingLease).
		Build()

	rl, err := NewResourceLocker(client, metav1.NamespaceDefault, 15*time.Second, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("NewResourceLocker() error = %v", err)
	}
	defer rl.Close()

	ctx := context.Background()

	if err := rl.AcquireLock(ctx, leaseName, lockerID); err != nil {
		t.Fatalf("AcquireLock() error = %v", err)
	}

	if _, exists := rl.cancelFuncs.Load(leaseName); !exists {
		t.Error("Expected cancel func to be stored after AcquireLock")
	}

	if err := rl.ReleaseLock(ctx, leaseName, lockerID); err != nil {
		t.Fatalf("ReleaseLock() error = %v", err)
	}

	if _, exists := rl.cancelFuncs.Load(leaseName); exists {
		t.Error("Expected cancel func to be removed after ReleaseLock")
	}
}
