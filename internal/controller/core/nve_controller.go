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
	"github.com/ironcore-dev/network-operator/internal/conditions"
	"github.com/ironcore-dev/network-operator/internal/deviceutil"
	"github.com/ironcore-dev/network-operator/internal/provider"
)

// NetworkVirtualizationEdgeReconciler reconciles a NVE object
type NetworkVirtualizationEdgeReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// WatchFilterValue is the label value used to filter events prior to reconciliation.
	WatchFilterValue string

	// Recorder is used to record events for the controller.
	// More info: https://book.kubebuilder.io/reference/raising-events
	Recorder record.EventRecorder

	// Provider is the driver that will be used to create & delete the dns.
	Provider provider.ProviderFunc

	// RequeueInterval is the duration after which the controller should requeue the reconciliation,
	// regardless of changes.
	RequeueInterval time.Duration
}

// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=networkvirtualizationedges,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=networkvirtualizationedges/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=networkvirtualizationedges/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.22.1/pkg/reconcile
func (r *NetworkVirtualizationEdgeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling resource")

	obj := new(v1alpha1.NetworkVirtualizationEdge)
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Resource not found. Ignoring reconciliation since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get resource")
		return ctrl.Result{}, err
	}

	prov, ok := r.Provider().(provider.NVEProvider)
	if !ok {
		if meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.NotImplementedReason,
			Message: "Provider does not implement provider NVEProvider",
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

	s := &nveScope{
		Device:         device,
		NVE:            obj,
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
	if conditions.InitializeConditions(obj, v1alpha1.ReadyCondition) {
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

	if err = r.reconcile(ctx, s); err != nil {
		log.Error(err, "Failed to reconcile resource")
		return ctrl.Result{}, err
	}

	// force a periodic requeue to enforce state is in sync
	return ctrl.Result{RequeueAfter: Jitter(r.RequeueInterval)}, nil
}

// nveScope holds k8s objects used during a reconciliation.
type nveScope struct {
	Device         *v1alpha1.Device
	NVE            *v1alpha1.NetworkVirtualizationEdge
	Connection     *deviceutil.Connection
	ProviderConfig *provider.ProviderConfig
	Provider       provider.NVEProvider
}

func (r *NetworkVirtualizationEdgeReconciler) reconcile(ctx context.Context, s *nveScope) (reterr error) {
	if s.NVE.Labels == nil {
		s.NVE.Labels = make(map[string]string)
	}
	s.NVE.Labels[v1alpha1.DeviceLabel] = s.Device.Name

	if !controllerutil.HasControllerReference(s.NVE) {
		if err := controllerutil.SetOwnerReference(s.Device, s.NVE, r.Scheme, controllerutil.WithBlockOwnerDeletion(true)); err != nil {
			return err
		}
	}

	if err := r.validateUniqueNVEPerDevice(ctx, s); err != nil {
		return err
	}

	if err := r.validateProviderConfigRef(ctx, s); err != nil {
		return err
	}

	sourceIf, err := r.validateInterfaceRef(ctx, &s.NVE.Spec.SourceInterfaceRef, s)
	if err != nil {
		return err
	}

	anycastIf, err := r.validateInterfaceRef(ctx, s.NVE.Spec.AnycastSourceInterfaceRef, s)
	if err != nil {
		return err
	}

	if err := s.Provider.Connect(ctx, s.Connection); err != nil {
		return fmt.Errorf("failed to connect to provider: %w", err)
	}
	defer func() {
		if err := s.Provider.Disconnect(ctx, s.Connection); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	defer func() {
		conditions.RecomputeReady(s.NVE)
	}()

	err = s.Provider.EnsureNVE(ctx, &provider.NVERequest{
		NVE:                    s.NVE,
		ProviderConfig:         s.ProviderConfig,
		SourceInterface:        sourceIf,
		AnycastSourceInterface: anycastIf,
	})

	cond := conditions.FromError(err)
	conditions.Set(s.NVE, cond)
	if err != nil {
		return err
	}

	status, err := s.Provider.GetNVEStatus(ctx, &provider.NVERequest{
		NVE:            s.NVE,
		ProviderConfig: s.ProviderConfig,
	})
	if err != nil {
		return fmt.Errorf("failed to get NVE status: %w", err)
	}

	s.NVE.Status.SourceInterfaceName = status.SourceInterfaceName
	s.NVE.Status.AnycastSourceInterfaceName = status.AnycastSourceInterfaceName
	s.NVE.Status.HostReachability = status.HostReachabilityType

	cond = metav1.Condition{
		Type:    v1alpha1.OperationalCondition,
		Status:  metav1.ConditionTrue,
		Reason:  v1alpha1.OperationalReason,
		Message: "NVE is operationally up",
	}
	if !status.OperStatus {
		cond.Status = metav1.ConditionFalse
		cond.Reason = v1alpha1.DegradedReason
		cond.Message = "NVE is operationally down"
	}
	conditions.Set(s.NVE, cond)

	return nil
}

func (r *NetworkVirtualizationEdgeReconciler) validateUniqueNVEPerDevice(ctx context.Context, s *nveScope) error {
	var list v1alpha1.NetworkVirtualizationEdgeList
	if err := r.List(ctx, &list,
		client.InNamespace(s.NVE.Namespace),
		client.MatchingFields{".spec.deviceRef.name": s.NVE.Spec.DeviceRef.Name},
	); err != nil {
		return err
	}
	for _, nve := range list.Items {
		if nve.Name != s.NVE.Name {
			conditions.Set(s.NVE, metav1.Condition{
				Type:    v1alpha1.ConfiguredCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.NVEAlreadyExistsReason,
				Message: fmt.Sprintf("Another NVE (%s) already exists for device %s", nve.Name, s.NVE.Spec.DeviceRef.Name),
			})
			return reconcile.TerminalError(fmt.Errorf("only one NVE is allowed per device (%s)", s.NVE.Spec.DeviceRef.Name))
		}
	}
	return nil
}

// validateInterfaceRef checks that the referenced interface exists, is of type Loopback, and belongs to the same device as the NVE.
func (r *NetworkVirtualizationEdgeReconciler) validateInterfaceRef(ctx context.Context, interfaceRef *v1alpha1.LocalObjectReference, s *nveScope) (*v1alpha1.Interface, error) {
	if interfaceRef == nil {
		return nil, nil
	}
	intf := new(v1alpha1.Interface)
	intf.Name = interfaceRef.Name
	intf.Namespace = s.NVE.Namespace

	if err := r.Get(ctx, client.ObjectKey{Name: intf.Name, Namespace: intf.Namespace}, intf); err != nil {
		if apierrors.IsNotFound(err) {
			conditions.Set(s.NVE, metav1.Condition{
				Type:    v1alpha1.ConfiguredCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.WaitingForDependenciesReason,
				Message: fmt.Sprintf("interface resource '%s' not found in namespace '%s'", intf.Name, intf.Namespace),
			})
			return nil, reconcile.TerminalError(fmt.Errorf("referenced interface %q not found", s.NVE.Spec.SourceInterfaceRef.Name))
		}
		return nil, fmt.Errorf("failed to get interface %q: %w", s.NVE.Spec.SourceInterfaceRef.Name, err)
	}

	if intf.Spec.Type != v1alpha1.InterfaceTypeLoopback {
		conditions.Set(s.NVE, metav1.Condition{
			Type:    v1alpha1.ConfiguredCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.InvalidInterfaceTypeReason,
			Message: fmt.Sprintf("interface referenced by '%s' must be of type 'Loopback'", interfaceRef.Name),
		})
		return nil, reconcile.TerminalError(fmt.Errorf("interface referenced by '%s' must be of type 'Loopback'", interfaceRef.Name))
	}

	if s.NVE.Spec.DeviceRef.Name != intf.Spec.DeviceRef.Name {
		conditions.Set(s.NVE, metav1.Condition{
			Type:    v1alpha1.ConfiguredCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.CrossDeviceReferenceReason,
			Message: fmt.Sprintf("interface '%s' deviceRef '%s' does not match NVE deviceRef '%s'", intf.Name, intf.Spec.DeviceRef.Name, s.NVE.Spec.DeviceRef.Name),
		})
		return nil, reconcile.TerminalError(fmt.Errorf("interface '%s' deviceRef '%s' does not match NVE deviceRef '%s'", intf.Name, intf.Spec.DeviceRef.Name, s.NVE.Spec.DeviceRef.Name))
	}
	return intf, nil
}

// validateProviderConfigRef checks if the referenced provider configuration is compatible with the target platform.
func (r *NetworkVirtualizationEdgeReconciler) validateProviderConfigRef(_ context.Context, s *nveScope) error {
	if s.NVE.Spec.ProviderConfigRef == nil {
		return nil
	}
	gv, err := schema.ParseGroupVersion(s.NVE.Spec.ProviderConfigRef.APIVersion)
	if err != nil {
		conditions.Set(s.NVE, metav1.Condition{
			Type:    v1alpha1.ConfiguredCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.IncompatibleProviderConfigRef,
			Message: fmt.Sprintf("ProviderConfigRef is not compatible with Device: %v", err),
		})
		return reconcile.TerminalError(fmt.Errorf("invalid API version %q: %w", s.NVE.Spec.ProviderConfigRef.APIVersion, err))
	}

	if found := slices.Contains(v1alpha1.NetworkVirtualizationEdgeDependencies, schema.GroupVersionKind{
		Group:   gv.Group,
		Version: gv.Version,
		Kind:    s.NVE.Spec.ProviderConfigRef.Kind,
	}); !found {
		conditions.Set(s.NVE, metav1.Condition{
			Type:    v1alpha1.ConfiguredCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.IncompatibleProviderConfigRef,
			Message: fmt.Sprintf("ProviderConfigRef is not compatible with Device: %v", err),
		})
		return reconcile.TerminalError(fmt.Errorf("unsupported provider config ref kind %q for NVE on the provider", gv))
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NetworkVirtualizationEdgeReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
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

	// Index NVEs by their DeviceRef.name for uniqueness checks.
	if err := mgr.GetFieldIndexer().IndexField(ctx, &v1alpha1.NetworkVirtualizationEdge{}, ".spec.deviceRef.name", func(obj client.Object) []string {
		vpc := obj.(*v1alpha1.NetworkVirtualizationEdge)
		return []string{vpc.Spec.DeviceRef.Name}
	}); err != nil {
		return err
	}

	c := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.NetworkVirtualizationEdge{}).
		Named("nve").
		WithEventFilter(filter).
		Watches(
			&v1alpha1.Interface{},
			handler.EnqueueRequestsFromMapFunc(r.mapInterfaceToNVEs),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					return false
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			}),
		)

	for _, gvk := range v1alpha1.NetworkVirtualizationEdgeDependencies {
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(gvk)
		c = c.Watches(
			obj,
			handler.EnqueueRequestsFromMapFunc(r.mapProviderConfigToNVEs),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		)
	}
	return c.Complete(r)
}

// mapProviderConfigToNVEs is a [handler.MapFunc] to re-enqueue NVEs that require reconciliation, i.e.,
// whose referenced provider configuration has changed.
func (r *NetworkVirtualizationEdgeReconciler) mapProviderConfigToNVEs(ctx context.Context, obj client.Object) []reconcile.Request {
	log := ctrl.LoggerFrom(ctx, "Object", klog.KObj(obj))

	list := &v1alpha1.NetworkVirtualizationEdgeList{}
	if err := r.List(ctx, list, client.InNamespace(obj.GetNamespace())); err != nil {
		log.Error(err, "Failed to list NVEs")
		return nil
	}

	gkv := obj.GetObjectKind().GroupVersionKind()

	var requests []reconcile.Request
	for _, m := range list.Items {
		if m.Spec.ProviderConfigRef != nil &&
			m.Spec.ProviderConfigRef.Name == obj.GetName() &&
			m.Spec.ProviderConfigRef.Kind == gkv.Kind &&
			m.Spec.ProviderConfigRef.APIVersion == gkv.GroupVersion().Identifier() {
			log.Info("Enqueuing NVE for reconciliation", "NVE", klog.KObj(&m))
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

// mapInterfaceToNVEs is a [handler.MapFunc] to re-enqueue NVEs that reference the given Interface.
func (r *NetworkVirtualizationEdgeReconciler) mapInterfaceToNVEs(ctx context.Context, obj client.Object) []reconcile.Request {
	intf, ok := obj.(*v1alpha1.Interface)
	if !ok {
		panic(fmt.Sprintf("Expected an Interface but got a %T", obj))
	}
	log := ctrl.LoggerFrom(ctx)
	nves := &v1alpha1.NetworkVirtualizationEdgeList{}
	if err := r.List(ctx, nves, client.InNamespace(obj.GetNamespace())); err != nil {
		log.Error(err, "Failed to list NVEs")
		return nil
	}

	requests := []ctrl.Request{}
	for _, i := range nves.Items {
		if i.Spec.SourceInterfaceRef.Name == intf.Spec.Name ||
			(i.Spec.AnycastSourceInterfaceRef != nil && i.Spec.AnycastSourceInterfaceRef.Name == intf.Spec.Name) {
			log.Info("Enqueuing NVE for reconciliation", "NVE", klog.KObj(&i))
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

func (r *NetworkVirtualizationEdgeReconciler) finalize(ctx context.Context, s *nveScope) (reterr error) {
	if err := s.Provider.Connect(ctx, s.Connection); err != nil {
		return fmt.Errorf("failed to connect to provider: %w", err)
	}
	defer func() {
		if err := s.Provider.Disconnect(ctx, s.Connection); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	// TDO: do we need the other or just works with refs and finalizers?
	return s.Provider.DeleteNVE(ctx, &provider.NVERequest{
		NVE: s.NVE,
	})
}
