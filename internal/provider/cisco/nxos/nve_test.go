// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"testing"
	"time"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/proto"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	nxv1alpha1 "github.com/ironcore-dev/network-operator/api/cisco/nx/v1alpha1"
	corev1alpha1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
	"github.com/ironcore-dev/network-operator/test/gnmi/server"
)

func init() {
	nve := &NVE{
		AdminSt:          AdminStEnabled,
		HostReach:        HostReachBGP,
		AdvertiseVmac:    true,
		SourceInterface:  "lo0",
		AnycastInterface: NewOption("lo1"),
		SuppressARP:      true,
		McastGroupL2:     NewOption("237.0.0.1"),
		McastGroupL3:     NewOption(""),
		HoldDownTime:     300,
	}
	Register("nve", nve)

	vni := &VNI{
		Vni:              100010,
		McastGroup:       NewOption("239.1.1.100"),
		SuppressARP:      suppressARPOff,
		MultisiteIngRepl: MultisiteIngReplDisable,
	}
	Register("vni", vni)

	vniMultisite := &VNI{
		Vni:              100010,
		MultisiteIngRepl: MultisiteIngReplEnable,
	}
	Register("vni_multisite_ingress_replication", vniMultisite)

	vniMultisiteOptimized := &VNI{
		Vni:              100010,
		MultisiteIngRepl: MultisiteIngReplEnableOptimized,
	}
	Register("vni_multisite_ingress_replication_optimized", vniMultisiteOptimized)
	infraVLANs := &NVEInfraVLANs{}
	infraVLANs.InfraVLANList.Set(&NVEInfraVLAN{ID: 4052})
	infraVLANs.InfraVLANList.Set(&NVEInfraVLAN{ID: 4092})
	Register("infra_vlans", infraVLANs)

	ffw := &FabricFwd{
		AdminSt: "enabled",
		Address: "00:00:11:11:22:22",
	}
	Register("fabric_forward", ffw)
}

func TestNVEMultisiteCoexistence(t *testing.T) {
	ctx := t.Context()
	fakeServer, err := server.NewTestServer(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(fakeServer.Close)
	fakeServer.State().SetBuf([]byte("{}"))
	var requests []*gpb.SetRequest
	conn, err := grpc.NewClient(fakeServer.GRPCAddr(),
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})), // #nosec G402 -- in-process test server
		grpc.WithUnaryInterceptor(func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
			if set, ok := req.(*gpb.SetRequest); ok {
				requests = append(requests, proto.Clone(set).(*gpb.SetRequest))
			}
			return invoker(ctx, method, req, reply, cc, opts...)
		}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	gnmi, err := gnmiext.New(ctx, conn)
	if err != nil {
		t.Fatal(err)
	}
	p := &Provider{client: gnmi}
	nveReq := &provider.NVERequest{
		NVE: &corev1alpha1.NetworkVirtualizationEdge{Spec: corev1alpha1.NetworkVirtualizationEdgeSpec{
			AdminState: corev1alpha1.AdminStateUp, HostReachability: corev1alpha1.HostReachabilityTypeBGP,
		}},
		SourceInterface: &corev1alpha1.Interface{Spec: corev1alpha1.InterfaceSpec{Name: "Loopback1"}},
	}
	bgwReq := &BorderGatewaySettingsRequest{
		BorderGateway: &nxv1alpha1.BorderGateway{Spec: nxv1alpha1.BorderGatewaySpec{
			AdminState: corev1alpha1.AdminStateUp, MultisiteID: 1, DelayRestoreTime: metav1.Duration{Duration: 180 * time.Second},
		}},
		SourceInterface: &corev1alpha1.Interface{Spec: corev1alpha1.InterfaceSpec{Name: "Loopback9"}},
	}
	nvePath, err := gnmiext.StringToStructuredPath(new(NVE).XPath())
	if err != nil {
		t.Fatal(err)
	}
	bgwPath, err := gnmiext.StringToStructuredPath(new(MultisiteBorderGatewayInterface).XPath())
	if err != nil {
		t.Fatal(err)
	}
	check := func(multisite bool) {
		t.Helper()
		var got map[string]any
		if err := json.Unmarshal(fakeServer.State().Get(nvePath), &got); err != nil {
			t.Fatal(err)
		}
		if got["sourceInterface"] != "lo1" {
			t.Fatalf("primary source overwritten: %v", got)
		}
		if multisite && got["multisiteBordergwInterface"] != "Loopback9" {
			t.Fatalf("multisite source overwritten: %v", got)
		}
		if !multisite && got["multisiteBordergwInterface"] != nil {
			t.Fatalf("multisite source not removed: %v", got)
		}
	}
	if err := p.EnsureNVE(ctx, nveReq); err != nil {
		t.Fatal(err)
	}
	if err := p.EnsureBorderGatewaySettings(ctx, bgwReq); err != nil {
		t.Fatal(err)
	}
	check(true)
	// Force actual writes on both sides, rather than only testing skipped operations.
	nveReq.NVE.Spec.SuppressARP = true
	if err := p.EnsureNVE(ctx, nveReq); err != nil {
		t.Fatal(err)
	}
	check(true)
	bgwReq.SourceInterface.Spec.Name = "Loopback100"
	if err := p.EnsureBorderGatewaySettings(ctx, bgwReq); err != nil {
		t.Fatal(err)
	}
	bgwReq.SourceInterface.Spec.Name = "Loopback9"
	if err := p.EnsureBorderGatewaySettings(ctx, bgwReq); err != nil {
		t.Fatal(err)
	}
	check(true)
	bgwReq.BorderGateway.Spec.StormControl = []nxv1alpha1.StormControl{{Level: "1", Traffic: nxv1alpha1.TrafficTypeBroadcast}}
	if err := p.EnsureBorderGatewaySettings(ctx, bgwReq); err != nil {
		t.Fatal(err)
	}
	bgwReq.BorderGateway.Spec.StormControl = nil
	if err := p.EnsureBorderGatewaySettings(ctx, bgwReq); err != nil {
		t.Fatal(err)
	}
	stormPath, err := gnmiext.StringToStructuredPath(new(StormControlItems).XPath())
	if err != nil {
		t.Fatal(err)
	}
	if len(fakeServer.State().Get(stormPath)) != 0 {
		t.Fatal("clearing storm control left stale configuration")
	}
	count := len(requests)
	if err := p.EnsureNVE(ctx, nveReq); err != nil {
		t.Fatal(err)
	}
	if err := p.EnsureBorderGatewaySettings(ctx, bgwReq); err != nil {
		t.Fatal(err)
	}
	if len(requests) != count {
		t.Fatalf("unchanged configuration caused Set requests: %v", requests[count:])
	}
	sawParentUpdate, sawLeafReplace := false, false
	for _, req := range requests {
		for _, update := range req.GetUpdate() {
			if proto.Equal(update.GetPath(), nvePath) {
				sawParentUpdate = true
			}
		}
		for _, replace := range req.GetReplace() {
			if proto.Equal(replace.GetPath(), nvePath) {
				t.Fatal("NVE parent was replaced")
			}
			if proto.Equal(replace.GetPath(), bgwPath) {
				sawLeafReplace = true
			}
		}
	}
	if !sawParentUpdate || !sawLeafReplace {
		t.Fatal("expected an NVE parent merge and a multisite leaf replacement")
	}
	count = len(requests)
	if err := p.ResetBorderGatewaySettings(ctx); err != nil {
		t.Fatal(err)
	}
	check(false)
	for _, req := range requests[count:] {
		for _, path := range req.GetDelete() {
			if proto.Equal(path, nvePath) {
				t.Fatal("BorderGateway cleanup deleted the NVE parent")
			}
		}
	}
	count = len(requests)
	if err := p.EnsureNVE(ctx, nveReq); err != nil {
		t.Fatal(err)
	}
	if len(requests) != count {
		t.Fatal("multisite cleanup caused NVE reconciliation to rewrite unchanged configuration")
	}
}

func TestNVEUnsetOptions(t *testing.T) {
	for _, value := range []string{"", "DME_UNSET_PROPERTY_MARKER", "lo9"} {
		t.Run(value, func(t *testing.T) {
			data, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			got := NewOption("old")
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			if value == "lo9" {
				if got.Value == nil || *got.Value != value {
					t.Fatalf("configured option lost: %v", got)
				}
			} else if got.Value != nil {
				t.Fatalf("unset option caused a false diff: %v", got)
			}
		})
	}
}
