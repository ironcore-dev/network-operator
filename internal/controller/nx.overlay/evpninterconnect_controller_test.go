// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxoverlay

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	corev1alpha1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	nxoverlayv1alpha1 "github.com/ironcore-dev/network-operator/api/nx.overlay/v1alpha1"
	poolv1alpha1 "github.com/ironcore-dev/network-operator/api/pool/v1alpha1"
)

var _ = Describe("EVPNInterconnect Controller", func() {
	Context("When reconciling a resource", func() {
		var (
			ic      *nxoverlayv1alpha1.EVPNInterconnect
			icKey   client.ObjectKey
			poolKey client.ObjectKey
		)

		BeforeEach(func() {
			By("Creating an IndexPool for Multi-Site ID allocation")
			pool := &poolv1alpha1.IndexPool{
				GenerateName: "multisite-id-pool-",
				Namespace:    metav1.NamespaceDefault,
				Spec: poolv1alpha1.IndexPoolSpec{
					Ranges: []corev1alpha1.IndexRange{{Start: 100, End: 199}},
				},
			}
			Expect(k8sClient.Create(ctx, pool)).To(Succeed())
			poolKey = client.ObjectKeyFromObject(pool)
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, pool))).To(Succeed())
			})

			By("Creating the EVPNInterconnect resource")
			ic = &nxoverlayv1alpha1.EVPNInterconnect{
				GenerateName: "test-ic-",
				Namespace:    metav1.NamespaceDefault,
				Spec: nxoverlayv1alpha1.EVPNInterconnectSpec{
					MultiSiteIDIndexPoolRef: corev1alpha1.TypedLocalObjectReference{
						APIVersion: poolv1alpha1.GroupVersion.String(),
						Kind:       "IndexPool",
						Name:       pool.Name,
					},
					IPAddressPoolRef: corev1alpha1.LocalObjectReference{Name: "unused-ip-pool"},
					Interfaces: []nxoverlayv1alpha1.DCIInterface{
						{
							InterfaceRef:  corev1alpha1.LocalObjectReference{Name: "eth0"},
							Address:       corev1alpha1.MustParsePrefix("10.0.0.0/31"),
							RemoteAddress: corev1alpha1.MustParsePrefix("10.0.0.1/31"),
						},
						{
							InterfaceRef:  corev1alpha1.LocalObjectReference{Name: "eth1"},
							Address:       corev1alpha1.MustParsePrefix("10.0.1.0/31"),
							RemoteAddress: corev1alpha1.MustParsePrefix("10.0.1.1/31"),
						},
					},
					DCI: nxoverlayv1alpha1.DCI{
						Underlay: nxoverlayv1alpha1.Underlay{Protocol: nxoverlayv1alpha1.UnderlayProtocolOSPF},
						Overlay: nxoverlayv1alpha1.Overlay{
							Protocol:                 nxoverlayv1alpha1.OverlayProtocolBGP,
							EVPNInterconnectSelector: metav1.LabelSelector{},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, ic)).To(Succeed())
			icKey = client.ObjectKeyFromObject(ic)
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, ic))).To(Succeed())
				Eventually(func(g Gomega) {
					g.Expect(client.IgnoreNotFound(k8sClient.Get(ctx, icKey, &nxoverlayv1alpha1.EVPNInterconnect{}))).To(Succeed())
				}).Should(Succeed())

				// envtest has no garbage collector; delete Claims before the pool disappears
				Expect(k8sClient.DeleteAllOf(ctx, &poolv1alpha1.Claim{}, client.InNamespace(icKey.Namespace))).To(Succeed())
				Eventually(func(g Gomega) {
					list := &poolv1alpha1.ClaimList{}
					g.Expect(k8sClient.List(ctx, list, client.InNamespace(icKey.Namespace))).To(Succeed())
					g.Expect(list.Items).To(BeEmpty())
				}).Should(Succeed())
			})
		})

		It("adds a finalizer to the EVPNInterconnect", func() {
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, icKey, ic)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(ic, nxoverlayv1alpha1.FinalizerName)).To(BeTrue())
			}).Should(Succeed())
		})

		It("creates a Claim owned by the EVPNInterconnect and allocates a Multi-Site ID from the pool", func() {
			By("Waiting for the EVPNInterconnect to be reconciled")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, icKey, ic)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(ic, nxoverlayv1alpha1.FinalizerName)).To(BeTrue())
			}).Should(Succeed())

			claimName := "multisiteid-" + hash(ic)

			By("Verifying the Claim is created with the correct pool reference and owner")
			claim := &poolv1alpha1.Claim{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: claimName, Namespace: icKey.Namespace}, claim)).To(Succeed())
				g.Expect(claim.Spec.PoolRef).To(Equal(corev1alpha1.TypedLocalObjectReference{
					APIVersion: poolv1alpha1.GroupVersion.String(),
					Kind:       "IndexPool",
					Name:       poolKey.Name,
				}))
				g.Expect(claim.Labels).To(HaveKeyWithValue(nxoverlayv1alpha1.EVPNInterconnectLabel, icKey.Name))
				g.Expect(claim.OwnerReferences).To(ContainElement(SatisfyAll(
					HaveField("Kind", "EVPNInterconnect"),
					HaveField("Name", icKey.Name),
					HaveField("Controller", HaveValue(BeTrue())),
				)))
			}).Should(Succeed())

			By("Verifying the pool controller allocates an Index from the pool range")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(claim), claim)).To(Succeed())
				g.Expect(claim.Status.Value).NotTo(BeEmpty())
				g.Expect(claim.Status.Conditions).To(ContainElement(SatisfyAll(
					HaveField("Type", poolv1alpha1.AllocatedCondition),
					HaveField("Status", metav1.ConditionTrue),
				)))
			}).Should(Succeed())

			Expect(claim.Status.AllocationRef).NotTo(BeNil())
			index := &poolv1alpha1.Index{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: icKey.Namespace, Name: claim.Status.AllocationRef.Name}, index)).To(Succeed())
			Expect(index.Spec.PoolRef.Name).To(Equal(poolKey.Name))
		})
	})
})

var _ = Describe("reconcileInterfaces", func() {
	var (
		r       *EVPNInterconnectReconciler
		created []client.Object
	)

	// newState returns a ReconcileState with all maps initialized, as the real
	// reconcile() does; the function nil-map panics otherwise.
	newState := func() *ReconcileState {
		return &ReconcileState{
			vteps:      make(map[string][]corev1alpha1.NetworkVirtualizationEdge),
			devices:    make(map[string]corev1alpha1.Device),
			interfaces: make(map[string]corev1alpha1.Interface),
		}
	}

	// createDevice creates a minimal valid Device and tracks it for cleanup.
	createDevice := func(name string) {
		device := &corev1alpha1.Device{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: metav1.NamespaceDefault},
			Spec: corev1alpha1.DeviceSpec{
				Endpoint: corev1alpha1.Endpoint{Address: "192.168.10.2:9339"},
				Provider: "test-provider",
			},
		}
		Expect(k8sClient.Create(ctx, device)).To(Succeed())
		created = append(created, device)
	}

	// createSwitchportInterface creates a physical access-switchport interface on
	// the given device and tracks it for cleanup.
	createSwitchportInterface := func(name, deviceName string) {
		intf := &corev1alpha1.Interface{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: metav1.NamespaceDefault},
			Spec: corev1alpha1.InterfaceSpec{
				DeviceRef:  corev1alpha1.LocalObjectReference{Name: deviceName},
				Name:       "Ethernet1/1",
				Type:       corev1alpha1.InterfaceTypePhysical,
				AdminState: corev1alpha1.AdminStateUp,
				Switchport: &corev1alpha1.Switchport{Mode: corev1alpha1.SwitchportModeAccess, AccessVlan: 10},
			},
		}
		Expect(k8sClient.Create(ctx, intf)).To(Succeed())
		created = append(created, intf)
	}

	// createLoopbackInterface creates a non-physical (Loopback) interface and
	// tracks it for cleanup.
	createLoopbackInterface := func(name, deviceName string) {
		intf := &corev1alpha1.Interface{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: metav1.NamespaceDefault},
			Spec: corev1alpha1.InterfaceSpec{
				DeviceRef:  corev1alpha1.LocalObjectReference{Name: deviceName},
				Name:       "Loopback0",
				Type:       corev1alpha1.InterfaceTypeLoopback,
				AdminState: corev1alpha1.AdminStateUp,
			},
		}
		Expect(k8sClient.Create(ctx, intf)).To(Succeed())
		created = append(created, intf)
	}

	// dciInterface builds an in-memory DCIInterface referencing an interface by name.
	dciInterface := func(interfaceName, address string, mtu *int32) nxoverlayv1alpha1.DCIInterface {
		return nxoverlayv1alpha1.DCIInterface{
			InterfaceRef: corev1alpha1.LocalObjectReference{Name: interfaceName},
			Address:      corev1alpha1.MustParsePrefix(address),
			MTU:          mtu,
		}
	}

	// newIC builds an in-memory EVPNInterconnect; it is never persisted because
	// reconcileInterfaces only reads its Namespace, Name and Spec.Interfaces.
	newIC := func(interfaces ...nxoverlayv1alpha1.DCIInterface) *nxoverlayv1alpha1.EVPNInterconnect {
		return &nxoverlayv1alpha1.EVPNInterconnect{
			ObjectMeta: metav1.ObjectMeta{Name: "test-ic", Namespace: metav1.NamespaceDefault},
			Spec:       nxoverlayv1alpha1.EVPNInterconnectSpec{Interfaces: interfaces},
		}
	}

	getInterface := func(name string) *corev1alpha1.Interface {
		intf := &corev1alpha1.Interface{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceDefault, Name: name}, intf)).To(Succeed())
		return intf
	}

	BeforeEach(func() {
		r = &EVPNInterconnectReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		created = nil
	})

	AfterEach(func() {
		for _, obj := range created {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, obj))).To(Succeed())
		}
	})

	It("configures addressing and MTU on the happy path", func() {
		createDevice("dev-a")
		createDevice("dev-b")
		createSwitchportInterface("intf-a", "dev-a")
		createSwitchportInterface("intf-b", "dev-b")

		mtu := int32(9000)
		ic := newIC(
			dciInterface("intf-a", "10.0.0.1/31", &mtu),
			dciInterface("intf-b", "10.0.0.3/31", nil),
		)
		state := newState()

		res, err := r.reconcileInterfaces(ctx, ic, state)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.IsZero()).To(BeTrue())

		intfA := getInterface("intf-a")
		Expect(intfA.Spec.Switchport).To(BeNil())
		Expect(intfA.Spec.IPv4).NotTo(BeNil())
		Expect(intfA.Spec.IPv4.Addresses).To(HaveLen(1))
		Expect(intfA.Spec.IPv4.Addresses[0].String()).To(Equal("10.0.0.1/31"))
		Expect(intfA.Spec.MTU).To(Equal(int32(9000)))

		intfB := getInterface("intf-b")
		Expect(intfB.Spec.Switchport).To(BeNil())
		Expect(intfB.Spec.IPv4).NotTo(BeNil())
		Expect(intfB.Spec.IPv4.Addresses).To(HaveLen(1))
		Expect(intfB.Spec.IPv4.Addresses[0].String()).To(Equal("10.0.0.3/31"))
		Expect(intfB.Spec.MTU).To(Equal(int32(0)), "MTU must be untouched when dci.MTU is nil")

		Expect(state.devices).To(HaveLen(2))
		Expect(state.interfaces).To(HaveLen(2))
	})

	It("is idempotent and does not patch again when nothing changed", func() {
		createDevice("dev-a")
		createDevice("dev-b")
		createSwitchportInterface("intf-a", "dev-a")
		createSwitchportInterface("intf-b", "dev-b")

		ic := newIC(
			dciInterface("intf-a", "10.0.0.1/31", nil),
			dciInterface("intf-b", "10.0.0.3/31", nil),
		)

		_, err := r.reconcileInterfaces(ctx, ic, newState())
		Expect(err).NotTo(HaveOccurred())

		rvA := getInterface("intf-a").ResourceVersion
		rvB := getInterface("intf-b").ResourceVersion

		_, err = r.reconcileInterfaces(ctx, ic, newState())
		Expect(err).NotTo(HaveOccurred())

		Expect(getInterface("intf-a").ResourceVersion).To(Equal(rvA))
		Expect(getInterface("intf-b").ResourceVersion).To(Equal(rvB))
	})

	It("returns a terminal error when both interfaces are on the same device", func() {
		createDevice("dev-a")
		createSwitchportInterface("intf-a", "dev-a")
		createSwitchportInterface("intf-b", "dev-a")

		ic := newIC(
			dciInterface("intf-a", "10.0.0.1/31", nil),
			dciInterface("intf-b", "10.0.0.3/31", nil),
		)

		_, err := r.reconcileInterfaces(ctx, ic, newState())
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("exactly two distinct devices"))
	})

	It("returns a terminal error when an interface is referenced more than once", func() {
		createDevice("dev-a")
		createSwitchportInterface("intf-a", "dev-a")

		ic := newIC(
			dciInterface("intf-a", "10.0.0.1/31", nil),
			dciInterface("intf-a", "10.0.0.3/31", nil),
		)

		_, err := r.reconcileInterfaces(ctx, ic, newState())
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("referenced more than once"))
	})

	It("returns a terminal error when a referenced interface is not Physical", func() {
		createDevice("dev-a")
		createDevice("dev-b")
		createLoopbackInterface("intf-lo", "dev-a")
		createSwitchportInterface("intf-b", "dev-b")

		ic := newIC(
			dciInterface("intf-lo", "10.0.0.1/31", nil),
			dciInterface("intf-b", "10.0.0.3/31", nil),
		)

		_, err := r.reconcileInterfaces(ctx, ic, newState())
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(`has type "Loopback", expected "Physical"`))
	})

	It("waits without error when a referenced interface does not exist", func() {
		createDevice("dev-a")
		createDevice("dev-b")
		createSwitchportInterface("intf-a", "dev-a")

		ic := newIC(
			dciInterface("intf-a", "10.0.0.1/31", nil),
			dciInterface("intf-missing", "10.0.0.3/31", nil),
		)
		state := newState()

		res, err := r.reconcileInterfaces(ctx, ic, state)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.IsZero()).To(BeTrue())

		intfA := getInterface("intf-a")
		Expect(intfA.Spec.Switchport).NotTo(BeNil(), "no interface should be patched when a reference is unresolved")
		Expect(intfA.Spec.IPv4).To(BeNil())
	})
})
