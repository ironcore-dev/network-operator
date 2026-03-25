<!--
# SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
# SPDX-License-Identifier: Apache-2.0
-->

# kubectl-net

A [kubectl plugin](https://kubernetes.io/docs/tasks/extend-kubectl/kubectl-plugins/) for managing [network-operator](https://github.com/ironcore-dev/network-operator) custom resources.

## Installation

Install via `go install`:

```bash
go install github.com/ironcore-dev/network-operator/kubectl-net/cmd@latest
```

> Ensure your `$GOBIN` (or default `$GOPATH/bin`) is on your `$PATH`.

Verify kubectl discovers the plugin:

```bash
kubectl plugin list | grep kubectl-net
```

You can now run the plugin as `kubectl net`.

<details>
<summary>Build from source</summary>

```bash
go build -o kubectl-net ./cmd/kubectl-net.go
sudo install -m 0755 kubectl-net /usr/local/bin/kubectl-net
kubectl plugin list | grep kubectl-net
```

</details>

## Usage

![demo](examples/demo.gif)

### Get resources

The `get` subcommand works like `kubectl get` and supports the same output flags (`-o yaml`, `-o json`, `-o wide`, `-o name`, `-o jsonpath=...`, etc.).

```bash
# List all devices (server-side table output, same as kubectl get)
kubectl net get devices

# Get a single device
kubectl net get devices leaf1

# Filter interfaces by device
kubectl net get interfaces --device leaf1

# Filter interfaces by VRF
kubectl net get interfaces --vrf default

# Filter interfaces by aggregate
kubectl net get interfaces --aggregate ae0

# Filter VLANs by EVPN instance
kubectl net get vlans --evi evi-100

# Filter VLANs by routed VLAN interface
kubectl net get vlans --routed-vlan irb100

# Output as YAML
kubectl net get devices leaf1 -o yaml
```

### Pause and unpause resources

The `pause` and `unpause` subcommands set or remove the `networking.metal.ironcore.dev/paused` annotation. For devices, `--recursive` also sets `spec.paused` to propagate to child resources.

```bash
# Pause a single interface
kubectl net pause interfaces lo0

# Unpause interface
kubectl net unpause interfaces lo0

# Pause a device and all child resources
kubectl net pause devices leaf1 --recursive

# Unpause a device and all child resources
kubectl net unpause devices leaf1 --recursive

# Pause all interfaces on a device
kubectl net pause interfaces --device leaf1
```

### Shell completion

```bash
# Generate and source completion for your shell
source <(kubectl net completion bash)
source <(kubectl net completion zsh)
kubectl net completion fish | source
```

## Label shorthand flags

| Flag             | Label                                       | Available for |
| ---------------- | ------------------------------------------- | ------------- |
| `--device`, `-d` | `networking.metal.ironcore.dev/device-name` | All resources |
| `--aggregate`    | `networking.metal.ironcore.dev/aggregate`   | Interfaces    |
| `--vrf`          | `networking.metal.ironcore.dev/vrf`         | Interfaces    |
| `--routed-vlan`  | `networking.metal.ironcore.dev/routed-vlan` | VLANs         |
| `--evi`          | `networking.metal.ironcore.dev/l2vni`       | VLANs         |
