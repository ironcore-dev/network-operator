// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	api "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

var _ = Describe("User Secret and ConfigMap references", func() {
	refs := []struct {
		path      string
		configure func(*api.User, string)
	}{
		{path: "spec.password.secretKeyRef.namespace", configure: func(obj *api.User, namespace string) {
			obj.Spec.Password.SecretKeyRef = api.SecretKeySelector{Name: "secret", Namespace: namespace, Key: "key"}
		}},
		{path: "spec.sshPublicKey.secretKeyRef.namespace", configure: func(obj *api.User, namespace string) {
			obj.Spec.SSHPublicKey = &api.SSHPublicKeySource{SecretKeyRef: api.SecretKeySelector{Name: "secret", Namespace: namespace, Key: "key"}}
		}},
	}
	for _, ref := range refs {
		for _, namespace := range []string{"", "tenant-a", "tenant-b"} {
			It(ref.path+" validates namespace "+namespace, func() {
				obj := &api.User{}
				obj.SetName("test")
				obj.SetNamespace("tenant-a")
				ref.configure(obj, namespace)
				validator := &UserCustomValidator{}
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
