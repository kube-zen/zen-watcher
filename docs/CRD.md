# Custom Resource Definitions (CRDs)

## Observation CRD

The `Observation` CRD is the core data model for zen-watcher. It stores all security, compliance, and infrastructure events as Kubernetes resources.

### Canonical Location

**The Observation CRD is defined and maintained in this repository:**

- **Canonical file**: `deployments/crds/observation_crd.yaml`
- **This is the source of truth** - all changes to the CRD schema must be made here

### Syncing to Helm Charts

The CRD is automatically synced to the Helm charts repository:

- **Helm charts location**: `charts/zen-watcher/templates/observation_crd.yaml` (in the separate helm-charts repository)
- **This file is a copy** - do not edit it directly in the helm-charts repo
- Helm charts are published to ArtifactHub and available via `helm install zen-watcher kube-zen/zen-watcher`

### Sync Process

To sync the CRD to the helm-charts repository:

```bash
# From zen-watcher repository root
make sync-crd-to-chart
```

This will copy the canonical CRD to the helm-charts repository. After syncing:

1. Commit the change in the helm-charts repository
2. Update the chart version if the CRD change is breaking
3. Document any migration steps needed

### Checking for Drift

To verify the CRD in helm-charts matches the canonical version:

```bash
make check-crd-drift
```

This is useful in CI/CD pipelines to detect accidental edits in the helm-charts repository.

### CRD Schema

The Observation CRD defines:

- **Group**: `zen.kube-zen.io`
- **Version**: `v1`
- **Kind**: `Observation`
- **Plural**: `observations`
- **Short names**: `obs`, `obsv`
- **Scope**: `Namespaced`

#### Required Fields

- `spec.source` - Tool that detected the event (trivy, falco, kyverno, etc.)
- `spec.category` - Event category (security, compliance, performance, operations, cost)
- `spec.severity` - Severity level (critical, high, medium, low, info)
- `spec.eventType` - Type of event (vulnerability, runtime-threat, policy-violation)

#### Optional Fields

- `spec.resource` - Affected Kubernetes resource
  - `spec.resource.namespace` - **Intentionally preserved** to support granular RBAC policies (e.g., 'security-team can only view Observations in prod-* namespaces') and compliance auditing. This enables multi-tenancy controls while maintaining infrastructure-blind design (no cluster-unique identifiers like AWS account ID).
- `spec.details` - Event-specific details (flexible JSON)
- `spec.detectedAt` - Timestamp when event was detected
- `spec.ttlSecondsAfterCreation` - TTL in seconds after creation (Kubernetes native style, like Jobs). Observation will be deleted by GC after this duration. If not set, uses default TTL from GC configuration.
- `status.processed` - Whether this event has been processed
- `status.lastProcessedAt` - Timestamp when event was last processed

#### TTL and Retention

Observations support TTL (Time To Live) to prevent CRD bloat and etcd pressure:

1. **`spec.ttlSecondsAfterCreation`** (Kubernetes native) - Per-Observation TTL in seconds
   - Highest priority - set per observation
   - Example: `spec.ttlSecondsAfterCreation: 3600` (1 hour)
   - If not set, uses default TTL from GC configuration

2. **Default TTL** (Global) - Set via `OBSERVATION_TTL_DAYS` or `OBSERVATION_TTL_SECONDS`
   - Fallback if `spec.ttlSecondsAfterCreation` is not set per-observation
   - Default: 7 days (604800 seconds)

The garbage collector checks `spec.ttlSecondsAfterCreation` first, then falls back to the default TTL. Observations are deleted after their TTL expires.

**TTL Validation Bounds:**
- **Minimum TTL**: 60 seconds (1 minute) - Prevents immediate deletion due to misconfiguration
- **Maximum TTL**: 365 days (1 year) - Prevents indefinite retention
- Values outside these bounds are automatically adjusted with a warning logged

### Versioning

When making changes to the CRD:

1. **Non-breaking changes** (add optional fields): No version bump needed
2. **Breaking changes** (remove fields, change required fields): 
   - Update CRD version in `spec.versions`
   - Document migration path
   - Update helm chart version

### See Also

- [Kubernetes CRD Documentation](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/)
- [Helm Charts Repository](../README.md#installation)

