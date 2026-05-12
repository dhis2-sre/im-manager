# K3s User Management Scripts

Manage Kubernetes users with ServiceAccounts and RBAC.

## Scripts

- **k3s-user-add.sh** - Create a user
- **k3s-user-del.sh** - Delete a user
- **k3s-user-list.sh** - List managed users

## Usage

### Add User

```bash
# Namespace-scoped access (to specific namespace)
./k3s-user-add.sh <username> <namespace>

# Cluster-wide access (to all namespaces)
./k3s-user-add.sh <username> --cluster-wide
```

Creates a kubeconfig file `<username>-config.yaml`.

### Delete User

```bash
# With username only (auto-detects scope and namespace)
./k3s-user-del.sh <username>

# With namespace
./k3s-user-del.sh <username> <namespace>

# For cluster-wide
./k3s-user-del.sh <username> --cluster-wide
```

### List Users

```bash
# All users in cluster
./k3s-user-list.sh

# Users in specific namespace
./k3s-user-list.sh <namespace>
```

## Notes

- Cluster-wide users are stored in `cluster-users` namespace
- Users are labeled with `dhis2.org/user=true`
- Duplicate usernames are prevented
