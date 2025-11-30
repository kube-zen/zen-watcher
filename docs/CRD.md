# Custom Resource Definitions (CRDs)

## Observation CRD

The `Observation` CRD is the core data model for zen-watcher. It stores all security, compliance, and infrastructure events as Kubernetes resources.

### Canonical Location

**The Observation CRD is defined and maintained in this repository:**

- **Canonical file**: `deployments/crds/observation_crd.yaml`
- **This is the source of truth** - all changes to the CRD schema must be made here

### Syncing to Helm Charts

The CRD is automatically synced to the Helm charts repository:

- **Helm charts location**: `helm-charts/charts/zen-watcher/templates/observation_crd.yaml`
- **This file is a copy** - do not edit it directly in the helm-charts repo

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
- `spec.category` - Event category (security, compliance, performance)
- `spec.severity` - Severity level (critical, high, medium, low, info)
- `spec.eventType` - Type of event (vulnerability, runtime-threat, policy-violation)

#### Optional Fields

- `spec.resource` - Affected Kubernetes resource
- `spec.details` - Event-specific details (flexible JSON)
- `spec.detectedAt` - Timestamp when event was detected
- `status.processed` - Whether this event has been processed
- `status.lastProcessedAt` - Timestamp when event was last processed

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

