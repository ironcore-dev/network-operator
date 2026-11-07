// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package evpn

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
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
	evpnv1alpha1 "github.com/ironcore-dev/network-operator/api/evpn/v1alpha1"
	overlayv1alpha1 "github.com/ironcore-dev/network-operator/api/overlay/v1alpha1"
	poolv1alpha1 "github.com/ironcore-dev/network-operator/api/pool/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/conditions"
)

// NetworkAttachmentReconciler reconciles a NetworkAttachment object.
type NetworkAttachmentReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// WatchFilterValue is the label value used to filter events prior to reconciliation.
	WatchFilterValue string

	// Recorder is used to record events for the controller.
	// More info: https://book.kubebuilder.io/reference/raising-events
	Recorder events.EventRecorder
}

// +kubebuilder:rbac:groups=overlay.networking.metal.ironcore.dev,resources=networkattachments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=overlay.networking.metal.ironcore.dev,resources=networkattachments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=overlay.networking.metal.ironcore.dev,resources=networkattachments/finalizers,verbs=update
// +kubebuilder:rbac:groups=overlay.networking.metal.ironcore.dev,resources=networks,verbs=get;list;watch
// +kubebuilder:rbac:groups=evpn.networking.metal.ironcore.dev,resources=fabrics,verbs=get;list;watch
// +kubebuilder:rbac:groups=pool.networking.metal.ironcore.dev,resources=claims,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=devices,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=interfaces,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=vlans,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=evpninstances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.24.1/pkg/reconcile
func (r *NetworkAttachmentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling resource")

	na := new(overlayv1alpha1.NetworkAttachment)
	if err := r.Get(ctx, req.NamespacedName, na); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get resource")
		return ctrl.Result{}, err
	}

	if !na.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(na, evpnv1alpha1.FinalizerName) {
			if err := r.finalize(ctx, na); err != nil {
				log.Error(err, "Failed to finalize resource")
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(na, evpnv1alpha1.FinalizerName)
			if err := r.Update(ctx, na); err != nil {
				log.Error(err, "Failed to remove finalizer from resource")
				return ctrl.Result{}, err
			}
		}
		log.Info("Resource is being deleted, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers
	if !controllerutil.ContainsFinalizer(na, evpnv1alpha1.FinalizerName) {
		controllerutil.AddFinalizer(na, evpnv1alpha1.FinalizerName)
		if err := r.Update(ctx, na); err != nil {
			log.Error(err, "Failed to add finalizer to resource")
			return ctrl.Result{}, err
		}
		log.Info("Added finalizer to resource")
		return ctrl.Result{}, nil
	}

	orig := na.DeepCopy()
	if conditions.InitializeConditions(na, v1alpha1.ReadyCondition) {
		log.V(1).Info("Initializing status conditions")
		return ctrl.Result{}, r.Status().Update(ctx, na)
	}

	defer func() {
		if !equality.Semantic.DeepEqual(orig.Status, na.Status) {
			// Pass obj.DeepCopy() to avoid Patch() modifying obj and interfering with metadata update below
			if err := r.Status().Patch(ctx, na.DeepCopy(), client.MergeFrom(orig)); err != nil {
				log.Error(err, "Failed to update status")
				reterr = kerrors.NewAggregate([]error{reterr, err})
			}
		}
		if !equality.Semantic.DeepEqual(orig.ObjectMeta, na.ObjectMeta) {
			if err := r.Patch(ctx, na, client.MergeFrom(orig)); err != nil {
				log.Error(err, "Failed to update resource metadata")
				reterr = kerrors.NewAggregate([]error{reterr, err})
			}
		}
	}()

	if err := r.reconcile(ctx, na); err != nil {
		log.Error(err, "Failed to reconcile resource")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

const (
	networkAttachmentEncapsulationKey = ".spec.encapsulation"
	networkAttachmentNetworkRefKey    = ".spec.networkRef.name"
	networkAttachmentInterfaceRefKey  = ".status.interfaces.interfaceRef.name"
	controllerOwnerUIDKey             = ".metadata.controller.uid"
)

// SetupWithManager sets up the controller with the Manager.
func (r *NetworkAttachmentReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	labelSelector := metav1.LabelSelector{}
	if r.WatchFilterValue != "" {
		labelSelector.MatchLabels = map[string]string{v1alpha1.WatchLabel: r.WatchFilterValue}
	}

	filter, err := predicate.LabelSelectorPredicate(labelSelector)
	if err != nil {
		return fmt.Errorf("create label selector predicate: %w", err)
	}

	if err := mgr.GetFieldIndexer().IndexField(ctx, &overlayv1alpha1.NetworkAttachment{}, networkAttachmentEncapsulationKey, func(obj client.Object) []string {
		na := obj.(*overlayv1alpha1.NetworkAttachment)
		return []string{na.Spec.Encapsulation.Key()}
	}); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(ctx, &overlayv1alpha1.NetworkAttachment{}, networkAttachmentNetworkRefKey, func(obj client.Object) []string {
		na := obj.(*overlayv1alpha1.NetworkAttachment)
		return []string{na.Spec.NetworkRef.Name}
	}); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(ctx, &overlayv1alpha1.NetworkAttachment{}, networkAttachmentInterfaceRefKey, func(obj client.Object) []string {
		na := obj.(*overlayv1alpha1.NetworkAttachment)
		names := make([]string, 0, len(na.Status.Interfaces))
		for _, status := range na.Status.Interfaces {
			names = append(names, status.InterfaceRef.Name)
		}
		return names
	}); err != nil {
		return err
	}

	for _, obj := range []client.Object{&v1alpha1.VLAN{}, &v1alpha1.EVPNInstance{}} {
		if err := mgr.GetFieldIndexer().IndexField(ctx, obj, controllerOwnerUIDKey, func(obj client.Object) []string {
			owner := metav1.GetControllerOf(obj)
			if owner == nil {
				return nil
			}
			return []string{string(owner.UID)}
		}); err != nil {
			return err
		}
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&overlayv1alpha1.NetworkAttachment{}).
		Named("evpn-networkattachment").
		WithEventFilter(filter).
		Owns(&v1alpha1.VLAN{}).
		Owns(&v1alpha1.EVPNInstance{}).
		// Watches enqueues NetworkAttachments that request the same encapsulation.
		Watches(
			&overlayv1alpha1.NetworkAttachment{},
			handler.EnqueueRequestsFromMapFunc(r.networkAttachmentToNetworkAttachments),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldNA := e.ObjectOld.(*overlayv1alpha1.NetworkAttachment)
					newNA := e.ObjectNew.(*overlayv1alpha1.NetworkAttachment)
					return !equality.Semantic.DeepEqual(oldNA.Spec.InterfaceSelector, newNA.Spec.InterfaceSelector)
				},
				GenericFunc: func(event.GenericEvent) bool {
					return false
				},
			}),
		).
		// Watches enqueues NetworkAttachments when their referenced Network is created, starts deletion, or is deleted.
		Watches(
			&overlayv1alpha1.Network{},
			handler.EnqueueRequestsFromMapFunc(r.networkToNetworkAttachments),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					return !equality.Semantic.DeepEqual(e.ObjectOld.GetDeletionTimestamp(), e.ObjectNew.GetDeletionTimestamp())
				},
				GenericFunc: func(event.GenericEvent) bool {
					return false
				},
			}),
		).
		// Watches enqueues NetworkAttachments when their shared VNI Claim changes.
		Watches(
			&poolv1alpha1.Claim{},
			handler.EnqueueRequestsFromMapFunc(r.claimToNetworkAttachments),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldClaim := e.ObjectOld.(*poolv1alpha1.Claim)
					newClaim := e.ObjectNew.(*poolv1alpha1.Claim)
					return !equality.Semantic.DeepEqual(oldClaim.Status, newClaim.Status)
				},
				GenericFunc: func(event.GenericEvent) bool {
					return false
				},
			}),
		).
		// Watches enqueues NetworkAttachments when matching Interfaces change.
		Watches(
			&v1alpha1.Interface{},
			handler.EnqueueRequestsFromMapFunc(r.interfaceToNetworkAttachments),
			builder.WithPredicates(predicate.Or(
				predicate.LabelChangedPredicate{},
				predicate.Funcs{
					UpdateFunc: func(e event.UpdateEvent) bool {
						oldInterface := e.ObjectOld.(*v1alpha1.Interface)
						newInterface := e.ObjectNew.(*v1alpha1.Interface)
						return oldInterface.Spec.Type != newInterface.Spec.Type ||
							!equality.Semantic.DeepEqual(oldInterface.Spec.Switchport, newInterface.Spec.Switchport)
					},
					GenericFunc: func(event.GenericEvent) bool {
						return false
					},
				},
			)),
		).
		// Watches enqueues NetworkAttachments when EVPN instances on selected Devices are created or deleted.
		Watches(
			&v1alpha1.EVPNInstance{},
			handler.EnqueueRequestsFromMapFunc(r.evpnInstanceToNetworkAttachments),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(event.UpdateEvent) bool {
					return false
				},
				GenericFunc: func(event.GenericEvent) bool {
					return false
				},
			}),
		).
		// Watches enqueues NetworkAttachments when Device labels change.
		Watches(
			&v1alpha1.Device{},
			handler.EnqueueRequestsFromMapFunc(r.deviceToNetworkAttachments),
			builder.WithPredicates(predicate.LabelChangedPredicate{}),
		).
		// Watches enqueues NetworkAttachments when Fabric device selectors or iBGP configuration change.
		Watches(
			&evpnv1alpha1.Fabric{},
			handler.EnqueueRequestsFromMapFunc(r.fabricToNetworkAttachments),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldFabric := e.ObjectOld.(*evpnv1alpha1.Fabric)
					newFabric := e.ObjectNew.(*evpnv1alpha1.Fabric)
					return !equality.Semantic.DeepEqual(oldFabric.Spec.DeviceSelector, newFabric.Spec.DeviceSelector) ||
						!equality.Semantic.DeepEqual(oldFabric.Spec.Overlay.IBGP, newFabric.Spec.Overlay.IBGP)
				},
				GenericFunc: func(event.GenericEvent) bool {
					return false
				},
			}),
		).
		Complete(r)
}

func (r *NetworkAttachmentReconciler) reconcile(ctx context.Context, na *overlayv1alpha1.NetworkAttachment) error {
	if na.Spec.Encapsulation.Type != overlayv1alpha1.EncapsulationTypeVLAN {
		conditions.Set(na, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.NotImplementedReason,
			Message: fmt.Sprintf("Encapsulation type %q is not supported", na.Spec.Encapsulation.Type),
		})
		return nil
	}

	network := &overlayv1alpha1.Network{}
	err := r.Get(ctx, client.ObjectKey{Namespace: na.Namespace, Name: na.Spec.NetworkRef.Name}, network)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("get network %q: %w", na.Spec.NetworkRef.Name, err)
	}
	if apierrors.IsNotFound(err) || !network.DeletionTimestamp.IsZero() {
		if err := r.reconcileInterfaceMembership(ctx, na, nil); err != nil {
			return err
		}
		if err := r.deleteStaleResources(ctx, na, nil); err != nil {
			return err
		}
		conditions.Set(na, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  evpnv1alpha1.NetworkNotFoundReason,
			Message: fmt.Sprintf("Network %q is missing or being deleted", na.Spec.NetworkRef.Name),
		})
		return nil
	}

	interfaces, err := r.resolveInterfaces(ctx, na)
	if err != nil {
		return err
	}
	if len(interfaces.Items) == 0 {
		if err := r.reconcileInterfaceMembership(ctx, na, nil); err != nil {
			return err
		}
		if err := r.deleteStaleResources(ctx, na, nil); err != nil {
			return err
		}
		conditions.Set(na, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  evpnv1alpha1.NoMatchingInterfacesReason,
			Message: "Interface selector matches no interfaces",
		})
		return nil
	}

	devices, err := r.resolveDevices(ctx, na, interfaces)
	if err != nil {
		return err
	}

	fabric, err := r.resolveFabric(ctx, na, devices)
	if err != nil {
		return err
	}

	forwardingContextRef := v1alpha1.TypedLocalObjectRefFromObject(fabric)
	if !equality.Semantic.DeepEqual(na.Status.ForwardingContextRef, forwardingContextRef) {
		// Persist the forwarding context reference before mutation; the status update retriggers reconciliation.
		na.Status.ForwardingContextRef = forwardingContextRef
		return nil
	}

	claim, err := r.resolveVNIClaim(ctx, na, network, fabric)
	if err != nil {
		return err
	}

	vni, routeTarget, err := r.computeVNIRouteTarget(na, fabric, claim)
	if err != nil {
		return err
	}

	if err := r.validateEncapsulationConflicts(ctx, na, devices, vni); err != nil {
		return err
	}

	if err := r.validatePeerConflicts(ctx, na, devices); err != nil {
		return err
	}

	desiredInterfaces := make(map[string]overlayv1alpha1.NetworkAttachmentInterfaceStatus, len(interfaces.Items))
	for i := range interfaces.Items {
		intf := &interfaces.Items[i]
		desiredInterfaces[intf.Name] = overlayv1alpha1.NetworkAttachmentInterfaceStatus{
			DeviceRef:    v1alpha1.LocalObjectReference{Name: intf.Spec.DeviceRef.Name},
			InterfaceRef: v1alpha1.LocalObjectReference{Name: intf.Name},
		}
	}

	if r.persistInterfaceOwnership(na, desiredInterfaces) {
		// Persist ownership before mutation; the status update retriggers reconciliation.
		return nil
	}

	if err := r.reconcileEncapsulation(ctx, na, devices, fabric, vni, routeTarget); err != nil {
		return err
	}

	if err := r.reconcileInterfaceMembership(ctx, na, desiredInterfaces); err != nil {
		return err
	}

	if err := r.deleteStaleResources(ctx, na, devices); err != nil {
		return err
	}

	conditions.Set(na, metav1.Condition{
		Type:    v1alpha1.ReadyCondition,
		Status:  metav1.ConditionTrue,
		Reason:  v1alpha1.ReadyReason,
		Message: "EVPN attachment realized",
	})

	return nil
}

func (r *NetworkAttachmentReconciler) resolveInterfaces(ctx context.Context, na *overlayv1alpha1.NetworkAttachment) (*v1alpha1.InterfaceList, error) {
	selector, err := metav1.LabelSelectorAsSelector(&na.Spec.InterfaceSelector)
	if err != nil {
		return nil, reconcile.TerminalError(fmt.Errorf("parse interface selector: %w", err))
	}

	interfaces := &v1alpha1.InterfaceList{}
	if err := r.List(
		ctx,
		interfaces,
		client.InNamespace(na.Namespace),
		client.MatchingLabelsSelector{Selector: selector},
	); err != nil {
		return nil, fmt.Errorf("list selected interfaces: %w", err)
	}

	var errs []error
	for i := range interfaces.Items {
		intf := &interfaces.Items[i]
		switch {
		case intf.Spec.Type != v1alpha1.InterfaceTypePhysical && intf.Spec.Type != v1alpha1.InterfaceTypeAggregate:
			errs = append(errs, fmt.Errorf("interface %q has type %q, want %q or %q", intf.Name, intf.Spec.Type, v1alpha1.InterfaceTypePhysical, v1alpha1.InterfaceTypeAggregate))
		case intf.Spec.Switchport == nil:
			errs = append(errs, fmt.Errorf("interface %q has no switchport configuration", intf.Name))
		case intf.Spec.Switchport.Mode != v1alpha1.SwitchportModeTrunk:
			errs = append(errs, fmt.Errorf("interface %q has switchport mode %q, want %q", intf.Name, intf.Spec.Switchport.Mode, v1alpha1.SwitchportModeTrunk))
		case intf.Spec.Switchport.AllowedVlansMode == v1alpha1.AllowedVlansModeUnmanaged:
			errs = append(errs, fmt.Errorf("interface %q has allowed VLAN mode %q, want %q", intf.Name, intf.Spec.Switchport.AllowedVlansMode, v1alpha1.AllowedVlansModeExact))
		}
	}
	if len(errs) > 0 {
		conditions.Set(na, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  evpnv1alpha1.InvalidInterfaceReason,
			Message: kerrors.NewAggregate(errs).Error(),
		})
		return nil, reconcile.TerminalError(kerrors.NewAggregate(errs))
	}

	return interfaces, nil
}

func (r *NetworkAttachmentReconciler) resolveDevices(ctx context.Context, na *overlayv1alpha1.NetworkAttachment, interfaces *v1alpha1.InterfaceList) (map[string]*v1alpha1.Device, error) {
	devices := make(map[string]*v1alpha1.Device)

	var errs []error
	for i := range interfaces.Items {
		name := interfaces.Items[i].Spec.DeviceRef.Name
		if _, ok := devices[name]; ok {
			continue
		}
		device := &v1alpha1.Device{}
		if err := r.Get(ctx, client.ObjectKey{Namespace: na.Namespace, Name: name}, device); err != nil {
			if apierrors.IsNotFound(err) {
				errs = append(errs, fmt.Errorf("device %q referenced by interface %q not found", name, interfaces.Items[i].Name))
				continue
			}
			return nil, fmt.Errorf("get device %q: %w", name, err)
		}
		devices[name] = device
	}
	if len(errs) > 0 {
		conditions.Set(na, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  evpnv1alpha1.DeviceResolutionFailedReason,
			Message: kerrors.NewAggregate(errs).Error(),
		})
		return nil, reconcile.TerminalError(kerrors.NewAggregate(errs))
	}

	return devices, nil
}

func (r *NetworkAttachmentReconciler) resolveFabric(ctx context.Context, na *overlayv1alpha1.NetworkAttachment, devices map[string]*v1alpha1.Device) (*evpnv1alpha1.Fabric, error) {
	var fabric *evpnv1alpha1.Fabric

	fabrics := &evpnv1alpha1.FabricList{}
	if err := r.List(ctx, fabrics, client.InNamespace(na.Namespace)); err != nil {
		return nil, fmt.Errorf("list fabrics: %w", err)
	}

	var errs []error
	for _, device := range devices {
		var matchingFabrics []*evpnv1alpha1.Fabric
		for i := range fabrics.Items {
			fabricSelector, err := metav1.LabelSelectorAsSelector(&fabrics.Items[i].Spec.DeviceSelector)
			if err != nil {
				return nil, fmt.Errorf("parse device selector for fabric %q: %w", fabrics.Items[i].Name, err)
			}
			if fabricSelector.Matches(labels.Set(device.Labels)) {
				matchingFabrics = append(matchingFabrics, &fabrics.Items[i])
			}
		}
		switch len(matchingFabrics) {
		case 0:
			errs = append(errs, fmt.Errorf("device %q does not match any fabric", device.Name))
		case 1:
			matchedFabric := matchingFabrics[0]
			if fabric == nil {
				fabric = matchedFabric
				continue
			}
			if fabric.Name != matchedFabric.Name {
				errs = append(errs, fmt.Errorf("device %q resolves to fabric %q, want %q", device.Name, matchedFabric.Name, fabric.Name))
			}
		default:
			errs = append(errs, fmt.Errorf("device %q matches %d fabrics, want exactly one", device.Name, len(matchingFabrics)))
		}
	}
	if len(errs) > 0 {
		conditions.Set(na, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  evpnv1alpha1.FabricResolutionFailedReason,
			Message: kerrors.NewAggregate(errs).Error(),
		})
		return nil, reconcile.TerminalError(kerrors.NewAggregate(errs))
	}

	if fabric.Spec.Overlay.IBGP == nil {
		conditions.Set(na, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  evpnv1alpha1.UnsupportedASNumberReason,
			Message: fmt.Sprintf("Fabric %q has no iBGP configuration", fabric.Name),
		})
		return nil, reconcile.TerminalError(fmt.Errorf("fabric %q has no iBGP configuration", fabric.Name))
	}

	return fabric, nil
}

func (r *NetworkAttachmentReconciler) resolveVNIClaim(ctx context.Context, na *overlayv1alpha1.NetworkAttachment, network *overlayv1alpha1.Network, fabric *evpnv1alpha1.Fabric) (*poolv1alpha1.Claim, error) {
	claim := &poolv1alpha1.Claim{
		Name:      "vni-" + hash(network),
		Namespace: na.Namespace,
	}
	if err := r.Get(ctx, client.ObjectKeyFromObject(claim), claim); err != nil {
		if apierrors.IsNotFound(err) {
			conditions.Set(na, metav1.Condition{
				Type:    v1alpha1.ReadyCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.WaitingForDependenciesReason,
				Message: fmt.Sprintf("Waiting for VNI claim %q to be created", claim.Name),
			})
			return nil, reconcile.TerminalError(fmt.Errorf("VNI claim %q not found", claim.Name))
		}
		return nil, fmt.Errorf("get VNI claim %q: %w", claim.Name, err)
	}

	if owner := metav1.GetControllerOf(claim); owner == nil ||
		owner.APIVersion != evpnv1alpha1.GroupVersion.String() || owner.Kind != "Fabric" || owner.UID != fabric.UID {
		conditions.Set(na, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  evpnv1alpha1.StretchedNetworkNotSupportedReason,
			Message: fmt.Sprintf("VNI claim %q is not bound to fabric %q", claim.Name, fabric.Name),
		})
		return nil, reconcile.TerminalError(fmt.Errorf("VNI claim %q is not bound to fabric %q", claim.Name, fabric.Name))
	}

	allocated := conditions.Get(claim, poolv1alpha1.AllocatedCondition)
	if allocated == nil || allocated.Status != metav1.ConditionTrue || claim.Status.Value == "" {
		conditions.Set(na, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.WaitingForDependenciesReason,
			Message: fmt.Sprintf("Waiting for VNI claim %q to be allocated", claim.Name),
		})
		return nil, reconcile.TerminalError(fmt.Errorf("VNI claim %q is not allocated", claim.Name))
	}

	return claim, nil
}

func (r *NetworkAttachmentReconciler) computeVNIRouteTarget(na *overlayv1alpha1.NetworkAttachment, fabric *evpnv1alpha1.Fabric, claim *poolv1alpha1.Claim) (vni uint64, rt string, err error) {
	asNumber := fabric.Spec.Overlay.IBGP.ASNumber.String()
	// RFC 8365 auto-derivation requires a 2-byte ASN; bit size 16 rejects larger and dotted ASNs.
	asn, err := strconv.ParseUint(asNumber, 10, 16)
	if err != nil || asn == 0 {
		conditions.Set(na, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  evpnv1alpha1.UnsupportedASNumberReason,
			Message: fmt.Sprintf("Fabric %q has unsupported AS number %q, want 1..65535", fabric.Name, asNumber),
		})
		return 0, "", reconcile.TerminalError(fmt.Errorf("fabric %q has unsupported AS number %q, want 1..65535", fabric.Name, asNumber))
	}

	vni, err = strconv.ParseUint(claim.Status.Value, 10, 24)
	if err != nil || vni == 0 || vni > 16777214 {
		conditions.Set(na, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  evpnv1alpha1.InvalidVNIReason,
			Message: fmt.Sprintf("Claim %q has invalid VNI %q", claim.Name, claim.Status.Value),
		})
		return 0, "", reconcile.TerminalError(fmt.Errorf("claim %q has invalid VNI %q", claim.Name, claim.Status.Value))
	}

	// RFC 8365 divides the 32-bit local administrator as follows:
	//
	//	bit 31       bits 30..28  bits 27..24  bits 23..0
	//	+------------+------------+------------+----------------+
	//	| A          | Type       | Domain ID  | Service ID     |
	//	| 1 bit      | 3 bits     | 4 bits     | 24 bits        |
	//	+------------+------------+------------+----------------+
	//
	// A=0 means auto-derived, Type=1 means VXLAN, Domain ID=0, and Service ID is the VNI.
	localAdministrator := uint32(1<<28) | uint32(vni)

	// Represent the RFC 8365 auto-derived route target in ASN:local-administrator notation.
	rt = fmt.Sprintf("%d:%d", asn, localAdministrator)

	return vni, rt, nil
}

func (r *NetworkAttachmentReconciler) validatePeerConflicts(ctx context.Context, na *overlayv1alpha1.NetworkAttachment, devices map[string]*v1alpha1.Device) error {
	peers := &overlayv1alpha1.NetworkAttachmentList{}
	if err := r.List(
		ctx,
		peers,
		client.InNamespace(na.Namespace),
		client.MatchingFields{networkAttachmentEncapsulationKey: na.Spec.Encapsulation.Key()},
	); err != nil {
		return fmt.Errorf("list network attachments with encapsulation %q: %w", na.Spec.Encapsulation.Key(), err)
	}

	var errs []error
	for i := range peers.Items {
		peer := &peers.Items[i]
		if peer.Name == na.Name {
			continue
		}

		peerSelector, err := metav1.LabelSelectorAsSelector(&peer.Spec.InterfaceSelector)
		if err != nil {
			return fmt.Errorf("parse interface selector for network attachment %q: %w", peer.Name, err)
		}

		peerInterfaces := &v1alpha1.InterfaceList{}
		if err := r.List(
			ctx,
			peerInterfaces,
			client.InNamespace(peer.Namespace),
			client.MatchingLabelsSelector{Selector: peerSelector},
		); err != nil {
			return fmt.Errorf("list interfaces for network attachment %q: %w", peer.Name, err)
		}

		for j := range peerInterfaces.Items {
			deviceName := peerInterfaces.Items[j].Spec.DeviceRef.Name
			if _, ok := devices[deviceName]; !ok {
				continue
			}
			errs = append(errs, fmt.Errorf("network attachment %q also requests encapsulation %q on device %q", peer.Name, na.Spec.Encapsulation.Key(), deviceName))
			break
		}
	}
	if len(errs) > 0 {
		conditions.Set(na, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  evpnv1alpha1.EncapsulationConflictReason,
			Message: kerrors.NewAggregate(errs).Error(),
		})
		return reconcile.TerminalError(kerrors.NewAggregate(errs))
	}

	return nil
}

func (r *NetworkAttachmentReconciler) validateEncapsulationConflicts(ctx context.Context, na *overlayv1alpha1.NetworkAttachment, devices map[string]*v1alpha1.Device, vni uint64) error {
	var errs []error
	for _, device := range devices {
		vlans := &v1alpha1.VLANList{}
		if err := r.List(ctx, vlans, client.InNamespace(na.Namespace), client.MatchingFields{v1alpha1.DeviceRefIndexKey: device.Name}); err != nil {
			return fmt.Errorf("list VLANs on device %q: %w", device.Name, err)
		}
		targetVLANs := sets.New[string]()
		for i := range vlans.Items {
			vlan := &vlans.Items[i]
			if int32(vlan.Spec.ID) != na.Spec.Encapsulation.ID {
				continue
			}
			targetVLANs.Insert(vlan.Name)
			if !metav1.IsControlledBy(vlan, na) {
				errs = append(errs, fmt.Errorf("VLAN %q already provides VLAN %d on device %q", vlan.Name, na.Spec.Encapsulation.ID, device.Name))
			}
		}
		evis := &v1alpha1.EVPNInstanceList{}
		if err := r.List(ctx, evis, client.InNamespace(na.Namespace), client.MatchingFields{v1alpha1.DeviceRefIndexKey: device.Name}); err != nil {
			return fmt.Errorf("list EVPN instances on device %q: %w", device.Name, err)
		}
		for i := range evis.Items {
			evi := &evis.Items[i]
			if metav1.IsControlledBy(evi, na) {
				continue
			}
			if evi.Spec.VNI == int32(vni) { // #nosec G115 -- VNI validation bounds the value to int32.
				errs = append(errs, fmt.Errorf("EVPN instance %q already provides VNI %d on device %q", evi.Name, vni, device.Name))
				continue
			}
			if evi.Spec.VLANRef != nil && targetVLANs.Has(evi.Spec.VLANRef.Name) {
				errs = append(errs, fmt.Errorf("EVPN instance %q already references VLAN %q on device %q", evi.Name, evi.Spec.VLANRef.Name, device.Name))
			}
		}
	}
	if len(errs) > 0 {
		conditions.Set(na, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  evpnv1alpha1.ResourceConflictReason,
			Message: kerrors.NewAggregate(errs).Error(),
		})
		return reconcile.TerminalError(kerrors.NewAggregate(errs))
	}
	return nil
}

func (r *NetworkAttachmentReconciler) persistInterfaceOwnership(na *overlayv1alpha1.NetworkAttachment, desiredInterfaces map[string]overlayv1alpha1.NetworkAttachmentInterfaceStatus) (changed bool) {
	ownedInterfaces := make(map[string]overlayv1alpha1.NetworkAttachmentInterfaceStatus, len(na.Status.Interfaces))
	for _, status := range na.Status.Interfaces {
		ownedInterfaces[status.InterfaceRef.Name] = status
	}

	ownershipChanged := false
	for name, desired := range desiredInterfaces {
		if existing, ok := ownedInterfaces[name]; !ok || existing != desired {
			ownershipChanged = true
			break
		}
	}
	if ownershipChanged {
		maps.Copy(ownedInterfaces, desiredInterfaces)
		na.Status.Interfaces = slices.Collect(maps.Values(ownedInterfaces))
		slices.SortFunc(na.Status.Interfaces, func(a, b overlayv1alpha1.NetworkAttachmentInterfaceStatus) int {
			return cmp.Compare(a.InterfaceRef.Name, b.InterfaceRef.Name)
		})
		return true
	}

	return false
}

func (r *NetworkAttachmentReconciler) reconcileEncapsulation(ctx context.Context, na *overlayv1alpha1.NetworkAttachment, devices map[string]*v1alpha1.Device, fabric *evpnv1alpha1.Fabric, vni uint64, routeTarget string) error {
	for _, device := range devices {
		prefix := "attachment-" + hash(na, device)
		vlan := &v1alpha1.VLAN{Name: prefix + "-vlan", Namespace: na.Namespace}
		if _, err := controllerutil.CreateOrPatch(ctx, r.Client, vlan, func() error {
			if vlan.Labels == nil {
				vlan.Labels = make(map[string]string)
			}
			vlan.Labels[evpnv1alpha1.NetworkLabel] = na.Spec.NetworkRef.Name
			vlan.Labels[evpnv1alpha1.NetworkAttachmentLabel] = na.Name
			vlan.Labels[evpnv1alpha1.FabricLabel] = fabric.Name
			vlan.Spec.DeviceRef = v1alpha1.LocalObjectReference{Name: device.Name}
			vlan.Spec.ID = int16(na.Spec.Encapsulation.ID) // #nosec G115 -- the API limits VLAN IDs to 1..4094.
			vlan.Spec.AdminState = v1alpha1.AdminStateUp
			return controllerutil.SetControllerReference(na, vlan, r.Scheme)
		}); err != nil {
			return fmt.Errorf("reconcile VLAN on device %q: %w", device.Name, err)
		}

		evi := &v1alpha1.EVPNInstance{Name: prefix + "-evi", Namespace: na.Namespace}
		if _, err := controllerutil.CreateOrPatch(ctx, r.Client, evi, func() error {
			if evi.Labels == nil {
				evi.Labels = make(map[string]string)
			}
			evi.Labels[evpnv1alpha1.NetworkLabel] = na.Spec.NetworkRef.Name
			evi.Labels[evpnv1alpha1.NetworkAttachmentLabel] = na.Name
			evi.Labels[evpnv1alpha1.FabricLabel] = fabric.Name
			evi.Spec.DeviceRef = v1alpha1.LocalObjectReference{Name: device.Name}
			evi.Spec.VNI = int32(vni) // #nosec G115 -- VNI validation bounds the value to int32.
			evi.Spec.Type = v1alpha1.EVPNInstanceTypeBridged
			evi.Spec.RouteDistinguisher = "Auto"
			evi.Spec.RouteTargets = []v1alpha1.EVPNRouteTarget{{Value: routeTarget, Action: v1alpha1.RouteTargetActionBoth}}
			evi.Spec.VLANRef = &v1alpha1.LocalObjectReference{Name: vlan.Name}
			return controllerutil.SetControllerReference(na, evi, r.Scheme)
		}); err != nil {
			return fmt.Errorf("reconcile EVPN instance on device %q: %w", device.Name, err)
		}
	}
	return nil
}

func (r *NetworkAttachmentReconciler) reconcileInterfaceMembership(ctx context.Context, na *overlayv1alpha1.NetworkAttachment, desiredInterfaces map[string]overlayv1alpha1.NetworkAttachmentInterfaceStatus) error {
	// Remove the allowed VLAN from owned Interfaces that are no longer selected.
	for _, owned := range na.Status.Interfaces {
		if _, desired := desiredInterfaces[owned.InterfaceRef.Name]; desired {
			continue
		}

		if err := client.IgnoreNotFound(r.patchInterface(ctx, client.ObjectKey{Namespace: na.Namespace, Name: owned.InterfaceRef.Name}, func(intf *v1alpha1.Interface) (bool, error) {
			changed, err := intf.Spec.Switchport.RemoveAllowedVLAN(na.Spec.Encapsulation.ID)
			if err != nil {
				return false, fmt.Errorf("remove VLAN %d: %w", na.Spec.Encapsulation.ID, err)
			}
			return changed, nil
		})); err != nil {
			return err
		}
	}

	// Add the requested VLAN to every selected Interface.
	for _, intf := range desiredInterfaces {
		if err := r.patchInterface(ctx, client.ObjectKey{Namespace: na.Namespace, Name: intf.InterfaceRef.Name}, func(intf *v1alpha1.Interface) (bool, error) {
			changed, err := intf.Spec.Switchport.AddAllowedVLAN(na.Spec.Encapsulation.ID)
			if err != nil {
				return false, fmt.Errorf("add VLAN %d: %w", na.Spec.Encapsulation.ID, err)
			}
			return changed, nil
		}); err != nil {
			return err
		}
	}

	na.Status.Interfaces = slices.Collect(maps.Values(desiredInterfaces))
	slices.SortFunc(na.Status.Interfaces, func(a, b overlayv1alpha1.NetworkAttachmentInterfaceStatus) int {
		return cmp.Compare(a.InterfaceRef.Name, b.InterfaceRef.Name)
	})

	return nil
}

func (r *NetworkAttachmentReconciler) deleteStaleResources(ctx context.Context, na *overlayv1alpha1.NetworkAttachment, devices map[string]*v1alpha1.Device) error {
	vlanRefs := sets.New[string]()

	evis := &v1alpha1.EVPNInstanceList{}
	if err := r.List(ctx, evis, client.InNamespace(na.Namespace), client.MatchingFields{controllerOwnerUIDKey: string(na.UID)}); err != nil {
		return fmt.Errorf("list EVPN instances controlled by network attachment %q: %w", na.Name, err)
	}
	for i := range evis.Items {
		evi := &evis.Items[i]
		if !metav1.IsControlledBy(evi, na) {
			continue
		}
		if _, desired := devices[evi.Spec.DeviceRef.Name]; desired {
			continue
		}
		if evi.Spec.VLANRef != nil {
			vlanRefs.Insert(evi.Spec.VLANRef.Name)
		}
		if evi.DeletionTimestamp.IsZero() {
			if err := client.IgnoreNotFound(r.Delete(ctx, evi)); err != nil {
				return fmt.Errorf("delete EVPN instance %q: %w", evi.Name, err)
			}
		}
	}

	vlans := &v1alpha1.VLANList{}
	if err := r.List(ctx, vlans, client.InNamespace(na.Namespace), client.MatchingFields{controllerOwnerUIDKey: string(na.UID)}); err != nil {
		return fmt.Errorf("list VLANs controlled by network attachment %q: %w", na.Name, err)
	}
	for i := range vlans.Items {
		vlan := &vlans.Items[i]
		if !metav1.IsControlledBy(vlan, na) {
			continue
		}
		if _, desired := devices[vlan.Spec.DeviceRef.Name]; desired {
			continue
		}
		if vlanRefs.Has(vlan.Name) {
			continue
		}
		if vlan.DeletionTimestamp.IsZero() {
			if err := client.IgnoreNotFound(r.Delete(ctx, vlan)); err != nil {
				return fmt.Errorf("delete VLAN %q: %w", vlan.Name, err)
			}
		}
	}

	return nil
}

func (r *NetworkAttachmentReconciler) finalize(ctx context.Context, na *overlayv1alpha1.NetworkAttachment) error {
	var errs []error
	for _, owned := range na.Status.Interfaces {
		err := r.patchInterface(ctx, client.ObjectKey{Namespace: na.Namespace, Name: owned.InterfaceRef.Name}, func(intf *v1alpha1.Interface) (bool, error) {
			changed, err := intf.Spec.Switchport.RemoveAllowedVLAN(na.Spec.Encapsulation.ID)
			if err != nil {
				return false, fmt.Errorf("remove VLAN %d: %w", na.Spec.Encapsulation.ID, err)
			}
			return changed, nil
		})
		if err := client.IgnoreNotFound(err); err != nil {
			errs = append(errs, err)
		}
	}
	return kerrors.NewAggregate(errs)
}

// patchInterface retrieves the Interface identified by key, applies the mutate function to it, and patches it if it was changed.
func (r *NetworkAttachmentReconciler) patchInterface(ctx context.Context, key client.ObjectKey, mutate func(*v1alpha1.Interface) (changed bool, err error)) error {
	intf := &v1alpha1.Interface{}
	if err := r.Get(ctx, key, intf); err != nil {
		return fmt.Errorf("get interface %q: %w", key.Name, err)
	}
	orig := intf.DeepCopy()
	changed, err := mutate(intf)
	if err != nil {
		return fmt.Errorf("mutate interface %q: %w", intf.Name, err)
	}
	if changed {
		if err := r.Patch(ctx, intf, client.MergeFrom(orig)); err != nil {
			return fmt.Errorf("patch interface %q: %w", intf.Name, err)
		}
	}
	return nil
}

// networkAttachmentToNetworkAttachments is a [handler.MapFunc] that enqueues
// NetworkAttachments requesting the same encapsulation on a selected Device.
func (r *NetworkAttachmentReconciler) networkAttachmentToNetworkAttachments(ctx context.Context, obj client.Object) []ctrl.Request {
	na, ok := obj.(*overlayv1alpha1.NetworkAttachment)
	if !ok {
		panic(fmt.Sprintf("Expected a NetworkAttachment but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx, "NetworkAttachment", klog.KObj(na))

	selector, err := metav1.LabelSelectorAsSelector(&na.Spec.InterfaceSelector)
	if err != nil {
		log.Error(err, "Failed to parse interfaceSelector")
		return nil
	}

	interfaces := &v1alpha1.InterfaceList{}
	if err := r.List(
		ctx,
		interfaces,
		client.InNamespace(na.Namespace),
		client.MatchingLabelsSelector{Selector: selector},
	); err != nil {
		log.Error(err, "Failed to list Interfaces")
		return nil
	}

	devices := make(map[string]struct{})
	for i := range interfaces.Items {
		devices[interfaces.Items[i].Spec.DeviceRef.Name] = struct{}{}
	}

	list := &overlayv1alpha1.NetworkAttachmentList{}
	if err := r.List(
		ctx,
		list,
		client.InNamespace(na.Namespace),
		client.MatchingFields{networkAttachmentEncapsulationKey: na.Spec.Encapsulation.Key()},
	); err != nil {
		log.Error(err, "Failed to list NetworkAttachments")
		return nil
	}

	var requests []ctrl.Request
	for i := range list.Items {
		peer := &list.Items[i]
		if peer.Name == na.Name {
			continue
		}

		peerSelector, err := metav1.LabelSelectorAsSelector(&peer.Spec.InterfaceSelector)
		if err != nil {
			log.Error(err, "Failed to parse NetworkAttachment interfaceSelector", "NetworkAttachment", klog.KObj(peer))
			continue
		}

		peerInterfaces := &v1alpha1.InterfaceList{}
		if err := r.List(
			ctx,
			peerInterfaces,
			client.InNamespace(peer.Namespace),
			client.MatchingLabelsSelector{Selector: peerSelector},
		); err != nil {
			log.Error(err, "Failed to list Interfaces", "NetworkAttachment", klog.KObj(peer))
			continue
		}

		for j := range peerInterfaces.Items {
			if _, ok := devices[peerInterfaces.Items[j].Spec.DeviceRef.Name]; !ok {
				continue
			}
			log.V(2).Info("Enqueuing NetworkAttachment for reconciliation", "NetworkAttachment", klog.KObj(peer))
			requests = append(requests, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(peer)})
			break
		}
	}

	return requests
}

// networkToNetworkAttachments is a [handler.MapFunc] that enqueues
// NetworkAttachments referencing the changed Network.
func (r *NetworkAttachmentReconciler) networkToNetworkAttachments(ctx context.Context, obj client.Object) []ctrl.Request {
	network, ok := obj.(*overlayv1alpha1.Network)
	if !ok {
		panic(fmt.Sprintf("Expected a Network but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx, "Network", klog.KObj(network))

	list := &overlayv1alpha1.NetworkAttachmentList{}
	if err := r.List(
		ctx,
		list,
		client.InNamespace(network.Namespace),
		client.MatchingFields{networkAttachmentNetworkRefKey: network.Name},
	); err != nil {
		log.Error(err, "Failed to list NetworkAttachments")
		return nil
	}

	requests := make([]ctrl.Request, 0, len(list.Items))
	for i := range list.Items {
		log.V(2).Info("Enqueuing NetworkAttachment for reconciliation", "NetworkAttachment", klog.KObj(&list.Items[i]))
		requests = append(requests, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(&list.Items[i])})
	}

	return requests
}

// claimToNetworkAttachments is a [handler.MapFunc] that enqueues NetworkAttachments
// referencing the Network associated with a VNI Claim.
func (r *NetworkAttachmentReconciler) claimToNetworkAttachments(ctx context.Context, obj client.Object) []ctrl.Request {
	claim, ok := obj.(*poolv1alpha1.Claim)
	if !ok {
		panic(fmt.Sprintf("Expected a Claim but got a %T", obj))
	}

	networkName := claim.Labels[evpnv1alpha1.NetworkLabel]
	if networkName == "" {
		return nil
	}

	log := ctrl.LoggerFrom(ctx, "Claim", klog.KObj(claim))

	list := &overlayv1alpha1.NetworkAttachmentList{}
	if err := r.List(
		ctx,
		list,
		client.InNamespace(claim.Namespace),
		client.MatchingFields{networkAttachmentNetworkRefKey: networkName},
	); err != nil {
		log.Error(err, "Failed to list NetworkAttachments")
		return nil
	}

	requests := make([]ctrl.Request, 0, len(list.Items))
	for i := range list.Items {
		log.V(2).Info("Enqueuing NetworkAttachment for reconciliation", "NetworkAttachment", klog.KObj(&list.Items[i]))
		requests = append(requests, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(&list.Items[i])})
	}

	return requests
}

// interfaceToNetworkAttachments is a [handler.MapFunc] that enqueues NetworkAttachments
// when an Interface matches their selector or is recorded in their status.
func (r *NetworkAttachmentReconciler) interfaceToNetworkAttachments(ctx context.Context, obj client.Object) []ctrl.Request {
	intf, ok := obj.(*v1alpha1.Interface)
	if !ok {
		panic(fmt.Sprintf("Expected an Interface but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx, "Interface", klog.KObj(intf))
	list := &overlayv1alpha1.NetworkAttachmentList{}
	if err := r.List(ctx, list, client.InNamespace(intf.Namespace)); err != nil {
		log.Error(err, "Failed to list NetworkAttachments")
		return nil
	}

	requests := make([]ctrl.Request, 0, len(list.Items))
	for i := range list.Items {
		na := &list.Items[i]
		selector, err := metav1.LabelSelectorAsSelector(&na.Spec.InterfaceSelector)
		if err != nil {
			log.Error(err, "Failed to parse NetworkAttachment interfaceSelector", "NetworkAttachment", klog.KObj(na))
			continue
		}
		if selector.Matches(labels.Set(intf.Labels)) {
			log.V(2).Info("Enqueuing NetworkAttachment for reconciliation", "NetworkAttachment", klog.KObj(na))
			requests = append(requests, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(na)})
		}
	}

	owned := &overlayv1alpha1.NetworkAttachmentList{}
	if err := r.List(
		ctx,
		owned,
		client.InNamespace(intf.Namespace),
		client.MatchingFields{networkAttachmentInterfaceRefKey: intf.Name},
	); err != nil {
		log.Error(err, "Failed to list NetworkAttachments owning Interface")
		return requests
	}
	for i := range owned.Items {
		log.V(2).Info("Enqueuing NetworkAttachment for reconciliation", "NetworkAttachment", klog.KObj(&owned.Items[i]))
		requests = append(requests, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(&owned.Items[i])})
	}

	return requests
}

// evpnInstanceToNetworkAttachments is a [handler.MapFunc] that enqueues
// the NetworkAttachment owning the VLAN referenced by an EVPNInstance.
// Attachment-owned EVPNInstances already trigger reconciliation through Owns().
func (r *NetworkAttachmentReconciler) evpnInstanceToNetworkAttachments(ctx context.Context, obj client.Object) []ctrl.Request {
	evi, ok := obj.(*v1alpha1.EVPNInstance)
	if !ok {
		panic(fmt.Sprintf("Expected an EVPNInstance but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx, "EVPNInstance", klog.KObj(evi))

	// Attachment-owned EVPNInstances already trigger reconciliation through Owns().
	if owner := metav1.GetControllerOf(evi); owner != nil && owner.Kind == "NetworkAttachment" && owner.APIVersion == overlayv1alpha1.GroupVersion.String() {
		return nil
	}

	if evi.Spec.VLANRef == nil {
		return nil
	}

	vlan := &v1alpha1.VLAN{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: evi.Namespace, Name: evi.Spec.VLANRef.Name}, vlan); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err, "Failed to get referenced VLAN")
		}
		return nil
	}

	owner := metav1.GetControllerOf(vlan)
	if owner == nil || owner.Kind != "NetworkAttachment" || owner.APIVersion != overlayv1alpha1.GroupVersion.String() {
		return nil
	}

	// Another EVPNInstance references an attachment-owned VLAN.
	// Reconcile that attachment to detect the conflict or recover after deletion.
	log.V(2).Info("Enqueuing NetworkAttachment for reconciliation", "name", owner.Name, "namespace", vlan.Namespace)
	return []ctrl.Request{{Namespace: vlan.Namespace, Name: owner.Name}}
}

// deviceToNetworkAttachments is a [handler.MapFunc] that enqueues NetworkAttachments
// selecting Interfaces on the changed Device.
func (r *NetworkAttachmentReconciler) deviceToNetworkAttachments(ctx context.Context, obj client.Object) []ctrl.Request {
	device, ok := obj.(*v1alpha1.Device)
	if !ok {
		panic(fmt.Sprintf("Expected a Device but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx, "Device", klog.KObj(device))

	interfaces := &v1alpha1.InterfaceList{}
	if err := r.List(
		ctx,
		interfaces,
		client.InNamespace(device.Namespace),
		client.MatchingFields{v1alpha1.DeviceRefIndexKey: device.Name},
	); err != nil {
		log.Error(err, "Failed to list Interfaces")
		return nil
	}

	var requests []ctrl.Request
	for i := range interfaces.Items {
		requests = append(requests, r.interfaceToNetworkAttachments(ctx, &interfaces.Items[i])...)
	}

	return requests
}

// fabricToNetworkAttachments is a [handler.MapFunc] that enqueues NetworkAttachments
// selecting Interfaces on Devices matched by the changed Fabric.
func (r *NetworkAttachmentReconciler) fabricToNetworkAttachments(ctx context.Context, obj client.Object) []ctrl.Request {
	fabric, ok := obj.(*evpnv1alpha1.Fabric)
	if !ok {
		panic(fmt.Sprintf("Expected a Fabric but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx, "Fabric", klog.KObj(fabric))

	selector, err := metav1.LabelSelectorAsSelector(&fabric.Spec.DeviceSelector)
	if err != nil {
		log.Error(err, "Failed to parse Fabric deviceSelector")
		return nil
	}

	devices := &v1alpha1.DeviceList{}
	if err := r.List(
		ctx,
		devices,
		client.InNamespace(fabric.Namespace),
		client.MatchingLabelsSelector{Selector: selector},
	); err != nil {
		log.Error(err, "Failed to list Devices")
		return nil
	}

	var requests []ctrl.Request
	for i := range devices.Items {
		requests = append(requests, r.deviceToNetworkAttachments(ctx, &devices.Items[i])...)
	}

	return requests
}

// hash returns a deterministic 128-bit hash of the ordered object UIDs,
// making collisions negligible without collision-recovery state.
func hash(objects ...client.Object) string {
	if len(objects) == 0 {
		panic("hash requires at least one object")
	}
	uids := make([]string, len(objects))
	for i, obj := range objects {
		uids[i] = string(obj.GetUID())
	}
	hash := sha256.Sum256([]byte(strings.Join(uids, "/")))
	return hex.EncodeToString(hash[:16])
}
