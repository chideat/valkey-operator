---
title: Deploy Valkey Sentinel on Kubernetes
description: Run a Valkey primary with replicas and Valkey Sentinel on Kubernetes with Valkey Operator, connect through Sentinel and test a failover.
---

# Deploy Valkey Sentinel on Kubernetes

Valkey Sentinel keeps a Valkey primary available. The Sentinel nodes watch the primary and its replicas. When the primary stops answering and enough of them agree on it, they promote a replica and give clients the new primary's address.

Valkey Operator runs the whole setup from one `Valkey` resource with `arch: failover`: the data pods, the Sentinel pods and the Services that clients use.

## Before you start

- A Kubernetes cluster, version 1.33 or later, with at least three schedulable nodes. By default the operator puts each data pod on a different node, and each Sentinel pod as well.
- Valkey Operator installed; see [Installing the Operator](user-guide.md#installing-the-operator).

## Create the instance

```yaml title="valkey-sentinel.yaml"
apiVersion: rds.valkey.buf.red/v1alpha1
kind: Valkey
metadata:
  name: my-valkey
  namespace: default
spec:
  arch: failover
  version: "9.1"
  replicas:
    # Data pods, the primary included: one primary and two replicas.
    replicasOfShard: 3
  sentinel:
    replicas: 3
    # Sentinel nodes that must agree the primary is down before a failover starts.
    quorum: 2
  resources:
    requests:
      cpu: 250m
      memory: 512Mi
    limits:
      cpu: 500m
      memory: 512Mi
```

```bash
kubectl apply -f valkey-sentinel.yaml
kubectl get valkey my-valkey -w
```

The instance is ready when its `STATUS` is `Ready`.

## What the operator creates

| Resource | Name | Purpose |
|---|---|---|
| StatefulSet | `rfr-my-valkey` | The data pods: the primary and its replicas |
| StatefulSet | `rfs-my-valkey` | The Sentinel pods |
| Service | `rfr-my-valkey-readwrite` | Port 6379 on the current primary |
| Service | `rfr-my-valkey-readonly` | Port 6379 on the replicas |
| Service | `rfs-my-valkey` | Headless Service of the Sentinel pods, port 26379 |

The operator labels the current primary with `valkey.buf.red/role=master` and the replicas with `valkey.buf.red/role=replica`, and moves the labels after a failover, so the two Services keep pointing at the right pods.

## Get the passwords

The operator generates two passwords for a new instance and keeps them in Secrets:

| Password | Secret |
|---|---|
| The `default` user, for the data pods | `failover-acl-my-valkey-default-secret` |
| The Sentinel nodes | `sentinel-acl-my-valkey-default-secret` |

```bash
PASSWORD=$(kubectl get secret failover-acl-my-valkey-default-secret -o jsonpath='{.data.password}' | base64 -d)
SENTINEL_PASSWORD=$(kubectl get secret sentinel-acl-my-valkey-default-secret -o jsonpath='{.data.password}' | base64 -d)
```

!!! note
    Operator releases after v2.1.0 generate these passwords. See [Passwords](user-guide.md#passwords) for when the `default` user starts to require its password, how to change it, and how to use a Secret of your own.

## Connect

A client that supports Sentinel, such as valkey-go, redis-py or Jedis, is given the Sentinel addresses, the name of the monitored primary, `mymaster`, and both passwords. It asks Sentinel where the primary is and connects there, and asks again after a failover.

To try it from inside the cluster:

```bash
kubectl run valkey-client --rm -it --restart=Never --image valkey/valkey:9.1 \
  --env PASSWORD="$PASSWORD" --env SENTINEL_PASSWORD="$SENTINEL_PASSWORD" -- sh

# In the client pod: ask Sentinel for the primary, then write to it.
valkey-cli -h rfs-my-valkey -p 26379 -a "$SENTINEL_PASSWORD" --no-auth-warning \
  SENTINEL get-master-addr-by-name mymaster
valkey-cli -h rfr-my-valkey-readwrite -a "$PASSWORD" --no-auth-warning SET greeting hello
```

To reach the instance from outside the cluster, set `spec.access.serviceType` to `NodePort` or `LoadBalancer`.

## Test a failover

Delete the primary pod and watch Sentinel promote a replica:

```bash
kubectl delete pod -l app.kubernetes.io/name=my-valkey,valkey.buf.red/role=master
kubectl get pods -l app.kubernetes.io/name=my-valkey,valkey.buf.red/role=master -w
```

Ask Sentinel for the primary again from the client pod: it now names the promoted replica. The StatefulSet recreates the deleted pod, and it rejoins as a replica.

## Scale and upgrade

- To add or remove replicas, change `spec.replicas.replicasOfShard`.
- To upgrade Valkey, change `spec.version` to a newer supported version, and the operator rolls the pods over to it. The supported versions are listed in the [Operator Overview](operator-overview.md#supported-valkey-versions).

## Next steps

- [User Guide](user-guide.md) for storage, monitoring, TLS and customizing the generated resources.
- [Valkey Cluster on Kubernetes](valkey-cluster-on-kubernetes.md) when one primary is not enough and the data has to be sharded.
- [FAQ](../faq.md)
