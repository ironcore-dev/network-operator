// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"errors"
	"fmt"
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

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/apistatus"
	"github.com/ironcore-dev/network-operator/internal/conditions"
	"github.com/ironcore-dev/network-operator/internal/deviceutil"
	"github.com/ironcore-dev/network-operator/internal/paused"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/resourcelock"
)

// CommunitySetReconciler reconciles a CommunitySet object.
type CommunitySetReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	WatchFilterValue string
	Recorder         events.EventRecorder
	Provider         provider.ProviderFunc
	Locker           *resourcelock.ResourceLocker
}

// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=communitysets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=communitysets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=communitysets/finalizers,verbs=update
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

func (r *CommunitySetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(3).Info("Reconciling resource")

	obj := new(v1alpha1.CommunitySet)
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(3).Info("Resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get resource")
		return ctrl.Result{}, err
	}

	prov, ok := r.Provider().(provider.CommunitySetProvider)
	if !ok {
		if meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.NotImplementedReason,
			Message: "Provider does not implement provider.CommunitySetProvider",
		}) {
			return ctrl.Result{}, r.Status().Update(ctx, obj)
		}
		return ctrl.Result{}, nil
	}

	device, err := deviceutil.GetDeviceByName(ctx, r, obj.Namespace, obj.Spec.DeviceRef.Name)
	if err != nil {
		return ctrl.Result{}, err
	}

	if isPaused, err := paused.EnsureCondition(ctx, r.Client, device, obj); isPaused || err != nil {
		return ctrl.Result{}, err
	}

	if err := r.Locker.AcquireLock(ctx, device.Name, "communityset-controller"); err != nil {
		if errors.Is(err, resourcelock.ErrLockAlreadyHeld) {
			log.V(3).Info("Device is already locked, requeuing reconciliation")
			return ctrl.Result{RequeueAfter: Jitter(time.Second), Priority: new(LockWaitPriorityDefault)}, nil
		}
		log.Error(err, "Failed to acquire device lock")
		return ctrl.Result{}, err
	}
	defer func() {
		if err := r.Locker.ReleaseLock(ctx, device.Name, "communityset-controller"); err != nil {
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

	s := &communitySetScope{
		Device:         device,
		CommunitySet:   obj,
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

	defer func() {
		if !equality.Semantic.DeepEqual(orig.ObjectMeta, obj.ObjectMeta) {
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

	return ctrl.Result{}, nil
}

func (r *CommunitySetReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	labelSelector := metav1.LabelSelector{}
	if r.WatchFilterValue != "" {
		labelSelector.MatchLabels = map[string]string{v1alpha1.WatchLabel: r.WatchFilterValue}
	}

	filter, err := predicate.LabelSelectorPredicate(labelSelector)
	if err != nil {
		return fmt.Errorf("failed to create label selector predicate: %w", err)
	}

	if err := mgr.GetFieldIndexer().IndexField(ctx, &v1alpha1.CommunitySet{}, v1alpha1.DeviceRefIndexKey, func(obj client.Object) []string {
		o := obj.(*v1alpha1.CommunitySet)
		return []string{o.Spec.DeviceRef.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.CommunitySet{}).
		Named("communityset").
		WithEventFilter(filter).
		Watches(
			&v1alpha1.Device{},
			handler.EnqueueRequestsFromMapFunc(r.deviceToCommunitySets),
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

type communitySetScope struct {
	Device         *v1alpha1.Device
	CommunitySet   *v1alpha1.CommunitySet
	Connection     *deviceutil.Connection
	ProviderConfig *provider.ProviderConfig
	Provider       provider.CommunitySetProvider
}

func (r *CommunitySetReconciler) reconcile(ctx context.Context, s *communitySetScope) (reterr error) {
	if s.CommunitySet.Labels == nil {
		s.CommunitySet.Labels = make(map[string]string)
	}
	s.CommunitySet.Labels[v1alpha1.DeviceLabel] = s.Device.Name

	if !controllerutil.HasControllerReference(s.CommunitySet) {
		if err := controllerutil.SetOwnerReference(s.Device, s.CommunitySet, r.Scheme, controllerutil.WithBlockOwnerDeletion(true)); err != nil {
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

	err := s.Provider.EnsureCommunitySet(ctx, &provider.CommunitySetRequest{
		CommunitySet:   s.CommunitySet,
		ProviderConfig: s.ProviderConfig,
	})

	cond := conditions.FromError(err)
	cond.Type = v1alpha1.ReadyCondition
	conditions.Set(s.CommunitySet, cond)

	return err
}

func (r *CommunitySetReconciler) finalize(ctx context.Context, s *communitySetScope) (reterr error) {
	if err := s.Provider.Connect(ctx, s.Connection); err != nil {
		return fmt.Errorf("failed to connect to provider: %w", err)
	}
	defer func() {
		if err := s.Provider.Disconnect(ctx, s.Connection); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	return s.Provider.DeleteCommunitySet(ctx, &provider.CommunitySetRequest{
		CommunitySet:   s.CommunitySet,
		ProviderConfig: s.ProviderConfig,
	})
}

func (r *CommunitySetReconciler) deviceToCommunitySets(ctx context.Context, obj client.Object) []ctrl.Request {
	device, ok := obj.(*v1alpha1.Device)
	if !ok {
		panic(fmt.Sprintf("Expected a Device but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx, "Device", klog.KObj(device))

	list := new(v1alpha1.CommunitySetList)
	if err := r.List(
		ctx, list,
		client.InNamespace(device.Namespace),
		client.MatchingFields{v1alpha1.DeviceRefIndexKey: device.Name},
	); err != nil {
		log.Error(err, "Failed to list CommunitySets")
		return nil
	}

	requests := make([]ctrl.Request, 0, len(list.Items))
	for i := range list.Items {
		log.V(2).Info("Enqueuing CommunitySet for reconciliation", "CommunitySet", klog.KObj(&list.Items[i]))
		requests = append(requests, ctrl.Request{
			Name:      list.Items[i].Name,
			Namespace: list.Items[i].Namespace,
		})
	}

	return requests
}
