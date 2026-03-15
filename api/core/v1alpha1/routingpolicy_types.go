// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// RoutingPolicySpec defines the desired state of RoutingPolicy
type RoutingPolicySpec struct {
	// DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="DeviceRef is immutable"
	DeviceRef LocalObjectReference `json:"deviceRef"`

	// ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this interface.
	// This reference is used to link the Banner to its provider-specific configuration.
	// +optional
	ProviderConfigRef *TypedLocalObjectReference `json:"providerConfigRef,omitempty"`

	// Name is the identifier of the RoutingPolicy on the device.
	// Immutable.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Name is immutable"
	Name string `json:"name"`

	// A list of policy statements to apply.
	// +required
	// +listType=map
	// +listMapKey=sequence
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=100
	Statements []PolicyStatement `json:"statements"`
}

type PolicyStatement struct {
	// The sequence number of the policy statement.
	// +required
	// +kubebuilder:validation:Minimum=1
	Sequence int32 `json:"sequence"`

	// Conditions define the match criteria for this statement.
	// If no conditions are specified, the statement matches all routes.
	// +optional
	Conditions *PolicyConditions `json:"conditions,omitempty"`

	// Actions define what to do when conditions match.
	// +required
	Actions PolicyActions `json:"actions"`
}

// PolicyConditions defines the match criteria for a policy statement.
type PolicyConditions struct {
	// MatchPrefixSet matches routes against a PrefixSet resource.
	// +optional
	MatchPrefixSet *PrefixSetMatchCondition `json:"matchPrefixSet,omitempty"`
}

// PrefixSetMatchCondition defines the condition for matching against a PrefixSet.
type PrefixSetMatchCondition struct {
	// PrefixSetRef references a PrefixSet in the same namespace.
	// The PrefixSet must exist and belong to the same device.
	// +required
	PrefixSetRef LocalObjectReference `json:"prefixSetRef"`
}

// PolicyActions defines the actions to take when a policy statement matches.
// +kubebuilder:validation:XValidation:rule="self.routeDisposition == 'AcceptRoute' || !has(self.bgpActions)",message="bgpActions cannot be specified when routeDisposition is RejectRoute"
type PolicyActions struct {
	// RouteDisposition specifies whether to accept or reject the route.
	// +required
	RouteDisposition RouteDisposition `json:"routeDisposition"`

	// BgpActions specifies BGP-specific actions to apply when the route is accepted.
	// Only applicable when RouteDisposition is AcceptRoute.
	// +optional
	BgpActions *BgpActions `json:"bgpActions,omitempty"`
}

// RouteDisposition defines the final disposition of a route.
// +kubebuilder:validation:Enum=AcceptRoute;RejectRoute
type RouteDisposition string

const (
	// AcceptRoute permits the route and applies any configured actions.
	AcceptRoute RouteDisposition = "AcceptRoute"
	// RejectRoute denies the route immediately.
	RejectRoute RouteDisposition = "RejectRoute"
)

// BgpActions defines BGP-specific actions for a policy statement.
// +kubebuilder:validation:XValidation:rule="has(self.setCommunity) || has(self.setExtCommunity) || has(self.setASPath)",message="at least one BGP action must be specified"
type BgpActions struct {
	// SetCommunity configures BGP standard community attributes.
	// +optional
	SetCommunity *SetCommunityAction `json:"setCommunity,omitempty"`

	// SetExtCommunity configures BGP extended community attributes.
	// +optional
	SetExtCommunity *SetExtCommunityAction `json:"setExtCommunity,omitempty"`

	// SetASPath configures modifications to the BGP AS path attribute.
	// +optional
	SetASPath *SetASPathAction `json:"setASPath,omitempty"`
}

// SetASPathAction defines actions to modify the BGP AS path attribute.
// +kubebuilder:validation:XValidation:rule="has(self.prepend) || has(self.replace) || has(self.asNumber)",message="at least one AS path action must be specified"
type SetASPathAction struct {
	// Prepend configures prepending to the AS path.
	// +optional
	Prepend *SetASPathPrepend `json:"prepend,omitempty"`

	// Replace configures replacement of AS numbers in the AS path.
	// +optional
	Replace *SetASPathReplace `json:"replace,omitempty"`

	// ASNumber sets the AS path to the specified AS number.
	// Supports both plain format (1-4294967295) and dotted notation (1-65535.0-65535) as per RFC 5396.
	// +optional
	ASNumber *intstr.IntOrString `json:"asNumber,omitempty"`
}

// SetASPathPrepend configures prepending to the BGP AS path.
// Either asNumber or useLastAS must be specified, but not both.
// +kubebuilder:validation:XValidation:rule="has(self.asNumber) != has(self.useLastAS)",message="exactly one of asNumber or useLastAS must be specified"
type SetASPathPrepend struct {
	// ASNumber is the autonomous system number to prepend to the AS path.
	// Supports both plain format (1-4294967295) and dotted notation (1-65535.0-65535) as per RFC 5396.
	// +optional
	ASNumber *intstr.IntOrString `json:"asNumber,omitempty"`

	// UseLastAS prepends the last AS number in the existing AS path the specified number of times.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=10
	UseLastAS *int32 `json:"useLastAS,omitempty"`
}

// SetASPathReplace configures replacement of AS numbers in the BGP AS path.
// Either privateAS or asNumber must be specified, but not both.
// +kubebuilder:validation:XValidation:rule="has(self.privateAS) != has(self.asNumber)",message="exactly one of privateAS or asNumber must be specified"
type SetASPathReplace struct {
	// PrivateAS, when set to true, targets all private AS numbers in the path for replacement.
	// +optional
	PrivateAS bool `json:"privateAS,omitempty"`

	// ASNumber targets a specific AS number in the path for replacement.
	// Supports both plain format (1-4294967295) and dotted notation (1-65535.0-65535) as per RFC 5396.
	// +optional
	ASNumber *intstr.IntOrString `json:"asNumber,omitempty"`

	// Replacement is the AS number to substitute in place of matched AS numbers.
	// Supports both plain format (1-4294967295) and dotted notation (1-65535.0-65535) as per RFC 5396.
	// +required
	Replacement intstr.IntOrString `json:"replacement"`
}

// SetCommunityAction defines the action to set BGP standard communities.
type SetCommunityAction struct {
	// Communities is the list of BGP standard communities to set.
	// The communities must be in the format defined by [RFC 1997].
	// [RFC 1997]: https://datatracker.ietf.org/doc/html/rfc1997
	// +required
	// +kubebuilder:validation:MinItems=1
	Communities []string `json:"communities"`
}

// SetExtCommunityAction defines the action to set BGP extended communities.
type SetExtCommunityAction struct {
	// Communities is the list of BGP extended communities to set.
	// The communities must be in the format defined by [RFC 4360].
	// [RFC 4360]: https://datatracker.ietf.org/doc/html/rfc4360
	// +required
	// +kubebuilder:validation:MinItems=1
	Communities []string `json:"communities"`
}

// RoutingPolicyStatus defines the observed state of RoutingPolicy.
type RoutingPolicyStatus struct {
	// StatementsSummary provides a human-readable summary of the number of policy statements.
	// +optional
	StatementsSummary string `json:"statementsSummary,omitempty"`

	// The conditions are a list of status objects that describe the state of the RoutingPolicy.
	// +listType=map
	// +listMapKey=type
	// +patchStrategy=merge
	// +patchMergeKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=routingpolicies
// +kubebuilder:resource:singular=routingpolicy
// +kubebuilder:resource:shortName=routemap
// +kubebuilder:printcolumn:name="Routing Policy",type=string,JSONPath=`.spec.name`
// +kubebuilder:printcolumn:name="Device",type=string,JSONPath=`.spec.deviceRef.name`
// +kubebuilder:printcolumn:name="Statements",type=string,JSONPath=`.status.statementsSummary`,priority=1
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Paused",type=string,JSONPath=`.status.conditions[?(@.type=="Paused")].status`,priority=1
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// RoutingPolicy is the Schema for the routingpolicies API
type RoutingPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +required
	Spec RoutingPolicySpec `json:"spec,omitempty"`

	// Status of the resource. This is set and updated automatically.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status RoutingPolicyStatus `json:"status,omitempty,omitzero"`
}

// GetConditions implements conditions.Getter.
func (p *RoutingPolicy) GetConditions() []metav1.Condition {
	return p.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (p *RoutingPolicy) SetConditions(conditions []metav1.Condition) {
	p.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// RoutingPolicyList contains a list of RoutingPolicy
type RoutingPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []RoutingPolicy `json:"items"`
}

var (
	RoutingPolicyDependencies   []schema.GroupVersionKind
	routingPolicyDependenciesMu sync.Mutex
)

func RegisterRoutingPolicyDependency(gvk schema.GroupVersionKind) {
	routingPolicyDependenciesMu.Lock()
	defer routingPolicyDependenciesMu.Unlock()
	RoutingPolicyDependencies = append(RoutingPolicyDependencies, gvk)
}

func init() {
	SchemeBuilder.Register(&RoutingPolicy{}, &RoutingPolicyList{})
}
