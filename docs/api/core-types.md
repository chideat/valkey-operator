# Core Types API Reference

## Overview

Core types are shared structures used across all ValkeyOperator Custom Resources.

## Architecture Types

### Arch

Defines the architecture type for Valkey instances:

- `cluster` - Valkey Cluster mode
- `failover` - Valkey Sentinel-based failover
- `replica` - Primary-replica using etcd for leader election
- `sentinel` - Standalone Sentinel

## Storage Configuration

### Storage

| Field | Type | Description |
|-------|------|-------------|
| `storageClassName` | string | Storage class name for persistent volumes |
| `capacity` | *resource.Quantity | Storage capacity (defaults to 2x memory limit if not set) |
| `accessMode` | corev1.PersistentVolumeAccessMode | Access mode (default: ReadWriteOnce) |
| `retainAfterDeleted` | bool | Whether to retain storage after deletion |

## Access Configuration

### InstanceAccess

| Field | Type | Description |
|-------|------|-------------|
| `serviceType` | corev1.ServiceType | Kubernetes service type (ClusterIP, NodePort, LoadBalancer) |
| `ipFamilyPrefer` | IPFamilyPrefer | IP family preference (IPv4, IPv6, or dual-stack) |
| `loadBalancerIP` | string | Static IP for LoadBalancer services |
| `loadBalancerSourceRanges` | []string | Source IP ranges for LoadBalancer |
| `annotations` | map[string]string | Service annotations |
| `nodePortSequence` | string | NodePort sequence assignment |

## Monitoring Configuration

### Exporter

| Field | Type | Description |
|-------|------|-------------|
| `image` | string | Exporter image |
| `imagePullPolicy` | corev1.PullPolicy | Image pull policy |
| `resources` | *corev1.ResourceRequirements | Resource requirements |
| `securityContext` | *corev1.SecurityContext | Security context |

## Node Information

### ValkeyNode

| Field | Type | Description |
|-------|------|-------------|
| `ip` | string | Node IP address |
| `port` | int32 | Node port |
| `role` | NodeRole | Node role (master, slave, sentinel) |
| `slots` | string | Cluster slots (for cluster mode) |
| `flags` | string | Node flags |
| `masterId` | string | Master node ID (for replicas) |

## Affinity Policies

### AffinityPolicy

Predefined affinity policies for pod scheduling:

- `SoftAntiAffinity` - Prefer different nodes, allow same-node scheduling
- `AntiAffinityInShard` - Enforce different nodes within shards
- `AntiAffinity` - Enforce all pods on different nodes
- `CustomAffinity` - Use custom affinity rules

## Module Configuration

### ValkeyModule

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Module name |
| `path` | string | Module file path |
| `args` | []string | Module arguments (Valkey 8.0+) |

## Examples

### Basic Storage Configuration
```yaml
storage:
  storageClassName: fast-ssd
  capacity: 10Gi
  accessMode: ReadWriteOnce
  retainAfterDeleted: true
```

### Access Configuration with NodePort
```yaml
access:
  serviceType: NodePort
  nodePortSequence: "30000-30002"
  annotations:
    service.beta.kubernetes.io/aws-load-balancer-type: nlb
```

### Exporter Configuration
```yaml
exporter:
  image: oliver006/redis_exporter:v1.67.0-alpine
  resources:
    requests:
      memory: 64Mi
      cpu: 50m
    limits:
      memory: 128Mi
      cpu: 100m
```

### Module Configuration
```yaml
modules:
  - name: RedisJSON
    path: /opt/redis-stack/lib/rejson.so
  - name: RedisSearch
    path: /opt/redis-stack/lib/redisearch.so
    args: ["MAXSEARCHRESULTS", "10000"]
```