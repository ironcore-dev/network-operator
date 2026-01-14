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
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	nxv1alpha1 "github.com/ironcore-dev/network-operator/api/cisco/nx/v1alpha1"
	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/conditions"
	"github.com/ironcore-dev/network-operator/internal/deviceutil"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos"
	"github.com/ironcore-dev/network-operator/internal/resourcelock"
)

// BorderGatewayReconciler reconciles a BorderGateway object
type BorderGatewayReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// WatchFilterValue is the label value used to filter events prior to reconciliation.
	WatchFilterValue string

	// Recorder is used to record events for the controller.
	// More info: https://book.kubebuilder.io/reference/raising-events
	Recorder record.EventRecorder

	// Provider is the driver that will be used to create & delete the bordergateway.
	Provider provider.ProviderFunc

	// Locker is used to synchronize operations on resources targeting the same device.
	Locker *resourcelock.ResourceLocker
}

// +kubebuilder:rbac:groups=nx.cisco.networking.metal.ironcore.dev,resources=bordergateways,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=nx.cisco.networking.metal.ironcore.dev,resources=bordergateways/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=nx.cisco.networking.metal.ironcore.dev,resources=bordergateways/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.2/pkg/reconcile
//
// For more details about the method shape, read up here:
// - https://ahmet.im/blog/controller-pitfalls/#reconcile-method-shape
func (r *BorderGatewayReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling resource")

	obj := new(nxv1alpha1.BorderGateway)
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			// If the custom resource is not found then it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			log.Info("Resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get resource")
		return ctrl.Result{}, err
	}

	prov, ok := r.Provider().(Provider)
	if !ok {
		if meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.ErrorReason,
			Message: "Invalid provider configured for BorderGateway reconciler",
		}) {
			return ctrl.Result{}, r.Status().Update(ctx, obj)
		}
		return ctrl.Result{}, nil
	}

	device, err := deviceutil.GetDeviceByName(ctx, r, obj.Namespace, obj.Spec.DeviceRef.Name)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := r.Locker.AcquireLock(ctx, device.Name, "cisco-nx-border-gateway-controller"); err != nil {
		if errors.Is(err, resourcelock.ErrLockAlreadyHeld) {
			log.Info("Device is already locked, requeuing reconciliation")
			return ctrl.Result{RequeueAfter: time.Second * 5}, nil
		}
		log.Error(err, "Failed to acquire device lock")
		return ctrl.Result{}, err
	}
	defer func() {
		if err := r.Locker.ReleaseLock(ctx, device.Name, "cisco-nx-border-gateway-controller"); err != nil {
			log.Error(err, "Failed to release device lock")
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	conn, err := deviceutil.GetDeviceConnection(ctx, r, device)
	if err != nil {
		return ctrl.Result{}, err
	}

	s := &borderGatewayScope{
		Device:        device,
		BorderGateway: obj,
		Connection:    conn,
		Provider:      prov,
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
		log.Info("Resource is being deleted, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers
	if !controllerutil.ContainsFinalizer(obj, v1alpha1.FinalizerName) {
		controllerutil.AddFinalizer(obj, v1alpha1.FinalizerName)
		if err := r.Update(ctx, obj); err != nil {
			log.Error(err, "Failed to add finalizer to resource")
			return ctrl.Result{}, err
		}
		log.Info("Added finalizer to resource")
		return ctrl.Result{}, nil
	}

	orig := obj.DeepCopy()
	if conditions.InitializeConditions(obj, v1alpha1.ReadyCondition) {
		log.Info("Initializing status conditions")
		return ctrl.Result{}, r.Status().Update(ctx, obj)
	}

	// Always attempt to update the metadata/status after reconciliation
	defer func() {
		if !equality.Semantic.DeepEqual(orig.ObjectMeta, obj.ObjectMeta) {
			if err := r.Patch(ctx, obj, client.MergeFrom(orig)); err != nil {
				log.Error(err, "Failed to update resource metadata")
				reterr = kerrors.NewAggregate([]error{reterr, err})
			}
			return
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
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

var (
	borderGatewaySourceInterfaceRefKey       = ".spec.sourceInterfaceRef.name"
	borderGatewayInterconnectInterfaceRefKey = ".spec.interconnectInterfaceRefs.name"
	borderGatewayBGPPeerRefKey               = ".spec.bgpPeerRefs.name"
)

// SetupWithManager sets up the controller with the Manager.
func (r *BorderGatewayReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	labelSelector := metav1.LabelSelector{}
	if r.WatchFilterValue != "" {
		labelSelector.MatchLabels = map[string]string{v1alpha1.WatchLabel: r.WatchFilterValue}
	}

	filter, err := predicate.LabelSelectorPredicate(labelSelector)
	if err != nil {
		return fmt.Errorf("failed to create label selector predicate: %w", err)
	}

	if err := mgr.GetFieldIndexer().IndexField(ctx, &nxv1alpha1.BorderGateway{}, borderGatewaySourceInterfaceRefKey, func(obj client.Object) []string {
		bg := obj.(*nxv1alpha1.BorderGateway)
		return []string{bg.Spec.SourceInterfaceRef.Name}
	}); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(ctx, &nxv1alpha1.BorderGateway{}, borderGatewayInterconnectInterfaceRefKey, func(obj client.Object) []string {
		bg := obj.(*nxv1alpha1.BorderGateway)
		refs := make([]string, 0, len(bg.Spec.InterconnectInterfaceRefs))
		for _, ref := range bg.Spec.InterconnectInterfaceRefs {
			refs = append(refs, ref.Name)
		}
		return refs
	}); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(ctx, &nxv1alpha1.BorderGateway{}, borderGatewayBGPPeerRefKey, func(obj client.Object) []string {
		bg := obj.(*nxv1alpha1.BorderGateway)
		refs := make([]string, 0, len(bg.Spec.BGPPeerRefs))
		for _, ref := range bg.Spec.BGPPeerRefs {
			refs = append(refs, ref.Name)
		}
		return refs
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&nxv1alpha1.BorderGateway{}).
		Named("bordergateway").
		WithEventFilter(filter).
		// Watches enqueues BorderGateways for updates in referenced source Interface resources.
		// Only triggers on create and delete events since interface names are immutable.
		Watches(
			&v1alpha1.Interface{},
			handler.EnqueueRequestsFromMapFunc(r.sourceInterfaceToBorderGateway),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					return false
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			}),
		).
		// Watches enqueues BorderGateways for updates in referenced interconnect Interface resources.
		// Only triggers on create and delete events since interface names are immutable.
		Watches(
			&v1alpha1.Interface{},
			handler.EnqueueRequestsFromMapFunc(r.interconnectInterfaceToBorderGateway),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					return false
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			}),
		).
		// Watches enqueues BorderGateways for updates in referenced BGPPeer resources.
		// Only triggers on create and delete events since BGP peer names are immutable.
		Watches(
			&v1alpha1.BGPPeer{},
			handler.EnqueueRequestsFromMapFunc(r.bgpPeerToBorderGateway),
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
type borderGatewayScope struct {
	Device        *v1alpha1.Device
	BorderGateway *nxv1alpha1.BorderGateway
	Connection    *deviceutil.Connection
	Provider      Provider
}

func (r *BorderGatewayReconciler) reconcile(ctx context.Context, s *borderGatewayScope) (reterr error) {
	if s.BorderGateway.Labels == nil {
		s.BorderGateway.Labels = make(map[string]string)
	}

	s.BorderGateway.Labels[v1alpha1.DeviceLabel] = s.Device.Name

	// Ensure the BorderGateway is owned by the Device.
	if !controllerutil.HasControllerReference(s.BorderGateway) {
		if err := controllerutil.SetOwnerReference(s.Device, s.BorderGateway, r.Scheme, controllerutil.WithBlockOwnerDeletion(true)); err != nil {
			return err
		}
	}

	sourceInterface, err := r.reconcileSourceInterface(ctx, s)
	if err != nil {
		return err
	}

	interconnectInterfaces, err := r.reconcileInterconnectInterfaces(ctx, s)
	if err != nil {
		return err
	}

	bgpPeers, err := r.reconcileBGPPeers(ctx, s)
	if err != nil {
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

	// Ensure the BorderGateway is realized on the provider.
	err = s.Provider.EnsureBorderGatewaySettings(ctx, &nxos.BorderGatewaySettingsRequest{
		BorderGateway:   s.BorderGateway,
		SourceInterface: sourceInterface,
		Interconnects:   interconnectInterfaces,
		Peers:           bgpPeers,
	})

	cond := conditions.FromError(err)
	// As this resource is configuration only, we use the Configured condition as top-level Ready condition.
	cond.Type = v1alpha1.ReadyCondition
	conditions.Set(s.BorderGateway, cond)

	return err
}

// reconcileSourceInterface ensures that the referenced source interface exists, belongs to the same device,
// and is of type Loopback.
func (r *BorderGatewayReconciler) reconcileSourceInterface(ctx context.Context, s *borderGatewayScope) (*v1alpha1.Interface, error) {
	key := client.ObjectKey{
		Name:      s.BorderGateway.Spec.SourceInterfaceRef.Name,
		Namespace: s.BorderGateway.Namespace,
	}

	intf := new(v1alpha1.Interface)
	if err := r.Get(ctx, key, intf); err != nil {
		if apierrors.IsNotFound(err) {
			conditions.Set(s.BorderGateway, metav1.Condition{
				Type:    v1alpha1.ReadyCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.InterfaceNotFoundReason,
				Message: fmt.Sprintf("source interface %q not found", key),
			})
			return nil, reconcile.TerminalError(fmt.Errorf("source interface %q not found", key))
		}
		return nil, fmt.Errorf("failed to get source interface %q: %w", key, err)
	}

	if intf.Spec.DeviceRef.Name != s.Device.Name {
		conditions.Set(s.BorderGateway, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.CrossDeviceReferenceReason,
			Message: fmt.Sprintf("source interface %q does not belong to device %q", intf.Name, s.Device.Name),
		})
		return nil, reconcile.TerminalError(fmt.Errorf("source interface %q does not belong to device %q", intf.Name, s.Device.Name))
	}

	if intf.Spec.Type != v1alpha1.InterfaceTypeLoopback {
		conditions.Set(s.BorderGateway, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.InvalidInterfaceTypeReason,
			Message: fmt.Sprintf("source interface %q is not of type Loopback, got %q", intf.Name, intf.Spec.Type),
		})
		return nil, reconcile.TerminalError(fmt.Errorf("source interface %q is not of type Loopback, got %q", intf.Name, intf.Spec.Type))
	}

	return intf, nil
}

// reconcileInterconnectInterfaces ensures that all interconnect interfaces exist and belong to the same device.
func (r *BorderGatewayReconciler) reconcileInterconnectInterfaces(ctx context.Context, s *borderGatewayScope) ([]nxos.BorderGatewayInterconnect, error) {
	interconnects := make([]nxos.BorderGatewayInterconnect, 0, len(s.BorderGateway.Spec.InterconnectInterfaceRefs))
	for _, ref := range s.BorderGateway.Spec.InterconnectInterfaceRefs {
		intf := new(v1alpha1.Interface)
		if err := r.Get(ctx, client.ObjectKey{Name: ref.Name, Namespace: s.BorderGateway.Namespace}, intf); err != nil {
			if apierrors.IsNotFound(err) {
				conditions.Set(s.BorderGateway, metav1.Condition{
					Type:    v1alpha1.ReadyCondition,
					Status:  metav1.ConditionFalse,
					Reason:  v1alpha1.InterfaceNotFoundReason,
					Message: fmt.Sprintf("interconnect interface %q not found", ref.Name),
				})
				return nil, reconcile.TerminalError(fmt.Errorf("interconnect interface %q not found", ref.Name))
			}
			return nil, fmt.Errorf("failed to get interconnect interface %q: %w", ref.Name, err)
		}

		if intf.Spec.DeviceRef.Name != s.Device.Name {
			conditions.Set(s.BorderGateway, metav1.Condition{
				Type:    v1alpha1.ReadyCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.CrossDeviceReferenceReason,
				Message: fmt.Sprintf("interconnect interface %q does not belong to device %q", intf.Name, s.Device.Name),
			})
			return nil, reconcile.TerminalError(fmt.Errorf("interconnect interface %q does not belong to device %q", intf.Name, s.Device.Name))
		}

		if intf.Spec.Type != v1alpha1.InterfaceTypePhysical {
			conditions.Set(s.BorderGateway, metav1.Condition{
				Type:    v1alpha1.ReadyCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.InvalidInterfaceTypeReason,
				Message: fmt.Sprintf("interconnect interface %q is not of type Physical, got %q", intf.Name, intf.Spec.Type),
			})
			return nil, reconcile.TerminalError(fmt.Errorf("interconnect interface %q is not of type Physical, got %q", intf.Name, intf.Spec.Type))
		}

		interconnects = append(interconnects, nxos.BorderGatewayInterconnect{
			Interface: intf,
			Tracking:  ref.Tracking,
		})
	}

	return interconnects, nil
}

// reconcileBGPPeers ensures that all BGP peers exist and belong to the same device.
func (r *BorderGatewayReconciler) reconcileBGPPeers(ctx context.Context, s *borderGatewayScope) ([]nxos.BorderGatewayPeer, error) {
	peers := make([]nxos.BorderGatewayPeer, 0, len(s.BorderGateway.Spec.BGPPeerRefs))
	for _, ref := range s.BorderGateway.Spec.BGPPeerRefs {
		peer := new(v1alpha1.BGPPeer)
		if err := r.Get(ctx, client.ObjectKey{Name: ref.Name, Namespace: s.BorderGateway.Namespace}, peer); err != nil {
			if apierrors.IsNotFound(err) {
				conditions.Set(s.BorderGateway, metav1.Condition{
					Type:    v1alpha1.ReadyCondition,
					Status:  metav1.ConditionFalse,
					Reason:  v1alpha1.BGPPeerNotFoundReason,
					Message: fmt.Sprintf("BGP peer %q not found", ref.Name),
				})
				return nil, reconcile.TerminalError(fmt.Errorf("BGP peer %q not found", ref.Name))
			}
			return nil, fmt.Errorf("failed to get BGP peer %q: %w", ref.Name, err)
		}

		if peer.Spec.DeviceRef.Name != s.Device.Name {
			conditions.Set(s.BorderGateway, metav1.Condition{
				Type:    v1alpha1.ReadyCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.CrossDeviceReferenceReason,
				Message: fmt.Sprintf("BGP peer %q does not belong to device %q", peer.Name, s.Device.Name),
			})
			return nil, reconcile.TerminalError(fmt.Errorf("BGP peer %q does not belong to device %q", peer.Name, s.Device.Name))
		}

		peers = append(peers, nxos.BorderGatewayPeer{
			BGPPeer:  peer,
			PeerType: ref.PeerType,
		})
	}

	return peers, nil
}

func (r *BorderGatewayReconciler) finalize(ctx context.Context, s *borderGatewayScope) (reterr error) {
	if err := s.Provider.Connect(ctx, s.Connection); err != nil {
		return fmt.Errorf("failed to connect to provider: %w", err)
	}
	defer func() {
		if err := s.Provider.Disconnect(ctx, s.Connection); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	return s.Provider.ResetBorderGatewaySettings(ctx)
}

// sourceInterfaceToBorderGateway is a [handler.MapFunc] to be used to enqueue requests for reconciliation
// for a BorderGateway to update when its referenced source Interface changes.
func (r *BorderGatewayReconciler) sourceInterfaceToBorderGateway(ctx context.Context, obj client.Object) []ctrl.Request {
	intf, ok := obj.(*v1alpha1.Interface)
	if !ok {
		panic(fmt.Sprintf("Expected an Interface but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx, "Source Interface", klog.KObj(intf))

	bgws := new(nxv1alpha1.BorderGatewayList)
	if err := r.List(ctx, bgws, client.InNamespace(intf.Namespace), client.MatchingFields{borderGatewaySourceInterfaceRefKey: intf.Spec.Name}); err != nil {
		log.Error(err, "Failed to list BorderGateways")
		return nil
	}

	requests := []ctrl.Request{}
	for _, bg := range bgws.Items {
		if bg.Spec.SourceInterfaceRef.Name == intf.Name {
			log.Info("Enqueuing BorderGateway for reconciliation", "BorderGateway", klog.KObj(&bg))
			requests = append(requests, ctrl.Request{
				NamespacedName: client.ObjectKey{
					Name:      bg.Name,
					Namespace: bg.Namespace,
				},
			})
		}
	}

	return requests
}

// interconnectInterfaceToBorderGateway is a [handler.MapFunc] to be used to enqueue requests for reconciliation
// for a BorderGateway to update when one of its referenced interconnect interfaces changes.
func (r *BorderGatewayReconciler) interconnectInterfaceToBorderGateway(ctx context.Context, obj client.Object) []ctrl.Request {
	intf, ok := obj.(*v1alpha1.Interface)
	if !ok {
		panic(fmt.Sprintf("Expected an Interface but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx, "Interconnect Interface", klog.KObj(intf))

	bgws := new(nxv1alpha1.BorderGatewayList)
	if err := r.List(ctx, bgws, client.InNamespace(intf.Namespace), client.MatchingFields{borderGatewayInterconnectInterfaceRefKey: intf.Spec.Name}); err != nil {
		log.Error(err, "Failed to list BorderGateways")
		return nil
	}

	requests := []ctrl.Request{}
	for _, bg := range bgws.Items {
		if slices.ContainsFunc(bg.Spec.InterconnectInterfaceRefs, func(ref nxv1alpha1.InterconnectInterfaceReference) bool {
			return ref.Name == intf.Name
		}) {
			log.Info("Enqueuing BorderGateway for reconciliation", "BorderGateway", klog.KObj(&bg))
			requests = append(requests, ctrl.Request{
				NamespacedName: client.ObjectKey{
					Name:      bg.Name,
					Namespace: bg.Namespace,
				},
			})
		}
	}

	return requests
}

// bgpPeerToBorderGateway is a [handler.MapFunc] to be used to enqueue requests for reconciliation
// for a BorderGateway to update when one of its referenced BGP peers changes.
func (r *BorderGatewayReconciler) bgpPeerToBorderGateway(ctx context.Context, obj client.Object) []ctrl.Request {
	peer, ok := obj.(*v1alpha1.BGPPeer)
	if !ok {
		panic(fmt.Sprintf("Expected a BGPPeer but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx, "BGP Peer", klog.KObj(peer))

	bgws := new(nxv1alpha1.BorderGatewayList)
	if err := r.List(ctx, bgws, client.InNamespace(peer.Namespace), client.MatchingFields{borderGatewayBGPPeerRefKey: peer.Name}); err != nil {
		log.Error(err, "Failed to list BorderGateways")
		return nil
	}

	requests := []ctrl.Request{}
	for _, bg := range bgws.Items {
		if slices.ContainsFunc(bg.Spec.BGPPeerRefs, func(ref nxv1alpha1.BGPPeerReference) bool {
			return ref.Name == peer.Name
		}) {
			log.Info("Enqueuing BorderGateway for reconciliation", "BorderGateway", klog.KObj(&bg))
			requests = append(requests, ctrl.Request{
				NamespacedName: client.ObjectKey{
					Name:      bg.Name,
					Namespace: bg.Namespace,
				},
			})
		}
	}

	return requests
}
