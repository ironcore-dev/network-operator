// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iface

import (
	"math"
	"net/netip"
	"reflect"
	"testing"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

func mustParsePrefixes(t *testing.T, ss []string) []netip.Prefix {
	t.Helper()
	var out []netip.Prefix
	for _, s := range ss {
		p, err := netip.ParsePrefix(s)
		if err != nil {
			t.Fatalf("failed to parse prefix %q: %v", s, err)
		}
		out = append(out, p)
	}
	return out
}

func TestL3Config_ConflictingAddressingModes(t *testing.T) {
	t.Run("Unnumbered after Numbered", func(t *testing.T) {
		c, err := NewL3Config(
			WithMedium(L3MediumTypeP2P),
			WithNumberedAddressingIPv4([]string{"10.0.0.1/24"}),
			WithMedium(L3MediumTypeP2P),
			WithUnnumberedAddressing("lo0"),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if c.addressingMode != AddressingModeUnnumbered {
			t.Errorf("expected AddressingModeUnnumbered, got %v", c.addressingMode)
		}
		if c.unnumberedLoopback != "lo0" {
			t.Errorf("expected addressesInterface to be 'lo0', got %s", c.unnumberedLoopback)
		}
		if len(c.prefixesIPv4) != 0 {
			t.Errorf("expected no IPv4 addresses, got %v", c.prefixesIPv4)
		}
		if len(c.prefixesIPv6) != 0 {
			t.Errorf("expected no IPv6 addresses, got %v", c.prefixesIPv6)
		}
	})
	t.Run("Numbered after Unnumbered", func(t *testing.T) {
		c, err := NewL3Config(
			WithMedium(L3MediumTypeP2P),
			WithUnnumberedAddressing("lo0"),
			WithNumberedAddressingIPv4([]string{"10.0.0.1/24"}),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if c.addressingMode != AddressingModeNumbered {
			t.Errorf("expected AddressingModeNumbered, got %v", c.addressingMode)
		}
		if c.unnumberedLoopback != "" {
			t.Errorf("expected no addressesInterface, got %s", c.unnumberedLoopback)
		}
		if len(c.prefixesIPv4) != 1 || c.prefixesIPv4[0].String() != "10.0.0.1/24" {
			t.Errorf("expected IPv4 addresses to be 10.0.0.1/24, got %v", c.prefixesIPv4)
		}
		if len(c.prefixesIPv6) != 0 {
			t.Errorf("expected no IPv6 addresses, got %v", c.prefixesIPv6)
		}
	})
}

func TestWithNumberedAddressing_IPv4(t *testing.T) {
	cfg, err := NewL3Config(WithNumberedAddressingIPv4([]string{"10.0.0.1/24", "10.0.1.1/24"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Run("TestWithNumberedAddressing_IPv4_config_valid", func(t *testing.T) {
		if cfg.addressingMode != AddressingModeNumbered {
			t.Errorf("expected AddressingModeNumbered, got %v", cfg.addressingMode)
		}
		expected := mustParsePrefixes(t, []string{"10.0.0.1/24", "10.0.1.1/24"})
		if !reflect.DeepEqual(cfg.prefixesIPv4, expected) {
			t.Errorf("expected addresses %v, got %v", expected, cfg.prefixesIPv4)
		}
	})
	t.Run("TestWithNumberedAddressing_IPv4_config_invalid", func(t *testing.T) {
		_, err = NewL3Config(WithNumberedAddressingIPv4([]string{"10.0.0.1/24", "266.266.266.266/24"}))
		if err == nil {
			t.Error("expected error for invalid IPv4 address, got nil")
		}
	})
}

func TestL3Config_EmptyAddressList(t *testing.T) {
	t.Run("WithNumberedAddressingIPv4_empty", func(t *testing.T) {
		_, err := NewL3Config(WithNumberedAddressingIPv4([]string{}))
		if err == nil {
			t.Error("expected error for empty IPv4 address list, got nil")
		}
	})
	t.Run("WithNumberedAddressingIPv6_empty", func(t *testing.T) {
		_, err := NewL3Config(WithNumberedAddressingIPv6([]string{}))
		if err == nil {
			t.Error("expected error for empty IPv6 address list, got nil")
		}
	})
}

func TestL3Config_OverlapAddresses(t *testing.T) {
	t.Run("WithNumberedAddressingIPv4_duplicates", func(t *testing.T) {
		_, err := NewL3Config(WithNumberedAddressingIPv4([]string{"10.0.0.1/24", "10.0.0.1/24"}))
		if err == nil {
			t.Fatal("expected error for duplicate IPv4 addresses, got nil")
		}
	})
	t.Run("WithNumberedAddressingIPv4_overlap", func(t *testing.T) {
		_, err := NewL3Config(WithNumberedAddressingIPv4([]string{"10.0.0.1/24", "10.0.0.1/8"}))
		if err == nil {
			t.Fatal("expected error for overlapping IPv4 addresses, got nil")
		}
	})
	t.Run("WithNumberedAddressingIPv6_duplicates", func(t *testing.T) {
		_, err := NewL3Config(WithNumberedAddressingIPv6([]string{"2001:db8::1/64", "2001:db8::1/64"}))
		if err == nil {
			t.Fatal("expected error for duplicate IPv6 addresses, got nil")
		}
	})
	t.Run("WithNumberedAddressingIPv6_overlap", func(t *testing.T) {
		_, err := NewL3Config(WithNumberedAddressingIPv6([]string{"2001:db8::1/64", "2001:db8::2/32"}))
		if err == nil {
			t.Fatal("expected error for overlapping IPv6 addresses, got nil")
		}
	})
}

func TestWithNumberedAddressing_IPv6(t *testing.T) {
	cfg, err := NewL3Config(WithNumberedAddressingIPv6([]string{"2001:db8::1/64", "2002:db8::1/64"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Run("TestWithNumberedAddressing_IPv6_config_valid", func(t *testing.T) {
		if cfg.addressingMode != AddressingModeNumbered {
			t.Errorf("expected AddressingModeNumbered, got %v", cfg.addressingMode)
		}
		expected := mustParsePrefixes(t, []string{"2001:db8::1/64", "2002:db8::1/64"})
		if !reflect.DeepEqual(cfg.prefixesIPv6, expected) {
			t.Errorf("expected addresses %v, got %v", expected, cfg.prefixesIPv6)
		}
	})
	t.Run("TestWithNumberedAddressing_IPv6_config_invalid", func(t *testing.T) {
		_, err = NewL3Config(WithNumberedAddressingIPv6([]string{"2001:db8::1/64", "zzzz:db8::1/64"}))
		if err == nil {
			t.Error("expected error for invalid IPv6 address, got nil")
		}
	})
}

func TestWithNumberedAddressing_IPv4AndIPv6(t *testing.T) {
	cfg, err := NewL3Config(
		WithNumberedAddressingIPv4([]string{"10.0.0.1/24", "10.0.1.1/24"}),
		WithNumberedAddressingIPv6([]string{"2001:db8::1/64", "2002:db8::1/64"}),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Run("TestWithNumberedAddressing_IPv4AndIPv6_config", func(t *testing.T) {
		expectedV4 := mustParsePrefixes(t, []string{"10.0.0.1/24", "10.0.1.1/24"})
		expectedV6 := mustParsePrefixes(t, []string{"2001:db8::1/64", "2002:db8::1/64"})
		if !reflect.DeepEqual(cfg.prefixesIPv4, expectedV4) {
			t.Errorf("expected IPv4 addresses %v, got %v", expectedV4, cfg.prefixesIPv4)
		}
		if !reflect.DeepEqual(cfg.prefixesIPv6, expectedV6) {
			t.Errorf("expected IPv6 addresses %v, got %v", expectedV6, cfg.prefixesIPv6)
		}
		if cfg.addressingMode != AddressingModeNumbered {
			t.Errorf("expected AddressingModeNumbered, got %v", cfg.addressingMode)
		}
		if cfg.unnumberedLoopback != "" {
			t.Error("expected no interface for numbered addressing, got", cfg.unnumberedLoopback)
		}
	})

	t.Run("TestWithNumberedAddressing_IPv4AndIPv6_updates", func(t *testing.T) {
		updates, err := cfg.ToYGOT("eth1/1", "default")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		foundAllV4, foundAllV6 := math.MaxInt32, math.MaxInt32
		for _, u := range updates {
			if ru, ok := u.(gnmiext.ReplacingUpdate); ok {
				if ifList, ok := ru.Value.(*nxos.Cisco_NX_OSDevice_System_Ipv4Items_InstItems_DomItems_DomList_IfItems_IfList); ok {
					// check we have the correct number of IPv4 addresses
					addrItems := ifList.GetAddrItems()
					if addrItems == nil || len(addrItems.AddrList) != len(cfg.prefixesIPv4) {
						t.Errorf("expected %d IPv4 address, got %d", len(cfg.prefixesIPv4), len(addrItems.AddrList))
					}
					foundAllV4 = len(addrItems.AddrList)
					// check if the expected IPv4 address is present
					for _, prefix := range cfg.prefixesIPv4 {
						if ifList.GetAddrItems().GetAddrList(prefix.String()) != nil {
							foundAllV4--
						}
					}
				}
				if ifList, ok := ru.Value.(*nxos.Cisco_NX_OSDevice_System_Ipv6Items_InstItems_DomItems_DomList_IfItems_IfList); ok {
					// check we have the correct number of IPv6 addresses
					addrItems := ifList.GetAddrItems()
					if addrItems == nil || len(addrItems.AddrList) != len(cfg.prefixesIPv6) {
						t.Errorf("expected 1 IPv6 address, got %d", len(addrItems.AddrList))
					}
					foundAllV6 = len(addrItems.AddrList)
					for _, prefix := range cfg.prefixesIPv6 {
						if ifList.GetAddrItems().GetAddrList(prefix.String()) != nil {
							foundAllV6--
						}
					}
				}
			}
			if foundAllV4 == 0 && foundAllV6 == 0 {
				break
			}
		}
		if foundAllV4 != 0 {
			t.Error("expected IPv4 address to be present in updates")
		}
		if foundAllV6 != 0 {
			t.Error("expected IPv6 address to be present in updates")
		}
	})
}

func TestWithUnnumberedAddressing(t *testing.T) {
	t.Run("Interface in P2P medium, valid loopback", func(t *testing.T) {
		cfg, err := NewL3Config(
			WithMedium(L3MediumTypeP2P),
			WithUnnumberedAddressing("lo0"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.addressingMode != AddressingModeUnnumbered {
			t.Errorf("expected AddressingModeUnnumbered, got %v", cfg.addressingMode)
		}
		expected := "lo0"
		if !reflect.DeepEqual(cfg.unnumberedLoopback, expected) {
			t.Errorf("expected addresses %v, got %v", expected, cfg.unnumberedLoopback)
		}
		if cfg.prefixesIPv4 != nil || cfg.prefixesIPv6 != nil {
			t.Error("expected no addresses set for unnumbered addressing")
		}
	})
	t.Run("Invalid Loopback Interface", func(t *testing.T) {
		_, err := NewL3Config(WithUnnumberedAddressing("eth1/1"))
		if err == nil {
			t.Error("expected error for invalid loopback interface name, got nil")
		}
	})
	t.Run("Interface not in P2P medium", func(t *testing.T) {
		_, err := NewL3Config(
			WithUnnumberedAddressing("lo0"),
		)
		if err == nil {
			t.Error("expected error for unnumbered addressing on non-P2P medium, got nil")
		}
	})
}

func TestWithMedium(t *testing.T) {
	t.Run("Valid Medium Type", func(t *testing.T) {
		cfg, err := NewL3Config(WithMedium(L3MediumTypeP2P))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.medium != L3MediumTypeP2P {
			t.Errorf("expected Medium %v, got %v", L3MediumTypeP2P, cfg.medium)
		}
	})
	t.Run("Invalid Medium Type", func(t *testing.T) {
		_, err := NewL3Config(WithMedium(L3MediumType(99)))
		if err == nil {
			t.Error("expected error for invalid medium type, got nil")
		}
	})
}

func TestToYGOT_Numbered(t *testing.T) {
	cfg, err := NewL3Config(
		WithNumberedAddressingIPv4([]string{"10.0.0.1/24"}),
		WithMedium(L3MediumTypeBroadcast),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	updates, err := cfg.ToYGOT("eth1/1", "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(updates) == 0 {
		t.Fatal("expected at least one update")
	}
}

func TestToYGOT_Unnumbered(t *testing.T) {
	t.Run("Unnumbered addressing", func(t *testing.T) {
		cfg, err := NewL3Config(
			WithMedium(L3MediumTypeP2P),
			WithUnnumberedAddressing("lo0"),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		updates, err := cfg.ToYGOT("eth1/1", "default")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(updates) == 0 {
			t.Fatal("expected at least one update")
		}
	})
	t.Run("Unnumbered addressing with medium not P2P", func(t *testing.T) {
		for _, medium := range []L3MediumType{L3MediumTypeBroadcast, L3MediumTypeUnset} {
			_, err := NewL3Config(
				WithMedium(medium),
				WithUnnumberedAddressing("lo0"),
			)
			if err == nil {
				t.Fatal("expected error for unnumbered addressing on non-P2P medium, got nil")
			}
		}
	})
}
