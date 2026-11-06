# Zero-Touch Provisioning for Cisco NX-OS

Zero-Touch Provisioning (ZTP), referred to as POAP (Power-On Auto Provisioning) on Cisco NX-OS, lets a switch bootstrap itself automatically on first boot, without manual intervention.

This page documents only the Cisco NX-OS specifics. For the platform-agnostic model (the end-to-end flow, DHCP, the `Device` resource, lifecycle phases, HTTP endpoints, source validation, and the provider hook interface) see [Device Provisioning](provisioning.md).

## Boot script

The network operator ships its own POAP boot script at [`hack/ztp/nxos.py`](https://github.com/ironcore-dev/network-operator/blob/main/hack/ztp/nxos.py). It is not Cisco's reference POAP script; it talks to the operator's provisioning API and uses `install all` (see below). Site-specific values like the provisioning server URL have been replaced with placeholders and must be adapted to your environment before use.

A switch enters POAP when it boots with no startup configuration (`write erase` + `reload`). NX-OS expects the boot-script location in DHCP option 67 (`bootfile-name`) and fetches the script over TFTP. The script reads the switch serial from `show version` (`proc_board_id`) and uses it as the `serial` parameter for the operator handshake.

## Image download and upgrade

The boot script downloads the NX-OS image from the URL provided by the operator and verifies the checksum. Cisco's reference POAP script uses `boot nxos`, which is no longer recommended in current Cisco documentation. The network operator uses `install all` instead:

```
install all nxos <image-path>
```

`install all` runs compatibility checks and also updates BIOS firmware when the new image contains a newer version. Since NX-OS 10.5(3) the EPLD firmware is bundled into the `.bin`, so `install all` upgrades it too when required (with an exception for switches affected by the Secure Boot vulnerability, which need a manual `install epld`).

Driving `install all` from within POAP has some quirks the operator's boot script works around:

- `install all` refuses to run while `boot poap enable` is set. The script disables POAP (`no boot poap enable`) and saves the running config before invoking it.
- The script runs `install all nxos <image> no-reload` so it can stage the device configuration before the reboot, rather than letting `install all` reboot immediately.

## Applying configuration across the reboot

POAP applies configuration through the reboot rather than on the running system, which has its own quirks the operator's boot script works around:

- Configuration meant to apply after the upgrade is staged via NX-OS `scheduled-config`. Writing `scheduled-config` directly does not reliably survive the POAP reboot; the script instead writes the config to a file on `bootflash:` and copies it into `scheduled-config`, which is the only approach found to work consistently.
- POAP completing without a reboot (via script exit codes, as Cisco's [reference POAP script](https://github.com/CiscoSE/Cisco-POAP/blob/master/poap.py#L3-L9) suggests) could not be made to work and is not documented by Cisco, so the operator's boot script always relies on the reboot to apply the staged configuration.

## Password hash formats

The operator never sends a plaintext admin password to the switch (see [Password handling](provisioning.md#password-handling) for why). The NX-OS provider's `HashProvisioningPassword` hook hashes the password with scrypt (NX-OS type 9) by default, using a 10-byte zero-free random salt and Cisco's custom base64 alphabet, and returns the algorithm label `scrypt`.
