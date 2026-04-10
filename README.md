# Valkey Operator

[![Coverage Status](https://coveralls.io/repos/github/chideat/valkey-operator/badge.svg?branch=main)](https://coveralls.io/github/chideat/valkey-operator?branch=main)
[![Go Report Card](https://goreportcard.com/badge/github.com/chideat/valkey-operator)](https://goreportcard.com/report/github.com/chideat/valkey-operator)

**Valkey Operator** is a production-grade Kubernetes operator for deploying and managing highly available [Valkey](https://valkey.io/) instances. It provides a unified `Valkey` [Custom Resource](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) that supports **Cluster**, **Failover** (Sentinel-based HA), and **Standalone** (Replica) architectures, as well as fine-grained CRDs (`Cluster`, `Failover`, `Sentinel`, `User`) for advanced use cases.

## Features

* **Multiple architectures** — Cluster (sharded), Failover (Sentinel-based HA), and Standalone/Replica modes.
* **Unified `Valkey` CRD** — A single high-level resource to manage any architecture via the `rds.valkey.buf.red` API group.
* **ACL management** — Dedicated `User` CRD for managing Valkey ACL rules, roles, and permissions.
* **TLS encryption** — Native TLS support with cert-manager integration.
* **Monitoring** — Built-in Prometheus exporter sidecar ([oliver006/redis_exporter](https://github.com/oliver006/redis_exporter)).
* **Persistent storage** — Configurable PersistentVolumeClaims with storage class and retention policies.
* **Networking** — ClusterIP, NodePort, and LoadBalancer service types; IPv4/IPv6 dual-stack support.
* **Valkey modules** — Load custom Valkey modules (Valkey 8.0+).
* **Online scaling** — Horizontal scale-up/down with automatic slot rebalancing for Cluster mode.
* **Graceful upgrades** — Rolling version upgrades with zero downtime.
* **Scheduling control** — Node selectors, tolerations, and multiple affinity policies (SoftAntiAffinity, AntiAffinityInShard, AntiAffinity, Custom).
* **Multi-architecture images** — Supports `linux/amd64` and `linux/arm64`.

## Quickstart

With a Kubernetes cluster and `kubectl` configured:

```bash
# Install the operator (requires cert-manager for webhook TLS)
kubectl apply -k https://github.com/chideat/valkey-operator/config/default

# Deploy a 3-shard Valkey Cluster
kubectl apply -f https://raw.githubusercontent.com/chideat/valkey-operator/main/docs/examples/basic/cluster.yaml

# Watch the cluster converge
kubectl get valkey valkey-cluster -w
```

Other quick examples:

```bash
# Deploy a Sentinel-based failover instance
kubectl apply -f https://raw.githubusercontent.com/chideat/valkey-operator/main/docs/examples/basic/failover.yaml

# Deploy a standalone instance
kubectl apply -f https://raw.githubusercontent.com/chideat/valkey-operator/main/docs/examples/basic/standalone.yaml
```

For detailed installation and configuration instructions, see the [User Guide](./docs/guides/user-guide.md).

## Supported Versions

| Valkey Version | K8s 1.31 | K8s 1.32 | K8s 1.33 | Status |
|:--------------:|:--------:|:--------:|:--------:|:------:|
| 7.2.x          | ✅       | ✅       |          | Stable |
| 8.0.x          | ✅       | ✅       |          | Stable |
| 8.1.x          | ✅       | ✅       |          | Stable |
| 8.2.x          |          | ✅       | ✅       | Stable |
| 9.0.x          |          | ✅       | ✅       | Stable |
| 9.1.x          |          | ✅       | ✅       | Preview |

## API Groups

The operator ships two API groups at different abstraction levels:

| API Group | CRDs | Purpose |
|-----------|------|---------|
| `rds.valkey.buf.red/v1alpha1` | `Valkey` | Unified high-level resource — recommended for most users |
| `valkey.buf.red/v1alpha1` | `Cluster`, `Failover`, `Sentinel`, `User` | Fine-grained control for advanced use cases |

Shared types (Storage, Access, Exporter, etc.) are defined in the `api/core` package and reused across both groups.

## Documentation

* **[Operator Overview](./docs/guides/operator-overview.md)** — Architecture and core concepts
* **[User Guide](./docs/guides/user-guide.md)** — Installation, configuration, and usage
* **[Best Practices](./docs/guides/best-practices.md)** — Production deployment recommendations
* **[API Reference](./docs/api/index.md)** — Detailed CRD and type documentation
* **[Examples](./docs/examples/)** — Ready-to-use manifests (basic, production, ACL users)
* **[Configuration Samples](./config/samples/)** — Minimal CRD samples for each resource

## Contributing

This project follows the typical GitHub pull request model. Before starting any work, please comment on an [existing issue](https://github.com/chideat/valkey-operator/issues) or file a new one. See [CONTRIBUTING.md](./CONTRIBUTING.md) for guidelines.

## Releasing

Create a versioned tag (e.g. `v2.0.0`) and push it. The release pipeline builds multi-architecture Docker images and publishes them to Docker Hub and GitHub Container Registry.

## License

[Apache License 2.0](LICENSE)
