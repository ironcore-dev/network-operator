// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package openconfig

import (
	"context"
	"fmt"
	"time"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/apistatus"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

var _ provider.ManagementAccessProvider = (*Provider)(nil)

const grpcServerName = "gnmi"

func (p *Provider) EnsureManagementAccess(ctx context.Context, req *provider.EnsureManagementAccessRequest) error {
	ma := req.ManagementAccess
	if err := validateManagementAccessSpec(ma.Spec); err != nil {
		return err
	}
	grpcServer := &GRPCServer{
		Name:            grpcServerName,
		Enable:          ma.Spec.GRPC.Enabled,
		Port:            ma.Spec.GRPC.Port,
		CertificateID:   ma.Spec.GRPC.CertificateID,
		NetworkInstance: ma.Spec.GRPC.VrfName,
	}
	sshServer := &SSHServer{
		Enable:  ma.Spec.SSH.Enabled,
		Timeout: uint32(ma.Spec.SSH.Timeout.Seconds()),
	}
	return p.client.Update(ctx, grpcServer, sshServer)
}

func (p *Provider) DeleteManagementAccess(ctx context.Context) error {
	return p.client.Delete(ctx, &GRPCServer{Name: grpcServerName}, &SSHServer{})
}

func validateManagementAccessSpec(spec v1alpha1.ManagementAccessSpec) error {
	var violations []apistatus.FieldViolation
	gnmi := spec.GRPC.GNMI
	if gnmi.MaxConcurrentCall != 8 || gnmi.KeepAliveTimeout.Duration != 10*time.Minute {
		violations = append(violations, apistatus.FieldViolation{
			Field:       "spec.grpc.gnmi",
			Description: "gnmi configuration is not supported by the OpenConfig gRPC server model",
		})
	}
	if spec.SSH.SessionLimit != 32 {
		violations = append(violations, apistatus.FieldViolation{
			Field:       "spec.ssh.sessionLimit",
			Description: "sessionLimit is not supported by the OpenConfig SSH server model",
		})
	}
	if len(violations) > 0 {
		return apistatus.NewUnsupportedFieldError(violations...)
	}
	return nil
}

// Compile-time assertions.
var (
	_ gnmiext.DataElement = (*GRPCServer)(nil)
	_ gnmiext.DataElement = (*SSHServer)(nil)
)

// GRPCServer targets the OpenConfig grpc-server list item config container.
type GRPCServer struct {
	Name            string `json:"name"`
	Enable          bool   `json:"enable"`
	Port            int32  `json:"port"`
	CertificateID   string `json:"certificate-id,omitempty"`
	NetworkInstance string `json:"network-instance,omitempty"`
}

func (g *GRPCServer) XPath() string {
	return fmt.Sprintf("openconfig-system:system/grpc-servers/grpc-server[name=%s]/config", g.Name)
}

// SSHServer targets the OpenConfig ssh-server config container.
type SSHServer struct {
	Enable  bool   `json:"enable"`
	Timeout uint32 `json:"timeout,omitempty"`
}

func (*SSHServer) XPath() string {
	return "openconfig-system:system/ssh-server/config"
}
