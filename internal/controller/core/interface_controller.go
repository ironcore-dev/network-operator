// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"
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

// InterfaceReconciler reconciles a Interface object
type InterfaceReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// WatchFilterValue is the label value used to filter events prior to reconciliation.
	WatchFilterValue string

	// Recorder is used to record events for the controller.
	// More info: https://book.kubebuilder.io/reference/raising-events
	Recorder events.EventRecorder

	// Provider is the driver that will be used to create & delete the interface.
	Provider provider.ProviderFunc

	// Locker is used to synchronize operations on resources targeting the same device.
	Locker *resourcelock.ResourceLocker

	// RequeueInterval is the duration after which the controller should requeue the reconciliation,
	// regardless of changes.
	RequeueInterval time.Duration
}

// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=interfaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=interfaces/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=interfaces/finalizers,verbs=update
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=vlans,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=vlans/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=vrfs,verbs=get;list;watch
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

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
	log.V(3).Info("Reconciling resource")

	obj := new(v1alpha1.Interface)
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

	if isPaused, requeue, err := paused.EnsureCondition(ctx, r.Client, device, obj); isPaused || requeue || err != nil {
		return ctrl.Result{Requeue: requeue}, err
	}

	if err := r.Locker.AcquireLock(ctx, device.Name, "interface-controller"); err != nil {
		if errors.Is(err, resourcelock.ErrLockAlreadyHeld) {
			log.V(3).Info("Device is already locked, requeuing reconciliation")
			return ctrl.Result{RequeueAfter: Jitter(time.Second), Priority: new(LockWaitPriorityHigh)}, nil
		}
		log.Error(err, "Failed to acquire device lock")
		return ctrl.Result{}, err
	}
	defer func() {
		if err := r.Locker.ReleaseLock(ctx, device.Name, "interface-controller"); err != nil {
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
	if conditions.InitializeConditions(obj, v1alpha1.ReadyCondition, v1alpha1.ConfiguredCondition, v1alpha1.OperationalCondition) {
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

	if err := r.reconcile(ctx, s); err != nil {
		log.Error(err, "Failed to reconcile resource")
		return ctrl.Result{}, apistatus.WrapTerminalError(err)
	}

	return ctrl.Result{RequeueAfter: Jitter(r.RequeueInterval)}, nil
}

const (
	interfaceTypeKey          = ".spec.type"
	interfaceUnnumberedRefKey = ".spec.ipv4.unnumbered.interfaceRef.name"
	interfaceVlanRefKey       = ".spec.vlanRef.name"
	interfaceVrfRefKey        = ".spec.vrfRef.name"
	interfaceParentRefKey     = ".spec.parentInterfaceRef.name"
)

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

	if err := mgr.GetFieldIndexer().IndexField(ctx, &v1alpha1.Interface{}, interfaceTypeKey, func(obj client.Object) []string {
		intf := obj.(*v1alpha1.Interface)
		return []string{string(intf.Spec.Type)}
	}); err != nil {
		return err
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

	if err := mgr.GetFieldIndexer().IndexField(ctx, &v1alpha1.Interface{}, interfaceVlanRefKey, func(obj client.Object) []string {
		intf := obj.(*v1alpha1.Interface)
		if intf.Spec.VlanRef == nil {
			return nil
		}
		return []string{intf.Spec.VlanRef.Name}
	}); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(ctx, &v1alpha1.Interface{}, interfaceVrfRefKey, func(obj client.Object) []string {
		intf := obj.(*v1alpha1.Interface)
		if intf.Spec.VrfRef == nil {
			return nil
		}
		return []string{intf.Spec.VrfRef.Name}
	}); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(ctx, &v1alpha1.Interface{}, v1alpha1.DeviceRefIndexKey, func(obj client.Object) []string {
		o := obj.(*v1alpha1.Interface)
		return []string{o.Spec.DeviceRef.Name}
	}); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(ctx, &v1alpha1.Interface{}, interfaceParentRefKey, func(obj client.Object) []string {
		intf := obj.(*v1alpha1.Interface)
		if intf.Spec.ParentInterfaceRef == nil {
			return nil
		}
		return []string{intf.Spec.ParentInterfaceRef.Name}
	}); err != nil {
		return err
	}

	bldr := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Interface{}, builder.WithPredicates(predicate.Or(
			predicate.GenerationChangedPredicate{},
			predicate.LabelChangedPredicate{},
			predicate.AnnotationChangedPredicate{},
			interfaceUpdatePredicate{},
		))).
		Named("interface").
		WithEventFilter(filter)

	for _, gvk := range v1alpha1.InterfaceDependencies {
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(gvk)

		bldr = bldr.Watches(
			obj,
			handler.EnqueueRequestsFromMapFunc(r.interfacesForProviderConfig),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		)
	}

	return bldr.
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
		// Watches enqueues subinterfaces when their parent interface changes.
		Watches(
			&v1alpha1.Interface{},
			handler.EnqueueRequestsFromMapFunc(r.parentToSubinterfaces),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					return false
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			}),
		).
		// Watches enqueues Aggregate Interfaces for updates in referenced member resources.
		Watches(
			&v1alpha1.Interface{},
			handler.EnqueueRequestsFromMapFunc(r.interfaceToAggregate),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					return false
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			}),
		).
		// Watches enqueues member Physical Interfaces when their parent Aggregate changes.
		// Only triggers when Aggregate Spec fields change that affect member reconciliation.
		Watches(
			&v1alpha1.Interface{},
			handler.EnqueueRequestsFromMapFunc(r.aggregateToMembers),
			builder.WithPredicates(predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool {
					return false
				},
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldIntf := e.ObjectOld.(*v1alpha1.Interface)
					newIntf := e.ObjectNew.(*v1alpha1.Interface)
					// Only trigger when fields that affect member Physical interface
					// reconciliation change (e.g. layer, VRF membership, MTU).
					return !equality.Semantic.DeepEqual(oldIntf.Spec.IPv4, newIntf.Spec.IPv4) ||
						!equality.Semantic.DeepEqual(oldIntf.Spec.Switchport, newIntf.Spec.Switchport) ||
						!equality.Semantic.DeepEqual(oldIntf.Spec.VrfRef, newIntf.Spec.VrfRef) ||
						oldIntf.Spec.MTU != newIntf.Spec.MTU
				},
				DeleteFunc: func(e event.DeleteEvent) bool {
					return false
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			}),
		).
		// Watches enqueues RoutedVLAN Interfaces for updates in referenced VLAN resources.
		// Only triggers on create and delete events since VLAN IDs are immutable.
		Watches(
			&v1alpha1.VLAN{},
			handler.EnqueueRequestsFromMapFunc(r.vlanToRoutedVLAN),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					return false
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			}),
		).
		// Watches enqueues Interfaces for updates in referenced VRF resources.
		// Only triggers on create and delete events since VRF names are immutable.
		Watches(
			&v1alpha1.VRF{},
			handler.EnqueueRequestsFromMapFunc(r.vrfToInterface),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					return false
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			}),
		).
		// Watches enqueues Interfaces for updates in referenced Device resources.
		// Triggers on create, delete, and update events when the device's effective pause state changes.
		Watches(
			&v1alpha1.Device{},
			handler.EnqueueRequestsFromMapFunc(r.deviceToInterfaces),
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

// interfaceUpdatePredicate passes status-only updates through unless the
// neighbor ExpirationTime is the only change. Without this filter the status
// patch would immediately trigger a redundant second reconcile because the
// controller updates the ExpirationTime in the status during every reconcile loop.
// This would cause repeated reconciles every time the controller tries to update
// the status, even if there are no changes to the spec.
type interfaceUpdatePredicate struct {
	predicate.Funcs
}

// Update implements predicate.Predicate.
func (interfaceUpdatePredicate) Update(e event.UpdateEvent) bool {
	oldIntf, ok := e.ObjectOld.(*v1alpha1.Interface)
	if !ok {
		return true
	}
	newIntf, ok := e.ObjectNew.(*v1alpha1.Interface)
	if !ok {
		return true
	}
	// Always reconcile if conditions haven't been fully initialized.
	// InitializeConditions adds Ready/Configured/Operational, and paused.EnsureCondition
	// adds Paused. Until all 4 are present, we must allow reconciles to complete setup.
	if len(newIntf.Status.Conditions) < 4 {
		return true
	}
	oldStatus := oldIntf.Status.DeepCopy()
	newStatus := newIntf.Status.DeepCopy()
	for i := range oldStatus.Neighbors {
		oldStatus.Neighbors[i].ExpirationTime = metav1.Time{}
	}
	for i := range newStatus.Neighbors {
		newStatus.Neighbors[i].ExpirationTime = metav1.Time{}
	}
	return !equality.Semantic.DeepEqual(oldStatus, newStatus)
}

// scope holds the different objects that are read and used during the reconcile.
type scope struct {
	Device         *v1alpha1.Device
	Interface      *v1alpha1.Interface
	Connection     *deviceutil.Connection
	ProviderConfig *provider.ProviderConfig
	Provider       provider.InterfaceProvider
}

func (r *InterfaceReconciler) reconcile(ctx context.Context, s *scope) (reterr error) {
	if s.Interface.Labels == nil {
		s.Interface.Labels = make(map[string]string)
	}

	s.Interface.Labels[v1alpha1.DeviceLabel] = s.Device.Name

	// Ensure the Interface (except subinterfaces) is owned by the Device.
	// Subinterfaces have their parent interface as owner, and the parent interface is owned by the Device.
	if !controllerutil.HasControllerReference(s.Interface) && s.Interface.Spec.Type != v1alpha1.InterfaceTypeSubinterface {
		if err := controllerutil.SetOwnerReference(s.Device, s.Interface, r.Scheme, controllerutil.WithBlockOwnerDeletion(true)); err != nil {
			return err
		}
	}

	defer func() {
		conditions.RecomputeReady(s.Interface)
	}()

	var members []*v1alpha1.Interface
	if s.Interface.Spec.Aggregation != nil {
		var err error
		members, err = r.reconcileMemberInterfaces(ctx, s)
		if err != nil {
			return err
		}
	}

	var aggregateParent *v1alpha1.Interface
	if s.Interface.Spec.Type == v1alpha1.InterfaceTypePhysical && s.Interface.Status.MemberOf != nil {
		aggregateParent = new(v1alpha1.Interface)
		key := client.ObjectKey{Name: s.Interface.Status.MemberOf.Name, Namespace: s.Interface.Namespace}
		if err := r.Get(ctx, key, aggregateParent); err != nil {
			if !apierrors.IsNotFound(err) {
				return fmt.Errorf("failed to get aggregate parent %q: %w", s.Interface.Status.MemberOf.Name, err)
			}
			aggregateParent = nil
		}
	}

	if s.Interface.Spec.Type == v1alpha1.InterfaceTypeSubinterface {
		err := r.reconcileSubinterfaces(ctx, s)
		if err != nil {
			return err
		}
	}

	var multiChassisID *int16
	if s.Interface.Spec.Aggregation != nil && s.Interface.Spec.Aggregation.MultiChassis != nil {
		multiChassisID = &s.Interface.Spec.Aggregation.MultiChassis.ID
	}

	var vlan *v1alpha1.VLAN
	if s.Interface.Spec.VlanRef != nil {
		var err error
		vlan, err = r.reconcileVLAN(ctx, s)
		if err != nil {
			return err
		}
	}

	var vrf *v1alpha1.VRF
	if s.Interface.Spec.VrfRef != nil {
		var err error
		vrf, err = r.reconcileVRF(ctx, s)
		if err != nil {
			return err
		}
	}

	var ip provider.IPv4
	if s.Interface.Spec.IPv4 != nil && (len(s.Interface.Spec.IPv4.Addresses) > 0 || s.Interface.Spec.IPv4.Unnumbered != nil) {
		var err error
		ip, err = r.reconcileIPv4(ctx, s)
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

	// Ensure the Interface is realized on the provider.
	err := s.Provider.EnsureInterface(ctx, &provider.EnsureInterfaceRequest{
		Interface:       s.Interface,
		ProviderConfig:  s.ProviderConfig,
		IPv4:            ip,
		Members:         members,
		MultiChassisID:  multiChassisID,
		AggregateParent: aggregateParent,
		VLAN:            vlan,
		VRF:             vrf,
	})

	cond := conditions.FromError(err)
	conditions.Set(s.Interface, cond)

	if err != nil {
		return err
	}

	status, err := s.Provider.GetInterfaceStatus(ctx, &provider.InterfaceRequest{
		Interface:      s.Interface,
		ProviderConfig: s.ProviderConfig,
	})
	if err != nil {
		return fmt.Errorf("failed to get interface status: %w", err)
	}

	r.reconcileInterfaceStatus(ctx, s, &status)

	return nil
}

func (r *InterfaceReconciler) reconcileInterfaceStatus(ctx context.Context, s *scope, status *provider.InterfaceStatus) {
	// Neighbor adjacencies is only metadata and should not prevent reconciliation
	if s.Interface.Spec.Type == v1alpha1.InterfaceTypePhysical && len(status.LLDPAdjacencies) > 0 {
		if err := r.updateNeighborAdjacenciesStatus(ctx, s, status); err != nil {
			ctrl.LoggerFrom(ctx).Error(err, "failed to update neighbor adjacency status", "interface", klog.KObj(s.Interface))
		}
	} else {
		s.Interface.Status.Neighbors = nil
	}

	cond := metav1.Condition{
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
	if status.OperMessage != "" {
		cond.Message = fmt.Sprintf("Device returned %q", status.OperMessage)
	}
	conditions.Set(s.Interface, cond)
}

// updateNeighborAdjacenciesStatus updates the Interface status with the LLDP neighbor adjacencies returned by the provider.
// It validates the adjacencies by looking at the corresponding label/annotation.
// It first attempts to validate through label (neighbor is managed by the operator and exists as a kubernetes resource).
// If that fails, it attempts to validate through annotation (neighbor is not managed by the operator).
// Only returns an error if there is an issue during the validation process, but does not return an error if the validation fails (i.e. the adjacency is marked as invalid).
func (r *InterfaceReconciler) updateNeighborAdjacenciesStatus(ctx context.Context, s *scope, status *provider.InterfaceStatus) error {
	type neighborKey struct{ ChassisID, PortID string }

	existingNeighbors := make(map[neighborKey]v1alpha1.Neighbor)
	for _, n := range s.Interface.Status.Neighbors {
		existingNeighbors[neighborKey{n.ChassisID, n.PortID}] = n
	}

	log := ctrl.LoggerFrom(ctx)
	if len(status.LLDPAdjacencies) > 1 {
		log.V(1).Info("Multiple LLDP adjacencies found for a single interface, will validate each adjacency against one single label/annotation", "interface", klog.KObj(s.Interface), "adjacencyCount", len(status.LLDPAdjacencies))
	}

	var errs []error
	neighbors := make([]v1alpha1.Neighbor, 0, len(status.LLDPAdjacencies))
	for _, adj := range status.LLDPAdjacencies {
		chassisIDType, ok := v1alpha1.ChassisIDTypeFromValue(adj.ChassisIDType)
		if !ok {
			log.V(1).Info("Skipping LLDP adjacency with unknown chassis ID type", "chassisID", adj.ChassisID, "chassisIDType", adj.ChassisIDType)
			continue
		}
		portIDType, ok := v1alpha1.PortIDTypeFromValue(adj.PortIDType)
		if !ok {
			log.V(1).Info("Skipping LLDP adjacency with unknown port ID type", "chassisID", adj.ChassisID, "portIDType", adj.PortIDType)
			continue
		}

		adjacency := v1alpha1.Neighbor{
			SystemName:        adj.SysName,
			SystemDescription: adj.SysDescription,
			ChassisID:         adj.ChassisID,
			ChassisIDType:     chassisIDType,
			PortID:            adj.PortID,
			PortIDType:        portIDType,
			PortDescription:   adj.PortDescription,
			ExpirationTime:    metav1.NewTime(time.Now().Add(adj.TTL).Truncate(time.Second)),
		}

		// NOTE: the operator runs a single provider currently, so s.Provider is used for the
		// remote device as well. If multi-provider support is added, the remote device's
		// provider must be resolved here instead.
		if neighborLabelValue, ok := s.Interface.Labels[v1alpha1.PhysicalInterfaceNeighborLabel]; ok {
			var err error
			if adjacency.Validation, err = r.validateLLDPAdjacencyThroughLabel(ctx, s.Provider, s.Interface, &adjacency, neighborLabelValue); err != nil {
				errs = append(errs, fmt.Errorf("failed to validate LLDP adjacency %q/%q through label %q: %w", adj.ChassisID, adj.PortID, neighborLabelValue, err))
			}
		}

		if neighborAnnotationValue, ok := s.Interface.Annotations[v1alpha1.PhysicalInterfaceNeighborRawAnnotation]; ok && adjacency.Validation == "" {
			var err error
			if adjacency.Validation, err = r.validateLLDPAdjacencyThroughAnnotation(ctx, s.Interface, &adjacency, neighborAnnotationValue); err != nil {
				errs = append(errs, fmt.Errorf("failed to validate LLDP adjacency %q/%q through annotation %q: %w", adj.ChassisID, adj.PortID, neighborAnnotationValue, err))
			}
		}
		neighbors = append(neighbors, adjacency)
	}

	s.Interface.Status.Neighbors = neighbors

	return kerrors.NewAggregate(errs)
}

func (r *InterfaceReconciler) validateLLDPAdjacencyThroughLabel(ctx context.Context, remoteProvider provider.InterfaceProvider, intf *v1alpha1.Interface, n *v1alpha1.Neighbor, label string) (v1alpha1.NeighborValidation, error) {
	key := client.ObjectKey{
		Name:      label,
		Namespace: intf.Namespace,
	}

	remoteIntf := new(v1alpha1.Interface)
	if err := r.Get(ctx, key, remoteIntf); err != nil {
		if !apierrors.IsNotFound(err) {
			return "", fmt.Errorf("failed to get neighbor interface %w", err)
		}
		return v1alpha1.NeighborNotFound, nil
	}

	log := ctrl.LoggerFrom(ctx, "LLDP validation", klog.KObj(intf))

	remoteDevice, err := deviceutil.GetDeviceByName(ctx, r, remoteIntf.Namespace, remoteIntf.Spec.DeviceRef.Name)
	if err != nil {
		return "", fmt.Errorf("could not find the device for interface %q: %w", remoteIntf.Name, err)
	}

	if remoteDevice.Status.Hostname == "" {
		return "", fmt.Errorf("the neighbor device does not have a hostname yet, cannot validate adjacency: neighborInterface=%q", remoteIntf.Name)
	}

	if remoteDevice.Status.Hostname != n.SystemName {
		log.V(1).Info("the neighbor device hostname does not match", "expected", n.SystemName, "actual", remoteDevice.Status.Hostname)
		return v1alpha1.NeighborDeviceMismatch, nil
	}

	equal, err := remoteProvider.InterfaceNameEqual(ctx, remoteIntf.Spec.Name, n.PortID)
	if err != nil {
		return "", fmt.Errorf("failed to compare interface names %q and %q: %w", remoteIntf.Spec.Name, n.PortID, err)
	}
	if !equal {
		log.V(1).Info("the neighbor interface name does not match", "expected", n.PortID, "actual", remoteIntf.Spec.Name)
		return v1alpha1.NeighborPortMismatch, nil
	}

	return v1alpha1.NeighborVerified, nil
}

func (r *InterfaceReconciler) validateLLDPAdjacencyThroughAnnotation(ctx context.Context, intf *v1alpha1.Interface, n *v1alpha1.Neighbor, annotation string) (v1alpha1.NeighborValidation, error) {
	remoteDeviceID, remotePortID, ok := strings.Cut(annotation, "::")
	if !ok || remoteDeviceID == "" || remotePortID == "" {
		return "", errors.New("invalid neighbor annotation value, expected format is <deviceIdentifier>::<portID>")
	}

	log := ctrl.LoggerFrom(ctx, "LLDP validation", klog.KObj(intf))
	if remoteDeviceID != n.ChassisID && remoteDeviceID != n.SystemName {
		log.V(1).Info("the neighbor device identifier does not match", "annotationValue", remoteDeviceID, "chassisID", n.ChassisID, "systemName", n.SystemName)
		return v1alpha1.NeighborDeviceMismatch, nil
	}

	if remotePortID != n.PortID {
		log.V(1).Info("the neighbor port identifier does not match", "annotationValue", remotePortID, "portID", n.PortID)
		return v1alpha1.NeighborPortMismatch, nil
	}

	return v1alpha1.NeighborVerified, nil
}

func (r *InterfaceReconciler) reconcileIPv4(ctx context.Context, s *scope) (provider.IPv4, error) {
	switch {
	case len(s.Interface.Spec.IPv4.Addresses) > 0:
		addrs := make([]netip.Prefix, len(s.Interface.Spec.IPv4.Addresses))
		for i, addr := range s.Interface.Spec.IPv4.Addresses {
			addrs[i] = addr.Prefix
		}
		return provider.IPv4AddressList(addrs), nil

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
					Reason:  v1alpha1.InterfaceNotFoundReason,
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
				Reason:  v1alpha1.CrossDeviceReferenceReason,
				Message: fmt.Sprintf("referenced interface %q for unnumbered ipv4 configuration does not belong to device %q", intf.Name, s.Device.Name),
			})
			return nil, reconcile.TerminalError(fmt.Errorf("referenced interface %q for unnumbered ipv4 configuration does not belong to device %q", intf.Name, s.Device.Name))
		}

		if intf.Spec.Type != v1alpha1.InterfaceTypeLoopback {
			conditions.Set(s.Interface, metav1.Condition{
				Type:    v1alpha1.ConfiguredCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.InvalidInterfaceTypeReason,
				Message: fmt.Sprintf("referenced interface %q for unnumbered ipv4 configuration is not of type Loopback, got %q", intf.Name, intf.Spec.Type),
			})
			return nil, reconcile.TerminalError(fmt.Errorf("referenced interface %q for unnumbered ipv4 configuration is not of type Loopback, got %q", intf.Name, intf.Spec.Type))
		}

		return provider.IPv4Unnumbered{SourceInterface: intf.Spec.Name}, nil
	default:
		panic("unreachable")
	}
}

// reconcileVLAN ensures that the referenced VLAN exists, belongs to the same device as the RoutedVLAN interface.
// It also updates the VLAN to reference the RoutedVLAN interface by setting its RoutedBy status field.
func (r *InterfaceReconciler) reconcileVLAN(ctx context.Context, s *scope) (*v1alpha1.VLAN, error) {
	key := client.ObjectKey{
		Name:      s.Interface.Spec.VlanRef.Name,
		Namespace: s.Interface.Namespace,
	}

	vlan := new(v1alpha1.VLAN)
	if err := r.Get(ctx, key, vlan); err != nil {
		if apierrors.IsNotFound(err) {
			conditions.Set(s.Interface, metav1.Condition{
				Type:    v1alpha1.ConfiguredCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.VLANNotFoundReason,
				Message: fmt.Sprintf("referenced VLAN %q not found", key),
			})
			return nil, reconcile.TerminalError(fmt.Errorf("referenced VLAN %q not found", key))
		}
		return nil, fmt.Errorf("failed to get referenced VLAN %q: %w", key, err)
	}

	if vlan.Spec.DeviceRef.Name != s.Device.Name {
		conditions.Set(s.Interface, metav1.Condition{
			Type:    v1alpha1.ConfiguredCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.CrossDeviceReferenceReason,
			Message: fmt.Sprintf("referenced VLAN %q does not belong to device %q", vlan.Name, s.Device.Name),
		})
		return nil, reconcile.TerminalError(fmt.Errorf("referenced VLAN %q does not belong to device %q", vlan.Name, s.Device.Name))
	}

	if vlan.Status.RoutedBy != nil && vlan.Status.RoutedBy.Name != s.Interface.Name {
		conditions.Set(s.Interface, metav1.Condition{
			Type:    v1alpha1.ConfiguredCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.VLANAlreadyInUseReason,
			Message: fmt.Sprintf("VLAN %q is already in use by routed VLAN interface %q", vlan.Name, vlan.Status.RoutedBy.Name),
		})
		return nil, reconcile.TerminalError(fmt.Errorf("VLAN %q is already in use by routed VLAN interface %q", vlan.Name, vlan.Status.RoutedBy.Name))
	}

	if vlan.Status.RoutedBy == nil {
		vlan.Status.RoutedBy = &v1alpha1.LocalObjectReference{Name: s.Interface.Name}
		if err := r.Status().Update(ctx, vlan); err != nil {
			return nil, fmt.Errorf("failed to update VLAN %q status: %w", vlan.Name, err)
		}
	}

	if vlan.Labels == nil {
		vlan.Labels = make(map[string]string)
	}

	if vlan.Labels[v1alpha1.RoutedVLANLabel] != s.Interface.Name {
		vlan.Labels[v1alpha1.RoutedVLANLabel] = s.Interface.Name
		if err := r.Update(ctx, vlan); err != nil {
			return nil, fmt.Errorf("failed to update VLAN %q labels: %w", vlan.Name, err)
		}
	}

	return vlan, nil
}

// reconcileVRF ensures that the referenced VRF exists and belongs to the same device as the Interface.
// It also adds a label to the Interface indicating which VRF it belongs to. This can be used for lookup purposes.
func (r *InterfaceReconciler) reconcileVRF(ctx context.Context, s *scope) (*v1alpha1.VRF, error) {
	key := client.ObjectKey{
		Name:      s.Interface.Spec.VrfRef.Name,
		Namespace: s.Interface.Namespace,
	}

	vrf := new(v1alpha1.VRF)
	if err := r.Get(ctx, key, vrf); err != nil {
		if apierrors.IsNotFound(err) {
			conditions.Set(s.Interface, metav1.Condition{
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
		conditions.Set(s.Interface, metav1.Condition{
			Type:    v1alpha1.ConfiguredCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.CrossDeviceReferenceReason,
			Message: fmt.Sprintf("referenced VRF %q does not belong to device %q", vrf.Name, s.Device.Name),
		})
		return nil, reconcile.TerminalError(fmt.Errorf("referenced VRF %q does not belong to device %q", vrf.Name, s.Device.Name))
	}

	// Add label to interface indicating which VRF it belongs to
	if s.Interface.Labels == nil {
		s.Interface.Labels = make(map[string]string)
	}

	if s.Interface.Labels[v1alpha1.VRFLabel] != vrf.Name {
		s.Interface.Labels[v1alpha1.VRFLabel] = vrf.Name
	}

	return vrf, nil
}

// reconcileMemberInterfaces ensures that all member interfaces exist and belong to the same device as the aggregate interface.
// It also updates the member interfaces to reference the aggregate interface by setting their MemberOf status field and [v1alpha1.AggregateLabel] label.
func (r *InterfaceReconciler) reconcileMemberInterfaces(ctx context.Context, s *scope) ([]*v1alpha1.Interface, error) {
	members := make([]*v1alpha1.Interface, 0, len(s.Interface.Spec.Aggregation.MemberInterfaceRefs))
	for _, ref := range s.Interface.Spec.Aggregation.MemberInterfaceRefs {
		intf := new(v1alpha1.Interface)
		if err := r.Get(ctx, client.ObjectKey{Name: ref.Name, Namespace: s.Interface.Namespace}, intf); err != nil {
			if apierrors.IsNotFound(err) {
				conditions.Set(s.Interface, metav1.Condition{
					Type:    v1alpha1.ConfiguredCondition,
					Status:  metav1.ConditionFalse,
					Reason:  v1alpha1.InterfaceNotFoundReason,
					Message: fmt.Sprintf("member interface %q not found", ref.Name),
				})
				return nil, reconcile.TerminalError(fmt.Errorf("member interface %q not found", ref.Name))
			}
			return nil, fmt.Errorf("failed to get member interface %q: %w", ref.Name, err)
		}

		if intf.Spec.DeviceRef.Name != s.Device.Name {
			conditions.Set(s.Interface, metav1.Condition{
				Type:    v1alpha1.ConfiguredCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.CrossDeviceReferenceReason,
				Message: fmt.Sprintf("member interface %q does not belong to device %q", intf.Name, s.Device.Name),
			})
			return nil, reconcile.TerminalError(fmt.Errorf("member interface %q does not belong to device %q", intf.Name, s.Device.Name))
		}

		if intf.Status.MemberOf != nil && intf.Status.MemberOf.Name != s.Interface.Name {
			conditions.Set(s.Interface, metav1.Condition{
				Type:    v1alpha1.ConfiguredCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.MemberInterfaceAlreadyInUseReason,
				Message: fmt.Sprintf("member interface %q is already part of aggregate interface %q", intf.Name, *intf.Status.MemberOf),
			})
			return nil, reconcile.TerminalError(fmt.Errorf("member interface %q is already part of aggregate interface %q", intf.Name, *intf.Status.MemberOf))
		}

		if intf.Spec.Type != v1alpha1.InterfaceTypePhysical {
			conditions.Set(s.Interface, metav1.Condition{
				Type:    v1alpha1.ConfiguredCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.InvalidInterfaceTypeReason,
				Message: fmt.Sprintf("member interface %q is not of type Physical", intf.Name),
			})
			return nil, reconcile.TerminalError(fmt.Errorf("member interface %q is not of type Physical", intf.Name))
		}

		if intf.Status.MemberOf == nil {
			intf.Status.MemberOf = &v1alpha1.LocalObjectReference{Name: s.Interface.Name}
			if err := r.Status().Update(ctx, intf); err != nil {
				return nil, fmt.Errorf("failed to update member interface %q status: %w", intf.Name, err)
			}
		}

		if intf.Labels == nil {
			intf.Labels = make(map[string]string)
		}

		if intf.Labels[v1alpha1.AggregateLabel] != s.Interface.Name {
			intf.Labels[v1alpha1.AggregateLabel] = s.Interface.Name
			if err := r.Update(ctx, intf); err != nil {
				return nil, fmt.Errorf("failed to update member interface %q labels: %w", intf.Name, err)
			}
		}

		members = append(members, intf)
	}

	return members, nil
}

// reconcileSubinterfaces ensures that the parent interfaces exist and belong to the same device as the subinterface.
// It also updates the subinterfaces owner reference to the parent interface
func (r *InterfaceReconciler) reconcileSubinterfaces(ctx context.Context, s *scope) error {
	parentIntf := new(v1alpha1.Interface)

	key := client.ObjectKey{Name: s.Interface.Spec.ParentInterfaceRef.Name, Namespace: s.Interface.Namespace}
	if err := r.Get(ctx, key, parentIntf); err != nil {
		if apierrors.IsNotFound(err) {
			conditions.Set(s.Interface, metav1.Condition{
				Type:    v1alpha1.ConfiguredCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.ParentInterfaceNotFoundReason,
				Message: fmt.Sprintf("referenced parent interface %q for not found", key),
			})
			return reconcile.TerminalError(fmt.Errorf("failed to get parent interface %q: %w", s.Interface.Spec.ParentInterfaceRef.Name, err))
		}
		return err
	}

	// Check matching device reference
	if parentIntf.Spec.DeviceRef.Name != s.Device.Name {
		conditions.Set(s.Interface, metav1.Condition{
			Type:    v1alpha1.ConfiguredCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.CrossDeviceReferenceReason,
			Message: fmt.Sprintf("parent interface %q belongs to a different device", key),
		})
		return reconcile.TerminalError(fmt.Errorf("parent interface %q belongs to device %q in not ready state", s.Interface.Spec.ParentInterfaceRef.Name, parentIntf.Spec.DeviceRef.Name))
	}

	// Check if parent interface is an aggregate or physical interface
	if parentIntf.Spec.Type != v1alpha1.InterfaceTypePhysical && parentIntf.Spec.Type != v1alpha1.InterfaceTypeAggregate {
		conditions.Set(s.Interface, metav1.Condition{
			Type:    v1alpha1.ConfiguredCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.InvalidParentInterfaceTypeReason,
			Message: fmt.Sprintf("parent interface %q is not of type Physical or Aggregate, got %q", key, parentIntf.Spec.Type),
		})
		return reconcile.TerminalError(fmt.Errorf("parent interface %q is not of type Physical or Aggregate, got %q", s.Interface.Spec.ParentInterfaceRef.Name, parentIntf.Spec.Type))
	}

	// L2 interfaces do not support subinterfaces config
	if parentIntf.Spec.Switchport != nil {
		conditions.Set(s.Interface, metav1.Condition{
			Type:    v1alpha1.ConfiguredCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.InvalidInterfaceTypeReason,
			Message: fmt.Sprintf("parent interface %q is an L2 interface", key),
		})
		return reconcile.TerminalError(fmt.Errorf("parent interface %q is an L2 interface", s.Interface.Spec.ParentInterfaceRef.Name))
	}

	// Ensure the Subinterface is owned by the parent interface.
	if !controllerutil.HasControllerReference(s.Interface) {
		if err := controllerutil.SetOwnerReference(parentIntf, s.Interface, r.Scheme, controllerutil.WithBlockOwnerDeletion(true)); err != nil {
			return reconcile.TerminalError(err)
		}
	}

	// Parent interface must be configured
	if !conditions.IsConfigured(parentIntf) {
		conditions.Set(s.Interface, metav1.Condition{
			Type:    v1alpha1.ConfiguredCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.ParentInterfaceNotConfiguredReason,
			Message: fmt.Sprintf("parent interface %q not ready", key),
		})
		return reconcile.TerminalError(fmt.Errorf("parent interface %q in not ready state", s.Interface.Spec.ParentInterfaceRef.Name))
	}

	return nil
}

func (r *InterfaceReconciler) finalize(ctx context.Context, s *scope) (reterr error) {
	if s.Interface.Spec.Aggregation != nil {
		if err := r.finalizeMemberInterfaces(ctx, s); err != nil {
			return err
		}
	}

	if err := r.finalizeVLAN(ctx, s); err != nil {
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

	return s.Provider.DeleteInterface(ctx, &provider.InterfaceRequest{
		Interface:      s.Interface,
		ProviderConfig: s.ProviderConfig,
	})
}

// finalizeMemberInterfaces removes the aggregate interface references from all member interfaces.
func (r *InterfaceReconciler) finalizeMemberInterfaces(ctx context.Context, s *scope) error {
	for _, ref := range s.Interface.Spec.Aggregation.MemberInterfaceRefs {
		intf := new(v1alpha1.Interface)
		if err := r.Get(ctx, client.ObjectKey{Name: ref.Name, Namespace: s.Interface.Namespace}, intf); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return err
		}

		if intf.Status.MemberOf != nil && intf.Status.MemberOf.Name == s.Interface.Name {
			intf.Status.MemberOf = nil
			if err := r.Status().Update(ctx, intf); err != nil {
				return fmt.Errorf("failed to update member interface %q status: %w", intf.Name, err)
			}
		}

		if intf.Labels != nil && intf.Labels[v1alpha1.AggregateLabel] == s.Interface.Name {
			delete(intf.Labels, v1alpha1.AggregateLabel)
			if err := r.Update(ctx, intf); err != nil {
				return fmt.Errorf("failed to update member interface %q labels: %w", intf.Name, err)
			}
		}
	}

	return nil
}

// finalizeVLAN removes the routed VLAN interface reference from the VLAN.
func (r *InterfaceReconciler) finalizeVLAN(ctx context.Context, s *scope) error {
	if s.Interface.Spec.VlanRef == nil {
		return nil
	}

	vlan := new(v1alpha1.VLAN)
	if err := r.Get(ctx, client.ObjectKey{Name: s.Interface.Spec.VlanRef.Name, Namespace: s.Interface.Namespace}, vlan); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if vlan.Status.RoutedBy != nil && vlan.Status.RoutedBy.Name == s.Interface.Name {
		vlan.Status.RoutedBy = nil
		if err := r.Status().Update(ctx, vlan); err != nil {
			return fmt.Errorf("failed to update VLAN %q status: %w", vlan.Name, err)
		}
	}

	if vlan.Labels != nil && vlan.Labels[v1alpha1.RoutedVLANLabel] == s.Interface.Name {
		delete(vlan.Labels, v1alpha1.RoutedVLANLabel)
		if err := r.Update(ctx, vlan); err != nil {
			return fmt.Errorf("failed to update VLAN %q labels: %w", vlan.Name, err)
		}
	}

	return nil
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
			log.V(2).Info("Enqueuing Interface for reconciliation", "Interface", klog.KObj(&i))
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

// interfaceToAggregate is a [handler.MapFunc] to be used to enqueue requests for reconciliation
// for a aggregate Interface to update when one of its referenced member interfaces gets updated.
func (r *InterfaceReconciler) interfaceToAggregate(ctx context.Context, obj client.Object) []ctrl.Request {
	intf, ok := obj.(*v1alpha1.Interface)
	if !ok {
		panic(fmt.Sprintf("Expected a Interface but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx, "Member", klog.KObj(intf))

	interfaces := new(v1alpha1.InterfaceList)
	if err := r.List(ctx, interfaces, client.InNamespace(intf.Namespace), client.MatchingFields{interfaceTypeKey: string(v1alpha1.InterfaceTypeAggregate)}); err != nil {
		log.Error(err, "Failed to list Interfaces")
		return nil
	}

	requests := []ctrl.Request{}
	for _, i := range interfaces.Items {
		if slices.ContainsFunc(i.Spec.Aggregation.MemberInterfaceRefs, func(member v1alpha1.LocalObjectReference) bool {
			return member.Name == intf.Name
		}) {
			log.V(2).Info("Enqueuing Aggregate Interface for reconciliation", "Aggregate", klog.KObj(&i))
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

// aggregateToMembers is a [handler.MapFunc] to be used to enqueue requests for reconciliation
// for member Physical Interfaces when their parent Aggregate Interface gets updated.
func (r *InterfaceReconciler) aggregateToMembers(ctx context.Context, obj client.Object) []ctrl.Request {
	intf, ok := obj.(*v1alpha1.Interface)
	if !ok {
		panic(fmt.Sprintf("Expected a Interface but got a %T", obj))
	}

	if intf.Spec.Type != v1alpha1.InterfaceTypeAggregate {
		return nil
	}

	log := ctrl.LoggerFrom(ctx, "Aggregate", klog.KObj(intf))

	requests := make([]ctrl.Request, 0, len(intf.Spec.Aggregation.MemberInterfaceRefs))
	for _, ref := range intf.Spec.Aggregation.MemberInterfaceRefs {
		log.V(2).Info("Enqueuing member Interface for reconciliation", "Member", ref.Name)
		requests = append(requests, ctrl.Request{
			NamespacedName: client.ObjectKey{
				Name:      ref.Name,
				Namespace: intf.Namespace,
			},
		})
	}

	return requests
}

// parentToSubinterfaces is a [handler.MapFunc] to be used to enqueue requests for reconciliation
// for Subinterfaces based on their parent Interface.
func (r *InterfaceReconciler) parentToSubinterfaces(ctx context.Context, obj client.Object) []ctrl.Request {
	intf, ok := obj.(*v1alpha1.Interface)
	if !ok {
		panic(fmt.Sprintf("Expected a Interface but got a %T", obj))
	}

	if intf.Spec.Type != v1alpha1.InterfaceTypePhysical && intf.Spec.Type != v1alpha1.InterfaceTypeAggregate {
		return nil
	}

	log := ctrl.LoggerFrom(ctx, "Interface", klog.KObj(intf))
	interfaces := new(v1alpha1.InterfaceList)

	// List all interfaces in the same namespace with a parent interface reference to the physical interface.
	if err := r.List(ctx, interfaces, client.InNamespace(intf.Namespace), client.MatchingFields{interfaceParentRefKey: intf.Name}); err != nil {
		log.Error(err, "Failed to list Interfaces")
		return nil
	}

	requests := []ctrl.Request{}
	for _, i := range interfaces.Items {
		if i.Spec.ParentInterfaceRef != nil && i.Spec.ParentInterfaceRef.Name == intf.Name {
			log.V(2).Info("Enqueuing SubInterface for reconciliation", "SubInterface", klog.KObj(&i))
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

// vlanToRoutedVLAN is a [handler.MapFunc] to be used to enqueue requests for reconciliation
// for a RoutedVLAN Interface when its referenced VLAN changes.
func (r *InterfaceReconciler) vlanToRoutedVLAN(ctx context.Context, obj client.Object) []ctrl.Request {
	vlan, ok := obj.(*v1alpha1.VLAN)
	if !ok {
		panic(fmt.Sprintf("Expected a VLAN but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx, "VLAN", klog.KObj(vlan))

	interfaces := new(v1alpha1.InterfaceList)
	if err := r.List(ctx, interfaces, client.InNamespace(vlan.Namespace), client.MatchingFields{interfaceVlanRefKey: vlan.Name}); err != nil {
		log.Error(err, "Failed to list Interfaces")
		return nil
	}

	requests := []ctrl.Request{}
	for _, i := range interfaces.Items {
		if i.Spec.VlanRef != nil && i.Spec.VlanRef.Name == vlan.Name {
			log.V(2).Info("Enqueuing RoutedVLAN Interface for reconciliation", "Interface", klog.KObj(&i))

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

// vrfToInterface is a [handler.MapFunc] to be used to enqueue requests for reconciliation
// for Interfaces when their referenced VRF changes.
func (r *InterfaceReconciler) vrfToInterface(ctx context.Context, obj client.Object) []ctrl.Request {
	vrf, ok := obj.(*v1alpha1.VRF)
	if !ok {
		panic(fmt.Sprintf("Expected a VRF but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx, "VRF", klog.KObj(vrf))

	interfaces := new(v1alpha1.InterfaceList)
	if err := r.List(ctx, interfaces, client.InNamespace(vrf.Namespace), client.MatchingFields{interfaceVrfRefKey: vrf.Name}); err != nil {
		log.Error(err, "Failed to list Interfaces")
		return nil
	}

	requests := []ctrl.Request{}
	for _, i := range interfaces.Items {
		if i.Spec.VrfRef != nil && i.Spec.VrfRef.Name == vrf.Name {
			log.V(2).Info("Enqueuing Interface for reconciliation", "Interface", klog.KObj(&i))

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

// interfacesForProviderConfig is a [handler.MapFunc] to be used to enqueue requests for reconciliation
// for an Interface to update when one of its referenced provider configurations gets updated.
func (r *InterfaceReconciler) interfacesForProviderConfig(ctx context.Context, obj client.Object) []reconcile.Request {
	log := ctrl.LoggerFrom(ctx, "Object", klog.KObj(obj))

	list := &v1alpha1.InterfaceList{}
	if err := r.List(ctx, list, client.InNamespace(obj.GetNamespace())); err != nil {
		log.Error(err, "Failed to list Interfacees")
		return nil
	}

	gkv := obj.GetObjectKind().GroupVersionKind()

	var requests []reconcile.Request
	for _, m := range list.Items {
		if m.Spec.ProviderConfigRef != nil &&
			m.Spec.ProviderConfigRef.Name == obj.GetName() &&
			m.Spec.ProviderConfigRef.Kind == gkv.Kind &&
			m.Spec.ProviderConfigRef.APIVersion == gkv.GroupVersion().Identifier() {
			log.V(2).Info("Enqueuing Interface for reconciliation", "Interface", klog.KObj(&m))
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

// deviceToInterfaces is a [handler.MapFunc] to be used to enqueue requests for reconciliation
// for Interfaces when their referenced Device's effective pause state changes.
func (r *InterfaceReconciler) deviceToInterfaces(ctx context.Context, obj client.Object) []ctrl.Request {
	device, ok := obj.(*v1alpha1.Device)
	if !ok {
		panic(fmt.Sprintf("Expected a Device but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx, "Device", klog.KObj(device))

	interfaces := new(v1alpha1.InterfaceList)
	if err := r.List(
		ctx, interfaces,
		client.InNamespace(device.Namespace),
		client.MatchingFields{v1alpha1.DeviceRefIndexKey: device.Name},
	); err != nil {
		log.Error(err, "Failed to list Interfaces")
		return nil
	}

	requests := make([]ctrl.Request, 0, len(interfaces.Items))
	for _, i := range interfaces.Items {
		log.V(2).Info("Enqueuing Interface for reconciliation", "Interface", klog.KObj(&i))
		requests = append(requests, ctrl.Request{
			NamespacedName: client.ObjectKey{
				Name:      i.Name,
				Namespace: i.Namespace,
			},
		})
	}

	return requests
}
