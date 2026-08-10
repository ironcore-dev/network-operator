# Config Backups

`ConfigBackup` defines a configuration backup policy for a `Device`.

The controller supports three backup types:

- **Local** — write timestamped backups to a device-local filesystem path
- **Startup** — persist the running configuration as the device startup configuration
- **Remote** — upload the running configuration to an S3-compatible object store

## Key Behaviors

- Optional cron-based scheduling for recurring backups
- One-shot backups when `spec.schedule` is omitted
- Automatic rotation of old backups (Local and Remote)
- Device storage threshold checks before writing new local backups
- Remote endpoint health check (`RemoteEndpointReady` condition)
- Optional client-side encryption for remote backups (AES-256-GCM or ChaCha20-Poly1305)
- Status reporting for last backup result, next scheduled backup, and discovered backup inventory

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
  path: "bootflash:///backups/" # vendor-specific device path (Cisco NX-OS)
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

## Remote Backup Example

```yaml
apiVersion: networking.metal.ironcore.dev/v1alpha1
kind: ConfigBackup
metadata:
  name: leaf-1-remote
spec:
  deviceRef:
    name: leaf-switch-1
  schedule: "0 */4 * * *"
  type: Remote
  path: "leaf-1/"
  retention:
    keepLast: 10
  s3:
    endpoint: "https://s3.eu-central-1.amazonaws.com"
    bucket: network-config-backups
    region: eu-central-1
    credentialsSecretRef:
      name: s3-backup-credentials
```

The `credentialsSecretRef` must point to a Secret containing `accessKeyID` and `secretAccessKey` keys.

### Encrypted Remote Backup

To enable encryption, add the `encryption` field to the S3 configuration:

```diff
  s3:
    endpoint: "https://s3.eu-central-1.amazonaws.com"
    bucket: network-config-backups
    region: eu-central-1
    credentialsSecretRef:
      name: s3-backup-credentials
+   encryption:
+     algorithm: AES-256-GCM
+     keySecret:
+       name: backup-encryption-key
+       key: encryption-key
```

Supported algorithms:

| Algorithm           | Key Size | Description                                        |
| ------------------- | -------- | -------------------------------------------------- |
| `AES-256-GCM`       | 32 bytes | AES-256 in Galois/Counter Mode                     |
| `ChaCha20-Poly1305` | 32 bytes | ChaCha20 stream cipher with Poly1305 authenticator |

Encryption is performed in the controller pod before upload. The nonce is prepended to the ciphertext. The `status.lastBackup` reports which algorithm and key Secret were used.

## Status Conditions

| Condition             | Description                                        |
| --------------------- | -------------------------------------------------- |
| `Ready`               | Whether the last backup operation succeeded        |
| `RemoteEndpointReady` | Whether the S3 endpoint is reachable (Remote only) |

## Notes

- `Startup` backups always keep a single logical copy.
- Local backup rotation only applies to `type: Local`.
- Remote backup rotation uses S3 ListObjects/DeleteObjects to enforce `retention.keepLast`.
- `spec.path` is the device-local path for Local backups, or the S3 key prefix for Remote backups.
- `storageThreshold` only applies to Local backups (S3 does not expose free-space information).
- The controller watches referenced Secrets and re-reconciles when they are created or updated.
