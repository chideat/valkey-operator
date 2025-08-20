# Operator Overview

## What is ValkeyOperator?

ValkeyOperator is a production-ready Kubernetes operator that automates the deployment, management, and operation of [Valkey](https://valkey.io/) instances in Kubernetes clusters. Valkey is a high-performance data structure server that serves as a drop-in replacement for Redis.

## Key Features

### Multi-Architecture Support
- **Cluster Mode**: Deploy and manage Valkey cluster instances with automatic sharding and replication
- **Sentinel Mode**: High-availability failover using Valkey Sentinel
- **Standalone Sentinel**: Deploy standalone sentinel instances for monitoring external Valkey deployments
- **RDS-Style Instances**: Simplified managed instances with reduced configuration complexity

### Production-Ready Features
- **High Availability**: Automatic failover and recovery mechanisms
- **Scaling**: Online scale up/down operations without downtime
- **Version Upgrades**: Graceful rolling updates with minimal service disruption
- **Persistent Storage**: Configurable persistent volumes with retention policies
- **Security**: TLS encryption, ACL support, network policies
- **Monitoring**: Built-in Prometheus exporter integration

### Kubernetes Native
- **Custom Resource Definitions (CRDs)**: Native Kubernetes API integration
- **Controller Pattern**: Declarative configuration with continuous reconciliation
- **RBAC Integration**: Fine-grained permission control
- **Service Discovery**: Automatic service creation and management
- **Resource Management**: CPU/memory limits and requests

## Architecture

### Operator Components

```
┌─────────────────────────────────────────────────────────────┐
│                    Kubernetes Cluster                      │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐    ┌─────────────────┐                │
│  │  Valkey         │    │   Valkey        │                │
│  │  Operator       │    │   Instances     │                │
│  │  Controller     │    │                 │                │
│  │                 │    │ ┌─────────────┐ │                │
│  │ ┌─────────────┐ │    │ │   Cluster   │ │                │
│  │ │ Reconciler  │ │    │ │  Failover   │ │                │
│  │ │   Engine    │ │◄──►│ │  Sentinel   │ │                │
│  │ └─────────────┘ │    │ │    User     │ │                │
│  │                 │    │ └─────────────┘ │                │
│  └─────────────────┘    └─────────────────┘                │
└─────────────────────────────────────────────────────────────┘
```

### Controller Responsibilities

1. **Resource Management**: Creates and manages Kubernetes resources (StatefulSets, Services, ConfigMaps, Secrets)
2. **Configuration Synchronization**: Ensures Valkey configurations match desired state
3. **Health Monitoring**: Continuously monitors cluster health and triggers recovery actions
4. **Scaling Operations**: Handles horizontal scaling of clusters and failover groups
5. **Version Management**: Orchestrates rolling updates and version upgrades

## Supported Valkey Versions

| Valkey Version | Kubernetes Versions | Status |
|----------------|-------------------|---------|
| 7.2.x | 1.31, 1.32 | ✅ Supported |
| 8.0.x | 1.31, 1.32 | ✅ Supported |
| 8.1.x | 1.31, 1.32 | ✅ Supported |

## Use Cases

### Development and Testing
- Quick setup of Valkey instances for application development
- Ephemeral instances with easy cleanup
- Multiple isolated environments

### Production Workloads
- High-availability data stores with automatic failover
- Distributed caching layers with consistent hashing
- Session stores for web applications
- Message queues and pub/sub systems

### Multi-tenant Environments
- Isolated Valkey instances per tenant
- Resource quotas and limits
- Network isolation with policies

## Comparison with Alternatives

### vs. Helm Charts
- **Operator**: Continuous reconciliation, automated operations, self-healing
- **Helm**: One-time deployment, manual operations, limited automation

### vs. Managed Services
- **Operator**: Full control, Kubernetes-native, cost-effective
- **Managed**: Less control, vendor lock-in, potentially higher cost

### vs. Manual Deployment
- **Operator**: Automated, consistent, best practices built-in
- **Manual**: Error-prone, time-consuming, requires deep expertise

## Getting Started

### Quick Installation

```bash
# Install the operator
kubectl apply -k https://github.com/chideat/valkey-operator/config/default

# Deploy a simple cluster
cat <<EOF | kubectl apply -f -
apiVersion: valkey.buf.red/v1alpha1
kind: Cluster
metadata:
  name: quickstart
  namespace: default
spec:
  image: valkey/valkey:8.0-alpine
  replicas:
    shards: 3
    replicasOfShard: 1
EOF

# Check status
kubectl get cluster quickstart -w
```

### Next Steps

1. Read the [User Guide](../guides/user-guide.md) for detailed usage instructions
2. Explore [API Reference](../api/index.md) for configuration options
3. Check [Examples](../examples/) for common deployment patterns
4. Review [Best Practices](../guides/best-practices.md) for production deployments

## Community and Support

- **GitHub Repository**: [chideat/valkey-operator](https://github.com/chideat/valkey-operator)
- **Issues and Bug Reports**: [GitHub Issues](https://github.com/chideat/valkey-operator/issues)
- **Contributing**: See [CONTRIBUTING.md](../../CONTRIBUTING.md)
- **License**: [Apache 2.0](../../LICENSE)