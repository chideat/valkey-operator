# ValkeyOperator Documentation

Welcome to the ValkeyOperator documentation. This guide will help you deploy, configure, and manage Valkey instances in Kubernetes using the ValkeyOperator.

## Quick Start

1. **[Operator Overview](./guides/operator-overview.md)** - Understanding ValkeyOperator architecture and capabilities
2. **[User Guide](./guides/user-guide.md)** - Complete installation and usage guide
3. **[Examples](./examples/)** - Ready-to-use configuration examples

## API Reference

- **[API Overview](./api/index.md)** - Complete API reference overview
- **[Cluster API](./api/v1alpha1-cluster.md)** - Valkey cluster configuration
- **[Failover API](./api/v1alpha1-failover.md)** - Sentinel-based failover configuration
- **[Sentinel API](./api/v1alpha1-sentinel.md)** - Standalone sentinel configuration
- **[User API](./api/v1alpha1-user.md)** - User and ACL management
- **[Core Types](./api/core-types.md)** - Shared types and structures

## Architecture Support

ValkeyOperator supports multiple Valkey deployment architectures:

### 🔧 **Cluster Mode**
High-performance distributed clusters with automatic sharding and replication.

```yaml
apiVersion: valkey.buf.red/v1alpha1
kind: Cluster
metadata:
  name: my-cluster
spec:
  replicas:
    shards: 3
    replicasOfShard: 1
```

### 🔄 **Failover Mode** 
High-availability using Valkey Sentinel for automatic failover.

```yaml
apiVersion: valkey.buf.red/v1alpha1
kind: Failover
metadata:
  name: my-failover
spec:
  replicas: 3
  sentinel:
    replicas: 3
    quorum: 2
```

### 👁️ **Sentinel Mode**
Standalone sentinel instances for monitoring external Valkey deployments.

```yaml
apiVersion: valkey.buf.red/v1alpha1
kind: Sentinel
metadata:
  name: my-sentinel
spec:
  replicas: 3
```

### 🔐 **User Management**
ACL-based user authentication and authorization.

```yaml
apiVersion: valkey.buf.red/v1alpha1
kind: User
metadata:
  name: app-user
spec:
  username: myapp
  aclRules: "+@read +@write -@dangerous ~app:*"
```

## Features

- ✅ **Multi-Architecture Support** - Cluster, Failover, Sentinel modes
- ✅ **High Availability** - Automatic failover and recovery
- ✅ **Horizontal Scaling** - Online scale up/down operations
- ✅ **Version Upgrades** - Graceful rolling updates
- ✅ **Persistent Storage** - Configurable storage with retention
- ✅ **Security** - TLS encryption and ACL support
- ✅ **Monitoring** - Built-in Prometheus exporter
- ✅ **IPv4/IPv6** - Dual-stack networking support
- ✅ **Node Scheduling** - Affinity, tolerations, node selectors

## Supported Versions

| Valkey Version | Kubernetes | Status |
|---------------|------------|---------|
| 7.2.x | 1.31, 1.32 | ✅ Supported |
| 8.0.x | 1.31, 1.32 | ✅ Supported |
| 8.1.x | 1.31, 1.32 | ✅ Supported |

## Examples by Use Case

| Use Case | Example | Description |
|----------|---------|-------------|
| **Development** | [Simple Cluster](./examples/basic/simple-cluster.yaml) | Basic cluster for development |
| **Production** | [Production Cluster](./examples/production/production-cluster.yaml) | Production-ready cluster with HA |
| **High Availability** | [HA Failover](./examples/basic/simple-failover.yaml) | Sentinel-based failover |
| **Security** | [Secure Cluster](./examples/production/secure-cluster.yaml) | TLS and ACL configuration |
| **Monitoring** | [With Monitoring](./examples/advanced/monitoring.yaml) | Prometheus integration |

## Quick Installation

```bash
# Install the operator
kubectl apply -k https://github.com/chideat/valkey-operator/config/default

# Deploy a cluster
kubectl apply -f docs/examples/basic/simple-cluster.yaml

# Check status
kubectl get cluster simple-cluster -w
```

## Getting Help

- 📖 **Documentation**: You're reading it!
- 🐛 **Bug Reports**: [GitHub Issues](https://github.com/chideat/valkey-operator/issues)
- 💡 **Feature Requests**: [GitHub Issues](https://github.com/chideat/valkey-operator/issues)
- 🤝 **Contributing**: [CONTRIBUTING.md](../CONTRIBUTING.md)

## License

ValkeyOperator is licensed under the [Apache 2.0 License](../LICENSE).