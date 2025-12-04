# Zen-Watcher Security Model

## Overview

Zen-watcher is a security tool and therefore must operate under a comprehensive security model. This document describes the security boundaries, threat model, and design decisions.

## 🎯 Security Principles

1. **Zero Trust** - No external dependencies, no egress traffic, no credentials
2. **Least Privilege** - Minimum RBAC permissions required
3. **Defense in Depth** - Multiple layers of security controls
4. **Transparency** - Open source, auditable, documented
5. **Kubernetes-Native** - Leverage platform security primitives

## 🔒 Trust Boundaries

### What Zen-Watcher Trusts

✅ **Kubernetes API Server**
- Source of truth for all data
- RBAC enforcement
- Admission webhooks (if configured)

✅ **Source Security Tools**
- Trivy Operator: Trusted vulnerability data
- Kyverno: Trusted policy enforcement
- Falco: Trusted runtime detection
- Checkov, KubeBench, Audit: Trusted scan results

✅ **Kubernetes Informer Pattern**
- Watch API resilient to disconnections
- Automatic resync on failures

### What Zen-Watcher Does NOT Trust

❌ **Webhook Payloads** (Falco, Audit)
- Validated and sanitized
- Optional token authentication
- Optional IP allowlist
- Rate limiting per endpoint
- Bounded channel capacity (backpressure)

❌ **ConfigMap Contents** (Checkov, KubeBench)
- JSON parsing with error handling
- Invalid data skipped, not crashed
- No code execution from ConfigMap data

❌ **ObservationMapping CRDs**
- JSONPath expressions sanitized
- Mapping validation before informer creation
- Errors isolated per mapping (don't crash adapter)

❌ **ObservationFilter CRDs**
- Validated before application
- Invalid filters fall back to last-good-config
- No filter can execute arbitrary code

## 🛡️ Security Layers

### Layer 1: Container Security

**Non-Privileged Execution:**
```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 1001
  runAsGroup: 1001
  allowPrivilegeEscalation: false
```

**Read-Only Filesystem:**
```yaml
securityContext:
  readOnlyRootFilesystem: true
```

**Dropped Capabilities:**
```yaml
securityContext:
  capabilities:
    drop: ["ALL"]
```

**Seccomp Profile:**
```yaml
securityContext:
  seccompProfile:
    type: RuntimeDefault
```

### Layer 2: Network Security

**NetworkPolicy:**
```yaml
# Ingress: Only from webhook sources (Falco, Audit)
# Ingress: Only from Prometheus (metrics scraping)
# Egress: Only to Kubernetes API (443)
# Egress: Only to CoreDNS (53)
```

**No External Dependencies:**
- Zero outbound traffic to internet
- No external databases or services
- Pure Kubernetes primitives only

### Layer 3: RBAC

**Read-Only Permissions:**
- ConfigMaps (filter config + scan results)
- PolicyReports (Kyverno)
- VulnerabilityReports (Trivy)
- Pods, Namespaces (metadata only)

**Write Permissions (Scoped):**
- Observations CRDs only
- No write to ConfigMaps, Secrets, or other resources

**No Cluster-Admin:**
- Zen-watcher never requires cluster-admin
- Follows principle of least privilege

See [SECURITY_RBAC.md](SECURITY_RBAC.md) for detailed permission rationale.

### Layer 4: Data Sanitization

**Input Validation:**
- Webhook payloads: JSON schema validation
- ConfigMap data: Safe JSON parsing
- CRD fields: Kubernetes validation via OpenAPI schema
- ObservationMapping: JSONPath sanitization

**Output Sanitization:**
- No sensitive data in logs (passwords, tokens filtered)
- Observation CRDs: Structured, validated fields only
- Metrics: No PII or sensitive metadata

### Layer 5: Resource Limits

**Memory Protection:**
```yaml
resources:
  limits:
    memory: 512Mi  # OOMKill before consuming cluster resources
```

**Dedup Cache:**
- Fixed max size (default 5000)
- LRU eviction prevents unbounded growth

**Webhook Channels:**
- Bounded capacity (100 Falco, 200 Audit)
- Backpressure via HTTP 503 (not crash)

**Observation TTL:**
- Default 7 days
- Automatic GC prevents etcd exhaustion

## 🚨 Threat Model

### Threat 1: Malicious Webhook Payloads

**Attack:** Attacker sends crafted webhook to crash or exploit zen-watcher

**Mitigations:**
- Token authentication (optional but recommended)
- IP allowlist (optional)
- Rate limiting (100 req/min per IP)
- Bounded channels (backpressure, not crash)
- JSON parsing with error handling
- No code execution from payloads

**Residual Risk:** LOW (multiple layers of defense)

### Threat 2: RBAC Privilege Escalation

**Attack:** Compromised zen-watcher pod used to escalate privileges

**Mitigations:**
- Read-only permissions to source CRDs
- Write only to Observation CRDs (isolated API group)
- No Secret or ConfigMap write permissions
- No exec or port-forward permissions
- Container runs as non-root with dropped capabilities

**Residual Risk:** LOW (least privilege enforced)

### Threat 3: Observation CRD Injection

**Attack:** Attacker creates fake Observation CRDs to pollute data

**Mitigations:**
- Zen-watcher doesn't trust existing Observations (doesn't read them except for GC)
- Deduplication based on content fingerprint (fake duplicates ignored)
- RBAC controls who can create Observation CRDs
- Audit logs track CRD creation (Kubernetes audit)

**Residual Risk:** MEDIUM (requires RBAC misconfiguration)

### Threat 4: Resource Exhaustion (DoS)

**Attack:** Flood zen-watcher with events to exhaust resources

**Mitigations:**
- Webhook rate limiting
- Bounded channel capacity (backpressure)
- Deduplication (duplicates don't create CRDs)
- Filtering (unwanted events dropped early)
- Memory limits (OOMKill before affecting cluster)
- Pod resource quotas (recommended in production)

**Residual Risk:** LOW (multiple DoS protections)

### Threat 5: Etcd Storage Exhaustion

**Attack:** Create excessive Observations to fill etcd

**Mitigations:**
- Automatic TTL (7 days default, configurable)
- Hourly GC cleanup
- Filtering reduces volume
- `ObservationsLive` metric monitors footprint
- Alert on unbounded growth

**Residual Risk:** LOW (automatic cleanup, monitoring)

### Threat 6: Supply Chain Attack

**Attack:** Compromised zen-watcher image

**Mitigations:**
- Official images from kubezen/zen-watcher only
- Image signing with Cosign (optional)
- SBOM generation and publication
- Trivy scan in CI
- Transparent build process (Dockerfile in repo)

**Residual Risk:** LOW (verifiable supply chain)

## 🔐 Security Best Practices

### For Production Deployments

**1. Enable Webhook Authentication**
```yaml
webhookSecurity:
  authToken:
    enabled: true
    secretName: zen-watcher-webhook-token
```

**2. Apply NetworkPolicy**
```yaml
networkPolicy:
  enabled: true
  ingressRules:
    - from:
      - namespaceSelector:
          matchLabels:
            name: falco
    - from:
      - namespaceSelector:
          matchLabels:
            name: monitoring
```

**3. Enable Image Signature Verification**
```yaml
image:
  verifySignature: true
  cosignPublicKey: |
    -----BEGIN PUBLIC KEY-----
    ...
    -----END PUBLIC KEY-----
```

**4. Set Resource Quotas**
```yaml
resources:
  limits:
    memory: 512Mi
    cpu: 500m
```

**5. Enable Pod Security Standards**
```yaml
namespace:
  podSecurity:
    enforce: restricted
    audit: restricted
    warn: restricted
```

**6. Regular Security Audits**
- Review RBAC permissions quarterly
- Scan zen-watcher image with Trivy
- Check for CVEs in dependencies
- Audit Observation CRDs for sensitive data
- Review NetworkPolicy rules

## 📊 Security Monitoring

### Metrics to Alert On

```promql
# Unauthorized webhook attempts (if auth enabled)
rate(zen_watcher_webhook_requests_total{outcome="unauthorized"}[5m]) > 0

# RBAC denials (indicates misconfiguration)
# Check logs: kubectl logs | grep forbidden

# High observation creation errors
rate(zen_watcher_observations_create_errors_total[5m]) > 1
```

### Security Logs to Monitor

```bash
# Check for security-related errors
kubectl logs -n zen-system deployment/zen-watcher | jq 'select(.level=="error" or .level=="warn") | select(.operation | test("auth|rbac|webhook"))'
```

## 🔗 Related Documentation

- [SECURITY.md](SECURITY.md) - Security policy and reporting
- [SECURITY_RBAC.md](SECURITY_RBAC.md) - Detailed RBAC permissions
- [SECURITY_THREAT_MODEL.md](SECURITY_THREAT_MODEL.md) - Comprehensive threat analysis
- [STABILITY.md](STABILITY.md) - Production operations
- [OPERATIONS.md](OPERATIONS.md) - Day-to-day operations guide

## 📞 Security Contact

**Report security issues to:** security@kube-zen.com  
**DO NOT** open public GitHub issues for vulnerabilities.

We will respond within 24 hours.

