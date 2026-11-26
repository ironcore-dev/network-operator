// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package srlinux

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/apistatus"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

func TestASNToUint32(t *testing.T) {
	tests := []struct {
		name    string
		asn     intstr.IntOrString
		want    uint32
		wantErr bool
	}{
		{name: "int", asn: intstr.FromInt32(65000), want: 65000},
		{name: "plain string", asn: intstr.FromString("65000"), want: 65000},
		{name: "plain string max", asn: intstr.FromString("4294967295"), want: 4294967295},
		{name: "dotted", asn: intstr.FromString("1.10"), want: 65546},
		{name: "dotted max", asn: intstr.FromString("65535.65535"), want: 4294967295},
		{name: "dotted zero high part", asn: intstr.FromString("0.1"), want: 1},
		{name: "int zero", asn: intstr.FromInt32(0), wantErr: true},
		{name: "int negative", asn: intstr.FromInt32(-1), wantErr: true},
		{name: "string zero", asn: intstr.FromString("0"), wantErr: true},
		{name: "dotted zero", asn: intstr.FromString("0.0"), wantErr: true},
		{name: "empty", asn: intstr.FromString(""), wantErr: true},
		{name: "not a number", asn: intstr.FromString("abc"), wantErr: true},
		{name: "negative string", asn: intstr.FromString("-1"), wantErr: true},
		{name: "above uint32", asn: intstr.FromString("4294967296"), wantErr: true},
		{name: "dotted high part out of range", asn: intstr.FromString("65536.0"), wantErr: true},
		{name: "dotted low part out of range", asn: intstr.FromString("1.65536"), wantErr: true},
		{name: "dotted missing low part", asn: intstr.FromString("1."), wantErr: true},
		{name: "dotted missing high part", asn: intstr.FromString(".1"), wantErr: true},
		{name: "dotted three parts", asn: intstr.FromString("1.2.3"), wantErr: true},
		{name: "leading space", asn: intstr.FromString(" 1"), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := asnToUint32(tt.asn, "spec.test")
			if tt.wantErr {
				assertStatusCode(t, err, apistatus.CodeInvalidArgument, "spec.test")
				return
			}
			if err != nil {
				t.Fatalf("asnToUint32(%q) error = %v", tt.asn.String(), err)
			}
			if got != tt.want {
				t.Errorf("asnToUint32(%q) = %d, want %d", tt.asn.String(), got, tt.want)
			}
		})
	}
}

func TestToASPathAction(t *testing.T) {
	asn := func(s string) *intstr.IntOrString {
		v := intstr.Parse(s)
		return &v
	}
	useLastAS := int32(1)

	tests := []struct {
		name      string
		action    *v1alpha1.SetASPathAction
		want      *RoutingPolicyASPathAction
		wantCode  apistatus.Code
		wantField string
	}{
		{
			name:   "prepend",
			action: &v1alpha1.SetASPathAction{Prepend: &v1alpha1.SetASPathPrepend{ASNumber: asn("1.10")}},
			want:   &RoutingPolicyASPathAction{Prepend: &RoutingPolicyASPathPrepend{ASNumber: 65546, RepeatN: 1}},
		},
		{
			name:   "asNumber",
			action: &v1alpha1.SetASPathAction{ASNumber: asn("65500")},
			want:   &RoutingPolicyASPathAction{Replace: []uint32{65500}},
		},
		{
			name:      "prepend useLastAS is unsupported",
			action:    &v1alpha1.SetASPathAction{Prepend: &v1alpha1.SetASPathPrepend{UseLastAS: &useLastAS}},
			wantCode:  apistatus.CodeUnsupportedField,
			wantField: "spec.statements[].actions.bgpActions.setASPath.prepend.useLastAS",
		},
		{
			name:      "replace privateAS is unsupported",
			action:    &v1alpha1.SetASPathAction{Replace: &v1alpha1.SetASPathReplace{PrivateAS: true, Replacement: intstr.FromInt32(65100)}},
			wantCode:  apistatus.CodeUnsupportedField,
			wantField: "spec.statements[].actions.bgpActions.setASPath.replace",
		},
		{
			name:      "invalid prepend asNumber",
			action:    &v1alpha1.SetASPathAction{Prepend: &v1alpha1.SetASPathPrepend{ASNumber: asn("70000.1")}},
			wantCode:  apistatus.CodeInvalidArgument,
			wantField: "spec.statements[].actions.bgpActions.setASPath.prepend.asNumber",
		},
		{
			// SR Linux can only set the whole path; mapping this would drop the other AS numbers silently.
			name:      "replace of a given AS number is unsupported",
			action:    &v1alpha1.SetASPathAction{Replace: &v1alpha1.SetASPathReplace{ASNumber: asn("65001"), Replacement: intstr.FromString("65100")}},
			wantCode:  apistatus.CodeUnsupportedField,
			wantField: "spec.statements[].actions.bgpActions.setASPath.replace",
		},
		{
			name:      "invalid asNumber",
			action:    &v1alpha1.SetASPathAction{ASNumber: asn("4294967296")},
			wantCode:  apistatus.CodeInvalidArgument,
			wantField: "spec.statements[].actions.bgpActions.setASPath.asNumber",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := toASPathAction(tt.action)
			if tt.wantCode != 0 {
				assertStatusCode(t, err, tt.wantCode, tt.wantField)
				return
			}
			if err != nil {
				t.Fatalf("toASPathAction() error = %v", err)
			}
			gotJSON, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			wantJSON, err := json.Marshal(tt.want)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(gotJSON, wantJSON) {
				t.Errorf("toASPathAction() = %s, want %s", gotJSON, wantJSON)
			}
		})
	}
}

// TestRoutingPolicyJSON checks the payload against what SR Linux sends and expects: AS numbers are JSON
// numbers, and the list key is not part of the payload.
func TestRoutingPolicyJSON(t *testing.T) {
	// Shape of a keyed get of routing-policy/policy[name=p] on SR Linux 26.7.1: no "name" leaf.
	device := `{"statement":[{"name":"10","action":{"policy-result":"accept","bgp":{` +
		`"as-path":{"prepend":{"as-number":65000,"repeat-n":2},"replace":[65100,4200000000]},` +
		`"communities":{"replace":"rp-p-10"}}}}]}`

	var got RoutingPolicy
	if err := json.Unmarshal([]byte(device), &got); err != nil {
		t.Fatalf("unmarshal device payload: %v", err)
	}
	if len(got.Statement) != 1 || got.Statement[0].Name != "10" {
		t.Fatalf("statements = %+v, want only statement 10", got.Statement)
	}
	s := got.Statement[0]
	if p := s.Action.BGP.ASPath.Prepend; p.ASNumber != 65000 || p.RepeatN != 2 {
		t.Errorf("prepend = %+v, want as-number 65000 repeat-n 2", *p)
	}
	if r := s.Action.BGP.ASPath.Replace; !slices.Equal(r, []uint32{65100, 4200000000}) {
		t.Errorf("replace = %v, want [65100 4200000000]", r)
	}

	out, err := json.Marshal(&RoutingPolicy{Name: "p"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), `"name"`) {
		t.Errorf("policy payload %s contains the key leaf", out)
	}
	out, err = json.Marshal(&CommunitySet{Name: "rp-p-10", Member: []string{"65000:1"}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), `"name"`) {
		t.Errorf("community-set payload %s contains the key leaf", out)
	}
}

func TestEnsureRoutingPolicyStatementOrder(t *testing.T) {
	mock := &gnmiext.ClientMock{
		GetConfigFunc: func(context.Context, ...gnmiext.DataElement) error { return gnmiext.ErrNil },
		DoFunc:        func(context.Context, *gnmiext.SetBuilder) error { return nil },
	}
	p := &Provider{client: mock}

	// Out of order, and with sequences that sort differently as strings ("10" < "100" < "5").
	seqs := []int32{100, 5, 30, 10}
	stmts := make([]provider.PolicyStatement, 0, len(seqs))
	for _, seq := range seqs {
		stmts = append(stmts, provider.PolicyStatement{
			Sequence: seq,
			Actions:  v1alpha1.PolicyActions{RouteDisposition: v1alpha1.AcceptRoute},
		})
	}
	if err := p.EnsureRoutingPolicy(t.Context(), &provider.EnsureRoutingPolicyRequest{Name: "p", Statements: stmts}); err != nil {
		t.Fatalf("EnsureRoutingPolicy() error = %v", err)
	}

	calls := mock.DoCalls()
	if len(calls) != 1 {
		t.Fatalf("Do called %d times, want 1", len(calls))
	}
	var pol *RoutingPolicy
	calls[0].SetBuilder.Split(func(e gnmiext.DataElement) bool {
		if rp, ok := e.(*RoutingPolicy); ok {
			pol = rp
		}
		return false
	})
	if pol == nil {
		t.Fatal("Do did not write the policy")
	}
	got := make([]string, 0, len(pol.Statement))
	for _, s := range pol.Statement {
		got = append(got, s.Name)
	}
	if want := []string{"5", "10", "30", "100"}; !slices.Equal(got, want) {
		t.Errorf("statement order = %v, want %v", got, want)
	}

	// The payload must keep that order, and a device with the statements in another order must not compare
	// equal, so the client rewrites it.
	b, err := json.Marshal(pol)
	if err != nil {
		t.Fatal(err)
	}
	onDevice := RoutingPolicy{Name: pol.Name} // the client reads back into a copy that keeps the key
	if err := json.Unmarshal(b, &onDevice); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(pol, &onDevice) {
		t.Errorf("round trip changed the policy: %s", b)
	}
	onDevice.Statement[0], onDevice.Statement[1] = onDevice.Statement[1], onDevice.Statement[0]
	if reflect.DeepEqual(pol, &onDevice) {
		t.Error("policies with different statement order compare equal")
	}
}

// devicePolicy returns a GetConfig func that fills the policy as if the device had it, with a statement
// for each given community-set reference.
func devicePolicy(refs map[string]string) func(context.Context, ...gnmiext.DataElement) error {
	return func(_ context.Context, elems ...gnmiext.DataElement) error {
		pol := elems[0].(*RoutingPolicy)
		for stmt, set := range refs {
			pol.Statement = append(pol.Statement, &RoutingPolicyStatement{
				Name: stmt,
				Action: RoutingPolicyAction{
					PolicyResult: SRLPolicyResultAccept,
					BGP:          &RoutingPolicyActionBGP{Communities: &RoutingPolicyCommunities{Replace: set}},
				},
			})
		}
		return nil
	}
}

func xpaths(elems []gnmiext.DataElement) []string {
	out := make([]string, 0, len(elems))
	for _, e := range elems {
		out = append(out, e.XPath())
	}
	slices.Sort(out)
	return out
}

// setOps returns the sorted XPaths that a SetBuilder writes and deletes. The builder does not expose the
// operation mode outside gnmiext, so a community-set without members counts as a delete: the sets that
// EnsureRoutingPolicy writes always have members, and the orphans it deletes carry only their name.
func setOps(b *gnmiext.SetBuilder) (written, deleted []string) {
	b.Split(func(e gnmiext.DataElement) bool {
		if set, ok := e.(*CommunitySet); ok && len(set.Member) == 0 {
			deleted = append(deleted, e.XPath())
		} else {
			written = append(written, e.XPath())
		}
		return false
	})
	slices.Sort(written)
	slices.Sort(deleted)
	return written, deleted
}

func TestEnsureRoutingPolicy(t *testing.T) {
	errDevice := errors.New("device unavailable")
	ok := func(context.Context, *gnmiext.SetBuilder) error { return nil }

	req := &provider.EnsureRoutingPolicyRequest{
		Name: "p",
		Statements: []provider.PolicyStatement{{
			Sequence: 10,
			Actions: v1alpha1.PolicyActions{
				RouteDisposition: v1alpha1.AcceptRoute,
				BgpActions: &v1alpha1.BgpActions{
					SetCommunity: &v1alpha1.SetCommunityAction{Communities: []string{"65000:1"}},
				},
			},
		}},
	}

	written := []string{"routing-policy/community-set[name=rp-p-10]", "routing-policy/policy[name=p]"}
	tests := []struct {
		name        string
		getConfig   func(context.Context, ...gnmiext.DataElement) error
		do          func(context.Context, *gnmiext.SetBuilder) error
		wantErr     error
		wantSets    int
		wantWritten []string
		wantDeleted []string
	}{
		{
			name:        "policy absent: writes set and policy, deletes nothing",
			getConfig:   func(context.Context, ...gnmiext.DataElement) error { return gnmiext.ErrNil },
			do:          ok,
			wantSets:    1,
			wantWritten: written,
		},
		{
			name:        "policy unchanged: deletes nothing",
			getConfig:   devicePolicy(map[string]string{"10": "rp-p-10"}),
			do:          ok,
			wantSets:    1,
			wantWritten: written,
		},
		{
			name: "orphans: deletes only sets synthesized for this policy, in the same Set",
			getConfig: devicePolicy(map[string]string{
				"10": "rp-p-10",
				"20": "rp-p-20",     // statement dropped: orphan
				"30": "rp-p-30-ext", // statement dropped: orphan
				"40": "user-set",    // not synthesized: keep
				"50": "rp-other-50", // another policy's set: keep
				"60": "rp-p2-60",    // policy "p2" shares the prefix "rp-p": keep
			}),
			do:          ok,
			wantSets:    1,
			wantWritten: written,
			wantDeleted: []string{"routing-policy/community-set[name=rp-p-20]", "routing-policy/community-set[name=rp-p-30-ext]"},
		},
		{
			name:      "read fails: returns error, writes nothing",
			getConfig: func(context.Context, ...gnmiext.DataElement) error { return errDevice },
			wantErr:   errDevice,
		},
		{
			name:        "set fails: returns error",
			getConfig:   devicePolicy(map[string]string{"20": "rp-p-20"}),
			do:          func(context.Context, *gnmiext.SetBuilder) error { return errDevice },
			wantErr:     errDevice,
			wantSets:    1,
			wantWritten: written,
			wantDeleted: []string{"routing-policy/community-set[name=rp-p-20]"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &gnmiext.ClientMock{GetConfigFunc: tt.getConfig, DoFunc: tt.do}
			p := &Provider{client: mock}

			err := p.EnsureRoutingPolicy(t.Context(), req)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("EnsureRoutingPolicy() error = %v, want %v", err, tt.wantErr)
			}
			calls := mock.DoCalls()
			if len(calls) != tt.wantSets {
				t.Fatalf("Do called %d times, want %d", len(calls), tt.wantSets)
			}
			if len(calls) == 0 {
				return
			}
			gotWritten, gotDeleted := setOps(calls[0].SetBuilder)
			if !slices.Equal(gotWritten, tt.wantWritten) {
				t.Errorf("written = %v, want %v", gotWritten, tt.wantWritten)
			}
			if !slices.Equal(gotDeleted, tt.wantDeleted) {
				t.Errorf("deleted = %v, want %v", gotDeleted, tt.wantDeleted)
			}
		})
	}
}

func TestEnsureRoutingPolicyUnknownDisposition(t *testing.T) {
	// The mock has no functions set, so any device call panics: an unknown disposition must fail before.
	p := &Provider{client: &gnmiext.ClientMock{}}

	err := p.EnsureRoutingPolicy(t.Context(), &provider.EnsureRoutingPolicyRequest{
		Name: "p",
		Statements: []provider.PolicyStatement{{
			Sequence: 10,
			Actions:  v1alpha1.PolicyActions{RouteDisposition: "Bogus"},
		}},
	})
	if err == nil || !strings.Contains(err.Error(), `unsupported action "Bogus"`) {
		t.Errorf("EnsureRoutingPolicy() error = %v, want unsupported action", err)
	}
}

func TestEnsureRoutingPolicyInvalidSpec(t *testing.T) {
	// The mock has no functions set, so any device call panics: an invalid spec must fail before.
	p := &Provider{client: &gnmiext.ClientMock{}}
	bad := intstr.FromString("70000.1")

	err := p.EnsureRoutingPolicy(t.Context(), &provider.EnsureRoutingPolicyRequest{
		Name: "p",
		Statements: []provider.PolicyStatement{{
			Sequence: 10,
			Actions: v1alpha1.PolicyActions{
				RouteDisposition: v1alpha1.AcceptRoute,
				BgpActions:       &v1alpha1.BgpActions{SetASPath: &v1alpha1.SetASPathAction{ASNumber: &bad}},
			},
		}},
	})
	assertStatusCode(t, err, apistatus.CodeInvalidArgument, "spec.statements[].actions.bgpActions.setASPath.asNumber")
}

func TestDeleteRoutingPolicy(t *testing.T) {
	errDevice := errors.New("device unavailable")
	ok := func(context.Context, ...gnmiext.DataElement) error { return nil }

	tests := []struct {
		name       string
		getConfig  func(context.Context, ...gnmiext.DataElement) error
		del        func(context.Context, ...gnmiext.DataElement) error
		wantErr    error
		wantDelete []string
	}{
		{
			name:      "policy absent: nothing to delete",
			getConfig: func(context.Context, ...gnmiext.DataElement) error { return gnmiext.ErrNil },
		},
		{
			name:       "deletes policy and its synthesized sets",
			getConfig:  devicePolicy(map[string]string{"10": "rp-p-10", "20": "user-set", "30": "rp-p-10"}),
			del:        ok,
			wantDelete: []string{"routing-policy/community-set[name=rp-p-10]", "routing-policy/policy[name=p]"},
		},
		{
			name:      "read fails: returns error",
			getConfig: func(context.Context, ...gnmiext.DataElement) error { return errDevice },
			wantErr:   errDevice,
		},
		{
			name:       "delete fails: returns error",
			getConfig:  devicePolicy(nil),
			del:        func(context.Context, ...gnmiext.DataElement) error { return errDevice },
			wantErr:    errDevice,
			wantDelete: []string{"routing-policy/policy[name=p]"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &gnmiext.ClientMock{GetConfigFunc: tt.getConfig, DeleteFunc: tt.del}
			p := &Provider{client: mock}

			err := p.DeleteRoutingPolicy(t.Context(), &provider.DeleteRoutingPolicyRequest{Name: "p"})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("DeleteRoutingPolicy() error = %v, want %v", err, tt.wantErr)
			}
			var got []string
			for _, c := range mock.DeleteCalls() {
				got = append(got, xpaths(c.DataElements)...)
			}
			if !slices.Equal(got, tt.wantDelete) {
				t.Errorf("Delete = %v, want %v", got, tt.wantDelete)
			}
		})
	}
}

func assertStatusCode(t *testing.T, err error, code apistatus.Code, field string) {
	t.Helper()
	se, ok := apistatus.FromError(err)
	if !ok {
		t.Fatalf("error = %v, want a StatusError with code %s", err, code)
	}
	if se.Code != code {
		t.Errorf("code = %s, want %s", se.Code, code)
	}
	if len(se.FieldViolations) != 1 || se.FieldViolations[0].Field != field {
		t.Errorf("field violations = %+v, want one for %s", se.FieldViolations, field)
	}
}
