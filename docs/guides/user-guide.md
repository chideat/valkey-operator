# ValkeyOperator User Guide

## Table of Contents

1. [Getting Started](#getting-started)
2. [Installing the Operator](#installing-the-operator)
3. [Basic Usage](#basic-usage)
4. [Architecture Overview](#architecture-overview)
5. [Configuration Examples](#configuration-examples)
6. [Monitoring](#monitoring)
7. [Security](#security)
8. [Troubleshooting](#troubleshooting)

## Getting Started

ValkeyOperator is a Kubernetes operator that automates the deployment and management of Valkey instances. It supports multiple architectures:

- **Cluster Mode** - High-performance distributed Valkey clusters
- **Failover Mode** - High-availability using Sentinel
- **Sentinel Mode** - Standalone sentinel instances
- **RDS Mode** - Simplified managed instances

## Installing the Operator

### Prerequisites

- Kubernetes 1.31+ or 1.32+
- kubectl configured to access your cluster
- Cluster administrator permissions

### Installation Methods

#### Method 1: Using Kustomize (Recommended)

```bash
# Install CRDs and operator
kubectl apply -k https://github.com/chideat/valkey-operator/config/default
```

#### Method 2: Using Manifests

```bash
# Download and apply manifests
curl -L https://github.com/chideat/valkey-operator/releases/latest/download/manifests.yaml | kubectl apply -f -
```

### Verify Installation

```bash
# Check if the operator is running
kubectl get pods -n valkey-operator-system

# Verify CRDs are installed
kubectl get crd | grep valkey
```

## Basic Usage

### Deploy a Simple Cluster

```yaml
apiVersion: valkey.buf.red/v1alpha1
kind: Cluster
metadata:
  name: my-cluster
  namespace: default
spec:
  image: valkey/valkey:8.0-alpine
  replicas:
    shards: 3
    replicasOfShard: 1
  resources:
    requests:
      memory: 1Gi
      cpu: 500m
    limits:
      memory: 2Gi
      cpu: 1000m
```

```bash
kubectl apply -f cluster.yaml
kubectl get cluster my-cluster -w
```

### Deploy a Failover Instance

```yaml
apiVersion: valkey.buf.red/v1alpha1
kind: Failover
metadata:
  name: my-failover
  namespace: default
spec:
  image: valkey/valkey:8.0-alpine
  replicas: 3
  sentinel:
    replicas: 3
    quorum: 2
  resources:
    requests:
      memory: 512Mi
      cpu: 250m
```

```bash
kubectl apply -f failover.yaml
kubectl get failover my-failover -w
```

## Architecture Overview

### Cluster Architecture

```
┌─────────────────────────────────────────┐
│              Valkey Cluster             │
├─────────────┬─────────────┬─────────────┤
│   Shard 1   │   Shard 2   │   Shard 3   │
│ Master + 1R │ Master + 1R │ Master + 1R │
│ Slots 0-5k  │ Slots 5k-10k│ Slots 10k-16k│
└─────────────┴─────────────┴─────────────┘
```

### Failover Architecture

```
┌─────────────────────────────────────────┐
│              Sentinel Cluster           │
│           (3 Sentinel Nodes)            │
└─────────────────┬───────────────────────┘
                  │ Monitors
┌─────────────────▼───────────────────────┐
│              Valkey Failover            │
│         Master + 2 Replicas             │
└─────────────────────────────────────────┘
```

## Configuration Examples

### Production Cluster with Storage

```yaml
apiVersion: valkey.buf.red/v1alpha1
kind: Cluster
metadata:
  name: prod-cluster
  namespace: valkey-production
spec:
  image: valkey/valkey:8.0-alpine
  replicas:
    shards: 6
    replicasOfShard: 2
  resources:
    requests:
      memory: 4Gi
      cpu: 2000m
    limits:
      memory: 8Gi
      cpu: 4000m
  storage:
    storageClassName: fast-ssd
    capacity: 50Gi
    retainAfterDeleted: true
  access:
    serviceType: LoadBalancer
  customConfigs:
    maxmemory-policy: allkeys-lru
    tcp-keepalive: "300"
    save: "900 1 300 10 60 10000"
  exporter:
    image: oliver006/redis_exporter:v1.67.0-alpine
  affinityPolicy: AntiAffinity
  tolerations:
    - key: valkey
      operator: Equal
      value: "true"
      effect: NoSchedule
```

### Secure Cluster with TLS and ACL

```yaml
apiVersion: valkey.buf.red/v1alpha1
kind: Cluster
metadata:
  name: secure-cluster
  namespace: valkey-secure
spec:
  image: valkey/valkey:8.0-alpine
  replicas:
    shards: 3
    replicasOfShard: 1
  access:
    serviceType: ClusterIP
    tls:
      enabled: true
      secretName: valkey-tls-cert
  customConfigs:
    requirepass: "false"  # Use ACL instead
    aclfile: "/data/users.acl"
---
apiVersion: valkey.buf.red/v1alpha1
kind: User
metadata:
  name: app-user
  namespace: valkey-secure
spec:
  accountType: custom
  arch: cluster
  username: myapp
  passwordSecrets:
    - app-user-password
  aclRules: "+@read +@write -@dangerous ~app:* &notifications:*"
  instanceName: secure-cluster
```

## Monitoring

### Prometheus Integration

The operator includes built-in Prometheus exporter support:

```yaml
spec:
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

### ServiceMonitor for Prometheus Operator

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: valkey-cluster
  namespace: valkey-system
spec:
  selector:
    matchLabels:
      app: valkey-cluster
      role: exporter
  endpoints:
    - port: exporter
      interval: 30s
      path: /metrics
```

## Security

### Network Policies

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: valkey-cluster-netpol
  namespace: valkey-system
spec:
  podSelector:
    matchLabels:
      app: valkey-cluster
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
  egress:
    - to: []
      ports:
        - protocol: TCP
          port: 6379
```

### Pod Security Context

```yaml
spec:
  securityContext:
    runAsNonRoot: true
    runAsUser: 999
    runAsGroup: 999
    fsGroup: 999
    seccompProfile:
      type: RuntimeDefault
```

## Troubleshooting

### Common Issues

#### Cluster Not Ready

```bash
# Check cluster status
kubectl describe cluster my-cluster

# Check pod logs
kubectl logs -l app=valkey-cluster -c valkey

# Check operator logs
kubectl logs -n valkey-operator-system deployment/valkey-operator-controller-manager
```

#### Storage Issues

```bash
# Check PVC status
kubectl get pvc -l app=valkey-cluster

# Check storage class
kubectl get storageclass

# Check node storage capacity
kubectl describe nodes
```

#### Network Connectivity

```bash
# Test connectivity between pods
kubectl exec -it <pod-name> -- valkey-cli cluster nodes

# Check service endpoints
kubectl get endpoints -l app=valkey-cluster

# Test external access
kubectl port-forward service/my-cluster 6379:6379
```

### Debug Mode

Enable debug logging in the operator:

```yaml
spec:
  containers:
    - name: manager
      args:
        - --zap-log-level=debug
```

### Performance Tuning

#### Memory Settings

```yaml
customConfigs:
  maxmemory: "1gb"
  maxmemory-policy: "allkeys-lru"
  maxmemory-samples: "10"
```

#### Network Settings

```yaml
customConfigs:
  tcp-keepalive: "300"
  tcp-backlog: "511"
  timeout: "300"
```

#### Persistence Settings

```yaml
customConfigs:
  save: "900 1 300 10 60 10000"
  stop-writes-on-bgsave-error: "yes"
  rdbcompression: "yes"
```