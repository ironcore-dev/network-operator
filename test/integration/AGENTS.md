# Integration Tests

Tests CRD → gNMI JSON flow using envtest + in-process gNMI server.

## Architecture

```
envtest (in-process K8s API)
    ↓
Real Controllers (PrefixSet, RoutingPolicy, Interface)
    ↓
Provider (NX-OS, IOS-XR, etc.)
    ↓
In-process gNMI Test Server (accumulates state as JSON)
```

## Multi-Provider Testing

Tests run for all providers that have testdata. Each provider gets its own
`Describe` block with isolated controller manager.

```
testdata/
├── nxos/           # Cisco NX-OS provider tests
│   ├── interfaces.txt
│   └── routingpolicy_prefixset.txt
└── iosxr/          # Cisco IOS-XR provider tests (when added)
    └── ...
```

To add tests for a new provider:
1. Create `testdata/<provider>/` directory
2. Add txtar test files with CRD YAML + expected gNMI JSON
3. The provider will be automatically discovered and tested

## Testdata Format (txtar)

```
-- kind/name --
apiVersion: networking.metal.ironcore.dev/v1alpha1
kind: PrefixSet
metadata:
  name: my-prefixset        # K8s object name (can differ from spec.name)
  namespace: default
spec:
  deviceRef:
    name: device            # substituted at runtime with generated device name
  name: MY-PREFIXSET        # Device/gNMI name (used in expected state)
  ...
-- state --
{
  "System": { ... expected gNMI JSON ... }
}
```

- Resources are created in order listed
- `deviceRef.name: device` is auto-replaced with the test's unique device name
- K8s `metadata.name` should differ from `spec.name` to verify correct field usage
- Comment expected state to explain why each gNMI path is present

## Resource Dependencies

Some resources depend on others (e.g., BGPPeer requires BGP, NVE requires Interface).
**Pack dependent resources into a single test file** rather than relying on test ordering.

Examples:
- `bgp_bgppeer.txt` - Interface (loopback) + BGPConfig + BGP + BGPPeer
- `nve.txt` - Interface (loopback) + NVE
- `ospf.txt` - Interface + OSPF
- `evpninstance.txt` - VRF + EVPNInstance

This ensures tests are self-contained and don't depend on alphabetical execution order.

## Adding New Tests

1. Create `testdata/<provider>/<name>.txt` with resource YAML + expected JSON state
2. Tests are auto-discovered from `*.txt` files in the provider directory
3. If new resource type, add GVK to `resourceRegistry` in `main_test.go`

## Key Behaviors

- **Condition check**: Interface uses `ConfiguredCondition`, others use `ReadyCondition`
- **JSON comparison**: Semantic (key order independent)
- **Cleanup**: Finalizers removed in two passes to avoid controller conflicts
- **Sequential creation**: Resources created and waited-for in order (handles dependencies)

## Run

```bash
# Run all provider tests
KUBEBUILDER_ASSETS=$(setup-envtest use 1.35 -p path) go test ./test/integration/...

# Verbose output
KUBEBUILDER_ASSETS=$(setup-envtest use 1.35 -p path) go test ./test/integration/... -v
```
