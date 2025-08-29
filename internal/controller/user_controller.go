// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
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
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/ironcore-dev/network-operator/api/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/clientutil"
	"github.com/ironcore-dev/network-operator/internal/deviceutil"
	"github.com/ironcore-dev/network-operator/internal/provider"
)

// UserReconciler reconciles a User object
type UserReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// WatchFilterValue is the label value used to filter events prior to reconciliation.
	WatchFilterValue string

	// Recorder is used to record events for the controller.
	// More info: https://book.kubebuilder.io/reference/raising-events
	Recorder record.EventRecorder

	// Provider is the driver that will be used to create & delete the user.
	Provider provider.ProviderFunc
}

// +kubebuilder:rbac:groups=networking.cloud.sap,resources=users,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.cloud.sap,resources=users/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.cloud.sap,resources=users/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.2/pkg/reconcile
//
// For more details about the method shape, read up here:
// - https://ahmet.im/blog/controller-pitfalls/#reconcile-method-shape
func (r *UserReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling resource")

	obj := new(v1alpha1.User)
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

	prov, ok := r.Provider().(provider.UserProvider)
	if !ok {
		meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.NotImplementedReason,
			Message: "Provider does not implement provider.UserProvider",
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

	var cfg *provider.ProviderConfig
	if obj.Spec.ProviderConfigRef != nil {
		cfg, err = provider.GetProviderConfig(ctx, r, obj.Namespace, obj.Spec.ProviderConfigRef)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	s := &userScope{
		Device:         device,
		User:           obj,
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
	if len(obj.Status.Conditions) == 0 {
		log.Info("Initializing status conditions")
		meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionUnknown,
			Reason:  v1alpha1.ReconcilePendingReason,
			Message: "Starting reconciliation",
		})
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
func (r *UserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	labelSelector := metav1.LabelSelector{}
	if r.WatchFilterValue != "" {
		labelSelector.MatchLabels = map[string]string{v1alpha1.WatchLabel: r.WatchFilterValue}
	}

	filter, err := predicate.LabelSelectorPredicate(labelSelector)
	if err != nil {
		return fmt.Errorf("failed to create label selector predicate: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.User{}).
		Named("user").
		WithEventFilter(filter).
		// Watches enqueues Users for referenced Secret resources.
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.secretToUser),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Complete(r)
}

// scope holds the different objects that are read and used during the reconcile.
type userScope struct {
	Device         *v1alpha1.Device
	User           *v1alpha1.User
	Connection     *deviceutil.Connection
	ProviderConfig *provider.ProviderConfig
	Provider       provider.UserProvider
}

func (r *UserReconciler) reconcile(ctx context.Context, s *userScope) (_ ctrl.Result, reterr error) {
	if s.User.Labels == nil {
		s.User.Labels = make(map[string]string)
	}

	s.User.Labels[v1alpha1.DeviceLabel] = s.Device.Name

	// Ensure the User is owned by the Device.
	if !controllerutil.HasControllerReference(s.User) {
		if err := controllerutil.SetOwnerReference(s.Device, s.User, r.Scheme, controllerutil.WithBlockOwnerDeletion(true)); err != nil {
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

	c := clientutil.NewClient(r, s.User.Namespace)
	pwd, err := c.Secret(ctx, &s.User.Spec.Password.SecretKeyRef)
	if err != nil {
		return ctrl.Result{}, err
	}

	roles := make([]string, 0, len(s.User.Spec.Roles))
	for _, r := range s.User.Spec.Roles {
		roles = append(roles, r.Name)
	}

	var sshKey string
	if s.User.Spec.SSHPublicKey != nil {
		sshKeyBytes, err := c.Secret(ctx, &s.User.Spec.SSHPublicKey.SecretKeyRef)
		if err != nil {
			return ctrl.Result{}, err
		}
		sshKey = string(sshKeyBytes)
	}

	// Ensure the User is realized on the provider.
	res, err := s.Provider.EnsureUser(ctx, &provider.EnsureUserRequest{
		Username:       s.User.Spec.Username,
		Password:       string(pwd),
		Roles:          roles,
		SSHKey:         sshKey,
		ProviderConfig: s.ProviderConfig,
	})
	for _, c := range res.Conditions {
		meta.SetStatusCondition(&s.User.Status.Conditions, c)
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	meta.SetStatusCondition(&s.User.Status.Conditions, metav1.Condition{
		Type:               v1alpha1.ReadyCondition,
		Status:             metav1.ConditionTrue,
		Reason:             v1alpha1.ReadyReason,
		Message:            "User configured successfully",
		ObservedGeneration: s.User.Generation,
	})

	return ctrl.Result{RequeueAfter: res.RequeueAfter}, nil
}

func (r *UserReconciler) finalize(ctx context.Context, s *userScope) (reterr error) {
	if err := s.Provider.Connect(ctx, s.Connection); err != nil {
		return fmt.Errorf("failed to connect to provider: %w", err)
	}
	defer func() {
		if err := s.Provider.Disconnect(ctx, s.Connection); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	return s.Provider.DeleteUser(ctx, &provider.DeleteUserRequest{
		Username:       s.User.Spec.Username,
		ProviderConfig: s.ProviderConfig,
	})
}

// secretToUser is a [handler.MapFunc] to be used to enqueue requests for reconciliation
// for a User to update when one of its referenced Secrets gets updated.
func (r *UserReconciler) secretToUser(ctx context.Context, obj client.Object) []ctrl.Request {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		panic(fmt.Sprintf("Expected a Secret but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx, "Secret", klog.KObj(secret))

	users := new(v1alpha1.UserList)
	if err := r.List(ctx, users); err != nil {
		log.Error(err, "Failed to list Users")
		return nil
	}

	requests := []ctrl.Request{}
	for _, u := range users.Items {
		if u.Spec.Password.SecretKeyRef.Name == secret.Name && u.Namespace == secret.Namespace {
			log.Info("Enqueuing User for reconciliation", "User", klog.KObj(&u))
			requests = append(requests, ctrl.Request{
				NamespacedName: client.ObjectKey{
					Name:      u.Name,
					Namespace: u.Namespace,
				},
			})
			continue
		}

		if u.Spec.SSHPublicKey != nil && u.Spec.SSHPublicKey.SecretKeyRef.Name == secret.Name && u.Namespace == secret.Namespace {
			log.Info("Enqueuing User for reconciliation", "User", klog.KObj(&u))
			requests = append(requests, ctrl.Request{
				NamespacedName: client.ObjectKey{
					Name:      u.Name,
					Namespace: u.Namespace,
				},
			})
		}
	}

	return requests
}
