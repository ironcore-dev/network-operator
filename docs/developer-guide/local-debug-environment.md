# Local Debug Environment

The Tilt setup can scale the in-cluster controller to zero replicas so you can run the manager locally in a debugger, attached to the same Kind cluster. This lets you set breakpoints and step through reconciliation while all other resources (CRDs, cert-manager, Prometheus, sample CRs) still live in the cluster.

## Prerequisites

- [Kind](https://kind.sigs.k8s.io/), [Tilt](https://tilt.dev/), and `kubectl` installed
- An IDE with a Go debugger (the repo ships a VS Code `launch.json`)

## Starting Tilt in debug mode

```bash
make tilt-debug-up
```

This sets `DEBUG=true` and runs `make tilt-up`, which creates the Kind cluster if needed. In debug mode the `Tiltfile`:

- Scales the `network-operator-controller-manager` Deployment to `replicas: 0`, freeing the cluster to be reconciled by your locally running manager instead.
- Applies the `Device` sample verbatim (skipping the `host.docker.internal` address rewrite used by the normal in-cluster flow).
- Update the `config/samples/device_v1alpha1_device.yaml` file to point to a real device on your network to test against a real device.

Everything else — CRDs, cert-manager, Prometheus, and the manually triggered sample resources — is deployed exactly as in a normal `make tilt-up` run.

## Launching the debugger

With Tilt running, start the manager from your IDE. In VS Code, add a launch configuration to `.vscode/launch.json`:

```json
{
  "name": "Kind Debug",
  "type": "go",
  "request": "launch",
  "mode": "debug",
  "program": "${workspaceFolder}/cmd",
  "env": {
    "KUBECONFIG": "${workspaceFolder}/.vscode/network-kind.kubeconfig",
    "ENABLE_WEBHOOKS": "false"
  },
  "args": [
    "--max-concurrent-reconciles",
    "10",
    "--provider",
    "openconfig",
    "--requeue-interval",
    "30s"
  ]
}
```

Key settings:

- `KUBECONFIG` points at the Kind cluster (export the kubeconfig with `kind get kubeconfig --name network-operator > .vscode/network-kind.kubeconfig`).
- `ENABLE_WEBHOOKS=false` — webhooks require the in-cluster certificate setup, so they are disabled when debugging locally.
- `--provider`, `--max-concurrent-reconciles`, and `--requeue-interval` match the flags the in-cluster manager runs with. Adjust `--provider` to the platform you are targeting.

Pick this configuration in the **Run and Debug** view and start it. Set breakpoints in the controller or provider code and trigger a sample resource in the Tilt UI to hit them.
