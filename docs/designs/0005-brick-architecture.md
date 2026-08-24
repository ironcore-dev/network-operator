# 0005: Brick Architecture

- **Status:** Accepted
- **Date:** 2026-08-10
- **Authors:** @network-operator-maintainers

## Summary

Every configuration resource (called a "brick") represents exactly one
configuration stanza applied to exactly one device. Bricks are the
atomic unit of network configuration in this project.

## Motivation

Network device configuration is composed of many independent stanzas.
Representing each as a separate Kubernetes resource enables independent
lifecycle management, fine-grained RBAC, selective reconciliation, and
clear ownership boundaries.

## Goals

- Define the smallest meaningful unit of configuration.
- Enable independent reconciliation of each stanza.
- Support applying configuration to multiple devices through
  composition, not resource-level multi-targeting.
- Support pausing and excluding individual units.

## Non-Goals

- Defining higher-level abstractions that compose bricks.
- Prescribing how bricks are generated or templated.

## Decision

### Definition

A brick is a Kubernetes custom resource that:

1. Corresponds to exactly one configuration stanza on exactly one
   device.
2. References its target device via a device reference field.
3. Is reconciled by its own dedicated controller.
4. Reports its own status conditions independently.

### One Brick, One Stanza

A brick must not span multiple independent configuration sections.
If a feature requires configuration across multiple sections, it is
modeled as multiple bricks with cross-references between them.

### One Brick, One Device

A brick targets exactly one device. Applying the same configuration
to multiple devices requires one brick per device. Higher-level tools
(Helm, fabric controllers) automate this.

### Eventual Consistency

Bricks are reconciled independently and without a prescribed order.
The operator does not enforce sequencing. Instead:

- Bricks that depend on other bricks check whether their dependency
  is ready at reconciliation time.
- If not ready, the brick reports a waiting state and returns without
  connecting to the device.
- Watches on dependencies trigger re-reconciliation when they become
  ready.

All bricks for a device can be created simultaneously in any order.
The system converges through repeated reconciliation.

### Ownership and Lifecycle

- Bricks are owned by their device. Deleting a Device cascades
  deletion to all its bricks. This relies on foreground cascading
  deletion, as owner-blocking semantics are only evaluated in that
  mode.[^1]
- A finalizer on each brick triggers cleanup on the device before
  the Kubernetes resource is removed.
- Bricks can be individually paused or collectively paused through
  their device.

**Guardrails:**

- New configuration resources must follow the brick pattern.
- Bricks must not embed configuration for another brick's domain.
- Cross-references between bricks must validate that both target the
  same device.
- Bricks must be reconcilable in any order — dependency ordering is
  handled through waiting and watches, never explicit sequencing.

## Consequences

- A fully configured device has many brick resources. This is by
  design — it enables independent management.
- Higher-level abstractions compose bricks but do not replace them.
- Adding a new device feature means adding a new CRD, controller, and
  provider interface following established patterns.

## Alternatives Considered

**Monolithic device configuration resource:** Rejected because it
prevents independent lifecycle management and makes RBAC coarse-grained.

**Grouped resources (e.g. one "routing" resource):** Rejected because
stanza boundaries differ per vendor, grouping creates implicit
dependencies, and it prevents selective provider implementation.

[^1]: https://kubernetes.io/docs/tasks/administer-cluster/use-cascading-deletion/#use-foreground-cascading-deletion
