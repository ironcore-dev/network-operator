# Numbered Resource Allocation

The Network Operator provides a pool-based allocation system for numbered resources such as
indices (ASNs, VLAN IDs, route distinguishers), IP addresses, and IP prefixes. Resources are
claimed via a `Claim` object that references a pool; the controller allocates a value
automatically and writes it back to the claim's status.

<script setup>
import { useData } from 'vitepress'
import { computed } from 'vue'
import allocationImageLight from '../assets/numbered-resources.png?url'
import allocationImageDark from '../assets/numbered-resources-dark.png?url'

const { isDark } = useData()
const allocationImage = computed(() => isDark.value ? allocationImageDark : allocationImageLight)
</script>

<img :src="allocationImage" alt="Numbered Resource Allocation Flow" style="margin: 2em 0;" />

## Pool Types

Three pool types are available, each targeting a different kind of resource:

| Pool Kind       | Allocates                   | Example use case                       |
| --------------- | --------------------------- | -------------------------------------- |
| `IndexPool`     | `uint64` Index              | ASN, VLAN ID, VNI, Route Distinguisher |
| `IPAddressPool` | Single IP Address           | Loopback Address, Router ID            |
| `IPPrefixPool`  | IP Prefix of a given length | Subnet                                 |

## Concepts

### Pool

A pool defines the set of values that can be allocated. Each pool type has a
`spec.reclaimPolicy` field controlling what happens when a `Claim` is deleted:

- **`Recycle`** (default) — the allocation is returned to the pool and can be reused.
- **`Retain`** — the allocation is kept in the pool status as reserved and will never be
  reused, even after the claim is gone.

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

Each pool tracks allocations and exposes utilisation counters:

```bash
$ kubectl get indexpools
NAME       ALLOCATED   AVAILABLE   AGE
asn-pool   3           True        10m
```

The `Available` condition is set to `True` while there is at least one free slot, and
transitions to `False` (reason `Exhausted`) when all values have been allocated.

### Claim

A `Claim` references a pool via `spec.poolRef` and, once reconciled, receives the
allocated value in `status.allocation`. The controller guarantees idempotency: if a
claim already has an allocation in the pool status (matched by name and UID), it is
returned as-is without allocating a new value.

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

| Condition status | Reason                      | Meaning                                                          |
| ---------------- | --------------------------- | ---------------------------------------------------------------- |
| `True`           | `Allocated`                 | A value has been successfully allocated.                         |
| `False`          | `PoolExhausted`             | No values remain in the pool (terminal error).                   |
| `False`          | `PreferredValueUnavailable` | The requested preferred value is not available (terminal error). |
| `False`          | `PoolNotFound`              | The referenced pool does not exist (yet).                        |

### Allocation Result

Once allocated, `status.allocation` contains a type-specific field plus a `value` string that
is always present regardless of pool type:

| Pool type       | Field       | Example         |
| --------------- | ----------- | --------------- |
| `IndexPool`     | `index`     | `65001`         |
| `IPAddressPool` | `ipAddress` | `"10.0.0.5"`    |
| `IPPrefixPool`  | `prefix`    | `"10.2.0.0/26"` |

For the `leaf1-asn` claim created above, the allocated ASN is accessible via the `value` field,
which is always set to the string representation of the allocated resource:

```bash
$ kubectl get claim leaf1-asn -o jsonpath='{.status.allocation.value}'
64512
```

## Requesting a Preferred Value

A specific value can be requested by setting the annotation
`pool.networking.metal.ironcore.dev/preferred-value` on the claim:

```yaml
apiVersion: pool.networking.metal.ironcore.dev/v1alpha1
kind: Claim
metadata:
  name: leaf1-asn
  annotations:
    pool.networking.metal.ironcore.dev/preferred-value: "65001"
spec:
  poolRef:
    kind: IndexPool
    name: asn-pool
```

The annotation works the same way for all pool types:

| Pool type       | Example annotation value |
| --------------- | ------------------------ |
| `IndexPool`     | `"65001"`                |
| `IPAddressPool` | `"10.0.0.42"`            |
| `IPPrefixPool`  | `"10.2.5.0/24"`          |

If the preferred value is outside the pool's configured ranges or already taken by another
claim, the claim enters a terminal error state with the `PreferredValueUnavailable` reason.
**Removing the annotation** re-triggers reconciliation and the controller falls back to
allocating the next available value automatically.

::: tip
The annotation is read but not consumed — it stays on the object. This means you can
inspect a claim and immediately see which value was requested.
:::

::: warning
A `PreferredValueUnavailable` error is terminal. The controller will not retry until
the annotation is changed or removed.
:::
