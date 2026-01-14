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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/ironcore-dev/network-operator/internal/conditions"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/resourcelock"

	nxv1 "github.com/ironcore-dev/network-operator/api/cisco/nx/v1alpha1"
	corev1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	controllercore "github.com/ironcore-dev/network-operator/internal/controller/core"
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
	Recorder record.EventRecorder

	// Provider is the driver that will be used to create & delete the vPC
	Provider provider.ProviderFunc

	// Locker is used to synchronize operations on resources targeting the same device.
	Locker *resourcelock.ResourceLocker

	// RequeueInterval is the duration after which the controller should requeue the reconciliation,
	// regardless of changes.
	RequeueInterval time.Duration
}

// // scope holds the different objects that are read and used during the reconcile.
type vpcdomainScope struct {
	Device     *corev1.Device
	VPCDomain  *nxv1.VPCDomain
	Connection *deviceutil.Connection
	Provider   Provider
	// VRF is the VRF referenced in the KeepAlive configuration
	VRF *corev1.VRF
	// PeerLink is the port-channel interface referenced as the peer link
	PeerLink *corev1.Interface
}

// +kubebuilder:rbac:groups=nx.cisco.networking.metal.ironcore.dev,resources=vpcdomains,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=nx.cisco.networking.metal.ironcore.dev,resources=vpcdomains/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=nx.cisco.networking.metal.ironcore.dev,resources=vpcdomains/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *VPCDomainReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := logf.FromContext(ctx)
	log.Info("Reconciling resource")

	obj := new(nxv1.VPCDomain)
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("VPCDomain resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get resource")
		return ctrl.Result{}, err
	}

	prov, ok := r.Provider().(Provider)
	if !ok {
		meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
			Type:    corev1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  corev1.NotImplementedReason,
			Message: "Provider does not implement provider.VPCDomainProvider",
		})
		return ctrl.Result{}, r.Status().Update(ctx, obj)
	}

	device, err := deviceutil.GetDeviceByName(ctx, r, obj.Namespace, obj.Spec.DeviceRef.Name)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := r.Locker.AcquireLock(ctx, device.Name, "cisco-nx-vpcdomain-controller"); err != nil {
		if errors.Is(err, resourcelock.ErrLockAlreadyHeld) {
			log.Info("Device is already locked, requeuing reconciliation")
			return ctrl.Result{RequeueAfter: time.Second * 5}, nil
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
		if controllerutil.ContainsFinalizer(obj, corev1.FinalizerName) {
			if err := r.finalize(ctx, s); err != nil {
				log.Error(err, "Failed to finalize resource")
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(obj, corev1.FinalizerName)
			if err := r.Update(ctx, obj); err != nil {
				log.Error(err, "Failed to remove finalizer from resource")
				return ctrl.Result{}, err
			}
		}
		log.Info("Resource is being deleted, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(obj, corev1.FinalizerName) {
		controllerutil.AddFinalizer(obj, corev1.FinalizerName)
		if err := r.Update(ctx, obj); err != nil {
			log.Error(err, "Failed to add finalizer to resource")
			return ctrl.Result{}, err
		}
		log.Info("Added finalizer to resource")
		return ctrl.Result{}, nil
	}

	orig := obj.DeepCopy()
	if conditions.InitializeConditions(obj, corev1.ReadyCondition) {
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

	err = r.reconcile(ctx, s)
	if err != nil {
		log.Error(err, "Failed to reconcile resource")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: controllercore.Jitter(r.RequeueInterval)}, nil
}

// reconcile contains the main reconciliation logic for the VPCDomain resource.
func (r *VPCDomainReconciler) reconcile(ctx context.Context, s *vpcdomainScope) (reterr error) {
	if s.VPCDomain.Labels == nil {
		s.VPCDomain.Labels = make(map[string]string)
	}
	s.VPCDomain.Labels[corev1.DeviceLabel] = s.Device.Name

	// Ensure owner reference to device
	if !controllerutil.HasControllerReference(s.VPCDomain) {
		if err := controllerutil.SetOwnerReference(s.Device, s.VPCDomain, r.Scheme, controllerutil.WithBlockOwnerDeletion(true)); err != nil {
			return err
		}
	}

	// Connect to remote device
	var connErr error
	if connErr = s.Provider.Connect(ctx, s.Connection); connErr != nil {
		r.resetStatus(ctx, &s.VPCDomain.Status)
		return kerrors.NewAggregate([]error{reterr, fmt.Errorf("failed to connect to provider: %w", connErr)})
	}
	defer func() {
		if err := s.Provider.Disconnect(ctx, s.Connection); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	defer func() {
		conditions.RecomputeReady(s.VPCDomain)
	}()

	// Reconcile referenced resources (validate and update vpcdomainScope)
	if err := r.reconcilePeerLink(ctx, s); err != nil {
		reterr = kerrors.NewAggregate([]error{reterr, fmt.Errorf("failed to reconcile referenced resource: %w", err)})
	}
	if err := r.reconcileKeepAliveVRF(ctx, s); err != nil {
		reterr = kerrors.NewAggregate([]error{reterr, fmt.Errorf("failed to reconcile referenced resource: %w", err)})
	}
	if reterr != nil {
		return reconcile.TerminalError(reterr)
	}

	//  Realize the vPC via the provider and update configuration status
	err := s.Provider.EnsureVPCDomain(ctx, s.VPCDomain, s.VRF, s.PeerLink)
	cond := conditions.FromError(err)
	conditions.Set(s.VPCDomain, cond)
	if err != nil {
		reterr = kerrors.NewAggregate([]error{reterr, fmt.Errorf("failed to reconcile resource: %w", err)})
	}

	// Retrieve and update status from the device, nil out on error
	status, err := s.Provider.GetStatusVPCDomain(ctx)
	if err != nil {
		r.resetStatus(ctx, &s.VPCDomain.Status)
		reterr = kerrors.NewAggregate([]error{reterr, fmt.Errorf("failed to get resource status: %w", err)})
	}

	if reterr != nil {
		return reterr
	}

	s.VPCDomain.Status.Role = status.Role

	slices.Sort(status.KeepAliveStatusMsg)
	s.VPCDomain.Status.KeepAliveStatusMsg = status.KeepAliveStatusMsg
	s.VPCDomain.Status.KeepAliveStatus = nxv1.StatusDown
	if status.KeepAliveStatus {
		s.VPCDomain.Status.KeepAliveStatus = nxv1.StatusUp
	}
	slices.Sort(s.VPCDomain.Status.KeepAliveStatusMsg)
	s.VPCDomain.Status.PeerStatusMsg = status.PeerStatusMsg
	s.VPCDomain.Status.PeerStatus = nxv1.StatusDown
	if status.PeerStatus {
		s.VPCDomain.Status.PeerStatus = nxv1.StatusUp
	}
	s.VPCDomain.Status.PeerUptime = metav1.Duration{Duration: status.PeerUptime}

	// Fetch the status of the port-channel forming the vPC's peer link
	peerlinkOperSt := false
	if s.PeerLink != nil {
		s.VPCDomain.Status.PeerLinkIf = s.PeerLink.Spec.Name
		if c := meta.FindStatusCondition(s.PeerLink.Status.Conditions, corev1.OperationalCondition); c != nil {
			s.VPCDomain.Status.PeerLinkIfOperStatus = nxv1.StatusDown
			if c.Status == metav1.ConditionTrue {
				s.VPCDomain.Status.PeerLinkIfOperStatus = nxv1.StatusUp
				peerlinkOperSt = true
			}
		}
	}

	cond = metav1.Condition{
		Type:    corev1.OperationalCondition,
		Status:  metav1.ConditionTrue,
		Reason:  corev1.OperationalReason,
		Message: "vPC domain is operational",
	}
	// See comment in this type's status definition for details on the operational condition.
	if !status.PeerStatus || !status.KeepAliveStatus || !peerlinkOperSt {
		cond = metav1.Condition{
			Type:    corev1.OperationalCondition,
			Status:  metav1.ConditionFalse,
			Reason:  corev1.DegradedReason,
			Message: "vPC domain is not fully operational. Either the peer-link interface is down, keepalive is not working, or the peer adjacency is not correctly formed.",
		}
	}
	conditions.Set(s.VPCDomain, cond)

	return reterr
}

// reconcilePeerLink checks that peer's interface reference exists and is of type Aggregate.
// Updates the scope accordingly.
// Ignores the interface status: Port-channels require the domain to be configured first.
func (r *VPCDomainReconciler) reconcilePeerLink(ctx context.Context, s *vpcdomainScope) error {
	intf := new(corev1.Interface)
	intf.Name = s.VPCDomain.Spec.Peer.InterfaceRef.Name
	intf.Namespace = s.VPCDomain.Namespace

	if err := r.Get(ctx, client.ObjectKey{Name: intf.Name, Namespace: intf.Namespace}, intf); err != nil {
		if apierrors.IsNotFound(err) {
			conditions.Set(s.VPCDomain, metav1.Condition{
				Type:    corev1.ConfiguredCondition,
				Status:  metav1.ConditionFalse,
				Reason:  corev1.WaitingForDependenciesReason,
				Message: fmt.Sprintf("interface resource '%s' not found in namespace '%s'", intf.Name, intf.Namespace),
			})
			return fmt.Errorf("member interface %s not found", intf.Name)
		}
		return fmt.Errorf("failed to get member interface %q: %w", intf.Name, err)
	}

	if intf.Spec.Type != corev1.InterfaceTypeAggregate && intf.Spec.Type != corev1.InterfaceTypePhysical {
		conditions.Set(s.VPCDomain, metav1.Condition{
			Type:    corev1.ConfiguredCondition,
			Status:  metav1.ConditionFalse,
			Reason:  corev1.InvalidInterfaceTypeReason,
			Message: fmt.Sprintf("interface referenced by '%s' must be of type %q or type %q", intf.Name, corev1.InterfaceTypeAggregate, corev1.InterfaceTypePhysical),
		})
		return fmt.Errorf("interface referenced by '%s' must be of type %q or type %q", intf.Name, corev1.InterfaceTypeAggregate, corev1.InterfaceTypePhysical)
	}

	if s.VPCDomain.Spec.DeviceRef.Name != intf.Spec.DeviceRef.Name {
		conditions.Set(s.VPCDomain, metav1.Condition{
			Type:    corev1.ConfiguredCondition,
			Status:  metav1.ConditionFalse,
			Reason:  corev1.CrossDeviceReferenceReason,
			Message: fmt.Sprintf("interface '%s' deviceRef '%s' does not match vPC deviceRef '%s'", intf.Name, intf.Spec.DeviceRef.Name, s.VPCDomain.Spec.DeviceRef.Name),
		})
		return fmt.Errorf("interface '%s' deviceRef '%s' does not match vPC deviceRef '%s'", intf.Name, intf.Spec.DeviceRef.Name, s.VPCDomain.Spec.DeviceRef.Name)
	}
	s.PeerLink = intf
	return nil
}

// reconcileKeepAliveVRF ensures that the referenced VRF resource exists
// Updates the scope accordingly.
func (r *VPCDomainReconciler) reconcileKeepAliveVRF(ctx context.Context, s *vpcdomainScope) error {
	if s.VPCDomain.Spec.Peer.KeepAlive.VRFRef == nil {
		return nil
	}
	vrf := new(corev1.VRF)
	vrf.Name = s.VPCDomain.Spec.Peer.KeepAlive.VRFRef.Name
	vrf.Namespace = s.VPCDomain.Namespace

	if err := r.Get(ctx, client.ObjectKey{Name: vrf.Name, Namespace: vrf.Namespace}, vrf); err != nil {
		if apierrors.IsNotFound(err) {
			conditions.Set(s.VPCDomain, metav1.Condition{
				Type:    corev1.ConfiguredCondition,
				Status:  metav1.ConditionFalse,
				Reason:  corev1.WaitingForDependenciesReason,
				Message: fmt.Sprintf("VRF resource '%s' not found in namespace '%s'", vrf.Name, vrf.Namespace),
			})
			return fmt.Errorf("VRF %q not found", vrf.Name)
		}
		return fmt.Errorf("failed to get VRF %q: %w", vrf.Name, err)
	}

	if s.VPCDomain.Spec.DeviceRef.Name != vrf.Spec.DeviceRef.Name {
		conditions.Set(s.VPCDomain, metav1.Condition{
			Type:    corev1.ConfiguredCondition,
			Status:  metav1.ConditionFalse,
			Reason:  corev1.CrossDeviceReferenceReason,
			Message: fmt.Sprintf("VRF '%s' deviceRef '%s' does not match VPCDomain deviceRef '%s'", vrf.Name, vrf.Spec.DeviceRef.Name, s.VPCDomain.Spec.DeviceRef.Name),
		})
		return fmt.Errorf("VRF '%s' deviceRef '%s' does not match VPCDomain deviceRef '%s'", vrf.Name, vrf.Spec.DeviceRef.Name, s.VPCDomain.Spec.DeviceRef.Name)
	}

	s.VRF = vrf
	return nil
}

// resetStatus resets the status fields of the VPCDomain resource.
func (r *VPCDomainReconciler) resetStatus(_ context.Context, s *nxv1.VPCDomainStatus) {
	s.PeerStatus = nxv1.StatusUnknown
	s.PeerStatusMsg = []string{}
	s.Role = nxv1.VPCDomainRoleUnknown
	s.PeerUptime = metav1.Duration{}
}

// SetupWithManager sets up the controller with the Manager.
func (r *VPCDomainReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	labelSelector := metav1.LabelSelector{}
	if r.WatchFilterValue != "" {
		labelSelector.MatchLabels = map[string]string{corev1.WatchLabel: r.WatchFilterValue}
	}

	filter, err := predicate.LabelSelectorPredicate(labelSelector)
	if err != nil {
		return fmt.Errorf("failed to create label selector predicate: %w", err)
	}

	// Index vPCs by their peer interface reference
	if err := mgr.GetFieldIndexer().IndexField(ctx, &nxv1.VPCDomain{}, ".spec.peer.interfaceRef.name", func(obj client.Object) []string {
		vpc := obj.(*nxv1.VPCDomain)
		return []string{vpc.Spec.Peer.InterfaceRef.Name}
	}); err != nil {
		return err
	}

	// Index vPCs by their device reference
	if err := mgr.GetFieldIndexer().IndexField(ctx, &nxv1.VPCDomain{}, ".spec.deviceRef.name", func(obj client.Object) []string {
		vpc := obj.(*nxv1.VPCDomain)
		return []string{vpc.Spec.DeviceRef.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&nxv1.VPCDomain{}).
		Named("vpcdomain").
		WithEventFilter(filter).
		// Trigger reconciliation for changes in the operational status of the referenced interface: The device can shut down the port-channel by itself
		// in certain failure scenarios, e.g., incompatible configuration.
		Watches(
			&corev1.Interface{},
			handler.EnqueueRequestsFromMapFunc(r.mapInterfaceToVPCDomain),
			builder.WithPredicates(predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool {
					iface := e.Object.(*corev1.Interface)
					return iface.Spec.Type == corev1.InterfaceTypeAggregate || iface.Spec.Type == corev1.InterfaceTypePhysical
				},
				DeleteFunc: func(e event.DeleteEvent) bool {
					iface := e.Object.(*corev1.Interface)
					return iface.Spec.Type == corev1.InterfaceTypeAggregate || iface.Spec.Type == corev1.InterfaceTypePhysical
				},
				UpdateFunc: func(e event.UpdateEvent) bool {
					o := e.ObjectOld.(*corev1.Interface)
					n := e.ObjectNew.(*corev1.Interface)
					return (n.Spec.Type == corev1.InterfaceTypeAggregate || n.Spec.Type == corev1.InterfaceTypePhysical) && conditionChanged(o.Status.Conditions, n.Status.Conditions, corev1.OperationalCondition)
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			}),
		).
		Complete(r)
}

func (r *VPCDomainReconciler) mapInterfaceToVPCDomain(ctx context.Context, obj client.Object) []ctrl.Request {
	iface, ok := obj.(*corev1.Interface)
	if !ok {
		panic(fmt.Sprintf("Expected a Interface but got a %T", obj))
	}

	vpc := new(nxv1.VPCDomain)
	var vpcs nxv1.VPCDomainList
	if err := r.List(ctx, &vpcs,
		client.InNamespace(vpc.Namespace),
		client.MatchingFields{
			".spec.peer.interfaceRef.name": iface.Name,
			".spec.deviceRef.name":         iface.Spec.DeviceRef.Name,
		},
	); err != nil {
		return nil
	}

	requests := make([]reconcile.Request, 0, len(vpcs.Items))
	for i := range vpcs.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(&vpcs.Items[i]),
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

func conditionChanged(oldConds, newConds []metav1.Condition, t string) bool {
	o := meta.FindStatusCondition(oldConds, t)
	n := meta.FindStatusCondition(newConds, t)
	if o == nil && n == nil {
		return false
	}
	if o == nil || n == nil {
		return true
	}
	return o.Status != n.Status
}
