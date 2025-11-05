// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	nxosv1alpha1 "github.com/ironcore-dev/network-operator/api/cisco/nx/v1alpha1"
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
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=evpncontrolplanes,verbs=get;list;watch
// +kubebuilder:rbac:groups=nx.cisco.networking.metal.ironcore.dev,resources=nves,verbs=get;list;watch

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
			// If the custom resource is not found then it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			log.Info("Resource not found. Ignoring since object must be deleted")
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

	res, err := r.reconcile(ctx, s)
	if err != nil {
		log.Error(err, "Failed to reconcile resource")
		return ctrl.Result{}, err
	}

	return res, nil
}

// scope holds the different objects that are read and used during the reconcile.
type vtepScope struct {
	Device         *v1alpha1.Device
	VTEP           *v1alpha1.VTEP
	Connection     *deviceutil.Connection
	ProviderConfig *provider.ProviderConfig
	Provider       provider.VTEPProvider
}

// hasOwnerRef returns true if obj has an OwnerReference to owner.
func hasOwnerRef(obj metav1.Object, owner metav1.Object) bool {
	for _, ref := range obj.GetOwnerReferences() {
		if ref.UID == owner.GetUID() {
			return true
		}
	}
	return false
}

func (r *VTEPReconciler) reconcile(ctx context.Context, s *vtepScope) (_ ctrl.Result, reterr error) {
	if s.VTEP.Labels == nil {
		s.VTEP.Labels = make(map[string]string)
	}
	s.VTEP.Labels[v1alpha1.DeviceLabel] = s.Device.Name

	if err := r.setOwnerships(ctx, s); err != nil {
		return ctrl.Result{RequeueAfter: Jitter(r.RequeueInterval)}, err
	}

	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling VTEP resource")

	// Ensure the EVPNControlPlane referencing this VTEP, and set owner ref
	// var cpList v1alpha1.EVPNControlPlaneList
	// if err := r.List(ctx, &cpList,
	// 	client.InNamespace(s.VTEP.Namespace),
	// 	client.MatchingFields{evpnVtepRefIndexKey: s.VTEP.Name},
	// ); err != nil {
	// 	return ctrl.Result{}, fmt.Errorf("can not fetch/list EVPNControlPlanes: %w", err)
	// }
	// cp := &cpList.Items[0]
	// if hasOwnerRef(cp, s.VTEP) {
	// 	return ctrl.Result{}, fmt.Errorf("evpncontrolplane is already owned")
	// }
	// if cp.Spec.DeviceRef.Name != s.Device.Name {
	// 	return ctrl.Result{}, fmt.Errorf("evpncontrolplane device %s does not match vtep device %s", cp.Spec.DeviceRef.Name, s.Device.Name)
	// }
	// if err := controllerutil.SetOwnerReference(s.VTEP, cp, r.Scheme, controllerutil.WithBlockOwnerDeletion(true)); err != nil {
	// 	return ctrl.Result{}, fmt.Errorf("can not set owner ref: %w", err)
	// }

	// TODO: ensure provider specific config has an owner

	req := &provider.VTEPRequest{
		VTEP: s.VTEP,
	}

	// get control plane config
	req.ControlPlaneConfig = new(v1alpha1.EVPNControlPlane)
	if err := r.Get(ctx, types.NamespacedName{Namespace: s.VTEP.Namespace, Name: s.VTEP.Spec.ControlPlaneRef.Name}, req.ControlPlaneConfig); err != nil {
		if apierrors.IsNotFound(err) {
			conditions.Set(s.VTEP, metav1.Condition{
				Type:    v1alpha1.ConfiguredCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.WaitingForDependenciesReason,
				Message: fmt.Sprintf("EVPNControlPlane '%s' not found", s.VTEP.Spec.ControlPlaneRef.Name),
			})
			return ctrl.Result{}, fmt.Errorf("referenced EVPNControlPlane %q not found", s.VTEP.Spec.ControlPlaneRef.Name)
		}
		return ctrl.Result{}, fmt.Errorf("failed to get referenced EVPNControlPlane %q: %w", s.VTEP.Spec.ControlPlaneRef.Name, err)
	}

	// get provider specific config (e.g. NVE for NX-OS)
	switch p := s.VTEP.Spec.ProviderConfigRef.APIVersion; p {
	case nxosv1alpha1.GroupVersion.String():
		req.NVE = new(nxosv1alpha1.NVE)
		if err := r.Get(ctx, types.NamespacedName{Namespace: s.VTEP.Namespace, Name: s.VTEP.Spec.ProviderConfigRef.Name}, req.NVE); err != nil {
			if apierrors.IsNotFound(err) {
				conditions.Set(s.VTEP, metav1.Condition{
					Type:    v1alpha1.ConfiguredCondition,
					Status:  metav1.ConditionFalse,
					Reason:  v1alpha1.WaitingForDependenciesReason,
					Message: fmt.Sprintf("NVE '%s' not found", s.VTEP.Spec.ProviderConfigRef.Name),
				})
				return ctrl.Result{}, fmt.Errorf("referenced NVE %q not found", s.VTEP.Spec.ProviderConfigRef.Name)
			}
			return ctrl.Result{}, fmt.Errorf("failed to get referenced NVE %q: %w", s.VTEP.Spec.ProviderConfigRef.Name, err)
		}
	default:
		return ctrl.Result{}, fmt.Errorf("provider-config is not supported for API version %q", p)
	}

	// get referenced objects
	i, err := r.getInterface(ctx, s.VTEP.Spec.PrimaryInterfaceRef.Name, s)
	if err != nil {
		return ctrl.Result{}, err
	}
	req.PrimaryInterface = i

	i, err = r.getInterface(ctx, s.VTEP.Spec.AnycastInterfaceRef.Name, s)
	if err != nil {
		return ctrl.Result{}, err
	}
	req.AnycastInterface = i

	// Connect to remote device using the provider.
	if err := s.Provider.Connect(ctx, s.Connection); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to connect to provider: %w", err)
	}
	defer func() {
		if err := s.Provider.Disconnect(ctx, s.Connection); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	// Realize the VTEP on the remote device using the provider.
	err = s.Provider.EnsureVTEP(ctx, req)

	cond := conditions.FromError(err)
	conditions.Set(s.VTEP, cond)

	if err != nil {
		return ctrl.Result{}, err
	}

	// TODO: get operational status

	cond = metav1.Condition{
		Type:    v1alpha1.ConfiguredCondition,
		Status:  metav1.ConditionTrue,
		Reason:  v1alpha1.ConfiguredReason,
		Message: "VTEP is successfully configured",
	}
	conditions.Set(s.VTEP, cond)

	return ctrl.Result{RequeueAfter: Jitter(r.RequeueInterval)}, nil
}

func (r *VTEPReconciler) setOwnerships(ctx context.Context, s *vtepScope) error {
	// Ensure the VTEP is owned by the Device.
	if !controllerutil.HasControllerReference(s.VTEP) {
		if err := controllerutil.SetOwnerReference(s.Device, s.VTEP, r.Scheme, controllerutil.WithBlockOwnerDeletion(true)); err != nil {
			return err
		}
		// check control plane has an owner ref
		cp := new(v1alpha1.EVPNControlPlane)
		if err := r.Get(ctx, client.ObjectKey{Namespace: s.VTEP.Namespace, Name: s.VTEP.Spec.ControlPlaneRef.Name}, cp); err != nil {
			if apierrors.IsNotFound(err) {
				conditions.Set(s.VTEP, metav1.Condition{
					Type:    v1alpha1.ConfiguredCondition,
					Status:  metav1.ConditionFalse,
					Reason:  v1alpha1.WaitingForDependenciesReason,
					Message: fmt.Sprintf("EVPNControlPlane '%s' not found", s.VTEP.Spec.ControlPlaneRef.Name),
				})
				return fmt.Errorf("referenced EVPNControlPlane %q not found", s.VTEP.Spec.ControlPlaneRef.Name)
			}
			return fmt.Errorf("failed to get referenced EVPNControlPlane %q: %w", s.VTEP.Spec.ControlPlaneRef.Name, err)
		}

		if !controllerutil.HasControllerReference(cp) {
			if err := controllerutil.SetOwnerReference(s.VTEP, cp, r.Scheme, controllerutil.WithBlockOwnerDeletion(true)); err != nil {
				return fmt.Errorf("failed to set owner reference on EVPNControlPlane %q: %w", cp.Name, err)
			}
		} // TODO: else check if its correct owner?

		// check porovider specific config has an owner
		nve := new(nxosv1alpha1.NVE)
		if err := r.Get(ctx, client.ObjectKey{Namespace: s.VTEP.Namespace, Name: s.VTEP.Spec.ProviderConfigRef.Name}, nve); err != nil {
			if apierrors.IsNotFound(err) {
				conditions.Set(s.VTEP, metav1.Condition{
					Type:    v1alpha1.ConfiguredCondition,
					Status:  metav1.ConditionFalse,
					Reason:  v1alpha1.WaitingForDependenciesReason,
					Message: fmt.Sprintf("NVE '%s' not found", s.VTEP.Spec.ProviderConfigRef.Name),
				})
				return fmt.Errorf("referenced NVE %q not found", s.VTEP.Spec.ProviderConfigRef.Name)
			}
			return fmt.Errorf("failed to get referenced NVE %q: %w", s.VTEP.Spec.ProviderConfigRef.Name, err)
		}
		if !controllerutil.HasControllerReference(nve) {
			if err := controllerutil.SetOwnerReference(s.VTEP, nve, r.Scheme, controllerutil.WithBlockOwnerDeletion(true)); err != nil {
				return fmt.Errorf("failed to set owner reference on NVE %q: %w", nve.Name, err)
			}
		} // TODO: else check if its correct owner?
	}
	return nil
}

func (r *VTEPReconciler) getInterface(ctx context.Context, interfaceRefName string, s *vtepScope) (*v1alpha1.Interface, error) {
	intf := new(v1alpha1.Interface)
	intf.Name = interfaceRefName // todo: mmm....
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
	return intf, nil
}

const (
	evpnVtepRefIndexKey         = "spec.vtepRef.name"
	controlplaneVtepRefIndexKey = "spec.controlPlaneRef.name"
	deviceVtepRefIndexKey       = "spec.deviceRef.name"
)

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

	// TODO: how do you react to deletes of:
	// - the control plane?
	// - the referenced VTEP is now different? --> degraded but continues workiking?
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.VTEP{}).
		Named("vtep").
		WithEventFilter(filter).
		// Watches enqueues VTEPS whose referenced EVPNControlPlane spec has changed (ignoring status updates)
		Watches(
			&v1alpha1.EVPNControlPlane{},
			handler.EnqueueRequestsFromMapFunc(r.mapEVPNControlPlaneToVTEPs),
			builder.WithPredicates(predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool { return true },
				DeleteFunc: func(e event.DeleteEvent) bool { return true },
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldObj, ok1 := e.ObjectOld.(*v1alpha1.EVPNControlPlane)
					newObj, ok2 := e.ObjectNew.(*v1alpha1.EVPNControlPlane)
					if !ok1 || !ok2 {
						return false
					}
					return !reflect.DeepEqual(oldObj.Spec, newObj.Spec)
				},
				GenericFunc: func(e event.GenericEvent) bool { return false },
			}),
		).
		// Watches enqueues VTEPs whose provider config (NVE) spec has changed
		Watches(
			&nxosv1alpha1.NVE{},
			handler.EnqueueRequestsFromMapFunc(r.mapNVEToVTEPs),
			builder.WithPredicates(predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool { return true },
				DeleteFunc: func(e event.DeleteEvent) bool { return true },
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldObj, ok1 := e.ObjectOld.(*nxosv1alpha1.NVE)
					newObj, ok2 := e.ObjectNew.(*nxosv1alpha1.NVE)
					if !ok1 || !ok2 {
						return false
					}
					return !reflect.DeepEqual(oldObj.Spec, newObj.Spec)
				},
				GenericFunc: func(e event.GenericEvent) bool { return false },
			}),
		).
		Complete(r)
}

// mapEVPNControlPlaneToVTEPs enqueues all VTEPs that reference the modified EVPNControlPlane
func (r *VTEPReconciler) mapEVPNControlPlaneToVTEPs(ctx context.Context, obj client.Object) []ctrl.Request {
	nve, ok := obj.(*v1alpha1.EVPNControlPlane)
	if !ok {
		return nil
	}
	var vtepList v1alpha1.VTEPList
	if err := r.List(ctx, &vtepList,
		client.InNamespace(nve.Namespace),
		client.MatchingFields{
			controlplaneVtepRefIndexKey: nve.Name,
		},
	); err != nil {
		return nil
	}
	reqs := make([]ctrl.Request, 0, len(vtepList.Items))
	// TODO: we shouldn't have more than one VTEP referencing the same control plane
	// TODO: check is the same device
	for i := range vtepList.Items {
		reqs = append(reqs, ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      vtepList.Items[i].Name,
				Namespace: vtepList.Items[i].Namespace,
			},
		})
	}
	return reqs
}

// mapProviderConfigToVTEPs enqueues reconcile requests for VTEPs with OwnerReference to the given provider config
func (r *VTEPReconciler) mapNVEToVTEPs(ctx context.Context, obj client.Object) []ctrl.Request {
	// nve is a cisco specific provider, add other types if needed
	nve, ok := obj.(*nxosv1alpha1.NVE)
	if !ok {
		return nil
	}

	ownerRefs := nve.GetOwnerReferences()
	if len(ownerRefs) == 0 {
		return nil
	} else if len(ownerRefs) > 1 {
		panic("provider config has multiple owner references, not supported")
	}

	reqs := make([]ctrl.Request, 0, len(ownerRefs))
	for _, owner := range ownerRefs {
		if owner.Kind == "VTEP" && owner.APIVersion == v1alpha1.GroupVersion.String() {
			reqs = append(reqs, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      owner.Name,
					Namespace: nve.Namespace,
				},
			})
		}
	}
	return reqs
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
