---
title: Architecture
description: How Network Operator reconciles declarative CRDs into device configurations
gnosis_hash: 605d5949
body_hash: dcea5613
---

# Architecture

Network Operator is a set of Kubernetes controllers that continuously reconcile CRD manifests into running configuration on network devices. If you are familiar with how cert-manager or external-dns work, the model is the same: you describe desired state in YAML, Kubernetes stores it, and a controller loop makes the device match that description.

## The Reconciliation Model

Every configuration resource in network-operator follows the same lifecycle:

1. **You apply a manifest.** For example, an `Interface` or a `BGP` object with the settings you want.
2. **The controller detects the change.** Controllers are built on `controller-runtime` and watch their respective CRD kinds. Any create, update, or delete event triggers a reconcile.
3. **The controller resolves the target device.** Every configuration CRD carries a `deviceRef` field (a `LocalObjectReference`) that names the `Device` object in the same namespace. The controller looks up that `Device` to retrieve the management endpoint and credentials.
4. **The controller builds a platform-native payload.** It translates the abstract spec fields into the API format the device understands — for NX-OS, this is NX-API JSON. Other transports (e.g. gNMI) are planned.
5. **The controller pushes the configuration and updates status.** After the device acknowledges the change, the controller writes the result back to the resource's `.status.conditions`.

The loop is level-triggered, not edge-triggered. If a push fails, the controller re-queues and retries. If someone manually changes the device outside of Kubernetes, the next reconcile cycle detects the drift and corrects it.

## API Layers

The API is structured in four layers, each building on the one below:

| Layer | Purpose | Examples |
|---|---|---|
| **Physical** | Physical inventory — devices, interfaces, links | `Device`, `Interface` |
| **Bricks** | Vendor-abstract configuration, one brick per device | `BGP`, `OSPF`, `VRF`, `VLAN` |
| **Transit** | Translates network demands into brick configs | Routing policies, prefix sets |
| **Intent** | High-level intent — networks, external connections, routing domains | `EVPNInstance`, `NetworkVirtualizationEdge` |

Most day-to-day operator work happens at the Bricks and Intent layers. The Physical layer resources (`Device`) are typically created once during initial setup.

## Core CRDs and Platform-Specific CRDs

Network-operator separates **what you want** from **how a specific platform implements it**.

### Core CRDs

Core CRDs live in the `api/core/v1alpha1` package. They express vendor-neutral intent using fields that map to standard networking concepts. Examples include:

- `Interface` — describes an interface with `name`, `type`, `adminState`, `ipv4`, `mtu`, `switchport`, and references to `vlanRef`, `vrfRef`, and `parentInterfaceRef`.
- `BGP` — describes a BGP router with `asNumber`, `routerId`, `addressFamilies`, and an optional `vrfRef`.
- `VRF` — describes a VRF with `name`, `routeDistinguisher`, and `routeTargets`.
- `VLAN`, `OSPF`, `ISIS`, `PIM`, `EVPNInstance`, `NetworkVirtualizationEdge`, and many others.

### Platform-Specific CRDs

Platform CRDs live in vendor-specific packages (e.g. `api/cisco/nx/v1alpha1`). They provide vendor knobs that have no generic equivalent. Examples:

- `InterfaceConfig` — adds NX-OS-specific settings like `SpanningTree` port type, `BufferBoost`, and LACP `vpcConvergence` options.
- `BGPConfig` — adds NX-OS-specific BGP address family settings such as `advertisePIP` for EVPN and `exportGatewayIP` for symmetric IRB.
- `LLDPConfig` — adds NX-OS-specific `initDelay` and `holdTime` timers.
- `NetworkVirtualizationEdgeConfig` — adds NX-OS NVE options like `advertiseVirtualMAC`, `holdDownTime`, and `infraVLANs`.
- `ManagementAccessConfig` — adds NX-OS console timeout and SSH ACL settings.
- `System` — adds NX-OS system-level settings: `jumboMtu`, `reservedVlan`, `vlanLongName`.
- `VPCDomain`, `BorderGateway` — NX-OS-specific constructs for vPC and EVPN multisite.

### Linking Core to Platform CRDs

Core CRDs carry an optional `providerConfigRef` field of type `TypedLocalObjectReference`. When set, this field points to the corresponding platform-specific resource:

```yaml
# Core CRD
apiVersion: core/v1alpha1
kind: Interface
metadata:
  name: eth1-1
spec:
  deviceRef:
    name: leaf01
  providerConfigRef:
    apiVersion: cisco.nx/v1alpha1
    kind: InterfaceConfig
    name: eth1-1-nxos
  name: Ethernet1/1
  type: Physical
  adminState: Up
```

This decoupling lets you keep environment-independent intent in core resources and vendor-specific tuning in platform resources, rather than embedding NX-OS CLI details directly into the core spec.

## Device Registration and Credentials

Before any configuration resource can be reconciled, a `Device` object must exist in the same namespace.

### The Device Object

`DeviceSpec` holds two mandatory pieces of information:

- **`endpoint.address`** — the management address of the device in `IP:Port` format.
- **`endpoint.secretRef`** — a reference to a Kubernetes Secret of type `kubernetes.io/basic-auth` containing `username` and `password` keys.

Optionally, TLS can be configured via `endpoint.tls`, which accepts a CA certificate (`tls.ca`) and optionally a client certificate and key (`tls.certificate`) for mutual TLS.

```yaml
apiVersion: core/v1alpha1
kind: Device
metadata:
  name: leaf01
spec:
  endpoint:
    address: "192.0.2.10:443"
    secretRef:
      name: leaf01-credentials
      namespace: network
```

The `Device` also supports a `provisioning` field for bootstrap workflows (boot scripts, images), and a `paused` flag to halt all reconciliation activity on the device and all its child resources.

### DeviceRef in Configuration Resources

Every configuration CRD's spec includes a required `deviceRef` field. This is always a `LocalObjectReference` — it names a `Device` in the same namespace. The field is immutable after creation, meaning a configuration resource is permanently bound to one device. To move a config to a different device, you delete and recreate the resource.

### What the Device Status Reports

After connecting to a device, the controller populates `DeviceStatus` with discovered information: `manufacturer`, `model`, `serialNumber`, `firmwareVersion`, `lastRebootTime`, a list of physical `ports`, and a human-readable `portSummary`. The `phase` field reflects the device's current lifecycle state.

## Status Conditions and Finalizers

### Status Conditions

Every CRD — `Device`, `Interface`, `BGP`, `VRF`, etc. — has a `.status.conditions` field that contains a list of `metav1.Condition` objects. Conditions follow the standard Kubernetes convention with `type`, `status` (`True`/`False`/`Unknown`), `reason`, and `message`.

Standard condition types used across resources:

- **`Available`** — the resource is fully functional and the configuration is active on the device.
- **`Progressing`** — the controller is currently applying the configuration.
- **`Degraded`** — the configuration could not be applied or the device is not in the desired state.

Some resources expose richer status beyond conditions. For example:

- `BGPPeer` status includes `sessionState`, `lastEstablishedTime`, and per-address-family prefix counts (`acceptedPrefixes`, `advertisedPrefixes`).
- `OSPF` status includes a `neighbors` list with adjacency states and an `adjacencySummary`.
- `VPCDomain` status includes `role`, `keepaliveStatus`, `peerStatus`, and `peerLinkIfOperStatus`.
- `VLAN` status tracks which interface is providing Layer 3 routing (`routedBy`) and which EVPN instance provides the L2VNI (`bridgedBy`).

To check whether a resource has been successfully applied, inspect the conditions:

```bash
kubectl get interface eth1-1 -o jsonpath='{.status.conditions}'
```

### Finalizers

Finalizers ensure that when you delete a CRD resource, the controller first removes the corresponding configuration from the device before Kubernetes removes the object. Without finalizers, deleting a Kubernetes object would leave orphaned configuration on the device.

The finalizer is added to a resource when the controller first reconciles it. On deletion, Kubernetes sets a deletion timestamp but does not remove the object. The controller sees the deletion timestamp, pushes a removal operation to the device, then removes the finalizer to let Kubernetes complete the deletion.

### Pausing Reconciliation

The `Device` spec includes a `paused` field. Setting it to `true` halts reconciliation for the device and all configuration resources that reference it. This is useful when performing manual maintenance or investigating issues without triggering automated changes.

## Multi-Device and Multi-Vendor Support

### Multi-Device

Each configuration resource is scoped to exactly one device through its immutable `deviceRef`. To configure the same feature on multiple devices, you create one resource per device:

```yaml
# BGP on leaf01
apiVersion: core/v1alpha1
kind: BGP
metadata:
  name: leaf01-bgp
spec:
  deviceRef:
    name: leaf01
  asNumber: 65001
  routerId: 10.0.0.1

---
# BGP on leaf02
apiVersion: core/v1alpha1
kind: BGP
metadata:
  name: leaf02-bgp
spec:
  deviceRef:
    name: leaf02
  asNumber: 65002
  routerId: 10.0.0.2
```

Controllers reconcile all resources concurrently. There is no ordering dependency between resources on different devices unless you express it through cross-references (for example, a `BGPPeer` referencing a `BGP` instance via `bgpRef`).

### Ownership

Child resources are owned by their parent `Device`. This means that when a `Device` is deleted, all configuration resources referencing it are also subject to cleanup through the finalizer mechanism.

### Multi-Vendor

Vendor support is structured through the provider layer. Each vendor implements a provider that understands how to translate core CRD specs into device-native API calls:

- **NX-OS** uses NX-API (HTTP/JSON). This provider is currently implemented.
- **gNMI** is planned as an additional transport.

A different vendor would implement a new provider that consumes the same core CRDs and translates them into its own wire format. Platform-specific CRDs (like `InterfaceConfig` for NX-OS) are vendor-namespaced and linked to core resources via `providerConfigRef`, so adding a new vendor does not require changes to core CRD definitions.

From an operator's perspective, the YAML you write for core resources (`Interface`, `BGP`, `VRF`, etc.) is identical regardless of vendor. Vendor-specific tuning is expressed separately in platform CRDs and linked in by reference.
