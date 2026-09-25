---
name: provider-matrix
description: Generate or update the provider compatibility matrix at docs/provider-compatibility.md. Reads API types and provider source to determine what each provider supports, what is partially supported (with unsupported fields), and what is platform-exclusive.
allowed-tools: [Read, Bash, Edit, Write]
---

# provider-matrix

Generate the provider compatibility matrix at `docs/provider-compatibility.md`.

The matrix shows which Kubernetes API types (Kinds) each network device provider supports,
with partial support annotations and platform-exclusive kinds clearly marked.

> **Important:** Always read the current `docs/provider-compatibility.md` first before making
> any changes. After generating the new content, present it to the user for review.
> **Do not write to the file without explicit user approval.**

---

## Step 1 — Discover all API Kinds

Scan all `*_types.go` files for types marked `+kubebuilder:object:root=true`.
Exclude `*List` types. Organise by package:

**Core kinds** — `api/core/v1alpha1/`
These are provider-neutral and map 1:1 to provider interfaces in `internal/provider/provider.go`.
Each core kind has a corresponding `*Provider` interface (e.g. `DNS` → `DNSProvider`).

**Platform-exclusive kinds** — `api/cisco/nx/v1alpha1/` (and any future `api/<vendor>/*/`)
These are vendor-specific CRDs with no core provider interface.
Scan the directory for kinds and split them into two groups:
- **Config extensions**: have a `Register*Dependency` call in their `init()` — belong in the per-provider "Provider-specific types" table, not the main matrix.
- **Standalone**: no `Register*Dependency` call — appear as rows in the main matrix with N/A for all other providers.

**Non-provider API packages** — `api/evpn/`, `api/pool/`, `api/cisco/xr/`, `api/cisco/xe/`
These are infrastructure/orchestration types, not provider capability CRDs.
Do **not** include them in the matrix.

---

## Step 2 — Discover provider implementations

There are three registered providers. For each, check which core `*Provider` interfaces
it implements by looking for compile-time assertions or method presence.

### Cisco NX-OS
- Source: `internal/provider/cisco/nxos/`
- API: `api/cisco/nx/v1alpha1/`
- **Note:** All interface implementations live in a single large `provider.go` file. To find `NewUnsupportedFieldError` calls for a given interface, identify the method name from the interface definition and search for it in `provider.go`. Helper functions called from that method may also contain violations — read the called functions too.

### Cisco IOS-XR
- Source: `internal/provider/cisco/iosxr/`
- API: `api/cisco/xr/v1alpha1/` (may be empty — check for any CRDs)
- Read compile-time assertions in `provider.go` to discover implemented interfaces.

### Nokia SRL (OpenConfig)
- Source: `internal/provider/openconfig/`
- API: none (no OpenConfig-specific CRDs)
- **Note:** Each interface is implemented in its own dedicated file (e.g. `dns.go` → `DNSProvider`). The `var _ provider.XxxProvider` assertion at the top of each file tells you which interface it implements. Scan all files in the directory to collect the full list.

---

## Step 3 — Detect partial support (unsupported fields)

For each provider+interface combination that is implemented, check whether any
fields are reported as unsupported at runtime.

### How to find unsupported fields

Search for `apistatus.NewUnsupportedFieldError` calls in the provider source.
Each call contains one or more `apistatus.FieldViolation{Field: "...", Description: "..."}` literals.
Always read **both** `Field` and `Description`. Use the description to understand what is actually
unsupported — sometimes only specific values of a field are unsupported (e.g. a specific enum value),
not the field itself. In that case, the footnote should mention the specific constraint from the
description, not just the field path.

**Pattern A — inline:**
```go
return apistatus.NewUnsupportedFieldError(apistatus.FieldViolation{
    Field:       "spec.type",
    Description: "unsupported interface type: ...",
})
```

**Pattern B — accumulated slice:**
```go
var violations []apistatus.FieldViolation
if condition {
    violations = append(violations, apistatus.FieldViolation{
        Field:       "spec.adminState",
        Description: "adminState Down is not supported",
    })
}
if len(violations) > 0 {
    return apistatus.NewUnsupportedFieldError(violations...)
}
```

**Pattern C — helper function:**
The `FieldViolation` literals may be in a helper function called from the interface method
(e.g. `validateDNSSpec` called from `EnsureDNS`). Read the called function too.

**Important:** Only `NewUnsupportedFieldError` indicates a missing feature.
`NewInvalidArgumentError` indicates a bad value for a supported field — do NOT include these.
Common traps that are always `NewInvalidArgumentError`, not unsupported:
- Length or line limits
- Format constraints (e.g. a field must be a specific type or pattern)
- Invalid combinations of supported fields
If the provider's `Ensure*` method reads and uses the field, the field is supported.

Additionally, do NOT include a field if only some values of it are unsupported — whether that's
a `default:` case for unknown enum values, or a named case for a specific known value.
**Only include a field if the field itself is entirely unsupported** (the provider never reads
or acts on it regardless of its value).

For `fmt.Sprintf` field values like `fmt.Sprintf("spec.servers[%s].vrfName", addr)`,
normalise the dynamic part to `*`: → `spec.servers[*].vrfName`.

**Attribution:** To map a `FieldViolation` to an interface, find which interface method
contains (or calls into) the function with the violation. For NX-OS (single `provider.go`),
match by method name (e.g. `EnsureInterface` → `InterfaceProvider`). For OpenConfig,
each file is one interface.

### How to find ignored fields

Some fields are silently ignored (not configured on the device) rather than rejected.
These appear as code comments like:
```go
// PrependLocalAS / PrependGlobalAS not supported via OC local-as leaf
// spec.grpc.serverName is not configurable on this platform
```
Search for `// not supported`, `// unsupported`, `// ignored` comments near field access code.
These should also be listed as partial support notes.

---

## Step 4 — Write the matrix

Work through Steps 1–3 by reading the source files directly, then write `docs/provider-compatibility.md`
using the output format below.

**Commands to gather data:**

```bash
# Core kinds
grep -rn "kubebuilder:object:root=true" api/core/v1alpha1/ | grep -v List

# NX-OS standalone kinds (no Register*Dependency)
grep -rn "kubebuilder:object:root=true" api/cisco/nx/v1alpha1/ | grep -v List

# NX-OS config extensions (have Register*Dependency)
grep -rn "Register.*Dependency" api/cisco/nx/v1alpha1/

# IOS-XR interface assertions
grep "_ provider\." internal/provider/cisco/iosxr/provider.go

# OpenConfig interface assertions
grep -rn "var _ provider\." internal/provider/openconfig/ | grep -v _test

# All NewUnsupportedFieldError field+description values — OpenConfig
grep -A5 "FieldViolation{" internal/provider/openconfig/*.go | grep -E "Field:|Description:"

# All NewUnsupportedFieldError field+description values — NX-OS
grep -A5 "FieldViolation{" internal/provider/cisco/nxos/provider.go | grep -E "Field:|Description:"
```

After collecting the data, write the full markdown directly to `docs/provider-compatibility.md`.
The file header should say `<!-- To regenerate this matrix, run: /provider-matrix -->`.

---

## Output format

### File header

```markdown
<!--
SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
SPDX-License-Identifier: Apache-2.0
-->

# Provider Compatibility Matrix

This document provides a detailed overview of which API types are supported by each network device provider.

<!-- To regenerate this matrix, use the /provider-matrix skill -->
```

### Compatibility Matrix table

Columns: `Core Kind` + one column per provider (in registration order).
Rows: all core kinds + platform-exclusive standalone kinds, sorted alphabetically by Kind name.

```markdown
## Compatibility Matrix

| Core Kind | Cisco NX-OS | Cisco IOS-XR | Nokia SRL (OpenConfig) |
|-----------|--------|--------|--------|
| `AAA` | ✅ | — | ⚠️ [^1] |
| `AccessControlList` | ✅ | — | ✅ |
| `BorderGateway` | ✅ | N/A | N/A |
| `DNS` | ✅ | — | ⚠️ [^2] |
...
```

Cell values:
- `✅` — fully supported (implements the interface, no unsupported fields)
- `⚠️ [^N]` — partially supported (implements the interface but has unsupported fields; footnote lists them)
- `—` — not supported (interface not implemented)
- `N/A` — not applicable (platform-exclusive kind, other providers cannot support it by design)

Legend (always include):
```markdown
**Legend:**
- ✅ Supported
- ⚠️ Partial support (see footnotes)
- — Not supported
- N/A Not applicable (platform-exclusive feature)
```

### Footnotes

Immediately after the legend, list footnotes in order 1..N (matching their appearance in the table).
Use the description to phrase each entry accurately:
- If the whole field is unsupported: `unsupported fields: \`field\``

```markdown
[^1]: **Nokia SRL (OpenConfig) — AAA**: unsupported fields: `spec.authorization`, `spec.serverGroups[].type`
```

### Per-provider sections

One `##` section per provider, in registration order. Each section contains:
- The provider's display name as heading
- A one-line description of the provider
- `**Core API types:** X / Y` where X is the count of implemented core interfaces and Y is the total. Append `+ Z platform-exclusive` if the provider has standalone kinds.
- A `**Provider-specific types:**` table (only if the provider has config extensions or standalone kinds), with columns `Kind` and `Category`. Config extensions show `Extends core type \`<CoreKind>\``. Standalone kinds show `Provider-exclusive`.

Example structure:
```markdown
## <Provider Display Name>

<Provider description.>

**Core API types:** X / Y (+ Z platform-exclusive if applicable)

**Provider-specific types:** (omit section if none)

| Kind | Category |
|------|----------|
| `<ConfigExtensionKind>` | Extends core type `<CoreKind>` |
| `<StandaloneKind>` | Provider-exclusive |
```

### Contributing section

```markdown
## Contributing

To add support for a new API type or provider:

1. Define the provider interface in `internal/provider/`
2. Implement the interface methods in your provider package
3. Use the `/provider-matrix` skill to regenerate this document
4. Submit a pull request with your changes

The matrix is automatically generated by checking which provider interfaces
each provider implements, ensuring accuracy and eliminating manual maintenance.
```

---

## Verification checklist

After generating, verify:
- [ ] Every `+kubebuilder:object:root=true` non-List kind from `api/core/v1alpha1/` appears as a row (excluding `Device` which is internal)
- [ ] Every standalone kind from each vendor API dir (no `Register*Dependency`) appears as a row with N/A for all providers that don't own it
- [ ] Each provider's core count matches the number of capability interfaces it implements (excluding base interfaces: `Provider`, `ProvisioningProvider`, `DeviceProvider`)
- [ ] Each provider's platform-exclusive count matches the number of standalone kinds in its API dir
- [ ] Footnote numbers are sequential 1..N in left-to-right, top-to-bottom table order
- [ ] Every footnoted field came from `NewUnsupportedFieldError` — not `NewInvalidArgumentError`

## Final step — Human review

Present the generated content to the user and wait for approval before writing to `docs/provider-compatibility.md`.
Do not write the file automatically.
