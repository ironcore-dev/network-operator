# 0006: API Deprecation Strategy

- **Status:** Accepted
- **Date:** 2026-08-10
- **Authors:** @network-operator-maintainers

## Summary

Breaking changes to CRD specs follow a multi-step deprecation process
that prevents disruption to existing deployments.

## Motivation

The operator's CRDs are consumed by multiple teams and automated
systems. Removing or renaming a field without a migration path breaks
existing deployments.

## Goals

- Prevent breaking existing deployments on upgrades.
- Provide a clear, repeatable process for API evolution.
- Ensure deprecated fields are eventually removed.

## Non-Goals

- Defining when a new API version is warranted.
- Versioning strategy for the Helm chart.

## Decision

Breaking API changes must follow four steps:

1. **Add the new field** alongside the old one. Mark the old field as
   deprecated.
2. **Support both fields** in the controller. The old field is used
   when the new one is absent; the new field takes precedence when
   both are set.
3. **Migrate consumers** — update all known deployments to use the
   new field. Communicate via release notes.
4. **Remove the old field** once no known consumer uses it.

**Guardrails:**

- Each step must be a separate release.
- The old field must remain functional for at least one release cycle
  after the new field is introduced.
- Removal requires confirmation that known deployments have migrated.

## Consequences

- API evolution is slower but safe.
- The codebase temporarily carries compatibility code. This is
  acceptable as it is time-bounded.
- External consumers can migrate at their own pace within the
  deprecation window.

## Alternatives Considered

**Immediate removal with API version bump:** Rejected because CRD
conversion webhooks add significant complexity, especially for alpha
APIs with frequent changes.

**Never remove deprecated fields:** Rejected because it accumulates
maintenance burden and confuses users with redundant fields.
