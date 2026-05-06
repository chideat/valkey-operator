# Valkey Operator API Reference

This document contains the complete API reference for Valkey Operator Custom Resource Definitions (CRDs).

## Overview

Valkey Operator provides several Custom Resource Definitions for managing Valkey instances:

- **[Valkey](./rds-v1alpha1-valkey.md)** - Unified high-level resource for deploying Cluster, Failover, and Replica instances
- **[Cluster](./v1alpha1-reference.md)** - Fine-grained control over Valkey Cluster instances
- **[Failover](./v1alpha1-reference.md)** - Fine-grained control over Sentinel-based failover instances
- **[Sentinel](./v1alpha1-sentinel.md)** - Deploy and manage standalone Valkey Sentinel instances
- **[User](./v1alpha1-user.md)** - Manage Valkey users and ACL configurations

## API Groups

### rds.valkey.buf.red/v1alpha1

High-level simplified API — recommended for most users:

- `Valkey` - Unified resource supporting cluster, failover, and replica architectures

### valkey.buf.red/v1alpha1

Fine-grained API for advanced use cases:

- `Cluster` - Valkey Cluster with direct control over sharding and replication
- `Failover` - Sentinel-based failover with direct control over sentinel settings
- `Sentinel` - Standalone sentinel instances
- `User` - Valkey ACL user management

## Core Types

See [Core API Reference](./core-types.md) for shared types and structures used across all Custom Resources.

## Quick Reference

| Resource | Kind | API Version | Description |
|----------|------|-------------|-------------|
| Valkey | `Valkey` | `rds.valkey.buf.red/v1alpha1` | Unified Valkey instance (recommended) |
| Cluster | `Cluster` | `valkey.buf.red/v1alpha1` | Valkey Cluster (advanced) |
| Failover | `Failover` | `valkey.buf.red/v1alpha1` | Sentinel-based failover (advanced) |
| Sentinel | `Sentinel` | `valkey.buf.red/v1alpha1` | Standalone sentinel instance |
| User | `User` | `valkey.buf.red/v1alpha1` | Valkey user/ACL management |

For detailed field documentation and examples, see the individual resource documentation pages.