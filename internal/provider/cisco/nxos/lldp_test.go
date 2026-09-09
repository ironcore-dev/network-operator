// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"context"
	"slices"
	"testing"

	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

func init() {
	Register("lldp", &LLDP{
		HoldTime:  200,
		InitDelay: 5,
	})

	items := new(LLDPIfItems)
	items.IfList.Set(&LLDPIfItem{
		InterfaceName: "eth7/1",
		AdminRxSt:     NewOption(AdminStDisabled),
		AdminTxSt:     NewOption(AdminStDisabled),
	})
	items.IfList.Set(&LLDPIfItem{
		InterfaceName: "eth8/1",
		AdminTxSt:     NewOption(AdminStDisabled),
	})
	Register("lldp_if_items", items)
}

// TestDeleteLLDPResetsConfiguration verifies that deletion resets LLDP configuration before disabling the feature.
func TestDeleteLLDPResetsConfiguration(t *testing.T) {
	lldp := new(LLDP)
	lldp.Default()
	if lldp.HoldTime != defaultLLDPHoldTime || lldp.InitDelay != defaultLLDPInitDelay {
		t.Fatalf("LLDP.Default() = %#v, want platform defaults", lldp)
	}

	var paths []string
	client := &gnmiext.ClientMock{
		DeleteFunc: func(_ context.Context, elements ...gnmiext.DataElement) error {
			for _, element := range elements {
				paths = append(paths, "delete "+element.XPath())
			}
			return nil
		},
		UpdateFunc: func(_ context.Context, elements ...gnmiext.DataElement) error {
			for _, element := range elements {
				paths = append(paths, "update "+element.XPath())
			}
			return nil
		},
	}

	p := &Provider{client: client}

	paths = nil
	if err := p.DeleteLLDP(t.Context()); err != nil {
		t.Fatalf("DeleteLLDP() error = %v", err)
	}
	want := []string{
		"delete " + new(LLDP).XPath(),
		"update " + (&Feature{Name: "lldp"}).XPath(),
	}
	if !slices.Equal(paths, want) {
		t.Errorf("DeleteLLDP() operations = %v, want %v", paths, want)
	}
}
