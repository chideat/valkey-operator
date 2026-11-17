# Valkey Operator

[![Coverage Status](https://coveralls.io/repos/github/chideat/valkey-operator/badge.svg?branch=main)](https://coveralls.io/github/chideat/valkey-operator?branch=main)
[![Go Report Card](https://goreportcard.com/badge/github.com/chideat/valkey-operator)](https://goreportcard.com/report/github.com/chideat/valkey-operator)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/chideat)](https://artifacthub.io/packages/search?repo=chideat)

**Valkey Operator** is a production-ready Kubernetes operator for [Valkey](https://valkey.io/). It deploys and manages highly available [Valkey Cluster](https://valkey.io/topics/cluster-spec/), [Valkey Sentinel](https://valkey.io/topics/sentinel/) (failover) and replica instances from a single `Valkey` custom resource, and keeps them running: ACL users, TLS, online scaling and graceful version upgrades. This repository contains the [Custom Resource Definitions (CRDs)](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#customresourcedefinitions) for the whole lifecycle of a Valkey cluster, Sentinel or standalone instance.

Valkey Operator is an independent open source project, separate from the Valkey project's own operator, [valkey-io/valkey-operator](https://github.com/valkey-io/valkey-operator).

**Documentation:** <https://chideat.github.io/valkey-operator/>

| `spec.arch` | What runs |
|---|---|
| `cluster` | Valkey Cluster: the 16384 hash slots split over shards, each shard a primary with replicas, rebalanced online when shards are added or removed |
| `failover` | A primary with replicas, kept available by Valkey Sentinel |
| `replica` | A primary with optional replicas and no Sentinel |

## Features

* Standalone/Sentinel/Cluster valkey arch supported.
* Valkey ACL supported.
* Nodeport/LB access supported; nodeport assignement also supported.
* IPv4/IPv6 supported.
* Online scale up/down.
* Graceful version upgrade.
* Nodeselector, toleration and affinity supported.
* Generated StatefulSets, PodDisruptionBudgets and Services can be patched through `spec.overwrites`.
* High available in production environment.

## Quickstart

### Using Helm (Recommended)

If you have a Kubernetes cluster and `kubectl` configured to access it, the easiest way to install Valkey Operator is via Helm:

```bash
# Install Valkey Operator (cert-manager integration is enabled by default)
# Requires cert-manager to be installed: https://cert-manager.io/docs/installation/
helm install valkey-operator oci://ghcr.io/chideat/charts/valkey-operator \
  --namespace valkey-system --create-namespace

# Deploy a Valkey cluster
kubectl apply -f https://raw.githubusercontent.com/chideat/valkey-operator/main/docs/examples/basic/cluster.yaml

# Check the cluster status
kubectl get valkey valkey-cluster -w
```

> **Note:** The Helm chart uses [cert-manager](https://cert-manager.io/docs/installation/) to manage webhook TLS by default. Ensure cert-manager is installed before running `helm install`. Alternatively, disable webhooks with `--set webhook.enabled=false` if you don't need them.

### Using Kustomize

```bash
# Install Valkey Operator
kubectl apply -k https://github.com/chideat/valkey-operator/config/default
```

For detailed installation and configuration instructions, see the [User Guide](./docs/guides/user-guide.md).

## Supported Versions

| Valkey Version | Kubernetes Versions | Status  |
|----------------|---------------------|---------|
| 7.2.x          | 1.33, 1.34, 1.35    | Stable  |
| 8.0.x          | 1.33, 1.34, 1.35    | Stable  |
| 8.1.x          | 1.33, 1.34, 1.35    | Stable  |
| 9.0.x          | 1.33, 1.34, 1.35    | Stable  |
| 9.1.x          | 1.33, 1.34, 1.35    | Stable  |

The v2.1.0 release candidate passed the end-to-end suite on Kubernetes v1.33.12 and v1.34.11 (kind) for every Valkey version above, and on Kubernetes v1.35.6 for Valkey 7.2, 8.1 and 9.1.

## Documentation

The documentation is published at <https://chideat.github.io/valkey-operator/>, from the [docs](./docs/) directory:

* **[Operator Overview](./docs/guides/operator-overview.md)** - Architecture and core concepts
* **[User Guide](./docs/guides/user-guide.md)** - Complete installation and usage guide
* **[Valkey Sentinel on Kubernetes](./docs/guides/valkey-sentinel-on-kubernetes.md)** - A primary with replicas, kept available by Valkey Sentinel
* **[Valkey Cluster on Kubernetes](./docs/guides/valkey-cluster-on-kubernetes.md)** - A sharded Valkey Cluster, scaled online
* **[FAQ](./docs/faq.md)** - Short answers to common questions
* **[API Reference](./docs/api/index.md)** - Detailed API documentation
* **[Examples](./docs/examples/)** - Ready-to-use configuration examples

For a complete list of features and configuration options, see the [documentation directory](./docs/).

In addition, practical [examples](./docs/examples) and [configuration samples](./config/samples) can be found in this repository.

## Contributing

This project follows the typical GitHub pull request model. Before starting any work, please either comment on an [existing issue](https://github.com/chideat/valkey-operator/issues), or file a new one. For more details, please refer to the [CONTRIBUTING.md](./CONTRIBUTING.md) file.

## Releasing

To release a new version of Valkey Operator, create a versioned tag (e.g. `v0.1.0`) of the repo, and the release pipeline will generate a new draft release, along side release artefacts.

## License

[Licensed under Apache 2.0](LICENSE)
