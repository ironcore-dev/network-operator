// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package evpn

import (
	"context"
	"fmt"

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
	"github.com/ironcore-dev/network-operator/internal/deviceutil"
	"github.com/ironcore-dev/network-operator/internal/evpnutil"
)

// RoutingDomainReconciler reconciles a RoutingDomain object
type RoutingDomainReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// WatchFilterValue is the label value used to filter events prior to reconciliation.
	WatchFilterValue string

	// Recorder is used to record events for the controller.
	// More info: https://book.kubebuilder.io/reference/raising-events
	Recorder events.EventRecorder
}

// +kubebuilder:rbac:groups=overlay.networking.metal.ironcore.dev,resources=routingdomains,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=overlay.networking.metal.ironcore.dev,resources=routingdomains/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=overlay.networking.metal.ironcore.dev,resources=routingdomains/finalizers,verbs=update
// +kubebuilder:rbac:groups=overlay.networking.metal.ironcore.dev,resources=networks,verbs=get;list;watch
// +kubebuilder:rbac:groups=overlay.networking.metal.ironcore.dev,resources=networkattachments,verbs=get;list;watch
// +kubebuilder:rbac:groups=pool.networking.metal.ironcore.dev,resources=claims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=evpn.networking.metal.ironcore.dev,resources=fabrics,verbs=get;list;watch
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.24.1/pkg/reconcile
func (r *RoutingDomainReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling resource")

	rd := &overlayv1alpha1.RoutingDomain{}
	if err := r.Get(ctx, req.NamespacedName, rd); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get resource")
		return ctrl.Result{}, err
	}

	if !rd.DeletionTimestamp.IsZero() {
		log.Info("Resource is being deleted, skipping reconciliation")
		if controllerutil.ContainsFinalizer(rd, evpnv1alpha1.FinalizerName) {
			if err := r.finalize(ctx, rd); err != nil {
				log.Error(err, "Failed to finalize resource")
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(rd, evpnv1alpha1.FinalizerName)
			if err := r.Update(ctx, rd); err != nil {
				log.Error(err, "Failed to remove finalizer from resource")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers
	if !controllerutil.ContainsFinalizer(rd, evpnv1alpha1.FinalizerName) {
		controllerutil.AddFinalizer(rd, evpnv1alpha1.FinalizerName)
		if err := r.Update(ctx, rd); err != nil {
			log.Error(err, "Failed to add finalizer to resource")
			return ctrl.Result{}, err
		}
		log.Info("Added finalizer to resource")
		return ctrl.Result{}, nil
	}

	orig := rd.DeepCopy()
	if conditions.InitializeConditions(rd, v1alpha1.ReadyCondition) {
		log.V(1).Info("Initializing status conditions")
		return ctrl.Result{}, r.Status().Update(ctx, rd)
	}

	defer func() {
		if !equality.Semantic.DeepEqual(orig.Status, rd.Status) {
			// Pass obj.DeepCopy() to avoid Patch() modifying obj and interfering with metadata update below
			if err := r.Status().Patch(ctx, rd.DeepCopy(), client.MergeFrom(orig)); err != nil {
				log.Error(err, "Failed to update status")
				reterr = kerrors.NewAggregate([]error{reterr, err})
			}
		}
		if !equality.Semantic.DeepEqual(orig.ObjectMeta, rd.ObjectMeta) {
			if err := r.Patch(ctx, rd, client.MergeFrom(orig)); err != nil {
				log.Error(err, "Failed to update resource metadata")
				reterr = kerrors.NewAggregate([]error{reterr, err})
			}
		}
	}()

	if err := r.reconcile(ctx, rd); err != nil {
		log.Error(err, "Failed to reconcile resource")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *RoutingDomainReconciler) reconcile(ctx context.Context, rd *overlayv1alpha1.RoutingDomain) error {
	devices, err := r.resolveDevices(ctx, rd)
	if err != nil {
		return fmt.Errorf("resolve devices for routing domain %q: %w", rd.Name, err)
	}

	if len(devices) == 0 {
		conditions.Set(rd, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  evpnv1alpha1.NoResourcesReason,
			Message: "RoutingDomain is not in use by any Network or no Devices are selected",
		})
		return r.deleteStaleResources(ctx, rd, nil)
	}

	fabric, err := r.resolveFabric(ctx, rd, devices)
	if err != nil {
		return fmt.Errorf("resolve fabric for routing domain %q: %w", rd.Name, err)
	}

	vniClaim, err := r.reconcileVNIClaim(ctx, rd, fabric)
	if err != nil {
		return fmt.Errorf("reconcile VNI claim for routing domain %q: %w", rd.Name, err)
	}
	if vniClaim == nil { // binding conflict or waiting for allocation
		return nil
	}

	vni, routeTarget, err := evpnutil.ComputeVNIRouteTarget(rd, fabric, vniClaim)
	if err != nil {
		return fmt.Errorf("compute VNI route target: %w", err)
	}

	if err := r.reconcileRoutingContext(ctx, rd, devices, fabric, vni, routeTarget); err != nil {
		return fmt.Errorf("reconcile routing context for routing domain %q: %w", rd.Name, err)
	}

	if err := r.deleteStaleResources(ctx, rd, devices); err != nil {
		return fmt.Errorf("delete stale resources for routing domain %q: %w", rd.Name, err)
	}

	conditions.Set(rd, metav1.Condition{
		Type:    v1alpha1.ReadyCondition,
		Status:  metav1.ConditionTrue,
		Reason:  v1alpha1.ReadyReason,
		Message: "Routing domain realized",
	})

	return nil
}

// resolveDevices returns the deduplicated set of Devices selected by the NetworkAttachments of all Networks that reference the given RoutingDomain.
func (r *RoutingDomainReconciler) resolveDevices(ctx context.Context, rd *overlayv1alpha1.RoutingDomain) (map[string]*v1alpha1.Device, error) {
	log := ctrl.LoggerFrom(ctx)
	networks := &overlayv1alpha1.NetworkList{}
	if err := r.List(ctx,
		networks,
		client.InNamespace(rd.Namespace),
		client.MatchingFields{networkRoutingDomainRefIndexKey: rd.Name},
	); err != nil {
		return nil, fmt.Errorf("list Networks for routing domain %q: %w", rd.Name, err)
	}
	devices := map[string]*v1alpha1.Device{}
	for n := range networks.Items {
		net := &networks.Items[n]
		attachments := &overlayv1alpha1.NetworkAttachmentList{}
		if err := r.List(ctx,
			attachments,
			client.InNamespace(net.Namespace),
			client.MatchingFields{networkAttachmentNetworkRefKey: net.Name},
		); err != nil {
			return nil, fmt.Errorf("list NetworkAttachments for network %q: %w", net.Name, err)
		}

		for a := range attachments.Items {
			na := &attachments.Items[a]
			if !na.DeletionTimestamp.IsZero() {
				continue
			}
			intfs := &v1alpha1.InterfaceList{}
			selector, err := metav1.LabelSelectorAsSelector(&na.Spec.InterfaceSelector)
			if err != nil {
				return nil, reconcile.TerminalError(fmt.Errorf("parse InterfaceSelector for NetworkAttachment %q: %w", na.Name, err))
			}
			if err := r.List(
				ctx,
				intfs,
				client.InNamespace(na.Namespace),
				client.MatchingLabelsSelector{Selector: selector},
			); err != nil {
				return nil, fmt.Errorf("list Interfaces for NetworkAttachment %q: %w", na.Name, err)
			}
			if len(intfs.Items) == 0 {
				log.Info("No Interfaces found for NetworkAttachment", "NetworkAttachment", na.Name, "InterfaceSelector", na.Spec.InterfaceSelector)
				continue
			}

			for i := range intfs.Items {
				name := intfs.Items[i].Spec.DeviceRef.Name
				if _, ok := devices[name]; ok {
					continue
				}
				device := &v1alpha1.Device{}
				if err := r.Get(ctx, client.ObjectKey{Name: name, Namespace: na.Namespace}, device); err != nil {
					return nil, fmt.Errorf("get Device %q for NetworkAttachment %q: %w", name, na.Name, err)
				}
				devices[name] = device
			}
		}
	}
	return devices, nil
}

func (r *RoutingDomainReconciler) resolveFabric(ctx context.Context, rd *overlayv1alpha1.RoutingDomain, devices map[string]*v1alpha1.Device) (*evpnv1alpha1.Fabric, error) {
	fabrics := &evpnv1alpha1.FabricList{}
	if err := r.List(ctx, fabrics, client.InNamespace(rd.Namespace)); err != nil {
		return nil, fmt.Errorf("list fabrics: %w", err)
	}

	fabric, err := deviceutil.SelectFabric(devices, fabrics.Items)
	if err != nil {
		conditions.Set(rd, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  evpnv1alpha1.FabricResolutionFailedReason,
			Message: err.Error(),
		})
		return nil, reconcile.TerminalError(err)
	}

	if fabric.Spec.Overlay.IBGP == nil {
		conditions.Set(rd, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  evpnv1alpha1.UnsupportedASNumberReason,
			Message: fmt.Sprintf("Fabric %q has no iBGP configuration", fabric.Name),
		})
		return nil, reconcile.TerminalError(fmt.Errorf("fabric %q has no iBGP configuration", fabric.Name))
	}
	return fabric, nil
}

func (r *RoutingDomainReconciler) reconcileVNIClaim(ctx context.Context, rd *overlayv1alpha1.RoutingDomain, fabric *evpnv1alpha1.Fabric) (*poolv1alpha1.Claim, error) {
	claim := &poolv1alpha1.Claim{
		Name:      "vni-" + hash(rd),
		Namespace: rd.Namespace,
	}

	bindingConflict := false
	_, err := controllerutil.CreateOrPatch(ctx, r.Client, claim, func() error {
		if owner := metav1.GetControllerOf(claim); owner != nil && (owner.APIVersion != evpnv1alpha1.GroupVersion.String() || owner.Kind != "Fabric" || owner.UID != fabric.UID) {
			bindingConflict = true
			return fmt.Errorf("VNI claim %q is already bound to fabric %q", claim.Name, owner.Name)
		}
		if claim.Labels == nil {
			claim.Labels = make(map[string]string)
		}
		claim.Labels[evpnv1alpha1.FabricLabel] = fabric.Name
		claim.Labels[evpnv1alpha1.RoutingDomainLabel] = rd.Name
		claim.Spec.PoolRef = fabric.Spec.VNIPool
		if err := controllerutil.SetOwnerReference(rd, claim, r.Scheme); err != nil {
			return err
		}
		return controllerutil.SetControllerReference(fabric, claim, r.Scheme)
	})
	if err != nil {
		if bindingConflict {
			conditions.Set(rd, metav1.Condition{
				Type:    v1alpha1.ReadyCondition,
				Status:  metav1.ConditionFalse,
				Reason:  evpnv1alpha1.StretchedNetworkNotSupportedReason,
				Message: err.Error(),
			})
			return nil, nil //nolint:nilnil // The condition reports the binding conflict; no Claim is ready to use.
		}
		return nil, fmt.Errorf("reconcile VNI claim %q: %w", claim.Name, err)
	}

	allocated := conditions.Get(claim, poolv1alpha1.AllocatedCondition)
	if allocated == nil || allocated.Status != metav1.ConditionTrue || claim.Status.Value == "" || !claim.DeletionTimestamp.IsZero() {
		conditions.Set(rd, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.WaitingForDependenciesReason,
			Message: fmt.Sprintf("Waiting for VNI claim %q to be allocated", claim.Name),
		})
		return nil, nil //nolint:nilnil // Allocation is pending, not an error; the Claim watch retriggers reconciliation.
	}

	return claim, nil
}

func (r *RoutingDomainReconciler) reconcileRoutingContext(ctx context.Context, rd *overlayv1alpha1.RoutingDomain, devices map[string]*v1alpha1.Device, fabric *evpnv1alpha1.Fabric, vni uint64, routeTarget string) error {
	for _, device := range devices {
		prefix := "routing-" + hash(rd, device)
		vrf := &v1alpha1.VRF{Name: prefix + "-vrf", Namespace: rd.Namespace}
		if _, err := controllerutil.CreateOrPatch(ctx, r.Client, vrf, func() error {
			if vrf.Labels == nil {
				vrf.Labels = make(map[string]string)
			}
			vrf.Labels[evpnv1alpha1.RoutingDomainLabel] = rd.Name
			vrf.Labels[evpnv1alpha1.FabricLabel] = fabric.Name

			vrf.Spec.Name = rd.Spec.Name
			vrf.Spec.DeviceRef = v1alpha1.LocalObjectReference{Name: device.Name}
			vrf.Spec.RouteDistinguisher = "Auto"
			vrf.Spec.RouteTargets = []v1alpha1.RouteTarget{
				{
					Value:           routeTarget,
					AddressFamilies: []v1alpha1.RouteTargetAF{v1alpha1.IPv4, v1alpha1.IPv4EVPN},
					Action:          v1alpha1.RouteTargetActionBoth,
				},
			}
			return controllerutil.SetControllerReference(rd, vrf, r.Scheme)
		}); err != nil {
			return fmt.Errorf("reconcile VRF %q for routing domain %q: %w", vrf.Name, rd.Name, err)
		}

		evi := &v1alpha1.EVPNInstance{Name: prefix + "-evi", Namespace: rd.Namespace}
		if _, err := controllerutil.CreateOrPatch(ctx, r.Client, evi, func() error {
			if evi.Labels == nil {
				evi.Labels = make(map[string]string)
			}
			evi.Labels[evpnv1alpha1.RoutingDomainLabel] = rd.Name
			evi.Labels[evpnv1alpha1.FabricLabel] = fabric.Name

			evi.Spec.DeviceRef = v1alpha1.LocalObjectReference{Name: device.Name}
			evi.Spec.Type = v1alpha1.EVPNInstanceTypeRouted
			evi.Spec.VNI = int32(vni) // #nosec G115 -- VNI validation bounds the value to int32.
			evi.Spec.VRFRef = &v1alpha1.LocalObjectReference{Name: vrf.Name}
			return controllerutil.SetControllerReference(rd, evi, r.Scheme)
		}); err != nil {
			return fmt.Errorf("reconcile EVPN instance %q for routing domain %q: %w", evi.Name, rd.Name, err)
		}
	}
	return nil
}

func (r *RoutingDomainReconciler) deleteStaleResources(ctx context.Context, rd *overlayv1alpha1.RoutingDomain, devices map[string]*v1alpha1.Device) error {
	vrfRefs := sets.New[string]()

	evis := &v1alpha1.EVPNInstanceList{}
	if err := r.List(ctx, evis, client.InNamespace(rd.Namespace), client.MatchingFields{controllerOwnerUIDKey: string(rd.UID)}); err != nil {
		return fmt.Errorf("list EVPN instances controlled by RoutingDomain %q: %w", rd.Name, err)
	}
	for i := range evis.Items {
		evi := &evis.Items[i]
		if !metav1.IsControlledBy(evi, rd) {
			continue
		}
		if _, desired := devices[evi.Spec.DeviceRef.Name]; desired {
			continue
		}
		if evi.Spec.VRFRef != nil {
			vrfRefs.Insert(evi.Spec.VRFRef.Name)
		}
		if evi.DeletionTimestamp.IsZero() {
			if err := client.IgnoreNotFound(r.Delete(ctx, evi)); err != nil {
				return fmt.Errorf("delete EVPN instance %q: %w", evi.Name, err)
			}
		}
	}

	vrfs := &v1alpha1.VRFList{}
	if err := r.List(ctx, vrfs, client.InNamespace(rd.Namespace), client.MatchingFields{controllerOwnerUIDKey: string(rd.UID)}); err != nil {
		return fmt.Errorf("list VRFs controlled by RoutingDomain %q: %w", rd.Name, err)
	}
	for i := range vrfs.Items {
		vrf := &vrfs.Items[i]
		if !metav1.IsControlledBy(vrf, rd) {
			continue
		}
		if _, desired := devices[vrf.Spec.DeviceRef.Name]; desired {
			continue
		}
		if vrfRefs.Has(vrf.Name) {
			continue
		}
		if vrf.DeletionTimestamp.IsZero() {
			if err := client.IgnoreNotFound(r.Delete(ctx, vrf)); err != nil {
				return fmt.Errorf("delete VRF %q: %w", vrf.Name, err)
			}
		}
	}
	return nil
}

func (r *RoutingDomainReconciler) finalize(ctx context.Context, rd *overlayv1alpha1.RoutingDomain) error {
	claim := &poolv1alpha1.Claim{Name: "vni-" + hash(rd), Namespace: rd.Namespace}
	// Claim finalization releases the allocation asynchronously after deletion is requested.
	if err := client.IgnoreNotFound(r.Delete(ctx, claim)); err != nil {
		return fmt.Errorf("delete VNI claim %q: %w", claim.Name, err)
	}
	return nil
}

// networkRoutingDomainRefIndexKey is the index key used to index Network resources by their RoutingDomainRef name.
const networkRoutingDomainRefIndexKey = ".spec.routingDomainRef.name"

// SetupWithManager sets up the controller with the Manager.
func (r *RoutingDomainReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	labelSelector := metav1.LabelSelector{}
	if r.WatchFilterValue != "" {
		labelSelector.MatchLabels = map[string]string{v1alpha1.WatchLabel: r.WatchFilterValue}
	}

	filter, err := predicate.LabelSelectorPredicate(labelSelector)
	if err != nil {
		return fmt.Errorf("create label selector predicate: %w", err)
	}

	if err := mgr.GetFieldIndexer().IndexField(ctx, &overlayv1alpha1.Network{}, networkRoutingDomainRefIndexKey, func(obj client.Object) []string {
		o := obj.(*overlayv1alpha1.Network)
		if o.Spec.RoutingDomainRef == nil {
			return nil
		}
		return []string{o.Spec.RoutingDomainRef.Name}
	}); err != nil {
		return err
	}

	for _, obj := range []client.Object{&poolv1alpha1.Claim{}, &v1alpha1.VRF{}} {
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
		For(&overlayv1alpha1.RoutingDomain{}).
		Named("evpn-routingdomain").
		WithEventFilter(filter).
		Owns(&v1alpha1.EVPNInstance{}).
		Owns(&v1alpha1.VRF{}).
		// Watches enqueues RoutingDomains when NetworkAttachments resolve their forwarding context, change InterfaceSelector or start deletion.
		Watches(
			&overlayv1alpha1.NetworkAttachment{},
			handler.EnqueueRequestsFromMapFunc(r.networkAttachmentToRoutingDomain),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldNA := e.ObjectOld.(*overlayv1alpha1.NetworkAttachment)
					newNA := e.ObjectNew.(*overlayv1alpha1.NetworkAttachment)
					return !equality.Semantic.DeepEqual(oldNA.Status.ForwardingContextRef, newNA.Status.ForwardingContextRef) ||
						!equality.Semantic.DeepEqual(oldNA.DeletionTimestamp, newNA.DeletionTimestamp) ||
						!equality.Semantic.DeepEqual(oldNA.Spec.InterfaceSelector, newNA.Spec.InterfaceSelector)
				},
				GenericFunc: func(event.GenericEvent) bool {
					return false
				},
			}),
		).
		// Watches enqueues RoutingDomains when the Networks referencing them change their RoutingDomainRef or start deletion.
		Watches(
			&overlayv1alpha1.Network{},
			handler.EnqueueRequestsFromMapFunc(r.networkToRoutingDomain),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldNet := e.ObjectOld.(*overlayv1alpha1.Network)
					newNet := e.ObjectNew.(*overlayv1alpha1.Network)
					return !equality.Semantic.DeepEqual(oldNet.Spec.RoutingDomainRef, newNet.Spec.RoutingDomainRef) ||
						!equality.Semantic.DeepEqual(oldNet.DeletionTimestamp, newNet.DeletionTimestamp)
				},
				GenericFunc: func(event.GenericEvent) bool {
					return false
				},
			}),
		).
		// Claims are Fabric-owned, so map their allocation and deletion changes to the RoutingDomain explicitly.
		Watches(
			&poolv1alpha1.Claim{},
			handler.EnqueueRequestsFromMapFunc(r.claimToRoutingDomain),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldClaim := e.ObjectOld.(*poolv1alpha1.Claim)
					newClaim := e.ObjectNew.(*poolv1alpha1.Claim)
					return !equality.Semantic.DeepEqual(oldClaim.Status, newClaim.Status) ||
						!equality.Semantic.DeepEqual(oldClaim.DeletionTimestamp, newClaim.DeletionTimestamp)
				},
				GenericFunc: func(event.GenericEvent) bool {
					return false
				},
			}),
		).
		// Watches enqueues RoutingDomains when the Fabric's DeviceSelector or iBGP configuration changes.
		Watches(
			&evpnv1alpha1.Fabric{},
			handler.EnqueueRequestsFromMapFunc(r.fabricToRoutingDomain),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldFabric := e.ObjectOld.(*evpnv1alpha1.Fabric)
					newFabric := e.ObjectNew.(*evpnv1alpha1.Fabric)
					return !equality.Semantic.DeepEqual(oldFabric.Spec.DeviceSelector, newFabric.Spec.DeviceSelector) ||
						!equality.Semantic.DeepEqual(oldFabric.Spec.Overlay.IBGP, newFabric.Spec.Overlay.IBGP)
				},
				GenericFunc: func(event.GenericEvent) bool { return false },
			}),
		).
		// Watches enqueues RoutingDomains when the Device's labels change, which may affect the Fabric selection.
		Watches(
			&v1alpha1.Device{},
			handler.EnqueueRequestsFromMapFunc(r.deviceToRoutingDomain),
			builder.WithPredicates(predicate.LabelChangedPredicate{}),
		).
		Complete(r)
}

// deviceToRoutingDomain enqueues RoutingDomains whose NetworkAttachments select Interfaces on the changed Device.
func (r *RoutingDomainReconciler) deviceToRoutingDomain(ctx context.Context, obj client.Object) []ctrl.Request {
	device, ok := obj.(*v1alpha1.Device)
	if !ok {
		panic(fmt.Sprintf("Expected a Device but got a %T", obj))
	}
	log := ctrl.LoggerFrom(ctx, "Device", klog.KObj(device))
	interfaces := &v1alpha1.InterfaceList{}
	if err := r.List(ctx, interfaces, client.InNamespace(device.Namespace), client.MatchingFields{v1alpha1.DeviceRefIndexKey: device.Name}); err != nil {
		log.Error(err, "Failed to list Interfaces")
		return nil
	}
	attachments := &overlayv1alpha1.NetworkAttachmentList{}
	if err := r.List(ctx, attachments, client.InNamespace(device.Namespace)); err != nil {
		log.Error(err, "Failed to list NetworkAttachments")
		return nil
	}
	seen := sets.New[ctrl.Request]()
	var requests []ctrl.Request
	for i := range attachments.Items {
		na := &attachments.Items[i]
		selector, err := metav1.LabelSelectorAsSelector(&na.Spec.InterfaceSelector)
		if err != nil {
			log.Error(err, "Failed to parse NetworkAttachment interfaceSelector", "NetworkAttachment", klog.KObj(na))
			continue
		}
		for j := range interfaces.Items {
			if !selector.Matches(labels.Set(interfaces.Items[j].Labels)) {
				continue
			}
			for _, request := range r.networkAttachmentToRoutingDomain(ctx, na) {
				if !seen.Has(request) {
					seen.Insert(request)
					requests = append(requests, request)
				}
			}
			break
		}
	}
	return requests
}

// fabricToRoutingDomain enqueues RoutingDomains using selected Devices or Claims bound to the changed Fabric.
func (r *RoutingDomainReconciler) fabricToRoutingDomain(ctx context.Context, obj client.Object) []ctrl.Request {
	fabric, ok := obj.(*evpnv1alpha1.Fabric)
	if !ok {
		panic(fmt.Sprintf("Expected a Fabric but got a %T", obj))
	}
	log := ctrl.LoggerFrom(ctx, "Fabric", klog.KObj(fabric))
	seen := sets.New[ctrl.Request]()
	var requests []ctrl.Request
	claims := &poolv1alpha1.ClaimList{}
	if err := r.List(ctx, claims, client.InNamespace(fabric.Namespace), client.MatchingFields{controllerOwnerUIDKey: string(fabric.UID)}); err != nil {
		log.Error(err, "Failed to list Claims")
	} else {
		for i := range claims.Items {
			for _, request := range r.claimToRoutingDomain(ctx, &claims.Items[i]) {
				if !seen.Has(request) {
					seen.Insert(request)
					requests = append(requests, request)
				}
			}
		}
	}
	selector, err := metav1.LabelSelectorAsSelector(&fabric.Spec.DeviceSelector)
	if err != nil {
		log.Error(err, "Failed to parse Fabric deviceSelector")
		return requests
	}
	devices := &v1alpha1.DeviceList{}
	if err := r.List(ctx, devices, client.InNamespace(fabric.Namespace), client.MatchingLabelsSelector{Selector: selector}); err != nil {
		log.Error(err, "Failed to list Devices")
		return requests
	}
	for i := range devices.Items {
		for _, request := range r.deviceToRoutingDomain(ctx, &devices.Items[i]) {
			if !seen.Has(request) {
				seen.Insert(request)
				requests = append(requests, request)
			}
		}
	}
	return requests
}

// networkAttachmentToRoutingDomain is a [handler.MapFunc] that enqueues
// the RoutingDomain referenced by Networks referenced by the changed NetworkAttachment.
func (r *RoutingDomainReconciler) networkAttachmentToRoutingDomain(ctx context.Context, obj client.Object) []ctrl.Request {
	na, ok := obj.(*overlayv1alpha1.NetworkAttachment)
	if !ok {
		panic(fmt.Sprintf("Expected a NetworkAttachment but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx, "Network", klog.KObj(na))
	network := &overlayv1alpha1.Network{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: na.Namespace, Name: na.Spec.NetworkRef.Name}, network); err != nil {
		log.Error(err, "Failed to get Network for NetworkAttachment")
		return nil
	}
	if network.Spec.RoutingDomainRef == nil {
		return nil
	}
	log.V(2).Info("Enqueuing RoutingDomain for reconciliation", "name", network.Spec.RoutingDomainRef.Name, "namespace", na.Namespace)
	return []ctrl.Request{{Namespace: na.Namespace, Name: network.Spec.RoutingDomainRef.Name}}
}

func (r *RoutingDomainReconciler) networkToRoutingDomain(ctx context.Context, obj client.Object) []ctrl.Request {
	network, ok := obj.(*overlayv1alpha1.Network)
	if !ok {
		panic(fmt.Sprintf("Expected a Network but got a %T", obj))
	}

	if network.Spec.RoutingDomainRef == nil {
		return nil
	}

	log := ctrl.LoggerFrom(ctx, "Network", klog.KObj(network))
	log.V(2).Info("Enqueuing RoutingDomain for reconciliation", "name", network.Spec.RoutingDomainRef.Name, "namespace", network.Namespace)
	return []ctrl.Request{{Namespace: network.Namespace, Name: network.Spec.RoutingDomainRef.Name}}
}

// claimToRoutingDomain is a [handler.MapFunc] that enqueues
// the RoutingDomain associated with a VNI Claim.
func (r *RoutingDomainReconciler) claimToRoutingDomain(ctx context.Context, obj client.Object) []ctrl.Request {
	claim, ok := obj.(*poolv1alpha1.Claim)
	if !ok {
		panic(fmt.Sprintf("Expected a Claim but got a %T", obj))
	}

	name := claim.Labels[evpnv1alpha1.RoutingDomainLabel]
	if name == "" {
		return nil
	}

	log := ctrl.LoggerFrom(ctx, "Claim", klog.KObj(claim))
	log.V(2).Info("Enqueuing RoutingDomain for reconciliation", "name", name, "namespace", claim.Namespace)
	return []ctrl.Request{{Namespace: claim.Namespace, Name: name}}
}
