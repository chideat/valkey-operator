# ValkeyOperator

[![Coverage Status](https://coveralls.io/repos/github/chideat/valkey-operator/badge.svg?branch=main)](https://coveralls.io/github/chideat/valkey-operator?branch=main)
[![Go Report Card](https://goreportcard.com/badge/github.com/chideat/valkey-operator)](https://goreportcard.com/report/github.com/chideat/valkey-operator)

**ValkeyOperator** is a production-ready kubernetes operator to deploy and manage high available [Valkey Sentinel](https://valkey.io/topics/sentinel/) and [Valkey Cluster](https://valkey.io/topics/cluster-spec/) instances. This repository contains multi [Custom Resource Definition (CRD)](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#customresourcedefinitions) designed for the lifecycle of Valkey standalone, sentinel or cluster instance.

## Features

* Standalone/Sentinel/Cluster valkey arch supported.
* Valkey ACL supported.
* Nodeport/LB access supported; nodeport assignement also supported.
* IPv4/IPv6 supported.
* Online scale up/down.
* Graceful version upgrade.
* Nodeselector, toleration and affinity supported.
* High available in production environment.

## Quickstart

If you have a Kubernetes cluster and `kubectl` configured to access it, run the following commands to deploy the operator and create a simple cluster:

```bash
# Install the ValkeyOperator
kubectl apply -k https://github.com/chideat/valkey-operator/config/default

# Deploy a simple Valkey cluster
kubectl apply -f https://raw.githubusercontent.com/chideat/valkey-operator/main/docs/examples/basic/simple-cluster.yaml

# Check the cluster status
kubectl get cluster simple-cluster -w
```

For detailed installation and configuration instructions, see the [User Guide](./docs/guides/user-guide.md).

## Supported Versions

| Version | K8s Versions | Supported |
|---------|:-------------|-----------|
| 7.2.x   | 1.31         | Yes       |
|         | 1.32         | Yes       |
| 8.0.x   | 1.31         | Yes       |
|         | 1.32         | Yes       |
| 8.1.x   | 1.31         | Yes       |
|         | 1.32         | Yes       |

## Documentation

ValkeyOperator is covered by comprehensive documentation:

* **[Operator Overview](./docs/guides/operator-overview.md)** - Architecture and core concepts
* **[User Guide](./docs/guides/user-guide.md)** - Complete installation and usage guide
* **[API Reference](./docs/api/index.md)** - Detailed API documentation
* **[Examples](./docs/examples/)** - Ready-to-use configuration examples

For a complete list of features and configuration options, see the [documentation directory](./docs/).

In addition, practical [examples](./docs/examples) and [configuration samples](./config/samples) can be found in this repository.

## Contributing

This project follows the typical GitHub pull request model. Before starting any work, please either comment on an [existing issue](https://github.com/chideat/valkey-operator/issues), or file a new one. For more details, please refer to the [CONTRIBUTING.md](./CONTRIBUTING.md) file.

## Releasing

To release a new version of the ValkeyOperator, create a versioned tag (e.g. `v0.1.0`) of the repo, and the release pipeline will generate a new draft release, along side release artefacts.

## License

[Licensed under Apache 2.0](LICENSE)
