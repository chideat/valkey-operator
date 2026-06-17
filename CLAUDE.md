# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Valkey Operator is a production-grade Kubernetes operator that automates deployment and management of [Valkey](https://valkey.io/) (Redis-compatible) instances. It supports three architectures: **Cluster**, **Sentinel/Failover**, and **Standalone**.

## Common Commands

```bash
# Build
make build              # Build manager and helper binaries
make manifests          # Regenerate CRDs, ClusterRole, WebhookConfiguration (run after API changes)
make generate           # Regenerate DeepCopy methods (run after API type changes)

# Test
make test               # Run unit tests with coverage
make test-e2e           # Run end-to-end tests (requires Kind cluster)
go test ./internal/builder/clusterbuilder/... -run TestFoo  # Run a single test or package

# Lint & Format
make lint               # golangci-lint
make lint-fix           # golangci-lint with auto-fix
make fmt                # go fmt
make vet                # go vet

# Docs
make docs               # Generate API docs
make docs-serve         # Serve docs at http://localhost:8080
```

After modifying types in `/api/`, always run `make generate && make manifests`.

## Architecture

### API Groups

Two API groups with different abstraction levels:
- **`valkey.buf.red`** (`/api/v1alpha1/`) — fine-grained control: `Cluster`, `Sentinel`, `Failover`, `User`
- **`rds.valkey.buf.red`** (`/api/rds/v1alpha1/`) — high-level simplified API: `Valkey`
- **`/api/core/`** — shared types reused across both groups (Access, Storage, Exporter, etc.)

### Key Packages

| Package | Role |
|---|---|
| `/internal/controller/` | Reconcilers for each CRD; entry points for K8s watch events |
| `/internal/builder/` | Constructs K8s resources (StatefulSets, Services, ConfigMaps) per architecture |
| `/internal/ops/` | Rule-based OpEngine that orchestrates multi-step operations |
| `/internal/actor/` | Async actor/command pattern (Requeue, Pause, Abort, Success) |
| `/internal/valkey/` | Valkey topology models for cluster, sentinel, and failover |
| `/internal/webhook/` | Validating/mutating webhooks gated by `ENABLE_WEBHOOKS` env var |
| `/pkg/kubernetes/` | K8s client wrappers and utilities |
| `/cmd/main.go` | Operator manager entry point |
| `/cmd/helper/main.go` | Helper binary for init containers and management commands |

### Reconciliation Flow

1. **Controller** watches CRD, calls `Reconcile()`
2. **Builder** packages construct/diff K8s resources
3. **OpEngine** runs rule-based checks to determine next operation
4. **Actors** execute async tasks and return commands (Requeue, Pause, Abort)
5. **Finalizers** handle cleanup on deletion (`ResourceCleanFinalizer`)

### Operations Engine

The `OpEngine` in `/internal/ops/` uses a rule-based system:
- Rules inspect instance state and return the next command
- `MaxCallDepth = 15` prevents infinite recursion
- Separate rule engines per architecture: `/internal/ops/cluster/`, `/internal/ops/sentinel/`, `/internal/ops/failover/`

### Standard Labels

```go
ManagedByLabelKey    = "app.kubernetes.io/managed-by"
ArchLabelKey         = "valkeyarch"
RoleLabelKey         = "valkey.buf.red/role"
ChecksumLabelKey     = "valkey.buf.red/checksum"
```

Checksums on ConfigMaps trigger pod restarts when config changes.

## Key Constants

```go
DefaultValkeyServerPort    = 6379
DefaultValkeyServerBusPort = 16379  // Cluster bus
DefaultValkeySentinelPort  = 26379
DefaultRequeueDuration     = 15 * time.Second
DefaultReconcileTimeout    = 5 * time.Minute
```

## Testing Patterns

- **Unit tests**: Ginkgo v2 + Gomega, colocated with source (`*_test.go`)
- **Mocking**: `miniredis` and `redigomock` for Valkey; `envtest` for K8s controller tests
- **E2E tests**: `/test/e2e/`, require a running Kind cluster
- `KUBEBUILDER_ASSETS` env var points to K8s API server binaries for envtest

## Environment Variables

| Variable | Purpose |
|---|---|
| `ENABLE_WEBHOOKS` | Set to `"false"` to disable validating webhooks |
| `KUBEBUILDER_ASSETS` | Path to envtest binaries (used in tests) |

## Operator Flags

```
--leader-elect              Enable leader election for HA
--webhook-cert-path         Directory containing webhook TLS certs
--health-probe-bind-address Health/readiness probe address (default ":8081")
--metrics-bind-address      Metrics endpoint (default "0" = disabled)
```

## Deployment

Uses Kustomize (`/config/`). Key overlays:
- `/config/default/` — standard deployment with webhooks
- `/config/crd/` — CRD manifests only
- `/config/samples/` — example CRs for all architectures

Requires cert-manager for webhook TLS certificates.
