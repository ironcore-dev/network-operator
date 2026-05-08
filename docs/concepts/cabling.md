# Interface Neighbor Validation

The `network-operator` provides a mechanism to verify that *physical* interfaces are connected
to the expected neighbor. Users can annotate/label `Interface` resources with the expected
neighboring interface. This information is then validated via Link-Layer Discovery Protocol
(LLDP).

## Overview

Every time the Interface controller reconciles a Physical `Interface`, it fetches the LLDP
adjacency table from the `Device` owning the `Interface`. The discovered neighbors are
stored in `.status.neighbors`, and each entry is optionally validated against user-provided
expectations expressed through a **label** or an **annotation** on the `Interface` resource.
The controller updates the `Interface` status with the result of the validation.

There are two mechanisms, depending on whether the remote end is managed by the operator.

### Label — the operator manages both interfaces

When both sides of a physical link are managed by the operator (i.e. both have an `Interface`
resource in the same namespace), use the `networking.metal.ironcore.dev/interface-neighbor`
**label**. The label value is the name of the peer `Interface` resource. Because both ends are
known, the controller can cross-validate the device hostname and port name expressed in the
**label** against the hostname and port name in the remote interface just through the
Kubernetes API.

```
         Both devices managed by the operator
         ─────────────────────────────────────

┌──────────────────┐        LLDP        ┌──────────────────┐
│   leaf-01        │◄──────────────────►│   spine-01       │
│   Ethernet1/1    │                    │   Ethernet1/48   │
└──────────────────┘                    └──────────────────┘
        │                                       │
        ▼                                       ▼
  Interface resource                      Interface resource
  leaf-01-eth1-1                          spine-01-eth1-48
  label: ...neighbor=spine-01-eth1-48     label: ...neighbor=leaf-01-eth1-1
```

Following this example, the user can label both interfaces and trigger a validation — assuming
`LLDP` is configured — with:
```bash
kubectl label interface leaf-01-eth1-1 networking.metal.ironcore.dev/interface-neighbor=spine-01-eth1-48
kubectl label interface spine-01-eth1-48 networking.metal.ironcore.dev/interface-neighbor=leaf-01-eth1-1
```

The controller resolves the referenced `Interface`, looks up the owning `Device`'s hostname, and
compares it against the LLDP `systemName`. It then compares the referenced interface name against
the LLDP `portID`, accounting for vendor-specific naming conventions (e.g. NX-OS `eth1/1` vs. `Ethernet1/1`).

### Annotation — the operator manages only one of the interfaces

When the remote end is **not** managed by the operator (e.g. a compute node that does not
belong to the fabric), there is no `Interface` resource to reference. In this case, use the
`networking.metal.ironcore.dev/interface-neighbor-raw` **annotation**. The value is a raw
identifier in the format `<deviceIdentifier>::<portID>`, where `deviceIdentifier` can be
either the LLDP Chassis ID (e.g. a MAC address) or the LLDP System Name.

```
         Remote device not managed by the operator
         ──────────────────────────────────────────

┌──────────────────┐        LLDP        ┌──────────────────┐
│   leaf-01        │◄──────────────────►│   server-01      │
│   Ethernet1/2    │                    │   Ethernet1/48   │
└──────────────────┘                    └──────────────────┘
        │                                       │
        ▼                                       ✗ no resource
  Interface resource
  leaf-01-eth1-2
  annotation: ...neighbor-raw=server-01::Ethernet1/48
```

The user can then annotate the managed `Interface` with:
```bash
kubectl annotate interface leaf-01-eth1-2 networking.metal.ironcore.dev/interface-neighbor-raw="server-01::Ethernet1/48"
```

The controller checks the annotation value against the `chassisID` and `systemName`
fields of the LLDP adjacency, as well as the interface name.

## `Interface` status: adjacencies and validation

When LLDP adjacencies are present, they appear in `.status.neighbors`:

```yaml
status:
  neighbors:
    - chassisId: "00:1a:2b:3c:4d:5e"
      chassisIdType: MACAddress
      portId: "Ethernet1/48"
      portIdType: InterfaceName
      systemName: spine-01
      systemDescription: "Cisco Nexus Operating System (NX-OS) Software"
      portDescription: "Ethernet1/48"
      expirationTime: "2026-05-08T12:00:00Z"
      validation: Verified
```

Each neighbor entry includes LLDP TLV data (chassis ID, port ID, system name, etc.) and an
optional `validation` result. The possible validation outcomes are:

| Value             | Meaning |
|-------------------|---------|
| `Verified`        | The LLDP neighbor matches the expected peer. |
| `DeviceMismatch`  | The neighbor's system name / chassis ID does not match the expected device. |
| `PortMismatch`    | The device matches, but the port does not. |
| `NotFound`        | The `Interface` resource referenced by the label does not exist. |
| _(empty)_         | No label or annotation is set — validation was not performed. |

For the full field definitions, see [Neighbor](../api-reference/index.md#neighbor) and
[NeighborValidation](../api-reference/index.md#neighborvalidation) in the API reference.

## Design Notes

### TTL and expiration

Rather than storing the raw TTL for each LLDP entry, the controller computes and exposes an
`expirationTime` field (`now() + TTL`). The operator does not actively track these expiration
times. Instead, it relies on:

1. The device removing expired neighbors from its LLDP table.
2. The controller's periodic requeue interval to sync the status.

This means that `Neighbor` entries may become temporarily stale if the interface is not
requeued in time — the status reflects the last fetched state, not real-time data. The user
should pay attention to the `ExpirationTime` to identify these occurrences.

### Relationship to the `LLDP` resource

This design intentionally deviates from the OpenConfig model, where all LLDP information
lives under the `lldp` subtree. In the network-operator, the `LLDP` resource controls
**configuration** (enabling/disabling LLDP on interfaces), while adjacency data is retrieved
by the `Interface` controller and stored in the `Interface` status. This avoids a runtime
dependency between the two controllers. If LLDP is enabled by means other than the operator,
adjacency data will still appear in the `Interface` status.

### Provider support

LLDP adjacency retrieval is performed during `GetInterfaceStatus` and is currently limited
to Physical interfaces. Each provider implements `InterfaceNameEqual` to handle
vendor-specific interface naming conventions when comparing the expected port against the
LLDP port ID.
