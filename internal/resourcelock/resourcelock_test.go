// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package resourcelock

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
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

	ctx := t.Context()
	leaseName := "test-lease"
	locker := &corev1.ConfigMap{}
	locker.Namespace = "test-namespace"
	locker.Name = "locker-1"

	release, err := rl.AcquireLock(ctx, leaseName, locker)
	if err != nil {
		t.Errorf("AcquireLock() error = %v", err)
		return
	}
	defer release(ctx) //nolint:errcheck

	lease := &coordinationv1.Lease{}
	key := types.NamespacedName{Namespace: metav1.NamespaceDefault, Name: leaseName}
	if err := client.Get(ctx, key, lease); err != nil {
		t.Fatalf("Get lease error = %v", err)
	}

	const lockerIDPrefix = "ConfigMap/test-namespace/locker-1/"
	if lease.Spec.HolderIdentity == nil || !strings.HasPrefix(*lease.Spec.HolderIdentity, lockerIDPrefix) {
		t.Errorf("Lease holder = %v, want prefix %q", lease.Spec.HolderIdentity, lockerIDPrefix)
	}

	if lease.Spec.LeaseDurationSeconds == nil || *lease.Spec.LeaseDurationSeconds != 15 {
		t.Errorf("Lease duration = %v, want %v", lease.Spec.LeaseDurationSeconds, 15)
	}
}

func TestAcquireLock_RejectsConcurrentAttemptForSameLocker(t *testing.T) {
	client := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
	rl, err := NewResourceLocker(client, metav1.NamespaceDefault, 15*time.Second, 5*time.Second)
	if err != nil {
		t.Fatalf("NewResourceLocker() error = %v", err)
	}

	ctx := t.Context()
	leaseName := "test-lease"
	locker := &corev1.ConfigMap{}
	locker.Namespace = "test-namespace"
	locker.Name = "locker-1"

	release, err := rl.AcquireLock(ctx, leaseName, locker)
	if err != nil {
		t.Fatalf("first AcquireLock() error = %v", err)
	}
	defer release(ctx) //nolint:errcheck

	if _, err := rl.AcquireLock(ctx, leaseName, locker); !errors.Is(err, ErrLockAlreadyHeld) {
		t.Errorf("second AcquireLock() error = %v, want %v", err, ErrLockAlreadyHeld)
	}
}

func TestAcquireLock_HeldByAnother(t *testing.T) {
	lockerID1 := "locker-1"
	leaseName := "test-lease"

	now := metav1.NewMicroTime(time.Now())
	leaseDuration := int32(15)
	lease := &coordinationv1.Lease{
		Name:      leaseName,
		Namespace: metav1.NamespaceDefault,
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

	ctx := t.Context()
	locker := &corev1.ConfigMap{}
	locker.Name = "locker-2"
	if _, err := rl.AcquireLock(ctx, leaseName, locker); !errors.Is(err, ErrLockAlreadyHeld) {
		t.Errorf("AcquireLock() error = %v, want %v", err, ErrLockAlreadyHeld)
	}
}

func TestAcquireLock_ClaimExpired(t *testing.T) {
	lockerID1 := "locker-1"
	leaseName := "test-lease"

	// Create an expired lease (renewed 20 seconds ago with 15 second duration)
	expiredTime := metav1.NewMicroTime(time.Now().Add(-20 * time.Second))
	leaseDuration := int32(15)
	existingLease := &coordinationv1.Lease{
		Name:      leaseName,
		Namespace: metav1.NamespaceDefault,
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

	ctx := t.Context()
	locker := &corev1.ConfigMap{}
	locker.Name = "locker-2"
	release, err := rl.AcquireLock(ctx, leaseName, locker)
	if err != nil {
		t.Errorf("AcquireLock() error = %v, expected to claim expired lease", err)
		return
	}
	defer release(ctx) //nolint:errcheck

	lease := &coordinationv1.Lease{}
	key := types.NamespacedName{Namespace: metav1.NamespaceDefault, Name: leaseName}
	if err := client.Get(ctx, key, lease); err != nil {
		t.Fatalf("Get lease error = %v", err)
	}

	const lockerIDPrefix = "ConfigMap/locker-2/"
	if lease.Spec.HolderIdentity == nil || !strings.HasPrefix(*lease.Spec.HolderIdentity, lockerIDPrefix) {
		t.Errorf("Lease holder = %v, want prefix %q", lease.Spec.HolderIdentity, lockerIDPrefix)
	}
}

func TestReleaseLock(t *testing.T) {
	lockerID := "locker-1"
	leaseName := "test-lease"

	now := metav1.NewMicroTime(time.Now())
	leaseDuration := int32(15)
	existingLease := &coordinationv1.Lease{
		Name:      leaseName,
		Namespace: metav1.NamespaceDefault,
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

	ctx := t.Context()
	if err := rl.releaseLock(ctx, leaseName, lockerID); err != nil {
		t.Errorf("releaseLock() error = %v", err)
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
		Name:      leaseName,
		Namespace: metav1.NamespaceDefault,
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

	ctx := t.Context()
	if err := rl.releaseLock(ctx, leaseName, lockerID2); err != nil {
		t.Errorf("releaseLock() error = %v, expected success (noop) when not owned", err)
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

	ctx, cancel := context.WithCancel(t.Context())
	rl.cancelFuncs.Store("locker-1", cancel)
	if err := rl.releaseLock(ctx, "non-existent-lease", "locker-1"); err != nil {
		t.Errorf("releaseLock() error = %v, expected success (noop) when lease not found", err)
	}
	select {
	case <-ctx.Done():
	default:
		t.Error("releaseLock() did not cancel renewal when lease was not found")
	}
}

func TestRenewLease(t *testing.T) {
	lockerID := "locker-1"
	leaseName := "test-lease"

	oldTime := metav1.NewMicroTime(time.Now().Add(-10 * time.Second))
	leaseDuration := int32(15)
	existingLease := &coordinationv1.Lease{
		Name:      leaseName,
		Namespace: metav1.NamespaceDefault,
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

	ctx := t.Context()
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
		Name:      leaseName,
		Namespace: metav1.NamespaceDefault,
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

	ctx := t.Context()
	if err := rl.renewLease(ctx, leaseName, lockerID2); err == nil {
		t.Errorf("renewLease() expected error when not owner, got nil")
	}
}

func TestReleaseLock_CancelsRenewalGoroutine(t *testing.T) {
	leaseName := "test-lease"
	client := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()

	rl, err := NewResourceLocker(client, metav1.NamespaceDefault, 15*time.Second, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("NewResourceLocker() error = %v", err)
	}
	defer rl.Close()

	ctx := t.Context()
	locker := &corev1.ConfigMap{}
	locker.Name = "test-locker"
	release, err := rl.AcquireLock(ctx, leaseName, locker)
	if err != nil {
		t.Fatalf("AcquireLock() error = %v", err)
	}

	lease := &coordinationv1.Lease{}
	key := types.NamespacedName{Namespace: metav1.NamespaceDefault, Name: leaseName}
	if err := client.Get(ctx, key, lease); err != nil {
		t.Fatalf("Get lease error = %v", err)
	}
	if lease.Spec.HolderIdentity == nil {
		t.Fatal("Lease holder is nil")
	}
	lockerID := *lease.Spec.HolderIdentity

	if _, exists := rl.cancelFuncs.Load(lockerID); !exists {
		t.Error("Expected cancel func to be stored after AcquireLock")
	}

	if err := release(ctx); err != nil {
		t.Fatalf("release() error = %v", err)
	}

	if _, exists := rl.cancelFuncs.Load(lockerID); exists {
		t.Error("Expected cancel func to be removed after release")
	}
}
