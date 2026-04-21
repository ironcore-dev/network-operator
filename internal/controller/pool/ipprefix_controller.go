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

// IPPrefixReconciler reconciles an IPPrefix object
type IPPrefixReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=pool.networking.metal.ironcore.dev,resources=ipprefixes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=pool.networking.metal.ironcore.dev,resources=ipprefixes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=pool.networking.metal.ironcore.dev,resources=ipprefixpools,verbs=get;list;watch

func (r *IPPrefixReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling resource")

	pfx := new(poolv1alpha1.IPPrefix)
	if err := r.Get(ctx, req.NamespacedName, pfx); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get resource")
		return ctrl.Result{}, err
	}

	if !pfx.DeletionTimestamp.IsZero() {
		log.Info("Resource is being deleted, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	orig := pfx.DeepCopy()
	defer func() {
		if !equality.Semantic.DeepEqual(orig.ObjectMeta, pfx.ObjectMeta) {
			if err := r.Patch(ctx, pfx.DeepCopy(), client.MergeFrom(orig)); err != nil {
				log.Error(err, "Failed to update resource metadata")
				reterr = kerrors.NewAggregate([]error{reterr, err})
			}
		}
		if !equality.Semantic.DeepEqual(orig.Status, pfx.Status) {
			if err := r.Status().Patch(ctx, pfx, client.MergeFrom(orig)); err != nil {
				log.Error(err, "Failed to update status")
				reterr = kerrors.NewAggregate([]error{reterr, err})
			}
		}
	}()

	pool := new(poolv1alpha1.IPPrefixPool)
	if err := r.Get(ctx, client.ObjectKey{Name: pfx.Spec.PoolRef.Name, Namespace: pfx.Namespace}, pool); err != nil {
		if apierrors.IsNotFound(err) {
			conditions.Set(pfx, metav1.Condition{
				Type:    poolv1alpha1.ValidCondition,
				Status:  metav1.ConditionFalse,
				Reason:  poolv1alpha1.PoolNotFoundForValidationReason,
				Message: fmt.Sprintf("Referenced pool %q not found", pfx.Spec.PoolRef.Name),
			})
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Ensure the pool owns this allocation for garbage collection on pool deletion.
	if err := controllerutil.SetOwnerReference(pool, pfx, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	candidate := pfx.Spec.Prefix.Masked()
	if !candidate.IsValid() {
		conditions.Set(pfx, metav1.Condition{
			Type:    poolv1alpha1.ValidCondition,
			Status:  metav1.ConditionFalse,
			Reason:  poolv1alpha1.ValueOutOfRangeReason,
			Message: fmt.Sprintf("Prefix %q is invalid", pfx.Spec.Prefix),
		})
		return ctrl.Result{}, nil
	}

	inRange := false
	for _, prefix := range pool.Spec.Prefixes {
		if int32(candidate.Bits()) == prefix.PrefixLength && prefix.Prefix.Masked().Contains(candidate.Addr()) { // #nosec G115
			inRange = true
			break
		}
	}

	if inRange {
		conditions.Set(pfx, metav1.Condition{
			Type:    poolv1alpha1.ValidCondition,
			Status:  metav1.ConditionTrue,
			Reason:  poolv1alpha1.ValueInRangeReason,
			Message: fmt.Sprintf("Prefix %q is within range of pool %q", pfx.Spec.Prefix, pfx.Spec.PoolRef.Name),
		})
	} else {
		conditions.Set(pfx, metav1.Condition{
			Type:    poolv1alpha1.ValidCondition,
			Status:  metav1.ConditionFalse,
			Reason:  poolv1alpha1.ValueOutOfRangeReason,
			Message: fmt.Sprintf("Prefix %q is out of range for pool %q", pfx.Spec.Prefix, pfx.Spec.PoolRef.Name),
		})
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *IPPrefixReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(ctx, &poolv1alpha1.IPPrefix{}, poolRefIndexKey, func(obj client.Object) []string {
		return []string{obj.(*poolv1alpha1.IPPrefix).Spec.PoolRef.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&poolv1alpha1.IPPrefix{}).
		Named("pool-ipprefix").
		// Watches enqueues IPPrefix objects based on changes to their referenced IPPrefixPool.
		// Triggers on create, spec update, and delete events since the pool's prefixes determine validity.
		Watches(
			&poolv1alpha1.IPPrefixPool{},
			handler.EnqueueRequestsFromMapFunc(r.ipPrefixesForPool),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Complete(r)
}

// ipPrefixesForPool maps an IPPrefixPool to all IPPrefix objects that reference it.
func (r *IPPrefixReconciler) ipPrefixesForPool(ctx context.Context, obj client.Object) []reconcile.Request {
	log := ctrl.LoggerFrom(ctx, "IPPrefixPool", klog.KObj(obj))

	prefixes := &poolv1alpha1.IPPrefixList{}
	if err := r.List(ctx, prefixes, client.InNamespace(obj.GetNamespace()), client.MatchingFields{poolRefIndexKey: obj.GetName()}); err != nil {
		log.Error(err, "Failed to list IPPrefix objects")
		return nil
	}

	requests := make([]reconcile.Request, len(prefixes.Items))
	for i, pfx := range prefixes.Items {
		log.Info("Enqueuing IPPrefix for reconciliation", "IPPrefix", klog.KObj(&pfx))
		requests[i] = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      pfx.Name,
				Namespace: pfx.Namespace,
			},
		}
	}
	return requests
}
