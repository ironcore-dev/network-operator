---
name: netop-check
description: Run local development checks for network-operator — lint, unit tests, and gNMI integration tests. Use before committing or opening a PR. All commands run on the host machine (no VM needed).
argument-hint: [lint | test | all]
allowed-tools: [Bash, Read, AskUserQuestion]
---

# netop-check

Runs local development checks:
1. Vet (go vet — fast static analysis)
2. Lint (golangci-lint)
3. Unit tests
4. gNMI functional tests

All commands run directly on the host machine in the repo root.

## Current changes

```bash
git diff HEAD
```

## Instructions

Run the phases below in order, or just the one the user asked for via `$ARGUMENTS`.

### Step 1: Vet

```bash
make vet
```

`go vet` catches real bugs — incorrect format strings, unreachable code, suspicious struct tags, etc. It's fast and should always pass.

If vet **fails** → show the errors and stop. These are likely bugs that need manual fixes before proceeding.

### Step 2: Lint

```bash
make lint
```

If lint **passes** → report success and continue.

If lint **fails** → show the errors and ask the user which fix to try:

- `make fmt` — fixes import ordering and formatting (goimports + gofumpt), style only
- `make lint-fix` — runs golangci-lint with `--fix`, auto-fixes some lint issues beyond formatting
- Both — run `make fmt` first, then `make lint-fix`

Then re-run `make lint` to confirm the remaining errors (if any) need manual fixes.

> **Note:** Neither command resolves logic or type errors — those need manual fixes.

### Step 3: Unit tests

```bash
make test
```

This runs all tests excluding `/e2e` and `/lab` subdirectories and produces `cover.out`.

If tests fail → show the failing test names and error output.

### Step 4: gNMI integration tests

```bash
make test-gnmi
```

This builds and runs the fake gNMI server from `test/gnmi/` and executes the integration tests against it. Fully standalone — no kind cluster or VM needed.

If tests fail → show the failing testdata files and the diff between expected and actual state.

### Summary

After all phases complete, print a report:

```
  Local Dev Report
  ────────────────────────────────────────────────────────
  Vet:         ✓ passed  (or ✗ N issues — list them)
  Lint:        ✓ passed  (or ✗ N issues — list them)
               (fmt offered: yes/no)
  Unit tests:  ✓ N passed, 0 failed  (or ✗ N failed — list failing tests)
  gNMI tests:  ✓ N passed, 0 failed  (or ✗ N failed — list failing testdata files)
  ────────────────────────────────────────────────────────
  Overall:     ✓ all checks passed  (or ✗ see above)
```

For gNMI test failures, show the diff between expected and actual state:
```
  FAIL: testdata/openconfig/banner.txt
  Expected: {"openconfig-system:system":{"config":{"login-banner":"..."}}}
  Actual:   {}
```

## References

- [go vet](https://pkg.go.dev/cmd/vet) — static analysis tool built into Go
- [golangci-lint](https://golangci-lint.run) — aggregated linter runner (custom build used here via `.custom-gcl.yaml`)
- [goimports](https://pkg.go.dev/golang.org/x/tools/cmd/goimports) — fixes import grouping and formatting (`make fmt`)
- [gofumpt](https://github.com/mvdan/gofumpt) — stricter gofmt, run alongside goimports (`make fmt`)
- [gnmic](https://gnmic.openconfig.net) — gNMI CLI client used for validation
