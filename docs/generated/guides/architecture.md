---
title: Architecture
description: How Network Operator reconciles declarative CRDs into device configurations
gnosis_hash: 0f39c23c
body_hash: e3d86a7b
---

# Architecture

## Overview

Network Operator is a set of Kubernetes controllers that reconcile CRD specifications into live network device configurations. The core idea is simple: you describe the desired state of a network device in a YAML manifest, apply it to Kubernetes, and the operator pushes the corresponding configuration to the device. No scripting, no manual CLI sessions — the operator handles translation and delivery.

The system follows standard controller-runtime patterns: watch CRDs for changes, compare desired state against actual device state, compute a diff, and push updates. This makes it composable with standard Kubernetes tooling — GitOps pipelines, admission webhooks, RBAC, and status monitoring all work as expected.

---

## The Reconciliation Model

When you apply a manifest to Kubernetes, the following sequence takes place:

1. **You apply a CRD manifest.** For example, an `Interface` spec describing a routed interface with an IPv4 address, or a `BGP` spec describing a BGP router instance.

2. **The controller detects the change.** controller-runtime watches the relevant CRD type and enqueues a reconciliation request whenever the object is created, updated, or deleted.

3. **The controller resolves the target Device.** Every configuration CRD carries a `deviceRef` field (of type `LocalObjectReference`) that names the `Device` object in the same namespace. The controller fetches that `Device` to obtain connection details.

4. **The controller builds the platform-native payload.** Using the spec fields, the controller constructs the vendor-specific API call — for example, an NX-API JSON payload for Cisco NX-OS.

5. **The controller pushes the config to the device and updates status.** After a successful push, the controller writes status conditions back to the object. On failure, it sets a `Degraded` condition and requeues for retry.

6. **Finalizers ensure cleanup on deletion.** When you delete a CRD object, the finalizer prevents immediate removal until the controller has removed the corresponding configuration from the device.

A concrete example: applying a `VRF` manifest with `deviceRef.name: leaf-01` causes the VRF controller to look up the `Device` named `leaf-01`, connect to it, and configure the VRF with the specified name, VNI, route distinguisher, and route targets.

---

## API Layers

The API is structured in four conceptual layers, from physical to intent:

| Layer | Description |
|---|---|
| **Physical** | Devices, interfaces, links — the raw hardware representation |
| **Bricks** | Vendor-abstract configuration; one brick maps to one device with status |
| **Transit** | Translates network demands into brick configurations |
| **Intent** | High-level constructs: networks, external connections, routing domains |

Most operators interact with the Physical and Bricks layers directly. The higher layers compose those primitives into fabric-wide constructs.

---

## Core CRDs and Platform-Specific CRDs

### Core CRDs

Core CRDs, defined under `api/core/v1alpha1`, express platform-agnostic intent. They cover a broad range of network constructs:

- **Physical layer:** `Device`, `Interface`, `VLAN`
- **Routing:** `BGP`, `BGPPeer`, `OSPF`, `ISIS`, `PIM`, `VRF`, `RoutingPolicy`, `PrefixSet`
- **Overlay:** `EVPNInstance`, `NetworkVirtualizationEdge`, `DHCPRelay`
- **Management & security:** `NTP`, `DNS`, `Syslog`, `SNMP`, `Banner`, `User`, `Certificate`, `AccessControlList`, `ManagementAccess`
- **Platform features:** `LLDP`, `VPCDomain`, `BorderGateway`, `System`

Each of these types exposes fields that are meaningful across vendors. For example, `BGPSpec` defines `asNumber`, `routerId`, and `addressFamilies` — concepts that exist on every BGP implementation.

Every core config CRD has a `providerConfigRef` field (of type `*TypedLocalObjectReference`) that optionally links to a platform-specific configuration object. If omitted, the provider applies the platform's default settings.

### Platform-Specific CRDs

Platform CRDs, defined under `api/cisco/nx/v1alpha1` (and similar paths for other vendors), carry vendor-specific knobs that have no generic equivalent. Examples include:

- **`InterfaceConfig`** — NX-OS-specific interface settings such as spanning-tree port type, BPDU guard, buffer boost, and LACP vPC convergence options.
- **`LLDPConfig`** — NX-OS LLDP `initDelay` and `holdTime` values.
- **`BGPConfig`** — NX-OS-specific BGP settings such as PIP advertisement for EVPN and gateway IP export for symmetric IRB.
- **`NetworkVirtualizationEdgeConfig`** — NX-OS NVE settings including virtual MAC advertisement and infra-VLAN list.
- **`ManagementAccessConfig`** — NX-OS console timeout and SSH VTY ACL settings.
- **`VPCDomain`** — Cisco vPC domain configuration including peer-link, keepalive, auto-recovery, and role priority.

The relationship is: the core CRD references the platform CRD via `providerConfigRef`. This keeps the core manifest portable while allowing per-platform customisation where needed.

```yaml
# Core CRD — platform-agnostic
apiVersion: network.example.io/v1alpha1
kind: LLDP
spec:
  deviceRef:
    name: leaf-01
  adminState: Up
  providerConfigRef:
    apiVersion: network.example.io/v1alpha1
    kind: LLDPConfig
    name: leaf-01-lldp-config

---
# Platform CRD — NX-OS specific knobs
apiVersion: network.cisco.nx/v1alpha1
kind: LLDPConfig
spec:
  initDelay: 5
  holdTime: 120
```

---

## Device Registration and Credentials

Before any configuration CRD can be reconciled, a `Device` object must exist in the same namespace.

### Device Spec

`DeviceSpec` contains two key sections:

**`endpoint`** (required) — specifies how to reach the device:
- `address`: management address in `IP:Port` format.
- `secretRef`: references a Kubernetes `Secret` of type `kubernetes.io/basic-auth`. The secret must contain `username` and `password` keys.
- `tls`: optional TLS configuration. The `ca` field selects a secret key for the CA certificate. The `certificate` field enables mTLS by referencing a `kubernetes.io/tls` secret containing `tls.crt` and `tls.key`.

**`provisioning`** (optional) — used for zero-touch provisioning. It carries an `image` reference (URL, checksum, checksum type) and a `bootScript` that can be sourced inline, from a `Secret`, or from a `ConfigMap`.

```yaml
apiVersion: network.example.io/v1alpha1
kind: Device
metadata:
  name: leaf-01
spec:
  endpoint:
    address: "192.0.2.10:443"
    secretRef:
      name: leaf-01-credentials
  paused: false
```

The `Device` resource is also where the operator writes back hardware inventory: `DeviceStatus` exposes `manufacturer`, `model`, `serialNumber`, `firmwareVersion`, `lastRebootTime`, and a `ports` list detailing each physical port and any associated `Interface` resource.

All configuration CRDs reference their device by name:

```yaml
spec:
  deviceRef:
    name: leaf-01
```

The `deviceRef` field is immutable — moving a configuration object to a different device requires deleting and recreating it.

### Pausing

`DeviceSpec` includes a `paused` boolean. When set to `true`, the device controller and all controllers managing objects that reference that device halt reconciliation. This is useful during maintenance windows or when you need to apply configuration changes manually without interference.

---

## Status Conditions and Finalizers

### Status Conditions

Every CRD exposes a `status.conditions` field, a list of `metav1.Condition` objects. The operator uses three standard condition types:

| Type | Meaning |
|---|---|
| `Available` | The resource is fully functional and the configuration is applied on the device |
| `Progressing` | The resource is being created or updated |
| `Degraded` | The resource failed to reach or maintain its desired state |

Each condition has a `status` of `True`, `False`, or `Unknown`, along with a `reason` and `message` that give actionable detail.

Some resources expose richer status fields beyond conditions. For example:

- `OSPFStatus` provides `neighbors` (a list of `OSPFNeighbor` with adjacency states) and an `adjacencySummary` string.
- `BGPPeerStatus` provides `sessionState`, `lastEstablishedTime`, and per-address-family `advertisedPrefixes` and `acceptedPrefixes` counts.
- `VPCDomainStatus` reports `role`, `keepaliveStatus`, `peerStatus`, and `peerLinkIfOperStatus`.
- `DeviceStatus` provides a `phase` and full hardware inventory.

These fields let you build monitoring and alerting on top of standard Kubernetes tooling (e.g., Prometheus with `kube-state-metrics`, or `kubectl get` for quick operational checks).

### Finalizers

All configuration CRDs use finalizers to ensure clean removal from the device when you delete the Kubernetes object. The sequence is:

1. You run `kubectl delete`.
2. Kubernetes sets the `deletionTimestamp` but does not remove the object because a finalizer is present.
3. The controller detects the deletion, removes the corresponding configuration from the device, then removes the finalizer.
4. Kubernetes completes the deletion.

This prevents orphaned configuration on devices when Kubernetes objects are removed.

### Ownership

Child resources are owned by their parent `Device`. This means cascading behaviour works as expected: if a `Device` is removed, owned resources are garbage-collected according to Kubernetes owner reference semantics.

---

## Multi-Device and Multi-Vendor Support

### Multi-Device

The operator supports arbitrarily many devices in a single namespace. Each `Device` object represents one physical or virtual network device. Configuration CRDs are scoped to individual devices via `deviceRef` — there is no implicit sharing of configuration across devices.

To apply the same logical configuration to multiple devices (for example, identical BGP settings on a spine tier), you create one CRD instance per device:

```yaml
# Spine 1
apiVersion: network.example.io/v1alpha1
kind: BGP
metadata:
  name: spine-01-bgp
spec:
  deviceRef:
    name: spine-01
  asNumber: 65000
  routerId: "10.0.0.1"

---
# Spine 2
apiVersion: network.example.io/v1alpha1
kind: BGP
metadata:
  name: spine-02-bgp
spec:
  deviceRef:
    name: spine-02
  asNumber: 65000
  routerId: "10.0.0.2"
```

This design keeps each object's lifecycle independent. You can pause, delete, or update configuration on one device without affecting others.

### Multi-Vendor

Multi-vendor support is structured through the provider layer:

- The **core CRDs** define the intent in vendor-neutral terms. Controllers translate these specs into vendor-specific API calls.
- The **platform CRD layer** (e.g., `api/cisco/nx/v1alpha1`) carries vendor-specific extensions, referenced optionally via `providerConfigRef`.
- The **provider layer** implements device communication. Currently, NX-API is used for Cisco NX-OS; gNMI support is planned for additional platforms.

When a controller reconciles a core CRD, it determines the target platform from the `Device` object (the operator discovers the device platform during initial connection). It then selects the appropriate provider and, if a `providerConfigRef` is present on the spec, merges the platform-specific configuration into the payload before pushing it to the device.

This architecture means you can manage heterogeneous fabrics from a single operator instance. Devices running different operating systems co-exist in the same namespace; each controller simply routes to the correct provider implementation based on the resolved `Device`.
