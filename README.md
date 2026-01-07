# Zen Watcher

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://go.dev/)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-1.26+-326CE5?logo=kubernetes&logoColor=white)](https://kubernetes.io/)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/zen-watcher)](https://artifacthub.io/packages/helm/kube-zen/zen-watcher)

**Status:** ✅ Actively Maintained | **Version:** 1.2.1 | **License:** [Apache 2.0](LICENSE)

> **Kubernetes Observation Collector: Turn Any Signal into a CRD**

Zen Watcher is an open-source Kubernetes operator that aggregates structured signals from any tool (security, compliance, performance, operations, cost) into unified `Observation` CRDs. Lightweight, standalone, and useful on its own.

> **💡 Project Philosophy:** This project is **built by operators, for operators**. We built Zen Watcher because we were tired of reinventing the same event aggregation wheel in every Kubernetes cluster. After spending 3 hours manually correlating security events from 4 different tools during a 2 AM incident, we realized: *operators need a single source of truth for all events, in a format they already understand: CRDs.* [Read our origin story →](docs/ORIGIN_STORY.md)

**What can you collect?** Zen Watcher handles **all event types**, not just security:
- 🔒 **Security**: Vulnerabilities, threats, policy violations (Trivy, Falco, Kyverno)
- ✅ **Compliance**: Audit logs, CIS benchmarks, policy checks
- ⚡ **Performance**: Latency spikes, resource exhaustion, crashes
- 🔧 **Operations**: Deployment failures, pod crashes, infrastructure health
- 💰 **Cost**: Resource waste, unused resources
- 🎯 **Custom**: Any domain you define

**Version:** 1.2.1 (OSS release) | **License:** [Apache 2.0](LICENSE) | **Status:** ✅ Actively Maintained

---

## 🚀 Quick Start (5 Minutes)

**Get Zen-Watcher running and see observations in your cluster:**

```bash
# 1. Add Helm repository
helm repo add kube-zen https://kube-zen.github.io/helm-charts
helm repo update

# 2. Install zen-watcher
helm install zen-watcher kube-zen/zen-watcher \
  --namespace zen-system \
  --create-namespace \
  --set ingester.createDefaultK8sEvents=true

# 3. Wait for pods to be ready
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=zen-watcher -n zen-system --timeout=60s

# 4. Query observations (this is what you get!)
kubectl get observations
```

**Expected Output:**
```
NAME                     TYPE       SEVERITY   CATEGORY      AGE
kube-apiserver-error     runtime    high       operations    2m
pod-crashloop-detected   runtime    medium     operations    5m
node-not-ready           runtime    warning    infrastructure 10m
```

**That's it!** You now have a unified view of all events in your cluster. See [Configuration](#-configuration) below for production-ready settings.

> **📖 Need more details?** See [docs/GETTING_STARTED_GENERIC.md](docs/GETTING_STARTED_GENERIC.md) for troubleshooting, advanced configuration, and integration guides.

---

## 📊 How It Works

![Zen Watcher Architecture](docs/images/zen-watcher.png)

---

## 🎯 What Zen-Watcher Is (And Is NOT)

**Zen-Watcher is:**
- ✅ **An observation aggregator** - Collects events from any tool into unified CRDs
- ✅ **A Kubernetes-native operator** - Uses CRDs, informers, and standard Kubernetes patterns
- ✅ **Read-only by design** - Never modifies or deletes your workloads
- ✅ **Tool-agnostic** - Works with Trivy, Falco, Kyverno, or any custom source
- ✅ **Standalone** - Useful on its own, no external dependencies

**Zen-Watcher is NOT:**
- ❌ **An alerting system** - We don't send alerts. We create CRDs. Use [kubewatch](https://github.com/robusta-dev/kubewatch) or [Robusta](https://home.robusta.dev/) to route observations to Slack/PagerDuty/SIEM
- ❌ **A Falco replacement** - We don't detect threats. We aggregate what Falco (and others) detect
- ❌ **An auto-remediation tool** - We intentionally avoid auto-remediation. You decide what to do with observations
- ❌ **A monitoring stack** - We don't replace Prometheus/Grafana. We complement them by providing structured event data
- ❌ **A policy engine** - We don't enforce policies. We observe policy violations from tools like Kyverno

**Our philosophy:** Zen-Watcher does one thing well—aggregate events into CRDs. Everything else (alerting, remediation, visualization) is handled by tools that specialize in those domains.

---

## 🔒 Stability Guarantees

**Zen-Watcher will never:**
- ❌ **Mutate workloads** - We never modify, patch, or delete your pods, deployments, or any cluster resources
- ❌ **Delete resources** - We only create `Observation` CRDs. We never delete anything except our own CRDs (via TTL)
- ❌ **Apply remediations automatically** - We intentionally avoid auto-remediation. You control what happens with observations
- ❌ **Store secrets** - We never hold credentials, API keys, or secrets in our code or ConfigMaps
- ❌ **Make external calls** - We only talk to the Kubernetes API. No cloud APIs, no external webhooks, no SaaS dependencies

**What we do:**
- ✅ **Create Observation CRDs** - That's it. Everything else is up to you
- ✅ **Read-only access** - We watch resources via informers (read-only)
- ✅ **Safe defaults** - Secure by default, opt-in for advanced features

**For production use:** Zen-Watcher is safe to run in production. We use it in production ourselves. See [docs/STABILITY_GUARANTEES.md](docs/STABILITY_GUARANTEES.md) for complete guarantees and API stability policy.

---

## ⚙️ Configuration

**If you don't know what to choose, use this. This is what we run.**

### Recommended (Production)

```bash
helm install zen-watcher kube-zen/zen-watcher \
  --namespace zen-system \
  --create-namespace \
  -f https://raw.githubusercontent.com/kube-zen/helm-charts/main/charts/zen-watcher/values-production.yaml
```

**What this gives you:**
- 2 replicas for HA
- NetworkPolicy enabled
- Webhook authentication enabled
- Conservative resource requests with generous limits
- 24-hour TTL on observations

**File:** [`values-production.yaml`](https://github.com/kube-zen/helm-charts/blob/main/charts/zen-watcher/values-production.yaml) in the Helm chart

### Minimal (Development/Local)

```bash
helm install zen-watcher kube-zen/zen-watcher \
  --namespace zen-system \
  --create-namespace \
  -f https://raw.githubusercontent.com/kube-zen/helm-charts/main/charts/zen-watcher/values-minimal.yaml
```

**What this gives you:**
- 1 replica (single-node clusters)
- NetworkPolicy disabled (for local dev)
- Webhook authentication disabled (for local dev)
- Minimal resource requests
- 1-hour TTL on observations

**File:** [`values-minimal.yaml`](https://github.com/kube-zen/helm-charts/blob/main/charts/zen-watcher/values-minimal.yaml) in the Helm chart

> **Note:** For air-gapped environments, download the values files and use `-f values-production.yaml` locally. See [docs/DEPLOYMENT_HELM.md](docs/DEPLOYMENT_HELM.md) for details.

---

## 🔒 Security Defaults

**What is enabled by default on `helm install`:**

✅ **Enabled (Secure by default):**
- **CRDs**: Observation and Ingester CRDs are installed automatically (`crds.enabled=true`)
- **NetworkPolicy**: Network traffic restrictions enabled (`networkPolicy.enabled=true`)
  - Ingress: Metrics scraping from Prometheus namespaces only
  - Egress: DNS (port 53) and Kubernetes API (port 443) only
- **RBAC**: ClusterRole/ClusterRoleBinding created with least-privilege permissions
- **Container Security**: Non-root, read-only filesystem, dropped capabilities, seccomp profile
- **ServiceAccount**: Token automount enabled (required for Kubernetes API access)
- **Request Body Size Limit**: 1MiB maximum (prevents DoS)
- **Rate Limiting**: 100 requests/minute per IP with TTL-based cleanup

❌ **Disabled (Requires opt-in):**
- **Webhook Authentication**: No authentication on webhook endpoints by default
  - **Production**: Enable per-ingester authentication via `Ingester` CRD `spec.webhook.auth`
  - See [SOURCE_ADAPTERS.md](docs/SOURCE_ADAPTERS.md#authentication-configuration)
- **Default Ingester**: No Ingester created automatically (`ingester.createDefaultK8sEvents=false`)
  - Create an Ingester resource to start collecting events
  - Quick start: `helm install ... --set ingester.createDefaultK8sEvents=true`
- **Trusted Proxy CIDRs**: Empty by default (proxy headers not trusted)
  - Configure `server.trustedProxyCIDRs` if behind trusted proxies/load balancers
- **IP Allowlists**: Not enabled by default

**Trust posture:**
- ✅ **Secure by default** for container, network, and RBAC security
- ⚠️ **Webhook endpoints are unauthenticated by default** - enable authentication for production
- ⚠️ **No default event collection** - create an Ingester to start collecting events

See [docs/SECURITY.md](docs/SECURITY.md) for complete security documentation (threat model, security layers, RBAC). For vulnerability reporting, see [VULNERABILITY_DISCLOSURE.md](VULNERABILITY_DISCLOSURE.md).

---

## 🔒 Zero Blast Radius Security

**Zen Watcher's core architecture delivers zero blast radius in the event of compromise.**

- ✅ **Zero secrets in core**: The core binary requires zero secrets—secrets live only in optional, isolated sync controllers
- ✅ **Zero egress traffic**: No outbound network traffic, no external dependencies
- ✅ **Zero external dependencies**: All data stored in Kubernetes-native CRDs (etcd)

This follows the proven pattern used by major CNCF projects:
- **Prometheus**: Collects metrics, but doesn't handle alert destination secrets—AlertManager does that
- **Flux**: Reconciles git state, but offloads application operations to other controllers
- **Zen Watcher**: Core only aggregates to etcd—all sensitive external operations live strictly outside that perimeter

---

## ✨ Key Features

### YAML-Only Configuration

Add any new source with a simple YAML configuration using the `Ingester` CRD. No code required!

**Five input methods:**
1. **🔍 Logs** - Monitor pod logs with regex patterns
2. **📡 Webhooks** - Receive HTTP webhooks from external tools (Falco, Audit, etc.) via static nginx configuration
3. **🗂️ ConfigMaps** - Watch ConfigMaps via informer (event-driven, recommended)
4. **📋 CRDs (Informers)** - Watch Kubernetes Custom Resource Definitions
5. **🎯 Kubernetes Events** - Native cluster events (any category: security, operations, performance, etc.)

**Quick Example:**
```yaml
apiVersion: zen.kube-zen.io/v1alpha1
kind: Ingester
metadata:
  name: my-tool-source
  namespace: zen-system
spec:
  source: my-tool  # Required: Must match pattern ^[a-z0-9-]+$
                    # Allowed values: kubernetes-events, falco, trivy, kyverno, checkov, kube-bench, cert-manager, sealed-secrets, or any custom source name
  ingester: logs    # Required: Must be one of: logs, webhook, informer
  logs:
    podSelector: app=my-tool  # Required: Kubernetes label selector (e.g., "app=my-tool" or "app in (tool1,tool2)")
    patterns:
      - regex: "ERROR: (?P<message>.*)"
        type: error
        priority: 0.8
  filters:
    minPriority: 0.5
  deduplication:
    enabled: true
    window: "1h"
  destinations:
    - type: crd
      value: observations  # Example: writes to zen.kube-zen.io/v1/observations
      # OR use gvr to write to any resource:
      # gvr:
      #   group: "your.group.com"
      #   version: "v1"
      #   resource: "yourresource"
```

See [docs/SOURCE_ADAPTERS.md](docs/SOURCE_ADAPTERS.md) for complete examples.

### Advanced Noise Reduction

- **SHA-256 content fingerprinting**: Accurate duplicate detection
- **Per-source token bucket rate limiting**: Prevents one noisy tool from overwhelming the system
- **Time-bucketed aggregation**: Collapses repeating events within configurable windows
- **Configurable processing order**: Choose `filter_first` or `dedup_first` based on your workload patterns

**Result**: <100ms CPU spikes and minimal etcd churn—even under firehose conditions


### Comprehensive Observability

- 📊 20+ Prometheus metrics on :9090
- 🎨 6 pre-built Grafana dashboards (Executive, Operations, Security, Main, Namespace Health, Explorer)
- 📝 Structured logging
- 🏥 Health and readiness probes

**Example Output:**
```bash
$ kubectl get observations -n zen-system
NAME                                    SOURCE   SEVERITY   CATEGORY   AGE
obs-trivy-vuln-abc123                   trivy    HIGH       security   5m
obs-falco-suspicious-process            falco    CRITICAL   security   2m
obs-kyverno-policy-violation            kyverno  MEDIUM     security   10m
obs-k8s-pod-crash                       k8s      WARNING    operations 1h
```

**Dashboard Preview:**
- 📊 **Executive Dashboard**: Strategic KPIs, security posture score, ROI metrics
- 🔧 **Operations Dashboard**: Real-time health, SLA tracking, capacity planning
- 🔒 **Security Dashboard**: Threat intelligence, attack chain visualization
- 📈 **Main Dashboard**: Unified navigation, cross-dashboard correlation
- 🏢 **Namespace Health**: Per-namespace compliance tracking
- 🔍 **Data Explorer**: Advanced query builder, saved searches

*Note: Screenshots available in [config/dashboards/README.md](config/dashboards/README.md)*

### Production-Ready

- Non-privileged containers
- Read-only filesystem
- Minimal footprint (~29MB image, <10m CPU, <50MB RAM)
- Horizontal Pod Autoscaling (HPA) support
- NetworkPolicy and PodSecurity support
- RBAC with minimal required permissions

---

## 🔌 Integrations

> **Need alerts in Slack, PagerDuty, or SIEM?**  
> Zen Watcher writes `Observation` CRDs for **any event type** (security, operations, performance, compliance, cost). Use [kubewatch](https://github.com/robusta-dev/kubewatch) or [Robusta](https://home.robusta.dev/) to route them to 30+ destinations—no coding required.

**Watch Events with kubectl:**
```bash
# All events (any category: security, operations, performance, compliance, cost)
kubectl get observations -n zen-system

# High severity only
kubectl get observations -n zen-system -o json | \
  jq '.items[] | select(.spec.severity == "HIGH")'

# Filter by category (examples)
kubectl get observations -n zen-system -o json | \
  jq '.items[] | select(.spec.category == "operations")'  # Operations events

kubectl get observations -n zen-system -o json | \
  jq '.items[] | select(.spec.category == "performance")'  # Performance events
```

**For complete integration guide**, see [docs/INTEGRATIONS.md](docs/INTEGRATIONS.md).

---

## 📈 Resource Usage

### Typical Load (1000 events/day):
- **CPU:** <10m average
- **Memory:** <50MB
- **Storage:** ~2MB in etcd
- **Network:** None (local only)

### Heavy Load (10,000 events/day):
- **CPU:** <20m average
- **Memory:** <80MB
- **Storage:** ~20MB in etcd
- **Network:** None (local only)

---

## 📚 Documentation

- [Use Cases](docs/USE_CASES.md) - Practical use cases and how to combine ingester examples ⭐ **NEW**
- [Installation Guide](docs/DEPLOYMENT_HELM.md) - Complete deployment instructions
- [Source Adapters](docs/SOURCE_ADAPTERS.md) - How to add new sources
- [Manual Webhook Adapter](docs/MANUAL_WEBHOOK_ADAPTER.md) - Configure webhooks for Falco, Audit, and other tools
- [Observation API](docs/CRD.md) - API reference
- [Integrations](docs/INTEGRATIONS.md) - How to consume Observations
- [Deduplication](docs/DEDUPLICATION.md) - Deduplication strategies
- [CRD Documentation](docs/CRD.md) - Detailed CRD documentation

---

## 🤝 Contributing

Contributions welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

**New to open source?** Look for issues labeled `good first issue` to get started.

**Code of Conduct**: All contributors must follow our [Code of Conduct](CODE_OF_CONDUCT.md).

**Governance**: See [GOVERNANCE.md](GOVERNANCE.md) for project governance, maintainer process, and decision-making.

---

## 👥 Maintainers

**Zen-Watcher is actively maintained by the Zen Team.**

**Who we are:** We're Kubernetes operators who built this because we needed it. We use Zen-Watcher in production, so it needs to work.

**Maintainer commitment:**
- ✅ **Actively maintained** - Regular releases, responsive to issues (target: 48-hour PR review)
- ✅ **Production-ready** - We use this in production ourselves
- ✅ **Long-term** - This isn't a side project; it's a core tool we depend on

**Where to talk:**
- 💬 **GitHub Discussions**: [github.com/kube-zen/zen-watcher/discussions](https://github.com/kube-zen/zen-watcher/discussions) - Ask questions, share ideas, connect with the community
- 📧 **Email**: zen@kube-zen.io (general inquiries)
- 🔒 **Security**: security@kube-zen.io (vulnerability reports - see [VULNERABILITY_DISCLOSURE.md](VULNERABILITY_DISCLOSURE.md))
- 🐛 **Issues**: [GitHub Issues](https://github.com/kube-zen/zen-watcher/issues) for bug reports and feature requests

**Who answers when this breaks?** We do. See [MAINTAINERS](MAINTAINERS) for our commitment and responsibilities.

---

## 💬 Support & Community

**Primary Community Platform**: [GitHub Discussions](https://github.com/kube-zen/zen-watcher/discussions) - Ask questions, share ideas, and connect with the community.

**Email Support**: zen@kube-zen.io (general inquiries)

**Security Issues**: security@kube-zen.io (vulnerability reports - see [VULNERABILITY_DISCLOSURE.md](VULNERABILITY_DISCLOSURE.md))

**Issues**: [GitHub Issues](https://github.com/kube-zen/zen-watcher/issues) for bug reports and feature requests

---

## 💰 Funding & Sponsorship

**Support the Project**: Zen Watcher is an open-source project maintained by the community. Your sponsorship helps ensure continued development, maintenance, and support.

**GitHub Sponsors**: [Sponsor us on GitHub](https://github.com/sponsors/kube-zen) - Support the project directly through GitHub Sponsors. (Profile pending approval)

**Why Sponsor?**
- Ensure long-term maintenance and support
- Accelerate feature development
- Support the open-source Kubernetes ecosystem
- Get priority support and early access to features

**Corporate Sponsors**: For enterprise sponsorship opportunities, contact team@kube-zen.io

---

## 📄 License

**License:** [Apache License 2.0](LICENSE) - See [LICENSE](LICENSE) for full text.

**Third-Party Dependencies:** See [NOTICE](NOTICE) for a summary of third-party software licenses.

**Vulnerability Scanning:** We use `govulncheck` to scan for known vulnerabilities. See [docs/BUILD.md](docs/BUILD.md#dependency-management) for details.

---

**Repository:** [github.com/kube-zen/zen-watcher](https://github.com/kube-zen/zen-watcher)  
**Helm Charts:** [github.com/kube-zen/helm-charts](https://github.com/kube-zen/helm-charts)  
**Version:** 1.2.1  
**Go Version:** 1.25+ (tested on 1.25)  
**Kubernetes:** Client libs v0.28.15 (tested on clusters 1.26+)
