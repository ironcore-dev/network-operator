// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/ironcore-dev/network-operator/api/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/deviceutil"
	"github.com/ironcore-dev/network-operator/internal/provider"
)

// InterfaceReconciler reconciles a Interface object
type InterfaceReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// WatchFilterValue is the label value used to filter events prior to reconciliation.
	WatchFilterValue string

	// Recorder is used to record events for the controller.
	// More info: https://book.kubebuilder.io/reference/raising-events
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=networking.cloud.sap,resources=interfaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.cloud.sap,resources=interfaces/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.cloud.sap,resources=interfaces/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.2/pkg/reconcile
//
// For more details about the method shape, read up here:
// - https://ahmet.im/blog/controller-pitfalls/#reconcile-method-shape
func (r *InterfaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling resource")

	obj := new(v1alpha1.Interface)
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			// If the custom resource is not found then it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			log.Info("Resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get resource")
		return ctrl.Result{}, err
	}

	if !obj.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(obj, v1alpha1.FinalizerName) {
			if err := r.finalize(ctx, obj); err != nil {
				log.Error(err, "Failed to finalize resource")
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(obj, v1alpha1.FinalizerName)
			if err := r.Update(ctx, obj); err != nil {
				log.Error(err, "Failed to remove finalizer from resource")
				return ctrl.Result{}, err
			}
		}
		log.Info("Resource is being deleted, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers
	if !controllerutil.ContainsFinalizer(obj, v1alpha1.FinalizerName) {
		controllerutil.AddFinalizer(obj, v1alpha1.FinalizerName)
		if err := r.Update(ctx, obj); err != nil {
			log.Error(err, "Failed to add finalizer to resource")
			return ctrl.Result{}, err
		}
		log.Info("Added finalizer to resource")
		return ctrl.Result{}, nil
	}

	orig := obj.DeepCopy()
	if len(obj.Status.Conditions) == 0 {
		log.Info("Initializing status conditions")
		meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionUnknown,
			Reason:  v1alpha1.ReconcilePendingReason,
			Message: "Starting reconciliation",
		})
		return ctrl.Result{}, r.Status().Update(ctx, obj)
	}

	// Always attempt to update the metadata/status after reconciliation
	defer func() {
		if !equality.Semantic.DeepEqual(orig.ObjectMeta, obj.ObjectMeta) {
			if err := r.Patch(ctx, obj, client.MergeFrom(orig)); err != nil {
				log.Error(err, "Failed to update resource metadata")
				reterr = kerrors.NewAggregate([]error{reterr, err})
			}
			return
		}

		if !equality.Semantic.DeepEqual(orig.Status, obj.Status) {
			if err := r.Status().Patch(ctx, obj, client.MergeFrom(orig)); err != nil {
				log.Error(err, "Failed to update status")
				reterr = kerrors.NewAggregate([]error{reterr, err})
			}
		}
	}()

	device, err := deviceutil.GetDeviceByName(ctx, r, obj.Namespace, obj.Spec.DeviceName)
	if err != nil {
		return  ctrl.Result{}, err
	}

	res, err := r.reconcile(ctx, device, obj)
	if err != nil {
		log.Error(err, "Failed to reconcile resource")
		return ctrl.Result{}, err
	}

	return res, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *InterfaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	labelSelector := metav1.LabelSelector{}
	if r.WatchFilterValue != "" {
		labelSelector.MatchLabels = map[string]string{v1alpha1.WatchLabel: r.WatchFilterValue}
	}

	filter, err := predicate.LabelSelectorPredicate(labelSelector)
	if err != nil {
		return fmt.Errorf("failed to create label selector predicate: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Interface{}).
		Named("interface").
		WithEventFilter(filter).
		Complete(r)
}


func (r *InterfaceReconciler) reconcile(ctx context.Context, dev *v1alpha1.Device, iface *v1alpha1.Interface) (_ ctrl.Result, reterr error) {
	if iface.Labels == nil {
		iface.Labels = make(map[string]string)
	}

	iface.Labels[v1alpha1.DeviceLabel] = iface.Name

	// Ensure the Interface is owned by the Device.
	if !controllerutil.HasControllerReference(iface) {
		if err := controllerutil.SetOwnerReference(dev, iface, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Ensure the Interface is realized on the provider.
	ifaceProvider, err := provider.GetInterfaceProvider(ctx, r, iface)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get interface provider: %w", err)
	}
	res, err := ifaceProvider.EnsureInterface(ctx, &provider.InterfaceRequest{
		Interface:      iface,
		ProviderConfig: r.GetProviderConfig(ctx, iface),
	})
	if err != nil {
		return ctrl.Result{}, err
	}

	meta.SetStatusCondition(&iface.Status.Conditions, metav1.Condition{
		Type:               v1alpha1.ReadyCondition,
		Status:             metav1.ConditionTrue,
		Reason:             v1alpha1.ReadyReason,
		Message:            "Interface configured successfully",
		ObservedGeneration: iface.Generation,
	})

	return ctrl.Result{RequeueAfter: res.RequeueAfter}, nil
}

func (r *InterfaceReconciler) finalize(ctx context.Context, obj *v1alpha1.Interface) (reterr error) {
	prov, err := provider.GetInterfaceProvider(ctx, r, obj)
	if err != nil {
		// If the provider does not implement the InterfaceProvider interface, we cannot delete the interface.
		return fmt.Errorf("provider does not implement provider.InterfaceProvider: %w", err)
	}
	return prov.DeleteInterface(ctx, &provider.InterfaceRequest{
		Interface:      obj,
		ProviderConfig: r.GetProviderConfig(ctx, obj),
	})
}

// this function should be also moved & optimized in provider.go
func (r *InterfaceReconciler) GetProviderConfig(ctx context.Context, iface *v1alpha1.Interface) *provider.ProviderConfig {
	device, err := deviceutil.GetDeviceByName(ctx, r, iface.Namespace, iface.Spec.DeviceName)
	if err != nil {
		return nil
	}

	cfg, err := provider.GetProviderConfig(ctx, r, device.Namespace, device.Spec.ProviderConfigRef)
	if err != nil {
		return nil
	}
	return cfg
}




