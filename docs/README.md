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

## Unified Valkey Resource

ValkeyOperator uses a unified `Valkey` resource to manage all deployment architectures. You can specify the desired architecture using the `arch` field.

- `arch: replica` - Standalone Valkey instance.
- `arch: cluster` - Valkey cluster with automatic sharding.
- `arch: failover` - High-availability setup with Sentinel.

Here is an example of a Valkey cluster:
```yaml
apiVersion: rds.valkey.buf.red/v1alpha1
kind: Valkey
metadata:
  name: my-valkey-cluster
spec:
  arch: cluster
  version: "8.0"
  replicas:
    shards: 3
    replicasOfShard: 1
```

## Features

- ✅ **Unified API** - Manage all Valkey architectures with a single CRD.
- ✅ **Multi-Architecture Support** - `replica`, `cluster`, and `failover` modes.
- ✅ **High Availability** - Automatic failover and recovery.
- ✅ **Horizontal Scaling** - Online scale up/down operations.
- ✅ **Version Upgrades** - Graceful rolling updates.
- ✅ **Persistent Storage** - Configurable storage with retention.
- ✅ **Security** - TLS encryption and ACL support.
- ✅ **Monitoring** - Built-in Prometheus exporter.
- ✅ **IPv4/IPv6** - Dual-stack networking support.
- ✅ **Node Scheduling** - Affinity, tolerations, node selectors.

## Supported Versions

| Valkey Version | Kubernetes | Status |
|---------------|------------|---------|
| 7.2.x | 1.31, 1.32 | ✅ Supported |
| 8.0.x | 1.31, 1.32 | ✅ Supported |
| 8.1.x | 1.31, 1.32 | ✅ Supported |

## Examples by Use Case

| Use Case | Example | Description |
|----------|---------|-------------|
| **Standalone** | [Valkey Standalone](./examples/basic/standalone.yaml) | Basic standalone Valkey instance |
| **Cluster** | [Valkey Cluster](./examples/basic/cluster.yaml) | Valkey cluster with sharding |
| **High Availability** | [Valkey Failover](./examples/basic/failover.yaml) | Sentinel-based failover |
| **User Management** | [ACL Users](./examples/users/acl-users.yaml) | User and ACL configuration |
| **Monitoring** | [With Monitoring](./examples/advanced/monitoring.yaml) | Prometheus integration |

## Quick Installation

```bash
# Install the operator
kubectl apply -k https://github.com/chideat/valkey-operator/config/default

# Deploy a standalone Valkey instance
kubectl apply -f docs/examples/basic/standalone.yaml

# Check status
kubectl get valkey valkey-standalone -w
```

## Getting Help

- 📖 **Documentation**: You're reading it!
- 🐛 **Bug Reports**: [GitHub Issues](https://github.com/chideat/valkey-operator/issues)
- 💡 **Feature Requests**: [GitHub Issues](https://github.com/chideat/valkey-operator/issues)
- 🤝 **Contributing**: [CONTRIBUTING.md](../CONTRIBUTING.md)

## License

ValkeyOperator is licensed under the [Apache 2.0 License](../LICENSE).