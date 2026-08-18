# 0003: Status Conditions

- **Status:** Accepted
- **Date:** 2026-08-10
- **Authors:** @network-operator-maintainers

## Summary

All managed resources use a layered set of status conditions to
communicate their state. A top-level Ready condition is derived from
independent sub-conditions, each representing a distinct stage of
reconciliation. Staleness is communicated through observedGeneration.

## Motivation

Network resources go through multiple stages during reconciliation:
dependency resolution, device connection, configuration, and
operational state retrieval. A single top-level Ready condition cannot
express which stage failed or whether a resource is configured but
operationally degraded.

## Goals

- Provide clear, independent signals for each reconciliation stage.
- Ensure stale conditions are distinguishable from current ones.
- Define a consistent pattern all controllers must follow.
- Allow the top-level Ready condition to be computed automatically.

## Non-Goals

- Defining resource-specific status fields beyond conditions.
- Prescribing how external systems consume conditions.

## Decision

### Condition Types

Configuration resources carry these condition types:

- **Ready** — top-level rollup. True only when all other conditions
  (except Paused) are True. Computed automatically at the end of
  every reconciliation; controllers never set it directly.
- **Configured** — whether the desired state has been applied to the
  device. Updated at multiple points during reconciliation.
- **Operational** — live state read back from the device. Only updated
  after successful configuration.
- **Paused** — whether reconciliation is suspended. Excluded from the
  Ready rollup.

The Device resource uses a different set appropriate to its role
(it represents the device itself, not configuration applied to it).

### ObservedGeneration

Every condition carries observedGeneration, set automatically to the
object's current generation. This is critical for interpretation:

- If observedGeneration is less than the object's generation, the
  condition is stale. This can mean reconciliation has not yet run,
  or that an earlier stage failed and prevented the condition from
  being re-evaluated.
- Helper functions that check whether a resource is ready or
  configured return false when observedGeneration does not match,
  regardless of the condition's status value.

This is especially important for the Operational condition: when
configuration fails, the Operational condition retains its previous
value since the device was never queried for the new spec. The stale
observedGeneration tells consumers not to trust it.

### Reconciliation Flow

Controllers follow this sequence:

1. **Initialize** — first reconciliation sets all conditions to Unknown.
2. **Resolve dependencies** — if missing or invalid, set
   Configured=False with an appropriate reason and return. Watches on
   dependencies trigger re-reconciliation when they become ready.
3. **Connect and apply** — call the provider. The result is reflected
   in the Configured condition.
4. **Fetch operational state** — only if configuration succeeded.
   Update the Operational condition. Skipped on failure, leaving the
   previous value with a now-stale observedGeneration.
5. **Recompute Ready** — derive from sub-conditions via deferred call.

**Guardrails:**

- The Configured condition must be updated before returning from
  reconciliation, even if failure occurred during dependency
  resolution.
- The Operational condition must only be updated after successful
  configuration.
- Controllers must not set Ready directly — it is always derived.
- Consumers must compare observedGeneration before trusting a
  condition's status.

## Consequences

- Users can determine at a glance whether failure is in dependency
  resolution, configuration, or operational health.
- Automation can wait for Ready=True with matching observedGeneration
  to confirm a spec change is fully applied and verified.
- The Operational condition may appear True while Configured is False
  after a spec change — the stale observedGeneration communicates this
  correctly.

## Alternatives Considered

**Single Ready condition with detailed message:** Rejected because it
forces consumers to parse messages to determine the failure stage.

**Resetting conditions at the start of every reconciliation:** Rejected
because updating a condition's lastTransitionTime on every loop (even
Success→Success) triggers infinite reconciliation. Conditions must be
set explicitly at each failure/success point.

**Setting Operational to Unknown on configuration failure:** Rejected
because the device may still be running the previous configuration.
ObservedGeneration already communicates staleness without misreporting
the actual device state.
