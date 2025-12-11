# Observation Examples

**Purpose**: Canonical examples of valid Observation CRDs that match the schema, validation rules, and intended usage patterns.

**Status**: ✅ These examples validate against the current CRD schema (v1)

---

## Examples

### 01-hello-world.yaml
**Minimal "Hello World" Observation** - Simplest valid Observation with only required fields.

**Use Case**: Starting point for understanding the Observation model.

**Fields**:
- Required: `source`, `category`, `severity`, `eventType`
- Optional: None (minimal example)

---

### 02-security-vulnerability.yaml
**Security Event: Vulnerability Detection** - Trivy-style CVE detection.

**Use Case**: Security scanning tools (Trivy, Grype, Snyk) detecting vulnerabilities in container images.

**Fields**:
- Required: All required fields
- Optional: `resource` (Pod), `details` (CVE metadata), `detectedAt`, `ttlSecondsAfterCreation`
- Category: `security`
- Severity: `high`

---

### 03-security-policy-violation.yaml
**Security Event: Policy Violation** - Kyverno-style policy violation.

**Use Case**: Policy engines (Kyverno, OPA Gatekeeper) detecting policy violations.

**Fields**:
- Required: All required fields
- Optional: `resource` (Deployment), `details` (policy metadata), `detectedAt`, `ttlSecondsAfterCreation`
- Category: `security`
- Severity: `medium`

---

### 04-compliance-audit.yaml
**Compliance Event: Audit Finding** - CIS Benchmark-style compliance check.

**Use Case**: Compliance scanning tools (kube-bench, kube-hunter) detecting compliance violations.

**Fields**:
- Required: All required fields
- Optional: `resource` (Node), `details` (benchmark metadata), `detectedAt`, `ttlSecondsAfterCreation`
- Category: `compliance`
- Severity: `medium`

---

### 05-performance-crashloop.yaml
**Performance/Operations Event: Pod CrashLoopBackOff** - Kubernetes pod crash detection.

**Use Case**: Kubernetes watchers detecting pod failures and crash loops.

**Fields**:
- Required: All required fields
- Optional: `resource` (Pod), `details` (crash metadata), `detectedAt`, `ttlSecondsAfterCreation`
- Category: `operations`
- Severity: `high`
- TTL: Short (1 hour) - will be resolved or escalated quickly

---

### 06-performance-latency.yaml
**Performance Event: High Latency** - Prometheus-style latency spike detection.

**Use Case**: Monitoring tools (Prometheus, Datadog) detecting performance degradation.

**Fields**:
- Required: All required fields
- Optional: `resource` (Deployment), `details` (latency metrics), `detectedAt`, `ttlSecondsAfterCreation`
- Category: `performance`
- Severity: `medium`

---

### 07-cost-inefficiency.yaml
**Cost Event: Resource Inefficiency** - Cost optimization opportunity.

**Use Case**: Cost management tools (kube-cost, Kubecost) detecting resource waste.

**Fields**:
- Required: All required fields
- Optional: `resource` (Deployment), `details` (cost metrics), `detectedAt`, `ttlSecondsAfterCreation`
- Category: `cost`
- Severity: `low`

---

### 08-webhook-originated.yaml
**Webhook-Originated Event** - zen-hook style webhook event.

**Use Case**: Dynamic webhook gateway (zen-hook) receiving webhooks from external services (GitHub, GitLab, CI/CD).

**Fields**:
- Required: All required fields
- Optional: `resource` (Repository), `details` (webhook payload), `detectedAt`, `ttlSecondsAfterCreation`
- Labels: Webhook-specific labels (`zen.io/webhook-source`, `zen.io/webhook-event`, `zen.io/webhook-id`)
- Annotations: Webhook metadata (`zen.io/webhook-received-at`, `zen.io/webhook-delivery-attempt`)
- Category: `security` (or `operations`, `compliance`, etc. depending on webhook type)
- Severity: `medium`

**See**: `docs/DYNAMIC_WEBHOOKS_WATCHER_INTEGRATION.md` for webhook integration contract.

---

## Field Mapping

### Essential vs Optional Fields

**Essential (Required)**:
- `spec.source` - Identifies the tool/system
- `spec.category` - Classifies the event (security, compliance, performance, operations, cost)
- `spec.severity` - Prioritizes the event (critical, high, medium, low, info)
- `spec.eventType` - Describes the event type (vulnerability, policy_violation, etc.)

**Optional (Recommended)**:
- `spec.resource` - Links to affected Kubernetes resource (enables remediation)
- `spec.details` - Stores tool-specific metadata (preserves context)
- `spec.detectedAt` - Tracks when event occurred (vs when processed)
- `spec.ttlSecondsAfterCreation` - Controls automatic cleanup (prevents etcd bloat)

**Optional (Status)**:
- `status.processed` - Tracks processing state
- `status.lastProcessedAt` - Tracks processing history

### Category and Severity Mapping

**Category Enum**:
- `security` - Security-related events (vulnerabilities, threats, policy violations)
- `compliance` - Compliance-related events (audit findings, policy checks)
- `performance` - Performance-related events (latency spikes, resource exhaustion)
- `operations` - Operations-related events (pod crashes, deployment failures)
- `cost` - Cost/efficiency-related events (resource waste, unused resources)

**Severity Enum**:
- `critical` - Immediate action required
- `high` - High priority, should be addressed soon
- `medium` - Medium priority, can be addressed in normal workflow
- `low` - Low priority, informational
- `info` - Informational only

**See**: `docs/OBSERVATION_API_PUBLIC_GUIDE.md` for complete field documentation.

---

## Validation

All examples in this directory validate against the current CRD schema:

- ✅ Required fields present
- ✅ Enum values match (`category`, `severity`)
- ✅ Pattern validation passes (`source`, `eventType`)
- ✅ TTL within valid range (1 to 31536000 seconds)
- ✅ Labels follow conventions (`zen.io/*`)

**Validation Test**: These examples are used as fixtures in `pkg/watcher/observation_creator_validation_test.go` to ensure they stay in sync with the schema.

---

## Usage

### Apply Examples

```bash
# Apply a single example
kubectl apply -f examples/observations/02-security-vulnerability.yaml

# Apply all examples
kubectl apply -f examples/observations/
```

### Validate Against CRD

```bash
# Validate an example against the CRD schema
kubectl apply --dry-run=client -f examples/observations/02-security-vulnerability.yaml
```

### Use as Test Fixtures

These examples are referenced in:
- `pkg/watcher/observation_creator_validation_test.go` - Schema validation tests
- Future integration tests

---

## Related Documentation

- **API Guide**: `docs/OBSERVATION_API_PUBLIC_GUIDE.md` - Complete API documentation
- **Versioning Plan**: `docs/OBSERVATION_VERSIONING_AND_RELEASE_PLAN.md` - Versioning strategy
- **CRD Definition**: `deployments/crds/observation_crd.yaml` - Complete CRD schema
- **Webhook Integration**: `docs/DYNAMIC_WEBHOOKS_WATCHER_INTEGRATION.md` - Webhook contract

---

**These examples are the "golden" starting point for creating Observations. They demonstrate correct usage patterns and validate against the current CRD schema.**
