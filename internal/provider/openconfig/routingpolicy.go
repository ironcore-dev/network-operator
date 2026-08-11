// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package openconfig

import (
	"context"
	"fmt"
	"strconv"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/apistatus"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

var _ provider.RoutingPolicyProvider = (*Provider)(nil)

func (p *Provider) EnsureRoutingPolicy(ctx context.Context, req *provider.EnsureRoutingPolicyRequest) error {
	pd := &PolicyDefinition{
		Name: req.Name,
		Config: &PolicyDefinitionConfig{
			Name: req.Name,
		},
		Statements: &PolicyStatements{},
	}

	for _, stmt := range req.Statements {
		// OC uses string names for statements; convert sequence number.
		name := strconv.Itoa(int(stmt.Sequence))

		s := &PolicyStatementElement{
			Name: name,
			Config: &PolicyStatementConfig{
				Name: name,
			},
			Actions: &PolicyStatementActions{
				Config: &PolicyStatementActionsConfig{
					PolicyResult: toPolicyResult(stmt.Actions.RouteDisposition),
				},
			},
		}

		for _, cond := range stmt.Conditions {
			switch c := cond.(type) {
			case provider.MatchPrefixSetCondition:
				s.Conditions = &PolicyStatementConditions{
					MatchPrefixSet: &PolicyMatchPrefixSet{
						Config: &PolicyMatchPrefixSetConfig{
							PrefixSet:       c.PrefixSet.Spec.Name,
							MatchSetOptions: MatchSetOptionAny,
						},
					},
				}
			}
		}

		// BGP actions (set-local-pref only — community actions not supported on SRLinux OC).
		if stmt.Actions.BgpActions != nil {
			if stmt.Actions.BgpActions.SetCommunity != nil {
				return apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{
					Field:       "spec.statements[].actions.bgpActions.setCommunity",
					Description: "openconfig provider does not support inline community-set on SRLinux",
				})
			}
			if stmt.Actions.BgpActions.SetExtCommunity != nil {
				return apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{
					Field:       "spec.statements[].actions.bgpActions.setExtCommunity",
					Description: "openconfig provider does not support inline ext-community-set on SRLinux",
				})
			}
			if stmt.Actions.BgpActions.SetASPath != nil {
				return apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{
					Field:       "spec.statements[].actions.bgpActions.setASPath",
					Description: "openconfig provider does not support AS-path manipulation on SRLinux",
				})
			}
		}

		pd.Statements.Statement.Set(s)
	}

	return p.client.Update(ctx, pd)
}

func (p *Provider) DeleteRoutingPolicy(ctx context.Context, req *provider.DeleteRoutingPolicyRequest) error {
	pd := &PolicyDefinition{Name: req.Name}
	return p.client.Delete(ctx, pd)
}

func toPolicyResult(rd v1alpha1.RouteDisposition) PolicyResult {
	switch rd {
	case v1alpha1.AcceptRoute:
		return PolicyResultAcceptRoute
	case v1alpha1.RejectRoute:
		return PolicyResultRejectRoute
	default:
		return PolicyResultRejectRoute
	}
}

// PolicyResult represents the OpenConfig routing policy result action.
type PolicyResult string

const (
	PolicyResultAcceptRoute PolicyResult = "ACCEPT_ROUTE"
	PolicyResultRejectRoute PolicyResult = "REJECT_ROUTE"
)

// MatchSetOption represents the match-set-options identity.
type MatchSetOption string

const (
	MatchSetOptionAny MatchSetOption = "ANY"
)

// MaskLengthRangeExact is the mask-length-range value for exact prefix matches.
const MaskLengthRangeExact = "exact"

// Compile-time assertion.
var _ gnmiext.DataElement = (*PolicyDefinition)(nil)

// PolicyDefinition targets a policy-definition entry.
type PolicyDefinition struct {
	Name       string                  `json:"-"`
	Config     *PolicyDefinitionConfig `json:"config,omitempty"`
	Statements *PolicyStatements       `json:"statements,omitempty"`
}

func (pd *PolicyDefinition) XPath() string {
	return fmt.Sprintf("openconfig-routing-policy:routing-policy/policy-definitions/policy-definition[name=%s]", pd.Name)
}

// PolicyDefinitionConfig holds policy-definition/config.
type PolicyDefinitionConfig struct {
	Name string `json:"name"`
}

// PolicyStatements holds the statement list.
type PolicyStatements struct {
	Statement gnmiext.List[string, *PolicyStatementElement] `json:"statement,omitempty"`
}

// PolicyStatementElement represents a single statement.
type PolicyStatementElement struct {
	Name       string                     `json:"name"`
	Config     *PolicyStatementConfig     `json:"config,omitempty"`
	Conditions *PolicyStatementConditions `json:"conditions,omitempty"`
	Actions    *PolicyStatementActions    `json:"actions,omitempty"`
}

func (s *PolicyStatementElement) Key() string { return s.Name }

// PolicyStatementConfig holds statement config.
type PolicyStatementConfig struct {
	Name string `json:"name"`
}

// PolicyStatementConditions holds statement conditions.
type PolicyStatementConditions struct {
	MatchPrefixSet *PolicyMatchPrefixSet `json:"match-prefix-set,omitempty"`
}

// PolicyMatchPrefixSet holds match-prefix-set.
type PolicyMatchPrefixSet struct {
	Config *PolicyMatchPrefixSetConfig `json:"config,omitempty"`
}

// PolicyMatchPrefixSetConfig holds match-prefix-set config.
type PolicyMatchPrefixSetConfig struct {
	PrefixSet       string         `json:"prefix-set"`
	MatchSetOptions MatchSetOption `json:"match-set-options"`
}

// PolicyStatementActions holds statement actions.
type PolicyStatementActions struct {
	Config *PolicyStatementActionsConfig `json:"config,omitempty"`
}

// PolicyStatementActionsConfig holds actions config.
type PolicyStatementActionsConfig struct {
	PolicyResult PolicyResult `json:"policy-result"`
}
