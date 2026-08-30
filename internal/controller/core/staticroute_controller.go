// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/events"
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
	"github.com/ironcore-dev/network-operator/internal/apistatus"
	"github.com/ironcore-dev/network-operator/internal/conditions"
	"github.com/ironcore-dev/network-operator/internal/deviceutil"
	"github.com/ironcore-dev/network-operator/internal/paused"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/resourcelock"
)

// StaticRouteReconciler reconciles a StaticRoute object
type StaticRouteReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// WatchFilterValue is the label value used to filter events prior to reconciliation.
	WatchFilterValue string

	// Recorder is used to record events for the controller.
	Recorder events.EventRecorder

	// Provider is the driver that will be used to create & delete the static route.
	Provider provider.ProviderFunc

	// Locker is used to synchronize operations on resources targeting the same device.
	Locker *resourcelock.ResourceLocker

	// RequeueInterval is the duration after which the controller should requeue the reconciliation,
	// regardless of changes.
	RequeueInterval time.Duration
}

// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=staticroutes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=staticroutes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=staticroutes/finalizers,verbs=update
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=vrfs,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=interfaces,verbs=get;list;watch
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *StaticRouteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(3).Info("Reconciling resource")

	obj := new(v1alpha1.StaticRoute)
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(3).Info("Resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get resource")
		return ctrl.Result{}, err
	}

	prov, ok := r.Provider().(provider.StaticRouteProvider)
	if !ok {
		if meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.NotImplementedReason,
			Message: "Provider does not implement provider.StaticRouteProvider",
		}) {
			return ctrl.Result{}, r.Status().Update(ctx, obj)
		}
		return ctrl.Result{}, nil
	}

	device, err := deviceutil.GetDeviceByName(ctx, r, obj.Namespace, obj.Spec.DeviceRef.Name)
	if err != nil {
		return ctrl.Result{}, err
	}

	if isPaused, requeue, err := paused.EnsureCondition(ctx, r.Client, device, obj); isPaused || requeue || err != nil {
		return ctrl.Result{Requeue: requeue}, err
	}

	if err := r.Locker.AcquireLock(ctx, device.Name, "staticroute-controller"); err != nil {
		if errors.Is(err, resourcelock.ErrLockAlreadyHeld) {
			log.V(3).Info("Device is already locked, requeuing reconciliation")
			return ctrl.Result{RequeueAfter: Jitter(time.Second), Priority: new(LockWaitPriorityHigh)}, nil
		}
		log.Error(err, "Failed to acquire device lock")
		return ctrl.Result{}, err
	}
	defer func() {
		if err := r.Locker.ReleaseLock(ctx, device.Name, "staticroute-controller"); err != nil {
			log.Error(err, "Failed to release device lock")
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	conn, err := deviceutil.GetDeviceConnection(ctx, r, device)
	if err != nil {
		return ctrl.Result{}, err
	}

	var cfg *provider.ProviderConfig
	if obj.Spec.ProviderConfigRef != nil {
		cfg, err = provider.GetProviderConfig(ctx, r, obj.Namespace, obj.Spec.ProviderConfigRef)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	s := &staticRouteScope{
		Device:         device,
		StaticRoute:    obj,
		Connection:     conn,
		ProviderConfig: cfg,
		Provider:       prov,
	}

	if !obj.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(obj, v1alpha1.FinalizerName) {
			if err := r.finalize(ctx, s); err != nil {
				log.Error(err, "Failed to finalize resource")
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(obj, v1alpha1.FinalizerName)
			if err := r.Update(ctx, obj); err != nil {
				log.Error(err, "Failed to remove finalizer from resource")
				return ctrl.Result{}, err
			}
		}
		log.V(3).Info("Resource is being deleted, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(obj, v1alpha1.FinalizerName) {
		controllerutil.AddFinalizer(obj, v1alpha1.FinalizerName)
		if err := r.Update(ctx, obj); err != nil {
			log.Error(err, "Failed to add finalizer to resource")
			return ctrl.Result{}, err
		}
		log.V(1).Info("Added finalizer to resource")
		return ctrl.Result{}, nil
	}

	orig := obj.DeepCopy()
	if conditions.InitializeConditions(obj, v1alpha1.ReadyCondition, v1alpha1.ConfiguredCondition) {
		log.V(1).Info("Initializing status conditions")
		return ctrl.Result{}, r.Status().Update(ctx, obj)
	}

	defer func() {
		if !equality.Semantic.DeepEqual(orig.ObjectMeta, obj.ObjectMeta) {
			if err := r.Patch(ctx, obj.DeepCopy(), client.MergeFrom(orig)); err != nil {
				log.Error(err, "Failed to update resource metadata")
				reterr = kerrors.NewAggregate([]error{reterr, err})
			}
		}
		if !equality.Semantic.DeepEqual(orig.Status, obj.Status) {
			if err := r.Status().Patch(ctx, obj, client.MergeFrom(orig)); err != nil {
				log.Error(err, "Failed to update status")
				reterr = kerrors.NewAggregate([]error{reterr, err})
			}
		}
	}()

	if err := r.reconcile(ctx, s); err != nil {
		log.Error(err, "Failed to reconcile resource")
		return ctrl.Result{}, apistatus.WrapTerminalError(err)
	}

	return ctrl.Result{RequeueAfter: r.RequeueInterval}, nil
}

const (
	staticRouteVrfRefKey       = ".spec.vrfRef.name"
	staticRouteInterfaceRefKey = ".spec.interfaceRef.name"
)

// SetupWithManager sets up the controller with the Manager.
func (r *StaticRouteReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	if r.RequeueInterval == 0 {
		return errors.New("requeue interval must not be 0")
	}

	labelSelector := metav1.LabelSelector{}
	if r.WatchFilterValue != "" {
		labelSelector.MatchLabels = map[string]string{v1alpha1.WatchLabel: r.WatchFilterValue}
	}

	filter, err := predicate.LabelSelectorPredicate(labelSelector)
	if err != nil {
		return fmt.Errorf("failed to create label selector predicate: %w", err)
	}

	if err := mgr.GetFieldIndexer().IndexField(ctx, &v1alpha1.StaticRoute{}, staticRouteVrfRefKey, func(obj client.Object) []string {
		sr := obj.(*v1alpha1.StaticRoute)
		if sr.Spec.VrfRef == nil {
			return nil
		}
		return []string{sr.Spec.VrfRef.Name}
	}); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(ctx, &v1alpha1.StaticRoute{}, v1alpha1.DeviceRefIndexKey, func(obj client.Object) []string {
		o := obj.(*v1alpha1.StaticRoute)
		return []string{o.Spec.DeviceRef.Name}
	}); err != nil {
		return err
	}

	bldr := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.StaticRoute{}).
		Named("staticroute").
		WithEventFilter(filter)

	for _, gvk := range v1alpha1.StaticRouteDependencies {
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(gvk)

		bldr = bldr.Watches(
			obj,
			handler.EnqueueRequestsFromMapFunc(r.staticRoutesForProviderConfig),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		)
	}

	return bldr.
		Watches(
			&v1alpha1.VRF{},
			handler.EnqueueRequestsFromMapFunc(r.vrfToStaticRoute),
			builder.WithPredicates(predicate.Funcs{
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
				UpdateFunc: func(e event.UpdateEvent) bool {
					return false
				},
			}),
		).
		Watches(
			&v1alpha1.Device{},
			handler.EnqueueRequestsFromMapFunc(r.deviceToStaticRoutes),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					return paused.DevicePausedChanged(e.ObjectOld, e.ObjectNew)
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			}),
		).
		Complete(r)
}

// staticRouteScope holds the different objects that are read and used during the reconcile.
type staticRouteScope struct {
	Device         *v1alpha1.Device
	StaticRoute    *v1alpha1.StaticRoute
	Connection     *deviceutil.Connection
	ProviderConfig *provider.ProviderConfig
	Provider       provider.StaticRouteProvider
}

func (r *StaticRouteReconciler) reconcile(ctx context.Context, s *staticRouteScope) (reterr error) {
	if s.StaticRoute.Labels == nil {
		s.StaticRoute.Labels = make(map[string]string)
	}

	s.StaticRoute.Labels[v1alpha1.DeviceLabel] = s.Device.Name

	if !controllerutil.HasControllerReference(s.StaticRoute) {
		if err := controllerutil.SetOwnerReference(s.Device, s.StaticRoute, r.Scheme, controllerutil.WithBlockOwnerDeletion(true)); err != nil {
			return err
		}
	}

	defer func() {
		conditions.RecomputeReady(s.StaticRoute)
	}()

	var vrf *v1alpha1.VRF
	if s.StaticRoute.Spec.VrfRef != nil {
		var err error
		vrf, err = r.reconcileVRF(ctx, s)
		if err != nil {
			return err
		}
	}

	if err := s.Provider.Connect(ctx, s.Connection); err != nil {
		return fmt.Errorf("failed to connect to provider: %w", err)
	}
	defer func() {
		if err := s.Provider.Disconnect(ctx, s.Connection); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	err := s.Provider.EnsureStaticRoute(ctx, &provider.StaticRouteRequest{
		StaticRoute:    s.StaticRoute,
		ProviderConfig: s.ProviderConfig,
		VRF:            vrf,
	})

	cond := conditions.FromError(err)
	conditions.Set(s.StaticRoute, cond)

	return err
}

// reconcileVRF ensures that the referenced VRF exists and belongs to the same device as the StaticRoute.
// add a label to the vrf to indicate that it is being used by the static route
func (r *StaticRouteReconciler) reconcileVRF(ctx context.Context, s *staticRouteScope) (*v1alpha1.VRF, error) {
	key := client.ObjectKey{
		Name:      s.StaticRoute.Spec.VrfRef.Name,
		Namespace: s.StaticRoute.Namespace,
	}

	vrf := new(v1alpha1.VRF)
	if err := r.Get(ctx, key, vrf); err != nil {
		if apierrors.IsNotFound(err) {
			conditions.Set(s.StaticRoute, metav1.Condition{
				Type:    v1alpha1.ConfiguredCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.VRFNotFoundReason,
				Message: fmt.Sprintf("referenced VRF %q not found", key),
			})
			return nil, reconcile.TerminalError(fmt.Errorf("referenced VRF %q not found", key))
		}
		return nil, fmt.Errorf("failed to get referenced VRF %q: %w", key, err)
	}

	if vrf.Spec.DeviceRef.Name != s.Device.Name {
		conditions.Set(s.StaticRoute, metav1.Condition{
			Type:    v1alpha1.ConfiguredCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.CrossDeviceReferenceReason,
			Message: fmt.Sprintf("referenced VRF %q does not belong to device %q", vrf.Name, s.Device.Name),
		})
		return nil, reconcile.TerminalError(fmt.Errorf("referenced VRF %q does not belong to device %q", vrf.Name, s.Device.Name))
	}

	return vrf, nil
}

func (r *StaticRouteReconciler) finalize(ctx context.Context, s *staticRouteScope) (reterr error) {
	if err := s.Provider.Connect(ctx, s.Connection); err != nil {
		return fmt.Errorf("failed to connect to provider: %w", err)
	}
	defer func() {
		if err := s.Provider.Disconnect(ctx, s.Connection); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	vrf, err := r.finalizeVRF(ctx, s)
	if err != nil {
		return err
	}

	return s.Provider.DeleteStaticRoute(ctx, &provider.StaticRouteRequest{
		StaticRoute:    s.StaticRoute,
		ProviderConfig: s.ProviderConfig,
		VRF:            vrf,
	})
}

func (r *StaticRouteReconciler) finalizeVRF(ctx context.Context, s *staticRouteScope) (vrf *v1alpha1.VRF, reterr error) {
	key := client.ObjectKey{
		Name:      s.StaticRoute.Spec.VrfRef.Name,
		Namespace: s.StaticRoute.Namespace,
	}

	vrf = new(v1alpha1.VRF)
	if err := r.Get(ctx, key, vrf); err != nil {
		if apierrors.IsNotFound(err) {
			// VRF not found - nothing to clean up
			return nil, client.IgnoreNotFound(err)
		}
		return nil, fmt.Errorf("failed to get referenced VRF %q: %w", key, err)
	}

	return vrf, nil
}

// vrfToStaticRoute is a [handler.MapFunc] to be used to enqueue requests for reconciliation
// for StaticRoutes when their referenced VRF changes.
func (r *StaticRouteReconciler) vrfToStaticRoute(ctx context.Context, obj client.Object) []ctrl.Request {
	vrf, ok := obj.(*v1alpha1.VRF)
	if !ok {
		panic(fmt.Sprintf("Expected a VRF but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx, "VRF", klog.KObj(vrf))

	staticRoutes := new(v1alpha1.StaticRouteList)
	if err := r.List(ctx, staticRoutes, client.InNamespace(vrf.Namespace), client.MatchingFields{staticRouteVrfRefKey: vrf.Name}); err != nil {
		log.Error(err, "Failed to list StaticRoutes")
		return nil
	}

	requests := []ctrl.Request{}
	for _, sr := range staticRoutes.Items {
		if sr.Spec.VrfRef != nil && sr.Spec.VrfRef.Name == vrf.Name {
			log.V(2).Info("Enqueuing StaticRoute for reconciliation", "StaticRoute", klog.KObj(&sr))

			requests = append(requests, ctrl.Request{
				NamespacedName: client.ObjectKey{
					Name:      sr.Name,
					Namespace: sr.Namespace,
				},
			})
		}
	}

	return requests
}

// staticRoutesForProviderConfig is a [handler.MapFunc] to be used to enqueue requests for reconciliation
// for a StaticRoute to update when one of its referenced provider configurations gets updated.
func (r *StaticRouteReconciler) staticRoutesForProviderConfig(ctx context.Context, obj client.Object) []reconcile.Request {
	log := ctrl.LoggerFrom(ctx, "Object", klog.KObj(obj))

	list := &v1alpha1.StaticRouteList{}
	if err := r.List(ctx, list, client.InNamespace(obj.GetNamespace())); err != nil {
		log.Error(err, "Failed to list StaticRoutes")
		return nil
	}

	gkv := obj.GetObjectKind().GroupVersionKind()

	var requests []reconcile.Request
	for _, m := range list.Items {
		if m.Spec.ProviderConfigRef != nil &&
			m.Spec.ProviderConfigRef.Name == obj.GetName() &&
			m.Spec.ProviderConfigRef.Kind == gkv.Kind &&
			m.Spec.ProviderConfigRef.APIVersion == gkv.GroupVersion().Identifier() {
			log.V(2).Info("Enqueuing StaticRoute for reconciliation", "StaticRoute", klog.KObj(&m))
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      m.Name,
					Namespace: m.Namespace,
				},
			})
		}
	}

	return requests
}

// deviceToStaticRoutes is a [handler.MapFunc] to be used to enqueue requests for reconciliation
// for StaticRoutes when their referenced Device's effective pause state changes.
func (r *StaticRouteReconciler) deviceToStaticRoutes(ctx context.Context, obj client.Object) []ctrl.Request {
	device, ok := obj.(*v1alpha1.Device)
	if !ok {
		panic(fmt.Sprintf("Expected a Device but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx, "Device", klog.KObj(device))

	staticRoutes := new(v1alpha1.StaticRouteList)
	if err := r.List(
		ctx, staticRoutes,
		client.InNamespace(device.Namespace),
		client.MatchingFields{v1alpha1.DeviceRefIndexKey: device.Name},
	); err != nil {
		log.Error(err, "Failed to list StaticRoutes")
		return nil
	}

	requests := make([]ctrl.Request, 0, len(staticRoutes.Items))
	for _, sr := range staticRoutes.Items {
		log.V(2).Info("Enqueuing StaticRoute for reconciliation", "StaticRoute", klog.KObj(&sr))
		requests = append(requests, ctrl.Request{
			NamespacedName: client.ObjectKey{
				Name:      sr.Name,
				Namespace: sr.Namespace,
			},
		})
	}

	return requests
}
