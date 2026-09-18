// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

func init() {
	var vty VTY
	vty.ExecTmeoutItems.Timeout = int(time.Hour.Minutes())
	vty.SsLmtItems.SesLmt = 8

	Register("console", &Console{Timeout: int(time.Hour.Minutes())})
	Register("vty_acl", &VTYAccessClass{Name: "TEST-ACL"})
	Register("vty", &vty)
}

// TestDeleteManagementAccessDeletesAccessClass verifies that deletion cleans up the VTY access class outside the terminal subtree.
func TestDeleteManagementAccessDeletesAccessClass(t *testing.T) {
	var paths []string
	client := &gnmiext.ClientMock{
		DeleteFunc: func(_ context.Context, elements ...gnmiext.DataElement) error {
			for _, element := range elements {
				paths = append(paths, element.XPath())
			}
			return nil
		},
	}
	p := &Provider{client: client}

	if err := p.DeleteManagementAccess(t.Context(), new(provider.DeleteManagementAccessRequest)); err != nil {
		t.Fatalf("DeleteManagementAccess() error = %v", err)
	}
	want := new(VTYAccessClass).XPath()
	if !slices.Contains(paths, want) {
		t.Errorf("DeleteManagementAccess() paths = %v, want %q", paths, want)
	}
}
