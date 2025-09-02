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
| `annotations` | map[string]string | Annotations for the storage service |
| `storageClassName` | *string | Storage class name for persistent volumes |
| `capacity` | *resource.Quantity | Storage capacity (defaults to 2x memory limit if not set) |
| `accessMode` | corev1.PersistentVolumeAccessMode | Access mode (default: ReadWriteOnce) |
| `retainAfterDeleted` | bool | Whether to retain storage after deletion |

## Access Configuration

### InstanceAccess

| Field | Type | Description |
|-------|------|-------------|
| `serviceType` | corev1.ServiceType | Kubernetes service type (ClusterIP, NodePort, LoadBalancer) |
| `annotations` | map[string]string | Service annotations |
| `ipFamilyPrefer` | corev1.IPFamily | IP family preference (IPv4, IPv6) |
| `ports` | string | NodePort sequence assignment (e.g., "30000:30000,30001:30001") |
| `enableTLS` | bool | Enable TLS for external access |
| `certIssuer` | string | Certificate issuer for TLS |
| `certIssuerType` | string | Certificate issuer type (ClusterIssuer or Issuer) |

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
| `id` | string | Valkey cluster node ID |
| `shardId` | string | Cluster shard ID of the node |
| `role` | NodeRole | Node role (master, replica, sentinel) |
| `ip` | string | Node IP address |
| `port` | string | Node port |
| `slots` | string | Cluster slots (for cluster mode) |
| `masterRef` | string | Master node ID (for replicas) |
| `statefulSet` | string | StatefulSet name of the pod |
| `podName` | string | Pod name |
| `nodeName` | string | Kubernetes node name where the pod is running |

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
