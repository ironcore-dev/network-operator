// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sync"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	toolscache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/ironcore-dev/network-operator/api/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/clientutil"
)

// ResourceSetReconciler reconciles a ResourceSet object
type ResourceSetReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Cache  cache.Cache

	// WatchFilterValue is the label value used to filter events prior to reconciliation.
	WatchFilterValue string

	// Recorder is used to record events for the controller.
	// More info: https://book.kubebuilder.io/reference/raising-events
	Recorder record.EventRecorder

	watchedGKVs map[schema.GroupVersionKind]struct{}
	events      chan<- event.TypedGenericEvent[*v1alpha1.ResourceSet]
	mu          sync.Mutex
}

// +kubebuilder:rbac:groups=networking.cloud.sap,resources=resourcesets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.cloud.sap,resources=resourcesets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.cloud.sap,resources=resourcesets/finalizers,verbs=update
// +kubebuilder:rbac:groups=networking.cloud.sap,resources=devices,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.cloud.sap,resources=*,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.2/pkg/reconcile
//
// For more details about the method shape, read up here:
// - https://ahmet.im/blog/controller-pitfalls/#reconcile-method-shape
func (r *ResourceSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling resource")

	obj := new(v1alpha1.ResourceSet)
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

	ctx = clientutil.IntoContext(ctx, r.Client, obj.Namespace)

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

	if err := r.reconcile(ctx, obj); err != nil {
		log.Error(err, "Failed to reconcile resource")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ResourceSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Cache = mgr.GetCache()
	r.watchedGKVs = make(map[schema.GroupVersionKind]struct{})

	ch := make(chan event.TypedGenericEvent[*v1alpha1.ResourceSet])
	r.events = ch

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ResourceSet{}).
		Named("resourceset").
		// Watches enqueues ResourceSets for referenced Device resources.
		Watches(&v1alpha1.Device{}, handler.EnqueueRequestsFromMapFunc(r.deviceToResourceSet)).
		// Watches events from the channel and enqueues ResourceSets for reconciliation.
		WatchesRawSource(source.Channel(ch, &handler.TypedEnqueueRequestForObject[*v1alpha1.ResourceSet]{})).
		Complete(r)
}

func (r *ResourceSetReconciler) reconcile(ctx context.Context, res *v1alpha1.ResourceSet) error {
	log := ctrl.LoggerFrom(ctx)

	selector, err := metav1.LabelSelectorAsSelector(&res.Spec.Selector)
	if err != nil {
		log.Error(err, "Failed to convert label selector", "selector", res.Spec.Selector)
		return err
	}

	list := &v1alpha1.DeviceList{}
	if err := r.List(ctx, list, &client.ListOptions{LabelSelector: selector, Namespace: res.Namespace}); err != nil {
		log.Error(err, "Failed to list devices for ResourceSet", "selector", selector)
		return err
	}

	if len(list.Items) == 0 {
		log.Info("No devices found for ResourceSet", "selector", selector)
		r.Recorder.Eventf(res, "Warning", "NoDevicesFound", "No devices found matching the selector %s", selector)
		meta.SetStatusCondition(&res.Status.Conditions, metav1.Condition{
			Type:               v1alpha1.ReadyCondition,
			Status:             metav1.ConditionFalse,
			Reason:             v1alpha1.NoDevicesFoundReason,
			Message:            "No devices found matching the selector",
			ObservedGeneration: res.Generation,
		})
		return nil
	}

	keep := make(map[int]struct{})
	for _, resource := range res.Spec.Resources {
		gvk := schema.FromAPIVersionAndKind(resource.APIVersion, resource.Kind)
		if gvk.Group != v1alpha1.GroupVersion.Group {
			meta.SetStatusCondition(&res.Status.Conditions, metav1.Condition{
				Type:               v1alpha1.ReadyCondition,
				Status:             metav1.ConditionFalse,
				Reason:             v1alpha1.NotReadyReason,
				Message:            "ResourceSet only supports resources from the networking.cloud.sap group",
				ObservedGeneration: res.Generation,
			})
			log.Error(nil, "ResourceSet only supports resources from the networking.cloud.sap group", "resource", resource)
			r.Recorder.Eventf(res, "Warning", "UnsupportedResource", "Resource %s (%s) not supported by ResourceSet", resource.Name, gvk)
			return nil
		}
		if err := r.watch(ctx, gvk); err != nil {
			return err
		}

		for _, device := range list.Items {
			name := fmt.Sprintf("%s-%s-%s", res.Name, device.Name, resource.Name)

			managed := v1alpha1.ManagedResource{
				Name:       name,
				Kind:       resource.Kind,
				APIVersion: resource.APIVersion,
				Namespace:  device.Namespace,
				TargetName: device.Name,
			}

			mlog := log.WithValues("resource", managed.Name, "namespace", managed.Namespace, "kind", managed.Kind, "apiVersion", managed.APIVersion)

			idx := slices.Index(res.Status.ManagedResources, managed)
			if idx == -1 {
				mlog.Info("Creating managed resource")
				// Ensure the status is updated with the new resource and let the controller-runtime requeue the request.
				res.Status.ManagedResources = append(res.Status.ManagedResources, managed)
				return nil
			}

			// Keep track of the resources we want to keep, so that we can garbage collect the rest.
			keep[idx] = struct{}{}

			obj := &unstructured.Unstructured{}
			obj.SetName(name)
			obj.SetGroupVersionKind(gvk)
			obj.SetNamespace(device.Namespace)

			if res.Spec.Mode == v1alpha1.ResourceSetModeApplyOnce {
				err := r.Get(ctx, client.ObjectKeyFromObject(obj), obj)
				if err == nil {
					mlog.Info("Resource already exists, skipping creation")
					continue
				}

				if !apierrors.IsNotFound(err) {
					mlog.Error(err, "Failed to get existing resource")
					return err
				}
			}

			mlog.Info("Creating or updating resource")
			result, err := controllerutil.CreateOrPatch(ctx, r.Client, obj, func() error {
				content := make(map[string]any)
				if err := json.Unmarshal(resource.Template.Raw, &content); err != nil {
					mlog.Error(err, "Failed to unmarshal resource template")
					return fmt.Errorf("failed to unmarshal resource template: %w", err)
				}

				obj.SetUnstructuredContent(content)

				obj.SetName(name)
				obj.SetGroupVersionKind(gvk)
				obj.SetNamespace(device.Namespace)
				obj.SetLabels(map[string]string{
					v1alpha1.WatchLabel:  r.WatchFilterValue,
					v1alpha1.DeviceLabel: device.Name,
					v1alpha1.OwnerLabel:  res.Name,
				})

				if err := controllerutil.SetControllerReference(res, obj, r.Scheme); err != nil {
					return fmt.Errorf("failed to set owner reference: %w", err)
				}
				if err := controllerutil.SetOwnerReference(&device, obj, r.Scheme); err != nil {
					return fmt.Errorf("failed to set owner reference for device %s: %w", device.Name, err)
				}
				return nil
			})
			if err != nil {
				mlog.Error(err, "Failed to create or update resource")
				r.Recorder.Eventf(res, "Warning", "ResourceUpdateFailed", "Failed to create or update %s %s/%s: %v", gvk.Kind, obj.GetNamespace(), obj.GetName(), err)
				return fmt.Errorf("failed to create or update resource %s: %w", obj.GetName(), err)
			}

			mlog.Info("Resource created or updated successfully")
			switch result {
			case controllerutil.OperationResultCreated:
				r.Recorder.Eventf(res, "Normal", "ResourceCreated", "%s %s/%s created successfully", gvk.Kind, obj.GetNamespace(), obj.GetName())
			case controllerutil.OperationResultUpdated:
				r.Recorder.Eventf(res, "Normal", "ResourceUpdated", "%s %s/%s updated successfully", gvk.Kind, obj.GetNamespace(), obj.GetName())
			}
		}
	}

	for idx, managed := range res.Status.ManagedResources {
		if _, ok := keep[idx]; ok {
			// This resource is still managed, skip deletion
			continue
		}

		mlog := log.WithValues("resource", managed.Name, "namespace", managed.Namespace, "kind", managed.Kind, "apiVersion", managed.APIVersion)

		obj := &unstructured.Unstructured{}
		obj.SetName(managed.Name)
		obj.SetNamespace(managed.Namespace)
		obj.SetGroupVersionKind(schema.FromAPIVersionAndKind(managed.APIVersion, managed.Kind))

		mlog.Info("Deleting managed resource")
		if err := r.Delete(ctx, obj); err != nil {
			if !apierrors.IsNotFound(err) {
				mlog.Error(err, "Failed to delete managed resource")
				r.Recorder.Eventf(res, "Warning", "ResourceDeletionFailed", "Failed to delete %s %s/%s: %v", managed.Kind, managed.Namespace, managed.Name, err)
				return fmt.Errorf("failed to delete managed resource %s/%s: %w", managed.Kind, managed.Name, err)
			}
			mlog.Info("Managed resource already deleted")
			continue
		}

		mlog.Info("Managed resource deleted successfully")
		r.Recorder.Eventf(res, "Normal", "ResourceDeleted", "%s %s/%s deleted successfully", managed.Kind, managed.Namespace, managed.Name)
	}

	log.Info("ResourceSet reconciled successfully")
	r.Recorder.Eventf(res, "Normal", "ResourceSetReconciled", "ResourceSet reconciled successfully")

	meta.SetStatusCondition(&res.Status.Conditions, metav1.Condition{
		Type:               v1alpha1.ReadyCondition,
		Status:             metav1.ConditionTrue,
		Reason:             v1alpha1.AllResourcesReadyReason,
		Message:            "ResourceSet reconciled successfully",
		ObservedGeneration: res.Generation,
	})

	return nil
}

func (r *ResourceSetReconciler) finalize(ctx context.Context, res *v1alpha1.ResourceSet) error {
	for _, managed := range res.Status.ManagedResources {
		log := ctrl.LoggerFrom(ctx).WithValues("resource", managed.Name, "namespace", managed.Namespace, "kind", managed.Kind, "apiVersion", managed.APIVersion)

		obj := &unstructured.Unstructured{}
		obj.SetName(managed.Name)
		obj.SetNamespace(managed.Namespace)
		obj.SetGroupVersionKind(schema.FromAPIVersionAndKind(managed.APIVersion, managed.Kind))

		log.Info("Deleting managed resource")
		if err := r.Delete(ctx, obj); err != nil {
			if !apierrors.IsNotFound(err) {
				log.Error(err, "Failed to delete managed resource")
				r.Recorder.Eventf(res, "Warning", "ResourceDeletionFailed", "Failed to delete %s %s/%s: %v", managed.Kind, managed.Namespace, managed.Name, err)
				return fmt.Errorf("failed to delete managed resource %s/%s: %w", managed.Kind, managed.Name, err)
			}
			log.Info("Managed resource already deleted")
			continue
		}

		log.Info("Managed resource deleted successfully")
		r.Recorder.Eventf(res, "Normal", "ResourceDeleted", "%s %s/%s deleted successfully", managed.Kind, managed.Namespace, managed.Name)
	}
	return nil
}

func (r *ResourceSetReconciler) watch(ctx context.Context, gvk schema.GroupVersionKind) error {
	log := ctrl.LoggerFrom(ctx)

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.watchedGKVs[gvk]; ok {
		return nil // Already watching this GVK
	}

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)

	informer, err := r.Cache.GetInformerForKind(ctx, gvk)
	if err != nil {
		log.Error(err, "Failed to get informer for GVK", "gvk", gvk)
		return err
	}

	fn := func(obj any) {
		select {
		case <-ctx.Done():
			return
		default:
		}

		accessor, aErr := meta.Accessor(obj)
		if aErr != nil {
			log.Error(aErr, "Failed to get metadata accessor for object", "gvk", gvk, "object", obj)
			return
		}

		for _, owner := range accessor.GetOwnerReferences() {
			if owner.APIVersion == v1alpha1.GroupVersion.String() && owner.Kind == "ResourceSet" {
				log.Info("Enqueuing ResourceSet event", "name", owner.Name, "namespace", accessor.GetNamespace())
				r.events <- event.TypedGenericEvent[*v1alpha1.ResourceSet]{
					Object: &v1alpha1.ResourceSet{
						ObjectMeta: metav1.ObjectMeta{
							Name:      owner.Name,
							Namespace: accessor.GetNamespace(),
						},
					},
				}
			}
		}
	}

	_, err = informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc:    fn,
		UpdateFunc: func(oldObj any, newObj any) { fn(newObj) },
		DeleteFunc: fn,
	})
	if err != nil {
		return err
	}

	r.watchedGKVs[gvk] = struct{}{}
	return nil
}

// deviceToResourceSet is a [handler.MapFunc] to be used to enqueue requests for reconciliation
// for a ResourceSet to update when one of its references Devices.
func (r *ResourceSetReconciler) deviceToResourceSet(ctx context.Context, obj client.Object) []ctrl.Request {
	device, ok := obj.(*v1alpha1.Device)
	if !ok {
		panic(fmt.Sprintf("Expected a Device but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx, "Device", klog.KObj(device))

	deviceLabels := labels.Set(device.GetLabels())
	if len(deviceLabels) == 0 {
		return []reconcile.Request{}
	}

	list := new(v1alpha1.ResourceSetList)
	if err := r.List(ctx, list); err != nil {
		log.Error(err, "Failed to list ResourceSet")
		return nil
	}

	requests := []ctrl.Request{}
	for _, rs := range list.Items {
		selector, err := metav1.LabelSelectorAsSelector(&rs.Spec.Selector)
		if err != nil {
			log.Error(err, "failed to parse selector for ResourceSet", "ResourceSet", rs.Name)
			continue
		}

		if selector.Matches(deviceLabels) {
			log.Info("Enqueuing ResourceSet for reconciliation", "ResourceSet", klog.KObj(&rs))
			requests = append(requests, ctrl.Request{
				NamespacedName: client.ObjectKey{
					Name:      rs.Name,
					Namespace: rs.Namespace,
				},
			})
		}
	}

	return requests
}
