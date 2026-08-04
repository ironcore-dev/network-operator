// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iosxr

import (
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

// RoutePolicy represents an IOS-XR route policy configuration element.
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
	Statement PolicyStatement
	Name      string
}

// NewPolicyString creates a PolicyString from provider-agnostic policy statements.
// Statements are sorted by sequence number and linked in evaluation order.
func NewPolicyString(name string, providerStatements []provider.PolicyStatement) (*PolicyString, error) {
	// Sort statements by sequence number (lowest first)
	statements := make([]provider.PolicyStatement, len(providerStatements))
	copy(statements, providerStatements)
	slices.SortFunc(statements, func(a, b provider.PolicyStatement) int {
		if a.Sequence < b.Sequence {
			return -1
		}
		if a.Sequence > b.Sequence {
			return 1
		}
		return 0
	})

	// Build condition string and actions for each statement
	var iosxrStatements []*Statement
	for _, stmt := range statements {
		conditions, err := NewConditions(stmt.Conditions)
		if err != nil {
			return nil, fmt.Errorf("failed to build condition for statement %d: %w", stmt.Sequence, err)
		}

		actions, err := NewActions(stmt.Actions)
		if err != nil {
			return nil, fmt.Errorf("failed to build actions for statement %d: %w", stmt.Sequence, err)
		}
		iosxrStmt := NewStatement(conditions, actions)
		iosxrStatements = append(iosxrStatements, iosxrStmt)
	}

	// Link statements together (Next/Back pointers)
	for i := range iosxrStatements {
		if i > 0 {
			iosxrStatements[i].previous = iosxrStatements[i-1]
		}
		if i < len(iosxrStatements)-1 {
			iosxrStatements[i].next = iosxrStatements[i+1]
		}
	}

	return &PolicyString{
		Name:      name,
		Statement: iosxrStatements[0],
	}, nil
}

func (p *PolicyString) String() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "route-policy %s\n  ", p.Name)
	sb.WriteString(p.Statement.String())
	sb.WriteString("\nend-policy\n")
	return sb.String()
}

// PolicyStatement represents a single statement in a route policy.
type PolicyStatement interface {
	String() string
	Next() PolicyStatement
	Previous() PolicyStatement
}

// Statement is a policy statement with conditions, actions, and route disposition.
// Statements are linked to form if-elseif-endif chains in IOS-XR RPL syntax.
type Statement struct {
	Action      Actions
	Condition   Conditions
	Disposition string
	next        PolicyStatement
	previous    PolicyStatement
}

// NewStatement creates a Statement with the specified conditions and actions.
func NewStatement(condition Conditions, actions Actions) *Statement {
	return &Statement{
		Condition:   condition,
		Action:      actions,
		Disposition: "done",
	}
}

func (s *Statement) Next() PolicyStatement {
	return s.next
}

func (s *Statement) Previous() PolicyStatement {
	return s.previous
}

func (s *Statement) String() string {
	var sb strings.Builder

	// first element starts if
	if s.previous == nil {
		fmt.Fprintf(&sb, "if %s then\n", s.Condition.String())
	} else {
		fmt.Fprintf(&sb, "elseif %s then\n", s.Condition.String())
	}

	sb.WriteString(s.Action.String())

	// no other element, close action
	if s.next == nil {
		sb.WriteString("endif")
	}

	res := sb.String()
	if s.next != nil {
		res = res + s.next.String()
	}
	return res
}

// PolicyCondition represents a single route matching condition.
type PolicyCondition interface {
	String() string
}

// Conditions represents a set of route matching conditions combined with logical AND.
type Conditions struct {
	Conditions []PolicyCondition
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
	Action() string
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
		sb.WriteString(action.Action())
	}

	// Route disposition: RejectRoute -> "drop", AcceptRoute -> "done".
	// IOS-XR "pass" is not used as it would continue policy evaluation.
	if al.RouteDisposition == v1alpha1.RejectRoute {
		sb.WriteString("    drop\n  ")
	} else {
		sb.WriteString("    done\n  ")
	}
	return sb.String()
}

func NewActions(actions v1alpha1.PolicyActions) (Actions, error) {
	var result Actions

	if actions.BgpActions != nil && actions.BgpActions.SetCommunity != nil {
		result.Actions = append(result.Actions, &ComAction{
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

func (pa *ExtCommAction) Action() string {
	communities := strings.Join(pa.Values, ", ")
	return fmt.Sprintf("set extcommunity rt (%s)\n", communities)
}

// ComAction sets BGP community attributes.
type ComAction struct {
	Values []string
}

func (pa *ComAction) Action() string {
	communities := strings.Join(pa.Values, ", ")
	return fmt.Sprintf("set community (%s)\n", communities)
}

// ASPathAction modifies BGP AS path attributes.
type ASPathAction struct {
	PathAction v1alpha1.SetASPathAction
}

func (pa *ASPathAction) Action() string {
	var sb strings.Builder

	// Handle Prepend action
	if pa.PathAction.Prepend != nil {
		if pa.PathAction.Prepend.ASNumber != nil {
			asNum := formatASNumber(pa.PathAction.Prepend.ASNumber)
			fmt.Fprintf(&sb, "prepend as-path %s\n", asNum)
		} else if pa.PathAction.Prepend.UseLastAS != nil {
			fmt.Fprintf(&sb, "prepend as-path most-recent %d\n", *pa.PathAction.Prepend.UseLastAS)
		}
		return sb.String()
	}

	// Handle Replace action
	if pa.PathAction.Replace != nil {
		replacement := formatASNumber(&pa.PathAction.Replace.Replacement)

		if pa.PathAction.Replace.PrivateAS {
			sb.WriteString("replace as-path private-as\n")
			return sb.String()
		}
		if pa.PathAction.Replace.ASNumber != nil {
			// Replace specific AS number
			// todo implement replacement logic for specific AS number if needed
			// targetAS := formatASNumber(pa.PathAction.Replace.ASNumber)
			fmt.Fprintf(&sb, "replace as-path all '%s'\n", replacement)
			return sb.String()
		}
		fmt.Fprintf(&sb, "replace as-path all %s\n", replacement)
		return sb.String()
	}

	// Handle direct ASNumber set (sets the AS path to a single AS number)
	if pa.PathAction.ASNumber != nil {
		asNum := formatASNumber(pa.PathAction.ASNumber)
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

	return Conditions{Conditions: condList.Conditions}, nil
}
