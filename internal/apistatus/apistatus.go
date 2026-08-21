// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

// Package apistatus provides a typed structured error for provider implementations.
package apistatus

import (
	"errors"
	"fmt"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Code identifies the category of a [StatusError].
// The zero value is not a valid code.
type Code uint8

const (
	// CodeInvalidArgument signals that one or more spec field values have an
	// incorrect format or structure. The field is supported, but the value
	// does not meet the provider's requirements. The resource cannot be
	// realized until the spec is corrected.
	CodeInvalidArgument Code = iota + 1

	// CodeUnsupportedField signals that one or more spec fields are not
	// supported by the provider. The resource cannot be realized until
	// the spec is changed.
	CodeUnsupportedField

	// CodeFailedPrecondition signals that a precondition on the device or
	// environment is not met. Unlike other codes, this is retryable — the
	// precondition may become true on a future attempt (e.g. a BGP process
	// configured out-of-band). [WrapTerminalError] does not promote these
	// errors to terminal.
	CodeFailedPrecondition
)

// Valid reports whether c is a known, non-zero Code.
func (c Code) Valid() bool {
	switch c {
	case CodeInvalidArgument, CodeUnsupportedField, CodeFailedPrecondition:
		return true
	default:
		return false
	}
}

// String returns the string representation of c.
func (c Code) String() string {
	switch c {
	case CodeInvalidArgument:
		return "InvalidArgument"
	case CodeUnsupportedField:
		return "UnsupportedField"
	case CodeFailedPrecondition:
		return "FailedPrecondition"
	default:
		return fmt.Sprintf("Code(%d)", c)
	}
}

// FieldViolation describes a problem with a specific spec field.
type FieldViolation struct {
	// Field is the dot-separated path to the field, e.g. "spec.mtu".
	Field string
	// Description explains why the field is invalid, e.g. "not supported on this platform".
	Description string
}

// StatusError is a structured error returned by provider implementations.
// It carries a Code, an optional top-level Message, and an optional list of
// FieldViolations. Whether the error is terminal depends on the Code — see
// [WrapTerminalError].
type StatusError struct {
	Code            Code
	Message         string
	FieldViolations []FieldViolation
}

// Is implements the errors.Is interface, reporting whether target is a [*StatusError].
// This allows [errors.Is] to be used for type detection in error chains.
func (e *StatusError) Is(target error) bool {
	_, ok := FromError(target) //nolint:errcheck
	return ok
}

// Error implements the error interface.
func (e *StatusError) Error() string {
	parts := make([]string, 0, 1+len(e.FieldViolations))
	if e.Message != "" {
		parts = append(parts, e.Message)
	}
	for _, v := range e.FieldViolations {
		parts = append(parts, fmt.Sprintf("field %s: %s", v.Field, v.Description))
	}
	if len(parts) == 0 {
		return e.Code.String()
	}
	return e.Code.String() + ": " + strings.Join(parts, "; ")
}

// NewInvalidArgumentError returns a [StatusError] with [CodeInvalidArgument] for
// one or more spec field values that have an incorrect format or structure.
func NewInvalidArgumentError(violations ...FieldViolation) *StatusError {
	return &StatusError{
		Code:            CodeInvalidArgument,
		FieldViolations: violations,
	}
}

// NewUnsupportedFieldError returns a [StatusError] with [CodeUnsupportedField] for
// one or more spec fields that are not supported by the provider.
func NewUnsupportedFieldError(violations ...FieldViolation) *StatusError {
	return &StatusError{
		Code:            CodeUnsupportedField,
		FieldViolations: violations,
	}
}

// NewFailedPreconditionError returns a [StatusError] with [CodeFailedPrecondition]
// for a precondition that is not yet met. These errors are retryable —
// controller-runtime exponential backoff will requeue the request so the
// precondition can be rechecked on a future attempt.
func NewFailedPreconditionError(message string) *StatusError {
	return &StatusError{
		Code:    CodeFailedPrecondition,
		Message: message,
	}
}

// FromError extracts a [*StatusError] from err.
// The boolean reports whether the extraction succeeded.
func FromError(err error) (*StatusError, bool) {
	se, ok := errors.AsType[*StatusError](err)
	return se, ok
}

// WrapTerminalError wraps err as a [reconcile.TerminalError] if it contains a
// [*StatusError] with a non-retryable code ([CodeInvalidArgument] or
// [CodeUnsupportedField]). [CodeFailedPrecondition] errors are returned
// unchanged so the controller-runtime exponential backoff can requeue them —
// the precondition may become true on a future attempt.
// All other errors are also returned unchanged.
//
// Example usage in a controller:
//
//	err := s.Provider.EnsureBGPPeer(ctx, &provider.BGPPeerRequest{...})
//	cond := conditions.FromError(err)
//	conditions.Set(s.BGPPeer, cond)
//	if err != nil {
//	    return apistatus.WrapTerminalError(err)
//	}
func WrapTerminalError(err error) error {
	if se, ok := FromError(err); ok {
		switch se.Code {
		case CodeInvalidArgument, CodeUnsupportedField:
			return reconcile.TerminalError(err)
		default:
			// Other codes are not considered terminal — return the original error
		}
	}
	return err
}
