// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package evpn

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	evpnv1alpha1 "github.com/ironcore-dev/network-operator/api/evpn/v1alpha1"
	overlayv1alpha1 "github.com/ironcore-dev/network-operator/api/overlay/v1alpha1"
	poolv1alpha1 "github.com/ironcore-dev/network-operator/api/pool/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/conditions"
)

// NetworkReconciler reconciles a Network object.
type NetworkReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// WatchFilterValue is the label value used to filter events prior to reconciliation.
	WatchFilterValue string

	// Recorder is used to record events for the controller.
	// More info: https://book.kubebuilder.io/reference/raising-events
	Recorder events.EventRecorder
}

// +kubebuilder:rbac:groups=overlay.networking.metal.ironcore.dev,resources=networks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=overlay.networking.metal.ironcore.dev,resources=networks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=overlay.networking.metal.ironcore.dev,resources=networks/finalizers,verbs=update
// +kubebuilder:rbac:groups=overlay.networking.metal.ironcore.dev,resources=networkattachments,verbs=get;list;watch
// +kubebuilder:rbac:groups=evpn.networking.metal.ironcore.dev,resources=fabrics,verbs=get;list;watch
// +kubebuilder:rbac:groups=pool.networking.metal.ironcore.dev,resources=claims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=evpninstances,verbs=get;list;watch
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.24.1/pkg/reconcile
func (r *NetworkReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling resource")

	net := new(overlayv1alpha1.Network)
	if err := r.Get(ctx, req.NamespacedName, net); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get resource")
		return ctrl.Result{}, err
	}

	if !net.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(net, evpnv1alpha1.FinalizerName) {
			complete, err := r.finalize(ctx, net)
			if err != nil {
				log.Error(err, "Failed to finalize resource")
				return ctrl.Result{}, err
			}
			if !complete {
				return ctrl.Result{}, nil
			}
			controllerutil.RemoveFinalizer(net, evpnv1alpha1.FinalizerName)
			if err := r.Update(ctx, net); err != nil {
				log.Error(err, "Failed to remove finalizer from resource")
				return ctrl.Result{}, err
			}
		}
		log.Info("Resource is being deleted, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers
	if !controllerutil.ContainsFinalizer(net, evpnv1alpha1.FinalizerName) {
		controllerutil.AddFinalizer(net, evpnv1alpha1.FinalizerName)
		if err := r.Update(ctx, net); err != nil {
			return ctrl.Result{}, fmt.Errorf("add Network finalizer: %w", err)
		}
		log.Info("Added finalizer to resource")
		return ctrl.Result{}, nil
	}

	orig := net.DeepCopy()
	if conditions.InitializeConditions(net, v1alpha1.ReadyCondition) {
		log.V(1).Info("Initializing status conditions")
		return ctrl.Result{}, r.Status().Update(ctx, net)
	}

	defer func() {
		if !equality.Semantic.DeepEqual(orig.Status, net.Status) {
			// Pass obj.DeepCopy() to avoid Patch() modifying obj and interfering with metadata update below
			if err := r.Status().Patch(ctx, net.DeepCopy(), client.MergeFrom(orig)); err != nil {
				log.Error(err, "Failed to update status")
				reterr = kerrors.NewAggregate([]error{reterr, err})
			}
		}
		if !equality.Semantic.DeepEqual(orig.ObjectMeta, net.ObjectMeta) {
			if err := r.Patch(ctx, net, client.MergeFrom(orig)); err != nil {
				log.Error(err, "Failed to update resource metadata")
				reterr = kerrors.NewAggregate([]error{reterr, err})
			}
		}
	}()

	if err := r.reconcile(ctx, net); err != nil {
		log.Error(err, "Failed to reconcile resource")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NetworkReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	labelSelector := metav1.LabelSelector{}
	if r.WatchFilterValue != "" {
		labelSelector.MatchLabels = map[string]string{v1alpha1.WatchLabel: r.WatchFilterValue}
	}

	filter, err := predicate.LabelSelectorPredicate(labelSelector)
	if err != nil {
		return fmt.Errorf("create label selector predicate: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&overlayv1alpha1.Network{}).
		Named("evpn-network").
		WithEventFilter(filter).
		// Watches enqueues Networks when attachments resolve their forwarding context or start deletion.
		Watches(
			&overlayv1alpha1.NetworkAttachment{},
			handler.EnqueueRequestsFromMapFunc(r.networkAttachmentToNetworks),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldNA := e.ObjectOld.(*overlayv1alpha1.NetworkAttachment)
					newNA := e.ObjectNew.(*overlayv1alpha1.NetworkAttachment)
					return !equality.Semantic.DeepEqual(oldNA.Status.ForwardingContextRef, newNA.Status.ForwardingContextRef) ||
						!equality.Semantic.DeepEqual(oldNA.DeletionTimestamp, newNA.DeletionTimestamp) ||
						!equality.Semantic.DeepEqual(oldNA.Status.Interfaces, newNA.Status.Interfaces)
				},
				GenericFunc: func(event.GenericEvent) bool {
					return false
				},
			}),
		).
		// Claims are Fabric-owned, so map their allocation and deletion changes to the Network explicitly.
		Watches(
			&poolv1alpha1.Claim{},
			handler.EnqueueRequestsFromMapFunc(r.claimToNetworks),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldClaim := e.ObjectOld.(*poolv1alpha1.Claim)
					newClaim := e.ObjectNew.(*poolv1alpha1.Claim)
					return !equality.Semantic.DeepEqual(oldClaim.Status, newClaim.Status) ||
						!equality.Semantic.DeepEqual(oldClaim.DeletionTimestamp, newClaim.DeletionTimestamp)
				},
				GenericFunc: func(event.GenericEvent) bool {
					return false
				},
			}),
		).
		// Watches enqueues Networks as their EVPNInstances finish cleanup.
		Watches(
			&v1alpha1.EVPNInstance{},
			handler.EnqueueRequestsFromMapFunc(r.evpnInstanceToNetworks),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					return !equality.Semantic.DeepEqual(e.ObjectOld.GetDeletionTimestamp(), e.ObjectNew.GetDeletionTimestamp())
				},
				GenericFunc: func(event.GenericEvent) bool { return false },
			}),
		).
		Complete(r)
}

func (r *NetworkReconciler) reconcile(ctx context.Context, net *overlayv1alpha1.Network) error {
	// Network realization currently supports only EVPN; revisit dispatch when another forwarding mechanism is added.
	attachments := &overlayv1alpha1.NetworkAttachmentList{}
	if err := r.List(ctx, attachments, client.InNamespace(net.Namespace), client.MatchingFields{networkAttachmentNetworkRefKey: net.Name}); err != nil {
		return fmt.Errorf("list network attachments for network %q: %w", net.Name, err)
	}

	var forwardingContextRef *v1alpha1.TypedLocalObjectReference
	for i := range attachments.Items {
		na := &attachments.Items[i]
		if !na.DeletionTimestamp.IsZero() {
			continue
		}
		ref := na.Status.ForwardingContextRef
		if ref == nil {
			continue
		}
		if forwardingContextRef != nil && *forwardingContextRef != *ref {
			conditions.Set(net, metav1.Condition{
				Type:    v1alpha1.ReadyCondition,
				Status:  metav1.ConditionFalse,
				Reason:  evpnv1alpha1.StretchedNetworkNotSupportedReason,
				Message: "NetworkAttachments resolve to different forwarding contexts",
			})
			return nil
		}
		forwardingContextRef = ref
	}

	claim := &poolv1alpha1.Claim{
		Name:      "vni-" + hash(net),
		Namespace: net.Namespace,
	}
	err := r.Get(ctx, client.ObjectKeyFromObject(claim), claim)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("get VNI claim %q: %w", claim.Name, err)
	}
	if err == nil && forwardingContextRef == nil {
		owner := metav1.GetControllerOf(claim)
		if owner == nil {
			conditions.Set(net, metav1.Condition{
				Type:    v1alpha1.ReadyCondition,
				Status:  metav1.ConditionFalse,
				Reason:  evpnv1alpha1.FabricResolutionFailedReason,
				Message: fmt.Sprintf("VNI claim %q has no Fabric controller owner", claim.Name),
			})
			return nil
		}
		forwardingContextRef = &v1alpha1.TypedLocalObjectReference{
			APIVersion: owner.APIVersion,
			Kind:       owner.Kind,
			Name:       owner.Name,
		}
	}

	if forwardingContextRef == nil {
		conditions.Set(net, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.WaitingForDependenciesReason,
			Message: "Waiting for NetworkAttachments to resolve a forwarding context",
		})
		return nil
	}

	fabric := &evpnv1alpha1.Fabric{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: net.Namespace, Name: forwardingContextRef.Name}, fabric); err != nil {
		if apierrors.IsNotFound(err) {
			conditions.Set(net, metav1.Condition{
				Type:    v1alpha1.ReadyCondition,
				Status:  metav1.ConditionFalse,
				Reason:  evpnv1alpha1.FabricResolutionFailedReason,
				Message: fmt.Sprintf("Fabric %q not found", forwardingContextRef.Name),
			})
			return nil
		}
		return fmt.Errorf("get fabric %q: %w", forwardingContextRef.Name, err)
	}

	bindingConflict := false
	_, err = controllerutil.CreateOrPatch(ctx, r.Client, claim, func() error {
		if owner := metav1.GetControllerOf(claim); owner != nil && (owner.APIVersion != evpnv1alpha1.GroupVersion.String() || owner.Kind != "Fabric" || owner.UID != fabric.UID) {
			bindingConflict = true
			return fmt.Errorf("VNI claim %q is already bound to fabric %q", claim.Name, owner.Name)
		}
		if claim.Labels == nil {
			claim.Labels = make(map[string]string)
		}
		claim.Labels[evpnv1alpha1.FabricLabel] = fabric.Name
		claim.Labels[evpnv1alpha1.NetworkLabel] = net.Name
		claim.Spec.PoolRef = fabric.Spec.VNIPool
		return controllerutil.SetControllerReference(fabric, claim, r.Scheme)
	})
	if err != nil {
		if bindingConflict {
			conditions.Set(net, metav1.Condition{
				Type:    v1alpha1.ReadyCondition,
				Status:  metav1.ConditionFalse,
				Reason:  evpnv1alpha1.StretchedNetworkNotSupportedReason,
				Message: err.Error(),
			})
			return nil
		}
		return fmt.Errorf("reconcile VNI claim %q: %w", claim.Name, err)
	}

	allocated := conditions.Get(claim, poolv1alpha1.AllocatedCondition)
	if allocated == nil || allocated.Status != metav1.ConditionTrue || claim.Status.Value == "" || !claim.DeletionTimestamp.IsZero() {
		conditions.Set(net, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.WaitingForDependenciesReason,
			Message: fmt.Sprintf("Waiting for VNI claim %q to be allocated", claim.Name),
		})
		return nil
	}

	conditions.Set(net, metav1.Condition{
		Type:    v1alpha1.ReadyCondition,
		Status:  metav1.ConditionTrue,
		Reason:  v1alpha1.ReadyReason,
		Message: "Network is ready",
	})

	return nil
}

func (r *NetworkReconciler) finalize(ctx context.Context, net *overlayv1alpha1.Network) (complete bool, err error) {
	log := ctrl.LoggerFrom(ctx)
	attachments := &overlayv1alpha1.NetworkAttachmentList{}
	if err := r.List(ctx, attachments, client.InNamespace(net.Namespace), client.MatchingFields{networkAttachmentNetworkRefKey: net.Name}); err != nil {
		return false, fmt.Errorf("list network attachments for network %q: %w", net.Name, err)
	}
	for i := range attachments.Items {
		if len(attachments.Items[i].Status.Interfaces) > 0 {
			log.Info("Waiting for NetworkAttachment Interface membership cleanup", "attachment", attachments.Items[i].Name)
			return false, nil
		}
	}
	evis := &v1alpha1.EVPNInstanceList{}
	if err := r.List(ctx, evis, client.InNamespace(net.Namespace), client.MatchingLabels{evpnv1alpha1.NetworkLabel: net.Name}); err != nil {
		return false, fmt.Errorf("list EVPN instances for network %q: %w", net.Name, err)
	}
	if len(evis.Items) > 0 {
		log.Info("Waiting for Network EVPNInstances to be deleted", "count", len(evis.Items))
		return false, nil
	}
	claim := &poolv1alpha1.Claim{Name: "vni-" + hash(net), Namespace: net.Namespace}
	// Claim finalization releases the allocation asynchronously after deletion is requested.
	if err := client.IgnoreNotFound(r.Delete(ctx, claim)); err != nil {
		return false, fmt.Errorf("delete VNI claim %q: %w", claim.Name, err)
	}
	return true, nil
}

// evpnInstanceToNetworks enqueues the Network associated with an EVPNInstance.
func (r *NetworkReconciler) evpnInstanceToNetworks(ctx context.Context, obj client.Object) []ctrl.Request {
	evi, ok := obj.(*v1alpha1.EVPNInstance)
	if !ok {
		panic(fmt.Sprintf("Expected an EVPNInstance but got a %T", obj))
	}
	name := evi.Labels[evpnv1alpha1.NetworkLabel]
	if name == "" {
		return nil
	}
	ctrl.LoggerFrom(ctx, "EVPNInstance", klog.KObj(evi)).V(2).Info("Enqueuing Network for reconciliation", "name", name, "namespace", evi.Namespace)
	return []ctrl.Request{{Namespace: evi.Namespace, Name: name}}
}

// networkAttachmentToNetworks is a [handler.MapFunc] that enqueues
// the Network referenced by the changed NetworkAttachment.
func (r *NetworkReconciler) networkAttachmentToNetworks(ctx context.Context, obj client.Object) []ctrl.Request {
	na, ok := obj.(*overlayv1alpha1.NetworkAttachment)
	if !ok {
		panic(fmt.Sprintf("Expected a NetworkAttachment but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx, "NetworkAttachment", klog.KObj(na))
	log.V(2).Info("Enqueuing Network for reconciliation", "name", na.Spec.NetworkRef.Name, "namespace", na.Namespace)
	return []ctrl.Request{{Namespace: na.Namespace, Name: na.Spec.NetworkRef.Name}}
}

// claimToNetworks is a [handler.MapFunc] that enqueues
// the Network associated with a VNI Claim.
func (r *NetworkReconciler) claimToNetworks(ctx context.Context, obj client.Object) []ctrl.Request {
	claim, ok := obj.(*poolv1alpha1.Claim)
	if !ok {
		panic(fmt.Sprintf("Expected a Claim but got a %T", obj))
	}

	name := claim.Labels[evpnv1alpha1.NetworkLabel]
	if name == "" {
		return nil
	}

	log := ctrl.LoggerFrom(ctx, "Claim", klog.KObj(claim))
	log.V(2).Info("Enqueuing Network for reconciliation", "name", name, "namespace", claim.Namespace)
	return []ctrl.Request{{Namespace: claim.Namespace, Name: name}}
}
