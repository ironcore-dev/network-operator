---
title: Architecture
description: How Network Operator reconciles declarative CRDs into device configurations
gnosis_hash: 76f0627f
body_hash: a46e9e82
---

# Architecture

Network Operator is a set of Kubernetes controllers that translate CRD specifications into live network device configuration. This guide explains how the system is structured, how your YAML manifests become device commands, and how the operator handles multi-device, multi-vendor environments.

## The Reconciliation Model

The core interaction pattern is straightforward: you describe the desired state of a network resource in a CRD manifest, apply it to Kubernetes, and the operator takes responsibility for making the device match that description.

The reconciliation loop works as follows:

1. **You apply a manifest.** For example, you create a `VLAN` resource describing VLAN 100 on a specific device.
2. **The controller detects the change.** Each CRD type has a dedicated controller built on controller-runtime. The controller watches for create, update, and delete events on its resource kind.
3. **The controller resolves the target device.** Every configuration CRD carries a `deviceRef` field (a `LocalObjectReference`) that names the `Device` resource in the same namespace. The controller reads the `Device` to retrieve the management endpoint and credentials.
4. **The controller builds the platform-native payload.** For NX-OS targets this is an NX-API JSON body. The controller translates the abstract CRD fields into the exact structures the device API expects.
5. **The controller pushes the configuration.** The payload is sent to the device's management address defined in `Device.spec.endpoint.address`.
6. **The controller updates status conditions.** After the push, the controller writes the outcome back to `status.conditions` on the resource. If something goes wrong, the condition reflects that; if the device accepted the configuration, the resource transitions to a ready state.

This loop is level-triggered, not event-driven: the controller will re-reconcile on any relevant change and will retry on failure, converging toward the desired state over time.

### Pausing Reconciliation

The `Device` resource exposes a `spec.paused` boolean. When set to `true`, controllers stop processing the device and all resources that reference it. This is useful during maintenance windows or when you need to manually intervene on a device without the operator fighting your changes.

## Core CRDs and Platform-Specific CRDs

The API is split into two layers that work together.

### Core CRDs (Platform-Agnostic Intent)

Core CRDs live under `api/core/v1alpha1` and describe *what* you want without prescribing vendor-specific behavior. Examples include:

- `Interface` — defines interface type, admin state, IP addressing, switchport mode, VRF membership, BFD, and MTU.
- `VLAN` — defines a VLAN ID, name, and admin state.
- `BGP` / `BGPPeer` — defines a BGP instance and its peers, address families, and route policies.
- `VRF` — defines a VRF name, VNI, route distinguisher, and route targets.
- `EVPNInstance` — defines an EVPN instance with VNI, type (bridged or routed), route targets, and VLAN reference.
- `NetworkVirtualizationEdge` — defines the NVE (VTEP) endpoint including source interface, anycast gateway, and host reachability method.

These types are sufficient to express intent for most network configurations. The fields map to concepts that are consistent across vendors.

### Platform-Specific CRDs (Vendor Knobs)

When a vendor exposes controls that have no meaningful cross-platform equivalent, a separate platform config CRD carries those fields. Examples for NX-OS include:

- `BGPConfig` — adds NX-OS-specific BGP options such as `advertisePIP` for EVPN and `exportGatewayIP` for symmetric IRB.
- `LLDPConfig` — adds NX-OS `initDelay` and `holdTime` timers.
- `ManagementAccessConfig` — adds NX-OS VTY console timeout and SSH ACL name.
- `NetworkVirtualizationEdgeConfig` — adds NX-OS NVE options such as `advertiseVirtualMAC`, `holdDownTime`, and `infraVLANs`.
- `InterfaceConfig` — adds NX-OS spanning-tree port type, BPDU guard/filter, buffer boost, and LACP vPC convergence settings.
- `VPCDomain` — configures the Cisco vPC domain including peer-link, keepalive, role priority, and auto-recovery.

### Linking Core and Platform CRDs

The link between a core CRD and its platform-specific extension is the `providerConfigRef` field present on every core resource spec. This is a `TypedLocalObjectReference` that carries the `apiVersion`, `kind`, and `name` of the platform config object:

```yaml
providerConfigRef:
  apiVersion: cisco.nx/v1alpha1
  kind: InterfaceConfig
  name: eth1-0-config
```

When the controller reconciles a core resource, it checks for a `providerConfigRef`. If present, it reads the referenced platform config and merges its vendor-specific fields into the configuration payload before pushing to the device.

## Device Registration and Credentials

Every configuration resource in the operator is scoped to a `Device`. The `Device` CRD is the anchor for all device-level state and connectivity information.

### Defining a Device

A minimal `Device` looks like:

```yaml
apiVersion: core/v1alpha1
kind: Device
metadata:
  name: leaf-01
  namespace: network
spec:
  endpoint:
    address: "192.0.2.10:443"
    secretRef:
      name: leaf-01-credentials
```

The `endpoint.address` is the management IP and port. The `endpoint.secretRef` points to a Kubernetes `Secret` of type `kubernetes.io/basic-auth` containing `username` and `password` keys. The secret is read by the controller at reconciliation time; credentials are never stored in the CRD itself.

### TLS

For gRPC-based transports, `endpoint.tls` carries the CA certificate reference and an optional client certificate for mutual TLS:

```yaml
endpoint:
  tls:
    ca:
      key: ca.crt
    certificate:
      secretRef:
        name: leaf-01-mtls
```

### Device Status

After the operator connects to a device, it populates `Device.status` with discovered information: `manufacturer`, `model`, `serialNumber`, `firmwareVersion`, `lastRebootTime`, and a `ports` list. The `portSummary` field provides a quick human-readable count grouped by speed (e.g., `"2/4 (10g), 4/64 (100g)"`). This information is read-only and reflects what the operator observed from the device.

### Device Provisioning

For zero-touch provisioning, `Device.spec.provisioning` can specify a boot image URL with checksum and a `bootScript` template (inline, from a `Secret`, or from a `ConfigMap`). Provisioning history is tracked in `Device.status.provisioning`.

## Status Conditions and Finalizers

### Status Conditions

Every CRD in the operator exposes a `status.conditions` field — a list of `metav1.Condition` objects. Conditions provide structured, machine-readable state that controllers and external tooling can watch. Standard condition types used across resources include:

- **Available** — the resource is fully functional and the configuration has been successfully applied to the device.
- **Progressing** — the controller is currently creating or updating the resource.
- **Degraded** — the resource failed to reach or maintain its desired state.

Some resources expose additional computed status fields beyond conditions. For example:
- `BGPPeer.status.sessionState` reports the operational BGP session state (e.g., Established).
- `BGPPeer.status.addressFamilies` contains per-AFI/SAFI prefix counts.
- `OSPF.status.neighbors` lists OSPF neighbor adjacency states.
- `VPCDomain.status` reports `role`, `keepaliveStatus`, `peerStatus`, and `peerLinkIfOperStatus`.
- `VLAN.status.routedBy` and `VLAN.status.bridgedBy` reflect cross-resource ownership once an `Interface` or `EVPNInstance` references that VLAN.

### Finalizers

Finalizers ensure that when you delete a CRD resource, the corresponding configuration is removed from the device before Kubernetes removes the object. The controller adds a finalizer to the resource when it first reconciles it. On deletion:

1. Kubernetes marks the object for deletion but does not remove it (the finalizer blocks removal).
2. The controller detects the deletion timestamp, pushes the removal configuration to the device, then removes the finalizer.
3. Kubernetes garbage-collects the object.

This prevents configuration drift where a Kubernetes object is deleted but the device retains stale configuration.

## Multi-Device and Multi-Vendor Support

### Multiple Devices

Each configuration CRD is explicitly bound to one device via `deviceRef`. To configure the same feature across multiple devices, you create one resource per device:

```yaml
# leaf-01 VLAN
apiVersion: core/v1alpha1
kind: VLAN
metadata:
  name: vlan100-leaf01
spec:
  deviceRef:
    name: leaf-01
  id: 100

# leaf-02 VLAN
apiVersion: core/v1alpha1
kind: VLAN
metadata:
  name: vlan100-leaf02
spec:
  deviceRef:
    name: leaf-02
  id: 100
```

The `deviceRef` is immutable after creation. To move a configuration to a different device, you must delete the resource and create a new one targeting the new device.

Child resources are logically owned by their parent `Device`. When a `Device` is deleted, its finalizer ensures cleanup of all device configuration before the object is removed.

### Multiple Vendors

Vendor differences are isolated to the provider layer and the platform-specific CRDs. The core CRD schema remains the same regardless of which vendor's device you are targeting. The controller for each core CRD implements the translation to the appropriate vendor API (NX-API for NX-OS, gNMI planned for other platforms). When you need vendor-specific settings, you attach a platform config via `providerConfigRef`. If no `providerConfigRef` is set, the controller applies the target platform's defaults.

## EVPN/VXLAN Fabric Provisioning: A Concrete Example

EVPN/VXLAN fabric provisioning shows how multiple CRDs compose into a coherent feature. Consider bringing up a new leaf switch as a VXLAN VTEP in a BGP EVPN fabric. The following resources are involved, in dependency order.

### 1. Register the Device

```yaml
apiVersion: core/v1alpha1
kind: Device
metadata:
  name: leaf-01
spec:
  endpoint:
    address: "10.0.0.1:443"
    secretRef:
      name: leaf-01-creds
```

### 2. Configure Underlay Interfaces and Loopbacks

Create `Interface` resources for the physical uplinks (routed, with IPv4 addresses) and the loopback used as the NVE source. The loopback will carry the VTEP IP.

```yaml
apiVersion: core/v1alpha1
kind: Interface
metadata:
  name: lo0-leaf01
spec:
  deviceRef:
    name: leaf-01
  name: loopback0
  type: Loopback
  adminState: Up
  ipv4:
    addresses:
      - 10.0.255.1/32
```

### 3. Configure the VRF (L3VNI)

```yaml
apiVersion: core/v1alpha1
kind: VRF
metadata:
  name: tenant-a-leaf01
spec:
  deviceRef:
    name: leaf-01
  name: TenantA
  vni: 50000
  routeDistinguisher: "65000:50000"
  routeTargets:
    - value: "65000:50000"
      action: Both
      addressFamilies: [L2vpnEvpn]
```

### 4. Configure the VLAN (L2VNI)

```yaml
apiVersion: core/v1alpha1
kind: VLAN
metadata:
  name: vlan100-leaf01
spec:
  deviceRef:
    name: leaf-01
  id: 100
  name: TenantA-Web
  adminState: Active
```

The controller will set `vlan100-leaf01.status.bridgedBy` once an `EVPNInstance` references this VLAN.

### 5. Configure the NVE (VTEP)

`NetworkVirtualizationEdge` is the VTEP endpoint. It references the loopback for the source IP, sets EVPN-based host reachability, and optionally configures an anycast gateway MAC for distributed routing:

```yaml
apiVersion: core/v1alpha1
kind: NetworkVirtualizationEdge
metadata:
  name: nve1-leaf01
spec:
  deviceRef:
    name: leaf-01
  adminState: Up
  sourceInterfaceRef:
    name: lo0-leaf01
  hostReachability: EVPN
  suppressARP: true
  anycastGateway:
    virtualMAC: "00:00:5E:00:01:01"
```

For NX-OS-specific options like `advertiseVirtualMAC` and `infraVLANs`, attach a `NetworkVirtualizationEdgeConfig` via `providerConfigRef`.

### 6. Create the EVPN Instance (L2VNI)

```yaml
apiVersion: core/v1alpha1
kind: EVPNInstance
metadata:
  name: evi-100-leaf01
spec:
  deviceRef:
    name: leaf-01
  vni: 10100
  type: Bridged
  vlanRef:
    name: vlan100-leaf01
  routeDistinguisher: "65000:10100"
  routeTargets:
    - value: "65000:10100"
      action: Both
```

When this resource is reconciled, the controller sets `vlan100-leaf01.status.bridgedBy` to reference `evi-100-leaf01`, establishing the cross-resource link visible in status.

### 7. Configure BGP with EVPN Address Family

```yaml
apiVersion: core/v1alpha1
kind: BGP
metadata:
  name: bgp-leaf01
spec:
  deviceRef:
    name: leaf-01
  asNumber: 65000
  routerId: "10.0.255.1"
  addressFamilies:
    l2vpnEvpn:
      enabled: true
      routeTargetPolicy:
        retainAll: true
```

Then create `BGPPeer` resources for each spine, with the `l2vpnEvpn` address family enabled and `routeReflectorClient: false` on leaf peers.

### 8. Configure the SVI for Routing (RoutedVLAN Interface)

For symmetric IRB, create a `RoutedVLAN` `Interface` that references VLAN 100 and lives in `TenantA` VRF. Enabling `ipv4.anycastGateway: true` on this interface causes the controller to use the virtual MAC defined in the NVE resource:

```yaml
apiVersion: core/v1alpha1
kind: Interface
metadata:
  name: svi100-leaf01
spec:
  deviceRef:
    name: leaf-01
  name: Vlan100
  type: RoutedVLAN
  adminState: Up
  vlanRef:
    name: vlan100-leaf01
  vrfRef:
    name: tenant-a-leaf01
  ipv4:
    addresses:
      - 10.100.0.1/24
    anycastGateway: true
```

The controller sets `vlan100-leaf01.status.routedBy` to reference `svi100-leaf01`.

### What Happens End-to-End

After all of these resources are applied, each controller independently reconciles its piece:

- The `Interface` controller pushes loopback and physical interface configs via NX-API.
- The `VRF` controller creates the VRF with its L3VNI and route targets.
- The `VLAN` controller creates VLAN 100.
- The `NetworkVirtualizationEdge` controller creates the NVE interface.
- The `EVPNInstance` controller creates the MAC-VRF (L2VNI) under the NVE and links it to VLAN 100.
- The `BGP` and `BGPPeer` controllers configure BGP with the EVPN address family and peer sessions.
- The `Interface` controller for the SVI creates the routed VLAN interface with the anycast gateway MAC
