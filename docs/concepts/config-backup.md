# Config Backups

`ConfigBackup` defines an on-device configuration backup policy for a `Device`.

The controller can either:

- write timestamped backups to a device-local filesystem path, or
- persist the running configuration as the device startup configuration

This resource is intended for fast local restore workflows and for auditing recent configuration history directly on the device.

## Key Behaviors

- Optional cron-based scheduling for recurring backups
- One-shot backups when `spec.schedule` is omitted
- Automatic rotation of old local backup files
- Device storage threshold checks before writing new backups
- Status reporting for last backup result, next scheduled backup, and discovered backup inventory
- Best-effort checksum reporting when the implementation can retrieve it

## Local Backup Example

```yaml
apiVersion: networking.metal.ironcore.dev/v1alpha1
kind: ConfigBackup
metadata:
  name: leaf-1-local
spec:
  deviceRef:
    name: leaf-switch-1
  schedule: "0 2 * * *"
  type: Local
  path: "<device-local-path>"
  retention:
    keepLast: 5
  storageThreshold:
    minFreePercent: 10
```

## Startup Backup Example

```yaml
apiVersion: networking.metal.ironcore.dev/v1alpha1
kind: ConfigBackup
metadata:
  name: leaf-1-startup
spec:
  deviceRef:
    name: leaf-switch-1
  type: Startup
```

## Notes

- `Startup` backups always keep a single logical copy.
- Local backup rotation only applies to `type: Local`.
- `spec.path` is interpreted by the backing implementation and may use provider-specific device-local path formats.
- `checksum` and `sizeBytes` are optional status fields and depend on what the implementation can retrieve from the device.
