// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package clientutil

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	klient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/ironcore-dev/network-operator/api/v1alpha1"
)

func TestSecret(t *testing.T) {
	tests := []struct {
		name    string
		secret  *corev1.Secret
		wantErr bool
	}{
		{
			name: "valid secret",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: metav1.NamespaceDefault,
				},
				Data: map[string][]byte{
					"foobar": []byte("baz"),
				},
			},
		},
		{
			name: "valid stringData secret",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: metav1.NamespaceDefault,
				},
				StringData: map[string]string{
					"foobar": "baz",
				},
			},
		},
		{
			name: "missing field",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: metav1.NamespaceDefault,
				},
				Data: map[string][]byte{
					"some-other-key": []byte("unknown"),
				},
			},
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)

			client := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithObjects(test.secret).
				Build()

			c := NewClient(client, metav1.NamespaceDefault)

			v, err := c.Secret(t.Context(), &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "test-secret",
				},
				Key: "foobar",
			})
			if test.wantErr {
				g.Expect(err).To(HaveOccurred())
				g.Expect(v).To(BeNil())
				return
			}
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(v).ToNot(BeNil())
		})
	}
}

func TestConfigMap(t *testing.T) {
	tests := []struct {
		name    string
		cm      *corev1.ConfigMap
		wantErr bool
	}{
		{
			name: "valid configmap reference",
			cm: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-configmap",
					Namespace: metav1.NamespaceDefault,
				},
				Data: map[string]string{
					"foobar": "baz",
				},
			},
		},
		{
			name: "valid binaryData configmap reference",
			cm: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-configmap",
					Namespace: metav1.NamespaceDefault,
				},
				BinaryData: map[string][]byte{
					"foobar": []byte("baz"),
				},
			},
		},
		{
			name: "missing field",
			cm: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-configmap",
					Namespace: metav1.NamespaceDefault,
				},
				Data: map[string]string{
					"some-other-key": "unknown",
				},
			},
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)

			client := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithObjects(test.cm).
				Build()

			c := NewClient(client, metav1.NamespaceDefault)

			v, err := c.ConfigMap(t.Context(), &corev1.ConfigMapKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "test-configmap",
				},
				Key: "foobar",
			})
			if test.wantErr {
				g.Expect(err).To(HaveOccurred())
				g.Expect(v).To(BeNil())
				return
			}
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(v).ToNot(BeNil())
		})
	}
}

func TestBasicAuth(t *testing.T) {
	tests := []struct {
		name    string
		secret  *corev1.Secret
		wantErr bool
	}{
		{
			name: "valid secret",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: metav1.NamespaceDefault,
				},
				Data: map[string][]byte{
					"username": []byte("testuser"),
					"password": []byte("testpass"),
				},
				Type: corev1.SecretTypeBasicAuth,
			},
		},
		{
			name: "invalid type",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: metav1.NamespaceDefault,
				},
				Data: map[string][]byte{
					"username": []byte("testuser"),
					"password": []byte("testpass"),
				},
				Type: corev1.SecretTypeOpaque,
			},
			wantErr: true,
		},
		{
			name: "missing username",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: metav1.NamespaceDefault,
				},
				Data: map[string][]byte{
					"password": []byte("testpass"),
				},
				Type: corev1.SecretTypeBasicAuth,
			},
			wantErr: true,
		},
		{
			name: "missing password",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: metav1.NamespaceDefault,
				},
				Data: map[string][]byte{
					"username": []byte("testuser"),
				},
				Type: corev1.SecretTypeBasicAuth,
			},
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)

			client := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithObjects(test.secret).
				Build()

			c := NewClient(client, metav1.NamespaceDefault)

			user, pass, err := c.BasicAuth(t.Context(), &corev1.SecretReference{Name: "test-secret"})
			if test.wantErr {
				g.Expect(err).To(HaveOccurred())
				g.Expect(user).To(BeNil())
				g.Expect(pass).To(BeNil())
				return
			}
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(user).ToNot(BeNil())
			g.Expect(pass).ToNot(BeNil())
		})
	}
}

func TestCertificate(t *testing.T) {
	g := NewWithT(t)

	pk, err := rsa.GenerateKey(rand.Reader, 2048)
	g.Expect(err).ToNot(HaveOccurred())

	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{Organization: []string{"Go Test Corp"}},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour * 24),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &pk.PublicKey, pk)
	g.Expect(err).ToNot(HaveOccurred())

	var cert bytes.Buffer
	err = pem.Encode(&cert, &pem.Block{Type: "CERTIFICATE", Bytes: der})
	g.Expect(err).ToNot(HaveOccurred())

	var key bytes.Buffer
	err = pem.Encode(&key, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(pk)})
	g.Expect(err).ToNot(HaveOccurred())

	tests := []struct {
		name    string
		secret  *corev1.Secret
		wantErr bool
	}{
		{
			name: "valid certificate secret",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-certificate",
					Namespace: metav1.NamespaceDefault,
				},
				Data: map[string][]byte{
					corev1.TLSCertKey:       cert.Bytes(),
					corev1.TLSPrivateKeyKey: key.Bytes(),
				},
				Type: corev1.SecretTypeTLS,
			},
		},
		{
			name: "invalid type",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-certificate",
					Namespace: metav1.NamespaceDefault,
				},
				Data: map[string][]byte{
					corev1.TLSCertKey:       cert.Bytes(),
					corev1.TLSPrivateKeyKey: key.Bytes(),
				},
			},
			wantErr: true,
		},
		{
			name: "missing certificate",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-certificate",
					Namespace: metav1.NamespaceDefault,
				},
				Data: map[string][]byte{
					corev1.TLSPrivateKeyKey: key.Bytes(),
				},
				Type: corev1.SecretTypeTLS,
			},
			wantErr: true,
		},
		{
			name: "missing private key",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-certificate",
					Namespace: metav1.NamespaceDefault,
				},
				Data: map[string][]byte{
					corev1.TLSCertKey: cert.Bytes(),
				},
				Type: corev1.SecretTypeTLS,
			},
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)

			client := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithObjects(test.secret).
				Build()

			c := NewClient(client, metav1.NamespaceDefault)

			cert, err := c.Certificate(t.Context(), &corev1.SecretReference{
				Name: "test-certificate",
			})
			if test.wantErr {
				g.Expect(err).To(HaveOccurred())
				g.Expect(cert).To(BeNil())
				return
			}
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(cert).ToNot(BeNil())
		})
	}
}

func TestTemplate(t *testing.T) {
	tests := []struct {
		name    string
		object  klient.Object
		src     *v1alpha1.TemplateSource
		want    []byte
		wantErr bool
	}{
		{
			name: "inline template",
			src: &v1alpha1.TemplateSource{
				Inline: ptr.To("inline template content"),
			},
			want: []byte("inline template content"),
		},
		{
			name: "secret template",
			object: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: metav1.NamespaceDefault,
				},
				Data: map[string][]byte{
					"poap.txt": []byte("secret template content"),
				},
			},
			src: &v1alpha1.TemplateSource{
				SecretRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "test-secret",
					},
					Key: "poap.txt",
				},
			},
			want: []byte("secret template content"),
		},
		{
			name: "secret template",
			object: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-configmap",
					Namespace: metav1.NamespaceDefault,
				},
				Data: map[string]string{
					"poap.txt": "configmap template content",
				},
			},
			src: &v1alpha1.TemplateSource{
				ConfigMapRef: &corev1.ConfigMapKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "test-configmap",
					},
					Key: "poap.txt",
				},
			},
			want: []byte("configmap template content"),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)

			objects := []klient.Object{}
			if test.object != nil {
				objects = append(objects, test.object)
			}

			client := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithObjects(objects...).
				Build()

			c := NewClient(client, metav1.NamespaceDefault)

			v, err := c.Template(t.Context(), test.src)
			if test.wantErr {
				g.Expect(err).To(HaveOccurred())
				g.Expect(v).To(BeNil())
				return
			}
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(v).ToNot(BeNil())
			g.Expect(v).To(Equal(test.want))
		})
	}
}
