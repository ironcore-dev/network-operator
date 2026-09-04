---
name: netop-setup
description: One-time setup of the network-operator test environment. Provisions a colima VM, creates a kind cluster with cert-manager, and deploys a containerlab network device. Use this before the first test session or after a full teardown. Say "no vm" or "skip vm" to skip the VM provisioning step.
argument-hint: [no-vm | skip-vm]
allowed-tools: [Bash, Read, Write, AskUserQuestion]
---

# netop-setup

Sets up the full test environment for network-operator from scratch:
1. Setup VM (profile: `network-operator`)
2. Install tools in VM
3. Kind cluster + cert-manager
4. Containerlab network device

> **No-VM shortcut:** pass `no-vm`, `skip vm`, `without vm`, or `run without VM` to skip Steps 1 and 2.

## Environment

At the start of the session, ask the user which provider they plan to test (if not already known from `$ARGUMENTS`):
- **openconfig / Nokia SRL** → `PROVIDER=openconfig`
- **cisco** → `PROVIDER=cisco`

Also ask which VM tool they are using (default: colima):
- **colima** → `VM_EXEC="colima exec -p network-operator --"`
- **multipass** → `VM_EXEC="multipass exec network-operator --"`

These variables are used in every command below:

```
PROVIDER=openconfig      # or cisco
VM_EXEC="colima exec -p network-operator --"   # or multipass exec network-operator --
```

`LOCALBIN` is set persistently in the VM's `~/.bashrc` during Step 2 — no need to prefix it on any `make` command.

> **No-VM case:** `LOCALBIN` is not set. The Makefile default (`./bin`) applies automatically.

The VM wrapper for all Step 2+ commands is:
```bash
$VM_EXEC bash -c "<command>"
```

> The host home directory is mounted at the same path inside the VM — commands run from the same directory as on the host, so no `cd` is needed.

> All commands in Steps 2, 3, and 4 use this wrapper. It is not repeated in each step — just substitute `<command>` with the bare command shown.

## Step 1: Setup VM

> **Skip Steps 1 and 2** if the user passes `no-vm` or any similar phrasing.

Check the state of the `network-operator` colima profile:

```bash
colima list
```

- **Running** → check specs match defaults (4 CPU, 8 GB, 60 GB disk). If they differ, warn the user and ask if they want to recreate:
  ```bash
  colima delete -p network-operator
  # then create as below
  ```
- **Stopped** → start it:
  ```bash
  colima start --profile network-operator
  ```
- **Not listed** → create it:
  ```bash
  colima start --cpu 4 --memory 8 --disk 60 --network-address --profile network-operator
  ```

Verify after start:
```bash
colima list
```

Ensure `~/.local/bin` exists, is on PATH, and `LOCALBIN` is exported in the VM:

```bash
mkdir -p ~/.local/bin
grep -qxF 'export LOCALBIN="$HOME/.local/bin"' ~/.bashrc || echo 'export LOCALBIN="$HOME/.local/bin"' >> ~/.bashrc
grep -qxF 'export PATH="$LOCALBIN:$PATH"'      ~/.bashrc || echo 'export PATH="$LOCALBIN:$PATH"'      >> ~/.bashrc
export LOCALBIN="$HOME/.local/bin"
export PATH="$LOCALBIN:$PATH"
```

## Step 2: Install tools in VM

> **Skip this step** if the user is running without a VM.

Install required tools if not already present:

```bash
sudo apt-get update -qq
sudo apt-get install -y make curl jq vim snapd
which yq   || sudo snap install yq
which go   || sudo snap install go --classic
which kubectl || sudo snap install kubectl --classic
which k    || sudo snap alias kubectl k
which gnmic || bash -c "$(curl -sL https://get-gnmic.openconfig.net)"
```

## Step 3: Kind cluster + cert-manager

```bash
make kind
make kind-create
```

Wait for node ready:
```bash
kubectl wait --for=condition=Ready node --all --timeout=120s
```

Install cert-manager:
```bash
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.18.2/cert-manager.yaml
kubectl wait --for=condition=Available deployment --all -n cert-manager --timeout=120s
```

Verify:
```bash
kubectl get nodes
kubectl get pods -n cert-manager
```

## Step 4: Containerlab device

Ask the user which device type to use:

**Option A — Nokia SRL (default, arm64-compatible)**
**Option B — Remote Cisco device (team cloud via SSH port forwarding)**

### Option A: Nokia SRL

Check if containerlab is installed:
```bash
containerlab version 2>/dev/null || bash -c "$(curl -sL https://get.containerlab.dev)"
```

Write the topology file to `/tmp/srl01.clab.yml`:

```yaml
name: srlceos01

topology:
  nodes:
    srl:
      kind: nokia_srlinux
      image: ghcr.io/nokia/srlinux:26.7.1
      startup-config: |-
        system name host-name srl
        system grpc-server mgmt yang-models openconfig
      ports:
        - 57022:22
        - 57400:57400

  links:
    - endpoints: ["srl:ethernet-1/1", "srl:ethernet-1/2"]
```

Deploy:
```bash
containerlab deploy -d -t /tmp/srl01.clab.yml
```

If the container already exists or the user wants to reconfigure:
```bash
containerlab deploy -d --reconfigure -t /tmp/srl01.clab.yml
```

Wait until running:
```bash
docker inspect -f '{{.State.Status}}' clab-srlceos01-srl
```

Show device IP:
```bash
containerlab inspect -t /tmp/srl01.clab.yml
```

The Nokia SRL management IP is typically `172.20.20.2` — confirm and note it for `/netop-test`.

### Option B: Remote Cisco device

> **Note:** Local Cisco N9Kv deployment is not possible on Apple Silicon — nested virtualization required for QEMU x86 emulation is not supported. Use a remote device instead (direct access or via SSH port forwarding — that's the user's responsibility).

Ask the user for the device connection details:
- `CISCO_IP` — IP address reachable from the VM (e.g. `10.0.0.5` or `127.0.0.1` if port-forwarded)
- `CISCO_PORT` — gNMI port (default: `57400`)
- `CISCO_USER` — gNMI username (default: `admin`)
- `CISCO_PASSWORD` — gNMI password

Verify connectivity from inside the VM:
```bash
nc -z $CISCO_IP $CISCO_PORT && echo "device reachable" || echo "device not reachable — check IP, port, and any required port forwarding"
```

> **Localhost warning:** If the user provides `127.0.0.1` or `localhost` as `CISCO_IP`, warn them that this refers to the VM itself, not the Mac host. Detect the Mac host IP as seen from the VM (its default gateway) and use that instead:
> ```bash
> HOST_IP=$(ip route | awk '/default/ {print $3}')
> echo "Use $HOST_IP instead of 127.0.0.1"
> ```
> Update `CISCO_IP` to `$HOST_IP` before proceeding.

Note these values for `/netop-test`:
```
GNMI_TARGET=$CISCO_IP:$CISCO_PORT
GNMI_USER=$CISCO_USER
GNMI_PASSWORD=$CISCO_PASSWORD
```

## Summary

Run the following to show the full state of the dev environment:

```bash
# Docker version
docker --version

# Kubernetes cluster version and nodes
kubectl version
kubectl get nodes

# All pods (wait until ready)
kubectl wait --for=condition=Ready pod --all -A --timeout=120s && kubectl get pods -A

# Containerlab device status
containerlab inspect -a
```

Print a final summary:
- Colima profile: `network-operator` (CPU, memory, disk)
- Docker version
- Kind cluster: Kubernetes version, node count
- cert-manager: all deployments available
- Network device: name, kind, IP/endpoint
- If `PROVIDER=cisco`: `GNMI_TARGET`, `GNMI_USER`, `GNMI_PASSWORD` confirmed and device reachable from VM
- Next step: run `/netop-test` to build and deploy the operator

## References

- [colima](https://github.com/abiosoft/colima) — container runtimes on macOS with minimal setup
- [kind](https://kind.sigs.k8s.io/docs/user/quick-start/) — Kubernetes in Docker
- [kubectl](https://kubernetes.io/docs/reference/kubectl/) — Kubernetes CLI reference
- [cert-manager](https://cert-manager.io/docs/) — X.509 certificate management for Kubernetes
- [containerlab](https://containerlab.dev/cmd/) — network topology emulation (CLI reference)
- [Nokia SRL containerlab kind](https://containerlab.dev/manual/kinds/nokia_srlinux/) — Nokia SR Linux node configuration
- [gnmic](https://gnmic.openconfig.net) — gNMI CLI client
