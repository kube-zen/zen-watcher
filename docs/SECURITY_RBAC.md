# RBAC Security Documentation

## Overview

This document provides detailed rationale for each RBAC permission granted to `zen-watcher`. All permissions follow the principle of least privilege.

## ClusterRole Permissions

### Core Kubernetes Resources (Read-Only)

#### ConfigMaps
```yaml
apiGroups: [""]
resources: ["configmaps"]
verbs: ["get", "list", "watch"]
```
**Rationale:**
- Required to read filter configuration from `zen-watcher-filter` ConfigMap
- Required to read kube-bench and Checkov scan results from ConfigMaps
- Read-only access prevents accidental modification of configuration
- Scoped to specific namespaces via `WATCH_NAMESPACE` environment variable

#### Pods
```yaml
apiGroups: [""]
resources: ["pods"]
verbs: ["get", "list", "watch"]
```
**Rationale:**
- Required for kube-bench watcher to identify nodes and schedule scan jobs
- Used for metadata enrichment of observations
- Read-only access prevents pod manipulation

#### Pods/Log
```yaml
apiGroups: [""]
resources: ["pods/log"]
verbs: ["get", "list", "watch"]
```
**Rationale:**
- Required for reading pod logs (currently unused but reserved for future audit log collection)
- Read-only access prevents log manipulation

#### Namespaces
```yaml
apiGroups: [""]
resources: ["namespaces"]
verbs: ["get", "list", "watch"]
```
**Rationale:**
- Required to discover namespaces for scanning (kube-bench, Checkov)
- Required to validate namespace existence before creating observations
- Read-only access prevents namespace manipulation

### Policy Reports (Read-Only)

#### PolicyReports and ClusterPolicyReports
```yaml
apiGroups: ["wgpolicyk8s.io"]
resources: ["policyreports", "clusterpolicyreports"]
verbs: ["get", "list", "watch"]
```
**Rationale:**
- Required to monitor Kyverno policy violations
- Read-only access prevents tampering with policy reports
- Cluster-wide read necessary to detect violations across all namespaces

### Kyverno Policies (Read-Only)

#### ClusterPolicies and Policies
```yaml
apiGroups: ["kyverno.io"]
resources: ["clusterpolicies", "policies"]
verbs: ["get", "list", "watch"]
```
**Rationale:**
- Required to enrich PolicyReport observations with policy metadata
- Read-only access prevents policy manipulation
- Cluster-wide read necessary to access cluster-level policies

### Trivy Security Reports (Read-Only)

#### VulnerabilityReports and Related Resources
```yaml
apiGroups: ["aquasecurity.github.io"]
resources: 
  - vulnerabilityreports
  - clustervulnerabilityreports
  - configauditreports
  - clusterconfigauditreports
  - exposedsecretsreports
  - clusterexposedsecretsreports
  - rbachassessments
  - clusterrbachassessments
verbs: ["get", "list", "watch"]
```
**Rationale:**
- Required to monitor Trivy Operator vulnerability scan results
- Read-only access prevents tampering with security reports
- Cluster-wide read necessary to detect vulnerabilities across all namespaces
- Multiple resource types cover all Trivy Operator report kinds

**Security Considerations:**
- Vulnerability data is sensitive but zen-watcher only aggregates, doesn't expose externally
- RBAC controls who can read the Observation CRDs containing vulnerability summaries

### Observation CRDs (Full Access)

#### Observations
```yaml
apiGroups: ["zen.kube-zen.io"]
resources: ["observations"]
verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
```
**Rationale:**
- **create**: Required to create Observation CRDs from security events
- **get/list/watch**: Required for deduplication checks and GC operations
- **update/patch**: Required for GC TTL updates and status updates
- **delete**: Required for garbage collection of expired observations
- Full access is necessary as zen-watcher is the sole creator and manager of Observations

**Security Considerations:**
- Observations are created only from validated sources (Trivy, Falco, Kyverno, etc.)
- Deduplication prevents duplicate CRD creation
- Rate limiting prevents flooding (see threat model)
- GC prevents unbounded growth

#### ObservationFilters (Read-Only) ✨ NEW in v1.0.10
```yaml
apiGroups: ["zen.kube-zen.io"]
resources: ["observationfilters"]
verbs: ["get", "list", "watch"]
```
**Rationale:**
- Required to dynamically load filter configuration from ObservationFilter CRDs
- Read-only access prevents filter tampering
- Watches for changes to reload filter config dynamically
- Merges with ConfigMap-based filters

**Security Considerations:**
- Filters are validated before application
- Invalid filters fall back to last-good-config
- Filter changes are logged for audit trail

#### ObservationMappings (Read-Only) ✨ NEW in v1.0.10
```yaml
apiGroups: ["zen.kube-zen.io"]
resources: ["observationmappings"]
verbs: ["get", "list", "watch"]
```
**Rationale:**
- Required for generic CRD adapter to discover mapping configurations
- Read-only access prevents mapping tampering
- Watches for changes to dynamically create/destroy informers for source CRDs
- Enables "long tail" tool integration without code changes

**Security Considerations:**
- Mappings validated before informer creation
- JSONPath expressions sanitized to prevent code injection
- Malformed mappings logged but don't crash the adapter
- Per-mapping error metrics for observability

### Trivy Reports (Read-Only)

#### VulnerabilityReports and Related Resources
```yaml
apiGroups: ["aquasecurity.github.io"]
resources: ["vulnerabilityreports", "clustervulnerabilityreports", "configauditreports", "clusterconfigauditreports", "exposedsecretsreports", "clusterexposedsecretsreports", "rbachassessments", "clusterrbachassessments"]
verbs: ["get", "list", "watch"]
```
**Rationale:**
- Required to monitor Trivy Operator security scans
- Read-only access prevents tampering with scan results
- Cluster-wide read necessary to detect vulnerabilities across all namespaces
- Multiple resource types needed for comprehensive security coverage

## Security Boundaries

### What zen-watcher CANNOT Do

1. **Cannot modify workloads**: No write access to pods, deployments, services, etc.
2. **Cannot modify security policies**: No write access to Kyverno policies or PolicyReports
3. **Cannot access secrets**: No access to Secret resources (by design)
4. **Cannot escalate privileges**: No access to RBAC resources (ClusterRoles, RoleBindings, etc.)
5. **Cannot modify cluster configuration**: No access to nodes, persistent volumes, etc.

### What zen-watcher CAN Do

1. **Read security reports**: Can read PolicyReports, VulnerabilityReports, ConfigMaps
2. **Create observations**: Can create Observation CRDs (its primary function)
3. **Manage observations**: Can update, patch, and delete Observation CRDs (for GC)

## Least Privilege Validation

### Permission Audit Checklist

- ✅ All read permissions are scoped to necessary resources only
- ✅ No write permissions except for Observation CRDs (zen-watcher's own resource)
- ✅ No access to secrets or sensitive data
- ✅ No RBAC escalation capabilities
- ✅ No cluster administration permissions
- ✅ Namespace scoping via `WATCH_NAMESPACE` environment variable

### Recommended Auditing

To audit RBAC permissions:

```bash
# Check what permissions zen-watcher has
kubectl describe clusterrole zen-watcher

# Verify service account binding
kubectl get clusterrolebinding -l app.kubernetes.io/name=zen-watcher

# Test permissions (dry-run)
kubectl auth can-i create observations --as=system:serviceaccount:zen-system:zen-watcher
kubectl auth can-i get secrets --as=system:serviceaccount:zen-system:zen-watcher  # Should be "no"
```

## Future Improvements

1. **Namespace-scoped Role**: For single-namespace deployments, use Role instead of ClusterRole
2. **Resource Names**: Add resourceNames restrictions where possible
3. **Verbs Granularity**: Further restrict verbs (e.g., remove "delete" if GC is disabled)

