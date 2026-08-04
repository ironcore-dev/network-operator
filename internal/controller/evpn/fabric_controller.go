// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package evpn

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
	evpnv1alpha1 "github.com/ironcore-dev/network-operator/api/evpn/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/conditions"
	"github.com/ironcore-dev/network-operator/internal/provider"
)

// FabricReconciler reconciles a Fabric object
type FabricReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// WatchFilterValue is the label value used to filter events prior to reconciliation.
	WatchFilterValue string

	// Recorder is used to record events for the controller.
	// More info: https://book.kubebuilder.io/reference/raising-events
	Recorder events.EventRecorder

	// Provider is the driver that will be used to create & delete the interface.
	Provider provider.ProviderFunc
}

// +kubebuilder:rbac:groups=evpn.networking.metal.ironcore.dev,resources=fabrics,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=evpn.networking.metal.ironcore.dev,resources=fabrics/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=evpn.networking.metal.ironcore.dev,resources=fabrics/finalizers,verbs=update
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.2/pkg/reconcile
func (r *FabricReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling resource")

	fabric := new(evpnv1alpha1.Fabric)
	if err := r.Get(ctx, req.NamespacedName, fabric); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get resource")
		return ctrl.Result{}, err
	}

	if !fabric.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(fabric, evpnv1alpha1.FinalizerName) {
			if err := r.finalize(ctx, fabric); err != nil {
				log.Error(err, "Failed to finalize resource")
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(fabric, evpnv1alpha1.FinalizerName)
			if err := r.Update(ctx, fabric); err != nil {
				log.Error(err, "Failed to remove finalizer from resource")
				return ctrl.Result{}, err
			}
		}
		log.Info("Resource is being deleted, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers
	if !controllerutil.ContainsFinalizer(fabric, evpnv1alpha1.FinalizerName) {
		controllerutil.AddFinalizer(fabric, evpnv1alpha1.FinalizerName)
		if err := r.Update(ctx, fabric); err != nil {
			log.Error(err, "Failed to add finalizer to resource")
			return ctrl.Result{}, err
		}
		log.Info("Added finalizer to resource")
		return ctrl.Result{}, nil
	}

	orig := fabric.DeepCopy()
	if conditions.InitializeConditions(fabric, v1alpha1.ReadyCondition) {
		log.V(1).Info("Initializing status conditions")
		return ctrl.Result{}, r.Status().Update(ctx, fabric)
	}

	defer func() {
		if !equality.Semantic.DeepEqual(orig.ObjectMeta, fabric.ObjectMeta) {
			// Pass obj.DeepCopy() to avoid Patch() modifying obj and interfering with status update below
			if err := r.Patch(ctx, fabric.DeepCopy(), client.MergeFrom(orig)); err != nil {
				log.Error(err, "Failed to update resource metadata")
				reterr = kerrors.NewAggregate([]error{reterr, err})
			}
		}
		if !equality.Semantic.DeepEqual(orig.Status, fabric.Status) {
			if err := r.Status().Patch(ctx, fabric, client.MergeFrom(orig)); err != nil {
				log.Error(err, "Failed to update status")
				reterr = kerrors.NewAggregate([]error{reterr, err})
			}
		}
	}()

	res, err := r.reconcile(ctx, fabric)
	if err != nil {
		log.Error(err, "Failed to reconcile resource")
		return ctrl.Result{}, err
	}

	return res, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *FabricReconciler) SetupWithManager(mgr ctrl.Manager) error {
	labelSelector := metav1.LabelSelector{}
	if r.WatchFilterValue != "" {
		labelSelector.MatchLabels = map[string]string{v1alpha1.WatchLabel: r.WatchFilterValue}
	}

	filter, err := predicate.LabelSelectorPredicate(labelSelector)
	if err != nil {
		return fmt.Errorf("failed to create label selector predicate: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&evpnv1alpha1.Fabric{}).
		WithEventFilter(filter).
		Named("evpn-fabric").
		Complete(r)
}

// ReconcileFunc defines a function type for reconciliation phases.
// Each phase should return a non-zero Result or an error if it wants to stop the reconciliation loop.
type ReconcileFunc func(context.Context, *evpnv1alpha1.Fabric) (ctrl.Result, error)

func (r *FabricReconciler) reconcile(ctx context.Context, fabric *evpnv1alpha1.Fabric) (ctrl.Result, error) {
	phases := []ReconcileFunc{
		// r.reconcileNodes,
	}
	for _, phase := range phases {
		res, err := phase(ctx, fabric)
		if err != nil || !res.IsZero() {
			return res, err
		}
	}
	conditions.Set(fabric, metav1.Condition{
		Type:    v1alpha1.ReadyCondition,
		Status:  metav1.ConditionTrue,
		Reason:  v1alpha1.ReadyReason,
		Message: "Resource is ready",
	})
	return ctrl.Result{}, nil
}

func (r *FabricReconciler) finalize(ctx context.Context, fabric *evpnv1alpha1.Fabric) error {
	_ = ctx
	_ = fabric
	return nil
}
