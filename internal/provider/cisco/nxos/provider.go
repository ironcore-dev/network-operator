// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package nxos

import (
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	"log/slog"

	"github.com/go-logr/logr"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc"
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
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/logging"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/ntp"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/snmp"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/term"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/user"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/vlan"
)

var (
	_ provider.Provider                 = &Provider{}
	_ provider.BannerProvider           = &Provider{}
	_ provider.UserProvider             = &Provider{}
	_ provider.DNSProvider              = &Provider{}
	_ provider.NTPProvider              = &Provider{}
	_ provider.ACLProvider              = &Provider{}
	_ provider.CertificateProvider      = &Provider{}
	_ provider.SNMPProvider             = &Provider{}
	_ provider.SyslogProvider           = &Provider{}
	_ provider.ManagementAccessProvider = &Provider{}
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

func init() {
	provider.Register("cisco-nxos-gnmi", NewProvider)
}
