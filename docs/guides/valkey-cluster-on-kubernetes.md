---
title: Deploy Valkey Cluster on Kubernetes
description: Run a sharded Valkey Cluster on Kubernetes with Valkey Operator, connect with a cluster-aware client, and add shards online.
---

# Deploy Valkey Cluster on Kubernetes

Valkey Cluster spreads the keyspace over shards: it splits the 16384 hash slots between them, and each shard is a primary with its replicas. When a primary fails, the other nodes of the cluster promote one of its replicas; no Sentinel is involved.

Valkey Operator runs a Valkey Cluster from one `Valkey` resource with `arch: cluster`. It creates the pods, joins them into a cluster, assigns the slots, and moves slots when you add or remove shards.

## Before you start

- A Kubernetes cluster, version 1.33 or later, with at least as many schedulable nodes as there are pods in one shard. By default the operator puts the pods of a shard on different nodes, so that a node failure takes at most one copy of each slot with it.
- Valkey Operator installed; see [Installing the Operator](user-guide.md#installing-the-operator).

## Create the instance

```yaml title="valkey-cluster.yaml"
apiVersion: rds.valkey.buf.red/v1alpha1
kind: Valkey
metadata:
  name: my-valkey-cluster
  namespace: default
spec:
  arch: cluster
  version: "9.1"
  replicas:
    shards: 3
    # Pods in each shard, the primary included: one primary and one replica.
    replicasOfShard: 2
  resources:
    requests:
      cpu: 250m
      memory: 512Mi
    limits:
      cpu: 500m
      memory: 512Mi
```

```bash
kubectl apply -f valkey-cluster.yaml
kubectl get valkey my-valkey-cluster -w
```

The instance is ready when its `STATUS` is `Ready`.

## What the operator creates

| Resource | Name | Purpose |
|---|---|---|
| StatefulSet | `drc-my-valkey-cluster-0` to `-2` | One per shard: its primary and replicas |
| Service | `my-valkey-cluster` | Port 6379 on every node, to start a client from |
| Secret | `cluster-acl-my-valkey-cluster-default-secret` | The password of the `default` user |

## Connect

A cluster-aware client connects to any node, learns from it which node holds which slots, and then talks to each node directly. Get the password, then try it from inside the cluster:

```bash
PASSWORD=$(kubectl get secret cluster-acl-my-valkey-cluster-default-secret -o jsonpath='{.data.password}' | base64 -d)

kubectl run valkey-client --rm -it --restart=Never --image valkey/valkey:9.1 \
  --env PASSWORD="$PASSWORD" -- sh

# In the client pod: -c follows the cluster's redirects to the node that owns a key.
valkey-cli -c -h my-valkey-cluster -a "$PASSWORD" --no-auth-warning SET greeting hello
valkey-cli -c -h my-valkey-cluster -a "$PASSWORD" --no-auth-warning CLUSTER INFO
```

!!! note
    Operator releases after v2.1.0 generate the `default` user's password. See [Passwords](user-guide.md#passwords) for when it starts to be required and how to change it.

The nodes announce their pod addresses, which only clients inside the Kubernetes cluster can reach. For clients outside it, set `spec.access.serviceType` to `NodePort` or `LoadBalancer`, and the nodes announce addresses that those clients can reach instead.

## Add or remove shards

Change `spec.replicas.shards`. While the operator moves slots to the new shards, or off the shards being removed, the instance shows `Rebalancing`. Clients keep working during the move: a cluster-aware client follows the redirects.

```bash
kubectl patch valkey my-valkey-cluster --type merge -p '{"spec":{"replicas":{"shards":4}}}'
kubectl get valkey my-valkey-cluster -w
```

## Upgrade

Change `spec.version` to a newer supported version, and the operator rolls the pods over to it. The supported versions are listed in the [Operator Overview](operator-overview.md#supported-valkey-versions).

## Next steps

- [User Guide](user-guide.md) for storage, monitoring, TLS and customizing the generated resources.
- [Valkey Sentinel on Kubernetes](valkey-sentinel-on-kubernetes.md) when one primary with replicas is enough.
- [FAQ](../faq.md)
