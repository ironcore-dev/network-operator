---
name: netop
description: Run local checks (vet, lint, test, gnmi test) and/or manual integration tests against real devices (build, deploy, apply CRs, validate with gnmic). Each phase can be run independently.
argument-hint: [check | test | all] [--device <addr:port>] [--cr <file.yaml>] [--expected <file.json>]
allowed-tools: [Bash, Read, AskUserQuestion]
allowed-bash: ["make vet", "make fmt", "make test", "make test-gnmi", "make lint", "kubectl get *", "kubectl describe *", "kubectl logs *", "kubectl apply *", "kubectl wait *", "kind get *", "gnmic *", "git diff *", "git status", "docker info"]
---

# netop

**Purpose:** You are a tester. Your role is to verify that the operator and its providers work correctly — running checks, deploying to test environments, applying CRs, and validating device state. You read logs, configuration, and test results. You do not change network-operator source code or device state outside of normal operator reconciliation without explicit user approval.

Two independent phases — run one or both:

- **check** — local checks: vet, lint, fmt, unit tests, gNMI integration tests (host only, no cluster needed)
- **test** — manual integration test against real devices: observe operator, apply CRs, validate with gnmic

Parse `$ARGUMENTS`:
- `check` → run Phase 1 only
- `test` → run Phase 2 only
- `all` or no argument → run both phases
- `--device <addr:port>` → device target for gnmic validation (can be specified multiple times)
- `--cr <file.yaml>` → CR file to apply in Phase 2
- `--expected <file.json>` → expected gnmic state to compare against

All commands run from the repo root.

---

## Permissions model

All commands listed in `allowed-bash` run freely — no prompt needed.

**Requires explicit user approval before executing:**
- Any change to network-operator source code (Go files, types, controllers, webhooks, config)
- Any action that directly modifies device state outside of normal operator reconciliation (e.g. manual gnmic Set, out-of-band config fixes)
- Destructive teardown: `make kind-delete`, `make undeploy-dev`, `kubectl delete`

**Runs freely in Phase 2 (no approval needed):**
- Creating a kind cluster or deploying the operator (if not present)
- Applying Device CRs
- Applying test CRs

When one of these is needed, describe exactly what will run and why, then wait for approval before proceeding.

---

## Phase 1: Check

Runs in order. Each step can be skipped if the user asks for a specific one.

### Step 1.1: Vet

```bash
make vet
```

Fast static analysis — catches real bugs (bad format strings, unreachable code, suspicious struct tags).
Stop if vet fails — these are likely bugs that need manual fixes.

### Step 1.2: Lint

```bash
make lint
```

If lint fails, report the issues. Do NOT run `make lint-fix` automatically — ask the user first:
> "Lint found N issues. Run `make lint-fix` to auto-fix some of them — proceed?"

### Step 1.3: Fmt check (only if vet or lint failed)

Run `make fmt` only when vet or lint reported failures — formatting drift is only worth surfacing when there are already problems to fix:

```bash
make fmt
git diff --name-only
```

`make fmt` is idempotent — it only reformats files. If `git diff` shows changed files, report them as formatting drift. Do NOT stage or commit the changes — just report.

Skip this step entirely when both vet and lint passed.

### Step 1.4: Unit tests

```bash
make test
```

Runs all tests excluding `/e2e` and `/lab`, produces `cover.out`.

### Step 1.4: gNMI integration tests

```bash
make test-gnmi
```

Builds a fake gNMI server from `test/gnmi/` and runs integration tests against it. Fully standalone — no cluster or VM needed.

### Phase 1 summary

```
  Local Check Report
  ─────────────────────────────────────────────────────
  Vet:         ✓ passed  (or ✗ N issues)
  Lint:        ✓ passed  (or ✗ N issues)
  Fmt:         ✓ no drift  (or ✗ N files need formatting)  ← only shown when vet or lint failed
  Unit tests:  ✓ N passed, 0 failed  (or ✗ list failing tests)
  gNMI tests:  ✓ N passed, 0 failed  (or ✗ list failing testdata files + diff)
  ─────────────────────────────────────────────────────
  Overall:     ✓ all checks passed  (or ✗ see above)
```

---

## Phase 2: Test

Manual integration test against one or more real devices.

### Step 2.0: Gather test parameters

If not provided via `$ARGUMENTS`, ask the user:

1. **Device(s):** address:port and credentials for each target (e.g. Nokia SRL at `172.20.20.2:57400`, Juniper at `172.20.20.4:57401`). Default credentials for Nokia SRL: `admin / NokiaSrl1!`.
2. **What to test:** which CR kind and what configuration (e.g. "OpenConfig DNS with servers 8.8.8.8 and 1.1.1.1"). If the user points to a sample file, use that.
3. **Expected output:** optional — if provided, compare gnmic response against it. Otherwise infer from the CR spec.

Store for use in later steps:
```
DEVICES=(<addr:port> ...)
CREDENTIALS=(<user:pass> ...)
TEST_CR=<path or inline yaml>
EXPECTED=<path or empty>
```

### Step 2.1: Observe kind cluster

First verify Docker is available (regardless of whether it's Docker Desktop, colima, or any other runtime):

```bash
docker info 2>&1 | head -5
```

If Docker is not running or not accessible, tell the user and stop — do not attempt to start any Docker runtime.

Then check cluster state — read only:

```bash
kind get clusters
kubectl get nodes
kubectl get pods -n network-operator-system
```

If the `network-operator` cluster does not exist or the operator is not running, proceed with setup automatically.

### Step 2.1a: Install cert-manager (if not present)

cert-manager is required before deploying the network operator. Check if it is already installed:

```bash
kubectl get namespace cert-manager 2>/dev/null
```

If not present, install it and wait for it to be ready

```bash
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.18.2/cert-manager.yaml
kubectl wait --for=condition=Available deployment --all -n cert-manager --timeout=120s
```

### Step 2.1b: Build and load the controller image (if cluster was just created or operator not deployed)

Use a fixed local image tag `network-operator-dev:latest` to avoid colliding with the upstream
`ghcr.io/ironcore-dev/network-operator:latest` image that `config/manager/kustomization.yaml`
maps the `controller` name to by default.

Build the image:

```bash
make docker-build IMG=network-operator-dev:latest
```

Load it into kind:

```bash
bin/kind load docker-image network-operator-dev:latest --name network-operator
```

### Step 2.2: Observe operator

Read the current operator state and logs — no changes:

```bash
kubectl get deployment -n network-operator-system network-operator-controller-manager
kubectl logs -n network-operator-system -l control-plane=controller-manager --tail=50
```

If the operator is not deployed, deploy it:

```bash
make deploy-dev IMG=network-operator-dev:latest PROVIDER=openconfig KUBECTL="kubectl --context kind-network-operator"
```

Then wait for it to be ready:

```bash
kubectl --context kind-network-operator wait --for=condition=Available deployment/network-operator-controller-manager -n network-operator-system --timeout=120s
```

If the operator crashes, check logs before asking the user — do not restart or redeploy without understanding the error.

### Step 2.3: Observe Device resources

Read existing Device CRs:

```bash
kubectl get device -A
kubectl describe device -A
```

If no Device CRs exist for the target devices, write the Device CR + Secret manifests to `/tmp/netop-devices.yaml` and apply them:

```bash
kubectl apply -f /tmp/netop-devices.yaml
```

### Step 2.4: Apply test CRs

If the user has provided a CR to apply, run `kubectl apply -f <cr-file>` directly.

Write any generated CR manifests to `/tmp/netop-<kind>-<name>.yaml` (e.g. `/tmp/netop-banner-vjunos.yaml`) and apply from there:

```bash
kubectl apply -f /tmp/netop-<kind>-<name>.yaml
```

If the user pointed to a testdata file (`test/gnmi/testdata/`), parse it first (read-only):

```
-- <kind>/<name> --
<CR YAML>
-- state --
<expected JSON>
```

Show the CR YAML section, use the state section as expected output, then apply it.

After applying (if approved), observe reconciliation — read only:

```bash
kubectl get <kind> -A
kubectl logs -n network-operator-system -l control-plane=controller-manager --tail=50
```

Expect `READY=True`. If not ready within ~30s, show operator logs — do not restart or redeploy.

### Step 2.5: Validate with gnmic

For each CR and each device, find the gNMI path from the provider source (`internal/provider/openconfig/<resource>.go`, look for `XPath()`), then run:

```bash
gnmic -a <addr> --port <port> -u <user> -p '<password>' --skip-verify --encoding JSON_IETF get --path '<xpath>'
```

Always print the full JSON response without truncation. This is read-only — no gnmic Set calls.

**Validation:**
- Expected output provided → compare field by field
- No expected output → infer expected values from the CR spec and validate those fields

### Phase 2 summary

Print a structured report after all CRs are validated.

For each CR tested:
1. Applied YAML: `kubectl get <kind> <name> -n <namespace> -o yaml`
2. gnmic command and full JSON response
3. Relevant operator logs: `kubectl logs ... | grep -i '<kind>\|error\|warn'`

End with a summary table:

```
  Test Report
  ┌──────────────┬──────────┬───────────┬───────┬──────────────────────────────┬──────────────────┬─────────────────┐
  │   CR Name    │   Kind   │ Namespace │ Ready │          gNMI Path           │      Device      │     Result      │
  ├──────────────┼──────────┼───────────┼───────┼──────────────────────────────┼──────────────────┼─────────────────┤
  │ dns          │ DNS      │ default   │ True  │ openconfig-system:system/dns │ 172.20.20.2:57400│ ✓ value matches │
  │ dns          │ DNS      │ default   │ True  │ openconfig-system:system/dns │ 172.20.20.4:57401│ ✓ value matches │
  └──────────────┴──────────┴───────────┴───────┴──────────────────────────────┴──────────────────┴─────────────────┘
```

Results:
- `✓ value matches` — gnmic response matches expected/inferred value
- `✗ mismatch` — differs (show diff inline below table)
- `✗ not found` — gnmic returned empty or error

---

## Cleanup

Only perform cleanup steps when the user explicitly asks. Always ask for confirmation before any destructive action.

Undeploy the operator:

```bash
make undeploy-dev PROVIDER=<provider>
```

Delete the kind cluster:

```bash
make kind-delete
```

---

## References

- **go vet**: https://pkg.go.dev/cmd/vet
- **golangci-lint**: https://golangci-lint.run
- **gnmic**: https://gnmic.openconfig.net
- **kind**: https://kind.sigs.k8s.io/docs/user/quick-start/
- **kubectl**: https://kubernetes.io/docs/reference/kubectl/
- **OpenConfig YANG schemas**: https://openconfig.net/projects/models/schemadocs/
- **Juniper vJunos-Evolved**: https://www.juniper.net/documentation/us/en/software/vjunos/vjunos-evolved/
- **Cisco NX-OS gNMI**: https://developer.cisco.com/docs/nx-os/#!using-gnmi
