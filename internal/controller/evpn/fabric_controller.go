// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package evpn

import (
	"cmp"
	"context"
	"fmt"
	"net/netip"
	"slices"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	evpnv1alpha1 "github.com/ironcore-dev/network-operator/api/evpn/v1alpha1"
	poolv1alpha1 "github.com/ironcore-dev/network-operator/api/pool/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/conditions"
	"github.com/ironcore-dev/network-operator/internal/provider"
)

// FabricReconciler reconciles a Fabric object
type FabricReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// WatchFilterValue is the label value used to filter events prior to reconciliation.
	WatchFilterValue string

	// Recorder is used to record events for the controller.
	// More info: https://book.kubebuilder.io/reference/raising-events
	Recorder events.EventRecorder

	// Provider is the driver that will be used to create interfaces.
	Provider provider.ProviderFunc
}

// +kubebuilder:rbac:groups=evpn.networking.metal.ironcore.dev,resources=fabrics,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=evpn.networking.metal.ironcore.dev,resources=fabrics/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=evpn.networking.metal.ironcore.dev,resources=fabrics/finalizers,verbs=update
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=devices,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=interfaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=ospf,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=isis,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=bgp,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=bgppeers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=pool.networking.metal.ironcore.dev,resources=claims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=pool.networking.metal.ironcore.dev,resources=ipaddresspools,verbs=get;list;watch
// +kubebuilder:rbac:groups=pool.networking.metal.ironcore.dev,resources=ipprefixpools,verbs=get;list;watch
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.2/pkg/reconcile
func (r *FabricReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling resource")

	fabric := new(evpnv1alpha1.Fabric)
	if err := r.Get(ctx, req.NamespacedName, fabric); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get resource")
		return ctrl.Result{}, err
	}

	if _, ok := r.Provider().(provider.InterfaceProvider); !ok {
		if meta.SetStatusCondition(&fabric.Status.Conditions, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.NotImplementedReason,
			Message: "Provider does not implement provider.InterfaceProvider",
		}) {
			return ctrl.Result{}, r.Status().Update(ctx, fabric)
		}
		return ctrl.Result{}, nil
	}

	if !fabric.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(fabric, evpnv1alpha1.FinalizerName) {
			if err := r.finalize(ctx, fabric); err != nil {
				log.Error(err, "Failed to finalize resource")
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(fabric, evpnv1alpha1.FinalizerName)
			if err := r.Update(ctx, fabric); err != nil {
				log.Error(err, "Failed to remove finalizer from resource")
				return ctrl.Result{}, err
			}
		}
		log.Info("Resource is being deleted, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers
	if !controllerutil.ContainsFinalizer(fabric, evpnv1alpha1.FinalizerName) {
		controllerutil.AddFinalizer(fabric, evpnv1alpha1.FinalizerName)
		if err := r.Update(ctx, fabric); err != nil {
			log.Error(err, "Failed to add finalizer to resource")
			return ctrl.Result{}, err
		}
		log.Info("Added finalizer to resource")
		return ctrl.Result{}, nil
	}

	orig := fabric.DeepCopy()
	if conditions.InitializeConditions(fabric, v1alpha1.ReadyCondition) {
		log.V(1).Info("Initializing status conditions")
		return ctrl.Result{}, r.Status().Update(ctx, fabric)
	}

	defer func() {
		if !equality.Semantic.DeepEqual(orig.ObjectMeta, fabric.ObjectMeta) {
			// Pass obj.DeepCopy() to avoid Patch() modifying obj and interfering with status update below
			if err := r.Patch(ctx, fabric.DeepCopy(), client.MergeFrom(orig)); err != nil {
				log.Error(err, "Failed to update resource metadata")
				reterr = kerrors.NewAggregate([]error{reterr, err})
			}
		}
		if !equality.Semantic.DeepEqual(orig.Status, fabric.Status) {
			if err := r.Status().Patch(ctx, fabric, client.MergeFrom(orig)); err != nil {
				log.Error(err, "Failed to update status")
				reterr = kerrors.NewAggregate([]error{reterr, err})
			}
		}
	}()

	res, err := r.reconcile(ctx, fabric)
	if err != nil {
		log.Error(err, "Failed to reconcile resource")
		return ctrl.Result{}, err
	}

	return res, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *FabricReconciler) SetupWithManager(mgr ctrl.Manager) error {
	labelSelector := metav1.LabelSelector{}
	if r.WatchFilterValue != "" {
		labelSelector.MatchLabels = map[string]string{v1alpha1.WatchLabel: r.WatchFilterValue}
	}

	filter, err := predicate.LabelSelectorPredicate(labelSelector)
	if err != nil {
		return fmt.Errorf("failed to create label selector predicate: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&evpnv1alpha1.Fabric{}).
		Owns(&poolv1alpha1.Claim{}).
		Owns(&v1alpha1.Interface{}).
		Owns(&v1alpha1.OSPF{}).
		Owns(&v1alpha1.ISIS{}).
		Owns(&v1alpha1.BGP{}).
		Owns(&v1alpha1.BGPPeer{}).
		// Re-reconcile when a Device's labels change so that devices newly
		// matching a deviceSelector are enrolled into the fabric.
		Watches(
			&v1alpha1.Device{},
			handler.EnqueueRequestsFromMapFunc(r.devicesToFabrics),
			builder.WithPredicates(predicate.LabelChangedPredicate{}),
		).
		// Re-reconcile when an Interface's labels change so that interfaces
		// newly matching the spec.underlay.interfaceSelector are enrolled into the fabric.
		Watches(
			&v1alpha1.Interface{},
			handler.EnqueueRequestsFromMapFunc(r.interfacesToFabrics),
			builder.WithPredicates(predicate.LabelChangedPredicate{}),
		).
		WithEventFilter(filter).
		Named("evpn-fabric").
		Complete(r)
}

// ReconcileFunc defines a function type for reconciliation phases.
// Each phase should return a non-zero Result or an error if it wants to stop the reconciliation loop.
type ReconcileFunc func(context.Context, *evpnv1alpha1.Fabric, *ReconcileState) (ctrl.Result, error)

// ReconcileState accumulates per-device interface references across reconciliation phases
// so that later phases (e.g. IGP provisioning) can consume them without redundant API calls.
type ReconcileState struct {
	loopbacks map[string][]*v1alpha1.Interface // device name → loopback Interfaces
	uplinks   map[string][]*v1alpha1.Interface // device name → underlay uplink Interfaces
}

func (r *FabricReconciler) reconcile(ctx context.Context, fabric *evpnv1alpha1.Fabric) (ctrl.Result, error) {
	state := &ReconcileState{
		loopbacks: make(map[string][]*v1alpha1.Interface),
		uplinks:   make(map[string][]*v1alpha1.Interface),
	}
	phases := []ReconcileFunc{
		r.reconcileSystemLoopbacks,
		r.reconcileVTEPLoopbacks,
		r.reconcileAnycastRPLoopbacks,
		r.reconcileUnderlayLinks,
		r.reconcileUnderlayIGP,
		r.reconcileOverlayBGP,
	}
	for _, phase := range phases {
		res, err := phase(ctx, fabric, state)
		if err != nil || !res.IsZero() {
			return res, err
		}
	}
	conditions.Set(fabric, metav1.Condition{
		Type:    v1alpha1.ReadyCondition,
		Status:  metav1.ConditionTrue,
		Reason:  v1alpha1.ReadyReason,
		Message: "Resource is ready",
	})
	return ctrl.Result{}, nil
}

func (r *FabricReconciler) finalize(ctx context.Context, fabric *evpnv1alpha1.Fabric) error {
	_ = ctx
	_ = fabric
	return nil
}

const (
	LoopbackRouterID    = 0   // Router-ID and BGP source address, present on all fabric devices
	LoopbackVTEP        = 1   // Primary VTEP address, present on VTEP devices
	LoopbackVTEPAnycast = 2   // Anycast VTEP address, present on VTEP devices (deprecated in favour of ESI)
	LoopbackAnycastRP   = 100 // PIM anycast rendezvous point address, shared across RP devices
)

// reconcileSystemLoopbacks ensures lo0 (Router-ID / BGP source) exists on every fabric device.
func (r *FabricReconciler) reconcileSystemLoopbacks(ctx context.Context, fabric *evpnv1alpha1.Fabric, state *ReconcileState) (ctrl.Result, error) {
	selector, err := metav1.LabelSelectorAsSelector(&fabric.Spec.DeviceSelector)
	if err != nil {
		return ctrl.Result{}, reconcile.TerminalError(fmt.Errorf("invalid deviceSelector: %w", err))
	}
	devices := &v1alpha1.DeviceList{}
	if err := r.List(ctx, devices, client.InNamespace(fabric.Namespace), client.MatchingLabelsSelector{Selector: selector}); err != nil {
		return ctrl.Result{}, fmt.Errorf("listing all fabric devices for system loopbacks: %w", err)
	}
	// Range by index only — list.Items are value types; using the second loop
	// variable would copy the entire struct on each iteration.
	for i := range devices.Items {
		claimName := fmt.Sprintf("%s-%s-lo%d", fabric.Name, devices.Items[i].Name, LoopbackRouterID)
		claim, err := r.reconcileLoopbackClaim(ctx, fabric, claimName)
		if err != nil {
			return ctrl.Result{}, err
		}
		intf, err := r.reconcileLoopbackInterface(ctx, fabric, &devices.Items[i], LoopbackRouterID, claim)
		if err != nil {
			return ctrl.Result{}, err
		}
		if intf != nil {
			state.loopbacks[devices.Items[i].Name] = append(state.loopbacks[devices.Items[i].Name], intf)
		}
	}
	return ctrl.Result{}, nil
}

// reconcileVTEPLoopbacks ensures lo1 (primary VTEP) and lo2 (anycast VTEP) exist on VTEP devices.
func (r *FabricReconciler) reconcileVTEPLoopbacks(ctx context.Context, fabric *evpnv1alpha1.Fabric, state *ReconcileState) (ctrl.Result, error) {
	selector, err := metav1.LabelSelectorAsSelector(&fabric.Spec.VTEP.DeviceSelector)
	if err != nil {
		return ctrl.Result{}, reconcile.TerminalError(fmt.Errorf("invalid vtep deviceSelector: %w", err))
	}
	devices := &v1alpha1.DeviceList{}
	if err := r.List(ctx, devices, client.InNamespace(fabric.Namespace), client.MatchingLabelsSelector{Selector: selector}); err != nil {
		return ctrl.Result{}, fmt.Errorf("listing VTEP devices: %w", err)
	}
	for i := range devices.Items {
		for _, id := range []int{LoopbackVTEP, LoopbackVTEPAnycast} {
			claimName := fmt.Sprintf("%s-%s-lo%d", fabric.Name, devices.Items[i].Name, id)
			claim, err := r.reconcileLoopbackClaim(ctx, fabric, claimName)
			if err != nil {
				return ctrl.Result{}, err
			}
			intf, err := r.reconcileLoopbackInterface(ctx, fabric, &devices.Items[i], id, claim)
			if err != nil {
				return ctrl.Result{}, err
			}
			if intf != nil {
				state.loopbacks[devices.Items[i].Name] = append(state.loopbacks[devices.Items[i].Name], intf)
			}
		}
	}
	return ctrl.Result{}, nil
}

// reconcileAnycastRPLoopbacks ensures lo100 (PIM anycast RP) exists on RP devices.
// One claim is allocated per AnycastRendezvousPoint group; all RP devices in the group
// share that single address.
func (r *FabricReconciler) reconcileAnycastRPLoopbacks(ctx context.Context, fabric *evpnv1alpha1.Fabric, state *ReconcileState) (ctrl.Result, error) {
	if fabric.Spec.BUM.PIM == nil {
		return ctrl.Result{}, nil
	}
	for _, rp := range fabric.Spec.BUM.PIM.AnycastRendezvousPoints {
		claimName := fmt.Sprintf("%s-%s-lo%d", fabric.Name, rp.Name, LoopbackAnycastRP)
		claim, err := r.reconcileLoopbackClaim(ctx, fabric, claimName)
		if err != nil {
			return ctrl.Result{}, err
		}
		selector, err := metav1.LabelSelectorAsSelector(&rp.DeviceSelector)
		if err != nil {
			return ctrl.Result{}, reconcile.TerminalError(fmt.Errorf("invalid anycast rendezvous-point deviceSelector %q: %w", rp.Name, err))
		}
		devices := &v1alpha1.DeviceList{}
		if err := r.List(ctx, devices, client.InNamespace(fabric.Namespace), client.MatchingLabelsSelector{Selector: selector}); err != nil {
			return ctrl.Result{}, fmt.Errorf("listing RP devices for %q: %w", rp.Name, err)
		}
		for i := range devices.Items {
			intf, err := r.reconcileLoopbackInterface(ctx, fabric, &devices.Items[i], LoopbackAnycastRP, claim)
			if err != nil {
				return ctrl.Result{}, err
			}
			if intf != nil {
				state.loopbacks[devices.Items[i].Name] = append(state.loopbacks[devices.Items[i].Name], intf)
			}
		}
	}
	return ctrl.Result{}, nil
}

// reconcileLoopbackClaim ensures a Claim with the given name exists and matches the desired spec.
// Returns the Claim object so callers can pass it directly to reconcileLoopbackInterface.
func (r *FabricReconciler) reconcileLoopbackClaim(ctx context.Context, fabric *evpnv1alpha1.Fabric, claimName string) (*poolv1alpha1.Claim, error) {
	claim := &poolv1alpha1.Claim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      claimName,
			Namespace: fabric.Namespace,
		},
	}
	res, err := controllerutil.CreateOrPatch(ctx, r.Client, claim, func() error {
		if claim.Labels == nil {
			claim.Labels = make(map[string]string)
		}
		claim.Labels[evpnv1alpha1.FabricLabel] = fabric.Name
		claim.Spec = poolv1alpha1.ClaimSpec{
			PoolRef: v1alpha1.TypedLocalObjectReference{
				APIVersion: poolv1alpha1.GroupVersion.String(),
				Kind:       "IPAddressPool",
				Name:       fabric.Spec.Loopbacks.IPAddressPoolRef.Name,
			},
		}
		return controllerutil.SetControllerReference(fabric, claim, r.Scheme)
	})
	if err != nil {
		return nil, fmt.Errorf("reconciling claim %s: %w", claimName, err)
	}
	if res == controllerutil.OperationResultCreated {
		r.Recorder.Eventf(fabric, nil, "Normal", "ClaimCreated", "Reconcile", "Created loopback address claim %s", claimName)
	}
	return claim, nil
}

// reconcileLoopbackInterface creates or updates the Interface for a given device loopback
// once its Claim is allocated. Returns nil if the claim is not yet allocated; the Owns() watch
// on Claim will re-enqueue this Fabric when the pool controller updates the claim status.
func (r *FabricReconciler) reconcileLoopbackInterface(ctx context.Context, fabric *evpnv1alpha1.Fabric, device *v1alpha1.Device, loopbackID int, claim *poolv1alpha1.Claim) (*v1alpha1.Interface, error) {
	cond := conditions.Get(claim, poolv1alpha1.AllocatedCondition)
	if cond == nil || cond.Status != metav1.ConditionTrue || claim.Status.Value == "" {
		return nil, nil //nolint:nilnil // claim not yet allocated; Owns() watch will re-enqueue
	}

	prefix, err := v1alpha1.ParsePrefix(claim.Status.Value + "/32")
	if err != nil {
		return nil, reconcile.TerminalError(fmt.Errorf("parsing allocated address %q: %w", claim.Status.Value, err))
	}

	handle, err := r.Provider().(provider.InterfaceProvider).LoopbackInterfaceName(loopbackID)
	if err != nil {
		return nil, reconcile.TerminalError(fmt.Errorf("resolving loopback interface name for id %d: %w", loopbackID, err))
	}

	name := fmt.Sprintf("%s-%s-%s", fabric.Name, device.Name, handle)
	intf := &v1alpha1.Interface{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: fabric.Namespace,
		},
	}
	res, err := controllerutil.CreateOrPatch(ctx, r.Client, intf, func() error {
		if intf.Labels == nil {
			intf.Labels = make(map[string]string)
		}
		intf.Labels[evpnv1alpha1.FabricLabel] = fabric.Name
		intf.Spec.DeviceRef = v1alpha1.LocalObjectReference{Name: device.Name}
		intf.Spec.Name = handle
		intf.Spec.Type = v1alpha1.InterfaceTypeLoopback
		intf.Spec.AdminState = v1alpha1.AdminStateUp
		switch loopbackID {
		case LoopbackRouterID:
			intf.Spec.Description = "Router-ID, BGP Source"
		case LoopbackVTEP:
			intf.Spec.Description = "Primary VTEP"
		case LoopbackVTEPAnycast:
			intf.Spec.Description = "VTEP Anycast"
		case LoopbackAnycastRP:
			intf.Spec.Description = "Rendezvous Point"
		}
		if intf.Spec.IPv4 == nil {
			intf.Spec.IPv4 = &v1alpha1.InterfaceIPv4{}
		}
		if len(intf.Spec.IPv4.Addresses) == 0 || intf.Spec.IPv4.Addresses[0] != prefix {
			intf.Spec.IPv4.Addresses = []v1alpha1.IPPrefix{prefix}
		}
		return controllerutil.SetOwnerReference(fabric, intf, r.Scheme)
	})
	if err != nil {
		return nil, fmt.Errorf("reconciling interface %s: %w", name, err)
	}
	if res == controllerutil.OperationResultCreated {
		r.Recorder.Eventf(fabric, nil, "Normal", "InterfaceCreated", "Reconcile", "Created loopback interface %s", name)
	}
	return intf, nil
}

// reconcileUnderlayLinks patches pre-existing Interface resources matched by
// spec.underlay.interfaceSelector with MTU 9216 and IPv4 configuration.
// For unnumbered addressing, interfaces borrow the IPv4 address from their device's lo0.
// For numbered addressing, one /31 prefix Claim is allocated per link pair (identified by
// PhysicalInterfaceNeighborLabel); both ends derive their host address from that prefix.
func (r *FabricReconciler) reconcileUnderlayLinks(ctx context.Context, fabric *evpnv1alpha1.Fabric, state *ReconcileState) (ctrl.Result, error) {
	intfSelector, err := metav1.LabelSelectorAsSelector(&fabric.Spec.Underlay.InterfaceSelector)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("invalid underlay interfaceSelector: %w", err)
	}
	interfaces := &v1alpha1.InterfaceList{}
	if err := r.List(ctx, interfaces, client.InNamespace(fabric.Namespace), client.MatchingLabelsSelector{Selector: intfSelector}); err != nil {
		return ctrl.Result{}, fmt.Errorf("listing underlay interfaces: %w", err)
	}
	deviceSelector, err := metav1.LabelSelectorAsSelector(&fabric.Spec.DeviceSelector)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("invalid deviceSelector: %w", err)
	}
	devices := &v1alpha1.DeviceList{}
	if err := r.List(ctx, devices, client.InNamespace(fabric.Namespace), client.MatchingLabelsSelector{Selector: deviceSelector}); err != nil {
		return ctrl.Result{}, fmt.Errorf("listing devices: %w", err)
	}
	deviceSet := sets.New[string]()
	for i := range devices.Items {
		deviceSet.Insert(devices.Items[i].Name)
	}
	for i := range interfaces.Items {
		intf := &interfaces.Items[i]
		if intf.Spec.Type != v1alpha1.InterfaceTypePhysical {
			return ctrl.Result{}, fmt.Errorf("interface %s has type %s, expected %s", intf.Name, intf.Spec.Type, v1alpha1.InterfaceTypePhysical)
		}
		if !deviceSet.Has(intf.Spec.DeviceRef.Name) {
			return ctrl.Result{}, fmt.Errorf("interface %s references device %s which is not part of the fabric", intf.Name, intf.Spec.DeviceRef.Name)
		}
		state.uplinks[intf.Spec.DeviceRef.Name] = append(state.uplinks[intf.Spec.DeviceRef.Name], intf)
		var err error
		switch {
		case fabric.Spec.Underlay.Addressing.Unnumbered:
			err = r.reconcileUnderlayInterfaceUnnumbered(ctx, fabric, intf)
		case fabric.Spec.Underlay.Addressing.IPPrefixPoolRef != nil:
			err = r.reconcileUnderlayInterfaceNumbered(ctx, fabric, intf)
		}
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

// reconcileUnderlayInterfaceUnnumbered patches the interface to borrow the IPv4 address
// from the device's lo0 interface created by reconcileSystemLoopbacks.
func (r *FabricReconciler) reconcileUnderlayInterfaceUnnumbered(ctx context.Context, fabric *evpnv1alpha1.Fabric, intf *v1alpha1.Interface) error {
	orig := intf.DeepCopy()
	intf.Spec.MTU = 9216
	intf.Spec.IPv4 = &v1alpha1.InterfaceIPv4{
		Unnumbered: &v1alpha1.InterfaceIPv4Unnumbered{
			InterfaceRef: v1alpha1.LocalObjectReference{
				Name: fmt.Sprintf("%s-%s-lo0", fabric.Name, intf.Spec.DeviceRef.Name),
			},
		},
	}
	if equality.Semantic.DeepEqual(orig.Spec, intf.Spec) {
		return nil
	}
	// TODO: switch to server-side apply with field ownership once applyconfiguration generation is available
	if err := r.Patch(ctx, intf, client.MergeFrom(orig)); err != nil {
		return fmt.Errorf("patching underlay interface %s: %w", intf.Name, err)
	}
	return nil
}

// reconcileUnderlayInterfaceNumbered allocates a /31 prefix per link pair via a Claim and
// assigns a host address to this interface. The peer is identified by PhysicalInterfaceNeighborLabel;
// if absent the interface is skipped (link not yet complete). The claim name is derived from
// the two interface names sorted alphabetically so both ends resolve to the same Claim.
// The lexicographically first interface receives addr 0 of the /31; the second receives addr 1.
func (r *FabricReconciler) reconcileUnderlayInterfaceNumbered(ctx context.Context, fabric *evpnv1alpha1.Fabric, intf *v1alpha1.Interface) error {
	peerName, ok := intf.Labels[v1alpha1.PhysicalInterfaceNeighborLabel]
	if !ok {
		ctrl.LoggerFrom(ctx).V(1).Info("Skipping interface without neighbor label", "interface", intf.Name)
		return nil
	}

	// Stable claim name: sort the two interface names so both ends agree.
	a, b := intf.Name, peerName
	claimName := fmt.Sprintf("%s-%s-%s-p2p", fabric.Name, min(a, b), max(a, b))

	claim, err := r.reconcileUnderlayPrefixClaim(ctx, fabric, claimName)
	if err != nil {
		return err
	}

	cond := conditions.Get(claim, poolv1alpha1.AllocatedCondition)
	if cond == nil || cond.Status != metav1.ConditionTrue || claim.Status.Value == "" {
		return nil
	}

	prefix, err := v1alpha1.ParsePrefix(claim.Status.Value)
	if err != nil {
		return reconcile.TerminalError(fmt.Errorf("parsing allocated prefix %q for claim %s: %w", claim.Status.Value, claimName, err))
	}

	// Assign addr 0 to the lex-first interface, addr 1 to the second.
	addr := prefix.Addr()
	if cmp.Compare(intf.Name, peerName) > 0 {
		addr = addr.Next()
	}

	orig := intf.DeepCopy()
	intf.Spec.MTU = 9216
	intf.Spec.IPv4 = &v1alpha1.InterfaceIPv4{
		Addresses: []v1alpha1.IPPrefix{{Prefix: netip.PrefixFrom(addr, prefix.Bits())}},
	}
	if equality.Semantic.DeepEqual(orig.Spec, intf.Spec) {
		return nil
	}
	// TODO: switch to server-side apply with field ownership once applyconfiguration generation is available
	if err := r.Patch(ctx, intf, client.MergeFrom(orig)); err != nil {
		return fmt.Errorf("patching underlay interface %s: %w", intf.Name, err)
	}
	return nil
}

// reconcileUnderlayPrefixClaim ensures a Claim against an IPPrefixPool exists for the given link.
func (r *FabricReconciler) reconcileUnderlayPrefixClaim(ctx context.Context, fabric *evpnv1alpha1.Fabric, claimName string) (*poolv1alpha1.Claim, error) {
	claim := &poolv1alpha1.Claim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      claimName,
			Namespace: fabric.Namespace,
		},
	}
	res, err := controllerutil.CreateOrPatch(ctx, r.Client, claim, func() error {
		if claim.Labels == nil {
			claim.Labels = make(map[string]string)
		}
		claim.Labels[evpnv1alpha1.FabricLabel] = fabric.Name
		claim.Spec = poolv1alpha1.ClaimSpec{
			PoolRef: v1alpha1.TypedLocalObjectReference{
				APIVersion: poolv1alpha1.GroupVersion.String(),
				Kind:       "IPPrefixPool",
				Name:       fabric.Spec.Underlay.Addressing.IPPrefixPoolRef.Name,
			},
		}
		return controllerutil.SetControllerReference(fabric, claim, r.Scheme)
	})
	if err != nil {
		return nil, fmt.Errorf("reconciling prefix claim %s: %w", claimName, err)
	}
	if res == controllerutil.OperationResultCreated {
		r.Recorder.Eventf(fabric, nil, "Normal", "ClaimCreated", "Reconcile", "Created underlay prefix claim %s", claimName)
	}
	return claim, nil
}

// reconcileUnderlayIGP materialises the underlay IGP (OSPF or ISIS) as one resource per
// fabric device. Loopbacks and uplinks are read from the reconcileState accumulated by
// earlier phases. Devices whose lo0 is not yet allocated are skipped; the Owns() watch on
// Interface re-enqueues the Fabric once lo0 appears.
func (r *FabricReconciler) reconcileUnderlayIGP(ctx context.Context, fabric *evpnv1alpha1.Fabric, state *ReconcileState) (ctrl.Result, error) {
	selector, err := metav1.LabelSelectorAsSelector(&fabric.Spec.DeviceSelector)
	if err != nil {
		return ctrl.Result{}, reconcile.TerminalError(fmt.Errorf("invalid deviceSelector: %w", err))
	}

	devices := &v1alpha1.DeviceList{}
	if err := r.List(ctx, devices, client.InNamespace(fabric.Namespace), client.MatchingLabelsSelector{Selector: selector}); err != nil {
		return ctrl.Result{}, fmt.Errorf("listing devices: %w", err)
	}

	for i := range devices.Items {
		device := &devices.Items[i]

		loopbacks := slices.Clone(state.loopbacks[device.Name])
		uplinks := slices.Clone(state.uplinks[device.Name])
		slices.SortFunc(loopbacks, func(a, b *v1alpha1.Interface) int { return cmp.Compare(a.Name, b.Name) })
		slices.SortFunc(uplinks, func(a, b *v1alpha1.Interface) int { return cmp.Compare(a.Name, b.Name) })

		lo0Name := fmt.Sprintf("%s-%s-lo%d", fabric.Name, device.Name, LoopbackRouterID)

		idx := slices.IndexFunc(loopbacks, func(intf *v1alpha1.Interface) bool { return intf.Name == lo0Name })
		if idx < 0 {
			ctrl.LoggerFrom(ctx).V(1).Info("Skipping IGP reconciliation: lo0 not yet allocated", "device", device.Name)
			continue
		}

		lo0 := loopbacks[idx]
		if lo0.Spec.IPv4 == nil || len(lo0.Spec.IPv4.Addresses) == 0 {
			return ctrl.Result{}, reconcile.TerminalError(fmt.Errorf("lo0 interface %s has no IPv4 address", lo0Name))
		}

		routerID := lo0.Spec.IPv4.Addresses[0].Addr().String()
		name := fmt.Sprintf("%s-%s-underlay", fabric.Name, device.Name)

		switch fabric.Spec.Underlay.Protocol {
		case evpnv1alpha1.UnderlayProtocolOSPF:
			if err := r.reconcileOSPF(ctx, device, fabric, name, routerID, loopbacks, uplinks); err != nil {
				return ctrl.Result{}, err
			}
		case evpnv1alpha1.UnderlayProtocolISIS:
			if err := r.reconcileISIS(ctx, device, fabric, name, routerID, loopbacks, uplinks); err != nil {
				return ctrl.Result{}, err
			}
		}
	}
	return ctrl.Result{}, nil
}

// reconcileOSPF creates or updates the underlay OSPF resource for a fabric device.
// All loopbacks are placed in area 0.0.0.0 as passive (advertised but no adjacencies);
// uplinks are placed in area 0.0.0.0 as active.
func (r *FabricReconciler) reconcileOSPF(ctx context.Context, device *v1alpha1.Device, fabric *evpnv1alpha1.Fabric, name, routerID string, loopbacks, uplinks []*v1alpha1.Interface) error {
	ospf := &v1alpha1.OSPF{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: fabric.Namespace,
		},
	}
	res, err := controllerutil.CreateOrPatch(ctx, r.Client, ospf, func() error {
		if ospf.Labels == nil {
			ospf.Labels = make(map[string]string)
		}
		ospf.Labels[evpnv1alpha1.FabricLabel] = fabric.Name
		ospf.Spec.DeviceRef = v1alpha1.LocalObjectReference{Name: device.Name}
		ospf.Spec.AdminState = v1alpha1.AdminStateUp
		ospf.Spec.Instance = "UNDERLAY"
		ospf.Spec.RouterID = routerID
		ospf.Spec.LogAdjacencyChanges = new(true)
		ospf.Spec.InterfaceRefs = make([]v1alpha1.OSPFInterface, 0, len(loopbacks)+len(uplinks))
		for _, lo := range loopbacks {
			ospf.Spec.InterfaceRefs = append(ospf.Spec.InterfaceRefs, v1alpha1.OSPFInterface{
				LocalObjectReference: v1alpha1.LocalObjectReference{Name: lo.Name},
				Area:                 "0.0.0.0",
				Passive:              new(true),
			})
		}
		for _, eth := range uplinks {
			ospf.Spec.InterfaceRefs = append(ospf.Spec.InterfaceRefs, v1alpha1.OSPFInterface{
				LocalObjectReference: v1alpha1.LocalObjectReference{Name: eth.Name},
				Area:                 "0.0.0.0",
			})
		}
		return controllerutil.SetControllerReference(fabric, ospf, r.Scheme)
	})
	if err != nil {
		return fmt.Errorf("reconciling OSPF %s: %w", name, err)
	}
	if res == controllerutil.OperationResultCreated {
		r.Recorder.Eventf(fabric, nil, "Normal", "OSPFCreated", "Reconcile", "Created underlay OSPF %s", name)
	}
	return nil
}

// reconcileISIS creates or updates the underlay ISIS resource for a fabric device.
// Cisco EVPN-VXLAN guidance: Level2, OverloadBit=OnStartup, AddressFamilies=[IPv4Unicast].
// The NET is derived from the device's lo0 IPv4 (see isisNETFromIPv4).
// ISIS has no per-interface passive flag in the API; loopbacks are simply added to
// InterfaceRefs and rely on the protocol's intrinsic behaviour (no neighbors form on
// loopbacks).
func (r *FabricReconciler) reconcileISIS(ctx context.Context, device *v1alpha1.Device, fabric *evpnv1alpha1.Fabric, name, routerID string, loopbacks, uplinks []*v1alpha1.Interface) error {
	net, err := isisNETFromIPv4(routerID)
	if err != nil {
		return err
	}

	isis := &v1alpha1.ISIS{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: fabric.Namespace,
		},
	}
	res, err := controllerutil.CreateOrPatch(ctx, r.Client, isis, func() error {
		if isis.Labels == nil {
			isis.Labels = make(map[string]string)
		}
		isis.Labels[evpnv1alpha1.FabricLabel] = fabric.Name
		isis.Spec.DeviceRef = v1alpha1.LocalObjectReference{Name: device.Name}
		isis.Spec.AdminState = v1alpha1.AdminStateUp
		isis.Spec.Instance = "UNDERLAY"
		isis.Spec.NetworkEntityTitle = net
		isis.Spec.Type = v1alpha1.ISISLevel2
		isis.Spec.OverloadBit = v1alpha1.OverloadBitOnStartup
		isis.Spec.AddressFamilies = []v1alpha1.AddressFamily{v1alpha1.AddressFamilyIPv4Unicast}
		refs := make([]v1alpha1.LocalObjectReference, 0, len(loopbacks)+len(uplinks))
		for _, lo := range loopbacks {
			refs = append(refs, v1alpha1.LocalObjectReference{Name: lo.Name})
		}
		for _, up := range uplinks {
			refs = append(refs, v1alpha1.LocalObjectReference{Name: up.Name})
		}
		isis.Spec.InterfaceRefs = refs
		return controllerutil.SetControllerReference(fabric, isis, r.Scheme)
	})
	if err != nil {
		return fmt.Errorf("reconciling ISIS %s: %w", name, err)
	}
	if res == controllerutil.OperationResultCreated {
		r.Recorder.Eventf(fabric, nil, "Normal", "ISISCreated", "Reconcile", "Created underlay ISIS %s", name)
	}
	return nil
}

// isisNETFromIPv4 derives a Network Entity Title from an IPv4 address by zero-padding
// each octet to three digits and regrouping into 4-hex-digit system-ID chunks. For
// example "10.0.0.10" → "010.000.000.010" → "0100.0000.0010" → "49.0001.0100.0000.0010.00".
// Area 49.0001 (private) is conventional for EVPN fabrics.
func isisNETFromIPv4(addr string) (string, error) {
	ip, err := netip.ParseAddr(addr)
	if err != nil || !ip.Is4() {
		return "", fmt.Errorf("invalid IPv4 address %q", addr)
	}
	octets := ip.As4()
	padded := fmt.Sprintf("%03d%03d%03d%03d", octets[0], octets[1], octets[2], octets[3])
	systemID := fmt.Sprintf("%s.%s.%s", padded[0:4], padded[4:8], padded[8:12])
	return fmt.Sprintf("49.0001.%s.00", systemID), nil
}

// reconcileOverlayBGP materialises the iBGP overlay control plane. For each route reflector
// group it creates BGPPeer resources for each directional peering relationship. When
// deviceSelector and clientDeviceSelector resolve to the same set, a full mesh is created;
// otherwise RR-to-client peering is established.
func (r *FabricReconciler) reconcileOverlayBGP(ctx context.Context, fabric *evpnv1alpha1.Fabric, state *ReconcileState) (ctrl.Result, error) {
	if fabric.Spec.Overlay.Protocol != evpnv1alpha1.OverlayProtocolIBGP || fabric.Spec.Overlay.IBGP == nil {
		return ctrl.Result{}, nil
	}

	// Create a BGP instance on every fabric device.
	selector, err := metav1.LabelSelectorAsSelector(&fabric.Spec.DeviceSelector)
	if err != nil {
		return ctrl.Result{}, reconcile.TerminalError(fmt.Errorf("invalid deviceSelector: %w", err))
	}

	devices := &v1alpha1.DeviceList{}
	if err := r.List(ctx, devices, client.InNamespace(fabric.Namespace), client.MatchingLabelsSelector{Selector: selector}); err != nil {
		return ctrl.Result{}, fmt.Errorf("listing devices: %w", err)
	}

	for i := range devices.Items {
		device := &devices.Items[i]
		lo0Name := fmt.Sprintf("%s-%s-lo%d", fabric.Name, device.Name, LoopbackRouterID)

		loopbacks := state.loopbacks[device.Name]
		idx := slices.IndexFunc(loopbacks, func(intf *v1alpha1.Interface) bool { return intf.Name == lo0Name })
		if idx < 0 {
			ctrl.LoggerFrom(ctx).V(1).Info("Skipping BGP reconciliation: lo0 not yet allocated", "device", device.Name)
			continue
		}

		lo0 := loopbacks[idx]
		if lo0.Spec.IPv4 == nil || len(lo0.Spec.IPv4.Addresses) == 0 {
			return ctrl.Result{}, reconcile.TerminalError(fmt.Errorf("lo0 interface %s has no IPv4 address", lo0Name))
		}

		routerID := lo0.Spec.IPv4.Addresses[0].Addr().String()
		name := fmt.Sprintf("%s-%s-overlay", fabric.Name, device.Name)

		if err := r.reconcileBGP(ctx, device, fabric, name, routerID); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Create BGPPeer resources per route reflector group.
	for _, group := range fabric.Spec.Overlay.IBGP.RouteReflectors {
		rrSelector, err := metav1.LabelSelectorAsSelector(&group.DeviceSelector)
		if err != nil {
			return ctrl.Result{}, reconcile.TerminalError(fmt.Errorf("invalid deviceSelector in RR group %q: %w", group.Name, err))
		}
		clientSelector, err := metav1.LabelSelectorAsSelector(&group.ClientDeviceSelector)
		if err != nil {
			return ctrl.Result{}, reconcile.TerminalError(fmt.Errorf("invalid clientDeviceSelector in RR group %q: %w", group.Name, err))
		}

		rrDevices := &v1alpha1.DeviceList{}
		if err := r.List(ctx, rrDevices, client.InNamespace(fabric.Namespace), client.MatchingLabelsSelector{Selector: rrSelector}); err != nil {
			return ctrl.Result{}, fmt.Errorf("listing RR devices for group %q: %w", group.Name, err)
		}
		clientDevices := &v1alpha1.DeviceList{}
		if err := r.List(ctx, clientDevices, client.InNamespace(fabric.Namespace), client.MatchingLabelsSelector{Selector: clientSelector}); err != nil {
			return ctrl.Result{}, fmt.Errorf("listing client devices for group %q: %w", group.Name, err)
		}

		// Determine peering mode: full mesh or RR-to-client.
		rrNames := sets.New[string]()
		for i := range rrDevices.Items {
			rrNames.Insert(rrDevices.Items[i].Name)
		}
		clientNames := sets.New[string]()
		for i := range clientDevices.Items {
			clientNames.Insert(clientDevices.Items[i].Name)
		}

		if rrNames.Equal(clientNames) {
			// Full mesh: every device peers with every other.
			for i := range rrDevices.Items {
				for j := range rrDevices.Items {
					if i == j {
						continue
					}
					if err := r.reconcileBGPPeer(ctx, &rrDevices.Items[i], &rrDevices.Items[j], fabric, state, false); err != nil {
						return ctrl.Result{}, err
					}
				}
			}
		} else {
			// RR-to-client: each RR peers with each client.
			for i := range rrDevices.Items {
				for j := range clientDevices.Items {
					if rrDevices.Items[i].Name == clientDevices.Items[j].Name {
						continue
					}
					if err := r.reconcileBGPPeer(ctx, &rrDevices.Items[i], &clientDevices.Items[j], fabric, state, true); err != nil {
						return ctrl.Result{}, err
					}
					if err := r.reconcileBGPPeer(ctx, &clientDevices.Items[j], &rrDevices.Items[i], fabric, state, false); err != nil {
						return ctrl.Result{}, err
					}
				}
			}
		}
	}
	return ctrl.Result{}, nil
}

// reconcileBGP creates or updates the overlay BGP instance for a fabric device.
func (r *FabricReconciler) reconcileBGP(ctx context.Context, device *v1alpha1.Device, fabric *evpnv1alpha1.Fabric, name, routerID string) error {
	bgp := &v1alpha1.BGP{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: fabric.Namespace,
		},
	}
	res, err := controllerutil.CreateOrPatch(ctx, r.Client, bgp, func() error {
		if bgp.Labels == nil {
			bgp.Labels = make(map[string]string)
		}
		bgp.Labels[evpnv1alpha1.FabricLabel] = fabric.Name
		bgp.Spec.DeviceRef = v1alpha1.LocalObjectReference{Name: device.Name}
		bgp.Spec.AdminState = v1alpha1.AdminStateUp
		bgp.Spec.ASNumber = fabric.Spec.Overlay.IBGP.ASNumber
		bgp.Spec.RouterID = routerID
		bgp.Spec.AddressFamilies = &v1alpha1.BGPAddressFamilies{
			L2vpnEvpn: &v1alpha1.BGPL2vpnEvpn{
				BGPAddressFamily: v1alpha1.BGPAddressFamily{Enabled: true},
				RouteTargetPolicy: &v1alpha1.BGPRouteTargetPolicy{
					RetainAll: true,
				},
			},
		}
		return controllerutil.SetControllerReference(fabric, bgp, r.Scheme)
	})
	if err != nil {
		return fmt.Errorf("reconciling BGP %s: %w", name, err)
	}
	if res == controllerutil.OperationResultCreated {
		r.Recorder.Eventf(fabric, nil, "Normal", "BGPCreated", "Reconcile", "Created overlay BGP %s", name)
	}
	return nil
}

// reconcileBGPPeer creates or updates a directional BGP peer between two fabric devices.
// If rrClient is true, the remote device is marked as a route-reflector client of the local device.
func (r *FabricReconciler) reconcileBGPPeer(ctx context.Context, local, remote *v1alpha1.Device, fabric *evpnv1alpha1.Fabric, state *ReconcileState, rrClient bool) error {
	// Resolve remote lo0 address for the peer address.
	remoteLo0Name := fmt.Sprintf("%s-%s-lo%d", fabric.Name, remote.Name, LoopbackRouterID)
	remoteLoopbacks := state.loopbacks[remote.Name]

	idx := slices.IndexFunc(remoteLoopbacks, func(intf *v1alpha1.Interface) bool { return intf.Name == remoteLo0Name })
	if idx < 0 {
		ctrl.LoggerFrom(ctx).V(1).Info("Skipping BGPPeer: remote lo0 not yet allocated", "local", local.Name, "remote", remote.Name)
		return nil
	}

	remoteLo0 := remoteLoopbacks[idx]
	if remoteLo0.Spec.IPv4 == nil || len(remoteLo0.Spec.IPv4.Addresses) == 0 {
		return reconcile.TerminalError(fmt.Errorf("lo0 interface %s has no IPv4 address", remoteLo0Name))
	}

	peerAddr := remoteLo0.Spec.IPv4.Addresses[0].Addr().String()
	localLo0Name := fmt.Sprintf("%s-%s-lo%d", fabric.Name, local.Name, LoopbackRouterID)
	name := fmt.Sprintf("%s-%s-%s", fabric.Name, local.Name, remote.Name)

	peer := &v1alpha1.BGPPeer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: fabric.Namespace,
		},
	}
	res, err := controllerutil.CreateOrPatch(ctx, r.Client, peer, func() error {
		if peer.Labels == nil {
			peer.Labels = make(map[string]string)
		}
		peer.Labels[evpnv1alpha1.FabricLabel] = fabric.Name
		peer.Spec.DeviceRef = v1alpha1.LocalObjectReference{Name: local.Name}
		peer.Spec.AdminState = v1alpha1.AdminStateUp
		peer.Spec.BgpRef = v1alpha1.LocalObjectReference{Name: fmt.Sprintf("%s-%s-overlay", fabric.Name, local.Name)}
		peer.Spec.Address = peerAddr
		peer.Spec.ASNumber = fabric.Spec.Overlay.IBGP.ASNumber
		peer.Spec.LocalAddress = &v1alpha1.BGPPeerLocalAddress{
			InterfaceRef: v1alpha1.LocalObjectReference{Name: localLo0Name},
		}
		peer.Spec.AddressFamilies = &v1alpha1.BGPPeerAddressFamilies{
			L2vpnEvpn: &v1alpha1.BGPPeerAddressFamily{
				Enabled:              true,
				SendCommunity:        v1alpha1.BGPCommunityTypeBoth,
				RouteReflectorClient: rrClient,
			},
		}
		return controllerutil.SetControllerReference(fabric, peer, r.Scheme)
	})
	if err != nil {
		return fmt.Errorf("reconciling BGPPeer %s: %w", name, err)
	}
	if res == controllerutil.OperationResultCreated {
		r.Recorder.Eventf(fabric, nil, "Normal", "BGPPeerCreated", "Reconcile", "Created overlay BGPPeer %s", name)
	}
	return nil
}

// devicesToFabrics is a [handler.MapFunc] that enqueues all Fabrics whose
// spec.deviceSelector matches the labels of the changed Device.
func (r *FabricReconciler) devicesToFabrics(ctx context.Context, obj client.Object) []ctrl.Request {
	device, ok := obj.(*v1alpha1.Device)
	if !ok {
		panic(fmt.Sprintf("Expected a Device but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx)

	fabricList := &evpnv1alpha1.FabricList{}
	if err := r.List(ctx, fabricList, client.InNamespace(device.Namespace)); err != nil {
		log.Error(err, "Failed to list Fabrics")
		return nil
	}

	var requests []ctrl.Request
	for _, fabric := range fabricList.Items {
		selector, err := metav1.LabelSelectorAsSelector(&fabric.Spec.DeviceSelector)
		if err != nil {
			log.Error(err, "Failed to parse deviceSelector", "fabric", fabric.Name)
			continue
		}
		if selector.Matches(labels.Set(device.Labels)) {
			log.V(2).Info("Enqueuing Fabric for reconciliation", "fabric", fabric.Name)
			requests = append(requests, ctrl.Request{
				NamespacedName: client.ObjectKeyFromObject(&fabric),
			})
		}
	}
	return requests
}

// interfacesToFabrics is a [handler.MapFunc] that enqueues all Fabrics whose
// spec.underlay.interfaceSelector matches the labels of the changed Interface.
func (r *FabricReconciler) interfacesToFabrics(ctx context.Context, obj client.Object) []ctrl.Request {
	intf, ok := obj.(*v1alpha1.Interface)
	if !ok {
		panic(fmt.Sprintf("Expected an Interface but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx)

	fabricList := &evpnv1alpha1.FabricList{}
	if err := r.List(ctx, fabricList, client.InNamespace(intf.Namespace)); err != nil {
		log.Error(err, "Failed to list Fabrics")
		return nil
	}

	var requests []ctrl.Request
	for _, fabric := range fabricList.Items {
		selector, err := metav1.LabelSelectorAsSelector(&fabric.Spec.Underlay.InterfaceSelector)
		if err != nil {
			log.Error(err, "Failed to parse underlay interfaceSelector", "fabric", fabric.Name)
			continue
		}
		if selector.Matches(labels.Set(intf.Labels)) {
			log.V(2).Info("Enqueuing Fabric for reconciliation", "fabric", fabric.Name)
			requests = append(requests, ctrl.Request{
				NamespacedName: client.ObjectKeyFromObject(&fabric),
			})
		}
	}
	return requests
}
