---
name: cisco-yang-explore
description: Navigate a Cisco NX-OS 10.6-4 YANG path and explain its structure, leaves, and enum values
argument-hint: "<gnmi-path> [\\nAttributes: attr1, attr2, ...]"
allowed-tools: [Bash, Read, AskUserQuestion]
allowed-bash:
  - "git clone *"
  - "git -C * sparse-checkout *"
  - "git -C * log --oneline -1"
  - "grep -rn *"
  - "grep -n *"
  - "find * -name *.yang"
  - "ls *"
---

# yang-explore

**Purpose**: Given a Cisco NX-OS gNMI path, locate the terminal node in the vendor YANG
model files, follow any `uses`/grouping references, resolve enum typedefs, and produce a
structured report of the node's type, keys, leaves, and raw definition block.

The YANG files can be hundreds of thousands of lines; **never read whole files**.
Always use `grep` to find a line number, then use the `Read` tool with `offset` and
`limit` to read a targeted window.

---

## Step 0 — Ensure the YANG repo is available

Only the `vendor/cisco/nx/10.6-4` subdirectory of the `YangModels/yang` repo is needed.
The full repo is over 4GB, so use a sparse, blobless, shallow clone rather than cloning
everything.

Default clone location: `/tmp/cisco-yang-explore`

Check whether the main device model already exists at the default location:

```bash
ls /tmp/cisco-yang-explore/vendor/cisco/nx/10.6-4/Cisco-NX-OS-device.yang
```

**If the file exists**, set `YANG_DIR=/tmp/cisco-yang-explore/vendor/cisco/nx/10.6-4`
and continue to Step 1.

**If the file is missing**, the repo has not been cloned. **Always ask the user** (via
`AskUserQuestion`) whether to clone it to `/tmp/cisco-yang-explore` (the default) or to a
different location. With `<DEST>` the chosen location, clone just the needed
subdirectory (may take a minute or two):

```bash
git clone --depth 1 --filter=blob:none --sparse https://github.com/YangModels/yang <DEST>
git -C <DEST> sparse-checkout set vendor/cisco/nx/10.6-4
```

Set `YANG_DIR=<DEST>/vendor/cisco/nx/10.6-4`, then confirm the file now exists:

```bash
ls "$YANG_DIR"/Cisco-NX-OS-device.yang
```

If the clone failed or the file is still missing, report the error and stop.

---

## Step 1 — Parse $ARGUMENTS

`$ARGUMENTS` may span two lines:

- **Line 1** (required): the full gNMI path, e.g.
  `System/bgp-items/inst-items/dom-items/Dom-list[name={vrf}]/af-items/DomAf-list[type=ipv4-ucast]/interleak-items/InterLeakP-list[proto=static]`
- **Line 2** (optional): starts with `Attributes:` followed by a comma-separated list, e.g.
  `Attributes: rtMap, inst, asn, scope`

Parse as follows:

1. Split `$ARGUMENTS` on the first newline to separate the path from the optional attributes line.
2. Split the path by `/` to get segments. Skip the leading `System` segment.
3. For each remaining segment, split on `[` to separate the **node name** from the optional
   **key qualifier**:
   - `bgp-items` → node `bgp-items`, no qualifier
   - `Dom-list[name={vrf}]` → node `Dom-list`, qualifier `name={vrf}`
   - `InterLeakP-list[proto=static]` → node `InterLeakP-list`, qualifier `proto=static`
4. Record:
   - `TERMINAL_NODE` = last segment's node name (e.g. `InterLeakP-list`)
   - `TERMINAL_QUALIFIER` = last segment's qualifier, may be empty
   - `PARENT_NODE` = second-to-last segment's node name (e.g. `interleak-items`)
   - `REQUESTED_ATTRS` = parsed attribute names from line 2 (empty if absent)

**Node type hints from NX-OS naming conventions**:
- `*-items` suffix → almost always a `container`
- `*-list` suffix → almost always a `list` (with key qualifiers in the path)

---

## Step 2 — Locate the terminal node definition

Search all YANG files in `YANG_DIR` for the terminal node:

```bash
grep -rn "list TERMINAL_NODE\b" "$YANG_DIR"/
grep -rn "container TERMINAL_NODE\b" "$YANG_DIR"/
```

(Replace `TERMINAL_NODE` with the actual node name, e.g. `InterLeakP-list`.)

**If zero matches**: try without the word-boundary anchor:
```bash
grep -rn "TERMINAL_NODE" "$YANG_DIR"/
```
Report "node name not found in YANG files" if still nothing.

**If exactly one match**: record `DEFINITION_FILE` and `DEFINITION_LINE`.

**If multiple matches** (same name defined in multiple feature submodule files): run the
disambiguation sub-step below.

### Disambiguation (only when multiple matches)

For each candidate file, check whether the **parent node** also appears in that file:

```bash
grep -n "PARENT_NODE" <candidate-file>
```

The candidate that contains both the terminal node definition and a nearby reference to the
parent node is the correct one. If still ambiguous, walk one more level up to the grandparent.

---

## Step 3 — Read the terminal node definition block

Use the `Read` tool (not `cat`):
- `file_path`: absolute path to `DEFINITION_FILE`
- `offset`: `DEFINITION_LINE - 1` (Read uses 0-based line offset)
- `limit`: `200`

From the read block, extract:
- The YANG node type (`list`, `container`, `leaf`)
- The list key statement: `key "field1 field2 ...";` — these fields are all mandatory
- Any `uses <grouping-name>;` statements (collect all grouping names for Step 4)
- Any `leaf <name> { ... }` blocks defined directly

If 200 lines is not enough (the block is still open), increase `limit` to `400` and re-read.

---

## Step 4 — Expand grouping references

For each grouping name collected in Step 3 (e.g. `bgp_InterLeakP-group`):

1. Search for its definition:
   ```bash
   grep -rn "grouping bgp_InterLeakP-group" "$YANG_DIR"/
   ```
2. Use the `Read` tool with `offset = line - 1`, `limit = 150` to read the grouping body.
3. Extract all `leaf` statements: name, type, default, mandatory.
4. If the grouping itself contains `uses` statements, follow those one level deeper
   (two levels of nesting is almost always sufficient for NX-OS YANG).

---

## Step 5 — Resolve enum and non-trivial typedef definitions

For every leaf type that is **not** a YANG built-in (`string`, `uint8`, `uint16`, `uint32`,
`uint64`, `int8`, `int16`, `int32`, `int64`, `boolean`, `empty`, `decimal64`), find the
typedef:

```bash
grep -rn "typedef <type-name>" "$YANG_DIR"/
```

Use the `Read` tool with `offset = line - 1`, `limit = 50`. For `enumeration` types,
extract all `enum "<value>";` entries — these are the allowed values. For `union` types,
note each member type and resolve non-built-in members.

---

## Step 6 — Look up specific requested attributes

If `REQUESTED_ATTRS` is non-empty, for each requested attribute name:

1. Check whether it already appears as a `leaf` in the definition block (Step 3) or in any
   expanded grouping (Step 4). If yes, it is captured — flag it in the report.
2. If not found, search the definition file:
   ```bash
   grep -n "leaf <attr-name>" <DEFINITION_FILE>
   ```
3. If still absent, search all YANG files:
   ```bash
   grep -rn "leaf <attr-name>" "$YANG_DIR"/
   ```
4. If not found anywhere near the terminal node, note "not found in this path node".
5. **Key leaf rule**: if the attr is one of the list key fields (from `key "..."`), mark
   it as `mandatory true (list key)` regardless of any `mandatory` statement in the leaf.

---

## Step 7 — Produce the report

Output a structured report in this format:

```
## YANG Exploration: <TERMINAL_NODE>

**Full path**: <path from $ARGUMENTS line 1>
**Node type**: list | container | leaf
**Defined in**: vendor/cisco/nx/10.6-4/<filename>.yang  Line: N
**List key(s)**: proto asn inst    ← omit this row for containers/leaves

---

### Leaves

| Leaf | Type | Default | Mandatory | Notes |
|------|------|---------|-----------|-------|
| proto | bgp-route-proto | — | true (list key) | |
| rtMap | string | "" | optional | |

---

### Enum types

For each enum-typed leaf, list allowed values:

**bgp-route-proto**: `unspecified`, `static`, `direct`, `bgp`, `ospf`, `connected`, ...

---

### Requested attributes    ← omit this section if no attributes were requested

For each attribute in REQUESTED_ATTRS:
- `rtMap`: leaf, type string, default "", defined in grouping bgp_InterLeakP-group
- `scope`: NOT FOUND in this node

---

### Raw YANG — terminal node definition

\```yang
list InterLeakP-list {
  key "proto asn inst";
  uses bgp_InterLeakP-group;
}
\```

### Raw YANG — grouping bgp_InterLeakP-group

\```yang
grouping bgp_InterLeakP-group {
  leaf proto { ... }
  leaf rtMap { ... }
  ...
}
\```
```

Omit any section that has no content. If the terminal node is a `container`, omit the
"List key(s)" row and instead note any nested lists or containers found directly inside it.
