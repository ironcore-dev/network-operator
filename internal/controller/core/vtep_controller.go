// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package core

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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	nxv1alpha1 "github.com/ironcore-dev/network-operator/api/cisco/nx/v1alpha1"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/conditions"
	"github.com/ironcore-dev/network-operator/internal/deviceutil"
	"github.com/ironcore-dev/network-operator/internal/provider"
)

// VTEPReconciler reconciles a VTEP object
type VTEPReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// WatchFilterValue is the label value used to filter events prior to reconciliation.
	WatchFilterValue string

	// Recorder is used to record events for the controller.
	// More info: https://book.kubebuilder.io/reference/raising-events
	Recorder record.EventRecorder

	// Provider is the driver that will be used to create & delete the dns.
	Provider provider.ProviderFunc

	// RequeueInterval is the duration after which the controller should requeue the reconciliation,
	// regardless of changes.
	RequeueInterval time.Duration
}

// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=vteps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=vteps/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=vteps/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.22.1/pkg/reconcile
func (r *VTEPReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling resource")

	obj := new(v1alpha1.VTEP)
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Resource not found. Ignoring reconciliation since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get resource")
		return ctrl.Result{}, err
	}

	prov, ok := r.Provider().(provider.VTEPProvider)
	if !ok {
		if meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.NotImplementedReason,
			Message: "Provider does not implement provider VTEPProvider",
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

	s := &vtepScope{
		Device:         device,
		VTEP:           obj,
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

	if err = r.reconcile(ctx, s); err != nil {
		log.Error(err, "Failed to reconcile resource")
		return ctrl.Result{}, err
	}

	// force a periodic requeue to enforce state is in sync
	return ctrl.Result{RequeueAfter: Jitter(r.RequeueInterval)}, nil
}

// vtepScope holds k8s objects used during a reconciliation.
type vtepScope struct {
	Device         *v1alpha1.Device
	VTEP           *v1alpha1.VTEP
	Connection     *deviceutil.Connection
	ProviderConfig *provider.ProviderConfig
	Provider       provider.VTEPProvider
}

func (r *VTEPReconciler) reconcile(ctx context.Context, s *vtepScope) (reterr error) {
	if s.VTEP.Labels == nil {
		s.VTEP.Labels = make(map[string]string)
	}
	s.VTEP.Labels[v1alpha1.DeviceLabel] = s.Device.Name

	if !controllerutil.HasControllerReference(s.VTEP) {
		if err := controllerutil.SetOwnerReference(s.Device, s.VTEP, r.Scheme, controllerutil.WithBlockOwnerDeletion(true)); err != nil {
			return err
		}
	}

	// referenced provider config should be compatible with device
	if s.ProviderConfig != nil {
		if err := r.validateProviderConfigRef(ctx, s); err != nil {
			conditions.Set(s.VTEP, metav1.Condition{
				Type:    v1alpha1.ConfiguredCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.IncompatibleProviderConfigRef,
				Message: fmt.Sprintf("ProviderConfigRef is not compatible with Device: %v", err),
			})
			return fmt.Errorf("provider config reference is not compatible with OS installed on the device: %w", err)
		}
	}

	primaryIf, err := r.validateInterfaceRef(ctx, s.VTEP.Spec.PrimaryInterfaceRef.Name, s)
	if err != nil {
		return fmt.Errorf("vtep: failed to validate primary interface reference: %w", err)
	}

	anycastIf, err := r.validateInterfaceRef(ctx, s.VTEP.Spec.AnycastInterfaceRef.Name, s)
	if err != nil {
		return fmt.Errorf("vtep: failed to validate anycast interface reference: %w", err)
	}

	if primaryIf.Name == anycastIf.Name {
		conditions.Set(s.VTEP, metav1.Condition{
			Type:    v1alpha1.ConfiguredCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.ErrorReason,
			Message: "PrimaryInterfaceRef and AnycastInterfaceRef cannot refer to the same interface",
		})
		return errors.New("vtep: primary interface and anycast interface cannot be the same")
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
		conditions.RecomputeReady(s.VTEP)
	}()

	// Ensure the ManagementAccess is realized on the provider.
	err = s.Provider.EnsureVTEP(ctx, &provider.VTEPRequest{
		VTEP:             s.VTEP,
		ProviderConfig:   s.ProviderConfig,
		PrimaryInterface: primaryIf,
		AnycastInterface: anycastIf,
	})

	cond := conditions.FromError(err)
	conditions.Set(s.VTEP, cond)

	if err != nil {
		return err
	}

	status, err := s.Provider.GetStatusVTEP(ctx, &provider.VTEPRequest{
		VTEP:           s.VTEP,
		ProviderConfig: s.ProviderConfig,
	})
	if err != nil {
		return fmt.Errorf("failed to get VTEP status: %w", err)
	}

	cond = metav1.Condition{
		Type:    v1alpha1.OperationalCondition,
		Status:  metav1.ConditionTrue,
		Reason:  v1alpha1.OperationalReason,
		Message: "VTEP is operationally up",
	}
	if !status.OperStatus {
		cond.Status = metav1.ConditionFalse
		cond.Reason = v1alpha1.DegradedReason
		cond.Message = "VTEP is operationally down"
	}
	conditions.Set(s.VTEP, cond)

	return nil
}

// validateInterfaceRef checks that the referenced interface exists, is of type Loopback, and belongs to the same device as the VTEP.
// TODO: discuss with team if these should be done in an admission webhook instead of each time during reconciliation.
func (r *VTEPReconciler) validateInterfaceRef(ctx context.Context, interfaceRefName string, s *vtepScope) (*v1alpha1.Interface, error) {
	intf := new(v1alpha1.Interface)
	intf.Name = interfaceRefName
	intf.Namespace = s.VTEP.Namespace

	if err := r.Get(ctx, client.ObjectKey{Name: intf.Name, Namespace: intf.Namespace}, intf); err != nil {
		if apierrors.IsNotFound(err) {
			conditions.Set(s.VTEP, metav1.Condition{
				Type:    v1alpha1.ConfiguredCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.WaitingForDependenciesReason,
				Message: fmt.Sprintf("interface resource '%s' not found in namespace '%s'", intf.Name, intf.Namespace),
			})
			return nil, fmt.Errorf("member interface %q not found", s.VTEP.Spec.PrimaryInterfaceRef.Name)
		}
		return nil, fmt.Errorf("failed to get member interface %q: %w", s.VTEP.Spec.PrimaryInterfaceRef.Name, err)
	}

	if intf.Spec.Type != v1alpha1.InterfaceTypeLoopback {
		conditions.Set(s.VTEP, metav1.Condition{
			Type:    v1alpha1.ConfiguredCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.InvalidInterfaceTypeReason,
			Message: fmt.Sprintf("interface referenced by '%s' must be of type 'Loopback'", interfaceRefName),
		})
		return nil, fmt.Errorf("interface referenced by '%s' must be of type 'Loopback'", interfaceRefName)
	}

	if s.VTEP.Spec.DeviceRef.Name != intf.Spec.DeviceRef.Name {
		conditions.Set(s.VTEP, metav1.Condition{
			Type:    v1alpha1.ConfiguredCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.CrossDeviceReferenceReason,
			Message: fmt.Sprintf("interface '%s' deviceRef '%s' does not match VTEP deviceRef '%s'", intf.Name, intf.Spec.DeviceRef.Name, s.VTEP.Spec.DeviceRef.Name),
		})
		return nil, fmt.Errorf("interface '%s' deviceRef '%s' does not match VTEP deviceRef '%s'", intf.Name, intf.Spec.DeviceRef.Name, s.VTEP.Spec.DeviceRef.Name)
	}
	return intf, nil
}

// validateProviderConfigRef checks if the referenced provider configuration is compatible with the device. Currently checking only for Cisco NXOS.
func (r *VTEPReconciler) validateProviderConfigRef(_ context.Context, s *vtepScope) error {
	// first check if the kind and api version are supported by the provider
	if s.VTEP.Spec.ProviderConfigRef.Kind != "VTEPConfig" || s.VTEP.Spec.ProviderConfigRef.APIVersion != nxv1alpha1.GroupVersion.String() {
		return fmt.Errorf("provider config kind %q with api version %q is not supported for VTEP on the provider", s.VTEP.Spec.ProviderConfigRef.Kind, s.VTEP.Spec.ProviderConfigRef.APIVersion)
	}
	// check compatibility with device model and os
	if s.Device.Status.Manufacturer != nxv1alpha1.CompatibleManufacturer {
		return fmt.Errorf("device %q manufacturer %q is not compatible, expected %q", s.Device.Name, s.Device.Status.Manufacturer, nxv1alpha1.CompatibleManufacturer)
	}

	if !slices.Contains(nxv1alpha1.CompatibleModels, s.Device.Status.Model) {
		return fmt.Errorf("device %q model %q is not compatible, expected one of %v", s.Device.Name, s.Device.Status.Model, nxv1alpha1.CompatibleModels)
	}

	if !slices.Contains(nxv1alpha1.CompatibleFirmwareVersions, s.Device.Status.FirmwareVersion) {
		return fmt.Errorf("device %q firmware version %q is not compatible, expected one of %v", s.Device.Name, s.Device.Status.FirmwareVersion, nxv1alpha1.CompatibleFirmwareVersions)
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *VTEPReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
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

	c := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.VTEP{}).
		Named("vtep").
		WithEventFilter(filter)

	for _, gvk := range v1alpha1.VTEPDependencies {
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(gvk)
		// TODO: check type?
		c = c.Watches(
			obj,
			handler.EnqueueRequestsFromMapFunc(r.mapProviderConfigToVTEPs),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		)
	}

	return c.Complete(r)
}

// mapProviderConfigToVTEPs is a [handler.MapFunc] to re-enqueue VTEPs that require reconciliation, i.e.,
// whose referenced provider configuration has changed.
func (r *VTEPReconciler) mapProviderConfigToVTEPs(ctx context.Context, obj client.Object) []reconcile.Request {
	log := ctrl.LoggerFrom(ctx, "Object", klog.KObj(obj))

	list := &v1alpha1.VTEPList{}
	if err := r.List(ctx, list, client.InNamespace(obj.GetNamespace())); err != nil {
		log.Error(err, "Failed to list VTEPs")
		return nil
	}

	gkv := obj.GetObjectKind().GroupVersionKind()

	var requests []reconcile.Request
	for _, m := range list.Items {
		if m.Spec.ProviderConfigRef != nil &&
			m.Spec.ProviderConfigRef.Name == obj.GetName() &&
			m.Spec.ProviderConfigRef.Kind == gkv.Kind &&
			m.Spec.ProviderConfigRef.APIVersion == gkv.GroupVersion().Identifier() {
			log.Info("Enqueuing VTEP for reconciliation", "VTEP", klog.KObj(&m))
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

func (r *VTEPReconciler) finalize(ctx context.Context, s *vtepScope) (reterr error) {
	if err := s.Provider.Connect(ctx, s.Connection); err != nil {
		return fmt.Errorf("failed to connect to provider: %w", err)
	}
	defer func() {
		if err := s.Provider.Disconnect(ctx, s.Connection); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	// TDO: do we need the other or just works with refs and finalizers?
	return s.Provider.DeleteVTEP(ctx, &provider.VTEPRequest{
		VTEP: s.VTEP,
	})
}
