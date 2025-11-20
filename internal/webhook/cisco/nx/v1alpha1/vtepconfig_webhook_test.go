// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	nxv1alpha1 "github.com/ironcore-dev/network-operator/api/cisco/nx/v1alpha1"
)

var _ = Describe("VTEPConfig Webhook", func() {
	var (
		obj       *nxv1alpha1.VTEPConfig
		oldObj    *nxv1alpha1.VTEPConfig
		validator VTEPConfigCustomValidator
	)

	BeforeEach(func() {
		obj = &nxv1alpha1.VTEPConfig{
			Spec: nxv1alpha1.VTEPConfigSpec{
				InfraVLANs: []nxv1alpha1.VLANListItem{
					{ID: 10},
					{RangeMin: 20, RangeMax: 25},
					{RangeMin: 100, RangeMax: 110},
				},
			},
		}
		oldObj = obj.DeepCopy()
		validator = VTEPConfigCustomValidator{}
	})

	Context("ValidateCreate InfraVLANs", func() {
		It("accepts single VLAN via ID", func() {
			obj.Spec.InfraVLANs = []nxv1alpha1.VLANListItem{{ID: 100}}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("accepts multiple non-overlapping ranges and single IDs", func() {
			obj.Spec.InfraVLANs = []nxv1alpha1.VLANListItem{
				{RangeMin: 1, RangeMax: 10},
				{RangeMin: 20, RangeMax: 30},
				{ID: 40},
				{RangeMin: 50, RangeMax: 60},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("rejects out-of-range low VLAN ID", func() {
			obj.Spec.InfraVLANs = []nxv1alpha1.VLANListItem{{ID: 0}}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("rejects out-of-range high VLAN ID", func() {
			obj.Spec.InfraVLANs = []nxv1alpha1.VLANListItem{{ID: 3968}}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("rejects range with only min", func() {
			obj.Spec.InfraVLANs = []nxv1alpha1.VLANListItem{{RangeMin: 10}}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("rejects range with only max", func() {
			obj.Spec.InfraVLANs = []nxv1alpha1.VLANListItem{{RangeMax: 10}}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("rejects reversed range", func() {
			obj.Spec.InfraVLANs = []nxv1alpha1.VLANListItem{{RangeMin: 20, RangeMax: 10}}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("rejects range with ID present", func() {
			obj.Spec.InfraVLANs = []nxv1alpha1.VLANListItem{{ID: 10, RangeMin: 10, RangeMax: 20}}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("rejects overlapping ranges (shared boundary)", func() {
			obj.Spec.InfraVLANs = []nxv1alpha1.VLANListItem{
				{RangeMin: 10, RangeMax: 20},
				{RangeMin: 20, RangeMax: 30},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("rejects overlapping ID inside a range", func() {
			obj.Spec.InfraVLANs = []nxv1alpha1.VLANListItem{
				{RangeMin: 10, RangeMax: 20},
				{ID: 15},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("rejects total VLAN count > 512", func() {
			// 1-400 plus 401-600 totals 600
			obj.Spec.InfraVLANs = []nxv1alpha1.VLANListItem{
				{RangeMin: 1, RangeMax: 400},
				{RangeMin: 401, RangeMax: 600},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("ValidateUpdate InfraVLANs", func() {
		It("allows unchanged valid config", func() {
			newObj := oldObj.DeepCopy()
			_, err := validator.ValidateUpdate(ctx, oldObj, newObj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("rejects newly introduced overlap", func() {
			newObj := oldObj.DeepCopy()
			newObj.Spec.InfraVLANs = []nxv1alpha1.VLANListItem{
				{RangeMin: 1, RangeMax: 10},
				{RangeMin: 11, RangeMax: 20},
				{RangeMin: 15, RangeMax: 25},
			}
			_, err := validator.ValidateUpdate(ctx, oldObj, newObj)
			Expect(err).To(HaveOccurred())
		})

		It("rejects update adding out-of-range VLAN", func() {
			newObj := oldObj.DeepCopy()
			newObj.Spec.InfraVLANs = append(newObj.Spec.InfraVLANs, nxv1alpha1.VLANListItem{ID: 3968})
			_, err := validator.ValidateUpdate(ctx, oldObj, newObj)
			Expect(err).To(HaveOccurred())
		})

		It("rejects update causing total VLAN count overflow", func() {
			newObj := oldObj.DeepCopy()
			newObj.Spec.InfraVLANs = []nxv1alpha1.VLANListItem{
				{RangeMin: 1, RangeMax: 300},
				{RangeMin: 301, RangeMax: 650},
			}
			_, err := validator.ValidateUpdate(ctx, oldObj, newObj)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("ValidateDelete", func() {
		It("allows delete on VTEPConfig object", func() {
			_, err := validator.ValidateDelete(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("rejects delete when object type is wrong", func() {
			_, err := validator.ValidateDelete(ctx, &nxv1alpha1.VTEPConfigList{})
			Expect(err).To(HaveOccurred())
		})
	})
})

// ...existing code...
