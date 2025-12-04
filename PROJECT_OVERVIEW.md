# Zen Watcher - Project Overview

## What is Zen Watcher?

Zen Watcher is an open-source Kubernetes operator that aggregates security, compliance, and observability events from multiple sources into a unified, extensible CRD-based system. Core features are production-ready and battle-tested.

**Version**: 1.1.0  
**License**: Apache 2.0  
**Language**: Go 1.23+ (tested on 1.23 and 1.24)  
**Platform**: Kubernetes 1.26-1.30 (client libs: k8s.io/* v0.28.15)  
**Architecture**: Modular adapter-based with 6 first-class sources + generic CRD adapter

---

## 🎯 Purpose

Provide a **central event aggregation hub** for Kubernetes clusters that:
- Collects events from multiple security and compliance tools
- Stores events as native Kubernetes CRDs
- Enables easy integration with observability tools (Grafana, Prometheus, Loki)
- Supports extensible event categories and sources
- Operates independently without external dependencies

---

## 🏗️ Architecture

**Design Principles**:
- Kubernetes-native (CRD-based storage)
- Zero external dependencies
- Extensible by design
- Security-first approach
- Production-ready from day one

**Components**:
1. **Source Adapters** - Modular adapters for 6 sources (Trivy, Kyverno, Falco, Audit, Checkov, KubeBench)
2. **Generic CRD Adapter** - ObservationMapping-driven integration for custom CRDs
3. **Centralized ObservationCreator** - Filtering, dedup, normalization, CRD creation
4. **Filter System** - Dynamic filtering via ConfigMap + ObservationFilter CRDs
5. **Metrics Exporter** - 20+ Prometheus metrics
6. **HTTP Server** - Webhook endpoints + health/ready probes

---

## 📦 Key Components

### Observation CRD

The core data model - stores all events as Kubernetes resources:

```yaml
apiVersion: zen.kube-zen.io/v1
kind: Observation
metadata:
  name: trivy-vuln-abc123
  namespace: production
  labels:
    source: trivy
    category: security
    severity: HIGH
spec:
  source: trivy
  category: security
  severity: HIGH
  eventType: vulnerability
  resource:
    kind: Deployment
    name: myapp
    namespace: production
  detectedAt: "2024-12-04T10:00:00Z"
  details:
    cve: "CVE-2024-1234"
    package: "openssl"
    fixVersion: "1.1.1w"
  ttlSecondsAfterCreation: 604800  # 7 days
```

### Source Adapters (Modular Architecture)

**First-Class Adapters** (6 sources):
- **TrivyAdapter**: VulnerabilityReport CRD informer
- **KyvernoAdapter**: PolicyReport CRD informer
- **FalcoAdapter**: Webhook-based (HTTP POST /falco/webhook)
- **AuditAdapter**: Webhook-based (HTTP POST /audit/webhook)
- **CheckovAdapter**: ConfigMap polling (5-minute interval)
- **KubeBenchAdapter**: ConfigMap polling (5-minute interval)

**Generic Adapter**:
- **CRDSourceAdapter**: ObservationMapping CRD-driven for custom integrations

### Metrics System

20+ Prometheus metrics covering:
- Event collection and processing (by source, category, severity)
- Adapter lifecycle and performance
- Filter decisions and reload status
- Deduplication effectiveness
- Webhook health and backpressure
- GC performance and observation backlog
- ObservationMapping usage (for generic CRD adapter)

---

## 🔐 Security Features

Built with security-first principles:

- **Container Security**: Non-privileged, read-only root filesystem, dropped capabilities
- **Network Security**: NetworkPolicy for micro-segmentation
- **RBAC**: Least-privilege access control
- **Pod Security**: Restricted PSS profile
- **Supply Chain**: SBOM generation, image signing support
- **Compliance**: CIS, NIST, PCI-DSS compatible

---

## 📊 Monitoring

### Grafana Dashboard

16-panel dashboard providing:
- Real-time health monitoring
- Event rate tracking
- Category and severity distribution
- Watcher performance
- Resource usage visualization

### Prometheus Metrics

Comprehensive metrics for:
- Event processing (rate, volume, errors)
- Watcher health (status, errors, duration)
- CRD operations (latency, success rate)
- API performance (requests, latency)
- System resources (CPU, memory, goroutines)

### Alerting

20+ pre-configured alerts:
- Critical: Service down, high event rates, SLO violations
- Warning: Resource pressure, slow operations
- Info: Configuration notices

---

## 🚀 Deployment Options

### Helm Chart (Recommended)

```bash
helm install zen-watcher ./charts/zen-watcher
```

**Features**:
- One-command installation
- Configurable via values.yaml
- Automatic CRD installation
- Security defaults
- Optional features (HA, monitoring, NetworkPolicy)

### Kubernetes Manifests

```bash
kubectl apply -f src/crd/zen_event_crd.yaml
kubectl apply -f deploy/k8s-deployment.yaml
```

**Suitable for**:
- Simple deployments
- Custom configurations
- GitOps workflows

---

## 🎯 Use Cases

1. **Security Event Aggregation**
   - Centralize vulnerability scanning results
   - Track runtime security threats
   - Monitor policy violations

2. **Compliance Monitoring**
   - Kubernetes audit trail
   - CIS benchmark compliance
   - Regulatory reporting

3. **Observability Integration**
   - Single source for Grafana dashboards
   - Prometheus metrics collection
   - Loki log aggregation

4. **Multi-Tool Correlation**
   - Correlate events across tools
   - Unified view of security posture
   - Trend analysis

5. **GitOps Workflows**
   - Events as Kubernetes resources
   - Version controlled
   - Declarative management

---

## 📚 Documentation Structure

### Getting Started
- `README.md` - Main documentation
- `charts/zen-watcher/README.md` - Helm installation guide
- `examples/README.md` - Integration examples

### Operations
- `docs/OPERATIONAL_EXCELLENCE.md` - Operations best practices
- `monitoring/README.md` - Monitoring setup
- `dashboards/README.md` - Dashboard guide

### Security
- `docs/SECURITY.md` - Security policy
- `docs/SBOM.md` - Software Bill of Materials
- `docs/COSIGN.md` - Image signing guide

### Development
- `CONTRIBUTING.md` - Contribution guidelines
- `CHANGELOG.md` - Version history
- Source code documentation (inline)

---

## 🌟 Why Zen Watcher?

### Problems It Solves

**Before**: 
- Events scattered across multiple tools
- No unified view of security posture
- Complex integration with observability tools
- Each tool requires separate monitoring

**With Zen Watcher**:
- ✅ Single event aggregation point
- ✅ Unified CRD-based storage
- ✅ Easy Grafana/Prometheus integration
- ✅ One dashboard for all tools
- ✅ Kubernetes-native approach

### Benefits

- **Simplicity**: One operator, multiple sources
- **Extensibility**: Add custom categories and sources
- **Native**: Kubernetes CRDs, no external services
- **Observable**: Built-in metrics and dashboards
- **Secure**: Security best practices by default
- **Open**: Apache 2.0 license, community-driven

---

## 🔧 Technology Stack

- **Language**: Go 1.23+ (tested on 1.23 and 1.24)
- **Platform**: Kubernetes 1.28+
- **Storage**: Kubernetes CRDs
- **Metrics**: Prometheus format
- **Container**: Alpine-based, non-root
- **Deployment**: Helm 3.8+

---

## 📊 Project Stats

- **Version**: 1.0.0
- **Go Modules**: 30+
- **Metrics**: 20+ families
- **Alerts**: 20+ rules
- **Dashboard Panels**: 16
- **Documentation**: 13 guides
- **Examples**: 5+ integration examples
- **Security**: 100% non-privileged

---

## 🤝 Community

### Contributing

We welcome contributions! See `CONTRIBUTING.md` for:
- How to contribute
- Development setup
- Code standards
- Review process

### Support

- **Issues**: GitHub Issues
- **Discussions**: GitHub Discussions
- **Security**: security@kube-zen.com
- **General**: support@kube-zen.com

### Roadmap

See `README.md#roadmap` for upcoming features:
- Event deduplication
- Multi-cluster support
- Webhook notifications
- Event retention policies
- AI-powered correlation
- Plugin system

---

## 🎓 Getting Help

1. **Quick Start**: See `README.md`
2. **Helm Guide**: See `charts/zen-watcher/README.md`
3. **Operations**: See `docs/OPERATIONAL_EXCELLENCE.md`
4. **Monitoring**: See `monitoring/README.md`
5. **Security**: See `docs/SECURITY.md`
6. **Examples**: See `examples/` directory

---

## ⭐ Star Us!

If you find Zen Watcher useful, please star the repository on GitHub!

---

**Built with ❤️ for the Kubernetes community**

