# Release Notes: Zen Watcher v1.2.0

> **📋 For complete change history**, see [CHANGELOG.md](CHANGELOG.md).  
> This file is a **curated release summary** for GitHub release pages and user announcements.

**Version**: v1.2.0  
**Release Date**: 2025-01-05  
**Status**: Final

**Purpose:**
- **RELEASE_NOTES_v*.md** (this file): Curated summary for this specific release (highlights, breaking changes, migration guides)
- **CHANGELOG.md**: Complete change history for all versions (detailed technical changes)

---

## Summary

Zen Watcher v1.2.0 is the **first production-ready release** with synchronized versioning, enhanced security defaults, comprehensive observability, and consolidated documentation. This release focuses on operational excellence, security hardening, and improved developer/operator experience.

**Key Highlights:**
- 🔒 **Secure by Default**: Webhook authentication now required by default
- 📊 **Enhanced Observability**: Leader election metrics and PrometheusRule alerts
- 🔧 **Helm Chart Improvements**: TTL/GC tuning, PrometheusRule installation
- 📚 **Documentation Consolidation**: Single source of truth for HA and scaling
- ✅ **Version Synchronization**: All components aligned to 1.2.0

---

## Breaking Changes

### 1. Webhook Authentication Required by Default

**Impact**: ⚠️ **BREAKING** - Existing deployments without webhook authentication will need to configure it.

**What Changed:**
- Webhook authentication is now **required by default** (secure by default)
- Requests without authentication will be rejected with clear error messages

**Migration Steps:**

**Option 1: Enable Authentication (Recommended for Production)**
```yaml
# values.yaml
server:
  webhook:
    authTokenSecret:
      name: zen-watcher-webhook-auth
      key: token
```

**Option 2: Disable Authentication (Development/Testing Only)**
```yaml
# values.yaml
server:
  webhook:
    authDisabled: true  # NOT RECOMMENDED for production
```

**Create Secret:**
```bash
# Generate secure token
WEBHOOK_TOKEN=$(openssl rand -hex 32)

# Create Secret
kubectl create secret generic zen-watcher-webhook-auth \
  --from-literal=token="$WEBHOOK_TOKEN" \
  -n zen-system
```

**Reference**: [Security Documentation](docs/SECURITY_FEATURES.md#webhook-authentication)

### 2. Leader Election Mode Changes

**Impact**: ⚠️ **BREAKING** - If you were using `leaderElection.mode: controller-runtime` or `leaderElection.mode: zenlead`, you must update.

**What Changed:**
- Removed `controller-runtime` and `zenlead` as valid `leaderElection.mode` options
- Only `builtin` (default) and `disabled` are supported

**Migration Steps:**

**For Standard Deployments:**
```yaml
# values.yaml
leaderElection:
  mode: builtin  # Default, recommended
```

**For zen-lead Integration:**
```yaml
# values.yaml
leaderElection:
  mode: disabled  # Then configure zen-lead separately
```

**Reference**: [Leader Election Documentation](docs/HIGH_AVAILABILITY_AND_SCALING.md#leader-election-architecture)

### 3. NetworkPolicy Egress Defaults

**Impact**: ⚠️ **BREAKING** - If you had NetworkPolicy egress enabled with broad CIDR defaults, you must now provide explicit API destinations.

**What Changed:**
- NetworkPolicy egress is now **disabled by default** (`networkPolicy.egress.enabled: false`)
- When enabled, explicit API destinations are **required** (no more broad `10.0.0.0/8` default)
- Chart will fail to render if `egress.enabled=true` and `allowKubernetesAPI=true` but no API destination is provided

**Migration Steps:**

**If You Had Egress Enabled:**
```yaml
# values.yaml
networkPolicy:
  egress:
    enabled: true
    allowKubernetesAPI: true
    # For on-prem clusters:
    kubernetesServiceIP: "10.96.0.1/32"  # Get with: kubectl get svc kubernetes -n default -o jsonpath='{.spec.clusterIP}/32'
    # For managed control planes (EKS/GKE/AKS):
    kubernetesAPICIDRs:
      - "10.100.0.0/16"  # Your API server CIDR
```

**Reference**: [NetworkPolicy Documentation](docs/SECURITY_FEATURES.md#networkpolicy-configuration)

---

## New Features

### Security Enhancements

1. **Webhook Authentication Required by Default**
   - Authentication now required by default (secure by default)
   - Configurable via Helm values (`server.webhook.authToken` or `server.webhook.authTokenSecret`)
   - Explicit opt-out available for development/testing (`server.webhook.authDisabled: true`)

2. **NetworkPolicy Safe Defaults**
   - Ingress enabled, egress disabled by default
   - Explicit API destinations required when egress is enabled
   - Prevents API block-by-default surprises

3. **Kubernetes API Health Checks**
   - Readiness probe validates Kubernetes API connectivity
   - Prevents silent failures from misconfigured NetworkPolicies
   - Structured error logging with actionable messages

### Observability & Monitoring

1. **Leader Election Metrics**
   - `zenwatcher_leader_election_transitions_total` - Leader transition counter
   - `zenwatcher_is_leader` - Current leader status (0/1)
   - `zenwatcher_failover_duration_seconds` - Failover duration histogram
   - `zenwatcher_source_watch_restarts_total{source=..., gvr=...}` - Watch restarts per source
   - `zenwatcher_source_watch_last_event_timestamp_seconds{source=..., gvr=...}` - Last event timestamp per source

2. **PrometheusRule Resource**
   - Helm chart can install PrometheusRule with pre-configured alerts
   - Alerts for leader election flapping, source staleness, ingestion drop, failover duration
   - Enable via `prometheusRule.enabled: true`

3. **Enhanced Dashboards**
   - New leader election and informer status panels in operations dashboard
   - Per-source staleness visualization
   - Failover duration tracking

### Helm Chart Enhancements

1. **Retention/GC Tuning**
   - TTL and garbage collection configurable via Helm values
   - `retention.defaultTTLDays` (default: 7 days)
   - `retention.gcInterval` (default: "1h")
   - `retention.gcTimeout` (default: "5m")
   - Recommended profiles for high-volume, standard, and compliance deployments

2. **Version Synchronization**
   - Single source of truth: `VERSION` file
   - All components synchronized (image, chart, code, git tag)

### Documentation

1. **Consolidated HA & Scaling Guide**
   - New `HIGH_AVAILABILITY_AND_SCALING.md` consolidates:
     - `SCALING.md` (deprecated)
     - `LEADER_ELECTION.md` (deprecated)
     - HA sections from `OPERATIONAL_EXCELLENCE.md`
   - Single source of truth for HA, scaling, and leader election

2. **Documentation Consolidation**
   - Merged redundant documentation files
   - Fixed all broken links in README.md
   - Improved navigation and discoverability

---

## Improvements

- **Version Alignment**: All versions synchronized from single `VERSION` file (1.2.0)
- **Health Probe Wiring**: Fixed port mismatch (now correctly probes `:8080`)
- **Helm Chart Defaults**: Fixed `leaderElection.mode` default to `builtin`
- **Documentation Quality**: Consolidated guides, fixed broken links, improved clarity
- **Security Posture**: Secure by default with explicit opt-out for development

---

## Bug Fixes

- Fixed `leaderElection.mode` default causing install failures
- Fixed health probe port mismatch (`:8081` → `:8080`)
- Fixed broken documentation links (`DEDUPLICATION.md` → `PROCESSING_PIPELINE.md`)
- Removed old version references from documentation and CI scripts

---

## Known Limitations

### Informer Failover Gap

**What**: Informer-based sources (Trivy, Kyverno, Kubernetes Events) have a processing gap during leader failover (typically 10-15 seconds).

**Impact**:
- **State-like sources** (PolicyReports, VulnerabilityReports): Recoverable via full resync (brief latency, not data loss)
- **Event-like sources** (Kubernetes Events): Potentially not recoverable for events during failover window

**Mitigation**:
- Use namespace sharding for critical services
- Deploy dedicated `zen-watcher` instances for critical namespaces
- Monitor with staleness alerts

**Reference**: [Informer Failover Gap Documentation](docs/HIGH_AVAILABILITY_AND_SCALING.md#informer-failover-gap)

---

## Upgrade Instructions

### From 1.0.0-alpha / 1.0.19 / 1.0.20

1. **Review Breaking Changes** (see above)

2. **Update Helm Repository:**
   ```bash
   helm repo update kube-zen
   ```

3. **Backup Current Configuration:**
   ```bash
   helm get values zen-watcher -n zen-system > backup-values.yaml
   ```

4. **Configure Webhook Authentication:**
   ```bash
   # Generate token
   WEBHOOK_TOKEN=$(openssl rand -hex 32)
   
   # Create Secret
   kubectl create secret generic zen-watcher-webhook-auth \
     --from-literal=token="$WEBHOOK_TOKEN" \
     -n zen-system
   ```

5. **Update Helm Values:**
   ```yaml
   # values.yaml
   image:
     tag: "1.2.0"
   
   server:
     webhook:
       authTokenSecret:
         name: zen-watcher-webhook-auth
         key: token
   
   leaderElection:
     mode: builtin  # Update if you had controller-runtime or zenlead
   
   # If using NetworkPolicy egress, add explicit API destinations:
   networkPolicy:
     egress:
       enabled: true
       allowKubernetesAPI: true
       kubernetesServiceIP: "10.96.0.1/32"  # Adjust for your cluster
   ```

6. **Upgrade:**
   ```bash
   helm upgrade zen-watcher kube-zen/zen-watcher \
     --namespace zen-system \
     --values values.yaml \
     --wait
   ```

7. **Verify:**
   ```bash
   # Check pod status
   kubectl get pods -n zen-system -l app.kubernetes.io/name=zen-watcher
   
   # Check readiness
   kubectl get pods -n zen-system -l app.kubernetes.io/name=zen-watcher -o jsonpath='{.items[0].status.conditions[?(@.type=="Ready")].status}'
   
   # Check logs for authentication
   kubectl logs -n zen-system -l app.kubernetes.io/name=zen-watcher | grep -i auth
   ```

---

## References

- **Changelog**: [CHANGELOG.md](CHANGELOG.md)
- **Versioning Strategy**: [docs/VERSIONING.md](docs/VERSIONING.md)
- **High Availability Guide**: [docs/HIGH_AVAILABILITY_AND_SCALING.md](docs/HIGH_AVAILABILITY_AND_SCALING.md)
- **Security Features**: [docs/SECURITY_FEATURES.md](docs/SECURITY_FEATURES.md) (threat model, security layers, RBAC)
- **Vulnerability Reporting**: [SECURITY.md](SECURITY.md) (root)
- **Deployment Guide**: [docs/DEPLOYMENT_HELM.md](docs/DEPLOYMENT_HELM.md)
- **Roadmap**: [ROADMAP.md](ROADMAP.md)

---

## Support

- **GitHub Issues**: [https://github.com/kube-zen/zen-watcher/issues](https://github.com/kube-zen/zen-watcher/issues)
- **Documentation**: [https://github.com/kube-zen/zen-watcher/tree/main/docs](https://github.com/kube-zen/zen-watcher/tree/main/docs)
- **Helm Charts**: [https://github.com/kube-zen/helm-charts](https://github.com/kube-zen/helm-charts)

---

**Thank you for using Zen Watcher!** 🎉

