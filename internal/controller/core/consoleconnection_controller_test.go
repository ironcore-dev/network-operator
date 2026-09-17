// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"net"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

var _ = Describe("ConsoleConnection Controller", func() {
	Context("When reconciling a resource", func() {
		var (
			name string
			key  client.ObjectKey
		)

		BeforeEach(func() {
			By("Creating the Device")
			device := &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-console-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{
						Address: "192.168.10.2:9339",
					},
				},
			}
			Expect(k8sClient.Create(ctx, device)).To(Succeed())
			name = device.Name
			key = client.ObjectKey{Name: name, Namespace: metav1.NamespaceDefault}

			By("Creating the auth secret")
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name + "-console",
					Namespace: metav1.NamespaceDefault,
				},
				Type: corev1.SecretTypeBasicAuth,
				Data: map[string][]byte{
					corev1.BasicAuthUsernameKey: []byte("admin"),
					corev1.BasicAuthPasswordKey: []byte("password"),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())
		})

		AfterEach(func() {
			By("Cleaning up the ConsoleConnection resource")
			cc := &v1alpha1.ConsoleConnection{}
			cc.Name = name
			cc.Namespace = metav1.NamespaceDefault
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, cc))).To(Succeed())

			By("Waiting for the ConsoleConnection to be deleted")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, key, &v1alpha1.ConsoleConnection{})
				g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
			}).Should(Succeed())

			By("Cleaning up the secret")
			secret := &corev1.Secret{}
			secret.Name = name + "-console"
			secret.Namespace = metav1.NamespaceDefault
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, secret))).To(Succeed())

			By("Cleaning up the Device resource")
			device := &v1alpha1.Device{}
			device.Name = name
			device.Namespace = metav1.NamespaceDefault
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, device))).To(Succeed())
		})

		It("Should add a finalizer and set owner reference", func() {
			resource := &v1alpha1.ConsoleConnection{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.ConsoleConnectionSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
					Endpoint: v1alpha1.ConsoleEndpoint{
						Address:   "10.0.0.1:2001",
						SecretRef: v1alpha1.SecretReference{Name: name + "-console"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			Eventually(func(g Gomega) {
				cc := &v1alpha1.ConsoleConnection{}
				g.Expect(k8sClient.Get(ctx, key, cc)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(cc, v1alpha1.FinalizerName)).To(BeTrue())
			}).Should(Succeed())

			Eventually(func(g Gomega) {
				cc := &v1alpha1.ConsoleConnection{}
				g.Expect(k8sClient.Get(ctx, key, cc)).To(Succeed())
				g.Expect(cc.Labels).To(HaveKeyWithValue(v1alpha1.DeviceLabel, name))
				g.Expect(cc.OwnerReferences).To(HaveLen(1))
				g.Expect(cc.OwnerReferences[0].Kind).To(Equal("Device"))
				g.Expect(cc.OwnerReferences[0].Name).To(Equal(name))
			}).Should(Succeed())
		})

		It("Should report ConsoleServerUnreachable when console server is not reachable", func() {
			resource := &v1alpha1.ConsoleConnection{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.ConsoleConnectionSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
					Endpoint: v1alpha1.ConsoleEndpoint{
						Address:   "192.0.2.1:2001",
						SecretRef: v1alpha1.SecretReference{Name: name + "-console"},
					},
					Verification: v1alpha1.ConsoleVerification{
						Expect: &v1alpha1.ConsoleExpect{String: new("anything")},
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			Eventually(func(g Gomega) {
				cc := &v1alpha1.ConsoleConnection{}
				g.Expect(k8sClient.Get(ctx, key, cc)).To(Succeed())
				g.Expect(cc.Status.LastCheckTime).NotTo(BeNil())
				g.Expect(cc.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition),
					HaveField("Status", metav1.ConditionFalse),
					HaveField("Reason", v1alpha1.ConsoleServerUnreachableReason),
				)))
			}).Should(Succeed())
		})

		It("Should report ConsoleServerAuthFailure when credentials are wrong", func() {
			addr, cleanup := StartTestSSHServer("other", nil)
			DeferCleanup(cleanup)

			resource := &v1alpha1.ConsoleConnection{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.ConsoleConnectionSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
					Endpoint: v1alpha1.ConsoleEndpoint{
						Address:   addr,
						SecretRef: v1alpha1.SecretReference{Name: name + "-console"}, // has admin/password, server expects admin/other
					},
					Verification: v1alpha1.ConsoleVerification{
						Expect: &v1alpha1.ConsoleExpect{String: new("anything")},
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			Eventually(func(g Gomega) {
				cc := &v1alpha1.ConsoleConnection{}
				g.Expect(k8sClient.Get(ctx, key, cc)).To(Succeed())
				g.Expect(cc.Status.LastCheckTime).NotTo(BeNil())
				g.Expect(cc.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition),
					HaveField("Status", metav1.ConditionFalse),
					HaveField("Reason", v1alpha1.ConsoleServerAuthFailureReason),
				)))
			}).Should(Succeed())
		})

		It("Should report Dead when no output is received", func() {
			addr, cleanup := StartTestSSHServer("password", nil)
			DeferCleanup(cleanup)

			resource := &v1alpha1.ConsoleConnection{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.ConsoleConnectionSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
					Endpoint: v1alpha1.ConsoleEndpoint{
						Address:   addr,
						SecretRef: v1alpha1.SecretReference{Name: name + "-console"},
					},
					Timeout: metav1.Duration{Duration: 2 * time.Second},
					Verification: v1alpha1.ConsoleVerification{
						Strategy: v1alpha1.ConsoleVerificationWait,
						Expect:   &v1alpha1.ConsoleExpect{String: new("anything")},
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			Eventually(func(g Gomega) {
				cc := &v1alpha1.ConsoleConnection{}
				g.Expect(k8sClient.Get(ctx, key, cc)).To(Succeed())
				g.Expect(cc.Status.LastCheckTime).NotTo(BeNil())
				g.Expect(cc.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition),
					HaveField("Status", metav1.ConditionFalse),
					HaveField("Reason", v1alpha1.ConsoleDeadReason),
				)))
			}).Should(Succeed())
		})

		It("Should report Alive when output does not match expected string", func() {
			addr, cleanup := StartTestSSHServer("password", []byte("switch-B login:"))
			DeferCleanup(cleanup)

			resource := &v1alpha1.ConsoleConnection{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.ConsoleConnectionSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
					Endpoint: v1alpha1.ConsoleEndpoint{
						Address:   addr,
						SecretRef: v1alpha1.SecretReference{Name: name + "-console"},
					},
					Timeout: metav1.Duration{Duration: 2 * time.Second},
					Verification: v1alpha1.ConsoleVerification{
						Strategy: v1alpha1.ConsoleVerificationWait,
						Expect:   &v1alpha1.ConsoleExpect{String: new("switch-A")},
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			Eventually(func(g Gomega) {
				cc := &v1alpha1.ConsoleConnection{}
				g.Expect(k8sClient.Get(ctx, key, cc)).To(Succeed())
				g.Expect(cc.Status.LastCheckTime).NotTo(BeNil())
				g.Expect(cc.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition),
					HaveField("Status", metav1.ConditionFalse),
					HaveField("Reason", v1alpha1.ConsoleAliveReason),
				)))
			}).Should(Succeed())
		})

		It("Should report Verified when output matches expected string", func() {
			addr, cleanup := StartTestSSHServer("password", []byte("switch-A login:"))
			DeferCleanup(cleanup)

			resource := &v1alpha1.ConsoleConnection{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.ConsoleConnectionSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
					Endpoint: v1alpha1.ConsoleEndpoint{
						Address:   addr,
						SecretRef: v1alpha1.SecretReference{Name: name + "-console"},
					},
					Verification: v1alpha1.ConsoleVerification{
						Strategy: v1alpha1.ConsoleVerificationWait,
						Expect:   &v1alpha1.ConsoleExpect{String: new("switch-A")},
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			Eventually(func(g Gomega) {
				cc := &v1alpha1.ConsoleConnection{}
				g.Expect(k8sClient.Get(ctx, key, cc)).To(Succeed())
				g.Expect(cc.Status.LastCheckTime).NotTo(BeNil())
				g.Expect(cc.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", v1alpha1.ConsoleVerifiedReason),
				)))
			}).Should(Succeed())
		})

		It("Should report Verified using Device hostname when expect is omitted", func() {
			addr, cleanup := StartTestSSHServer("password", []byte("mydevice>"))
			DeferCleanup(cleanup)

			Eventually(func(g Gomega) {
				device := &v1alpha1.Device{}
				g.Expect(k8sClient.Get(ctx, key, device)).To(Succeed())
				device.Status.Hostname = "mydevice"
				g.Expect(k8sClient.Status().Update(ctx, device)).To(Succeed())
			}).Should(Succeed())

			resource := &v1alpha1.ConsoleConnection{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: v1alpha1.ConsoleConnectionSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: name},
					Endpoint: v1alpha1.ConsoleEndpoint{
						Address:   addr,
						SecretRef: v1alpha1.SecretReference{Name: name + "-console"},
					},
					Verification: v1alpha1.ConsoleVerification{
						Strategy: v1alpha1.ConsoleVerificationWait,
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			Eventually(func(g Gomega) {
				cc := &v1alpha1.ConsoleConnection{}
				g.Expect(k8sClient.Get(ctx, key, cc)).To(Succeed())
				g.Expect(cc.Status.LastCheckTime).NotTo(BeNil())
				g.Expect(cc.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", v1alpha1.ReadyCondition),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", v1alpha1.ConsoleVerifiedReason),
				)))
			}).Should(Succeed())
		})
	})
})

// StartTestSSHServer starts an in-process SSH server on an ephemeral port.
// It accepts password authentication with the given user/pass. After a shell
// request, it writes output to the channel (nil output means write nothing).
// The server accepts one connection at a time and resets for each new one.
// Returns the listener address and a cleanup function.
func StartTestSSHServer(pass string, output []byte) (addr string, cleanup func()) {
	hostKey, err := rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).NotTo(HaveOccurred())
	signer, err := ssh.NewSignerFromKey(hostKey)
	Expect(err).NotTo(HaveOccurred())

	config := &ssh.ServerConfig{
		PasswordCallback: func(_ ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			if string(password) == pass {
				return &ssh.Permissions{}, nil
			}
			return nil, errors.New("invalid credentials")
		},
	}
	config.AddHostKey(signer)

	ln, err := new(net.ListenConfig).Listen(ctx, "tcp", "127.0.0.1:0")
	Expect(err).NotTo(HaveOccurred())

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer GinkgoRecover()
		defer wg.Done()
		for {
			tcpConn, err := ln.Accept()
			if err != nil {
				return // listener closed
			}
			go HandleSSHConn(config, tcpConn, output)
		}
	}()

	return ln.Addr().String(), func() {
		ln.Close()
		wg.Wait()
	}
}

func HandleSSHConn(config *ssh.ServerConfig, tcpConn net.Conn, output []byte) {
	defer GinkgoRecover()
	defer tcpConn.Close()

	sshConn, chans, reqs, err := ssh.NewServerConn(tcpConn, config)
	if err != nil {
		return // auth failure or handshake error
	}
	defer sshConn.Close()
	go ssh.DiscardRequests(reqs)

	for newChan := range chans {
		if newChan.ChannelType() != "session" {
			_ = newChan.Reject(ssh.UnknownChannelType, "unsupported channel type") //nolint:errcheck
			continue
		}
		ch, requests, err := newChan.Accept()
		if err != nil {
			return
		}
		go func() {
			defer ch.Close()
			for req := range requests {
				if req.Type == "shell" {
					_ = req.Reply(true, nil) //nolint:errcheck
					if output != nil {
						ch.Write(output) //nolint:errcheck
					}
					// Hold the channel open until the client disconnects.
					buf := make([]byte, 1)
					for {
						if _, err := ch.Read(buf); err != nil {
							return
						}
					}
				}
				_ = req.Reply(false, nil) //nolint:errcheck
			}
		}()
	}
}
