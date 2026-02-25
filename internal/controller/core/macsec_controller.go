// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"

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
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/conditions"
	"github.com/ironcore-dev/network-operator/internal/deviceutil"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/resourcelock"
)

// MacSecReconciler reconciles a MacSec object
type MacSecReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// WatchFilterValue is the label value used to filter events prior to reconciliation.
	WatchFilterValue string

	// Recorder is used to record events for the controller.
	// More info: https://book.kubebuilder.io/reference/raising-events
	Recorder record.EventRecorder

	// Provider is the driver that will be used to create & delete the macsec.
	Provider provider.ProviderFunc

	// Locker is used to synchronize operations on resources targeting the same device.
	Locker *resourcelock.ResourceLocker

	// RequeueInterval is the duration after which the controller should requeue the reconciliation,
	// regardless of changes.
	RequeueInterval time.Duration
}

// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=macsecs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=macsecs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=macsecs/finalizers,verbs=update
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=devices,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=interfaces,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *MacSecReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling MacSec resource")

	obj := new(v1alpha1.MacSec)
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			// If the custom resource is not found then it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			log.Info("MacSec resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get MacSec resource")
		return ctrl.Result{}, err
	}

	prov, ok := r.Provider().(provider.MacSecProvider)
	if !ok {
		if meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.NotImplementedReason,
			Message: "Provider does not implement provider.MacSecProvider",
		}) {
			return ctrl.Result{}, r.Status().Update(ctx, obj)
		}
		return ctrl.Result{}, nil
	}

	// Validate that the referenced device exists
	device, err := deviceutil.GetDeviceByName(ctx, r, obj.Namespace, obj.Spec.DeviceRef.Name)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Validate that the referenced interface exists
	intf, err := GetInterfaceByName(ctx, r, obj.Namespace, obj.Spec.InterfaceRef.Name)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Validate that all pre-shared key secrets exist
	secrets, err := r.validatePreSharedKeySecrets(ctx, obj)
	if err != nil {
		log.Error(err, "Pre-shared key validation failed")
		if meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.ErrorReason,
			Message: fmt.Sprintf("Pre-shared key validation failed: %v", err),
		}) {
			return ctrl.Result{}, r.Status().Update(ctx, obj)
		}
		return ctrl.Result{}, nil
	}
	if err := r.Locker.AcquireLock(ctx, device.Name, "macsec-controller"); err != nil {
		if errors.Is(err, resourcelock.ErrLockAlreadyHeld) {
			log.Info("Device is already locked, requeuing reconciliation")
			return ctrl.Result{RequeueAfter: time.Second * 5}, nil
		}
		log.Error(err, "Failed to acquire device lock")
		return ctrl.Result{}, err
	}
	defer func() {
		if err := r.Locker.ReleaseLock(ctx, device.Name, "macsec-controller"); err != nil {
			log.Error(err, "Failed to release device lock")
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	conn, err := deviceutil.GetDeviceConnection(ctx, r, device)
	if err != nil {
		return ctrl.Result{}, err
	}

	s := &macSecScope{
		Device:     device,
		MacSec:     obj,
		Interface:  intf,
		Secrets:    secrets,
		Connection: conn,
		Provider:   prov,
	}

	if !obj.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(obj, v1alpha1.FinalizerName) {
			if err := r.finalize(ctx, s); err != nil {
				log.Error(err, "Failed to finalize MacSec resource")
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(obj, v1alpha1.FinalizerName)
			if err := r.Update(ctx, obj); err != nil {
				log.Error(err, "Failed to remove finalizer from MacSec resource")
				return ctrl.Result{}, err
			}
		}
		log.Info("MacSec resource is being deleted, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers
	if !controllerutil.ContainsFinalizer(obj, v1alpha1.FinalizerName) {
		controllerutil.AddFinalizer(obj, v1alpha1.FinalizerName)
		if err := r.Update(ctx, obj); err != nil {
			log.Error(err, "Failed to add finalizer to MacSec resource")
			return ctrl.Result{}, err
		}
		log.Info("Added finalizer to MacSec resource")
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
				log.Error(err, "Failed to update MacSec resource metadata")
				reterr = kerrors.NewAggregate([]error{reterr, err})
			}
			return
		}

		if !equality.Semantic.DeepEqual(orig.Status, obj.Status) {
			if err := r.Status().Patch(ctx, obj, client.MergeFrom(orig)); err != nil {
				log.Error(err, "Failed to update MacSec status")
				reterr = kerrors.NewAggregate([]error{reterr, err})
			}
		}
	}()

	res, err := r.reconcile(ctx, s)
	if err != nil {
		log.Error(err, "Failed to reconcile MacSec resource")
		return ctrl.Result{}, err
	}

	return res, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MacSecReconciler) SetupWithManager(mgr ctrl.Manager) error {
	labelSelector := metav1.LabelSelector{}
	if r.WatchFilterValue != "" {
		labelSelector.MatchLabels = map[string]string{v1alpha1.WatchLabel: r.WatchFilterValue}
	}

	filter, err := predicate.LabelSelectorPredicate(labelSelector)
	if err != nil {
		return fmt.Errorf("failed to create label selector predicate: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.MacSec{}).
		Named("macsec").
		WithEventFilter(filter).
		// Watches enqueues MacSecs for referenced Secret resources.
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.secretToMacSec),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Complete(r)
}

// scope holds the different objects that are read and used during the reconcile.
type macSecScope struct {
	Device     *v1alpha1.Device
	MacSec     *v1alpha1.MacSec
	Interface  *v1alpha1.Interface
	Secrets    []corev1.Secret
	Connection *deviceutil.Connection
	Provider   provider.MacSecProvider
}

func (r *MacSecReconciler) reconcile(ctx context.Context, s *macSecScope) (_ ctrl.Result, reterr error) {
	if s.MacSec.Labels == nil {
		s.MacSec.Labels = make(map[string]string)
	}

	s.MacSec.Labels[v1alpha1.DeviceLabel] = s.Device.Name

	// Ensure the MacSec is owned by the Device.
	if !controllerutil.HasControllerReference(s.MacSec) {
		if err := controllerutil.SetOwnerReference(s.Device, s.MacSec, r.Scheme, controllerutil.WithBlockOwnerDeletion(true)); err != nil {
			return ctrl.Result{}, err
		}
	}

	if err := s.Provider.Connect(ctx, s.Connection); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to connect to provider: %w", err)
	}
	defer func() {
		if err := s.Provider.Disconnect(ctx, s.Connection); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()
	// Ensure the MacSec is realized on the provider.
	err := s.Provider.EnsureMacSec(ctx, &provider.EnsureMacSecRequest{
		MacSec:  s.MacSec,
		Secrets: s.Secrets,
	})

	cond := conditions.FromError(err)
	// As this resource is configuration only, we use the Configured condition as top-level Ready condition.
	cond.Type = v1alpha1.ReadyCondition
	conditions.Set(s.MacSec, cond)

	return ctrl.Result{RequeueAfter: Jitter(r.RequeueInterval)}, nil
}

func (r *MacSecReconciler) finalize(ctx context.Context, s *macSecScope) (reterr error) {
	if err := s.Provider.Connect(ctx, s.Connection); err != nil {
		return fmt.Errorf("failed to connect to provider: %w", err)
	}
	defer func() {
		if err := s.Provider.Disconnect(ctx, s.Connection); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	return s.Provider.DeleteMacSec(ctx, &provider.DeleteMacSecRequest{
		MacSec: s.MacSec,
	})
}

// validatePreSharedKeySecrets validates that all pre-shared key secrets referenced in the MacSec spec exist
func (r *MacSecReconciler) validatePreSharedKeySecrets(ctx context.Context, macSec *v1alpha1.MacSec) ([]corev1.Secret, error) {
	secrets := []corev1.Secret{}
	for _, psk := range macSec.Spec.PreSharedKeyRef {
		secret := new(corev1.Secret)
		if err := r.Get(ctx, client.ObjectKey{
			Namespace: macSec.Namespace,
			Name:      psk.Name,
		}, secret); err != nil {
			return nil, fmt.Errorf("pre-shared key secret not found: %s", psk.Name)
		}
		secrets = append(secrets, *secret)
		for _, key := range []string{"lifetime", "connectivityKeyName", "algorithm"} {
			_, ok := secret.Data[key]
			if !ok {
				return nil, fmt.Errorf("pre-shared key secret %s does not contain a '%s' field", psk.Name, key)
			}
			fmt.Println("secret-data", secret.StringData)
		}
	}
	return secrets, nil
}

func GetInterfaceByName(ctx context.Context, r client.Reader, namespace, name string) (*v1alpha1.Interface, error) {
	obj := new(v1alpha1.Interface)
	if err := r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, obj); err != nil {
		return nil, fmt.Errorf("failed to get %s/%s: %w", v1alpha1.GroupVersion.WithKind("Interface").String(), name, err)
	}
	return obj, nil
}

// secretToMacSec is a [handler.MapFunc] to be used to enqueue requests for reconciliation
// for a MacSec to update when one of its referenced Secrets gets updated.
func (r *MacSecReconciler) secretToMacSec(ctx context.Context, obj client.Object) []ctrl.Request {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		panic(fmt.Sprintf("Expected a Secret but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx)

	macsecs := new(v1alpha1.MacSecList)
	if err := r.List(ctx, macsecs); err != nil {
		log.Error(err, "Failed to list MacSecs")
		return nil
	}

	requests := []ctrl.Request{}
	for _, macsec := range macsecs.Items {
		// Check if this secret is referenced by any of the pre-shared key references
		for _, psk := range macsec.Spec.PreSharedKeyRef {
			if psk.Name == secret.Name && macsec.Namespace == secret.Namespace {
				log.Info("Enqueuing MacSec for reconciliation")
				requests = append(requests, ctrl.Request{
					NamespacedName: client.ObjectKey{
						Name:      macsec.Name,
						Namespace: macsec.Namespace,
					},
				})
			}
		}
	}

	return requests
}
