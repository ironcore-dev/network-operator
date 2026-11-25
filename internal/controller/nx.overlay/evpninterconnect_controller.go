// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxoverlay

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

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

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	nxoverlayv1alpha1 "github.com/ironcore-dev/network-operator/api/nx.overlay/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/conditions"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	kerrors "k8s.io/apimachinery/pkg/util/errors"

	poolv1alpha1 "github.com/ironcore-dev/network-operator/api/pool/v1alpha1"
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
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=interfaces,verbs=get;list;watch;patch
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=devices,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the EVPNInterconnect object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.24.1/pkg/reconcile
func (r *EVPNInterconnectReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling resource")

	var reterr error

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
			if err := r.finalize(ctx, ic); err != nil {
				log.Error(err, "Failed to finalize resource")
				return ctrl.Result{}, err
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

func (r *EVPNInterconnectReconciler) finalize(ctx context.Context, ic *nxoverlayv1alpha1.EVPNInterconnect) error {
	_ = ctx
	_ = ic
	return nil
}

func (r *EVPNInterconnectReconciler) reconcile(ctx context.Context, ic *nxoverlayv1alpha1.EVPNInterconnect) (ctrl.Result, error) {
	defer func() {
		// TODO:
		// r.computeUnderlayConverged(ctx, fabric)
		// r.computeOverlayConverged(ctx, fabric)
		// conditions.RecomputeReady(fabric)
	}()

	// TODO

	state := &ReconcileState{
		vteps:      make(map[string][]v1alpha1.NetworkVirtualizationEdge),
		devices:    make(map[string]v1alpha1.Device),
		interfaces: make(map[string]v1alpha1.Interface),
	}

	// The ordering of phases is critical. Each phase may depend on resources
	// provisioned by earlier phases:
	//
	//  1. claimMultiSiteID - claims the Multi-Site ID from the pool
	//  1. reconcileInterfaces - configures addressing and MTU
	//  1. reconcileUnderlay (configures underlay e.g., create OSPF brick)
	//  1. reconcileVTEP - creates a VTEP brick and claims an IP address from the pool
	//  1. reconcileBorderGateway - creates BGW brick (BGP peer configuration)

	// Phases that find their prerequisites missing (e.g. Claims not yet allocated)
	// skip the affected device and return without error. The Owns() watches on
	// Claim and Interface will re-enqueue the Fabric once the dependency appears.
	phases := []ReconcileFunc{
		r.claimMultiSiteID,
		r.reconcileInterfaces,
		r.reconcileUnderlay,
		r.reconcileVTEP,
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
			PoolRef: ic.Spec.MultiSiteIDIndexPoolRef,
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

	state.multiSiteID = claim
	return ctrl.Result{}, nil
}

func (r *EVPNInterconnectReconciler) reconcileInterfaces(ctx context.Context, ic *nxoverlayv1alpha1.EVPNInterconnect, state *ReconcileState) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	seenInterfaces := make(map[string]bool)
	seenDevices := make(map[string]bool)
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

		state.devices[deviceName] = *device
		state.interfaces[interfaceName] = *intf
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

	return ctrl.Result{}, nil
}

func (r *EVPNInterconnectReconciler) reconcileUnderlay(ctx context.Context, ic *nxoverlayv1alpha1.EVPNInterconnect, state *ReconcileState) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

func (r *EVPNInterconnectReconciler) reconcileVTEP(ctx context.Context, ic *nxoverlayv1alpha1.EVPNInterconnect, state *ReconcileState) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

func (r *EVPNInterconnectReconciler) reconcileBorderGateway(ctx context.Context, ic *nxoverlayv1alpha1.EVPNInterconnect, state *ReconcileState) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

// ReconcileFunc defines a function type for reconciliation phases.
// Each phase should return a non-zero Result or an error if it wants to stop the reconciliation loop.
type ReconcileFunc func(context.Context, *nxoverlayv1alpha1.EVPNInterconnect, *ReconcileState) (ctrl.Result, error)

// ReconcileState accumulates EVPN Interconnect state across reconciliation phases
type ReconcileState struct {
	vteps       map[string][]v1alpha1.NetworkVirtualizationEdge // DCI-facing VTEPs in the group
	multiSiteID *poolv1alpha1.Claim                             // Multi-Site ID claimed from index pool
	devices     map[string]v1alpha1.Device
	interfaces  map[string]v1alpha1.Interface
}

// SetupWithManager sets up the controller with the Manager.
func (r *EVPNInterconnectReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&nxoverlayv1alpha1.EVPNInterconnect{}).
		Owns(&poolv1alpha1.Claim{}).
		// Re-reconcile when a referenced DCI Interface's spec changes so drift is re-converged
		Watches(
			&v1alpha1.Interface{},
			handler.EnqueueRequestsFromMapFunc(r.interfacesToInterconnects),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Named("nx.overlay-evpninterconnect").
		Complete(r)
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
