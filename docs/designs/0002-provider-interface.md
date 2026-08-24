# 0002: Provider Interface

- **Status:** Accepted
- **Date:** 2026-08-10
- **Authors:** @network-operator-maintainers

## Summary

All device communication is abstracted behind a provider interface.
Controllers never interact with devices directly — they go through a
provider that encapsulates vendor-specific and protocol-specific logic.

## Motivation

The operator supports multiple device vendors and protocols. Without a
provider abstraction, every controller would need vendor-specific
branches, making the codebase unmaintainable and untestable.

## Goals

- Decouple controller logic from device communication details.
- Allow adding new vendors without modifying existing controllers.
- Enable testing controllers without real hardware.
- Support capability-based feature detection at runtime.
- Allow vendor-specific configuration extensions without polluting the
  core API.

## Non-Goals

- Defining a universal network configuration data model.

## Decision

### Registration

Providers self-register at startup. The operator selects the active
provider(s) based on configuration. Provider implementations are
compiled in and imported in the main entrypoint.

### Capability Interfaces

Each resource type has a corresponding provider interface. Providers
implement only the interfaces they support. Controllers use type
assertions to detect capabilities at runtime and report an error
condition when a required capability is missing.

All per-resource interfaces follow a consistent Ensure/Delete pattern,
with optional status retrieval methods for operational state.

### Vendor-Specific Extensions (providerConfigRef)

Core API resources define a vendor-neutral configuration surface. When
a vendor requires additional settings, they are expressed as a separate
vendor-specific CRD referenced via `spec.providerConfigRef`.

The controller fetches the referenced object as unstructured data and
passes it through to the provider, which deserializes it into the
appropriate type. This keeps vendor awareness entirely within the
provider — core controllers never interpret vendor-specific content.

Vendor-specific CRDs live under dedicated API groups, separate from
the core API group.

### Vendor-Specific Resources

Some resources are entirely vendor-specific with no core equivalent.
These are defined as standalone CRDs under the vendor API group with
their own controllers, following the same patterns as core controllers
(device reference, pausing, conditions).

### Structured Errors (apistatus)

The core API includes fields supported by the majority of vendors.
Providers that cannot support a field communicate this through
structured errors with three categories:

- **Unsupported field** — terminal; the resource cannot be realized
  until the field is removed.
- **Invalid argument** — terminal; the value violates
  provider-specific constraints.
- **Failed precondition** — retryable; a device-side dependency is
  not yet met.

These errors are surfaced to the user through status conditions with
the specific field path and description.

### Protocol Preferences

gNMI is the preferred protocol. Provider implementations use
hand-crafted Go structs rather than code generated from YANG models.
Complete YANG models generate large amounts of code covering far more
than what this project needs. Additionally, generating from a model at
a particular version makes cross-version compatibility difficult, while
hand-crafted structs can accommodate version-specific differences
through custom marshalling.

A shared transport layer provides gRPC client setup with automatic
terminal error detection — non-retryable gRPC status codes are wrapped
to prevent infinite retry loops.

### In-Tree Implementations

Provider implementations live in-tree. Splitting into dedicated
repositories is deferred until complexity justifies it.

**Guardrails:**

- Controllers must never import vendor-specific packages.
- New device features must be added as capability interfaces — never
  as provider-specific code in a controller.
- Vendor-specific extensions to core resources must use
  providerConfigRef — never additional fields on the core type.
- All provider request structs include a ProviderConfig field;
  providers ignore it when nil.

## Consequences

- Adding a new vendor requires only implementing interfaces and
  registering. No controller changes needed.
- Controllers must gracefully handle missing capabilities.
- The providerConfigRef mechanism allows extending any core resource
  without forking the core API.

## Alternatives Considered

**Out-of-process provider as a gRPC service:** A provider could run as
a separate service, potentially on the device itself. Rejected for now
due to operational complexity. Remains an option if provider isolation
or on-device execution becomes a requirement.
