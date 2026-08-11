---
name: containerlab
description: Deploy a containerlab network device. Defaults to a topology with Nokia SRL, Juniper vJunos-Evolved, and Cisco N9Kv. Pass a topology file path or image to override.
argument-hint: [--topology <path>] [--image <image>] [--output <path>]
allowed-tools: [Bash, Read, Write, AskUserQuestion]
allowed-bash: ["containerlab *", "docker *", "gnmic *", "curl *"]
---

# containerlab

Deploys a containerlab network device topology.

> **Scope limit:** Only `containerlab` (deploy/inspect/destroy) and `gnmic` (`capabilities` and `get`) may be used. Do not SSH into containerlab nodes, run `docker exec`, modify VM networking or Docker configuration, or make any other changes to the device environment — ask the user first.

## Arguments

Parse `$ARGUMENTS` for:
- `--topology <path>` → use this topology file instead of the default
- `--image <image>` → override the device image in the default topology
- `--output <path>` → write the generated topology file here (default: `/tmp/dev.clab.yml`)

If no topology is provided, use the default topology defined below.

## Deploy network device

If the user provided `--topology <path>`, deploy that file directly:

```bash
containerlab deploy -d -t <path>
```

Otherwise, write the default topology to `--output` (default: `/tmp/dev.clab.yml`), substituting `--image` if provided (default: `ghcr.io/nokia/srlinux:26.7.1`):

```yaml
name: dev-topology

topology:
  nodes:
    vjunos:
      kind: juniper_vjunosevolved
      image: vrnetlab/juniper_vjunosevolved:26.2R1.7-EVO
      ports:
        - 8001:22
        - 57401:57400

    nokia_srl:
      kind: nokia_srlinux
      image: ghcr.io/nokia/srlinux:26.7.1
      startup-config: |-
        system name host-name nokia
        system grpc-server mgmt yang-models openconfig
      ports:
        - 8002:22
        - 57402:57400

    cisco_nxos:
      kind: cisco_n9kv
      image: vrnetlab/cisco_n9kv:9300-10.6.3-lite
      env:
        QEMU_MEMORY: 6144
        QEMU_SMP: 2
      startup-config: |
        hostname cisco_nxos
        feature ospf
        feature openconfig
        feature grpc
        grpc use-vrf management
        no ip domain-lookup
      ports:
        - 8003:22
        - 57403:50051

  links: []
```

Write it to `$OUTPUT_PATH`, then deploy:

```bash
containerlab deploy -d -t $OUTPUT_PATH
```

If the container already exists:

```bash
containerlab deploy -d --reconfigure -t <topology-file>
```

Wait for all nodes to be healthy (poll every 15s, up to 10 minutes) using `docker inspect` — more reliable than parsing `containerlab inspect` table output:

```bash
docker inspect --format '{{.Name}}: {{.State.Health.Status}}' <node1> <node2> ...
```

Repeat until all vrnetlab nodes show `healthy`. Nokia SRL has no health check — it's ready when running. For vrnetlab-based nodes (Juniper, Cisco) this can take 8–15 minutes.

Once healthy, write a gnmic config file at `$OUTPUT_DIR/gnmic-config.yaml` (where `$OUTPUT_DIR` is the directory of `$OUTPUT_PATH`) with all nodes from the topology. Use `127.0.0.1` with the forwarded port for each node. For Nokia SRL use `skip-verify: true`; for vrnetlab nodes omit it (plain gRPC).

Default credentials, ports and TLS settings per node kind:
- Nokia SRL: `admin / NokiaSrl1!`, forwarded port, `skip-verify: true`
- Juniper vJunosEvolved: `admin / admin@123`, forwarded port, `insecure: true`  (if the image has TLS configured, use `skip-verify: true` instead)
- Cisco N9Kv: `admin / admin`, forwarded port, `skip-verify: true`

Example for a topology with all three:

```yaml
timeout: 10s

targets:
  juniper:
    address: 127.0.0.1:57401
    username: admin
    password: admin@123
    insecure: true

  srl:
    address: 127.0.0.1:57402
    username: admin
    password: NokiaSrl1!
    skip-verify: true

  nxos:
    address: 127.0.0.1:57403
    username: admin
    password: admin
    skip-verify: true
```

Then validate all nodes at once:

```bash
gnmic --config $OUTPUT_DIR/gnmic-config.yaml capabilities
```

Show the supported encodings and YANG model count per node. If a node fails, show the error and note it may still be booting.

## Summary

Once all nodes are healthy, show the full topology state:

```bash
containerlab inspect -a
```

Print a final summary:
- Containerlab version
- For each node: name, kind, management IP, gNMI endpoint, health state, gnmic capabilities result (encodings supported, YANG model count)
- Next step: run `/netop` to test device

## References

- **containerlab**: https://containerlab.dev/cmd/
- **Nokia SRL containerlab kind**: https://containerlab.dev/manual/kinds/nokia_srlinux/
- **Juniper vJunos-Evolved containerlab kind**: https://containerlab.dev/manual/kinds/vjunos-evolved/
- **Cisco NX-OS containerlab kind**: https://containerlab.dev/manual/kinds/cisco_nxos/
