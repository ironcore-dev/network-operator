// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/ironcore-dev/network-operator/api/v1alpha1"
)

var _ = Describe("Certificate Controller", func() {
	Context("When reconciling a resource", func() {
		const name = "test-certificate"
		key := client.ObjectKey{Name: name, Namespace: metav1.NamespaceDefault}

		BeforeEach(func() {
			By("Creating the custom resource for the Kind Device")
			device := &v1alpha1.Device{}
			if err := k8sClient.Get(ctx, key, device); errors.IsNotFound(err) {
				resource := &v1alpha1.Device{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: metav1.NamespaceDefault,
					},
					Spec: v1alpha1.DeviceSpec{
						Endpoint: v1alpha1.Endpoint{
							Address: "192.168.10.2:9339",
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}

			cert, priv, err := CreateSelfSignedCertificate()
			Expect(err).NotTo(HaveOccurred())

			By("Creating the custom resource for the Kind Secret")
			secret := &corev1.Secret{}
			if err := k8sClient.Get(ctx, key, secret); errors.IsNotFound(err) {
				resource := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: metav1.NamespaceDefault,
					},
					Data: map[string][]byte{
						corev1.TLSCertKey:       cert,
						corev1.TLSPrivateKeyKey: priv,
					},
					Type: corev1.SecretTypeTLS,
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}

			By("Creating the custom resource for the Kind Certificate")
			certificate := &v1alpha1.Certificate{}
			if err := k8sClient.Get(ctx, key, certificate); errors.IsNotFound(err) {
				resource := &v1alpha1.Certificate{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: metav1.NamespaceDefault,
					},
					Spec: v1alpha1.CertificateSpec{
						DeviceRef: v1alpha1.LocalObjectReference{Name: name},
						ID:        "cert1",
						SecretRef: v1alpha1.SecretReference{Name: name},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			var resource client.Object = &v1alpha1.Certificate{}
			err := k8sClient.Get(ctx, key, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Certificate")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

			resource = &v1alpha1.Device{}
			err = k8sClient.Get(ctx, key, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Device")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

			By("Ensuring the resource is deleted from the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.Certs.Has("cert1")).To(BeFalse(), "Certificate should be deleted from the provider")
			}).Should(Succeed())
		})

		It("Should successfully reconcile the resource", func() {
			By("Adding a finalizer to the resource")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Certificate{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(resource, v1alpha1.FinalizerName)).To(BeTrue())
			}).Should(Succeed())

			By("Adding the device label to the resource")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Certificate{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Labels).To(HaveKeyWithValue(v1alpha1.DeviceLabel, name))
			}).Should(Succeed())

			By("Adding the device as a owner reference")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Certificate{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.OwnerReferences).To(HaveLen(1))
				g.Expect(resource.OwnerReferences[0].Kind).To(Equal("Device"))
				g.Expect(resource.OwnerReferences[0].Name).To(Equal(name))
			}).Should(Succeed())

			By("Updating the resource status")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.Certificate{}
				g.Expect(k8sClient.Get(ctx, key, resource)).To(Succeed())
				g.Expect(resource.Status.Conditions).To(HaveLen(1))
				g.Expect(resource.Status.Conditions[0].Type).To(Equal(v1alpha1.ReadyCondition))
				g.Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())

			By("Ensuring the resource is created in the provider")
			Eventually(func(g Gomega) {
				g.Expect(testProvider.Certs.Has("cert1")).To(BeTrue(), "Certificate should be present in the provider")
			}).Should(Succeed())
		})
	})
})

// CreateSelfSignedCertificate creates a new self-signed certificate
// and returns the PEM encoded certificate and private key.
func CreateSelfSignedCertificate() (cert, priv []byte, err error) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"SAP SE"},
			Country:       []string{"DE"},
			Province:      []string{"ST"},
			Locality:      []string{""},
			StreetAddress: []string{"Postplatz 1"},
			PostalCode:    []string{"01067"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour), // Valid for 1 year
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privKey.PublicKey, privKey)
	if err != nil {
		return nil, nil, err
	}

	var certBuf bytes.Buffer
	err = pem.Encode(&certBuf, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to encode certificate: %w", err)
	}

	var privBuf bytes.Buffer
	err = pem.Encode(&privBuf, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privKey),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to encode private key: %w", err)
	}

	return certBuf.Bytes(), privBuf.Bytes(), nil
}
