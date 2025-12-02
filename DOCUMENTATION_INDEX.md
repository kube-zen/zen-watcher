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

3. **[PROJECT_OVERVIEW.md](PROJECT_OVERVIEW.md)** - Project summary
   - Purpose and goals
   - Architecture details
   - Technology stack
   - Use cases

### Operations (5 files)

4. **[docs/OPERATIONAL_EXCELLENCE.md](docs/OPERATIONAL_EXCELLENCE.md)** - Ops best practices
   - Health checks
   - Monitoring setup
   - Logging
   - High availability
   - Backup & recovery
   - Troubleshooting runbooks

5. **[docs/SCALING.md](docs/SCALING.md)** - Scaling strategy and recommendations
   - Single-replica deployment (recommended)
   - Namespace sharding for scale-out
   - Leader election roadmap
   - Performance tuning

6. **[docs/FILTERING.md](docs/FILTERING.md)** - Source-level filtering guide
   - Filter configuration
   - Dynamic ConfigMap reloading (no restart required)
   - Per-source filter rules
   - Examples and best practices
   - Troubleshooting

7. **[docs/SOURCE_ADAPTERS.md](docs/SOURCE_ADAPTERS.md)** - Writing new source adapters
   - SourceAdapter interface
   - Event normalization model
   - Implementation patterns (informer, webhook, polling)
   - Best practices and examples
   - Testing guide

8. **[monitoring/README.md](monitoring/README.md)** - Monitoring guide
   - Prometheus metrics
   - Alert rules
   - VictoriaMetrics setup
   - Query examples

9. **[dashboards/README.md](dashboards/README.md)** - Dashboard documentation
   - Grafana setup
   - Dashboard features
   - Metrics reference
   - Query examples

### Security (3 files)

10. **[docs/SECURITY.md](docs/SECURITY.md)** - Security policy
   - Vulnerability reporting
   - Security features
   - Best practices
   - Compliance
   - Incident response

11. **[docs/SBOM.md](docs/SBOM.md)** - Software Bill of Materials
   - SBOM generation
   - Vulnerability scanning
   - Supply chain security
   - Compliance

12. **[docs/COSIGN.md](docs/COSIGN.md)** - Image signing
   - Cosign setup
   - Image verification
   - Key management
   - CI/CD integration

### Deployment (2 files)

13. **[charts/zen-watcher/README.md](charts/zen-watcher/README.md)** - Helm chart guide
    - Installation
    - Configuration
    - Security settings
    - Troubleshooting

14. **[charts/HELM_SUMMARY.md](charts/HELM_SUMMARY.md)** - Helm features
    - Security features
    - Configuration options
    - Compliance info

### Development (2 files)

15. **[CONTRIBUTING.md](CONTRIBUTING.md)** - Contribution guide
    - How to contribute
    - Development setup
    - Code standards
    - Review process

16. **[CHANGELOG.md](CHANGELOG.md)** - Version history
    - Release notes
    - Features added
    - Bug fixes

### Examples (2 files)

17. **[examples/README.md](examples/README.md)** - Integration examples
    - Query examples
    - Grafana setup
    - Prometheus config
    - Loki integration

18. **[dashboards/DASHBOARD_GUIDE.md](dashboards/DASHBOARD_GUIDE.md)** - Dashboard details
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
2. [monitoring/README.md](monitoring/README.md)
3. [dashboards/README.md](dashboards/README.md)

### For Operators

**Deployment?**
1. [charts/zen-watcher/README.md](charts/zen-watcher/README.md)
2. [docs/OPERATIONAL_EXCELLENCE.md](docs/OPERATIONAL_EXCELLENCE.md)

**Security?**
1. [docs/SECURITY.md](docs/SECURITY.md)
2. [docs/SBOM.md](docs/SBOM.md)
3. [docs/COSIGN.md](docs/COSIGN.md)

### For Developers

**Contributing?**
1. [CONTRIBUTING.md](CONTRIBUTING.md)
2. [PROJECT_OVERVIEW.md](PROJECT_OVERVIEW.md)
3. [README.md](README.md#development)

**Monitoring?**
1. [monitoring/README.md](monitoring/README.md)
2. [dashboards/README.md](dashboards/README.md)
3. Source code: `src/metrics/metrics.go`

---

## 🔍 Find Information By Topic

### Installation
- [README.md#quick-start](README.md#quick-start)
- [QUICK_START.md](QUICK_START.md)
- [charts/zen-watcher/README.md](charts/zen-watcher/README.md)

### Configuration
- [README.md#configuration](README.md#configuration)
- [docs/FILTERING.md](docs/FILTERING.md) - Source-level filtering
- [helm/zen-watcher/values.yaml](helm/zen-watcher/values.yaml)
- [docs/OPERATIONAL_EXCELLENCE.md](docs/OPERATIONAL_EXCELLENCE.md)

### Scaling
- [docs/SCALING.md](docs/SCALING.md) - Complete scaling strategy
- [README.md#scaling](README.md#scaling) - Quick reference
- [docs/OPERATIONAL_EXCELLENCE.md](docs/OPERATIONAL_EXCELLENCE.md) - Resource management

### Security
- [docs/SECURITY.md](docs/SECURITY.md)
- [docs/SBOM.md](docs/SBOM.md)
- [docs/COSIGN.md](docs/COSIGN.md)
- [charts/zen-watcher/README.md#security](charts/zen-watcher/README.md#security)

### Monitoring
- [monitoring/README.md](monitoring/README.md)
- [dashboards/README.md](dashboards/README.md)
- [dashboards/DASHBOARD_GUIDE.md](dashboards/DASHBOARD_GUIDE.md)
- [README.md#monitoring-dashboards](README.md#monitoring-dashboards)

### Troubleshooting
- [QUICK_START.md#common-issues](QUICK_START.md#common-issues)
- [docs/OPERATIONAL_EXCELLENCE.md](docs/OPERATIONAL_EXCELLENCE.md)
- [charts/zen-watcher/README.md#troubleshooting](charts/zen-watcher/README.md#troubleshooting)

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
1. charts/zen-watcher/README.md
2. docs/SECURITY.md
3. docs/OPERATIONAL_EXCELLENCE.md
4. monitoring/README.md

### Developers
1. PROJECT_OVERVIEW.md
2. CONTRIBUTING.md
3. Source code in `src/`
4. README.md#development

---

## 🎯 Quick Links

| Topic | Document |
|-------|----------|
| Getting Started | [README.md](README.md) |
| 5-min Setup | [QUICK_START.md](QUICK_START.md) |
| Helm Install | [charts/zen-watcher/README.md](charts/zen-watcher/README.md) |
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

- **Total Documentation Files**: 18
- **Total Lines**: 6,000+
- **Getting Started**: 3 guides
- **Operations**: 6 guides (including filtering, scaling, and source adapters)
- **Security**: 3 guides
- **Deployment**: 2 guides
- **Development**: 2 guides
- **Examples**: 2 guides

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

