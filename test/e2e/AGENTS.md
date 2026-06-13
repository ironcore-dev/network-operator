# E2E Tests

Integration tests that validate the full reconciliation pipeline from Kubernetes CRD to gNMI JSON output.

## Rationale

Unit tests verify individual functions but miss the gaps between layers: CRD validation, controller logic, provider field mapping, and gNMI path construction. For example, a controller passing the wrong value to a provider.

Two test modes via Go build tags (`//go:build envtest` vs `//go:build !envtest`):

- **Envtest mode** — runs tests sequantially without infrastructure using envtest + in-process gnmi-test-server. Tests complete in seconds.
- **Cluster mode** — runs tests in parallel against Kind with full operator installation. Validates RBAC, webhooks, metrics.
Each reconciliation test is isolated: unique namespace + dedicated gnmi-test-server pod.

## Commands

```bash
make test-e2e-envtest PROVIDER=cisco-nxos-gnmi    # no cluster required
make test-e2e-kind PROVIDER=cisco-nxos-gnmi       # requires Kind cluster "network"
```

**Provider options:** `openconfig`, `cisco-nxos-gnmi`

## Mode Comparison

| Aspect | Envtest | Cluster |
|--------|---------|---------|
| **K8s API** | In-process (envtest) | Kind cluster |
| **gNMI server** | In-process (gnmi-test-server) | Deployed pod per test |
| **Controllers** | In-process per test | Deployed operator pod |
| **Speed** | ~10s | ~3-4min (parallel) |
| **Dependencies** | None | Docker + Kind |

| Coverage | Envtest | Cluster |
|----------|:-------:|:-------:|
| Controller reconciliation | ✅ | ✅ |
| Status conditions | ✅ | ✅ |
| gNMI payload generation | ✅ | ✅ |
| RBAC / ServiceAccount | ❌ | ✅ |
| Webhook TLS + cert-manager | ❌ | ✅ |
| Container image build | ❌ | ✅ |
| Metrics endpoint | ❌ | ✅ |

## Environment Variables

| Variable | Purpose |
|----------|---------|
| `PROVIDER` / `E2E_PROVIDER` | Filter tests to specific provider |
| `PROMETHEUS_INSTALL_SKIP=true` | Skip Prometheus installation (cluster mode) |
| `CERT_MANAGER_INSTALL_SKIP=true` | Skip CertManager installation (cluster mode) |

## Architecture

Both modes implement `TestEnvironment` interface via `testutil/`:

| Method | Purpose |
|--------|---------|
| `Setup()` / `Teardown()` | Initialize/cleanup K8s + gNMI server |
| `Client()` / `RESTConfig()` | K8s client for resources and managers |
| `GNMIAddress()` | Endpoint for Device CRDs |
| `GetGNMIState()` / `ClearGNMIState()` | Verify and reset gNMI state |
| `PreloadGNMIState()` | Set initial state before reconciliation |
| `IsEnvtest()` | Detect mode for conditional logic |

### GNMI test server

The server accumulates gNMI Set operations and exposes state via `GetState()`.

When configured with `WithNXOSBehavior()`:
- Strips fields with value `"DME_UNSET_PROPERTY_MARKER"` when storing (the marker means "unset this field", not "store this literal string")
- Returns empty TypedValue for non-existent paths (instead of NOT_FOUND error), matching real NX-OS behavior


## Testdata Format

Location: `test/e2e/testdata/<provider>/<feature>.txt` — auto-discovered, txtar format.

Each test file contains K8s YAML resources and expected gNMI JSON state:

```
-- state/preload --              # OPTIONAL: initial gNMI state
{"System": {"procsys-items": {"bootTime": "1700000000"}}}

-- <kind>/<name> --              # Resource to create
apiVersion: networking.metal.ironcore.dev/v1alpha1
kind: Interface
spec:
  deviceRef:
    name: device                 # substituted at runtime
  ...

-- <kind>/<name> --              # Multiple resources supported
apiVersion: networking.metal.ironcore.dev/v1alpha1
kind: BGPPeer
spec:
  deviceRef:
    name: device
  ...

-- state/expect --               # Expected gNMI JSON (array order ignored)
{"System": {"intf-items": ...}}
```

### Test Flow

1. Parse txtar file → extract resources + expected state
2. Create test namespace
3. Deploy/connect to gnmi-test-server
4. Preload gNMI state if `state/preload` section exists
5. Create Device pointing to gnmi-test-server address
6. Apply resources **sequentially**, waiting for `Configured` condition before next
7. Compare final gNMI state vs `state/expect` using semantic JSON comparison

### Important Notes

> **gNMI State Differences:** The JSON stored in gnmi-test-server may differ from actual provider output. For example, NXOS `DME_UNSET_PROPERTY_MARKER` values are filtered out by the test server.

> **Resource Dependencies:** Resources are created in file order. Use ordering to handle dependencies (e.g., BGPPeer after BGP, RoutingPolicy after PrefixSet).

## Existing Test Files

```
test/e2e/testdata/cisco-nxos-gnmi/
├── acl.txt
├── banner.txt
├── bgp_bgppeer.txt
├── dhcprelay.txt
├── dns.txt
├── evpninstance.txt
├── interfaceconfig.txt
├── interfaces.txt
├── isis.txt
├── lldp.txt
├── managementaccess.txt
├── ntp.txt
├── nve.txt
├── ospf.txt
├── pim.txt
├── routedvlan.txt
├── routingpolicy_prefixset.txt
├── snmp.txt
├── subinterface.txt
├── syslog.txt
├── vpcdomain.txt
└── vrf.txt
```
