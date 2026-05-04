// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package apistatus_test

import (
	"errors"
	"fmt"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/ironcore-dev/network-operator/internal/apistatus"
)

func TestCode(t *testing.T) {
	tests := []struct {
		code      apistatus.Code
		wantStr   string
		wantValid bool
	}{
		{apistatus.Code(0), "Code(0)", false},
		{apistatus.CodeInvalidArgument, "InvalidArgument", true},
		{apistatus.CodeUnsupportedField, "UnsupportedField", true},
		{apistatus.CodeFailedPrecondition, "FailedPrecondition", true},
		{apistatus.Code(99), "Code(99)", false},
	}
	for _, test := range tests {
		if got := test.code.String(); got != test.wantStr {
			t.Errorf("Code(%d).String() = %q, want %q", test.code, got, test.wantStr)
		}
		if got := test.code.Valid(); got != test.wantValid {
			t.Errorf("Code(%d).Valid() = %v, want %v", test.code, got, test.wantValid)
		}
	}
}

func TestStatusError(t *testing.T) {
	tests := []struct {
		name           string
		err            *apistatus.StatusError
		wantCode       apistatus.Code
		wantMessage    string
		wantViolations []apistatus.FieldViolation
	}{
		{
			name:           "InvalidArgument",
			err:            apistatus.NewInvalidArgumentError(apistatus.FieldViolation{Field: "spec.name", Description: "invalid interface format, expected e.g. eth1/1"}),
			wantCode:       apistatus.CodeInvalidArgument,
			wantViolations: []apistatus.FieldViolation{{Field: "spec.name", Description: "invalid interface format, expected e.g. eth1/1"}},
		},
		{
			name:           "UnsupportedField",
			err:            apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{Field: "spec.mtu", Description: "MTU configuration is not supported on this platform"}),
			wantCode:       apistatus.CodeUnsupportedField,
			wantViolations: []apistatus.FieldViolation{{Field: "spec.mtu", Description: "MTU configuration is not supported on this platform"}},
		},
		{
			name:        "FailedPrecondition",
			err:         apistatus.NewFailedPreconditionError("BGP instance must be configured before BGP peers can be realized"),
			wantCode:    apistatus.CodeFailedPrecondition,
			wantMessage: "BGP instance must be configured before BGP peers can be realized",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.err.Code != test.wantCode {
				t.Errorf("Code = %v, want %v", test.err.Code, test.wantCode)
			}
			if test.wantMessage != "" && test.err.Message != test.wantMessage {
				t.Errorf("Message = %q, want %q", test.err.Message, test.wantMessage)
			}
			if len(test.wantViolations) > 0 {
				if len(test.err.FieldViolations) != len(test.wantViolations) || test.err.FieldViolations[0] != test.wantViolations[0] {
					t.Errorf("FieldViolations = %+v, want %+v", test.err.FieldViolations, test.wantViolations)
				}
			}
		})
	}
}

func TestFromError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		ok   bool
	}{
		{"StatusError", apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{Field: "spec.mtu", Description: "x"}), true},
		{"Wrapped", fmt.Errorf("outer: %w", apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{Field: "spec.mtu", Description: "x"})), true},
		{"Plain", errors.New("plain error"), false},
		{"Nil", nil, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, ok := apistatus.FromError(test.err); ok != test.ok { //nolint:errcheck
				t.Errorf("FromError ok = %v, want %v", ok, test.ok)
			}
		})
	}
}

func TestWrapTerminalError(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		wantTerminal bool
	}{
		{"InvalidArgument", apistatus.NewInvalidArgumentError(apistatus.FieldViolation{Field: "spec.mtu", Description: "x"}), true},
		{"UnsupportedField", apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{Field: "spec.type", Description: "x"}), true},
		{"WrappedUnsupportedField", fmt.Errorf("outer: %w", apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{Field: "spec.type", Description: "x"})), true},
		{"FailedPrecondition", apistatus.NewFailedPreconditionError("BGP instance must be configured before BGP peers can be realized"), false},
		{"WrappedFailedPrecondition", fmt.Errorf("outer: %w", apistatus.NewFailedPreconditionError("BGP instance must be configured")), false},
		{"Plain", errors.New("transient"), false},
		{"Nil", nil, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := apistatus.WrapTerminalError(test.err)
			if errors.Is(got, reconcile.TerminalError(nil)) != test.wantTerminal {
				t.Errorf("IsTerminalError = %v, want %v", !test.wantTerminal, test.wantTerminal)
			}
		})
	}
}
