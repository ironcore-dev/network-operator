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
)

// Configurable represents a configuration item with a YANG path.
type Configurable interface {
	// XPath returns the YANG path for this configuration item.
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
	GetState(ctx context.Context, conf ...Configurable) error
	GetConfig(ctx context.Context, conf ...Configurable) error
	Patch(ctx context.Context, conf ...Configurable) error
	Update(ctx context.Context, conf ...Configurable) error
	Delete(ctx context.Context, conf ...Configurable) error
}

// Client is a gNMI client offering convenience methods for device configuration
// using gNMI.
type client struct {
	gnmi         gpb.GNMIClient
	encoding     gpb.Encoding
	capabilities *Capabilities
	logger       logr.Logger
}

var (
	_ Client = &client{}
)

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
	for _, e := range res.SupportedEncodings {
		switch e {
		case gpb.Encoding_JSON, gpb.Encoding_JSON_IETF:
			encoding = e
		default:
			// Ignore unsupported encodings.
		}
	}
	if encoding == -1 {
		return nil, fmt.Errorf("gnmiext: unsupported encoding: %v", res.SupportedEncodings)
	}
	capabilities := &Capabilities{SupportedModels: make([]Model, len(res.GetSupportedModels()))}
	for i, model := range res.GetSupportedModels() {
		capabilities.SupportedModels[i] = Model{
			Name:         model.Name,
			Organization: model.Organization,
			Version:      model.Version,
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

// GetConfig retrieves config and unmarshals it into the provided targets.
// If some of the values for the given xpaths are not defined, [ErrNil] is returned.
func (c *client) GetConfig(ctx context.Context, conf ...Configurable) error {
	return c.get(ctx, gpb.GetRequest_CONFIG, conf...)
}

// GetState retrieves state and unmarshals it into the provided targets.
// If some of the values for the given xpaths are not defined, [ErrNil] is returned.
func (c *client) GetState(ctx context.Context, conf ...Configurable) error {
	return c.get(ctx, gpb.GetRequest_STATE, conf...)
}

// Update replaces the configuration for the given set of items.4c890d
// If the current configuration equals the desired configuration, the operation is skipped.
// For partial updates that merge changes, use [Client.Patch] instead.
func (c *client) Update(ctx context.Context, conf ...Configurable) error {
	return c.set(ctx, false, conf...)
}

// Patch merges the configuration for the given set of items.
// If the current configuration equals the desired configuration, the operation is skipped.
// For full replacement of configuration, use [Client.Update] instead.
func (c *client) Patch(ctx context.Context, conf ...Configurable) error {
	return c.set(ctx, true, conf...)
}

// Delete resets the configuration for the given set of items.
// If an item implements [Defaultable], it's reset to default value.
// Otherwise, the configuration is deleted.
func (c *client) Delete(ctx context.Context, conf ...Configurable) error {
	if len(conf) == 0 {
		return nil
	}
	r := new(gpb.SetRequest)
	for _, cf := range conf {
		path, err := StringToStructuredPath(cf.XPath())
		if err != nil {
			return err
		}
		if d, ok := cf.(Defaultable); ok {
			d.Default()
			b, err := c.Marshal(cf)
			if err != nil {
				return err
			}
			c.logger.V(1).Info("Resetting to default", "path", cf.XPath(), "payload", string(b))
			r.Replace = append(r.Replace, &gpb.Update{
				Path: path,
				Val:  c.Encode(b),
			})
			continue
		}
		c.logger.V(1).Info("Deleting", "path", cf.XPath())
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
func (c *client) get(ctx context.Context, dt gpb.GetRequest_DataType, conf ...Configurable) error {
	if len(conf) == 0 {
		return nil
	}
	r := &gpb.GetRequest{
		Type:     dt,
		Encoding: c.encoding,
	}
	for _, cf := range conf {
		path, err := StringToStructuredPath(cf.XPath())
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
	if len(res.Notification) != len(conf) {
		// This should never happen. If it does, it indicates a bug in the
		// gNMI server.
		return fmt.Errorf("gnmiext: unexpected number of notifications: got %d, want %d", len(res.Notification), len(conf))
	}
	// prevent bounds check in for the range loop below
	// [Bounds Check Elimination]: https://go101.org/optimizations/5-bce.html
	_ = res.Notification[len(conf)-1]
	for i, cf := range conf {
		n := res.Notification[i]
		switch len(n.Update) {
		case 0:
			return ErrNil
		case 1:
			b, err := c.Decode(n.Update[0].Val)
			if err != nil {
				return err
			}
			// Some target devices (e.g., Cisco NX-OS) have an incorrect
			// implementation of the [gNMI spec] and return an empty [gpb.TypedValue]
			// instead of a NotFound status error when the requested path is
			// syntactically correct but does not exist on the device.
			//
			// [gNMI spec]: https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#334-getresponse-behavior-table
			if len(b) == 0 {
				return ErrNil
			}
			if err := c.Unmarshal(b, cf); err != nil {
				return err
			}
		default:
			return fmt.Errorf("gnmiext: unexpected number of updates: %d", len(n.Update))
		}
	}
	return nil
}

// set applies the provided configuration items. If patch is true, a
// partial update is performed by merging the changes into the existing
// configuration. Otherwise, a full replacement is done.
// If the current configuration equals the desired configuration, the operation
// is skipped.
func (c *client) set(ctx context.Context, patch bool, conf ...Configurable) error {
	if len(conf) == 0 {
		return nil
	}
	r := new(gpb.SetRequest)
	for _, cf := range conf {
		path, err := StringToStructuredPath(cf.XPath())
		if err != nil {
			return err
		}
		got := cp.Deep(cf)
		err = c.GetConfig(ctx, got)
		if err != nil && !errors.Is(err, ErrNil) {
			return fmt.Errorf("gnmiext: failed to retrieve current config for %s: %w", cf.XPath(), err)
		}
		// If the current configuration is equal to the desired configuration, skip the update.
		// This avoids unnecessary updates and potential disruptions.
		if err == nil && reflect.DeepEqual(cf, got) {
			c.logger.V(1).Info("Configuration is already up-to-date", "path", cf.XPath())
			continue
		}
		b, err := c.Marshal(cf)
		if err != nil {
			return err
		}
		c.logger.V(1).Info("Updating", "path", cf.XPath(), "payload", string(b), "patch", patch)
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
	if len(r.Update) == 0 && len(r.Replace) == 0 {
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
	switch rv.Kind() {
	case reflect.Pointer:
		zeroUnknownFields(b, rv.Elem())
	case reflect.Struct:
		rt := rv.Type()
		for i := range rt.NumField() {
			if tag, ok := rt.Field(i).Tag.Lookup("json"); ok {
				parts := strings.Split(tag, ",")
				if parts[0] != "" && parts[0] != "-" {
					sf := rv.Field(i)
					raw := gjson.GetBytes(b, parts[0]).Raw
					if raw != "" {
						zeroUnknownFields([]byte(raw), sf)
						continue
					}
					if !sf.IsZero() && sf.CanSet() {
						sf.Set(reflect.Zero(sf.Type()))
					}
				}
			}
		}
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
		v, ok := val.Value.(*gpb.TypedValue_JsonVal)
		if !ok {
			return nil, fmt.Errorf("gnmiext: unexpected value type: expected JsonVal, got %T", val.Value)
		}
		return v.JsonVal, nil
	case gpb.Encoding_JSON_IETF:
		v, ok := val.Value.(*gpb.TypedValue_JsonIetfVal)
		if !ok {
			return nil, fmt.Errorf("gnmiext: unexpected value type: expected JsonIetfVal, got %T", val.Value)
		}
		return v.JsonIetfVal, nil
	default:
		panic("gnmiext: unsupported encoding")
	}
}

// StringToStructuredPath converts a string xpath to a structured path.
//
// It is a wrapper around [ygot.StringToStructuredPath] that additionally supports
// origin prefixes, such as "openconfig-interfaces:interfaces/interface[name=eth1/1]".
func StringToStructuredPath(xpath string) (*gpb.Path, error) {
	var model string
	if idx := strings.Index(xpath, ":"); idx > 0 {
		model = xpath[:idx]
		xpath = xpath[idx+1:]
	}
	path, err := ygot.StringToStructuredPath(xpath)
	if err != nil {
		return nil, fmt.Errorf("gnmiext: failed to convert xpath '%s' to path: %w", xpath, err)
	}
	path.Origin = model
	return path, nil
}
