// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// log is for logging in this package.
var acllog = logf.Log.WithName("accesscontrollist-resource")

// SetupAccessControlListWebhookWithManager registers the webhook for AccessControlList in the manager.
func SetupAccessControlListWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &v1alpha1.AccessControlList{}).
		WithValidator(&AccessControlListCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-networking-metal-ironcore-dev-v1alpha1-accesscontrollist,mutating=false,failurePolicy=Fail,sideEffects=None,groups=networking.metal.ironcore.dev,resources=accesscontrollist,verbs=create;update,versions=v1alpha1,name=accesscontrollist-v1alpha1.kb.io,admissionReviewVersions=v1

// AccessControlListCustomValidator struct is responsible for validating the AccessControlList resource
// when it is created, updated, or deleted.
type AccessControlListCustomValidator struct{}

var _ admission.Validator[*v1alpha1.AccessControlList] = &AccessControlListCustomValidator{}

// ValidateCreate implements [admission.Validator] so a webhook will be registered for the type AccessControlList.
func (v *AccessControlListCustomValidator) ValidateCreate(_ context.Context, obj *v1alpha1.AccessControlList) (admission.Warnings, error) {
	acllog.Info("Validation for AccessControlList upon creation", "name", obj.GetName())
	return nil, validateACLEntries(obj)
}

// ValidateUpdate implements [admission.Validator] so a webhook will be registered for the type AccessControlList.
func (v *AccessControlListCustomValidator) ValidateUpdate(_ context.Context, _, obj *v1alpha1.AccessControlList) (admission.Warnings, error) {
	acllog.Info("Validation for AccessControlList upon update", "name", obj.GetName())
	return nil, validateACLEntries(obj)
}

// ValidateDelete implements [admission.Validator] so a webhook will be registered for the type AccessControlList.
func (v *AccessControlListCustomValidator) ValidateDelete(_ context.Context, _ *v1alpha1.AccessControlList) (admission.Warnings, error) {
	return nil, nil
}

// validateACLEntries ensures all entries use a consistent IP address family
// and that each entry's source and destination addresses use the same family.
func validateACLEntries(acl *v1alpha1.AccessControlList) error {
	if len(acl.Spec.Entries) == 0 {
		return nil
	}
	// Determine the ACL-wide address family from the first entry's source.
	is6 := acl.Spec.Entries[0].SourceAddress.Addr().Is6()
	for i, entry := range acl.Spec.Entries {
		srcIs6 := entry.SourceAddress.Addr().Is6()
		dstIs6 := entry.DestinationAddress.Addr().Is6()
		// Reject mixed families within an entry.
		if srcIs6 != dstIs6 {
			return fmt.Errorf("entries[%d]: sourceAddress and destinationAddress must use the same IP address family", i)
		}
		// Reject mixed families across entries.
		if srcIs6 != is6 {
			return fmt.Errorf("entries[%d]: all entries must use the same IP address family", i)
		}
	}
	return nil
}
