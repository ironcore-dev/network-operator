---
name: netop-test
description: Build and deploy the network-operator, apply custom resources, and validate configuration via gnmic. Use after /netop-setup to run the dev/test loop against a real containerlab device. Also handles kind cluster and VM cleanup. Say "no vm" or "local" to run commands on the host machine instead.
argument-hint: [<cr-file.yaml> [expected-result.json] | no-vm | local]
allowed-tools: [Bash, Read, Write, AskUserQuestion]
---

# netop-test

Runs the network-operator dev/test loop:
1. Build Docker image and load into kind
2. Deploy (or redeploy) the operator
3. Apply custom resources
4. Validate with gnmic
5. Test report
6. Cleanup (optional)

Prerequisites: `/netop-setup` has been run — VM, kind cluster, and containerlab device are all running.

> **No-VM shortcut:** pass `no-vm`, `local`, or any similar phrasing to run all commands directly on the host machine instead of inside the VM.

## Arguments

The user can pass optional arguments via `$ARGUMENTS`:

- **CR file** (`@my-sample.yaml`) — a custom CR YAML to apply instead of picking from `config/samples/`. Read the file, apply it directly.
- **Expected result** (`@my-expected-result.json`) — a JSON file with the expected gnmic state after reconciliation. Use it to compare against the actual `gnmic get` response in Step 4.
- **`no-vm` / `local`** — run all commands on the host machine instead of inside the VM.

Examples:
```
/netop-test
/netop-test no-vm
/netop-test @config/samples/v1alpha1_banner.yaml
/netop-test @config/samples/v1alpha1_banner.yaml @test/gnmi/testdata/openconfig/banner.txt
```

If an expected result file is provided, use it as the ground truth in Step 5 instead of inferring the expected value from the CR spec.

### Parsing testdata files (`test/gnmi/testdata/`)

If the user passes a file from `test/gnmi/testdata/` (e.g. `@test/gnmi/testdata/openconfig/banner.txt`), parse it as follows:

```
# <description comment>
-- <kind>/<name> --
<CR YAML to apply>
-- state --
<expected JSON state after reconciliation>
```

- Everything between `-- <kind>/<name> --` and `-- state --` is the CR YAML → apply it in Step 3
- Everything after `-- state --` is the expected gnmic state JSON → use it as the expected value in Step 5
- The `deviceRef.name` in the CR YAML refers to `device` by default — replace it with the actual device name (`leaf1`) before applying

Example (`banner.txt`):
```
# Banner PreLogin
-- banners/banner --
apiVersion: networking.metal.ironcore.dev/v1alpha1
kind: Banner
...
-- state --
{
  "openconfig-system:system": {
    "config": {
      "login-banner": "Unauthorized access is prohibited."
    }
  }
}
```

## Environment

```
VM_EXEC="colima exec -p network-operator --"   # or: multipass exec network-operator --
                                                # or: empty ("") to run locally
```

All commands below are shown as bare commands. Wrap them with `$VM_EXEC bash -c "<command>"` when running in the VM, or run them directly on the host when `no-vm` / `local` is passed.

> **Why no `LOCALBIN` prefix?** `LOCALBIN` is set in the VM's `~/.bashrc` during `/netop-setup` — all `make` calls pick it up automatically, and tools like `kind` and `kustomize` are on `PATH`. When running locally, the Makefile default (`./bin`) applies.

## Step 1: Build & load image

Build the operator image:
```bash
make docker-build IMG=ghcr.io/ironcore-dev/network-operator:latest
```

Load the image into the kind cluster:
```bash
kind load docker-image ghcr.io/ironcore-dev/network-operator:latest --name network-operator
```

## Step 2: Deploy the operator

Ask the user: **fresh deploy or redeploy?**

**Fresh deploy** — operator not yet running in the cluster:

Ensure kustomize is installed and set the image on the fly (without modifying tracked files):
```bash
make kustomize
cd config/develop && kustomize edit set image controller=ghcr.io/ironcore-dev/network-operator:latest && kustomize build . | kubectl apply -f - && git checkout kustomization.yaml
```

> `git checkout kustomization.yaml` reverts the image edit so the file stays clean.
> `config/develop/manager_patch.yaml` sets `--provider=openconfig` — read it first to confirm the provider is correct for the current session.

**Redeploy** — operator already running, restart with the new image:
```bash
kubectl rollout restart deployment/network-operator-controller-manager -n network-operator-system
kubectl rollout status deployment/network-operator-controller-manager -n network-operator-system --timeout=60s
```

Check manager logs for startup errors:
```bash
kubectl logs -n network-operator-system -l control-plane=controller-manager --tail=30
```

## Step 3: Apply custom resources

Ask the user: **use samples from `config/samples/` or provide a custom YAML path?**

The `Device` resource must be applied first — other resources depend on it. Copy to a temp file, patch address and credentials for the Nokia SRL device, then apply:
```bash
cp config/samples/v1alpha1_device.yaml /tmp/device.yaml
sed -i 's|address: .*|address: 172.20.20.2:57400|' /tmp/device.yaml
sed -i 's|password: .*|password: NokiaSrl1!|' /tmp/device.yaml
kubectl apply -f /tmp/device.yaml
```

If the user is using a different device, ask for the correct address and credentials before patching.

Verify the device is reconciled:
```bash
kubectl get device -A
```

Then apply additional resources:
```bash
kubectl apply -f config/samples/<resource>.yaml
```

After applying any CR, check reconciliation status:
```bash
kubectl get <kind> -A
```

Look for `READY=True`. If not ready, check operator logs:
```bash
kubectl logs -n network-operator-system -l control-plane=controller-manager --tail=50
```

### Generic CRD test pattern

For any CR you want to test:

1. **Find the sample** in `config/samples/` (e.g. `v1alpha1_banner.yaml`)
2. **Check the CR references the correct device** — `deviceRef.name: leaf1` or label `networking.metal.ironcore.dev/device-name: leaf1`
3. **Apply it:** `kubectl apply -f config/samples/v1alpha1_<name>.yaml`
4. **Verify reconciliation:** `kubectl get <Kind> -A` — expect `READY=True`
5. **Find the gNMI path** — open `internal/provider/openconfig/<resource>.go` and look for the `XPath()` method
   - Banner (PreLogin): `openconfig-system:system/config/login-banner`
   - DNS: `openconfig-system:system/dns`
6. **Validate with gnmic** (see Step 4)

## Step 4: Validate with gnmic

For each CR tested, run a gnmic get using the XPath from the provider source:
```bash
gnmic -a 172.20.20.2 --port 57400 -u admin -p 'NokiaSrl1!' --skip-verify --encoding JSON_IETF get --path '<xpath>'
```

Ask the user if they want to query a different path or device. Substitute accordingly.

### Optional: Show device capabilities

If the user asks to see what the device supports, run a gnmic capabilities request:
```bash
gnmic -a 172.20.20.2 --port 57400 -u admin -p 'NokiaSrl1!' --skip-verify capabilities
```

This returns the supported YANG models, encodings, and gNMI version — useful for confirming which OpenConfig paths are available on the device before testing.

### Optional: Query device configuration

If the user asks to see a specific part of the device configuration, run a gnmic get with the path they provide:
```bash
gnmic -a 172.20.20.2 --port 57400 -u admin -p 'NokiaSrl1!' --skip-verify --encoding JSON_IETF get --path '<xpath>'
```

Examples the user might ask:
- `show me device configuration openconfig-system:system/dns`
- `show me device configuration openconfig-interfaces:interfaces`
- `get openconfig-system:system/config/login-banner`

Always print the full JSON response without truncation.

If the user asks to see the running lab topology:
```bash
containerlab inspect -a
```

This lists all running containerlab labs, node names, kinds, images, states, and management IP addresses.

## Step 5: Test report

After all CRs have been applied and validated, print a structured test report.

For each CR tested show:
1. **Applied YAML:** `kubectl get <kind> <name> -n <namespace> -o yaml`
2. **gnmic validation:** exact command and full JSON response (always shown regardless of whether an expected file was provided)
3. **Operator logs:** `kubectl logs -n network-operator-system -l control-plane=controller-manager --tail=100 | grep -i '<kind>\|error\|warn'`

**Validation logic:**
- **Expected file provided** (`-- state --` section or JSON file) → compare gnmic response against it field by field
- **No expected file** → infer expected values from the CR spec fields (e.g. `spec.message.inline` for Banner) and validate those fields in the response
- **Always** print the full raw gnmic JSON response regardless — never truncate it

End with a box-drawing summary table:

```
  Test Report

  ┌──────────────┬──────────────┬───────────┬───────┬─────────────────────────────────────────┬─────────────────┐
  │   CR Name    │     Kind     │ Namespace │ Ready │                gNMI Path                │     Result      │
  ├──────────────┼──────────────┼───────────┼───────┼─────────────────────────────────────────┼─────────────────┤
  │ banner       │ Banner       │ default   │ True  │ openconfig-system:system/config/...      │ ✓ value matches │
  └──────────────┴──────────────┴───────────┴───────┴─────────────────────────────────────────┴─────────────────┘
```

If a CR maps to multiple gNMI paths (e.g. ManagementAccess has gRPC and SSH), add one row per path.

Mark result as:
- `✓ value matches` — gnmic response matches expected/inferred value
- `✗ mismatch` — differs (show diff inline below table)
- `✗ not found` — gnmic returned empty or error

Below the table, always show the full gnmic JSON response for each row:
```
  gnmic get openconfig-system:system/config/login-banner
  ──────────────────────────────────────────────────────
  {
    "openconfig-system:system": {
      "config": {
        "login-banner": "###################################################\n#   WARNING: ..."
      }
    }
  }
```

## Step 6: Cleanup

**Always ask before any destructive action.**

Delete the kind cluster:
```bash
kind delete cluster --name network-operator
```

Stop the VM (keeps data):
```bash
colima stop --profile network-operator
```

Delete the VM (destroys all data — only if user explicitly confirms):
```bash
colima delete -p network-operator
```

## References

- [kustomize](https://kubectl.docs.kubernetes.io/references/kustomize/) — Kubernetes configuration management
- [kind](https://kind.sigs.k8s.io/docs/user/quick-start/) — Kubernetes in Docker
- [kubectl](https://kubernetes.io/docs/reference/kubectl/) — Kubernetes CLI reference
- [gnmic](https://gnmic.openconfig.net) — gNMI CLI client
- [OpenConfig YANG  doc](https://openconfig.net/projects/models/schemadocs/) — OpenConfig YANG schemas
