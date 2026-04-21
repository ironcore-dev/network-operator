// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package pool

import (
	"context"
	"strconv"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	poolv1alpha1 "github.com/ironcore-dev/network-operator/api/pool/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/conditions"
)

// IndexPoolReconciler reconciles an IndexPool object
type IndexPoolReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=pool.networking.metal.ironcore.dev,resources=indexpools,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=pool.networking.metal.ironcore.dev,resources=indexpools/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=pool.networking.metal.ironcore.dev,resources=indices,verbs=get;list;watch

func (r *IndexPoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling resource")

	pool := new(poolv1alpha1.IndexPool)
	if err := r.Get(ctx, req.NamespacedName, pool); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get resource")
		return ctrl.Result{}, err
	}

	if !pool.DeletionTimestamp.IsZero() {
		log.Info("Resource is being deleted, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	orig := pool.DeepCopy()
	defer func() {
		if !equality.Semantic.DeepEqual(orig.ObjectMeta, pool.ObjectMeta) {
			if err := r.Patch(ctx, pool.DeepCopy(), client.MergeFrom(orig)); err != nil {
				log.Error(err, "Failed to update resource metadata")
				reterr = kerrors.NewAggregate([]error{reterr, err})
			}
		}
		if !equality.Semantic.DeepEqual(orig.Status, pool.Status) {
			if err := r.Status().Patch(ctx, pool, client.MergeFrom(orig)); err != nil {
				log.Error(err, "Failed to update status")
				reterr = kerrors.NewAggregate([]error{reterr, err})
			}
		}
	}()

	indices := &poolv1alpha1.IndexList{}
	if err := r.List(
		ctx, indices,
		client.InNamespace(pool.Namespace),
		client.MatchingFields{poolRefIndexKey: pool.Name},
	); err != nil {
		return ctrl.Result{}, err
	}

	pool.Status.Allocated = int64(len(indices.Items))
	pool.Status.Total = strconv.FormatInt(pool.Total(), 10)

	if pool.IsExhausted() {
		conditions.Set(pool, metav1.Condition{
			Type:    poolv1alpha1.AvailableCondition,
			Status:  metav1.ConditionFalse,
			Reason:  poolv1alpha1.ExhaustedReason,
			Message: "Pool has no free capacity",
		})
	} else {
		conditions.Set(pool, metav1.Condition{
			Type:    poolv1alpha1.AvailableCondition,
			Status:  metav1.ConditionTrue,
			Reason:  poolv1alpha1.HasCapacityReason,
			Message: "Pool has free capacity",
		})
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *IndexPoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&poolv1alpha1.IndexPool{}).
		Named("pool-indexpool").
		// Watches enqueues IndexPools based on updates of contained Index resources.
		// Only triggers on create and delete events since poolRefs are immutable.
		Watches(
			&poolv1alpha1.Index{},
			handler.EnqueueRequestsFromMapFunc(func(_ context.Context, obj client.Object) []reconcile.Request {
				return []reconcile.Request{{
					NamespacedName: types.NamespacedName{
						Name:      obj.(*poolv1alpha1.Index).Spec.PoolRef.Name,
						Namespace: obj.GetNamespace(),
					},
				}}
			}),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					return false
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			}),
		).
		Complete(r)
}
