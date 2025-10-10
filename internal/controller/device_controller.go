// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/ironcore-dev/network-operator/api/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/clientutil"
)

const DefaultRequeueAfter = 30 * time.Second

// DeviceReconciler reconciles a Device object
type DeviceReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// WatchFilterValue is the label value used to filter events prior to reconciliation.
	WatchFilterValue string

	// Recorder is used to record events for the controller.
	// More info: https://book.kubebuilder.io/reference/raising-events
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=networking.cloud.sap,resources=devices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.cloud.sap,resources=devices/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.cloud.sap,resources=devices/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;update;list;watch
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.2/pkg/reconcile
//
// For more details about the method shape, read up here:
// - https://ahmet.im/blog/controller-pitfalls/#reconcile-method-shape
func (r *DeviceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling resource")

	obj := new(v1alpha1.Device)
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

	c := clientutil.NewClient(r.Client, req.Namespace)
	ctx = clientutil.NewContext(ctx, c)

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

	// Always attempt to update the status after reconciliation
	defer func() {
		if !equality.Semantic.DeepEqual(orig.Status, obj.Status) {
			if err := r.Status().Patch(ctx, obj, client.MergeFrom(orig)); err != nil {
				log.Error(err, "Failed to update status")
				reterr = kerrors.NewAggregate([]error{reterr, err})
			}
		}
	}()

	switch obj.Status.Phase {
	case v1alpha1.DevicePhasePending:
		if obj.Spec.Bootstrap == nil {
			// Skip provisioning if no bootstrap configuration is provided.
			obj.Status.Phase = v1alpha1.DevicePhaseActive
			return ctrl.Result{}, nil
		}

		log.Info("Device is in pending phase, starting provisioning")
		tmpl, err := c.Template(ctx, obj.Spec.Bootstrap.Template)
		if err != nil {
			log.Error(err, "Failed to get template for device provisioning")
			meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
				Type:               v1alpha1.ReadyCondition,
				Status:             metav1.ConditionFalse,
				Reason:             v1alpha1.NotReadyReason,
				Message:            fmt.Sprintf("Failed to get template for device provisioning: %v", err),
				ObservedGeneration: obj.Generation,
			})
			obj.Status.Phase = v1alpha1.DevicePhaseFailed
			r.Recorder.Event(obj, "Warning", "ProvisioningFailed", "Device provisioning failed due to template retrieval error")
			return ctrl.Result{}, err
		}
		meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
			Type:               v1alpha1.ReadyCondition,
			Status:             metav1.ConditionFalse,
			Reason:             v1alpha1.ProvisioningReason,
			Message:            "Device is being provisioned",
			ObservedGeneration: obj.Generation,
		})
		obj.Status.Phase = v1alpha1.DevicePhaseProvisioning
		r.Recorder.Event(obj, "Normal", "ProvisioningStarted", "Device provisioning has started")
		// TODO(swagner-de): Start POAP Process.
		_ = tmpl // <-- Use the template.
		return ctrl.Result{RequeueAfter: DefaultRequeueAfter}, nil

	case v1alpha1.DevicePhaseProvisioning:
		log.Info("Device is in provisioning phase, checking completion")
		// TODO(swagner-de): Check if POAP Process is complete.
		ready := true // <-- This should be replaced with actual readiness check logic.
		if !ready {
			// If the device is not ready yet, we requeue the request to check again later.
			return ctrl.Result{RequeueAfter: DefaultRequeueAfter}, nil
		}
		log.Info("Device provisioning is complete, updating status")
		meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
			Type:               v1alpha1.ReadyCondition,
			Status:             metav1.ConditionTrue,
			Reason:             v1alpha1.ReadyReason,
			Message:            "Device is ready for use",
			ObservedGeneration: obj.Generation,
		})
		obj.Status.Phase = v1alpha1.DevicePhaseActive
		r.Recorder.Event(obj, "Normal", "ProvisioningComplete", "Device provisioning has completed successfully")
		// Trigger a status update and let the controller requeue the request
		return ctrl.Result{}, nil

	case v1alpha1.DevicePhaseActive:
		if err := r.reconcile(ctx, obj); err != nil {
			log.Error(err, "Failed to reconcile resource")
			return ctrl.Result{}, err
		}

	case v1alpha1.DevicePhaseFailed:
		meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
			Type:               v1alpha1.ReadyCondition,
			Status:             metav1.ConditionFalse,
			Reason:             v1alpha1.NotReadyReason,
			Message:            "Device provisioning has failed",
			ObservedGeneration: obj.Generation,
		})

	default:
		log.Info("Device is in an unknown phase, resetting to active")
		obj.Status.Phase = v1alpha1.DevicePhaseActive
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DeviceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	labelSelector := metav1.LabelSelector{}
	if r.WatchFilterValue != "" {
		labelSelector.MatchLabels = map[string]string{v1alpha1.WatchLabel: r.WatchFilterValue}
	}

	filter, err := predicate.LabelSelectorPredicate(labelSelector)
	if err != nil {
		return fmt.Errorf("failed to create label selector predicate: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Device{}).
		Named("device").
		WithEventFilter(filter).
		// Watches enqueues Devices for referenced Secret resources.
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.secretToDevices),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		// Watches enqueues Devices for referenced ConfigMap resources.
		Watches(
			&corev1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(r.configMapToDevices),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Complete(r)
}

func (r *DeviceReconciler) reconcile(ctx context.Context, device *v1alpha1.Device) error {
	log := ctrl.LoggerFrom(ctx)

	c, ok := clientutil.FromContext(ctx)
	if !ok {
		return errors.New("failed to get controller client from context")
	}

	if ref := device.Spec.Endpoint.SecretRef; ref != nil {
		secret := new(corev1.Secret)
		if err := c.Get(ctx, client.ObjectKey{Name: ref.Name, Namespace: device.Namespace}, secret); err != nil {
			log.Error(err, "Failed to get endpoint secret for device")
			return err
		}
		if !controllerutil.ContainsFinalizer(secret, v1alpha1.FinalizerName) {
			controllerutil.AddFinalizer(secret, v1alpha1.FinalizerName)
			if err := r.Update(ctx, secret); err != nil {
				log.Error(err, "Failed to add finalizer to endpoint secret")
				return err
			}
			log.Info("Added finalizer to endpoint secret")
		}
	}

	meta.SetStatusCondition(&device.Status.Conditions, metav1.Condition{
		Type:               v1alpha1.ReadyCondition,
		Status:             metav1.ConditionTrue,
		Reason:             v1alpha1.AllResourcesReadyReason,
		Message:            "All owned resources are ready",
		ObservedGeneration: device.Generation,
	})
	return nil
}

func (r *DeviceReconciler) finalize(ctx context.Context, device *v1alpha1.Device) error {
	log := ctrl.LoggerFrom(ctx)

	c, ok := clientutil.FromContext(ctx)
	if !ok {
		return errors.New("failed to get controller client from context")
	}

	if ref := device.Spec.Endpoint.SecretRef; ref != nil {
		secret := new(corev1.Secret)
		if err := c.Get(ctx, client.ObjectKey{Name: ref.Name, Namespace: device.Namespace}, secret); err != nil {
			log.Error(err, "Failed to get endpoint secret for device")
			return err
		}
		if controllerutil.ContainsFinalizer(secret, v1alpha1.FinalizerName) {
			controllerutil.RemoveFinalizer(secret, v1alpha1.FinalizerName)
			if err := r.Update(ctx, secret); err != nil {
				log.Error(err, "Failed to remove finalizer from endpoint secret")
				return err
			}
			log.Info("Removed finalizer from endpoint secret")
		}
	}

	return nil
}

// secretToDevices is a [handler.MapFunc] to be used to enqueue requests for reconciliation
// for a Device to update when one of its referenced Secrets gets updated.
func (r *DeviceReconciler) secretToDevices(ctx context.Context, obj client.Object) []ctrl.Request {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		panic(fmt.Sprintf("Expected a Secret but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx, "Secret", klog.KObj(secret))

	devices := new(v1alpha1.DeviceList)
	if err := r.List(ctx, devices); err != nil {
		log.Error(err, "Failed to list Devices")
		return nil
	}

	requests := []ctrl.Request{}
	for _, dev := range devices.Items {
		if slices.ContainsFunc(dev.GetSecretRefs(), func(ref corev1.SecretReference) bool {
			return ref.Name == secret.Name && ref.Namespace == secret.Namespace
		}) {
			log.Info("Enqueuing Device for reconciliation", "Device", klog.KObj(&dev))
			requests = append(requests, ctrl.Request{
				NamespacedName: client.ObjectKey{
					Name:      dev.Name,
					Namespace: dev.Namespace,
				},
			})
		}
	}

	return requests
}

// configMapToDevices is a [handler.MapFunc] to be used to enqueue requests for reconciliation
// for a Device to update when one of its referenced ConfigMaps gets updated.
func (r *DeviceReconciler) configMapToDevices(ctx context.Context, obj client.Object) []ctrl.Request {
	cm, ok := obj.(*corev1.ConfigMap)
	if !ok {
		panic(fmt.Sprintf("Expected a ConfigMap but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx, "ConfigMap", klog.KObj(cm))

	devices := new(v1alpha1.DeviceList)
	if err := r.List(ctx, devices); err != nil {
		log.Error(err, "Failed to list Devices")
		return nil
	}

	requests := []ctrl.Request{}
	for _, dev := range devices.Items {
		if slices.ContainsFunc(dev.GetConfigMapRefs(), func(ref corev1.ObjectReference) bool {
			return ref.Name == cm.Name && ref.Namespace == cm.Namespace
		}) {
			log.Info("Enqueuing Device for reconciliation", "Device", klog.KObj(&dev))
			requests = append(requests, ctrl.Request{
				NamespacedName: client.ObjectKey{
					Name:      dev.Name,
					Namespace: dev.Namespace,
				},
			})
		}
	}

	return requests
}
