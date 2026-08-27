// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testMatrix() *matrix {
	return &matrix{
		kinds:   []string{"BGP", "DNS", "VLAN"},
		columns: []column{{name: "openconfig"}, {name: "cisco-nxos-gnmi"}},
		support: map[string]map[string]cell{
			"BGP": {
				"openconfig":      {implemented: true, unsupportedFields: []string{"spec.adminState"}},
				"cisco-nxos-gnmi": {implemented: true},
			},
			"DNS": {
				"openconfig":      {implemented: true, unsupportedFields: []string{"spec.a", "spec.b"}},
				"cisco-nxos-gnmi": {implemented: false},
			},
			"VLAN": {
				"openconfig":      {implemented: false},
				"cisco-nxos-gnmi": {implemented: true, unsupportedFields: []string{"spec.mode"}},
			},
		},
	}
}

func TestRenderNotesGroupedByProvider(t *testing.T) {
	out := testMatrix().render()
	ocIdx := strings.Index(out, "### openconfig")
	nxIdx := strings.Index(out, "### cisco-nxos-gnmi")
	if ocIdx < 0 || nxIdx < 0 {
		t.Fatalf("missing provider headings in Notes:\n%s", out)
	}
	if ocIdx > nxIdx {
		t.Errorf("provider headings out of column order: openconfig should precede cisco-nxos-gnmi")
	}

	for _, want := range []string{
		"<a id=\"note-bgp-openconfig\"></a>\n**BGP**",
		"<a id=\"note-dns-openconfig\"></a>\n**DNS**",
		"<a id=\"note-vlan-cisco-nxos-gnmi\"></a>\n**VLAN**",
		"- `spec.adminState`",
		"- `spec.a`",
		"- `spec.b`",
		"- `spec.mode`",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("render output missing %q", want)
		}
	}
}

func TestRenderGlyphLinksAndFullCells(t *testing.T) {
	out := testMatrix().render()
	if !strings.Contains(out, "["+glyphPartial+"](#note-bgp-openconfig)") {
		t.Errorf("partial glyph not linked to note anchor:\n%s", out)
	}
	// A full cell renders the bare glyph with no link.
	if strings.Contains(out, "["+glyphFull+"](") {
		t.Errorf("full glyph should not be a link")
	}
}

func TestRenderResourceLinks(t *testing.T) {
	dir := t.TempDir()
	ref := filepath.Join(dir, "index.md")
	if err := os.WriteFile(ref, []byte("#### BGP\n\n#### DNS\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := apiRefAnchors(ref)
	if !got["bgp"] || !got["dns"] {
		t.Fatalf("apiRefAnchors missing headings: %v", got)
	}

	anchors := map[string]bool{"bgp": true}
	if link := resourceCell("BGP", anchors); link != "[BGP](/api-reference/#bgp)" {
		t.Errorf("linked kind = %q", link)
	}
	if plain := resourceCell("VLAN", anchors); plain != "VLAN" {
		t.Errorf("unmatched kind should be plain text, got %q", plain)
	}
}

func TestApiRefAnchorsMissingFile(t *testing.T) {
	if got := apiRefAnchors(filepath.Join(t.TempDir(), "nope.md")); len(got) != 0 {
		t.Errorf("missing file should yield empty set, got %v", got)
	}
}

func TestSlug(t *testing.T) {
	tests := []struct{ in, want string }{
		{"openconfig", "openconfig"},
		{"cisco-nxos-gnmi", "cisco-nxos-gnmi"},
		{"BGPPeer", "bgppeer"},
		{"a.b c", "a-b-c"},
	}
	for _, tt := range tests {
		if got := slug(tt.in); got != tt.want {
			t.Errorf("slug(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
