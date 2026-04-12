// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nx

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
	"k8s.io/apimachinery/pkg/runtime"
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

	"github.com/ironcore-dev/network-operator/internal/conditions"
	"github.com/ironcore-dev/network-operator/internal/paused"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/resourcelock"

	nxv1alpha1 "github.com/ironcore-dev/network-operator/api/cisco/nx/v1alpha1"
	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	corecontroller "github.com/ironcore-dev/network-operator/internal/controller/core"
	"github.com/ironcore-dev/network-operator/internal/deviceutil"
)

// VPCDomainReconciler reconciles a VPCDomain object
type VPCDomainReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// WatchFilterValue is the label value used to filter events prior to reconciliation.
	WatchFilterValue string

	// Recorder is used to record events for the controller.
	// More info: https://book.kubebuilder.io/reference/raising-events
	Recorder events.EventRecorder

	// Provider is the driver that will be used to create & delete the vPC
	Provider provider.ProviderFunc

	// Locker is used to synchronize operations on resources targeting the same device.
	Locker *resourcelock.ResourceLocker

	// RequeueInterval is the duration after which the controller should requeue the reconciliation,
	// regardless of changes.
	RequeueInterval time.Duration
}

// +kubebuilder:rbac:groups=nx.cisco.networking.metal.ironcore.dev,resources=vpcdomains,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=nx.cisco.networking.metal.ironcore.dev,resources=vpcdomains/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=nx.cisco.networking.metal.ironcore.dev,resources=vpcdomains/finalizers,verbs=update
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *VPCDomainReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(3).Info("Reconciling resource")

	obj := new(nxv1alpha1.VPCDomain)
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

	prov, ok := r.Provider().(Provider)
	if !ok {
		meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.NotImplementedReason,
			Message: "Invalid provider configured for VPCDomain reconciler",
		})
		return ctrl.Result{}, r.Status().Update(ctx, obj)
	}

	device, err := deviceutil.GetDeviceByName(ctx, r, obj.Namespace, obj.Spec.DeviceRef.Name)
	if err != nil {
		return ctrl.Result{}, err
	}

	if isPaused, requeue, err := paused.EnsureCondition(ctx, r.Client, device, obj); isPaused || requeue || err != nil {
		return ctrl.Result{Requeue: requeue}, err
	}

	if err := r.Locker.AcquireLock(ctx, device.Name, "cisco-nx-vpcdomain-controller"); err != nil {
		if errors.Is(err, resourcelock.ErrLockAlreadyHeld) {
			log.V(3).Info("Device is already locked, requeuing reconciliation")
			return ctrl.Result{RequeueAfter: corecontroller.Jitter(time.Second), Priority: new(corecontroller.LockWaitPriorityDefault)}, nil
		}
		log.Error(err, "Failed to acquire device lock")
		return ctrl.Result{}, err
	}
	defer func() {
		if err := r.Locker.ReleaseLock(ctx, device.Name, "cisco-nx-vpcdomain-controller"); err != nil {
			log.Error(err, "Failed to release device lock")
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	conn, err := deviceutil.GetDeviceConnection(ctx, r, device)
	if err != nil {
		return ctrl.Result{}, err
	}

	s := &vpcdomainScope{
		Device:     device,
		VPCDomain:  obj,
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
		log.V(3).Info("Resource is being deleted, skipping reconciliation")
		return ctrl.Result{}, nil
	}

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
	if conditions.InitializeConditions(obj, v1alpha1.ReadyCondition) {
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

	if err = r.reconcile(ctx, s); err != nil {
		log.Error(err, "Failed to reconcile resource")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: corecontroller.Jitter(r.RequeueInterval)}, nil
}

const vpcDomainPeerLinkRefKey = ".spec.peer.interfaceRef.name"

// SetupWithManager sets up the controller with the Manager.
func (r *VPCDomainReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	labelSelector := metav1.LabelSelector{}
	if r.WatchFilterValue != "" {
		labelSelector.MatchLabels = map[string]string{v1alpha1.WatchLabel: r.WatchFilterValue}
	}

	filter, err := predicate.LabelSelectorPredicate(labelSelector)
	if err != nil {
		return fmt.Errorf("failed to create label selector predicate: %w", err)
	}

	// Index vPCs by their peer interface reference
	if err := mgr.GetFieldIndexer().IndexField(ctx, &nxv1alpha1.VPCDomain{}, vpcDomainPeerLinkRefKey, func(obj client.Object) []string {
		vpc := obj.(*nxv1alpha1.VPCDomain)
		return []string{vpc.Spec.Peer.InterfaceRef.Name}
	}); err != nil {
		return err
	}

	// Index vPCs by their device reference
	if err := mgr.GetFieldIndexer().IndexField(ctx, &nxv1alpha1.VPCDomain{}, v1alpha1.DeviceRefIndexKey, func(obj client.Object) []string {
		vpc := obj.(*nxv1alpha1.VPCDomain)
		return []string{vpc.Spec.DeviceRef.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&nxv1alpha1.VPCDomain{}).
		Named("vpcdomain").
		WithEventFilter(filter).
		// Trigger reconciliation for changes in the operational status of the referenced interface: The device can shut down the port-channel by itself
		// in certain failure scenarios, e.g., incompatible configuration.
		Watches(
			&v1alpha1.Interface{},
			handler.EnqueueRequestsFromMapFunc(r.mapInterfaceToVPCDomain),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldInterface := e.ObjectOld.(*v1alpha1.Interface)
					newInterface := e.ObjectNew.(*v1alpha1.Interface)
					oldOperational := conditions.Get(oldInterface, v1alpha1.OperationalCondition)
					newOperational := conditions.Get(newInterface, v1alpha1.OperationalCondition)
					return ((oldOperational == nil) != (newOperational == nil)) || (newOperational != nil && oldOperational.Status != newOperational.Status)
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			}),
		).
		// Watches enqueues VPCDomains for updates in referenced Device resources.
		// Triggers on create, delete, and update events when the device's effective pause state changes.
		Watches(
			&v1alpha1.Device{},
			handler.EnqueueRequestsFromMapFunc(r.deviceToVPCDomains),
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

// // scope holds the different objects that are read and used during the reconcile.
type vpcdomainScope struct {
	Device     *v1alpha1.Device
	VPCDomain  *nxv1alpha1.VPCDomain
	Connection *deviceutil.Connection
	Provider   Provider
}

// reconcile contains the main reconciliation logic for the VPCDomain resource.
func (r *VPCDomainReconciler) reconcile(ctx context.Context, s *vpcdomainScope) (reterr error) {
	if s.VPCDomain.Labels == nil {
		s.VPCDomain.Labels = make(map[string]string)
	}
	s.VPCDomain.Labels[v1alpha1.DeviceLabel] = s.Device.Name

	// Ensure owner reference to device
	if !controllerutil.HasControllerReference(s.VPCDomain) {
		if err := controllerutil.SetOwnerReference(s.Device, s.VPCDomain, r.Scheme, controllerutil.WithBlockOwnerDeletion(true)); err != nil {
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

	defer func() {
		conditions.RecomputeReady(s.VPCDomain)
	}()

	peerLink, err := r.reconcilePeerLink(ctx, s)
	if err != nil {
		reterr = kerrors.NewAggregate([]error{reterr, fmt.Errorf("failed to reconcile referenced resource: %w", err)})
	}

	var vrf *v1alpha1.VRF
	if s.VPCDomain.Spec.Peer.KeepAlive.VrfRef == nil {
		vrf, err = r.reconcileKeepAliveVRF(ctx, s)
		if err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, fmt.Errorf("failed to reconcile referenced resource: %w", err)})
		}
	}

	if reterr != nil {
		return reterr
	}

	//  Realize the vPC via the provider and update configuration status
	err = s.Provider.EnsureVPCDomain(ctx, s.VPCDomain, vrf, peerLink)
	cond := conditions.FromError(err)
	conditions.Set(s.VPCDomain, cond)
	if err != nil {
		reterr = kerrors.NewAggregate([]error{reterr, fmt.Errorf("failed to reconcile resource: %w", err)})
	}

	// Retrieve and update status from the device, nil out on error
	status, err := s.Provider.GetStatusVPCDomain(ctx)
	if err != nil {
		s.VPCDomain.Status.Reset()
		reterr = kerrors.NewAggregate([]error{reterr, fmt.Errorf("failed to get resource status: %w", err)})
	}

	if reterr != nil {
		return reterr
	}

	s.VPCDomain.Status.Role = status.Role

	slices.Sort(status.KeepAliveStatusMsg)
	s.VPCDomain.Status.KeepAliveStatusMsg = status.KeepAliveStatusMsg
	s.VPCDomain.Status.KeepAliveStatus = nxv1alpha1.StatusDown
	if status.KeepAliveStatus {
		s.VPCDomain.Status.KeepAliveStatus = nxv1alpha1.StatusUp
	}
	slices.Sort(s.VPCDomain.Status.KeepAliveStatusMsg)
	s.VPCDomain.Status.PeerStatusMsg = status.PeerStatusMsg
	s.VPCDomain.Status.PeerStatus = nxv1alpha1.StatusDown
	if status.PeerStatus {
		s.VPCDomain.Status.PeerStatus = nxv1alpha1.StatusUp
	}
	s.VPCDomain.Status.PeerUptime = metav1.Duration{Duration: status.PeerUptime}

	// Fetch the status of the port-channel forming the vPC's peer link
	peerlinkOperSt := false
	s.VPCDomain.Status.PeerLinkIf = peerLink.Spec.Name
	if cond := conditions.Get(peerLink, v1alpha1.OperationalCondition); cond != nil {
		s.VPCDomain.Status.PeerLinkIfOperStatus = nxv1alpha1.StatusDown
		if cond.Status == metav1.ConditionTrue {
			s.VPCDomain.Status.PeerLinkIfOperStatus = nxv1alpha1.StatusUp
			peerlinkOperSt = true
		}
	}

	cond = metav1.Condition{
		Type:    v1alpha1.OperationalCondition,
		Status:  metav1.ConditionTrue,
		Reason:  v1alpha1.OperationalReason,
		Message: "vPC domain is operational",
	}
	// See comment in this type's status definition for details on the operational condition.
	if !status.PeerStatus || !status.KeepAliveStatus || !peerlinkOperSt {
		cond = metav1.Condition{
			Type:    v1alpha1.OperationalCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.DegradedReason,
			Message: "vPC domain is not fully operational. Either the peer-link interface is down, keepalive is not working, or the peer adjacency is not correctly formed.",
		}
	}
	conditions.Set(s.VPCDomain, cond)

	return nil
}

// reconcilePeerLink checks that peer's interface reference exists and is of type Aggregate.
// Ignores the interface status: Port-channels require the domain to be configured first.
func (r *VPCDomainReconciler) reconcilePeerLink(ctx context.Context, s *vpcdomainScope) (*v1alpha1.Interface, error) {
	intf := new(v1alpha1.Interface)
	intf.Name = s.VPCDomain.Spec.Peer.InterfaceRef.Name
	intf.Namespace = s.VPCDomain.Namespace

	if err := r.Get(ctx, client.ObjectKeyFromObject(intf), intf); err != nil {
		if apierrors.IsNotFound(err) {
			conditions.Set(s.VPCDomain, metav1.Condition{
				Type:    v1alpha1.ConfiguredCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.WaitingForDependenciesReason,
				Message: fmt.Sprintf("interface resource '%s' not found in namespace '%s'", intf.Name, intf.Namespace),
			})
			return nil, reconcile.TerminalError(fmt.Errorf("member interface %s not found", intf.Name))
		}
		return nil, fmt.Errorf("failed to get member interface %q: %w", intf.Name, err)
	}

	if intf.Spec.Type != v1alpha1.InterfaceTypeAggregate && intf.Spec.Type != v1alpha1.InterfaceTypePhysical {
		conditions.Set(s.VPCDomain, metav1.Condition{
			Type:    v1alpha1.ConfiguredCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.InvalidInterfaceTypeReason,
			Message: fmt.Sprintf("interface referenced by '%s' must be of type %q or type %q", intf.Name, v1alpha1.InterfaceTypeAggregate, v1alpha1.InterfaceTypePhysical),
		})
		return nil, reconcile.TerminalError(fmt.Errorf("interface referenced by '%s' must be of type %q or type %q", intf.Name, v1alpha1.InterfaceTypeAggregate, v1alpha1.InterfaceTypePhysical))
	}

	if s.VPCDomain.Spec.DeviceRef.Name != intf.Spec.DeviceRef.Name {
		conditions.Set(s.VPCDomain, metav1.Condition{
			Type:    v1alpha1.ConfiguredCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.CrossDeviceReferenceReason,
			Message: fmt.Sprintf("interface '%s' deviceRef '%s' does not match vPC deviceRef '%s'", intf.Name, intf.Spec.DeviceRef.Name, s.VPCDomain.Spec.DeviceRef.Name),
		})
		return nil, reconcile.TerminalError(fmt.Errorf("interface '%s' deviceRef '%s' does not match vPC deviceRef '%s'", intf.Name, intf.Spec.DeviceRef.Name, s.VPCDomain.Spec.DeviceRef.Name))
	}

	return intf, nil
}

// reconcileKeepAliveVRF ensures that the referenced VRF resource exists
func (r *VPCDomainReconciler) reconcileKeepAliveVRF(ctx context.Context, s *vpcdomainScope) (*v1alpha1.VRF, error) {
	vrf := new(v1alpha1.VRF)
	vrf.Name = s.VPCDomain.Spec.Peer.KeepAlive.VrfRef.Name
	vrf.Namespace = s.VPCDomain.Namespace

	if err := r.Get(ctx, client.ObjectKeyFromObject(vrf), vrf); err != nil {
		if apierrors.IsNotFound(err) {
			conditions.Set(s.VPCDomain, metav1.Condition{
				Type:    v1alpha1.ConfiguredCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.WaitingForDependenciesReason,
				Message: fmt.Sprintf("VRF resource '%s' not found in namespace '%s'", vrf.Name, vrf.Namespace),
			})
			return nil, reconcile.TerminalError(fmt.Errorf("VRF %q not found", vrf.Name))
		}
		return nil, fmt.Errorf("failed to get VRF %q: %w", vrf.Name, err)
	}

	if s.VPCDomain.Spec.DeviceRef.Name != vrf.Spec.DeviceRef.Name {
		conditions.Set(s.VPCDomain, metav1.Condition{
			Type:    v1alpha1.ConfiguredCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.CrossDeviceReferenceReason,
			Message: fmt.Sprintf("VRF '%s' deviceRef '%s' does not match VPCDomain deviceRef '%s'", vrf.Name, vrf.Spec.DeviceRef.Name, s.VPCDomain.Spec.DeviceRef.Name),
		})
		return nil, reconcile.TerminalError(fmt.Errorf("VRF '%s' deviceRef '%s' does not match VPCDomain deviceRef '%s'", vrf.Name, vrf.Spec.DeviceRef.Name, s.VPCDomain.Spec.DeviceRef.Name))
	}

	return vrf, nil
}

func (r *VPCDomainReconciler) mapInterfaceToVPCDomain(ctx context.Context, obj client.Object) []ctrl.Request {
	iface, ok := obj.(*v1alpha1.Interface)
	if !ok {
		panic(fmt.Sprintf("Expected a Interface but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx, "Interface", klog.KObj(iface))

	list := new(nxv1alpha1.VPCDomainList)
	if err := r.List(ctx, list,
		client.InNamespace(iface.Namespace),
		client.MatchingFields{vpcDomainPeerLinkRefKey: iface.Name},
	); err != nil {
		log.Error(err, "Failed to list VPCDomains")
		return nil
	}

	requests := make([]reconcile.Request, 0, len(list.Items))
	for _, i := range list.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      i.Name,
				Namespace: i.Namespace,
			},
		})
	}

	return requests
}

func (r *VPCDomainReconciler) finalize(ctx context.Context, s *vpcdomainScope) (reterr error) {
	if err := s.Provider.Connect(ctx, s.Connection); err != nil {
		return fmt.Errorf("failed to connect to provider: %w", err)
	}
	defer func() {
		if err := s.Provider.Disconnect(ctx, s.Connection); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()
	return s.Provider.DeleteVPCDomain(ctx)
}

// deviceToVPCDomains is a [handler.MapFunc] to be used to enqueue requests for reconciliation
// for VPCDomains when their referenced Device's effective pause state changes.
func (r *VPCDomainReconciler) deviceToVPCDomains(ctx context.Context, obj client.Object) []ctrl.Request {
	device, ok := obj.(*v1alpha1.Device)
	if !ok {
		panic(fmt.Sprintf("Expected a Device but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx, "Device", klog.KObj(device))

	list := new(nxv1alpha1.VPCDomainList)
	if err := r.List(ctx, list,
		client.InNamespace(device.Namespace),
		client.MatchingFields{v1alpha1.DeviceRefIndexKey: device.Name},
	); err != nil {
		log.Error(err, "Failed to list VPCDomains")
		return nil
	}

	requests := make([]ctrl.Request, 0, len(list.Items))
	for _, i := range list.Items {
		log.V(2).Info("Enqueuing VPCDomain for reconciliation", "VPCDomain", klog.KObj(&i))
		requests = append(requests, ctrl.Request{
			NamespacedName: client.ObjectKey{
				Name:      i.Name,
				Namespace: i.Namespace,
			},
		})
	}

	return requests
}
