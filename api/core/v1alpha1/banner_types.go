// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// BannerSpec defines the desired state of Banner
type BannerSpec struct {
	// DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="DeviceRef is immutable"
	DeviceRef LocalObjectReference `json:"deviceRef"`

	// ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this interface.
	// This reference is used to link the Banner to its provider-specific configuration.
	// +optional
	ProviderConfigRef *TypedLocalObjectReference `json:"providerConfigRef,omitempty"`

	// Type specifies the banner type to configure, either PreLogin or PostLogin.
	// Immutable.
	// +optional
	// +kubebuilder:default=PreLogin
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Type is immutable"
	Type BannerType `json:"type,omitempty"`

	// Message is the banner message to display.
	// +required
	Message TemplateSource `json:"message"`
}

// BannerType represents the type of banner to configure
// +kubebuilder:validation:Enum=PreLogin;PostLogin
type BannerType string

const (
	// BannerTypePreLogin represents the login banner displayed before user authentication.
	// This corresponds to the openconfig-system login-banner leaf.
	BannerTypePreLogin BannerType = "PreLogin"
	// BannerTypePostLogin represents the message banner displayed after user authentication.
	// This corresponds to the openconfig-system motd-banner leaf.
	BannerTypePostLogin BannerType = "PostLogin"
)

// BannerStatus defines the observed state of Banner.
type BannerStatus struct {
	// The conditions are a list of status objects that describe the state of the Banner.
	//+listType=map
	//+listMapKey=type
	//+patchStrategy=merge
	//+patchMergeKey=type
	//+optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=banners
// +kubebuilder:resource:singular=banner
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.type`
// +kubebuilder:printcolumn:name="Device",type=string,JSONPath=`.spec.deviceRef.name`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Banner is the Schema for the banners API
type Banner struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +required
	Spec BannerSpec `json:"spec,omitempty"`

	// Status of the resource. This is set and updated automatically.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status BannerStatus `json:"status,omitempty,omitzero"`
}

// GetConditions implements conditions.Getter.
func (b *Banner) GetConditions() []metav1.Condition {
	return b.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (b *Banner) SetConditions(conditions []metav1.Condition) {
	b.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// BannerList contains a list of Banner
type BannerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Banner `json:"items"`
}

var (
	BannerDependencies   []schema.GroupVersionKind
	bannerDependenciesMu sync.Mutex
)

func RegisterBannerDependency(gvk schema.GroupVersionKind) {
	bannerDependenciesMu.Lock()
	defer bannerDependenciesMu.Unlock()
	BannerDependencies = append(BannerDependencies, gvk)
}

func init() {
	SchemeBuilder.Register(&Banner{}, &BannerList{})
}
