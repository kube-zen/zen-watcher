# Zen Watcher

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://go.dev/)
[![CI](https://github.com/kube-zen/zen-watcher/workflows/CI/badge.svg)](https://github.com/kube-zen/zen-watcher/actions)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-1.26+-326CE5?logo=kubernetes&logoColor=white)](https://kubernetes.io/)

> **Kubernetes Observation Collector: Turn Any Signal into a CRD**

Zen Watcher is an open-source Kubernetes operator that aggregates structured signals from any tool (security, compliance, performance, operations, cost) into unified `Observation` CRDs. Lightweight, standalone, and useful on its own.

**What can you collect?** Zen Watcher handles **all event types**, not just security:
- 🔒 **Security**: Vulnerabilities, threats, policy violations (Trivy, Falco, Kyverno)
- ✅ **Compliance**: Audit logs, CIS benchmarks, policy checks
- ⚡ **Performance**: Latency spikes, resource exhaustion, crashes
- 🔧 **Operations**: Deployment failures, pod crashes, infrastructure health
- 💰 **Cost**: Resource waste, unused resources
- 🎯 **Custom**: Any domain you define

**Version:** 1.2.1 (OSS release)

## 📊 How It Works

![Zen Watcher Architecture](docs/images/zen-watcher.png)

## 🚀 Quick Start

> **📖 For a complete getting started guide**, see [docs/GETTING_STARTED.md](docs/GETTING_STARTED.md) which includes detailed prerequisites, troubleshooting, and advanced configuration.

### Prerequisites

- Kubernetes 1.26+
- Helm 3.8+ (for Helm installation)
- kubectl configured to access your cluster

**Helm Repositories:** When using installation scripts, the following repositories are automatically added:
- `ingress-nginx`, `vm`, `grafana`, `aqua`, `falcosecurity`, `kyverno`, `kube-zen`

For air-gapped environments, use `--offline` flag (see [DEPLOYMENT_HELM.md](docs/DEPLOYMENT_HELM.md)).

### Install via Helm

```bash
# Add the Helm repository
helm repo add kube-zen https://kube-zen.github.io/helm-charts
helm repo update

# Install zen-watcher
helm install zen-watcher kube-zen/zen-watcher \
  --namespace zen-system \
  --create-namespace

# Verify installation
kubectl get pods -n zen-system
```

### Apply Example Ingester

```bash
# Apply Trivy Ingester
kubectl apply -f examples/ingesters/trivy-informer.yaml

# Check Ingesters
kubectl get ingesters
```

### Query Observations

```bash
# Using obsctl CLI
obsctl list --namespace zen-system

# Or using kubectl
kubectl get observations
```

<<<<<<< HEAD
**See [DEPLOYMENT_HELM.md](docs/DEPLOYMENT_HELM.md) for complete installation guide.**
=======
**Next Steps:**
- **Complete Installation Guide**: [docs/DEPLOYMENT_HELM.md](docs/DEPLOYMENT_HELM.md)
- **Detailed Getting Started**: [docs/GETTING_STARTED.md](docs/GETTING_STARTED.md) (includes troubleshooting, monitoring setup, and advanced configuration)
>>>>>>> fee8d0f (docs: add architecture diagram to README)

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

See [SECURITY.md](docs/SECURITY.md) for complete security documentation.

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

## Compatibility

**zen-watcher v1.2.1** requires **zen-sdk v0.2.9-alpha**

This version compatibility is tested and verified. For other zen-sdk versions, see the [zen-sdk compatibility matrix](https://github.com/kube-zen/zen-sdk#compatibility).
- [CRD Documentation](docs/CRD.md) - Detailed CRD documentation

---

## 🤝 Contributing

Contributions welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

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

Apache License 2.0 - See [LICENSE](LICENSE) for details.

---

**Repository:** [github.com/kube-zen/zen-watcher](https://github.com/kube-zen/zen-watcher)  
**Helm Charts:** [github.com/kube-zen/helm-charts](https://github.com/kube-zen/helm-charts)  
**Version:** 1.2.1  
**Go Version:** 1.25+ (tested on 1.25)  
**Kubernetes:** Client libs v0.28.15 (tested on clusters 1.26+)
