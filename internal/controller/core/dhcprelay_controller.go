// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
	"github.com/ironcore-dev/network-operator/internal/conditions"
	"github.com/ironcore-dev/network-operator/internal/deviceutil"
	"github.com/ironcore-dev/network-operator/internal/paused"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/resourcelock"
)

// DHCPRelayReconciler reconciles a DHCPRelay object
type DHCPRelayReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// WatchFilterValue is the label value used to filter events prior to reconciliation.
	WatchFilterValue string

	// Recorder is used to record events for the controller.
	// More info: https://book.kubebuilder.io/reference/raising-events
	Recorder events.EventRecorder

	// Provider is the driver that will be used to create & delete the dhcp relay configuration.
	Provider provider.ProviderFunc

	// Locker is used to synchronize operations on resources targeting the same device.
	Locker *resourcelock.ResourceLocker

	// RequeueInterval is the duration after which the controller should requeue the reconciliation,
	// regardless of changes.
	RequeueInterval time.Duration
}

// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=dhcprelays,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=dhcprelays/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=dhcprelays/finalizers,verbs=update
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.22.1/pkg/reconcile
//
// For more details about the method shape, read up here:
// - https://ahmet.im/blog/controller-pitfalls/#reconcile-method-shape
func (r *DHCPRelayReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(3).Info("Reconciling resource")

	obj := new(v1alpha1.DHCPRelay)
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			// If the custom resource is not found then it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			log.V(3).Info("Resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get resource")
		return ctrl.Result{}, err
	}

	prov, ok := r.Provider().(provider.DHCPRelayProvider)
	if !ok {
		if meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.NotImplementedReason,
			Message: "Provider does not implement provider.DHCPRelayProvider",
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

	if err := r.Locker.AcquireLock(ctx, device.Name, "dhcprelay-controller"); err != nil {
		if errors.Is(err, resourcelock.ErrLockAlreadyHeld) {
			log.V(3).Info("Device is already locked, requeuing reconciliation")
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
		log.Error(err, "Failed to acquire device lock")
		return ctrl.Result{}, err
	}
	defer func() {
		if err := r.Locker.ReleaseLock(ctx, device.Name, "dhcprelay-controller"); err != nil {
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

	s := &dhcprelayScope{
		Device:         device,
		DHCPRelay:      obj,
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

	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers
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
	if conditions.InitializeConditions(obj, v1alpha1.ReadyCondition) {
		log.V(1).Info("Initializing status conditions")
		return ctrl.Result{}, r.Status().Update(ctx, obj)
	}

	// Always attempt to update the metadata/status after reconciliation
	defer func() {
		if !equality.Semantic.DeepEqual(orig.ObjectMeta, obj.ObjectMeta) {
			// Pass obj.DeepCopy() to avoid Patch() modifying obj and interfering with status update below
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

	res, err := r.reconcile(ctx, s)
	if err != nil {
		log.Error(err, "Failed to reconcile resource")
		return ctrl.Result{}, err
	}

	return res, nil
}

// scope holds the different objects that are read and used during the reconcile.
type dhcprelayScope struct {
	Device         *v1alpha1.Device
	DHCPRelay      *v1alpha1.DHCPRelay
	Connection     *deviceutil.Connection
	ProviderConfig *provider.ProviderConfig
	Provider       provider.DHCPRelayProvider
	interfaces     []*v1alpha1.Interface
	vrf            *v1alpha1.VRF
}

func (r *DHCPRelayReconciler) reconcile(ctx context.Context, s *dhcprelayScope) (_ ctrl.Result, reterr error) {
	if s.DHCPRelay.Labels == nil {
		s.DHCPRelay.Labels = make(map[string]string)
	}
	s.DHCPRelay.Labels[v1alpha1.DeviceLabel] = s.Device.Name

	// Ensure the DHCPRelay is owned by the Device.
	if !controllerutil.HasControllerReference(s.DHCPRelay) {
		if err := controllerutil.SetOwnerReference(s.Device, s.DHCPRelay, r.Scheme, controllerutil.WithBlockOwnerDeletion(true)); err != nil {
			return ctrl.Result{}, err
		}
	}

	if err := r.validateUniqueResourcePerDevice(ctx, s); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.validateProviderConfigRef(ctx, s); err != nil {
		return ctrl.Result{}, err
	}

	interfaces, err := r.reconcileInterfaceRefs(ctx, s)
	if err != nil {
		return ctrl.Result{}, err
	}
	s.interfaces = interfaces

	vrf, err := r.reconcileVRFRef(ctx, s)
	if err != nil {
		return ctrl.Result{}, err
	}
	s.vrf = vrf

	// Connect to remote device using the provider.
	if err := s.Provider.Connect(ctx, s.Connection); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to connect to provider: %w", err)
	}
	defer func() {
		if err := s.Provider.Disconnect(ctx, s.Connection); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	// Ensure the DHCPRelay is realized on the remote device.
	err = s.Provider.EnsureDHCPRelay(ctx, &provider.DHCPRelayRequest{
		DHCPRelay:      s.DHCPRelay,
		ProviderConfig: s.ProviderConfig,
		Interfaces:     s.interfaces,
		VRF:            s.vrf,
	})

	cond := conditions.FromError(err)
	// As this resource is configuration only, we use the Configured condition as top-level Ready condition.
	cond.Type = v1alpha1.ReadyCondition
	conditions.Set(s.DHCPRelay, cond)

	if err != nil {
		return ctrl.Result{}, err
	}

	// Retrieve and update the status from the device; this include the list of interfaces that are actually configured on the device.
	status, err := s.Provider.GetDHCPRelayStatus(ctx, &provider.DHCPRelayRequest{
		DHCPRelay:      s.DHCPRelay,
		ProviderConfig: s.ProviderConfig,
		Interfaces:     s.interfaces,
	})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get DHCP relay status: %w", err)
	}

	s.DHCPRelay.Status.ConfiguredInterfaces = status.ConfiguredInterfaces

	return ctrl.Result{RequeueAfter: Jitter(r.RequeueInterval)}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DHCPRelayReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
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

	bldr := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.DHCPRelay{}).
		Named("dhcprelay").
		WithEventFilter(filter)

	for _, gvk := range v1alpha1.DHCPRelayDependencies {
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(gvk)

		bldr = bldr.Watches(
			obj,
			handler.EnqueueRequestsFromMapFunc(r.mapProviderConfigToDHCPRelay),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		)
	}

	return bldr.
		// Watches enqueues DHCPRelays for updates in referenced Device resources.
		// Triggers on create, delete, and update events when the device's effective pause state changes.
		Watches(
			&v1alpha1.Device{},
			handler.EnqueueRequestsFromMapFunc(r.deviceToDHCPRelays),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					return paused.DevicePausedChanged(e.ObjectOld, e.ObjectNew)
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			}),
		).
		// Watches enqueues DHCPRelays when referenced Interface resources are configured.
		Watches(
			&v1alpha1.Interface{},
			handler.EnqueueRequestsFromMapFunc(r.interfaceToDHCPRelays),
			builder.WithPredicates(predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool {
					return false
				},
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldIntf := e.ObjectOld.(*v1alpha1.Interface)
					newIntf := e.ObjectNew.(*v1alpha1.Interface)
					// Only trigger when Configured condition changes (not operational status).
					return conditions.IsConfigured(oldIntf) != conditions.IsConfigured(newIntf)
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			}),
		).
		// Watches enqueues DHCPRelays when referenced VRF resources are configured.
		Watches(
			&v1alpha1.VRF{},
			handler.EnqueueRequestsFromMapFunc(r.vrfToDHCPRelays),
			builder.WithPredicates(predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool {
					return false
				},
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldVRF := e.ObjectOld.(*v1alpha1.VRF)
					newVRF := e.ObjectNew.(*v1alpha1.VRF)
					// Only trigger when Configured condition changes (not operational status).
					return conditions.IsConfigured(oldVRF) != conditions.IsConfigured(newVRF)
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			}),
		).
		Complete(r)
}

// validateProviderConfigRef checks if the referenced provider configuration is compatible with the target platform.
func (r *DHCPRelayReconciler) validateProviderConfigRef(_ context.Context, s *dhcprelayScope) error {
	if s.DHCPRelay.Spec.ProviderConfigRef == nil {
		return nil
	}

	gv, err := schema.ParseGroupVersion(s.DHCPRelay.Spec.ProviderConfigRef.APIVersion)
	if err != nil {
		conditions.Set(s.DHCPRelay, metav1.Condition{
			Type:    v1alpha1.ConfiguredCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.IncompatibleProviderConfigRef,
			Message: fmt.Sprintf("Invalid API version in ProviderConfigRef: %v", err),
		})
		return reconcile.TerminalError(fmt.Errorf("invalid API version %q: %w", s.DHCPRelay.Spec.ProviderConfigRef.APIVersion, err))
	}

	gvk := gv.WithKind(s.DHCPRelay.Spec.ProviderConfigRef.Kind)

	if ok := slices.Contains(v1alpha1.DHCPRelayDependencies, gvk); !ok {
		conditions.Set(s.DHCPRelay, metav1.Condition{
			Type:    v1alpha1.ConfiguredCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.IncompatibleProviderConfigRef,
			Message: fmt.Sprintf("ProviderConfigRef kind '%s' with API version '%s' is not compatible with this type", s.DHCPRelay.Spec.ProviderConfigRef.Kind, s.DHCPRelay.Spec.ProviderConfigRef.APIVersion),
		})
		return reconcile.TerminalError(fmt.Errorf("unsupported ProviderConfigRef Kind %q on this provider", gv))
	}

	return nil
}

// reconcileInterfaceRefs fetches all referenced interfaces and validates them
func (r *DHCPRelayReconciler) reconcileInterfaceRefs(ctx context.Context, s *dhcprelayScope) ([]*v1alpha1.Interface, error) {
	if len(s.DHCPRelay.Spec.InterfaceRefs) == 0 {
		return nil, nil
	}

	interfaces := make([]*v1alpha1.Interface, 0, len(s.DHCPRelay.Spec.InterfaceRefs))
	for _, ifRef := range s.DHCPRelay.Spec.InterfaceRefs {
		iface, err := r.reconcileInterfaceRef(ctx, ifRef, s)
		if err != nil {
			return nil, err
		}
		interfaces = append(interfaces, iface)
	}

	return interfaces, nil
}

// reconcileInterfaceRef checks that the referenced interface exists and belongs to the same device as the DHCPRelay.
func (r *DHCPRelayReconciler) reconcileInterfaceRef(ctx context.Context, interfaceRef v1alpha1.LocalObjectReference, s *dhcprelayScope) (*v1alpha1.Interface, error) {
	intf := new(v1alpha1.Interface)
	if err := r.Get(ctx, types.NamespacedName{
		Name:      interfaceRef.Name,
		Namespace: s.DHCPRelay.Namespace,
	}, intf); err != nil {
		if apierrors.IsNotFound(err) {
			conditions.Set(s.DHCPRelay, metav1.Condition{
				Type:    v1alpha1.ConfiguredCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.WaitingForDependenciesReason,
				Message: fmt.Sprintf("Interface %s not found", interfaceRef.Name),
			})
			return nil, reconcile.TerminalError(fmt.Errorf("interface %s not found", interfaceRef.Name))
		}
		return nil, fmt.Errorf("failed to get interface %s: %w", interfaceRef.Name, err)
	}

	// Verify the interface belongs to the same device
	if intf.Spec.DeviceRef.Name != s.Device.Name {
		conditions.Set(s.DHCPRelay, metav1.Condition{
			Type:    v1alpha1.ConfiguredCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.CrossDeviceReferenceReason,
			Message: fmt.Sprintf("Interface %s belongs to device %s, not %s", interfaceRef.Name, intf.Spec.DeviceRef.Name, s.Device.Name),
		})
		return nil, reconcile.TerminalError(fmt.Errorf("interface %s belongs to different device", interfaceRef.Name))
	}

	switch intf.Spec.Type {
	case v1alpha1.InterfaceTypePhysical, v1alpha1.InterfaceTypeAggregate, v1alpha1.InterfaceTypeRoutedVLAN:
		// Supported types, do nothing
	default:
		conditions.Set(s.DHCPRelay, metav1.Condition{
			Type:    v1alpha1.ConfiguredCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.InvalidInterfaceTypeReason,
			Message: fmt.Sprintf("Interface %s has invalid type %s (only Physical, Aggregate, and RoutedVLAN types are supported)", interfaceRef.Name, intf.Spec.Type),
		})
		return nil, reconcile.TerminalError(fmt.Errorf("interface %s has an invalid type: %s", interfaceRef.Name, intf.Spec.Type))
	}

	// Verify the interface configuration is applied to the device (not operational status)
	if !conditions.IsConfigured(intf) {
		conditions.Set(s.DHCPRelay, metav1.Condition{
			Type:    v1alpha1.ConfiguredCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.WaitingForDependenciesReason,
			Message: fmt.Sprintf("Interface %s is not configured on the device", interfaceRef.Name),
		})
		return nil, reconcile.TerminalError(fmt.Errorf("interface %s is not configured", interfaceRef.Name))
	}

	// Verify the interface has required IP addressing configured based on server address types
	if intf.Spec.IPv4 == nil {
		conditions.Set(s.DHCPRelay, metav1.Condition{
			Type:    v1alpha1.ConfiguredCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.IPAddressingNotFoundReason,
			Message: fmt.Sprintf("Interface %s has no IPv4 configuration (address or unnumbered required for DHCP relay)", interfaceRef.Name),
		})
		return nil, reconcile.TerminalError(fmt.Errorf("interface %s has no IPv4 configuration", interfaceRef.Name))
	}

	return intf, nil
}

func (r *DHCPRelayReconciler) reconcileVRFRef(ctx context.Context, s *dhcprelayScope) (*v1alpha1.VRF, error) {
	if s.DHCPRelay.Spec.VrfRef == nil {
		return nil, nil
	}
	vrf := new(v1alpha1.VRF)
	if err := r.Get(ctx, types.NamespacedName{
		Name:      s.DHCPRelay.Spec.VrfRef.Name,
		Namespace: s.DHCPRelay.Namespace,
	}, vrf); err != nil {
		if apierrors.IsNotFound(err) {
			conditions.Set(s.DHCPRelay, metav1.Condition{
				Type:    v1alpha1.ConfiguredCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.WaitingForDependenciesReason,
				Message: fmt.Sprintf("VRF %s not found", s.DHCPRelay.Spec.VrfRef.Name),
			})
			return nil, reconcile.TerminalError(fmt.Errorf("vrf %s not found", s.DHCPRelay.Spec.VrfRef.Name))
		}
		return nil, fmt.Errorf("failed to get VRF %s: %w", s.DHCPRelay.Spec.VrfRef.Name, err)
	}

	// Verify the VRF belongs to the same device
	if vrf.Spec.DeviceRef.Name != s.Device.Name {
		conditions.Set(s.DHCPRelay, metav1.Condition{
			Type:    v1alpha1.ConfiguredCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.CrossDeviceReferenceReason,
			Message: fmt.Sprintf("VRF %s belongs to device %s, not %s", s.DHCPRelay.Spec.VrfRef.Name, vrf.Spec.DeviceRef.Name, s.Device.Name),
		})
		return nil, reconcile.TerminalError(fmt.Errorf("vrf %s belongs to different device", s.DHCPRelay.Spec.VrfRef.Name))
	}

	// Verify the VRF configuration is applied to the device (not operational status)
	if !conditions.IsConfigured(vrf) {
		conditions.Set(s.DHCPRelay, metav1.Condition{
			Type:    v1alpha1.ConfiguredCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.WaitingForDependenciesReason,
			Message: fmt.Sprintf("VRF %s is not configured on the device", s.DHCPRelay.Spec.VrfRef.Name),
		})
		return nil, reconcile.TerminalError(fmt.Errorf("vrf %s is not configured", s.DHCPRelay.Spec.VrfRef.Name))
	}

	return vrf, nil
}

func (r *DHCPRelayReconciler) validateUniqueResourcePerDevice(ctx context.Context, s *dhcprelayScope) error {
	var list v1alpha1.DHCPRelayList
	if err := r.List(ctx, &list,
		client.InNamespace(s.DHCPRelay.Namespace),
		client.MatchingLabels{v1alpha1.DeviceLabel: s.Device.Name},
	); err != nil {
		return err
	}
	for _, dhcprelay := range list.Items {
		if dhcprelay.Name != s.DHCPRelay.Name {
			conditions.Set(s.DHCPRelay, metav1.Condition{
				Type:    v1alpha1.ConfiguredCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.DuplicateResourceOnDevice,
				Message: fmt.Sprintf("Another DHCPRelay (%s) already exists for device %s", dhcprelay.Name, s.DHCPRelay.Spec.DeviceRef.Name),
			})
			return reconcile.TerminalError(fmt.Errorf("only one DHCPRelay resource allowed per device (%s)", s.DHCPRelay.Spec.DeviceRef.Name))
		}
	}
	return nil
}

func (r *DHCPRelayReconciler) mapProviderConfigToDHCPRelay(ctx context.Context, obj client.Object) []reconcile.Request {
	log := ctrl.LoggerFrom(ctx, "Object", klog.KObj(obj))

	list := &v1alpha1.DHCPRelayList{}
	if err := r.List(ctx, list, client.InNamespace(obj.GetNamespace())); err != nil {
		log.Error(err, "failed to list DHCPRelays")
		return nil
	}

	gkv := obj.GetObjectKind().GroupVersionKind()

	var requests []reconcile.Request
	for _, m := range list.Items {
		if m.Spec.ProviderConfigRef != nil &&
			m.Spec.ProviderConfigRef.Name == obj.GetName() &&
			m.Spec.ProviderConfigRef.Kind == gkv.Kind &&
			m.Spec.ProviderConfigRef.APIVersion == gkv.GroupVersion().Identifier() {
			log.V(2).Info("Found matching DHCPRelay for provider config change, enqueuing for reconciliation", "DHCPRelay", klog.KObj(&m))
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

func (r *DHCPRelayReconciler) finalize(ctx context.Context, s *dhcprelayScope) (reterr error) {
	if err := s.Provider.Connect(ctx, s.Connection); err != nil {
		return fmt.Errorf("failed to connect to provider: %w", err)
	}
	defer func() {
		if err := s.Provider.Disconnect(ctx, s.Connection); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	return s.Provider.DeleteDHCPRelay(ctx, &provider.DHCPRelayRequest{
		DHCPRelay:      s.DHCPRelay,
		ProviderConfig: s.ProviderConfig,
	})
}

// deviceToDHCPRelays is a [handler.MapFunc] to be used to enqueue requests for reconciliation
func (r *DHCPRelayReconciler) deviceToDHCPRelays(ctx context.Context, obj client.Object) []ctrl.Request {
	device, ok := obj.(*v1alpha1.Device)
	if !ok {
		panic(fmt.Sprintf("Expected a Device but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx, "Device", klog.KObj(device))

	list := new(v1alpha1.DHCPRelayList)
	if err := r.List(ctx, list,
		client.InNamespace(device.Namespace),
		client.MatchingLabels{v1alpha1.DeviceLabel: device.Name},
	); err != nil {
		log.Error(err, "Failed to list DHCPRelays")
		return nil
	}

	requests := make([]ctrl.Request, 0, len(list.Items))
	for _, i := range list.Items {
		log.V(2).Info("Enqueuing DHCPRelay for reconciliation", "DHCPRelay", klog.KObj(&i))
		requests = append(requests, ctrl.Request{
			NamespacedName: client.ObjectKey{
				Name:      i.Name,
				Namespace: i.Namespace,
			},
		})
	}

	return requests
}

// interfaceToDHCPRelays is a [handler.MapFunc] that enqueues DHCPRelays referencing the given Interface.
func (r *DHCPRelayReconciler) interfaceToDHCPRelays(ctx context.Context, obj client.Object) []ctrl.Request {
	intf, ok := obj.(*v1alpha1.Interface)
	if !ok {
		panic(fmt.Sprintf("Expected an Interface but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx, "Interface", klog.KObj(intf))

	list := new(v1alpha1.DHCPRelayList)
	if err := r.List(ctx, list,
		client.InNamespace(intf.Namespace),
		client.MatchingLabels{v1alpha1.DeviceLabel: intf.Spec.DeviceRef.Name},
	); err != nil {
		log.Error(err, "Failed to list DHCPRelays")
		return nil
	}

	var requests []ctrl.Request
	for _, dhcpRelay := range list.Items {
		for _, ifRef := range dhcpRelay.Spec.InterfaceRefs {
			if ifRef.Name == intf.Name {
				log.V(2).Info("Enqueuing DHCPRelay for reconciliation", "DHCPRelay", klog.KObj(&dhcpRelay))
				requests = append(requests, ctrl.Request{
					NamespacedName: client.ObjectKey{
						Name:      dhcpRelay.Name,
						Namespace: dhcpRelay.Namespace,
					},
				})
				break
			}
		}
	}

	return requests
}

// vrfToDHCPRelays is a [handler.MapFunc] that enqueues DHCPRelays referencing the given VRF.
func (r *DHCPRelayReconciler) vrfToDHCPRelays(ctx context.Context, obj client.Object) []ctrl.Request {
	vrf, ok := obj.(*v1alpha1.VRF)
	if !ok {
		panic(fmt.Sprintf("Expected a VRF but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx, "VRF", klog.KObj(vrf))

	list := new(v1alpha1.DHCPRelayList)
	if err := r.List(ctx, list,
		client.InNamespace(vrf.Namespace),
		client.MatchingLabels{v1alpha1.DeviceLabel: vrf.Spec.DeviceRef.Name},
	); err != nil {
		log.Error(err, "Failed to list DHCPRelays")
		return nil
	}

	var requests []ctrl.Request
	for _, dhcpRelay := range list.Items {
		if dhcpRelay.Spec.VrfRef != nil && dhcpRelay.Spec.VrfRef.Name == vrf.Name {
			log.V(2).Info("Enqueuing DHCPRelay for reconciliation", "DHCPRelay", klog.KObj(&dhcpRelay))
			requests = append(requests, ctrl.Request{
				NamespacedName: client.ObjectKey{
					Name:      dhcpRelay.Name,
					Namespace: dhcpRelay.Namespace,
				},
			})
		}
	}

	return requests
}
