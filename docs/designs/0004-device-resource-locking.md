# 0004: Device Resource Locking

- **Status:** Accepted
- **Date:** 2026-08-10
- **Authors:** @network-operator-maintainers

## Summary

Only one controller may configure a given device at a time. A
per-device lock serializes all configuration operations targeting the
same device. Controllers that cannot acquire the lock requeue with
priority-based ordering to reduce starvation.

## Motivation

Network devices expose management APIs with limited concurrency
capabilities. Some reject concurrent sessions, others produce
inconsistent state under parallel writes. Since each configuration
operation is applied as a single atomic gNMI Set request, the lock
prevents interleaving of multiple atomic operations that could leave
the device in an inconsistent intermediate state.

## Goals

- Prevent concurrent configuration of the same device.
- Minimize latency for resources waiting to configure a device.
- Prioritize foundational resources whose reconciliation unblocks
  dependent resources.

## Non-Goals

- Fine-grained locking per configuration subtree.
- Fairness guarantees across controllers.
- Distributed locking across multiple operator instances (handled by
  the watch-filter mechanism which partitions devices).

## Decision

### Per-Device Lock

A single lock is held per device, implemented as a Kubernetes Lease
resource. When a controller begins reconciliation, it attempts to
acquire the lock. If held by another controller, it requeues with a
short jittered delay and an elevated priority. After reconciliation,
the lock is released. Leases expire automatically if a controller
crashes without releasing.

### Wait Priorities

When a controller cannot acquire the lock, it requeues at one of two
priority levels:

- **High** — foundational resource types commonly referenced by other
  resources. Reconciling them first unblocks dependent resources.
- **Default** — all other resource types. Higher than the queue
  baseline used by periodic requeues of already-reconciled resources,
  ensuring lock-waiting resources are always served first.

A jitter of ±10% is applied to all periodic requeues to reduce lock
convoy effects.

### Known Shortcomings

**Coarse granularity:** The lock covers the entire device. Controllers
configuring independent subtrees are serialized unnecessarily, adding
latency during initial device onboarding.

**No fairness guarantee:** Lower-priority resources may be repeatedly
delayed under sustained contention from higher-priority resources.
The jittered delay provides randomization but not bounded wait time.

**Lease overhead:** Each lock operation involves Kubernetes API calls.
Under high contention this adds API server load proportional to
waiting controllers times retry frequency.

## Consequences

- Initial device configuration is serialized and takes longer than
  parallel application would.
- New resource types automatically participate by following the
  established controller pattern.
- The priority mechanism ensures dependent resources are not blocked
  indefinitely by foundational resources.

## Alternatives Considered

**Per-subtree locking:** Lock at configuration path granularity to
allow safe parallelism. Rejected due to the complexity of defining
non-overlapping subtrees across vendors and because some devices have
global commit semantics.

**No locking:** Rely on the device API to handle concurrency. Rejected
because device behavior under concurrent access varies by vendor —
some silently corrupt state, and finding a safe concurrency limit per
model is impractical.

**Queue-based serialization:** Route all configuration through a
single work queue per device. Rejected because it requires
architectural changes to controller-runtime's reconciliation model.
