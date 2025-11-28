// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"fmt"

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

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/conditions"
	"github.com/ironcore-dev/network-operator/internal/deviceutil"
	"github.com/ironcore-dev/network-operator/internal/provider"
)

// EVPNInstanceReconciler reconciles a EVPNInstance object
type EVPNInstanceReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// WatchFilterValue is the label value used to filter events prior to reconciliation.
	WatchFilterValue string

	// Recorder is used to record events for the controller.
	// More info: https://book.kubebuilder.io/reference/raising-events
	Recorder record.EventRecorder

	// Provider is the driver that will be used to create & delete the evpninstance.
	Provider provider.ProviderFunc
}

// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=evpninstances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=evpninstances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=evpninstances/finalizers,verbs=update
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=vlans,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=vlans/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.2/pkg/reconcile
//
// For more details about the method shape, read up here:
// - https://ahmet.im/blog/controller-pitfalls/#reconcile-method-shape
func (r *EVPNInstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling resource")

	obj := new(v1alpha1.EVPNInstance)
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

	prov, ok := r.Provider().(provider.EVPNInstanceProvider)
	if !ok {
		if meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.NotImplementedReason,
			Message: "Provider does not implement provider.EVPNInstanceProvider",
		}) {
			return ctrl.Result{}, r.Status().Update(ctx, obj)
		}
		return ctrl.Result{}, nil
	}

	device, err := deviceutil.GetDeviceByName(ctx, r, obj.Namespace, obj.Spec.DeviceRef.Name)
	if err != nil {
		return ctrl.Result{}, err
	}

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

	s := &eviScope{
		Device:         device,
		EVPNInstance:   obj,
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

var eviVlanRefKey = ".spec.vlanRef.name"

// SetupWithManager sets up the controller with the Manager.
func (r *EVPNInstanceReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	labelSelector := metav1.LabelSelector{}
	if r.WatchFilterValue != "" {
		labelSelector.MatchLabels = map[string]string{v1alpha1.WatchLabel: r.WatchFilterValue}
	}

	filter, err := predicate.LabelSelectorPredicate(labelSelector)
	if err != nil {
		return fmt.Errorf("failed to create label selector predicate: %w", err)
	}

	if err := mgr.GetFieldIndexer().IndexField(ctx, &v1alpha1.EVPNInstance{}, eviVlanRefKey, func(obj client.Object) []string {
		evi := obj.(*v1alpha1.EVPNInstance)
		if evi.Spec.VLANRef == nil {
			return nil
		}
		return []string{evi.Spec.VLANRef.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.EVPNInstance{}).
		Named("evpninstance").
		WithEventFilter(filter).
		// Watches enqueues EVPNInstances for updates in referenced VLAN resources.
		// Only triggers on create and delete events since VLAN IDs are immutable.
		Watches(
			&v1alpha1.VLAN{},
			handler.EnqueueRequestsFromMapFunc(r.vlanToEVPNInstance),
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

// eviScope holds the different objects that are read and used during the reconcile.
type eviScope struct {
	Device         *v1alpha1.Device
	EVPNInstance   *v1alpha1.EVPNInstance
	Connection     *deviceutil.Connection
	ProviderConfig *provider.ProviderConfig
	Provider       provider.EVPNInstanceProvider
}

func (r *EVPNInstanceReconciler) reconcile(ctx context.Context, s *eviScope) (reterr error) {
	if s.EVPNInstance.Labels == nil {
		s.EVPNInstance.Labels = make(map[string]string)
	}

	s.EVPNInstance.Labels[v1alpha1.DeviceLabel] = s.Device.Name

	// Ensure the EVPNInstance is owned by the Device.
	if !controllerutil.HasControllerReference(s.EVPNInstance) {
		if err := controllerutil.SetOwnerReference(s.Device, s.EVPNInstance, r.Scheme, controllerutil.WithBlockOwnerDeletion(true)); err != nil {
			return err
		}
	}

	var vlan *v1alpha1.VLAN
	if s.EVPNInstance.Spec.Type == v1alpha1.EVPNInstanceTypeBridged && s.EVPNInstance.Spec.VLANRef != nil {
		var err error
		vlan, err = r.reconcileVLAN(ctx, s)
		if err != nil {
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

	// Ensure the EVPNInstance is realized on the provider.
	err := s.Provider.EnsureEVPNInstance(ctx, &provider.EVPNInstanceRequest{
		EVPNInstance:   s.EVPNInstance,
		ProviderConfig: s.ProviderConfig,
		VLAN:           vlan,
	})

	cond := conditions.FromError(err)
	// As this resource is configuration only, we use the Configured condition as top-level Ready condition.
	cond.Type = v1alpha1.ReadyCondition
	conditions.Set(s.EVPNInstance, cond)

	return err
}

// reconcileVLAN ensures that the referenced VLAN exists, belongs to the same device as the EVPNInstance.
// It also updates the VLAN to reference the EVPNInstance by setting its BridgedBy status field.
func (r *EVPNInstanceReconciler) reconcileVLAN(ctx context.Context, s *eviScope) (*v1alpha1.VLAN, error) {
	key := client.ObjectKey{
		Name:      s.EVPNInstance.Spec.VLANRef.Name,
		Namespace: s.EVPNInstance.Namespace,
	}

	vlan := new(v1alpha1.VLAN)
	if err := r.Get(ctx, key, vlan); err != nil {
		if apierrors.IsNotFound(err) {
			conditions.Set(s.EVPNInstance, metav1.Condition{
				Type:    v1alpha1.ReadyCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.VLANNotFoundReason,
				Message: fmt.Sprintf("referenced VLAN %q not found", key),
			})
			return nil, reconcile.TerminalError(fmt.Errorf("referenced VLAN %q not found", key))
		}
		return nil, fmt.Errorf("failed to get referenced VLAN %q: %w", key, err)
	}

	if vlan.Spec.DeviceRef.Name != s.Device.Name {
		conditions.Set(s.EVPNInstance, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.CrossDeviceReferenceReason,
			Message: fmt.Sprintf("referenced VLAN %q does not belong to device %q", vlan.Name, s.Device.Name),
		})
		return nil, reconcile.TerminalError(fmt.Errorf("referenced VLAN %q does not belong to device %q", vlan.Name, s.Device.Name))
	}

	if vlan.Status.BridgedBy != nil && vlan.Status.BridgedBy.Name != s.EVPNInstance.Name {
		conditions.Set(s.EVPNInstance, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.VLANAlreadyInUseReason,
			Message: fmt.Sprintf("VLAN %q is already in use by EVPNInstance %q", vlan.Name, vlan.Status.BridgedBy.Name),
		})
		return nil, reconcile.TerminalError(fmt.Errorf("VLAN %q is already in use by EVPNInstance %q", vlan.Name, vlan.Status.BridgedBy.Name))
	}

	if vlan.Status.BridgedBy == nil {
		vlan.Status.BridgedBy = &v1alpha1.LocalObjectReference{Name: s.EVPNInstance.Name}
		if err := r.Status().Update(ctx, vlan); err != nil {
			return nil, fmt.Errorf("failed to update VLAN %q status: %w", vlan.Name, err)
		}
	}

	if vlan.Labels == nil {
		vlan.Labels = make(map[string]string)
	}

	if vlan.Labels[v1alpha1.L2VNILabel] != s.EVPNInstance.Name {
		vlan.Labels[v1alpha1.L2VNILabel] = s.EVPNInstance.Name
		if err := r.Update(ctx, vlan); err != nil {
			return nil, fmt.Errorf("failed to update VLAN %q labels: %w", vlan.Name, err)
		}
	}

	return vlan, nil
}

func (r *EVPNInstanceReconciler) finalize(ctx context.Context, s *eviScope) (reterr error) {
	if err := r.finalizeVLAN(ctx, s); err != nil {
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

	return s.Provider.DeleteEVPNInstance(ctx, &provider.EVPNInstanceRequest{
		EVPNInstance:   s.EVPNInstance,
		ProviderConfig: s.ProviderConfig,
	})
}

// finalizeVLAN removes the EVPNInstance reference from the VLAN.
func (r *EVPNInstanceReconciler) finalizeVLAN(ctx context.Context, s *eviScope) error {
	if s.EVPNInstance.Spec.VLANRef == nil {
		return nil
	}

	vlan := new(v1alpha1.VLAN)
	if err := r.Get(ctx, client.ObjectKey{Name: s.EVPNInstance.Spec.VLANRef.Name, Namespace: s.EVPNInstance.Namespace}, vlan); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if vlan.Status.BridgedBy != nil && vlan.Status.BridgedBy.Name == s.EVPNInstance.Name {
		vlan.Status.BridgedBy = nil
		if err := r.Status().Update(ctx, vlan); err != nil {
			return fmt.Errorf("failed to update VLAN %q status: %w", vlan.Name, err)
		}
	}

	if vlan.Labels != nil && vlan.Labels[v1alpha1.L2VNILabel] == s.EVPNInstance.Name {
		delete(vlan.Labels, v1alpha1.L2VNILabel)
		if err := r.Update(ctx, vlan); err != nil {
			return fmt.Errorf("failed to update VLAN %q labels: %w", vlan.Name, err)
		}
	}

	return nil
}

// vlanToEVPNInstance is a [handler.MapFunc] to be used to enqueue requests for reconciliation
// for an EVPNInstance when its referenced VLAN changes.
func (r *EVPNInstanceReconciler) vlanToEVPNInstance(ctx context.Context, obj client.Object) []ctrl.Request {
	vlan, ok := obj.(*v1alpha1.VLAN)
	if !ok {
		panic(fmt.Sprintf("Expected a VLAN but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx, "VLAN", klog.KObj(vlan))

	evpnInstances := new(v1alpha1.EVPNInstanceList)
	if err := r.List(ctx, evpnInstances, client.InNamespace(vlan.Namespace), client.MatchingFields{eviVlanRefKey: vlan.Name}); err != nil {
		log.Error(err, "Failed to list EVPNInstances")
		return nil
	}

	requests := []ctrl.Request{}
	for _, evi := range evpnInstances.Items {
		if evi.Spec.VLANRef != nil && evi.Spec.VLANRef.Name == vlan.Name {
			log.Info("Enqueuing EVPNInstance for reconciliation", "EVPNInstance", klog.KObj(&evi))

			requests = append(requests, ctrl.Request{
				NamespacedName: client.ObjectKey{
					Name:      evi.Name,
					Namespace: evi.Namespace,
				},
			})
		}
	}

	return requests
}
