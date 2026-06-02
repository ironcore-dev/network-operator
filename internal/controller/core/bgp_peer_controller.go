// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"cmp"
	"context"
	"errors"
	"fmt"
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

// bgpPeerBGPRefIndexKey is the field index key for BGPPeer.Spec.BgpRef.Name.
const bgpPeerBGPRefIndexKey = ".spec.bgpRef.name"

// bgpPeerRoutingPolicyRefIndexKey is the field index key for all RoutingPolicy names
// referenced by BGPPeer address families.
const bgpPeerRoutingPolicyRefIndexKey = ".spec.addressFamilies.routingPolicyRefs"

// BGPPeerReconciler reconciles a BGPPeer object
type BGPPeerReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// WatchFilterValue is the label value used to filter events prior to reconciliation.
	WatchFilterValue string

	// Recorder is used to record events for the controller.
	// More info: https://book.kubebuilder.io/reference/raising-events
	Recorder events.EventRecorder

	// Provider is the driver that will be used to create & delete the bgppeer.
	Provider provider.ProviderFunc

	// Locker is used to synchronize operations on resources targeting the same device.
	Locker *resourcelock.ResourceLocker

	// RequeueInterval is the duration after which the controller should requeue the reconciliation,
	// regardless of changes.
	RequeueInterval time.Duration
}

// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=bgppeers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=bgppeers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=bgppeers/finalizers,verbs=update
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=bgp,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=vrfs,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=routingpolicies,verbs=get;list;watch
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.2/pkg/reconcile
//
// For more details about the method shape, read up here:
// - https://ahmet.im/blog/controller-pitfalls/#reconcile-method-shape
func (r *BGPPeerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(3).Info("Reconciling resource")

	obj := new(v1alpha1.BGPPeer)
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

	prov, ok := r.Provider().(provider.BGPPeerProvider)
	if !ok {
		if meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.NotImplementedReason,
			Message: "Provider does not implement provider.BGPPeerProvider",
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

	if err := r.Locker.AcquireLock(ctx, device.Name, "bgppeer-controller"); err != nil {
		if errors.Is(err, resourcelock.ErrLockAlreadyHeld) {
			log.V(3).Info("Device is already locked, requeuing reconciliation")
			return ctrl.Result{RequeueAfter: Jitter(time.Second), Priority: new(LockWaitPriorityDefault)}, nil
		}
		log.Error(err, "Failed to acquire device lock")
		return ctrl.Result{}, err
	}
	defer func() {
		if err := r.Locker.ReleaseLock(ctx, device.Name, "bgppeer-controller"); err != nil {
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

	s := &bgpPeerScope{
		Device:         device,
		BGPPeer:        obj,
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

// SetupWithManager sets up the controller with the Manager.
func (r *BGPPeerReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
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

	if err := mgr.GetFieldIndexer().IndexField(ctx, &v1alpha1.BGPPeer{}, v1alpha1.DeviceRefIndexKey, func(obj client.Object) []string {
		o := obj.(*v1alpha1.BGPPeer)
		return []string{o.Spec.DeviceRef.Name}
	}); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(ctx, &v1alpha1.BGPPeer{}, bgpPeerBGPRefIndexKey, func(obj client.Object) []string {
		o := obj.(*v1alpha1.BGPPeer)
		return []string{o.Spec.BgpRef.Name}
	}); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(ctx, &v1alpha1.BGPPeer{}, bgpPeerRoutingPolicyRefIndexKey, func(obj client.Object) []string {
		o := obj.(*v1alpha1.BGPPeer)
		if o.Spec.AddressFamilies == nil {
			return nil
		}
		var names []string
		for _, af := range []*v1alpha1.BGPPeerAddressFamily{
			o.Spec.AddressFamilies.Ipv4Unicast,
			o.Spec.AddressFamilies.Ipv6Unicast,
			o.Spec.AddressFamilies.L2vpnEvpn,
		} {
			if af == nil {
				continue
			}
			if af.InboundRoutingPolicyRef != nil {
				names = append(names, af.InboundRoutingPolicyRef.Name)
			}
			if af.OutboundRoutingPolicyRef != nil {
				names = append(names, af.OutboundRoutingPolicyRef.Name)
			}
		}
		return names
	}); err != nil {
		return err
	}

	bldr := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.BGPPeer{}).
		Named("bgppeer").
		WithEventFilter(filter)

	for _, gvk := range v1alpha1.BGPPeerDependencies {
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(gvk)

		bldr = bldr.Watches(
			obj,
			handler.EnqueueRequestsFromMapFunc(r.bgpPeersForProviderConfig),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		)
	}

	return bldr.
		// Watches enqueues BGPPeers for updates in referenced Device resources.
		// Triggers on create, delete, and update events when the device's effective pause state changes.
		Watches(
			&v1alpha1.Device{},
			handler.EnqueueRequestsFromMapFunc(r.deviceToBGPPeers),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					return paused.DevicePausedChanged(e.ObjectOld, e.ObjectNew)
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			}),
		).
		// Watches enqueues BGPPeers for updates in BGP resources on the same device.
		// Only triggers on create, delete and update events when the BGP ready state changes.
		Watches(
			&v1alpha1.BGP{},
			handler.EnqueueRequestsFromMapFunc(r.bgpToBGPPeers),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldBGP := e.ObjectOld.(*v1alpha1.BGP)
					newBGP := e.ObjectNew.(*v1alpha1.BGP)
					return conditions.IsReady(oldBGP) != conditions.IsReady(newBGP)
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			}),
		).
		// Watches enqueues BGPPeers for updates in VRF resources referenced by their BGP.
		// Triggers on create, delete, and update events when the VRF's ready state changes.
		Watches(
			&v1alpha1.VRF{},
			handler.EnqueueRequestsFromMapFunc(r.vrfToBGPPeers),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldVRF := e.ObjectOld.(*v1alpha1.VRF)
					newVRF := e.ObjectNew.(*v1alpha1.VRF)
					return conditions.IsReady(oldVRF) != conditions.IsReady(newVRF)
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			}),
		).
		// Watches enqueues BGPPeers when a referenced RoutingPolicy is created or deleted.
		// Only triggers on create and delete events since RoutingPolicy names are immutable.
		Watches(
			&v1alpha1.RoutingPolicy{},
			handler.EnqueueRequestsFromMapFunc(r.routingPolicyToBGPPeers),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					return false
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			}),
		).
		Complete(r)
}

// scope holds the different objects that are read and used during the reconcile.
type bgpPeerScope struct {
	Device         *v1alpha1.Device
	BGPPeer        *v1alpha1.BGPPeer
	Connection     *deviceutil.Connection
	ProviderConfig *provider.ProviderConfig
	Provider       provider.BGPPeerProvider
}

func (r *BGPPeerReconciler) reconcile(ctx context.Context, s *bgpPeerScope) (reterr error) {
	if s.BGPPeer.Labels == nil {
		s.BGPPeer.Labels = make(map[string]string)
	}

	s.BGPPeer.Labels[v1alpha1.DeviceLabel] = s.Device.Name

	// Ensure the BGPPeer is owned by the Device.
	if !controllerutil.HasControllerReference(s.BGPPeer) {
		if err := controllerutil.SetOwnerReference(s.Device, s.BGPPeer, r.Scheme, controllerutil.WithBlockOwnerDeletion(true)); err != nil {
			return err
		}
	}

	defer func() {
		conditions.RecomputeReady(s.BGPPeer)
	}()

	bgp, err := r.reconcileBGP(ctx, s.BGPPeer, s.Device)
	if err != nil {
		return err
	}

	// BGP has no operational condition, so its ready condition reflects only successful configuration.
	// Wait for the BGP watch to re-trigger rather than requeuing periodically.
	if !conditions.IsReady(bgp) {
		conditions.Set(s.BGPPeer, metav1.Condition{
			Type:    v1alpha1.ConfiguredCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.WaitingForDependenciesReason,
			Message: fmt.Sprintf("BGP %s is not yet ready", s.BGPPeer.Spec.BgpRef.Name),
		})
		return nil
	}

	var vrf *v1alpha1.VRF
	if bgp.Spec.VrfRef != nil {
		vrf, err = r.reconcileVRF(ctx, s.BGPPeer, bgp, s.Device)
		if err != nil {
			return err
		}
	}

	var inbound, outbound map[v1alpha1.BGPAddressFamilyType]string
	if s.BGPPeer.Spec.AddressFamilies != nil {
		inbound, outbound, err = r.reconcileRoutingPolicies(ctx, s.BGPPeer, s.Device)
		if err != nil {
			return err
		}
	}

	var sourceInterface string
	if addr := s.BGPPeer.Spec.LocalAddress; addr != nil {
		intf := new(v1alpha1.Interface)
		if err := r.Get(ctx, client.ObjectKey{Name: addr.InterfaceRef.Name, Namespace: s.BGPPeer.Namespace}, intf); err != nil {
			if apierrors.IsNotFound(err) {
				conditions.Set(s.BGPPeer, metav1.Condition{
					Type:    v1alpha1.ConfiguredCondition,
					Status:  metav1.ConditionFalse,
					Reason:  v1alpha1.InterfaceNotFoundReason,
					Message: fmt.Sprintf("source interface %q not found", addr.InterfaceRef.Name),
				})
				return reconcile.TerminalError(fmt.Errorf("source interface %q not found", addr.InterfaceRef.Name))
			}
			return fmt.Errorf("failed to get source interface %q: %w", addr.InterfaceRef.Name, err)
		}

		if intf.Spec.DeviceRef.Name != s.Device.Name {
			conditions.Set(s.BGPPeer, metav1.Condition{
				Type:    v1alpha1.ConfiguredCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.CrossDeviceReferenceReason,
				Message: fmt.Sprintf("source interface %q does not belong to device %q", intf.Name, s.Device.Name),
			})
			return reconcile.TerminalError(fmt.Errorf("source interface %q does not belong to device %q", intf.Name, s.Device.Name))
		}
		sourceInterface = intf.Spec.Name
	}

	if err := s.Provider.Connect(ctx, s.Connection); err != nil {
		return fmt.Errorf("failed to connect to provider: %w", err)
	}
	defer func() {
		if err := s.Provider.Disconnect(ctx, s.Connection); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	// Ensure the BGPPeer is realized on the provider.
	err = s.Provider.EnsureBGPPeer(ctx, &provider.EnsureBGPPeerRequest{
		BGPPeer:                 s.BGPPeer,
		ProviderConfig:          s.ProviderConfig,
		SourceInterface:         sourceInterface,
		BGP:                     bgp,
		VRF:                     vrf,
		InboundRoutingPolicies:  inbound,
		OutboundRoutingPolicies: outbound,
	})

	cond := conditions.FromError(err)
	conditions.Set(s.BGPPeer, cond)

	if err != nil {
		return err
	}

	status, err := s.Provider.GetPeerStatus(ctx, &provider.BGPPeerStatusRequest{
		BGPPeer:        s.BGPPeer,
		ProviderConfig: s.ProviderConfig,
		VRF:            vrf,
	})
	if err != nil {
		return fmt.Errorf("failed to get bgp peer status: %w", err)
	}

	cond = metav1.Condition{
		Type:    v1alpha1.OperationalCondition,
		Status:  metav1.ConditionTrue,
		Reason:  v1alpha1.OperationalReason,
		Message: "Session is established",
	}
	if status.SessionState != v1alpha1.BGPPeerSessionStateEstablished {
		cond.Status = metav1.ConditionFalse
		cond.Reason = string(status.SessionState)
		cond.Message = "Session is not established"
	}
	conditions.Set(s.BGPPeer, cond)

	s.BGPPeer.Status.SessionState = status.SessionState
	if !status.LastEstablishedTime.IsZero() {
		s.BGPPeer.Status.LastEstablishedTime = new(metav1.NewTime(status.LastEstablishedTime))
	}
	s.BGPPeer.Status.ObservedGeneration = s.BGPPeer.Generation
	s.BGPPeer.Status.AddressFamilies = nil

	for af, stats := range status.AddressFamilies {
		s.BGPPeer.Status.AddressFamilies = append(s.BGPPeer.Status.AddressFamilies, v1alpha1.AddressFamilyStatus{
			AfiSafi:            af,
			AcceptedPrefixes:   int64(stats.Accepted),
			AdvertisedPrefixes: int64(stats.Advertised),
		})
	}

	slices.SortStableFunc(s.BGPPeer.Status.AddressFamilies, func(i, j v1alpha1.AddressFamilyStatus) int {
		return cmp.Compare(string(i.AfiSafi), string(j.AfiSafi))
	})

	var summaries []string
	for _, af := range s.BGPPeer.Status.AddressFamilies {
		summaries = append(summaries, fmt.Sprintf("%d (%s)", af.AdvertisedPrefixes, af.AfiSafi))
	}
	s.BGPPeer.Status.AdvertisedPrefixesSummary = strings.Join(summaries, ", ")

	return nil
}

func (r *BGPPeerReconciler) finalize(ctx context.Context, s *bgpPeerScope) (reterr error) {
	bgp := new(v1alpha1.BGP)
	if err := r.Get(ctx, types.NamespacedName{
		Name:      s.BGPPeer.Spec.BgpRef.Name,
		Namespace: s.BGPPeer.Namespace,
	}, bgp); err != nil {
		// If the BGP is not found, we can assume it was deleted and with it the BGP peer reference, so we can proceed with deletion without error.
		return client.IgnoreNotFound(err)
	}
	if bgp.Spec.DeviceRef.Name != s.Device.Name {
		return reconcile.TerminalError(fmt.Errorf("bgp %s belongs to different device", s.BGPPeer.Spec.BgpRef.Name))
	}

	var vrf *v1alpha1.VRF
	if bgp.Spec.VrfRef != nil {
		vrf = new(v1alpha1.VRF)
		if err := r.Get(ctx, types.NamespacedName{
			Name:      bgp.Spec.VrfRef.Name,
			Namespace: s.BGPPeer.Namespace,
		}, vrf); err != nil {
			// If the VRF is not found, we can assume it was deleted and with it the BGP peer reference, so we can proceed with deletion without error.
			return client.IgnoreNotFound(err)
		}
		if vrf.Spec.DeviceRef.Name != s.Device.Name {
			return reconcile.TerminalError(fmt.Errorf("vrf %s belongs to different device", bgp.Spec.VrfRef.Name))
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

	return s.Provider.DeleteBGPPeer(ctx, &provider.DeleteBGPPeerRequest{
		BGPPeer:        s.BGPPeer,
		ProviderConfig: s.ProviderConfig,
		BGP:            bgp,
		VRF:            vrf,
	})
}

// reconcileBGP resolves the referenced BGP instance.
// Sets ConfiguredCondition and returns a terminal error when the BGP is not found
// or belongs to a different device.
func (r *BGPPeerReconciler) reconcileBGP(ctx context.Context, peer *v1alpha1.BGPPeer, device *v1alpha1.Device) (*v1alpha1.BGP, error) {
	bgp := new(v1alpha1.BGP)
	if err := r.Get(ctx, types.NamespacedName{
		Name:      peer.Spec.BgpRef.Name,
		Namespace: peer.Namespace,
	}, bgp); err != nil {
		if apierrors.IsNotFound(err) {
			conditions.Set(peer, metav1.Condition{
				Type:    v1alpha1.ConfiguredCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.BGPNotFoundReason,
				Message: fmt.Sprintf("BGP %s not found", peer.Spec.BgpRef.Name),
			})
			return nil, reconcile.TerminalError(fmt.Errorf("bgp %s not found", peer.Spec.BgpRef.Name))
		}
		return nil, fmt.Errorf("failed to get BGP %s: %w", peer.Spec.BgpRef.Name, err)
	}

	if bgp.Spec.DeviceRef.Name != device.Name {
		conditions.Set(peer, metav1.Condition{
			Type:    v1alpha1.ConfiguredCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.CrossDeviceReferenceReason,
			Message: fmt.Sprintf("BGP %s belongs to device %s, not %s", peer.Spec.BgpRef.Name, bgp.Spec.DeviceRef.Name, device.Name),
		})
		return nil, reconcile.TerminalError(fmt.Errorf("bgp %s belongs to different device", peer.Spec.BgpRef.Name))
	}

	return bgp, nil
}

// Returns nil when no VrfRef is set, meaning the default VRF should be used.
// Sets ConfiguredCondition and returns a terminal error when the VRF is not found or belongs to a different device.
func (r *BGPPeerReconciler) reconcileVRF(ctx context.Context, peer *v1alpha1.BGPPeer, bgp *v1alpha1.BGP, device *v1alpha1.Device) (*v1alpha1.VRF, error) {
	vrf := new(v1alpha1.VRF)
	if err := r.Get(ctx, types.NamespacedName{
		Name:      bgp.Spec.VrfRef.Name,
		Namespace: peer.Namespace,
	}, vrf); err != nil {
		if apierrors.IsNotFound(err) {
			conditions.Set(peer, metav1.Condition{
				Type:    v1alpha1.ConfiguredCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.VRFNotFoundReason,
				Message: fmt.Sprintf("VRF %s not found", bgp.Spec.VrfRef.Name),
			})
			return nil, reconcile.TerminalError(fmt.Errorf("vrf %s not found", bgp.Spec.VrfRef.Name))
		}
		return nil, fmt.Errorf("failed to get VRF %s: %w", bgp.Spec.VrfRef.Name, err)
	}

	if vrf.Spec.DeviceRef.Name != device.Name {
		conditions.Set(peer, metav1.Condition{
			Type:    v1alpha1.ConfiguredCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.CrossDeviceReferenceReason,
			Message: fmt.Sprintf("VRF %s belongs to device %s, not %s", bgp.Spec.VrfRef.Name, vrf.Spec.DeviceRef.Name, device.Name),
		})
		return nil, reconcile.TerminalError(fmt.Errorf("vrf %s belongs to different device", bgp.Spec.VrfRef.Name))
	}

	if !conditions.IsReady(vrf) {
		// VRF uses ReadyCondition as its top-level configured state (no separate ConfiguredCondition).
		conditions.Set(peer, metav1.Condition{
			Type:    v1alpha1.ConfiguredCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.WaitingForDependenciesReason,
			Message: fmt.Sprintf("Waiting for VRF %s to become ready", bgp.Spec.VrfRef.Name),
		})
		return nil, reconcile.TerminalError(fmt.Errorf("vrf %s is not yet ready", bgp.Spec.VrfRef.Name))
	}

	return vrf, nil
}

// reconcileRoutingPolicies resolves the inbound and outbound routing policy references
// on each enabled address family of the BGPPeer.
// Returns two maps from BGPAddressFamilyType to the device-level policy name.
// Sets ConfiguredCondition and returns a terminal error when a referenced policy
// is not found or belongs to a different device.
func (r *BGPPeerReconciler) reconcileRoutingPolicies(ctx context.Context, peer *v1alpha1.BGPPeer, device *v1alpha1.Device) (inbound, outbound map[v1alpha1.BGPAddressFamilyType]string, err error) {
	inbound = make(map[v1alpha1.BGPAddressFamilyType]string)
	outbound = make(map[v1alpha1.BGPAddressFamilyType]string)

	for afType, af := range map[v1alpha1.BGPAddressFamilyType]*v1alpha1.BGPPeerAddressFamily{
		v1alpha1.BGPAddressFamilyIpv4Unicast: peer.Spec.AddressFamilies.Ipv4Unicast,
		v1alpha1.BGPAddressFamilyIpv6Unicast: peer.Spec.AddressFamilies.Ipv6Unicast,
		v1alpha1.BGPAddressFamilyL2vpnEvpn:   peer.Spec.AddressFamilies.L2vpnEvpn,
	} {
		if af == nil || !af.Enabled {
			continue
		}
		for ref, m := range map[*v1alpha1.LocalObjectReference]*map[v1alpha1.BGPAddressFamilyType]string{
			af.InboundRoutingPolicyRef:  &inbound,
			af.OutboundRoutingPolicyRef: &outbound,
		} {
			if ref == nil {
				continue
			}
			rp := new(v1alpha1.RoutingPolicy)
			if err := r.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: peer.Namespace}, rp); err != nil {
				if apierrors.IsNotFound(err) {
					conditions.Set(peer, metav1.Condition{
						Type:    v1alpha1.ConfiguredCondition,
						Status:  metav1.ConditionFalse,
						Reason:  v1alpha1.RoutingPolicyNotFoundReason,
						Message: fmt.Sprintf("RoutingPolicy %s not found", ref.Name),
					})
					return nil, nil, reconcile.TerminalError(fmt.Errorf("routing policy %s not found", ref.Name))
				}
				return nil, nil, fmt.Errorf("failed to get RoutingPolicy %s: %w", ref.Name, err)
			}
			if rp.Spec.DeviceRef.Name != device.Name {
				conditions.Set(peer, metav1.Condition{
					Type:    v1alpha1.ConfiguredCondition,
					Status:  metav1.ConditionFalse,
					Reason:  v1alpha1.CrossDeviceReferenceReason,
					Message: fmt.Sprintf("RoutingPolicy %s belongs to device %s, not %s", ref.Name, rp.Spec.DeviceRef.Name, device.Name),
				})
				return nil, nil, reconcile.TerminalError(fmt.Errorf("routing policy %s belongs to different device", ref.Name))
			}
			(*m)[afType] = rp.Spec.Name
		}
	}

	return inbound, outbound, nil
}

// deviceToBGPPeers is a [handler.MapFunc] to be used to enqueue requests for reconciliation
// for BGPPeers when their referenced Device's effective pause state changes.
func (r *BGPPeerReconciler) deviceToBGPPeers(ctx context.Context, obj client.Object) []ctrl.Request {
	device, ok := obj.(*v1alpha1.Device)
	if !ok {
		panic(fmt.Sprintf("Expected a Device but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx, "Device", klog.KObj(device))

	list := new(v1alpha1.BGPPeerList)
	if err := r.List(
		ctx, list,
		client.InNamespace(device.Namespace),
		client.MatchingFields{v1alpha1.DeviceRefIndexKey: device.Name},
	); err != nil {
		log.Error(err, "Failed to list BGPPeers")
		return nil
	}

	requests := make([]ctrl.Request, 0, len(list.Items))
	for _, i := range list.Items {
		log.V(2).Info("Enqueuing BGPPeer for reconciliation", "BGPPeer", klog.KObj(&i))
		requests = append(requests, ctrl.Request{
			NamespacedName: client.ObjectKey{
				Name:      i.Name,
				Namespace: i.Namespace,
			},
		})
	}

	return requests
}

// bgpPeersForProviderConfig is a [handler.MapFunc] to be used to enqueue requests for reconciliation
// for a BGPPeer to update when one of its referenced provider configurations gets updated.
func (r *BGPPeerReconciler) bgpPeersForProviderConfig(ctx context.Context, obj client.Object) []reconcile.Request {
	log := ctrl.LoggerFrom(ctx, "Object", klog.KObj(obj))

	list := &v1alpha1.BGPPeerList{}
	if err := r.List(ctx, list, client.InNamespace(obj.GetNamespace())); err != nil {
		log.Error(err, "Failed to list BGPPeers")
		return nil
	}

	gkv := obj.GetObjectKind().GroupVersionKind()

	var requests []reconcile.Request
	for _, m := range list.Items {
		if m.Spec.ProviderConfigRef != nil &&
			m.Spec.ProviderConfigRef.Name == obj.GetName() &&
			m.Spec.ProviderConfigRef.Kind == gkv.Kind &&
			m.Spec.ProviderConfigRef.APIVersion == gkv.GroupVersion().Identifier() {
			log.V(2).Info("Enqueuing BGPPeer for reconciliation", "BGPPeer", klog.KObj(&m))
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

// bgpToBGPPeers is a [handler.MapFunc] to be used to enqueue requests for reconciliation
// for BGPPeers when a BGP resource is created, deleted or updated on the same device.
func (r *BGPPeerReconciler) bgpToBGPPeers(ctx context.Context, obj client.Object) []ctrl.Request {
	bgp, ok := obj.(*v1alpha1.BGP)
	if !ok {
		panic(fmt.Sprintf("Expected a BGP but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx, "BGP", klog.KObj(bgp))

	list := new(v1alpha1.BGPPeerList)
	if err := r.List(
		ctx, list,
		client.InNamespace(bgp.Namespace),
		client.MatchingFields{bgpPeerBGPRefIndexKey: bgp.Name},
	); err != nil {
		log.Error(err, "Failed to list BGPPeers")
		return nil
	}

	requests := make([]ctrl.Request, 0, len(list.Items))
	for _, i := range list.Items {
		log.V(2).Info("Enqueuing BGPPeer for reconciliation", "BGPPeer", klog.KObj(&i))
		requests = append(requests, ctrl.Request{
			NamespacedName: client.ObjectKey{
				Name:      i.Name,
				Namespace: i.Namespace,
			},
		})
	}

	return requests
}

// vrfToBGPPeers is a [handler.MapFunc] to be used to enqueue requests for reconciliation
// for BGPPeers when the VRF referenced by their BGP is created or deleted.
func (r *BGPPeerReconciler) vrfToBGPPeers(ctx context.Context, obj client.Object) []ctrl.Request {
	vrf, ok := obj.(*v1alpha1.VRF)
	if !ok {
		panic(fmt.Sprintf("Expected a VRF but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx, "VRF", klog.KObj(vrf))

	bgpList := new(v1alpha1.BGPList)
	if err := r.List(
		ctx, bgpList,
		client.InNamespace(vrf.Namespace),
		client.MatchingFields{bgpVrfRefIndexKey: vrf.Name},
	); err != nil {
		log.Error(err, "Failed to list BGPs")
		return nil
	}

	var requests []ctrl.Request
	for _, bgp := range bgpList.Items {
		peerList := new(v1alpha1.BGPPeerList)
		if err := r.List(
			ctx, peerList,
			client.InNamespace(vrf.Namespace),
			client.MatchingFields{bgpPeerBGPRefIndexKey: bgp.Name},
		); err != nil {
			log.Error(err, "Failed to list BGPPeers")
			return nil
		}

		for _, p := range peerList.Items {
			log.V(2).Info("Enqueuing BGPPeer for reconciliation", "BGPPeer", klog.KObj(&p))
			requests = append(requests, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      p.Name,
					Namespace: p.Namespace,
				},
			})
		}
	}

	return requests
}

// routingPolicyToBGPPeers is a [handler.MapFunc] to be used to enqueue requests for reconciliation
// for BGPPeers when a RoutingPolicy referenced by one of their address families is created or deleted.
func (r *BGPPeerReconciler) routingPolicyToBGPPeers(ctx context.Context, obj client.Object) []ctrl.Request {
	rp, ok := obj.(*v1alpha1.RoutingPolicy)
	if !ok {
		panic(fmt.Sprintf("Expected a RoutingPolicy but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx, "RoutingPolicy", klog.KObj(rp))

	list := new(v1alpha1.BGPPeerList)
	if err := r.List(
		ctx, list,
		client.InNamespace(rp.Namespace),
		client.MatchingFields{bgpPeerRoutingPolicyRefIndexKey: rp.Name},
	); err != nil {
		log.Error(err, "Failed to list BGPPeers")
		return nil
	}

	requests := make([]ctrl.Request, 0, len(list.Items))
	for _, p := range list.Items {
		log.V(2).Info("Enqueuing BGPPeer for reconciliation", "BGPPeer", klog.KObj(&p))
		requests = append(requests, ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      p.Name,
				Namespace: p.Namespace,
			},
		})
	}

	return requests
}
