// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ironcore-dev/network-operator/internal/deviceutil"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
	"github.com/ironcore-dev/network-operator/internal/transport/nxapi"
)

func TestCleanCLIOutput(t *testing.T) {
	in := "Installer will perform compatibility check first. Please wait. \nInstaller will exit before reload\nInstaller is forced disruptive\n\nVerifying image bootflash:/nxos64-cs.10.6.3.F.bin for boot variable \"nxos\".\n[#                   ]   0%[####################] 100% -- SUCCESS\n\nVerifying EPLD/FPGA image //bootflash/nxos64-cs.10.6.3.F.bin.\n[#                   ]   0%[####################] 100% -- SUCCESS\n\nVerifying image type.\n[#                   ]   0%[####################] 100% -- SUCCESS\n\nPreparing \"nxos\" version info using image bootflash:/nxos64-cs.10.6.3.F.bin.\n[#                   ]   0%[####################] 100% -- SUCCESS\n\nPreparing \"bios\" version info using image bootflash:/nxos64-cs.10.6.3.F.bin.\n[#                   ]   0%[####################] 100% -- SUCCESS\n\nPerforming module support checks.\n[####################] 100% -- SUCCESS\n\nNotifying services about system upgrade.\n[####################] 100% -- SUCCESS\n\n\n\nCompatibility check is done:\nModule  bootable          Impact  Install-type  Reason\n------  --------  --------------  ------------  ------\n     1       yes      disruptive         reset  default upgrade is not hitless\n    27       yes      disruptive         reset  default upgrade is not hitless\n\n\n\nImages will be upgraded according to following table:\nModule       Image                  Running-Version(pri:alt)           New-Version  Upg-Required\n------  ----------  ----------------------------------------  --------------------  ------------\n     1       lcn9k                                   10.6(2)               10.6(3)           yes\n    27        nxos                                   10.6(2)               10.6(3)           yes\n    27        bios     v05.53(01/22/2025):v05.47(04/28/2022)    v05.53(01/22/2025)            no\n\n\nFPGA microcode will be upgraded according to following table:\nModule  Type  EPLD               Running-Version  Flashed-Version*   New-Version  Upg-Required\n------  ----  -------------      ---------------  ----------------   -----------  ------------\n    27   SUP  MI FPGA            0x5               0x5               0x5                   No\n    27   SUP  IO FPGA            0x17              0x17              0x18                 Yes\n* If Running-Version and Flashed-Version are different it implies that the system has not yet been reloaded for the new version to take effect\n\nEPLD Upgrade may result in multiple modules going offline.\n\nAdditional info for this installation:\n--------------------------------------\n\nOption \"no-reload\" has been used - it is necessary reload device after installation without saving config.\nSaving config before can result incorrect startup  config load after reload with new version of NXOS.\n\nService \"vpc\" in vdc 1: Vpc is enabled, Please make sure both Vpc peer switches have same boot mode using 'show boot mode' and proceed \n\n\n\n\n\nInstall is in progress, please wait.\n[#                   ]   0%\nSetting boot variables.\n[####################] 100% -- SUCCESS\n\nPerforming configuration copy.\n[#                   ]   0%[#                   ]   0%[######              ]  25%[###########         ]  50%[################    ]  75%[####################] 100%\nPerforming configuration copy.\n[####################] 100% -- SUCCESS\n\nModule 1: Refreshing compact flash and upgrading bios/loader/bootrom.\nWarning: please do not remove or power off the module at this time.\n[#                   ]   0%\nModule 1: Refreshing compact flash and upgrading bios/loader/bootrom.\nWarning: please do not remove or power off the module at this time.\n[####################] 100% -- SUCCESS\n\nModule 27: Refreshing compact flash and upgrading bios/loader/bootrom.\nWarning: please do not remove or power off the module at this time.\n[#                   ]   0%\nModule 27: Refreshing compact flash and upgrading bios/loader/bootrom.\nWarning: please do not remove or power off the module at this time.\n[####################] 100% -- SUCCESS\n\nEPLD/FPGA upgrade can take upto  4  mins\n[#                   ]   0%\nPerforming EPLD/FPGA upgrade .\n[####################] 100% -- SUCCESS\n\n\n"

	want := "Installer will perform compatibility check first. Please wait.\nInstaller will exit before reload\nInstaller is forced disruptive\nVerifying image bootflash:/nxos64-cs.10.6.3.F.bin for boot variable \"nxos\".\nSUCCESS\nVerifying EPLD/FPGA image //bootflash/nxos64-cs.10.6.3.F.bin.\nSUCCESS\nVerifying image type.\nSUCCESS\nPreparing \"nxos\" version info using image bootflash:/nxos64-cs.10.6.3.F.bin.\nSUCCESS\nPreparing \"bios\" version info using image bootflash:/nxos64-cs.10.6.3.F.bin.\nSUCCESS\nPerforming module support checks.\nSUCCESS\nNotifying services about system upgrade.\nSUCCESS\nCompatibility check is done:\nModule  bootable          Impact  Install-type  Reason\n------  --------  --------------  ------------  ------\n     1       yes      disruptive         reset  default upgrade is not hitless\n    27       yes      disruptive         reset  default upgrade is not hitless\nImages will be upgraded according to following table:\nModule       Image                  Running-Version(pri:alt)           New-Version  Upg-Required\n------  ----------  ----------------------------------------  --------------------  ------------\n     1       lcn9k                                   10.6(2)               10.6(3)           yes\n    27        nxos                                   10.6(2)               10.6(3)           yes\n    27        bios     v05.53(01/22/2025):v05.47(04/28/2022)    v05.53(01/22/2025)            no\nFPGA microcode will be upgraded according to following table:\nModule  Type  EPLD               Running-Version  Flashed-Version*   New-Version  Upg-Required\n------  ----  -------------      ---------------  ----------------   -----------  ------------\n    27   SUP  MI FPGA            0x5               0x5               0x5                   No\n    27   SUP  IO FPGA            0x17              0x17              0x18                 Yes\n* If Running-Version and Flashed-Version are different it implies that the system has not yet been reloaded for the new version to take effect\nEPLD Upgrade may result in multiple modules going offline.\nAdditional info for this installation:\n--------------------------------------\nOption \"no-reload\" has been used - it is necessary reload device after installation without saving config.\nSaving config before can result incorrect startup  config load after reload with new version of NXOS.\nService \"vpc\" in vdc 1: Vpc is enabled, Please make sure both Vpc peer switches have same boot mode using 'show boot mode' and proceed\nInstall is in progress, please wait.\nSetting boot variables.\nSUCCESS\nPerforming configuration copy.\nPerforming configuration copy.\nSUCCESS\nModule 1: Refreshing compact flash and upgrading bios/loader/bootrom.\nWarning: please do not remove or power off the module at this time.\nModule 1: Refreshing compact flash and upgrading bios/loader/bootrom.\nWarning: please do not remove or power off the module at this time.\nSUCCESS\nModule 27: Refreshing compact flash and upgrading bios/loader/bootrom.\nWarning: please do not remove or power off the module at this time.\nModule 27: Refreshing compact flash and upgrading bios/loader/bootrom.\nWarning: please do not remove or power off the module at this time.\nSUCCESS\nEPLD/FPGA upgrade can take upto  4  mins\nPerforming EPLD/FPGA upgrade .\nSUCCESS"

	got := cleanCLIOutput(in)
	if got != want {
		t.Errorf("cleanCLIOutput mismatch:\ngot:\n%s\n\nwant:\n%s", got, want)
	}
}

// fakeGNMI is a minimal gnmiext.Client that returns a canned running version.
type fakeGNMI struct {
	gnmiext.Client
	version   string
	bootImage string
	// getStateErr, when non-nil, is returned by GetState to simulate an
	// unreachable device (e.g. mid-reload).
	getStateErr error
}

func (f *fakeGNMI) GetState(_ context.Context, elems ...gnmiext.DataElement) error {
	if f.getStateErr != nil {
		return f.getStateErr
	}
	for _, e := range elems {
		switch v := e.(type) {
		case *FirmwareVersion:
			*v = FirmwareVersion(f.version)
		case *BootImage:
			*v = BootImage(f.bootImage)
		}
	}
	return nil
}

func (f *fakeGNMI) GetConfig(_ context.Context, elems ...gnmiext.DataElement) error {
	for _, e := range elems {
		if h, ok := e.(*Hostname); ok {
			*h = Hostname("test-switch")
		}
	}
	return nil
}

func TestUpgradeFirmwareAlreadyOnTarget(t *testing.T) {
	client, conn := nxapiStub(t, func(cmds []string) []string {
		bodies := make([]string, len(cmds))
		for i := range bodies {
			bodies[i] = "null"
		}
		return bodies
	})
	p := &Provider{client: &fakeGNMI{bootImage: "bootflash://nxos64-cs.10.6.3.F.bin"}, nxapi: client}
	target := provider.TargetFirmware{
		URL: "https://repo.example/nxos64-cs.10.6.3.F.bin",
		MD5: "48c0db0a564c442f123eba8724ef352f",
	}
	if err := p.UpgradeFirmware(t.Context(), conn, target); err != nil {
		t.Fatalf("expected nil (already upgraded), got %v", err)
	}
}

func TestUpgradeFirmwareUnreachableDuringProbe(t *testing.T) {
	// Device unreachable during the completion check (e.g. mid-reload) must be
	// treated as in progress so the reconcile requeues instead of failing.
	p := &Provider{client: &fakeGNMI{getStateErr: status.Error(codes.Unavailable, "connection refused")}}
	target := provider.TargetFirmware{URL: "https://repo.example/nxos64-cs.10.6.3.F.bin"}
	err := p.UpgradeFirmware(t.Context(), &deviceutil.Connection{}, target)
	if !errors.Is(err, provider.ErrUpgradeInProgress) {
		t.Fatalf("expected ErrUpgradeInProgress for unreachable device, got %v", err)
	}
}

// nxapiStub starts an httptest server that responds to each NX-API batch using
// the provided handler, which maps the list of commands to a JSON body string
// (the ".result.body" payload) for each command, in order. It returns both the
// client and the connection so tests can pass conn to UpgradeFirmware for the
// long-timeout client it creates internally.
func nxapiStub(t *testing.T, handler func(cmds []string) []string) (*nxapi.Client, *deviceutil.Connection) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqCmds []struct {
			Params struct {
				Cmd string `json:"cmd"`
			} `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&reqCmds); err != nil {
			t.Fatalf("stub: decode request: %v", err)
		}
		cmds := make([]string, len(reqCmds))
		for i, c := range reqCmds {
			cmds[i] = c.Params.Cmd
		}
		bodies := handler(cmds)
		w.Header().Set("Content-Type", "application/json-rpc")
		fmt.Fprint(w, "[")
		for i, b := range bodies {
			if i > 0 {
				fmt.Fprint(w, ",")
			}
			fmt.Fprintf(w, `{"jsonrpc":"2.0","result":{"body":%s},"id":%d}`, b, i+1)
		}
		fmt.Fprint(w, "]")
	}))
	t.Cleanup(srv.Close)
	_, port, _ := net.SplitHostPort(srv.Listener.Addr().String()) //nolint:errcheck
	conn := &deviceutil.Connection{Address: srv.Listener.Addr().String(), Username: "admin", Password: "secret"}
	client, err := nxapi.NewClient(conn, nxapi.WithPort(port))
	if err != nil {
		t.Fatalf("stub: new client: %v", err)
	}
	return client, conn
}

func TestListDirectoryBytesfree(t *testing.T) {
	client, _ := nxapiStub(t, func(cmds []string) []string {
		if cmds[0] != "dir bootflash:" {
			t.Errorf("cmd = %q, want 'dir bootflash:'", cmds[0])
		}
		return []string{`{"bytesfree":3664789504}`}
	})
	p := &Provider{nxapi: client}
	dir, err := p.ListDirectory(t.Context(), "bootflash:")
	if err != nil {
		t.Fatalf("ListDirectory error: %v", err)
	}
	if dir.Bytesfree != 3664789504 {
		t.Errorf("dir.Bytesfree = %d, want 3664789504", dir.Bytesfree)
	}
}

func TestFileMD5(t *testing.T) {
	client, _ := nxapiStub(t, func(cmds []string) []string {
		want := "show file bootflash:nxos64-cs.10.6.3.F.bin md5sum"
		if cmds[0] != want {
			t.Errorf("cmd = %q, want %q", cmds[0], want)
		}
		return []string{`{"file_content_md5sum":"48c0db0a564c442f123eba8724ef352f\n"}`}
	})
	p := &Provider{nxapi: client}
	got, err := p.fileMD5(t.Context(), "nxos64-cs.10.6.3.F.bin")
	if err != nil {
		t.Fatalf("fileMD5 error: %v", err)
	}
	if got != "48c0db0a564c442f123eba8724ef352f" {
		t.Errorf("fileMD5 = %q", got)
	}
}

// nxapiErrorStub starts an httptest server that responds to every NX-API batch
// with a single JSON-RPC error carrying the given code and message, so tests
// can exercise how helpers react to device-side command failures.
func nxapiErrorStub(t *testing.T, code int, message string) *nxapi.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json-rpc")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `[{"jsonrpc":"2.0","error":{"code":%d,"message":%q},"id":1}]`, code, message)
	}))
	t.Cleanup(srv.Close)
	_, port, _ := net.SplitHostPort(srv.Listener.Addr().String()) //nolint:errcheck
	conn := &deviceutil.Connection{Address: srv.Listener.Addr().String(), Username: "admin", Password: "secret"}
	client, err := nxapi.NewClient(conn, nxapi.WithPort(port))
	if err != nil {
		t.Fatalf("error stub: new client: %v", err)
	}
	return client
}

func TestFileMD5NotFound(t *testing.T) {
	p := &Provider{nxapi: nxapiErrorStub(t, 1, "No such file or directory")}
	got, err := p.fileMD5(t.Context(), "nxos64-cs.10.6.3.F.bin")
	if err != nil {
		t.Fatalf("fileMD5 error: %v", err)
	}
	if got != "" {
		t.Errorf("fileMD5 = %q, want empty string for missing file", got)
	}
}

func TestFileMD5RealError(t *testing.T) {
	p := &Provider{nxapi: nxapiErrorStub(t, 500, "internal device error")}
	if _, err := p.fileMD5(t.Context(), "nxos64-cs.10.6.3.F.bin"); err == nil {
		t.Fatal("expected error for non-not-found RPC failure, got nil")
	}
}

func TestConfigSessionActive(t *testing.T) {
	client1, _ := nxapiStub(t, func(cmds []string) []string {
		return []string{`{"TABLE_session":{"ROW_session":[{"session":"s1"}]}}`}
	})
	p := &Provider{nxapi: client1}
	active, err := p.configSessionActive(t.Context())
	if err != nil {
		t.Fatalf("configSessionActive error: %v", err)
	}
	if !active {
		t.Error("expected active session, got false")
	}

	client2, _ := nxapiStub(t, func(cmds []string) []string {
		return []string{`{}`}
	})
	p2 := &Provider{nxapi: client2}
	active2, err := p2.configSessionActive(t.Context())
	if err != nil {
		t.Fatalf("configSessionActive error: %v", err)
	}
	if active2 {
		t.Error("expected no active session, got true")
	}
}

func TestRemoteImageSize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Errorf("method = %s, want HEAD", r.Method)
		}
		w.Header().Set("Content-Length", "3005853696")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	got, err := remoteImageSize(t.Context(), srv.URL+"/nxos64-cs.10.6.3.F.bin")
	if err != nil {
		t.Fatalf("remoteImageSize error: %v", err)
	}
	if got != 3005853696 {
		t.Errorf("remoteImageSize = %d, want 3005853696", got)
	}
}

func TestUpgradeFirmwareCopyStep(t *testing.T) {
	// Device on old version, image absent -> preflight + copy issued -> in progress.
	var got []string
	copied := false
	client, conn := nxapiStub(t, func(cmds []string) []string {
		got = append(got, cmds...)
		bodies := make([]string, len(cmds))
		for i, c := range cmds {
			switch {
			case c == "show file bootflash:nxos64-cs.10.6.3.F.bin md5sum":
				if copied {
					bodies[i] = `{"file_content_md5sum":"48c0db0a564c442f123eba8724ef352f"}` // present after copy
				} else {
					bodies[i] = `{"file_content_md5sum":""}` // absent before copy
				}
			case c == "show configuration session summary":
				bodies[i] = `{}`
			case c == "dir bootflash:":
				bodies[i] = `{"bytesfree":6000000000}`
			case strings.HasPrefix(c, "run bash"):
				copied = true
				bodies[i] = `""`
			default:
				bodies[i] = `""`
			}
		}
		return bodies
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "3005853696")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	p := &Provider{
		client: &fakeGNMI{bootImage: "bootflash://nxos64-cs.10.6.2.F.bin"},
		nxapi:  client,
	}
	target := provider.TargetFirmware{
		URL: srv.URL + "/nxos64-cs.10.6.3.F.bin",
		MD5: "48c0db0a564c442f123eba8724ef352f",
	}
	err := p.UpgradeFirmware(t.Context(), conn, target)
	if !errors.Is(err, provider.ErrUpgradeInProgress) {
		t.Fatalf("expected ErrUpgradeInProgress, got %v", err)
	}
	joined := strings.Join(got, "|")
	if !strings.Contains(joined, "run bash") {
		t.Errorf("wget copy command not issued; got %v", got)
	}
}

func TestUpgradeFirmwareInsufficientSpace(t *testing.T) {
	client, conn := nxapiStub(t, func(cmds []string) []string {
		bodies := make([]string, len(cmds))
		for i, c := range cmds {
			switch c {
			case "show file bootflash:nxos64-cs.10.6.3.F.bin md5sum":
				bodies[i] = `""`
			case "show configuration session summary":
				bodies[i] = `{}`
			case "dir bootflash:":
				bodies[i] = `{"bytesfree":"1000"}`
			default:
				bodies[i] = `""`
			}
		}
		return bodies
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "3005853696")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	p := &Provider{client: &fakeGNMI{version: "10.6(2)"}, nxapi: client}
	target := provider.TargetFirmware{URL: srv.URL + "/nxos64-cs.10.6.3.F.bin", MD5: "abc"}
	err := p.UpgradeFirmware(t.Context(), conn, target)
	if err == nil || errors.Is(err, provider.ErrUpgradeInProgress) {
		t.Fatalf("expected hard error for insufficient space, got %v", err)
	}
}

func TestUpgradeFirmwareInstallAndReload(t *testing.T) {
	// Image present with matching md5 -> impact + save + install(no-reload) + reload.
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqCmds []struct {
			Params struct {
				Cmd string `json:"cmd"`
			} `json:"params"`
		}
		json.NewDecoder(r.Body).Decode(&reqCmds) //nolint:errcheck
		cmds := make([]string, len(reqCmds))
		for i, c := range reqCmds {
			cmds[i] = c.Params.Cmd
		}
		got = append(got, cmds...)

		// The reload request drops the connection.
		if len(cmds) == 1 && cmds[0] == "reload" {
			hj, ok := w.(http.Hijacker)
			if !ok {
				t.Fatal("no hijacker")
			}
			conn, _, _ := hj.Hijack() //nolint:errcheck
			conn.Close()
			return
		}
		w.Header().Set("Content-Type", "application/json-rpc")
		fmt.Fprint(w, "[")
		for i, c := range cmds {
			if i > 0 {
				fmt.Fprint(w, ",")
			}
			body := `{"file_content_md5sum":""}`
			if c == "show file bootflash:nxos64-cs.10.6.3.F.bin md5sum" {
				body = `{"file_content_md5sum":"48c0db0a564c442f123eba8724ef352f"}`
			}
			fmt.Fprintf(w, `{"jsonrpc":"2.0","result":{"body":%s},"id":%d}`, body, i+1)
		}
		fmt.Fprint(w, "]")
	}))
	t.Cleanup(srv.Close)
	_, port, _ := net.SplitHostPort(srv.Listener.Addr().String()) //nolint:errcheck
	conn := &deviceutil.Connection{Address: srv.Listener.Addr().String(), Username: "admin", Password: "secret"}
	client, err := nxapi.NewClient(conn, nxapi.WithPort(port))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	p := &Provider{client: &fakeGNMI{version: "10.6(2)"}, nxapi: client}
	target := provider.TargetFirmware{URL: "https://repo.example/nxos64-cs.10.6.3.F.bin", MD5: "48c0db0a564c442f123eba8724ef352f"}
	err = p.UpgradeFirmware(t.Context(), conn, target)
	if !errors.Is(err, provider.ErrUpgradeInProgress) {
		t.Fatalf("expected ErrUpgradeInProgress after reload, got %v", err)
	}
	joined := strings.Join(got, "|")
	for _, want := range []string{
		"show install all impact nxos bootflash:nxos64-cs.10.6.3.F.bin",
		"copy running-config startup-config",
		"install all nxos bootflash:nxos64-cs.10.6.3.F.bin no-reload",
		"reload",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing command %q; got %v", want, got)
		}
	}
}
