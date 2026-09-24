# Bulk Brick Lifecycle Lab Runbook

This runbook describes how to manually run the bulk brick lifecycle test against
real or virtual Cisco NX-OS devices in the ORA showroom clabernetes cluster.
It is intended for evidence-driven validation: every Kubernetes result must be
confirmed from the device CLI.

## Goal

Validate that bulk-created bricks converge on a multi-device fleet, and that
bulk deletion leaves no device configuration behind. Pay special attention to
NX-OS BGP cleanup:

- The default BGP instance must disappear after the last managed BGP brick is
  deleted.
- Deleting one VRF-scoped BGP brick must remove only that VRF's BGP domain while
  leaving other VRF BGP domains untouched.
- Internal ownership marker peer templates named
  `__operator-managed--<vrf-name>__` must not be left behind.

## Prerequisites

- Access to the ORA showroom Kubernetes cluster that runs clabernetes.
- `kubectl` configured with the `clabernetes` context from the ORA dashboard
  Setup flow.
- `clabverter` installed and available in `PATH`.
- `gnmic` installed for out-of-band gNMI checks.
- `helm` installed if deploying the operator from the chart.
- A network-operator image that contains the code under test.

Verify local tooling:

```sh
kubectl config get-contexts
command -v clabverter
command -v gnmic
command -v helm
```

Use an explicit context for all cluster commands:

```sh
export KCTX=clabernetes
export LAB_NS=lab--i000000-bulk-brick
export TEST_NS=network-operator-bulk-brick
```

## 1. Create The clabernetes Lab

Use the existing EVPN/VXLAN Containerlab topology as the fleet baseline. The
topology contains two spines, three leaves, and two Linux hosts. By default it
uses the existing Cisco NX-OS image from the example:

```yaml
topology:
  kinds:
    cisco_n9kv:
      image: ${IMAGE:=vrnetlab/cisco_n9kv:9300-10.4.6}
```

Only override `IMAGE` if the lab owner explicitly asks for a different NX-OS
image.

In ORA showroom, the public `vrnetlab/cisco_n9kv:9300-10.4.6` image may not be
pullable by the clabernetes workers. If the NX-OS pods fail with `pull access
denied for vrnetlab/cisco_n9kv`, keep the repository topology unchanged and use
a temporary copy with an ORA-internal image that is already present in existing
labs:

```sh
mkdir -p /tmp/network-operator-bulk-brick/clab
perl -pe 's#image: \$\{IMAGE:=vrnetlab/cisco_n9kv:9300-10\.4\.6\}#image: keppel.eu-de-1.cloud.sap/ccloud/containerlab/cisco_n9kv:9300-10.4.3#' \
  examples/cisco-n9k-evpn-vxlan/topology.clab.yml \
  > /tmp/network-operator-bulk-brick/clab/topology.clab.yml
cd /tmp/network-operator-bulk-brick/clab
```

Deploy the lab:

```sh
kubectl --context "$KCTX" create namespace "$LAB_NS"
mkdir -p /tmp/network-operator-bulk-brick/clab
cp examples/cisco-n9k-evpn-vxlan/topology.clab.yml /tmp/network-operator-bulk-brick/clab/topology.clab.yml
cd /tmp/network-operator-bulk-brick/clab
clabverter --debug --destinationNamespace "$LAB_NS" --stdout | kubectl --context "$KCTX" apply -f -
kubectl --context "$KCTX" -n "$LAB_NS" get pods,svc -w
```

NX-OS images can take about 10 minutes to become usable. Continue only when all
switch services exist and SSH/gNMI respond.

## 2. Confirm Device Access

Find service names and ports:

```sh
kubectl --context "$KCTX" -n "$LAB_NS" get svc
```

For human CLI evidence, port-forward one device at a time, or use one terminal
per device with unique local ports:

```sh
kubectl --context "$KCTX" -n "$LAB_NS" port-forward svc/evpn-vxlan-fabric-leaf1 8022:22 9339:9339 8443:443
ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null admin@localhost -p 8022
gnmic -a localhost:9339 -u admin -p admin --skip-verify get --path 'System/bgp-items'
```

For the operator, do not use `localhost` unless the controller also runs on the
same machine. When the operator runs inside the cluster, configure each `Device`
with the clabernetes service DNS name:

```yaml
spec:
  endpoint:
    address: evpn-vxlan-fabric-leaf1.<lab-namespace>.svc.cluster.local:9339
    secretRef:
      name: fabric-credentials
```

## 3. Deploy Network Operator With NX-OS Provider

The controller must run with the Cisco NX-OS provider:

```sh
helm upgrade --install network-operator ./charts/network-operator \
  --kube-context "$KCTX" \
  --namespace network-operator \
  --create-namespace \
  --set manager.args='{--leader-elect,--provider=cisco-nxos-gnmi}'
```

If testing local changes, build and push an image first, then override the chart
image repository and tag:

```sh
make docker-build IMG=<registry>/network-operator:<tag>
docker push <registry>/network-operator:<tag>
helm upgrade --install network-operator ./charts/network-operator \
  --kube-context "$KCTX" \
  --namespace network-operator \
  --create-namespace \
  --set manager.image.repository=<registry>/network-operator \
  --set manager.image.tag=<tag> \
  --set manager.args='{--leader-elect,--provider=cisco-nxos-gnmi}'
```

Check readiness:

```sh
kubectl --context "$KCTX" -n network-operator rollout status deploy/network-operator-controller-manager
kubectl --context "$KCTX" get crds | grep networking.metal.ironcore.dev
```

## 4. Prepare Test Manifests

Copy the EVPN/VXLAN Kubernetes manifests to a scratch directory and patch the
five `Device` endpoints to the clabernetes service DNS names. Keep credentials
as `admin/admin` unless the image was customized.

```sh
mkdir -p /tmp/network-operator-bulk-brick
cp -R examples/cisco-n9k-evpn-vxlan/kubernetes /tmp/network-operator-bulk-brick/kubernetes
```

Patch these files in the scratch copy:

- `01-devices/leaf1.yaml`
- `01-devices/leaf2.yaml`
- `01-devices/leaf3.yaml`
- `01-devices/spine1.yaml`
- `01-devices/spine2.yaml`

Use addresses like:

```text
evpn-vxlan-fabric-leaf1.<lab-namespace>.svc.cluster.local:9339
evpn-vxlan-fabric-leaf2.<lab-namespace>.svc.cluster.local:9339
evpn-vxlan-fabric-leaf3.<lab-namespace>.svc.cluster.local:9339
evpn-vxlan-fabric-spine1.<lab-namespace>.svc.cluster.local:9339
evpn-vxlan-fabric-spine2.<lab-namespace>.svc.cluster.local:9339
```

Create the namespace used by the test CRs:

```sh
kubectl --context "$KCTX" create namespace "$TEST_NS"
```

## 5. Evidence Commands

Run these commands before apply, after apply, after partial deletion, and after
bulk deletion. Save the output in the tracking issue.

Kubernetes state:

```sh
kubectl --context "$KCTX" -n "$TEST_NS" get devices,interfaces,vrfs,bgp,bgppeers,vlans,networkvirtualizationedges,ospf,pim -o wide
kubectl --context "$KCTX" -n "$TEST_NS" get events --sort-by=.lastTimestamp
kubectl --context "$KCTX" -n network-operator logs deploy/network-operator-controller-manager --since=30m
```

NX-OS CLI state for each switch:

```text
show running-config bgp
show running-config bgp | include __operator-managed
show bgp sessions
show vrf
show running-config interface
show vlan brief
show nve vni
```

The exact command set can be narrowed per test case, but BGP cleanup evidence
must always include both `show running-config bgp` and the marker check.

## 6. Test Case 1: Bulk Create

Start from a clean device state. If the lab was reused, run the bulk delete case
first and confirm no residual config remains.

Apply the full fleet configuration:

```sh
kubectl --context "$KCTX" -n "$TEST_NS" apply -k /tmp/network-operator-bulk-brick/kubernetes
kubectl --context "$KCTX" -n "$TEST_NS" wait --for=condition=Ready devices --all --timeout=10m
kubectl --context "$KCTX" -n "$TEST_NS" wait --for=condition=Ready interfaces --all --timeout=10m
kubectl --context "$KCTX" -n "$TEST_NS" wait --for=condition=Ready vrfs --all --timeout=10m
kubectl --context "$KCTX" -n "$TEST_NS" wait --for=condition=Ready bgp --all --timeout=10m
kubectl --context "$KCTX" -n "$TEST_NS" wait --for=condition=Ready bgppeers --all --timeout=10m
```

Record PASS only if:

- All CRs report `Ready=True`.
- Every expected interface, VRF, VLAN, NVE, OSPF, BGP, and BGP peer is visible
  in the device CLI.
- No unexpected operator errors appear in logs.

## 7. Test Case 2: Bulk Delete

Delete all bricks and wait for finalizers to finish:

Copy the manifest tree and remove `01-devices` from the copied
`kustomization.yaml`. The bulk delete case validates brick cleanup while keeping
the devices registered and reachable for CLI checks.

```sh
cp -R /tmp/network-operator-bulk-brick/kubernetes /tmp/network-operator-bulk-brick/kubernetes-bricks-only
$EDITOR /tmp/network-operator-bulk-brick/kubernetes-bricks-only/kustomization.yaml
kubectl --context "$KCTX" -n "$TEST_NS" delete -k /tmp/network-operator-bulk-brick/kubernetes-bricks-only --ignore-not-found
kubectl --context "$KCTX" -n "$TEST_NS" wait --for=delete interfaces --all --timeout=10m
kubectl --context "$KCTX" -n "$TEST_NS" wait --for=delete vrfs --all --timeout=10m
kubectl --context "$KCTX" -n "$TEST_NS" wait --for=delete bgp --all --timeout=10m
kubectl --context "$KCTX" -n "$TEST_NS" wait --for=delete bgppeers --all --timeout=10m
kubectl --context "$KCTX" -n "$TEST_NS" wait --for=delete vlans --all --timeout=10m
kubectl --context "$KCTX" -n "$TEST_NS" wait --for=delete networkvirtualizationedges --all --timeout=10m
```

Record PASS only if device CLI shows no operator-created residual config. For
NX-OS BGP, `show running-config bgp` must not contain the deleted domains,
neighbors, address families, or `__operator-managed--` markers.

## 8. Test Case 3: Delete Device With Dependents

Re-apply at least the resources targeting `leaf1`, then delete the device with
foreground cascading deletion:

```sh
kubectl --context "$KCTX" -n "$TEST_NS" apply -k /tmp/network-operator-bulk-brick/kubernetes
kubectl --context "$KCTX" -n "$TEST_NS" wait --for=condition=Ready devices/leaf1 --timeout=10m
kubectl --context "$KCTX" -n "$TEST_NS" delete device leaf1 --cascade=foreground
kubectl --context "$KCTX" -n "$TEST_NS" wait --for=delete device/leaf1 --timeout=10m
kubectl --context "$KCTX" -n "$TEST_NS" get bgp,bgppeers,vrfs,interfaces,vlans,networkvirtualizationedges,ospf,pim -l networking.metal.ironcore.dev/device=leaf1
```

Record PASS only if all leaf1-owned bricks are deleted and leaf1's CLI has no
remaining operator-created config. Other devices must remain configured.

## 9. Test Case 4: NX-OS Default BGP Instance Removal

Run this on a clean leaf or after bulk delete. Apply only a `Device` and one
default-scoped `BGP` brick:

```yaml
apiVersion: networking.metal.ironcore.dev/v1alpha1
kind: BGP
metadata:
  name: leaf1-default-bgp
spec:
  deviceRef:
    name: leaf1
  asNumber: 65000
  routerId: 10.255.0.1
  addressFamilies:
    ipv4Unicast:
      enabled: true
```

Verify from CLI:

```text
show running-config bgp
show running-config bgp | include __operator-managed--default__
```

Delete the BGP CR and wait for deletion:

```sh
kubectl --context "$KCTX" -n "$TEST_NS" delete bgp leaf1-default-bgp
kubectl --context "$KCTX" -n "$TEST_NS" wait --for=delete bgp/leaf1-default-bgp --timeout=10m
```

Record PASS only if the global `router bgp` configuration and
`__operator-managed--default__` marker are gone from the CLI.

## 10. Test Case 5: NX-OS Per-VRF BGP Removal

Run this on a clean leaf or after the default BGP cleanup test. Apply two VRFs
and two VRF-scoped BGP bricks on the same device:

```yaml
apiVersion: networking.metal.ironcore.dev/v1alpha1
kind: VRF
metadata:
  name: leaf1-tenant-a
spec:
  deviceRef:
    name: leaf1
  name: TENANT_A
---
apiVersion: networking.metal.ironcore.dev/v1alpha1
kind: VRF
metadata:
  name: leaf1-tenant-b
spec:
  deviceRef:
    name: leaf1
  name: TENANT_B
---
apiVersion: networking.metal.ironcore.dev/v1alpha1
kind: BGP
metadata:
  name: leaf1-tenant-a-bgp
spec:
  deviceRef:
    name: leaf1
  vrfRef:
    name: leaf1-tenant-a
  asNumber: 65000
  routerId: 10.255.0.11
  addressFamilies:
    ipv4Unicast:
      enabled: true
---
apiVersion: networking.metal.ironcore.dev/v1alpha1
kind: BGP
metadata:
  name: leaf1-tenant-b-bgp
spec:
  deviceRef:
    name: leaf1
  vrfRef:
    name: leaf1-tenant-b
  asNumber: 65000
  routerId: 10.255.0.12
  addressFamilies:
    ipv4Unicast:
      enabled: true
```

Verify both VRF BGP domains and both markers exist:

```text
show running-config bgp
show running-config bgp | include __operator-managed--TENANT_A__
show running-config bgp | include __operator-managed--TENANT_B__
```

Delete only one VRF-scoped BGP brick:

```sh
kubectl --context "$KCTX" -n "$TEST_NS" delete bgp leaf1-tenant-a-bgp
kubectl --context "$KCTX" -n "$TEST_NS" wait --for=delete bgp/leaf1-tenant-a-bgp --timeout=10m
```

Record PASS only if:

- The `TENANT_A` BGP domain and `__operator-managed--TENANT_A__` marker are gone.
- The `TENANT_B` BGP domain and `__operator-managed--TENANT_B__` marker remain.
- The global BGP instance remains while `TENANT_B` is still managed.

Then delete the second BGP brick and verify global cleanup:

```sh
kubectl --context "$KCTX" -n "$TEST_NS" delete bgp leaf1-tenant-b-bgp
kubectl --context "$KCTX" -n "$TEST_NS" wait --for=delete bgp/leaf1-tenant-b-bgp --timeout=10m
```

Record PASS only if no BGP config and no ownership markers remain.

## Result Template

Use this table in the tracking issue:

| Case | CRs applied/removed | Operator result | Device CLI evidence | Result | Issue |
| ---- | ------------------- | --------------- | ------------------- | ------ | ----- |
| Bulk create | | | | PASS/FAIL | |
| Bulk delete | | | | PASS/FAIL | |
| Delete device with dependents | | | | PASS/FAIL | |
| NX-OS default BGP removal | | | | PASS/FAIL | |
| NX-OS per-VRF BGP removal | | | | PASS/FAIL | |

For every leftover or discrepancy, file a separate bug issue and link it from
the `Issue` column.

## Cleanup

Remove test CRs, the operator, and the lab namespace when finished:

```sh
kubectl --context "$KCTX" -n "$TEST_NS" delete -k /tmp/network-operator-bulk-brick/kubernetes --ignore-not-found
kubectl --context "$KCTX" delete namespace "$TEST_NS" --ignore-not-found
helm --kube-context "$KCTX" -n network-operator uninstall network-operator
cd /tmp/network-operator-bulk-brick/clab
clabverter --debug --destinationNamespace "$LAB_NS" --stdout | kubectl --context "$KCTX" delete -f - --ignore-not-found
kubectl --context "$KCTX" delete namespace "$LAB_NS"
```