# Zen Watcher Documentation Index

Complete guide to Zen Watcher documentation.

---

## 📚 Documentation Files

### Getting Started (3 files)

1. **[README.md](README.md)** - Main project documentation
   - Features overview
   - Architecture
   - Quick start guide
   - Usage examples
   - Configuration

2. **[QUICK_START.md](QUICK_START.md)** - 5-minute setup guide
   - Fast installation
   - Basic usage
   - Monitoring setup
   - Troubleshooting

### Operations (7 files)

4. **[docs/OPERATIONS.md](docs/OPERATIONS.md)** - Day-to-day operations guide
   - Health checks
   - Common operations
   - Troubleshooting runbooks
   - Monitoring & alerting
   - Backup & recovery

5. **[docs/STABILITY.md](docs/STABILITY.md)** - Production readiness guide
   - HA configuration
   - Capacity planning
   - Failure modes
   - Recovery procedures

6. **[docs/OPERATIONAL_EXCELLENCE.md](docs/OPERATIONAL_EXCELLENCE.md)** - Ops best practices
   - Monitoring setup
   - Logging
   - Performance tuning
   - Security hardening

7. **[docs/SCALING.md](docs/SCALING.md)** - Scaling strategy and recommendations
   - Single-replica deployment (recommended)
   - Namespace sharding for scale-out
   - Leader election roadmap
   - Performance tuning

8. **[docs/FILTERING.md](docs/FILTERING.md)** - Source-level filtering guide
   - Filter configuration
   - Dynamic ConfigMap reloading (no restart required)
   - ObservationFilter CRD usage
   - Per-source filter rules
   - Examples and best practices
   - Troubleshooting

9. **[docs/SOURCE_ADAPTERS.md](docs/SOURCE_ADAPTERS.md)** - Writing new source adapters
   - SourceAdapter interface
   - Event normalization model
   - Implementation patterns (informer, webhook, polling)
   - ObservationMapping CRD for generic adapters
   - Best practices and examples
   - Testing guide

10. **[config/monitoring/README.md](config/monitoring/README.md)** - Monitoring guide
   - Prometheus metrics
   - Alert rules
   - VictoriaMetrics setup
   - Query examples

### Security (5 files)

10. **[docs/SECURITY.md](docs/SECURITY.md)** - Security policy
   - Vulnerability reporting
   - Security features
   - Best practices
   - Compliance
   - Incident response

11. **[docs/SECURITY_MODEL.md](docs/SECURITY_MODEL.md)** - Security model & threat analysis
   - Trust boundaries
   - Threat model
   - Security layers
   - Mitigations

12. **[docs/SECURITY_RBAC.md](docs/SECURITY_RBAC.md)** - RBAC permissions
   - Permission rationale
   - ClusterRole details
   - Security audit guide

13. **[docs/SBOM.md](docs/SBOM.md)** - Software Bill of Materials
   - SBOM generation
   - Vulnerability scanning
   - Supply chain security
   - Compliance

14. **[docs/COSIGN.md](docs/COSIGN.md)** - Image signing
   - Cosign setup
   - Image verification
   - Key management
   - CI/CD integration

### Deployment (2 files)

13. **[Helm Charts Repository](https://github.com/kube-zen/helm-charts)** - Helm chart guide
    - Installation
    - Configuration
    - Security settings
    - Troubleshooting
    - Chart values and upgrade paths

### Development (4 files)

15. **[CONTRIBUTING.md](CONTRIBUTING.md)** - Contribution guide
    - How to contribute
    - Development setup
    - Code standards
    - PR workflow
    - Review process

16. **[CHANGELOG.md](CHANGELOG.md)** - Version history
    - Release notes
    - Features added
    - Bug fixes

17. **[VERSIONING.md](VERSIONING.md)** - Versioning strategy
    - Semantic versioning
    - Image and chart sync
    - Release process


### Examples (2 files)

17. **[examples/README.md](examples/README.md)** - Integration examples
    - Query examples
    - Grafana setup
    - Prometheus config
    - Loki integration

18. **[config/dashboards/DASHBOARD_GUIDE.md](config/dashboards/DASHBOARD_GUIDE.md)** - Dashboard details
   - Panel descriptions
   - How to read metrics
   - Customization
   - Troubleshooting

---

## 🎯 Documentation by Role

### For Users

**First time?**
1. [README.md](README.md) - Start here
2. [QUICK_START.md](QUICK_START.md) - Get running fast
3. [examples/README.md](examples/README.md) - See examples

**Daily operations?**
1. [docs/OPERATIONAL_EXCELLENCE.md](docs/OPERATIONAL_EXCELLENCE.md)
2. [config/monitoring/README.md](config/monitoring/README.md)
3. [config/dashboards/README.md](config/dashboards/README.md)

### For Operators

**Deployment?**
1. [README.md#installation](README.md#installation)
2. [Helm Charts Repository](https://github.com/kube-zen/helm-charts)
3. [docs/OPERATIONAL_EXCELLENCE.md](docs/OPERATIONAL_EXCELLENCE.md)

**Security?**
1. [docs/SECURITY.md](docs/SECURITY.md)
2. [docs/SBOM.md](docs/SBOM.md)
3. [docs/COSIGN.md](docs/COSIGN.md)

### For Developers

**Contributing?**
1. [CONTRIBUTING.md](CONTRIBUTING.md)
2. [DEVELOPER_GUIDE.md](DEVELOPER_GUIDE.md)
3. [README.md](README.md#development)

**Monitoring?**
1. [config/monitoring/README.md](config/monitoring/README.md)
2. [config/dashboards/README.md](config/dashboards/README.md)
3. Source code: `pkg/metrics/definitions.go`

---

## 🔍 Find Information By Topic

### Installation
- [README.md#quick-start](README.md#quick-start)
- [QUICK_START.md](QUICK_START.md)
- [charts/zen-watcher/README.md](charts/zen-watcher/README.md)

### Configuration
- [README.md#configuration](README.md#configuration)
- [docs/FILTERING.md](docs/FILTERING.md) - Source-level filtering
- [charts/zen-watcher/values.yaml](charts/zen-watcher/values.yaml) - Helm chart values
- [docs/OPERATIONAL_EXCELLENCE.md](docs/OPERATIONAL_EXCELLENCE.md)

### Scaling
- [docs/SCALING.md](docs/SCALING.md) - Complete scaling strategy
- [README.md#scaling](README.md#scaling) - Quick reference
- [docs/OPERATIONAL_EXCELLENCE.md](docs/OPERATIONAL_EXCELLENCE.md) - Resource management

### Security
- [docs/SECURITY.md](docs/SECURITY.md)
- [docs/SBOM.md](docs/SBOM.md)
- [docs/COSIGN.md](docs/COSIGN.md)
- [Helm Charts Repository - Security](https://github.com/kube-zen/helm-charts) - Chart security settings

### Monitoring
- [config/monitoring/README.md](config/monitoring/README.md)
- [config/dashboards/README.md](config/dashboards/README.md)
- [config/dashboards/DASHBOARD_GUIDE.md](config/dashboards/DASHBOARD_GUIDE.md)
- [README.md#monitoring-dashboards](README.md#monitoring-dashboards)

### Troubleshooting
- [QUICK_START.md#common-issues](QUICK_START.md#common-issues)
- [docs/OPERATIONAL_EXCELLENCE.md](docs/OPERATIONAL_EXCELLENCE.md)
- [README.md#troubleshooting](README.md#troubleshooting)

### API Reference
- [README.md#api-endpoints](README.md#api-endpoints)
- [README.md#crd-schema](README.md#crd-schema)
- [monitoring/README.md#metrics-summary](monitoring/README.md#metrics-summary)

---

## 📖 Reading Order

### New Users
1. README.md
2. QUICK_START.md
3. examples/README.md
4. docs/OPERATIONAL_EXCELLENCE.md

### Operators
1. [README.md#installation](README.md#installation)
2. [Helm Charts Repository](https://github.com/kube-zen/helm-charts)
3. [docs/SECURITY.md](docs/SECURITY.md)
4. [docs/OPERATIONAL_EXCELLENCE.md](docs/OPERATIONAL_EXCELLENCE.md)
5. [config/monitoring/README.md](config/monitoring/README.md)

### Developers
1. [DEVELOPER_GUIDE.md](DEVELOPER_GUIDE.md)
2. [CONTRIBUTING.md](CONTRIBUTING.md)
3. Source code in `pkg/` and `cmd/`
4. [README.md#development](README.md#development)

---

## 🎯 Quick Links

| Topic | Document |
|-------|----------|
| Getting Started | [README.md](README.md) |
| 5-min Setup | [QUICK_START.md](QUICK_START.md) |
| Helm Install | [README.md#installation](README.md#installation) / [Helm Charts](https://github.com/kube-zen/helm-charts) |
| Filtering | [docs/FILTERING.md](docs/FILTERING.md) |
| Scaling | [docs/SCALING.md](docs/SCALING.md) |
| Source Adapters | [docs/SOURCE_ADAPTERS.md](docs/SOURCE_ADAPTERS.md) |
| Security | [docs/SECURITY.md](docs/SECURITY.md) |
| Operations | [docs/OPERATIONAL_EXCELLENCE.md](docs/OPERATIONAL_EXCELLENCE.md) |
| Monitoring | [monitoring/README.md](monitoring/README.md) |
| Dashboard | [dashboards/README.md](dashboards/README.md) |
| Examples | [examples/README.md](examples/README.md) |
| Contributing | [CONTRIBUTING.md](CONTRIBUTING.md) |

---

## 📝 Document Statistics

- **Total Documentation Files**: 50+ (including all markdown files)
- **Total Lines**: 10,000+
- **Getting Started**: 3 guides
- **Operations**: 7 guides (including filtering, scaling, stability, and source adapters)
- **Security**: 5 guides (including RBAC and threat model)
- **Deployment**: Installation guide + Helm charts repository
- **Development**: 4 guides (including versioning, changelog)
- **Examples**: 3 use cases + integration examples

---

## 🆕 Latest Updates

See [CHANGELOG.md](CHANGELOG.md) for version history and updates.

---

## 💡 Tips

1. **Start with README.md** - Gives complete overview
2. **Use QUICK_START.md** - For fast deployment
3. **Bookmark INDEX** - Quick access to all docs
4. **Check examples/** - Learn from working configurations
5. **Read OPERATIONAL_EXCELLENCE** - Production best practices

---

**Need help? See [README.md#support](README.md#support)**

