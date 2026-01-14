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
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/conditions"
	"github.com/ironcore-dev/network-operator/internal/deviceutil"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/resourcelock"
)

// BGPPeerReconciler reconciles a BGPPeer object
type BGPPeerReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// WatchFilterValue is the label value used to filter events prior to reconciliation.
	WatchFilterValue string

	// Recorder is used to record events for the controller.
	// More info: https://book.kubebuilder.io/reference/raising-events
	Recorder record.EventRecorder

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
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

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
	log.Info("Reconciling resource")

	obj := new(v1alpha1.BGPPeer)
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

	if err := r.Locker.AcquireLock(ctx, device.Name, "bgppeer-controller"); err != nil {
		if errors.Is(err, resourcelock.ErrLockAlreadyHeld) {
			log.Info("Device is already locked, requeuing reconciliation")
			return ctrl.Result{RequeueAfter: time.Second * 5}, nil
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
	if conditions.InitializeConditions(obj, v1alpha1.ReadyCondition, v1alpha1.ConfiguredCondition, v1alpha1.OperationalCondition) {
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

// SetupWithManager sets up the controller with the Manager.
func (r *BGPPeerReconciler) SetupWithManager(mgr ctrl.Manager) error {
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

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.BGPPeer{}).
		Named("bgppeer").
		WithEventFilter(filter).
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

func (r *BGPPeerReconciler) reconcile(ctx context.Context, s *bgpPeerScope) (_ ctrl.Result, reterr error) {
	if s.BGPPeer.Labels == nil {
		s.BGPPeer.Labels = make(map[string]string)
	}

	s.BGPPeer.Labels[v1alpha1.DeviceLabel] = s.Device.Name

	// Ensure the BGPPeer is owned by the Device.
	if !controllerutil.HasControllerReference(s.BGPPeer) {
		if err := controllerutil.SetOwnerReference(s.Device, s.BGPPeer, r.Scheme, controllerutil.WithBlockOwnerDeletion(true)); err != nil {
			return ctrl.Result{}, err
		}
	}

	defer func() {
		conditions.RecomputeReady(s.BGPPeer)
	}()

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
				return ctrl.Result{}, reconcile.TerminalError(fmt.Errorf("source interface %q not found", addr.InterfaceRef.Name))
			}
			return ctrl.Result{}, fmt.Errorf("failed to get source interface %q: %w", addr.InterfaceRef.Name, err)
		}

		if intf.Spec.DeviceRef.Name != s.Device.Name {
			conditions.Set(s.BGPPeer, metav1.Condition{
				Type:    v1alpha1.ConfiguredCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.CrossDeviceReferenceReason,
				Message: fmt.Sprintf("source interface %q does not belong to device %q", intf.Name, s.Device.Name),
			})
			return ctrl.Result{}, reconcile.TerminalError(fmt.Errorf("source interface %q does not belong to device %q", intf.Name, s.Device.Name))
		}
		sourceInterface = intf.Spec.Name
	}

	if err := s.Provider.Connect(ctx, s.Connection); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to connect to provider: %w", err)
	}
	defer func() {
		if err := s.Provider.Disconnect(ctx, s.Connection); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	// Ensure the BGPPeer is realized on the provider.
	err := s.Provider.EnsureBGPPeer(ctx, &provider.EnsureBGPPeerRequest{
		BGPPeer:         s.BGPPeer,
		ProviderConfig:  s.ProviderConfig,
		SourceInterface: sourceInterface,
	})

	cond := conditions.FromError(err)
	conditions.Set(s.BGPPeer, cond)

	if err != nil {
		return ctrl.Result{}, err
	}

	status, err := s.Provider.GetPeerStatus(ctx, &provider.BGPPeerStatusRequest{
		BGPPeer:        s.BGPPeer,
		ProviderConfig: s.ProviderConfig,
	})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get bgp peer status: %w", err)
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
		s.BGPPeer.Status.LastEstablishedTime = ptr.To(metav1.NewTime(status.LastEstablishedTime))
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

	return ctrl.Result{RequeueAfter: Jitter(r.RequeueInterval)}, err
}

func (r *BGPPeerReconciler) finalize(ctx context.Context, s *bgpPeerScope) (reterr error) {
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
	})
}
