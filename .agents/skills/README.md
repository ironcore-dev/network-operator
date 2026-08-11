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
