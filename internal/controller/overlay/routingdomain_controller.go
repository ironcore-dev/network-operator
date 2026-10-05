// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package overlay

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	overlayv1alpha1 "github.com/ironcore-dev/network-operator/api/overlay/v1alpha1"
)

// RoutingDomainReconciler reconciles a RoutingDomain object
type RoutingDomainReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=overlay.networking.metal.ironcore.dev,resources=routingdomains,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=overlay.networking.metal.ironcore.dev,resources=routingdomains/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=overlay.networking.metal.ironcore.dev,resources=routingdomains/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the RoutingDomain object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.24.1/pkg/reconcile
func (r *RoutingDomainReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = logf.FromContext(ctx)

	// TODO(user): your logic here

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RoutingDomainReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&overlayv1alpha1.RoutingDomain{}).
		Named("overlay-routingdomain").
		Complete(r)
}
