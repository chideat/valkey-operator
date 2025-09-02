# User API Reference

## Overview

The `User` resource defines Valkey users and Access Control List (ACL) configurations for authentication and authorization.

## Resource Definition

```yaml
apiVersion: valkey.buf.red/v1alpha1
kind: User
metadata:
  name: my-user
  namespace: default
spec:
  # User specification
status:
  # User status
```

## UserSpec

| Field | Type | Description |
|-------|------|-------------|
| `accountType` | AccountType | User account type (system or custom) |
| `arch` | core.Arch | Architecture type (failover, cluster, replica) |
| `username` | string | Username (required) |
| `passwordSecrets` | []string | List of password secret names |
| `aclRules` | string | ACL rules string |
| `instanceName` | string | Instance name (required, 1-63 characters) |

## AccountType Values

- `system` - System account (managed by operator)
- `custom` - Custom user account

## UserStatus

| Field | Type | Description |
|-------|------|-------------|
| `phase` | UserPhase | Current phase of the user |
| `message` | string | Status message |
| `aclRules` | string | Current ACL rules applied to Valkey |

## UserPhase Values

- `Fail` - User creation or update failed
- `Ready` - User is ready and configured
- `Pending` - User configuration is pending

## ACL Rules

ACL rules follow Valkey ACL syntax. Examples:

- `+@all` - Allow all commands
- `-@dangerous` - Deny dangerous commands
- `~*` - Allow access to all keys
- `&*` - Allow access to all channels
- `+@read -@write` - Read-only access

## Example

```yaml
apiVersion: valkey.buf.red/v1alpha1
kind: User
metadata:
  name: app-user
  namespace: valkey-system
spec:
  accountType: custom
  arch: cluster
  username: myapp
  passwordSecrets:
    - app-user-password
  aclRules: "+@read ~app:* &notification:*"
  instanceName: my-cluster
---
apiVersion: v1
kind: Secret
metadata:
  name: app-user-password
  namespace: valkey-system
type: Opaque
data:
  password: bXlzZWNyZXRwYXNzd29yZA== # mysecretpassword (base64 encoded)
```

## Common ACL Patterns

### Read-Only User
```yaml
spec:
  aclRules: "+@read -@write ~*"
```

### Application User with Limited Access
```yaml
spec:
  aclRules: "+@all -@dangerous ~app:* &notifications:*"
```

### Admin User
```yaml
spec:
  aclRules: "+@all ~* &*"
```

### Monitoring User
```yaml
spec:
  aclRules: "+info +ping +client +config|get ~*"
```