// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package pool

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	poolv1alpha1 "github.com/ironcore-dev/network-operator/api/pool/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/conditions"
)

// IndexReconciler reconciles an Index object
type IndexReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=pool.networking.metal.ironcore.dev,resources=indices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=pool.networking.metal.ironcore.dev,resources=indices/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=pool.networking.metal.ironcore.dev,resources=indexpools,verbs=get;list;watch

func (r *IndexReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling resource")

	idx := new(poolv1alpha1.Index)
	if err := r.Get(ctx, req.NamespacedName, idx); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get resource")
		return ctrl.Result{}, err
	}

	if !idx.DeletionTimestamp.IsZero() {
		log.Info("Resource is being deleted, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	orig := idx.DeepCopy()
	defer func() {
		if !equality.Semantic.DeepEqual(orig.ObjectMeta, idx.ObjectMeta) {
			if err := r.Patch(ctx, idx.DeepCopy(), client.MergeFrom(orig)); err != nil {
				log.Error(err, "Failed to update resource metadata")
				reterr = kerrors.NewAggregate([]error{reterr, err})
			}
		}
		if !equality.Semantic.DeepEqual(orig.Status, idx.Status) {
			if err := r.Status().Patch(ctx, idx, client.MergeFrom(orig)); err != nil {
				log.Error(err, "Failed to update status")
				reterr = kerrors.NewAggregate([]error{reterr, err})
			}
		}
	}()

	pool := new(poolv1alpha1.IndexPool)
	if err := r.Get(ctx, client.ObjectKey{Name: idx.Spec.PoolRef.Name, Namespace: idx.Namespace}, pool); err != nil {
		if apierrors.IsNotFound(err) {
			conditions.Set(idx, metav1.Condition{
				Type:    poolv1alpha1.ValidCondition,
				Status:  metav1.ConditionFalse,
				Reason:  poolv1alpha1.PoolNotFoundForValidationReason,
				Message: fmt.Sprintf("Referenced pool %q not found", idx.Spec.PoolRef.Name),
			})
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Ensure the pool owns this allocation for garbage collection on pool deletion.
	if err := controllerutil.SetOwnerReference(pool, idx, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	if idx.Spec.Index < 0 {
		conditions.Set(idx, metav1.Condition{
			Type:    poolv1alpha1.ValidCondition,
			Status:  metav1.ConditionFalse,
			Reason:  poolv1alpha1.ValueOutOfRangeReason,
			Message: fmt.Sprintf("Index %d is out of range for pool %q", idx.Spec.Index, idx.Spec.PoolRef.Name),
		})
		return ctrl.Result{}, nil
	}

	value := idx.Spec.Index

	inRange := false
	for _, r := range pool.Spec.Ranges {
		if value >= r.Start && value <= r.End {
			inRange = true
			break
		}
	}

	if inRange {
		conditions.Set(idx, metav1.Condition{
			Type:    poolv1alpha1.ValidCondition,
			Status:  metav1.ConditionTrue,
			Reason:  poolv1alpha1.ValueInRangeReason,
			Message: fmt.Sprintf("Index %d is within range of pool %q", idx.Spec.Index, idx.Spec.PoolRef.Name),
		})
	} else {
		conditions.Set(idx, metav1.Condition{
			Type:    poolv1alpha1.ValidCondition,
			Status:  metav1.ConditionFalse,
			Reason:  poolv1alpha1.ValueOutOfRangeReason,
			Message: fmt.Sprintf("Index %d is out of range for pool %q", idx.Spec.Index, idx.Spec.PoolRef.Name),
		})
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *IndexReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(ctx, &poolv1alpha1.Index{}, poolRefIndexKey, func(obj client.Object) []string {
		return []string{obj.(*poolv1alpha1.Index).Spec.PoolRef.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&poolv1alpha1.Index{}).
		Named("pool-index").
		// Watches enqueues Index objects based on changes to their referenced IndexPool.
		// Triggers on create, spec update, and delete events since the pool's ranges determine validity.
		Watches(
			&poolv1alpha1.IndexPool{},
			handler.EnqueueRequestsFromMapFunc(r.indicesForPool),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Complete(r)
}

// indicesForPool maps an IndexPool to all Index objects that reference it.
func (r *IndexReconciler) indicesForPool(ctx context.Context, obj client.Object) []reconcile.Request {
	log := ctrl.LoggerFrom(ctx, "IndexPool", klog.KObj(obj))

	indices := &poolv1alpha1.IndexList{}
	if err := r.List(ctx, indices, client.InNamespace(obj.GetNamespace()), client.MatchingFields{poolRefIndexKey: obj.GetName()}); err != nil {
		log.Error(err, "Failed to list Index objects")
		return nil
	}

	requests := make([]reconcile.Request, len(indices.Items))
	for i, idx := range indices.Items {
		log.Info("Enqueuing Index for reconciliation", "Index", klog.KObj(&idx))
		requests[i] = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      idx.Name,
				Namespace: idx.Namespace,
			},
		}
	}
	return requests
}
