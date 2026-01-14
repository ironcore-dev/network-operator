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
	"github.com/ironcore-dev/network-operator/internal/resourcelock"
)

// RoutingPolicyReconciler reconciles a RoutingPolicy object
type RoutingPolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// WatchFilterValue is the label value used to filter events prior to reconciliation.
	WatchFilterValue string

	// Recorder is used to record events for the controller.
	// More info: https://book.kubebuilder.io/reference/raising-events
	Recorder record.EventRecorder

	// Provider is the driver that will be used to create & delete the routingpolicy.
	Provider provider.ProviderFunc

	// Locker is used to synchronize operations on resources targeting the same device.
	Locker *resourcelock.ResourceLocker
}

// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=routingpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=routingpolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=routingpolicies/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.2/pkg/reconcile
//
// For more details about the method shape, read up here:
// - https://ahmet.im/blog/controller-pitfalls/#reconcile-method-shape
func (r *RoutingPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling resource")

	obj := new(v1alpha1.RoutingPolicy)
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

	prov, ok := r.Provider().(provider.RoutingPolicyProvider)
	if !ok {
		if meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.NotImplementedReason,
			Message: "Provider does not implement provider.RoutingPolicyProvider",
		}) {
			return ctrl.Result{}, r.Status().Update(ctx, obj)
		}
		return ctrl.Result{}, nil
	}

	device, err := deviceutil.GetDeviceByName(ctx, r, obj.Namespace, obj.Spec.DeviceRef.Name)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := r.Locker.AcquireLock(ctx, device.Name, "routingpolicy-controller"); err != nil {
		if errors.Is(err, resourcelock.ErrLockAlreadyHeld) {
			log.Info("Device is already locked, requeuing reconciliation")
			return ctrl.Result{RequeueAfter: time.Second * 5}, nil
		}
		log.Error(err, "Failed to acquire device lock")
		return ctrl.Result{}, err
	}
	defer func() {
		if err := r.Locker.ReleaseLock(ctx, device.Name, "routingpolicy-controller"); err != nil {
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

	s := &routingPolicyScope{
		Device:         device,
		RoutingPolicy:  obj,
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

var routingPolicyPrefixSetRefKey = ".spec.statements[].conditions.matchPrefixSet.prefixSetRef.name"

// SetupWithManager sets up the controller with the Manager.
func (r *RoutingPolicyReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	labelSelector := metav1.LabelSelector{}
	if r.WatchFilterValue != "" {
		labelSelector.MatchLabels = map[string]string{v1alpha1.WatchLabel: r.WatchFilterValue}
	}

	filter, err := predicate.LabelSelectorPredicate(labelSelector)
	if err != nil {
		return fmt.Errorf("failed to create label selector predicate: %w", err)
	}

	if err := mgr.GetFieldIndexer().IndexField(ctx, &v1alpha1.RoutingPolicy{}, routingPolicyPrefixSetRefKey, func(obj client.Object) []string {
		rp := obj.(*v1alpha1.RoutingPolicy)
		var names []string
		for _, stmt := range rp.Spec.Statements {
			if stmt.Conditions != nil && stmt.Conditions.MatchPrefixSet != nil {
				names = append(names, stmt.Conditions.MatchPrefixSet.PrefixSetRef.Name)
			}
		}
		return names
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.RoutingPolicy{}).
		Named("routingpolicy").
		WithEventFilter(filter).
		// Watches enqueues RoutingPolicies for updates in referenced PrefixSet resources.
		// Only triggers on create and delete events since PrefixSet names are immutable.
		Watches(
			&v1alpha1.PrefixSet{},
			handler.EnqueueRequestsFromMapFunc(r.prefixSetToRoutingPolicy),
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

// scope holds the different objects that are read and used during the reconcile.
type routingPolicyScope struct {
	Device         *v1alpha1.Device
	RoutingPolicy  *v1alpha1.RoutingPolicy
	Connection     *deviceutil.Connection
	ProviderConfig *provider.ProviderConfig
	Provider       provider.RoutingPolicyProvider
}

func (r *RoutingPolicyReconciler) reconcile(ctx context.Context, s *routingPolicyScope) (reterr error) {
	if s.RoutingPolicy.Labels == nil {
		s.RoutingPolicy.Labels = make(map[string]string)
	}

	s.RoutingPolicy.Labels[v1alpha1.DeviceLabel] = s.Device.Name

	// Ensure the RoutingPolicy is owned by the Device.
	if !controllerutil.HasControllerReference(s.RoutingPolicy) {
		if err := controllerutil.SetOwnerReference(s.Device, s.RoutingPolicy, r.Scheme, controllerutil.WithBlockOwnerDeletion(true)); err != nil {
			return err
		}
	}

	statements, err := r.reconcileStatements(ctx, s)
	if err != nil {
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

	// Ensure the RoutingPolicy is realized on the provider.
	err = s.Provider.EnsureRoutingPolicy(ctx, &provider.EnsureRoutingPolicyRequest{
		Name:           s.RoutingPolicy.Spec.Name,
		Statements:     statements,
		ProviderConfig: s.ProviderConfig,
	})

	cond := conditions.FromError(err)
	// As this resource is configuration only, we use the Configured condition as top-level Ready condition.
	cond.Type = v1alpha1.ReadyCondition
	conditions.Set(s.RoutingPolicy, cond)

	return err
}

func (r *RoutingPolicyReconciler) reconcileStatements(ctx context.Context, s *routingPolicyScope) ([]provider.PolicyStatement, error) {
	statements := make([]provider.PolicyStatement, 0, len(s.RoutingPolicy.Spec.Statements))

	for _, stmt := range s.RoutingPolicy.Spec.Statements {
		var cond []provider.PolicyCondition
		if stmt.Conditions != nil && stmt.Conditions.MatchPrefixSet != nil {
			prefixSet, err := r.reconcilePrefixSet(ctx, s, stmt.Conditions.MatchPrefixSet)
			if err != nil {
				return nil, err
			}
			cond = append(cond, provider.MatchPrefixSetCondition{
				PrefixSet: prefixSet,
			})
		}

		statements = append(statements, provider.PolicyStatement{
			Sequence:   stmt.Sequence,
			Conditions: cond,
			Actions:    stmt.Actions,
		})
	}

	return statements, nil
}

// reconcilePrefixSet ensures that the referenced PrefixSet exists and belongs to the same device as the RoutingPolicy.
func (r *RoutingPolicyReconciler) reconcilePrefixSet(ctx context.Context, s *routingPolicyScope, c *v1alpha1.PrefixSetMatchCondition) (*v1alpha1.PrefixSet, error) {
	key := client.ObjectKey{
		Name:      c.PrefixSetRef.Name,
		Namespace: s.RoutingPolicy.Namespace,
	}

	prefixSet := new(v1alpha1.PrefixSet)
	if err := r.Get(ctx, key, prefixSet); err != nil {
		if apierrors.IsNotFound(err) {
			conditions.Set(s.RoutingPolicy, metav1.Condition{
				Type:    v1alpha1.ReadyCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.PrefixSetNotFoundReason,
				Message: fmt.Sprintf("referenced PrefixSet %q not found", key),
			})
			return nil, reconcile.TerminalError(fmt.Errorf("referenced PrefixSet %q not found", key))
		}
		return nil, fmt.Errorf("failed to get referenced PrefixSet %q: %w", key, err)
	}

	if prefixSet.Spec.DeviceRef.Name != s.Device.Name {
		conditions.Set(s.RoutingPolicy, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.CrossDeviceReferenceReason,
			Message: fmt.Sprintf("referenced PrefixSet %q does not belong to device %q", prefixSet.Name, s.Device.Name),
		})
		return nil, reconcile.TerminalError(fmt.Errorf("referenced PrefixSet %q does not belong to device %q", prefixSet.Name, s.Device.Name))
	}

	return prefixSet, nil
}

func (r *RoutingPolicyReconciler) finalize(ctx context.Context, s *routingPolicyScope) (reterr error) {
	if err := s.Provider.Connect(ctx, s.Connection); err != nil {
		return fmt.Errorf("failed to connect to provider: %w", err)
	}
	defer func() {
		if err := s.Provider.Disconnect(ctx, s.Connection); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	return s.Provider.DeleteRoutingPolicy(ctx, &provider.DeleteRoutingPolicyRequest{
		Name: s.RoutingPolicy.Spec.Name,
	})
}

// prefixSetToRoutingPolicy is a [handler.MapFunc] to be used to enqueue requests for reconciliation
// for RoutingPolicies when their referenced PrefixSet changes.
func (r *RoutingPolicyReconciler) prefixSetToRoutingPolicy(ctx context.Context, obj client.Object) []ctrl.Request {
	prefixSet, ok := obj.(*v1alpha1.PrefixSet)
	if !ok {
		panic(fmt.Sprintf("Expected a PrefixSet but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx, "PrefixSet", klog.KObj(prefixSet))

	routingPolicies := new(v1alpha1.RoutingPolicyList)
	if err := r.List(ctx, routingPolicies, client.InNamespace(prefixSet.Namespace), client.MatchingFields{routingPolicyPrefixSetRefKey: prefixSet.Spec.Name}); err != nil {
		log.Error(err, "Failed to list RoutingPolicies")
		return nil
	}

	requests := []ctrl.Request{}
	for _, rp := range routingPolicies.Items {
		for _, stmt := range rp.Spec.Statements {
			if stmt.Conditions != nil && stmt.Conditions.MatchPrefixSet != nil && stmt.Conditions.MatchPrefixSet.PrefixSetRef.Name == prefixSet.Spec.Name {
				log.Info("Enqueuing RoutingPolicy for reconciliation", "RoutingPolicy", klog.KObj(&rp))
				requests = append(requests, ctrl.Request{
					NamespacedName: client.ObjectKey{
						Name:      rp.Name,
						Namespace: rp.Namespace,
					},
				})
				break
			}
		}
	}

	return requests
}
