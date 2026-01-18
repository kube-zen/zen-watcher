# Zen Watcher Documentation

Welcome to the Zen Watcher documentation! This directory is organized by topic to help you find what you need quickly.

## 📚 Quick Navigation

### 🚀 Getting Started
**New to Zen Watcher? Start here!**

- **[QUICKSTART.md](getting-started/QUICKSTART.md)** - Quick start guide (5 minutes)
- **[GETTING_STARTED_GENERIC.md](getting-started/GETTING_STARTED_GENERIC.md)** - Complete installation guide
- **[DEPLOYMENT_HELM.md](getting-started/DEPLOYMENT_HELM.md)** - Helm deployment instructions
- **[DEPLOYMENT_SCENARIOS.md](getting-started/DEPLOYMENT_SCENARIOS.md)** - Different deployment patterns
- **[USE_CASES.md](getting-started/USE_CASES.md)** - Practical use cases and examples
- **[WHY_ZEN_WATCHER.md](getting-started/WHY_ZEN_WATCHER.md)** - Project rationale

### ⚙️ Operations
**Running and operating Zen Watcher**

- **[TROUBLESHOOTING.md](operations/TROUBLESHOOTING.md)** - Common issues and solutions
- **[OPERATIONAL_EXCELLENCE.md](operations/OPERATIONAL_EXCELLENCE.md)** - Best practices for operations
- **[SCALING.md](operations/SCALING.md)** - Scaling strategy and recommendations
- **[HIGH_AVAILABILITY_AND_SCALING.md](operations/HIGH_AVAILABILITY_AND_SCALING.md)** - HA deployment patterns
- **[PERFORMANCE.md](operations/PERFORMANCE.md)** - Performance optimization
- **[OBSERVABILITY.md](operations/OBSERVABILITY.md)** - Monitoring and observability setup
- **[LOGGING.md](operations/LOGGING.md)** - Logging configuration
- **[FILTERING.md](operations/FILTERING.md)** - Event filtering guide
- **[PROCESSING_PIPELINE.md](operations/PROCESSING_PIPELINE.md)** - Event processing pipeline

### 🔒 Security
**Security features and configuration**

- **[SECURITY.md](security/SECURITY.md)** - Security features and threat model
- **[SECURITY_RBAC.md](security/SECURITY_RBAC.md)** - RBAC permissions
- **[SECURITY_HARDENING_IMPLEMENTATION.md](security/SECURITY_HARDENING_IMPLEMENTATION.md)** - Security hardening guide
- **[METRICS_SECURITY_VIOLATIONS.md](security/METRICS_SECURITY_VIOLATIONS.md)** - Security violations tracking
- **[COSIGN.md](security/COSIGN.md)** - Image signing with Cosign
- **[SBOM.md](security/SBOM.md)** - Software Bill of Materials

### 🏗️ Reference
**API documentation and architecture**

- **[CRD.md](reference/CRD.md)** - Observation and Ingester API reference
- **[API_STABILITY_POLICY.md](reference/API_STABILITY_POLICY.md)** - API versioning policy
- **[STABILITY_GUARANTEES.md](reference/STABILITY_GUARANTEES.md)** - Stability guarantees
- **[VERSIONING.md](reference/VERSIONING.md)** - Versioning strategy
- **[CRD_CONTRACT.md](reference/CRD_CONTRACT.md)** - CRD ownership model
- **[INGESTER_API.md](reference/INGESTER_API.md)** - Ingester API documentation
- **[CONFIGURATION.md](reference/CONFIGURATION.md)** - Configuration reference
- **[ARCHITECTURE.md](reference/ARCHITECTURE.md)** - System architecture
- **[ARCHITECTURE_GVR_ALLOWLIST.md](reference/ARCHITECTURE_GVR_ALLOWLIST.md)** - GVR allowlist architecture
- **[LEADER_ELECTION.md](reference/LEADER_ELECTION.md)** - Leader election mechanism
- **[ORIGIN_STORY.md](reference/ORIGIN_STORY.md)** - Project origin story

### 🛠️ Development
**Contributing and developing**

- **[DEVELOPER_GUIDE.md](development/DEVELOPER_GUIDE.md)** - Developer guide
- **[BUILD.md](development/BUILD.md)** - Build instructions
- **[PROJECT_STRUCTURE.md](development/PROJECT_STRUCTURE.md)** - Code organization
- **[TOOLING_GUIDE.md](development/TOOLING_GUIDE.md)** - Development tools
- **[CONTRIBUTOR_TASKS.md](development/CONTRIBUTOR_TASKS.md)** - Contributor tasks
- **[CI_INTEGRATION.md](development/CI_INTEGRATION.md)** - CI/CD integration
- **[E2E_VALIDATION_GUIDE.md](development/E2E_VALIDATION_GUIDE.md)** - End-to-end testing
- **[ERROR_HANDLING_PATTERNS.md](development/ERROR_HANDLING_PATTERNS.md)** - Error handling patterns

### 🔌 Advanced Topics
**Advanced features and integrations**

- **[SOURCE_ADAPTERS.md](advanced/SOURCE_ADAPTERS.md)** - How to add new sources
- **[MANUAL_WEBHOOK_ADAPTER.md](advanced/MANUAL_WEBHOOK_ADAPTER.md)** - Webhook configuration
- **[INTEGRATIONS.md](advanced/INTEGRATIONS.md)** - Integration guide
- **[PLUGINS_AND_HOOKS.md](advanced/PLUGINS_AND_HOOKS.md)** - Plugin system
- **[DASHBOARD.md](advanced/DASHBOARD.md)** - Dashboard guide
- **[ALERT_RULES.md](advanced/ALERT_RULES.md)** - Alert rules configuration
- **[AUDIT_REPORT.md](advanced/AUDIT_REPORT.md)** - Audit report
- **[GO_SDK_OVERVIEW.md](advanced/GO_SDK_OVERVIEW.md)** - Go SDK usage
- **[IMAGE_AND_REGISTRY_GUIDE.md](advanced/IMAGE_AND_REGISTRY_GUIDE.md)** - Image management
- **[IMAGE_SIZE_OPTIMIZATION.md](advanced/IMAGE_SIZE_OPTIMIZATION.md)** - Image optimization
- **[OSS_LAUNCH_READINESS.md](advanced/OSS_LAUNCH_READINESS.md)** - OSS launch readiness
- **[RELIABILITY_PROFILES.md](advanced/RELIABILITY_PROFILES.md)** - Reliability profiles
- **[STRESS_TEST_RESULTS.md](advanced/STRESS_TEST_RESULTS.md)** - Stress test results
- **[THIRD_PARTY_LICENSES.md](advanced/THIRD_PARTY_LICENSES.md)** - Third-party licenses
- **[RELEASE.md](advanced/RELEASE.md)** - Release documentation
- **[RELEASE_NOTES_TEMPLATE.md](advanced/RELEASE_NOTES_TEMPLATE.md)** - Release notes template

### 📊 Alerting
**Alerting and incident response**

- **[alerting/SECURITY_ALERTING_OVERVIEW.md](alerting/SECURITY_ALERTING_OVERVIEW.md)** - Security alerting system
- **[alerting/ALERTING-INTEGRATION-GUIDE.md](alerting/ALERTING-INTEGRATION-GUIDE.md)** - Alert integration guide
- **[alerting/ALERT_TESTING.md](alerting/ALERT_TESTING.md)** - Alert testing
- **[alerting/SILENCE-MANAGEMENT.md](alerting/SILENCE-MANAGEMENT.md)** - Silence management
- **[alerting/INCIDENT_RESPONSE_SUMMARY.md](alerting/INCIDENT_RESPONSE_SUMMARY.md)** - Incident response

### 📋 Playbooks
**Operational playbooks**

- **[playbooks/PLAYBOOK_KUBEWATCH.md](playbooks/PLAYBOOK_KUBEWATCH.md)** - Kubewatch integration
- **[playbooks/PLAYBOOK_ROBUSTA.md](playbooks/PLAYBOOK_ROBUSTA.md)** - Robusta integration
- **[playbooks/PLAYBOOK_SIEM_EXPORT.md](playbooks/PLAYBOOK_SIEM_EXPORT.md)** - SIEM export
- **[playbooks/PLAYBOOK_PROM_ALERTS.md](playbooks/PLAYBOOK_PROM_ALERTS.md)** - Prometheus alerts

---

## 🗺️ Documentation Index

For a complete index of all documentation, see **[INDEX.md](INDEX.md)**.

---

## 💡 Quick Links by Role

### New Users
1. [README.md](../README.md) - Start here for overview
2. [getting-started/QUICKSTART.md](getting-started/QUICKSTART.md) - Get running in 5 minutes
3. [getting-started/USE_CASES.md](getting-started/USE_CASES.md) - See examples

### Operators
1. [getting-started/DEPLOYMENT_HELM.md](getting-started/DEPLOYMENT_HELM.md) - Installation
2. [operations/OPERATIONAL_EXCELLENCE.md](operations/OPERATIONAL_EXCELLENCE.md) - Best practices
3. [operations/TROUBLESHOOTING.md](operations/TROUBLESHOOTING.md) - Troubleshooting

### Developers
1. [development/DEVELOPER_GUIDE.md](development/DEVELOPER_GUIDE.md) - Developer guide
2. [reference/ARCHITECTURE.md](reference/ARCHITECTURE.md) - Architecture
3. [advanced/SOURCE_ADAPTERS.md](advanced/SOURCE_ADAPTERS.md) - Adding sources

---

**Need help?** See the main [README.md](../README.md) for support information.
