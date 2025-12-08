# Zen Watcher Documentation Index

Complete guide to all Zen Watcher documentation files.

---

## 📚 Core Documentation

### Security & Compliance
- **[SECURITY_COMPLIANCE_DASHBOARD.md](SECURITY_COMPLIANCE_DASHBOARD.md)** - Comprehensive security and compliance monitoring dashboard design
- **[SECURITY_COMPLIANCE_IMPLEMENTATION_PLAN.md](SECURITY_COMPLIANCE_IMPLEMENTATION_PLAN.md)** - Implementation roadmap and technical plan

### Getting Started

1. **[README.md](../README.md)** - Main project documentation
   - Features overview (9 sources: Trivy, Falco, Kyverno, Checkov, KubeBench, Audit, cert-manager, sealed-secrets, Kubernetes Events)
   - Architecture overview
   - Quick start guide (4 minutes to working system)
   - YAML-only source creation
   - ObservationSourceConfig CRD
   - Auto-optimization overview
   - Configuration reference
   - Usage examples

2. **[QUICK_START.md](../QUICK_START.md)** - 5-minute setup guide
   - Fast installation
   - Basic usage
   - Monitoring setup
   - Troubleshooting

---

## 📖 Operations Documentation

### Core Operations

3. **[OPERATIONS.md](OPERATIONS.md)** - Day-to-day operations guide
   - Health checks
   - Common operations
   - Troubleshooting runbooks
   - Monitoring & alerting
   - Backup & recovery

4. **[OPERATIONAL_EXCELLENCE.md](OPERATIONAL_EXCELLENCE.md)** - Ops best practices
   - Monitoring setup
   - Logging
   - Performance tuning
   - Security hardening

5. **[STABILITY.md](STABILITY.md)** - Production readiness guide
   - HA configuration
   - Capacity planning
   - Failure modes
   - Recovery procedures

6. **[SCALING.md](SCALING.md)** - Scaling strategy and recommendations
   - Single-replica deployment (recommended)
   - Namespace sharding for scale-out
   - Leader election roadmap
   - Performance tuning

### Source Management

7. **[SOURCE_ADAPTERS.md](SOURCE_ADAPTERS.md)** - Complete source adapter guide ⭐ **UPDATED**
   - YAML-only source creation (no code needed!)
   - All 4 input methods: logs, webhooks, ConfigMaps, CRDs
   - ObservationSourceConfig CRD documentation
   - Auto-optimization configuration
   - Thresholds and warnings
   - Processing order control
   - Best practices and examples
   - SourceAdapter interface (for advanced users)

8. **[FILTERING.md](FILTERING.md)** - Source-level filtering guide
   - Filter configuration
   - Dynamic ConfigMap reloading (no restart required)
   - ObservationFilter CRD usage
   - Per-source filter rules
   - Examples and best practices
   - Troubleshooting

9. **[DEDUPLICATION.md](DEDUPLICATION.md)** - Deduplication system
   - SHA-256 content fingerprinting
   - Per-source rate limiting
   - Time-bucketed deduplication
   - Configuration and tuning
   - Performance characteristics
   - Troubleshooting

10. **[NORMALIZATION.md](NORMALIZATION.md)** - Event normalization
    - Severity normalization rules
    - Category and event type assignment
    - Resource normalization
    - Tool-specific mappings
    - Normalization in processing pipeline

### Auto-Optimization

11. **[OPTIMIZATION_USAGE.md](OPTIMIZATION_USAGE.md)** - Auto-optimization usage guide ⭐ **NEW**
    - CLI commands (analyze, apply, auto, history, list)
    - Configuration via ObservationSourceConfig
    - Processing order logic
    - Metrics and alerts
    - Best practices
    - Examples

12. **[DASHBOARD_OPTIMIZATION_UPDATES.md](DASHBOARD_OPTIMIZATION_UPDATES.md)** - Dashboard updates
    - Optimization insights panels
    - Source efficiency metrics
    - Auto-optimization status
    - Grafana dashboard configuration

---

## 🔐 Security Documentation

16. **[SECURITY.md](SECURITY.md)** - Security policy
    - Vulnerability reporting
    - Security features
    - Best practices
    - Compliance
    - Incident response

17. **[SECURITY_MODEL.md](SECURITY_MODEL.md)** - Security model & threat analysis
    - Trust boundaries
    - Threat model
    - Security layers
    - Mitigations

18. **[SECURITY_RBAC.md](SECURITY_RBAC.md)** - RBAC permissions
    - Permission rationale
    - ClusterRole details
    - Security audit guide

19. **[SECURITY_THREAT_MODEL.md](SECURITY_THREAT_MODEL.md)** - Threat modeling
    - Threat identification
    - Risk assessment
    - Mitigation strategies

20. **[SBOM.md](SBOM.md)** - Software Bill of Materials
    - SBOM generation
    - Vulnerability scanning
    - Supply chain security
    - Compliance

21. **[COSIGN.md](COSIGN.md)** - Image signing
    - Cosign setup
    - Image verification
    - Key management
    - CI/CD integration

---

## 🏗️ Architecture Documentation

22. **[ARCHITECTURE.md](ARCHITECTURE.md)** - System architecture
    - Design principles
    - Component architecture
    - Data flow
    - Security model
    - Performance characteristics
    - Future considerations

23. **[CRD.md](CRD.md)** - Custom Resource Definitions
    - Observation CRD schema
    - ObservationFilter CRD
    - ObservationMapping CRD
    - ObservationSourceConfig CRD
    - Schema reference
    - Sync process

---

## 🔌 Integration Documentation

26. **[INTEGRATIONS.md](INTEGRATIONS.md)** - Integration guide
    - Security tool integrations (6 tools)
    - External service integrations
    - kubewatch/Robusta setup
    - OpenAPI schema
    - Schema sync guidance
    - Controller examples

27. **[TESTING_FALCO_AUDIT.md](TESTING_FALCO_AUDIT.md)** - Falco and audit testing
    - Testing procedures
    - Example configurations
    - Troubleshooting

---

## 📊 Monitoring & Observability

29. **[config/monitoring/README.md](../config/monitoring/README.md)** - Monitoring guide
    - Prometheus metrics
    - Alert rules
    - VictoriaMetrics setup
    - Query examples

30. **[config/dashboards/DASHBOARD_GUIDE.md](../config/dashboards/DASHBOARD_GUIDE.md)** - Dashboard details
    - Panel descriptions
    - How to read metrics
    - Customization
    - Troubleshooting

31. **[LOGGING.md](LOGGING.md)** - Logging guide
    - Structured logging
    - Log levels
    - Correlation IDs
    - Best practices

---

## 🚀 Development Documentation

32. **[DEVELOPER_GUIDE.md](DEVELOPER_GUIDE.md)** - Developer guide
    - Development setup
    - Code structure
    - Building and testing
    - Contribution workflow

33. **[DEVELOPMENT.md](DEVELOPMENT.md)** - Development practices
    - Development workflow
    - Code standards
    - Testing strategies

34. **[PROJECT_STRUCTURE.md](PROJECT_STRUCTURE.md)** - Project structure
    - Directory layout
    - Code organization
    - File naming conventions

35. **[CONTRIBUTING.md](../CONTRIBUTING.md)** - Contribution guide
    - How to contribute
    - Development setup
    - Code standards
    - PR workflow
    - Review process

36. **[CHANGELOG.md](../CHANGELOG.md)** - Version history
    - Release notes
    - Features added
    - Bug fixes

37. **[VERSIONING.md](VERSIONING.md)** - Versioning strategy
    - Semantic versioning
    - Image and chart sync
    - Release process

38. **[RELEASE.md](RELEASE.md)** - Release process
    - Release checklist
    - Version tagging
    - Changelog generation

---

    - Best practices

---

## 🎯 Configuration & Deployment

45. **[DEPLOYMENT_SCENARIOS.md](DEPLOYMENT_SCENARIOS.md)** - Deployment scenarios
    - Different deployment patterns
    - Use case examples
    - Configuration examples

46. **[PERFORMANCE.md](PERFORMANCE.md)** - Performance guide
    - Performance characteristics
    - Benchmarking
    - Optimization tips
    - Resource usage

---

## 📦 Additional Resources

47. **[examples/README.md](../examples/README.md)** - Integration examples
    - Query examples
    - Grafana setup
    - Prometheus config
    - Loki integration

48. **[ADAPTER_MIGRATION.md](ADAPTER_MIGRATION.md)** - Adapter migration guide
    - Migration from old adapters
    - Compatibility notes
    - Migration steps

49. **[THIRD_PARTY_LICENSES.md](THIRD_PARTY_LICENSES.md)** - Third-party licenses
    - License information
    - Dependencies
    - Compliance

---

## 🎯 Documentation by Role

### For New Users

**First time?**
1. [README.md](../README.md) - Start here for overview
2. [QUICK_START.md](../QUICK_START.md) - Get running in 5 minutes
3. [examples/README.md](../examples/README.md) - See examples

**Adding a new source?**
1. [README.md#adding-new-sources-just-yaml](../README.md) - YAML-only guide
2. [SOURCE_ADAPTERS.md](SOURCE_ADAPTERS.md) - Complete configuration guide
3. [OPTIMIZATION_USAGE.md](OPTIMIZATION_USAGE.md) - Auto-optimization setup

**Daily operations?**
1. [OPERATIONAL_EXCELLENCE.md](OPERATIONAL_EXCELLENCE.md) - Best practices
2. [config/monitoring/README.md](../config/monitoring/README.md) - Monitoring
3. [config/dashboards/DASHBOARD_GUIDE.md](../config/dashboards/DASHBOARD_GUIDE.md) - Dashboards

### For Operators

**Deployment?**
1. [README.md#installation](../README.md#installation) - Installation guide
2. [Helm Charts Repository](https://github.com/kube-zen/helm-charts) - Helm charts
3. [OPERATIONAL_EXCELLENCE.md](OPERATIONAL_EXCELLENCE.md) - Production setup

**Configuration?**
1. [README.md#configuration](../README.md#configuration) - Environment variables
2. [SOURCE_ADAPTERS.md](SOURCE_ADAPTERS.md) - ObservationSourceConfig CRD
3. [FILTERING.md](FILTERING.md) - Source-level filtering
4. [OPTIMIZATION_USAGE.md](OPTIMIZATION_USAGE.md) - Auto-optimization

**Security?**
1. [SECURITY.md](SECURITY.md) - Security policy
2. [SBOM.md](SBOM.md) - Software Bill of Materials
3. [COSIGN.md](COSIGN.md) - Image signing

**Monitoring?**
1. [config/monitoring/README.md](../config/monitoring/README.md) - Prometheus metrics
2. [OPTIMIZATION_USAGE.md](OPTIMIZATION_USAGE.md) - Optimization metrics
3. [DASHBOARD_OPTIMIZATION_UPDATES.md](DASHBOARD_OPTIMIZATION_UPDATES.md) - Dashboard setup

### For Developers

**Contributing?**
1. [CONTRIBUTING.md](../CONTRIBUTING.md) - Contribution guide
2. [DEVELOPER_GUIDE.md](DEVELOPER_GUIDE.md) - Developer guide
3. [PROJECT_STRUCTURE.md](PROJECT_STRUCTURE.md) - Code structure

**Adding features?**
1. [SOURCE_ADAPTERS.md](SOURCE_ADAPTERS.md) - Source adapter guide
2. [ARCHITECTURE.md](ARCHITECTURE.md) - Architecture overview
3. [DEVELOPMENT.md](DEVELOPMENT.md) - Development practices

**Understanding internals?**
1. [ARCHITECTURE.md](ARCHITECTURE.md) - System architecture
2. [DEDUPLICATION.md](DEDUPLICATION.md) - Deduplication system
3. [NORMALIZATION.md](NORMALIZATION.md) - Event normalization

---

## 🔍 Find Information By Topic

### Adding New Sources

- **[README.md#adding-new-sources](../README.md)** - YAML-only source creation ⭐
- **[SOURCE_ADAPTERS.md](SOURCE_ADAPTERS.md)** - Complete guide with all 4 input methods ⭐
- **[INTEGRATIONS.md](INTEGRATIONS.md)** - Integration examples

### Auto-Optimization

- **[INTELLIGENT_EVENT_PIPELINE.md](INTELLIGENT_EVENT_PIPELINE.md)** - Complete guide to the intelligent event integrity system ⭐
- **[OPTIMIZATION_USAGE.md](OPTIMIZATION_USAGE.md)** - Usage guide and CLI commands ⭐
- **[SOURCE_ADAPTERS.md#auto-optimization](SOURCE_ADAPTERS.md)** - Configuration
- **[DASHBOARD_OPTIMIZATION_UPDATES.md](DASHBOARD_OPTIMIZATION_UPDATES.md)** - Dashboard setup

### Thresholds & Warnings

- **[SOURCE_ADAPTERS.md#thresholds-and-warnings](SOURCE_ADAPTERS.md)** - Complete threshold documentation ⭐
- **[README.md#advanced-configuration](../README.md)** - Quick overview
- **[OPTIMIZATION_USAGE.md](OPTIMIZATION_USAGE.md)** - Alert configuration

### ObservationSourceConfig CRD

- **[SOURCE_ADAPTERS.md](SOURCE_ADAPTERS.md)** - Complete CRD documentation ⭐
- **[README.md#advanced-configuration](../README.md)** - Quick reference
- **[CRD.md](CRD.md)** - CRD schema reference

### Installation

- [README.md#quick-start](../README.md#quick-start) - Quick start guide
- [QUICK_START.md](../QUICK_START.md) - 5-minute setup
- [Helm Charts Repository](https://github.com/kube-zen/helm-charts) - Helm charts

### Configuration

- [README.md#configuration](../README.md#configuration) - Environment variables
- [SOURCE_ADAPTERS.md](SOURCE_ADAPTERS.md) - ObservationSourceConfig CRD
- [FILTERING.md](FILTERING.md) - Source-level filtering
- [OPERATIONAL_EXCELLENCE.md](OPERATIONAL_EXCELLENCE.md) - Best practices

### Scaling

- [SCALING.md](SCALING.md) - Complete scaling strategy
- [README.md#scaling](../README.md#scaling) - Quick reference
- [OPERATIONAL_EXCELLENCE.md](OPERATIONAL_EXCELLENCE.md) - Resource management

### Deduplication

- [DEDUPLICATION.md](DEDUPLICATION.md) - Complete deduplication documentation
- [README.md#intelligent-noise-reduction](../README.md#intelligent-noise-reduction) - Quick reference
- [ARCHITECTURE.md#intelligent-event-integrity](ARCHITECTURE.md#intelligent-event-integrity) - Design principles

### Normalization

- [NORMALIZATION.md](NORMALIZATION.md) - Event normalization rules and mappings
- [SOURCE_ADAPTERS.md](SOURCE_ADAPTERS.md) - How to implement normalization in adapters

### Security

- [SECURITY.md](SECURITY.md) - Security policy
- [SBOM.md](SBOM.md) - Software Bill of Materials
- [COSIGN.md](COSIGN.md) - Image signing
- [SECURITY_MODEL.md](SECURITY_MODEL.md) - Security model
- [SECURITY_RBAC.md](SECURITY_RBAC.md) - RBAC permissions
- [Helm Charts Repository - Security](https://github.com/kube-zen/helm-charts) - Chart security settings

### Monitoring

- [config/monitoring/README.md](../config/monitoring/README.md) - Prometheus metrics
- [config/dashboards/DASHBOARD_GUIDE.md](../config/dashboards/DASHBOARD_GUIDE.md) - Dashboard guide
- [OPTIMIZATION_USAGE.md](OPTIMIZATION_USAGE.md) - Optimization metrics
- [README.md#observability](../README.md#observability) - Quick reference

### Troubleshooting

- [QUICK_START.md#common-issues](../QUICK_START.md#common-issues) - Common issues
- [OPERATIONS.md](OPERATIONS.md) - Troubleshooting runbooks
- [OPERATIONAL_EXCELLENCE.md](OPERATIONAL_EXCELLENCE.md) - Operational guide
- [README.md#troubleshooting](../README.md#troubleshooting) - Quick troubleshooting

---

## 🎯 Quick Links

| Topic | Document |
|-------|----------|
| Getting Started | [README.md](../README.md) |
| 5-min Setup | [QUICK_START.md](../QUICK_START.md) |
| Add New Source (YAML-only) | [SOURCE_ADAPTERS.md](SOURCE_ADAPTERS.md) ⭐ |
| Auto-Optimization | [OPTIMIZATION_USAGE.md](OPTIMIZATION_USAGE.md) ⭐ |
| Thresholds & Warnings | [SOURCE_ADAPTERS.md#thresholds](SOURCE_ADAPTERS.md) ⭐ |
| Helm Install | [README.md#installation](../README.md#installation) / [Helm Charts](https://github.com/kube-zen/helm-charts) |
| Filtering | [FILTERING.md](FILTERING.md) |
| Scaling | [SCALING.md](SCALING.md) |
| Security | [SECURITY.md](SECURITY.md) |
| Operations | [OPERATIONAL_EXCELLENCE.md](OPERATIONAL_EXCELLENCE.md) |
| Monitoring | [config/monitoring/README.md](../config/monitoring/README.md) |
| Dashboard | [config/dashboards/DASHBOARD_GUIDE.md](../config/dashboards/DASHBOARD_GUIDE.md) |
| Examples | [examples/README.md](../examples/README.md) |
| Contributing | [CONTRIBUTING.md](../CONTRIBUTING.md) |

---

## 📝 Document Statistics

- **Total Documentation Files**: 50+ markdown files
- **Total Lines**: 15,000+
- **Getting Started**: 2 guides
- **Operations**: 15+ guides (including filtering, scaling, optimization, source adapters)
- **Security**: 6 guides (including RBAC and threat model)
- **Architecture**: 4 guides
- **Integration**: 3 guides
- **Development**: 8 guides (including refactoring)
- **Monitoring**: 3 guides

---

## 🆕 Latest Updates

### Recent Additions (2025)

- ✅ **9 Sources Documentation** - Added cert-manager, sealed-secrets, and Kubernetes Events
- ✅ **YAML-Only Source Creation** - No code needed to add sources
- ✅ **Auto-Optimization Guide** - Complete optimization documentation
- ✅ **Thresholds & Warnings** - Comprehensive threshold configuration
- ✅ **ObservationSourceConfig CRD** - Complete CRD documentation
- ✅ **4 Input Methods** - Logs, webhooks, ConfigMaps, CRDs all documented

See [CHANGELOG.md](../CHANGELOG.md) for complete version history and updates.

---

## 📖 Reading Order

### New Users
1. [README.md](../README.md) - Overview
2. [QUICK_START.md](../QUICK_START.md) - 5-minute setup
3. [examples/README.md](../examples/README.md) - Examples
4. [OPERATIONAL_EXCELLENCE.md](OPERATIONAL_EXCELLENCE.md) - Best practices

### Adding a New Source
1. [README.md#adding-new-sources](../README.md) - Quick YAML example
2. [SOURCE_ADAPTERS.md](SOURCE_ADAPTERS.md) - Complete guide
3. [OPTIMIZATION_USAGE.md](OPTIMIZATION_USAGE.md) - Enable auto-optimization

### Operators
1. [README.md#installation](../README.md#installation) - Installation
2. [Helm Charts Repository](https://github.com/kube-zen/helm-charts) - Helm charts
3. [SECURITY.md](SECURITY.md) - Security setup
4. [OPERATIONAL_EXCELLENCE.md](OPERATIONAL_EXCELLENCE.md) - Operations
5. [config/monitoring/README.md](../config/monitoring/README.md) - Monitoring

### Developers
1. [DEVELOPER_GUIDE.md](DEVELOPER_GUIDE.md) - Developer guide
2. [CONTRIBUTING.md](../CONTRIBUTING.md) - Contribution guide
3. [ARCHITECTURE.md](ARCHITECTURE.md) - Architecture
4. [SOURCE_ADAPTERS.md](SOURCE_ADAPTERS.md) - Source adapters

---

## 💡 Tips

1. **Start with README.md** - Gives complete overview with all 9 sources
2. **Use QUICK_START.md** - For fast deployment in 5 minutes
3. **Bookmark this INDEX** - Quick access to all 50+ documentation files
4. **Check SOURCE_ADAPTERS.md** - For adding new sources with just YAML
5. **Read OPTIMIZATION_USAGE.md** - For auto-optimization setup
6. **Check examples/** - Learn from working configurations
7. **Read OPERATIONAL_EXCELLENCE** - Production best practices

---

**Need help? See [README.md#support](../README.md#support)**
