# ValkeyOperator Best Practices

This guide provides best practices for deploying and managing ValkeyOperator in production environments.

## Table of Contents

1. [Production Deployment](#production-deployment)
2. [Resource Planning](#resource-planning)
3. [Security Configuration](#security-configuration)
4. [High Availability](#high-availability)
5. [Monitoring and Observability](#monitoring-and-observability)
6. [Backup and Recovery](#backup-and-recovery)
7. [Performance Optimization](#performance-optimization)
8. [Operational Procedures](#operational-procedures)

## Production Deployment

### Namespace Isolation

Always deploy production workloads in dedicated namespaces:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: valkey-production
  labels:
    tier: production
    team: platform
```

### Resource Quotas

Set appropriate resource quotas to prevent resource exhaustion:

```yaml
apiVersion: v1
kind: ResourceQuota
metadata:
  name: valkey-quota
  namespace: valkey-production
spec:
  hard:
    requests.cpu: "20"
    requests.memory: 80Gi
    limits.cpu: "40"
    limits.memory: 160Gi
    persistentvolumeclaims: "10"
```

### Network Policies

Implement network policies for security:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: valkey-netpol
  namespace: valkey-production
spec:
  podSelector:
    matchLabels:
      app: valkey
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              name: application
      ports:
        - protocol: TCP
          port: 6379
```

## Resource Planning

### CPU Allocation

- **Development**: 250m - 500m CPU
- **Testing**: 500m - 1000m CPU  
- **Production**: 2000m - 4000m CPU

### Memory Allocation

- Reserve 20-30% memory overhead for OS and Valkey operations
- Set memory limits 50-100% higher than memory requests
- Use memory-optimized nodes for large datasets

```yaml
resources:
  requests:
    memory: "4Gi"
    cpu: "2000m"
  limits:
    memory: "8Gi"      # 2x requests for burst capacity
    cpu: "4000m"       # 2x requests for burst capacity
```

### Storage Planning

- Use SSD storage for production workloads
- Size storage at 2-3x memory allocation for RDB snapshots
- Enable storage retention for data protection

```yaml
storage:
  storageClassName: fast-ssd
  capacity: 150Gi              # 3x memory for snapshots
  retainAfterDeleted: true     # Protect against accidental deletion
```

## Security Configuration

### TLS Encryption

Always enable TLS for production:

```yaml
access:
  tls:
    enabled: true
    secretName: valkey-tls-cert
customConfigs:
  tls-port: "6380"
  port: "0"                    # Disable non-TLS port
```

### ACL Configuration

Use ACL instead of simple passwords:

```yaml
customConfigs:
  requirepass: ""              # Disable password auth
  aclfile: "/data/users.acl"   # Use ACL file
```

### Pod Security Context

Configure secure pod contexts:

```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 999
  runAsGroup: 999
  fsGroup: 999
  seccompProfile:
    type: RuntimeDefault
```

## High Availability

### Cluster Configuration

For production clusters:

```yaml
replicas:
  shards: 6                    # Even number of shards
  replicasOfShard: 2          # At least 2 replicas per shard
```

### Anti-Affinity Rules

Ensure pods are distributed across nodes:

```yaml
affinityPolicy: AntiAffinity   # Strict anti-affinity
```

### Failover Configuration

For sentinel-based setups:

```yaml
sentinel:
  replicas: 3                  # Always odd number
  quorum: 2                    # Majority quorum
  monitorConfig:
    down-after-milliseconds: "30000"
    failover-timeout: "180000"
    parallel-syncs: "1"
```

### Multi-Zone Deployment

Use zone anti-affinity for regional HA:

```yaml
customAffinity:
  podAntiAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
      - labelSelector:
          matchLabels:
            app: valkey
        topologyKey: topology.kubernetes.io/zone
```

## Monitoring and Observability

### Prometheus Integration

Enable monitoring with proper resource allocation:

```yaml
exporter:
  resources:
    requests:
      memory: "64Mi"
      cpu: "50m"
    limits:
      memory: "128Mi"
      cpu: "100m"
```

### ServiceMonitor Configuration

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: valkey-monitoring
spec:
  selector:
    matchLabels:
      app: valkey
  endpoints:
    - port: exporter
      interval: 30s
      scrapeTimeout: 10s
```

### Key Metrics to Monitor

- Memory usage vs limits
- CPU utilization
- Connection count
- Commands per second
- Replication lag
- Cluster slot distribution

### Alerting Rules

```yaml
groups:
  - name: valkey.rules
    rules:
      - alert: ValkeyHighMemoryUsage
        expr: valkey_memory_used_bytes / valkey_memory_max_bytes > 0.8
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Valkey memory usage is high"
      
      - alert: ValkeyDown
        expr: up{job="valkey"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Valkey instance is down"
```

## Backup and Recovery

### RDB Configuration

Configure periodic snapshots:

```yaml
customConfigs:
  save: "900 1 300 10 60 10000"     # Multiple save points
  stop-writes-on-bgsave-error: "yes"
  rdbcompression: "yes"
  rdbchecksum: "yes"
```

### External Backup Strategy

Use tools like `valkey-dump-go` for external backups:

```bash
# Example backup script
kubectl exec valkey-cluster-0 -- valkey-cli --rdb /tmp/backup.rdb
kubectl cp valkey-cluster-0:/tmp/backup.rdb ./backup-$(date +%Y%m%d).rdb
```

### Point-in-Time Recovery

Combine RDB snapshots with AOF for better recovery:

```yaml
customConfigs:
  appendonly: "yes"
  appendfsync: "everysec"
  auto-aof-rewrite-percentage: "100"
  auto-aof-rewrite-min-size: "64mb"
```

## Performance Optimization

### Memory Configuration

```yaml
customConfigs:
  maxmemory-policy: "allkeys-lru"
  maxmemory-samples: "10"
  hash-max-ziplist-entries: "512"
  hash-max-ziplist-value: "64"
  list-max-ziplist-size: "-2"
  set-max-intset-entries: "512"
  zset-max-ziplist-entries: "128"
  zset-max-ziplist-value: "64"
```

### Network Optimization

```yaml
customConfigs:
  tcp-keepalive: "300"
  tcp-backlog: "511"
  timeout: "300"
  hz: "10"
  dynamic-hz: "yes"
```

### Node Affinity

Use memory-optimized nodes:

```yaml
nodeSelector:
  node-type: "memory-optimized"
  storage-type: "nvme-ssd"
```

## Operational Procedures

### Scaling Operations

Scale gradually and monitor:

```bash
# Scale up shards (cluster mode)
kubectl patch valkey my-cluster --type='merge' -p='{"spec":{"replicas":{"shards":9}}}'

# Monitor scaling progress
kubectl get valkey my-cluster -w
```

### Version Upgrades

```bash
# UP version
kubectl patch valkey my-cluster --type='merge' -p='{"spec":{"version":"8.1"}}'

# Monitor scaling progress
kubectl get valkey my-cluster -w
```

### Troubleshooting Commands

```bash
# Check cluster status
kubectl describe cluster my-cluster

# View pod logs
kubectl logs -l app=valkey-cluster -c valkey --tail=100

# Connect to Valkey CLI
kubectl exec -it valkey-cluster-0 -- valkey-cli

# Check cluster nodes
kubectl exec -it valkey-cluster-0 -- valkey-cli cluster nodes

# Monitor real-time stats
kubectl exec -it valkey-cluster-0 -- valkey-cli --stat
```

### Maintenance Windows

Schedule maintenance during low-traffic periods:

1. Enable maintenance mode if supported
2. Perform operations shard by shard
3. Verify cluster health after each change
4. Document all changes

### Disaster Recovery

Maintain runbooks for common scenarios:

1. Node failure recovery
2. Zone outage recovery
3. Complete cluster rebuild
4. Data corruption recovery

## Configuration Validation

Use validation tools before applying changes:

```bash
# Validate YAML syntax
kubectl apply --dry-run=client -f cluster.yaml

# Validate with operator
kubectl apply --dry-run=server -f cluster.yaml
```
