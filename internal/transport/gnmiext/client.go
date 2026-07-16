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
	"slices"
	"strings"
	"time"

	cp "github.com/felix-kaestner/copy"
	"github.com/go-logr/logr"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
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

// mode represents the type of gNMI Set operation.
type mode uint8

const (
	replace mode = iota + 1 // gNMI replace — used by SetBuilder.Update (full state declaration)
	update                  // gNMI update  — used by SetBuilder.Patch (partial merge)
	del                     // gNMI delete  — used by SetBuilder.Delete
)

// operation pairs a data element with the Set operation to apply.
type operation struct {
	el   DataElement
	mode mode
}

// SetBuilder accumulates gNMI Set operations to be executed as one or
// more batched SetRequest RPCs.
type SetBuilder struct {
	ops   []operation
	limit int
}

// Update queues a full replacement of the given elements (gNMI replace).
func (b *SetBuilder) Update(elements ...DataElement) *SetBuilder {
	for _, el := range elements {
		b.ops = append(b.ops, operation{el: el, mode: replace})
	}
	return b
}

// Patch queues a partial merge of the given elements (gNMI update).
func (b *SetBuilder) Patch(elements ...DataElement) *SetBuilder {
	for _, el := range elements {
		b.ops = append(b.ops, operation{el: el, mode: update})
	}
	return b
}

// Delete queues removal of the given elements (gNMI delete).
func (b *SetBuilder) Delete(elements ...DataElement) *SetBuilder {
	for _, el := range elements {
		b.ops = append(b.ops, operation{el: el, mode: del})
	}
	return b
}

// Limit sets the maximum number of operations per SetRequest RPC.
// If unset or <= 0, all operations are sent in a single request.
func (b *SetBuilder) Limit(n int) *SetBuilder {
	b.limit = n
	return b
}

type Client interface {
	Capabilities() *Capabilities
	GetConfig(context.Context, ...DataElement) error
	GetState(context.Context, ...DataElement) error
	Do(context.Context, *SetBuilder) error
	Patch(context.Context, ...DataElement) error
	Update(context.Context, ...DataElement) error
	Delete(context.Context, ...DataElement) error
}

// Client is a gNMI client offering convenience methods for device configuration
// using gNMI.
type client struct {
	gnmi         gnmipb.GNMIClient
	encoding     gnmipb.Encoding
	capabilities *Capabilities
	logger       logr.Logger
	target       string
}

var _ Client = &client{}

// New creates a new Client by negotiating capabilities with the gNMI server by
// carrying out a Capabilities RPC.
// Returns an error if the device doesn't support JSON encoding.
// By default, the client uses [slog.Default] for logging.
// Use [WithLogger] to provide a custom logger.
func New(ctx context.Context, conn grpc.ClientConnInterface, opts ...Option) (Client, error) {
	gnmi := gnmipb.NewGNMIClient(conn)
	res, err := gnmi.Capabilities(ctx, &gnmipb.CapabilityRequest{})
	if err != nil {
		return nil, fmt.Errorf("gnmiext: failed to retrieve capabilities: %w", err)
	}
	encoding := gnmipb.Encoding(-1)
	for _, e := range res.GetSupportedEncodings() {
		switch e {
		case gnmipb.Encoding_JSON, gnmipb.Encoding_JSON_IETF:
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
	c := &client{gnmi, encoding, capabilities, logger, ""}
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

// WithTarget sets the device target label used in metrics.
func WithTarget(target string) Option {
	return func(c *client) {
		c.target = target
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
func (c *client) GetConfig(ctx context.Context, elements ...DataElement) error {
	return c.get(ctx, gnmipb.GetRequest_CONFIG, elements...)
}

// GetState retrieves state and unmarshals it into the provided targets.
// If some of the values for the given xpaths are not defined, [ErrNil] is returned.
func (c *client) GetState(ctx context.Context, elements ...DataElement) error {
	return c.get(ctx, gnmipb.GetRequest_STATE, elements...)
}

// Update replaces the configuration for the given set of items.
// If the current configuration equals the desired configuration, the operation is skipped.
// For partial updates that merge changes, use [Client.Patch] instead.
func (c *client) Update(ctx context.Context, elements ...DataElement) error {
	r := new(gnmipb.SetRequest)
	if err := c.set(ctx, r, false, elements...); err != nil {
		return err
	}
	return c.do(ctx, r)
}

// Patch merges the configuration for the given set of items.
// If the current configuration equals the desired configuration, the operation is skipped.
// For full replacement of configuration, use [Client.Update] instead.
func (c *client) Patch(ctx context.Context, elements ...DataElement) error {
	r := new(gnmipb.SetRequest)
	if err := c.set(ctx, r, true, elements...); err != nil {
		return err
	}
	return c.do(ctx, r)
}

// Delete resets the configuration for the given set of items.
// If an item implements [Defaultable], it's reset to default value.
// Otherwise, the configuration is deleted.
func (c *client) Delete(ctx context.Context, elements ...DataElement) error {
	r := new(gnmipb.SetRequest)
	if err := c.delete(ctx, r, elements...); err != nil {
		return err
	}
	return c.do(ctx, r)
}

// Do executes the accumulated operations in the SetBuilder as one or
// more gNMI Set RPCs, respecting the configured limit.
func (c *client) Do(ctx context.Context, b *SetBuilder) error {
	chunks := [][]operation{b.ops}
	if b.limit > 0 {
		chunks = slices.Collect(slices.Chunk(b.ops, b.limit))
	}
	for _, chunk := range chunks {
		r := new(gnmipb.SetRequest)
		for _, op := range chunk {
			switch op.mode {
			case replace:
				if err := c.set(ctx, r, false, op.el); err != nil {
					return err
				}
			case update:
				if err := c.set(ctx, r, true, op.el); err != nil {
					return err
				}
			case del:
				if err := c.delete(ctx, r, op.el); err != nil {
					return err
				}
			}
		}
		if err := c.do(ctx, r); err != nil {
			return err
		}
	}
	return nil
}

// get retrieves data of the specified type (CONFIG or STATE) and unmarshals it
// into the provided targets. If some of the values for the given xpaths are not
// defined, [ErrNil] is returned.
func (c *client) get(ctx context.Context, dt gnmipb.GetRequest_DataType, elements ...DataElement) error {
	if len(elements) == 0 {
		return nil
	}
	r := &gnmipb.GetRequest{
		Type:     dt,
		Encoding: c.encoding,
	}
	for _, el := range elements {
		path, err := StringToStructuredPath(el.XPath())
		if err != nil {
			return err
		}
		r.Path = append(r.Path, path)
	}
	op := "get_config"
	if dt == gnmipb.GetRequest_STATE {
		op = "get_state"
	}
	start := time.Now()
	res, err := c.gnmi.Get(ctx, r)
	if err != nil {
		rpcDurationSeconds.WithLabelValues(c.target, "Get", "error").Observe(time.Since(start).Seconds())
		for _, p := range r.GetPath() {
			operationsTotal.WithLabelValues(c.target, "Get", op, metricPath(p), "error").Inc()
		}
		return fmt.Errorf("gnmiext: failed to perform get rpc: %w", err)
	}
	rpcDurationSeconds.WithLabelValues(c.target, "Get", "success").Observe(time.Since(start).Seconds())
	for _, p := range r.GetPath() {
		operationsTotal.WithLabelValues(c.target, "Get", op, metricPath(p), "success").Inc()
	}
	// As per [gNMI spec] the response MUST contain one notification
	// for each path in the request.
	//
	// [gNMI spec]: https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#332-the-getresponse-message
	notifications := res.GetNotification()
	if len(notifications) != len(elements) {
		// This should never happen. If it does, it indicates a bug in the
		// gNMI server.
		return fmt.Errorf("gnmiext: unexpected number of notifications: got %d, want %d", len(notifications), len(elements))
	}
	// prevent bounds check in for the range loop below
	// [Bounds Check Elimination]: https://go101.org/optimizations/5-bce.html
	_ = notifications[len(elements)-1]
	for i, el := range elements {
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
			// implementation of the [gNMI spec] and return an empty [gnmipb.TypedValue]
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
			if err := c.Unmarshal(b, el); err != nil {
				return err
			}
		default:
			return fmt.Errorf("gnmiext: unexpected number of updates: %d", len(n.GetUpdate()))
		}
	}
	return nil
}

// set appends update or replace operations to the given SetRequest.
// If patch is true, elements are appended as gNMI update (merge);
// otherwise as gNMI replace (full replacement).
// Elements whose current configuration already matches are skipped.
func (c *client) set(ctx context.Context, r *gnmipb.SetRequest, patch bool, elements ...DataElement) error {
	op := "replace"
	if patch {
		op = "update"
	}
	for _, el := range elements {
		path, err := StringToStructuredPath(el.XPath())
		if err != nil {
			return err
		}
		got := cp.Deep(el)
		err = c.GetConfig(ctx, got)
		if err != nil && !errors.Is(err, ErrNil) && status.Code(err) != codes.NotFound {
			return fmt.Errorf("gnmiext: failed to retrieve current config for %s: %w", el.XPath(), err)
		}
		// If the current configuration is equal to the desired configuration, skip the update.
		// This avoids unnecessary updates and potential disruptions.
		if err == nil && reflect.DeepEqual(el, got) {
			c.logger.V(2).Info("Configuration is already up-to-date", "path", el.XPath())
			operationsSkippedTotal.WithLabelValues(c.target, op, metricPath(path)).Inc()
			continue
		}
		b, err := c.Marshal(el)
		if err != nil {
			return err
		}
		c.logger.V(1).Info("Updating", "path", el.XPath(), "payload", string(b), "operation", op)
		u := &gnmipb.Update{
			Path: path,
			Val:  c.Encode(b),
		}
		if patch {
			r.Update = append(r.Update, u)
			continue
		}
		r.Replace = append(r.Replace, u)
	}
	return nil
}

// delete appends delete operations to the given SetRequest.
// If an element implements [Defaultable], a replace-with-default is
// appended instead of a path deletion.
func (c *client) delete(_ context.Context, r *gnmipb.SetRequest, elements ...DataElement) error {
	for _, el := range elements {
		path, err := StringToStructuredPath(el.XPath())
		if err != nil {
			return err
		}
		if d, ok := el.(Defaultable); ok {
			d.Default()
			b, err := c.Marshal(el)
			if err != nil {
				return err
			}
			c.logger.V(1).Info("Resetting to default", "path", el.XPath(), "payload", string(b))
			r.Replace = append(r.Replace, &gnmipb.Update{
				Path: path,
				Val:  c.Encode(b),
			})
			continue
		}
		c.logger.V(1).Info("Deleting", "path", el.XPath())
		r.Delete = append(r.Delete, path)
	}
	return nil
}

// do fires the given SetRequest and records operation metrics. Returns nil if the request is empty.
func (c *client) do(ctx context.Context, r *gnmipb.SetRequest) error {
	if len(r.GetUpdate()) == 0 && len(r.GetReplace()) == 0 && len(r.GetDelete()) == 0 {
		return nil
	}
	start := time.Now()
	_, err := c.gnmi.Set(ctx, r)
	st := "success"
	if err != nil {
		st = "error"
	}
	rpcDurationSeconds.WithLabelValues(c.target, "Set", st).Observe(time.Since(start).Seconds())
	for _, u := range r.GetReplace() {
		operationsTotal.WithLabelValues(c.target, "Set", "replace", metricPath(u.GetPath()), st).Inc()
	}
	for _, u := range r.GetUpdate() {
		operationsTotal.WithLabelValues(c.target, "Set", "update", metricPath(u.GetPath()), st).Inc()
	}
	for _, d := range r.GetDelete() {
		operationsTotal.WithLabelValues(c.target, "Set", "delete", metricPath(d), st).Inc()
	}
	if err != nil {
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

// Encode encodes the provided byte slice into a [gnmipb.TypedValue] using the client's encoding.
func (c *client) Encode(b []byte) *gnmipb.TypedValue {
	switch c.encoding {
	case gnmipb.Encoding_JSON:
		return &gnmipb.TypedValue{
			Value: &gnmipb.TypedValue_JsonVal{
				JsonVal: b,
			},
		}
	case gnmipb.Encoding_JSON_IETF:
		return &gnmipb.TypedValue{
			Value: &gnmipb.TypedValue_JsonIetfVal{
				JsonIetfVal: b,
			},
		}
	default:
		panic("gnmiext: unsupported encoding")
	}
}

// Decode decodes the provided [gnmipb.TypedValue] into the provided destination using the client's encoding.
func (c *client) Decode(val *gnmipb.TypedValue) ([]byte, error) {
	switch c.encoding {
	case gnmipb.Encoding_JSON:
		v, ok := val.GetValue().(*gnmipb.TypedValue_JsonVal)
		if !ok {
			return nil, fmt.Errorf("gnmiext: unexpected value type: expected JsonVal, got %T", val.GetValue())
		}
		return v.JsonVal, nil
	case gnmipb.Encoding_JSON_IETF:
		v, ok := val.GetValue().(*gnmipb.TypedValue_JsonIetfVal)
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
func StringToStructuredPath(xpath string) (*gnmipb.Path, error) {
	path, err := ygot.StringToStructuredPath(xpath)
	if err != nil {
		return nil, fmt.Errorf("gnmiext: failed to convert xpath '%s' to path: %w", xpath, err)
	}
	return path, nil
}
