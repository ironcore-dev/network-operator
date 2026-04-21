// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"
	"fmt"
	"math/big"
	"net/netip"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// IPAddressPoolSpec defines the desired state of IPAddressPool
type IPAddressPoolSpec struct {
	// Prefixes defines the CIDR ranges that can be allocated.
	// +required
	// +kubebuilder:validation:MinItems=1
	Prefixes []corev1alpha1.IPPrefix `json:"prefixes"`

	// ReclaimPolicy controls what happens to an allocation when a claim is deleted.
	// Recycle returns the allocation to the pool. Retain keeps it reserved.
	// Immutable.
	// +optional
	// +kubebuilder:default=Recycle
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="reclaimPolicy is immutable"
	ReclaimPolicy ReclaimPolicy `json:"reclaimPolicy,omitempty"`
}

// IPAddressPoolStatus defines the observed state of IPAddressPool.
type IPAddressPoolStatus struct {
	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// Allocated is the number of allocated IP addresses.
	// +optional
	Allocated int64 `json:"allocated"`

	// Total is the number of allocatable IP addresses.
	// +optional
	Total string `json:"total,omitempty"`

	// conditions represent the current state of the IPAddressPool resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=ipaddresspools
// +kubebuilder:resource:singular=ipaddresspool
// +kubebuilder:resource:shortName=ippool
// +kubebuilder:printcolumn:name="Allocated",type=integer,JSONPath=`.status.allocated`
// +kubebuilder:printcolumn:name="Total",type=string,JSONPath=`.status.total`,priority=1
// +kubebuilder:printcolumn:name="Available",type=string,JSONPath=`.status.conditions[?(@.type=="Available")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:validation:XValidation:rule="size(self.metadata.name) <= 43",message="pool name must not exceed 43 characters"

// IPAddressPool is the Schema for the ipaddresspools API
type IPAddressPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +required
	Spec IPAddressPoolSpec `json:"spec"`

	// Status of the resource. This is set and updated automatically.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status IPAddressPoolStatus `json:"status,omitzero"`
}

// Total returns the total number of allocatable IP addresses in the pool.
func (p *IPAddressPool) Total() *big.Int {
	total := new(big.Int)
	for _, prefix := range p.Spec.Prefixes {
		bits := 32
		if prefix.Addr().Is6() {
			bits = 128
		}
		count := new(big.Int).Lsh(big.NewInt(1), uint(bits-prefix.Bits())) // #nosec G115
		total.Add(total, count)
	}
	return total
}

// IsExhausted returns true if all available IP addresses have been allocated.
func (p *IPAddressPool) IsExhausted() bool {
	total := p.Total()
	if total.Sign() == 0 {
		return true
	}
	allocated := big.NewInt(p.Status.Allocated)
	return allocated.Cmp(total) >= 0
}

// GetConditions implements conditions.Getter.
func (p *IPAddressPool) GetConditions() []metav1.Condition {
	return p.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (p *IPAddressPool) SetConditions(conditions []metav1.Condition) {
	p.Status.Conditions = conditions
}

// ReclaimPolicy returns the pool's reclaim policy.
func (p *IPAddressPool) ReclaimPolicy() ReclaimPolicy {
	return p.Spec.ReclaimPolicy
}

// ListAllocations lists all IPAddress objects matching the given options.
func (p *IPAddressPool) ListAllocations(ctx context.Context, c client.Client, opts ...client.ListOption) ([]Allocation, error) {
	list := &IPAddressList{}
	if err := c.List(ctx, list, opts...); err != nil {
		return nil, err
	}
	allocs := make([]Allocation, len(list.Items))
	for i := range list.Items {
		allocs[i] = &list.Items[i]
	}
	return allocs, nil
}

// sanitizeValue replaces characters that are invalid in Kubernetes names.
func sanitizeValue(value string) string {
	return strings.NewReplacer(".", "-", "/", "-", ":", "-").Replace(value)
}

// Allocate finds the first free IP address in the pool's prefixes and returns
// an IPAddress allocation object for the given claim.
func (p *IPAddressPool) Allocate(claim *Claim, existing []Allocation) (Allocation, error) {
	allocated := make(map[netip.Addr]struct{}, len(existing))
	for _, obj := range existing {
		allocated[obj.(*IPAddress).Spec.Address.Addr] = struct{}{}
	}
	for _, prefix := range p.Spec.Prefixes {
		masked := prefix.Masked()
		for addr := masked.Addr(); masked.Contains(addr); addr = addr.Next() {
			if _, taken := allocated[addr]; !taken {
				value := addr.String()
				return &IPAddress{
					TypeMeta: metav1.TypeMeta{
						APIVersion: GroupVersion.String(),
						Kind:       "IPAddress",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("%s-%s", p.Name, sanitizeValue(value)),
						Namespace: p.Namespace,
					},
					Spec: IPAddressSpec{
						PoolRef: corev1alpha1.TypedLocalObjectReference{
							APIVersion: GroupVersion.String(),
							Kind:       "IPAddressPool",
							Name:       p.Name,
						},
						Address: corev1alpha1.IPAddr{Addr: addr},
						ClaimRef: &ClaimRef{
							Name: claim.Name,
							UID:  claim.UID,
						},
					},
				}, nil
			}
		}
	}
	return nil, ErrPoolExhausted
}

// +kubebuilder:object:root=true

// IPAddressPoolList contains a list of IPAddressPool
type IPAddressPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []IPAddressPool `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, &IPAddressPool{}, &IPAddressPoolList{})
		return nil
	})
}
