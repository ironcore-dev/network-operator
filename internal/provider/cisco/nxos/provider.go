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
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/logging"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/ntp"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/snmp"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/term"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/vlan"
)

// API Object Annotations to set NX-OS specific attributes.
const (
	// This label can be set to true to simulate the configuration changes without applying them to the switch.
	DryRunAnnotation = "nxos.cisco.network.ironcore.dev/dry-run"
	// This label can be set to configure the default severity level for logging.
	LogDefaultSeverityAnnotation = "nxos.cisco.network.ironcore.dev/log-default-severity"
	// This label can be set to severity level for log history.
	LogHistorySeverityAnnotation = "nxos.cisco.network.ironcore.dev/log-history-severity"
	// This label can be set to the size of the log history.
	LogHistorySizeAnnotation = "nxos.cisco.network.ironcore.dev/log-history-size"
	// This label can be set to the origin ID for logging.
	LogOriginIDAnnotation = "nxos.cisco.network.ironcore.dev/log-origin-id"
	// This label can be set to the source interface to be used to reach the syslog servers.
	LogSrcIfAnnotation = "nxos.cisco.network.ironcore.dev/log-src-if"
	// This label can be set to enable the long-name option for VLANs.
	VlanLongNameAnnotation = "nxos.cisco.network.ironcore.dev/vlan-long-name"
	// This label can be set to configure the control plane policing (CoPP) profile for the device.
	CoppProfileAnnotation = "nxos.cisco.network.ironcore.dev/copp-profile"
)

type Provider struct{}

func (p *Provider) CreateInterface(ctx context.Context, _ *v1alpha1.Interface) error {
	log := ctrl.LoggerFrom(ctx)
	log.Error(provider.ErrUnimplemented, "CreateInterface not implemented")
	return nil
}

func (p *Provider) DeleteInterface(ctx context.Context, _ *v1alpha1.Interface) error {
	log := ctrl.LoggerFrom(ctx)
	log.Error(provider.ErrUnimplemented, "DeleteInterface not implemented")
	return nil
}

func (p *Provider) CreateDevice(ctx context.Context, device *v1alpha1.Device) error {
	log := ctrl.LoggerFrom(ctx)

	c, ok := clientutil.FromContext(ctx)
	if !ok {
		return errors.New("failed to get controller client from context")
	}

	conn, err := deviceutil.GetDeviceGrpcClient(ctx, c, device)
	if err != nil {
		return fmt.Errorf("failed to create grpc connection: %w", err)
	}
	defer conn.Close()

	var opts []gnmiext.Option
	var isDryRun bool
	v, ok := device.Annotations[DryRunAnnotation]
	if ok && v == "true" {
		opts = append(opts, gnmiext.WithDryRun())
		isDryRun = true
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
		&DNS{Spec: device.Spec.DNS},
		&NTP{Spec: device.Spec.NTP},
		&ACL{Spec: device.Spec.ACL},
		&Trustpoints{Spec: device.Spec.PKI, DryRun: isDryRun},
		&SNMP{Spec: device.Spec.SNMP},
		&GRPC{Spec: device.Spec.GRPC},
		&Banner{Spec: device.Spec.Banner},
		&VLAN{LongName: device.Annotations[VlanLongNameAnnotation] == "true"},
		&Copp{Profile: device.Annotations[CoppProfileAnnotation]},
		&Logging{
			Spec:            device.Spec.Logging,
			DefaultSeverity: device.Annotations[LogDefaultSeverityAnnotation],
			HistoryLevel:    device.Annotations[LogHistorySeverityAnnotation],
			HistorySize:     device.Annotations[LogHistorySizeAnnotation],
			OriginID:        device.Annotations[LogOriginIDAnnotation],
			SrcIf:           device.Annotations[LogSrcIfAnnotation],
		},
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

func (p *Provider) DeleteDevice(ctx context.Context, _ *v1alpha1.Device) error {
	log := ctrl.LoggerFrom(ctx)
	log.Error(provider.ErrUnimplemented, "DeleteDevice not implemented")
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
	conn, err := deviceutil.GetDeviceGrpcClient(ctx, c, d)
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
	_ Step = (*NTP)(nil)
	_ Step = (*VTY)(nil)
	_ Step = (*Console)(nil)
	_ Step = (*ACL)(nil)
	_ Step = (*Trustpoints)(nil)
	_ Step = (*NXAPI)(nil)
	_ Step = (*GRPC)(nil)
	_ Step = (*SNMP)(nil)
	_ Step = (*Logging)(nil)
	_ Step = (*VLAN)(nil)
	_ Step = (*Features)(nil)
	_ Step = (*DNS)(nil)
	_ Step = (*Copp)(nil)
	_ Step = (*Banner)(nil)
)

type NTP struct{ Spec *v1alpha1.NTP }

func (step *NTP) Name() string             { return "NTP" }
func (step *NTP) Deps() []client.ObjectKey { return nil }
func (step *NTP) Exec(ctx context.Context, s *Scope) error {
	n := &ntp.NTP{
		EnableLogging: false,
		SrcInterface:  step.Spec.SrcIf,
		Servers:       make([]*ntp.Server, len(step.Spec.Servers)),
	}
	for i, s := range step.Spec.Servers {
		n.Servers[i] = &ntp.Server{
			Name:      s.Address,
			Preferred: s.Prefer,
			Vrf:       s.NetworkInstance,
		}
	}
	return s.GNMI.Update(ctx, n)
}

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

type ACL struct{ Spec []*v1alpha1.ACL }

func (step *ACL) Name() string             { return "ACL" }
func (step *ACL) Deps() []client.ObjectKey { return nil }
func (step *ACL) Exec(ctx context.Context, s *Scope) error {
	if len(step.Spec) == 0 {
		return nil
	}

	for _, item := range step.Spec {
		rules := make([]*acl.Rule, len(item.Entries))
		for j, rule := range item.Entries {
			var action acl.Action
			switch rule.Action {
			case v1alpha1.ActionPermit:
				action = acl.Permit
			case v1alpha1.ActionDeny:
				action = acl.Deny
			default:
				return fmt.Errorf("unsupported ACL action: %s", rule.Action)
			}

			rules[j] = &acl.Rule{
				Seq:         uint32(rule.Sequence), //nolint:gosec
				Action:      action,
				Source:      rule.SourceAddress.Prefix,
				Destination: rule.DestinationAddress.Prefix,
			}
		}

		a := &acl.ACL{
			Name:  item.Name,
			Rules: rules,
		}

		if err := s.GNMI.Update(ctx, a); err != nil {
			return fmt.Errorf("failed to update ACL %s: %w", item.Name, err)
		}
	}

	return nil
}

type Trustpoints struct {
	Spec   *v1alpha1.PKI
	DryRun bool
}

func (step *Trustpoints) Name() string { return "Trustpoints" }

func (step *Trustpoints) Deps() []client.ObjectKey {
	if step.Spec == nil {
		return nil
	}
	keys := make([]client.ObjectKey, 0, len(step.Spec.Certificates))
	for _, trustpoint := range step.Spec.Certificates {
		keys = append(keys, client.ObjectKey{
			Namespace: trustpoint.Source.SecretRef.Namespace,
			Name:      trustpoint.Source.SecretRef.Name,
		})
	}
	return keys
}

func (step *Trustpoints) Exec(ctx context.Context, s *Scope) error {
	if step.Spec == nil {
		return nil
	}
	for _, trustpoint := range step.Spec.Certificates {
		tp := &crypto.Trustpoint{ID: trustpoint.Name}
		if err := s.GNMI.Update(ctx, tp); err != nil {
			return fmt.Errorf("failed to get trustpoint %s: %w", trustpoint.Name, err)
		}
		cert, err := s.Client.Certificate(ctx, trustpoint.Source.SecretRef)
		if err != nil {
			return fmt.Errorf("failed to get trustpoint certificate from secret: %w", err)
		}
		key, ok := cert.PrivateKey.(*rsa.PrivateKey)
		if !ok {
			return fmt.Errorf("unsupported private key type: expected *rsa.PrivateKey, got %T", cert.PrivateKey)
		}
		c := &crypto.Certificate{Key: key, Cert: cert.Leaf}
		if err := c.Load(ctx, s.Conn, trustpoint.Name); err != nil {
			return fmt.Errorf("failed to load trustpoint certificate: %w", err)
		}
	}
	return nil
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

type SNMP struct{ Spec *v1alpha1.SNMP }

func (step *SNMP) Name() string             { return "SNMP" }
func (step *SNMP) Deps() []client.ObjectKey { return nil }
func (step *SNMP) Exec(ctx context.Context, sc *Scope) error {
	s := &snmp.SNMP{Enable: false}
	if step.Spec != nil {
		s = &snmp.SNMP{
			Enable:      true,
			Contact:     step.Spec.Contact,
			Location:    step.Spec.Location,
			SrcIf:       step.Spec.SrcIf,
			Hosts:       make([]*snmp.Host, len(step.Spec.Destinations)),
			Communities: make([]*snmp.Community, len(step.Spec.Communities)),
			Traps:       step.Spec.Traps,
		}

		for i, h := range step.Spec.Destinations {
			var version snmp.Version
			switch h.Type {
			case "v1":
				version = snmp.V1
			case "v2c":
				version = snmp.V2c
			case "v3":
				version = snmp.V3
			default:
				return fmt.Errorf("unsupported SNMP version: %s", h.Type)
			}

			s.Hosts[i] = &snmp.Host{
				Address:   h.Address,
				Type:      h.Type,
				Version:   version,
				Community: h.Target,
				Vrf:       h.NetworkInstance,
			}
		}

		for i, c := range step.Spec.Communities {
			s.Communities[i] = &snmp.Community{
				Name:  c.Name,
				Group: c.Group,
				ACL:   c.ACL,
			}
		}
	}

	return sc.GNMI.Update(ctx, s)
}

type Logging struct {
	Spec            *v1alpha1.Logging
	OriginID        string
	SrcIf           string
	HistorySize     string
	HistoryLevel    string
	DefaultSeverity string
}

func (step *Logging) Name() string             { return "Logging" }
func (step *Logging) Deps() []client.ObjectKey { return nil }
func (step *Logging) Exec(ctx context.Context, s *Scope) error {
	severity := func(s v1alpha1.Severity) logging.SeverityLevel {
		switch s {
		case v1alpha1.SeverityEmergency:
			return logging.Emergency
		case v1alpha1.SeverityAlert:
			return logging.Alert
		case v1alpha1.SeverityCritical:
			return logging.Critical
		case v1alpha1.SeverityError:
			return logging.Error
		case v1alpha1.SeverityWarning:
			return logging.Warning
		case v1alpha1.SeverityNotice:
			return logging.Notice
		case v1alpha1.SeverityInfo:
			return logging.Informational
		case v1alpha1.SeverityDebug:
			return logging.Debug
		default:
			return logging.Informational
		}
	}

	historySize, err := strconv.Atoi(step.HistorySize)
	if err != nil {
		ctrl.LoggerFrom(ctx).Error(err, "Failed to parse history size", "HistorySize", step.HistorySize)
		historySize = 500
	}

	l := &logging.Logging{Enable: false}
	if step.Spec != nil {
		l = &logging.Logging{
			Enable:          true,
			OriginID:        step.OriginID,
			SrcIf:           step.SrcIf,
			Servers:         make([]*logging.SyslogServer, len(step.Spec.Servers)),
			History:         logging.History{Size: uint32(historySize), Severity: severity(v1alpha1.Severity(step.HistoryLevel))}, //nolint:gosec
			DefaultSeverity: severity(v1alpha1.Severity(step.DefaultSeverity)),
			Facilities:      make([]*logging.Facility, len(step.Spec.Facilities)),
		}

		for i, s := range step.Spec.Servers {
			l.Servers[i] = &logging.SyslogServer{
				Host:  s.Address,
				Port:  uint32(s.Port), //nolint:gosec
				Proto: logging.UDP,
				Vrf:   s.NetworkInstance,
				Level: severity(s.Severity),
			}
		}

		for i, f := range step.Spec.Facilities {
			l.Facilities[i] = &logging.Facility{
				Name:     f.Name,
				Severity: severity(f.Severity),
			}
		}
	}

	return s.GNMI.Update(ctx, l)
}

type VLAN struct{ LongName bool }

func (step *VLAN) Name() string             { return "VLAN" }
func (step *VLAN) Deps() []client.ObjectKey { return nil }
func (step *VLAN) Exec(ctx context.Context, s *Scope) error {
	v := &vlan.VLAN{LongName: step.LongName}
	return s.GNMI.Update(ctx, v)
}

type Features struct{ Spec []string }

func (step *Features) Name() string             { return "Features" }
func (step *Features) Deps() []client.ObjectKey { return nil }
func (step *Features) Exec(ctx context.Context, s *Scope) error {
	return s.GNMI.Update(ctx, feat.Features(step.Spec))
}

type DNS struct{ Spec *v1alpha1.DNS }

func (step *DNS) Name() string             { return "DNS" }
func (step *DNS) Deps() []client.ObjectKey { return nil }
func (step *DNS) Exec(ctx context.Context, s *Scope) error {
	d := &dns.DNS{Enable: false}
	if step.Spec != nil {
		d = &dns.DNS{
			Enable:     true,
			DomainName: step.Spec.Domain,
			Providers:  make([]*dns.Provider, len(step.Spec.Servers)),
		}
		for i, p := range step.Spec.Servers {
			d.Providers[i] = &dns.Provider{Addr: p.Address, Vrf: p.NetworkInstance, SrcIf: step.Spec.SrcIf}
		}
	}
	return s.GNMI.Update(ctx, d)
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

type Banner struct{ Spec *v1alpha1.TemplateSource }

func (step *Banner) Name() string { return "Banner" }
func (step *Banner) Deps() []client.ObjectKey {
	if step.Spec == nil {
		return nil
	}
	if step.Spec.SecretRef != nil {
		return []client.ObjectKey{
			{
				Name: step.Spec.SecretRef.Name,
			},
		}
	}
	// TODO(felix-kaestner): Support ConfigMap references
	return nil
}

func (step *Banner) Exec(ctx context.Context, s *Scope) error {
	if step.Spec == nil {
		return nil
	}
	message, err := s.Client.Template(ctx, step.Spec)
	if err != nil {
		return err
	}
	b := &banner.Banner{Message: string(message), Delimiter: "^"}
	return s.GNMI.Update(ctx, b)
}

func init() {
	provider.Register("cisco-nxos-gnmi", &Provider{})
}
