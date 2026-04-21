# Pausing Reconciliation

Network operators may need to temporarily prevent controllers from reconciling
resources — for example during maintenance windows or manual debugging sessions.

## Pausing a Device

Setting `spec.paused: true` on a Device pauses reconciliation of the Device
**and all of its child resources** (Interfaces, VRFs, VLANs, BGP, etc.).

```yaml
apiVersion: networking.metal.ironcore.dev/v1alpha1
kind: Device
metadata:
  name: leaf-01
spec:
  paused: true
  endpoint:
    address: 10.0.0.1
```

## Pausing Individual Resources

The `networking.metal.ironcore.dev/paused` annotation can be applied to any
resource to pause its reconciliation independently of the parent Device.

```yaml
apiVersion: networking.metal.ironcore.dev/v1alpha1
kind: VRF
metadata:
  name: vrf-prod
  annotations:
    networking.metal.ironcore.dev/paused: "true"
spec:
  deviceRef:
    name: leaf-01
  name: prod
```

::: tip
You can quickly pause and unpause a resource using `kubectl annotate`:
```bash
# Pause
kubectl annotate vrf vrf-prod networking.metal.ironcore.dev/paused=true

# Unpause
kubectl annotate vrf vrf-prod networking.metal.ironcore.dev/paused-
```
:::

## Automatic Pausing

Child resources are also paused automatically when the device is not in a
state where configuration can be applied. This happens without any manual
intervention. The following conditions trigger automatic pausing, evaluated
in priority order:

| Priority | Cause                                                             |
| -------- | ----------------------------------------------------------------- |
| 1        | `spec.paused: true` on the Device                                 |
| 2        | Device `status.phase` is not `Running`                            |
| 3        | Device `Reachable` condition is not `True`                        |
| 4        | `networking.metal.ironcore.dev/paused` annotation on the resource |

::: info
The Device itself is exempt from automatic pausing (priorities 2 and 3). It
must continue reconciling in order to transition out of non-Running phases
and to update the `Reachable` condition.
:::

## Paused Condition

Every resource reflects its pause state in `.status.conditions` with a `Paused`
condition. The `Paused` column is visible with `-o wide`:

```bash
$ kubectl get vrfs -o wide
NAME       VRF    DEVICE    READY     PAUSED   AGE
vrf-prod   prod   leaf-01   Unknown   True     5m
```

The condition message indicates the reason:

```yaml
conditions:
  - type: Paused
    status: "True"
    reason: Paused
    message: "Device spec.paused is set to true"
```

## Effect on the Ready Condition

When a resource is paused, the `Ready` condition is set to `Unknown` with reason `Paused`. 

```yaml
conditions:
  - type: Ready
    status: "Unknown"
    reason: Paused
    message: "Reconciliation is paused"
  - type: Paused
    status: "True"
    reason: Paused
    message: "Device spec.paused is set to true"
```

The `kubectl get` output reflects this:

```bash 
$ kubectl get vrfs -o wide
NAME       VRF    DEVICE    READY     PAUSED   AGE
vrf-prod   prod   leaf-01   Unknown   True     5m
```

Once the resource is unpaused, the controller runs a full reconcile and
immediately sets `Ready` back to `True` or `False` based on the observed state.

::: info Why Unknown and not False?
`False` would imply the resource is broken. `Unknown` is the honest signal:
the operator has stopped actively verifying the resource, so its current
state is simply not known.
:::
