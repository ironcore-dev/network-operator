// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nx

import (
	"context"
	"errors"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	nxv1alpha "github.com/ironcore-dev/network-operator/api/cisco/nx/v1alpha1"
	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/conditions"
	"github.com/ironcore-dev/network-operator/internal/providerconfig"
)

func init() {
	providerconfig.RegisterValidator(nxv1alpha.GroupVersion.WithKind("LLDPConfig"), LLDPConfigValidationFunc)
}

// LLDPConfigScope contains pre-fetched and validated data for LLDP configuration.
// The provider decodes this from unstructured data.
type LLDPConfigScope struct {
	// Interfaces is a map with keys being the interface name (K8s resource name)
	// and values being the corresponding Interface resources.
	Interfaces map[string]*v1alpha1.Interface `json:"interfaces"`
}

func LLDPConfigValidationFunc(ctx context.Context, r client.Reader, parent client.Object, ref *v1alpha1.TypedLocalObjectReference) (*providerconfig.Scope, error) {
	lldp, ok := parent.(*v1alpha1.LLDP)
	if !ok {
		return nil, errors.New("parent is not LLDP")
	}

	cfg := new(nxv1alpha.LLDPConfig)
	if err := r.Get(ctx, client.ObjectKey{Namespace: lldp.Namespace, Name: ref.Name}, cfg); err != nil {
		return nil, fmt.Errorf("failed to get LLDPConfig %q: %w", ref.Name, err)
	}

	s := &LLDPConfigScope{
		Interfaces: make(map[string]*v1alpha1.Interface),
	}
	// Ensure all referenced interfaces exist and belong to the same device as the LLDP.
	for _, ifCfg := range cfg.Spec.InterfaceRefs {
		intf := new(v1alpha1.Interface)
		if err := r.Get(ctx, client.ObjectKey{Name: ifCfg.Name, Namespace: lldp.Namespace}, intf); err != nil {
			if apierrors.IsNotFound(err) {
				msg := fmt.Sprintf("Referenced interface %q not found", ifCfg.Name)
				conditions.Set(lldp, metav1.Condition{
					Type:    v1alpha1.ConfiguredCondition,
					Status:  metav1.ConditionFalse,
					Reason:  v1alpha1.InterfaceNotFoundReason,
					Message: msg,
				})
				return nil, fmt.Errorf("%s: %w", msg, err)
			}
			return nil, fmt.Errorf("failed to get referenced interface %q: %w", ifCfg.Name, err)
		}
		if intf.Spec.DeviceRef.Name != lldp.Spec.DeviceRef.Name {
			msg := fmt.Sprintf("Referenced interface %q belongs to device %q, expected %q", ifCfg.Name, intf.Spec.DeviceRef.Name, lldp.Spec.DeviceRef.Name)
			conditions.Set(lldp, metav1.Condition{
				Type:    v1alpha1.ConfiguredCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.CrossDeviceReferenceReason,
				Message: msg,
			})
			return nil, errors.New(msg)
		}
		if intf.Spec.Type != v1alpha1.InterfaceTypePhysical && intf.Spec.Type != v1alpha1.InterfaceTypeAggregate {
			msg := fmt.Sprintf("Referenced interface %q is of type %q, expected %q or %q", ifCfg.Name, intf.Spec.Type, v1alpha1.InterfaceTypePhysical, v1alpha1.InterfaceTypeAggregate)
			conditions.Set(lldp, metav1.Condition{
				Type:    v1alpha1.ConfiguredCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.InvalidInterfaceTypeReason,
				Message: msg,
			})
			return nil, errors.New(msg)
		}
		s.Interfaces[ifCfg.Name] = intf
	}

	m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(s)
	if err != nil {
		return nil, fmt.Errorf("failed to convert provider scope to unstructured: %w", err)
	}
	u := &unstructured.Unstructured{Object: m}
	return providerconfig.NewScope(u), nil
}
