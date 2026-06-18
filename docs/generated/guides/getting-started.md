---
title: Getting Started
description: Deploy Network Operator and configure your first network device
gnosis_hash: 4124bd7e
body_hash: e6312e7f
---

# Getting Started

This guide walks you through installing network-operator, registering your first Cisco NX-OS switch, pushing an interface configuration, and verifying that the configuration was applied. By the end, you will have a working foundation to build out BGP, VLANs, and routing policies.

## Prerequisites

Before you begin, ensure the following are available in your environment:

- **Kubernetes cluster** (v1.26 or later recommended) with sufficient permissions to install CRDs and deploy controllers. A single namespace is sufficient for getting started.
- **kubectl** configured to reach the cluster (`kubectl cluster-info` should succeed).
- **Helm 3.10+** for chart installation (`helm version` should succeed).
- **Network reachability** from the Kubernetes worker nodes to the management address of your NX-OS switches. The controller connects to each device over the address specified in the `Device` resource.
- **Device credentials** stored in a Kubernetes Secret of type `kubernetes.io/basic-auth` containing `username` and `password` keys. Create one now:

```bash
kubectl create namespace network-operator
kubectl -n network-operator create secret generic spine-01-creds \
  --type=kubernetes.io/basic-auth \
  --from-literal=username=admin \
  --from-literal=password=<your-password>
```

---

## Installing network-operator via Helm

Add the chart repository and install the operator into the `network-operator` namespace:

```bash
helm repo add network-operator https://charts.example.com/network-operator
helm repo update

helm install network-operator network-operator/network-operator \
  --namespace network-operator \
  --create-namespace \
  --wait
```

Verify that the controller pod is running:

```bash
kubectl -n network-operator get pods
```

You should see output similar to:

```
NAME                                  READY   STATUS    RESTARTS   AGE
network-operator-controller-<hash>    1/1     Running   0          30s
```

The Helm chart installs all CRDs automatically. Confirm they are registered:

```bash
kubectl get crds | grep network-operator
```

---

## Registering a Network Device

Create a `Device` resource to register your NX-OS switch with the operator. The `spec.endpoint.address` field must be the management IP and port of the device. The `spec.endpoint.secretRef` field points to the `kubernetes.io/basic-auth` Secret you created above.

```yaml
# spine-01.yaml
apiVersion: core.network-operator.example.com/v1alpha1
kind: Device
metadata:
  name: spine-01
  namespace: network-operator
spec:
  endpoint:
    address: "10.0.0.1:22"
    secretRef:
      name: spine-01-creds
      namespace: network-operator
```

Apply it:

```bash
kubectl apply -f spine-01.yaml
```

The controller will connect to the device and populate `status` fields including `manufacturer`, `model`, `serialNumber`, `firmwareVersion`, and the list of physical ports. Check the device status:

```bash
kubectl -n network-operator get device spine-01 -o yaml
```

Look for `status.phase` to become `Ready` and inspect the discovered `status.ports`. A healthy device will also have a `status.conditions` entry of type `Ready` with `status: "True"`.

> **Note:** If `status.phase` does not become `Ready` within a few minutes, verify network reachability and check the controller logs:
> ```bash
> kubectl -n network-operator logs -l app=network-operator-controller
> ```

### Pausing reconciliation

If you need to temporarily prevent the operator from pushing changes to a device (for example, during a maintenance window), set `spec.paused: true` on the `Device` resource. The controller will stop reconciling all objects associated with that device until `paused` is removed or set back to `false`.

---

## Applying a Basic Interface Configuration

Once the device is registered, create an `Interface` resource to configure a routed Layer 3 interface. Every interface resource must reference its owning device via the `spec.deviceRef.name` field.

The following example configures `Ethernet1/1` on `spine-01` as a routed Layer 3 interface with an IPv4 address and an MTU of 9216 bytes, which is typical for a data center fabric spine uplink.

```yaml
# spine-01-eth1-1.yaml
apiVersion: core.network-operator.example.com/v1alpha1
kind: Interface
metadata:
  name: spine-01-eth1-1
  namespace: network-operator
spec:
  deviceRef:
    name: spine-01
  name: Ethernet1/1
  type: Ethernet
  adminState: Up
  description: "Uplink to leaf-01 Ethernet1/49"
  mtu: 9216
  ipv4:
    addresses:
      - "192.168.100.0/31"
```

Apply it:

```bash
kubectl apply -f spine-01-eth1-1.yaml
```

### Configuring a loopback interface

Loopback interfaces are commonly used as BGP router IDs and NVE source interfaces. Configure one alongside your Ethernet interface:

```yaml
# spine-01-lo0.yaml
apiVersion: core.network-operator.example.com/v1alpha1
kind: Interface
metadata:
  name: spine-01-lo0
  namespace: network-operator
spec:
  deviceRef:
    name: spine-01
  name: loopback0
  type: Loopback
  adminState: Up
  description: "Router ID / BGP source"
  ipv4:
    addresses:
      - "10.255.0.1/32"
```

```bash
kubectl apply -f spine-01-lo0.yaml
```

---

## Verifying the Configuration Was Pushed to the Device

After applying an `Interface` resource, the controller reconciles the desired state against the device. Check the resource status:

```bash
kubectl -n network-operator get interface spine-01-eth1-1 -o yaml
```

In the output, inspect the `status.conditions` list. A successfully applied configuration will contain a condition of type `Ready` with `status: "True"` and a `reason` indicating the configuration was pushed. For example:

```yaml
status:
  conditions:
    - type: Ready
      status: "True"
      reason: ConfigurationApplied
      lastTransitionTime: "2024-06-01T12:00:00Z"
```

If the condition shows `status: "False"`, the `message` field will describe the error. Common issues include connectivity failures, authentication errors, or unsupported configuration values for the target platform.

You can also list all interface resources and their readiness at a glance:

```bash
kubectl -n network-operator get interfaces
```

To confirm the configuration on the device itself, SSH to the switch and verify:

```bash
# On the NX-OS device:
show interface Ethernet1/1
show ip interface Ethernet1/1
```

The interface should show the configured IP address, MTU, and admin state.

---

## Next Steps

With a device registered and a basic interface configured, you are ready to build out the full data center network fabric. The following CRDs are the logical next steps:

### BGP routing

Use the `BGP` CRD to configure a BGP router instance on the device, specifying `spec.asNumber` and `spec.routerId`. Create `BGPPeer` resources to define eBGP or iBGP neighbors, referencing the BGP instance via `spec.bgpRef.name` and the local source interface via `spec.localAddress.interfaceRef`. For NX-OS-specific address-family settings such as EVPN PIP advertisement, use the `BGPConfig` CRD from the `cisco/nx` API group.

### VLANs

Define VLANs on a device using the `VLAN` CRD, setting `spec.id` (1–4094) and an optional `spec.name`. Trunk or access switchport behavior on physical interfaces is controlled through `spec.switchport` on the `Interface` resource, where `spec.switchport.mode`, `spec.switchport.allowedVlans`, and `spec.switchport.accessVlan` give you full Layer 2 control.

### Routing policies and prefix filtering

Create `PrefixSet` resources to define named lists of IP prefixes with optional mask length ranges. Reference these in `RoutingPolicy` resources using `spec.statements[].conditions.matchPrefixSet.prefixSetRef`. Policies can accept or reject routes and apply BGP community tagging via `spec.statements[].actions.bgpActions`. Attach policies to BGP peers using `spec.addressFamilies.ipv4Unicast.inboundRoutingPolicyRef` and `spec.addressFamilies.ipv4Unicast.outboundRoutingPolicyRef` on the `BGPPeer` resource.

### VXLAN / EVPN overlay

For a VXLAN BGP EVPN fabric, configure a `NetworkVirtualizationEdge` (NVE) resource pointing its `spec.sourceInterfaceRef` to a loopback interface. Then create `EVPNInstance` resources with the appropriate `spec.vni` and `spec.type` (Bridged for L2VNI) to bind VLANs to the overlay. VRF resources with `spec.vni` set enable L3VNI support for inter-tenant routing.

### Device services

Once the core forwarding plane is established, configure operational services such as `NTP`, `DNS`, `Syslog`, and `SNMP` by creating the respective CRDs and referencing the device via `spec.deviceRef.name` in each resource — the same pattern used by all configuration objects in network-operator.
