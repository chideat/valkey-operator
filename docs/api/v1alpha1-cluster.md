# Cluster API Reference

## Overview

The `Cluster` resource defines a Valkey cluster instance with multiple shards and replicas.

## Resource Definition

```yaml
apiVersion: valkey.buf.red/v1alpha1
kind: Cluster
metadata:
  name: my-cluster
  namespace: default
spec:
  # Cluster specification
status:
  # Cluster status
```

## ClusterSpec

| Field | Type | Description |
|-------|------|-------------|
| `image` | string | Valkey image to use |
| `imagePullPolicy` | corev1.PullPolicy | Image pull policy |
| `imagePullSecrets` | []corev1.LocalObjectReference | Image pull secrets |
| `replicas` | ClusterReplicas | Number of cluster replicas and shards |
| `customConfigs` | map[string]string | Custom Valkey configuration (key-value format) |
| `resources` | corev1.ResourceRequirements | Resource requirements |
| `access` | core.InstanceAccess | Access configuration for the cluster |
| `storage` | *core.Storage | Storage configuration |
| `exporter` | *core.Exporter | Monitoring exporter configuration |
| `annotations` | map[string]string | Pod annotations |
| `affinityPolicy` | *core.AffinityPolicy | Affinity policy (SoftAntiAffinity, AntiAffinityInShard, AntiAffinity, CustomAffinity) |
| `customAffinity` | *corev1.Affinity | Custom affinity rules |
| `nodeSelector` | map[string]string | Node selector |
| `tolerations` | []corev1.Toleration | Pod tolerations |
| `securityContext` | *corev1.PodSecurityContext | Pod security context |
| `modules` | []core.ValkeyModule | Valkey modules to load |

## ClusterReplicas

| Field | Type | Description |
|-------|------|-------------|
| `shards` | int32 | Number of cluster shards (3-128, default: 3) |
| `shardsConfig` | []*ShardConfig | Configuration for each shard |
| `replicasOfShard` | int32 | Number of replicas for each master node (0-5) |

## ShardConfig

| Field | Type | Description |
|-------|------|-------------|
| `slots` | string | Slot range for the shard (e.g., "0-1000,1002,1005-1100") |

## ClusterStatus

| Field | Type | Description |
|-------|------|-------------|
| `phase` | ClusterPhase | Current phase of the cluster |
| `message` | string | Status message |
| `nodes` | []core.ValkeyNode | Cluster node details |
| `clusterStatus` | ClusterServiceStatus | Service status (InService/OutOfService) |
| `shards` | []*ClusterShards | Shard status information |

## ClusterPhase Values

- `Ready` - Cluster is ready and serving traffic
- `Failed` - Cluster failed to deploy or is in error state
- `Paused` - Cluster is paused
- `Creating` - Cluster is being created
- `RollingUpdate` - Cluster is undergoing a rolling update
- `Rebalancing` - Cluster is rebalancing slots

## Example

```yaml
apiVersion: valkey.buf.red/v1alpha1
kind: Cluster
metadata:
  name: example-cluster
  namespace: valkey-system
spec:
  image: valkey/valkey:8.0-alpine
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
  storage:
    storageClassName: fast-ssd
    capacity: 10Gi
  access:
    serviceType: ClusterIP
  customConfigs:
    maxmemory-policy: "allkeys-lru"
    tcp-keepalive: "300"
```