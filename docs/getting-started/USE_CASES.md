# Use Cases: Aggregating Events with Zen Watcher

This document shows practical use cases and how to combine one or more ingester examples to aggregate specific types of events.

## Quick Reference: Available Examples

| Example | File | Event Type | Category |
|---------|------|------------|----------|
| Trivy Scanner | `examples/ingesters/trivy-informer.yaml` | Vulnerabilities | Security |
| Kyverno Policies | `examples/ingesters/kyverno-informer.yaml` | Policy Violations | Security |
| Kube-bench | `examples/ingesters/kube-bench-informer.yaml` | CIS Benchmarks | Compliance |
| Kubernetes Events | `examples/ingesters/high-rate-kubernetes-events.yaml` | Cluster Events | Operations |
| Custom CRD | `examples/ingesters/custom-crd-destination.yaml` | Any CRD | Custom |
| ConfigMap Output | `examples/ingesters/configmap-destination.yaml` | Write to ConfigMaps | Custom |

---

## Use Case 1: Security Monitoring Dashboard

**Goal**: Aggregate all security events for a security operations dashboard.

**Use Cases**:
- Real-time security event visibility
- Security team dashboard
- Incident response automation

**Ingesters to Combine**:
1. `trivy-informer.yaml` - Vulnerability scanning
2. `kyverno-informer.yaml` - Policy violations
3. Optional: Falco webhook (runtime threats) - requires manual webhook setup

**Quick Start**:
```bash
# Apply all security ingesters
kubectl apply -f examples/ingesters/trivy-informer.yaml
kubectl apply -f examples/ingesters/kyverno-informer.yaml

# Verify they're running
kubectl get ingesters -n zen-system

# View all security observations
kubectl get observations -n zen-system -l zen.io/category=security
```

**What You Get**:
- All vulnerabilities from Trivy (filtered by minPriority: 0.3)
- All policy violations from Kyverno
- Unified `Observation` CRDs with `category: security`
- Ready for Grafana security dashboard

**Query Examples**:
```bash
# All high-severity security events
kubectl get observations -n zen-system \
  -l zen.io/category=security,zen.io/severity=HIGH

# Trivy vulnerabilities only
kubectl get observations -n zen-system \
  -l zen.io/source=trivy

# Kyverno policy violations only
kubectl get observations -n zen-system \
  -l zen.io/source=kyverno
```

---

## Use Case 2: Compliance Reporting

**Goal**: Collect compliance-related events for audit reports and compliance dashboards.

**Use Cases**:
- SOC2/ISO27001 audit reports
- CIS benchmark compliance tracking
- Policy compliance monitoring
- Audit log aggregation

**Ingesters to Combine**:
1. `kube-bench-informer.yaml` - CIS benchmark results
2. Optional: Kubernetes Audit webhook - requires manual webhook setup
3. Optional: `kyverno-informer.yaml` - For compliance policy violations

**Quick Start**:
```bash
# Apply compliance ingesters
kubectl apply -f examples/ingesters/kube-bench-informer.yaml

# Verify
kubectl get ingesters -n zen-system

# View compliance observations
kubectl get observations -n zen-system -l zen.io/category=compliance
```

**What You Get**:
- CIS benchmark results from Kube-bench (24h deduplication window)
- Unified compliance observations
- Ready for compliance dashboards and reporting

**Export for Audit**:
```bash
# Export all compliance events for last 30 days (if you have jq)
kubectl get observations -n zen-system \
  -l zen.io/category=compliance \
  -o json | jq '.items[] | select(.metadata.creationTimestamp > "2024-01-01")' > compliance-audit.json

# Export as CSV (requires custom script or jq)
kubectl get observations -n zen-system \
  -l zen.io/category=compliance \
  -o json | jq -r '.items[] | [.metadata.name, .spec.category, .spec.severity, .spec.eventType] | @csv' > compliance.csv
```

---

## Use Case 3: Operations Monitoring

**Goal**: Monitor operational events like pod failures, deployments, and infrastructure health.

**Use Cases**:
- SRE dashboards
- Incident detection
- Operational health monitoring
- Deployment tracking

**Ingesters to Combine**:
1. `high-rate-kubernetes-events.yaml` - Cluster events (pod crashes, deployments, etc.)
2. Optional: Log-based ingester for application errors (custom configuration)

**Quick Start**:
```bash
# Apply operations ingester
kubectl apply -f examples/ingesters/high-rate-kubernetes-events.yaml

# Verify
kubectl get ingesters -n zen-system

# View operations observations
kubectl get observations -n zen-system -l zen.io/category=operations
```

**What You Get**:
- Kubernetes events (30s deduplication window for high-frequency events)
- Pod failures, deployments, node issues
- Unified operations observations

**Query Examples**:
```bash
# All pod-related events
kubectl get observations -n zen-system \
  -l zen.io/category=operations \
  -o json | jq '.items[] | select(.spec.resource.kind == "Pod")'

# Critical operational events
kubectl get observations -n zen-system \
  -l zen.io/category=operations,zen.io/severity=CRITICAL
```

---

## Use Case 4: Full Security Stack

**Goal**: Complete security monitoring with all available security tools.

**Use Cases**:
- Enterprise security operations center (SOC)
- Comprehensive security monitoring
- Multi-layered security visibility

**Ingesters to Combine**:
1. `trivy-informer.yaml` - Vulnerabilities
2. `kyverno-informer.yaml` - Policy violations
3. `kube-bench-informer.yaml` - Compliance checks
4. Falco webhook (runtime threats) - requires manual setup
5. Optional: Checkov ConfigMap watcher (IaC security) - custom configuration

**Quick Start**:
```bash
# Apply all security-related ingesters
kubectl apply -f examples/ingesters/trivy-informer.yaml
kubectl apply -f examples/ingesters/kyverno-informer.yaml
kubectl apply -f examples/ingesters/kube-bench-informer.yaml

# Verify all are running
kubectl get ingesters -n zen-system

# View all security and compliance observations
kubectl get observations -n zen-system \
  -l 'zen.io/category in (security,compliance)'
```

**What You Get**:
- Vulnerabilities from Trivy
- Policy violations from Kyverno
- Compliance findings from Kube-bench
- Unified format for all security events
- Ready for SIEM integration via kubewatch/Robusta

---

## Use Case 5: Multi-Domain Observability

**Goal**: Aggregate events from all domains (security, compliance, operations, performance, cost).

**Use Cases**:
- Executive dashboards
- Cross-domain correlation
- Unified observability platform
- Multi-team visibility

**Ingesters to Combine**:
1. `trivy-informer.yaml` - Security (vulnerabilities)
2. `kyverno-informer.yaml` - Security (policy violations)
3. `kube-bench-informer.yaml` - Compliance
4. `high-rate-kubernetes-events.yaml` - Operations
5. Custom ingesters for performance/cost monitoring (requires custom configuration)

**Quick Start**:
```bash
# Apply all available ingesters
kubectl apply -f examples/ingesters/trivy-informer.yaml
kubectl apply -f examples/ingesters/kyverno-informer.yaml
kubectl apply -f examples/ingesters/kube-bench-informer.yaml
kubectl apply -f examples/ingesters/high-rate-kubernetes-events.yaml

# Verify
kubectl get ingesters -n zen-system

# View all observations grouped by category
kubectl get observations -n zen-system --sort-by=.spec.category
```

**Query by Domain**:
```bash
# Security events
kubectl get observations -n zen-system -l zen.io/category=security

# Compliance events
kubectl get observations -n zen-system -l zen.io/category=compliance

# Operations events
kubectl get observations -n zen-system -l zen.io/category=operations

# All critical events across all domains
kubectl get observations -n zen-system -l zen.io/severity=CRITICAL
```

---

## Use Case 6: Custom Tool Integration

**Goal**: Integrate a custom security/compliance tool that creates Kubernetes CRDs.

**Use Cases**:
- Internal security scanners
- Custom compliance tools
- Proprietary monitoring tools
- Any tool that creates CRDs

**Approach**:
Use `custom-crd-destination.yaml` as a template and adapt it for your CRD.

**Quick Start**:
```bash
# 1. Identify your CRD's GVR (Group, Version, Resource)
kubectl api-resources | grep YourCustomResource

# 2. Copy the custom CRD example
cp examples/ingesters/custom-crd-destination.yaml my-custom-tool.yaml

# 3. Edit my-custom-tool.yaml:
#    - Update spec.informer.gvr to match your CRD
#    - Configure field extraction (JSONPath)
#    - Set normalization (domain, type)
#    - Configure filters and deduplication

# 4. Apply
kubectl apply -f my-custom-tool.yaml
```

**Example Configuration**:
```yaml
apiVersion: zen.kube-zen.io/v1alpha1
kind: Ingester
metadata:
  name: my-custom-scanner
  namespace: zen-system
spec:
  source: my-custom-tool
  ingester: informer
  informer:
    gvr:
      group: security.example.com
      version: v1
      resource: securityreports
  normalization:
    domain: security  # or compliance, operations, performance, cost
    type: custom_scan
  destinations:
    - type: crd
      value: observations
```

---

## Use Case 7: Write to Custom Resources

**Goal**: Write observations to ConfigMaps or custom CRDs instead of (or in addition to) Observation CRDs.

**Use Cases**:
- Integration with existing tools that read ConfigMaps
- Custom resource schemas
- Legacy system integration

**Approach**:
Use `configmap-destination.yaml` or `custom-crd-destination.yaml` as templates.

**Quick Start**:
```bash
# Copy example
cp examples/ingesters/configmap-destination.yaml my-configmap-ingester.yaml

# Edit to use your desired ingester (Trivy, Kyverno, etc.)
# Update destinations section to write to ConfigMaps

# Apply
kubectl apply -f my-configmap-ingester.yaml
```

---

## Combining Examples: Best Practices

### 1. Namespace Organization

**Pattern**: Group related ingesters in the same namespace
```bash
# Security namespace
kubectl apply -f examples/ingesters/trivy-informer.yaml -n security
kubectl apply -f examples/ingesters/kyverno-informer.yaml -n security

# Operations namespace
kubectl apply -f examples/ingesters/high-rate-kubernetes-events.yaml -n operations
```

### 2. Filter Configuration

**Pattern**: Use filters to reduce noise per use case
```yaml
# For security dashboard: Only HIGH and CRITICAL
spec:
  filters:
    minPriority: 0.7  # HIGH and above

# For compliance: All findings
spec:
  filters:
    minPriority: 0.0  # Include all
```

### 3. Deduplication Windows

**Pattern**: Adjust deduplication windows based on event frequency
```yaml
# High-frequency events (Kubernetes events): Short window
processing:
  dedup:
    window: "30s"

# Low-frequency events (vulnerability scans): Long window
processing:
  dedup:
    window: "24h"
```

---

## Querying Combined Observations

### By Category
```bash
# All security events
kubectl get observations -n zen-system -l zen.io/category=security

# All compliance events
kubectl get observations -n zen-system -l zen.io/category=compliance

# All operations events
kubectl get observations -n zen-system -l zen.io/category=operations
```

### By Severity Across All Domains
```bash
# All critical events (any category)
kubectl get observations -n zen-system -l zen.io/severity=CRITICAL

# All high-severity security events
kubectl get observations -n zen-system \
  -l 'zen.io/category=security,zen.io/severity=HIGH'
```

### By Source
```bash
# All Trivy observations
kubectl get observations -n zen-system -l zen.io/source=trivy

# All Kyverno observations
kubectl get observations -n zen-system -l zen.io/source=kyverno
```

### Complex Queries
```bash
# Critical security events from Trivy in the last hour
kubectl get observations -n zen-system \
  -l 'zen.io/category=security,zen.io/source=trivy,zen.io/severity=CRITICAL' \
  -o json | jq '.items[] | select(.metadata.creationTimestamp > "'$(date -u -d '1 hour ago' +%Y-%m-%dT%H:%M:%SZ)'")'
```

---

## Integration with Alerting

Once you have observations, integrate with alerting tools:

### Using kubewatch
```yaml
# kubewatch watches Observation CRDs
resources:
  - name: Observation
    namespace: zen-system
    group: zen.kube-zen.io
    version: v1
    filters:
      - labelSelector: "zen.io/severity=CRITICAL"
```

### Using Robusta
```yaml
# Robusta watches Observation CRDs
customResourceTriggers:
  - apiVersion: zen.kube-zen.io/v1
    kind: Observation
    on_create:
      - action: send_to_slack
        filters:
          - severity: CRITICAL
```

See [INTEGRATIONS.md](INTEGRATIONS.md) for complete integration guides.

---

## Next Steps

1. **Start Simple**: Begin with one ingester (e.g., `trivy-informer.yaml`)
2. **Add Gradually**: Add more ingesters as needed
3. **Customize Filters**: Adjust filters to reduce noise
4. **Integrate Alerting**: Connect to kubewatch or Robusta for notifications
5. **Build Dashboards**: Use Grafana with the pre-built dashboards

For detailed configuration, see:
- [SOURCE_ADAPTERS.md](SOURCE_ADAPTERS.md) - Complete source adapter documentation
- [INGESTER_API.md](INGESTER_API.md) - Ingester CRD API reference
- [INTEGRATIONS.md](INTEGRATIONS.md) - Integration patterns

