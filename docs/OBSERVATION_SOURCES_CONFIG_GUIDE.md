# Observation Sources Configuration Guide

**Purpose**: Operator-facing guide for configuring observation sources in zen-watcher.

**Last Updated**: 2025-12-10

---

## Overview

zen-watcher collects observations from various sources (security scanners, policy engines, webhook gateways, etc.) using the `ObservationSourceConfig` CRD. This guide provides canonical patterns for common source types.

**See Also**:
- **API Reference**: `docs/OBSERVATION_API_PUBLIC_GUIDE.md`
- **Config Audit**: `docs/OBSERVATION_SOURCES_CONFIG_AUDIT.md`
- **Getting Started**: `docs/GETTING_STARTED_GENERIC.md`

---

## Canonical Source Patterns

### Pattern 1: Kubernetes Events Source

**Use Case**: Monitor Kubernetes native events (Pod crashes, deployment failures, etc.)

**Configuration**:
```yaml
apiVersion: zen.kube-zen.io/v1alpha1
kind: ObservationSourceConfig
metadata:
  name: kubernetes-events
  namespace: zen-system
spec:
  source: kubernetes-events
  ingester: k8s-events
  normalization:
    domain: operations
    type: k8s_event
    priority:
      Warning: 0.6
      Normal: 0.2
  dedup:
    window: "1h"
    strategy: fingerprint
  filter:
    minPriority: 0.5  # Only Warning+ events
```

**Required Fields**:
- `spec.source`: Unique identifier (e.g., `kubernetes-events`)
- `spec.ingester`: `k8s-events`

**Recommended Defaults**:
- `dedup.window`: `1h` (events repeat frequently)
- `filter.minPriority`: `0.5` (filter out Normal events)

**Common Pitfalls**:
- ❌ Not setting `filter.minPriority` → Too many low-priority events
- ❌ Setting `dedup.window` too short → Duplicate observations for recurring events

---

### Pattern 2: Security Scanner Source (Informer-Based)

**Use Case**: Security scanners that emit Kubernetes CRDs (Trivy, Kyverno, etc.)

**Configuration**:
```yaml
apiVersion: zen.kube-zen.io/v1alpha1
kind: ObservationSourceConfig
metadata:
  name: trivy-scanner
  namespace: zen-system
spec:
  source: trivy
  ingester: informer
  informer:
    gvr:
      group: aquasecurity.github.io
      version: v1alpha1
      resource: vulnerabilityreports
    namespace: ""  # Empty = all namespaces
    labelSelector: ""  # Optional
  normalization:
    domain: security
    type: vulnerability
    priority:
      CRITICAL: 1.0
      HIGH: 0.8
      MEDIUM: 0.5
      LOW: 0.2
  dedup:
    window: "24h"
    strategy: fingerprint
  filter:
    minPriority: 0.5  # Only HIGH and CRITICAL
    excludeNamespaces:
      - kube-system
      - kube-public
  rateLimit:
    maxPerMinute: 100
    burst: 200
```

**Required Fields**:
- `spec.source`: Unique identifier
- `spec.ingester`: `informer`
- `spec.informer.gvr.group`: CRD group
- `spec.informer.gvr.version`: CRD version
- `spec.informer.gvr.resource`: CRD resource name (plural)

**Recommended Defaults**:
- `dedup.window`: `24h` (vulnerabilities don't change frequently)
- `filter.minPriority`: `0.5` (filter out LOW severity)
- `rateLimit.maxPerMinute`: `100` (adjust based on scanner output)

**Common Pitfalls**:
- ❌ Missing `spec.informer.gvr.*` fields → Adapter creation fails
- ❌ Not setting `filter.minPriority` → Too many LOW severity observations
- ❌ Setting `dedup.window` too short → Duplicate observations for same vulnerability

---

### Pattern 3: Webhook Gateway Source

**Use Case**: External webhook gateways that send HTTP webhooks to zen-watcher

**Configuration**:
```yaml
apiVersion: zen.kube-zen.io/v1alpha1
kind: ObservationSourceConfig
metadata:
  name: webhook-gateway
  namespace: zen-system
spec:
  source: webhook-gateway
  ingester: webhook
  webhook:
    path: /webhook/gateway
    port: 8080
    bufferSize: 100
    auth:
      type: bearer
      secretName: webhook-auth-secret
  normalization:
    domain: security
    type: security_alert
    priority:
      CRITICAL: 1.0
      HIGH: 0.8
      MEDIUM: 0.5
      LOW: 0.2
  dedup:
    window: "1h"
    strategy: fingerprint
  rateLimit:
    maxPerMinute: 200
    burst: 400
```

**Required Fields**:
- `spec.source`: Unique identifier
- `spec.ingester`: `webhook`

**Recommended Defaults**:
- `webhook.path`: `/webhook/{source}` (auto-generated if not specified)
- `webhook.port`: `8080` (default)
- `webhook.bufferSize`: `100` (adjust based on webhook volume)
- `dedup.window`: `1h` (webhooks may repeat)

**Common Pitfalls**:
- ❌ Not setting `webhook.auth` → Unauthenticated webhooks (security risk)
- ❌ Setting `rateLimit.burst` < `rateLimit.maxPerMinute` → Immediate throttling
- ❌ Missing `normalization` → Observations have incorrect category/type

---

## Moving from Demo to Real Sources

After following `docs/GETTING_STARTED_GENERIC.md` (Path B), configure your first real source:

1. **Identify your source type**:
   - Security scanner with CRDs? → Use Pattern 2 (Informer-Based)
   - External webhook gateway? → Use Pattern 3 (Webhook Gateway)
   - Kubernetes events? → Use Pattern 1 (Kubernetes Events)

2. **Create ObservationSourceConfig**:
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: zen.kube-zen.io/v1alpha1
   kind: ObservationSourceConfig
   metadata:
     name: my-source
     namespace: zen-system
   spec:
     # Use one of the patterns above
   EOF
   ```

3. **Verify source is configured**:
   ```bash
   kubectl get observationsourceconfigs -n zen-system
   kubectl describe observationsourceconfig my-source -n zen-system
   ```

4. **Check for observations**:
   ```bash
   kubectl get observations -n zen-system
   kubectl get observations -n zen-system -o json | jq '.items[] | select(.spec.source == "my-source")'
   ```

---

## Troubleshooting

### No Observations Being Created

**Check**:
1. Source config is valid: `kubectl get observationsourceconfig my-source -n zen-system -o yaml`
2. Source tool is running: `kubectl get pods -n <tool-namespace>`
3. zen-watcher logs: `kubectl logs -n zen-system -l app.kubernetes.io/name=zen-watcher`

**Common Issues**:
- Invalid `spec.informer.gvr` → Check CRD exists: `kubectl get crd vulnerabilityreports.aquasecurity.github.io`
- Webhook not receiving requests → Check webhook endpoint: `curl http://zen-watcher:8080/webhook/gateway`
- Filter too restrictive → Lower `spec.filter.minPriority` or remove filters

### Too Many Observations

**Check**:
1. Deduplication window: Increase `spec.dedup.window`
2. Filter settings: Increase `spec.filter.minPriority`
3. Rate limiting: Lower `spec.rateLimit.maxPerMinute`

### Observations Have Wrong Category/Type

**Check**:
1. Normalization config: Verify `spec.normalization.domain` and `spec.normalization.type`
2. Priority mapping: Verify `spec.normalization.priority` matches source values

---

## Related Documentation

- **Config Audit**: `docs/OBSERVATION_SOURCES_CONFIG_AUDIT.md` - Detailed field analysis
- **API Guide**: `docs/OBSERVATION_API_PUBLIC_GUIDE.md` - Observation CRD API
- **Getting Started**: `docs/GETTING_STARTED_GENERIC.md` - Installation guide
