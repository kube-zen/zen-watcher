# Status Integrity and RBAC Hardening

## Overview

This document outlines the status integrity guarantees and RBAC hardening for zen-watcher CRDs. Status fields are **controller-owned** and must be updated via the status subresource only.

## Status Subresource Updates

All controllers update CRD status using the **status subresource** (`/status` endpoint), not the full object:

- **Ingester CRD**: Updated via `UpdateStatus()` method on dynamic client
- **Observation CRD**: Status is read-only for end users (created by controllers)
- **DeliveryFlow CRD** (zen-platform): Updated via `UpdateStatus()` method
- **Destination CRD** (zen-platform): Status updates via status subresource

### Implementation

Controllers use the dynamic client's `UpdateStatus()` method:

```go
// Correct: Update via status subresource
_, err = resourceClient.UpdateStatus(ctx, statusObject, metav1.UpdateOptions{})

// Incorrect: Do NOT update full object with status
_, err = resourceClient.Update(ctx, fullObject, metav1.UpdateOptions{})
```

## RBAC Hardening

### Controller RBAC

The zen-watcher controller has the following RBAC permissions:

```yaml
rules:
  # Read/write access to Observations CRD (for creating observations)
  - apiGroups: ["zen.kube-zen.io"]
    resources: ["observations"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  # Read-only access to Ingester CRD (spec only)
  - apiGroups: ["zen.kube-zen.io"]
    resources: ["ingesters"]
    verbs: ["get", "list", "watch"]
  # Status subresource access for Ingester (controller-only)
  - apiGroups: ["zen.kube-zen.io"]
    resources: ["ingesters/status"]
    verbs: ["get", "update", "patch"]
```

### End User RBAC

**End users (customer-facing roles) must NOT have status update permissions:**

```yaml
# ❌ DO NOT grant this to end users
rules:
  - apiGroups: ["zen.kube-zen.io"]
    resources: ["ingesters/status", "observations/status"]
    verbs: ["update", "patch"]  # Controller-only

# ✅ Correct: End users can only read status
rules:
  - apiGroups: ["zen.kube-zen.io"]
    resources: ["ingesters", "observations"]
    verbs: ["get", "list", "watch"]  # Read-only, no status write
```

### No Wildcard Grants

**Never use wildcard RBAC grants for status updates:**

```yaml
# ❌ DO NOT use wildcards
rules:
  - apiGroups: ["*"]
    resources: ["*"]
    verbs: ["*"]

# ❌ DO NOT grant update on all resources
rules:
  - apiGroups: ["zen.kube-zen.io"]
    resources: ["*"]
    verbs: ["update", "patch"]
```

## Admission Policy

### Recommended: ValidatingAdmissionWebhook

For production deployments, implement a ValidatingAdmissionWebhook that:

1. **Rejects status updates from non-controller service accounts**
2. **Allows status updates only from controller service accounts** (e.g., `zen-watcher` SA)
3. **Rejects spec updates that attempt to set status fields**

Example policy logic:

```go
// Pseudo-code for admission webhook
func validateStatusUpdate(req *admissionv1.AdmissionRequest) error {
    // Allow if from controller service account
    if req.UserInfo.Username == "system:serviceaccount:zen-system:zen-watcher" {
        return nil
    }
    
    // Reject status updates from other users
    if req.SubResource == "status" {
        return fmt.Errorf("status updates only allowed from controller")
    }
    
    // Reject spec updates that include status
    if hasStatusFields(req.Object) {
        return fmt.Errorf("status fields cannot be set via spec update")
    }
    
    return nil
}
```

### Alternative: OPA/Gatekeeper Policy

If using OPA/Gatekeeper, create a policy:

```rego
package zen.status

deny[msg] {
    input.request.subResource == "status"
    not input.request.userInfo.username == "system:serviceaccount:zen-system:zen-watcher"
    msg := "Status updates only allowed from controller service account"
}
```

## Verification

### Check RBAC Permissions

```bash
# Verify controller has status update permissions
kubectl auth can-i update ingesters/status --as=system:serviceaccount:zen-system:zen-watcher -n zen-system

# Verify end user does NOT have status update permissions
kubectl auth can-i update ingesters/status --as=system:serviceaccount:default:end-user -n default
```

### Audit Status Updates

Monitor status updates in audit logs:

```bash
# Check audit logs for status updates
kubectl logs -n kube-system -l component=kube-apiserver | grep "ingesters/status"
```

## Trust Anchor

Status fields serve as a **trust anchor** for:

- **Operational visibility**: Source health, last seen timestamps
- **Billing signals**: Bytes/events sent (via metrics, not status)
- **Failover tracking**: Active targets, failover reasons
- **Entitlement state**: Entitled condition for commercial features

**Status integrity is critical** - compromised status can lead to:
- Incorrect operational decisions
- Billing discrepancies
- Failover failures
- Security bypasses

---

**Last Updated**: 2025-01-01  
**Policy Version**: 1.0

