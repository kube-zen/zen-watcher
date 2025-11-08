# Changelog

All notable changes to Zen Watcher will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2024-11-04

### 🎉 Initial Release

First public release of Zen Watcher - Universal Kubernetes Event Aggregator.

### ✨ Features

**Core Functionality**:
- CRD-based event storage (`ZenEvent`)
- Multi-source event collection (Trivy, Falco, Kyverno, Audit, Kube-bench)
- Extensible event categories (security, compliance, performance, observability, custom)
- Flexible event sources and types
- Zero external dependencies

**Monitoring & Observability**:
- 20+ Prometheus metrics
- Pre-built Grafana dashboard with 16 panels
- 20+ alerting rules
- Health and readiness probes
- SLO tracking

**Security**:
- Non-privileged containers
- Read-only root filesystem
- NetworkPolicy support
- Pod Security Standards (restricted)
- RBAC least-privilege
- Image signing support (Cosign)
- SBOM generation

**Deployment**:
- Production-ready Helm chart
- Kubernetes manifests
- Comprehensive configuration options
- ServiceMonitor support (Prometheus Operator)
- High availability support
- Autoscaling (HPA)

**Documentation**:
- Complete user guide
- Security best practices
- Operations guide
- Monitoring setup guide
- API reference
- Contributing guidelines
- Examples and tutorials

### 📦 Components

- **Watchers**: Trivy, Falco, Kyverno, Audit, Kube-bench
- **CRD Writer**: Converts events to Kubernetes CRDs
- **Metrics**: Prometheus-compatible metrics endpoint
- **API**: Health, readiness, status, and metrics endpoints

### 🎯 Use Cases

- Security event aggregation
- Compliance monitoring
- Centralized observability
- GitOps integration
- Multi-tool correlation

---

## Future Releases

See [Roadmap](README.md#roadmap) for planned features.
