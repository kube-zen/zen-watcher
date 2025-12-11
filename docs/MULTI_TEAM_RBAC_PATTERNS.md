# Multi-Team RBAC Patterns

This document describes patterns for using zen-watcher in multi-team Kubernetes clusters.

## Overview

zen-watcher supports multiple teams in the same cluster through:
- Namespace-scoped Ingesters and Observations
- RBAC for fine-grained access control
- Namespace filtering in Ingester specs

## Pattern 1: Team-Specific Ingesters (Per-Namespace)

Each team has their own namespace with their own Ingesters and Observations.

### Example: Team A Setup

```yaml
# Ingester in team-a namespace
apiVersion: zen.kube-zen.io/v1
kind: Ingester
metadata:
  name: team-a-trivy
  namespace: team-a
spec:
  source: trivy
  ingester: informer
  informer:
    namespace: team-a  # Only watch team-a resources
  destinations:
    - type: crd
      value: observations
```

### RBAC: Team Can Manage Own Ingesters

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: team-a-ingester-manager
  namespace: team-a
rules:
- apiGroups: ["zen.kube-zen.io"]
  resources: ["ingesters"]
  verbs: ["get", "list", "create", "update", "patch", "delete"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: team-a-ingester-manager
  namespace: team-a
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: team-a-ingester-manager
subjects:
- kind: Group
  name: team-a
  apiGroup: rbac.authorization.k8s.io
```

### RBAC: Team Can Read Own Observations

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: team-a-observation-reader
  namespace: team-a
rules:
- apiGroups: ["zen.kube-zen.io"]
  resources: ["observations"]
  verbs: ["get", "list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: team-a-observation-reader
  namespace: team-a
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: team-a-observation-reader
subjects:
- kind: Group
  name: team-a
  apiGroup: rbac.authorization.k8s.io
```

## Pattern 2: Shared Watchers with Namespace Filtering

Single zen-watcher instance with namespace-scoped Ingesters.

### Example: Team A Ingester

```yaml
apiVersion: zen.kube-zen.io/v1
kind: Ingester
metadata:
  name: team-a-trivy
  namespace: team-a
spec:
  source: trivy
  ingester: informer
  informer:
    namespace: team-a
    labelSelector: "team=team-a"  # Additional filtering
  filters:
    includeNamespaces:
      - team-a  # Only process team-a resources
  destinations:
    - type: crd
      value: observations
```

## Pattern 3: Cluster-Wide View with Namespace Labels

Cluster-wide Ingesters with Observations labeled by namespace.

### Example: Cluster-Wide Ingester

```yaml
apiVersion: zen.kube-zen.io/v1
kind: Ingester
metadata:
  name: cluster-wide-trivy
  namespace: zen-system
spec:
  source: trivy
  ingester: informer
  informer:
    namespace: ""  # Watch all namespaces
  destinations:
    - type: crd
      value: observations
      mapping:
        fieldMapping:
          - from: metadata.namespace
            to: zen.io/namespace  # Label by namespace
```

### RBAC: Ops Can View All Observations

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: observation-viewer
rules:
- apiGroups: ["zen.kube-zen.io"]
  resources: ["observations"]
  verbs: ["get", "list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: ops-observation-viewer
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: observation-viewer
subjects:
- kind: Group
  name: ops-team
  apiGroup: rbac.authorization.k8s.io
```

## Namespace Scoping Strategies

### Strategy 1: Ingester Namespace = Observation Namespace

**Pattern**: Ingesters create Observations in the same namespace.

**Use Case**: Team-specific isolation.

**Example**:
- Ingester in `team-a` namespace → Observations in `team-a` namespace
- Ingester in `team-b` namespace → Observations in `team-b` namespace

### Strategy 2: Centralized Observations Namespace

**Pattern**: All Observations created in a single namespace (e.g., `zen-observations`).

**Use Case**: Centralized aggregation and management.

**Note**: Requires zen-watcher ServiceAccount to have permissions to create Observations in the centralized namespace.

### Strategy 3: Cross-Namespace Observations

**Pattern**: Ingester in namespace A creates Observations in namespace B.

**Use Case**: Centralized security/compliance namespace.

**Note**: Requires zen-watcher ServiceAccount to have permissions to create Observations in the target namespace.

## zen-watcher ServiceAccount Permissions

### Option 1: Cluster-Wide Permissions

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: zen-watcher
rules:
- apiGroups: ["zen.kube-zen.io"]
  resources: ["observations"]
  verbs: ["create", "update", "patch"]
- apiGroups: ["zen.kube-zen.io"]
  resources: ["ingesters"]
  verbs: ["get", "list", "watch"]
```

### Option 2: Namespace-Scoped Permissions

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: zen-watcher
  namespace: zen-system
rules:
- apiGroups: ["zen.kube-zen.io"]
  resources: ["observations"]
  verbs: ["create", "update", "patch"]
```

## Best Practices

1. **Use namespace-scoped Ingesters** for team isolation
2. **Apply RBAC at namespace level** for fine-grained control
3. **Label Observations by namespace** for cross-namespace queries
4. **Centralize Ops view** via ClusterRole for observation viewing
5. **Limit zen-watcher permissions** to minimum required

## Related Documentation

- [Observation API Public Guide](OBSERVATION_API_PUBLIC_GUIDE.md) - Complete Observation CRD API reference
- [Ingester API](INGESTER_API.md) - Complete Ingester CRD API reference
- [RBAC Best Practices](https://kubernetes.io/docs/reference/access-authn-authz/rbac/)

