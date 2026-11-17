// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package srlinux

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/apistatus"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

var _ provider.RoutingPolicyProvider = (*Provider)(nil)

func (p *Provider) EnsureRoutingPolicy(ctx context.Context, req *provider.EnsureRoutingPolicyRequest) error {
	pol := &RoutingPolicy{Name: req.Name}

	// SR Linux community actions reference a named community-set rather than taking
	// inline values. For each statement with setCommunity/setExtCommunity we synthesize
	// a community-set named deterministically from the policy and statement, so Delete
	// can reconstruct and remove them.
	var synthesized []*CommunitySet

	// SR Linux evaluates statements in the order they are configured, so send them sorted by sequence.
	stmts := slices.SortedFunc(slices.Values(req.Statements), func(a, b provider.PolicyStatement) int {
		return cmp.Compare(a.Sequence, b.Sequence)
	})
	pol.Statement = make([]*RoutingPolicyStatement, 0, len(stmts))
	for _, stmt := range stmts {
		result, err := toSRLPolicyResult(stmt.Actions.RouteDisposition)
		if err != nil {
			return err
		}
		name := strconv.Itoa(int(stmt.Sequence))
		s := &RoutingPolicyStatement{
			Name: name,
			Action: RoutingPolicyAction{
				PolicyResult: result,
			},
		}

		for _, cond := range stmt.Conditions {
			switch c := cond.(type) {
			case provider.MatchPrefixSetCondition:
				s.ensureMatch().Prefix = &RoutingPolicyPrefixMatch{
					PrefixSet:       c.PrefixSet.Spec.Name,
					MatchSetOptions: MatchSetOptionsAny,
				}
			case provider.MatchCommunitySetCondition:
				opts := MatchSetOptionsAny
				if c.MatchAll {
					opts = MatchSetOptionsAll
				}
				match := &RoutingPolicyBGPMatch{}
				if c.CommunitySet.Spec.Type == v1alpha1.CommunitySetTypeExtended {
					match.ExtendedCommunity = &RoutingPolicyExtCommunityMatch{
						ExtendedCommunitySet: c.CommunitySet.Spec.Name,
						MatchSetOptions:      opts,
					}
				} else {
					match.StandardCommunity = &RoutingPolicyStdCommunityMatch{
						StandardCommunitySet: c.CommunitySet.Spec.Name,
						MatchSetOptions:      opts,
					}
				}
				s.ensureMatch().BGP = match
			}
		}

		if stmt.Actions.BgpActions != nil {
			sets, err := applyBGPActions(s, req.Name, stmt.Actions.BgpActions)
			if err != nil {
				return err
			}
			synthesized = append(synthesized, sets...)
		}

		pol.Statement = append(pol.Statement, s)
	}

	// Community-sets synthesized for an earlier version of this policy that the desired one no longer
	// references would be left behind on the device. Remove them once the policy stops using them.
	previous, err := p.synthesizedCommunitySets(ctx, req.Name)
	if err != nil && !errors.Is(err, gnmiext.ErrNil) {
		return err
	}

	// Write the sets, the policy and the orphan deletes in one Set RPC, so the device never keeps
	// the new policy without the cleanup (or the reverse) when part of the change fails.
	sb := new(gnmiext.SetBuilder)
	keep := make(map[string]bool, len(synthesized))
	for _, set := range synthesized {
		sb.Update(set)
		keep[set.Name] = true
	}
	sb.Update(pol)
	for _, set := range previous {
		if !keep[set.Name] {
			sb.Delete(set)
		}
	}
	return p.client.Do(ctx, sb)
}

// synthesizedCommunitySets returns the community-sets that this provider synthesized for the named
// policy, discovered from the policy as it currently exists on the device. It returns [gnmiext.ErrNil]
// if the policy does not exist.
func (p *Provider) synthesizedCommunitySets(ctx context.Context, policyName string) ([]*CommunitySet, error) {
	current := &RoutingPolicy{Name: policyName}
	if err := p.client.GetConfig(ctx, current); err != nil {
		return nil, err
	}
	var sets []*CommunitySet
	seen := map[string]bool{}
	for _, s := range current.Statement {
		if s.Action.BGP == nil || s.Action.BGP.Communities == nil {
			continue
		}
		for _, name := range s.Action.BGP.Communities.referencedSets() {
			if seen[name] || !isSynthesizedCommunitySet(name, policyName) {
				continue
			}
			seen[name] = true
			sets = append(sets, &CommunitySet{Name: name})
		}
	}
	return sets, nil
}

// applyBGPActions realizes the supported BGP actions onto the statement. It returns
// any community-sets synthesized for setCommunity/setExtCommunity so the caller can
// write them alongside the policy.
func applyBGPActions(s *RoutingPolicyStatement, policyName string, a *v1alpha1.BgpActions) ([]*CommunitySet, error) {
	bgp := &RoutingPolicyActionBGP{}
	var synthesized []*CommunitySet

	if a.SetCommunity != nil {
		set := newSynthesizedCommunitySet(policyName, s.Name, a.SetCommunity.Communities)
		bgp.ensureCommunities().set(set.Name, a.SetCommunity.Operation)
		synthesized = append(synthesized, set)
	}
	if a.SetExtCommunity != nil {
		set := newSynthesizedCommunitySet(policyName, s.Name+ExtCommunitySuffix, a.SetExtCommunity.Communities)
		bgp.ensureCommunities().set(set.Name, a.SetExtCommunity.Operation)
		synthesized = append(synthesized, set)
	}
	if a.SetNextHop != nil {
		bgp.NextHop = &RoutingPolicyNextHopAction{Set: a.SetNextHop.Address.String()}
	}
	if a.SetASPath != nil {
		asPath, err := toASPathAction(a.SetASPath)
		if err != nil {
			return nil, err
		}
		bgp.ASPath = asPath
	}
	s.Action.BGP = bgp
	return synthesized, nil
}

// toASPathAction maps the API AS-path action to the SR Linux native as-path container.
// SR Linux supports prepend (as-number + repeat-n) and setting the whole path (as-path replace).
func toASPathAction(a *v1alpha1.SetASPathAction) (*RoutingPolicyASPathAction, error) {
	out := &RoutingPolicyASPathAction{}
	switch {
	case a.Prepend != nil:
		if a.Prepend.UseLastAS != nil {
			return nil, apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{
				Field:       "spec.statements[].actions.bgpActions.setASPath.prepend.useLastAS",
				Description: "useLastAS is not supported by the Nokia SR Linux routing-policy model",
			})
		}
		p := &RoutingPolicyASPathPrepend{RepeatN: 1}
		if a.Prepend.ASNumber != nil {
			asn, err := asnToUint32(*a.Prepend.ASNumber, "spec.statements[].actions.bgpActions.setASPath.prepend.asNumber")
			if err != nil {
				return nil, err
			}
			p.ASNumber = asn
		}
		out.Prepend = p
	case a.Replace != nil:
		// The API replaces matching AS numbers (a given one, or all private ones) inside the path. The SR Linux
		// as-path replace action instead clears the whole path, so mapping one onto the other would change routing
		// silently. Setting the whole path is available as setASPath.asNumber.
		return nil, apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{
			Field:       "spec.statements[].actions.bgpActions.setASPath.replace",
			Description: "replacing AS numbers within the AS path is not supported by the Nokia SR Linux routing-policy model; use setASPath.asNumber to set the whole AS path",
		})
	case a.ASNumber != nil:
		asn, err := asnToUint32(*a.ASNumber, "spec.statements[].actions.bgpActions.setASPath.asNumber")
		if err != nil {
			return nil, err
		}
		out.Replace = []uint32{asn}
	}
	return out, nil
}

// asnToUint32 converts an AS number in plain (1-4294967295) or dotted (high.low, RFC 5396) notation to
// the uint32 that SR Linux uses on the wire. The device returns AS numbers as JSON numbers, so they must
// not be modelled as strings.
func asnToUint32(asn intstr.IntOrString, field string) (uint32, error) {
	invalid := func() error {
		return apistatus.NewInvalidArgumentError(apistatus.FieldViolation{
			Field:       field,
			Description: fmt.Sprintf("invalid AS number %q", asn.String()),
		})
	}
	if asn.Type == intstr.Int {
		if asn.IntVal < 1 {
			return 0, invalid()
		}
		return uint32(asn.IntVal), nil
	}
	high, low, dotted := strings.Cut(asn.StrVal, ".")
	if !dotted {
		v, err := strconv.ParseUint(asn.StrVal, 10, 32)
		if err != nil || v == 0 {
			return 0, invalid()
		}
		return uint32(v), nil
	}
	h, err := strconv.ParseUint(high, 10, 16)
	if err != nil {
		return 0, invalid()
	}
	l, err := strconv.ParseUint(low, 10, 16)
	if err != nil || h|l == 0 {
		return 0, invalid()
	}
	return uint32(h)<<16 | uint32(l), nil
}

func (p *Provider) DeleteRoutingPolicy(ctx context.Context, req *provider.DeleteRoutingPolicyRequest) error {
	// Discover the synthesized community-sets referenced by the policy so they are removed
	// alongside it and not left as orphans.
	sets, err := p.synthesizedCommunitySets(ctx, req.Name)
	if err != nil {
		if errors.Is(err, gnmiext.ErrNil) {
			return nil
		}
		return err
	}
	targets := []gnmiext.DataElement{&RoutingPolicy{Name: req.Name}}
	for _, set := range sets {
		targets = append(targets, set)
	}
	return p.client.Delete(ctx, targets...)
}

// toSRLPolicyResult maps the API route disposition to the SR Linux policy-result. An unknown
// disposition is an error rather than a silent reject, as in the NX-OS provider.
func toSRLPolicyResult(rd v1alpha1.RouteDisposition) (SRLPolicyResult, error) {
	switch rd {
	case v1alpha1.AcceptRoute:
		return SRLPolicyResultAccept, nil
	case v1alpha1.RejectRoute:
		return SRLPolicyResultReject, nil
	default:
		return "", fmt.Errorf("routing policy: unsupported action %q", rd)
	}
}

// SRLPolicyResult represents the SR Linux native routing policy result values.
type SRLPolicyResult string

const (
	SRLPolicyResultAccept        SRLPolicyResult = "accept"
	SRLPolicyResultReject        SRLPolicyResult = "reject"
	SRLPolicyResultNextPolicy    SRLPolicyResult = "next-policy"
	SRLPolicyResultNextStatement SRLPolicyResult = "next-statement"
)

// MatchSetOptions represents the match-set-options for prefix and community matching.
type MatchSetOptions string

const (
	MatchSetOptionsAny MatchSetOptions = "any"
	MatchSetOptionsAll MatchSetOptions = "all"
)

var _ gnmiext.DataElement = (*RoutingPolicy)(nil)

// RoutingPolicy targets a single policy entry in the SR Linux native routing-policy container.
type RoutingPolicy struct {
	// Name is the policy key. It is part of the XPath and not of the payload: SR Linux omits the key leaf
	// of a keyed get, so a `json:"name"` tag would be zeroed on read-back and break comparison and deletion.
	Name string `json:"-"`
	// Statement is a slice, not a [gnmiext.List]: the YANG list is `ordered-by user` and SR Linux evaluates
	// statements in configuration order, so the order is part of the configuration and of the comparison.
	Statement []*RoutingPolicyStatement `json:"statement,omitempty"`
}

func (rp *RoutingPolicy) XPath() string {
	return fmt.Sprintf("routing-policy/policy[name=%s]", rp.Name)
}

// RoutingPolicyStatement represents a single statement in a routing policy.
type RoutingPolicyStatement struct {
	Name   string              `json:"name"`
	Match  *RoutingPolicyMatch `json:"match,omitempty"`
	Action RoutingPolicyAction `json:"action"`
}

func (s *RoutingPolicyStatement) ensureMatch() *RoutingPolicyMatch {
	if s.Match == nil {
		s.Match = &RoutingPolicyMatch{}
	}
	return s.Match
}

// RoutingPolicyMatch holds the match conditions for a statement.
type RoutingPolicyMatch struct {
	Prefix *RoutingPolicyPrefixMatch `json:"prefix,omitempty"`
	BGP    *RoutingPolicyBGPMatch    `json:"bgp,omitempty"`
}

// RoutingPolicyPrefixMatch holds the prefix-set match condition.
type RoutingPolicyPrefixMatch struct {
	PrefixSet       string          `json:"prefix-set"`
	MatchSetOptions MatchSetOptions `json:"match-set-options"`
}

// RoutingPolicyBGPMatch holds BGP-specific match conditions. Standard and extended
// community matches use their dedicated containers, each referencing a named
// standard-community-set or extended-community-set respectively.
type RoutingPolicyBGPMatch struct {
	StandardCommunity *RoutingPolicyStdCommunityMatch `json:"standard-community,omitempty"`
	ExtendedCommunity *RoutingPolicyExtCommunityMatch `json:"extended-community,omitempty"`
}

// RoutingPolicyStdCommunityMatch maps to match/bgp/standard-community. The
// standard-community-set is a reference to a named standard-community-set.
type RoutingPolicyStdCommunityMatch struct {
	StandardCommunitySet string          `json:"standard-community-set"`
	MatchSetOptions      MatchSetOptions `json:"match-set-options"`
}

// RoutingPolicyExtCommunityMatch maps to match/bgp/extended-community. The
// extended-community-set is a reference to a named extended-community-set.
type RoutingPolicyExtCommunityMatch struct {
	ExtendedCommunitySet string          `json:"extended-community-set"`
	MatchSetOptions      MatchSetOptions `json:"match-set-options"`
}

// RoutingPolicyAction holds the action for a statement.
type RoutingPolicyAction struct {
	PolicyResult SRLPolicyResult         `json:"policy-result"`
	BGP          *RoutingPolicyActionBGP `json:"bgp,omitempty"`
}

// RoutingPolicyActionBGP holds BGP-specific actions for a statement.
type RoutingPolicyActionBGP struct {
	ASPath      *RoutingPolicyASPathAction  `json:"as-path,omitempty"`
	Communities *RoutingPolicyCommunities   `json:"communities,omitempty"`
	NextHop     *RoutingPolicyNextHopAction `json:"next-hop,omitempty"`
}

func (b *RoutingPolicyActionBGP) ensureCommunities() *RoutingPolicyCommunities {
	if b.Communities == nil {
		b.Communities = &RoutingPolicyCommunities{}
	}
	return b.Communities
}

// RoutingPolicyCommunities maps to action/bgp/communities. add and replace are each a
// reference to a community-set name; replace clears existing communities first.
type RoutingPolicyCommunities struct {
	Add     string `json:"add,omitempty"`
	Replace string `json:"replace,omitempty"`
}

// set records the community-set reference according to the API operation.
func (c *RoutingPolicyCommunities) set(name string, op v1alpha1.CommunityOperation) {
	if op == v1alpha1.CommunityOperationAdd {
		c.Add = name
		return
	}
	c.Replace = name
}

// referencedSets returns the community-set names referenced by this action.
func (c *RoutingPolicyCommunities) referencedSets() []string {
	var names []string
	if c.Add != "" {
		names = append(names, c.Add)
	}
	if c.Replace != "" {
		names = append(names, c.Replace)
	}
	return names
}

// RoutingPolicyASPathAction maps to action/bgp/as-path. prepend and replace are
// mutually exclusive; replace takes precedence on the device when both are set.
type RoutingPolicyASPathAction struct {
	Prepend *RoutingPolicyASPathPrepend `json:"prepend,omitempty"`
	Replace []uint32                    `json:"replace,omitempty"`
}

// RoutingPolicyASPathPrepend maps to action/bgp/as-path/prepend.
type RoutingPolicyASPathPrepend struct {
	ASNumber uint32 `json:"as-number"`
	RepeatN  int32  `json:"repeat-n,omitempty"`
}

// RoutingPolicyNextHopAction maps to action/bgp/next-hop. Set carries an IP address
// or the literal "self".
type RoutingPolicyNextHopAction struct {
	Set string `json:"set"`
}

// SynthesizedCommunitySetPrefix marks community-sets created by this provider to hold
// inline setCommunity/setExtCommunity values, so they can be identified for cleanup.
const SynthesizedCommunitySetPrefix = "rp-"

// ExtCommunitySuffix distinguishes a statement's extended-community set from its
// standard-community set when both are configured.
const ExtCommunitySuffix = "-ext"

var _ gnmiext.DataElement = (*CommunitySet)(nil)

// CommunitySet targets a single community-set entry in the SR Linux native
// routing-policy container. Members hold standard or extended community values.
type CommunitySet struct {
	// Name is the set key. Like [RoutingPolicy.Name] it is part of the XPath only, since SR Linux omits
	// the key leaf of a keyed get.
	Name   string   `json:"-"`
	Member []string `json:"member,omitempty"`
}

func (c *CommunitySet) XPath() string {
	return fmt.Sprintf("routing-policy/community-set[name=%s]", c.Name)
}

// newSynthesizedCommunitySet builds a community-set named deterministically from the
// policy and statement so Delete can reconstruct and remove it.
func newSynthesizedCommunitySet(policyName, statementName string, members []string) *CommunitySet {
	return &CommunitySet{
		Name:   fmt.Sprintf("%s%s-%s", SynthesizedCommunitySetPrefix, policyName, statementName),
		Member: members,
	}
}

// isSynthesizedCommunitySet reports whether name belongs to a community-set this
// provider synthesized for the given policy.
func isSynthesizedCommunitySet(name, policyName string) bool {
	return strings.HasPrefix(name, SynthesizedCommunitySetPrefix+policyName+"-")
}
