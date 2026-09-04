<!--
# SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
# SPDX-License-Identifier: Apache-2.0
-->

# network-operator

[![REUSE status](https://api.reuse.software/badge/github.com/ironcore-dev/network-operator)](https://api.reuse.software/info/github.com/ironcore-dev/network-operator)
[![Go Report Card](https://goreportcard.com/badge/github.com/ironcore-dev/network-operator)](https://goreportcard.com/report/github.com/ironcore-dev/network-operator)
[![GitHub License](https://img.shields.io/static/v1?label=License&message=Apache-2.0&color=blue)](LICENSE)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](https://makeapullrequest.com)

`network-operator` is a Kubernetes operator for automating network device provisioning.

## Description

Network-operator is a project built using Kubebuilder and controller-runtime to facilitate the provisioning of network devices. It provides a robust and scalable solution for managing networking infrastructure, ensuring seamless integration and automation within Kubernetes environments.

## Getting Started

### Prerequisites

- go version v1.26.0+
- docker version 28+.
- kubectl version v1.33.1+.
- Access to a Kubernetes v1.33.0+ cluster.
- [Git LFS](https://git-lfs.com) installed (`git lfs install`)
- kind version 0.32.0+
- Tilt version v0.37.3+
- gh version 2.93.0+
- coreutils 9.11+

### To Deploy on the cluster

**Build your image to the tag specified by `IMG`:**

```sh
make docker-build IMG=<some-registry>/network-operator:tag
```

**NOTE:** This image ought to be published in the personal registry you specified. And it is required to have access to pull the image from the working environment. Make sure you have the proper permission to the registry if the above commands don’t work.

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/network-operator:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

> **NOTE**: Ensure that the samples have default values to test it out.

### To Uninstall

**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## kubectl Plugin

The `kubectl-net` plugin extends kubectl with shorthand flags and resource lifecycle operations tailored to network-operator.

```bash
kubectl net get interfaces --device leaf1
kubectl net pause devices leaf1 --recursive
```

See [kubectl-net/README.md](kubectl-net/README.md) for installation and full usage documentation.

## Project Distribution

Following are the steps to build the installer and distribute this project to users.

1. Build the installer for the image built and published in the registry:

```sh
make build-installer IMG=<some-registry>/network-operator:tag
```

> NOTE: The makefile target mentioned above generates an 'install.yaml' file in the dist directory. This file contains all the resources built with Kustomize, which are necessary to install this project without its dependencies.

2. Using the installer

Users can just run kubectl apply -f <URL for YAML BUNDLE> to install the project, i.e.:

```sh
kubectl apply -f https://raw.githubusercontent.com/<org>/network-operator/<tag or branch>/dist/install.yaml
```

## Claude Code Skills

This project includes [Claude Code](https://claude.ai/code) skills for interactive development workflows. Skills are located in `.claude/skills/` and invoked via slash commands.

### `/netop-setup`

Set up the full test environment (colima VM, kind cluster, cert-manager, containerlab device):

```
/netop-setup
```

Say `no vm` or `skip vm` to skip VM provisioning if it's already running. Supports colima and multipass.

**Example — Nokia SRL:**

```
/netop-setup
```

```
  Colima VM network-operator — arm64, 4 CPU, 8 GB, 60 GB, running
  Kind cluster network-operator — Kubernetes v1.36.1, node Ready
  cert-manager v1.18.2 — all deployments available
  Nokia SRL — clab-srlceos01-srl running at 172.20.20.2

  Next step: run /netop-test.
```

**Example — Cisco device accessible on `127.0.0.1:57400`:**

```
/netop-setup cisco device accessible on 127.0.0.1:57400 with user admin and password admin
```

```
  Colima VM network-operator — arm64, 4 CPU, 8 GB, 60 GB, running
  Kind cluster network-operator — Kubernetes v1.36.1, node Ready
  cert-manager v1.18.2 — all deployments available
  Cisco device — reachable from VM at 192.168.5.2:57400 (Mac host IP as seen from VM — use this instead of 127.0.0.1)

  Next step: run /netop-test with GNMI_TARGET=192.168.5.2:57400.
```

### `/netop-test`

Build, deploy and test the operator against a real containerlab device:

```
/netop-test
```

You can pass CR files and expected results directly:

```
/netop-test @config/samples/v1alpha1_banner.yaml
/netop-test @test/gnmi/testdata/openconfig/banner.txt
/netop-test @my-cr.yaml @my-expected.json
```

- **`config/samples/`** — ready-made sample CRs for all supported resource types
- **`test/gnmi/testdata/`** — testdata files containing both the CR YAML and expected gnmic state in one file (parsed automatically)
- **Custom files** — pass any CR YAML and optionally a JSON file with the expected gnmic state

You can also ask the skill to inspect the environment or show device capabilities in natural language:

```
/netop-test show me the device topology
/netop-test show me device capabilities
/netop-test show me device configuration openconfig-system:system/dns
```

- `show me the device topology` → runs `containerlab inspect -a` to list all running labs and their node IPs
- `show me device capabilities` → runs `gnmic capabilities` to show supported YANG models, encodings, and gNMI version
- `show me device configuration <xpath>` → runs `gnmic get --path <xpath>` and prints the full JSON response

**Example output:**

```
  Test Report

  ┌──────────────┬──────────────┬───────────┬───────┬──────────────────────────────────────────────┬─────────────────┐
  │   CR Name    │     Kind     │ Namespace │ Ready │                  gNMI Path                   │     Result      │
  ├──────────────┼──────────────┼───────────┼───────┼──────────────────────────────────────────────┼─────────────────┤
  │ banner       │ Banner       │ default   │ True  │ openconfig-system:system/config/login-banner  │ ✓ value matches │
  └──────────────┴──────────────┴───────────┴───────┴──────────────────────────────────────────────┴─────────────────┘

  gnmic get openconfig-system:system/config/login-banner
  ──────────────────────────────────────────────────────
  {
    "openconfig-system:system": {
      "config": {
        "login-banner": "###################################################\n#   WARNING: Unauthorized access is prohibited.   #\n###################################################\n"
      }
    }
  }
```

### `/netop-check`

Run local checks before committing or opening a PR:

```
/netop-check
```

Runs `go vet`, `golangci-lint`, unit tests (`make test`), and gNMI integration tests (`make test-gnmi`). If lint fails, offers to run `make fmt` or `make lint-fix` to auto-fix issues. Prints a summary report at the end.

**Example output:**

```
  Local Dev Report
  ────────────────────────────────────────────────────────
  Vet:         ✓ passed
  Lint:        ✓ passed
  Unit tests:  ✓ 15 packages passed, 0 failed
  gNMI tests:  ✓ N passed, 0 failed
  ────────────────────────────────────────────────────────
  Overall:     ✓ all checks passed
```

## Support, Feedback, Contributing

This project is open to feature requests/suggestions, bug reports etc. via [GitHub issues](https://github.com/ironcore-dev/network-operator/issues). Contribution and feedback are encouraged and always welcome. For more information about how to contribute, the project structure, as well as additional contribution information, see our [Contribution Guidelines](CONTRIBUTING.md).

## Security / Disclosure

If you find any bug that may be a security problem, please follow our instructions at [in our security policy](https://github.com/ironcore-dev/network-operator/security/policy) on how to report it. Please do not create GitHub issues for security-related doubts or problems.

## Code of Conduct

We as members, contributors, and leaders pledge to make participation in our community a harassment-free experience for everyone. By participating in this project, you agree to abide by its [Code of Conduct](https://github.com/SAP/.github/blob/main/CODE_OF_CONDUCT.md) at all times.

## Licensing

Copyright 2025 SAP SE or an SAP affiliate company and IronCore contributors. Please see our [LICENSE](LICENSE) for copyright and license information. Detailed information including third-party components and their licensing/copyright information is available [via the REUSE tool](https://api.reuse.software/info/github.com/ironcore-dev/network-operator).

<p align="center"><img alt="Bundesministerium für Wirtschaft und Energie (BMWE)-EU funding logo" src="https://apeirora.eu/assets/img/BMWK-EU.png" width="400"/></p>
