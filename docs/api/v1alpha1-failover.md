# Failover API Reference

## Overview

The `Failover` resource defines a Valkey failover instance managed by Sentinel for high availability.

## Resource Definition

```yaml
apiVersion: valkey.buf.red/v1alpha1
kind: Failover
metadata:
  name: my-failover
  namespace: default
spec:
  # Failover specification
status:
  # Failover status
```

## FailoverSpec

| Field | Type | Description |
|-------|------|-------------|
| `image` | string | Valkey image to use |
| `imagePullPolicy` | corev1.PullPolicy | Image pull policy |
| `imagePullSecrets` | []corev1.LocalObjectReference | Image pull secrets |
| `replicas` | int32 | Number of Valkey replicas |
| `resources` | corev1.ResourceRequirements | Resource requirements |
| `customConfigs` | map[string]string | Custom Valkey configuration |
| `storage` | *core.Storage | Storage configuration |
| `exporter` | *core.Exporter | Monitoring exporter configuration |
| `access` | core.InstanceAccess | Access configuration |
| `podAnnotations` | map[string]string | Pod annotations |
| `affinity` | *corev1.Affinity | Pod affinity rules |
| `tolerations` | []corev1.Toleration | Pod tolerations |
| `nodeSelector` | map[string]string | Node selector |
| `securityContext` | *corev1.PodSecurityContext | Pod security context |
| `sentinel` | *SentinelSettings | Sentinel configuration |
| `modules` | []core.ValkeyModule | Valkey modules to load |

## SentinelSettings

| Field | Type | Description |
|-------|------|-------------|
| `sentinelReference` | *SentinelReference | Reference to external sentinel cluster |
| `monitorConfig` | map[string]string | Sentinel monitor configuration |
| `quorum` | *int32 | Number of sentinels required for quorum |

Plus all fields from `SentinelSpec` when managing embedded sentinel.

## SentinelReference

| Field | Type | Description |
|-------|------|-------------|
| `nodes` | []SentinelMonitorNode | Sentinel node addresses (minimum 3) |
| `auth` | Authorization | Sentinel authentication |

## FailoverStatus

| Field | Type | Description |
|-------|------|-------------|
| `phase` | FailoverPhase | Current phase of the failover |
| `message` | string | Status message |
| `nodes` | []core.ValkeyNode | Valkey node details |
| `tlsSecret` | string | TLS secret name |
| `monitor` | MonitorStatus | Monitor status information |

## FailoverPhase Values

- `Ready` - Failover instance is ready and serving traffic
- `Failed` - Failover instance failed to deploy or is in error state
- `Paused` - Failover instance is paused
- `Creating` - Failover instance is being created

## MonitorStatus

| Field | Type | Description |
|-------|------|-------------|
| `policy` | FailoverPolicy | Failover policy (sentinel/manual) |
| `name` | string | Monitor name |
| `username` | string | Sentinel username |
| `passwordSecret` | string | Password secret name |
| `oldPasswordSecret` | string | Old password secret name |
| `tlsSecret` | string | TLS secret name |
| `nodes` | []SentinelMonitorNode | Sentinel monitor nodes |

## Example

```yaml
apiVersion: valkey.buf.red/v1alpha1
kind: Failover
metadata:
  name: example-failover
  namespace: valkey-system
spec:
  image: valkey/valkey:8.0-alpine
  replicas: 3
  resources:
    requests:
      memory: "512Mi"
      cpu: "250m"
    limits:
      memory: "1Gi"
      cpu: "500m"
  storage:
    storageClassName: standard
    capacity: 5Gi
  sentinel:
    replicas: 3
    quorum: 2
    monitorConfig:
      down-after-milliseconds: "30000"
      failover-timeout: "180000"
      parallel-syncs: "1"
  access:
    serviceType: ClusterIP
  customConfigs:
    maxmemory-policy: "allkeys-lru"
```