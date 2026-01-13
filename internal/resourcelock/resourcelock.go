// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

// Package resourcelock provides utilities for locking Kubernetes resources using Leases.
package resourcelock

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;delete

// ErrLockAlreadyHeld is returned when a lock is held by another locker.
var ErrLockAlreadyHeld = errors.New("resourcelock: lock is held by another locker")

// ResourceLocker provides methods to acquire and release locks on Kubernetes resources using Leases.
// Locks are implemented using coordination.k8s.io/v1 Lease resources and are automatically
// renewed in the background until the context is cancelled or ReleaseLock is called.
type ResourceLocker struct {
	client               client.Client
	leaseDurationSeconds int32
	renewPeriod          time.Duration
	namespace            string
	cancelFuncs          sync.Map // map[string]context.CancelFunc
}

// NewResourceLocker creates a new ResourceLocker with the given configuration.
// The namespace specifies where Lease resources will be created.
// The renewPeriod must be shorter than leaseDuration to ensure the lease is renewed before expiration.
func NewResourceLocker(c client.Client, namespace string, leaseDuration, renewPeriod time.Duration) (*ResourceLocker, error) {
	if renewPeriod >= leaseDuration {
		return nil, fmt.Errorf("resourcelock: renewPeriod (%v) must be shorter than leaseDuration (%v)", renewPeriod, leaseDuration)
	}
	return &ResourceLocker{
		client:               c,
		leaseDurationSeconds: int32(leaseDuration.Seconds()),
		renewPeriod:          renewPeriod,
		namespace:            namespace,
	}, nil
}

// AcquireLock tries to acquire a lock on the specified Kubernetes Lease.
// It creates or updates a lease resource and starts a background goroutine to renew
// the lease until the context is cancelled. The name parameter specifies the name of the lease
// resource to use for locking, which identifies the resource being locked. Multiple callers using
// the same name compete for the same lock. The lockerID parameter is a unique identifier for the
// specific lock holder (e.g., reconciler, caller), which identifies who is attempting to acquire
// or currently holds the lock. Returns ErrLockAlreadyHeld if the lock is currently held
// by another locker.
func (rl *ResourceLocker) AcquireLock(ctx context.Context, name, lockerID string) error {
	log := ctrl.LoggerFrom(ctx).WithValues("namespace", rl.namespace, "lease", name, "locker", lockerID)

	now := metav1.NewMicroTime(time.Now())

	lease := &coordinationv1.Lease{}
	if err := rl.client.Get(ctx, client.ObjectKey{Namespace: rl.namespace, Name: name}, lease); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("resourcelock: failed to get lease: %w", err)
		}

		// Lease doesn't exist, create it
		lease = &coordinationv1.Lease{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: rl.namespace,
			},
			Spec: coordinationv1.LeaseSpec{
				HolderIdentity:       &lockerID,
				LeaseDurationSeconds: &rl.leaseDurationSeconds,
				AcquireTime:          &now,
				RenewTime:            &now,
			},
		}

		if err := rl.client.Create(ctx, lease); err != nil {
			if apierrors.IsAlreadyExists(err) {
				log.V(2).Info("Lease was created by another locker")
				return ErrLockAlreadyHeld
			}
			return fmt.Errorf("resourcelock: failed to create lease: %w", err)
		}

		log.Info("Lock acquired, lease created")
		rl.startRenewal(ctx, name, lockerID)
		return nil
	}

	// Lease exists - check if we already own it
	if lease.Spec.HolderIdentity != nil && *lease.Spec.HolderIdentity == lockerID {
		log.V(2).Info("Lock already held by this locker")
		rl.startRenewal(ctx, name, lockerID)
		return nil
	}

	// Check if lease is still valid (not expired)
	if lease.Spec.RenewTime != nil && lease.Spec.LeaseDurationSeconds != nil {
		expirationTime := lease.Spec.RenewTime.Add(time.Duration(*lease.Spec.LeaseDurationSeconds) * time.Second)
		if time.Now().Before(expirationTime) {
			log.V(2).Info("Lock held by another locker", "currentLocker", *lease.Spec.HolderIdentity)
			return ErrLockAlreadyHeld
		}
	}

	previousLocker := ""
	if lease.Spec.HolderIdentity != nil {
		previousLocker = *lease.Spec.HolderIdentity
	}

	lease.Spec.HolderIdentity = &lockerID
	lease.Spec.LeaseDurationSeconds = &rl.leaseDurationSeconds
	lease.Spec.AcquireTime = &now
	lease.Spec.RenewTime = &now

	if err := rl.client.Update(ctx, lease); err != nil {
		if apierrors.IsConflict(err) {
			log.V(2).Info("Lease was claimed by another locker")
			return ErrLockAlreadyHeld
		}
		return fmt.Errorf("resourcelock: failed to update lease: %w", err)
	}

	log.Info("Lock acquired, claimed expired lease", "previousLocker", previousLocker)
	rl.startRenewal(ctx, name, lockerID)
	return nil
}

// ReleaseLock releases the lock on the specified Kubernetes Lease.
// The name parameter specifies the name of the lease to release. The lockerID parameter is the
// unique identifier of the lock holder attempting to release the lock. If the current holder
// does not match the provided lockerID, the lease is not deleted.
func (rl *ResourceLocker) ReleaseLock(ctx context.Context, name, lockerID string) error {
	log := ctrl.LoggerFrom(ctx).WithValues("namespace", rl.namespace, "lease", name, "locker", lockerID)

	lease := &coordinationv1.Lease{}
	if err := rl.client.Get(ctx, client.ObjectKey{Namespace: rl.namespace, Name: name}, lease); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(2).Info("Lease not found, nothing to release")
			return nil
		}
		return fmt.Errorf("resourcelock: failed to get lease: %w", err)
	}

	if lease.Spec.HolderIdentity != nil && *lease.Spec.HolderIdentity != lockerID {
		log.V(2).Info("Not the current locker, skipping release", "currentLocker", *lease.Spec.HolderIdentity)
		return nil
	}

	if err := rl.client.Delete(ctx, lease); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(2).Info("Lease already deleted")
			return nil
		}
		return fmt.Errorf("resourcelock: failed to delete lease: %w", err)
	}

	if cancel, ok := rl.cancelFuncs.LoadAndDelete(name); ok {
		cancel.(context.CancelFunc)()
		log.V(2).Info("Stopped renewal goroutine")
	}

	log.Info("Lock released")
	return nil
}

// startRenewal starts a background goroutine to renew the lease.
// If a renewal goroutine is already running for this lock, it will be cancelled first.
func (rl *ResourceLocker) startRenewal(ctx context.Context, name, lockerID string) {
	if oldCancel, loaded := rl.cancelFuncs.Load(name); loaded {
		oldCancel.(context.CancelFunc)()
	}

	renewCtx, cancel := context.WithCancel(ctx)
	rl.cancelFuncs.Store(name, cancel)

	go rl.renewUntilContextDone(renewCtx, name, lockerID)
}

// Start implements manager.Runnable and blocks until the context is cancelled.
// When the context is cancelled, it calls Close to cancel all active renewal goroutines.
func (rl *ResourceLocker) Start(ctx context.Context) error {
	<-ctx.Done()
	rl.Close()
	return nil
}

// Close cancels all active renewal goroutines.
// This should be called when the ResourceLocker is no longer needed to ensure proper cleanup.
func (rl *ResourceLocker) Close() {
	rl.cancelFuncs.Range(func(key, value any) bool {
		if cancel, ok := value.(context.CancelFunc); ok {
			cancel()
		}
		rl.cancelFuncs.Delete(key)
		return true
	})
}

// renewUntilContextDone periodically renews the lease until the context is cancelled.
func (rl *ResourceLocker) renewUntilContextDone(ctx context.Context, name, lockerID string) {
	log := ctrl.LoggerFrom(ctx).WithValues("namespace", rl.namespace, "lease", name, "locker", lockerID)

	ticker := time.NewTicker(rl.renewPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := rl.renewLease(ctx, name, lockerID); err != nil {
				if apierrors.IsNotFound(err) {
					log.V(2).Info("Lease not found during renewal, stopping renewal goroutine")
					return
				}
				log.Error(err, "Failed to renew lease")
			}
		case <-ctx.Done():
			return
		}
	}
}

// renewLease updates the RenewTime of the lease to extend its validity for the current holder.
func (rl *ResourceLocker) renewLease(ctx context.Context, name, lockerID string) error {
	lease := &coordinationv1.Lease{}
	if err := rl.client.Get(ctx, client.ObjectKey{Namespace: rl.namespace, Name: name}, lease); err != nil {
		return fmt.Errorf("resourcelock: failed to get lease for renewal: %w", err)
	}

	if lease.Spec.HolderIdentity == nil || *lease.Spec.HolderIdentity != lockerID {
		return fmt.Errorf("resourcelock: no longer the holder of lease %s/%s", rl.namespace, name)
	}

	now := metav1.NewMicroTime(time.Now())
	lease.Spec.RenewTime = &now

	if err := rl.client.Update(ctx, lease); err != nil {
		return fmt.Errorf("resourcelock: failed to update lease: %w", err)
	}

	return nil
}
