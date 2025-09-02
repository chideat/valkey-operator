# ValkeyOperator Examples

This directory contains example configurations for various ValkeyOperator use cases.

## Basic Examples

### [Valkey Standalone](./basic/standalone.yaml)
A simple standalone Valkey instance.

### [Valkey Cluster](./basic/cluster.yaml)
A simple Valkey cluster.

### [Valkey Failover](./basic/failover.yaml)
A simple Valkey failover setup with Sentinel.

### [Standalone Sentinel](./basic/standalone-sentinel.yaml)
Standalone sentinel for monitoring external Valkey instances.

## Production Examples

### [Production Cluster](./production/cluster.yaml)
A production-ready Valkey cluster with persistence, monitoring, and security.

## Advanced Examples

### [Multi-Tenant Setup](./advanced/multi-tenant.yaml)
Multiple isolated Valkey instances with network policies.

### [Custom Modules](./advanced/custom-modules.yaml)
Valkey instance with custom modules loaded.

### [Monitoring Integration](./advanced/monitoring.yaml)
Complete monitoring setup with Prometheus and Grafana.

## RDS Examples

### [RDS-Style Instance](./rds/simple-rds.yaml)
Simplified RDS-style Valkey instance.

## User Management Examples

### [ACL Users](./users/acl-users.yaml)
Various user configurations with different ACL rules.

### [Application Users](./users/app-users.yaml)
User configurations for common application patterns.

## Getting Started

1. Choose an example that matches your use case
2. Customize the configuration for your environment
3. Apply the configuration:
   ```bash
   kubectl apply -f <example-file>.yaml
   ```
4. Monitor the deployment:
   ```bash
   kubectl get <resource-type> <resource-name> -w
   ```

## Directory Structure

```
examples/
├── basic/              # Simple examples for getting started
├── production/         # Production-ready configurations
├── advanced/           # Advanced use cases and integrations
├── rds/               # RDS-style examples
├── users/             # User and ACL management examples
└── README.md          # This file
```