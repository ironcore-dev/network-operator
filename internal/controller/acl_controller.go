// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/ironcore-dev/network-operator/api/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/conditions"
	"github.com/ironcore-dev/network-operator/internal/provider"
)

// AccessControlListReconciler reconciles a AccessControlList object
type AccessControlListReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// WatchFilterValue is the label value used to filter events prior to reconciliation.
	WatchFilterValue string

	// Recorder is used to record events for the controller.
	// More info: https://book.kubebuilder.io/reference/raising-events
	Recorder record.EventRecorder

	// Provider is the driver that will be used to create & delete the accesscontrollist.
	Provider provider.ProviderFunc
}

// +kubebuilder:rbac:groups=networking.cloud.sap,resources=accesscontrollists,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.cloud.sap,resources=accesscontrollists/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.cloud.sap,resources=accesscontrollists/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// SetupWithManager sets up the controller with the Manager.
func (r *AccessControlListReconciler) SetupWithManager(mgr ctrl.Manager) error {
	labelSelector := metav1.LabelSelector{}
	if r.WatchFilterValue != "" {
		labelSelector.MatchLabels = map[string]string{v1alpha1.WatchLabel: r.WatchFilterValue}
	}

	filter, err := predicate.LabelSelectorPredicate(labelSelector)
	if err != nil {
		return fmt.Errorf("failed to create label selector predicate: %w", err)
	}

	rec := AsReconciler(r.Client, r.Provider, r)
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.AccessControlList{}).
		Named("accesscontrollist").
		WithEventFilter(filter).
		Complete(rec)
}

func (r *AccessControlListReconciler) Reconcile(ctx context.Context, s *TypedScope[*v1alpha1.AccessControlList, provider.ACLProvider]) (reterr error) {
	if s.Resource.Labels == nil {
		s.Resource.Labels = make(map[string]string)
	}

	s.Resource.Labels[v1alpha1.DeviceLabel] = s.Device.Name

	// Ensure the AccessControlList is owned by the Device.
	if !controllerutil.HasControllerReference(s.Resource) {
		if err := controllerutil.SetOwnerReference(s.Device, s.Resource, r.Scheme, controllerutil.WithBlockOwnerDeletion(true)); err != nil {
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

	// Ensure the AccessControlList is realized on the provider.
	err := s.Provider.EnsureACL(ctx, &provider.EnsureACLRequest{
		ACL:            s.Resource,
		ProviderConfig: s.ProviderConfig,
	})

	cond := conditions.FromError(err)
	// As this resource is configuration only, we use the Configured condition as top-level Ready condition.
	cond.Type = v1alpha1.ReadyCondition
	conditions.Set(s.Resource, cond)

	return err
}

func (r *AccessControlListReconciler) Finalize(ctx context.Context, s *TypedScope[*v1alpha1.AccessControlList, provider.ACLProvider]) (reterr error) {
	if err := s.Provider.Connect(ctx, s.Connection); err != nil {
		return fmt.Errorf("failed to connect to provider: %w", err)
	}
	defer func() {
		if err := s.Provider.Disconnect(ctx, s.Connection); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	return s.Provider.DeleteACL(ctx, &provider.DeleteACLRequest{
		Name:           s.Resource.Spec.Name,
		ProviderConfig: s.ProviderConfig,
	})
}
