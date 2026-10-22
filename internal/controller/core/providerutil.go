// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"fmt"
	"slices"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/conditions"
	"github.com/ironcore-dev/network-operator/internal/provider"
)

// conditionObject combines conditions.Setter with client.Object so the function can
// both set status conditions and read the object's namespace.
type conditionObject interface {
	conditions.Setter
	client.Object
}

// validateProviderConfigRef fetches the provider configuration referenced by ref and validates
// that its GVK is listed in deps. It sets a ConfiguredCondition on obj to surface errors:
//   - NotFound: ProviderConfigNotFoundReason, wrapped as TerminalError (the watch on provider
//     configs will re-trigger reconciliation when the referenced resource appears).
//   - Incompatible GVK: IncompatibleProviderConfigRef, wrapped as TerminalError.
//   - Other fetch errors: ErrorReason, returned as-is so the controller requeues.
func validateProviderConfigRef(ctx context.Context, r client.Reader, obj conditionObject, ref *v1alpha1.TypedLocalObjectReference, deps []schema.GroupVersionKind) (*provider.ProviderConfig, error) {
	if ref == nil {
		return nil, nil
	}
	cfg, err := provider.GetProviderConfig(ctx, r, obj.GetNamespace(), ref)
	if err != nil {
		if apierrors.IsNotFound(err) {
			conditions.Set(obj, metav1.Condition{
				Type:    v1alpha1.ConfiguredCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.ProviderConfigNotFoundReason,
				Message: fmt.Sprintf("Failed to get ProviderConfigRef: %v", err),
			})
			return nil, reconcile.TerminalError(err)
		}
		conditions.Set(obj, metav1.Condition{
			Type:    v1alpha1.ConfiguredCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.ErrorReason,
			Message: fmt.Sprintf("Failed to get ProviderConfigRef: %v", err),
		})
		return nil, err
	}
	gv, err := schema.ParseGroupVersion(ref.APIVersion)
	if err != nil {
		conditions.Set(obj, metav1.Condition{
			Type:    v1alpha1.ConfiguredCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.IncompatibleProviderConfigRef,
			Message: fmt.Sprintf("Invalid API version in ProviderConfigRef: %v", err),
		})
		return nil, reconcile.TerminalError(fmt.Errorf("invalid API version %q: %w", ref.APIVersion, err))
	}
	gvk := schema.GroupVersionKind{Group: gv.Group, Version: gv.Version, Kind: ref.Kind}
	if !slices.Contains(deps, gvk) {
		conditions.Set(obj, metav1.Condition{
			Type:    v1alpha1.ConfiguredCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.IncompatibleProviderConfigRef,
			Message: fmt.Sprintf("ProviderConfigRef kind %q with API version %q is not compatible with this device type", ref.Kind, ref.APIVersion),
		})
		return nil, reconcile.TerminalError(fmt.Errorf("unsupported ProviderConfigRef Kind %q on this provider", gvk))
	}
	return cfg, nil
}
