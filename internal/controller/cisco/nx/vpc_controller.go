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

// VPCReconciler reconciles a VPC object
type VPCReconciler struct {
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
type vpcScope struct {
	Device     *corev1.Device
	VPC        *nxv1.VPC
	Connection *deviceutil.Connection
	Provider   Provider
	// VRF is the VRF referenced in the KeepAlive configuration
	VRF *corev1.VRF
}

// +kubebuilder:rbac:groups=nx.cisco.networking.metal.ironcore.dev,resources=vpcs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=nx.cisco.networking.metal.ironcore.dev,resources=vpcs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=nx.cisco.networking.metal.ironcore.dev,resources=vpcs/finalizers,verbs=update
// +kubebuilder:rbac:groups=nx.cisco.networking.metal.ironcore.dev,resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *VPCReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := logf.FromContext(ctx)
	log.Info("Reconciling resource")

	obj := new(nxv1.VPC)
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("VPC resource not found. Ignoring since object must be deleted.")
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
			Message: "Provider does not implement provider.VPCProvider",
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

	s := &vpcScope{
		Device:     device,
		VPC:        obj,
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

// reconcile contains the main reconciliation logic for the VPC resource.
func (r *VPCReconciler) reconcile(ctx context.Context, s *vpcScope) (reterr error) {
	if s.VPC.Labels == nil {
		s.VPC.Labels = make(map[string]string)
	}
	s.VPC.Labels[corev1.DeviceLabel] = s.Device.Name

	// Ensure owner reference to device
	if !controllerutil.HasControllerReference(s.VPC) {
		if err := controllerutil.SetOwnerReference(s.Device, s.VPC, r.Scheme, controllerutil.WithBlockOwnerDeletion(true)); err != nil {
			return err
		}
	}

	defer func() {
		conditions.RecomputeReady(s.VPC)
	}()

	// Validate refs but don't return early, we want to update .VPC.status fields with data from the remote device state
	if err := r.validateInterfaceRef(ctx, s); err != nil {
		reterr = kerrors.NewAggregate([]error{reterr, fmt.Errorf("vpc: failed to validate peer interface reference: %w", err)})
	}
	if err := r.validateVRFRef(ctx, s); err != nil {
		reterr = kerrors.NewAggregate([]error{reterr, fmt.Errorf("vpc: failed to validate KeepAlive VRF reference: %w", err)})
	}

	// Connect to remote device
	var connErr error
	if connErr = s.Provider.Connect(ctx, s.Connection); connErr != nil {
		r.resetStatus(ctx, &s.VPC.Status)
		return kerrors.NewAggregate([]error{reterr, fmt.Errorf("failed to connect to provider: %w", connErr)})
	}
	defer func() {
		if err := s.Provider.Disconnect(ctx, s.Connection); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	//  Realize the vPC via the provider and update configuration status
	err := s.Provider.EnsureVPC(ctx, s.VPC, s.VRF)
	cond := conditions.FromError(err)
	conditions.Set(s.VPC, cond)
	if err != nil {
		reterr = kerrors.NewAggregate([]error{reterr, fmt.Errorf("vpc: failed to ensure vPC configuration: %w", err)})
	}

	// Retrieve and update operational status and nil out the status on error to avoid stale state
	status, err := s.Provider.GetStatusVPC(ctx)
	if err != nil {
		r.resetStatus(ctx, &s.VPC.Status)
		return kerrors.NewAggregate([]error{reterr, fmt.Errorf("failed to get interface status: %w", err)})
	}

	s.VPC.Status.Role = status.Role
	s.VPC.Status.PeerUptime = metav1.Duration{Duration: status.PeerUptime}

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
	conditions.Set(s.VPC, cond)

	return reterr
}

// validateInterfaceRef validates that the peer's interface reference exists and is of type Aggregate.
// Must ignore aggregate status: Port-channels require the domain to be configured first.
func (r *VPCReconciler) validateInterfaceRef(ctx context.Context, s *vpcScope) error {
	intf := new(corev1.Interface)
	intf.Name = s.VPC.Spec.Peer.InterfaceAggregateRef.Name
	intf.Namespace = s.VPC.Namespace

	if err := r.Get(ctx, client.ObjectKey{Name: intf.Name, Namespace: intf.Namespace}, intf); err != nil {
		if apierrors.IsNotFound(err) {
			conditions.Set(s.VPC, metav1.Condition{
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
		conditions.Set(s.VPC, metav1.Condition{
			Type:    corev1.ConfiguredCondition,
			Status:  metav1.ConditionFalse,
			Reason:  corev1.InvalidInterfaceTypeReason,
			Message: fmt.Sprintf("interface referenced by '%s' must be of type %q", intf.Name, corev1.InterfaceTypeAggregate),
		})
		return fmt.Errorf("interface referenced by '%s' must be of type %q", intf.Name, corev1.InterfaceTypeAggregate)
	}

	if s.VPC.Spec.DeviceRef.Name != intf.Spec.DeviceRef.Name {
		conditions.Set(s.VPC, metav1.Condition{
			Type:    corev1.ConfiguredCondition,
			Status:  metav1.ConditionFalse,
			Reason:  corev1.CrossDeviceReferenceReason,
			Message: fmt.Sprintf("interface '%s' deviceRef '%s' does not match vPC deviceRef '%s'", intf.Name, intf.Spec.DeviceRef.Name, s.VPC.Spec.DeviceRef.Name),
		})
		return fmt.Errorf("interface '%s' deviceRef '%s' does not match vPC deviceRef '%s'", intf.Name, intf.Spec.DeviceRef.Name, s.VPC.Spec.DeviceRef.Name)
	}
	return nil
}

// validateVRFRef validates the VRF reference in the KeepAlive configuration, and updates the scope accordingly.
func (r *VPCReconciler) validateVRFRef(ctx context.Context, s *vpcScope) error {
	if s.VPC.Spec.Peer.KeepAlive.VRFRef == nil {
		return nil
	}

	vrf := new(corev1.VRF)
	vrf.Name = s.VPC.Spec.Peer.KeepAlive.VRFRef.Name
	vrf.Namespace = s.VPC.Namespace

	if err := r.Get(ctx, client.ObjectKey{Name: vrf.Name, Namespace: vrf.Namespace}, vrf); err != nil {
		if apierrors.IsNotFound(err) {
			conditions.Set(s.VPC, metav1.Condition{
				Type:    corev1.ConfiguredCondition,
				Status:  metav1.ConditionFalse,
				Reason:  corev1.WaitingForDependenciesReason,
				Message: fmt.Sprintf("VRF resource '%s' not found in namespace '%s'", vrf.Name, vrf.Namespace),
			})
			return fmt.Errorf("VRF %q not found", vrf.Name)
		}
		return fmt.Errorf("failed to get VRF %q: %w", vrf.Name, err)
	}

	if s.VPC.Spec.DeviceRef.Name != vrf.Spec.DeviceRef.Name {
		conditions.Set(s.VPC, metav1.Condition{
			Type:    corev1.ConfiguredCondition,
			Status:  metav1.ConditionFalse,
			Reason:  corev1.CrossDeviceReferenceReason,
			Message: fmt.Sprintf("VRF '%s' deviceRef '%s' does not match VPC deviceRef '%s'", vrf.Name, vrf.Spec.DeviceRef.Name, s.VPC.Spec.DeviceRef.Name),
		})
		return fmt.Errorf("VRF '%s' deviceRef '%s' does not match VPC deviceRef '%s'", vrf.Name, vrf.Spec.DeviceRef.Name, s.VPC.Spec.DeviceRef.Name)
	}

	s.VRF = vrf
	return nil
}

func (r *VPCReconciler) resetStatus(_ context.Context, s *nxv1.VPCStatus) {
	s.Role = nxv1.VPCRoleUnknown
	s.PeerUptime = metav1.Duration{Duration: 0}
}

// SetupWithManager sets up the controller with the Manager.
func (r *VPCReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
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
	if err := mgr.GetFieldIndexer().IndexField(ctx, &nxv1.VPC{}, ".spec.peer.interfaceAggregateRef.name", func(obj client.Object) []string {
		vpc := obj.(*nxv1.VPC)
		return []string{vpc.Spec.Peer.InterfaceAggregateRef.Name}
	}); err != nil {
		return err
	}

	// Index vPCs by their device reference
	if err := mgr.GetFieldIndexer().IndexField(ctx, &nxv1.VPC{}, ".spec.deviceRef.name", func(obj client.Object) []string {
		vpc := obj.(*nxv1.VPC)
		return []string{vpc.Spec.DeviceRef.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&nxv1.VPC{}).
		Named("vpc").
		WithEventFilter(filter).
		// Trigger reconciliation also for updates, e.g., if port-channel goes down
		Watches(
			&corev1.Interface{},
			handler.EnqueueRequestsFromMapFunc(r.mapAggregateToVPC),
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
			handler.EnqueueRequestsFromMapFunc(r.mapVRFToVPC),
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

func (r *VPCReconciler) mapAggregateToVPC(ctx context.Context, obj client.Object) []ctrl.Request {
	iface, ok := obj.(*corev1.Interface)
	if !ok {
		panic(fmt.Sprintf("Expected a Interface but got a %T", obj))
	}

	vpc := new(nxv1.VPC)
	var vpcs nxv1.VPCList
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

func (r *VPCReconciler) mapVRFToVPC(ctx context.Context, obj client.Object) []ctrl.Request {
	vrf, ok := obj.(*corev1.VRF)
	if !ok {
		panic(fmt.Sprintf("Expected a VRF but got a %T", obj))
	}

	vpc := new(nxv1.VPC)
	var vpcs nxv1.VPCList
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

func (r *VPCReconciler) finalize(ctx context.Context, s *vpcScope) (reterr error) {
	if err := s.Provider.Connect(ctx, s.Connection); err != nil {
		return fmt.Errorf("failed to connect to provider: %w", err)
	}
	defer func() {
		if err := s.Provider.Disconnect(ctx, s.Connection); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()
	return s.Provider.DeleteVPC(ctx)
}
