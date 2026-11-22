// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	api "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

var _ = Describe("Device Secret and ConfigMap references", func() {
	refs := []struct {
		path      string
		configure func(*api.Device, string)
	}{
		{path: "spec.endpoint.secretRef.namespace", configure: func(obj *api.Device, namespace string) {
			obj.Spec.Endpoint.SecretRef = &api.SecretReference{Name: "secret", Namespace: namespace}
		}},
		{path: "spec.endpoint.tls.ca.namespace", configure: func(obj *api.Device, namespace string) {
			obj.Spec.Endpoint.TLS = &api.TLS{CA: api.SecretKeySelector{Name: "secret", Namespace: namespace, Key: "key"}}
		}},
		{path: "spec.endpoint.tls.certificate.secretRef.namespace", configure: func(obj *api.Device, namespace string) {
			obj.Spec.Endpoint.TLS = &api.TLS{Certificate: &api.CertificateSource{SecretRef: api.SecretReference{Name: "secret", Namespace: namespace}}}
		}},
		{path: "spec.provisioning.bootScript.secretRef.namespace", configure: func(obj *api.Device, namespace string) {
			obj.Spec.Provisioning = &api.Provisioning{BootScript: api.TemplateSource{SecretRef: &api.SecretKeySelector{Name: "secret", Namespace: namespace, Key: "key"}}}
		}},
		{path: "spec.provisioning.bootScript.configMapRef.namespace", configure: func(obj *api.Device, namespace string) {
			obj.Spec.Provisioning = &api.Provisioning{BootScript: api.TemplateSource{ConfigMapRef: &api.ConfigMapKeySelector{Name: "config", Namespace: namespace, Key: "key"}}}
		}},
	}
	for _, ref := range refs {
		for _, namespace := range []string{"", "tenant-a", "tenant-b"} {
			It(ref.path+" validates namespace "+namespace, func() {
				obj := &api.Device{}
				obj.SetName("test")
				obj.SetNamespace("tenant-a")
				ref.configure(obj, namespace)
				validator := &DeviceCustomValidator{}
				_, createErr := validator.ValidateCreate(ctx, obj)
				_, updateErr := validator.ValidateUpdate(ctx, obj.DeepCopy(), obj)
				if namespace == "tenant-b" {
					Expect(apierrors.IsInvalid(createErr)).To(BeTrue())
					Expect(createErr.Error()).To(ContainSubstring(ref.path))
					Expect(apierrors.IsInvalid(updateErr)).To(BeTrue())
					Expect(validator.ValidateDelete(ctx, obj)).Error().NotTo(HaveOccurred())
					repaired := obj.DeepCopy()
					ref.configure(repaired, "")
					Expect(validator.ValidateUpdate(ctx, obj, repaired)).Error().NotTo(HaveOccurred())
				} else {
					Expect(createErr).NotTo(HaveOccurred())
					Expect(updateErr).NotTo(HaveOccurred())
				}
			})
		}
	}
})
