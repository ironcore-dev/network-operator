# 0001: Device Lifecycle Phases

- **Status:** Accepted
- **Date:** 2026-08-10
- **Authors:** @network-operator-maintainers

## Summary

A Device resource progresses through a fixed set of phases that
represent its provisioning and operational state. Configuration
resources are automatically held back until their device reaches the
appropriate phase.

## Motivation

Network devices go through distinct stages: initial discovery, optional
zero-touch provisioning, post-provisioning verification, and
steady-state operation. Without explicit phase tracking, controllers
cannot determine whether a device is ready to accept configuration.

## Goals

- Define a clear progression of device states.
- Ensure configuration resources only apply when the device is ready.
- Provide a recovery path from terminal failure states.
- Support devices that skip provisioning entirely.

## Non-Goals

- Defining provisioning script content or format.
- Specifying vendor-specific boot sequences.

## Decision

A Device has exactly one phase at any time: `Pending`, `Provisioning`,
`Provisioned`, `Running`, or `Failed`. The phase progresses linearly
from Pending toward Running, with failure branches leading to Failed.

Devices without provisioning configuration skip directly from Pending
to Running. The Failed phase is terminal and requires explicit manual
intervention to reset.

### Automatic Pausing of Child Resources

Configuration resources are automatically paused when their referenced
device is not in Running phase or is not reachable. This is enforced
centrally by the `paused` package, which all child-resource controllers
invoke before reconciliation. Individual controllers do not need to
check device phase themselves.

Reconciliation can also be paused explicitly through a field on the
Device spec (affecting the device and all its children) or through an
annotation on any individual resource.

**Guardrails:**

- The device controller must not skip phases or transition backwards
  except through explicit maintenance operations.
- A device in Failed phase must not automatically retry.
- New configuration resource controllers must call the shared paused
  check — they inherit phase gating without additional logic.

## Consequences

- Adding new configuration resources does not require per-controller
  phase checks.
- Adding new phases requires updating the paused package, not every
  controller.

## Alternatives Considered

**Single Ready condition instead of phases:** Rejected because a single
condition cannot distinguish provisioning-in-progress from
post-verification from failed.

**Automatic retry on failure:** Rejected because provisioning failures
often require physical intervention and retries would mask persistent
problems.
