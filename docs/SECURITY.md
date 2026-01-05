# Security Features and Model

> **📋 For vulnerability reporting process**, see [VULNERABILITY_DISCLOSURE.md](../VULNERABILITY_DISCLOSURE.md) (root).  
> This document is the **central, authoritative document** for all product security features, threat model, and security configuration.

> **Note:** For detailed RBAC documentation, see [RBAC Security Documentation](./SECURITY_RBAC.md).

---

## Quick Security Checklist

- ✅ Non-root container execution
- ✅ Read-only root filesystem
- ✅ Least-privilege RBAC
- ✅ NetworkPolicy restrictions
- ✅ Webhook authentication (token-based, IP allowlist)
- ✅ Rate limiting (100 req/min per IP)
- ✅ Garbage collection (prevents CRD bloat)
- ✅ Resource quotas (recommended)

---

## Security Commitment

Zen Watcher is a security tool and therefore must maintain the highest security standards. We take security seriously and follow industry best practices.

## Reporting Security Vulnerabilities

**Please DO NOT open public GitHub issues for security vulnerabilities.**

For vulnerability reporting process, supported versions, and response timeline, see [VULNERABILITY_DISCLOSURE.md](../VULNERABILITY_DISCLOSURE.md) (root).

**Quick Reference:**
- **Email**: security@kube-zen.io (preferred)
- **GitHub Security Advisory**: Use the "Report a vulnerability" button
- **Response Time**: Within 24 hours

---

## Security Principles

1. **Zero Trust** - No external dependencies, no egress traffic, no credentials
2. **Least Privilege** - Minimum RBAC permissions required
3. **Defense in Depth** - Multiple layers of security controls
4. **Transparency** - Open source, auditable, documented
5. **Kubernetes-Native** - Leverage platform security primitives

---

## Trust Boundaries

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

❌ **Ingester CRDs**
- Validated before application
- Invalid filters fall back to last-good-config
- No filter can execute arbitrary code

---

## Security Layers

### Layer 1: Container Security

#### Non-Privileged Execution
```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 1001
  runAsGroup: 1001
  allowPrivilegeEscalation: false
```

#### Read-Only Root Filesystem
```yaml
securityContext:
  readOnlyRootFilesystem: true
```

#### Dropped Capabilities
```yaml
securityContext:
  capabilities:
    drop:
      - ALL
```

#### Seccomp Profile
```yaml
securityContext:
  seccompProfile:
    type: RuntimeDefault
```

### Layer 2: Network Security

#### NetworkPolicy
Zen Watcher implements strict network policies:

**Ingress**: Only from webhook sources (Falco, Audit) and Prometheus (metrics scraping)  
**Egress**: Only to Kubernetes API (443) and CoreDNS (53)

```yaml
networkPolicy:
  enabled: true
  policyTypes:
    - Ingress
    - Egress
```

#### No External Dependencies
- Zero outbound traffic to internet
- No external databases or services
- Pure Kubernetes primitives only

#### Service Mesh Compatibility
Compatible with Istio, Linkerd, and other service meshes.

### Layer 3: RBAC

#### Least Privilege
Zen Watcher uses minimal RBAC permissions:

- **Read-only** access to ConfigMaps, Pods, Namespaces
- **Read-only** access to security tool CRDs
- **Read/Write** access only to its own Observation CRDs

#### ServiceAccount
Dedicated ServiceAccount with limited permissions.

See [SECURITY_RBAC.md](SECURITY_RBAC.md) for detailed permission rationale.

### Layer 4: Data Sanitization

#### Input Validation
- Webhook payloads: JSON schema validation
- ConfigMap data: Safe JSON parsing
- CRD fields: Kubernetes validation via OpenAPI schema

#### Output Sanitization
- No sensitive data in logs (passwords, tokens filtered)
- Observation CRDs: Structured, validated fields only
- Metrics: No PII or sensitive metadata

### Layer 5: Resource Limits

#### Memory Protection
```yaml
resources:
  limits:
    memory: 512Mi  # OOMKill before consuming cluster resources
```

#### Dedup Cache
- Fixed max size (default 5000)
- LRU eviction prevents unbounded growth

#### Rate Limiter Cache
- TTL-based cleanup (1 hour inactivity)
- Automatic removal of stale entries prevents unbounded growth
- Cleanup runs hourly, removing entries not accessed in the last hour

#### Request Body Size Limit
- Maximum request body size: 1MiB (1048576 bytes) by default
- Configurable via `SERVER_MAX_REQUEST_BYTES` environment variable or `server.maxRequestBytes` Helm value
- Prevents DoS attacks via large request bodies
- Requests exceeding the limit receive HTTP 413 (Request Entity Too Large)

#### Webhook Channels
- Bounded capacity (100 Falco, 200 Audit)
- Backpressure via HTTP 503 (not crash)

#### Observation TTL
- Default 7 days
- Automatic GC prevents etcd exhaustion

---

## Webhook Security

Zen Watcher exposes webhook endpoints for Falco and Audit events. **In production, enable authentication:**

```yaml
webhookSecurity:
  authToken:
    enabled: true
    secretName: "zen-watcher-webhook-token"
  ipAllowlist:
    enabled: true
    allowedIPs: ["10.0.0.0/8"]
  rateLimit:
    enabled: true
    requestsPerMinute: 100
```

See the [Threat Model](#threat-model) section below for details.

---

## Security Features

### 1. Container Security

All container security features are enabled by default:
- Non-privileged execution
- Read-only root filesystem
- Dropped capabilities
- Seccomp profile

### 2. Network Security

NetworkPolicy is enabled by default and restricts:
- Ingress to webhook sources and Prometheus
- Egress to Kubernetes API and DNS only

### 3. RBAC

Least-privilege RBAC with read-only access to source CRDs and write access only to Observation CRDs.

### 4. Pod Security Standards

Compliant with Kubernetes **restricted** Pod Security Standard:

```yaml
podSecurityStandards:
  enforce: "restricted"
  audit: "restricted"
  warn: "restricted"
```

### 5. Supply Chain Security

#### Image Signing (Cosign)
All official images are signed with Cosign:

```bash
# Verify image signature
cosign verify --key cosign.pub kubezen/zen-watcher:1.2.1
```

#### SBOM (Software Bill of Materials)
Every release includes an SBOM:

```bash
# Generate SBOM
syft kubezen/zen-watcher:1.2.1 -o spdx-json > sbom.json

# Scan SBOM for vulnerabilities
grype sbom:sbom.json
```

#### Image Scanning
All images are scanned with:
- Trivy (vulnerabilities)
- Grype (vulnerabilities)
- Syft (SBOM generation)

### 6. Secrets Management

#### No Hardcoded Secrets
- No secrets in code
- No secrets in images
- Secrets managed via Kubernetes Secrets

#### Sensitive Data
Uses Kubernetes Secrets for:
- Image pull secrets
- API keys (if needed)

---

## Security Best Practices

### For Production Deployments

**1. Enable Webhook Authentication (Generic Webhook Adapter)**

For Ingester-based webhook sources, configure per-ingester authentication. For detailed configuration examples and secret creation instructions, see [SOURCE_ADAPTERS.md](SOURCE_ADAPTERS.md#authentication-configuration).

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

**2. Configure Trusted Proxy CIDRs (Helm)**
```yaml
server:
  trustedProxyCIDRs:
    - "10.0.0.0/8"      # Private network proxies
    - "172.16.0.0/12"   # Private network proxies
```

**Note:** Empty list (default) = proxy headers never trusted (secure by default). This prevents IP spoofing attacks in rate limiting and IP allowlists.

**3. Configure Request Body Size Limit (Helm)**
```yaml
server:
  maxRequestBytes: 2097152  # 2MiB (default: 1MiB)
```

**Note:** Default is 1MiB (1048576 bytes). This prevents DoS attacks via large request bodies. Webhooks exceeding this limit receive HTTP 413 (Request Entity Too Large).

**4. Apply NetworkPolicy**
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

**5. Set Resource Quotas**
```yaml
resources:
  limits:
    memory: 512Mi
    cpu: 500m
```

**6. Enable Pod Security Standards**
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

### Deployment

1. **Enable All Security Features**
   ```bash
   helm install zen-watcher kube-zen/zen-watcher \
     --namespace zen-system \
     --create-namespace \
     --set networkPolicy.enabled=true \
     --set podSecurityStandards.enabled=true \
     --set image.verifySignature=true
   ```

2. **Use Resource Limits**
   ```yaml
   resources:
     limits:
       cpu: 200m
       memory: 256Mi
     requests:
       cpu: 100m
       memory: 128Mi
   ```

3. **Enable Pod Disruption Budget**
   ```yaml
   podDisruptionBudget:
     enabled: true
     minAvailable: 1
   ```

4. **Use Dedicated Namespace**
   ```bash
   kubectl create namespace zen-system
   ```

### Monitoring

1. **Enable ServiceMonitor**
   ```yaml
   serviceMonitor:
     enabled: true
   ```

2. **Watch for Security Events**
   ```bash
   kubectl get observations -n zen-system --watch
   ```

3. **Monitor Logs**
   ```bash
   kubectl logs -n zen-system -l app=zen-watcher -f
   ```

### Updates

1. **Keep Up to Date**
   - Subscribe to security advisories
   - Update regularly
   - Test updates in staging first

2. **Verify Updates**
   ```bash
   # Verify new image signature
   cosign verify --key cosign.pub kubezen/zen-watcher:1.2.1
   
   # Check for vulnerabilities
   trivy image kubezen/zen-watcher:1.2.1
   ```

---

## Security Monitoring

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

---

## Security Scanning

### Scan the Image

```bash
# Trivy
trivy image kubezen/zen-watcher:1.2.1

# Grype
grype kubezen/zen-watcher:1.2.1

# Snyk (if you have access)
snyk container test kubezen/zen-watcher:1.2.1
```

### Scan the Deployment

```bash
# Kubescape
kubescape scan workload deployment/zen-watcher -n zen-system

# Kube-bench
kube-bench run --targets node,policies

# Falco (runtime monitoring)
kubectl logs -n falco -l app=falco
```

---

## Compliance

### CIS Kubernetes Benchmark
Zen Watcher follows CIS Kubernetes Benchmark recommendations:
- ✅ 5.2.1 Minimize the admission of privileged containers
- ✅ 5.2.2 Minimize the admission of containers wishing to share the host process ID namespace
- ✅ 5.2.3 Minimize the admission of containers wishing to share the host IPC namespace
- ✅ 5.2.4 Minimize the admission of containers wishing to share the host network namespace
- ✅ 5.2.5 Minimize the admission of containers with allowPrivilegeEscalation
- ✅ 5.2.6 Minimize the admission of root containers
- ✅ 5.2.7 Minimize the admission of containers with the NET_RAW capability
- ✅ 5.2.8 Minimize the admission of containers with added capabilities
- ✅ 5.2.9 Minimize the admission of containers with capabilities assigned

### NIST Guidelines
Follows NIST 800-190 Application Container Security Guide.

### PCI-DSS
Suitable for PCI-DSS environments with proper configuration.

---

## Security Checklist

Before deploying to production:

- [ ] NetworkPolicy enabled
- [ ] Pod Security Standards enforced
- [ ] Image signature verified
- [ ] SBOM generated and reviewed
- [ ] Vulnerability scan passed
- [ ] Resource limits set
- [ ] RBAC reviewed
- [ ] Secrets properly managed
- [ ] Monitoring enabled
- [ ] Logs centralized
- [ ] Backup strategy in place

---

## Incident Response

### Suspected Compromise

1. **Isolate**
   ```bash
   # Scale down
   kubectl scale deployment zen-watcher -n zen-system --replicas=0
   
   # Block network
   kubectl apply -f - <<EOF
   apiVersion: networking.k8s.io/v1
   kind: NetworkPolicy
   metadata:
     name: deny-all
     namespace: zen-system
   spec:
     podSelector:
       matchLabels:
         app: zen-watcher
     policyTypes:
     - Ingress
     - Egress
   EOF
   ```

2. **Investigate**
   ```bash
   # Collect logs
   kubectl logs -n zen-system deployment/zen-watcher --previous > incident-logs.txt
   
   # Check events
   kubectl get events -n zen-system
   
   # Check Observations
   kubectl get observations -n zen-system
   ```

3. **Remediate**
   - Update to latest version
   - Rotate secrets
   - Review access logs
   - Apply security patches

4. **Report**
   - Contact security@kube-zen.com
   - Document timeline
   - Share findings

---

## Security Roadmap

Future security enhancements:
- [ ] OPA/Gatekeeper policies
- [ ] Admission controller integration
- [ ] mTLS for all communications
- [ ] Hardware security module (HSM) support
- [ ] FIPS 140-2 compliance
- [ ] Air-gapped deployment support

---

## Related Documentation

- [SECURITY_RBAC.md](SECURITY_RBAC.md) - Detailed RBAC permissions
- [OPERATIONAL_EXCELLENCE.md](OPERATIONAL_EXCELLENCE.md) - Operational excellence guide (includes stability and HA)

---

## Resources

- [Kubernetes Security Best Practices](https://kubernetes.io/docs/concepts/security/overview/)
- [OWASP Kubernetes Top 10](https://owasp.org/www-project-kubernetes-top-ten/)
- [CIS Kubernetes Benchmark](https://www.cisecurity.org/benchmark/kubernetes)
- [NSA/CISA Kubernetes Hardening Guide](https://media.defense.gov/2022/Aug/29/2003066362/-1/-1/0/CTR_KUBERNETES_HARDENING_GUIDANCE_1.2_20220829.PDF)

---

## Contact

- **Security Issues**: security@kube-zen.com
- **General Questions**: support@kube-zen.com
- **GitHub**: https://github.com/kube-zen/zen-watcher/security

**DO NOT** open public GitHub issues for vulnerabilities.

We will respond within 24 hours.
