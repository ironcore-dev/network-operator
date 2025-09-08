// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package nxos

import (
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/mitchellh/hashstructure/v2"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/ironcore-dev/network-operator/api/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/clientutil"
	"github.com/ironcore-dev/network-operator/internal/deviceutil"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/acl"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/api"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/banner"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/copp"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/crypto"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/dns"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/feat"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/iface"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/logging"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/ntp"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/snmp"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/term"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/user"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/vlan"
)

// API Object Annotations to set NX-OS specific attributes.
const (
	// This label can be set to true to simulate the configuration changes without applying them to the switch.
	DryRunAnnotation = "nxos.cisco.network.ironcore.dev/dry-run"
	// This label can be set to enable the long-name option for VLANs.
	VlanLongNameAnnotation = "nxos.cisco.network.ironcore.dev/vlan-long-name"
	// This label can be set to configure the control plane policing (CoPP) profile for the device.
	CoppProfileAnnotation = "nxos.cisco.network.ironcore.dev/copp-profile"
)

var (
	_ provider.Provider            = &Provider{}
	_ provider.InterfaceProvider   = &Provider{}
	_ provider.BannerProvider      = &Provider{}
	_ provider.UserProvider        = &Provider{}
	_ provider.DNSProvider         = &Provider{}
	_ provider.NTPProvider         = &Provider{}
	_ provider.ACLProvider         = &Provider{}
	_ provider.CertificateProvider = &Provider{}
	_ provider.SNMPProvider        = &Provider{}
	_ provider.SyslogProvider      = &Provider{}
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

func (p *Provider) EnsureInterface(ctx context.Context, req *provider.InterfaceRequest) (res provider.Result, reterr error) {
	defer func() {
		res = WithErrorConditions(res, reterr)
	}()

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
		return provider.Result{}, p.client.Update(ctx, i)
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

func (p *Provider) EnsureBanner(ctx context.Context, req *provider.BannerRequest) (res provider.Result, reterr error) {
	defer func() {
		res = WithErrorConditions(res, reterr)
	}()

	b := &banner.Banner{Message: req.Message, Delimiter: "^"}
	return provider.Result{}, p.client.Update(ctx, b)
}

func (p *Provider) DeleteBanner(ctx context.Context) error {
	return p.client.Reset(ctx, &banner.Banner{})
}

func (p *Provider) EnsureUser(ctx context.Context, req *provider.EnsureUserRequest) (res provider.Result, reterr error) {
	defer func() {
		res = WithErrorConditions(res, reterr)
	}()

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

func (p *Provider) EnsureDNS(ctx context.Context, req *provider.EnsureDNSRequest) (res provider.Result, reterr error) {
	defer func() {
		res = WithErrorConditions(res, reterr)
	}()

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

func (p *Provider) EnsureNTP(ctx context.Context, req *provider.EnsureNTPRequest) (res provider.Result, reterr error) {
	defer func() {
		res = WithErrorConditions(res, reterr)
	}()

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

func (p *Provider) EnsureACL(ctx context.Context, req *provider.EnsureACLRequest) (res provider.Result, reterr error) {
	defer func() {
		res = WithErrorConditions(res, reterr)
	}()

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
			Protocol:    acl.ProtocolFrom(strings.ToLower(string(entry.Protocol))),
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

func (p *Provider) EnsureCertificate(ctx context.Context, req *provider.EnsureCertificateRequest) (res provider.Result, reterr error) {
	defer func() {
		res = WithErrorConditions(res, reterr)
	}()

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

func (p *Provider) EnsureSNMP(ctx context.Context, req *provider.EnsureSNMPRequest) (res provider.Result, reterr error) {
	defer func() {
		res = WithErrorConditions(res, reterr)
	}()

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

func (p *Provider) EnsureSyslog(ctx context.Context, req *provider.EnsureSyslogRequest) (res provider.Result, reterr error) {
	defer func() {
		res = WithErrorConditions(res, reterr)
	}()

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

func (p *Provider) CreateDevice(ctx context.Context, device *v1alpha1.Device) error {
	log := ctrl.LoggerFrom(ctx)

	c, ok := clientutil.FromContext(ctx)
	if !ok {
		return errors.New("failed to get controller client from context")
	}

	cc, err := deviceutil.GetDeviceConnection(ctx, c, device)
	if err != nil {
		return fmt.Errorf("failed to get device connection details: %w", err)
	}

	conn, err := deviceutil.NewGrpcClient(ctx, cc)
	if err != nil {
		return fmt.Errorf("failed to create grpc connection: %w", err)
	}
	defer conn.Close()

	var opts []gnmiext.Option
	v, ok := device.Annotations[DryRunAnnotation]
	if ok && v == "true" {
		opts = append(opts, gnmiext.WithDryRun())
	}
	opts = append(opts, gnmiext.WithLogger(slog.New(logr.ToSlogHandler(log))))

	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	gnmi, err := gnmiext.NewClient(cctx, gpb.NewGNMIClient(conn), true, opts...)
	if err != nil {
		if s, ok := status.FromError(err); ok {
			log.Error(err, "Failed to connect to device", "Message", s.Message())

			var reason string
			switch s.Code() {
			case codes.DeadlineExceeded, codes.Unavailable:
				reason = v1alpha1.DeviceUnreachableReason
			case codes.Unauthenticated:
				reason = v1alpha1.DeviceUnauthenticatedReason
			}

			meta.SetStatusCondition(&device.Status.Conditions, metav1.Condition{
				Type:               v1alpha1.ReadyCondition,
				Status:             metav1.ConditionFalse,
				Reason:             reason,
				Message:            err.Error(),
				ObservedGeneration: device.Generation,
			})
			return nil
		}

		meta.SetStatusCondition(&device.Status.Conditions, metav1.Condition{
			Type:               v1alpha1.ReadyCondition,
			Status:             metav1.ConditionFalse,
			Reason:             v1alpha1.DeviceUnsupportedReason,
			Message:            err.Error(),
			ObservedGeneration: device.Generation,
		})
		return nil
	}

	s := &Scope{
		Client: c,
		Conn:   conn,
		GNMI:   gnmi,
	}

	steps := []Step{
		// Features that need to be enabled on the device
		&Features{Spec: []string{
			"bfd",
			"bgp",
			"grpc",
			"isis",
			"lacp",
			"lldp",
			"netconf",
			"nxapi",
			"pim",
			"vpc",
		}},
		// Steps that depend on the device spec
		&GRPC{Spec: device.Spec.GRPC},
		&VLAN{LongName: device.Annotations[VlanLongNameAnnotation] == "true"},
		&Copp{Profile: device.Annotations[CoppProfileAnnotation]},
		// Static steps that are always executed
		&NXAPI{},
		&Console{},
		&VTY{},
	}

	errs := []error{}
	for _, step := range steps {
		hash, err := hashstructure.Hash(step, hashstructure.FormatV2, nil)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		check := strconv.FormatUint(hash, 10)
		if deps := step.Deps(); len(deps) > 0 {
			v, err := c.ListResourceVersions(ctx, deps...)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			check += ":" + strings.Join(v, ":")
		}

		name := step.Name()
		cond := meta.FindStatusCondition(device.Status.Conditions, name)
		if cond != nil && cond.Status == metav1.ConditionTrue && cond.Message == check {
			log.Info(name + " configuration already up to date, skipping")
			continue
		}

		if err := step.Exec(ctx, s); err != nil {
			errs = append(errs, err)
			continue
		}

		meta.SetStatusCondition(&device.Status.Conditions, metav1.Condition{
			Type:               name,
			Status:             metav1.ConditionTrue,
			Reason:             v1alpha1.ReadyReason,
			Message:            check,
			ObservedGeneration: device.Generation,
		})
	}

	if len(errs) > 0 {
		meta.SetStatusCondition(&device.Status.Conditions, metav1.Condition{
			Type:               v1alpha1.ReadyCondition,
			Status:             metav1.ConditionFalse,
			Reason:             v1alpha1.NotReadyReason,
			Message:            "One or more errors occurred during reconciliation",
			ObservedGeneration: device.Generation,
		})
		return kerrors.NewAggregate(errs)
	}

	meta.SetStatusCondition(&device.Status.Conditions, metav1.Condition{
		Type:               v1alpha1.ReadyCondition,
		Status:             metav1.ConditionTrue,
		Reason:             v1alpha1.ReadyReason,
		Message:            "Switch has been configured and is ready for use",
		ObservedGeneration: device.Generation,
	})

	return nil
}

func (p *Provider) GetGrpcClient(ctx context.Context, obj metav1.Object) (*grpc.ClientConn, error) {
	c, ok := clientutil.FromContext(ctx)
	if !ok {
		return nil, errors.New("failed to get controller client from context")
	}
	d, err := deviceutil.GetDeviceFromMetadata(ctx, c, obj)
	if err != nil {
		return nil, fmt.Errorf("failed to get device from metadata: %w", err)
	}
	cc, err := deviceutil.GetDeviceConnection(ctx, c, d)
	if err != nil {
		return nil, fmt.Errorf("failed to get device connection details: %w", err)
	}
	conn, err := deviceutil.NewGrpcClient(ctx, cc)
	if err != nil {
		return nil, fmt.Errorf("failed to create grpc connection: %w", err)
	}
	return conn, nil
}

// Scope holds the different objects that are read and used during the reconcile.
type Scope struct {
	Client *clientutil.Client
	Conn   *grpc.ClientConn
	GNMI   gnmiext.Client
}

// Step is an interface that defines a reconciliation step.
// Each step is responsible for a specific part of the switch configuration.
// It is only executed if the coresponding part of the [v1alpha1.SwitchSpec]
// or any of the dependencies listed by Deps have changed.
// This is done to avoid unnecessary API calls and to speed up the reconciliation process.
type Step interface {
	// Name returns the name of the step.
	Name() string

	// Exec executes the reconciliation step.
	Exec(ctx context.Context, s *Scope) error

	// Deps returns a list of dependent resources than should trigger a reconciliation if changed.
	// Currently, only secret references are supported.
	Deps() []client.ObjectKey
}

var (
	_ Step = (*VTY)(nil)
	_ Step = (*Console)(nil)
	_ Step = (*NXAPI)(nil)
	_ Step = (*GRPC)(nil)
	_ Step = (*VLAN)(nil)
	_ Step = (*Features)(nil)
	_ Step = (*Copp)(nil)
)

type VTY struct{}

func (step *VTY) Name() string             { return "VTY" }
func (step *VTY) Deps() []client.ObjectKey { return nil }
func (step *VTY) Exec(ctx context.Context, s *Scope) error {
	v := &term.VTY{
		SessionLimit: 8,
		Timeout:      5, // minutes
	}
	return s.GNMI.Update(ctx, v)
}

type Console struct{}

func (step *Console) Name() string             { return "Console" }
func (step *Console) Deps() []client.ObjectKey { return nil }
func (step *Console) Exec(ctx context.Context, s *Scope) error {
	c := &term.Console{Timeout: 5} // minutes
	return s.GNMI.Update(ctx, c)
}

type NXAPI struct{ Spec *v1alpha1.Certificate }

func (step *NXAPI) Name() string             { return "NXAPI" }
func (step *NXAPI) Deps() []client.ObjectKey { return nil }
func (step *NXAPI) Exec(ctx context.Context, s *Scope) error {
	n := &api.NXAPI{Enable: false}
	if step.Spec != nil {
		n = &api.NXAPI{Enable: true, Cert: &api.Trustpoint{ID: step.Spec.Name}}
	}
	return s.GNMI.Update(ctx, n)
}

type GRPC struct{ Spec *v1alpha1.GRPC }

func (step *GRPC) Name() string             { return "GRPC" }
func (step *GRPC) Deps() []client.ObjectKey { return nil }
func (step *GRPC) Exec(ctx context.Context, s *Scope) error {
	g := &api.GRPC{Enable: false}
	if step.Spec != nil {
		g = &api.GRPC{
			Enable:     true,
			Port:       uint32(step.Spec.Port), //nolint:gosec
			Vrf:        step.Spec.NetworkInstance,
			Trustpoint: step.Spec.CertificateID,
			GNMI:       nil,
		}
		if step.Spec.GNMI != nil {
			g.GNMI = &api.GNMI{
				MaxConcurrentCall: uint16(step.Spec.GNMI.MaxConcurrentCall), //nolint:gosec
				KeepAliveTimeout:  uint32(step.Spec.GNMI.KeepAliveTimeout.Seconds()),
				MinSampleInterval: uint32(step.Spec.GNMI.MinSampleInterval.Seconds()),
			}
		}
	}
	return s.GNMI.Update(ctx, g)
}

type VLAN struct{ LongName bool }

func (step *VLAN) Name() string             { return "VLAN" }
func (step *VLAN) Deps() []client.ObjectKey { return nil }
func (step *VLAN) Exec(ctx context.Context, s *Scope) error {
	v := &vlan.Settings{LongName: step.LongName}
	return s.GNMI.Update(ctx, v)
}

type Features struct{ Spec []string }

func (step *Features) Name() string             { return "Features" }
func (step *Features) Deps() []client.ObjectKey { return nil }
func (step *Features) Exec(ctx context.Context, s *Scope) error {
	return s.GNMI.Update(ctx, feat.Features(step.Spec))
}

type Copp struct{ Profile string }

func (step *Copp) Name() string             { return "COPP" }
func (step *Copp) Deps() []client.ObjectKey { return nil }
func (step *Copp) Exec(ctx context.Context, s *Scope) error {
	if step.Profile == "" {
		return nil
	}
	var profile copp.Profile
	switch strings.ToLower(step.Profile) {
	case "strict":
		profile = copp.Strict
	case "moderate":
		profile = copp.Moderate
	case "dense":
		profile = copp.Dense
	case "lenient":
		profile = copp.Lenient
	default:
		profile = copp.Unknown
	}
	c := &copp.COPP{Profile: profile}
	return s.GNMI.Update(ctx, c)
}

func WithErrorConditions(res provider.Result, err error) provider.Result {
	cond := metav1.Condition{
		Type:    "Configured",
		Status:  metav1.ConditionTrue,
		Reason:  "Success",
		Message: "Successfully applied configuration via gNMI",
	}
	if err != nil {
		cond.Status = metav1.ConditionFalse
		cond.Reason = "Error"
		cond.Message = err.Error()

		// If the error is a gRPC status error, extract the code and message
		if statusErr, ok := status.FromError(err); ok {
			cond.Reason = statusErr.Code().String()
			cond.Message = statusErr.Message()
		}
	}
	meta.SetStatusCondition(&res.Conditions, cond)
	return res
}

func init() {
	provider.Register("cisco-nxos-gnmi", NewProvider)
}
