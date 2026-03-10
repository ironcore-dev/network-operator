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

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/annotations"
	"github.com/ironcore-dev/network-operator/internal/conditions"
	"github.com/ironcore-dev/network-operator/internal/deviceutil"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/resourcelock"
)

// LLDPReconciler reconciles a LLDP object
type LLDPReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// WatchFilterValue is the label value used to filter events prior to reconciliation.
	WatchFilterValue string

	// Recorder is used to record events for the controller.
	// More info: https://book.kubebuilder.io/reference/raising-events
	Recorder record.EventRecorder

	// Provider is the driver that will be used to create & delete the LLDP.
	Provider provider.ProviderFunc

	// Locker is used to synchronize operations on resources targeting the same device.
	Locker *resourcelock.ResourceLocker

	// RequeueInterval is the duration after which the controller should requeue the reconciliation,
	// regardless of changes.
	RequeueInterval time.Duration
}

// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=lldps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=lldps/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=lldps/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.22.4/pkg/reconcile
func (r *LLDPReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling resource")

	obj := new(v1alpha1.LLDP)
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Resource not found. Ignoring reconciliation since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get resource")
		return ctrl.Result{}, err
	}

	prov, ok := r.Provider().(provider.LLDPProvider)
	if !ok {
		if meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.NotImplementedReason,
			Message: "Provider does not implement provider LLDPProvider",
		}) {
			return ctrl.Result{}, r.Status().Update(ctx, obj)
		}
		return ctrl.Result{}, nil
	}

	device, err := deviceutil.GetDeviceByName(ctx, r, obj.Namespace, obj.Spec.DeviceRef.Name)
	if err != nil {
		return ctrl.Result{}, err
	}

	if annotations.IsPaused(device, obj) {
		log.Info("Reconciliation is paused for this object")
		return ctrl.Result{}, nil
	}

	// Prevent concurrent reconciliations of resources targeting the same device
	if err := r.Locker.AcquireLock(ctx, device.Name, "lldp-controller"); err != nil {
		if errors.Is(err, resourcelock.ErrLockAlreadyHeld) {
			log.Info("Device is already locked, requeuing reconciliation")
			return ctrl.Result{RequeueAfter: time.Second * 5}, nil
		}
		log.Error(err, "Failed to acquire device lock")
		return ctrl.Result{}, err
	}
	defer func() {
		if err := r.Locker.ReleaseLock(ctx, device.Name, "lldp-controller"); err != nil {
			log.Error(err, "Failed to release device lock")
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	conn, err := deviceutil.GetDeviceConnection(ctx, r, device)
	if err != nil {
		return ctrl.Result{}, err
	}

	s := &lldpScope{
		Device:     device,
		LLDP:       obj,
		Connection: conn,
		Provider:   prov,
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
	if conditions.InitializeConditions(obj, v1alpha1.ReadyCondition) {
		log.Info("Initializing status conditions")
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

type lldpScope struct {
	Device     *v1alpha1.Device
	LLDP       *v1alpha1.LLDP
	Connection *deviceutil.Connection
	Provider   provider.LLDPProvider
	// ProviderConfig is the resource referenced by LLDP.Spec.ProviderConfigRef, if any.
	ProviderConfig *provider.ProviderConfig
	// Interfaces are the Interface resources referenced by LLDP.Spec.InterfaceRefs.
	Interfaces []*v1alpha1.Interface
}

func (r *LLDPReconciler) reconcile(ctx context.Context, s *lldpScope) (_ ctrl.Result, reterr error) {
	if s.LLDP.Labels == nil {
		s.LLDP.Labels = make(map[string]string)
	}
	s.LLDP.Labels[v1alpha1.DeviceLabel] = s.Device.Name

	// Ensure LLDP resource is owned by the Device.
	if !controllerutil.HasControllerReference(s.LLDP) {
		if err := controllerutil.SetOwnerReference(s.Device, s.LLDP, r.Scheme, controllerutil.WithBlockOwnerDeletion(true)); err != nil {
			return ctrl.Result{}, err
		}
	}

	if err := r.validateUniqueLLDPPerDevice(ctx, s); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.validateProviderConfigRef(ctx, s); err != nil {
		return ctrl.Result{}, err
	}

	interfaces, err := r.reconcileInterfaceRefs(ctx, s)
	if err != nil {
		return ctrl.Result{}, err
	}
	s.Interfaces = interfaces

	if err := s.Provider.Connect(ctx, s.Connection); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to connect to provider: %w", err)
	}
	defer func() {
		if err := s.Provider.Disconnect(ctx, s.Connection); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	defer func() {
		conditions.RecomputeReady(s.LLDP)
	}()

	// Ensure the LLDP is realized on the remote device.
	err = s.Provider.EnsureLLDP(ctx, &provider.LLDPRequest{
		LLDP:           s.LLDP,
		ProviderConfig: s.ProviderConfig,
		Interfaces:     s.Interfaces,
	})

	cond := conditions.FromError(err)
	conditions.Set(s.LLDP, cond)

	if err != nil {
		return ctrl.Result{}, err
	}

	status, err := s.Provider.GetLLDPStatus(ctx, &provider.LLDPRequest{
		LLDP:           s.LLDP,
		ProviderConfig: s.ProviderConfig,
	})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get LLDP status: %w", err)
	}

	cond = metav1.Condition{
		Type:    v1alpha1.OperationalCondition,
		Status:  metav1.ConditionTrue,
		Reason:  v1alpha1.OperationalReason,
		Message: "LLDP is operationally up",
	}
	if !status.OperStatus {
		cond.Status = metav1.ConditionFalse
		cond.Reason = v1alpha1.DegradedReason
		cond.Message = "LLDP is operationally down"
	}
	conditions.Set(s.LLDP, cond)

	return ctrl.Result{RequeueAfter: Jitter(r.RequeueInterval)}, nil
}

// validateProviderConfigRef checks if the referenced provider configuration exists and is compatible with the target platform.
func (r *LLDPReconciler) validateProviderConfigRef(ctx context.Context, s *lldpScope) error {
	if s.LLDP.Spec.ProviderConfigRef == nil {
		return nil
	}

	cfg, err := provider.GetProviderConfig(ctx, r, s.LLDP.Namespace, s.LLDP.Spec.ProviderConfigRef)
	if err != nil {
		conditions.Set(s.LLDP, metav1.Condition{
			Type:    v1alpha1.ConfiguredCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.IncompatibleProviderConfigRef,
			Message: fmt.Sprintf("Failed to get ProviderConfigRef: %v", err),
		})
		return err
	}

	gv, err := schema.ParseGroupVersion(s.LLDP.Spec.ProviderConfigRef.APIVersion)
	if err != nil {
		conditions.Set(s.LLDP, metav1.Condition{
			Type:    v1alpha1.ConfiguredCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.IncompatibleProviderConfigRef,
			Message: fmt.Sprintf("ProviderConfigRef is not compatible with Device: %v", err),
		})
		return reconcile.TerminalError(fmt.Errorf("invalid API version %q: %w", s.LLDP.Spec.ProviderConfigRef.APIVersion, err))
	}

	gvk := schema.GroupVersionKind{
		Group:   gv.Group,
		Version: gv.Version,
		Kind:    s.LLDP.Spec.ProviderConfigRef.Kind,
	}

	if ok := slices.Contains(v1alpha1.LLDPDependencies, gvk); !ok {
		conditions.Set(s.LLDP, metav1.Condition{
			Type:    v1alpha1.ConfiguredCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.IncompatibleProviderConfigRef,
			Message: fmt.Sprintf("ProviderConfigRef kind '%s' with API version '%s' is not compatible with this device type", s.LLDP.Spec.ProviderConfigRef.Kind, s.LLDP.Spec.ProviderConfigRef.APIVersion),
		})
		return reconcile.TerminalError(fmt.Errorf("unsupported ProviderConfigRef Kind %q on this provider", gv))
	}

	s.ProviderConfig = cfg
	return nil
}

// reconcileInterfaceRefs fetches all referenced interfaces and validates them
func (r *LLDPReconciler) reconcileInterfaceRefs(ctx context.Context, s *lldpScope) ([]*v1alpha1.Interface, error) {
	if len(s.LLDP.Spec.InterfaceRefs) == 0 {
		return nil, nil
	}

	var interfaces []*v1alpha1.Interface
	for _, ifRef := range s.LLDP.Spec.InterfaceRefs {
		iface, err := r.reconcileInterfaceRef(ctx, ifRef, s)
		if err != nil {
			return nil, err
		}
		interfaces = append(interfaces, iface)
	}

	return interfaces, nil
}

// reconcileInterfaceRef checks that the referenced interface exists and belongs to the same device as the LLDP.
func (r *LLDPReconciler) reconcileInterfaceRef(ctx context.Context, interfaceRef v1alpha1.LLDPInterface, s *lldpScope) (*v1alpha1.Interface, error) {
	intf := new(v1alpha1.Interface)
	if err := r.Get(ctx, types.NamespacedName{
		Name:      interfaceRef.Name,
		Namespace: s.LLDP.Namespace,
	}, intf); err != nil {
		if apierrors.IsNotFound(err) {
			conditions.Set(s.LLDP, metav1.Condition{
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
		conditions.Set(s.LLDP, metav1.Condition{
			Type:    v1alpha1.ConfiguredCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.CrossDeviceReferenceReason,
			Message: fmt.Sprintf("Interface %s belongs to device %s, not %s", interfaceRef.Name, intf.Spec.DeviceRef.Name, s.Device.Name),
		})
		return nil, reconcile.TerminalError(fmt.Errorf("interface %s belongs to different device", interfaceRef.Name))
	}

	return intf, nil
}

func (r *LLDPReconciler) validateUniqueLLDPPerDevice(ctx context.Context, s *lldpScope) error {
	var list v1alpha1.LLDPList
	if err := r.List(ctx, &list,
		client.InNamespace(s.LLDP.Namespace),
		client.MatchingFields{".spec.deviceRef.name": s.LLDP.Spec.DeviceRef.Name},
	); err != nil {
		return err
	}
	for _, lldp := range list.Items {
		if lldp.Name != s.LLDP.Name {
			conditions.Set(s.LLDP, metav1.Condition{
				Type:    v1alpha1.ConfiguredCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.DuplicateResourceOnDevice,
				Message: fmt.Sprintf("Another LLDP (%s) already exists for device %s", lldp.Name, s.LLDP.Spec.DeviceRef.Name),
			})
			return reconcile.TerminalError(fmt.Errorf("only one LLDP resource allowed per device (%s)", s.LLDP.Spec.DeviceRef.Name))
		}
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LLDPReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
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

	if err := mgr.GetFieldIndexer().IndexField(ctx, &v1alpha1.LLDP{}, ".spec.deviceRef.name", func(obj client.Object) []string {
		lldp := obj.(*v1alpha1.LLDP)
		return []string{lldp.Spec.DeviceRef.Name}
	}); err != nil {
		return err
	}

	c := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.LLDP{}).
		Named("lldp").
		WithEventFilter(filter)

	for _, gvk := range v1alpha1.LLDPDependencies {
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(gvk)
		c = c.Watches(
			obj,
			handler.EnqueueRequestsFromMapFunc(r.mapProviderConfigToLLDP),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		)
	}

	// Watches enqueues LLDPs for updates in referenced Device resources.
	// Triggers on update events when the Paused spec field changes.
	c = c.Watches(
		&v1alpha1.Device{},
		handler.EnqueueRequestsFromMapFunc(r.deviceToLLDPs),
		builder.WithPredicates(predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldDevice := e.ObjectOld.(*v1alpha1.Device)
				newDevice := e.ObjectNew.(*v1alpha1.Device)
				// Only trigger when Paused spec field changes.
				return !equality.Semantic.DeepEqual(oldDevice.Spec.Paused, newDevice.Spec.Paused)
			},
			GenericFunc: func(e event.GenericEvent) bool {
				return false
			},
		}),
	)

	return c.Complete(r)
}

func (r *LLDPReconciler) mapProviderConfigToLLDP(ctx context.Context, obj client.Object) []reconcile.Request {
	log := ctrl.LoggerFrom(ctx, "Object", klog.KObj(obj))

	list := &v1alpha1.LLDPList{}
	if err := r.List(ctx, list, client.InNamespace(obj.GetNamespace())); err != nil {
		log.Error(err, "failed to list LLDPs")
		return nil
	}

	gkv := obj.GetObjectKind().GroupVersionKind()

	var requests []reconcile.Request
	for _, m := range list.Items {
		if m.Spec.ProviderConfigRef != nil &&
			m.Spec.ProviderConfigRef.Name == obj.GetName() &&
			m.Spec.ProviderConfigRef.Kind == gkv.Kind &&
			m.Spec.ProviderConfigRef.APIVersion == gkv.GroupVersion().Identifier() {
			log.Info("Found matching LLDP for provider config change, enqueuing for reconciliation", "LLDP", klog.KObj(&m))
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

func (r *LLDPReconciler) finalize(ctx context.Context, s *lldpScope) (reterr error) {
	if err := s.Provider.Connect(ctx, s.Connection); err != nil {
		return fmt.Errorf("failed to connect to provider: %w", err)
	}
	defer func() {
		if err := s.Provider.Disconnect(ctx, s.Connection); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	return s.Provider.DeleteLLDP(ctx, &provider.LLDPRequest{
		LLDP: s.LLDP,
	})
}

// deviceToLLDPs is a [handler.MapFunc] to be used to enqueue requests for reconciliation
// for LLDPs when their referenced Device's Paused spec field changes.
func (r *LLDPReconciler) deviceToLLDPs(ctx context.Context, obj client.Object) []ctrl.Request {
	device, ok := obj.(*v1alpha1.Device)
	if !ok {
		panic(fmt.Sprintf("Expected a Device but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx, "Device", klog.KObj(device))

	lldps := new(v1alpha1.LLDPList)
	if err := r.List(ctx, lldps,
		client.InNamespace(device.Namespace),
		client.MatchingLabels{v1alpha1.DeviceLabel: device.Name},
	); err != nil {
		log.Error(err, "Failed to list LLDPs")
		return nil
	}

	requests := make([]ctrl.Request, 0, len(lldps.Items))
	for _, l := range lldps.Items {
		log.Info("Enqueuing LLDP for reconciliation", "LLDP", klog.KObj(&l))
		requests = append(requests, ctrl.Request{
			NamespacedName: client.ObjectKey{
				Name:      l.Name,
				Namespace: l.Namespace,
			},
		})
	}

	return requests
}
