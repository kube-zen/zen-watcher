# Security Threat Model

## Overview

This document describes the threat model for `zen-watcher`, including potential attack vectors, mitigations, and security boundaries.

## Threat Vectors

### 1. Webhook Endpoint Attacks

#### Threat: Unauthenticated Webhook Access
**Risk:** High  
**Description:** Attackers could send malicious webhook payloads to `/falco/webhook` or `/audit/webhook` endpoints, potentially:
- Flooding the system with fake observations
- Exhausting resources (memory, CPU, etcd storage)
- Injecting malicious data into Observation CRDs

**Mitigations:**
- ✅ Per-ingester authentication (bearer token or basic auth via Kubernetes Secrets)
- ✅ IP allowlist (`WEBHOOK_ALLOWED_IPS`)
- ✅ IP spoofing protection (trusted proxy CIDRs - proxy headers only trusted from trusted proxies)
- ✅ Rate limiting (100 requests/minute per IP, configurable, with TTL-based cleanup)
- ✅ NetworkPolicy restrictions (only allow from Falco/Audit namespaces)
- 🔄 mTLS support (planned)

**Configuration:**
```yaml
webhookSecurity:
  authToken:
    enabled: true
    secretName: "zen-watcher-webhook-token"
  ipAllowlist:
    enabled: true
    allowedIPs: ["10.0.0.0/8"]  # Internal cluster IPs
  rateLimit:
    enabled: true
    requestsPerMinute: 100
```

### 2. Observation CRD Flooding

#### Threat: Resource Exhaustion via CRD Creation
**Risk:** Medium  
**Description:** A compromised namespace or malicious actor could flood the cluster with Observation CRDs, exhausting:
- etcd storage
- API server resources
- Network bandwidth

**Mitigations:**
- ✅ Rate limiting on webhook endpoints
- ✅ Deduplication prevents duplicate CRDs
- ✅ Garbage collection removes old observations (TTL-based)
- ✅ Resource quotas (recommended at namespace level)
- ✅ Channel buffering (100 Falco events, 200 Audit events)

**Recommended Kubernetes Resource Quotas:**

```yaml
apiVersion: v1
kind: ResourceQuota
metadata:
  name: observations-quota
  namespace: zen-system
spec:
  hard:
    # Limit Observation CRDs per namespace
    count/observations.zen.kube-zen.io: "10000"
```

### 3. RBAC Escalation

#### Threat: Privilege Escalation via zen-watcher Service Account
**Risk:** Low  
**Description:** If zen-watcher's service account is compromised, an attacker could:
- Read sensitive security reports
- Create/modify Observation CRDs
- Access cluster-wide resources

**Mitigations:**
- ✅ Least-privilege RBAC (read-only except for Observations)
- ✅ No access to secrets or RBAC resources
- ✅ No write access to workloads or policies
- ✅ Service account token rotation (Kubernetes native)
- ✅ NetworkPolicy restricts egress

**What zen-watcher CANNOT do:**
- Cannot modify workloads (pods, deployments, etc.)
- Cannot access secrets
- Cannot modify RBAC
- Cannot escalate privileges

### 4. Data Injection

#### Threat: Malicious Data in Observations
**Risk:** Medium  
**Description:** Attackers could inject malicious data into Observation CRDs, potentially:
- Exploiting downstream consumers
- Corrupting security reports
- Bypassing security controls

**Mitigations:**
- ✅ Input validation on webhook payloads
- ✅ Source validation (only trusted sources create observations)
- ✅ Deduplication prevents duplicate injection
- ✅ Filtering rules validate observation content
- ✅ Structured logging for audit trail

### 5. Denial of Service

#### Threat: Resource Exhaustion
**Risk:** Medium  
**Description:** Attackers could exhaust resources via:
- Webhook flooding
- Large payload sizes
- Excessive API calls

**Mitigations:**
- ✅ Rate limiting (100 req/min per IP)
- ✅ Request body size limit (1MiB default, configurable)
- ✅ Request timeouts (15s read/write)
- ✅ Channel buffering with drop-on-full
- ✅ Resource limits on pods
- ✅ GC prevents unbounded growth

#### Request Body Size Limit

**Threat:** Large request bodies could exhaust memory via DoS attacks.

**Mitigation:**
- Maximum request body size: 1MiB (1048576 bytes) by default
- Configurable via `SERVER_MAX_REQUEST_BYTES` environment variable or `server.maxRequestBytes` Helm value
- Requests exceeding the limit receive HTTP 413 (Request Entity Too Large)
- Applied to all webhook endpoints (Falco, Audit, generic webhook adapter)

## Security Boundaries

### Network Boundaries

**Ingress (Who can reach zen-watcher):**
- ✅ Metrics scraping: Prometheus/VictoriaMetrics namespaces only
- ✅ Falco webhooks: Falco namespace only (via NetworkPolicy)
- ✅ Audit webhooks: kube-system namespace only (via NetworkPolicy)
- ✅ Health/Ready: Cluster-internal only

**Egress (Where zen-watcher can reach):**
- ✅ Kubernetes API: Cluster API server only
- ✅ DNS: Cluster DNS only
- ✅ No external network access

### Data Boundaries

**What zen-watcher reads:**
- PolicyReports (Kyverno violations)
- VulnerabilityReports (Trivy scans)
- ConfigMaps (kube-bench, Checkov results)
- Pod metadata (for enrichment)

**What zen-watcher writes:**
- Observation CRDs only (zen-watcher's own resource)

**What zen-watcher never accesses:**
- Secrets
- RBAC resources
- Workload modifications
- Cluster configuration

## Quota Enforcement

### Recommended Resource Quotas

```yaml
# Limit Observation CRDs per namespace
apiVersion: v1
kind: ResourceQuota
metadata:
  name: observations-quota
  namespace: zen-system
spec:
  hard:
    count/observations.zen.kube-zen.io: "10000"
```

### Garbage Collection

- **Default TTL:** 7 days (configurable via `OBSERVATION_TTL_DAYS` or `OBSERVATION_TTL_SECONDS`)
- **GC Interval:** 1 hour (configurable via `GC_INTERVAL`)
- **Per-Observation Override:** Via `spec.ttlSecondsAfterCreation` field (Kubernetes native style)

### Rate Limiting

- **Webhook Rate Limit:** 100 requests/minute per IP (configurable)
- **Channel Buffers:** 100 (Falco), 200 (Audit)
- **Drop Policy:** Drop on full (prevents blocking)
- **Cache Management:** TTL-based cleanup (1 hour inactivity) prevents unbounded memory growth
- **IP Spoofing Protection:** Proxy headers (`X-Forwarded-For`, `X-Real-IP`) only trusted when `RemoteAddr` is from a trusted proxy CIDR (default: trust none)

### IP Spoofing Protection

**Threat:** Attackers could spoof IP addresses via `X-Forwarded-For` or `X-Real-IP` headers to bypass rate limiting or IP allowlists.

**Mitigation:**
- Proxy headers are only trusted when `RemoteAddr` matches a trusted proxy CIDR
- Default behavior: Empty trusted proxy list = proxy headers never trusted (secure by default)
- Configured via `server.trustedProxyCIDRs` in Helm values
- Prevents IP spoofing attacks in both rate limiting and IP allowlist checks

## Security Recommendations

### Production Deployment

1. **Enable Webhook Authentication (Generic Webhook Adapter):**
   
   For Ingester-based webhook sources, configure per-ingester authentication:
   
   ```yaml
   apiVersion: zen.kube-zen.io/v1alpha1
   kind: Ingester
   metadata:
     name: my-webhook-source
     namespace: zen-system
   spec:
     source: my-tool
     ingester: webhook
     webhook:
       auth:
         type: bearer
         secretName: webhook-auth-secret
   ```
   
   For detailed authentication configuration and secret creation examples, see [SOURCE_ADAPTERS.md](SOURCE_ADAPTERS.md#authentication-configuration).

2. **Configure Trusted Proxy CIDRs (Helm):**
   ```yaml
   server:
     trustedProxyCIDRs:
       - "10.0.0.0/8"      # Private network proxies
       - "172.16.0.0/12"   # Private network proxies
   ```
   
   **Note:** Empty list (default) = proxy headers never trusted (secure by default)

3. **Configure IP Allowlist:**
   ```yaml
   webhookSecurity:
     ipAllowlist:
       enabled: true
       allowedIPs: ["10.0.0.0/8", "172.16.0.0/12"]
   ```
   
   **Note:** IP allowlist also respects trusted proxy CIDRs (proxy headers only trusted from trusted proxies)

3. **Set Resource Quotas:**
   ```yaml
   apiVersion: v1
   kind: ResourceQuota
   metadata:
     name: observations-quota
   spec:
     hard:
       count/observations.zen.kube-zen.io: "10000"
   ```

4. **Enable NetworkPolicy:**
   ```yaml
   networkPolicy:
     enabled: true
   ```

5. **Monitor Rate Limits:**
   - Watch `zen_watcher_webhook_dropped_total` metric
   - Alert on high drop rates

### Monitoring and Alerting

**Key Metrics to Monitor:**
- `zen_watcher_webhook_requests_total` - Webhook request volume
- `zen_watcher_webhook_dropped_total` - Dropped requests (rate limit)
- `zen_watcher_observations_created_total` - Observation creation rate
- `zen_watcher_observations_deleted_total` - GC activity

**Recommended Alerts:**
- High webhook drop rate (>10% of requests)
- Observation creation spike (>1000/min)
- GC errors (indicates quota issues)

## Compliance Considerations

### Data Privacy
- Observations contain security event metadata only
- No sensitive data (secrets, credentials) stored
- GC ensures data retention limits

### Audit Trail
- All webhook requests logged
- Observation creation logged
- Metrics track all operations

### Least Privilege
- RBAC follows principle of least privilege
- No unnecessary permissions granted
- Regular RBAC audits recommended

