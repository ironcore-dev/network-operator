// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"cmp"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"fmt"
	"math"
	"net/netip"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	kerrors "k8s.io/apimachinery/pkg/util/errors"

	nxv1alpha1 "github.com/ironcore-dev/network-operator/api/cisco/nx/v1alpha1"
	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/deviceutil"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"
)

var (
	_ provider.Provider                 = (*Provider)(nil)
	_ provider.DeviceProvider           = (*Provider)(nil)
	_ provider.ProvisioningProvider     = (*Provider)(nil)
	_ provider.ACLProvider              = (*Provider)(nil)
	_ provider.BannerProvider           = (*Provider)(nil)
	_ provider.BGPProvider              = (*Provider)(nil)
	_ provider.BGPPeerProvider          = (*Provider)(nil)
	_ provider.CertificateProvider      = (*Provider)(nil)
	_ provider.DNSProvider              = (*Provider)(nil)
	_ provider.EVPNInstanceProvider     = (*Provider)(nil)
	_ provider.InterfaceProvider        = (*Provider)(nil)
	_ provider.ISISProvider             = (*Provider)(nil)
	_ provider.ManagementAccessProvider = (*Provider)(nil)
	_ provider.NTPProvider              = (*Provider)(nil)
	_ provider.OSPFProvider             = (*Provider)(nil)
	_ provider.PIMProvider              = (*Provider)(nil)
	_ provider.SNMPProvider             = (*Provider)(nil)
	_ provider.PrefixSetProvider        = (*Provider)(nil)
	_ provider.RoutingPolicyProvider    = (*Provider)(nil)
	_ provider.SyslogProvider           = (*Provider)(nil)
	_ provider.UserProvider             = (*Provider)(nil)
	_ provider.VLANProvider             = (*Provider)(nil)
	_ provider.VRFProvider              = (*Provider)(nil)
	_ provider.NVEProvider              = (*Provider)(nil)
)

type Provider struct {
	conn   *grpc.ClientConn
	client gnmiext.Client
}

func NewProvider() provider.Provider {
	return &Provider{}
}

func (p *Provider) Connect(ctx context.Context, conn *deviceutil.Connection) (err error) {
	p.conn, err = deviceutil.NewGrpcClient(ctx, conn, deviceutil.WithDefaultTimeout(30*time.Second))
	if err != nil {
		return fmt.Errorf("failed to create grpc connection: %w", err)
	}
	var opts []gnmiext.Option
	if logger, err := logr.FromContext(ctx); err == nil && !logger.IsZero() {
		opts = append(opts, gnmiext.WithLogger(logger))
	}
	p.client, err = gnmiext.New(ctx, p.conn, opts...)
	return err
}

func (p *Provider) Disconnect(_ context.Context, _ *deviceutil.Connection) error {
	return p.conn.Close()
}

func (p *Provider) HashProvisioningPassword(password string) (hashed, encryptType string, err error) {
	s := [10]byte{}
	if _, err := rand.Read(s[:]); err != nil {
		return "", "", err
	}
	e := Scrypt{Salt: s}
	hashed, pwdEncryptType, err := e.Encode(password)
	if err != nil {
		return "", "", err
	}
	return hashed, string(pwdEncryptType), nil
}

func (p *Provider) VerifyProvisioned(ctx context.Context, conn *deviceutil.Connection, device *v1alpha1.Device) bool {
	if err := p.Connect(ctx, conn); err != nil {
		return false
	}
	p.Disconnect(ctx, conn) //nolint:errcheck
	return true
}

func (p *Provider) Reboot(ctx context.Context, conn *deviceutil.Connection) error {
	return Reboot(ctx, p.conn)
}

func (p *Provider) FactoryReset(ctx context.Context, conn *deviceutil.Connection) error {
	return ResetToFactoryDefaults(ctx, p.conn)
}

func (p *Provider) Reprovision(ctx context.Context, conn *deviceutil.Connection) (reterr error) {
	if err := p.Connect(ctx, conn); err != nil {
		return err
	}
	defer func() {
		if err := p.Disconnect(ctx, conn); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()
	// This is currently defunct on NX-OS, as enabling POAP requires a `copy running-config startup-config` which we
	// cannot issue via GNMI
	// TODO add once NXAPI client is available
	poap := BootPOAP("enable")
	if err := p.client.Update(ctx, &poap); err != nil {
		return err
	}
	return Reboot(ctx, p.conn)
}

func (p *Provider) ListPorts(ctx context.Context) ([]provider.DevicePort, error) {
	ports := new(Ports)
	if err := p.client.GetState(ctx, ports); err != nil {
		return nil, err
	}

	//nolint:errcheck
	slices.SortFunc(ports.PhysIfList, func(i, j *Port) int {
		a, _ := strconv.Atoi(strings.SplitN(i.ID, "/", 2)[1])
		b, _ := strconv.Atoi(strings.SplitN(j.ID, "/", 2)[1])
		return cmp.Compare(a, b)
	})

	dp := make([]provider.DevicePort, len(ports.PhysIfList))
	for i, p := range ports.PhysIfList {
		var speeds []int32
		for s := range strings.SplitSeq(p.PhysItems.PortcapItems.Speed, ",") {
			if n, err := strconv.ParseInt(s, 10, 32); err == nil {
				if gbps := int32(n) / 1000; gbps > 0 && !slices.Contains(speeds, gbps) {
					speeds = append(speeds, gbps)
				}
			}
		}
		dp[i] = provider.DevicePort{
			ID:                  p.ID,
			Type:                p.PhysItems.PortcapItems.Type.String(),
			SupportedSpeedsGbps: speeds,
			Transceiver:         p.PhysItems.FcotItems.Description,
		}
	}

	return dp, nil
}

func (p *Provider) GetDeviceInfo(ctx context.Context) (*provider.DeviceInfo, error) {
	m := new(Model)
	s := new(SerialNumber)
	fw := new(FirmwareVersion)
	if err := p.client.GetState(ctx, m, s, fw); err != nil {
		return nil, err
	}

	return &provider.DeviceInfo{
		Manufacturer:    Manufacturer,
		Model:           string(*m),
		SerialNumber:    string(*s),
		FirmwareVersion: string(*fw),
	}, nil
}

func (p *Provider) EnsureACL(ctx context.Context, req *provider.EnsureACLRequest) error {
	a := new(ACL)
	a.Name = req.ACL.Spec.Name
	for i, entry := range req.ACL.Spec.Entries {
		action, err := ActionFrom(entry.Action)
		if err != nil {
			return err
		}
		if i > 0 && entry.SourceAddress.Addr().Is6() != a.Is6 {
			return errors.New("acl: rule contains both ipv4 and ipv6 rules")
		}
		a.Is6 = entry.SourceAddress.Addr().Is6()
		if entry.SourceAddress.Addr().Is4() != entry.DestinationAddress.Addr().Is4() {
			return errors.New("acl: rule contains mismatched ip versions in source and destination addresses")
		}
		a.SeqItems.ACEList.Set(&ACLEntry{
			SeqNum:          entry.Sequence,
			Action:          action,
			Protocol:        ProtocolFrom(entry.Protocol),
			SrcPrefix:       entry.SourceAddress.Addr().String(),
			SrcPrefixLength: entry.SourceAddress.Bits(),
			DstPrefix:       entry.DestinationAddress.Addr().String(),
			DstPrefixLength: entry.DestinationAddress.Bits(),
		})
	}

	return p.Update(ctx, a)
}

func (p *Provider) DeleteACL(ctx context.Context, req *provider.DeleteACLRequest) error {
	a := new(ACL)
	a.Name = req.Name
	// Check if the ACL is IPv4 by trying to fetch its config. If it does not exist, assume it's IPv6.
	// As the names are unique across both types, this will ensure we delete the correct one.
	if err := p.client.GetConfig(ctx, a); errors.Is(err, gnmiext.ErrNil) {
		a.Is6 = true
	}
	return p.client.Delete(ctx, a)
}

func (p *Provider) EnsureBanner(ctx context.Context, req *provider.EnsureBannerRequest) (reterr error) {
	// See: https://www.cisco.com/c/en/us/td/docs/dcn/nx-os/nexus9000/104x/configuration/fundamentals/cisco-nexus-9000-series-nx-os-fundamentals-configuration-guide-release-104x/m-basic-device-management.html#task_1174841
	lines := strings.Split(req.Message, "\n")
	if len(lines) > 40 {
		return errors.New("banner: maximum of 40 lines allowed")
	}
	for i, line := range lines {
		if n := utf8.RuneCountInString(line); n > 255 {
			return fmt.Errorf("banner: line %d exceeds 255 characters (%d)", i+1, n)
		}
	}

	t, err := BannerTypeFrom(req.Type)
	if err != nil {
		return err
	}

	b := new(Banner)
	b.Delimiter = "^"
	b.Message = req.Message
	b.Type = t

	return p.Patch(ctx, b)
}

func (p *Provider) DeleteBanner(ctx context.Context, req *provider.DeleteBannerRequest) error {
	t, err := BannerTypeFrom(req.Type)
	if err != nil {
		return err
	}

	b := new(Banner)
	b.Type = t
	return p.client.Delete(ctx, b)
}

func (p *Provider) EnsureBGP(ctx context.Context, req *provider.EnsureBGPRequest) (reterr error) {
	f := new(Feature)
	f.Name = "bgp"
	f.AdminSt = AdminStEnabled

	f2 := new(Feature)
	f2.Name = "evpn"
	f2.AdminSt = AdminStEnabled

	b := new(BGP)
	b.AdminSt = AdminStEnabled
	if req.BGP.Spec.AdminState == v1alpha1.AdminStateDown {
		b.AdminSt = AdminStDisabled
	}
	b.Asn = req.BGP.Spec.ASNumber.String()

	var asf AsFormat
	if err := p.client.GetConfig(ctx, &asf); err != nil && !errors.Is(err, gnmiext.ErrNil) {
		return err
	}

	var err error
	switch {
	case asf == "" && strings.Contains(b.Asn, "."):
		asf = AsFormatAsDot
		err = p.Update(ctx, &asf)
	case asf != "" && !strings.Contains(b.Asn, "."):
		err = p.client.Delete(ctx, &asf)
	}
	if err != nil {
		return err
	}

	dom := new(BGPDom)
	dom.Name = DefaultVRFName
	dom.RtrID = req.BGP.Spec.RouterID
	dom.RtrIDAuto = AdminStDisabled

	if req.BGP.Spec.AddressFamilies != nil {
		if af := req.BGP.Spec.AddressFamilies.Ipv4Unicast; af != nil && af.Enabled {
			item := new(BGPDomAfItem)
			item.Type = AddressFamilyIPv4Unicast
			if err := item.SetMultipath(af.Multipath); err != nil {
				return err
			}
			dom.AfItems.DomAfList.Set(item)
		}

		if af := req.BGP.Spec.AddressFamilies.Ipv6Unicast; af != nil && af.Enabled {
			item := new(BGPDomAfItem)
			item.Type = AddressFamilyIPv6Unicast
			if err := item.SetMultipath(af.Multipath); err != nil {
				return err
			}
			dom.AfItems.DomAfList.Set(item)
		}

		if af := req.BGP.Spec.AddressFamilies.L2vpnEvpn; af != nil && af.Enabled {
			item := new(BGPDomAfItem)
			item.Type = AddressFamilyL2EVPN
			if err := item.SetMultipath(af.Multipath); err != nil {
				return err
			}
			item.RetainRttAll = AdminStDisabled
			if af.RouteTargetPolicy != nil && af.RouteTargetPolicy.RetainAll {
				item.RetainRttAll = AdminStEnabled
			}
			dom.AfItems.DomAfList.Set(item)
		}
	}

	return p.Patch(ctx, f, f2, b, dom)
}

func (p *Provider) DeleteBGP(ctx context.Context, req *provider.DeleteBGPRequest) error {
	return p.client.Delete(ctx, new(BGP))
}

func (p *Provider) EnsureBGPPeer(ctx context.Context, req *provider.EnsureBGPPeerRequest) error {
	// Ensure that the BGP instance exists and is configured on the "default" domain
	// and return an error if it does not exist.
	bgp := new(BGPDom)
	bgp.Name = DefaultVRFName
	if err := p.client.GetConfig(ctx, bgp); err != nil {
		return fmt.Errorf("bgp peer: failed to get bgp instance 'default': %w", err)
	}

	pe := new(BGPPeer)
	pe.Addr = req.BGPPeer.Spec.Address
	pe.AdminSt = AdminStEnabled
	if req.BGPPeer.Spec.AdminState == v1alpha1.AdminStateDown {
		pe.AdminSt = AdminStDisabled
	}
	pe.Asn = req.BGPPeer.Spec.ASNumber.String()
	pe.AsnType = PeerAsnTypeNone
	pe.Name = req.BGPPeer.Spec.Description

	if req.SourceInterface != "" {
		srcIf, err := ShortName(req.SourceInterface)
		if err != nil {
			return fmt.Errorf("bgp peer: invalid source interface name %q: %w", req.SourceInterface, err)
		}
		pe.SrcIf = srcIf
	}

	if req.BGPPeer.Spec.AddressFamilies != nil {
		for t, af := range map[AddressFamily]*v1alpha1.BGPPeerAddressFamily{
			AddressFamilyIPv4Unicast: req.BGPPeer.Spec.AddressFamilies.Ipv4Unicast,
			AddressFamilyIPv6Unicast: req.BGPPeer.Spec.AddressFamilies.Ipv6Unicast,
			AddressFamilyL2EVPN:      req.BGPPeer.Spec.AddressFamilies.L2vpnEvpn,
		} {
			if af == nil || !af.Enabled {
				continue
			}
			item := new(BGPPeerAfItem)
			item.Type = t
			item.SendComStd = AdminStDisabled
			if af.SendCommunity == v1alpha1.BGPCommunityTypeStandard || af.SendCommunity == v1alpha1.BGPCommunityTypeBoth {
				item.SendComStd = AdminStEnabled
			}
			item.SendComExt = AdminStDisabled
			if af.SendCommunity == v1alpha1.BGPCommunityTypeExtended || af.SendCommunity == v1alpha1.BGPCommunityTypeBoth {
				item.SendComExt = AdminStEnabled
			}
			if af.RouteReflectorClient {
				item.Ctrl = NewOption(RouteReflectorClient)
			}
			pe.AfItems.PeerAfList.Set(item)
		}
	}

	return p.Update(ctx, pe)
}

func (p *Provider) DeleteBGPPeer(ctx context.Context, req *provider.DeleteBGPPeerRequest) error {
	b := new(BGPPeer)
	b.Addr = req.BGPPeer.Spec.Address
	return p.client.Delete(ctx, b)
}

func (p *Provider) GetPeerStatus(ctx context.Context, req *provider.BGPPeerStatusRequest) (provider.BGPPeerStatus, error) {
	ps := new(BGPPeerOperItems)
	ps.Addr = req.BGPPeer.Spec.Address
	if err := p.client.GetState(ctx, ps); err != nil && !errors.Is(err, gnmiext.ErrNil) {
		return provider.BGPPeerStatus{}, err
	}

	res := provider.BGPPeerStatus{
		SessionState:        ps.OperSt.ToSessionState(),
		LastEstablishedTime: ps.LastFlapTime,
		AddressFamilies:     make(map[v1alpha1.BGPAddressFamilyType]*provider.PrefixStats),
	}

	for _, af := range ps.AfItems.PeerAfList {
		sent, err := strconv.ParseUint(af.PfxSent, 10, 32)
		if err != nil {
			return provider.BGPPeerStatus{}, fmt.Errorf("bgp peer status: failed to parse sent prefixes %q: %w", af.PfxSent, err)
		}
		res.AddressFamilies[af.Type.ToAddressFamilyType()] = &provider.PrefixStats{
			Accepted:   af.AcceptedPaths,
			Advertised: uint32(sent),
		}
	}

	return res, nil
}

func (p *Provider) EnsureCertificate(ctx context.Context, req *provider.EnsureCertificateRequest) error {
	tp := new(Trustpoint)
	tp.Name = req.ID

	if err := p.Patch(ctx, tp); err != nil {
		return err
	}

	key, ok := req.Certificate.PrivateKey.(*rsa.PrivateKey)
	if !ok {
		return fmt.Errorf("unsupported private key type: expected *rsa.PrivateKey, got %T", req.Certificate.PrivateKey)
	}

	kp := new(KeyPair)
	kp.Name = req.ID
	if err := p.client.GetConfig(ctx, kp); !errors.Is(err, gnmiext.ErrNil) {
		// If the key pair already exists, we cannot update it, so we skip the rest of the process.
		return err
	}

	cert := &Certificate{Key: key, Cert: req.Certificate.Leaf}
	return cert.Load(ctx, p.conn, req.ID)
}

func (p *Provider) DeleteCertificate(ctx context.Context, req *provider.DeleteCertificateRequest) error {
	tp := new(Trustpoint)
	tp.Name = req.ID

	kp := new(KeyPair)
	kp.Name = req.ID

	return p.client.Delete(ctx, tp, kp)
}

func (p *Provider) EnsureDNS(ctx context.Context, req *provider.EnsureDNSRequest) error {
	d := new(DNS)
	d.AdminSt = AdminStEnabled
	if req.DNS.Spec.AdminState == v1alpha1.AdminStateDown {
		d.AdminSt = AdminStDisabled
	}

	pf := new(DNSProf)
	pf.Name = DefaultVRFName
	pf.DomItems.Name = req.DNS.Spec.Domain
	for _, s := range req.DNS.Spec.Servers {
		prov := new(DNSProv)
		prov.Addr = s.Address
		prov.SrcIf = req.DNS.Spec.SourceInterfaceName
		if s.VrfName == "" {
			pf.ProvItems.ProviderList.Set(prov)
			continue
		}
		vrf, ok := pf.VrfItems.VrfList.Get(s.VrfName)
		if !ok {
			vrf = new(DNSVrf)
			vrf.Name = s.VrfName
		}
		vrf.ProvItems.ProviderList.Set(prov)
		pf.VrfItems.VrfList.Set(vrf)
	}
	d.ProfItems.ProfList.Set(pf)

	return p.Update(ctx, d)
}

func (p *Provider) DeleteDNS(ctx context.Context) error {
	d := new(DNS)
	return p.client.Delete(ctx, d)
}

func (p *Provider) EnsureEVPNInstance(ctx context.Context, req *provider.EVPNInstanceRequest) (err error) {
	f := new(Feature)
	f.Name = "nvo"
	f.AdminSt = AdminStEnabled

	f2 := new(Feature)
	f2.Name = "vnsegment"
	f2.AdminSt = AdminStEnabled

	if err := p.Update(ctx, f, f2); err != nil {
		return err
	}

	conf := make([]gnmiext.Configurable, 0, 3)
	if req.EVPNInstance.Spec.Type == v1alpha1.EVPNInstanceTypeBridged {
		v := new(VLAN)
		v.FabEncap = "vlan-" + strconv.FormatInt(int64(req.VLAN.Spec.ID), 10)
		if err := p.client.GetConfig(ctx, v); err != nil {
			return fmt.Errorf("evpn instance: failed to get vlan %d: %w", req.VLAN.Spec.ID, err)
		}

		vxlan := new(VXLAN)
		vxlan.AccEncap = "vxlan-" + strconv.FormatInt(int64(req.EVPNInstance.Spec.VNI), 10)
		vxlan.FabEncap = v.FabEncap
		conf = append(conf, vxlan)
	}

	vni := new(VNI)
	vni.Vni = req.EVPNInstance.Spec.VNI
	if req.EVPNInstance.Spec.MulticastGroupAddress != "" {
		vni.McastGroup = NewOption(req.EVPNInstance.Spec.MulticastGroupAddress)
	}
	conf = append(conf, vni)

	switch req.EVPNInstance.Spec.Type {
	case v1alpha1.EVPNInstanceTypeBridged:
		evi := new(BDEVI)
		evi.Encap = "vxlan-" + strconv.FormatInt(int64(req.EVPNInstance.Spec.VNI), 10)
		evi.Rd, err = RouteDistinguisher(req.EVPNInstance.Spec.RouteDistinguisher)
		if err != nil {
			return fmt.Errorf("evpn instance: invalid route distinguisher: %w", err)
		}
		imports := &RttEntry{Type: RttEntryTypeImport}
		exports := &RttEntry{Type: RttEntryTypeExport}
		targets := req.EVPNInstance.Spec.RouteTargets
		if len(targets) == 0 {
			// If no route targets are specified, use 'route-target:unknown:0:0' for both import and export.
			// This is equivalent to 'route-target both auto' on the command line.
			targets = append(targets, v1alpha1.EVPNRouteTarget{Action: v1alpha1.RouteTargetActionBoth})
		}
		for _, rt := range targets {
			s, err := RouteTarget(rt.Value)
			if err != nil {
				return fmt.Errorf("evpn instance: invalid import route target: %w", err)
			}
			r := &Rtt{Rtt: s}
			switch rt.Action {
			case v1alpha1.RouteTargetActionImport:
				imports.EntItems.RttEntryList.Set(r)
			case v1alpha1.RouteTargetActionExport:
				exports.EntItems.RttEntryList.Set(r)
			case v1alpha1.RouteTargetActionBoth:
				imports.EntItems.RttEntryList.Set(r)
				exports.EntItems.RttEntryList.Set(r)
			}
		}
		if imports.EntItems.RttEntryList.Len() > 0 {
			evi.RttpItems.RttPList.Set(imports)
		}
		if exports.EntItems.RttEntryList.Len() > 0 {
			evi.RttpItems.RttPList.Set(exports)
		}
		conf = append(conf, evi)

	case v1alpha1.EVPNInstanceTypeRouted:
		vni.AssociateVrfFlag = true
	}

	return p.Update(ctx, conf...)
}

func (p *Provider) DeleteEVPNInstance(ctx context.Context, req *provider.EVPNInstanceRequest) error {
	conf := make([]gnmiext.Configurable, 0, 3)

	evi := new(BDEVI)
	evi.Encap = "vxlan-" + strconv.FormatInt(int64(req.EVPNInstance.Spec.VNI), 10)
	conf = append(conf, evi)

	vni := new(VNI)
	vni.Vni = req.EVPNInstance.Spec.VNI
	conf = append(conf, vni)

	if req.EVPNInstance.Spec.Type == v1alpha1.EVPNInstanceTypeBridged {
		bd := new(BDItems)
		if err := p.client.GetConfig(ctx, bd); err != nil && !errors.Is(err, gnmiext.ErrNil) {
			return err
		}

		if v := bd.GetByVXLAN(evi.Encap); v != nil {
			conf = append(conf, v)
		}
	}

	return p.client.Delete(ctx, conf...)
}

func (p *Provider) EnsureInterface(ctx context.Context, req *provider.EnsureInterfaceRequest) error {
	name, err := ShortName(req.Interface.Spec.Name)
	if err != nil {
		return err
	}

	var cfg nxv1alpha1.InterfaceConfig
	if req.ProviderConfig != nil {
		if err := req.ProviderConfig.Into(&cfg); err != nil {
			return err
		}
	}

	vrf := DefaultVRFName
	if req.VRF != nil {
		vrf = req.VRF.Spec.Name
	}

	var addr *AddrItem
	if req.IPv4 != nil {
		addr = new(AddrItem)
		addr.ID = name
		addr.Vrf = vrf

		switch v := req.IPv4.(type) {
		case provider.IPv4AddressList:
			for i, p := range v {
				nth := IntfAddrTypePrimary
				if i > 0 {
					nth = IntfAddrTypeSecondary
				}
				ip := &IntfAddr{
					Addr: p.String(),
					Type: nth,
				}
				addr.AddrItems.AddrList.Set(ip)
			}

		case provider.IPv4Unnumbered:
			addr.Unnumbered, err = ShortName(v.SourceInterface)
			if err != nil {
				return fmt.Errorf("invalid unnumbered source interface name %q: %w", v.SourceInterface, err)
			}
		}
	}

	if req.Interface.Spec.Type != v1alpha1.InterfaceTypeAggregate {
		del := make([]gnmiext.Configurable, 0, 2)
		addrs := new(AddrList)
		if err := p.client.GetConfig(ctx, addrs); err != nil && !errors.Is(err, gnmiext.ErrNil) {
			return err
		}
		for _, a := range addrs.GetAddrItemsByInterface(name) {
			if addr == nil || a.Vrf != vrf {
				del = append(del, a)
			}
		}
		if err := p.client.Delete(ctx, del...); err != nil {
			return err
		}
	}

	conf := make([]gnmiext.Configurable, 0, 4)
	switch req.Interface.Spec.Type {
	case v1alpha1.InterfaceTypePhysical:
		p := new(PhysIf)
		p.Default()
		p.ID = name
		p.Descr = req.Interface.Spec.Description
		if req.Interface.Spec.AdminState == v1alpha1.AdminStateUp {
			p.AdminSt = AdminStUp
		}
		p.UserCfgdFlags = UserFlagAdminState | UserFlagAdminLayer
		// TODO: If the interface is a member of a port-channel, do the following:
		// 1) If the mtu has been explicitly configured on the port-channel and matches the mtu on the physical interface, adopt the "admin_mtu" flag.
		// 2) If the mtu on the port-channel differs from the mtu on the physical interface, return an error.
		// 3) If the mtu has not been explicitly configured on the port-channel, do not adopt the "admin_mtu" flag.
		if req.Interface.Spec.MTU != 0 {
			p.MTU = req.Interface.Spec.MTU
			p.UserCfgdFlags |= UserFlagAdminMTU
		}
		if req.IPv4 != nil {
			p.Layer = Layer3
		}
		if addr.IsPointToPoint() {
			p.Medium = MediumPointToPoint
		}
		p.AccessVlan = "unknown"
		p.NativeVlan = "unknown"
		p.RtvrfMbrItems = NewVrfMember(name, vrf)

		if req.Interface.Spec.Switchport != nil {
			p.RtvrfMbrItems = nil
			p.AccessVlan = DefaultVLAN
			p.NativeVlan = DefaultVLAN
			switch req.Interface.Spec.Switchport.Mode {
			case v1alpha1.SwitchportModeAccess:
				p.Mode = SwitchportModeAccess
				p.AccessVlan = fmt.Sprintf("vlan-%d", req.Interface.Spec.Switchport.AccessVlan)
			case v1alpha1.SwitchportModeTrunk:
				p.Mode = SwitchportModeTrunk
				if req.Interface.Spec.Switchport.NativeVlan != 0 {
					p.NativeVlan = fmt.Sprintf("vlan-%d", req.Interface.Spec.Switchport.NativeVlan)
				}
				if len(req.Interface.Spec.Switchport.AllowedVlans) > 0 {
					p.TrunkVlans = Range(req.Interface.Spec.Switchport.AllowedVlans)
				}
			default:
				return fmt.Errorf("invalid switchport mode: %s", req.Interface.Spec.Switchport.Mode)
			}
		}

		if cfg.Spec.BufferBoost != nil {
			p.PhysExtdItems.BufferBoost = AdminStDisable
			if cfg.Spec.BufferBoost.Enabled {
				p.PhysExtdItems.BufferBoost = AdminStEnable
			}
		}

		if err := p.Validate(); err != nil {
			return err
		}

		conf = append(conf, p)

	case v1alpha1.InterfaceTypeLoopback:
		lb := new(Loopback)
		lb.ID = name
		lb.Descr = req.Interface.Spec.Description
		lb.AdminSt = AdminStDown
		if req.Interface.Spec.AdminState == v1alpha1.AdminStateUp {
			lb.AdminSt = AdminStUp
		}
		lb.RtvrfMbrItems = NewVrfMember(name, vrf)
		conf = append(conf, lb)

	case v1alpha1.InterfaceTypeAggregate:
		f := new(Feature)
		f.Name = "lacp"
		f.AdminSt = AdminStEnabled
		conf = append(conf, f)

		pcNum, err := strconv.Atoi(name[2:])
		if err != nil {
			return fmt.Errorf("iface: invalid port-channel number in name %q: %w", name, err)
		}
		if pcNum < 1 || pcNum > 4096 {
			return errors.New("iface: port-channel number must be between 1 and 4096")
		}

		pc := new(PortChannel)
		pc.ID = name
		pc.Descr = req.Interface.Spec.Description
		pc.AdminSt = AdminStDown
		if req.Interface.Spec.AdminState == v1alpha1.AdminStateUp {
			pc.AdminSt = AdminStUp
		}
		// Note: Layer 3 port-channel interfaces are not yet supported
		pc.Layer = Layer2
		pc.Mode = SwitchportModeAccess
		pc.AccessVlan = DefaultVLAN
		pc.NativeVlan = DefaultVLAN
		pc.TrunkVlans = DefaultVLANRange
		pc.UserCfgdFlags = UserFlagAdminState | UserFlagAdminLayer
		pc.AggrExtdItems.BufferBoost = AdminStEnable

		pc.MTU = DefaultMTU
		if req.Interface.Spec.MTU != 0 {
			pc.MTU = req.Interface.Spec.MTU
			pc.UserCfgdFlags |= UserFlagAdminMTU
		}

		pc.PcMode = PortChannelModeActive
		switch m := req.Interface.Spec.Aggregation.ControlProtocol.Mode; m {
		case v1alpha1.LACPModeActive:
			pc.PcMode = PortChannelModeActive
		case v1alpha1.LACPModePassive:
			pc.PcMode = PortChannelModePassive
		default:
			return fmt.Errorf("iface: unknown LACP mode: %s", m)
		}

		if req.Interface.Spec.Switchport != nil {
			switch req.Interface.Spec.Switchport.Mode {
			case v1alpha1.SwitchportModeAccess:
				pc.Mode = SwitchportModeAccess
				pc.AccessVlan = fmt.Sprintf("vlan-%d", req.Interface.Spec.Switchport.AccessVlan)
			case v1alpha1.SwitchportModeTrunk:
				pc.Mode = SwitchportModeTrunk
				if req.Interface.Spec.Switchport.NativeVlan != 0 {
					pc.NativeVlan = fmt.Sprintf("vlan-%d", req.Interface.Spec.Switchport.NativeVlan)
				}
				if len(req.Interface.Spec.Switchport.AllowedVlans) > 0 {
					pc.TrunkVlans = Range(req.Interface.Spec.Switchport.AllowedVlans)
				}
			default:
				return fmt.Errorf("invalid switchport mode: %s", req.Interface.Spec.Switchport.Mode)
			}
		}

		for _, member := range req.Members {
			n, err := ShortNamePhysicalInterface(member.Spec.Name)
			if err != nil {
				return err
			}
			pc.RsmbrIfsItems.RsMbrIfsList.Set(NewPortChannelMember(n))
		}

		v := new(VPCIfItems)
		if err := p.client.GetConfig(ctx, v); err != nil && !errors.Is(err, gnmiext.ErrNil) {
			return err
		}

		// Delete the existing VPC interface entry if the MultiChassisID has changed or got removed.
		if vpc := v.GetListItemByInterface(name); vpc != nil {
			if req.MultiChassisID == nil || int(*req.MultiChassisID) != vpc.ID {
				if err := p.client.Delete(ctx, vpc); err != nil {
					return err
				}
			}
		}

		if cfg.Spec.BufferBoost != nil {
			pc.AggrExtdItems.BufferBoost = AdminStDisable
			if cfg.Spec.BufferBoost.Enabled {
				pc.AggrExtdItems.BufferBoost = AdminStEnable
			}
		}

		conf = append(conf, pc)

		if req.MultiChassisID != nil {
			v := new(VPCIf)
			v.ID = int(*req.MultiChassisID)
			v.SetPortChannel(name)
			conf = append(conf, v)
		}

	case v1alpha1.InterfaceTypeRoutedVLAN:
		f := new(Feature)
		f.Name = "ifvlan"
		f.AdminSt = AdminStEnabled
		conf = append(conf, f)

		svi := new(SwitchVirtualInterface)
		svi.ID = name
		svi.Descr = req.Interface.Spec.Description
		svi.AdminSt = AdminStDown
		if req.Interface.Spec.AdminState == v1alpha1.AdminStateUp {
			svi.AdminSt = AdminStUp
		}
		svi.Medium = SVIMediumBroadcast
		svi.MTU = DefaultMTU
		if req.Interface.Spec.MTU != 0 {
			svi.MTU = req.Interface.Spec.MTU
		}
		svi.VlanID = req.VLAN.Spec.ID
		svi.RtvrfMbrItems = NewVrfMember(name, vrf)
		conf = append(conf, svi)

		fwif := new(FabricFwdIf)
		fwif.ID = name

		switch {
		case req.Interface.Spec.IPv4 != nil && req.Interface.Spec.IPv4.AnycastGateway:
			var mac FabricFwdAnycastMAC
			if err := p.client.GetConfig(ctx, &mac); err != nil {
				if errors.Is(err, gnmiext.ErrNil) {
					return errors.New("anycast gateway: no anycast MAC address configured on device")
				}
				return err
			}

			fwif.AdminSt = AdminStEnabled
			fwif.Mode = FwdModeAnycastGateway
			conf = append(conf, fwif)
		default:
			if err := p.client.Delete(ctx, fwif); err != nil {
				return err
			}
		}

	default:
		return fmt.Errorf("unsupported interface type: %s", req.Interface.Spec.Type)
	}

	if (req.Interface.Spec.Type == v1alpha1.InterfaceTypePhysical && req.IPv4 == nil) || req.Interface.Spec.Type == v1alpha1.InterfaceTypeAggregate {
		stp := new(SpanningTree)
		stp.IfName = name
		stp.Mode = SpanningTreeModeDefault
		stp.BPDUfilter = "default"
		stp.BPDUGuard = "default"
		if cfg.Spec.SpanningTree != nil {
			switch cfg.Spec.SpanningTree.PortType {
			case nxv1alpha1.SpanningTreePortTypeEdge:
				stp.Mode = SpanningTreeModeEdge
			case nxv1alpha1.SpanningTreePortTypeNetwork:
				stp.Mode = SpanningTreeModeNetwork
			}
			if cfg.Spec.SpanningTree.BPDUFilter != nil {
				stp.BPDUfilter = AdminStDisable
				if *cfg.Spec.SpanningTree.BPDUFilter {
					stp.BPDUfilter = AdminStEnable
				}
			}
			if cfg.Spec.SpanningTree.BPDUGuard != nil {
				stp.BPDUGuard = AdminStDisable
				if *cfg.Spec.SpanningTree.BPDUGuard {
					stp.BPDUGuard = AdminStEnable
				}
			}
		}
		conf = append(conf, stp)
	}

	// Add the address items last, as they depend on the interface being created first.
	if addr != nil {
		conf = append(conf, addr)
	}

	bfd := new(BFD)
	bfd.ID = name
	if req.Interface.Spec.BFD != nil {
		f := new(Feature)
		f.Name = "bfd"
		f.AdminSt = AdminStEnabled
		conf = append(conf, f)

		icmp := new(ICMPIf)
		icmp.ID = name
		icmp.Ctrl = "port-unreachable"
		conf = append(conf, icmp)

		bfd.AdminSt = AdminStDisabled
		if req.Interface.Spec.BFD.Enabled {
			bfd.AdminSt = AdminStEnabled
			bfd.IfkaItems.MinTxIntvlMs = 50
			if req.Interface.Spec.BFD.DesiredMinimumTxInterval != nil {
				bfd.IfkaItems.MinTxIntvlMs = req.Interface.Spec.BFD.DesiredMinimumTxInterval.Milliseconds()
			}
			bfd.IfkaItems.MinRxIntvlMs = 50
			if req.Interface.Spec.BFD.RequiredMinimumReceive != nil {
				bfd.IfkaItems.MinRxIntvlMs = req.Interface.Spec.BFD.RequiredMinimumReceive.Milliseconds()
			}
			bfd.IfkaItems.DetectMult = 3
			if req.Interface.Spec.BFD.DetectionMultiplier != nil {
				bfd.IfkaItems.DetectMult = *req.Interface.Spec.BFD.DetectionMultiplier
			}
			if err := bfd.Validate(); err != nil {
				return err
			}
		}
		conf = append(conf, bfd)
	} else {
		icmp := new(ICMPIf)
		icmp.ID = name
		switch req.Interface.Spec.Type {
		case v1alpha1.InterfaceTypePhysical:
			if err := p.client.Delete(ctx, icmp); err != nil {
				return err
			}
		case v1alpha1.InterfaceTypeLoopback:
			icmp.Ctrl = "port-unreachable,redirect"
			conf = append(conf, icmp)
		case v1alpha1.InterfaceTypeRoutedVLAN:
			icmp.Ctrl = "port-unreachable"
			conf = append(conf, icmp)
		}
	}

	return p.Update(ctx, conf...)
}

func (p *Provider) DeleteInterface(ctx context.Context, req *provider.InterfaceRequest) error {
	name, err := ShortName(req.Interface.Spec.Name)
	if err != nil {
		return err
	}

	conf := make([]gnmiext.Configurable, 0, 3)
	if req.Interface.Spec.Type != v1alpha1.InterfaceTypeAggregate {
		addrs := new(AddrList)
		if err := p.client.GetConfig(ctx, addrs); err != nil && !errors.Is(err, gnmiext.ErrNil) {
			return err
		}
		for _, addr := range addrs.GetAddrItemsByInterface(name) {
			conf = append(conf, addr)
		}
	}

	bfd := new(BFD)
	bfd.ID = name
	conf = append(conf, bfd)

	switch req.Interface.Spec.Type {
	case v1alpha1.InterfaceTypePhysical:
		i := new(PhysIf)
		i.ID = name
		conf = append(conf, i)

		// Delete any spanning tree config associated with the interface.
		stp := new(SpanningTree)
		stp.IfName = name
		if err = p.client.GetConfig(ctx, stp); err == nil {
			conf = append(conf, stp)
		}

		icmp := new(ICMPIf)
		icmp.ID = name
		conf = append(conf, icmp)

	case v1alpha1.InterfaceTypeLoopback:
		lb := new(Loopback)
		lb.ID = name
		conf = append(conf, lb)

	case v1alpha1.InterfaceTypeAggregate:
		pc := new(PortChannel)
		pc.ID = name
		conf = append(conf, pc)

		v := new(VPCIfItems)
		if err := p.client.GetConfig(ctx, v); err != nil && !errors.Is(err, gnmiext.ErrNil) {
			return err
		}

		// Make sure to delete any associated VPC interface.
		if vpc := v.GetListItemByInterface(name); vpc != nil {
			conf = append(conf, vpc)
		}

	case v1alpha1.InterfaceTypeRoutedVLAN:
		svi := new(SwitchVirtualInterface)
		svi.ID = name
		conf = append(conf, svi)

	default:
		return fmt.Errorf("unsupported interface type: %s", req.Interface.Spec.Type)
	}

	return p.client.Delete(ctx, conf...)
}

func (p *Provider) GetInterfaceStatus(ctx context.Context, req *provider.InterfaceRequest) (provider.InterfaceStatus, error) {
	name, err := ShortName(req.Interface.Spec.Name)
	if err != nil {
		return provider.InterfaceStatus{}, err
	}

	var (
		operSt  OperSt
		operMsg string
	)
	switch req.Interface.Spec.Type {
	case v1alpha1.InterfaceTypePhysical:
		phys := new(PhysIfOperItems)
		phys.ID = name
		if err := p.client.GetState(ctx, phys); err != nil && !errors.Is(err, gnmiext.ErrNil) {
			return provider.InterfaceStatus{}, err
		}
		operSt = phys.OperSt

	case v1alpha1.InterfaceTypeLoopback:
		lb := new(LoopbackOperItems)
		lb.ID = name
		if err := p.client.GetState(ctx, lb); err != nil && !errors.Is(err, gnmiext.ErrNil) {
			return provider.InterfaceStatus{}, err
		}
		operSt = lb.OperSt

	case v1alpha1.InterfaceTypeAggregate:
		pc := new(PortChannelOperItems)
		pc.ID = name
		if err := p.client.GetState(ctx, pc); err != nil && !errors.Is(err, gnmiext.ErrNil) {
			return provider.InterfaceStatus{}, err
		}
		operSt = pc.OperSt
		operMsg = pc.OperStQual

	case v1alpha1.InterfaceTypeRoutedVLAN:
		svi := new(SwitchVirtualInterfaceOperItems)
		svi.ID = name
		if err := p.client.GetState(ctx, svi); err != nil && !errors.Is(err, gnmiext.ErrNil) {
			return provider.InterfaceStatus{}, err
		}
		operSt = svi.OperSt

	default:
		return provider.InterfaceStatus{}, fmt.Errorf("unsupported interface type: %s", req.Interface.Spec.Type)
	}

	return provider.InterfaceStatus{
		OperStatus:  operSt == OperStUp,
		OperMessage: operMsg,
	}, nil
}

var ErrInterfaceNotFound = errors.New("one or more interfaces do not exist")

func (p *Provider) EnsureInterfacesExist(ctx context.Context, interfaces []*v1alpha1.Interface) (names []string, err error) {
	names = make([]string, 0, len(interfaces))
	for _, iface := range interfaces {
		name, err := ShortName(iface.Spec.Name)
		if err != nil {
			return nil, err
		}
		names = append(names, name)
	}

	exists, err := Exists(ctx, p.client, names...)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrInterfaceNotFound
	}
	return names, nil
}

func (p *Provider) EnsureISIS(ctx context.Context, req *provider.EnsureISISRequest) error {
	f := new(Feature)
	f.Name = "isis"
	f.AdminSt = AdminStEnabled

	conf := append(make([]gnmiext.Configurable, 0, 3), f)

	if slices.ContainsFunc(req.Interfaces, func(intf *v1alpha1.Interface) bool {
		return intf.Spec.BFD.Enabled
	}) {
		f := new(Feature)
		f.Name = "bfd"
		f.AdminSt = AdminStEnabled
		conf = append(conf, f)
	}

	i := new(ISIS)
	i.AdminSt = AdminStEnabled
	if req.ISIS.Spec.AdminState == v1alpha1.AdminStateDown {
		i.AdminSt = AdminStDisabled
	}
	i.Name = req.ISIS.Spec.Instance

	dom := new(ISISDom)
	dom.Name = DefaultVRFName
	dom.Net = req.ISIS.Spec.NetworkEntityTitle
	dom.IsType = ISISLevelFrom(req.ISIS.Spec.Type)
	dom.PassiveDflt = dom.IsType
	i.DomItems.DomList.Set(dom)

	switch req.ISIS.Spec.OverloadBit {
	case v1alpha1.OverloadBitNever:
	case v1alpha1.OverloadBitAlways:
	case v1alpha1.OverloadBitOnStartup:
		dom.OverloadItems.AdminSt = "bootup"
		dom.OverloadItems.BgpAsNumStr = "none"
		dom.OverloadItems.StartupTime = 61 // seconds
		dom.OverloadItems.Suppress = ""
	}

	var ipv4, ipv6 bool
	for _, af := range req.ISIS.Spec.AddressFamilies {
		item := new(ISISDomAf)
		switch af {
		case v1alpha1.AddressFamilyIPv4Unicast:
			item.Type = ISISAfIPv4Unicast
			ipv4 = true
		case v1alpha1.AddressFamilyIPv6Unicast:
			item.Type = ISISAfIPv6Unicast
			ipv6 = true
		}
		dom.AfItems.DomAfList.Set(item)
	}

	interfaceNames, err := p.EnsureInterfacesExist(ctx, req.Interfaces)
	if err != nil {
		return err
	}

	// prevent bounds check in for the range loop below
	// [Bounds Check Elimination]: https://go101.org/optimizations/5-bce.html
	_ = req.Interfaces[len(interfaceNames)-1]

	for i, iface := range req.Interfaces {
		intf := new(ISISInterface)
		intf.ID = interfaceNames[i]
		intf.NetworkTypeP2P = AdminStOff
		if iface.Spec.Type == v1alpha1.InterfaceTypePhysical {
			intf.NetworkTypeP2P = AdminStOn
		}
		if ipv4 {
			intf.V4Enable = true
			intf.V4Bfd = "inheritVrf"
			if iface.Spec.BFD.Enabled {
				intf.V4Bfd = "enabled"
			}
		}
		if ipv6 {
			intf.V6Enable = true
			intf.V6Bfd = "inheritVrf"
			if iface.Spec.BFD.Enabled {
				intf.V6Bfd = "enabled"
			}
		}
		dom.IfItems.IfList.Set(intf)
	}
	conf = append(conf, i)

	return p.Update(ctx, conf...)
}

func (p *Provider) DeleteISIS(ctx context.Context, req *provider.DeleteISISRequest) error {
	i := new(ISIS)
	i.Name = req.ISIS.Spec.Instance
	return p.client.Delete(ctx, i)
}

func (p *Provider) EnsureManagementAccess(ctx context.Context, req *provider.EnsureManagementAccessRequest) error {
	gf := new(Feature)
	gf.Name = "grpc"
	gf.AdminSt = AdminStEnabled
	if !req.ManagementAccess.Spec.GRPC.Enabled {
		return errors.New("management access: gRPC must be enabled")
	}

	sf := new(Feature)
	sf.Name = "ssh"
	sf.AdminSt = AdminStDisabled
	if req.ManagementAccess.Spec.SSH.Enabled {
		sf.AdminSt = AdminStEnabled
	}

	g := new(GRPC)
	g.Port = req.ManagementAccess.Spec.GRPC.Port
	g.UseVrf = req.ManagementAccess.Spec.GRPC.VrfName
	if g.UseVrf == "" {
		g.UseVrf = DefaultVRFName
	}
	if req.ManagementAccess.Spec.GRPC.CertificateID != "" {
		g.Cert = NewOption(req.ManagementAccess.Spec.GRPC.CertificateID)
	}
	if err := g.Validate(); err != nil {
		return err
	}

	gn := new(GNMI)
	gn.MaxCalls = req.ManagementAccess.Spec.GRPC.GNMI.MaxConcurrentCall
	gn.KeepAliveTimeout = int(req.ManagementAccess.Spec.GRPC.GNMI.KeepAliveTimeout.Seconds())
	if err := gn.Validate(); err != nil {
		return err
	}

	vty := new(VTY)
	vty.SsLmtItems.SesLmt = req.ManagementAccess.Spec.SSH.SessionLimit
	vty.ExecTmeoutItems.Timeout = int(req.ManagementAccess.Spec.SSH.Timeout.Minutes())
	if err := vty.Validate(); err != nil {
		return err
	}

	var cfg nxv1alpha1.ManagementAccessConfig
	if req.ProviderConfig != nil {
		if err := req.ProviderConfig.Into(&cfg); err != nil {
			return err
		}
	}

	con := new(Console)
	con.Timeout = int(cfg.Spec.Console.Timeout.Minutes())
	if err := con.Validate(); err != nil {
		return err
	}

	acl := new(VTYAccessClass)
	acl.Name = cfg.Spec.SSH.AccessControlListName

	if acl.Name == "" {
		if err := p.client.Delete(ctx, acl); err != nil && !errors.Is(err, gnmiext.ErrNil) {
			return err
		}
	}

	conf := make([]gnmiext.Configurable, 0, 7)
	conf = append(conf, gf, sf, g, gn, vty, con)
	if acl.Name != "" {
		conf = append(conf, acl)
	}

	return p.Patch(ctx, conf...)
}

func (p *Provider) DeleteManagementAccess(ctx context.Context) error {
	return p.client.Delete(
		ctx,
		new(GRPC),
		new(GNMI),
		new(VTY),
		new(Console),
	)
}

type NTPConfig struct {
	Log struct {
		Enable bool `json:"enable"`
	} `json:"log"`
}

func (p *Provider) EnsureNTP(ctx context.Context, req *provider.EnsureNTPRequest) error {
	f := new(Feature)
	f.Name = "ntpd"
	f.AdminSt = AdminStEnabled

	var cfg NTPConfig
	if req.ProviderConfig != nil {
		if err := req.ProviderConfig.Into(&cfg); err != nil {
			return err
		}
	}

	n := new(NTP)
	n.AdminSt = AdminStEnabled
	if req.NTP.Spec.AdminState == v1alpha1.AdminStateDown {
		n.AdminSt = AdminStDisabled
	}
	n.Logging = AdminStDisabled
	if cfg.Log.Enable {
		n.Logging = AdminStEnabled
	}
	for _, s := range req.NTP.Spec.Servers {
		prov := new(NTPProvider)
		prov.KeyID = 0
		prov.MaxPoll = 6
		prov.MinPoll = 4
		prov.Name = s.Address
		prov.Preferred = s.Prefer
		prov.ProvT = ProvTypeServer
		prov.Vrf = s.VrfName
		n.ProvItems.NtpProviderList.Set(prov)
	}
	n.SrcIfItems.SrcIf = req.NTP.Spec.SourceInterfaceName

	return p.Update(ctx, f, n)
}

func (p *Provider) DeleteNTP(ctx context.Context) error {
	n := new(NTP)

	f := new(Feature)
	f.Name = "ntpd"

	return p.client.Delete(ctx, n, f)
}

type NXOSPF struct {
	// PropagateDefaultRoute is equivalent to the CLI command `default-information originate`
	PropagateDefaultRoute *bool
	// RedistributionConfigs is a list of redistribution configurations for the OSPF process.
	RedistributionConfigs []RedistributionConfig
	// Distance is the administrative distance value (1-255) for OSPF routes. Cisco's default is 110.
	Distance int16
	// ReferenceBandwidthMbps is the reference bandwidth in Mbps used for OSPF calculations. By default Cisco NX-OS
	// assigns a cost that is the configured reference bandwidth divided by the interface bandwidth. The
	// the reference bandwidth in these devices is 40 Gbps. Must be between 1 and 999999 Mbps.
	ReferenceBandwidthMbps int32
	// MaxLSA is the maximum number of non self-generated LSAs (min 1)
	MaxLSA int32
}

// RedistributionConfig represents a redistribution configuration of a route map through a specific protocol.
type RedistributionConfig struct {
	// Protocol to redistribute, e.g., `direct`
	Protocol RtLeakProto
	// Route map to apply, e.g., `REDIST-ALL`
	RouteMapName string
}

func (p *Provider) EnsureOSPF(ctx context.Context, req *provider.EnsureOSPFRequest) error {
	var cfg NXOSPF
	if req.ProviderConfig != nil {
		if err := req.ProviderConfig.Into(&cfg); err != nil {
			return err
		}
	}

	conf := make([]gnmiext.Configurable, 0, 3)

	f := new(Feature)
	f.Name = "ospf"
	f.AdminSt = AdminStEnabled
	conf = append(conf, f)

	o := new(OSPF)
	o.AdminSt = AdminStEnabled
	if req.OSPF.Spec.AdminState == v1alpha1.AdminStateDown {
		o.AdminSt = AdminStDisabled
	}
	o.Name = req.OSPF.Spec.Instance
	conf = append(conf, o)

	dom := new(OSPFDom)
	dom.Name = DefaultVRFName
	dom.AdjChangeLogLevel = AdjChangeLogLevelNone
	if req.OSPF.Spec.LogAdjacencyChanges != nil && *req.OSPF.Spec.LogAdjacencyChanges {
		dom.AdjChangeLogLevel = AdjChangeLogLevelBrief
	}
	dom.AdminSt = AdminStEnabled
	if req.OSPF.Spec.AdminState == v1alpha1.AdminStateDown {
		dom.AdminSt = AdminStDisabled
	}
	dom.BwRef = DefaultBwRef // default 40 Gbps
	dom.BwRefUnit = BwRefUnitMbps
	if cfg.ReferenceBandwidthMbps != 0 {
		if cfg.ReferenceBandwidthMbps < 1 || cfg.ReferenceBandwidthMbps > 999999 {
			return fmt.Errorf("ospf: reference bandwidth %d is out of range (1-999999 Mbps)", cfg.ReferenceBandwidthMbps)
		}
		dom.BwRef = cfg.ReferenceBandwidthMbps
	}
	dom.Dist = DefaultDist
	if cfg.Distance != 0 {
		if cfg.Distance < 1 || cfg.Distance > 255 {
			return fmt.Errorf("ospf: distance %d is out of range (1-255)", cfg.Distance)
		}
		dom.Dist = cfg.Distance
	}
	dom.RtrID = req.OSPF.Spec.RouterID
	dom.Ctrl = "default-passive"
	o.DomItems.DomList.Set(dom)

	interfaces := make([]*v1alpha1.Interface, 0, len(req.Interfaces))
	for _, iface := range req.Interfaces {
		interfaces = append(interfaces, iface.Interface)
	}

	interfaceNames, err := p.EnsureInterfacesExist(ctx, interfaces)
	if err != nil {
		return err
	}

	// prevent bounds check in for the range loop below
	// [Bounds Check Elimination]: https://go101.org/optimizations/5-bce.html
	_ = req.Interfaces[len(interfaceNames)-1]

	for i, iface := range req.Interfaces {
		intf := new(OSPFInterface)
		intf.ID = interfaceNames[i]
		intf.AdminSt = AdminStEnabled
		intf.AdvertiseSecondaries = true
		intf.Area = iface.Area
		intf.NwT = NtwTypeUnspecified
		if iface.Interface.Spec.Type == v1alpha1.InterfaceTypePhysical {
			intf.NwT = NtwTypePointToPoint
		}
		intf.PassiveCtrl = PassiveControlUnspecified
		if iface.Passive == nil || !*iface.Passive {
			intf.PassiveCtrl = PassiveControlDisabled
		}
		intf.BFDCtrl = OspfBfdCtrlUnspecified
		if iface.Interface.Spec.BFD != nil {
			fb := new(Feature)
			fb.Name = "bfd"
			fb.AdminSt = AdminStEnabled
			conf = slices.Insert(conf, 1, gnmiext.Configurable(fb)) // insert before OSPF

			intf.BFDCtrl = OspfBfdCtrlDisabled
			if !iface.Interface.Spec.BFD.Enabled {
				intf.BFDCtrl = OspfBfdCtrlEnabled
			}
		}
		dom.IfItems.IfList.Set(intf)
	}

	for _, rc := range cfg.RedistributionConfigs {
		if rc.RouteMapName == "" {
			return errors.New("ospf: redistribution route map name cannot be empty")
		}
		rd := new(InterLeakP)
		rd.Proto = rc.Protocol
		rd.Asn = "none"
		rd.Inst = "none"
		rd.RtMap = rc.RouteMapName
		dom.InterleakItems.InterLeakPList.Set(rd)
	}

	if cfg.PropagateDefaultRoute != nil {
		dom.DefrtleakItems.Always = "no"
		if *cfg.PropagateDefaultRoute {
			dom.DefrtleakItems.Always = "yes"
		}
	}

	if cfg.MaxLSA != 0 {
		dom.MaxlsapItems.Action = MaxLSAActionReject
		dom.MaxlsapItems.MaxLsa = cfg.MaxLSA
	}

	return p.Update(ctx, conf...)
}

func (p *Provider) DeleteOSPF(ctx context.Context, req *provider.DeleteOSPFRequest) error {
	o := new(OSPF)
	o.Name = req.OSPF.Spec.Instance
	return p.client.Delete(ctx, o)
}

func (p *Provider) GetOSPFStatus(ctx context.Context, req *provider.OSPFStatusRequest) (provider.OSPFStatus, error) {
	name := make(map[string]*v1alpha1.Interface)
	for _, iface := range req.Interfaces {
		n, err := ShortName(iface.Interface.Spec.Name)
		if err != nil {
			return provider.OSPFStatus{}, err
		}
		name[n] = iface.Interface
	}

	st := new(OSPFOperItems)
	st.Name = req.OSPF.Spec.Instance

	if err := p.client.GetState(ctx, st); err != nil && !errors.Is(err, gnmiext.ErrNil) {
		return provider.OSPFStatus{}, err
	}

	neighbors := make([]provider.OSPFNeighbor, 0)
	for _, intf := range st.IfItems.IfList {
		i, ok := name[intf.ID]
		if !ok {
			continue
		}
		for _, adj := range intf.AdjItems.AdjList {
			neighbors = append(neighbors, provider.OSPFNeighbor{
				RouterID:            adj.ID,
				Address:             adj.PeerIP,
				Interface:           i,
				Priority:            adj.Prio,
				LastEstablishedTime: adj.AdjStatsItems.LastStChgTS,
				AdjacencyState:      adj.OperSt.ToNeighborState(),
			})
		}
	}

	return provider.OSPFStatus{
		OperStatus: st.OperSt == OperStUp,
		Neighbors:  neighbors,
	}, nil
}

func (p *Provider) EnsurePIM(ctx context.Context, req *provider.EnsurePIMRequest) error {
	f := new(Feature)
	f.Name = "pim"
	f.AdminSt = AdminStEnabled

	pim := new(PIM)
	pim.AdminSt = AdminStEnabled
	pim.InstItems.AdminSt = AdminStEnabled
	if req.PIM.Spec.AdminState == v1alpha1.AdminStateDown {
		pim.AdminSt = AdminStDisabled
		pim.InstItems.AdminSt = AdminStDisabled
	}

	dom := new(PIMDom)
	dom.Name = DefaultVRFName
	dom.AdminSt = AdminStEnabled
	if req.PIM.Spec.AdminState == v1alpha1.AdminStateDown {
		dom.AdminSt = AdminStDisabled
	}

	if err := p.Patch(ctx, f, pim, dom); err != nil {
		return err
	}

	rpItems := new(StaticRPItems)
	apItems := new(AnycastPeerItems)

	for _, rendezvousPoint := range req.PIM.Spec.RendezvousPoints {
		rp := new(StaticRP)
		rp.Addr = rendezvousPoint.Address + "/32"
		for _, group := range rendezvousPoint.MulticastGroups {
			if !group.IsValid() || !group.Addr().Is4() {
				return fmt.Errorf("pim: group list %q is not a valid IPv4 address prefix", group)
			}
			grp := new(StaticRPGrp)
			grp.GrpListName = group.String()
			rp.RpgrplistItems.RPGrpListList.Set(grp)
		}
		rpItems.StaticRPList.Set(rp)

		for _, addr := range rendezvousPoint.AnycastAddresses {
			peer := new(AnycastPeerAddr)
			peer.Addr = rendezvousPoint.Address + "/32"
			peer.RpSetAddr = addr + "/32"
			apItems.AcastRPPeerList.Set(peer)
		}
	}

	interfaces := make([]*v1alpha1.Interface, 0, len(req.Interfaces))
	for _, iface := range req.Interfaces {
		interfaces = append(interfaces, iface.Interface)
	}

	interfaceNames, err := p.EnsureInterfacesExist(ctx, interfaces)
	if err != nil {
		return err
	}

	// prevent bounds check in for the range loop below
	// [Bounds Check Elimination]: https://go101.org/optimizations/5-bce.html
	_ = req.Interfaces[len(interfaceNames)-1]

	ifItems := new(PIMIfItems)
	for i, name := range interfaceNames {
		intf := new(PIMIf)
		intf.ID = name
		switch req.Interfaces[i].Mode {
		case v1alpha1.PIMModeDense:
			return errors.New("pim: dense mode is not supported on Cisco NX-OS devices")
		case v1alpha1.PIMModeSparse:
			intf.PimSparseMode = true
		}
		ifItems.IfList.Set(intf)
	}

	conf := make([]gnmiext.Configurable, 0, 3)
	del := make([]gnmiext.Configurable, 0, 3)

	if len(rpItems.StaticRPList) > 0 {
		conf = append(conf, rpItems)
	} else {
		del = append(del, rpItems)
	}

	if len(apItems.AcastRPPeerList) > 0 {
		conf = append(conf, apItems)
	} else {
		del = append(del, apItems)
	}

	if len(ifItems.IfList) > 0 {
		conf = append(conf, ifItems)
	} else {
		del = append(del, ifItems)
	}

	if err := p.Update(ctx, conf...); err != nil {
		return err
	}

	return p.client.Delete(ctx, del...)
}

func (p *Provider) DeletePIM(ctx context.Context, _ *provider.DeletePIMRequest) error {
	pim := new(PIM)
	pim.AdminSt = AdminStDisabled
	pim.InstItems.AdminSt = AdminStDisabled

	dom := new(PIMDom)
	dom.Name = DefaultVRFName
	dom.AdminSt = AdminStDisabled

	if err := p.Patch(ctx, pim, dom); err != nil {
		return err
	}

	return p.client.Delete(ctx, new(StaticRPItems), new(AnycastPeerItems), new(PIMIfItems))
}

func (p *Provider) EnsurePrefixSet(ctx context.Context, req *provider.PrefixSetRequest) error {
	s := new(PrefixList)
	s.Name = req.PrefixSet.Spec.Name
	s.Is6 = req.PrefixSet.Is6()
	for _, entry := range req.PrefixSet.Spec.Entries {
		e := new(PrefixEntry)
		e.Action = ActionPermit
		e.Criteria = CriteriaExact
		e.Order = entry.Sequence
		e.Pfx = entry.Prefix.String()
		bits := int8(entry.Prefix.Bits()) // #nosec G115
		if entry.MaskLengthRange != nil && (entry.MaskLengthRange.Min != bits || entry.MaskLengthRange.Max != bits) {
			e.Criteria = CriteriaInexact
			e.ToPfxLen = entry.MaskLengthRange.Max
			if entry.MaskLengthRange.Min != bits {
				e.FromPfxLen = entry.MaskLengthRange.Min
			}
		}
		s.EntItems.EntryList.Set(e)
	}
	return p.Update(ctx, s)
}

func (p *Provider) DeletePrefixSet(ctx context.Context, req *provider.PrefixSetRequest) error {
	s := new(PrefixList)
	s.Name = req.PrefixSet.Spec.Name
	s.Is6 = req.PrefixSet.Is6()
	return p.client.Delete(ctx, s)
}

func (p *Provider) EnsureRoutingPolicy(ctx context.Context, req *provider.EnsureRoutingPolicyRequest) error {
	rm := new(RouteMap)
	rm.Name = req.Name
	for _, stmt := range req.Statements {
		e := new(RouteMapEntry)
		e.Order = stmt.Sequence

		for _, cond := range stmt.Conditions {
			switch v := cond.(type) {
			case provider.MatchPrefixSetCondition:
				e.SetPrefixSet(v.PrefixSet)
			default:
				return fmt.Errorf("routing policy: unsupported condition type %T", cond)
			}
		}

		switch stmt.Actions.RouteDisposition {
		case v1alpha1.AcceptRoute:
			e.Action = ActionPermit
		case v1alpha1.RejectRoute:
			e.Action = ActionDeny
		default:
			return fmt.Errorf("routing policy: unsupported action %q", stmt.Actions.RouteDisposition)
		}

		if stmt.Actions.BgpActions != nil {
			if stmt.Actions.BgpActions.SetCommunity != nil {
				if err := e.SetCommunities(stmt.Actions.BgpActions.SetCommunity.Communities); err != nil {
					return err
				}
			}
			if stmt.Actions.BgpActions.SetExtCommunity != nil {
				if err := e.SetExtCommunities(stmt.Actions.BgpActions.SetExtCommunity.Communities); err != nil {
					return err
				}
			}
		}

		rm.EntItems.EntryList.Set(e)
	}
	return p.Update(ctx, rm)
}

func (p *Provider) DeleteRoutingPolicy(ctx context.Context, req *provider.DeleteRoutingPolicyRequest) error {
	rm := new(RouteMap)
	rm.Name = req.Name
	return p.client.Delete(ctx, rm)
}

func (p *Provider) EnsureUser(ctx context.Context, req *provider.EnsureUserRequest) error {
	u := new(User)
	u.AllowExpired = "no"
	u.Expiration = "never"
	u.Name = req.Username
	u.SshauthItems.Data = req.SSHKey

	d := new(UserDomain)
	d.Name = "all"
	for _, role := range req.Roles {
		r := new(UserRole)
		r.Name = role
		d.RoleItems.UserRoleList.Set(r)
	}
	u.UserdomainItems.UserDomainList.Set(d)

	// If the user already exists and the password matches, retain the existing
	// password hash to avoid unnecessary updates.
	var enc Encoder = Plain{}
	user := new(User)
	user.Name = req.Username
	if err := p.client.GetConfig(ctx, user); err == nil {
		switch {
		case strings.HasPrefix(user.Pwd, "$5$"):
			if parts := strings.SplitN(user.Pwd, "$", 4); len(parts) >= 3 {
				// Algorithm expects the base64-encoded salt
				enc = Encrypt{Salt: []byte(parts[2])}
			}
		case strings.HasPrefix(user.Pwd, "$nx-pbkdf2$"):
			if salt, err := ParsePasswordSalt(user.Pwd); err == nil {
				enc = PBKDF2{Salt: salt}
			}
		case strings.HasPrefix(user.Pwd, "$nx-scrypt$"):
			if salt, err := ParsePasswordSalt(user.Pwd); err == nil {
				enc = Scrypt{Salt: salt}
			}
		}
	}

	if req.Password != "" {
		if err := u.SetPassword(req.Password, enc); err != nil {
			return fmt.Errorf("user: failed to encode password for user %q: %w", req.Username, err)
		}
	}

	return p.Patch(ctx, u)
}

func (p *Provider) DeleteUser(ctx context.Context, req *provider.DeleteUserRequest) error {
	u := new(User)
	u.Name = req.Username
	return p.client.Delete(ctx, u)
}

// EnsureSNMP ensures that the SNMP configuration on the device matches the desired state specified in the SNMP custom resource.
//
// It configures various SNMP components with the following default values:
//
// Communities:
//   - Default group: "network-operator" (used when Community.Group is empty)
//   - Access level: unspecified (CommAcess = unspecified)
//
// Hosts:
//   - Default port: 162 (standard SNMP trap port)
//   - Default security level: noauth (for v1/v2c), auth (for v3)
//   - Default notification type: traps (when Host.Type is not specified)
//   - Default version: v1 (when Host.Version is not specified)
//
// Traps:
//   - Individual traps are enabled by setting Trapstatus = "enable"
//
// System Information:
//   - Empty strings are converted to "DME_UNSET_PROPERTY_MARKER" for deletion
func (p *Provider) EnsureSNMP(ctx context.Context, req *provider.EnsureSNMPRequest) error {
	sysInfo := new(SNMPSysInfo)
	sysInfo.SysContact = NewOption(req.SNMP.Spec.Contact)
	sysInfo.SysLocation = NewOption(req.SNMP.Spec.Location)

	trapsSrcIf := new(SNMPSrcIf)
	trapsSrcIf.Type = Traps
	trapsSrcIf.Ifname = NewOption(req.SNMP.Spec.SourceInterfaceName)

	informsSrcIf := new(SNMPSrcIf)
	informsSrcIf.Type = Informs
	informsSrcIf.Ifname = NewOption(req.SNMP.Spec.SourceInterfaceName)

	communities := new(SNMPCommunityItems)
	for _, c := range req.SNMP.Spec.Communities {
		comm := new(SNMPCommunity)
		comm.Name = c.Name
		const defaultGroup = "network-operator"
		comm.GrpName = defaultGroup
		if c.Group != "" {
			comm.GrpName = c.Group
		}
		comm.CommAccess = "unspecified"
		comm.ACLItems.UseACLName = c.ACLName
		communities.CommSecPList.Set(comm)
	}

	hosts := new(SNMPHostItems)
	for _, h := range req.SNMP.Spec.Hosts {
		const port = 162
		host := new(SNMPHost)
		host.HostName = h.Address
		host.UDPPortID = port
		host.CommName = NewOption(h.Community)
		host.SecLevel = SecLevelNoAuth
		host.NotifType = strings.ToLower(h.Type)
		host.Version = h.Version
		if h.VrfName != "" {
			vrf := new(SNMPHostVrf)
			vrf.Vrfname = h.VrfName
			host.UsevrfItems.UseVrfList.Set(vrf)
		}
		if h.Version == "v3" {
			host.SecLevel = SecLevelAuth
		}
		hosts.HostList.Set(host)
	}

	// TODO: Once configured SNMP traps cannot be removed, so we do not
	//       attempt to disable individual traps that are not listed in
	//       the spec. Instead, we could consider adding a field to the
	//       SNMP spec.
	traps := new(SNMPTrapsItems)
	if err := p.client.GetConfig(ctx, traps); err != nil && !errors.Is(err, gnmiext.ErrNil) {
		return err
	}

	for _, t := range req.SNMP.Spec.Traps {
		parts := strings.Fields(t)
		rv := reflect.ValueOf(traps).Elem()
		for len(parts) > 0 {
			name := strings.ToUpper(parts[0][:1]) + parts[0][1:]
			name = strings.TrimSuffix(name, "-items") + "Items"
			name = strings.ReplaceAll(name, "-", "")
			rv = rv.FieldByName(name)
			if !rv.IsValid() {
				return fmt.Errorf("snmp: trap %q not found", t)
			}
			parts = parts[1:]
			rv = rv.Elem()
		}
		rv.Set(reflect.ValueOf(&SNMPTraps{Trapstatus: AdminStEnable}))
	}

	return p.Update(ctx, sysInfo, trapsSrcIf, informsSrcIf, communities, hosts, traps)
}

func (p *Provider) DeleteSNMP(ctx context.Context, req *provider.DeleteSNMPRequest) error {
	traps := new(SNMPTrapsItems)
	if err := p.Update(ctx, traps); err != nil {
		return err
	}

	trapsSrcIf := new(SNMPSrcIf)
	trapsSrcIf.Type = Traps

	informsSrcIf := new(SNMPSrcIf)
	informsSrcIf.Type = Informs

	return p.client.Delete(
		ctx,
		trapsSrcIf,
		informsSrcIf,
		new(SNMPSysInfo),
		new(SNMPCommunityItems),
		new(SNMPHostItems),
	)
}

type SyslogConfig struct {
	OriginID            string
	SourceInterfaceName string
	HistorySize         uint32
	HistoryLevel        v1alpha1.Severity
}

func (p *Provider) EnsureSyslog(ctx context.Context, req *provider.EnsureSyslogRequest) error {
	var cfg SyslogConfig
	cfg.OriginID = req.Syslog.Name
	cfg.SourceInterfaceName = "mgmt0"
	cfg.HistorySize = 500
	if req.ProviderConfig != nil {
		if err := req.ProviderConfig.Into(&cfg); err != nil {
			return err
		}
	}

	origin := new(SyslogOrigin)
	addr, err := netip.ParseAddr(cfg.OriginID)
	switch {
	case strings.ToLower(cfg.OriginID) == "hostname":
		origin.Idtype = "hostname"
	case err == nil && addr.IsValid():
		origin.Idtype = "ip"
		origin.Idvalue = addr.String()
	default:
		origin.Idtype = "string"
		origin.Idvalue = cfg.OriginID
	}

	srcIf := new(SyslogSrcIf)
	srcIf.AdminSt = AdminStEnabled
	srcIf.IfName = cfg.SourceInterfaceName

	hist := new(SyslogHistory)
	hist.Size = cfg.HistorySize
	hist.Level = SeverityLevelFrom(cfg.HistoryLevel)

	re := new(SyslogRemoteItems)
	for _, s := range req.Syslog.Spec.Servers {
		r := new(SyslogRemote)
		r.ForwardingFacility = "local7"
		r.Host = s.Address
		r.Port = s.Port
		r.Severity = SeverityLevelFrom(s.Severity)
		r.Transport = TransportUDP
		r.VrfName = s.VrfName
		re.RemoteDestList.Set(r)
	}

	fac := new(SyslogFacilityItems)
	if err := p.client.GetConfig(ctx, fac); err != nil && !errors.Is(err, gnmiext.ErrNil) {
		return err
	}

OUTER:
	for _, facility := range req.Syslog.Spec.Facilities {
		for _, f := range fac.FacilityList {
			if f.FacilityName == facility.Name {
				f.SeverityLevel = SeverityLevelFrom(facility.Severity)
				continue OUTER
			}
		}
		f := new(SyslogFacility)
		f.FacilityName = facility.Name
		f.SeverityLevel = SeverityLevelFrom(facility.Severity)
		fac.FacilityList.Set(f)
	}

	return p.Update(ctx, origin, srcIf, hist, re, fac)
}

func (p *Provider) DeleteSyslog(ctx context.Context) error {
	return p.client.Delete(
		ctx,
		new(SyslogOrigin),
		new(SyslogSrcIf),
		new(SyslogHistory),
		new(SyslogRemoteItems),
		new(SyslogFacilityItems),
	)
}

func (p *Provider) EnsureVLAN(ctx context.Context, req *provider.VLANRequest) error {
	v := new(VLAN)
	v.FabEncap = fmt.Sprintf("vlan-%d", req.VLAN.Spec.ID)
	v.AdminSt = BdStateActive
	v.BdState = BdStateActive
	if req.VLAN.Spec.AdminState == v1alpha1.AdminStateDown {
		v.BdState = BdStateInactive
	}
	if req.VLAN.Spec.Name != "" {
		v.Name = NewOption(req.VLAN.Spec.Name)
	}

	return p.Patch(ctx, v)
}

func (p *Provider) DeleteVLAN(ctx context.Context, req *provider.VLANRequest) error {
	v := new(VLAN)
	v.FabEncap = fmt.Sprintf("vlan-%d", req.VLAN.Spec.ID)
	return p.client.Delete(ctx, v)
}

func (p *Provider) GetVLANStatus(ctx context.Context, req *provider.VLANRequest) (provider.VLANStatus, error) {
	v := new(VLANOperItems)
	v.FabEncap = fmt.Sprintf("vlan-%d", req.VLAN.Spec.ID)
	if err := p.client.GetState(ctx, v); err != nil && !errors.Is(err, gnmiext.ErrNil) {
		return provider.VLANStatus{}, err
	}

	return provider.VLANStatus{
		OperStatus: v.OperSt == OperStUp,
	}, nil
}

func (p *Provider) EnsureVRF(ctx context.Context, req *provider.VRFRequest) error {
	v := new(VRF)
	v.Name = req.VRF.Spec.Name
	if req.VRF.Spec.Description != "" {
		v.Descr = NewOption(req.VRF.Spec.Description)
	}
	if req.VRF.Spec.VNI > 0 {
		v.L3Vni = true
		v.Encap = NewOption("vxlan-" + strconv.FormatUint(uint64(req.VRF.Spec.VNI), 10))
	}

	dom := new(VRFDom)
	dom.Name = req.VRF.Spec.Name
	v.DomItems.DomList.Set(dom)

	// pre: RD format has been already been validated by VRFCustomValidator
	if req.VRF.Spec.RouteDistinguisher != "" {
		tokens := strings.Split(req.VRF.Spec.RouteDistinguisher, ":")
		if strings.Contains(tokens[0], ".") {
			dom.Rd = "rd:ipv4-nn2:" + req.VRF.Spec.RouteDistinguisher
		} else {
			asn, err := strconv.ParseUint(tokens[0], 10, 32)
			if err != nil {
				return fmt.Errorf("invalid ASN in route distinguisher: %w", err)
			}
			dom.Rd = "rd:asn2-nn4:" + req.VRF.Spec.RouteDistinguisher
			if asn < math.MaxUint16 {
				dom.Rd = "rd:asn4-nn2:" + req.VRF.Spec.RouteDistinguisher
			}
		}
	}

	// configure route targets
	importEntryIPv4 := &RttEntry{Type: RttEntryTypeImport}
	exportEntryIPv4 := &RttEntry{Type: RttEntryTypeExport}
	importEntryIPv6 := &RttEntry{Type: RttEntryTypeImport}
	exportEntryIPv6 := &RttEntry{Type: RttEntryTypeExport}

	importEntryIPv4EVPN := &RttEntry{Type: RttEntryTypeImport}
	exportEntryIPv4EVPN := &RttEntry{Type: RttEntryTypeExport}
	importEntryIPv6EVPN := &RttEntry{Type: RttEntryTypeImport}
	exportEntryIPv6EVPN := &RttEntry{Type: RttEntryTypeExport}

	// route targets are already validated by VRFCustomValidator
	for _, rt := range req.VRF.Spec.RouteTargets {
		rttValue := "route-target:"
		tokens := strings.Split(rt.Value, ":")
		if strings.Contains(tokens[0], ".") {
			rttValue += "ipv4-nn2:" + rt.Value
		} else {
			asn, err := strconv.ParseUint(tokens[0], 10, 32)
			if err != nil {
				return fmt.Errorf("invalid ASN in route target: %w", err)
			}
			if asn > math.MaxUint16 {
				rttValue += "as4-nn2:" + rt.Value
			} else {
				nn, err := strconv.ParseUint(tokens[1], 10, 32)
				if err != nil {
					return fmt.Errorf("invalid number in route target: %w", err)
				}
				rttValue += "as2-nn2:" + rt.Value
				if nn > math.MaxUint16 {
					rttValue += "as2-nn4:" + rt.Value
				}
			}
		}
		rtt := Rtt{Rtt: rttValue}

		for _, af := range rt.AddressFamilies {
			switch af {
			case v1alpha1.IPv4:
				if rt.Action == v1alpha1.RouteTargetActionImport || rt.Action == v1alpha1.RouteTargetActionBoth {
					importEntryIPv4.EntItems.RttEntryList.Set(&rtt)
				}
				if rt.Action == v1alpha1.RouteTargetActionExport || rt.Action == v1alpha1.RouteTargetActionBoth {
					exportEntryIPv4.EntItems.RttEntryList.Set(&rtt)
				}
			case v1alpha1.IPv6:
				if rt.Action == v1alpha1.RouteTargetActionImport || rt.Action == v1alpha1.RouteTargetActionBoth {
					importEntryIPv6.EntItems.RttEntryList.Set(&rtt)
				}
				if rt.Action == v1alpha1.RouteTargetActionExport || rt.Action == v1alpha1.RouteTargetActionBoth {
					exportEntryIPv6.EntItems.RttEntryList.Set(&rtt)
				}
			case v1alpha1.IPv4EVPN:
				if rt.Action == v1alpha1.RouteTargetActionImport || rt.Action == v1alpha1.RouteTargetActionBoth {
					importEntryIPv4EVPN.EntItems.RttEntryList.Set(&rtt)
				}
				if rt.Action == v1alpha1.RouteTargetActionExport || rt.Action == v1alpha1.RouteTargetActionBoth {
					exportEntryIPv4EVPN.EntItems.RttEntryList.Set(&rtt)
				}
			case v1alpha1.IPv6EVPN:
				if rt.Action == v1alpha1.RouteTargetActionImport || rt.Action == v1alpha1.RouteTargetActionBoth {
					importEntryIPv6EVPN.EntItems.RttEntryList.Set(&rtt)
				}
				if rt.Action == v1alpha1.RouteTargetActionExport || rt.Action == v1alpha1.RouteTargetActionBoth {
					exportEntryIPv6EVPN.EntItems.RttEntryList.Set(&rtt)
				}
			default:
				return fmt.Errorf("unsupported address family for route target: %v", af)
			}
		}
	}

	if len(req.VRF.Spec.RouteTargets) > 0 {
		// Helper to add an AF with import/export entries
		addAF := func(afType1, afType2 AddressFamily, importE, exportE *RttEntry) {
			if importE.EntItems.RttEntryList.Len() == 0 && exportE.EntItems.RttEntryList.Len() == 0 {
				return
			}

			af := new(VRFDomAf)
			af.Type = afType1

			ctrl := new(VRFDomAfCtrl)
			ctrl.Type = afType2

			if importE.EntItems.RttEntryList.Len() > 0 {
				ctrl.RttpItems.RttPList.Set(importE)
			}
			if exportE.EntItems.RttEntryList.Len() > 0 {
				ctrl.RttpItems.RttPList.Set(exportE)
			}

			af.CtrlItems.AfCtrlList.Set(ctrl)
			dom.AfItems.DomAfList.Set(af)
		}

		addAF(AddressFamilyIPv4Unicast, AddressFamilyIPv4Unicast, importEntryIPv4, exportEntryIPv4)
		addAF(AddressFamilyIPv6Unicast, AddressFamilyIPv6Unicast, importEntryIPv6, exportEntryIPv6)
		addAF(AddressFamilyIPv4Unicast, AddressFamilyL2EVPN, importEntryIPv4EVPN, exportEntryIPv4EVPN)
		addAF(AddressFamilyIPv6Unicast, AddressFamilyL2EVPN, importEntryIPv6EVPN, exportEntryIPv6EVPN)
	}

	return p.Update(ctx, v)
}

func (p *Provider) DeleteVRF(ctx context.Context, req *provider.VRFRequest) error {
	v := new(VRF)
	v.Name = req.VRF.Spec.Name
	return p.client.Delete(ctx, v)
}

func (p *Provider) EnsureSystemSettings(ctx context.Context, s *nxv1alpha1.System) error {
	long := new(VLANSystem)
	long.LongName = s.Spec.VlanLongName

	res := new(VLANReservation)
	res.SysVlan = s.Spec.ReservedVlan

	sys := new(SystemJumboMTU)
	*sys = SystemJumboMTU(s.Spec.JumboMTU)

	return p.Patch(ctx, long, res, sys)
}

func (p *Provider) ResetSystemSettings(ctx context.Context) error {
	return p.client.Delete(
		ctx,
		new(VLANSystem),
		new(VLANReservation),
		new(SystemJumboMTU),
	)
}

// VPCDomainStatus represents the operational status of a vPC configuration on the device.
type VPCDomainStatus struct {
	// KeepAliveStatus indicates whether the keepalive link is operationally up (true) or down (false).
	KeepAliveStatus bool
	// KeepAliveStatusMsg provides additional human-readable information returned by the device
	KeepAliveStatusMsg []string
	// PeerStatus indicates whether the vPC peer is operationally up (true) or down (false).
	PeerStatus bool
	// PeerStatusMsg provides additional human-readable information about the vPC peer status.
	PeerStatusMsg []string
	// PeerUptime indicates the uptime of the vPC peer link in human-readable format provided by Cisco.
	PeerUptime time.Duration
	// Role represents the role of the vPC peer.
	Role nxv1alpha1.VPCDomainRole
}

// EnsureVPCDomain applies the vPC configuration on the device. It also ensures that the vPC feature
// is enabled on the device.
// `vrf` is a resource referencing the VRF to use in the keep-alive link configuration, can be nil.
// `pc` is a resource referencing a port-channel interface to use as vPC peer-link, must not be nil.
func (p *Provider) EnsureVPCDomain(ctx context.Context, vpcdomain *nxv1alpha1.VPCDomain, vrf *v1alpha1.VRF, pc *v1alpha1.Interface) (reterr error) {
	f := new(Feature)
	f.Name = "vpc"
	f.AdminSt = AdminStEnabled

	v := new(VPCDomain)
	v.ID = vpcdomain.Spec.DomainID
	v.RolePrio = vpcdomain.Spec.RolePriority
	v.SysPrio = vpcdomain.Spec.SystemPriority
	v.DelayRestoreSVI = vpcdomain.Spec.DelayRestoreSVI
	v.DelayRestoreVPC = vpcdomain.Spec.DelayRestoreVPC

	v.AdminSt = AdminStEnabled
	if vpcdomain.Spec.AdminState == v1alpha1.AdminStateDown {
		v.AdminSt = AdminStDisabled
	}

	v.FastConvergence = AdminStDisabled
	if vpcdomain.Spec.FastConvergence.Enabled {
		v.FastConvergence = AdminStEnabled
	}

	v.PeerSwitch = AdminStDisabled
	if vpcdomain.Spec.Peer.Switch.Enabled {
		v.PeerSwitch = AdminStEnabled
	}

	v.PeerGateway = AdminStDisabled
	if vpcdomain.Spec.Peer.Gateway.Enabled {
		v.PeerGateway = AdminStEnabled
	}

	v.L3PeerRouter = AdminStDisabled
	if vpcdomain.Spec.Peer.L3Router.Enabled {
		v.L3PeerRouter = AdminStEnabled
	}

	v.AutoRecovery = AdminStDisabled
	v.AutoRecoveryReloadDelay = 240
	if vpcdomain.Spec.Peer.AutoRecovery != nil && vpcdomain.Spec.Peer.AutoRecovery.Enabled {
		v.AutoRecovery = AdminStEnabled
		v.AutoRecoveryReloadDelay = vpcdomain.Spec.Peer.AutoRecovery.ReloadDelay
	}

	v.KeepAliveItems.DestIP = vpcdomain.Spec.Peer.KeepAlive.Destination
	v.KeepAliveItems.SrcIP = vpcdomain.Spec.Peer.KeepAlive.Source

	if vrf != nil {
		v.KeepAliveItems.VRF = vrf.Spec.Name
	}

	pcName, err := ShortNamePortChannel(pc.Spec.Name)
	if err != nil {
		return fmt.Errorf("vpc: failed to get short name for the port-channel interface %q: %w", pc.Spec.Name, err)
	}

	v.KeepAliveItems.PeerLinkItems.Id = pcName
	v.KeepAliveItems.PeerLinkItems.AdminSt = AdminStEnabled
	if vpcdomain.Spec.Peer.AdminState == v1alpha1.AdminStateDown {
		v.KeepAliveItems.PeerLinkItems.AdminSt = AdminStDisabled
	}

	return p.Patch(ctx, f, v)
}

func (p *Provider) DeleteVPCDomain(ctx context.Context) error {
	v := new(VPCDomain)
	return p.client.Delete(ctx, v)
}

// GetStatusVPCDomain retrieves the current status of the vPC configuration on the device.
func (p *Provider) GetStatusVPCDomain(ctx context.Context) (VPCDomainStatus, error) {
	vdOper := new(VPCDomainOper)
	if err := p.client.GetState(ctx, vdOper); err != nil && !errors.Is(err, gnmiext.ErrNil) {
		return VPCDomainStatus{}, err
	}

	vpcSt := VPCDomainStatus{}

	// Cisco returns a string composed of values coming from a bitmask, see:
	// https://pubhub.devnetcloud.com/media/dme-docs-10-4-3/docs/System/vpc%3AKeepalive/
	// Healthy links have these values "operational,peer-was-alive"
	vpcSt.KeepAliveStatus = false
	vpcSt.KeepAliveStatusMsg = strings.Split(vdOper.KeepAliveItems.OperSt, ",")
	if peerIsAlive(vdOper.KeepAliveItems.OperSt) {
		vpcSt.KeepAliveStatus = true
	}
	if vdOper.KeepAliveItems.PeerUpTime != "Peer is not alive" {
		if uptime, err := parsePeerUptime(vdOper.KeepAliveItems.PeerUpTime); err == nil {
			vpcSt.PeerUptime = *uptime
		}
	}

	vpcSt.PeerStatus = false
	vpcSt.PeerStatusMsg = strings.Split(vdOper.PeerStQual, ",")
	if vdOper.PeerStQual == "success" {
		vpcSt.PeerStatus = true
	}
	switch vdOper.Role {
	case vpcRoleElectionNotDone:
		vpcSt.Role = nxv1alpha1.VPCDomainRoleUnknown
	case vpcRolePrimary:
		vpcSt.Role = nxv1alpha1.VPCDomainRolePrimary
	case vpcRoleSecondary:
		vpcSt.Role = nxv1alpha1.VPCDomainRoleSecondary
	case vpcRolePrimaryOperationalSecondary:
		vpcSt.Role = nxv1alpha1.VPCDomainRolePrimaryOperationalSecondary
	case vpcRoleSecondaryOperationalPrimary:
		vpcSt.Role = nxv1alpha1.VPCDomainRoleSecondaryOperationalPrimary
	default:
		vpcSt.Role = nxv1alpha1.VPCDomainRoleUnknown
	}
	return vpcSt, nil
}

type BorderGatewaySettingsRequest struct {
	BorderGateway   *nxv1alpha1.BorderGateway
	SourceInterface *v1alpha1.Interface
	Interconnects   []BorderGatewayInterconnect
	Peers           []BorderGatewayPeer
}

type BorderGatewayInterconnect struct {
	Interface *v1alpha1.Interface
	Tracking  nxv1alpha1.InterconnectTrackingType
}

type BorderGatewayPeer struct {
	BGPPeer  *v1alpha1.BGPPeer
	PeerType nxv1alpha1.BGPPeerType
}

func (p *Provider) EnsureBorderGatewaySettings(ctx context.Context, req *BorderGatewaySettingsRequest) error {
	f := new(Feature)
	f.Name = "bgp"
	f.AdminSt = AdminStEnabled

	f2 := new(Feature)
	f2.Name = "ifvlan"
	f2.AdminSt = AdminStEnabled

	f3 := new(Feature)
	f3.Name = "vnsegment"
	f3.AdminSt = AdminStEnabled

	f4 := new(Feature)
	f4.Name = "evpn"
	f4.AdminSt = AdminStEnabled

	f5 := new(Feature)
	f5.Name = "nvo"
	f5.AdminSt = AdminStEnabled

	if err := p.Patch(ctx, f, f2, f3, f4, f5); err != nil {
		return err
	}

	conf := make([]gnmiext.Configurable, 0, 3)
	bg := new(MultisiteItems)
	bg.AdminSt = AdminStEnabled
	if req.BorderGateway.Spec.AdminState == v1alpha1.AdminStateDown {
		bg.AdminSt = AdminStDisabled
	}
	bg.SiteID = strconv.FormatInt(req.BorderGateway.Spec.MultisiteID, 10)
	bg.DelayRestoreSeconds = int64(math.Round(req.BorderGateway.Spec.DelayRestoreTime.Seconds()))
	if bg.DelayRestoreSeconds < 30 || bg.DelayRestoreSeconds > 1000 {
		return fmt.Errorf("border gateway: delay restore time %d seconds is out of range (30-1000)", bg.DelayRestoreSeconds)
	}
	conf = append(conf, bg)

	bgi := MultisiteBorderGatewayInterface(req.SourceInterface.Spec.Name)
	conf = append(conf, &bgi)

	sc := new(StormControlItems)
	for _, cfg := range req.BorderGateway.Spec.StormControl {
		ctrl := new(StormControlItem)
		f, err := strconv.ParseFloat(cfg.Level, 64)
		if err != nil {
			return fmt.Errorf("border gateway: invalid storm control level %q: %w", cfg.Level, err)
		}
		ctrl.Floatlevel = strconv.FormatFloat(f, 'f', 6, 64)
		switch cfg.Traffic {
		case nxv1alpha1.TrafficTypeBroadcast:
			ctrl.Name = StormControlTypeBroadcast
		case nxv1alpha1.TrafficTypeMulticast:
			ctrl.Name = StormControlTypeMulticast
		case nxv1alpha1.TrafficTypeUnicast:
			ctrl.Name = StormControlTypeUnicast
		default:
			return fmt.Errorf("border gateway: unsupported storm control traffic type %q", cfg.Traffic)
		}
		sc.EvpnStormControlList.Set(ctrl)
	}

	del := make([]gnmiext.Configurable, 0, 1)
	if sc.EvpnStormControlList.Len() == 0 {
		del = append(del, sc)
	} else {
		conf = append(conf, sc)
	}

	peerItems := new(MultisitePeerItems)
	trackingItems := new(MultisiteIfTrackingItems)
	if err := p.client.GetConfig(ctx, trackingItems, peerItems); err != nil && !errors.Is(err, gnmiext.ErrNil) {
		return err
	}

	interconnects := make(map[string]BorderGatewayInterconnect, len(req.Interconnects))
	for _, ic := range req.Interconnects {
		name, err := ShortName(ic.Interface.Spec.Name)
		if err != nil {
			return err
		}
		interconnects[name] = ic
	}

	for _, intf := range trackingItems.PhysIfList {
		ic, ok := interconnects[intf.ID]
		if !ok {
			if intf.MultisiteIfTracking != nil {
				del = append(del, intf.MultisiteIfTracking)
			}
			continue
		}

		if intf.MultisiteIfTracking == nil {
			intf.MultisiteIfTracking = new(MultisiteIfTracking)
		}
		intf.MultisiteIfTracking.IfName = intf.ID
		intf.MultisiteIfTracking.Tracking = MultisiteIfTrackingModeFrom(ic.Tracking)
		conf = append(conf, intf.MultisiteIfTracking)
	}

	for _, peer := range peerItems.PeerList {
		idx := slices.IndexFunc(req.Peers, func(p BorderGatewayPeer) bool {
			return p.BGPPeer.Spec.Address == peer.Addr
		})
		if idx == -1 {
			if peer.PeerType != "" {
				del = append(del, &MultisitePeer{Addr: peer.Addr})
			}
			continue
		}

		conf = append(conf, &MultisitePeer{Addr: peer.Addr, PeerType: BorderGatewayPeerTypeFrom(req.Peers[idx].PeerType)})
	}

	if err := p.client.Delete(ctx, del...); err != nil {
		return err
	}

	return p.Update(ctx, conf...)
}

func (p *Provider) ResetBorderGatewaySettings(ctx context.Context) error {
	conf := []gnmiext.Configurable{new(MultisiteItems), new(MultisiteBorderGatewayInterface), new(StormControlItems)}
	peerItems := new(MultisitePeerItems)
	trackingItems := new(MultisiteIfTrackingItems)
	if err := p.client.GetConfig(ctx, trackingItems, peerItems); err != nil && !errors.Is(err, gnmiext.ErrNil) {
		return err
	}
	for _, intf := range trackingItems.PhysIfList {
		if intf.MultisiteIfTracking != nil {
			conf = append(conf, intf.MultisiteIfTracking)
		}
	}
	for _, peer := range peerItems.PeerList {
		if peer.PeerType != "" {
			conf = append(conf, &MultisitePeer{Addr: peer.Addr})
		}
	}
	return p.client.Delete(ctx, conf...)
}

// EnsureNVE ensures that the NVE configuration on the device matches the desired state specified in the NVE custom resource.
// If no provider config is provided then the provider will use default settings.
func (p *Provider) EnsureNVE(ctx context.Context, req *provider.NVERequest) error {
	f1 := new(Feature)
	f1.Name = "evpn"
	f1.AdminSt = AdminStEnabled

	f2 := new(Feature)
	f2.Name = "nvo"
	f2.AdminSt = AdminStEnabled

	if err := p.Patch(ctx, f1, f2); err != nil {
		return err
	}

	if req.AnycastSourceInterface != nil && req.AnycastSourceInterface.Spec.Name == req.SourceInterface.Spec.Name {
		return errors.New("nve: anycast source interface cannot be the same as source interface")
	}

	n := new(NVE)
	n.AdminSt = AdminStDisabled
	if req.NVE.Spec.AdminState == v1alpha1.AdminStateUp {
		n.AdminSt = AdminStEnabled
	}
	n.SourceInterface = req.SourceInterface.Spec.Name

	if req.AnycastSourceInterface != nil {
		n.AnycastInterface = NewOption(req.AnycastSourceInterface.Spec.Name)
	}
	if req.NVE.Spec.MulticastGroups != nil && req.NVE.Spec.MulticastGroups.L2 != "" {
		n.McastGroupL2 = NewOption(req.NVE.Spec.MulticastGroups.L2)
	}
	if req.NVE.Spec.MulticastGroups != nil && req.NVE.Spec.MulticastGroups.L3 != "" {
		n.McastGroupL3 = NewOption(req.NVE.Spec.MulticastGroups.L3)
	}

	n.SuppressARP = req.NVE.Spec.SuppressARP

	switch req.NVE.Spec.HostReachability {
	case v1alpha1.HostReachabilityTypeBGP:
		n.HostReach = HostReachBGP
	case v1alpha1.HostReachabilityTypeFloodAndLearn:
		n.HostReach = HostReachFloodAndLearn
	default:
		return fmt.Errorf("invalid evpn host reachability type %q", req.NVE.Spec.HostReachability)
	}

	// defaults in this provider
	n.AdvertiseVmac = false
	n.HoldDownTime = 180

	vc := new(nxv1alpha1.NetworkVirtualizationEdgeConfig)
	if req.ProviderConfig != nil {
		if err := req.ProviderConfig.Into(vc); err != nil {
			return fmt.Errorf("failed to decode provider config: %w", err)
		}
		n.HoldDownTime = uint16(vc.Spec.HoldDownTime) // #nosec G115 -- kubebuilder validation
		n.AdvertiseVmac = vc.Spec.AdvertiseVirtualMAC
	}

	conf := make([]gnmiext.Configurable, 0, 3)
	conf = append(conf, n)

	iv := new(NVEInfraVLANs)
	for _, ivList := range vc.Spec.InfraVLANs {
		if ivList.ID != 0 {
			iv.InfraVLANList = append(iv.InfraVLANList, &NVEInfraVLAN{ID: uint32(ivList.ID)}) // #nosec G115 -- kubebuilder validation
			continue
		}
		for i := ivList.RangeMin; i <= ivList.RangeMax; i++ {
			iv.InfraVLANList = append(iv.InfraVLANList, &NVEInfraVLAN{ID: uint32(i)}) // #nosec G115 -- kubebuilder validation
		}
	}

	if len(iv.InfraVLANList) == 0 {
		if err := p.client.GetConfig(ctx, iv); err != nil && !errors.Is(err, gnmiext.ErrNil) {
			return err
		}
		if len(iv.InfraVLANList) != 0 {
			if err := p.client.Delete(ctx, iv); err != nil {
				return err
			}
		}
	} else {
		conf = append(conf, iv)
	}

	ag := new(FabricFwd)
	if req.NVE.Spec.AnycastGateway != nil {
		ag.AdminSt = string(AdminStEnabled)
		ag.Address = req.NVE.Spec.AnycastGateway.VirtualMAC
	}
	conf = append(conf, ag)

	return p.Patch(ctx, conf...)
}

func (p *Provider) DeleteNVE(ctx context.Context, req *provider.NVERequest) error {
	v := new(NVE)
	iv := new(NVEInfraVLANs)
	av := new(FabricFwd)
	return p.client.Delete(ctx, v, iv, av)
}

// GetNVEStatus retrieves the operational status of the NVE configuration on the device.
func (p *Provider) GetNVEStatus(ctx context.Context, req *provider.NVERequest) (provider.NVEStatus, error) {
	s := provider.NVEStatus{}

	op := new(NVEOper)
	if err := p.client.GetState(ctx, op); err != nil && !errors.Is(err, gnmiext.ErrNil) {
		return provider.NVEStatus{}, err
	}
	s.OperStatus = op.OperSt == OperStUp

	n := new(NVE)
	if err := p.client.GetConfig(ctx, n); err != nil && !errors.Is(err, gnmiext.ErrNil) {
		return provider.NVEStatus{}, err
	}
	s.SourceInterfaceName = n.SourceInterface
	if n.AnycastInterface.Value != nil {
		s.AnycastSourceInterfaceName = *n.AnycastInterface.Value
	}
	switch n.HostReach {
	case HostReachBGP:
		s.HostReachabilityType = "BGP"
	case HostReachFloodAndLearn:
		s.HostReachabilityType = "FloodAndLearn"
	case HostReachController:
		s.HostReachabilityType = "Controller"
	case HostReachOpenFlow:
		s.HostReachabilityType = "OpenFlow"
	case HostReachOpenFlowIR:
		s.HostReachabilityType = "OpenFlowIR"
	default:
		// unknown type, return as empty
	}
	return s, nil
}

func (p *Provider) Patch(ctx context.Context, conf ...gnmiext.Configurable) error {
	if NXVersion(p.client.Capabilities()) > VersionNX10_6_2 {
		return p.client.Patch(ctx, conf...)
	}
	fa, conf := separateFeatureActivation(conf)
	if err := p.client.Patch(ctx, fa...); err != nil {
		return err
	}
	return p.client.Patch(ctx, conf...)
}

func (p *Provider) Update(ctx context.Context, conf ...gnmiext.Configurable) error {
	if NXVersion(p.client.Capabilities()) > VersionNX10_6_2 {
		return p.client.Update(ctx, conf...)
	}
	fa, conf := separateFeatureActivation(conf)
	if err := p.client.Update(ctx, fa...); err != nil {
		return err
	}
	return p.client.Update(ctx, conf...)
}

// separateFeatureActivation separates feature activation configurations from other configurations.
// This is necessary for NX-OS versions <= 10.6(2) where feature activation must be performed before applying configurations.
// For more details, see: https://github.com/ironcore-dev/network-operator/issues/148
func separateFeatureActivation(conf []gnmiext.Configurable) (features, others []gnmiext.Configurable) {
	n := 0
	fa := make([]gnmiext.Configurable, 0, len(conf))
	for _, c := range conf {
		if f, ok := c.(*Feature); ok {
			fa = append(fa, f)
			continue
		}
		conf[n] = c
		n++
	}
	return fa, conf[:n:n]
}

func init() {
	provider.Register("cisco-nxos-gnmi", NewProvider)
}
