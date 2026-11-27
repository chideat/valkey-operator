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
- `db=0,2` - Allow access to databases 0 and 2 only (Valkey 9.1 or later)

### Database Rules

From Valkey 9.1, a user can be limited to some of the instance's databases:

| Rule | Meaning |
|------|---------|
| `db=<id>[,<id>...]` | Allow only the listed databases, for example `db=2,3`. Ids are integers from 0 to 2147483647. A later `db=` replaces an earlier one |
| `alldbs` | Allow every database. This is the default, so the operator leaves it out of the rules it writes |
| `resetdbs` | Allow no database |

The last of these rules in `aclRules` wins, as in `ACL SETUSER`. The operator writes the list sorted and without duplicates: `db=3,2,3` becomes `db=2,3`.

Database rules (`db=` and `resetdbs`) need Valkey 9.1 or later, which the operator checks, because an older Valkey refuses them:

- The admission webhook rejects a `User` with database rules whose instance's image is an older Valkey. When the version cannot be read from the image tag (an image by digest, a custom tag), the `User` is admitted with a warning.
- Before it applies a `User`, the operator checks the lowest version among the instance's image and its running nodes, so a user with database rules is not applied while an upgrade to 9.1 is in progress or when the version is unknown. The `User` is then `Fail`, its `message` says why, and the operator retries until the instance runs 9.1.

A malformed rule, such as `db=`, `db=a`, `db=1,,2` or `db=-1`, is rejected.

Valkey checks a command that names a database, such as `SELECT`, `SWAPDB` or `MOVE`, against these rules, and a command on keys against the current database. `FLUSHDB` empties only the selected database, so `+flushdb` with `db=` lets a user empty only its own databases, while `FLUSHALL`, which empties every database, needs access to all of them. See the [Valkey ACL documentation](https://valkey.io/topics/acl/) for the details.

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

### Application User Limited to Its Databases (Valkey 9.1 or later)
```yaml
spec:
  aclRules: "+@all -@dangerous +flushdb ~app:* &* db=2,3"
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