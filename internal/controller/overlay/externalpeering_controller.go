// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package overlay

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	overlayv1alpha1 "github.com/ironcore-dev/network-operator/api/overlay/v1alpha1"
	poolv1alpha1 "github.com/ironcore-dev/network-operator/api/pool/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/conditions"
)

// ExternalPeeringReconciler reconciles a ExternalPeering object
type ExternalPeeringReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// WatchFilterValue is the label value used to filter events prior to reconciliation.
	WatchFilterValue string

	// Recorder is used to record events for the controller.
	// More info: https://book.kubebuilder.io/reference/raising-events
	Recorder events.EventRecorder
}

// +kubebuilder:rbac:groups=overlay.networking.metal.ironcore.dev,resources=externalpeerings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=overlay.networking.metal.ironcore.dev,resources=externalpeerings/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=overlay.networking.metal.ironcore.dev,resources=externalpeerings/finalizers,verbs=update
// +kubebuilder:rbac:groups=overlay.networking.metal.ironcore.dev,resources=routingdomains,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=devices,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=interfaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=bgp,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=bgppeers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=vrfs,verbs=get;list;watch
// +kubebuilder:rbac:groups=pool.networking.metal.ironcore.dev,resources=claims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.24.1/pkg/reconcile
func (r *ExternalPeeringReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling resource")

	p := new(overlayv1alpha1.ExternalPeering)
	if err := r.Get(ctx, req.NamespacedName, p); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get resource")
		return ctrl.Result{}, err
	}

	if !p.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(p, overlayv1alpha1.FinalizerName) {
			if err := r.finalize(ctx, p); err != nil {
				log.Error(err, "Failed to finalize resource")
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(p, overlayv1alpha1.FinalizerName)
			if err := r.Update(ctx, p); err != nil {
				log.Error(err, "Failed to remove finalizer from resource")
				return ctrl.Result{}, err
			}
		}
		log.Info("Resource is being deleted, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers
	if !controllerutil.ContainsFinalizer(p, overlayv1alpha1.FinalizerName) {
		controllerutil.AddFinalizer(p, overlayv1alpha1.FinalizerName)
		if err := r.Update(ctx, p); err != nil {
			log.Error(err, "Failed to add finalizer to resource")
			return ctrl.Result{}, err
		}
		log.Info("Added finalizer to resource")
		return ctrl.Result{}, nil
	}

	orig := p.DeepCopy()
	if conditions.InitializeConditions(p, v1alpha1.ReadyCondition) {
		log.V(1).Info("Initializing status conditions")
		return ctrl.Result{}, r.Status().Update(ctx, p)
	}

	defer func() {
		if !equality.Semantic.DeepEqual(orig.ObjectMeta, p.ObjectMeta) {
			// Pass obj.DeepCopy() to avoid Patch() modifying obj and interfering with status update below
			if err := r.Patch(ctx, p.DeepCopy(), client.MergeFrom(orig)); err != nil {
				log.Error(err, "Failed to update resource metadata")
				reterr = kerrors.NewAggregate([]error{reterr, err})
			}
		}
		if !equality.Semantic.DeepEqual(orig.Status, p.Status) {
			if err := r.Status().Patch(ctx, p, client.MergeFrom(orig)); err != nil {
				log.Error(err, "Failed to update status")
				reterr = kerrors.NewAggregate([]error{reterr, err})
			}
		}
	}()

	res, err := r.reconcile(ctx, p)
	if err != nil {
		log.Error(err, "Failed to reconcile resource")
		return ctrl.Result{}, err
	}

	return res, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ExternalPeeringReconciler) SetupWithManager(mgr ctrl.Manager) error {
	labelSelector := metav1.LabelSelector{}
	if r.WatchFilterValue != "" {
		labelSelector.MatchLabels = map[string]string{v1alpha1.WatchLabel: r.WatchFilterValue}
	}

	filter, err := predicate.LabelSelectorPredicate(labelSelector)
	if err != nil {
		return fmt.Errorf("failed to create label selector predicate: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&overlayv1alpha1.ExternalPeering{}).
		Owns(&poolv1alpha1.Claim{}, builder.WithPredicates(predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldClaim := e.ObjectOld.(*poolv1alpha1.Claim)
				newClaim := e.ObjectNew.(*poolv1alpha1.Claim)
				return !equality.Semantic.DeepEqual(oldClaim.Status, newClaim.Status)
			},
			GenericFunc: func(event.GenericEvent) bool { return false },
		})).
		Owns(&v1alpha1.Interface{}).
		Owns(&v1alpha1.BGP{}).
		Owns(&v1alpha1.BGPPeer{}).
		Watches(
			&v1alpha1.Device{},
			handler.EnqueueRequestsFromMapFunc(r.devicesToExternalPeerings),
			builder.WithPredicates(predicate.LabelChangedPredicate{}),
		).
		Named("overlay-externalpeering").
		WithEventFilter(filter).
		Complete(r)
}

func (r *ExternalPeeringReconciler) reconcile(ctx context.Context, peering *overlayv1alpha1.ExternalPeering) (ctrl.Result, error) {
	defer conditions.RecomputeReady(peering)

	rd := new(overlayv1alpha1.RoutingDomain)
	if err := r.Get(ctx, client.ObjectKey{Namespace: peering.Namespace, Name: peering.Spec.RoutingDomainRef.Name}, rd); err != nil {
		return ctrl.Result{}, fmt.Errorf("get routing domain %q: %w", peering.Spec.RoutingDomainRef.Name, err)
	}

	selector, err := metav1.LabelSelectorAsSelector(&peering.Spec.DeviceSelector)
	if err != nil {
		return ctrl.Result{}, reconcile.TerminalError(fmt.Errorf("invalid deviceSelector: %w", err))
	}
	devices := &v1alpha1.DeviceList{}
	if err := r.List(ctx, devices, client.InNamespace(peering.Namespace), client.MatchingLabelsSelector{Selector: selector}); err != nil {
		return ctrl.Result{}, fmt.Errorf("listing devices: %w", err)
	}

	// Range by index only — list.Items are value types; using the second loop
	// variable would copy the entire struct on each iteration.
	for i := range devices.Items {
		claimName := fmt.Sprintf("%s-%s-%s", peering.Name, devices.Items[i].Name, peering.Spec.Peering.SourceAddress.Name)
		claim, err := r.reconcileLoopbackClaim(ctx, peering, claimName)
		if err != nil {
			return ctrl.Result{}, err
		}
		intf, err := r.reconcileLoopbackInterface(ctx, peering, &devices.Items[i], peering.Spec.Peering.SourceAddress.Name, claim)
		if err != nil {
			return ctrl.Result{}, err
		}
		if intf == nil {
			// Interface not yet ready, skip to the next device.
			continue
		}

		// TODO: Consider caching VRFs per device to avoid repeated API calls.
		vrfs := &v1alpha1.VRFList{}
		if err := r.List(ctx, vrfs, client.InNamespace(rd.Namespace), client.MatchingFields{v1alpha1.DeviceRefIndexKey: devices.Items[i].Name, v1alpha1.ControllerOwnerUIDKey: string(rd.UID)}); err != nil {
			return ctrl.Result{}, reconcile.TerminalError(fmt.Errorf("list VRFs controlled by RoutingDomain %q on device %q: %w", rd.Name, devices.Items[i].Name, err))
		}
		if len(vrfs.Items) == 0 {
			return ctrl.Result{}, reconcile.TerminalError(fmt.Errorf("no VRFs found for RoutingDomain %q on device %q", rd.Name, devices.Items[i].Name))
		}
		if len(vrfs.Items) > 1 {
			return ctrl.Result{}, reconcile.TerminalError(fmt.Errorf("multiple VRFs found for RoutingDomain %q on device %q", rd.Name, devices.Items[i].Name))
		}
		vrf := &vrfs.Items[0]

		bgp, err := r.reconcileBGP(ctx, peering, &devices.Items[i], vrf)
		if err != nil {
			return ctrl.Result{}, err
		}

		if err := r.reconcileBGPPeering(ctx, peering, &devices.Items[i], bgp, intf); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// reconcileLoopbackClaim ensures a Claim with the given name exists and matches the desired spec.
// Returns the Claim object so callers can pass it directly to reconcileLoopbackInterface.
func (r *ExternalPeeringReconciler) reconcileLoopbackClaim(ctx context.Context, peering *overlayv1alpha1.ExternalPeering, claimName string) (*poolv1alpha1.Claim, error) {
	claim := &poolv1alpha1.Claim{
		Name:      claimName,
		Namespace: peering.Namespace,
	}
	res, err := controllerutil.CreateOrPatch(ctx, r.Client, claim, func() error {
		if claim.Labels == nil {
			claim.Labels = make(map[string]string)
		}
		claim.Labels[overlayv1alpha1.ExternalPeeringLabel] = peering.Name
		claim.Spec = poolv1alpha1.ClaimSpec{
			PoolRef: v1alpha1.TypedLocalObjectReference{
				APIVersion: poolv1alpha1.GroupVersion.String(),
				Kind:       "IPAddressPool",
				Name:       peering.Spec.Peering.SourceAddress.IPAddressPoolRef.Name,
			},
		}
		return controllerutil.SetControllerReference(peering, claim, r.Scheme)
	})
	if err != nil {
		return nil, fmt.Errorf("reconciling claim %s: %w", claimName, err)
	}
	if res == controllerutil.OperationResultCreated {
		r.Recorder.Eventf(peering, nil, "Normal", "ClaimCreated", "Reconcile", "Created loopback address claim %s", claimName)
	}
	return claim, nil
}

// reconcileLoopbackInterface creates or updates the Interface for a given device loopback
// once its Claim is allocated. Returns nil if the claim is not yet allocated; the Owns() watch
// on Claim will re-enqueue this Fabric when the pool controller updates the claim status.
func (r *ExternalPeeringReconciler) reconcileLoopbackInterface(ctx context.Context, peering *overlayv1alpha1.ExternalPeering, device *v1alpha1.Device, handle string, claim *poolv1alpha1.Claim) (*v1alpha1.Interface, error) {
	cond := conditions.Get(claim, poolv1alpha1.AllocatedCondition)
	if cond == nil || cond.Status != metav1.ConditionTrue || claim.Status.Value == "" {
		return nil, nil //nolint:nilnil // claim not yet allocated; Owns() watch will re-enqueue
	}

	prefix, err := v1alpha1.ParsePrefix(claim.Status.Value + "/32")
	if err != nil {
		return nil, reconcile.TerminalError(fmt.Errorf("parsing allocated address %q: %w", claim.Status.Value, err))
	}

	name := fmt.Sprintf("%s-%s-%s", peering.Name, device.Name, handle)
	intf := &v1alpha1.Interface{
		Name:      name,
		Namespace: peering.Namespace,
	}
	res, err := controllerutil.CreateOrPatch(ctx, r.Client, intf, func() error {
		if intf.Labels == nil {
			intf.Labels = make(map[string]string)
		}
		intf.Labels[overlayv1alpha1.ExternalPeeringLabel] = peering.Name
		intf.Spec.DeviceRef = v1alpha1.LocalObjectReference{Name: device.Name}
		intf.Spec.Name = handle
		intf.Spec.Type = v1alpha1.InterfaceTypeLoopback
		intf.Spec.AdminState = v1alpha1.AdminStateUp
		intf.Spec.Description = "External Peering"
		if intf.Spec.IPv4 == nil {
			intf.Spec.IPv4 = &v1alpha1.InterfaceIPv4{}
		}
		if len(intf.Spec.IPv4.Addresses) == 0 || intf.Spec.IPv4.Addresses[0] != prefix {
			intf.Spec.IPv4.Addresses = []v1alpha1.IPPrefix{prefix}
		}
		return controllerutil.SetControllerReference(peering, intf, r.Scheme)
	})
	if err != nil {
		return nil, fmt.Errorf("reconciling interface %s: %w", name, err)
	}
	if res == controllerutil.OperationResultCreated {
		r.Recorder.Eventf(peering, nil, "Normal", "InterfaceCreated", "Reconcile", "Created loopback interface %s", name)
	}
	return intf, nil
}

// reconcileBGP creates or updates the BGP instance for the external peering on the given device.
func (r *ExternalPeeringReconciler) reconcileBGP(ctx context.Context, peering *overlayv1alpha1.ExternalPeering, device *v1alpha1.Device, vrf *v1alpha1.VRF) (*v1alpha1.BGP, error) {
	bgps := &v1alpha1.BGPList{}
	if err := r.List(ctx, bgps, client.InNamespace(peering.Namespace), client.MatchingFields{v1alpha1.DeviceRefIndexKey: device.Name}); err != nil {
		return nil, reconcile.TerminalError(fmt.Errorf("list BGP instances on device %q: %w", device.Name, err))
	}

	// Find the bgp instance in the default vrf (vrfRef == nil).
	var defaultBGPInstance *v1alpha1.BGP
	for i := range bgps.Items {
		if bgps.Items[i].Spec.VrfRef == nil {
			if defaultBGPInstance != nil {
				return nil, fmt.Errorf("multiple default BGP routers found on device %q", device.Name)
			}
			defaultBGPInstance = &bgps.Items[i]
		}
	}
	if defaultBGPInstance == nil {
		return nil, reconcile.TerminalError(fmt.Errorf("no default BGP router in default vrf found on device %q", device.Name))
	}

	name := fmt.Sprintf("%s-%s", peering.Name, device.Name)
	bgp := &v1alpha1.BGP{
		Name:      name,
		Namespace: peering.Namespace,
	}

	res, err := controllerutil.CreateOrPatch(ctx, r.Client, bgp, func() error {
		if bgp.Labels == nil {
			bgp.Labels = make(map[string]string)
		}
		bgp.Labels[overlayv1alpha1.ExternalPeeringLabel] = peering.Name
		bgp.Spec.DeviceRef = v1alpha1.LocalObjectReference{Name: device.Name}
		bgp.Spec.VrfRef = &v1alpha1.LocalObjectReference{Name: vrf.Name}
		bgp.Spec.AdminState = v1alpha1.AdminStateUp
		bgp.Spec.ASNumber = peering.Spec.Peering.LocalASNumber
		bgp.Spec.RouterID = defaultBGPInstance.Spec.RouterID
		return controllerutil.SetControllerReference(peering, bgp, r.Scheme)
	})
	if err != nil {
		return nil, fmt.Errorf("reconciling BGP instance %s: %w", name, err)
	}
	if res == controllerutil.OperationResultCreated {
		r.Recorder.Eventf(peering, nil, "Normal", "BGPCreated", "Reconcile", "Created BGP instance %s", name)
	}
	return bgp, nil
}

// reconcileBGPPeering creates or updates the BGP peer for the external peering on the given device.
func (r *ExternalPeeringReconciler) reconcileBGPPeering(ctx context.Context, peering *overlayv1alpha1.ExternalPeering, device *v1alpha1.Device, bgp *v1alpha1.BGP, intf *v1alpha1.Interface) error {
	name := fmt.Sprintf("%s-%s", peering.Name, device.Name)
	peer := &v1alpha1.BGPPeer{
		Name:      name,
		Namespace: peering.Namespace,
	}

	if peering.Spec.Peering.ListenRange.Bits() != 32 {
		return reconcile.TerminalError(fmt.Errorf("listenRange %s is not a /32; dynamic listen ranges are not yet supported", peering.Spec.Peering.ListenRange))
	}

	res, err := controllerutil.CreateOrPatch(ctx, r.Client, peer, func() error {
		if peer.Labels == nil {
			peer.Labels = make(map[string]string)
		}
		peer.Labels[overlayv1alpha1.ExternalPeeringLabel] = peering.Name
		peer.Spec.DeviceRef = v1alpha1.LocalObjectReference{Name: device.Name}
		peer.Spec.BgpRef = v1alpha1.LocalObjectReference{Name: bgp.Name}
		peer.Spec.AdminState = v1alpha1.AdminStateUp
		peer.Spec.Address = peering.Spec.Peering.ListenRange.Addr().String()
		peer.Spec.ASNumber = peering.Spec.Peering.RemoteASNumber
		peer.Spec.Description = "External Peering"
		peer.Spec.LocalAddress = &v1alpha1.BGPPeerLocalAddress{InterfaceRef: v1alpha1.LocalObjectReference{Name: intf.Name}}
		peer.Spec.LocalAS = peering.Spec.Peering.LocalAS
		peer.Spec.TTL = peering.Spec.Peering.TTL
		peer.Spec.BFD = peering.Spec.Peering.BFD
		return controllerutil.SetControllerReference(peering, peer, r.Scheme)
	})
	if err != nil {
		return fmt.Errorf("reconciling BGP peer %s: %w", name, err)
	}
	if res == controllerutil.OperationResultCreated {
		r.Recorder.Eventf(peering, nil, "Normal", "BGPPeerCreated", "Reconcile", "Created BGP peer %s", name)
	}
	return nil
}

// devicesToExternalPeerings is a [handler.MapFunc] that enqueues all
// ExternalPeerings whose spec.deviceSelector matches the changed Device.
func (r *ExternalPeeringReconciler) devicesToExternalPeerings(ctx context.Context, obj client.Object) []ctrl.Request {
	device, ok := obj.(*v1alpha1.Device)
	if !ok {
		panic(fmt.Sprintf("Expected a Device but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx)

	list := &overlayv1alpha1.ExternalPeeringList{}
	if err := r.List(ctx, list, client.InNamespace(device.Namespace)); err != nil {
		log.Error(err, "Failed to list ExternalPeerings")
		return nil
	}

	var requests []ctrl.Request
	for _, ep := range list.Items {
		selector, err := metav1.LabelSelectorAsSelector(&ep.Spec.DeviceSelector)
		if err != nil {
			log.Error(err, "Failed to parse deviceSelector", "externalPeering", ep.Name)
			continue
		}
		if selector.Matches(labels.Set(device.Labels)) {
			requests = append(requests, ctrl.Request{
				NamespacedName: client.ObjectKeyFromObject(&ep),
			})
		}
	}
	return requests
}

func (r *ExternalPeeringReconciler) finalize(_ context.Context, _ *overlayv1alpha1.ExternalPeering) error {
	return nil
}
