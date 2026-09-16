// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

// mockGNMI returns a gnmiext.ClientMock that reports the given running version
// and boot image on GetState and a canned hostname on GetConfig.
func mockGNMI(version, bootImage string) *gnmiext.ClientMock {
	return &gnmiext.ClientMock{
		GetStateFunc: func(_ context.Context, elems ...gnmiext.DataElement) error {
			for _, e := range elems {
				switch v := e.(type) {
				case *FirmwareVersion:
					*v = FirmwareVersion(version)
				case *BootImage:
					*v = BootImage(bootImage)
				}
			}
			return nil
		},
		GetConfigFunc: func(_ context.Context, elems ...gnmiext.DataElement) error {
			for _, e := range elems {
				if h, ok := e.(*Hostname); ok {
					*h = Hostname("test-switch")
				}
			}
			return nil
		},
	}
}

func TestUpgradeFirmwareAlreadyOnTarget(t *testing.T) {
	client := nxapi.NewClientMock(func(_ context.Context, r nxapi.Request) ([]json.RawMessage, error) {
		return []json.RawMessage{json.RawMessage(`null`)}, nil
	})

	p := &Provider{client: mockGNMI("", "bootflash://nxos64-cs.10.6.3.F.bin"), nxapi: client}
	target := provider.TargetFirmware{
		URL: "https://repo.example/nxos64-cs.10.6.3.F.bin",
		MD5: "48c0db0a564c442f123eba8724ef352f",
	}
	if err := p.UpgradeFirmware(t.Context(), nil, target); err != nil {
		t.Fatalf("expected nil (already upgraded), got %v", err)
	}
}

func TestListDirectoryBytesfree(t *testing.T) {
	client := nxapi.NewClientMock(func(_ context.Context, r nxapi.Request) ([]json.RawMessage, error) {
		if got := r.Commands()[0]; got != "dir bootflash:" {
			t.Errorf("cmd = %q, want 'dir bootflash:'", got)
		}
		return []json.RawMessage{json.RawMessage(`{"bytesfree":3664789504}`)}, nil
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
	client := nxapi.NewClientMock(func(_ context.Context, r nxapi.Request) ([]json.RawMessage, error) {
		want := "show file bootflash:nxos64-cs.10.6.3.F.bin md5sum"
		if got := r.Commands()[0]; got != want {
			t.Errorf("cmd = %q, want %q", got, want)
		}
		return []json.RawMessage{json.RawMessage(`{"file_content_md5sum":"48c0db0a564c442f123eba8724ef352f\n"}`)}, nil
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

func TestFileMD5NotFound(t *testing.T) {
	p := &Provider{nxapi: nxapi.MockErrorClient(1, "No such file or directory")}
	got, err := p.fileMD5(t.Context(), "nxos64-cs.10.6.3.F.bin")
	if err != nil {
		t.Fatalf("fileMD5 error: %v", err)
	}
	if got != "" {
		t.Errorf("fileMD5 = %q, want empty string for missing file", got)
	}
}

func TestFileMD5RealError(t *testing.T) {
	p := &Provider{nxapi: nxapi.MockErrorClient(500, "internal device error")}
	if _, err := p.fileMD5(t.Context(), "nxos64-cs.10.6.3.F.bin"); err == nil {
		t.Fatal("expected error for non-not-found RPC failure, got nil")
	}
}

func TestConfigSessionActive(t *testing.T) {
	client1 := nxapi.NewClientMock(func(_ context.Context, r nxapi.Request) ([]json.RawMessage, error) {
		if len(r.Commands()) == 1 && r.Commands()[0] == "show configuration session summary" {
			return []json.RawMessage{json.RawMessage(`{"TABLE_session":{"ROW_session":[{"session":"s1"}]}}`)}, nil
		}
		return nil, errors.New("unexpected command(s)")
	})
	p := &Provider{nxapi: client1}
	active, err := p.configSessionActive(t.Context())
	if err != nil {
		t.Fatalf("configSessionActive error: %v", err)
	}
	if !active {
		t.Error("expected active session, got false")
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
	client := nxapi.NewClientMock(func(_ context.Context, r nxapi.Request) ([]json.RawMessage, error) {
		cmds := r.Commands()
		got = append(got, cmds...)
		msgs := make([]json.RawMessage, len(cmds))
		for i, c := range cmds {
			switch {
			case c == "show file bootflash:nxos64-cs.10.6.3.F.bin md5sum":
				if copied {
					msgs[i] = json.RawMessage(`{"file_content_md5sum":"48c0db0a564c442f123eba8724ef352f"}`) // present after copy
				} else {
					msgs[i] = json.RawMessage(`{"file_content_md5sum":""}`) // absent before copy
				}
			case c == "dir bootflash:":
				msgs[i] = json.RawMessage(`{"bytesfree":6000000000}`)
			case strings.HasPrefix(c, "run bash"):
				copied = true
				msgs[i] = json.RawMessage(`""`)
			default:
				msgs[i] = json.RawMessage(`""`)
			}
		}
		return msgs, nil
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "3005853696")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	p := &Provider{
		client: mockGNMI("", "bootflash://nxos64-cs.10.6.2.F.bin"),
		nxapi:  client,
	}
	target := provider.TargetFirmware{
		URL: srv.URL + "/nxos64-cs.10.6.3.F.bin",
		MD5: "48c0db0a564c442f123eba8724ef352f",
	}
	err := p.UpgradeFirmware(t.Context(), nil, target)
	if !errors.Is(err, provider.ErrMaintenanceInProgress) {
		t.Fatalf("expected ErrUpgradeInProgress, got %v", err)
	}
	joined := strings.Join(got, "|")
	if !strings.Contains(joined, "run bash") {
		t.Errorf("wget copy command not issued; got %v", got)
	}
}

func TestUpgradeFirmwareInsufficientSpace(t *testing.T) {
	client := nxapi.NewClientMock(func(_ context.Context, r nxapi.Request) ([]json.RawMessage, error) {
		cmds := r.Commands()
		msgs := make([]json.RawMessage, len(cmds))
		for i, c := range cmds {
			if c == "dir bootflash:" {
				msgs[i] = json.RawMessage(`{"bytesfree":"1000"}`)
			} else {
				msgs[i] = json.RawMessage(`null`)
			}
		}
		return msgs, nil
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "3005853696")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	p := &Provider{client: mockGNMI("10.6(2)", ""), nxapi: client}
	target := provider.TargetFirmware{URL: srv.URL + "/nxos64-cs.10.6.3.F.bin", MD5: "abc"}
	err := p.UpgradeFirmware(t.Context(), nil, target)
	if err == nil || errors.Is(err, provider.ErrMaintenanceInProgress) {
		t.Fatalf("expected hard error for insufficient space, got %v", err)
	}
}

func TestUpgradeFirmwareInstallAndReload(t *testing.T) {
	// Image present with matching md5 -> impact + save + install(no-reload) + reload.
	var got []string
	client := nxapi.NewClientMock(func(_ context.Context, r nxapi.Request) ([]json.RawMessage, error) {
		cmds := r.Commands()
		got = append(got, cmds...)
		// The reload request drops the connection, surfacing as a transport
		// error that UpgradeFirmware treats as expected (device going down).
		if len(cmds) == 1 && cmds[0] == "reload" {
			return nil, io.EOF
		}
		msgs := make([]json.RawMessage, len(cmds))
		for i, c := range cmds {
			if c == "show file bootflash:nxos64-cs.10.6.3.F.bin md5sum" {
				msgs[i] = json.RawMessage(`{"file_content_md5sum":"48c0db0a564c442f123eba8724ef352f"}`)
			} else {
				msgs[i] = json.RawMessage(`null`)
			}
		}
		return msgs, nil
	})

	p := &Provider{client: mockGNMI("10.6(2)", ""), nxapi: client}
	target := provider.TargetFirmware{URL: "https://repo.example/nxos64-cs.10.6.3.F.bin", MD5: "48c0db0a564c442f123eba8724ef352f"}
	err := p.UpgradeFirmware(t.Context(), nil, target)
	if !errors.Is(err, provider.ErrMaintenanceInProgress) {
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
