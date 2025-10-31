// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"cmp"
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	"maps"
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

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/deviceutil"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"
)

var (
	_ provider.Provider                 = (*Provider)(nil)
	_ provider.DeviceProvider           = (*Provider)(nil)
	_ provider.ACLProvider              = (*Provider)(nil)
	_ provider.BannerProvider           = (*Provider)(nil)
	_ provider.CertificateProvider      = (*Provider)(nil)
	_ provider.DNSProvider              = (*Provider)(nil)
	_ provider.InterfaceProvider        = (*Provider)(nil)
	_ provider.ISISProvider             = (*Provider)(nil)
	_ provider.ManagementAccessProvider = (*Provider)(nil)
	_ provider.NTPProvider              = (*Provider)(nil)
	_ provider.PIMProvider              = (*Provider)(nil)
	_ provider.SNMPProvider             = (*Provider)(nil)
	_ provider.SyslogProvider           = (*Provider)(nil)
	_ provider.UserProvider             = (*Provider)(nil)
	_ provider.VRFProvider              = (*Provider)(nil)
)

type Provider struct {
	conn   *grpc.ClientConn
	client *gnmiext.Client
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
				speeds = append(speeds, int32(n))
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
		a.SeqItems.ACEList = append(a.SeqItems.ACEList, &ACLEntry{
			SeqNum:          entry.Sequence,
			Action:          action,
			Protocol:        ProtocolFrom(entry.Protocol),
			SrcPrefix:       entry.SourceAddress.Addr().String(),
			SrcPrefixLength: entry.SourceAddress.Bits(),
			DstPrefix:       entry.DestinationAddress.Addr().String(),
			DstPrefixLength: entry.DestinationAddress.Bits(),
		})
	}

	// Ensure a consistent ordering of ACL entries to avoid unnecessary updates
	slices.SortFunc(a.SeqItems.ACEList, func(i, j *ACLEntry) int {
		return cmp.Compare(j.SeqNum, i.SeqNum)
	})

	return p.client.Update(ctx, a)
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

func (p *Provider) EnsureBanner(ctx context.Context, req *provider.BannerRequest) (reterr error) {
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

	b := new(Banner)
	b.Delimiter = "^"
	b.Message = req.Message

	return p.client.Patch(ctx, b)
}

func (p *Provider) DeleteBanner(ctx context.Context) error {
	b := new(Banner)
	return p.client.Delete(ctx, b)
}

type BGPRequest struct {
	// The Autonomous System Number of the BGP instance.
	AsNumber int32
	// The Router Identifier of the BGP instance, must be an IPv4 address.
	RouterID netip.Addr
	// AddressFamilies is a list of address families configured for the BGP instance.
	AddressFamilies []v1alpha1.AddressFamily
	// Optional L2EVPN configuration.
	L2EVPN *L2EVPN
}

type L2EVPN struct {
	// Forward packets over multipath paths
	MaximumPaths uint8
	// Retain the routes based on Target VPN Extended Communities.
	// Can be "all" to retain all routes, or a specific route-map name.
	RetainRouteTarget string
}

func (p *Provider) EnsureBGP(ctx context.Context, req *BGPRequest) (reterr error) {
	if !req.RouterID.Is4() {
		return fmt.Errorf("bgp: router ID must be an IPv4 address, got %q", req.RouterID)
	}

	// TODO: support ASNs like '65000.100', ideally with a custom type
	if req.AsNumber <= 0 || req.AsNumber > 65535 {
		return fmt.Errorf("bgp: asn %d is out of range (1-65535)", req.AsNumber)
	}

	f := new(Feature)
	f.Name = "bgp"
	f.AdminSt = AdminStEnabled

	f2 := new(Feature)
	f2.Name = "evpn"
	f2.AdminSt = AdminStEnabled

	b := new(BGP)
	b.AdminSt = AdminStEnabled
	b.Asn = strconv.Itoa(int(req.AsNumber))

	dom := new(BGPDom)
	dom.Name = DefaultVRFName
	dom.RtrID = req.RouterID.String()
	dom.RtrIDAuto = AdminStDisabled

	for _, af := range req.AddressFamilies {
		item := new(BGPDomAfItem)
		switch af {
		case v1alpha1.AddressFamilyIPv4Unicast:
			item.Type = AddressFamilyIPv4Unicast
		case v1alpha1.AddressFamilyIPv6Unicast:
			item.Type = AddressFamilyIPv6Unicast
		// case v1alpha1.AddressFamilyL2VPNEvpn:
		case "L2EVPN":
			item.Type = AddressFamilyL2EVPN
			item.MaxExtEcmp = 1
			if req.L2EVPN != nil {
				item.MaxExtEcmp = req.L2EVPN.MaximumPaths
				item.RetainRttAll = AdminStDisabled
				item.RetainRttRtMap = req.L2EVPN.RetainRouteTarget
				if req.L2EVPN.RetainRouteTarget == "all" {
					item.RetainRttAll = AdminStEnabled
					item.RetainRttRtMap = "DME_UNSET_PROPERTY_MARKER"
				}
			}
		default:
			return fmt.Errorf("bgp: unsupported address family %q", af)
		}
		dom.AfItems.DomAfList = append(dom.AfItems.DomAfList, item)
	}
	b.DomItems.DomList = append(b.DomItems.DomList, dom)

	return p.client.Update(ctx, f, f2, b)
}

type BGPPeerRequest struct {
	// The BGP Peer's address.
	Addr netip.Addr
	// Neighbor specific description.
	Desc string
	// The Autonomous System Number of the Neighbor
	AsNumber int32
	// The local source interface for the BGP session and update messages.
	SrcIf string
	// AddressFamilies is a list of address families configured for the BGP peer.
	AddressFamilies []v1alpha1.AddressFamily
	// Optional L2EVPN configuration.
	L2EVPN *PeerL2EVPN
}

type PeerL2EVPN struct {
	// SendStandardCommunity indicates whether to send the standard community attribute.
	SendStandardCommunity bool
	// SendExtendedCommunity indicates whether to send the extended community attribute.
	SendExtendedCommunity bool
	// RouteReflectorClient indicates whether to configure this peer as a route reflector client.
	RouteReflectorClient bool
}

func (p *Provider) EnsureBGPPeer(ctx context.Context, req *BGPPeerRequest) error {
	if !req.Addr.IsValid() {
		return fmt.Errorf("bgp peer: neighbor address %q is not a valid IP address", req.Addr)
	}

	// TODO: support ASNs like '65000.100', ideally with a custom type
	if req.AsNumber <= 0 || req.AsNumber > 65535 {
		return fmt.Errorf("bgp peer: asn %d is out of range (1-65535)", req.AsNumber)
	}

	if req.SrcIf == "" {
		return errors.New("bgp peer: source interface cannot be empty")
	}

	// Ensure that the BGP instance exists and is configured on the "default" domain
	// and return an error if it does not exist.
	// Otherwise, by default of the gnmi specification, all missing nodes in the yang
	// tree would be created, which would mean that we would create a new BGP instance,
	// which is not what we want.
	// Returning an error here allows us to handle the case where the BGP instance is not
	// configured by requeuing the request for the BGP Peer on the k8s controller. This avoids
	// a race condition where the BGP instance is created after the BGP Peer is created.
	bgp := new(BGPDom)
	bgp.Name = DefaultVRFName
	if err := p.client.GetConfig(ctx, bgp); err != nil {
		return fmt.Errorf("bgp peer: failed to get bgp instance 'default': %w", err)
	}

	srcIf, err := ShortNameLoopback(req.SrcIf)
	if err != nil {
		return fmt.Errorf("bgp peer: invalid source interface name %q: %w", req.SrcIf, err)
	}

	pe := new(BGPPeer)
	pe.Addr = req.Addr.String()
	pe.Asn = strconv.Itoa(int(req.AsNumber))
	pe.AsnType = PeerAsnTypeNone
	pe.Name = req.Desc
	pe.SrcIf = srcIf

	for _, af := range req.AddressFamilies {
		item := new(BGPPeerAfItem)
		switch af {
		case v1alpha1.AddressFamilyIPv4Unicast:
			item.Type = AddressFamilyIPv4Unicast
		case v1alpha1.AddressFamilyIPv6Unicast:
			item.Type = AddressFamilyIPv6Unicast
		// case v1alpha1.AddressFamilyL2VPNEvpn:
		case "L2EVPN":
			item.Type = AddressFamilyL2EVPN
			if req.L2EVPN != nil {
				item.SendComStd = AdminStDisabled
				if req.L2EVPN.SendStandardCommunity {
					item.SendComStd = AdminStEnabled
				}
				item.SendComExt = AdminStDisabled
				if req.L2EVPN.SendStandardCommunity {
					item.SendComExt = AdminStEnabled
				}
				if req.L2EVPN.RouteReflectorClient {
					item.Ctrl = NewOption(RouteReflectorClient)
				}
			}
		default:
			return fmt.Errorf("bgp peer: unsupported address family %q", af)
		}
		pe.AfItems.PeerAfList = append(pe.AfItems.PeerAfList, item)
	}

	return p.client.Update(ctx, pe)
}

func (p *Provider) EnsureCertificate(ctx context.Context, req *provider.EnsureCertificateRequest) error {
	tp := new(Trustpoint)
	tp.Name = req.ID

	if err := p.client.Patch(ctx, tp); err != nil {
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
	pf := new(DNSProf)
	pf.Name = DefaultVRFName
	pf.DomItems.Name = req.DNS.Spec.Domain
	vrfs := map[string]*DNSVrf{}
	for _, s := range req.DNS.Spec.Servers {
		prov := new(DNSProv)
		prov.Addr = s.Address
		prov.SrcIf = req.DNS.Spec.SourceInterfaceName
		if s.VrfName == "" {
			pf.ProvItems.ProviderList = append(pf.ProvItems.ProviderList, prov)
			continue
		}
		vrf, ok := vrfs[s.VrfName]
		if !ok {
			vrf = new(DNSVrf)
			vrf.Name = s.VrfName
			vrfs[s.VrfName] = vrf
		}
		vrf.ProvItems.ProviderList = append(vrf.ProvItems.ProviderList, prov)
	}
	pf.VrfItems.VrfList = slices.Collect(maps.Values(vrfs))
	d.ProfItems.ProfList = append(d.ProfItems.ProfList, pf)

	// TODO: Ensure a consistent ordering of DNS providers to avoid unnecessary updates
	slices.SortFunc(pf.ProvItems.ProviderList, func(a, b *DNSProv) int {
		return strings.Compare(a.Addr, b.Addr)
	})
	for _, v := range pf.VrfItems.VrfList {
		slices.SortFunc(v.ProvItems.ProviderList, func(a, b *DNSProv) int {
			return strings.Compare(a.Addr, b.Addr)
		})
	}

	return p.client.Update(ctx, d)
}

func (p *Provider) DeleteDNS(ctx context.Context) error {
	d := new(DNS)
	return p.client.Delete(ctx, d)
}

func (p *Provider) EnsureInterface(ctx context.Context, req *provider.InterfaceRequest) error {
	name, err := ShortName(req.Interface.Spec.Name)
	if err != nil {
		return err
	}

	addr := new(AddrItem)
	addr.ID = name

	var prefixes []netip.Prefix
	switch v := req.IPv4.(type) {
	case provider.IPv4AddressList:
		for i, p := range v {
			prefixes = append(prefixes, p)
			nth := IntfAddrTypePrimary
			if i > 0 {
				nth = IntfAddrTypeSecondary
			}
			ip := &IntfAddr{
				Addr: p.String(),
				Type: nth,
			}
			if p.Addr().Is6() {
				return fmt.Errorf("invalid ipv4 address %q: not an ipv4 address", p.String())
			}
			addr.AddrItems.AddrList = append(addr.AddrItems.AddrList, ip)
		}

	case provider.IPv4Unnumbered:
		addr.Unnumbered, err = ShortName(v.SourceInterface)
		if err != nil {
			return fmt.Errorf("invalid unnumbered source interface name %q: %w", v.SourceInterface, err)
		}
	}

	for i, p := range prefixes {
		for j := i + 1; j < len(prefixes); j++ {
			if p.Overlaps(prefixes[j]) {
				return fmt.Errorf("overlapping IP prefixes: %s and %s", p, prefixes[j])
			}
		}
	}

	if req.Interface.Spec.Type != v1alpha1.InterfaceTypeAggregate && req.IPv4 == nil {
		// Ensure to delete any leftover IPv4 addresses if the spec does not contain any.
		if err := p.client.Delete(ctx, addr); err != nil {
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
		if req.Interface.Spec.MTU != 0 {
			p.MTU = req.Interface.Spec.MTU
			p.UserCfgdFlags = "admin_mtu," + p.UserCfgdFlags
		}
		if req.IPv4 != nil {
			p.Layer = Layer3
			p.UserCfgdFlags = "admin_layer," + p.UserCfgdFlags
		}
		if addr.Unnumbered != "" {
			p.Medium = MediumPointToPoint
		}
		p.RtvrfMbrItems = NewVrfMember(name, DefaultVRFName)

		if req.Interface.Spec.Switchport != nil {
			p.RtvrfMbrItems = nil
			switch req.Interface.Spec.Switchport.Mode {
			case v1alpha1.SwitchportModeAccess:
				p.Mode = SwitchportModeAccess
				p.AccessVlan = fmt.Sprintf("vlan-%d", req.Interface.Spec.Switchport.AccessVlan)
			case v1alpha1.SwitchportModeTrunk:
				p.Mode = SwitchportModeTrunk
				p.NativeVlan = fmt.Sprintf("vlan-%d", req.Interface.Spec.Switchport.NativeVlan)
				if len(req.Interface.Spec.Switchport.AllowedVlans) > 0 {
					p.TrunkVlans = Range(req.Interface.Spec.Switchport.AllowedVlans)
				}
			default:
				return fmt.Errorf("invalid switchport mode: %s", req.Interface.Spec.Switchport.Mode)
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
		lb.RtvrfMbrItems = NewVrfMember(name, DefaultVRFName)
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
		pc.UserCfgdFlags = "admin_state"

		pc.MTU = DefaultMTU
		if req.Interface.Spec.MTU != 0 {
			pc.MTU = req.Interface.Spec.MTU
			pc.UserCfgdFlags = "admin_mtu," + pc.UserCfgdFlags
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
				pc.NativeVlan = fmt.Sprintf("vlan-%d", req.Interface.Spec.Switchport.NativeVlan)
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
			pc.RsmbrIfsItems.RsMbrIfsList = append(pc.RsmbrIfsItems.RsMbrIfsList, NewPortChannelMember(n))
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

		if req.MultiChassisID != nil {
			v := new(VPCIf)
			v.ID = int(*req.MultiChassisID)
			v.SetPortChannel(name)
			conf = append(conf, v)
		}

		conf = append(conf, pc)

	default:
		return fmt.Errorf("unsupported interface type: %s", req.Interface.Spec.Type)
	}

	// Add the address items last, as they depend on the interface being created first.
	if req.IPv4 != nil {
		slices.SortFunc(addr.AddrItems.AddrList, func(a, b *IntfAddr) int {
			return strings.Compare(b.Addr, a.Addr)
		})
		conf = append(conf, addr)
	}

	return p.client.Update(ctx, conf...)
}

func (p *Provider) DeleteInterface(ctx context.Context, req *provider.InterfaceRequest) error {
	name, err := ShortName(req.Interface.Spec.Name)
	if err != nil {
		return err
	}

	conf := make([]gnmiext.Configurable, 0, 3)
	if req.Interface.Spec.Type != v1alpha1.InterfaceTypeAggregate {
		addr := new(AddrItem)
		addr.ID = name

		conf = append(conf, addr)
	}

	switch req.Interface.Spec.Type {
	case v1alpha1.InterfaceTypePhysical:
		p := new(PhysIf)
		p.ID = name
		conf = append(conf, p)

		stp := new(SpanningTree)
		stp.IfName = name
		conf = append(conf, stp)

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

	var operSt OperSt
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

	default:
		return provider.InterfaceStatus{}, fmt.Errorf("unsupported interface type: %s", req.Interface.Spec.Type)
	}

	return provider.InterfaceStatus{
		OperStatus: operSt == OperStUp,
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

	conf := []gnmiext.Configurable{f}

	if slices.ContainsFunc(req.Interfaces, func(intf provider.ISISInterface) bool {
		return intf.BFD
	}) {
		f := new(Feature)
		f.Name = "bfd"
		f.AdminSt = AdminStEnabled
		conf = append(conf, f)
	}

	i := new(ISIS)
	i.AdminSt = AdminStEnabled
	i.Name = req.ISIS.Spec.Instance

	dom := new(ISISDom)
	dom.Name = DefaultVRFName
	dom.Net = req.ISIS.Spec.NetworkEntityTitle
	dom.IsType = ISISLevelFrom(req.ISIS.Spec.Type)
	dom.PassiveDflt = dom.IsType
	i.DomItems.DomList = append(i.DomItems.DomList, dom)

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
		dom.AfItems.DomAfList = append(dom.AfItems.DomAfList, item)
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

	for i, iface := range req.Interfaces {
		intf := new(ISISInterface)
		intf.ID = interfaceNames[i]
		intf.NetworkTypeP2P = AdminStOff
		if iface.Interface.Spec.Type == v1alpha1.InterfaceTypePhysical {
			intf.NetworkTypeP2P = AdminStOn
		}
		if ipv4 {
			intf.V4Enable = true
			intf.V4Bfd = "inheritVrf"
			if iface.BFD {
				intf.V4Bfd = "enabled"
			}
		}
		if ipv6 {
			intf.V6Enable = true
			intf.V6Bfd = "inheritVrf"
			if iface.BFD {
				intf.V6Bfd = "enabled"
			}
		}
		dom.IfItems.IfList = append(dom.IfItems.IfList, intf)
	}
	conf = append(conf, i)

	// TODO: Ensure a consistent ordering of NTP providers to avoid unnecessary updates
	slices.SortFunc(dom.AfItems.DomAfList, func(a, b *ISISDomAf) int {
		return strings.Compare(string(b.Type), string(a.Type))
	})
	slices.SortFunc(dom.IfItems.IfList, func(a, b *ISISInterface) int {
		// Loopback interfaces are ordered ascending, physical interfaces descending
		if a.ID[:2] == "lo" && b.ID[:2] == "lo" {
			return cmp.Compare(a.ID, b.ID)
		}
		return cmp.Compare(b.ID, a.ID)
	})

	return p.client.Update(ctx, conf...)
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

	nf := new(Feature)
	nf.Name = "nxapi"
	nf.AdminSt = AdminStEnabled

	g := new(GRPC)
	g.Port = req.ManagementAccess.Spec.GRPC.Port
	g.UseVrf = req.ManagementAccess.Spec.GRPC.VrfName
	g.Cert = req.ManagementAccess.Spec.GRPC.CertificateID
	if err := g.Validate(); err != nil {
		return err
	}

	gn := new(GNMI)
	gn.MaxCalls = 16
	gn.KeepAliveTimeout = 600 // seconds
	if err := gn.Validate(); err != nil {
		return err
	}

	con := new(Console)
	con.Timeout = 10 // minutes
	if err := con.Validate(); err != nil {
		return err
	}

	vty := new(VTY)
	vty.SsLmtItems.SesLmt = 8
	vty.ExecTmeoutItems.Timeout = 10 // minutes
	if err := vty.Validate(); err != nil {
		return err
	}

	sysVlan := new(VLANSystem)
	sysVlan.LongName = true

	resVlan := new(VLANReservation)
	resVlan.SysVlan = 3850

	copp := new(CoPP)
	copp.Profile = Strict

	return p.client.Update(ctx, gf, nf, g, gn, con, vty, sysVlan, resVlan, copp)
}

func (p *Provider) DeleteManagementAccess(ctx context.Context) error {
	return p.client.Update(
		ctx,
		new(GRPC),
		new(GNMI),
		new(VLANSystem),
		new(VLANReservation),
		new(CoPP),
		new(Console),
		new(VTY),
	)
}

type NTPConfig struct {
	Log struct {
		Enable bool `json:"enable"`
	} `json:"log"`
}

func (p *Provider) EnsureNTP(ctx context.Context, req *provider.EnsureNTPRequest) error {
	var cfg NTPConfig
	if req.ProviderConfig != nil {
		if err := req.ProviderConfig.Into(&cfg); err != nil {
			return err
		}
	}

	n := new(NTP)
	n.AdminSt = AdminStEnabled
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
		n.ProvItems.NtpProviderList = append(n.ProvItems.NtpProviderList, prov)
	}
	n.SrcIfItems.SrcIf = req.NTP.Spec.SourceInterfaceName

	// TODO: Ensure a consistent ordering of NTP providers to avoid unnecessary updates
	slices.SortFunc(n.ProvItems.NtpProviderList, func(a, b *NTPProvider) int {
		return strings.Compare(a.Name, b.Name)
	})

	f := new(Feature)
	f.Name = "ntpd"
	f.AdminSt = AdminStEnabled

	return p.client.Update(ctx, f, n)
}

func (p *Provider) DeleteNTP(ctx context.Context) error {
	n := new(NTP)

	f := new(Feature)
	f.Name = "ntpd"

	return p.client.Delete(ctx, n, f)
}

type NVERequest struct {
	AdminSt              bool
	HostReach            HostReachType
	AdvertiseVirtualRmac *bool
	// the name of the loopback to use as source
	SourceInterface string
	// the name of the loopback to use for anycast
	AnycastInterface string
	SuppressARP      *bool
	// multicast group for L2 VTEP discovery
	McastL2 *netip.Addr
	// multicast group for L3 VTEP discovery
	McastL3      *netip.Addr
	HoldDownTime int16 // in seconds
}

func (p *Provider) EnsureNVE(ctx context.Context, req *NVERequest) error {
	f := new(Feature)
	f.Name = "nvo"
	f.AdminSt = AdminStEnabled

	f2 := new(Feature)
	f2.Name = "ngmvpn"
	f2.AdminSt = AdminStEnabled

	srcIf, err := ShortNameLoopback(req.SourceInterface)
	if err != nil {
		return err
	}

	anyIf, err := ShortNameLoopback(req.AnycastInterface)
	if err != nil {
		return err
	}

	nve := new(NVE)
	nve.ID = 1
	nve.AdminSt = AdminStDisabled
	if req.AdminSt {
		nve.AdminSt = AdminStEnabled
	}

	if srcIf == anyIf {
		return errors.New("nve: source and anycast interfaces must be different")
	}
	nve.SourceInterface = srcIf
	nve.AnycastInterface = anyIf

	if req.HostReach != HostReachBGP && req.HostReach != HostReachFloodAndLearn {
		return fmt.Errorf("nve: invalid host reach type %q", req.HostReach)
	}
	nve.HostReach = req.HostReach

	if req.AdvertiseVirtualRmac != nil {
		nve.AdvertiseVmac = *req.AdvertiseVirtualRmac
	}

	if req.SuppressARP != nil {
		nve.SuppressARP = *req.SuppressARP
	}

	if ip := req.McastL2; ip != nil {
		if !ip.Is4() || !ip.IsMulticast() {
			return fmt.Errorf("nve: invalid multicast IPv4 address: %s", ip)
		}
		nve.McastGroupL2 = ip.String()
	}

	if ip := req.McastL3; ip != nil {
		if !ip.Is4() || !ip.IsMulticast() {
			return fmt.Errorf("nve: invalid multicast IPv4 address: %s", ip)
		}
		nve.McastGroupL3 = ip.String()
	}

	if req.HoldDownTime != 0 {
		if req.HoldDownTime < 1 || req.HoldDownTime > 1500 {
			return fmt.Errorf("nve: hold down time %d is out of range (1-1500 seconds)", req.HoldDownTime)
		}
		nve.HoldDownTime = req.HoldDownTime
	}

	return p.client.Update(ctx, f, f2, nve)
}

type OSPFRouter struct {
	AdminSt bool
	// Name of the OSPF process, e.g., `UNDERLAY`
	Name string
	// RouterID is the router ID of the OSPF process, must be a valid IPv4 address and
	// must exist on a configured interface in the system.
	RouterID netip.Addr
	// PropagateDefaultRoute is equivalent to the CLI command `default-information originate`
	ProgateDefaultRoute bool
	// RedistributionConfigs is a list of redistribution configurations for the OSPF process.
	RedistributionConfigs []RedistributionConfig
	// LogLevel is the logging level for OSPF adjacency changes. By default "none"
	LogLevel AdjChangeLogLevel
	// Distance is the adminitrative distance value (1-255) for OSPF routes. Cisco's default is 110.
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

type OSPFInterfaceItem struct {
	Interface *v1alpha1.Interface
	// Area is the OSPF area for all interfaces, e.g., `0.0.0.0`.
	Area string
	// PassiveMode indicates the passive mode for the interface.
	PassiveMode *bool
}

type EnsureOSPFRequest struct {
	// Router is the OSPF router configuration.
	Router *OSPFRouter

	// Interfaces is a list of interfaces that should have PIM enabled.
	// If empty, PIM will not be enabled on any interfaces.
	Interfaces []*OSPFInterfaceItem
}

func (p *Provider) EnsureOSPF(ctx context.Context, req *EnsureOSPFRequest) error {
	f := new(Feature)
	f.Name = "ospf"
	f.AdminSt = AdminStEnabled

	o := new(OSPF)
	o.AdminSt = AdminStEnabled
	o.Name = req.Router.Name

	if !req.Router.RouterID.IsValid() || !req.Router.RouterID.Is4() {
		return fmt.Errorf("ospf: router ID %q is not a valid IPv4 address", req.Router.RouterID)
	}

	dom := new(OSPFDom)
	dom.Name = DefaultVRFName
	dom.AdjChangeLogLevel = AdjChangeLogLevelNone
	if req.Router.LogLevel != AdjChangeLogLevelNone {
		dom.AdjChangeLogLevel = req.Router.LogLevel
	}
	dom.AdminSt = AdminStEnabled
	dom.BwRef = DefaultBwRef // default 40 Gbps
	dom.BwRefUnit = BwRefUnitMbps
	if req.Router.ReferenceBandwidthMbps != 0 {
		if req.Router.ReferenceBandwidthMbps < 1 || req.Router.ReferenceBandwidthMbps > 999999 {
			return fmt.Errorf("ospf: reference bandwidth %d is out of range (1-999999 Mbps)", req.Router.ReferenceBandwidthMbps)
		}
		dom.BwRef = req.Router.ReferenceBandwidthMbps
	}
	dom.Dist = DefaultDist
	if req.Router.Distance != 0 {
		if req.Router.Distance < 1 || req.Router.Distance > 255 {
			return fmt.Errorf("ospf: distance %d is out of range (1-255)", req.Router.Distance)
		}
		dom.Dist = req.Router.Distance
	}
	dom.RtrID = req.Router.RouterID.String()
	o.DomItems.DomList = append(o.DomItems.DomList, dom)

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
		if !isValidOSPFArea(iface.Area) {
			return fmt.Errorf("ospf: area %q is not valid, must be a decimal number or dotted decimal format", iface.Area)
		}
		intf.NwT = NtwTypeUnspecified
		if iface.Interface.Spec.Type == v1alpha1.InterfaceTypePhysical {
			intf.NwT = NtwTypePointToPoint
		}
		intf.PassiveCtrl = PassiveControlUnspecified
		if iface.PassiveMode != nil {
			if *iface.PassiveMode {
				intf.PassiveCtrl = PassiveControlEnabled
			} else {
				intf.PassiveCtrl = PassiveControlDisabled
			}
		}
		dom.IfItems.IfList = append(dom.IfItems.IfList, intf)
	}

	for _, rc := range req.Router.RedistributionConfigs {
		if rc.RouteMapName == "" {
			return errors.New("ospf: redistribution route map name cannot be empty")
		}
		rd := new(InterLeakPList)
		rd.Proto = rc.Protocol
		rd.Asn = "none"
		rd.Inst = "none"
		rd.RtMap = rc.RouteMapName
		dom.InterleakItems.InterLeakPList = append(dom.InterleakItems.InterLeakPList, rd)
	}

	dom.DefrtleakItems.Always = "no"
	if req.Router.ProgateDefaultRoute {
		dom.DefrtleakItems.Always = "yes"
	}

	if req.Router.MaxLSA != 0 {
		dom.MaxlsapItems.Action = MaxLSAActionReject
		dom.MaxlsapItems.MaxLsa = req.Router.MaxLSA
	}

	return p.client.Update(ctx, f, o)
}

func (p *Provider) EnsurePIM(ctx context.Context, req *provider.EnsurePIMRequest) error {
	f := new(Feature)
	f.Name = "pim"
	f.AdminSt = AdminStEnabled

	rpItems := new(StaticRPItems)
	apItems := new(AnycastPeerItems)

	for _, rendezvousPoint := range req.PIM.Spec.RendezvousPoints {
		rp := new(StaticRP)
		rp.Addr = rendezvousPoint.Address
		for _, group := range rendezvousPoint.MulticastGroups {
			if !group.IsValid() || !group.Addr().Is4() {
				return fmt.Errorf("pim: group list %q is not a valid IPv4 address prefix", group)
			}
			grp := new(StaticRPGrp)
			grp.GrpListName = group.String()
			rp.RpgrplistItems.RPGrpListList = append(rp.RpgrplistItems.RPGrpListList, grp)
		}
		rpItems.StaticRPList = append(rpItems.StaticRPList, rp)

		for _, addr := range rendezvousPoint.AnycastAddresses {
			peer := new(AnycastPeerAddr)
			peer.Addr = rendezvousPoint.Address
			peer.RpSetAddr = addr
			apItems.AcastRPPeerList = append(apItems.AcastRPPeerList, peer)
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
		ifItems.IfList = append(ifItems.IfList, intf)
	}

	conf := make([]gnmiext.Configurable, 0, 4)
	conf = append(conf, f)

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

	if err := p.client.Update(ctx, conf...); err != nil {
		return err
	}

	return p.client.Delete(ctx, del...)
}

func (p *Provider) DeletePIM(ctx context.Context, _ *provider.DeletePIMRequest) error {
	return p.client.Delete(ctx, new(StaticRPItems), new(AnycastPeerItems), new(PIMIfItems))
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
		d.RoleItems.UserRoleList = append(d.RoleItems.UserRoleList, r)
	}
	u.UserdomainItems.UserDomainList = append(u.UserdomainItems.UserDomainList, d)

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

	return p.client.Patch(ctx, u)
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
		comm.CommAcess = "unspecified"
		comm.ACLItems.UseACLName = c.ACLName
		communities.CommSecPList = append(communities.CommSecPList, comm)
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
			host.UsevrfItems.UseVrfList = append(host.UsevrfItems.UseVrfList, vrf)
		}
		if h.Version == "v3" {
			host.SecLevel = SecLevelAuth
		}
		hosts.HostList = append(hosts.HostList, host)
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
		rv.Set(reflect.ValueOf(&SNMPTraps{Trapstatus: TrapstatusEnable}))
	}

	return p.client.Update(ctx, sysInfo, trapsSrcIf, informsSrcIf, communities, hosts, traps)
}

func (p *Provider) DeleteSNMP(ctx context.Context, req *provider.DeleteSNMPRequest) error {
	traps := new(SNMPTrapsItems)
	if err := p.client.Update(ctx, traps); err != nil {
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
		re.RemoteDestList = append(re.RemoteDestList, r)
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
		fac.FacilityList = append(fac.FacilityList, f)
	}

	return p.client.Update(ctx, origin, srcIf, hist, re, fac)
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

func (p *Provider) EnsureVRF(ctx context.Context, req *provider.VRFRequest) error {
	v := new(VRF)
	v.Name = req.VRF.Spec.Name
	if req.VRF.Spec.Description != "" {
		v.Descr = NewOption(req.VRF.Spec.Description)
	}

	if req.VRF.Spec.VNI > 0 {
		v.L3Vni = true
		v.Encap = "vxlan-" + strconv.FormatUint(uint64(req.VRF.Spec.VNI), 10)
	}
	var dom *VRFDom

	// pre: RD format has been already been validated by VRFCustomValidator
	if req.VRF.Spec.RouteDistinguisher != "" {
		dom = &VRFDom{Name: req.VRF.Spec.Name}
		tokens := strings.Split(req.VRF.Spec.RouteDistinguisher, ":")
		if strings.Contains(tokens[0], ".") {
			dom.Rd = "rd:ipv4-nn2:" + req.VRF.Spec.RouteDistinguisher
		} else {
			asn, err := strconv.ParseUint(tokens[0], 10, 32)
			if err != nil {
				return fmt.Errorf("invalid ASN in route distinguisher: %w", err)
			}
			dom.Rd = "rd:asn2-nn4:" + req.VRF.Spec.RouteDistinguisher
			if asn > math.MaxUint16 {
				dom.Rd = "rd:asn4-nn2:" + req.VRF.Spec.RouteDistinguisher
			}
		}
		v.DomItems = &VRFDomItems{DomList: []*VRFDom{dom}}
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

	importEntryIPv4.EntItems = new(RttEntItems)
	exportEntryIPv4.EntItems = new(RttEntItems)
	importEntryIPv6.EntItems = new(RttEntItems)
	exportEntryIPv6.EntItems = new(RttEntItems)

	importEntryIPv4EVPN.EntItems = new(RttEntItems)
	exportEntryIPv4EVPN.EntItems = new(RttEntItems)
	importEntryIPv6EVPN.EntItems = new(RttEntItems)
	exportEntryIPv6EVPN.EntItems = new(RttEntItems)

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
					importEntryIPv4.EntItems.RttEntryList = append(importEntryIPv4.EntItems.RttEntryList, &rtt)
				}
				if rt.Action == v1alpha1.RouteTargetActionExport || rt.Action == v1alpha1.RouteTargetActionBoth {
					exportEntryIPv4.EntItems.RttEntryList = append(exportEntryIPv4.EntItems.RttEntryList, &rtt)
				}
			case v1alpha1.IPv6:
				if rt.Action == v1alpha1.RouteTargetActionImport || rt.Action == v1alpha1.RouteTargetActionBoth {
					importEntryIPv6.EntItems.RttEntryList = append(importEntryIPv6.EntItems.RttEntryList, &rtt)
				}
				if rt.Action == v1alpha1.RouteTargetActionExport || rt.Action == v1alpha1.RouteTargetActionBoth {
					exportEntryIPv6.EntItems.RttEntryList = append(exportEntryIPv6.EntItems.RttEntryList, &rtt)
				}
			case v1alpha1.IPv4EVPN:
				if rt.Action == v1alpha1.RouteTargetActionImport || rt.Action == v1alpha1.RouteTargetActionBoth {
					importEntryIPv4EVPN.EntItems.RttEntryList = append(importEntryIPv4EVPN.EntItems.RttEntryList, &rtt)
				}
				if rt.Action == v1alpha1.RouteTargetActionExport || rt.Action == v1alpha1.RouteTargetActionBoth {
					exportEntryIPv4EVPN.EntItems.RttEntryList = append(exportEntryIPv4EVPN.EntItems.RttEntryList, &rtt)
				}
			case v1alpha1.IPv6EVPN:
				if rt.Action == v1alpha1.RouteTargetActionImport || rt.Action == v1alpha1.RouteTargetActionBoth {
					importEntryIPv6EVPN.EntItems.RttEntryList = append(importEntryIPv6EVPN.EntItems.RttEntryList, &rtt)
				}
				if rt.Action == v1alpha1.RouteTargetActionExport || rt.Action == v1alpha1.RouteTargetActionBoth {
					exportEntryIPv6EVPN.EntItems.RttEntryList = append(exportEntryIPv6EVPN.EntItems.RttEntryList, &rtt)
				}
			default:
				return fmt.Errorf("unsupported address family for route target: %v", af)
			}
		}
	}

	if len(req.VRF.Spec.RouteTargets) > 0 {
		if dom == nil {
			dom = &VRFDom{Name: req.VRF.Spec.Name}
		}
		dom.AfItems = &VRFDomAfItems{}

		// Helper to add an AF with import/export entries
		addAF := func(afType1, afType2 AddressFamily, importE, exportE *RttEntry) {
			if (importE == nil || len(importE.EntItems.RttEntryList) == 0) &&
				(exportE == nil || len(exportE.EntItems.RttEntryList) == 0) {
				return
			}
			af := &VRFDomAf{Type: afType1}
			af.CtrlItems = &VRFDomAfCtrlItems{}
			ctrl := &VRFDomAfCtrl{Type: afType2}
			ctrl.RttpItems = &VRFRttpItems{}

			if importE != nil && len(importE.EntItems.RttEntryList) > 0 {
				ctrl.RttpItems.RttPList = append(ctrl.RttpItems.RttPList, importE)
			}
			if exportE != nil && len(exportE.EntItems.RttEntryList) > 0 {
				ctrl.RttpItems.RttPList = append(ctrl.RttpItems.RttPList, exportE)
			}

			if len(ctrl.RttpItems.RttPList) == 0 {
				return
			}
			af.CtrlItems.AfCtrlList = append(af.CtrlItems.AfCtrlList, ctrl)
			dom.AfItems.DomAfList = append(dom.AfItems.DomAfList, af)
		}

		addAF(AddressFamilyIPv4Unicast, AddressFamilyIPv4Unicast, importEntryIPv4, exportEntryIPv4)
		addAF(AddressFamilyIPv6Unicast, AddressFamilyIPv6Unicast, importEntryIPv6, exportEntryIPv6)
		addAF(AddressFamilyIPv4Unicast, AddressFamilyL2EVPN, importEntryIPv4EVPN, exportEntryIPv4EVPN)
		addAF(AddressFamilyIPv6Unicast, AddressFamilyL2EVPN, importEntryIPv6EVPN, exportEntryIPv6EVPN)
	}

	if dom != nil {
		v.DomItems.DomList = append(v.DomItems.DomList, dom)
	}
	return p.client.Update(ctx, v)
}

func (p *Provider) DeleteVRF(ctx context.Context, req *provider.VRFRequest) error {
	v := new(VRF)
	v.Name = req.VRF.Spec.Name
	return p.client.Delete(ctx, v)
}

func init() {
	provider.Register("cisco-nxos-gnmi", NewProvider)
}
