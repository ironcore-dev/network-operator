# Claude Code Skills

This project includes [Claude Code](https://claude.ai/code) skills for interactive development workflows.
Skills live in `.agents/skills/` (symlinked from `.claude/skills/`) and are invoked via slash commands.

## `/containerlab`

Deploy a multi-vendor containerlab topology for local testing:

```
/containerlab
/containerlab --topology config/samples/my-topology.clab.yml
/containerlab --image ghcr.io/nokia/srlinux:25.3.2
/containerlab --output ~/labs/dev.clab.yml
```

Defaults to a topology with Nokia SRL, Juniper vJunos-Evolved, and Cisco N9Kv.
Waits for all nodes to become healthy, writes a gnmic config, and validates
capabilities on all nodes. Prints a summary with gNMI endpoints on completion.

## `/netop`

Run local checks and/or manual integration tests against real devices:

```
/netop               # run both phases
/netop check         # local checks only (no cluster needed)
/netop test          # integration test only
/netop test --device 172.20.20.2:57400 --cr config/samples/openconfig_v1alpha1_banner.yaml
```

**Phase 1 — check:** runs `make vet`, `make lint`, `make fmt`, `make test`, `make test-gnmi`
and prints a structured report. See AGENTS.md for commands, rules, and output format.

**Phase 2 — test:** deploys the operator to a local kind cluster, applies CRs,
and validates device state via gnmic. Installs cert-manager and builds/loads the
controller image if needed.

## `/cisco-yang-explore`

Navigate a Cisco NX-OS 10.6-4 YANG path and explain its structure, leaves, and enum values:

```
/cisco-yang-explore System/bgp-items/inst-items/dom-items/Dom-list[name={vrf}]/af-items/DomAf-list[type=ipv4-ucast]/interleak-items/InterLeakP-list[proto=static]
```

Locates the terminal node in the vendor YANG model, follows `uses`/grouping references,
resolves enum typedefs, and produces a structured report of the node's type, keys, leaves,
and raw definition block. Optionally pass a second line `Attributes: attr1, attr2, ...` to
look up specific leaves.

The models come from the `YangModels/yang` repo. On first use the skill asks where to clone
them (default `/tmp/cisco-yang-explore`) and does a sparse, shallow clone of only the
`vendor/cisco/nx/10.6-4` subdirectory to avoid the 4GB+ full checkout.
