// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nx

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	nxv1alpha "github.com/ironcore-dev/network-operator/api/cisco/nx/v1alpha1"
	corev1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(nxv1alpha.AddToScheme(scheme))
	return scheme
}

func TestLLDPConfigValidationFunc_Success(t *testing.T) {
	g := NewWithT(t)

	scheme := newTestScheme(t)

	lldp := &corev1.LLDP{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-lldp",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: corev1.LLDPSpec{
			DeviceRef:  corev1.LocalObjectReference{Name: "test-device"},
			AdminState: corev1.AdminStateUp,
		},
	}

	cfg := &nxv1alpha.LLDPConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-config",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: nxv1alpha.LLDPConfigSpec{
			InterfaceRefs: []nxv1alpha.LLDPInterface{{
				LocalObjectReference: corev1.LocalObjectReference{Name: "if1"},
			}},
		},
	}

	intf := &corev1.Interface{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "if1",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: corev1.InterfaceSpec{
			DeviceRef: corev1.LocalObjectReference{Name: "test-device"},
			Name:      "Ethernet1/1",
			Type:      corev1.InterfaceTypePhysical,
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cfg, intf).
		Build()

	ref := &corev1.TypedLocalObjectReference{
		APIVersion: nxv1alpha.GroupVersion.String(),
		Kind:       "LLDPConfig",
		Name:       cfg.Name,
	}

	scope, err := LLDPConfigValidationFunc(t.Context(), c, lldp, ref)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(scope).NotTo(BeNil())

	// Decode back into LLDPConfigScope
	var decoded LLDPConfigScope
	err = scope.Into(&decoded)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(decoded.Interfaces).To(HaveKey("if1"))
	g.Expect(decoded.Interfaces["if1"].Spec.DeviceRef.Name).To(Equal("test-device"))
}

func TestLLDPConfigValidationFunc_ParentNotLLDP(t *testing.T) {
	g := NewWithT(t)

	scheme := newTestScheme(t)

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	parent := &corev1.Interface{}
	ref := &corev1.TypedLocalObjectReference{
		APIVersion: nxv1alpha.GroupVersion.String(),
		Kind:       "LLDPConfig",
		Name:       "does-not-matter",
	}

	scope, err := LLDPConfigValidationFunc(t.Context(), c, parent, ref)
	g.Expect(err).To(HaveOccurred())
	g.Expect(scope).To(BeNil())
}

func TestLLDPConfigValidationFunc_InterfaceNotFoundSetsCondition(t *testing.T) {
	g := NewWithT(t)

	scheme := newTestScheme(t)

	lldp := &corev1.LLDP{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-lldp",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: corev1.LLDPSpec{
			DeviceRef:  corev1.LocalObjectReference{Name: "test-device"},
			AdminState: corev1.AdminStateUp,
		},
	}

	cfg := &nxv1alpha.LLDPConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-config",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: nxv1alpha.LLDPConfigSpec{
			InterfaceRefs: []nxv1alpha.LLDPInterface{{
				LocalObjectReference: corev1.LocalObjectReference{Name: "missing-if"},
			}},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cfg).
		Build()

	ref := &corev1.TypedLocalObjectReference{
		APIVersion: nxv1alpha.GroupVersion.String(),
		Kind:       "LLDPConfig",
		Name:       cfg.Name,
	}

	scope, err := LLDPConfigValidationFunc(t.Context(), c, lldp, ref)
	g.Expect(err).To(HaveOccurred())
	g.Expect(scope).To(BeNil())

	g.Expect(lldp.Status.Conditions).ToNot(BeEmpty())
	cond := lldp.Status.Conditions[len(lldp.Status.Conditions)-1]
	g.Expect(cond.Type).To(Equal(corev1.ConfiguredCondition))
	g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
	g.Expect(cond.Reason).To(Equal(corev1.InterfaceNotFoundReason))
}

func TestLLDPConfigValidationFunc_CrossDeviceReferenceSetsCondition(t *testing.T) {
	g := NewWithT(t)

	scheme := newTestScheme(t)

	lldp := &corev1.LLDP{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-lldp",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: corev1.LLDPSpec{
			DeviceRef:  corev1.LocalObjectReference{Name: "device-a"},
			AdminState: corev1.AdminStateUp,
		},
	}

	cfg := &nxv1alpha.LLDPConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-config",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: nxv1alpha.LLDPConfigSpec{
			InterfaceRefs: []nxv1alpha.LLDPInterface{{
				LocalObjectReference: corev1.LocalObjectReference{Name: "if1"},
			}},
		},
	}

	// Interface belongs to a different device
	intf := &corev1.Interface{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "if1",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: corev1.InterfaceSpec{
			DeviceRef: corev1.LocalObjectReference{Name: "device-b"},
			Name:      "Ethernet1/1",
			Type:      corev1.InterfaceTypePhysical,
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cfg, intf).
		Build()

	ref := &corev1.TypedLocalObjectReference{
		APIVersion: nxv1alpha.GroupVersion.String(),
		Kind:       "LLDPConfig",
		Name:       cfg.Name,
	}

	scope, err := LLDPConfigValidationFunc(t.Context(), c, lldp, ref)
	g.Expect(err).To(HaveOccurred())
	g.Expect(scope).To(BeNil())

	g.Expect(lldp.Status.Conditions).ToNot(BeEmpty())
	cond := lldp.Status.Conditions[len(lldp.Status.Conditions)-1]
	g.Expect(cond.Type).To(Equal(corev1.ConfiguredCondition))
	g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
	g.Expect(cond.Reason).To(Equal(corev1.CrossDeviceReferenceReason))
}
