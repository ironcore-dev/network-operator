// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package nxos

import (
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-logr/logr"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/ironcore-dev/network-operator/api/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/deviceutil"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/acl"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/api"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/banner"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/copp"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/crypto"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/dns"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/iface"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/isis"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/logging"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/ntp"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/snmp"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/term"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/user"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/vlan"
)

var (
	_ provider.Provider                 = &Provider{}
	_ provider.InterfaceProvider        = &Provider{}
	_ provider.BannerProvider           = &Provider{}
	_ provider.UserProvider             = &Provider{}
	_ provider.DNSProvider              = &Provider{}
	_ provider.NTPProvider              = &Provider{}
	_ provider.ACLProvider              = &Provider{}
	_ provider.CertificateProvider      = &Provider{}
	_ provider.SNMPProvider             = &Provider{}
	_ provider.SyslogProvider           = &Provider{}
	_ provider.ManagementAccessProvider = &Provider{}
	_ provider.ISISProvider             = &Provider{}
)

type Provider struct {
	conn   *grpc.ClientConn
	client gnmiext.Client
}

func NewProvider() provider.Provider {
	return &Provider{}
}

func (p *Provider) Connect(ctx context.Context, conn *deviceutil.Connection) (err error) {
	p.conn, err = deviceutil.NewGrpcClient(ctx, conn)
	if err != nil {
		return fmt.Errorf("failed to create grpc connection: %w", err)
	}
	log := slog.New(logr.ToSlogHandler(ctrl.LoggerFrom(ctx)))
	p.client, err = gnmiext.NewClient(ctx, gpb.NewGNMIClient(p.conn), true, gnmiext.WithLogger(log))
	if err != nil {
		return err
	}
	return nil
}

func (p *Provider) Disconnect(_ context.Context, _ *deviceutil.Connection) error {
	return p.conn.Close()
}

func (p *Provider) EnsureInterface(ctx context.Context, req *provider.InterfaceRequest) (provider.Result, error) {
	switch req.Interface.Spec.Type {
	case v1alpha1.InterfaceTypePhysical:
		var opts []iface.PhysIfOption
		opts = append(opts, iface.WithPhysIfAdminState(req.Interface.Spec.AdminState == v1alpha1.AdminStateUp))
		if req.Interface.Spec.Description != "" {
			opts = append(opts, iface.WithDescription(req.Interface.Spec.Description))
		}
		if req.Interface.Spec.MTU > 0 {
			opts = append(opts, iface.WithPhysIfMTU(uint32(req.Interface.Spec.MTU))) // #nosec
		}
		if req.Interface.Spec.Switchport != nil {
			var l2opts []iface.L2Option
			switch req.Interface.Spec.Switchport.Mode {
			case v1alpha1.SwitchportModeAccess:
				l2opts = append(l2opts, iface.WithAccessVlan(uint16(req.Interface.Spec.Switchport.AccessVlan))) // #nosec
			case v1alpha1.SwitchportModeTrunk:
				l2opts = append(l2opts, iface.WithNativeVlan(uint16(req.Interface.Spec.Switchport.NativeVlan))) // #nosec
				vlans := make([]uint16, 0, len(req.Interface.Spec.Switchport.AllowedVlans))
				for _, v := range req.Interface.Spec.Switchport.AllowedVlans {
					vlans = append(vlans, uint16(v)) // #nosec
				}
				l2opts = append(l2opts, iface.WithAllowedVlans(vlans))
			default:
				return provider.Result{}, fmt.Errorf("invalid switchport mode: %s", req.Interface.Spec.Switchport.Mode)
			}
			cfg, err := iface.NewL2Config(l2opts...)
			if err != nil {
				return provider.Result{}, err
			}
			opts = append(opts, iface.WithPhysIfL2(cfg))
		}
		if len(req.Interface.Spec.IPv4Addresses) > 0 {
			var l3opts []iface.L3Option
			switch {
			case len(req.Interface.Spec.IPv4Addresses[0]) >= 10 && req.Interface.Spec.IPv4Addresses[0][:10] == "unnumbered":
				l3opts = append(l3opts, iface.WithMedium(iface.L3MediumTypeP2P))
				l3opts = append(l3opts, iface.WithUnnumberedAddressing(req.Interface.Spec.IPv4Addresses[0][11:])) // Extract the source interface name
			default:
				l3opts = append(l3opts, iface.WithNumberedAddressingIPv4(req.Interface.Spec.IPv4Addresses))
			}
			// FIXME: don't hardcode P2P
			l3opts = append(l3opts, iface.WithMedium(iface.L3MediumTypeP2P))
			cfg, err := iface.NewL3Config(l3opts...)
			if err != nil {
				return provider.Result{}, err
			}
			opts = append(opts, iface.WithPhysIfL3(cfg))
		}
		i, err := iface.NewPhysicalInterface(req.Interface.Spec.Name, opts...)
		if err != nil {
			return provider.Result{}, err
		}
		if err := p.client.Update(ctx, i); err != nil {
			return provider.Result{}, err
		}
		s, err := i.GetStatus(ctx, p.client)
		if err != nil {
			return provider.Result{}, err
		}
		status := metav1.ConditionFalse
		if s.OperSt == "up" {
			status = metav1.ConditionTrue
		}
		return provider.Result{
			RequeueAfter: time.Second * 30,
			Conditions: []metav1.Condition{
				{
					Type:    "Operational",
					Status:  status,
					Reason:  "OperationalStatus",
					Message: fmt.Sprintf("Interface is %s (admin: %s)", s.OperSt, req.Interface.Spec.AdminState),
				},
			},
		}, p.client.Update(ctx, i)
	case v1alpha1.InterfaceTypeLoopback:
		var opts []iface.LoopbackOption
		opts = append(opts, iface.WithLoopbackAdminState(req.Interface.Spec.AdminState == v1alpha1.AdminStateUp))
		if len(req.Interface.Spec.IPv4Addresses) > 0 {
			var l3opts []iface.L3Option
			switch {
			case len(req.Interface.Spec.IPv4Addresses[0]) >= 10 && req.Interface.Spec.IPv4Addresses[0][:10] == "unnumbered":
				l3opts = append(l3opts, iface.WithUnnumberedAddressing(req.Interface.Spec.IPv4Addresses[0][11:])) // Extract the source interface name
			default:
				l3opts = append(l3opts, iface.WithNumberedAddressingIPv4(req.Interface.Spec.IPv4Addresses))
			}
			cfg, err := iface.NewL3Config(l3opts...)
			if err != nil {
				return provider.Result{}, err
			}
			opts = append(opts, iface.WithLoopbackL3(cfg))
		}
		var desc *string
		if req.Interface.Spec.Description != "" {
			desc = &req.Interface.Spec.Description
		}
		i, err := iface.NewLoopbackInterface(req.Interface.Spec.Name, desc, opts...)
		if err != nil {
			return provider.Result{}, err
		}
		return provider.Result{}, p.client.Update(ctx, i)
	}
	return provider.Result{}, fmt.Errorf("unsupported interface type: %s", req.Interface.Spec.Type)
}

func (p *Provider) DeleteInterface(ctx context.Context, req *provider.InterfaceRequest) error {
	switch req.Interface.Spec.Type {
	case v1alpha1.InterfaceTypePhysical:
		i, err := iface.NewPhysicalInterface(req.Interface.Spec.Name)
		if err != nil {
			return err
		}
		return p.client.Reset(ctx, i)
	case v1alpha1.InterfaceTypeLoopback:
		// FIXME: Description should no be a required field in the constructor
		i, err := iface.NewLoopbackInterface(req.Interface.Spec.Name, nil)
		if err != nil {
			return err
		}
		return p.client.Reset(ctx, i)
	}
	return fmt.Errorf("unsupported interface type: %s", req.Interface.Spec.Type)
}

func (p *Provider) EnsureBanner(ctx context.Context, req *provider.BannerRequest) (provider.Result, error) {
	b := &banner.Banner{Message: req.Message, Delimiter: "^"}
	return provider.Result{}, p.client.Update(ctx, b)
}

func (p *Provider) DeleteBanner(ctx context.Context) error {
	return p.client.Reset(ctx, &banner.Banner{})
}

func (p *Provider) EnsureUser(ctx context.Context, req *provider.EnsureUserRequest) (provider.Result, error) {
	opts := []user.UserOption{}
	if req.SSHKey != "" {
		opts = append(opts, user.WithSSHKey(req.SSHKey))
	}
	if len(req.Roles) > 0 {
		r := make([]user.Role, 0, len(req.Roles))
		for _, role := range req.Roles {
			r = append(r, user.Role{Name: role})
		}
		opts = append(opts, user.WithRoles(r...))
	}
	u, err := user.NewUser(req.Username, req.Password, opts...)
	if err != nil {
		return provider.Result{}, fmt.Errorf("failed to create user: %w", err)
	}
	return provider.Result{}, p.client.Update(ctx, u)
}

func (p *Provider) DeleteUser(ctx context.Context, req *provider.DeleteUserRequest) error {
	return p.client.Reset(ctx, &user.User{Name: req.Username})
}

func (p *Provider) EnsureDNS(ctx context.Context, req *provider.EnsureDNSRequest) (provider.Result, error) {
	d := &dns.DNS{
		Enable:     true,
		DomainName: req.DNS.Spec.Domain,
		Providers:  make([]*dns.Provider, len(req.DNS.Spec.Servers)),
	}
	for i, p := range req.DNS.Spec.Servers {
		d.Providers[i] = &dns.Provider{
			Addr:  p.Address,
			Vrf:   p.VrfName,
			SrcIf: req.DNS.Spec.SourceInterfaceName,
		}
	}
	return provider.Result{}, p.client.Update(ctx, d)
}

func (p *Provider) DeleteDNS(ctx context.Context) error {
	return p.client.Reset(ctx, &dns.DNS{})
}

type NTPConfig struct {
	Log struct {
		Enable bool `json:"enable"`
	} `json:"log"`
}

func (p *Provider) EnsureNTP(ctx context.Context, req *provider.EnsureNTPRequest) (provider.Result, error) {
	var cfg NTPConfig
	if req.ProviderConfig != nil {
		if err := req.ProviderConfig.Into(&cfg); err != nil {
			return provider.Result{}, err
		}
	}
	n := &ntp.NTP{
		EnableLogging: cfg.Log.Enable,
		SrcInterface:  req.NTP.Spec.SourceInterfaceName,
		Servers:       make([]*ntp.Server, len(req.NTP.Spec.Servers)),
	}
	for i, s := range req.NTP.Spec.Servers {
		n.Servers[i] = &ntp.Server{
			Name:      s.Address,
			Preferred: s.Prefer,
			Vrf:       s.VrfName,
		}
	}
	return provider.Result{}, p.client.Update(ctx, n)
}

func (p *Provider) DeleteNTP(ctx context.Context) error {
	return p.client.Reset(ctx, &ntp.NTP{})
}

func (p *Provider) EnsureACL(ctx context.Context, req *provider.EnsureACLRequest) (provider.Result, error) {
	a := &acl.ACL{
		Name:  req.ACL.Spec.Name,
		Rules: make([]*acl.Rule, len(req.ACL.Spec.Entries)),
	}
	for i, entry := range req.ACL.Spec.Entries {
		var action acl.Action
		switch entry.Action {
		case v1alpha1.ActionPermit:
			action = acl.Permit
		case v1alpha1.ActionDeny:
			action = acl.Deny
		default:
			return provider.Result{}, fmt.Errorf("unsupported ACL action: %s", entry.Action)
		}
		a.Rules[i] = &acl.Rule{
			Seq:         uint32(entry.Sequence), //nolint:gosec
			Action:      action,
			Protocol:    acl.ProtocolFrom(entry.Protocol),
			Description: entry.Description,
			Source:      entry.SourceAddress.Prefix,
			Destination: entry.DestinationAddress.Prefix,
		}
	}
	return provider.Result{}, p.client.Update(ctx, a)
}

func (p *Provider) DeleteACL(ctx context.Context, req *provider.DeleteACLRequest) error {
	return p.client.Reset(ctx, &acl.ACL{Name: req.Name})
}

func (p *Provider) EnsureCertificate(ctx context.Context, req *provider.EnsureCertificateRequest) (provider.Result, error) {
	tp := &crypto.Trustpoint{ID: req.ID}
	if err := p.client.Update(ctx, tp); err != nil {
		// Duo to a limitation in the NX-OS YANG model, trustpoints cannot be updated.
		if errors.Is(err, crypto.ErrAlreadyExists) {
			return provider.Result{}, nil
		}
		return provider.Result{}, err
	}
	key, ok := req.Certificate.PrivateKey.(*rsa.PrivateKey)
	if !ok {
		return provider.Result{}, fmt.Errorf("unsupported private key type: expected *rsa.PrivateKey, got %T", req.Certificate.PrivateKey)
	}
	cert := &crypto.Certificate{Key: key, Cert: req.Certificate.Leaf}
	return provider.Result{}, cert.Load(ctx, p.conn, req.ID)
}

func (p *Provider) DeleteCertificate(ctx context.Context, req *provider.DeleteCertificateRequest) error {
	tp := &crypto.Trustpoint{ID: req.ID}
	return p.client.Reset(ctx, tp)
}

func (p *Provider) EnsureSNMP(ctx context.Context, req *provider.EnsureSNMPRequest) (provider.Result, error) {
	s := &snmp.SNMP{
		Contact:     req.SNMP.Spec.Contact,
		Location:    req.SNMP.Spec.Location,
		SrcIf:       req.SNMP.Spec.SourceInterfaceName,
		Hosts:       make([]snmp.Host, len(req.SNMP.Spec.Hosts)),
		Communities: make([]snmp.Community, len(req.SNMP.Spec.Communities)),
		Traps:       req.SNMP.Spec.Traps,
	}
	for i, h := range req.SNMP.Spec.Hosts {
		s.Hosts[i] = snmp.Host{
			Address:   h.Address,
			Type:      snmp.MessageTypeFrom(h.Type),
			Version:   snmp.VersionFrom(h.Version),
			Vrf:       h.VrfName,
			Community: h.Community,
		}
	}
	for i, c := range req.SNMP.Spec.Communities {
		s.Communities[i] = snmp.Community{
			Name:    c.Name,
			Group:   c.Group,
			IPv4ACL: c.ACLName,
		}
	}
	return provider.Result{}, p.client.Update(ctx, s)
}

func (p *Provider) DeleteSNMP(ctx context.Context, req *provider.DeleteSNMPRequest) error {
	s := &snmp.SNMP{}
	return p.client.Reset(ctx, s)
}

type SyslogConfig struct {
	OriginID            string
	SourceInterfaceName string
	HistorySize         uint32
	HistoryLevel        v1alpha1.Severity
	DefaultSeverity     v1alpha1.Severity
}

func (p *Provider) EnsureSyslog(ctx context.Context, req *provider.EnsureSyslogRequest) (provider.Result, error) {
	var cfg SyslogConfig
	if req.ProviderConfig != nil {
		if err := req.ProviderConfig.Into(&cfg); err != nil {
			return provider.Result{}, err
		}
	}

	if cfg.OriginID == "" {
		cfg.OriginID = req.Syslog.Name
	}
	if cfg.SourceInterfaceName == "" {
		cfg.SourceInterfaceName = "mgmt0"
	}
	if cfg.HistorySize <= 0 {
		cfg.HistorySize = 500
	}

	l := &logging.Logging{
		Enable:          true,
		OriginID:        cfg.OriginID,
		SrcIf:           cfg.SourceInterfaceName,
		Servers:         make([]*logging.SyslogServer, len(req.Syslog.Spec.Servers)),
		History:         logging.History{Size: cfg.HistorySize, Severity: logging.SeverityLevelFrom(string(cfg.HistoryLevel))},
		DefaultSeverity: logging.SeverityLevelFrom(string(cfg.DefaultSeverity)),
		Facilities:      make([]*logging.Facility, len(req.Syslog.Spec.Facilities)),
	}

	for i, s := range req.Syslog.Spec.Servers {
		l.Servers[i] = &logging.SyslogServer{
			Host:  s.Address,
			Port:  uint32(s.Port), //nolint:gosec
			Proto: logging.UDP,
			Vrf:   s.VrfName,
			Level: logging.SeverityLevelFrom(string(s.Severity)),
		}
	}

	for i, f := range req.Syslog.Spec.Facilities {
		l.Facilities[i] = &logging.Facility{
			Name:     f.Name,
			Severity: logging.SeverityLevelFrom(string(f.Severity)),
		}
	}
	return provider.Result{}, p.client.Reset(ctx, l)
}

func (p *Provider) DeleteSyslog(ctx context.Context) error {
	l := &logging.Logging{}
	return p.client.Reset(ctx, l)
}

func (p *Provider) EnsureManagementAccess(ctx context.Context, req *provider.EnsureManagementAccessRequest) (provider.Result, error) {
	steps := []gnmiext.DeviceConf{
		// Steps that depend on the device spec
		&api.GRPC{
			Enable:     req.ManagementAccess.Spec.GRPC.Enabled,
			Port:       uint32(req.ManagementAccess.Spec.GRPC.Port), //nolint:gosec
			Vrf:        req.ManagementAccess.Spec.GRPC.VrfName,
			Trustpoint: req.ManagementAccess.Spec.GRPC.CertificateID,
			GNMI:       nil,
		},
		// Static steps that are always executed
		&vlan.Settings{LongName: true},
		&copp.COPP{Profile: copp.Strict},
		&term.Console{
			Timeout: 5, // minutes
		},
		&term.VTY{
			SessionLimit: 8,
			Timeout:      5, // minutes
		},
	}
	errs := make([]error, 0, len(steps))
	for _, step := range steps {
		if err := p.client.Update(ctx, step); err != nil {
			errs = append(errs, err)
		}
	}
	return provider.Result{}, errors.Join(errs...)
}

func (p *Provider) DeleteManagementAccess(ctx context.Context) error {
	steps := []gnmiext.DeviceConf{
		&vlan.Settings{},
		&copp.COPP{},
		&term.Console{},
		&term.VTY{},
	}
	errs := make([]error, 0, len(steps))
	for _, step := range steps {
		if err := p.client.Reset(ctx, step); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (p *Provider) EnsureISIS(ctx context.Context, req *provider.EnsureISISRequest) (provider.Result, error) {
	s := &isis.ISIS{
		Name: req.ISIS.Spec.Instance,
		NET:  req.ISIS.Spec.NetworkEntityTitle,
	}
	switch req.ISIS.Spec.Type {
	case v1alpha1.ISISLevel1:
		s.Level = isis.Level1
	case v1alpha1.ISISLevel2:
		s.Level = isis.Level2
	case v1alpha1.ISISLevel12:
		s.Level = isis.Level12
	}
	switch req.ISIS.Spec.OverloadBit {
	case v1alpha1.OverloadBitNever:
	case v1alpha1.OverloadBitAlways:
	case v1alpha1.OverloadBitOnStartup:
		s.OverloadBit = &isis.OverloadBit{OnStartup: 60} // seconds
	}
	var ipv4, ipv6 bool
	for _, af := range req.ISIS.Spec.AddressFamilies {
		switch af {
		case v1alpha1.AddressFamilyIPv4Unicast:
			s.AddressFamilies = append(s.AddressFamilies, isis.IPv4Unicast)
			ipv4 = true
		case v1alpha1.AddressFamilyIPv6Unicast:
			s.AddressFamilies = append(s.AddressFamilies, isis.IPv6Unicast)
			ipv6 = true
		}
	}
	if err := p.client.Update(ctx, s); err != nil {
		return provider.Result{}, err
	}
	var adjUp uint16 = 0
	for _, iface := range req.Interfaces {
		var opts []isis.IfOption
		opts = append(opts, isis.WithIPv4(ipv4))
		opts = append(opts, isis.WithIPv6(ipv6))
		if iface.Interface.Spec.Type == v1alpha1.InterfaceTypePhysical {
			opts = append(opts, isis.WithPointToPoint())
		}
		i, err := isis.NewInterface(iface.Interface.Spec.Name, req.ISIS.Spec.Instance, opts...)
		if err != nil {
			return provider.Result{}, err
		}
		if err := p.client.Update(ctx, i); err != nil {
			return provider.Result{}, err
		}
		// TODO: remove hard-coded vrf "default" and level "l1" for now
		up, err := s.GetAdjancencyStatus(ctx, p.client, iface.Interface.Spec.Name, "default", "l1")
		if err != nil {
			return provider.Result{}, err
		}
		if up {
			adjUp++
		}
	}
	status := metav1.ConditionFalse
	// TODO: OperSt should be up
	if adjUp == uint16(len(req.Interfaces)) {
		status = metav1.ConditionTrue
	}
	return provider.Result{
		RequeueAfter: time.Second * 30,
		Conditions: []metav1.Condition{
			{
				Type:    "Operational",
				Status:  status,
				Reason:  "OperationalStatus",
				Message: fmt.Sprintf("active adjancecies %d of %d)", adjUp, len(req.Interfaces)),
			},
		},
	}, nil
}

func (p *Provider) DeleteISIS(ctx context.Context, req *provider.DeleteISISRequest) error {
	s := &isis.ISIS{Name: req.ISIS.Spec.Instance}
	return p.client.Update(ctx, s)
}

func init() {
	provider.Register("cisco-nxos-gnmi", NewProvider)
}
