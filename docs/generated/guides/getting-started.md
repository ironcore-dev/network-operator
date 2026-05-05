---
title: Getting Started
description: Deploy Network Operator and configure your first network device
gnosis_hash: aa516507
body_hash: ff44ad1d
---

# Getting Started

This guide walks you through installing network-operator on a Kubernetes cluster and provisioning your first Cisco NX-OS switch using declarative CRD-based configuration. By the end, you will have a managed device registered, an interface configured, and the configuration verified as applied.

## Prerequisites

Before you begin, ensure the following tools and resources are available.

### Kubernetes Cluster

You need a running Kubernetes cluster (version 1.24 or later is recommended). The cluster must have network reachability to the out-of-band management interfaces of the switches you intend to manage.

Verify your cluster is accessible:

```bash
kubectl cluster-info
```

### kubectl

Install `kubectl` matching your cluster version. Confirm it is working:

```bash
kubectl version --client
```

### Helm

Install Helm v3.10 or later. Confirm the installation:

```bash
helm version
```

## Installing network-operator via Helm

The network-operator is distributed as a Helm chart located at `charts/network-operator` in the project repository.

### Add the Helm repository

If the chart is published to a Helm repository, add it first:

```bash
helm repo add network-operator https://charts.example.com/network-operator
helm repo update
```

If you are working from a local checkout of the repository, you can reference the chart path directly in the steps below.

### Create a namespace

It is recommended to install network-operator into a dedicated namespace:

```bash
kubectl create namespace network-operator
```

### Install the chart

Install the Helm chart with the release name `network-operator`:

```bash
helm install network-operator network-operator/network-operator \
  --namespace network-operator \
  --wait
```

To install from a local chart directory:

```bash
helm install network-operator ./charts/network-operator \
  --namespace network-operator \
  --wait
```

### Verify the installation

Confirm that the controller pods are running:

```bash
kubectl get pods -n network-operator
```

You should see output similar to:

```
NAME                                        READY   STATUS    RESTARTS   AGE
network-operator-controller-7d9f85b-xkp2n  1/1     Running   0          60s
```

The controller manages reconciliation loops for each CRD type and begins watching for resources as soon as it is running.

## Registering a Network Device

A `Device` resource represents a managed network switch. It contains the management address and the credentials needed to connect to the device. All other CRDs reference a `Device` by name through the `deviceRef` field.

### Create a credentials secret

The controller authenticates to the device using a Kubernetes secret of type `kubernetes.io/basic-auth`. Create one for your NX-OS switch:

```bash
kubectl create secret generic nxos-leaf01-creds \
  --type=kubernetes.io/basic-auth \
  --from-literal=username=admin \
  --from-literal=password=<your-password> \
  --namespace network-operator
```

### Apply the Device resource

Create a file named `device-leaf01.yaml` with the following content. The `endpoint.address` field must be in `IP:Port` format, and `endpoint.secretRef.name` must reference the secret created above.

```yaml
apiVersion: core.network-operator.example.com/v1alpha1
kind: Device
metadata:
  name: leaf01
  namespace: network-operator
spec:
  endpoint:
    address: "10.0.0.101:57400"
    secretRef:
      name: nxos-leaf01-creds
```

Apply it:

```bash
kubectl apply -f device-leaf01.yaml
```

### Verify the device is connected

The controller will attempt to connect to the device and populate the `status` fields. Check the device status:

```bash
kubectl get device leaf01 -n network-operator -o yaml
```

Look for the `status.phase` field and the `status.conditions` list. A healthy device will show a phase of `Ready` and a condition with `type: Available` and `status: "True"`. The controller also populates informational fields such as `status.manufacturer`, `status.model`, `status.firmwareVersion`, and `status.serialNumber` when the device is reachable.

## Applying a Basic Interface Configuration

With the device registered, you can now configure its interfaces using the `Interface` CRD. Every `Interface` resource must reference the owning device through `spec.deviceRef.name`.

### Configure a routed Layer 3 interface

The following example configures a physical Ethernet interface on `leaf01` with an IPv4 address. The `spec.type` field identifies the interface type, `spec.name` must match the interface name on the device, and `spec.adminState` controls whether the interface is brought up.

```yaml
apiVersion: core.network-operator.example.com/v1alpha1
kind: Interface
metadata:
  name: leaf01-eth1-1
  namespace: network-operator
spec:
  deviceRef:
    name: leaf01
  name: "Ethernet1/1"
  type: Physical
  adminState: Up
  description: "Uplink to spine01"
  mtu: 9216
  ipv4:
    addresses:
      - "192.168.100.1/31"
```

Apply the resource:

```bash
kubectl apply -f interface-eth1-1.yaml
```

### Configure a loopback interface

Loopback interfaces are commonly used as BGP router IDs and NVE source interfaces in data center fabrics:

```yaml
apiVersion: core.network-operator.example.com/v1alpha1
kind: Interface
metadata:
  name: leaf01-loopback0
  namespace: network-operator
spec:
  deviceRef:
    name: leaf01
  name: "Loopback0"
  type: Loopback
  adminState: Up
  description: "Router ID loopback"
  ipv4:
    addresses:
      - "10.0.255.1/32"
```

Apply it:

```bash
kubectl apply -f interface-loopback0.yaml
```

## Verifying the Configuration Was Pushed to the Device

The network-operator controller reconciles each resource against the actual device state and reports the result through the `status` field of the CRD.

### Check the Interface status

```bash
kubectl get interface leaf01-eth1-1 -n network-operator -o yaml
```

Examine the `status.conditions` list in the output. The controller uses standard condition types:

| Condition type | Meaning |
|---|---|
| `Available` | The configuration has been successfully applied and is active on the device. |
| `Progressing` | The controller is currently applying the configuration. |
| `Degraded` | The controller encountered an error pushing the configuration. |

A successfully applied interface will show a condition similar to:

```yaml
status:
  conditions:
    - type: Available
      status: "True"
      reason: ConfigurationApplied
      lastTransitionTime: "2024-01-15T10:30:00Z"
```

### Use kubectl to list all interface statuses

```bash
kubectl get interfaces -n network-operator
```

### Confirm directly on the device

You can also verify the configuration directly on the NX-OS switch using the NX-OS CLI:

```
leaf01# show interface Ethernet1/1
leaf01# show running-config interface Ethernet1/1
```

The IP address, MTU, description, and administrative state should match what was declared in the `Interface` resource.

### Pause reconciliation for troubleshooting

If you need to temporarily stop the controller from reconciling a device (for example, during a maintenance window), set `spec.paused: true` on the `Device` resource:

```bash
kubectl patch device leaf01 -n network-operator \
  --type=merge -p '{"spec":{"paused":true}}'
```

Remember to set `paused: false` to resume normal reconciliation.

## Next Steps

With a device registered and a basic interface configured, you are ready to build out the rest of your data center network using network-operator's declarative CRDs.

### BGP

Configure BGP routing using the `BGP` and `BGPPeer` CRDs. The `BGP` resource sets the router-level parameters such as `spec.asNumber` and `spec.routerId`, while `BGPPeer` resources define individual neighbor sessions. Each `BGPPeer` references the parent `BGP` instance through `spec.bgpRef.name`. For Cisco NX-OS-specific address-family tuning such as advertising the primary IP (`advertisePIP`) or gateway IP export, use the `BGPConfig` CRD from the `api/cisco/nx/v1alpha1` package.

### VLANs

Define VLANs with the `VLAN` CRD using `spec.id` and `spec.name`. Layer 3 switching for a VLAN is enabled by creating an `Interface` of type `RoutedVLAN` that references the VLAN via `spec.vlanRef.name`.

### Routing Policies

Control route advertisement and filtering using `PrefixSet` and `RoutingPolicy` resources. A `PrefixSet` defines named prefix lists referenced by `RoutingPolicy` statements. Policy statements contain `conditions` (prefix matching) and `actions` (route disposition and BGP attribute manipulation such as community tagging or AS-path prepending). Routing policies are attached to BGP peers via the `spec.addressFamilies.ipv4Unicast.inboundRoutingPolicyRef` and `outboundRoutingPolicyRef` fields on `BGPPeer`.

### VXLAN and EVPN

For VXLAN overlay fabrics, configure `NetworkVirtualizationEdge` (NVE) resources to define the VTEP, `EVPNInstance` resources to define VXLAN Network Identifiers (VNIs), and VRFs with route targets for L3VNI routing. Enable the `l2vpnEvpn` address family on your `BGP` resource to exchange EVPN routes between VTEPs.

### Additional Device Services

network-operator also manages operational services on NX-OS devices including `NTP`, `DNS`, `Syslog`, `SNMP`, `LLDP`, `User`, and `Banner` resources. Each follows the same pattern: create a resource in the same namespace as the target `Device` and reference it via `spec.deviceRef.name`.
