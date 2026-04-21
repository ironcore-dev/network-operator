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

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	poolv1alpha1 "github.com/ironcore-dev/network-operator/api/pool/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/conditions"
)

// ClaimReconciler reconciles a Claim object
type ClaimReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=pool.networking.metal.ironcore.dev,resources=claims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=pool.networking.metal.ironcore.dev,resources=claims/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=pool.networking.metal.ironcore.dev,resources=claims/finalizers,verbs=update
// +kubebuilder:rbac:groups=pool.networking.metal.ironcore.dev,resources=indices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=pool.networking.metal.ironcore.dev,resources=indexpools,verbs=get;list;watch
// +kubebuilder:rbac:groups=pool.networking.metal.ironcore.dev,resources=ipaddresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=pool.networking.metal.ironcore.dev,resources=ipaddresspools,verbs=get;list;watch
// +kubebuilder:rbac:groups=pool.networking.metal.ironcore.dev,resources=ipprefixes,verbs=get;list;watch;create;update;patch;delete
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

const (
	claimPoolRefKey  = ".spec.poolRef"
	claimRefIndexKey = ".spec.claimRef.name"
)

// errAllocationConflict is returned when a concurrent controller already created
// an allocation object with the same deterministic name. The caller should retry
// with a fresh view of existing allocations.
var errAllocationConflict = errors.New("allocation conflict")

// errMultipleAllocations is returned when more than one allocation object is
// bound to the same claim, indicating an inconsistency that requires manual cleanup.
var errMultipleAllocations = errors.New("multiple allocations bound to claim")

// SetupWithManager sets up the controller with the Manager.
func (r *ClaimReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	// Index Claims by their poolRef for pool-change watches.
	if err := mgr.GetFieldIndexer().IndexField(ctx, &poolv1alpha1.Claim{}, claimPoolRefKey, func(obj client.Object) []string {
		ref := obj.(*poolv1alpha1.Claim).Spec.PoolRef
		return []string{fmt.Sprintf("%s/%s/%s", ref.APIVersion, ref.Kind, ref.Name)}
	}); err != nil {
		return err
	}

	// Index Index/IPAddress/IPPrefix objects by claimRef.name so the claim controller
	// can efficiently look up the allocation object bound to a given claim.
	if err := mgr.GetFieldIndexer().IndexField(ctx, &poolv1alpha1.Index{}, claimRefIndexKey, func(obj client.Object) []string {
		ref := obj.(*poolv1alpha1.Index).Spec.ClaimRef
		if ref == nil {
			return nil
		}
		return []string{ref.Name}
	}); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(ctx, &poolv1alpha1.IPAddress{}, claimRefIndexKey, func(obj client.Object) []string {
		ref := obj.(*poolv1alpha1.IPAddress).Spec.ClaimRef
		if ref == nil {
			return nil
		}
		return []string{ref.Name}
	}); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(ctx, &poolv1alpha1.IPPrefix{}, claimRefIndexKey, func(obj client.Object) []string {
		ref := obj.(*poolv1alpha1.IPPrefix).Spec.ClaimRef
		if ref == nil {
			return nil
		}
		return []string{ref.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&poolv1alpha1.Claim{}).
		Named("pool-claim").
		// Watches enqueues Claims for updates in referenced IndexPool resources.
		// Triggers on create, delete, and update events when the allocated count changes.
		Watches(
			&poolv1alpha1.IndexPool{},
			handler.EnqueueRequestsFromMapFunc(r.claimsForPoolRef(poolv1alpha1.GroupVersion.WithKind("IndexPool"))),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldPool := e.ObjectOld.(*poolv1alpha1.IndexPool)
					newPool := e.ObjectNew.(*poolv1alpha1.IndexPool)
					return oldPool.Status.Allocated != newPool.Status.Allocated || oldPool.Status.Total != newPool.Status.Total
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			}),
		).
		// Watches enqueues Claims for updates in referenced IPAddressPool resources.
		// Triggers on create, delete, and update events when the allocated count changes.
		Watches(
			&poolv1alpha1.IPAddressPool{},
			handler.EnqueueRequestsFromMapFunc(r.claimsForPoolRef(poolv1alpha1.GroupVersion.WithKind("IPAddressPool"))),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldPool := e.ObjectOld.(*poolv1alpha1.IPAddressPool)
					newPool := e.ObjectNew.(*poolv1alpha1.IPAddressPool)
					return oldPool.Status.Allocated != newPool.Status.Allocated || oldPool.Status.Total != newPool.Status.Total
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			}),
		).
		// Watches enqueues Claims for updates in referenced IPPrefixPool resources.
		// Triggers on create, delete, and update events when the allocated count changes.
		Watches(
			&poolv1alpha1.IPPrefixPool{},
			handler.EnqueueRequestsFromMapFunc(r.claimsForPoolRef(poolv1alpha1.GroupVersion.WithKind("IPPrefixPool"))),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldPool := e.ObjectOld.(*poolv1alpha1.IPPrefixPool)
					newPool := e.ObjectNew.(*poolv1alpha1.IPPrefixPool)
					return oldPool.Status.Allocated != newPool.Status.Allocated || oldPool.Status.Total != newPool.Status.Total
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			}),
		).
		// Watches enqueues Claims when a pre-provisioned allocation object with the
		// allow-binding annotation is created, updated or deleted with a matching claimRef name.
		Watches(
			&poolv1alpha1.Index{},
			handler.EnqueueRequestsFromMapFunc(r.claimForAllocation),
			builder.WithPredicates(allowBindingPredicate()),
		).
		Watches(
			&poolv1alpha1.IPAddress{},
			handler.EnqueueRequestsFromMapFunc(r.claimForAllocation),
			builder.WithPredicates(allowBindingPredicate()),
		).
		Watches(
			&poolv1alpha1.IPPrefix{},
			handler.EnqueueRequestsFromMapFunc(r.claimForAllocation),
			builder.WithPredicates(allowBindingPredicate()),
		).
		Complete(r)
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

	var pool poolv1alpha1.Pool
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

	// Allocate a value from the pool. Retries on conflict when a concurrent
	// controller creates an allocation object with the same deterministic name.
	var bound poolv1alpha1.Allocation
	err = retry.OnError(retry.DefaultBackoff, func(err error) bool {
		return errors.Is(err, errAllocationConflict)
	}, func() error {
		if err := r.Get(ctx, client.ObjectKey{Name: ref.Name, Namespace: claim.Namespace}, pool); err != nil {
			return err
		}

		// Ensure the pool owns the claim for garbage collection on pool deletion.
		if err := controllerutil.SetOwnerReference(pool, claim, r.Scheme); err != nil {
			return err
		}

		// Look for an allocation object already referencing this claim by name.
		allocs, err := pool.ListAllocations(ctx, r.Client, client.InNamespace(claim.Namespace), client.MatchingFields{claimRefIndexKey: claim.Name})
		if err != nil {
			return err
		}
		var matched []poolv1alpha1.Allocation
		for _, alloc := range allocs {
			ref := alloc.ClaimRef()
			if ref == nil || ref.Name != claim.Name {
				continue
			}
			if ref.UID != claim.UID {
				// Stale UID — only rebind if the allow-binding annotation is set.
				if _, ok := alloc.GetAnnotations()[poolv1alpha1.AllowBindingAnnotation]; !ok {
					continue
				}
				alloc.SetClaimRef(&poolv1alpha1.ClaimRef{Name: claim.Name, UID: claim.UID})
				if err := r.Update(ctx, alloc); err != nil {
					return err
				}
			}
			matched = append(matched, alloc)
		}
		if len(matched) > 1 {
			return errMultipleAllocations
		}
		if len(matched) == 1 {
			bound = matched[0]
			return nil
		}

		if pool.IsExhausted() {
			return poolv1alpha1.ErrPoolExhausted
		}

		// List all allocation objects for this pool to determine used values.
		existing, err := pool.ListAllocations(ctx, r.Client, client.InNamespace(claim.Namespace), client.MatchingFields{poolRefIndexKey: pool.GetName()})
		if err != nil {
			return err
		}

		alloc, err := pool.Allocate(claim, existing)
		if err != nil {
			return err
		}

		// Set the pool as owner for garbage collection on pool deletion.
		if err := controllerutil.SetOwnerReference(pool, alloc, r.Scheme); err != nil {
			return err
		}

		// Create the allocation object. AlreadyExists means a concurrent controller
		// grabbed the same slot — retry with a fresh list.
		if err := r.Create(ctx, alloc); err != nil {
			if apierrors.IsAlreadyExists(err) {
				return errAllocationConflict
			}
			return err
		}

		bound = alloc
		return nil
	})

	switch {
	case apierrors.IsNotFound(err):
		conditions.Set(claim, metav1.Condition{
			Type:    poolv1alpha1.AllocatedCondition,
			Status:  metav1.ConditionFalse,
			Reason:  poolv1alpha1.PoolNotFoundReason,
			Message: fmt.Sprintf("Referenced pool %s not found", ref.Name),
		})
		return reconcile.TerminalError(err)
	case errors.Is(err, errMultipleAllocations):
		conditions.Set(claim, metav1.Condition{
			Type:    poolv1alpha1.AllocatedCondition,
			Status:  metav1.ConditionFalse,
			Reason:  poolv1alpha1.MultipleAllocationsReason,
			Message: "Multiple allocation objects are bound to this claim",
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
	case err != nil:
		return err
	default:
		conditions.Set(claim, metav1.Condition{
			Type:    poolv1alpha1.AllocatedCondition,
			Status:  metav1.ConditionTrue,
			Reason:  poolv1alpha1.AllocatedReason,
			Message: "Successfully allocated from pool",
		})
		claim.Status.Value = bound.Value()
		if gvks, _, err := r.Scheme.ObjectKinds(bound); err == nil && len(gvks) > 0 {
			bound.GetObjectKind().SetGroupVersionKind(gvks[0])
		}
		claim.Status.AllocationRef = v1alpha1.TypedLocalObjectRefFromObject(bound)
		return nil
	}
}

// finalize releases the allocation bound to the claim according to the pool's reclaim policy.
func (r *ClaimReconciler) finalize(ctx context.Context, claim *poolv1alpha1.Claim) error {
	ref := claim.Spec.PoolRef

	gv, err := schema.ParseGroupVersion(ref.APIVersion)
	if err != nil || gv.WithKind(ref.Kind).GroupVersion() != poolv1alpha1.GroupVersion {
		return nil //nolint:nilerr
	}

	var pool poolv1alpha1.Pool
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

	if err := r.Get(ctx, client.ObjectKey{Name: ref.Name, Namespace: claim.Namespace}, pool); err != nil {
		return client.IgnoreNotFound(err)
	}

	bound, err := pool.ListAllocations(ctx, r.Client, client.InNamespace(claim.Namespace), client.MatchingFields{claimRefIndexKey: claim.Name})
	if err != nil {
		return err
	}

	for _, alloc := range bound {
		if alloc.ClaimRef() == nil || alloc.ClaimRef().UID != claim.UID {
			continue
		}
		// Recycle: delete the allocation object to free the slot.
		if pool.ReclaimPolicy() == poolv1alpha1.ReclaimPolicyRecycle {
			return client.IgnoreNotFound(r.Delete(ctx, alloc))
		}
		// Retain: clear the claimRef so the allocation persists as reserved but unbound.
		alloc.SetClaimRef(nil)
		return r.Update(ctx, alloc)
	}

	return nil
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

// claimForAllocation enqueues the Claim referenced by an allocation object's claimRef.
func (r *ClaimReconciler) claimForAllocation(_ context.Context, obj client.Object) []reconcile.Request {
	alloc, ok := obj.(poolv1alpha1.Allocation)
	if !ok {
		return nil
	}
	ref := alloc.ClaimRef()
	if ref == nil {
		return nil
	}
	return []reconcile.Request{{
		NamespacedName: types.NamespacedName{
			Name:      ref.Name,
			Namespace: obj.GetNamespace(),
		},
	}}
}

// allowBindingPredicate filters for allocation objects that carry the allow-binding annotation.
func allowBindingPredicate() predicate.Funcs {
	hasAnnotation := func(obj client.Object) bool {
		_, ok := obj.GetAnnotations()[poolv1alpha1.AllowBindingAnnotation]
		return ok
	}
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return hasAnnotation(e.Object)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return hasAnnotation(e.ObjectNew)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return hasAnnotation(e.Object)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}
