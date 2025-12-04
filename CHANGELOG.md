# Changelog

All notable changes to zen-watcher will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.1.0] - 2024-12-04

### 🎉 OSS Launch Release

**Polished Launch Release** with complete documentation, CI automation, and production-ready features.

#### Added

**CI/CD & Automation:**
- CI scripts for test, build, and release (`scripts/ci-*.sh`)
- Automated testing with coverage reporting
- Build pipeline with Docker Hub publishing

**Documentation:**
- `docs/STABILITY.md` - Production readiness and HA patterns
- `VERSIONING.md` - Version sync strategy
- `OSS_LAUNCH_CHECKLIST.md` - Launch readiness guide
- Comparison table in README (vs Falco Sidekick, Kubescape)
- Polished Quick Start section with copy-paste commands
- Use case examples (`examples/use-cases/`)
  - Multi-tenant filtering
  - Custom CRD integration
  - Compliance reporting

**Tests:**
- Webhook adapter tests (Falco, Audit)
- Enhanced filter merge tests

**Helm Chart:**
- ArtifactHub metadata for publishing
- Enhanced Chart.yaml annotations
- Synced version (1.1.0) with image

#### Changed
- **Version Sync:** Image and chart now use same version (1.1.0)
- KEP status: draft → implementable
- README: Clearer positioning and value proposition
- SECURITY_RBAC.md: Added ObservationFilter and ObservationMapping permissions

#### Fixed
- Chart.yaml: Use `kubezen` (was `zubezen` in some places)
- build-and-sign.sh: Correct image name

---

## [1.0.10] - 2024-12-04

### 🎉 Major Features

#### Modular Adapter Architecture
- **Added** SourceAdapter interface for all 6 event sources
- **Implemented** adapter factory pattern for lifecycle management
- **Migrated** all sources to new adapter architecture (Trivy, Kyverno, Falco, Audit, Checkov, KubeBench)

#### Dynamic Filtering with CRDs
- **Added** ObservationFilter CRD for Kubernetes-native filtering
- **Implemented** filter merge semantics (ConfigMap + ObservationFilter CRD)
- **Added** comprehensive filter merge tests with 20+ test cases
- **Added** last-good-config fallback for filter errors

#### Generic CRD Integration
- **Added** ObservationMapping CRD for custom CRD integration
- **Implemented** CRDSourceAdapter for "long tail" tool support
- **Added** JSONPath-based field mapping from source CRDs to Observations
- **Enabled** zero-code integration of new security tools

#### Cluster-Blind Design
- **Removed** all CLUSTER_ID and TENANT_ID metadata references
- **Simplified** architecture to pure security event aggregation
- **Decoupled** from infrastructure concerns

### ✅ Improvements

#### Observability
- **Enhanced** metrics definitions for filter, adapter, mapping, dedup, GC
- **Enabled** VictoriaMetrics scraping by default (vmServiceScrape)
- **Added** Prometheus annotations to service for automatic discovery
- **Fixed** metrics exposure for all 6 sources (previously only informer-based)

#### Testing
- **Added** filter merger unit tests (pkg/filter/merger_test.go)
- **Added** ObservationFilter loader tests (pkg/config/observationfilter_loader_test.go)
- **Improved** test coverage for filter logic

#### Deployment & Demo
- **Automated** quick-demo.sh for 6/6 source validation
- **Added** mock data system via Helm chart templates
- **Reduced** deployment time to ~4-5 minutes
- **Added** non-interactive mode for CI/automation

#### Documentation
- **Created** docs/STABILITY.md - production readiness guide
- **Updated** KEP to "implementable" status with implementation history
- **Enhanced** docs/SECURITY_RBAC.md with new CRD permissions
- **Updated** README.md with v1.0.10 features
- **Added** DEPLOYMENT_SUCCESS.md

### 🐛 Bug Fixes
- **Fixed** ObservationFilter CRD validation error (removed conflicting additionalProperties)
- **Fixed** RBAC permissions for aquasecurity.github.io resources
- **Fixed** ConfigMap adapter metrics not incrementing
- **Fixed** Webhook adapter metrics not incrementing
- **Removed** unused imports (kube_bench_watcher.go)

### 🔧 Technical Changes
- **Centralized** event processing through AdapterLauncher
- **Standardized** Event struct across all adapters
- **Improved** deduplication with content-based fingerprinting
- **Enhanced** error handling and logging throughout

### 📦 Helm Chart (v1.0.10)
- **Added** observationfilter_crd.yaml template
- **Added** observationmapping_crd.yaml template
- **Added** mock-data-job.yaml for automated demos
- **Added** mock-kyverno-policy.yaml for non-blocking policy generation
- **Updated** service.yaml with Prometheus scrape annotations
- **Updated** RBAC with all required permissions
- **Set** mockData.enabled=true by default for demos
- **Set** vmServiceScrape.enabled=true by default
- **Updated** image tag to 1.0.19

### 🔒 Security
- **Verified** all RBAC permissions follow least privilege
- **Documented** security rationale for all permissions
- **Maintained** non-root, read-only filesystem, dropped capabilities
- **Added** webhook authentication documentation

## [1.0.0] - 2024-11-27

### Initial Release

- **Observation CRD** - Unified event format for security/compliance
- **6 Event Sources** - Trivy, Kyverno, Falco, Audit, Checkov, KubeBench
- **Filtering** - ConfigMap-based per-source filtering
- **Deduplication** - Sliding window with LRU eviction
- **Metrics** - Prometheus metrics (events_total, created, filtered, deduped)
- **Garbage Collection** - TTL-based cleanup (7-day default)
- **Security Hardening** - Non-root, read-only FS, NetworkPolicy
- **Grafana Dashboard** - Pre-built visualization
- **VictoriaMetrics Integration** - Metrics storage
- **Quick Demo** - Automated local deployment

---

## Version Numbering

- **Image versions** (e.g., 1.0.19): Application code releases
- **Chart versions** (e.g., 1.0.10): Helm chart releases
- Both follow semantic versioning independently
- Major releases sync both versions

## Links

- [GitHub Repository](https://github.com/kube-zen/zen-watcher)
- [Helm Charts](https://github.com/kube-zen/helm-charts)
- [Docker Hub](https://hub.docker.com/r/kubezen/zen-watcher)
- [Documentation](https://github.com/kube-zen/zen-watcher/tree/main/docs)
