// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"context"
	"encoding/json"
	"testing"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

type routeMapClientConn struct {
	grpc.ClientConnInterface
	invoke func(context.Context, string, any, any, ...grpc.CallOption) error
}

func (c *routeMapClientConn) Invoke(ctx context.Context, method string, args, reply any, opts ...grpc.CallOption) error {
	return c.invoke(ctx, method, args, reply, opts...)
}

func TestEnsureRoutingPolicyNextHop(t *testing.T) {
	var state []byte
	sets := 0
	conn := &routeMapClientConn{invoke: func(_ context.Context, _ string, args, reply any, _ ...grpc.CallOption) error {
		switch req := args.(type) {
		case *gpb.CapabilityRequest:
			reply.(*gpb.CapabilityResponse).SupportedEncodings = []gpb.Encoding{gpb.Encoding_JSON}
		case *gpb.GetRequest:
			if state == nil {
				return status.Error(codes.NotFound, "not configured")
			}
			reply.(*gpb.GetResponse).Notification = []*gpb.Notification{{Update: []*gpb.Update{{
				Path: req.GetPath()[0], Val: &gpb.TypedValue{Value: &gpb.TypedValue_JsonVal{JsonVal: state}},
			}}}}
		case *gpb.SetRequest:
			if len(req.Replace) != 1 {
				t.Fatalf("Expected one route-map replacement, got %v", req)
			}
			sets++
			state = req.Replace[0].Val.GetJsonVal()
		default:
			t.Fatalf("Unexpected gNMI request %T", req)
		}
		return nil
	}}
	client, err := gnmiext.New(t.Context(), conn)
	if err != nil {
		t.Fatal(err)
	}
	p := &Provider{client: client}
	for step, address := range []string{"10.0.0.1", "10.0.0.2", "2001:db8::1", ""} {
		bgp := &v1alpha1.BgpActions{}
		if address != "" {
			addr := v1alpha1.MustParseAddr(address)
			bgp.SetNextHop = &v1alpha1.SetNextHopAction{IPAddr: addr}
		}
		req := &provider.EnsureRoutingPolicyRequest{Name: "RM-IMPORT", Statements: []provider.PolicyStatement{{
			Sequence: 60, Actions: v1alpha1.PolicyActions{RouteDisposition: v1alpha1.AcceptRoute, BgpActions: bgp},
		}}}
		for range 2 {
			if err := p.EnsureRoutingPolicy(t.Context(), req); err != nil {
				t.Fatal(err)
			}
		}
		if sets != step+1 {
			t.Fatalf("Step %d: got %d Set requests, want %d", step, sets, step+1)
		}
		var rm RouteMap
		if err := json.Unmarshal(state, &rm); err != nil {
			t.Fatal(err)
		}
		entry, ok := rm.EntItems.EntryList.Get(60)
		if !ok || entry.Action != ActionPermit {
			t.Fatalf("Missing permit entry 60 in %s", state)
		}
		if address == "" {
			if len(entry.NhItems.SetNhList) != 0 {
				t.Fatalf("Next-hop configuration was not removed: %s", state)
			}
		} else if nextHop, ok := entry.NhItems.SetNhList.Get(address); !ok || nextHop.Addr != address || len(entry.NhItems.SetNhList) != 1 {
			t.Fatalf("Expected next-hop %s in %s", address, state)
		}
	}
}

func init() {
	e := &RouteMapEntry{}
	e.Order = 10
	e.Action = ActionPermit
	e.SrttItems.Additive = AdminStDisabled
	e.SrttItems.ItemItems.ItemList.Set(&ExtCommItem{Community: "route-target:as2-nn2:65137:107", Scope: RtExtComScopeTransitive})
	e.SregcommItems.Additive = AdminStDisabled
	e.SregcommItems.NoCommAttr = AdminStDisabled
	e.SregcommItems.ItemItems.ItemList.Set(&CommItem{Community: "regular:as2-nn2:65137:107"})
	e.MrtdstItems.RsrtDstAttItems.RsRtDstAttList.Set(&RsRtDstAtt{TDn: "/System/rpm-items/pfxlistv4-items/RuleV4-list[name='PL-CLOUD07']"})

	rm := &RouteMap{}
	rm.Name = "RM-REDIST"
	rm.EntItems.EntryList.Set(e)
	Register("route_map", rm)

	prependEntry := &RouteMapEntry{}
	prependEntry.Order = 10
	prependEntry.Action = ActionPermit
	prependEntry.SetASPathPrependItems.AS = "65000"

	prependRM := &RouteMap{}
	prependRM.Name = "RM-ASPATH-PREPEND"
	prependRM.EntItems.EntryList.Set(prependEntry)
	Register("route_map_aspath_prepend", prependRM)

	lastASEntry := &RouteMapEntry{}
	lastASEntry.Order = 10
	lastASEntry.Action = ActionPermit
	lastASEntry.SetASPathLastASItems.LastAS = 10

	lastASRM := &RouteMap{}
	lastASRM.Name = "RM-ASPATH-LASTAS"
	lastASRM.EntItems.EntryList.Set(lastASEntry)
	Register("route_map_aspath_lastas", lastASRM)

	replaceEntry1 := &RouteMapEntry{}
	replaceEntry1.Order = 10
	replaceEntry1.Action = ActionPermit
	replaceEntry1.SetASPathReplaceItems.MatchPrivateAS = true
	replaceEntry1.SetASPathReplaceItems.ReplaceAsn = "65000"
	replaceEntry1.SetASPathReplaceItems.ReplaceType = "asn"

	replaceEntry2 := &RouteMapEntry{}
	replaceEntry2.Order = 20
	replaceEntry2.Action = ActionPermit
	replaceEntry2.SetASPathReplaceItems.MatchAsnList = "65001"
	replaceEntry2.SetASPathReplaceItems.MatchPrivateAS = false
	replaceEntry2.SetASPathReplaceItems.ReplaceAsn = "65100"
	replaceEntry2.SetASPathReplaceItems.ReplaceType = "asn"

	replaceRM := &RouteMap{}
	replaceRM.Name = "RM-ASPATH-REPLACE"
	replaceRM.EntItems.EntryList.Set(replaceEntry1)
	replaceRM.EntItems.EntryList.Set(replaceEntry2)
	Register("route_map_aspath_replace", replaceRM)

	setEntry := &RouteMapEntry{}
	setEntry.Order = 10
	setEntry.Action = ActionPermit
	setEntry.SetASPathItems.AsnList = "65000"

	setRM := &RouteMap{}
	setRM.Name = "RM-ASPATH-SET"
	setRM.EntItems.EntryList.Set(setEntry)
	Register("route_map_aspath_set", setRM)

	pfxV4Entry := &RouteMapEntry{}
	pfxV4Entry.Order = 10
	pfxV4Entry.Action = ActionPermit
	pfxV4Entry.SetPrefixSet("PL-DEVICE-V4", false)

	pfxV4RM := &RouteMap{}
	pfxV4RM.Name = "RM-PREFIXSET-V4"
	pfxV4RM.EntItems.EntryList.Set(pfxV4Entry)
	Register("route_map_prefixset_v4", pfxV4RM)

	pfxV6Entry := &RouteMapEntry{}
	pfxV6Entry.Order = 10
	pfxV6Entry.Action = ActionPermit
	pfxV6Entry.SetPrefixSet("PL-DEVICE-V6", true)

	pfxV6RM := &RouteMap{}
	pfxV6RM.Name = "RM-PREFIXSET-V6"
	pfxV6RM.EntItems.EntryList.Set(pfxV6Entry)
	Register("route_map_prefixset_v6", pfxV6RM)

	commAnyEntry := &RouteMapEntry{}
	commAnyEntry.Order = 10
	commAnyEntry.Action = ActionPermit
	commAnyEntry.SetCommunitySet("CS-BLUE", v1alpha1.CommunitySetTypeStandard, false)

	commAnyRM := &RouteMap{}
	commAnyRM.Name = "RM-COMMUNITYSET-ANY"
	commAnyRM.EntItems.EntryList.Set(commAnyEntry)
	Register("route_map_communityset_any", commAnyRM)

	commAllEntry := &RouteMapEntry{}
	commAllEntry.Order = 10
	commAllEntry.Action = ActionPermit
	commAllEntry.SetCommunitySet("CS-BLUE", v1alpha1.CommunitySetTypeStandard, true)

	commAllRM := &RouteMap{}
	commAllRM.Name = "RM-COMMUNITYSET-ALL"
	commAllRM.EntItems.EntryList.Set(commAllEntry)
	Register("route_map_communityset_all", commAllRM)

	extCommAnyEntry := &RouteMapEntry{}
	extCommAnyEntry.Order = 10
	extCommAnyEntry.Action = ActionPermit
	extCommAnyEntry.SetCommunitySet("RT-BLUE", v1alpha1.CommunitySetTypeExtended, false)

	extCommAnyRM := &RouteMap{}
	extCommAnyRM.Name = "RM-EXTCOMMUNITYSET-ANY"
	extCommAnyRM.EntItems.EntryList.Set(extCommAnyEntry)
	Register("route_map_extcommunityset_any", extCommAnyRM)

	extCommAllEntry := &RouteMapEntry{}
	extCommAllEntry.Order = 10
	extCommAllEntry.Action = ActionPermit
	extCommAllEntry.SetCommunitySet("RT-BLUE", v1alpha1.CommunitySetTypeExtended, true)

	extCommAllRM := &RouteMap{}
	extCommAllRM.Name = "RM-EXTCOMMUNITYSET-ALL"
	extCommAllRM.EntItems.EntryList.Set(extCommAllEntry)
	Register("route_map_extcommunityset_all", extCommAllRM)

	comboEntry := &RouteMapEntry{}
	comboEntry.Order = 10
	comboEntry.Action = ActionPermit
	comboEntry.SetPrefixSet("PL-DEVICE-V4", false)
	comboEntry.SetCommunitySet("CS-BLUE", v1alpha1.CommunitySetTypeStandard, false)
	comboEntry.SetCommunitySet("RT-BLUE", v1alpha1.CommunitySetTypeExtended, true)

	comboRM := &RouteMap{}
	comboRM.Name = "RM-COMBINED-MATCH"
	comboRM.EntItems.EntryList.Set(comboEntry)
	Register("route_map_combined_match", comboRM)

	commAdditiveEntry := &RouteMapEntry{}
	commAdditiveEntry.Order = 10
	commAdditiveEntry.Action = ActionPermit
	if err := commAdditiveEntry.SetCommunities([]string{"65000:10", "65000:20"}, v1alpha1.CommunityOperationAdd); err != nil {
		panic(err)
	}

	commAdditiveRM := &RouteMap{}
	commAdditiveRM.Name = "RM-COMMUNITY-ADDITIVE"
	commAdditiveRM.EntItems.EntryList.Set(commAdditiveEntry)
	Register("route_map_community_additive", commAdditiveRM)

	extCommAdditiveEntry := &RouteMapEntry{}
	extCommAdditiveEntry.Order = 10
	extCommAdditiveEntry.Action = ActionPermit
	if err := extCommAdditiveEntry.SetExtCommunities([]string{"65000:10", "65000:20"}, v1alpha1.CommunityOperationAdd); err != nil {
		panic(err)
	}

	extCommAdditiveRM := &RouteMap{}
	extCommAdditiveRM.Name = "RM-EXTCOMMUNITY-ADDITIVE"
	extCommAdditiveRM.EntItems.EntryList.Set(extCommAdditiveEntry)
	Register("route_map_extcommunity_additive", extCommAdditiveRM)
}
