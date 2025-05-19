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

If you have a Kubernetes cluster and `kubectl` configured to access it, run the following command to instance the operator:

TODO

## Supported Versions

| Version | K8s Versions | Supported | Tested |
|---------|:-------------|-----------|--------|
| 7.2.x   | 1.31         | Yes       | Yes    |
|         | 1.32         | Yes       |        |
| 8.0.x   | 1.31         | Yes       | Yes    |
|         | 1.32         | Yes       |        |

## Documentation

ValkeyOperator is covered by following topics:

* **TODO** Operator overview
* **TODO** Deploying the operator
* **TODO** Deploying a Valkey sentinel/cluster instance
* **TODO** Monitoring the instance 

In addition, few [samples](./config/samples) can be find in this repo.

## Contributing

This project follows the typical GitHub pull request model. Before starting any work, please either comment on an [existing issue](https://github.com/chideat/valkey-operator/issues), or file a new one. For more details, please refer to the [CONTRIBUTING.md](./CONTRIBUTING.md) file.

## Releasing

To release a new version of the ValkeyOperator, create a versioned tag (e.g. `v0.1.0`) of the repo, and the release pipeline will generate a new draft release, along side release artefacts.

## License

[Licensed under Apache 2.0](LICENSE)
