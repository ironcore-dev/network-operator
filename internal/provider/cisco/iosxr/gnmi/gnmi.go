package gnmi

import (
	"context"
	"fmt"
	"log/slog"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GNMIClient = gpb.GNMIClient

type Client interface {
	//Exists(ctx context.Context, xpath string) (bool, error)
	Get(ctx context.Context, xpath string) error
	//Set(ctx context.Context, notification *gpb.Notification) error
	//Update(ctx context.Context, config DeviceConf) error
	//Reset(ctx context.Context, config DeviceConf) error
}

var _ Client = (*client)(nil)

type client struct {
	c GNMIClient

	logger   *slog.Logger
	logLevel slog.Level

	// maximum number of paths that can be updated in a single gNMI request. If the number of
	// paths exceeds this limit, this library will split the changes into multiple requests chunks
	// of this size
	maxPathsPerRequest int
}

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
func NewClient(ctx context.Context, c GNMIClient, withSkipVersionCheck bool) (Client, error) {

	_, err := c.Capabilities(ctx, &gpb.CapabilityRequest{}, grpc.WaitForReady(true))
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Unavailable {
			return nil, fmt.Errorf("%w: %w", nxos.ErrDeviceUnavailable, err)
		}

		return nil, err
	}

	client := &client{
		c:                  c,
		logger:             slog.Default(),
		logLevel:           slog.LevelInfo,
		maxPathsPerRequest: 20,
	}

	//for _, opt := range opts {
	//	opt(client)
	//}

	return client, nil
}

func (c *client) Get(ctx context.Context, xpath string) error {
	c.logger.Info("GNMI Get Called", "path", xpath)

	path, err := ygot.StringToStructuredPath(xpath)
	if err != nil {
		return fmt.Errorf("gnmiext: failed to convert xpath %s to path: %w", xpath, err)
	}
	c.logger.Info("xpath: ", "path", xpath)

	res, err := c.c.Get(ctx, &gpb.GetRequest{
		Path:     []*gpb.Path{path},
		Type:     gpb.GetRequest_CONFIG,
		Encoding: gpb.Encoding_JSON_IETF,
	})
	if err != nil {
		c.logger.Error("GNMI Get Response Error", "err", err)
		return err
	}
	c.logger.Info("GNMI Get Response", "response", res)
	return nil
}
