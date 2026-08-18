# Getting Started

This page covers installing the Network Operator into a Kubernetes cluster using the published Helm chart.

## Prerequisites

- A Kubernetes cluster (v1.28+)
- [Helm](https://helm.sh/) v3.12+
- [cert-manager](https://cert-manager.io/) installed in the cluster (required for webhook and metrics TLS certificates). If not yet installed, follow the [cert-manager installation guide](https://cert-manager.io/docs/installation/).
- Network connectivity from the cluster to your managed devices' management interfaces

## Installation

### 1. Install with Helm OCI

The chart is published as an OCI artifact at `ghcr.io/ironcore-dev/charts/network-operator`.

```bash
helm install network-operator \
  oci://ghcr.io/ironcore-dev/charts/network-operator \
  --namespace network-operator \
  --create-namespace
```

Available versions can be found on the [GitHub packages page](https://github.com/ironcore-dev/network-operator/pkgs/container/charts%2Fnetwork-operator).

### 2. Verify the installation

```bash
kubectl get pods -n network-operator
kubectl get crds | grep networking.metal.ironcore.dev
```

You should see the controller-manager pod running and CRDs like `devices.networking.metal.ironcore.dev`, `interfaces.networking.metal.ironcore.dev`, etc.

## Configuration

The Helm chart is configured through `values.yaml` overrides. See the [`charts/network-operator/values.yaml`](https://github.com/ironcore-dev/network-operator/blob/main/charts/network-operator/values.yaml) file for all available options and their defaults.

### Provider Selection

Pass the `--provider` flag via `manager.args`. Available providers:

| Provider             | Flag value         |
| -------------------- | ------------------ |
| OpenConfig (default) | `openconfig`       |
| Cisco NX-OS (gNMI)   | `cisco-nxos-gnmi`  |
| Cisco IOS-XR (gNMI)  | `cisco-iosxr-gnmi` |

```yaml
manager:
  args:
    - --leader-elect
    - --provider=cisco-nxos-gnmi
```

### Provisioning Server

The operator embeds an HTTP server for ZTP provisioning (port 8080) and a TFTP server (port 1069). These are exposed as container ports automatically.

To expose the provisioning endpoints outside the cluster, create a `Service` of type `LoadBalancer` or `NodePort` targeting ports 8080 (TCP) and 1069 (UDP).

## Upgrading

Upgrade to a newer version using `helm upgrade` with the desired `--version` flag. CRDs are updated automatically with the chart.

## Uninstalling

CRDs persist after a `helm uninstall` by default (`crd.keep: true`). To remove them manually:

```bash
kubectl get crds -o name | grep networking.metal.ironcore.dev | xargs kubectl delete
```

## Next Steps

- [Device Onboarding](../tutorials/device-onboarding.md) — create your first Device and walk through ZTP provisioning and day-2 configuration.
- [EVPN-VXLAN Fabric Tutorial](../tutorials/evpn-vxlan-fabric.md) — build a full EVPN-VXLAN fabric from scratch.
