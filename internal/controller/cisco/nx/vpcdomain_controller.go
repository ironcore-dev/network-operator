// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nx

import (
	"context"
	"fmt"
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
}

// +kubebuilder:rbac:groups=nx.cisco.networking.metal.ironcore.dev,resources=vpcdomains,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=nx.cisco.networking.metal.ironcore.dev,resources=vpcdomains/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=nx.cisco.networking.metal.ironcore.dev,resources=vpcdomains/finalizers,verbs=update
// +kubebuilder:rbac:groups=nx.cisco.networking.metal.ironcore.dev,resources=events,verbs=create;patch

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
		if controllerutil.ContainsFinalizer(obj, nxv1.FinalizerName) {
			if err := r.finalize(ctx, s); err != nil {
				log.Error(err, "Failed to finalize resource")
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(obj, nxv1.FinalizerName)
			if err := r.Update(ctx, obj); err != nil {
				log.Error(err, "Failed to remove finalizer from resource")
				return ctrl.Result{}, err
			}
		}
		log.Info("Resource is being deleted, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(obj, nxv1.FinalizerName) {
		controllerutil.AddFinalizer(obj, nxv1.FinalizerName)
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

	defer func() {
		conditions.RecomputeReady(s.VPCDomain)
	}()

	// Validate refs but don't return early, we want to update .VPCDomain.status fields with data from the remote device state
	if err := r.validateInterfaceRef(ctx, s); err != nil {
		reterr = kerrors.NewAggregate([]error{reterr, fmt.Errorf("failed to validate peer interface reference: %w", err)})
	}
	if err := r.validateVRFRef(ctx, s); err != nil {
		reterr = kerrors.NewAggregate([]error{reterr, fmt.Errorf("failed to validate KeepAlive VRF reference: %w", err)})
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

	//  Realize the vPC via the provider and update configuration status
	err := s.Provider.EnsureVPCDomain(ctx, s.VPCDomain, s.VRF)
	cond := conditions.FromError(err)
	conditions.Set(s.VPCDomain, cond)
	if err != nil {
		reterr = kerrors.NewAggregate([]error{reterr, fmt.Errorf("vpc: failed to ensure vPC domain configuration: %w", err)})
	}

	// Retrieve and update operational status and nil out the status on error to avoid stale state
	status, err := s.Provider.GetStatusVPCDomain(ctx)
	if err != nil {
		r.resetStatus(ctx, &s.VPCDomain.Status)
		return kerrors.NewAggregate([]error{reterr, fmt.Errorf("failed to get interface status: %w", err)})
	}

	s.VPCDomain.Status.Role = status.Role
	s.VPCDomain.Status.PeerUptime = metav1.Duration{Duration: status.PeerUptime}

	cond = metav1.Condition{
		Type:    corev1.OperationalCondition,
		Status:  metav1.ConditionTrue,
		Reason:  corev1.OperationalReason,
		Message: "vPC is up",
	}
	if !status.KeepAliveStatus {
		cond.Status = metav1.ConditionFalse
		cond.Reason = corev1.DegradedReason
		cond.Message = "vPC is down"
	}
	if status.KeepAliveStatusMessage != "" {
		cond.Message = fmt.Sprintf("%s, device returned %q", cond.Message, status.KeepAliveStatusMessage)
	}
	conditions.Set(s.VPCDomain, cond)

	return reterr
}

// validateInterfaceRef validates that the peer's interface reference exists and is of type Aggregate.
// Must ignore aggregate status: Port-channels require the domain to be configured first.
func (r *VPCDomainReconciler) validateInterfaceRef(ctx context.Context, s *vpcdomainScope) error {
	intf := new(corev1.Interface)
	intf.Name = s.VPCDomain.Spec.Peer.InterfaceAggregateRef.Name
	intf.Namespace = s.VPCDomain.Namespace

	if err := r.Get(ctx, client.ObjectKey{Name: intf.Name, Namespace: intf.Namespace}, intf); err != nil {
		if apierrors.IsNotFound(err) {
			conditions.Set(s.VPCDomain, metav1.Condition{
				Type:    corev1.ConfiguredCondition,
				Status:  metav1.ConditionFalse,
				Reason:  corev1.WaitingForDependenciesReason,
				Message: fmt.Sprintf("interface resource '%s' not found in namespace '%s'", intf.Name, intf.Namespace),
			})
			return fmt.Errorf("member interface %q not found", intf.Name)
		}
		return fmt.Errorf("failed to get member interface %q: %w", intf.Name, err)
	}

	if intf.Spec.Type != corev1.InterfaceTypeAggregate {
		conditions.Set(s.VPCDomain, metav1.Condition{
			Type:    corev1.ConfiguredCondition,
			Status:  metav1.ConditionFalse,
			Reason:  corev1.InvalidInterfaceTypeReason,
			Message: fmt.Sprintf("interface referenced by '%s' must be of type %q", intf.Name, corev1.InterfaceTypeAggregate),
		})
		return fmt.Errorf("interface referenced by '%s' must be of type %q", intf.Name, corev1.InterfaceTypeAggregate)
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
	return nil
}

// validateVRFRef validates the VRF reference in the KeepAlive configuration, and updates the scope accordingly.
func (r *VPCDomainReconciler) validateVRFRef(ctx context.Context, s *vpcdomainScope) error {
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

func (r *VPCDomainReconciler) resetStatus(_ context.Context, s *nxv1.VPCDomainStatus) {
	s.Role = nxv1.VPCDomainRoleUnknown
	s.PeerUptime = metav1.Duration{Duration: 0}
}

// SetupWithManager sets up the controller with the Manager.
func (r *VPCDomainReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	labelSelector := metav1.LabelSelector{}
	if r.WatchFilterValue != "" {
		labelSelector.MatchLabels = map[string]string{nxv1.WatchLabel: r.WatchFilterValue}
	}

	filter, err := predicate.LabelSelectorPredicate(labelSelector)
	if err != nil {
		return fmt.Errorf("failed to create label selector predicate: %w", err)
	}

	// Note: interface type indexer already defined in a different controller

	// Index vPCs by their peer interface reference
	if err := mgr.GetFieldIndexer().IndexField(ctx, &nxv1.VPCDomain{}, ".spec.peer.interfaceAggregateRef.name", func(obj client.Object) []string {
		vpc := obj.(*nxv1.VPCDomain)
		return []string{vpc.Spec.Peer.InterfaceAggregateRef.Name}
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
		Named("vpc").
		WithEventFilter(filter).
		// Trigger reconciliation also for updates, e.g., if port-channel goes down
		Watches(
			&corev1.Interface{},
			handler.EnqueueRequestsFromMapFunc(r.mapAggregateToVPCDomain),
			builder.WithPredicates(predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool {
					// Only trigger for Aggregate type
					iface := e.Object.(*corev1.Interface)
					return iface.Spec.Type == corev1.InterfaceTypeAggregate
				},
				DeleteFunc: func(e event.DeleteEvent) bool {
					iface := e.Object.(*corev1.Interface)
					return iface.Spec.Type == corev1.InterfaceTypeAggregate
				},
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldIface := e.ObjectOld.(*corev1.Interface)
					newIface := e.ObjectNew.(*corev1.Interface)
					return newIface.Spec.Type == corev1.InterfaceTypeAggregate &&
						!equality.Semantic.DeepEqual(oldIface.Status, newIface.Status)
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			}),
		).
		// Trigger reconciliation if the referenced VRF name changes
		Watches(
			&corev1.VRF{},
			handler.EnqueueRequestsFromMapFunc(r.mapVRFToVPCDomain),
			builder.WithPredicates(predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool {
					return true
				},
				DeleteFunc: func(e event.DeleteEvent) bool {
					return true
				},
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldVRF := e.ObjectOld.(*corev1.VRF)
					newVRF := e.ObjectNew.(*corev1.VRF)
					return oldVRF.Spec.Name != newVRF.Spec.Name ||
						!equality.Semantic.DeepEqual(oldVRF.Status, newVRF.Status)
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			}),
		).
		Complete(r)
}

func (r *VPCDomainReconciler) mapAggregateToVPCDomain(ctx context.Context, obj client.Object) []ctrl.Request {
	iface, ok := obj.(*corev1.Interface)
	if !ok {
		panic(fmt.Sprintf("Expected a Interface but got a %T", obj))
	}

	vpc := new(nxv1.VPCDomain)
	var vpcs nxv1.VPCDomainList
	if err := r.List(ctx, &vpcs,
		client.InNamespace(vpc.Namespace),
		client.MatchingFields{
			".spec.peer.interfaceAggregateRef.name": iface.Name,
			".spec.deviceRef.name":                  iface.Spec.DeviceRef.Name,
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

func (r *VPCDomainReconciler) mapVRFToVPCDomain(ctx context.Context, obj client.Object) []ctrl.Request {
	vrf, ok := obj.(*corev1.VRF)
	if !ok {
		panic(fmt.Sprintf("Expected a VRF but got a %T", obj))
	}

	vpc := new(nxv1.VPCDomain)
	var vpcs nxv1.VPCDomainList
	if err := r.List(ctx, &vpcs,
		client.InNamespace(vpc.Namespace),
		client.MatchingFields{
			".spec.peer.keepAlive.vrfRef.name": vrf.Name,
			".spec.deviceRef.name":             vrf.Spec.DeviceRef.Name,
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
