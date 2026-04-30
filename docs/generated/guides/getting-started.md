---
title: Getting Started
description: Deploy Network Operator and configure your first network device
gnosis_hash: 4357f246
body_hash: a7ac0394
---

# Getting Started

This guide walks a data center operator through installing network-operator, registering a Cisco NX-OS switch, pushing an initial interface configuration, and verifying the result. Subsequent sections point toward more advanced topics.

---

## Prerequisites

Before you begin, ensure the following are available:

- **Kubernetes cluster** (v1.26 or later recommended) with sufficient RBAC permissions to install CRDs and create namespaces.
- **kubectl** configured to talk to that cluster (`kubectl version` should succeed against the target API server).
- **Helm 3** installed locally (`helm version`).
- **Network reachability** from the cluster nodes (or from a dedicated egress point) to the management address of each NX-OS switch. network-operator connects to devices over gRPC/gNMI; ensure TCP port 50051 (or your device's configured gRPC port) is open between the cluster and the management network.
- **Device credentials** — a username and password that have the `network-admin` role on NX-OS, and optionally a CA certificate if the device uses TLS on its gRPC server.

---

## Installing network-operator via Helm

The Helm chart lives under `deploy/helm/network-operator` in the repository.

### 1. Add or clone the chart

```bash
# Clone the repository and reference the chart locally
git clone https://github.com/your-org/network-operator.git
cd network-operator
```

### 2. Create a dedicated namespace

```bash
kubectl create namespace network-operator
```

### 3. Install the chart

```bash
helm install network-operator deploy/helm/network-operator \
  --namespace network-operator \
  --set controller.replicaCount=2
```

Verify the controller pod reaches `Running`:

```bash
kubectl get pods -n network-operator
```

The controller manages a reconciliation loop for every CRD type. Once running, it watches for `Device`, `Interface`, `BGP`, `VLAN`, and all other resources in any namespace and pushes the desired state to the corresponding device.

---

## Registering a Network Device

Every managed resource references a `Device` object. The `Device` CRD (API group `core/v1alpha1`) holds the management address and authentication credentials.

### 1. Store device credentials in a Secret

The `DeviceSpec.endpoint.secretRef` field requires a secret of type `kubernetes.io/basic-auth` containing `username` and `password` keys.

```bash
kubectl create secret generic leaf01-credentials \
  --namespace network-operator \
  --type kubernetes.io/basic-auth \
  --from-literal=username=admin \
  --from-literal=password=<REDACTED>
```

### 2. Create the Device resource

```yaml
# leaf01-device.yaml
apiVersion: core.network-operator.io/v1alpha1
kind: Device
metadata:
  name: leaf01
  namespace: network-operator
spec:
  endpoint:
    address: "192.0.2.10:50051"   # NX-OS management IP and gRPC port
    secretRef:
      name: leaf01-credentials
      namespace: network-operator
```

Apply it:

```bash
kubectl apply -f leaf01-device.yaml
```

After a few seconds the controller discovers the device and populates `status`:

```bash
kubectl get device leaf01 -n network-operator -o yaml
```

Look for `status.phase` moving to `Ready`, and check `status.manufacturer`, `status.model`, and `status.firmwareVersion` to confirm the operator has successfully connected.

> **TLS:** If your NX-OS switch is configured with a self-signed or enterprise CA certificate on the gRPC server, add a `spec.endpoint.tls` block pointing to a Secret that contains the CA certificate under `spec.endpoint.tls.ca`.

---

## Applying a Basic Interface Configuration

The `Interface` CRD models ethernet, loopback, port-channel, routed-VLAN, and subinterfaces. Every `Interface` resource must reference the owning `Device` via `spec.deviceRef`.

### Example: configure a routed Layer 3 interface

The following manifest configures `Ethernet1/1` on `leaf01` as a routed interface with a /30 address, an MTU of 9216 bytes, and a description.

```yaml
# leaf01-eth1-1.yaml
apiVersion: core.network-operator.io/v1alpha1
kind: Interface
metadata:
  name: leaf01-eth1-1
  namespace: network-operator
spec:
  deviceRef:
    name: leaf01
  name: Ethernet1/1
  type: Ethernet
  adminState: Up
  description: "Uplink to spine01"
  mtu: 9216
  ipv4:
    addresses:
      - 10.0.0.1/30
```

Apply it:

```bash
kubectl apply -f leaf01-eth1-1.yaml
```

### Example: configure a loopback interface

Loopback interfaces are commonly used as BGP router-IDs and NVE source interfaces.

```yaml
# leaf01-lo0.yaml
apiVersion: core.network-operator.io/v1alpha1
kind: Interface
metadata:
  name: leaf01-lo0
  namespace: network-operator
spec:
  deviceRef:
    name: leaf01
  name: loopback0
  type: Loopback
  adminState: Up
  description: "Router-ID loopback"
  ipv4:
    addresses:
      - 10.255.0.1/32
```

```bash
kubectl apply -f leaf01-lo0.yaml
```

### Key `InterfaceSpec` fields reference

| Field | Purpose |
|---|---|
| `deviceRef.name` | Name of the `Device` object in the same namespace |
| `name` | Interface name exactly as it appears on the device (e.g., `Ethernet1/1`, `loopback0`) |
| `type` | `Ethernet`, `Loopback`, `Aggregate`, `RoutedVLAN`, `Subinterface` |
| `adminState` | `Up` or `Down` |
| `mtu` | Packet MTU in bytes |
| `ipv4.addresses` | List of IPv4 CIDR prefixes; first entry is the primary address |
| `switchport` | Layer 2 switchport configuration (access or trunk mode) |
| `vrfRef` | Assigns the interface to a non-default VRF |

---

## Verifying the Configuration Was Pushed to the Device

The operator follows a reconcile-then-report model: after applying a manifest it attempts to push the desired state to the device and then reflects the outcome in the resource's `status.conditions`.

### Check conditions on the Interface resource

```bash
kubectl get interface leaf01-eth1-1 -n network-operator -o yaml
```

A successful push produces a condition similar to:

```yaml
status:
  conditions:
    - type: Available
      status: "True"
      reason: ConfigurationApplied
      message: "Interface configuration successfully applied to device"
      lastTransitionTime: "2024-11-01T10:15:00Z"
```

If the push fails the `status` field is `"False"` and `message` contains the error returned by the device.

### Useful one-liners

```bash
# Watch all Interface resources in the namespace
kubectl get interfaces -n network-operator -w

# Check conditions across all managed resources for leaf01
kubectl get interfaces,bgp,vlan,vrf -n network-operator \
  -l core.network-operator.io/device=leaf01

# Describe a specific resource for full event and condition history
kubectl describe interface leaf01-eth1-1 -n network-operator
```

### Confirm on the device directly

SSH to `leaf01` and verify the configuration was applied:

```
leaf01# show running-config interface Ethernet1/1
interface Ethernet1/1
  description Uplink to spine01
  mtu 9216
  ip address 10.0.0.1/30
  no shutdown
```

---

## Next Steps

With a device registered and a first interface configured, you are ready to build out the rest of the fabric. The sections below highlight the most commonly used CRDs for a NX-OS data center deployment.

### BGP and BGP Peers

Use the `BGP` CRD to configure an eBGP or iBGP instance on `leaf01`, setting `spec.asNumber` and `spec.routerId`. Add neighbors with individual `BGPPeer` resources, each referencing the parent BGP instance via `spec.bgpRef`. For address-family configuration specific to NX-OS (such as EVPN PIP advertisement), attach a `BGPConfig` resource using `spec.providerConfigRef`.

### VLANs

Create `VLAN` resources with `spec.id` (1–4094), `spec.name`, and `spec.deviceRef`. For layer 3 routing over a VLAN, add a `RoutedVLAN` type `Interface` that references the VLAN via `spec.vlanRef`.

### VRFs

Use the `VRF` CRD to define tenant VRFs (`spec.name`, `spec.routeDistinguisher`, `spec.routeTargets`). Assign interfaces to VRFs via `spec.vrfRef` on the `Interface` resource.

### Routing Policies and Prefix Sets

Define match criteria with `PrefixSet` resources, then reference them from `RoutingPolicy` statements (`spec.statements[].conditions.matchPrefixSet`). Attach policies to BGP peers via `spec.addressFamilies.ipv4Unicast.inboundRoutingPolicyRef` or `outboundRoutingPolicyRef` on `BGPPeer`.

### VXLAN / EVPN Overlay

Create a `NetworkVirtualizationEdge` (NVE) resource with `spec.sourceInterfaceRef` pointing to a loopback, configure `EVPNInstance` resources for each L2VNI or L3VNI, and enable the L2VPN EVPN address family in BGP. For NX-OS-specific NVE settings (hold-down time, infra VLANs), attach a `NetworkVirtualizationEdgeConfig` via `spec.providerConfigRef`.

### Device Services

| CRD | Purpose |
|---|---|
| `NTP` | NTP server configuration and source interface |
| `DNS` | DNS servers, default domain, and source interface |
| `Syslog` | Remote syslog servers and facility configuration |
| `SNMP` | SNMP communities, trap destinations, and notification types |
| `LLDP` | System-wide and per-interface LLDP control |

Each service CRD follows the same pattern: set `spec.deviceRef.name` to the target device, fill in the relevant fields, and the controller reconciles the change onto the device.
