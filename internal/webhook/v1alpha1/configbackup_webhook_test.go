// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	api "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

var _ = Describe("ConfigBackup Secret and ConfigMap references", func() {
	refs := []struct {
		path      string
		configure func(*api.ConfigBackup, string)
	}{
		{path: "spec.s3.credentialsSecretRef.namespace", configure: func(obj *api.ConfigBackup, namespace string) {
			obj.Spec.S3 = &api.ConfigBackupS3{CredentialsSecretRef: api.SecretReference{Name: "secret", Namespace: namespace}}
		}},
		{path: "spec.s3.encryption.keySecret.namespace", configure: func(obj *api.ConfigBackup, namespace string) {
			obj.Spec.S3 = &api.ConfigBackupS3{Encryption: &api.ConfigBackupEncryption{KeySecret: api.SecretKeySelector{Name: "secret", Namespace: namespace, Key: "key"}}}
		}},
	}
	for _, ref := range refs {
		for _, namespace := range []string{"", "tenant-a", "tenant-b"} {
			It(ref.path+" validates namespace "+namespace, func() {
				obj := &api.ConfigBackup{}
				obj.SetName("test")
				obj.SetNamespace("tenant-a")
				ref.configure(obj, namespace)
				validator := &ConfigBackupCustomValidator{}
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
