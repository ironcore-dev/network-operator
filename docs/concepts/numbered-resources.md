# Numbered Resource Allocation

The Network Operator provides a pool-based allocation system for numbered resources such as
indices (ASNs, VLAN IDs, route distinguishers), IP addresses, and IP prefixes. Resources are
claimed via a `Claim` object that references a pool; the controller allocates a value by creating
a dedicated allocation object (`Index`, `IPAddress`, or `IPPrefix`) and writes a reference back
to the claim's status.

<script setup>
import { useData } from 'vitepress'
import { computed } from 'vue'
import allocationImageLight from '../assets/numbered-resources.svg?url'
import allocationImageDark from '../assets/numbered-resources-dark.svg?url'

const { isDark } = useData()
const allocationImage = computed(() => isDark.value ? allocationImageDark : allocationImageLight)
</script>

<img :src="allocationImage" alt="Numbered Resource Allocation Flow" style="margin: 2em 0;" />

## Pool Types

Three pool types are available, each targeting a different kind of resource:

| Pool Kind       | Allocates                   | Allocation Object | Example use case                       |
| --------------- | --------------------------- | ----------------- | -------------------------------------- |
| `IndexPool`     | `int64` Index               | `Index`           | ASN, VLAN ID, VNI, Route Distinguisher |
| `IPAddressPool` | Single IP Address           | `IPAddress`       | Loopback Address, Router ID            |
| `IPPrefixPool`  | IP Prefix of a given length | `IPPrefix`        | Subnet                                 |

## Concepts

### Pool

A pool defines the set of values that can be allocated. Each pool type has a
`spec.reclaimPolicy` field controlling what happens when a `Claim` is deleted:

- **`Recycle`** (default) — the allocation object is deleted and the value becomes available
  for reuse.
- **`Retain`** — the allocation object's `claimRef` is cleared but the object itself is kept,
  reserving the value. The allocation can be rebound later using the `allow-binding` annotation.

The example below creates an `IndexPool` covering the private-use ASN ranges defined in
[RFC 6996](https://datatracker.ietf.org/doc/html/rfc6996):

```yaml
apiVersion: pool.networking.metal.ironcore.dev/v1alpha1
kind: IndexPool
metadata:
  name: asn-pool
spec:
  ranges:
    - 64512..65534
    - 4200000000..4294967294
```

For IP-based pools, sample manifests for all pool types are available in the repository:

- [`config/samples/v1alpha1_indexpool.yaml`](https://github.com/ironcore-dev/network-operator/blob/main/config/samples/v1alpha1_indexpool.yaml)
- [`config/samples/v1alpha1_ipaddresspool.yaml`](https://github.com/ironcore-dev/network-operator/blob/main/config/samples/v1alpha1_ipaddresspool.yaml)
- [`config/samples/v1alpha1_ipprefixpool.yaml`](https://github.com/ironcore-dev/network-operator/blob/main/config/samples/v1alpha1_ipprefixpool.yaml)

#### Pool Status

Each pool tracks the number of allocations (by counting allocation objects) and exposes
utilisation counters:

```bash
$ kubectl get indexpools
NAME       ALLOCATED   AVAILABLE   AGE
asn-pool   3           True        10m
```

The `Available` condition is set to `True` while there is at least one free slot, and
transitions to `False` (reason `Exhausted`) when all values have been allocated.

### Allocation Objects

When a claim is reconciled, the controller creates a dedicated Kubernetes object — `Index`,
`IPAddress`, or `IPPrefix` — with a deterministic name derived from the pool name and the
allocated value (e.g. `asn-pool-64512`). This deterministic naming prevents duplicate
allocations: if two controllers race for the same value, exactly one `Create` succeeds and
the other retries with the next available value.

Each allocation object has:

- **`spec.poolRef`** — a back-reference to the owning pool.
- **`spec.claimRef`** — identifies the bound `Claim` by name and UID.
- **A value field** — `spec.index`, `spec.address`, or `spec.prefix` depending on the type.

The allocation type controllers validate each object's value against its pool's ranges/prefixes
and set a `Valid` condition.

Sample manifests for allocation objects (useful for pre-provisioning) are available:

- [`config/samples/v1alpha1_index.yaml`](https://github.com/ironcore-dev/network-operator/blob/main/config/samples/v1alpha1_index.yaml)
- [`config/samples/v1alpha1_ipaddress.yaml`](https://github.com/ironcore-dev/network-operator/blob/main/config/samples/v1alpha1_ipaddress.yaml)
- [`config/samples/v1alpha1_ipprefix.yaml`](https://github.com/ironcore-dev/network-operator/blob/main/config/samples/v1alpha1_ipprefix.yaml)

### Claim

A `Claim` references a pool via `spec.poolRef` and, once reconciled, receives the
allocated value in `status.value` and a reference to the allocation object in
`status.allocationRef`. The controller guarantees idempotency: if an allocation object
already exists with a matching `claimRef` (by name and UID), it is returned as-is without
allocating a new value.

The example below creates a claim against the `asn-pool` defined above:

```yaml
apiVersion: pool.networking.metal.ironcore.dev/v1alpha1
kind: Claim
metadata:
  name: leaf1-asn
spec:
  poolRef:
    kind: IndexPool
    name: asn-pool
```

Additional sample manifests for claims against each pool type are available in the repository:

- [`config/samples/v1alpha1_claim.yaml`](https://github.com/ironcore-dev/network-operator/blob/main/config/samples/v1alpha1_claim.yaml)

The `Allocated` condition on the claim reflects the current state:

| Condition status | Reason          | Meaning                                        |
| ---------------- | --------------- | ---------------------------------------------- |
| `True`           | `Allocated`     | A value has been successfully allocated.       |
| `False`          | `PoolExhausted` | No values remain in the pool (terminal error). |
| `False`          | `PoolNotFound`  | The referenced pool does not exist (yet).      |

### Allocation Result

Once allocated, `status.value` contains the string representation of the allocated resource
and `status.allocationRef` references the allocation object:

```bash
$ kubectl get claim leaf1-asn -o jsonpath='{.status.value}'
64512

$ kubectl get claim leaf1-asn -o jsonpath='{.status.allocationRef.name}'
asn-pool-64512
```

## Pre-provisioning and Rebinding

Instead of letting the controller pick the next available value, you can pre-provision an
allocation object with a specific value. To allow a `Claim` to bind to it, the allocation
object must:

1. Have a `spec.claimRef` with the `name` of the expected claim.
2. Carry the `pool.networking.metal.ironcore.dev/allow-binding` annotation.

When the claim controller finds an allocation whose `claimRef.name` matches but whose UID is
stale (or empty), and the annotation is present, it updates the UID to bind the allocation to
the current claim.

```yaml
apiVersion: pool.networking.metal.ironcore.dev/v1alpha1
kind: Index
metadata:
  name: asn-pool-65001
  annotations:
    pool.networking.metal.ironcore.dev/allow-binding: "true"
spec:
  poolRef:
    apiVersion: pool.networking.metal.ironcore.dev/v1alpha1
    kind: IndexPool
    name: asn-pool
  index: 65001
  claimRef:
    name: leaf1-asn
```

This mechanism also enables rebinding after a claim is deleted and recreated with the same
name — the retained allocation (with `Retain` reclaim policy) can be rebound if the
`allow-binding` annotation is set.

::: tip
The `allow-binding` annotation is only checked when the `claimRef.name` matches but the UID
does not. A fully bound allocation (matching name and UID) is always returned directly.
:::
