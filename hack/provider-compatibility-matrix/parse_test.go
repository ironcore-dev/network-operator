// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"reflect"
	"sort"
	"testing"

	"golang.org/x/tools/go/packages"
)

// loadTestPkg type-checks src as a standalone package for the collectors. An
// apistatus stub is prepended so the constructors resolve without the real dep.
func loadTestPkg(t *testing.T, src string) *packages.Package {
	t.Helper()
	const apistatusStub = `
type FieldViolation struct{ Field, Description string }
type statusError struct{}
func (statusError) Error() string { return "" }
func NewUnsupportedFieldError(...FieldViolation) error { return statusError{} }
func NewInvalidArgumentError(...FieldViolation) error { return statusError{} }
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "src.go", "package p\nimport \"fmt\"\nvar _ = fmt.Sprintf\n"+apistatusStub+src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	info := &types.Info{
		Defs: map[*ast.Ident]types.Object{},
		Uses: map[*ast.Ident]types.Object{},
	}
	conf := types.Config{Importer: importer.Default()}
	typ, err := conf.Check("p", fset, []*ast.File{file}, info)
	if err != nil {
		t.Fatalf("typecheck: %v", err)
	}
	return &packages.Package{Syntax: []*ast.File{file}, TypesInfo: info, Types: typ, Fset: fset}
}

func TestCollectViolations(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want map[string][]string
	}{
		{
			name: "style 1 inline literal",
			src: `
type Provider struct{}
func (Provider) EnsureBGP() error {
	return NewUnsupportedFieldError(FieldViolation{Field: "spec.adminState"})
}`,
			want: map[string][]string{"BGP": {"spec.adminState"}},
		},
		{
			name: "style 2 slice-collected via helper",
			src: `
type Provider struct{}
func (Provider) EnsureDNS() error { return validateDNS() }
func validateDNS() error {
	var v []FieldViolation
	v = append(v, FieldViolation{Field: "spec.adminState"})
	v = append(v, FieldViolation{Field: "spec.sourceInterfaceName"})
	return NewUnsupportedFieldError(v...)
}`,
			want: map[string][]string{"DNS": {"spec.adminState", "spec.sourceInterfaceName"}},
		},
		{
			name: "mixed constructors: invalid-argument fields excluded",
			src: `
type Provider struct{}
func (Provider) EnsureInterface() error {
	if false {
		return NewInvalidArgumentError(FieldViolation{Field: "spec.encapsulation"})
	}
	return NewUnsupportedFieldError(FieldViolation{Field: "spec.type"})
}`,
			want: map[string][]string{"Interface": {"spec.type"}},
		},
		{
			name: "transitive method-to-method chain",
			src: `
type Provider struct{}
func (p Provider) EnsureInterface() error { return p.ensureSub() }
func (Provider) ensureSub() error {
	return NewUnsupportedFieldError(FieldViolation{Field: "spec.encapsulation.type"})
}`,
			want: map[string][]string{"Interface": {"spec.encapsulation.type"}},
		},
		{
			name: "fmt.Sprintf field path has verbs stripped",
			src: `
type Provider struct{}
func (Provider) EnsureDNS() error {
	return NewUnsupportedFieldError(FieldViolation{Field: fmt.Sprintf("spec.servers[%s].vrfName", "x")})
}`,
			want: map[string][]string{"DNS": {"spec.servers[].vrfName"}},
		},
		{
			name: "style 2 guard: helper mixing invalid-argument captures nothing",
			src: `
type Provider struct{}
func (Provider) EnsureVLAN() error { return validateVLAN() }
func validateVLAN() error {
	var v []FieldViolation
	v = append(v, FieldViolation{Field: "spec.badInput"})
	if false {
		return NewInvalidArgumentError(v...)
	}
	return NewUnsupportedFieldError(v...)
}`,
			// The helper contains a NewInvalidArgumentError call, so the STYLE 2
			// slice-harvest is suppressed to avoid misattribution.
			want: map[string][]string{"VLAN": nil},
		},
		{
			name: "cycle is safe",
			src: `
type Provider struct{}
func (p Provider) EnsureBGP() error { return p.a() }
func (p Provider) a() error { return p.b() }
func (p Provider) b() error {
	_ = p.a()
	return NewUnsupportedFieldError(FieldViolation{Field: "spec.x"})
}`,
			want: map[string][]string{"BGP": {"spec.x"}},
		},
		{
			name: "closure with invalid-argument does not suppress outer style 2",
			src: `
type Provider struct{}
func (Provider) EnsureBGP() error {
	var v []FieldViolation
	v = append(v, FieldViolation{Field: "spec.mode"})
	_ = func() { NewInvalidArgumentError(FieldViolation{Field: "spec.bad"}) }
	return NewUnsupportedFieldError(v...)
}`,
			want: map[string][]string{"BGP": {"spec.mode"}},
		},
	}
	kinds := []string{"BGP", "DNS", "Interface", "VLAN"}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := loadTestPkg(t, tt.src)
			got := collectViolations(pkg, kinds)
			for kind, wantFields := range tt.want {
				var gotFields []string
				for f := range got[kind] {
					gotFields = append(gotFields, f)
				}
				sort.Strings(gotFields)
				sort.Strings(wantFields)
				if len(gotFields) == 0 && len(wantFields) == 0 {
					continue
				}
				if !reflect.DeepEqual(gotFields, wantFields) {
					t.Errorf("kind %s: got %v, want %v", kind, gotFields, wantFields)
				}
			}
		})
	}
}

func TestStripVerbs(t *testing.T) {
	tests := []struct{ in, want string }{
		{"spec.servers[%s].vrfName", "spec.servers[].vrfName"},
		{"spec.x[%d].y[%d]", "spec.x[].y[]"},
		{"spec.plain", "spec.plain"},
		{"a%[1]vb", "ab"},
		{"100%% done", "100% done"},
	}
	for _, tt := range tests {
		if got := stripVerbs(tt.in); got != tt.want {
			t.Errorf("stripVerbs(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
