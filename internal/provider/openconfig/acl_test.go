// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package openconfig

import (
	"testing"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/apistatus"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

func TestEnsureACLRejectsMixedSourceAndDestinationAddressFamilies(t *testing.T) {
	p := &Provider{client: &gnmiext.ClientMock{}}

	err := p.EnsureACL(t.Context(), &provider.ACLRequest{ACL: &v1alpha1.AccessControlList{
		Spec: v1alpha1.AccessControlListSpec{
			Name: "test",
			Entries: []v1alpha1.ACLEntry{{
				Sequence:           10,
				Action:             v1alpha1.ActionPermit,
				Protocol:           v1alpha1.ProtocolIP,
				SourceAddress:      v1alpha1.MustParsePrefix("192.0.2.0/24"),
				DestinationAddress: v1alpha1.MustParsePrefix("2001:db8::/64"),
			}},
		},
	}})
	statusErr, ok := apistatus.FromError(err)
	if !ok || statusErr.Code != apistatus.CodeInvalidArgument {
		t.Fatalf("EnsureACL() error = %v, want InvalidArgument", err)
	}
}

func TestEnsureACLRejectsMixedEntryAddressFamilies(t *testing.T) {
	p := &Provider{client: &gnmiext.ClientMock{}}

	err := p.EnsureACL(t.Context(), &provider.ACLRequest{ACL: &v1alpha1.AccessControlList{
		Spec: v1alpha1.AccessControlListSpec{
			Name: "test",
			Entries: []v1alpha1.ACLEntry{
				{
					Sequence:           10,
					Action:             v1alpha1.ActionPermit,
					Protocol:           v1alpha1.ProtocolIP,
					SourceAddress:      v1alpha1.MustParsePrefix("192.0.2.0/24"),
					DestinationAddress: v1alpha1.MustParsePrefix("198.51.100.0/24"),
				},
				{
					Sequence:           20,
					Action:             v1alpha1.ActionPermit,
					Protocol:           v1alpha1.ProtocolIP,
					SourceAddress:      v1alpha1.MustParsePrefix("2001:db8::/64"),
					DestinationAddress: v1alpha1.MustParsePrefix("2001:db8:1::/64"),
				},
			},
		},
	}})
	statusErr, ok := apistatus.FromError(err)
	if !ok || statusErr.Code != apistatus.CodeInvalidArgument {
		t.Fatalf("EnsureACL() error = %v, want InvalidArgument", err)
	}
}
