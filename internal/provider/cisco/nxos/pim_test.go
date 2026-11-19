// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"context"
	"crypto/tls"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
	testserver "github.com/ironcore-dev/network-operator/test/gnmi/server"
)

func init() {
	apItems := new(AnycastPeerItems)
	apItems.AcastRPPeerList.Set(&AnycastPeerAddr{Addr: "10.0.0.100/32", RpSetAddr: "10.0.0.2/32"})
	Register("pim_apr", apItems)

	ifItems := new(PIMIfItems)
	ifItems.IfList.Set(&PIMIf{ID: "eth1/1", PimSparseMode: true})
	Register("pim_intf", ifItems)

	rp := &StaticRP{Addr: "10.0.0.100/32"}
	rp.RpgrplistItems.RPGrpListList.Set(&StaticRPGrp{GrpListName: "224.0.0.0/4"})
	Register("pim_rp", rp)

	Register("pim_dom_log", &PIMDom{
		Name:       "default",
		AdminSt:    AdminStEnabled,
		LogNbhChng: true,
	})
}

func TestEnsurePIMAnycastRPIdempotency(t *testing.T) {
	for _, test := range []struct {
		name    string
		local   string
		peers   []string
		members []string
	}{
		{name: "local and remote", local: "10.0.0.1", peers: []string{"10.0.0.2"}, members: []string{"10.0.0.1", "10.0.0.2"}},
		{name: "single RP", local: "10.0.0.1", members: []string{"10.0.0.1"}},
		{name: "legacy members", peers: []string{"10.0.0.1", "10.0.0.2"}, members: []string{"10.0.0.1", "10.0.0.2"}},
		{name: "duplicate members", local: "10.0.0.1", peers: []string{"10.0.0.2", "10.0.0.1", "10.0.0.2"}, members: []string{"10.0.0.1", "10.0.0.2"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			server, err := testserver.NewTestServer(ctx)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(server.Close)
			server.State().SetBuf([]byte(`{"System":{"intf-items":{"lb-items":{"LbRtdIf-list":[{"id":"lo0"}]}}}}`))
			var setCalls atomic.Int64
			conn, err := grpc.NewClient(server.GRPCAddr(),
				grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})), //nolint:gosec // The test server uses a temporary self-signed certificate.
				grpc.WithUnaryInterceptor(func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
					if method == "/gnmi.gNMI/Set" {
						setCalls.Add(1)
					}
					return invoker(ctx, method, req, reply, cc, opts...)
				}),
			)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = conn.Close() })
			gnmiClient, err := gnmiext.New(ctx, conn)
			if err != nil {
				t.Fatal(err)
			}
			p := &Provider{client: gnmiClient}
			const rpAddr = "10.0.0.100"
			members := test.members
			req := &provider.EnsurePIMRequest{
				PIM: &v1alpha1.PIM{Spec: v1alpha1.PIMSpec{
					RendezvousPoints: []v1alpha1.RendezvousPoint{{
						Address: rpAddr, MulticastGroups: []v1alpha1.IPPrefix{v1alpha1.MustParsePrefix("224.0.0.0/4")}, AnycastAddresses: members,
					}},
				}},
				Interfaces: []provider.PIMInterface{{
					Interface: &v1alpha1.Interface{Spec: v1alpha1.InterfaceSpec{Name: "lo0"}}, Mode: v1alpha1.PIMModeSparse,
				}},
			}
			req.RendezvousPoints = []provider.PIMRendezvousPoint{{
				RendezvousPoint: &req.PIM.Spec.RendezvousPoints[0], PeerAddresses: test.peers,
			}}
			if test.local != "" {
				req.RendezvousPoints[0].LocalInterface = &v1alpha1.Interface{Spec: v1alpha1.InterfaceSpec{
					IPv4: &v1alpha1.InterfaceIPv4{Addresses: []v1alpha1.IPPrefix{v1alpha1.MustParsePrefix(test.local + "/32")}},
				}}
			}
			if err := p.EnsurePIM(ctx, req); err != nil {
				t.Fatal(err)
			}
			configured := new(AnycastPeerItems)
			if err := gnmiClient.GetConfig(ctx, configured); err != nil {
				t.Fatal(err)
			}
			if len(configured.AcastRPPeerList) != len(members) {
				t.Fatalf("expected complete RP membership, got %+v", configured.AcastRPPeerList)
			}
			for _, member := range members {
				if _, ok := configured.AcastRPPeerList.Get(AnycastPeerAddr{Addr: rpAddr + "/32", RpSetAddr: member + "/32"}); !ok {
					t.Fatalf("missing RP member %s", member)
				}
			}
			initialSets := setCalls.Load()
			if initialSets == 0 {
				t.Fatal("initial PIM configuration produced no Set requests")
			}
			if err := p.EnsurePIM(ctx, req); err != nil {
				t.Fatal(err)
			}
			if got := setCalls.Load() - initialSets; got != 0 {
				t.Fatalf("unchanged PIM configuration produced %d additional Set requests", got)
			}
		})
	}
}
