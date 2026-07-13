// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iosxr

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

var _ gnmiext.DataElement = (*RoutePolicy)(nil)

// RoutePolicy represents a route policy configuration element.
type RoutePolicy struct {
	Name string `json:"route-policy-name"`
	Body string `json:"rpl-route-policy"`
}

func (rp *RoutePolicy) XPath() string {
	return "Cisco-IOS-XR-policy-repository-cfg:routing-policy/route-policies/route-policy[route-policy-name=" + rp.Name + "]"
}

// NewEmptyAcceptRoutePolicy creates a pass-through route policy for the specified VRF.
func NewEmptyAcceptRoutePolicy(vrf string) RoutePolicy {
	name := fmt.Sprintf("RPL_%s_IN", vrf)
	return RoutePolicy{
		Name: name,
		Body: fmt.Sprintf("route-policy %s\n  pass\nend-policy\n", name),
	}
}

// PolicyString generates IOS-XR RPL syntax from ordered policy statements.
// Statements are evaluated sequentially until a match is found.
type PolicyString struct {
	Name       string
	Statements []Statement
}

// NewPolicyString creates a PolicyString from provider-agnostic policy statements.
// Statements are sorted by sequence number and evaluated in that order.
func NewPolicyString(name string, Statements []provider.PolicyStatement) (*PolicyString, error) {
	slices.SortFunc(Statements, func(a, b provider.PolicyStatement) int {
		return cmp.Compare(a.Sequence, b.Sequence)
	})

	result := &PolicyString{Name: name}
	for _, stmt := range Statements {
		conditions, err := NewConditions(stmt.Conditions)
		if err != nil {
			return nil, fmt.Errorf("failed to build condition for statement %d: %w", stmt.Sequence, err)
		}

		actions, err := NewActions(stmt.Actions)
		if err != nil {
			return nil, fmt.Errorf("failed to build actions for statement %d: %w", stmt.Sequence, err)
		}
		result.Statements = append(result.Statements, Statement{Conditions: conditions, Actions: actions})
	}

	return result, nil
}

func (p *PolicyString) String() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "route-policy %s\n", p.Name)
	for i := range p.Statements {
		keyword := "if"
		if i > 0 {
			keyword = "elseif"
		}
		fmt.Fprintf(&sb, "  %s %s then\n", keyword, p.Statements[i].Conditions.String())
		sb.WriteString(p.Statements[i].Actions.String())
	}

	// IOSXR drops routes by default if no statement matched
	// Change to accept by default, matching NX-OS/OpenConfig
	sb.WriteString("  else\n")
	sb.WriteString("     pass\n")
	sb.WriteString("  endif\n")
	sb.WriteString("end-policy\n")
	return sb.String()
}

// Statement is a single condition/action pair in a route policy.
type Statement struct {
	Conditions
	Actions
}

// Conditions represents a set of route matching conditions combined with logical AND.
type Conditions struct {
	Conditions []*MatchPrefixCondition
}

func (cl *Conditions) String() string {
	condStrings := make([]string, 0, len(cl.Conditions))
	for _, cond := range cl.Conditions {
		condStrings = append(condStrings, cond.String())
	}
	return strings.Join(condStrings, " and ")
}

// MatchPrefixCondition matches routes against a prefix set.
type MatchPrefixCondition struct {
	PrefixSet *v1alpha1.PrefixSet
}

func (c *MatchPrefixCondition) String() string {
	prefixes := make([]string, 0, len(c.PrefixSet.Spec.Entries))
	for _, entry := range c.PrefixSet.Spec.Entries {
		prefixes = append(prefixes, entry.Prefix.String())
	}
	return fmt.Sprintf("destination in (%s)", strings.Join(prefixes, ", "))
}

// Action represents a single route modification operation.
type Action interface {
	String() string
}

// Actions represents a set of route modification actions and the final route disposition.
type Actions struct {
	Actions          []Action
	RouteDisposition v1alpha1.RouteDisposition
}

func (al *Actions) String() string {
	var sb strings.Builder
	for _, action := range al.Actions {
		sb.WriteString("    ")
		sb.WriteString(action.String())
	}

	// Route disposition: RejectRoute -> "drop", AcceptRoute -> "done".
	// IOS-XR "pass" is not used as it would continue policy evaluation.
	if al.RouteDisposition == v1alpha1.RejectRoute {
		sb.WriteString("    drop\n")
	} else {
		sb.WriteString("    done\n")
	}
	return sb.String()
}

func NewActions(actions v1alpha1.PolicyActions) (Actions, error) {
	var result Actions

	if actions.BgpActions != nil && actions.BgpActions.SetCommunity != nil {
		result.Actions = append(result.Actions, &CommAction{
			Values: actions.BgpActions.SetCommunity.Communities,
		})
	}

	if actions.BgpActions != nil && actions.BgpActions.SetExtCommunity != nil {
		result.Actions = append(result.Actions, &ExtCommAction{
			Values: actions.BgpActions.SetExtCommunity.Communities,
		})
	}

	if actions.BgpActions != nil && actions.BgpActions.SetASPath != nil {
		result.Actions = append(result.Actions, &ASPathAction{
			PathAction: *actions.BgpActions.SetASPath,
		})
	}

	result.RouteDisposition = actions.RouteDisposition

	return result, nil
}

// ExtCommAction sets BGP extended community attributes.
type ExtCommAction struct {
	Values []string
}

func (eca *ExtCommAction) String() string {
	communities := strings.Join(eca.Values, ", ")
	return fmt.Sprintf("set extcommunity rt (%s)\n", communities)
}

// CommAction sets BGP community attributes.
type CommAction struct {
	Values []string
}

func (ca *CommAction) String() string {
	communities := strings.Join(ca.Values, ", ")
	return fmt.Sprintf("set community (%s)\n", communities)
}

// ASPathAction modifies BGP AS path attributes.
type ASPathAction struct {
	PathAction v1alpha1.SetASPathAction
}

func (apa *ASPathAction) String() string {
	var sb strings.Builder

	// Handle Prepend action
	if apa.PathAction.Prepend != nil {
		if apa.PathAction.Prepend.ASNumber != nil {
			asNum := formatASNumber(apa.PathAction.Prepend.ASNumber)
			fmt.Fprintf(&sb, "prepend as-path %s\n", asNum)
		} else if apa.PathAction.Prepend.UseLastAS != nil {
			fmt.Fprintf(&sb, "prepend as-path most-recent %d\n", *apa.PathAction.Prepend.UseLastAS)
		}
		return sb.String()
	}

	// Handle Replace action
	if apa.PathAction.Replace != nil {
		replacement := formatASNumber(&apa.PathAction.Replace.Replacement)

		if apa.PathAction.Replace.PrivateAS {
			sb.WriteString("replace as-path private-as\n")
			return sb.String()
		}
		if apa.PathAction.Replace.ASNumber != nil {
			// Replace specific AS number
			// todo implement replacement logic for specific AS number if needed
			// targetAS := formatASNumber(apa.PathAction.Replace.ASNumber)
			fmt.Fprintf(&sb, "replace as-path all '%s'\n", replacement)
			return sb.String()
		}
		fmt.Fprintf(&sb, "replace as-path all %s\n", replacement)
		return sb.String()
	}

	// Handle direct ASNumber set (sets the AS path to a single AS number)
	if apa.PathAction.ASNumber != nil {
		asNum := formatASNumber(apa.PathAction.ASNumber)
		sb.WriteString("set as-path ")
		sb.WriteString(asNum)
		return sb.String()
	}

	return sb.String()
}

// formatASNumber formats an IntOrString AS number for IOS XR RPL syntax.
// Handles both plain format (integer) and dotted notation (string).
func formatASNumber(asNum *intstr.IntOrString) string {
	if asNum.Type == intstr.Int {
		return strconv.Itoa(asNum.IntValue())
	}
	return asNum.StrVal
}

// NewConditions converts provider-agnostic conditions to IOS-XR conditions.
func NewConditions(conditions []provider.PolicyCondition) (Conditions, error) {
	var condList Conditions

	for _, cond := range conditions {
		switch c := cond.(type) {
		case provider.MatchPrefixSetCondition:
			if len(c.PrefixSet.Spec.Entries) == 0 {
				return Conditions{}, errors.New("prefix set has no entries")
			}
			matchPrefix := &MatchPrefixCondition{
				PrefixSet: c.PrefixSet,
			}
			condList.Conditions = append(condList.Conditions, matchPrefix)
		default:
			return Conditions{}, fmt.Errorf("unsupported condition type: %T", cond)
		}
	}

	return condList, nil
}
