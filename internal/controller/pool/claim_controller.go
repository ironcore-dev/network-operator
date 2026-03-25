// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package pool

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	poolv1alpha1 "github.com/ironcore-dev/network-operator/api/pool/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/conditions"
)

// ClaimReconciler reconciles a Claim object
type ClaimReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// Recorder is used to record events for the controller.
	// More info: https://book.kubebuilder.io/reference/raising-events
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=pool.networking.metal.ironcore.dev,resources=claims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=pool.networking.metal.ironcore.dev,resources=claims/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=pool.networking.metal.ironcore.dev,resources=claims/finalizers,verbs=update
// +kubebuilder:rbac:groups=pool.networking.metal.ironcore.dev,resources=indexpools,verbs=get;list;watch
// +kubebuilder:rbac:groups=pool.networking.metal.ironcore.dev,resources=indexpools/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=pool.networking.metal.ironcore.dev,resources=ipaddresspools,verbs=get;list;watch
// +kubebuilder:rbac:groups=pool.networking.metal.ironcore.dev,resources=ipaddresspools/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=pool.networking.metal.ironcore.dev,resources=ipprefixpools,verbs=get;list;watch
// +kubebuilder:rbac:groups=pool.networking.metal.ironcore.dev,resources=ipprefixpools/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ClaimReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling resource")

	claim := new(poolv1alpha1.Claim)
	if err := r.Get(ctx, req.NamespacedName, claim); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get resource")
		return ctrl.Result{}, err
	}

	if !claim.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(claim, poolv1alpha1.FinalizerName) {
			if err := r.finalize(ctx, claim); err != nil {
				log.Error(err, "Failed to finalize resource")
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(claim, poolv1alpha1.FinalizerName)
			if err := r.Update(ctx, claim); err != nil {
				log.Error(err, "Failed to remove finalizer from resource")
				return ctrl.Result{}, err
			}
		}
		log.Info("Resource is being deleted, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers
	if !controllerutil.ContainsFinalizer(claim, poolv1alpha1.FinalizerName) {
		controllerutil.AddFinalizer(claim, poolv1alpha1.FinalizerName)
		if err := r.Update(ctx, claim); err != nil {
			log.Error(err, "Failed to add finalizer to resource")
			return ctrl.Result{}, err
		}
		log.Info("Added finalizer to resource")
		return ctrl.Result{}, nil
	}

	orig := claim.DeepCopy()
	defer func() {
		if !equality.Semantic.DeepEqual(orig.ObjectMeta, claim.ObjectMeta) {
			// Pass obj.DeepCopy() to avoid Patch() modifying obj and interfering with status update below
			if err := r.Patch(ctx, claim.DeepCopy(), client.MergeFrom(orig)); err != nil {
				log.Error(err, "Failed to update resource metadata")
				reterr = kerrors.NewAggregate([]error{reterr, err})
			}
		}
		if !equality.Semantic.DeepEqual(orig.Status, claim.Status) {
			if err := r.Status().Patch(ctx, claim, client.MergeFrom(orig)); err != nil {
				log.Error(err, "Failed to update status")
				reterr = kerrors.NewAggregate([]error{reterr, err})
			}
		}
	}()

	if err := r.reconcile(ctx, claim); err != nil {
		log.Error(err, "Failed to reconcile resource")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

const claimPoolRefKey = ".spec.poolRef"

// SetupWithManager sets up the controller with the Manager.
func (r *ClaimReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(ctx, &poolv1alpha1.Claim{}, claimPoolRefKey, func(obj client.Object) []string {
		ref := obj.(*poolv1alpha1.Claim).Spec.PoolRef
		return []string{fmt.Sprintf("%s/%s/%s", ref.APIVersion, ref.Kind, ref.Name)}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&poolv1alpha1.Claim{}).
		Named("pool-claim").
		// Watches enqueues Claims for updates in referenced IndexPool resources.
		// Triggers on create, delete, and update events when the allocation status changes.
		Watches(
			&poolv1alpha1.IndexPool{},
			handler.EnqueueRequestsFromMapFunc(r.claimsForPoolRef(poolv1alpha1.GroupVersion.WithKind("IndexPool"))),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldPool := e.ObjectOld.(*poolv1alpha1.IndexPool)
					newPool := e.ObjectNew.(*poolv1alpha1.IndexPool)
					// Only trigger when Allocations status field changes.
					return !equality.Semantic.DeepEqual(oldPool.Status.Allocations, newPool.Status.Allocations)
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			}),
		).
		// Watches enqueues Claims for updates in referenced IPAddressPool resources.
		// Triggers on create, delete, and update events when the allocation status changes.
		Watches(
			&poolv1alpha1.IPAddressPool{},
			handler.EnqueueRequestsFromMapFunc(r.claimsForPoolRef(poolv1alpha1.GroupVersion.WithKind("IPAddressPool"))),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldPool := e.ObjectOld.(*poolv1alpha1.IPAddressPool)
					newPool := e.ObjectNew.(*poolv1alpha1.IPAddressPool)
					// Only trigger when Allocations status field changes.
					return !equality.Semantic.DeepEqual(oldPool.Status.Allocations, newPool.Status.Allocations)
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			}),
		).
		// Watches enqueues Claims for updates in referenced IPPrefixPool resources.
		// Triggers on create, delete, and update events when the allocation status changes.
		Watches(
			&poolv1alpha1.IPPrefixPool{},
			handler.EnqueueRequestsFromMapFunc(r.claimsForPoolRef(poolv1alpha1.GroupVersion.WithKind("IPPrefixPool"))),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldPool := e.ObjectOld.(*poolv1alpha1.IPPrefixPool)
					newPool := e.ObjectNew.(*poolv1alpha1.IPPrefixPool)
					// Only trigger when Allocations status field changes.
					return !equality.Semantic.DeepEqual(oldPool.Status.Allocations, newPool.Status.Allocations)
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			}),
		).
		Complete(r)
}

// Pool is an interface that abstracts over the different types of pools (IndexPool, IPAddressPool, IPPrefixPool) that a Claim can reference.
type Pool interface {
	client.Object

	// IsExhausted reports whether all allocatable resources in the pool are taken.
	IsExhausted() bool

	// FindAllocation returns the existing ClaimAllocation for the given claim, or nil if none exists.
	FindAllocation(claim *poolv1alpha1.Claim) *poolv1alpha1.ClaimAllocation

	// Allocate reserves the next available resource for the claim and records it in the pool status.
	// Returns ErrPoolExhausted when no resources are left.
	Allocate(claim *poolv1alpha1.Claim) (*poolv1alpha1.ClaimAllocation, error)

	// AllocatePreferred reserves the specific value given by preferred for the claim.
	// Returns ErrPreferredValueUnavailable if the value is outside the pool's configured
	// ranges/prefixes or is already taken by another claim.
	AllocatePreferred(claim *poolv1alpha1.Claim, preferred string) (*poolv1alpha1.ClaimAllocation, error)

	// Reclaim applies the pool's ReclaimPolicy for the given claim on deletion.
	Reclaim(claim *poolv1alpha1.Claim)
}

func (r *ClaimReconciler) reconcile(ctx context.Context, claim *poolv1alpha1.Claim) error {
	ref := claim.Spec.PoolRef

	gv, err := schema.ParseGroupVersion(ref.APIVersion)
	if err != nil {
		conditions.Set(claim, metav1.Condition{
			Type:    poolv1alpha1.AllocatedCondition,
			Status:  metav1.ConditionFalse,
			Reason:  poolv1alpha1.PoolRefInvalidReason,
			Message: fmt.Sprintf("Invalid apiVersion in poolRef: %v", err),
		})
		return reconcile.TerminalError(err)
	}

	if gv.WithKind(ref.Kind).GroupVersion() != poolv1alpha1.GroupVersion {
		conditions.Set(claim, metav1.Condition{
			Type:    poolv1alpha1.AllocatedCondition,
			Status:  metav1.ConditionFalse,
			Reason:  poolv1alpha1.PoolRefInvalidReason,
			Message: "PoolRef must reference a resource in apiVersion " + poolv1alpha1.GroupVersion.String(),
		})
		return reconcile.TerminalError(fmt.Errorf("poolRef apiVersion must be %s", poolv1alpha1.GroupVersion.String()))
	}

	var pool Pool
	switch ref.Kind {
	case "IndexPool":
		pool = new(poolv1alpha1.IndexPool)
	case "IPAddressPool":
		pool = new(poolv1alpha1.IPAddressPool)
	case "IPPrefixPool":
		pool = new(poolv1alpha1.IPPrefixPool)
	default:
		conditions.Set(claim, metav1.Condition{
			Type:    poolv1alpha1.AllocatedCondition,
			Status:  metav1.ConditionFalse,
			Reason:  poolv1alpha1.PoolRefInvalidReason,
			Message: "PoolRef kind must be one of IndexPool, IPAddressPool, or IPPrefixPool",
		})
		return reconcile.TerminalError(errors.New("poolRef kind must be one of IndexPool, IPAddressPool, or IPPrefixPool"))
	}

	preferred := claim.Annotations[poolv1alpha1.PreferredValueAnnotation]
	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if err := r.Get(ctx, client.ObjectKey{Name: claim.Spec.PoolRef.Name, Namespace: claim.Namespace}, pool); err != nil {
			return err
		}
		// Check if the claim is already allocated in the pool.
		if alloc := pool.FindAllocation(claim); alloc != nil {
			claim.Status.Allocation = alloc
			return nil
		}
		// If the claim has an allocation in its status that is not reflected in the pool,
		// we have an inconsistency that requires manual intervention.
		if claim.Status.Allocation != nil {
			return poolv1alpha1.ErrAllocationInconsistent
		}
		var alloc *poolv1alpha1.ClaimAllocation
		if preferred != "" {
			alloc, err = pool.AllocatePreferred(claim, preferred)
		} else {
			if pool.IsExhausted() {
				return poolv1alpha1.ErrPoolExhausted
			}
			alloc, err = pool.Allocate(claim)
		}
		if err != nil {
			return err
		}
		if err := r.Status().Update(ctx, pool); err != nil {
			return err
		}
		// Only update the claim status after successfully updating the pool status to avoid
		// inconsistencies where the claim status shows an allocation not reserved in the pool.
		claim.Status.Allocation = alloc
		return nil
	})

	switch {
	case apierrors.IsNotFound(err):
		conditions.Set(claim, metav1.Condition{
			Type:    poolv1alpha1.AllocatedCondition,
			Status:  metav1.ConditionFalse,
			Reason:  poolv1alpha1.PoolNotFoundReason,
			Message: fmt.Sprintf("Referenced pool %s not found", claim.Spec.PoolRef.Name),
		})
		return reconcile.TerminalError(err)
	case errors.Is(err, poolv1alpha1.ErrPoolExhausted):
		conditions.Set(claim, metav1.Condition{
			Type:    poolv1alpha1.AllocatedCondition,
			Status:  metav1.ConditionFalse,
			Reason:  poolv1alpha1.PoolExhaustedReason,
			Message: "Referenced pool is exhausted",
		})
		return reconcile.TerminalError(err)
	case errors.Is(err, poolv1alpha1.ErrPreferredValueUnavailable):
		conditions.Set(claim, metav1.Condition{
			Type:    poolv1alpha1.AllocatedCondition,
			Status:  metav1.ConditionFalse,
			Reason:  poolv1alpha1.PreferredValueUnavailableReason,
			Message: fmt.Sprintf("Preferred value %q is not available in pool %s; remove the annotation to allow any available value to be allocated", preferred, claim.Spec.PoolRef.Name),
		})
		return reconcile.TerminalError(err)
	case errors.Is(err, poolv1alpha1.ErrAllocationInconsistent):
		conditions.Set(claim, metav1.Condition{
			Type:    poolv1alpha1.AllocatedCondition,
			Status:  metav1.ConditionFalse,
			Reason:  poolv1alpha1.AllocationFailedReason,
			Message: fmt.Sprintf("Claim has an allocation in status that is not reflected in pool %s; manual intervention required", claim.Spec.PoolRef.Name),
		})
		return reconcile.TerminalError(err)
	case err != nil:
		return err
	default:
		conditions.Set(claim, metav1.Condition{
			Type:    poolv1alpha1.AllocatedCondition,
			Status:  metav1.ConditionTrue,
			Reason:  poolv1alpha1.AllocatedReason,
			Message: "Successfully allocated from pool",
		})
		return nil
	}
}

func (r *ClaimReconciler) finalize(ctx context.Context, claim *poolv1alpha1.Claim) error {
	ref := claim.Spec.PoolRef

	gv, err := schema.ParseGroupVersion(ref.APIVersion)
	if err != nil || gv.WithKind(ref.Kind).GroupVersion() != poolv1alpha1.GroupVersion {
		return nil //nolint:nilerr
	}

	var pool Pool
	switch ref.Kind {
	case "IndexPool":
		pool = new(poolv1alpha1.IndexPool)
	case "IPAddressPool":
		pool = new(poolv1alpha1.IPAddressPool)
	case "IPPrefixPool":
		pool = new(poolv1alpha1.IPPrefixPool)
	default:
		return nil
	}

	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if err := r.Get(ctx, client.ObjectKey{Name: ref.Name, Namespace: claim.Namespace}, pool); err != nil {
			return client.IgnoreNotFound(err)
		}
		pool.Reclaim(claim)
		return r.Status().Update(ctx, pool)
	})
}

// claimsForPoolRef returns a [handler.MapFunc] that enqueues requests for reconciliation
// for a Claim to update when one of its referenced pools gets updated.
func (r *ClaimReconciler) claimsForPoolRef(gvk schema.GroupVersionKind) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		log := ctrl.LoggerFrom(ctx, "Pool", klog.KObj(obj))

		key := fmt.Sprintf("%s/%s/%s", gvk.GroupVersion().Identifier(), gvk.Kind, obj.GetName())

		claims := &poolv1alpha1.ClaimList{}
		if err := r.List(ctx, claims, client.InNamespace(obj.GetNamespace()), client.MatchingFields{claimPoolRefKey: key}); err != nil {
			log.Error(err, "Failed to list Claims")
			return nil
		}

		var requests []reconcile.Request
		for _, claim := range claims.Items {
			log.Info("Enqueuing Claim for reconciliation", "Claim", klog.KObj(&claim))
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      claim.Name,
					Namespace: claim.Namespace,
				},
			})
		}

		return requests
	}
}
