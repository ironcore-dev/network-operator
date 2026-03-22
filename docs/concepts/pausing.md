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

## Paused Condition

Every resource reflects its pause state in `.status.conditions` with a `Paused`
condition. The `Paused` column is visible with `-o wide`:

```
$ kubectl get vrfs -o wide
NAME       VRF    DEVICE    READY   PAUSED   AGE
vrf-prod   prod   leaf-01   True    True     5m
```

The condition message indicates the reason:

```yaml
conditions:
  - type: Paused
    status: "True"
    reason: Paused
    message: "Device spec.paused is set to true"
```

::: warning
The `Paused` condition does not affect the `Ready` condition.
:::
