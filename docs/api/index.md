# ValkeyOperator API Reference

This document contains the complete API reference for ValkeyOperator Custom Resource Definitions (CRDs).

## Overview

ValkeyOperator provides several Custom Resource Definitions for managing Valkey instances:

- **[Valkey](./rds-v1alpha1-valkey.md)** - Unified Valkey instances for Cluster and Failover modes
- **[Sentinel](./v1alpha1-sentinel.md)** - Deploy and manage standalone Valkey sentinel instances
- **[User](./v1alpha1-user.md)** - Manage Valkey users and ACL configurations

## API Groups

### rds.valkey.buf.red/v1alpha1

This API group contains the main Valkey Custom Resource:

- `Valkey` - RDS-style Valkey instances with simplified configuration

## Core Types

See [Core API Reference](./core-types.md) for shared types and structures used across all Custom Resources.

## Quick Reference

| Resource | Kind | API Version | Description |
|----------|------|-------------|-------------|
| Valkey | `Valkey` | `rds.valkey.buf.red/v1alpha1` | Unified Valkey instance |
| Sentinel | `Sentinel` | `valkey.buf.red/v1alpha1` | Standalone sentinel instance |
| User | `User` | `valkey.buf.red/v1alpha1` | Valkey user/ACL management |

For detailed field documentation and examples, see the individual resource documentation pages.