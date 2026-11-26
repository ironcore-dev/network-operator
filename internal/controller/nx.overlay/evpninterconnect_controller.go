// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxoverlay

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	nxv1alpha1 "github.com/ironcore-dev/network-operator/api/cisco/nx/v1alpha1"
	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	nxoverlayv1alpha1 "github.com/ironcore-dev/network-operator/api/nx.overlay/v1alpha1"
	poolv1alpha1 "github.com/ironcore-dev/network-operator/api/pool/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/conditions"
	"github.com/ironcore-dev/network-operator/internal/provider"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
)

const (
	loopbackID              = 9
	loopbackDeviceLabel     = "nx.overlay.networking.metal.ironcore.dev/loopback-device"
	dependencyRetryInterval = 5 * time.Second
	delayRestoreTime        = 180 * time.Second
)

// EVPNInterconnectReconciler reconciles a EVPNInterconnect object
type EVPNInterconnectReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=nx.overlay.networking.metal.ironcore.dev,resources=evpninterconnects,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=nx.overlay.networking.metal.ironcore.dev,resources=evpninterconnects/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=nx.overlay.networking.metal.ironcore.dev,resources=evpninterconnects/finalizers,verbs=update
// +kubebuilder:rbac:groups=pool.networking.metal.ironcore.dev,resources=claims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=devices,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=interfaces,verbs=get;list;watch;create;patch;delete
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=networkvirtualizationedges,verbs=get;list;watch
// +kubebuilder:rbac:groups=nx.cisco.networking.metal.ironcore.dev,resources=bordergateways,verbs=get;list;watch;create;patch;delete
// +kubebuilder:rbac:groups=core.networking.metal.ironcore.dev,resources=ospfs,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the EVPNInterconnect object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.24.1/pkg/reconcile
func (r *EVPNInterconnectReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling resource")

	ic := new(nxoverlayv1alpha1.EVPNInterconnect)
	if err := r.Get(ctx, req.NamespacedName, ic); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get resource")
		return ctrl.Result{}, err
	}

	if !ic.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(ic, nxoverlayv1alpha1.FinalizerName) {
			res, err := r.finalize(ctx, ic)
			if err != nil {
				log.Error(err, "Failed to finalize resource")
				return ctrl.Result{}, err
			}
			if !res.IsZero() {
				return res, nil
			}
			controllerutil.RemoveFinalizer(ic, nxoverlayv1alpha1.FinalizerName)
			if err := r.Update(ctx, ic); err != nil {
				log.Error(err, "Failed to remove finalizer from resource")
				return ctrl.Result{}, err
			}
		}
		log.Info("Resource is being deleted, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers
	if !controllerutil.ContainsFinalizer(ic, nxoverlayv1alpha1.FinalizerName) {
		controllerutil.AddFinalizer(ic, nxoverlayv1alpha1.FinalizerName)
		if err := r.Update(ctx, ic); err != nil {
			log.Error(err, "Failed to add finalizer to resource")
			return ctrl.Result{}, err
		}
		log.Info("Added finalizer to resource")
		return ctrl.Result{}, nil
	}

	orig := ic.DeepCopy()
	if conditions.InitializeConditions(ic, v1alpha1.ReadyCondition, nxoverlayv1alpha1.UnderlayConvergedCondition, nxoverlayv1alpha1.OverlayConvergedCondition) {
		log.V(1).Info("Initializing status conditions")
		return ctrl.Result{}, r.Status().Update(ctx, ic)
	}

	defer func() {
		if !equality.Semantic.DeepEqual(orig.ObjectMeta, ic.ObjectMeta) {
			// Pass obj.DeepCopy() to avoid Patch() modifying obj and interfering with status update below
			if err := r.Patch(ctx, ic.DeepCopy(), client.MergeFrom(orig)); err != nil {
				log.Error(err, "Failed to update resource metadata")
				reterr = kerrors.NewAggregate([]error{reterr, err})
			}
		}
		if !equality.Semantic.DeepEqual(orig.Status, ic.Status) {
			if err := r.Status().Patch(ctx, ic, client.MergeFrom(orig)); err != nil {
				log.Error(err, "Failed to update status")
				reterr = kerrors.NewAggregate([]error{reterr, err})
			}
		}
	}()

	res, err := r.reconcile(ctx, ic)
	if err != nil {
		log.Error(err, "Failed to reconcile resource")
		return ctrl.Result{}, err
	}

	return res, nil
}

func (r *EVPNInterconnectReconciler) finalize(ctx context.Context, ic *nxoverlayv1alpha1.EVPNInterconnect) (ctrl.Result, error) {
	return r.cleanup(ctx, ic, nil)
}

func (r *EVPNInterconnectReconciler) reconcile(ctx context.Context, ic *nxoverlayv1alpha1.EVPNInterconnect) (ctrl.Result, error) {
	defer func() {
		// TODO:
		// r.computeUnderlayConverged(ctx, fabric)
		// r.computeOverlayConverged(ctx, fabric)
		// conditions.RecomputeReady(fabric)
	}()

	state := &ReconcileState{loopbacks: make(map[string]*v1alpha1.Interface)}
	// NVE discovery will populate VTEPs when implemented.

	// The ordering of phases is critical. Each phase may depend on resources
	// provisioned by earlier phases:
	//
	//  1. claimMultiSiteID - claims the Multi-Site ID from the pool
	//  1. reconcileInterfaces - configures addressing and MTU
	//  1. reconcileUnderlay (configures underlay e.g., create OSPF brick)
	//  1. reconcileLoopbacks - allocates addresses and creates multisite loopback9
	//  1. reconcileBorderGateway - waits for the Fabric NVE before configuring multisite

	// Phases that find their prerequisites missing (e.g. Claims not yet allocated)
	// skip the affected device or requeue without error. Resource watches enqueue
	// the interconnect when dependencies change.
	phases := []ReconcileFunc{
		r.claimMultiSiteID,
		r.reconcileInterfaces,
		r.reconcileLoopbacks,
		r.reconcileUnderlay,
		r.reconcileBorderGateway,
	}
	for _, phase := range phases {
		res, err := phase(ctx, ic, state)
		if err != nil || !res.IsZero() {
			return res, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *EVPNInterconnectReconciler) claimMultiSiteID(ctx context.Context, ic *nxoverlayv1alpha1.EVPNInterconnect, state *ReconcileState) (ctrl.Result, error) {
	claim := &poolv1alpha1.Claim{
		Namespace: ic.Namespace,
		Name:      "multisiteid-" + hash(ic),
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, claim, func() error {
		if claim.Labels == nil {
			claim.Labels = make(map[string]string)
		}
		claim.Labels[nxoverlayv1alpha1.EVPNInterconnectLabel] = ic.Name
		claim.Spec = poolv1alpha1.ClaimSpec{
			PoolRef: v1alpha1.TypedLocalObjectReference{
				APIVersion: poolv1alpha1.GroupVersion.String(),
				Kind:       "IndexPool",
				Name:       ic.Spec.MultiSiteIDIndexPoolRef.Name,
			},
		}
		return controllerutil.SetControllerReference(ic, claim, r.Scheme)
	})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("reconciling multi-site ID claim: %w", err)
	}

	cond := conditions.Get(claim, poolv1alpha1.AllocatedCondition)
	if cond == nil || cond.Status != metav1.ConditionTrue || claim.Status.Value == "" {
		return ctrl.Result{}, nil
	}

	id, err := strconv.ParseInt(claim.Status.Value, 10, 64)
	if err != nil {
		return ctrl.Result{}, reconcile.TerminalError(fmt.Errorf("parsing allocated multi-site ID %q: %w", claim.Status.Value, err))
	}
	state.multiSiteID = id
	return ctrl.Result{}, nil
}

func (r *EVPNInterconnectReconciler) reconcileInterfaces(ctx context.Context, ic *nxoverlayv1alpha1.EVPNInterconnect, state *ReconcileState) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	seenInterfaces := make(map[string]bool)
	seenDevices := make(map[string]bool)
	devices := make(map[string]*v1alpha1.Device)
	interfaces := make(map[string]*v1alpha1.Interface)
	resolved := make([]*v1alpha1.Interface, 0, len(ic.Spec.Interfaces))

	for i := range ic.Spec.Interfaces {
		dci := &ic.Spec.Interfaces[i]

		interfaceName := dci.InterfaceRef.Name
		if seenInterfaces[interfaceName] {
			return ctrl.Result{}, reconcile.TerminalError(fmt.Errorf("interface %q is referenced more than once", dci.InterfaceRef.Name))
		}
		seenInterfaces[interfaceName] = true

		intf := &v1alpha1.Interface{}
		if err := r.Get(ctx, client.ObjectKey{Namespace: ic.Namespace, Name: dci.InterfaceRef.Name}, intf); err != nil {
			if apierrors.IsNotFound(err) {
				log.V(1).Info("Referenced interface not found yet, waiting", "interface", dci.InterfaceRef.Name)
				return ctrl.Result{}, nil
			}
			return ctrl.Result{}, fmt.Errorf("getting interface %q: %w", dci.InterfaceRef.Name, err)
		}
		resolved = append(resolved, intf)
		interfaces[interfaceName] = intf

		deviceName := intf.Spec.DeviceRef.Name
		if seenDevices[deviceName] {
			continue
		}
		seenDevices[deviceName] = true

		device := &v1alpha1.Device{}
		if err := r.Get(ctx, client.ObjectKey{Namespace: ic.Namespace, Name: deviceName}, device); err != nil {
			if apierrors.IsNotFound(err) {
				log.V(1).Info("Referenced device not found yet, waiting", "device", deviceName)
				return ctrl.Result{}, nil
			}
			return ctrl.Result{}, fmt.Errorf("getting device %q: %w", deviceName, err)
		}

		devices[deviceName] = device
	}

	if len(seenDevices) != 2 {
		return ctrl.Result{}, reconcile.TerminalError(fmt.Errorf("exactly two distinct devices required, found %d", len(seenDevices)))
	}

	for i := range ic.Spec.Interfaces {
		dci := &ic.Spec.Interfaces[i]
		intf := resolved[i]

		orig := intf.DeepCopy()
		intf.Spec.Switchport = nil
		intf.Spec.IPv4 = &v1alpha1.InterfaceIPv4{
			Addresses: []v1alpha1.IPPrefix{dci.Address},
		}
		if dci.MTU != nil {
			intf.Spec.MTU = *dci.MTU
		}
		if equality.Semantic.DeepEqual(orig.Spec, intf.Spec) {
			continue
		}
		if err := r.Patch(ctx, intf, client.MergeFrom(orig)); err != nil {
			return ctrl.Result{}, fmt.Errorf("patching DCI interface %q: %w", intf.Name, err)
		}
		log.Info("Patched DCI interface", "interface", intf.Name)
	}

	state.devices = devices
	state.interfaces = interfaces
	return ctrl.Result{}, nil
}

func (r *EVPNInterconnectReconciler) reconcileUnderlay(ctx context.Context, ic *nxoverlayv1alpha1.EVPNInterconnect, state *ReconcileState) (ctrl.Result, error) {
	switch ic.Spec.DCI.Underlay.Protocol {
	case nxoverlayv1alpha1.UnderlayProtocolOSPF:
		return r.reconcileOSPF(ctx, ic, state)
	case nxoverlayv1alpha1.UnderlayProtocolBGP:
		return ctrl.Result{}, reconcile.TerminalError(errors.New("BGP underlay is not supported, yet"))
	default:
		return ctrl.Result{}, reconcile.TerminalError(fmt.Errorf("unknown underlay protocol %q", ic.Spec.DCI.Underlay.Protocol))
	}
}

func (r *EVPNInterconnectReconciler) reconcileOSPF(ctx context.Context, ic *nxoverlayv1alpha1.EVPNInterconnect, state *ReconcileState) (ctrl.Result, error) {
	if len(state.interfaces) == 0 {
		return ctrl.Result{}, errors.New("no interfaces resolved: reconcileInterfaces must run before reconcileUnderlay")
	}
	if len(state.loopbacks) == 0 {
		return ctrl.Result{}, errors.New("no loopback interfaces resolved: reconcileLoopbacks must run before reconcileUnderlay")
	}
	if len(state.devices) == 0 {
		return ctrl.Result{}, errors.New("no devices resolved: reconcileInterfaces must run before reconcileUnderlay")
	}

	byDevice := make(map[string][]*v1alpha1.Interface)
	for _, intf := range state.interfaces {
		byDevice[intf.Spec.DeviceRef.Name] = append(byDevice[intf.Spec.DeviceRef.Name], intf)
	}
	for deviceName, intfs := range byDevice {
		routerID, err := ospfRouterID(intfs)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("device %s: %w", deviceName, err)
		}

		loopback := state.loopbacks[deviceName]
		if loopback == nil {
			return ctrl.Result{}, fmt.Errorf("no loopback for device %s", deviceName)
		}

		name := "underlay-" + hash(ic, state.devices[deviceName])
		ospf := &v1alpha1.OSPF{Name: name, Namespace: ic.Namespace}
		res, err := controllerutil.CreateOrPatch(ctx, r.Client, ospf, func() error {
			if ospf.Labels == nil {
				ospf.Labels = make(map[string]string)
			}
			ospf.Labels[nxoverlayv1alpha1.EVPNInterconnectLabel] = ic.Name
			ospf.Spec.DeviceRef = v1alpha1.LocalObjectReference{Name: deviceName}
			ospf.Spec.Instance = "UNDERLAY"
			ospf.Spec.RouterID = routerID
			ospf.Spec.InterfaceRefs = make([]v1alpha1.OSPFInterface, 0, len(intfs)+1)
			for _, intf := range intfs {
				ospf.Spec.InterfaceRefs = append(ospf.Spec.InterfaceRefs, v1alpha1.OSPFInterface{
					Name: intf.Name,
					Area: "0.0.0.0",
				})
			}
			passive := true
			ospf.Spec.InterfaceRefs = append(ospf.Spec.InterfaceRefs, v1alpha1.OSPFInterface{
				Name:    loopback.Name,
				Area:    "0.0.0.0",
				Passive: &passive,
			})
			return controllerutil.SetOwnerReference(ic, ospf, r.Scheme)
		})
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("reconciling OSPF %s: %w", name, err)
		}
		if res == controllerutil.OperationResultCreated {
			ctrl.LoggerFrom(ctx).Info("Created underlay OSPF", "ospf", name)
		}
	}
	return ctrl.Result{}, nil
}

func ospfRouterID(intfs []*v1alpha1.Interface) (string, error) {
	routerID := ""
	for _, intf := range intfs {
		if intf.Spec.IPv4 == nil || len(intf.Spec.IPv4.Addresses) == 0 {
			return "", fmt.Errorf("interface %s has no IPv4 address", intf.Name)
		}
		addr := intf.Spec.IPv4.Addresses[0].Addr().String()
		if routerID == "" {
			routerID = addr
		} else if routerID != addr {
			return "", fmt.Errorf("interface %s has address %s, expected %s: all interfaces on the same device must share the same router-id", intf.Name, addr, routerID)
		}
	}
	return routerID, nil
}

// reconcileLoopbacks creates the separate multisite source loopback on each device.
func (r *EVPNInterconnectReconciler) reconcileLoopbacks(ctx context.Context, ic *nxoverlayv1alpha1.EVPNInterconnect, state *ReconcileState) (ctrl.Result, error) {
	// A nil device map means discovery has not run, rather than an empty membership.
	if state.devices == nil {
		return ctrl.Result{}, nil
	}
	if res, err := r.cleanup(ctx, ic, state.devices); err != nil || !res.IsZero() {
		return res, err
	}
	state.loopbacks = make(map[string]*v1alpha1.Interface)
	for _, deviceName := range slices.Sorted(maps.Keys(state.devices)) {
		device := state.devices[deviceName]
		name := fmt.Sprintf("%s-%s-lo%d", ic.Name, device.Name, loopbackID)
		claim := &poolv1alpha1.Claim{Name: name, Namespace: ic.Namespace}
		_, err := controllerutil.CreateOrPatch(ctx, r.Client, claim, func() error {
			if claim.Labels == nil {
				claim.Labels = make(map[string]string)
			}
			claim.Labels[loopbackDeviceLabel] = device.Name
			claim.Spec.PoolRef = v1alpha1.TypedLocalObjectReference{
				APIVersion: poolv1alpha1.GroupVersion.String(), Kind: "IPAddressPool", Name: ic.Spec.IPAddressPoolRef.Name,
			}
			return controllerutil.SetControllerReference(ic, claim, r.Scheme)
		})
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("reconciling loopback claim %s: %w", name, err)
		}
		if !claim.DeletionTimestamp.IsZero() {
			return ctrl.Result{RequeueAfter: dependencyRetryInterval}, nil
		}
		cond := conditions.Get(claim, poolv1alpha1.AllocatedCondition)
		if cond == nil || cond.Status != metav1.ConditionTrue || claim.Status.Value == "" {
			continue
		}
		prefix, err := v1alpha1.ParsePrefix(claim.Status.Value + "/32")
		if err != nil {
			return ctrl.Result{}, reconcile.TerminalError(fmt.Errorf("parsing allocated address %q: %w", claim.Status.Value, err))
		}
		if !prefix.Addr().Is4() {
			return ctrl.Result{}, reconcile.TerminalError(fmt.Errorf("allocated loopback address %q must be IPv4", claim.Status.Value))
		}
		prov, err := provider.LoadProvider[provider.InterfaceProvider](device.Spec.Provider)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("loading interface provider for device %s: %w", device.Name, err)
		}
		handle, err := prov.LoopbackInterfaceName(loopbackID)
		if err != nil {
			return ctrl.Result{}, reconcile.TerminalError(fmt.Errorf("resolving loopback interface name for device %s: %w", device.Name, err))
		}
		intf := &v1alpha1.Interface{Name: name, Namespace: ic.Namespace}
		_, err = controllerutil.CreateOrPatch(ctx, r.Client, intf, func() error {
			intf.Spec.DeviceRef = v1alpha1.LocalObjectReference{Name: device.Name}
			intf.Spec.Name = handle
			intf.Spec.Type = v1alpha1.InterfaceTypeLoopback
			intf.Spec.AdminState = v1alpha1.AdminStateUp
			intf.Spec.Description = "Multisite border gateway source"
			if intf.Spec.IPv4 == nil {
				intf.Spec.IPv4 = &v1alpha1.InterfaceIPv4{}
			}
			intf.Spec.IPv4.Addresses = []v1alpha1.IPPrefix{prefix}
			return controllerutil.SetControllerReference(ic, intf, r.Scheme)
		})
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("reconciling loopback interface %s: %w", name, err)
		}
		if !intf.DeletionTimestamp.IsZero() {
			return ctrl.Result{RequeueAfter: dependencyRetryInterval}, nil
		}
		state.loopbacks[device.Name] = intf
	}
	return ctrl.Result{}, nil
}

func (r *EVPNInterconnectReconciler) reconcileBorderGateway(ctx context.Context, ic *nxoverlayv1alpha1.EVPNInterconnect, state *ReconcileState) (ctrl.Result, error) {
	if len(state.devices) == 0 {
		return ctrl.Result{}, nil
	}
	pending := false
	for _, deviceName := range slices.Sorted(maps.Keys(state.devices)) {
		nves := state.vteps[deviceName]
		if len(nves) > 1 {
			return ctrl.Result{}, reconcile.TerminalError(fmt.Errorf("multiple NVEs target device %s", deviceName))
		}
		if len(nves) == 0 || !nves[0].DeletionTimestamp.IsZero() || !conditions.IsConfigured(&nves[0]) {
			ctrl.LoggerFrom(ctx).V(1).Info("Waiting for configured Fabric NVE", "device", deviceName)
			pending = true
		}
	}
	if pending || state.multiSiteID == 0 {
		return ctrl.Result{RequeueAfter: dependencyRetryInterval}, nil
	}
	refs := make(map[string][]nxv1alpha1.InterconnectInterfaceReference)
	for _, dci := range ic.Spec.Interfaces {
		intf := state.interfaces[dci.InterfaceRef.Name]
		if intf == nil || !intf.DeletionTimestamp.IsZero() {
			return ctrl.Result{RequeueAfter: dependencyRetryInterval}, nil
		}
		refs[intf.Spec.DeviceRef.Name] = append(refs[intf.Spec.DeviceRef.Name], nxv1alpha1.InterconnectInterfaceReference{
			LocalObjectReference: dci.InterfaceRef, Tracking: nxv1alpha1.InterconnectTrackingTypeDCI,
		})
	}
	for _, deviceName := range slices.Sorted(maps.Keys(state.devices)) {
		loopback := state.loopbacks[deviceName]
		if loopback == nil || !loopback.DeletionTimestamp.IsZero() {
			pending = true
			continue
		}
		name := fmt.Sprintf("%s-%s-bgw", ic.Name, deviceName)
		bgw := &nxv1alpha1.BorderGateway{Name: name, Namespace: ic.Namespace}
		_, err := controllerutil.CreateOrPatch(ctx, r.Client, bgw, func() error {
			bgw.Spec.DeviceRef = v1alpha1.LocalObjectReference{Name: deviceName}
			bgw.Spec.AdminState = v1alpha1.AdminStateUp
			bgw.Spec.MultisiteID = state.multiSiteID
			bgw.Spec.SourceInterfaceRef = v1alpha1.LocalObjectReference{Name: loopback.Name}
			bgw.Spec.InterconnectInterfaceRefs = refs[deviceName]
			if bgw.Spec.DelayRestoreTime.Duration == 0 {
				bgw.Spec.DelayRestoreTime = metav1.Duration{Duration: delayRestoreTime}
			}
			return controllerutil.SetControllerReference(ic, bgw, r.Scheme)
		})
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("reconciling BorderGateway %s: %w", name, err)
		}
		if !bgw.DeletionTimestamp.IsZero() {
			pending = true
		}
	}
	if pending {
		return ctrl.Result{RequeueAfter: dependencyRetryInterval}, nil
	}
	return ctrl.Result{}, nil
}

// ReconcileFunc defines a function type for reconciliation phases.
// Each phase should return a non-zero Result or an error if it wants to stop the reconciliation loop.
type ReconcileFunc func(context.Context, *nxoverlayv1alpha1.EVPNInterconnect, *ReconcileState) (ctrl.Result, error)

// ReconcileState accumulates EVPN Interconnect state across reconciliation phases
type ReconcileState struct {
	devices     map[string]*v1alpha1.Device
	vteps       map[string][]v1alpha1.NetworkVirtualizationEdge // NVEs keyed by device name
	interfaces  map[string]*v1alpha1.Interface                  // DCI interfaces keyed by resource name
	loopbacks   map[string]*v1alpha1.Interface
	multiSiteID int64
}

// SetupWithManager sets up the controller with the Manager.
func (r *EVPNInterconnectReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&nxoverlayv1alpha1.EVPNInterconnect{}).
		Owns(&poolv1alpha1.Claim{}).
		Owns(&v1alpha1.Interface{}).
		Owns(&v1alpha1.OSPF{}).
		Owns(&nxv1alpha1.BorderGateway{}).
		Watches(
			&v1alpha1.Interface{},
			handler.EnqueueRequestsFromMapFunc(r.interfacesToInterconnects),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Watches(
			&v1alpha1.NetworkVirtualizationEdge{},
			handler.EnqueueRequestsFromMapFunc(r.dependenciesToInterconnects),
		).
		Named("nx.overlay-evpninterconnect").
		Complete(r)
}

// cleanup deletes resources controlled by ic in dependency order: BorderGateways,
// Interfaces, then Claims, keeping loopback addresses allocated until their users
// are gone. It requeues until each resource type is fully deleted, including any
// pending child finalizers, before proceeding to the next type.
//
// A nil devices map selects all controlled resources for interconnect deletion.
// A non-nil map retains resources for devices still present; only BorderGateways,
// Interfaces, and device-labeled loopback Claims for removed devices are selected.
// An empty, non-nil map therefore removes all device resources but retains Claims
// without a loopback device label, such as the Multi-Site ID Claim.
func (r *EVPNInterconnectReconciler) cleanup(ctx context.Context, ic *nxoverlayv1alpha1.EVPNInterconnect, devices map[string]*v1alpha1.Device) (ctrl.Result, error) {
	for _, list := range []client.ObjectList{&nxv1alpha1.BorderGatewayList{}, &v1alpha1.InterfaceList{}, &poolv1alpha1.ClaimList{}} {
		if err := r.List(ctx, list, client.InNamespace(ic.Namespace)); err != nil {
			return ctrl.Result{}, fmt.Errorf("listing owned %T resources: %w", list, err)
		}
		var objects []client.Object
		switch list := list.(type) {
		case *nxv1alpha1.BorderGatewayList:
			for i := range list.Items {
				if _, keep := devices[list.Items[i].Spec.DeviceRef.Name]; !keep {
					objects = append(objects, &list.Items[i])
				}
			}
		case *v1alpha1.InterfaceList:
			for i := range list.Items {
				if _, keep := devices[list.Items[i].Spec.DeviceRef.Name]; !keep {
					objects = append(objects, &list.Items[i])
				}
			}
		case *poolv1alpha1.ClaimList:
			for i := range list.Items {
				deviceName, loopback := list.Items[i].Labels[loopbackDeviceLabel]
				if _, keep := devices[deviceName]; devices == nil || (loopback && !keep) {
					objects = append(objects, &list.Items[i])
				}
			}
		}
		pending := false
		for _, obj := range objects {
			if !metav1.IsControlledBy(obj, ic) {
				continue
			}
			pending = true
			if !obj.GetDeletionTimestamp().IsZero() {
				continue
			}
			uid := obj.GetUID()
			if err := r.Delete(ctx, obj, client.Preconditions{UID: &uid}); err != nil && !apierrors.IsNotFound(err) {
				return ctrl.Result{}, fmt.Errorf("deleting owned resource %s: %w", obj.GetName(), err)
			}
		}
		if pending {
			return ctrl.Result{RequeueAfter: dependencyRetryInterval}, nil
		}
	}
	return ctrl.Result{}, nil
}

func (r *EVPNInterconnectReconciler) dependenciesToInterconnects(ctx context.Context, obj client.Object) []ctrl.Request {
	interconnects := &nxoverlayv1alpha1.EVPNInterconnectList{}
	if err := r.List(ctx, interconnects, client.InNamespace(obj.GetNamespace())); err != nil {
		ctrl.LoggerFrom(ctx).Error(err, "Failed to list EVPNInterconnects")
		return nil
	}
	requests := make([]ctrl.Request, 0, len(interconnects.Items))
	for i := range interconnects.Items {
		requests = append(requests, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(&interconnects.Items[i])})
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

// interfacesToInterconnects maps an Interface event to the EVPNInterconnects
// that reference it by name in spec.interfaces.
func (r *EVPNInterconnectReconciler) interfacesToInterconnects(ctx context.Context, obj client.Object) []ctrl.Request {
	intf, ok := obj.(*v1alpha1.Interface)
	if !ok {
		panic(fmt.Sprintf("Expected an Interface but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx)

	icList := &nxoverlayv1alpha1.EVPNInterconnectList{}
	if err := r.List(ctx, icList, client.InNamespace(intf.Namespace)); err != nil {
		log.Error(err, "Failed to list EVPNInterconnects")
		return nil
	}

	var requests []ctrl.Request
	for i := range icList.Items {
		ic := &icList.Items[i]
		for j := range ic.Spec.Interfaces {
			if ic.Spec.Interfaces[j].InterfaceRef.Name == intf.Name {
				requests = append(requests, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(ic)})
				break
			}
		}
	}
	return requests
}
