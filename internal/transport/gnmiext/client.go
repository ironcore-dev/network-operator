// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package gnmiext

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"

	cp "github.com/felix-kaestner/copy"
	"github.com/go-logr/logr"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ygot/ygot"
	"github.com/tidwall/gjson"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// DataElement represents a data element addressable by a YANG path.
// A data element can refer to any level of the data tree — a single leaf,
// a container, or an entire subtree — and may carry either configuration
// or state data.
type DataElement interface {
	// XPath returns the YANG path for this data element.
	// It may include an origin prefix (e.g., "openconfig:system/config/hostname").
	XPath() string
}

// Defaultable represents a configuration item that resets to a default
// value instead of being deleted.
type Defaultable interface {
	// Default sets the receiver to its default value in-place.
	Default()
}

// Marshaler provides device-specific marshaling based on capabilities.
type Marshaler interface {
	// MarshalYANG serializes the receiver using device capabilities.
	MarshalYANG(*Capabilities) ([]byte, error)

	// UnmarshalYANG deserializes data into the receiver using device
	// capabilities.
	UnmarshalYANG(*Capabilities, []byte) error
}

// Model represents a YANG data model supported by a device.
type Model struct {
	Name         string
	Organization string
	Version      string
}

// Capabilities represents device capabilities including supported YANG models.
type Capabilities struct {
	SupportedModels []Model
}

type Client interface {
	Capabilities() *Capabilities
	GetConfig(context.Context, ...DataElement) error
	GetState(context.Context, ...DataElement) error
	Patch(context.Context, ...DataElement) error
	Update(context.Context, ...DataElement) error
	Delete(context.Context, ...DataElement) error
}

// Client is a gNMI client offering convenience methods for device configuration
// using gNMI.
type client struct {
	gnmi         gpb.GNMIClient
	encoding     gpb.Encoding
	capabilities *Capabilities
	logger       logr.Logger
}

var _ Client = &client{}

// New creates a new Client by negotiating capabilities with the gNMI server by
// carrying out a Capabilities RPC.
// Returns an error if the device doesn't support JSON encoding.
// By default, the client uses [slog.Default] for logging.
// Use [WithLogger] to provide a custom logger.
func New(ctx context.Context, conn grpc.ClientConnInterface, opts ...Option) (Client, error) {
	gnmi := gpb.NewGNMIClient(conn)
	res, err := gnmi.Capabilities(ctx, &gpb.CapabilityRequest{})
	if err != nil {
		return nil, fmt.Errorf("gnmiext: failed to retrieve capabilities: %w", err)
	}
	encoding := gpb.Encoding(-1)
	for _, e := range res.GetSupportedEncodings() {
		switch e {
		case gpb.Encoding_JSON, gpb.Encoding_JSON_IETF:
			encoding = e
		default:
			// Ignore unsupported encodings.
		}
	}
	if encoding == -1 {
		return nil, fmt.Errorf("gnmiext: unsupported encoding: %v", res.GetSupportedEncodings())
	}
	capabilities := &Capabilities{SupportedModels: make([]Model, len(res.GetSupportedModels()))}
	for i, model := range res.GetSupportedModels() {
		capabilities.SupportedModels[i] = Model{
			Name:         model.GetName(),
			Organization: model.GetOrganization(),
			Version:      model.GetVersion(),
		}
	}
	logger := logr.FromSlogHandler(slog.Default().Handler())
	c := &client{gnmi, encoding, capabilities, logger}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

type Option func(*client)

// WithLogger sets a custom logger for the client.
func WithLogger(logger logr.Logger) Option {
	return func(c *client) {
		c.logger = logger
	}
}

// ErrNil indicates that the value for a xpath is not defined.
var ErrNil = errors.New("gnmiext: nil")

// Capabilities returns the capabilities supported by the gNMI server.
func (c *client) Capabilities() *Capabilities {
	return c.capabilities
}

// GetConfig retrieves config and unmarshals it into the provided targets.
// If some of the values for the given xpaths are not defined, [ErrNil] is returned.
func (c *client) GetConfig(ctx context.Context, el ...DataElement) error {
	return c.get(ctx, gpb.GetRequest_CONFIG, el...)
}

// GetState retrieves state and unmarshals it into the provided targets.
// If some of the values for the given xpaths are not defined, [ErrNil] is returned.
func (c *client) GetState(ctx context.Context, el ...DataElement) error {
	return c.get(ctx, gpb.GetRequest_STATE, el...)
}

// Update replaces the configuration for the given set of items.4c890d
// If the current configuration equals the desired configuration, the operation is skipped.
// For partial updates that merge changes, use [Client.Patch] instead.
func (c *client) Update(ctx context.Context, el ...DataElement) error {
	return c.set(ctx, false, el...)
}

// Patch merges the configuration for the given set of items.
// If the current configuration equals the desired configuration, the operation is skipped.
// For full replacement of configuration, use [Client.Update] instead.
func (c *client) Patch(ctx context.Context, el ...DataElement) error {
	return c.set(ctx, true, el...)
}

// Delete resets the configuration for the given set of items.
// If an item implements [Defaultable], it's reset to default value.
// Otherwise, the configuration is deleted.
func (c *client) Delete(ctx context.Context, el ...DataElement) error {
	if len(el) == 0 {
		return nil
	}
	r := new(gpb.SetRequest)
	for _, e := range el {
		path, err := StringToStructuredPath(e.XPath())
		if err != nil {
			return err
		}
		if d, ok := e.(Defaultable); ok {
			d.Default()
			b, err := c.Marshal(e)
			if err != nil {
				return err
			}
			c.logger.V(1).Info("Resetting to default", "path", e.XPath(), "payload", string(b))
			r.Replace = append(r.Replace, &gpb.Update{
				Path: path,
				Val:  c.Encode(b),
			})
			continue
		}
		c.logger.V(1).Info("Deleting", "path", e.XPath())
		r.Delete = append(r.Delete, path)
	}
	if _, err := c.gnmi.Set(ctx, r); err != nil {
		return fmt.Errorf("gnmiext: failed to perform set rpc: %w", err)
	}
	return nil
}

// get retrieves data of the specified type (CONFIG or STATE) and unmarshals it
// into the provided targets. If some of the values for the given xpaths are not
// defined, [ErrNil] is returned.
func (c *client) get(ctx context.Context, dt gpb.GetRequest_DataType, el ...DataElement) error {
	if len(el) == 0 {
		return nil
	}
	r := &gpb.GetRequest{
		Type:     dt,
		Encoding: c.encoding,
	}
	for _, e := range el {
		path, err := StringToStructuredPath(e.XPath())
		if err != nil {
			return err
		}
		r.Path = append(r.Path, path)
	}
	res, err := c.gnmi.Get(ctx, r)
	if err != nil {
		return fmt.Errorf("gnmiext: failed to perform get rpc: %w", err)
	}
	// As per [gNMI spec] the response MUST contain one notification
	// for each path in the request.
	//
	// [gNMI spec]: https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#332-the-getresponse-message
	notifications := res.GetNotification()
	if len(notifications) != len(el) {
		// This should never happen. If it does, it indicates a bug in the
		// gNMI server.
		return fmt.Errorf("gnmiext: unexpected number of notifications: got %d, want %d", len(notifications), len(el))
	}
	// prevent bounds check in for the range loop below
	// [Bounds Check Elimination]: https://go101.org/optimizations/5-bce.html
	_ = notifications[len(el)-1]
	for i, e := range el {
		n := notifications[i]
		switch len(n.GetUpdate()) {
		case 0:
			return ErrNil
		case 1:
			b, err := c.Decode(n.GetUpdate()[0].GetVal())
			if err != nil {
				return err
			}
			// Some target devices (e.g., Cisco NX-OS) have an incorrect
			// implementation of the [gNMI spec] and return an empty [gpb.TypedValue]
			// instead of a NotFound status error when the requested path is
			// syntactically correct but does not exist on the device.
			//
			// Similarly, some devices (e.g., Cisco NX-OS) return a JSON null
			// value rather than omitting the update entirely (as Nokia SR Linux
			// does). Treat null the same as empty to avoid leaving the target
			// struct unchanged during unmarshal.
			//
			// [gNMI spec]: https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#334-getresponse-behavior-table
			if len(b) == 0 || string(b) == "null" {
				return ErrNil
			}
			if err := c.Unmarshal(b, e); err != nil {
				return err
			}
		default:
			return fmt.Errorf("gnmiext: unexpected number of updates: %d", len(n.GetUpdate()))
		}
	}
	return nil
}

// set applies the provided configuration items. If patch is true, a
// partial update is performed by merging the changes into the existing
// configuration. Otherwise, a full replacement is done.
// If the current configuration equals the desired configuration, the operation
// is skipped.
func (c *client) set(ctx context.Context, patch bool, el ...DataElement) error {
	if len(el) == 0 {
		return nil
	}
	r := new(gpb.SetRequest)
	for _, e := range el {
		path, err := StringToStructuredPath(e.XPath())
		if err != nil {
			return err
		}
		got := cp.Deep(e)
		err = c.GetConfig(ctx, got)
		if err != nil && !errors.Is(err, ErrNil) && status.Code(err) != codes.NotFound {
			return fmt.Errorf("gnmiext: failed to retrieve current config for %s: %w", e.XPath(), err)
		}
		// If the current configuration is equal to the desired configuration, skip the update.
		// This avoids unnecessary updates and potential disruptions.
		if err == nil && reflect.DeepEqual(e, got) {
			c.logger.V(2).Info("Configuration is already up-to-date", "path", e.XPath())
			continue
		}
		b, err := c.Marshal(e)
		if err != nil {
			return err
		}
		c.logger.V(1).Info("Updating", "path", e.XPath(), "payload", string(b), "patch", patch)
		u := &gpb.Update{
			Path: path,
			Val:  c.Encode(b),
		}
		if patch {
			r.Update = append(r.Update, u)
			continue
		}
		r.Replace = append(r.Replace, u)
	}
	if len(r.GetUpdate()) == 0 && len(r.GetReplace()) == 0 {
		// All configurations are already up-to-date.
		return nil
	}
	if _, err := c.gnmi.Set(ctx, r); err != nil {
		return fmt.Errorf("gnmiext: failed to perform set rpc: %w", err)
	}
	return nil
}

// Marshal marshals the provided value into a byte slice using the client's encoding.
// If the value implements the [Marshaler] interface, it will be marshaled using that.
// Otherwise, [json.Marshal] is used.
func (c *client) Marshal(v any) (b []byte, err error) {
	if m, ok := v.(Marshaler); ok {
		b, err = m.MarshalYANG(c.capabilities)
		if err != nil {
			return nil, fmt.Errorf("gnmiext: failed to marshal value: %w", err)
		}
		return b, nil
	}
	b, err = json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("gnmiext: failed to marshal value: %w", err)
	}
	return b, nil
}

// zeroUnknownFields sets struct fields to their zero value
// if they are not present in the provided JSON byte slice.
func zeroUnknownFields(b []byte, rv reflect.Value) {
	// IsNil is only valid for pointer, map, slice, chan, func, and interface kinds.
	switch rv.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func, reflect.Interface:
		if rv.IsNil() {
			return
		}
	default:
	}
	if len(b) == 0 && !rv.IsZero() && rv.CanSet() {
		rv.Set(reflect.Zero(rv.Type()))
		return
	}
	switch rv.Kind() {
	case reflect.Pointer:
		zeroUnknownFields(b, rv.Elem())
	case reflect.Struct:
		rt := rv.Type()
		for i := range rt.NumField() {
			if tag, ok := rt.Field(i).Tag.Lookup("json"); ok {
				parts := strings.Split(tag, ",")
				if parts[0] != "" && parts[0] != "-" {
					raw := gjson.GetBytes(b, parts[0]).Raw
					zeroUnknownFields([]byte(raw), rv.Field(i))
				}
			}
		}
	default:
		return
	}
}

// Unmarshal unmarshals the provided byte slice into the provided destination.
// If the destination implements the [Marshaler] interface, it will be unmarshaled using that.
// Otherwise, [json.Unmarshal] is used.
func (c *client) Unmarshal(b []byte, dst any) (err error) {
	// NOTE: If you query for list elements on Cisco NX-OS, the encoded payload
	// will be the wrapped in an array (even if only one element is requested), i.e.
	//
	// [
	// 	{
	// 		...
	// 	}
	// ]
	_, ok := dst.(interface {
		IsListItem()
	})
	if ok && b[0] == '[' && b[len(b)-1] == ']' {
		b = b[1 : len(b)-1]
	}
	zeroUnknownFields(b, reflect.ValueOf(dst))
	if um, ok := dst.(Marshaler); ok {
		if err := um.UnmarshalYANG(c.capabilities, b); err != nil {
			return fmt.Errorf("gnmiext: failed to unmarshal value: %w", err)
		}
		return nil
	}
	if err := json.Unmarshal(b, dst); err != nil {
		return fmt.Errorf("gnmiext: failed to unmarshal value: %w", err)
	}
	return nil
}

// Encode encodes the provided byte slice into a [gpb.TypedValue] using the client's encoding.
func (c *client) Encode(b []byte) *gpb.TypedValue {
	switch c.encoding {
	case gpb.Encoding_JSON:
		return &gpb.TypedValue{
			Value: &gpb.TypedValue_JsonVal{
				JsonVal: b,
			},
		}
	case gpb.Encoding_JSON_IETF:
		return &gpb.TypedValue{
			Value: &gpb.TypedValue_JsonIetfVal{
				JsonIetfVal: b,
			},
		}
	default:
		panic("gnmiext: unsupported encoding")
	}
}

// Decode decodes the provided [gpb.TypedValue] into the provided destination using the client's encoding.
func (c *client) Decode(val *gpb.TypedValue) ([]byte, error) {
	switch c.encoding {
	case gpb.Encoding_JSON:
		v, ok := val.GetValue().(*gpb.TypedValue_JsonVal)
		if !ok {
			return nil, fmt.Errorf("gnmiext: unexpected value type: expected JsonVal, got %T", val.GetValue())
		}
		return v.JsonVal, nil
	case gpb.Encoding_JSON_IETF:
		v, ok := val.GetValue().(*gpb.TypedValue_JsonIetfVal)
		if !ok {
			return nil, fmt.Errorf("gnmiext: unexpected value type: expected JsonIetfVal, got %T", val.GetValue())
		}
		return v.JsonIetfVal, nil
	default:
		panic("gnmiext: unsupported encoding")
	}
}

// StringToStructuredPath converts a string xpath to a structured path.
//
// Module prefixes (e.g. "openconfig-system:system/state") are kept in the
// first path element name per RFC 7951 Section 4 [1], which defines that
// the module name qualifies the first identifier in a JSON-encoded YANG path.
//
// [1]: https://datatracker.ietf.org/doc/html/rfc7951#section-4
func StringToStructuredPath(xpath string) (*gpb.Path, error) {
	path, err := ygot.StringToStructuredPath(xpath)
	if err != nil {
		return nil, fmt.Errorf("gnmiext: failed to convert xpath '%s' to path: %w", xpath, err)
	}
	return path, nil
}
