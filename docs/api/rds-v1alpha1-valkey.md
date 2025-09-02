# Valkey API Reference

## Overview

The `Valkey` resource is a unified API for deploying and managing Valkey instances in different architectures, such as `cluster` and `failover` modes. It simplifies the user experience by providing a single entry point for various Valkey setups.

## Resource Definition

```yaml
apiVersion: rds.valkey.buf.red/v1alpha1
kind: Valkey
metadata:
  name: my-valkey
  namespace: default
spec:
  # Valkey specification
status:
  # Valkey status
```

## ValkeySpec

| Field | Type | Description |
|-------|------|-------------|
| `version` | string | Valkey version to use (e.g., "8.0", "7.2") |
| `arch` | core.Arch | Architecture (`cluster`, `failover`, `replica`) |
| `replicas` | *ValkeyReplicas | Desired number of replicas for Valkey |
| `resources` | corev1.ResourceRequirements | Resource requirements |
| `customConfigs` | map[string]string | Custom Valkey configuration |
| `modules` | []core.ValkeyModule | Valkey modules to load |
| `storage` | *core.Storage | Storage configuration |
| `access` | core.InstanceAccess | Access configuration |
| `podAnnotations` | map[string]string | Pod annotations |
| `affinityPolicy` | *core.AffinityPolicy | Affinity policy |
| `customAffinity` | *corev1.Affinity | Custom affinity rules |
| `nodeSelector` | map[string]string | Node selector |
| `tolerations` | []corev1.Toleration | Pod tolerations |
| `securityContext` | *corev1.PodSecurityContext | Pod security context |
| `exporter` | *ValkeyExporter | Monitoring exporter configuration |
| `sentinel` | *SentinelSettings | Sentinel configuration (for failover arch) |

## ValkeyReplicas

| Field | Type | Description |
|-------|------|-------------|
| `shards` | int32 | Number of cluster shards (for cluster arch) |
| `shardsConfig` | []*v1alpha1.ShardConfig | Configuration for each shard |
| `replicasOfShard` | int32 | Number of replicas for each master node |

## ValkeyStatus

| Field | Type | Description |
|-------|------|-------------|
| `phase` | ValkeyPhase | Current phase of the Valkey instance |
| `message` | string | Status message |
| `nodes` | []core.ValkeyNode | Valkey node details |

## Example: Cluster Mode

```yaml
apiVersion: rds.valkey.buf.red/v1alpha1
kind: Valkey
metadata:
  name: example-valkey-cluster
  namespace: valkey-system
spec:
  arch: cluster
  version: "8.0"
  replicas:
    shards: 3
    replicasOfShard: 1
  resources:
    requests:
      memory: "1Gi"
      cpu: "500m"
    limits:
      memory: "2Gi"
      cpu: "1000m"
```

## Example: Failover Mode

```yaml
apiVersion: rds.valkey.buf.red/v1alpha1
kind: Valkey
metadata:
  name: example-valkey-failover
  namespace: valkey-system
spec:
  arch: failover
  version: "8.0"
  replicas:
    replicasOfShard: 3
  sentinel:
    replicas: 3
    quorum: 2
  resources:
    requests:
      memory: "512Mi"
      cpu: "250m"
```
