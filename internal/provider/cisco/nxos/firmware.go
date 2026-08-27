// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ironcore-dev/network-operator/internal/deviceutil"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/transport/nxapi"
)

// upgradeTimeout is the NX-API client timeout for long-running firmware
// commands (copy and install), which block synchronously for minutes.
const upgradeTimeout = 20 * time.Minute

const (
	// nxosDefaultSessionTimeout is the default NX-API session timeout in seconds.
	nxosDefaultSessionTimeout = 300
	// nxosFirmwareSessionTimeout is an increased timeout for long-running firmware operations (copy and install) to avoid NX-API session expiration.
	nxosFirmwareSessionTimeout = 1200
)

func (p *Provider) UpgradeFirmware(ctx context.Context, _ *deviceutil.Connection, target provider.TargetFirmware) error {
	logger := logr.FromContextOrDiscard(ctx)

	upgraded, err := p.isUpgraded(ctx, target)
	switch {
	case err != nil:
		return err
	case upgraded:
		return nil
	}

	// The copy and install commands block for several minutes, so they run on a
	// clone of the NX-API client that only differs in its longer timeout.
	nxapiUpgrade, err := p.nxapi.Clone(nxapi.WithTimeout(upgradeTimeout))
	if err != nil {
		return fmt.Errorf("failed to create long-timeout nxapi client: %w", err)
	}

	// Disable POAP and extend NX-API session timeouts before long-running commands.
	if _, err := p.nxapi.Do(ctx, nxapi.NewRequest(
		"configure",
		"no boot poap enable",
		fmt.Sprintf("system server session cmd-timeout %d", nxosFirmwareSessionTimeout),
	).WithRollback(nxapi.Stop)); err != nil {
		return fmt.Errorf("nxos firmware: prepare upgrade: %w", err)
	}

	targetFileName := path.Base(target.URL)

	if err := p.ensureFirmwareImage(ctx, nxapiUpgrade, target, targetFileName); err != nil {
		return err
	}
	if err := p.checkCompatibility(ctx, nxapiUpgrade, targetFileName); err != nil {
		return err
	}
	if err := p.doUpgrade(ctx, nxapiUpgrade, targetFileName); err != nil {
		return err
	}

	logger.V(1).Info("Reloading device to boot new firmware")
	// Reload is a separate request; connection drop is expected.
	if _, err := p.nxapi.Do(ctx, nxapi.NewRequest("reload")); err != nil && !nxapi.IsTransportError(err) {
		return fmt.Errorf("nxos firmware: reload failed: %w", err)
	}
	return provider.ErrUpgradeInProgress
}

// isUpgraded checks whether the device is already running the target firmware.
// If the device is running the target firmware, it also resets the NX-API session timeout to the default.
func (p *Provider) isUpgraded(ctx context.Context, target provider.TargetFirmware) (bool, error) {
	logger := logr.FromContextOrDiscard(ctx)

	// A transport-level failure here means the device is still unreachable
	// (typically mid-reload from a prior step), which is a normal in-progress
	// state rather than a failure, so signal the caller to requeue.
	bootImage := new(BootImage)
	if err := p.client.GetState(ctx, bootImage); err != nil {
		if isDeviceUnreachable(err) {
			logger.V(1).Info("Device unreachable during firmware completion check; treating as in progress")
			return false, provider.ErrUpgradeInProgress
		}
		return false, fmt.Errorf("nxos firmware: failed to read running version: %w", err)
	}

	targetFileName := path.Base(target.URL)
	if path.Base(string(*bootImage)) == targetFileName {
		logger.V(1).Info("Device already running target firmware", "filename", targetFileName)
		if _, err := p.nxapi.Do(ctx, nxapi.NewRequest(
			"configure",
			fmt.Sprintf("system server session cmd-timeout %d", nxosDefaultSessionTimeout),
		).WithRollback(nxapi.Stop)); err != nil {
			return false, fmt.Errorf("nxos firmware: reset session timeout: %w", err)
		}
		return true, nil
	}
	return false, nil
}

// ensureFirmwareImage ensures a valid firmware image is present on bootflash
func (p *Provider) ensureFirmwareImage(ctx context.Context, c *nxapi.Client, target provider.TargetFirmware, targetFileName string) error {
	logger := logr.FromContextOrDiscard(ctx)

	haveValidImage := false
	sum, err := p.fileMD5(ctx, targetFileName)
	if err != nil {
		return fmt.Errorf("nxos firmware: check existing image: %w", err)
	}
	switch {
	case sum == "":
		// absent — copy below.
	case target.MD5 == "":
		haveValidImage = true // no checksum to compare; presence is enough.
	case strings.EqualFold(sum, target.MD5):
		haveValidImage = true
	default:
		logger.V(1).Info("Stale image on bootflash, deleting", "file", targetFileName)
		if _, err := p.nxapi.Do(ctx, nxapi.NewRequest("delete bootflash:"+targetFileName+" no-prompt")); err != nil {
			return fmt.Errorf("nxos firmware: delete stale image: %w", err)
		}
	}

	if haveValidImage {
		return nil
	}

	size, err := remoteImageSize(ctx, target.URL)
	if err != nil {
		return err
	}
	dir, err := p.ListDirectory(ctx, "bootflash:")
	if err != nil {
		return fmt.Errorf("nxos firmware: dir bootflash: failed: %w", err)
	}
	if size > dir.Bytesfree {
		// TODO: Check if more than the current target and the current running image are present on the bootflash and delete them to free up space.
		return fmt.Errorf("nxos firmware: image (%d bytes) does not fit in bootflash free space (%d bytes)", size, dir.Bytesfree)
	}

	// The NX-OS `copy https://...` command unconditionally prompts
	// "Enter username:", which NX-API cannot answer. Depending on the
	// endpoint a dummy username results in 403 and http is not supported.
	// Downloading via `run bash wget` avoids the prompt entirely; bootflash
	// is mounted at /bootflash inside the bash shell. The management VRF is
	// a Linux netns, so wget must run inside it to reach the image server.
	dest := "/bootflash/" + targetFileName
	logger.V(1).Info("Copying firmware image to bootflash", "file", targetFileName)
	if _, err := c.Do(ctx, nxapi.NewRequest(
		"feature bash-shell",
		//nolint:dupword // NX-OS requires `run bash bash -c` here.
		`run bash bash -c 'ip netns exec management wget --no-verbose --output-document="$1" "$2"' -- `+strconv.Quote(dest)+` `+
			strconv.Quote(target.URL),
	).WithRollback(nxapi.Stop)); err != nil {
		return fmt.Errorf("nxos firmware: copy image: %w", err)
	}

	if target.MD5 != "" {
		sum, err := p.fileMD5(ctx, targetFileName)
		if err != nil {
			return fmt.Errorf("nxos firmware: verify md5: %w", err)
		}
		if !strings.EqualFold(sum, target.MD5) {
			return fmt.Errorf("nxos firmware: md5 mismatch after copy: got %s want %s", sum, target.MD5)
		}
	}
	logger.V(1).Info("Firmware image copied and verified")
	return nil
}

// checkCompatibility runs the software compatibility and install impact checks
// and logs their output.
func (p *Provider) checkCompatibility(ctx context.Context, c *nxapi.Client, targetFileName string) error {
	logger := logr.FromContextOrDiscard(ctx)
	compatRes, err := c.Do(ctx, nxapi.NewRequest(
		"show incompatibility-all nxos bootflash:"+targetFileName,
	).WithMethod(nxapi.MethodCLIASCII))
	if err != nil {
		return fmt.Errorf("nxos firmware: compatibility check failed: %w", err)
	}
	logCLIResult(logger, compatRes, "Software compatibility check result")

	impactRes, err := c.Do(ctx, nxapi.NewRequest(
		"show install all impact nxos bootflash:"+targetFileName,
	).WithMethod(nxapi.MethodCLIASCII))
	if err != nil {
		return fmt.Errorf("nxos firmware: install impact check failed: %w", err)
	}
	logCLIResult(logger, impactRes, "Install impact check result")
	return nil
}

// doUpgrade saves the running config, installs the firmware without
// reload, and resets the session timeout to the default.
func (p *Provider) doUpgrade(ctx context.Context, c *nxapi.Client, targetFileName string) error {
	logger := logr.FromContextOrDiscard(ctx)
	if _, err := p.nxapi.Do(ctx, nxapi.NewRequest(
		"copy running-config startup-config",
	).WithRollback(nxapi.Stop)); err != nil {
		return fmt.Errorf("nxos firmware: save config: %w", err)
	}

	// install all with no-reload keeps the connection up and returns the real result.
	logger.V(1).Info("Installing firmware (no-reload)", "file", targetFileName)
	installRes, err := c.Do(ctx, nxapi.NewRequest(
		"install all nxos bootflash:"+targetFileName+" no-reload",
	).WithMethod(nxapi.MethodCLIASCII).WithRollback(nxapi.Stop))
	if err != nil {
		return fmt.Errorf("nxos firmware: install failed: %w", err)
	}
	logCLIResult(logger, installRes, "Install result")
	return nil
}

// fileMD5 returns the md5 checksum of a file on bootflash, or an empty string
// if the file does not exist. A "file not found" RPC error from the device is
// treated as absence, not a failure, so callers can proceed to copy the image.
func (p *Provider) fileMD5(ctx context.Context, filename string) (string, error) {
	res, err := p.nxapi.Do(ctx, nxapi.NewRequest("show file bootflash:"+filename+" md5sum"))
	if err != nil {
		if isFileNotFound(err) {
			return "", nil
		}
		return "", err
	}
	if len(res) == 0 {
		return "", nil
	}
	var result struct {
		MD5Sum string `json:"file_content_md5sum"`
	}
	if err := json.Unmarshal(res[0], &result); err != nil {
		return "", fmt.Errorf("nxos firmware: failed to decode md5sum response: %w", err)
	}
	return strings.TrimSpace(result.MD5Sum), nil
}

// isFileNotFound reports whether err is an NX-API RPC error indicating that a
// referenced file does not exist on the device, so callers can distinguish a
// missing image (an expected pre-copy state) from a genuine failure.
func isFileNotFound(err error) bool {
	var rpcErr *nxapi.RPCError
	if !errors.As(err, &rpcErr) {
		return false
	}
	msg := strings.ToLower(rpcErr.Error())
	return strings.Contains(msg, "no such file") || strings.Contains(msg, "not found")
}

// isDeviceUnreachable reports whether err indicates the device is temporarily
// unreachable (as opposed to a logical error), covering both NX-API transport
// errors and gNMI/gRPC unavailability. This is expected while the device is
// rebooting after an install and should be treated as an in-progress state.
func isDeviceUnreachable(err error) bool {
	if err == nil {
		return false
	}
	if nxapi.IsTransportError(err) {
		return true
	}
	switch status.Code(err) {
	case codes.Unavailable, codes.DeadlineExceeded:
		return true
	default:
		return false
	}
}

// remoteImageSize issues an HTTP HEAD to the firmware URL and returns its
// Content-Length in bytes.
func remoteImageSize(ctx context.Context, rawURL string) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, rawURL, nil)
	if err != nil {
		return 0, fmt.Errorf("nxos firmware: build HEAD request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("nxos firmware: HEAD %s: %w", rawURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, fmt.Errorf("nxos firmware: HEAD %s returned status %d", rawURL, resp.StatusCode)
	}
	if resp.ContentLength < 0 {
		return 0, fmt.Errorf("nxos firmware: HEAD %s did not return a Content-Length", rawURL)
	}
	return resp.ContentLength, nil
}

// configSessionActive reports whether a configuration session is currently
// open on the device (which would block ISSU).
func (p *Provider) configSessionActive(ctx context.Context) (bool, error) {
	res, err := p.nxapi.Do(ctx, nxapi.NewRequest("show configuration session summary"))
	if err != nil {
		return false, err
	}
	if len(res) == 0 {
		return false, nil
	}
	var body struct {
		Table struct {
			Row json.RawMessage `json:"ROW_session"`
		} `json:"TABLE_session"`
	}
	if err := json.Unmarshal(res[0], &body); err != nil {
		return false, nil //nolint:nilerr // unmarshal failure means no session table => no sessions
	}
	return len(body.Table.Row) > 0, nil
}

// progressBarRe matches NX-OS CLI progress bar segments like
// "[####      ]  25%" that pollute cli_ascii output.
var progressBarRe = regexp.MustCompile(`\[[#\s]*\]\s*\d+%`)

// statusMarkerRe matches a trailing " -- SUCCESS" style status marker left on a
// line after its progress bars are stripped, capturing the marker word. The
// marker must be an uppercase word so table separator lines ending in a run of
// dashes (e.g. "------  ------") are not mistaken for markers.
var statusMarkerRe = regexp.MustCompile(`(?m)\s*--\s*([A-Z]+)\s*$`)

// cleanCLIOutput tidies raw cli_ascii command output for logging by stripping
// progress bar segments, moving trailing status markers (e.g. "-- SUCCESS")
// onto their own line without the leading "--", and dropping blank lines.
func cleanCLIOutput(s string) string {
	s = progressBarRe.ReplaceAllString(s, "")
	s = statusMarkerRe.ReplaceAllString(s, "\n$1")
	var kept []string
	for line := range strings.SplitSeq(s, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		kept = append(kept, strings.TrimRight(line, " \t"))
	}
	return strings.Join(kept, "\n")
}

// logCLIResult logs the cli_ascii output from an NX-API response, if present.
func logCLIResult(logger logr.Logger, res []json.RawMessage, msg string) {
	if len(res) == 0 {
		return
	}
	var output string
	if err := json.Unmarshal(res[0], &output); err == nil {
		logger.V(1).Info(msg, "output", cleanCLIOutput(output))
	}
}
