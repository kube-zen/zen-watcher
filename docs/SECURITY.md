# Security Policy and Best Practices

## Security Commitment

Zen Watcher is a security tool and therefore must maintain the highest security standards. We take security seriously and follow industry best practices.

## Reporting Security Vulnerabilities

**Please DO NOT open public GitHub issues for security vulnerabilities.**

Instead, please email security details to: **security@kube-zen.com**

We will respond within 24 hours and work with you to understand and address the issue.

### What to Include

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

## Security Features

### 1. Container Security

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

### 2. Network Security

#### NetworkPolicy
Zen Watcher implements strict network policies:

**Ingress**: Only from monitoring namespace (Prometheus)
**Egress**: Only to Kubernetes API and DNS

```yaml
networkPolicy:
  enabled: true
  policyTypes:
    - Ingress
    - Egress
```

#### Service Mesh Compatibility
Compatible with Istio, Linkerd, and other service meshes.

### 3. RBAC

#### Least Privilege
Zen Watcher uses minimal RBAC permissions:

- **Read-only** access to ConfigMaps, Pods, Namespaces
- **Read-only** access to security tool CRDs
- **Read/Write** access only to its own ZenAgentEvent CRDs

#### ServiceAccount
Dedicated ServiceAccount with limited permissions.

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
cosign verify --key cosign.pub zubezen/zen-watcher:1.0.0
```

#### SBOM (Software Bill of Materials)
Every release includes an SBOM:

```bash
# Generate SBOM
syft zubezen/zen-watcher:1.0.0 -o spdx-json > sbom.json

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

## Security Best Practices

### Deployment

1. **Enable All Security Features**
   ```bash
   helm install zen-watcher ./charts/zen-watcher \
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
   kubectl get zenevents -n zen-system --watch
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
   cosign verify --key cosign.pub zubezen/zen-watcher:1.1.0
   
   # Check for vulnerabilities
   trivy image zubezen/zen-watcher:1.1.0
   ```

## Security Scanning

### Scan the Image

```bash
# Trivy
trivy image zubezen/zen-watcher:1.0.0

# Grype
grype zubezen/zen-watcher:1.0.0

# Snyk (if you have access)
snyk container test zubezen/zen-watcher:1.0.0
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
   
   # Check ZenAgentEvents
   kubectl get zenevents -n zen-system
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

## Security Roadmap

Future security enhancements:
- [ ] OPA/Gatekeeper policies
- [ ] Admission controller integration
- [ ] mTLS for all communications
- [ ] Hardware security module (HSM) support
- [ ] FIPS 140-2 compliance
- [ ] Air-gapped deployment support

## Resources

- [Kubernetes Security Best Practices](https://kubernetes.io/docs/concepts/security/overview/)
- [OWASP Kubernetes Top 10](https://owasp.org/www-project-kubernetes-top-ten/)
- [CIS Kubernetes Benchmark](https://www.cisecurity.org/benchmark/kubernetes)
- [NSA/CISA Kubernetes Hardening Guide](https://media.defense.gov/2022/Aug/29/2003066362/-1/-1/0/CTR_KUBERNETES_HARDENING_GUIDANCE_1.2_20220829.PDF)

## Contact

- Security Issues: security@kube-zen.com
- General Questions: support@kube-zen.com
- GitHub: https://github.com/your-org/zen-watcher/security


