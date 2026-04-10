# Valkey Operator Examples

This directory contains example configurations for various Valkey Operator use cases.

## Basic Examples

### [Valkey Standalone](./basic/standalone.yaml)
A simple standalone Valkey instance using the replica architecture.

### [Valkey Cluster](./basic/cluster.yaml)
A 3-shard Valkey cluster with one replica per shard.

### [Valkey Failover](./basic/failover.yaml)
A Sentinel-based failover setup with automatic high availability.

### [Standalone Sentinel](./basic/standalone-sentinel.yaml)
Standalone sentinel for monitoring external Valkey instances.

## Production Examples

### [Production Cluster](./production/cluster.yaml)
A production-ready Valkey cluster with persistent storage, monitoring, anti-affinity, and custom configuration.

## User Management Examples

### [ACL Users](./users/acl-users.yaml)
Various user configurations with different ACL rules (read-only, application, cache, monitoring).

## Getting Started

1. Choose an example that matches your use case
2. Customize the configuration for your environment
3. Apply the configuration:
   ```bash
   kubectl apply -f <example-file>.yaml
   ```
4. Monitor the deployment:
   ```bash
   kubectl get valkey <resource-name> -w
   ```

## Directory Structure

```
examples/
├── basic/              # Simple examples for getting started
├── production/         # Production-ready configurations
├── users/             # User and ACL management examples
└── README.md          # This file
```