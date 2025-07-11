// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package gnmiext

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"reflect"
	"slices"
	"strings"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ygot/util"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
)

//go:generate go tool moq -out mock.go . GNMIClient Client
type GNMIClient = gpb.GNMIClient

type Client interface {
	Get(ctx context.Context, xpath string, dest ygot.GoStruct) error
	Set(ctx context.Context, notification *gpb.Notification) error
	Update(ctx context.Context, config DeviceConf) error
	Reset(ctx context.Context, config DeviceConf) error
}

type Update interface {
	isUpdate()
}

func (ReplacingUpdate) isUpdate() {}

// replacing updates enforce the replacement of the entire subtree at the given path, effectively
// deleting all existing nodes and replacing them with the new value.
type ReplacingUpdate struct {
	XPath string
	Value ygot.GoStruct
}

func (EditingUpdate) isUpdate() {}

// editing updates check the current configuration on a target device, to then compute and apply only
// the changes that are required to match the desired/specified configuration.
type EditingUpdate struct {
	XPath string
	// The desired value for the given path.
	Value ygot.GoStruct
	// IgnorePaths is a list of child sub-xpaths of [Value] that should be ignored when computing the diff.
	// These paths should not include the content of [XPath] but are considered rooted in the [Value].
	// This can be used to make a partial update to the configuration.
	IgnorePaths []string
}

// DeviceConf is an interface that must be implemented by all configuration types.
type DeviceConf interface {
	// ToYGOT returns a list that represents the desired configuration updates. `client``
	// is an optional parameter that can be used to retrieve the current configuration
	// from the target device, if needed by the implementation.
	ToYGOT(client Client) ([]Update, error)
	// return a list of updates that reconfigures a device with the default values
	// provided by the yGOT library. `client` is an optional parameter that can be used
	// to retrieve the current configuration from the target device, if the implementation
	// requires keeping some of the current values.
	Reset(client Client) ([]Update, error)
}

var (
	// ErrNil indicates that the value for a path is not defined.
	ErrNil = errors.New("gnmiext: nil")

	// ErrNotFound indicates that the path was not found on the target device.
	ErrNotFound = errors.New("gnmiext: not found")

	// ErrDeviceUnavailable indicates that the target device is not reachable through gNMI.
	ErrDeviceUnavailable = errors.New("gnmiext: device is not reachable")

	// ErrUnsupportedEncoding indicates that the target device does not support JSON encoding.
	ErrUnsupportedEncoding = errors.New("gnmiext: target device does not support JSON encoding")

	// ErrUnsupportedDevice indicates that the target device does not support the required NX-OS version.
	ErrUnsupportedDevice = errors.New("gnmiext: target device does not support the required nxos version")
)

var _ Client = (*client)(nil)

type client struct {
	c      GNMIClient
	dryRun bool

	logger   *slog.Logger
	logLevel slog.Level

	// maximum number of paths that can be updated in a single gNMI request. If the number of
	// paths exceeds this limit, this library will split the changes into multiple requests chunks
	// of this size
	maxPathsPerRequest int
}

type Option func(*client)

// NewClient creates a new instance of a gNMI client. Upon creation the client connects to the device, requests
// its capabilities and checks if the OS version, protocol encoding are supported by the client itself.
// This check can be skipped via the flag withSkipVersionCheck.
//
// The client supports the following options:
//   - WithDryRun: enables dry-run mode which prevents any changes from being applied to the target device.
//   - WithLogger: sets the logger to be used by the client.
//   - WithLogLevel: sets the default log level to be used by the client.
//   - WithoutConfirmedCommits: disables gNMI confirmed commits within each call to Client.Set()
//
// By default:
//   - Supported devices:
//     "Cisco-NX-OS-device" version "2024-03-26"
//   - The maximum number of paths that can be updated in a single gNMI request is 20 (Cisco default).
//     If the number of paths exceeds this limit, the changes are split into multiple chunks.
//   - Confirmed commits are enabled by default, see
//     https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-commit-confirmed.md
//     The rollback timeout is 10 seconds.
//   - Logging level is INFO.
func NewClient(ctx context.Context, c GNMIClient, withSkipVersionCheck bool, opts ...Option) (Client, error) {
	capabilities, err := c.Capabilities(ctx, &gpb.CapabilityRequest{}, grpc.WaitForReady(true))
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Unavailable {
			return nil, fmt.Errorf("%w: %w", ErrDeviceUnavailable, err)
		}

		return nil, err
	}

	if !slices.Contains(capabilities.SupportedEncodings, gpb.Encoding_JSON) {
		return nil, ErrUnsupportedEncoding
	}

	client := &client{
		c:                  c,
		logger:             slog.Default(),
		logLevel:           slog.LevelInfo,
		maxPathsPerRequest: 20,
	}

	for _, opt := range opts {
		opt(client)
	}

	if !withSkipVersionCheck {
		supported := false
		for _, m := range capabilities.SupportedModels {
			if m.Name == "Cisco-NX-OS-device" && m.Version == "2024-03-26" /* v10.4(3) */ {
				supported = true
				break
			}
		}
		if !supported {
			return nil, ErrUnsupportedDevice
		}
	}
	return client, nil
}

// WithDryRun enables dry-run mode which prevents any changes from being applied to the target device.
// Changes will still be logged.
func WithDryRun() Option {
	return func(c *client) {
		c.dryRun = true
	}
}

// WithLogger sets the logger to be used by the client.
func WithLogger(logger *slog.Logger) Option {
	return func(c *client) {
		c.logger = logger
	}
}

// WithLogLevel sets the default log level to be used by the client.
func WithLogLevel(level slog.Level) Option {
	return func(c *client) {
		c.logLevel = level
	}
}

// SetMaxPathsPerRequest sets the maximum number of paths that can be updated in a single gNMI request.
func (c *client) SetMaxPathsPerRequest(numPathsPerRequest int) {
	c.maxPathsPerRequest = numPathsPerRequest
}

func (c *client) log(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
	if c.logger != nil {
		c.logger.LogAttrs(ctx, level, msg, attrs...)
	}
}

// Get retrieves the configuration for the given XPath and unmarshals it into the given GoStruct.
//
// TODO: Retrieve multiple paths in a single request.
func (c *client) Get(ctx context.Context, xpath string, dest ygot.GoStruct) error {
	path, err := ygot.StringToStructuredPath(xpath)
	if err != nil {
		return fmt.Errorf("gnmiext: failed to convert xpath %s to path: %w", xpath, err)
	}

	res, err := c.c.Get(ctx, &gpb.GetRequest{
		Path:     []*gpb.Path{path},
		Type:     gpb.GetRequest_CONFIG,
		Encoding: gpb.Encoding_JSON,
	})
	if err != nil {
		return err
	}

	for _, n := range res.Notification {
		for _, u := range n.Update {
			if proto.Equal(u.Path, path) {
				v, ok := u.Val.Value.(*gpb.TypedValue_JsonVal)
				if !ok {
					return fmt.Errorf("gnmiext: unexpected json value type for xpath %s, got %T", xpath, u.Val.Value)
				}

				if len(v.JsonVal) == 0 {
					return ErrNil
				}

				return nxos.Unmarshal(v.JsonVal, dest)
			}
		}
	}

	return ErrNotFound
}

// Set applies the set of Update and Delete operations included in diff Notification. If the diff is empty or the
// client is in dry-run mode, no changes are applied. This function splits the different operations  into multiple
// chunks of either Update or Delete operations.
//   - if notifications imply more than Client.maxPathsPerRequest paths to be updated, the changes are split into multiple
//     requests chunks of size Client.maxPathsPerRequest.
func (c *client) Set(ctx context.Context, notification *gpb.Notification) error {
	if c.dryRun || (len(notification.Delete) == 0 && len(notification.Update) == 0) {
		return nil
	}

	for d := range slices.Chunk(notification.Delete, c.maxPathsPerRequest) {
		res, err := c.c.Set(ctx, &gpb.SetRequest{Delete: d})
		if err != nil {
			return err
		}
		c.log(ctx, c.logLevel, "Set", slog.Any("res", res))
	}
	for d := range slices.Chunk(notification.Update, c.maxPathsPerRequest) {
		res, err := c.c.Set(ctx, &gpb.SetRequest{Update: d})
		if err != nil {
			return err
		}
		c.log(ctx, c.logLevel, "Set", slog.Any("res", res))
	}
	return nil
}

// Update unifies the process of retrieving the current configuration,
// computing the difference to the desired configuration, and applying the changes.
// It effectively combines the Get, diff, and Set functions into a single operation.
func (c *client) Update(ctx context.Context, config DeviceConf) error {
	conf, err := config.ToYGOT(c)
	if err != nil {
		return fmt.Errorf("gnmiext: failed to convert configuration to YGot structures: %w", err)
	}
	return c.update(ctx, conf)
}

func (c client) update(ctx context.Context, conf []Update) error {
	for _, update := range conf {
		switch u := update.(type) {
		case ReplacingUpdate:
			err := c.applyReplacingUpdate(ctx, &u)
			if err != nil {
				return fmt.Errorf("gnmiext: failed to apply full update for xpath %s: %w", u.XPath, err)
			}
		case EditingUpdate:
			err := c.applyEditingUpdate(ctx, &u)
			if err != nil {
				return fmt.Errorf("gnmiext: failed to apply diff update for xpath %s: %w", u.XPath, err)
			}
		default:
			return fmt.Errorf("gnmiext: unsupported update type '%T'", update)
		}
	}
	return nil
}

// send a gNMI Set request to the target device by sending the entire JSON representation of the ygot object
func (c *client) applyReplacingUpdate(ctx context.Context, update *ReplacingUpdate) error {
	if err := ygot.ValidateGoStruct(update.Value); err != nil {
		return fmt.Errorf("ygot struct validation failed for %s: %w", update.XPath, err)
	}
	jsonVal, err := ygot.EmitJSON(update.Value, &ygot.EmitJSONConfig{
		Format:         ygot.RFC7951,
		SkipValidation: false,
	})
	if err != nil {
		return fmt.Errorf("gnmiext: failed to marshal GoStruct to JSON: %w", err)
	}

	c.log(ctx, slog.LevelDebug, "JSON value", slog.String("json", jsonVal))

	updatePath, err := ygot.StringToStructuredPath(update.XPath)
	if err != nil {
		return fmt.Errorf("gnmiext: failed to convert xpath %s to path: %w", update.XPath, err)
	}

	gUpdate := &gpb.Update{
		Path: updatePath,
		Val: &gpb.TypedValue{
			Value: &gpb.TypedValue_JsonVal{JsonVal: []byte(jsonVal)},
		},
	}

	_, err = c.c.Set(ctx, &gpb.SetRequest{Replace: []*gpb.Update{gUpdate}})
	return err
}

// send a gNMI Get request to the target device, computes the diff between the current and desired configuration,
// and sends a gNMI Set request only with the values that need to change to match the desired configuration.
func (c *client) applyEditingUpdate(ctx context.Context, update *EditingUpdate) error {
	if err := ygot.ValidateGoStruct(update.Value); err != nil {
		return fmt.Errorf("ygot struct validation failed for %s: %w", update.XPath, err)
	}

	got := reflect.New(reflect.TypeOf(update.Value).Elem()).Interface().(ygot.GoStruct)
	err := c.Get(ctx, update.XPath, got)
	if err != nil && !errors.Is(err, ErrNil) {
		return fmt.Errorf("gnmiext: failed to reflect %s onto a ygot.GoStruct: %w", update.XPath, err)
	}

	ignore := make([]*gpb.Path, 0, len(update.IgnorePaths))
	for _, p := range update.IgnorePaths {
		path, err := ygot.StringToStructuredPath(p)
		if err != nil {
			return fmt.Errorf("gnmiext: failed to convert xpath %s to path: %w", p, err)
		}
		ignore = append(ignore, path)
	}

	n, err := diff(update.XPath, got, update.Value, ignore...)
	if err != nil {
		return err
	}
	c.log(ctx, slog.LevelDebug, "Diff", slog.Any("changeset", ygot.FormatDiff(n)))

	err = c.Set(ctx, n)
	if err != nil {
		return err
	}
	return nil
}

// Reset applies the default configuration to the target device by computing the difference
// between the current configuration and the default configuration values set by yGoT.
// It effectively combines the Get, diff, and Set functions into a single operation.
func (c *client) Reset(ctx context.Context, config DeviceConf) error {
	conf, err := config.Reset(c)
	if err != nil {
		return fmt.Errorf("gnmiext: failed to convert configuration to YGot structures: %w", err)
	}
	return c.update(ctx, conf)
}

// isUpdateModifyingParentListKey checks if a gNMI Update attempts to change a value
// that is used to identify an element in a parent list. For example:
//   - "/System/dns-items/prof-items/Prof-list[name=default]/vrf-items/Vrf-list[name=mgmt0]/name"
//
// The function returns true if the update attempts to change a leaf node attribute
// that has the same name as the key used to identify elements in the list it is contained
// (the parent node). Such updates are not allowed by some devices and should be filtered out.
func isUpdateModifyingParentListKey(u *gpb.Update) bool {
	if len(u.Path.Elem) < 2 {
		return false
	}
	last := u.Path.Elem[len(u.Path.Elem)-1]
	prev := u.Path.Elem[len(u.Path.Elem)-2]
	if prev.Key != nil {
		for key := range prev.Key {
			if key == last.Name {
				return true
			}
		}
	}
	return false
}

// diff compares the current configuration with the desired configuration and
// computes the difference as a gNMI notification.
//
// It wraps the existing [ygot.Diff] function and extends it in the following ways:
//   - It prefixes the paths in the diff with the given XPath resulting in absolute paths that can be used in gNMI requests.
//   - It converts the TypedValue to JSON encoding since our target device currently only supports JSON encoding.
//   - It supports the deletion of list elements instead of deleting all their leaf nodes (see https://github.com/openconfig/ygot/issues/576).
func diff(xpath string, got, want ygot.GoStruct, ignore ...*gpb.Path) (*gpb.Notification, error) {
	n, err := ygot.Diff(got, want)
	if err != nil {
		return nil, fmt.Errorf("gnmiext: failed to compute diff: %w", err)
	}

	// Find list elements that are part of the current configuration but not the desired configuration.
	del, err := diffList(got, want)
	if err != nil {
		return nil, err
	}

	// Due to limitations in the [ygot.Diff] function, we need to manually filter out deletions, according to the following rules:
	//   - If a path is present in the update of the reverse diff, it should not be deleted. This allows us to specify a subset of a GoStruct as the desired configuration.
	//   - If a leaf node is part of a list element that is being deleted, it should not be deleted. This allows us to delete entire list elements instead of all their leaf nodes.
	n.Delete = slices.DeleteFunc(n.Delete, func(path *gpb.Path) bool {
		for _, d := range del {
			if util.PathMatchesPathElemPrefix(path, d) {
				return true
			}
		}
		return false
	})
	n.Delete = append(n.Delete, del...)

	// TODO: Use [gpb.SetRequest]'s `Prefix` field to define a common prefix for all paths.
	prefix, err := ygot.StringToStructuredPath(xpath)
	if err != nil {
		return nil, fmt.Errorf("failed to convert xpath %s to path: %w", xpath, err)
	}

	deletes := make([]*gpb.Path, 0, len(n.Delete))
OUTER:
	for i, d := range n.Delete {
		for _, path := range ignore {
			if util.PathMatchesPathElemPrefix(n.Delete[i], path) {
				continue OUTER
			}
		}

		n.Delete[i], err = util.JoinPaths(prefix, d)
		if err != nil {
			return nil, fmt.Errorf("gnmiext: failed to join paths: %w", err)
		}

		deletes = append(deletes, n.Delete[i])
	}
	n.Delete = deletes

	updates := make([]*gpb.Update, 0, len(n.Update))
	for _, u := range n.Update {
		if isUpdateModifyingParentListKey(u) {
			continue
		}

		u.Path, err = util.JoinPaths(prefix, u.Path)
		if err != nil {
			return nil, fmt.Errorf("gnmiext: failed to join paths: %w", err)
		}

		u.Val, err = valueToJSON(u.Val)
		if err != nil {
			return nil, err
		}

		updates = append(updates, u)
	}
	n.Update = updates

	return n, nil
}

// diffList compares the list elements of the current and desired configuration and
// returns a list of paths to be deleted.
//
// The [ygot.Diff] function currently does not support lists, and will instead generate updates
// for the leaf nodes when a list element is removed. Therefore, we need to manually compare the
// list elements and generate the list of paths to be deleted.
//
// See https://github.com/openconfig/ygot/issues/576 for more information.
func diffList(got, want ygot.GoStruct) ([]*gpb.Path, error) {
	orig, err := collect(got)
	if err != nil {
		return nil, err
	}

	mod, err := collect(want)
	if err != nil {
		return nil, err
	}

	paths := make([]*gpb.Path, 0, len(orig))
	for _, elem := range orig {
		if !slices.Contains(mod, elem) {
			paths = append(paths, ygot.MustStringToPath(elem))
		}
	}

	return paths, nil
}

// collect returns a list of contained list element paths for the given GoStruct.
func collect(s ygot.GoStruct) ([]string, error) {
	rv := reflect.ValueOf(s)
	if rv.Kind() != reflect.Ptr {
		return nil, fmt.Errorf("gnmiext: expected pointer to GoStruct, got %T", s)
	}
	for rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return nil, fmt.Errorf("gnmiext: expected GoStruct, got %T", s)
	}
	var paths []string
	for i := range rv.NumField() {
		f := rv.Field(i)
		if !f.IsValid() || f.IsZero() {
			continue
		}

		t := rv.Type().Field(i)
		sp, err := util.SchemaPaths(t)
		if err != nil {
			return nil, fmt.Errorf("gnmiext: failed to get schema paths for struct field %s: %w", t.Name, err)
		}
		if len(sp) != 1 {
			return nil, fmt.Errorf("gnmiext: invalid schema path for struct field %s", t.Name)
		}
		path := strings.Join(sp[0], "/")

		if f.Kind() == reflect.Ptr {
			gs, ok := f.Interface().(ygot.GoStruct)
			if !ok {
				continue
			}

			sub, err := collect(gs)
			if err != nil {
				return nil, err
			}

			for _, sub := range sub {
				paths = append(paths, fmt.Sprintf("%s/%s", path, sub))
			}

			continue
		}

		if f.Kind() != reflect.Map {
			continue
		}

		paths = slices.Grow(paths, f.Len())
		iter := f.MapRange()
		for iter.Next() {
			v := iter.Value()
			pk, err := ygot.PathKeyFromStruct(v)
			if err != nil {
				return nil, fmt.Errorf("gnmiext: failed to get path key: %w", err)
			}

			key := make([]string, 0, len(pk))
			keys := slices.Sorted(maps.Keys(pk))
			for _, k := range keys {
				key = append(key, fmt.Sprintf("%s=%s", k, pk[k]))
			}

			listKey := fmt.Sprintf("%s[%s]", path, strings.Join(key, ","))
			paths = append(paths, listKey)

			if gs, ok := v.Interface().(ygot.GoStruct); ok {
				sub, err := collect(gs)
				if err != nil {
					return nil, err
				}

				for _, sub := range sub {
					paths = append(paths, fmt.Sprintf("%s/%s", listKey, sub))
				}
			}
		}
	}
	return paths, nil
}

// valueToJSON converts a TypedValue to a JSON value. This is required since
// our target device currently only supports JSON encoding.
// TODO: replace with [ygot.EncodeTypedValue]
func valueToJSON(value *gpb.TypedValue) (*gpb.TypedValue, error) {
	var val any
	switch v := value.Value.(type) {
	case *gpb.TypedValue_StringVal:
		val = v.StringVal
	case *gpb.TypedValue_IntVal:
		val = v.IntVal
	case *gpb.TypedValue_UintVal:
		val = v.UintVal
	case *gpb.TypedValue_BoolVal:
		val = v.BoolVal
	case *gpb.TypedValue_BytesVal:
		val = v.BytesVal
	case *gpb.TypedValue_FloatVal:
		val = v.FloatVal //nolint:staticcheck
	case *gpb.TypedValue_DoubleVal:
		val = v.DoubleVal
	case *gpb.TypedValue_JsonVal:
		return value, nil
	case *gpb.TypedValue_DecimalVal, *gpb.TypedValue_LeaflistVal, *gpb.TypedValue_AnyVal, *gpb.TypedValue_JsonIetfVal, *gpb.TypedValue_AsciiVal, *gpb.TypedValue_ProtoBytes:
		return nil, fmt.Errorf("gnmiext: unsupported value type %T", value)
	}

	b, err := json.Marshal(val)
	if err != nil {
		return nil, fmt.Errorf("gnmiext: failed to marshal value: %w", err)
	}

	return &gpb.TypedValue{Value: &gpb.TypedValue_JsonVal{JsonVal: b}}, nil
}
