# Sentinel API Reference

## Overview

The `Sentinel` resource defines a standalone Valkey Sentinel instance for monitoring and managing Valkey deployments.

## Resource Definition

```yaml
apiVersion: valkey.buf.red/v1alpha1
kind: Sentinel
metadata:
  name: my-sentinel
  namespace: default
spec:
  # Sentinel specification
status:
  # Sentinel status
```

## SentinelSpec

| Field | Type | Description |
|-------|------|-------------|
| `image` | string | Valkey Sentinel image to use |
| `imagePullPolicy` | corev1.PullPolicy | Image pull policy |
| `imagePullSecrets` | []corev1.LocalObjectReference | Image pull secrets |
| `replicas` | int32 | Number of sentinel replicas (minimum 3) |
| `resources` | corev1.ResourceRequirements | Resource requirements |
| `customConfigs` | map[string]string | Custom Sentinel configuration |
| `exporter` | *core.Exporter | Monitoring exporter configuration |
| `access` | SentinelInstanceAccess | Access configuration |
| `affinity` | *corev1.Affinity | Pod affinity rules |
| `securityContext` | *corev1.PodSecurityContext | Pod security context |
| `tolerations` | []corev1.Toleration | Pod tolerations |
| `nodeSelector` | map[string]string | Node selector |
| `podAnnotations` | map[string]string | Pod annotations |

## SentinelInstanceAccess

Inherits from `core.InstanceAccess` and adds:

| Field | Type | Description |
|-------|------|-------------|
| `defaultPasswordSecret` | string | Secret containing default user password |
| `externalTLSSecret` | string | External TLS secret (if not provided, operator issues one) |

## SentinelStatus

| Field | Type | Description |
|-------|------|-------------|
| `phase` | SentinelPhase | Current phase of the sentinel |
| `message` | string | Status message |
| `nodes` | []core.ValkeyNode | Sentinel node details |
| `tlsSecret` | string | TLS secret name |

## SentinelPhase Values

- `Creating` - Sentinel is being created
- `Paused` - Sentinel is paused
- `Ready` - Sentinel is ready and monitoring
- `Failed` - Sentinel failed to deploy or is in error state

## Example

```yaml
apiVersion: valkey.buf.red/v1alpha1
kind: Sentinel
metadata:
  name: example-sentinel
  namespace: valkey-system
spec:
  image: valkey/valkey:8.0-alpine
  replicas: 3
  resources:
    requests:
      memory: "256Mi"
      cpu: "100m"
    limits:
      memory: "512Mi"
      cpu: "200m"
  access:
    serviceType: ClusterIP
    defaultPasswordSecret: sentinel-password
  customConfigs:
    sentinel-down-after-milliseconds: "30000"
    sentinel-failover-timeout: "180000"
    sentinel-parallel-syncs: "1"
  exporter:
    image: oliver006/redis_exporter:v1.67.0-alpine
    resources:
      requests:
        memory: "64Mi"
        cpu: "50m"
      limits:
        memory: "128Mi"
        cpu: "100m"
```