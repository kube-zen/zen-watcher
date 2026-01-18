# Changelog

All notable changes to zen-watcher will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.2.1] - 2025-01-25

### Fixed

- **Helm Chart**: Fixed NetworkPolicy defaults requiring explicit destinations when egress enabled
- **Helm Chart**: Exposed critical hardening controls (extraEnv, webhook auth, retention/GC knobs)
- **Helm Chart**: CRDs now properly shipped and installable via Helm (crds.enabled: true by default)
- **Documentation**: Updated all version references from 1.0.0-alpha/1.2.0 to 1.2.1
- **CI Script**: Fixed version fallback to read from VERSION file instead of hardcoded 1.0.19

### Changed

- **Helm Chart**: NetworkPolicy now requires explicit `kubernetesServiceIP` and/or `kubernetesAPICIDRs[]` when `egress.enabled=true` and `allowKubernetesAPI=true`
- **Helm Chart**: Default remains `egress.enabled=false` for safer community posture

## [1.2.0] - 2025-01-05

### 🎉 Production-Ready Release

**First production-ready release** with synchronized versioning, enhanced security defaults, comprehensive observability, and consolidated documentation.

#### Added

- **Secure by Default**: Webhook authentication now required by default
- **Enhanced Observability**: Leader election metrics and PrometheusRule alerts
- **Helm Chart Improvements**: TTL/GC tuning, PrometheusRule installation
- **Documentation Consolidation**: Single source of truth for HA and scaling
- **Version Synchronization**: All components aligned to 1.2.0

See [RELEASE_NOTES_v1.2.0.md](RELEASE_NOTES_v1.2.0.md) for complete details.

## [1.0.0-alpha] - 2025-12-11

### 🎉 Initial Alpha Release

**First public release** of Zen Watcher as an open-source Kubernetes security event aggregator.

#### Added

**Core Features:**
- **Ingester v1**: Complete Ingester CRD with informer/webhook/logs support
- **Canonical Pipeline**: Enforced pipeline order `source → (filter | dedup) → normalize → destinations[]`
- **Configurable Processing Order**: Manual selection of filter_first or dedup_first strategies per source
- **Filter Expressions (v1.1)**: Expression-based filtering with AND/OR/NOT, comparisons, macros
- **Plugin Hooks**: Compile-time hooks for extending pipeline (post-normalization, pre-CRD write)
- Multi-source event aggregation (Trivy, Falco, Kyverno, Checkov, Kube-bench, Audit logs)
- Kubernetes-native CRD storage (Observation CRDs)
- Intelligent noise reduction (SHA-256 fingerprinting, rate limiting, deduplication)
- Source-level filtering (ConfigMap, CRD-based, and expression-based)
- Comprehensive observability (Prometheus metrics, Grafana dashboards, structured logging)

**Tooling Suite:**
- **ingester-migrate**: CLI tool for migrating v1alpha1 → v1 Ingester specs
- **ingester-lint**: Safety validator for Ingester specs (dangerous settings, misconfigurations)
- **obsctl**: CLI for querying Observations (list, stats, get commands)
- **schema-doc-gen**: Automated schema documentation generator from CRDs

**Architecture:**
- Pure core, extensible ecosystem model (zero secrets, zero egress)
- Modular adapter architecture (SourceAdapter interface)
- Kubernetes-native event consumption via CRDs
- Infrastructure-blind design (preserves namespace/name/kind for RBAC)

**Deployment:**
- **Helm Chart**: Production-ready Helm chart with dev/staging/prod profiles
- **Container Images**: Minimal distroless-based images
- **E2E Validation**: Cluster-agnostic validation harness

**Documentation:**
- Complete API documentation (Ingester API, Observation API)
- Quick start guide with automated demo
- Integration guides (kubewatch, Robusta, custom controllers)
- Ecosystem playbooks (Kubewatch, Robusta, Prometheus, SIEM)
- Multi-team RBAC patterns
- Troubleshooting guide
- Performance and scaling guides

**CI/CD:**
- Automated testing and build pipeline
- Docker image builds and publishing
- Security scanning and SBOM generation
- Fuzz testing for pipeline robustness

#### Known Limitations / Not in This Release

- **Dedup Strategy v1.1**: Pluggable dedup strategies (deferred to post-alpha)
- **Observations Taxonomy**: Standardized field mappings for common tools (deferred to post-alpha)
- **Advanced Tooling Polish**: Unified CLI suite (deferred to post-alpha)

---

## [1.1.0] - 2024-12-04 (Deprecated - Pre-alpha)

### 🎉 OSS Launch Release

**Polished Launch Release** with complete documentation, CI automation, and production-ready features.

#### Added

**CI/CD & Automation:**
- CI scripts for test, build, and release (`scripts/ci-*.sh`)
- Automated testing with coverage reporting
- Build pipeline with Docker Hub publishing

**Documentation:**
- `docs/STABILITY.md` - Production readiness and HA patterns
- `docs/VERSIONING.md` - Version sync strategy
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
- Synced version (1.0.0-alpha) with image

#### Changed
- **Version Sync:** Image and chart now use same version (1.0.0-alpha)
- KEP status: draft → implementable
- README: Clearer positioning and value proposition
- SECURITY_RBAC.md: Added Ingester permissions

#### Fixed
- Chart.yaml: Use `kubezen` (was `zubezen` in some places)
- build-and-sign.sh: Correct image name

---

## [1.0.10] - 2024-12-04

### 🎉 Major Features

#### Modular Adapter Architecture
- **Added** SourceAdapter interface for all 6 event sources
- **Implemented** adapter factory pattern for lifecycle management
- Implemented ingester-based architecture for all sources (Trivy, Kyverno, Falco, Audit, Checkov, KubeBench)

#### Dynamic Filtering with CRDs
- **Added** Ingester CRD for Kubernetes-native filtering
- **Implemented** filter merge semantics (ConfigMap + Ingester CRD)
- **Added** comprehensive filter merge tests with 20+ test cases
- **Added** last-good-config fallback for filter errors

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
- **Fixed** metrics exposure for all 9 sources (previously only informer-based)

#### Testing
- **Added** filter merger unit tests (pkg/filter/merger_test.go)
- **Added** Ingester loader tests (pkg/config/ingester_loader_test.go)
- **Improved** test coverage for filter logic

#### Deployment & Demo
- **Automated** quick-demo.sh for 6/6 source validation
- **Added** mock data system via Helm chart templates
- **Reduced** deployment time to ~4-5 minutes
- **Added** non-interactive mode for CI/automation

#### Documentation
- **Created** docs/STABILITY.md - production readiness guide
- **Updated** KEP to "implementable" status with implementation history
- **Enhanced** docs/security/SECURITY_RBAC.md with Ingester CRD permissions
- **Updated** README.md with v1.0.10 features

### 🐛 Bug Fixes
- **Fixed** Ingester CRD validation error (removed conflicting additionalProperties)
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
- **Added** ingester_crd.yaml template
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
