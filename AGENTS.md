# AI Agent Guide

## Rules (Quick Reference)

- NEVER edit auto-generated files (see project structure for which ones)
- NEVER delete `// +kubebuilder:scaffold:*` markers — CLI injects code at these
- NEVER move files — the CLI expects files in specific locations
- NEVER create API/webhook files manually — use `kubebuilder create api` / `kubebuilder create webhook`

## Project Structure

```
cmd/main.go                            Manager entry (registers controllers/webhooks)
api/<group>/<version>/*_types.go       CRD schemas by group
api/<group>/<version>/zz_generated.*   Auto-generated (DO NOT EDIT)
internal/controller/<group>/*          Reconciliation logic
internal/webhook/<group>/<version>/*   Validation/defaulting Webhooks
config/crd/bases/*                     Generated CRDs (DO NOT EDIT)
config/rbac/role.yaml                  Generated RBAC (DO NOT EDIT)
config/samples/*                       Example CRs (edit these)
Makefile                               Build/test/deploy commands
PROJECT                                Kubebuilder metadata Auto-generated (DO NOT EDIT)
```

## After Making Changes

Tools are installed locally into `bin/` — the Makefile downloads them automatically on first use.

**After editing `*_types.go` or markers:**

```bash
make manifests  # Regenerate CRDs/RBAC from markers
make generate   # Regenerate DeepCopy methods
make charts     # Regenerate Helm chart manifests
```

**After editing any `*.go` files:**

```bash
make fmt        # Format code
make lint       # Lint code style
make test       # Run unit tests (uses envtest: real K8s API + etcd)
```

Tests use **Ginkgo + Gomega** (BDD style) for controller and webhook tests — check `suite_test.go` in each package for setup.
All other packages use the **standard library `testing` package** with table-driven tests where appropriate.

- Keep test files focused — only necessary cases, no exhaustive permutations
- Test helpers stay in the same file as their tests

**All of `make fmt lint test` must pass before work is considered complete.**

## Commits

- Run `git log` first and match the existing commit style
- Plain imperative mood, no semantic prefixes (no feat:/fix:/chore:)
- Max 72 characters per line

## Development Workflow

Always run against a dedicated development cluster (e.g. Kind, minikube), never a real dev/prod cluster. Always pass `--context` explicitly to kubectl commands.

```bash
# Build the controller image
make docker-build IMG=controller:latest

# Make the image available to the cluster (if using kind)
kind load docker-image controller:latest --name network-operator

# Deploy the controller
make deploy IMG=controller:latest KUBECTL="kubectl --context <context>"

# Apply sample resources
kubectl --context <context> apply -f config/samples/<resource>

# Inspect controller logs
kubectl --context <context> logs -n network-operator-system deployment/network-operator-controller-manager -c manager -f
```

## API Design

**Key markers for** `api/<group>/<version>/*_types.go`:

```go
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=".status.conditions[?(@.type=='Ready')].status"

// On fields:
// +kubebuilder:validation:Required
// +kubebuilder:validation:Minimum=1
// +kubebuilder:validation:MaxLength=100
// +kubebuilder:validation:Pattern="^[a-z]+$"
// +kubebuilder:default="value"
```

- **Use** `metav1.Condition` for status (not custom string fields)
- **Use predefined types**: `metav1.Time` instead of `string` for dates
- **Follow K8s API conventions**: Standard field names (`spec`, `status`, `metadata`)

## Controller Design

**RBAC markers in** `internal/<group>/controller/*_controller.go`:

```go
// +kubebuilder:rbac:groups=mygroup.example.com,resources=mykinds,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mygroup.example.com,resources=mykinds/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=mygroup.example.com,resources=mykinds/finalizers,verbs=update
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch
```

**Implementation rules:**

- **Idempotent reconciliation**: Safe to run multiple times
- **Re-fetch before updates**: `r.Get(ctx, req.NamespacedName, obj)` before `r.Update` to avoid conflicts
- **Structured logging**: `log := log.FromContext(ctx); log.Info("msg", "key", val)`
- **Owner references**: Enable automatic garbage collection (`SetControllerReference`)
- **Watch secondary resources**: Use `.Owns()` or `.Watches()`, not just `RequeueAfter`
- **Finalizers**: Clean up external resources

## Provider Implementation

Provider implementations live in `internal/provider/<platform>/` and realize the API types as actual device configuration. Follow the `internal/provider/openconfig/` package as the reference implementation.

**File structure:**

- One `.go` file per API resource (e.g. `interface.go`, `banner.go`, `device.go`)
- Each file contains: the provider interface implementation for that resource, plus all Go structs and helpers needed to realize it
- Keep it simple — avoid many small few-line functions and abstractions that don't serve a real purpose (YAGNI)
- Use string constants for any value used more than once — no raw string literals scattered across the code

**Idempotency is mandatory:**

A provider's `EnsureX` methods must be safe to call on every reconciliation without causing unnecessary device writes. After initial configuration, repeated calls must result in "configuration is already up to date" — no gNMI Set requests unless an actual drift exists.

The `gnmiext` package already implements a Get-and-Check approach: it diffs current device state against the desired configuration and only performs a gNMI Set when a real change is needed. This makes it safe for periodic reconciliation.

**Platform default values — critical pitfall:**

Optional fields in the API spec that map to optional fields in the provider Go struct require special handling:

1. If a field is unset in the API spec, the provider must adopt the platform's known default value in the struct (not leave it zero/empty)
2. Only overwrite with the API spec value when explicitly set by the user
3. This prevents false diffs: without this, the device returns its default → diff sees `"" != "default"` → unnecessary Set on every reconcile

**The `omitempty`/`omitzero` trap:**

Using `omitempty` or `omitzero` JSON struct tags on gNMI payload structs is dangerous for the same reason. An omitted field means "don't set this" in the gNMI payload, which is correct on first write. But on subsequent reconciles:

1. Device returns platform defaults for those fields in the Get response
2. The desired struct has those fields empty/zero (omitted from serialization)
3. Diff compares device state (with defaults) against desired (without) → mismatch
4. Operator performs a Set on every reconcile — breaking idempotency

**`omitempty` guidelines:**

1. **Safe:** The field's Go zero value matches the platform default or "absent" state. Omitting it from the payload is semantically equivalent to the device's default.
2. **Safe:** The field is a pointer or slice representing "not configured" (nil) vs "configured" (non-nil). Mutually exclusive choices (e.g. `accept`/`drop`) fall into this category.
3. **Dangerous:** The platform default is non-zero (e.g. `admin-state` defaults to `"enable"`, `port` defaults to `49`). Omitting the Go zero value would either misrepresent intent or cause a false diff on subsequent GET responses.
4. **Unnecessary:** The field is unconditionally set to a non-zero value by the provider code. The tag never triggers, but removing it documents intent — the field is always present.

**Rule:** After setting configuration once, the provider must produce no gNMI Set calls on subsequent reconciles when no user-facing configuration has changed. Test this explicitly.

## Logging

**Follow Kubernetes logging message style guidelines:**

- Start from a capital letter
- Do not end the message with a period
- Active voice: subject present (`"Deployment could not create Pod"`) or omitted (`"Could not create Pod"`)
- Past tense: `"Could not delete Pod"` not `"Cannot delete Pod"`
- Specify object type: `"Deleted Pod"` not `"Deleted"`
- Balanced key-value pairs

```go
log.Info("Starting reconciliation")
log.Info("Created Deployment", "name", deploy.Name)
log.Error(err, "Failed to create Pod", "name", name)
```

## Coding Philosophy

- **YAGNI** — don't build it until it's needed. No scaffolding "for later"
- **Reuse before writing** — check if the codebase, stdlib, or an existing dependency already solves it
- **No premature abstraction** — no interfaces with one implementation, no factories for a single entities, no config for a value that never changes
- **Deletion over addition** — remove complexity rather than add code
- **Fewest files, shortest diff** — the minimal change that solves the problem wins
- **Fix root causes** — fix bugs where all callers converge, not in each caller
- **Edge-case correctness** — between two approaches of similar size, pick the one correct on edge cases
- **Don't over-comment** — don't document trivial steps; avoid unnecessary empty lines between code segments
- **Range by index over list.Items** — Kubernetes `List` types hold items as value slices; using the second loop variable (`for _, item := range list.Items`) copies the struct each iteration. Use `for i := range list.Items` and reference `&list.Items[i]`

## References

### Go Style

- **Effective Go**: https://go.dev/doc/effective_go (idiomatic Go patterns and design)
- **Go Code Review Comments**: https://go.dev/wiki/CodeReviewComments (common review feedback, naming, error handling)
- **Google Go Style Guide**: https://google.github.io/styleguide/go/guide (naming, formatting, conventions)
- **Google Go Best Practices**: https://google.github.io/styleguide/go/best-practices (patterns, error handling, design)

### Essential Reading

- **Kubebuilder Book**: https://book.kubebuilder.io (comprehensive guide)
- **controller-runtime FAQ**: https://github.com/kubernetes-sigs/controller-runtime/blob/main/FAQ.md (common patterns and questions)
- **Good Practices**: https://book.kubebuilder.io/reference/good-practices.html (why reconciliation is idempotent, status conditions, etc.)
- **Logging Conventions**: https://github.com/kubernetes/community/blob/main/contributors/devel/sig-instrumentation/logging.md#message-style-guidelines (message style, verbosity levels)

### API Design & Implementation

- **API Conventions**: https://github.com/kubernetes/community/blob/main/contributors/devel/sig-architecture/api-conventions.md
- **API Changes:** https://github.com/kubernetes/community/blob/main/contributors/devel/sig-architecture/api_changes.md#adding-a-field
- **Operator Pattern**: https://kubernetes.io/docs/concepts/extend-kubernetes/operator/
- **Markers Reference**: https://book.kubebuilder.io/reference/markers.html

### Tools & Libraries

- **controller-runtime**: https://github.com/kubernetes-sigs/controller-runtime
- **controller-tools**: https://github.com/kubernetes-sigs/controller-tools
- **Kubebuilder Repo**: https://github.com/kubernetes-sigs/kubebuilder
