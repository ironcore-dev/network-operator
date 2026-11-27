// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	api "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

var _ = Describe("AAA Secret and ConfigMap references", func() {
	refs := []struct {
		path      string
		configure func(*api.AAA, string)
	}{
		{path: "spec.serverGroups[0].servers[0].tacacs.keySecretRef.namespace", configure: func(obj *api.AAA, namespace string) {
			obj.Spec.ServerGroups = []api.AAAServerGroup{{Servers: []api.AAAServer{{TACACS: &api.AAAServerTACACS{KeySecretRef: api.SecretKeySelector{Name: "secret", Namespace: namespace, Key: "key"}}}}}}
		}},
		{path: "spec.serverGroups[0].servers[0].radius.keySecretRef.namespace", configure: func(obj *api.AAA, namespace string) {
			obj.Spec.ServerGroups = []api.AAAServerGroup{{Servers: []api.AAAServer{{RADIUS: &api.AAAServerRADIUS{KeySecretRef: api.SecretKeySelector{Name: "secret", Namespace: namespace, Key: "key"}}}}}}
		}},
	}
	for _, ref := range refs {
		for _, namespace := range []string{"", "tenant-a", "tenant-b"} {
			It(ref.path+" validates namespace "+namespace, func() {
				obj := &api.AAA{}
				obj.SetName("test")
				obj.SetNamespace("tenant-a")
				ref.configure(obj, namespace)
				validator := &AAACustomValidator{}
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
