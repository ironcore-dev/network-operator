// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

var _ = Describe("RoutingPolicy Webhook", func() {
	var (
		obj       *v1alpha1.RoutingPolicy
		oldObj    *v1alpha1.RoutingPolicy
		validator RoutingPolicyCustomValidator
	)

	BeforeEach(func() {
		ctx = context.Background()
		obj = &v1alpha1.RoutingPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-routingpolicy",
				Namespace: "default",
			},
			Spec: v1alpha1.RoutingPolicySpec{
				DeviceRef: v1alpha1.LocalObjectReference{
					Name: "test-device",
				},
				Name: "test-policy",
				Statements: []v1alpha1.PolicyStatement{
					{
						Sequence: 1,
						Actions: v1alpha1.PolicyActions{
							RouteDisposition: v1alpha1.AcceptRoute,
						},
					},
				},
			},
		}
		oldObj = obj.DeepCopy()
		validator = RoutingPolicyCustomValidator{}
		Expect(validator).NotTo(BeNil(), "Expected validator to be initialized")
		Expect(oldObj).NotTo(BeNil(), "Expected oldObj to be initialized")
		Expect(obj).NotTo(BeNil(), "Expected obj to be initialized")
	})

	Context("When creating RoutingPolicy under Validating Webhook", func() {
		It("Should admit creation with no BGP actions", func() {
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should admit creation with valid integer AS number in setASPath.asNumber", func() {
			asn := intstr.FromInt32(65001)
			obj.Spec.Statements[0].Actions.BgpActions = &v1alpha1.BgpActions{
				SetASPath: &v1alpha1.SetASPathAction{
					ASNumber: &asn,
				},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should admit creation with valid string AS number in plain format in setASPath.asNumber", func() {
			asn := intstr.FromString("4294967295")
			obj.Spec.Statements[0].Actions.BgpActions = &v1alpha1.BgpActions{
				SetASPath: &v1alpha1.SetASPathAction{
					ASNumber: &asn,
				},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should admit creation with valid dotted notation AS number in setASPath.asNumber", func() {
			asn := intstr.FromString("65000.100")
			obj.Spec.Statements[0].Actions.BgpActions = &v1alpha1.BgpActions{
				SetASPath: &v1alpha1.SetASPathAction{
					ASNumber: &asn,
				},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should deny creation with zero AS number in setASPath.asNumber", func() {
			asn := intstr.FromInt32(0)
			obj.Spec.Statements[0].Actions.BgpActions = &v1alpha1.BgpActions{
				SetASPath: &v1alpha1.SetASPathAction{
					ASNumber: &asn,
				},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("statement[0].actions.bgpActions.setASPath.asNumber"))
		})

		It("Should deny creation with invalid string AS number in setASPath.asNumber", func() {
			asn := intstr.FromString("invalid")
			obj.Spec.Statements[0].Actions.BgpActions = &v1alpha1.BgpActions{
				SetASPath: &v1alpha1.SetASPathAction{
					ASNumber: &asn,
				},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("statement[0].actions.bgpActions.setASPath.asNumber"))
		})

		It("Should admit creation with valid AS number in prepend.asNumber", func() {
			asn := intstr.FromInt32(65001)
			obj.Spec.Statements[0].Actions.BgpActions = &v1alpha1.BgpActions{
				SetASPath: &v1alpha1.SetASPathAction{
					Prepend: &v1alpha1.SetASPathPrepend{
						ASNumber: &asn,
					},
				},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should deny creation with invalid AS number in prepend.asNumber", func() {
			asn := intstr.FromInt32(-1)
			obj.Spec.Statements[0].Actions.BgpActions = &v1alpha1.BgpActions{
				SetASPath: &v1alpha1.SetASPathAction{
					Prepend: &v1alpha1.SetASPathPrepend{
						ASNumber: &asn,
					},
				},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("statement[0].actions.bgpActions.setASPath.prepend.asNumber"))
		})

		It("Should admit creation with valid AS number in replace.asNumber and replacement", func() {
			asn := intstr.FromInt32(65001)
			replacement := intstr.FromInt32(65002)
			obj.Spec.Statements[0].Actions.BgpActions = &v1alpha1.BgpActions{
				SetASPath: &v1alpha1.SetASPathAction{
					Replace: &v1alpha1.SetASPathReplace{
						ASNumber:    &asn,
						Replacement: replacement,
					},
				},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should deny creation with invalid AS number in replace.asNumber", func() {
			asn := intstr.FromString("65536.100")
			replacement := intstr.FromInt32(65002)
			obj.Spec.Statements[0].Actions.BgpActions = &v1alpha1.BgpActions{
				SetASPath: &v1alpha1.SetASPathAction{
					Replace: &v1alpha1.SetASPathReplace{
						ASNumber:    &asn,
						Replacement: replacement,
					},
				},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("statement[0].actions.bgpActions.setASPath.replace.asNumber"))
		})

		It("Should deny creation with invalid AS number in replace.replacement", func() {
			replacement := intstr.FromString("invalid")
			obj.Spec.Statements[0].Actions.BgpActions = &v1alpha1.BgpActions{
				SetASPath: &v1alpha1.SetASPathAction{
					Replace: &v1alpha1.SetASPathReplace{
						PrivateAS:   true,
						Replacement: replacement,
					},
				},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("statement[0].actions.bgpActions.setASPath.replace.replacement"))
		})

		It("Should admit creation with valid dotted notation in replace.replacement", func() {
			replacement := intstr.FromString("65535.65535")
			obj.Spec.Statements[0].Actions.BgpActions = &v1alpha1.BgpActions{
				SetASPath: &v1alpha1.SetASPathAction{
					Replace: &v1alpha1.SetASPathReplace{
						PrivateAS:   true,
						Replacement: replacement,
					},
				},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should validate AS numbers across multiple statements", func() {
			validASN := intstr.FromInt32(65001)
			invalidASN := intstr.FromInt32(0)
			obj.Spec.Statements = []v1alpha1.PolicyStatement{
				{
					Sequence: 1,
					Actions: v1alpha1.PolicyActions{
						RouteDisposition: v1alpha1.AcceptRoute,
						BgpActions: &v1alpha1.BgpActions{
							SetASPath: &v1alpha1.SetASPathAction{
								ASNumber: &validASN,
							},
						},
					},
				},
				{
					Sequence: 2,
					Actions: v1alpha1.PolicyActions{
						RouteDisposition: v1alpha1.AcceptRoute,
						BgpActions: &v1alpha1.BgpActions{
							SetASPath: &v1alpha1.SetASPathAction{
								ASNumber: &invalidASN,
							},
						},
					},
				},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("statement[1]"))
		})

		It("Should admit creation with reject route disposition and no BGP actions", func() {
			obj.Spec.Statements[0].Actions.RouteDisposition = v1alpha1.RejectRoute
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should admit creation when setASPath is nil but bgpActions is set", func() {
			obj.Spec.Statements[0].Actions.BgpActions = &v1alpha1.BgpActions{
				SetCommunity: &v1alpha1.SetCommunityAction{
					Communities: []string{"65000:100"},
				},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should deny creation with AS number exceeding uint32 max in setASPath.asNumber", func() {
			asn := intstr.FromString("4294967296")
			obj.Spec.Statements[0].Actions.BgpActions = &v1alpha1.BgpActions{
				SetASPath: &v1alpha1.SetASPathAction{
					ASNumber: &asn,
				},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("Should deny creation with dotted notation high part out of range in prepend.asNumber", func() {
			asn := intstr.FromString("65536.0")
			obj.Spec.Statements[0].Actions.BgpActions = &v1alpha1.BgpActions{
				SetASPath: &v1alpha1.SetASPathAction{
					Prepend: &v1alpha1.SetASPathPrepend{
						ASNumber: &asn,
					},
				},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("prepend.asNumber"))
		})
	})

	Context("When updating RoutingPolicy under Validating Webhook", func() {
		It("Should admit update with valid AS numbers", func() {
			asn := intstr.FromInt32(65001)
			obj.Spec.Statements[0].Actions.BgpActions = &v1alpha1.BgpActions{
				SetASPath: &v1alpha1.SetASPathAction{
					ASNumber: &asn,
				},
			}
			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should deny update with invalid AS number", func() {
			asn := intstr.FromInt32(0)
			obj.Spec.Statements[0].Actions.BgpActions = &v1alpha1.BgpActions{
				SetASPath: &v1alpha1.SetASPathAction{
					ASNumber: &asn,
				},
			}
			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).To(HaveOccurred())
		})

		It("Should admit update from integer to dotted notation", func() {
			oldASN := intstr.FromInt32(65001)
			oldObj.Spec.Statements = []v1alpha1.PolicyStatement{
				{
					Sequence: 1,
					Actions: v1alpha1.PolicyActions{
						RouteDisposition: v1alpha1.AcceptRoute,
						BgpActions: &v1alpha1.BgpActions{
							SetASPath: &v1alpha1.SetASPathAction{
								ASNumber: &oldASN,
							},
						},
					},
				},
			}
			newASN := intstr.FromString("65000.100")
			obj.Spec.Statements[0].Actions.BgpActions = &v1alpha1.BgpActions{
				SetASPath: &v1alpha1.SetASPathAction{
					ASNumber: &newASN,
				},
			}
			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("When deleting RoutingPolicy under Validating Webhook", func() {
		It("Should admit deletion", func() {
			_, err := validator.ValidateDelete(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
