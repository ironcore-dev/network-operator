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
	lldp := &LLDP{
		HoldTime:  200,
		InitDelay: 5,
		PCEnable:  AdminStDisabled,
	}
	lldp.OptTlvSel.Default()
	Register("lldp", lldp)

	items := new(LLDPIfItems)
	items.IfList.Set(&LLDPIfItem{
		InterfaceName: "eth7/1",
		AdminRxSt:     AdminStDisabled,
		AdminTxSt:     AdminStDisabled,
	})
	items.IfList.Set(&LLDPIfItem{
		InterfaceName: "eth8/1",
		AdminRxSt:     AdminStEnabled,
		AdminTxSt:     AdminStDisabled,
	})
	Register("lldp_if_items", items)
}

func TestLLDPOptTLVBitmask(t *testing.T) {
	var platformDefault LLDPOptTLV
	platformDefault.Default()
	t.Run("UnmarshalText and String roundtrip for platform default", func(t *testing.T) {
		var parsed LLDPOptTLV
		if err := parsed.UnmarshalText([]byte(platformDefault.String())); err != nil {
			t.Fatalf("UnmarshalText() error = %v", err)
		}
		if parsed != platformDefault {
			t.Fatalf("roundtrip mismatch: got %d, want %d", parsed, platformDefault)
		}
	})
	t.Run("Unmarshal individual TLVs", func(t *testing.T) {
		var got LLDPOptTLV
		if err := got.UnmarshalText([]byte("port-desc,sys-name")); err != nil {
			t.Fatalf("UnmarshalText() error = %v", err)
		}
		want := LLDPOptTLVPortDesc | LLDPOptTLVSysName
		if got != want {
			t.Fatalf("UnmarshalText() = %d, want %d", got, want)
		}
	})
	t.Run("Unknown token returns error", func(t *testing.T) {
		var got LLDPOptTLV
		if err := got.UnmarshalText([]byte("port-desc,bogus,sys-name")); err == nil {
			t.Fatal("expected error for unknown token")
		}
	})
	t.Run("Bitwise operations", func(t *testing.T) {
		m := LLDPOptTLVPortDesc | LLDPOptTLVSysName
		if m&LLDPOptTLVPortDesc == 0 {
			t.Fatal("expected PortDesc bit set")
		}
		if m&LLDPOptTLVDcbxp != 0 {
			t.Fatal("expected Dcbxp bit unset")
		}
		m |= LLDPOptTLVDcbxp
		if m&LLDPOptTLVDcbxp == 0 {
			t.Fatal("expected Dcbxp bit set after OR")
		}
		m &^= LLDPOptTLVPortDesc
		if m&LLDPOptTLVPortDesc != 0 {
			t.Fatal("expected PortDesc bit unset after AND-NOT")
		}
	})
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
