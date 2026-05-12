// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package evpn

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
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
// +kubebuilder:rbac:groups=pool.networking.metal.ironcore.dev,resources=claims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=pool.networking.metal.ironcore.dev,resources=ipaddresspools,verbs=get;list;watch
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
		// Re-reconcile when a Device's labels change so that devices newly
		// matching a deviceSelector are enrolled into the fabric.
		Watches(
			&v1alpha1.Device{},
			handler.EnqueueRequestsFromMapFunc(r.devicesToFabrics),
			builder.WithPredicates(predicate.LabelChangedPredicate{}),
		).
		WithEventFilter(filter).
		Named("evpn-fabric").
		Complete(r)
}

// ReconcileFunc defines a function type for reconciliation phases.
// Each phase should return a non-zero Result or an error if it wants to stop the reconciliation loop.
type ReconcileFunc func(context.Context, *evpnv1alpha1.Fabric) (ctrl.Result, error)

func (r *FabricReconciler) reconcile(ctx context.Context, fabric *evpnv1alpha1.Fabric) (ctrl.Result, error) {
	phases := []ReconcileFunc{
		r.reconcileSystemLoopbacks,
		r.reconcileVTEPLoopbacks,
		r.reconcileAnycastRPLoopbacks,
	}
	for _, phase := range phases {
		res, err := phase(ctx, fabric)
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
func (r *FabricReconciler) reconcileSystemLoopbacks(ctx context.Context, fabric *evpnv1alpha1.Fabric) (ctrl.Result, error) {
	selector, err := metav1.LabelSelectorAsSelector(&fabric.Spec.DeviceSelector)
	if err != nil {
		return ctrl.Result{}, reconcile.TerminalError(fmt.Errorf("invalid deviceSelector: %w", err))
	}
	devices := &v1alpha1.DeviceList{}
	if err := r.List(ctx, devices, client.InNamespace(fabric.Namespace), client.MatchingLabelsSelector{Selector: selector}); err != nil {
		return ctrl.Result{}, fmt.Errorf("listing all fabric devices for system loopbacks: %w", err)
	}
	for i := range devices.Items {
		claimName := fmt.Sprintf("%s-%s-lo%d", fabric.Name, devices.Items[i].Name, LoopbackRouterID)
		claim, err := r.reconcileLoopbackClaim(ctx, fabric, claimName)
		if err != nil {
			return ctrl.Result{}, err
		}
		if err := r.reconcileLoopbackInterface(ctx, fabric, &devices.Items[i], LoopbackRouterID, claim); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

// reconcileVTEPLoopbacks ensures lo1 (primary VTEP) and lo2 (anycast VTEP) exist on VTEP devices.
func (r *FabricReconciler) reconcileVTEPLoopbacks(ctx context.Context, fabric *evpnv1alpha1.Fabric) (ctrl.Result, error) {
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
			if err := r.reconcileLoopbackInterface(ctx, fabric, &devices.Items[i], id, claim); err != nil {
				return ctrl.Result{}, err
			}
		}
	}
	return ctrl.Result{}, nil
}

// reconcileAnycastRPLoopbacks ensures lo100 (PIM anycast RP) exists on RP devices.
// One claim is allocated per AnycastRendezvousPoint group; all RP devices in the group
// share that single address.
func (r *FabricReconciler) reconcileAnycastRPLoopbacks(ctx context.Context, fabric *evpnv1alpha1.Fabric) (ctrl.Result, error) {
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
			if err := r.reconcileLoopbackInterface(ctx, fabric, &devices.Items[i], LoopbackAnycastRP, claim); err != nil {
				return ctrl.Result{}, err
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
// once its Claim is allocated. A no-op if the claim is not yet allocated; the Owns() watch
// on Claim will re-enqueue this Fabric when the pool controller updates the claim status.
func (r *FabricReconciler) reconcileLoopbackInterface(ctx context.Context, fabric *evpnv1alpha1.Fabric, device *v1alpha1.Device, loopbackID int, claim *poolv1alpha1.Claim) error {
	cond := conditions.Get(claim, poolv1alpha1.AllocatedCondition)
	if cond == nil || cond.Status != metav1.ConditionTrue || claim.Status.Value == "" {
		return nil
	}

	prefix, err := v1alpha1.ParsePrefix(claim.Status.Value + "/32")
	if err != nil {
		return reconcile.TerminalError(fmt.Errorf("parsing allocated address %q: %w", claim.Status.Value, err))
	}

	handle, err := r.Provider().(provider.InterfaceProvider).LoopbackInterfaceName(loopbackID)
	if err != nil {
		return reconcile.TerminalError(fmt.Errorf("resolving loopback interface name for id %d: %w", loopbackID, err))
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
		return fmt.Errorf("reconciling interface %s: %w", name, err)
	}
	if res == controllerutil.OperationResultCreated {
		r.Recorder.Eventf(fabric, nil, "Normal", "InterfaceCreated", "Reconcile", "Created loopback interface %s", name)
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
