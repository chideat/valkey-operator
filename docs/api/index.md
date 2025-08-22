# ValkeyOperator API Reference

This document contains the complete API reference for ValkeyOperator Custom Resource Definitions (CRDs).

## Overview

ValkeyOperator provides several Custom Resource Definitions for managing Valkey instances:

- **[Cluster](./v1alpha1-cluster.md)** - Deploy and manage Valkey cluster instances
- **[Failover](./v1alpha1-failover.md)** - Deploy and manage Valkey sentinel-based failover instances  
- **[Sentinel](./v1alpha1-sentinel.md)** - Deploy and manage standalone Valkey sentinel instances
- **[User](./v1alpha1-user.md)** - Manage Valkey users and ACL configurations
- **[RDS Valkey](./rds-v1alpha1-valkey.md)** - RDS-style Valkey instances

## API Groups

### valkey.buf.red/v1alpha1

This API group contains the main Valkey Custom Resources:

- `Cluster` - Valkey cluster mode instances
- `Failover` - Valkey failover (sentinel-managed) instances  
- `Sentinel` - Standalone sentinel instances
- `User` - Valkey user and ACL management

### rds.valkey.buf.red/v1alpha1

This API group contains RDS-style Valkey instances:

- `Valkey` - RDS-style Valkey instances with simplified configuration

## Core Types

See [Core API Reference](./core-types.md) for shared types and structures used across all Custom Resources.

## Quick Reference

| Resource | Kind | API Version | Description |
|----------|------|-------------|-------------|
| Cluster | `Cluster` | `valkey.buf.red/v1alpha1` | Valkey cluster instance |
| Failover | `Failover` | `valkey.buf.red/v1alpha1` | Sentinel-managed failover instance |
| Sentinel | `Sentinel` | `valkey.buf.red/v1alpha1` | Standalone sentinel instance |
| User | `User` | `valkey.buf.red/v1alpha1` | Valkey user/ACL management |
| RDS Valkey | `Valkey` | `rds.valkey.buf.red/v1alpha1` | RDS-style instance |

For detailed field documentation and examples, see the individual resource documentation pages.