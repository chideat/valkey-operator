---
title: Valkey Operator FAQ
description: Answers about Valkey Operator, the Kubernetes operator for Valkey Cluster and Valkey Sentinel - architectures, versions, installation, passwords, TLS, scaling and upgrades.
---

# Frequently Asked Questions

## What is Valkey Operator?

Valkey Operator is a Kubernetes operator for [Valkey](https://valkey.io/), the open source key-value data store forked from Redis OSS 7.2. You describe an instance in one `Valkey` custom resource, and the operator creates the pods, Services and configuration, forms the cluster or the Sentinel group, and keeps the instance in the state you asked for: it scales, upgrades and recovers it. It is written in Go and licensed under Apache 2.0.

## Is it the same project as valkey-io/valkey-operator?

No. Valkey Operator ([chideat/valkey-operator](https://github.com/chideat/valkey-operator)) is an independent project with its own API groups, `rds.valkey.buf.red` and `valkey.buf.red`. The Valkey project has a separate operator, [valkey-io/valkey-operator](https://github.com/valkey-io/valkey-operator), whose custom resources are in the `valkey.io` domain. The two are developed separately, and an instance of one cannot be managed by the other.

## Which Valkey architectures does it support?

| `spec.arch` | What runs |
|---|---|
| `cluster` | Valkey Cluster: the 16384 hash slots split over shards, each shard a primary with replicas. See [Valkey Cluster on Kubernetes](guides/valkey-cluster-on-kubernetes.md). |
| `failover` | A primary with replicas, kept available by Valkey Sentinel. See [Valkey Sentinel on Kubernetes](guides/valkey-sentinel-on-kubernetes.md). |
| `replica` | A primary with optional replicas and no Sentinel. |

The `Sentinel` resource also runs Sentinel nodes on their own, to watch Valkey instances outside the operator.

## Which Valkey and Kubernetes versions are supported?

Valkey 7.2, 8.0, 8.1, 9.0 and 9.1, on Kubernetes 1.33, 1.34 and 1.35. The table, and what each release was tested on, is in the [Operator Overview](guides/operator-overview.md#supported-valkey-versions).

## How do I install it?

With Helm or Kustomize; see [Installing the Operator](guides/user-guide.md#installing-the-operator). The admission webhooks need [cert-manager](https://cert-manager.io/) for their certificates, unless you turn the webhooks off.

## What does `replicasOfShard` count?

The pods in each shard, the primary included. `replicasOfShard: 2` is one primary and one replica; `replicasOfShard: 1` is a primary alone.

## Does it set a password?

Yes, for new instances, starting with the releases after v2.1.0. The operator generates a password for the `default` user and, for `arch: failover`, one for the Sentinel nodes, and keeps them in Secrets. See [Passwords](guides/user-guide.md#passwords).

## Does it support ACL users and TLS?

Yes. Each Valkey ACL user is a `User` resource, with its password in a Secret and its ACL rules in the resource; see the [User API](api/v1alpha1-user.md). TLS is turned on with `spec.access.enableTLS`, with certificates from a cert-manager issuer named in `spec.access.certIssuer`; see the [TLS and ACL example](guides/user-guide.md#secure-valkey-cluster-with-tls-and-acl).

## Can I scale and upgrade without downtime?

Shards and replicas are added and removed online: in a Valkey Cluster the operator moves the hash slots while clients keep working. A version upgrade is a change of `spec.version`, and the operator rolls the pods over to it.

## How do clients outside Kubernetes connect?

Set `spec.access.serviceType` to `NodePort` or `LoadBalancer`. The nodes then announce addresses that clients outside the cluster can reach, which Valkey Cluster and Sentinel clients need, since they connect to each node directly.

## Can I change what the operator generates?

Yes. The StatefulSets, PodDisruptionBudgets and Services it generates can be patched through `spec.overwrites`; see [Customizing Generated Resources](guides/user-guide.md#customizing-generated-resources).

## Does it work with Redis clients?

Valkey speaks the same protocol as Redis OSS 7.2, so Redis clients and tools connect to it unchanged, including clients for Valkey Cluster and Sentinel.

## How do I monitor it?

Each instance can run a Prometheus exporter next to Valkey, which Prometheus, or a `ServiceMonitor` of the Prometheus Operator, scrapes; see [Monitoring](guides/user-guide.md#monitoring).
