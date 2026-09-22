// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package overlay

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	overlayv1alpha1 "github.com/ironcore-dev/network-operator/api/overlay/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/conditions"
)

// NetworkReconciler reconciles a Network object
type NetworkReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// WatchFilterValue is the label value used to filter events prior to reconciliation.
	WatchFilterValue string

	// Recorder is used to record events for the controller.
	// More info: https://book.kubebuilder.io/reference/raising-events
	Recorder events.EventRecorder
}

// +kubebuilder:rbac:groups=overlay.networking.metal.ironcore.dev,resources=networks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=overlay.networking.metal.ironcore.dev,resources=networks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=overlay.networking.metal.ironcore.dev,resources=networks/finalizers,verbs=update
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.24.1/pkg/reconcile
func (r *NetworkReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling resource")

	net := new(overlayv1alpha1.Network)
	if err := r.Get(ctx, req.NamespacedName, net); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get resource")
		return ctrl.Result{}, err
	}

	if !net.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(net, overlayv1alpha1.FinalizerName) {
			if err := r.finalize(ctx, net); err != nil {
				log.Error(err, "Failed to finalize resource")
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(net, overlayv1alpha1.FinalizerName)
			if err := r.Update(ctx, net); err != nil {
				log.Error(err, "Failed to remove finalizer from resource")
				return ctrl.Result{}, err
			}
		}
		log.Info("Resource is being deleted, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers
	if !controllerutil.ContainsFinalizer(net, overlayv1alpha1.FinalizerName) {
		controllerutil.AddFinalizer(net, overlayv1alpha1.FinalizerName)
		if err := r.Update(ctx, net); err != nil {
			log.Error(err, "Failed to add finalizer to resource")
			return ctrl.Result{}, err
		}
		log.Info("Added finalizer to resource")
		return ctrl.Result{}, nil
	}

	orig := net.DeepCopy()
	if conditions.InitializeConditions(net, v1alpha1.ReadyCondition) {
		log.V(1).Info("Initializing status conditions")
		return ctrl.Result{}, r.Status().Update(ctx, net)
	}

	defer func() {
		if !equality.Semantic.DeepEqual(orig.Status, net.Status) {
			// Pass obj.DeepCopy() to avoid Patch() modifying obj and interfering with metadata update below
			if err := r.Status().Patch(ctx, net.DeepCopy(), client.MergeFrom(orig)); err != nil {
				log.Error(err, "Failed to update status")
				reterr = kerrors.NewAggregate([]error{reterr, err})
			}
		}
		if !equality.Semantic.DeepEqual(orig.ObjectMeta, net.ObjectMeta) {
			if err := r.Patch(ctx, net, client.MergeFrom(orig)); err != nil {
				log.Error(err, "Failed to update resource metadata")
				reterr = kerrors.NewAggregate([]error{reterr, err})
			}
		}
	}()

	res, err := r.reconcile(ctx, net)
	if err != nil {
		log.Error(err, "Failed to reconcile resource")
		return ctrl.Result{}, err
	}

	return res, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NetworkReconciler) SetupWithManager(mgr ctrl.Manager) error {
	labelSelector := metav1.LabelSelector{}
	if r.WatchFilterValue != "" {
		labelSelector.MatchLabels = map[string]string{v1alpha1.WatchLabel: r.WatchFilterValue}
	}

	filter, err := predicate.LabelSelectorPredicate(labelSelector)
	if err != nil {
		return fmt.Errorf("failed to create label selector predicate: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&overlayv1alpha1.Network{}).
		Named("overlay-network").
		WithEventFilter(filter).
		Complete(r)
}

func (r *NetworkReconciler) reconcile(_ context.Context, net *overlayv1alpha1.Network) (ctrl.Result, error) { //nolint:unparam
	conditions.RecomputeReady(net)
	return ctrl.Result{}, nil
}

func (r *NetworkReconciler) finalize(context.Context, *overlayv1alpha1.Network) error {
	return nil
}
