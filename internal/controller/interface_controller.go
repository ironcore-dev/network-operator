// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"time"

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
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/ironcore-dev/network-operator/api/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/conditions"
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

	// Provider is the driver that will be used to create & delete the interface.
	Provider provider.ProviderFunc

	// RequeueInterval is the duration after which the controller should requeue the reconciliation,
	// regardless of changes.
	RequeueInterval time.Duration
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

	prov, ok := r.Provider().(provider.InterfaceProvider)
	if !ok {
		if meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.NotImplementedReason,
			Message: "Provider does not implement provider.InterfaceProvider",
		}) {
			return ctrl.Result{}, r.Status().Update(ctx, obj)
		}
		return ctrl.Result{}, nil
	}

	device, err := deviceutil.GetDeviceByName(ctx, r, obj.Namespace, obj.Spec.DeviceRef.Name)
	if err != nil {
		return ctrl.Result{}, err
	}

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

	s := &scope{
		Device:         device,
		Interface:      obj,
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
	if conditions.InitializeConditions(obj, v1alpha1.ReadyCondition, v1alpha1.ConfiguredCondition, v1alpha1.OperationalCondition) {
		log.Info("Initializing status conditions")
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

	res, err := r.reconcile(ctx, s)
	if err != nil {
		log.Error(err, "Failed to reconcile resource")
		return ctrl.Result{}, err
	}

	return res, nil
}

var interfaceUnnumberedRefKey = ".spec.ipv4.unnumbered.interfaceRef.name"

// SetupWithManager sets up the controller with the Manager.
func (r *InterfaceReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
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

	if err := mgr.GetFieldIndexer().IndexField(ctx, &v1alpha1.Interface{}, interfaceUnnumberedRefKey, func(obj client.Object) []string {
		intf := obj.(*v1alpha1.Interface)
		if intf.Spec.IPv4 == nil || intf.Spec.IPv4.Unnumbered == nil {
			return nil
		}
		return []string{intf.Spec.IPv4.Unnumbered.InterfaceRef.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Interface{}).
		Named("interface").
		WithEventFilter(filter).
		// Watches enqueues Interfaces for updates in referenced ipv4 unnumbered resources.
		// Only triggers on create and delete events since interface names are immutable.
		Watches(
			&v1alpha1.Interface{},
			handler.EnqueueRequestsFromMapFunc(r.interfaceToUnnumbered),
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

// scope holds the different objects that are read and used during the reconcile.
type scope struct {
	Device         *v1alpha1.Device
	Interface      *v1alpha1.Interface
	Connection     *deviceutil.Connection
	ProviderConfig *provider.ProviderConfig
	Provider       provider.InterfaceProvider
}

func (r *InterfaceReconciler) reconcile(ctx context.Context, s *scope) (_ ctrl.Result, reterr error) {
	if s.Interface.Labels == nil {
		s.Interface.Labels = make(map[string]string)
	}

	s.Interface.Labels[v1alpha1.DeviceLabel] = s.Device.Name

	// Ensure the Interface is owned by the Device.
	if !controllerutil.HasControllerReference(s.Interface) {
		if err := controllerutil.SetOwnerReference(s.Device, s.Interface, r.Scheme, controllerutil.WithBlockOwnerDeletion(true)); err != nil {
			return ctrl.Result{}, err
		}
	}

	defer func() {
		conditions.RecomputeReady(s.Interface)
	}()

	ip, err := r.reconcileIPv4(ctx, s)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := s.Provider.Connect(ctx, s.Connection); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to connect to provider: %w", err)
	}
	defer func() {
		if err := s.Provider.Disconnect(ctx, s.Connection); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	// Ensure the Interface is realized on the provider.
	err = s.Provider.EnsureInterface(ctx, &provider.InterfaceRequest{
		Interface:      s.Interface,
		ProviderConfig: s.ProviderConfig,
		IPv4:           ip,
	})

	cond := conditions.FromError(err)
	conditions.Set(s.Interface, cond)

	if err != nil {
		return ctrl.Result{}, err
	}

	status, err := s.Provider.GetInterfaceStatus(ctx, &provider.InterfaceRequest{
		Interface:      s.Interface,
		ProviderConfig: s.ProviderConfig,
	})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get interface status: %w", err)
	}

	cond = metav1.Condition{
		Type:    v1alpha1.OperationalCondition,
		Status:  metav1.ConditionTrue,
		Reason:  v1alpha1.OperationalReason,
		Message: "Interface is operationally up",
	}
	if !status.OperStatus {
		cond.Status = metav1.ConditionFalse
		cond.Reason = v1alpha1.DegradedReason
		cond.Message = "Interface is operationally down"
	}
	conditions.Set(s.Interface, cond)

	return ctrl.Result{RequeueAfter: Jitter(r.RequeueInterval)}, nil
}

func (r *InterfaceReconciler) reconcileIPv4(ctx context.Context, s *scope) (ip provider.IPv4, _ error) {
	if s.Interface.Spec.IPv4 == nil {
		return nil, nil
	}

	switch {
	case len(s.Interface.Spec.IPv4.Addresses) > 0:
		addrs := make([]netip.Prefix, len(s.Interface.Spec.IPv4.Addresses))
		for i, addr := range s.Interface.Spec.IPv4.Addresses {
			addrs[i] = addr.Prefix
		}
		ip = provider.IPv4AddressList(addrs)

	case s.Interface.Spec.IPv4.Unnumbered != nil:
		key := client.ObjectKey{
			Name:      s.Interface.Spec.IPv4.Unnumbered.InterfaceRef.Name,
			Namespace: s.Interface.Namespace,
		}

		intf := new(v1alpha1.Interface)
		if err := r.Get(ctx, key, intf); err != nil {
			if apierrors.IsNotFound(err) {
				conditions.Set(s.Interface, metav1.Condition{
					Type:    v1alpha1.ConfiguredCondition,
					Status:  metav1.ConditionFalse,
					Reason:  v1alpha1.UnnumberedSourceInterfaceNotFoundReason,
					Message: fmt.Sprintf("referenced interface %q for unnumbered ipv4 configuration not found", key),
				})
				return nil, reconcile.TerminalError(fmt.Errorf("referenced interface %q for unnumbered ipv4 configuration not found", key))
			}
			return nil, fmt.Errorf("failed to get referenced interface %q for unnumbered ipv4 configuration: %w", key, err)
		}

		if intf.Spec.DeviceRef.Name != s.Device.Name {
			conditions.Set(s.Interface, metav1.Condition{
				Type:    v1alpha1.ConfiguredCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.UnnumberedCrossDeviceReferenceReason,
				Message: fmt.Sprintf("referenced interface %q for unnumbered ipv4 configuration does not belong to device %q", intf.Name, s.Device.Name),
			})
			return nil, reconcile.TerminalError(fmt.Errorf("referenced interface %q for unnumbered ipv4 configuration does not belong to device %q", intf.Name, s.Device.Name))
		}

		if intf.Spec.Type != v1alpha1.InterfaceTypeLoopback {
			conditions.Set(s.Interface, metav1.Condition{
				Type:    v1alpha1.ConfiguredCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.UnnumberedInvalidInterfaceTypeReason,
				Message: fmt.Sprintf("referenced interface %q for unnumbered ipv4 configuration is not of type Loopback, got %q", intf.Name, intf.Spec.Type),
			})
			return nil, reconcile.TerminalError(fmt.Errorf("referenced interface %q for unnumbered ipv4 configuration is not of type Loopback, got %q", intf.Name, intf.Spec.Type))
		}

		ip = provider.IPv4Unnumbered{SourceInterface: intf.Spec.Name}
	}

	return
}

func (r *InterfaceReconciler) finalize(ctx context.Context, s *scope) (reterr error) {
	if err := s.Provider.Connect(ctx, s.Connection); err != nil {
		return fmt.Errorf("failed to connect to provider: %w", err)
	}
	defer func() {
		if err := s.Provider.Disconnect(ctx, s.Connection); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	return s.Provider.DeleteInterface(ctx, &provider.InterfaceRequest{
		Interface:      s.Interface,
		ProviderConfig: s.ProviderConfig,
	})
}

// interfaceToUnnumbered is a [handler.MapFunc] to be used to enqueue requests for reconciliation
// for a Interface to update when its referenced unnumbered source Interface changes.
func (r *InterfaceReconciler) interfaceToUnnumbered(ctx context.Context, obj client.Object) []ctrl.Request {
	intf, ok := obj.(*v1alpha1.Interface)
	if !ok {
		panic(fmt.Sprintf("Expected a Interface but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx, "Unnumbered Reference", klog.KObj(intf))

	interfaces := new(v1alpha1.InterfaceList)
	if err := r.List(ctx, interfaces, client.InNamespace(intf.Namespace), client.MatchingFields{interfaceUnnumberedRefKey: intf.Spec.Name}); err != nil {
		log.Error(err, "Failed to list Interfaces")
		return nil
	}

	requests := []ctrl.Request{}
	for _, i := range interfaces.Items {
		if i.Spec.IPv4 != nil && i.Spec.IPv4.Unnumbered != nil && i.Spec.IPv4.Unnumbered.InterfaceRef.Name == intf.Name {
			log.Info("Enqueuing Interface for reconciliation", "Interface", klog.KObj(&i))
			requests = append(requests, ctrl.Request{
				NamespacedName: client.ObjectKey{
					Name:      i.Name,
					Namespace: i.Namespace,
				},
			})
		}
	}

	return requests
}
