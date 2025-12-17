# Zen Watcher

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?logo=go)](https://go.dev/)

> **Kubernetes Observation Collector: Turn Any Signal into a CRD**

Zen Watcher is an open-source Kubernetes operator that aggregates structured signals from security, compliance, and infrastructure tools into unified `Observation` CRDs. Lightweight, standalone, and useful on its own.

**Version:** 1.0.0-alpha (OSS release)

## 🚀 Quick Start

### Prerequisites

- Kubernetes 1.26+
- Helm 3.8+ (for Helm installation)
- kubectl configured to access your cluster

**Helm Repositories:** When using installation scripts, the following repositories are automatically added:
- `ingress-nginx`, `vm`, `grafana`, `aqua`, `falcosecurity`, `kyverno`, `kube-zen`

For air-gapped environments, use `--offline` flag (see [DEPLOYMENT_HELM.md](docs/DEPLOYMENT_HELM.md)).

### Install via Helm

```bash
# Install zen-watcher
helm install zen-watcher ./deployments/helm/zen-watcher \
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

**See [DEPLOYMENT_HELM.md](docs/DEPLOYMENT_HELM.md) for complete installation guide.**

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
5. **🎯 Kubernetes Events** - Native cluster events (security-focused)

**Quick Example:**
```yaml
apiVersion: zen.kube-zen.io/v1alpha1
kind: Ingester
metadata:
  name: my-tool-source
  namespace: zen-system
spec:
  source: my-tool
  ingester: logs
  logs:
    podSelector: app=my-tool
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
      value: observations
```

See [docs/SOURCE_ADAPTERS.md](docs/SOURCE_ADAPTERS.md) for complete examples.

### Intelligent Noise Reduction

- **SHA-256 content fingerprinting**: Accurate duplicate detection
- **Per-source token bucket rate limiting**: Prevents one noisy tool from overwhelming the system
- **Time-bucketed aggregation**: Collapses repeating events within configurable windows
- **Configurable processing order**: Choose `filter_first` or `dedup_first` based on your workload patterns

**Result**: <100ms CPU spikes and minimal etcd churn—even under firehose conditions

See [docs/INTELLIGENT_EVENT_PIPELINE.md](docs/INTELLIGENT_EVENT_PIPELINE.md) for details.

### Comprehensive Observability

- 📊 20+ Prometheus metrics on :9090
- 🎨 6 pre-built Grafana dashboards (Executive, Operations, Security, Main, Namespace Health, Explorer)
- 📝 Structured logging
- 🏥 Health and readiness probes

### Production-Ready

- Non-privileged containers
- Read-only filesystem
- Minimal footprint (~15MB image, <10m CPU, <50MB RAM)
- Horizontal Pod Autoscaling (HPA) support
- NetworkPolicy and PodSecurity support
- RBAC with minimal required permissions

---

## 🔌 Integrations

> **Need alerts in Slack, PagerDuty, or SIEM?**  
> Zen Watcher writes `Observation` CRDs. Use [kubewatch](https://github.com/robusta-dev/kubewatch) or [Robusta](https://home.robusta.dev/) to route them to 30+ destinations—no coding required.

**Watch Events with kubectl:**
```bash
# All events
kubectl get observations -n zen-system

# High severity only
kubectl get observations -n zen-system -o json | \
  jq '.items[] | select(.spec.severity == "HIGH")'
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

- [Installation Guide](docs/DEPLOYMENT_HELM.md) - Complete deployment instructions
- [Source Adapters](docs/SOURCE_ADAPTERS.md) - How to add new sources
- [Manual Webhook Adapter](docs/manual-webhook-adapter.md) - Configure webhooks for Falco, Audit, and other tools
- [Observation API](docs/OBSERVATION_API_PUBLIC_GUIDE.md) - API reference
- [Integrations](docs/INTEGRATIONS.md) - How to consume Observations
- [Intelligent Pipeline](docs/INTELLIGENT_EVENT_PIPELINE.md) - Noise reduction and optimization
- [Deduplication](docs/DEDUPLICATION.md) - Deduplication strategies
- [CRD Documentation](docs/CRD.md) - Detailed CRD documentation

---

## 🤝 Contributing

Contributions welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

---

## 📄 License

Apache License 2.0 - See [LICENSE](LICENSE) for details.

---

**Repository:** [github.com/kube-zen/zen-watcher](https://github.com/kube-zen/zen-watcher)  
**Helm Charts:** [github.com/kube-zen/helm-charts](https://github.com/kube-zen/helm-charts)  
**Version:** 1.0.0-alpha  
**Go Version:** 1.23+ (tested on 1.23 and 1.24)  
**Kubernetes:** Client libs v0.28.15 (tested on clusters 1.26+)
