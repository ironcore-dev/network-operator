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

// IPAddressReconciler reconciles an IPAddress object
type IPAddressReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=pool.networking.metal.ironcore.dev,resources=ipaddresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=pool.networking.metal.ironcore.dev,resources=ipaddresses/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=pool.networking.metal.ironcore.dev,resources=ipaddresspools,verbs=get;list;watch

func (r *IPAddressReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling resource")

	ipa := new(poolv1alpha1.IPAddress)
	if err := r.Get(ctx, req.NamespacedName, ipa); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get resource")
		return ctrl.Result{}, err
	}

	if !ipa.DeletionTimestamp.IsZero() {
		log.Info("Resource is being deleted, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	orig := ipa.DeepCopy()
	defer func() {
		if !equality.Semantic.DeepEqual(orig.ObjectMeta, ipa.ObjectMeta) {
			if err := r.Patch(ctx, ipa.DeepCopy(), client.MergeFrom(orig)); err != nil {
				log.Error(err, "Failed to update resource metadata")
				reterr = kerrors.NewAggregate([]error{reterr, err})
			}
		}
		if !equality.Semantic.DeepEqual(orig.Status, ipa.Status) {
			if err := r.Status().Patch(ctx, ipa, client.MergeFrom(orig)); err != nil {
				log.Error(err, "Failed to update status")
				reterr = kerrors.NewAggregate([]error{reterr, err})
			}
		}
	}()

	pool := new(poolv1alpha1.IPAddressPool)
	if err := r.Get(ctx, client.ObjectKey{Name: ipa.Spec.PoolRef.Name, Namespace: ipa.Namespace}, pool); err != nil {
		if apierrors.IsNotFound(err) {
			conditions.Set(ipa, metav1.Condition{
				Type:    poolv1alpha1.ValidCondition,
				Status:  metav1.ConditionFalse,
				Reason:  poolv1alpha1.PoolNotFoundForValidationReason,
				Message: fmt.Sprintf("Referenced pool %q not found", ipa.Spec.PoolRef.Name),
			})
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Ensure the pool owns this allocation for garbage collection on pool deletion.
	if err := controllerutil.SetOwnerReference(pool, ipa, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	addr := ipa.Spec.Address
	if !addr.IsValid() {
		conditions.Set(ipa, metav1.Condition{
			Type:    poolv1alpha1.ValidCondition,
			Status:  metav1.ConditionFalse,
			Reason:  poolv1alpha1.ValueOutOfRangeReason,
			Message: fmt.Sprintf("Address %q is not a valid IP address", addr),
		})
		return ctrl.Result{}, nil
	}

	inRange := false
	for _, prefix := range pool.Spec.Prefixes {
		if prefix.Masked().Contains(addr.Addr) {
			inRange = true
			break
		}
	}

	if inRange {
		conditions.Set(ipa, metav1.Condition{
			Type:    poolv1alpha1.ValidCondition,
			Status:  metav1.ConditionTrue,
			Reason:  poolv1alpha1.ValueInRangeReason,
			Message: fmt.Sprintf("Address %q is within range of pool %q", addr, ipa.Spec.PoolRef.Name),
		})
	} else {
		conditions.Set(ipa, metav1.Condition{
			Type:    poolv1alpha1.ValidCondition,
			Status:  metav1.ConditionFalse,
			Reason:  poolv1alpha1.ValueOutOfRangeReason,
			Message: fmt.Sprintf("Address %q is out of range for pool %q", addr, ipa.Spec.PoolRef.Name),
		})
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *IPAddressReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(ctx, &poolv1alpha1.IPAddress{}, poolRefIndexKey, func(obj client.Object) []string {
		return []string{obj.(*poolv1alpha1.IPAddress).Spec.PoolRef.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&poolv1alpha1.IPAddress{}).
		Named("pool-ipaddress").
		// Watches enqueues IPAddress objects based on changes to their referenced IPAddressPool.
		// Triggers on create, spec update, and delete events since the pool's prefixes determine validity.
		Watches(
			&poolv1alpha1.IPAddressPool{},
			handler.EnqueueRequestsFromMapFunc(r.ipAddressesForPool),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Complete(r)
}

// ipAddressesForPool maps an IPAddressPool to all IPAddress objects that reference it.
func (r *IPAddressReconciler) ipAddressesForPool(ctx context.Context, obj client.Object) []reconcile.Request {
	log := ctrl.LoggerFrom(ctx, "IPAddressPool", klog.KObj(obj))

	addresses := &poolv1alpha1.IPAddressList{}
	if err := r.List(ctx, addresses, client.InNamespace(obj.GetNamespace()), client.MatchingFields{poolRefIndexKey: obj.GetName()}); err != nil {
		log.Error(err, "Failed to list IPAddress objects")
		return nil
	}

	requests := make([]reconcile.Request, len(addresses.Items))
	for i, ipa := range addresses.Items {
		log.Info("Enqueuing IPAddress for reconciliation", "IPAddress", klog.KObj(&ipa))
		requests[i] = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      ipa.Name,
				Namespace: ipa.Namespace,
			},
		}
	}
	return requests
}
