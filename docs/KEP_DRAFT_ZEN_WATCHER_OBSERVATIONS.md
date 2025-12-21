# KEP Draft: Kubernetes Observation Collection Standard

**Status**: Pre-draft (Internal)  
**Last Updated**: 2025-12-10  
**Target SIG**: sig-observability (tentative)

> **Note**: This is an internal pre-draft document for zen-watcher. It is not an official Kubernetes Enhancement Proposal (KEP) and has not been submitted to any Kubernetes SIG. This document serves as a design target and preparation for potential future KEP submission.

---

## Summary

This proposal introduces a Kubernetes-native standard for collecting, normalizing, and aggregating observations from security, compliance, and infrastructure tools. The standard defines a Custom Resource Definition (CRD) model and processing pipeline that enables operators to unify heterogeneous event sources into a single, queryable, Kubernetes-native format.

**Key Value Proposition**:
- **Zero Blast Radius Security**: Core component holds zero secrets, zero egress traffic, zero external dependencies
- **Kubernetes-Native**: All data stored as Observation CRDs in etcd, no external database
- **Extensible**: Config-driven source integration via CRDs, no code changes required for new sources
- **Production-Ready**: Validated performance characteristics, comprehensive observability, enterprise-grade reliability

---

## Motivation

### Problem Statement

Kubernetes operators face a fundamental challenge: **security, compliance, and infrastructure tools generate events in incompatible formats**, making it difficult to:
- Correlate events across tools (e.g., a CVE detected by Trivy and a runtime threat detected by Falco)
- Apply consistent filtering, deduplication, and retention policies
- Build unified dashboards and alerting
- Maintain audit trails in a Kubernetes-native format
- Enable RBAC-based access control for security events

Current approaches require:
- Custom integration code for each tool
- External databases or message queues (increasing attack surface)
- Manual correlation and deduplication
- Complex RBAC policies across multiple systems

### Use Cases

1. **Security Operations**: Aggregate vulnerability scans (Trivy), runtime threats (Falco), and policy violations (Kyverno) into unified Observation CRDs for correlation and alerting
2. **Compliance Auditing**: Collect compliance check results (kube-bench, Checkov) as Observations for audit trails and reporting
3. **Infrastructure Monitoring**: Normalize certificate expiration warnings, pod crash loops, and resource quota violations into a consistent format
4. **Multi-Tenant Security**: Enable namespace-scoped RBAC policies for security teams to view only relevant Observations

---

## Goals

### Primary Goals

1. **Standardize Observation Model**: Define a Kubernetes CRD that can represent events from any security/compliance/infrastructure tool
2. **Enable Config-Driven Integration**: Allow new sources to be added via CRD configuration without code changes
3. **Provide Production-Grade Pipeline**: Filtering, deduplication, normalization, and TTL management
4. **Maintain Zero Blast Radius**: Core component never holds secrets or makes outbound connections
5. **Support Extensible Ecosystem**: Enable downstream consumers (alerting, SIEMs, mesh systems) via standard CRD interface

### Non-Goals

1. **Not a SIEM**: This proposal does not replace SIEM systems; it provides a Kubernetes-native aggregation layer
2. **Not a Remediation System**: Observations are read-only; actions are handled by separate controllers
3. **Not a SaaS Service**: Core component is fully in-cluster; SaaS integration is optional and handled by separate sync controllers
4. **Not a Replacement for Tool-Specific APIs**: Tools continue to expose their native APIs; this standard provides normalization

---

## Proposal

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────┐
│  Security/Compliance/Infrastructure Tools               │
│  (Trivy, Falco, Kyverno, cert-manager, etc.)            │
└──────────────┬──────────────────────────────────────────┘
               │
               │ Events (CRDs, Webhooks, ConfigMaps)
               │
┌──────────────▼──────────────────────────────────────────┐
│  zen-watcher (Core Aggregation Pipeline)                │
│  - Informer-based adapters (CRD sources)                 │
│  - Webhook adapters (push-based sources)                 │
│  - ConfigMap adapters (batch sources)                   │
│  - Filter → Normalize → Deduplicate → Create CRD        │
└──────────────┬──────────────────────────────────────────┘
               │
               │ Observation CRDs (zen.kube-zen.io/v1)
               │
┌──────────────▼──────────────────────────────────────────┐
│  etcd (Kubernetes Native Storage)                       │
└──────────────┬──────────────────────────────────────────┘
               │
       ┌───────┴────────┐
       │                │
┌──────▼──────┐  ┌──────▼──────┐
│ Sync        │  │ Action      │
│ Controllers │  │ Controllers  │
│ (zen-agent) │  │ (zen-agent)  │
└─────────────┘  └──────────────┘
```

### Core API: Observation CRD

**Group**: `zen.kube-zen.io`  
**Version**: `v1` (current), `v2` (future, see Design Details)  
**Kind**: `Observation`  
**Scope**: `Namespaced`

#### Required Fields

```yaml
spec:
  source: string          # Tool identifier (trivy, falco, kyverno, etc.)
  category: string        # Event category (security, compliance, performance)
  severity: string        # Severity level (critical, high, medium, low, info)
  eventType: string       # Type of event (vulnerability, runtime-threat, policy-violation)
```

#### Optional Fields

```yaml
spec:
  resource:               # Affected Kubernetes resource
    apiVersion: string
    kind: string
    name: string
    namespace: string     # Preserved for RBAC and multi-tenancy
  details: object         # Event-specific details (flexible JSON, preserved)
  detectedAt: string      # RFC3339 timestamp
  ttlSecondsAfterCreation: int64  # TTL in seconds (Kubernetes-native style)

status:
  processed: bool         # Whether processed by downstream consumers
  lastProcessedAt: string # RFC3339 timestamp
```

**See**: 
- `deployments/crds/observation_crd.yaml` for complete schema
- `docs/OBSERVATION_API_PUBLIC_GUIDE.md` for external-facing API contract
- `examples/observations/` for canonical examples

### Processing Pipeline

1. **Source Detection**: Informer/webhook/ConfigMap adapters detect events
2. **Filtering**: Per-source filter rules (via `Ingester` CRD)
3. **Normalization**: Convert tool-specific formats to Observation spec
4. **Deduplication**: Content-based fingerprinting + time-windowed deduplication
5. **CRD Creation**: Write Observation CRD to etcd
6. **TTL Management**: Automatic garbage collection after TTL expires

**See**: `docs/ARCHITECTURE.md` and `docs/DEDUPLICATION.md` for detailed pipeline documentation

---

## Design Details

### CRD Schema Evolution

**Current State (v1)**:
- ✅ Stable, production-ready
- ✅ Required fields: `source`, `category`, `severity`, `eventType`
- ✅ Optional fields: `resource`, `details`, `detectedAt`, `ttlSecondsAfterCreation`
- ✅ Status: `processed`, `lastProcessedAt`
- ✅ **Validation Hardening (v1alpha2)**: Enum validation for `severity` and `category`, maximum TTL validation, pattern validation improvements
  - **Status**: Implemented (non-breaking, backward-compatible)
  - **Reference**: `docs/OBSERVATION_VERSIONING_AND_RELEASE_PLAN.md`
- ⚠️ **Branded API Surface**: CRD group `zen.kube-zen.io` contains hard-coded branding
  - **Status**: Managed tech debt (see ``)
  - **Migration Plan**: Planned for v2 (neutral group migration, e.g., `observations.kubernetes.io`)

**Future State (v1beta1)** - **Planned**:
- Conditions pattern for status (replacing `processed` boolean)
- `observedGeneration` field for reconciliation tracking
- `phase` field for lifecycle tracking
- **Reference**: `docs/OBSERVATION_VERSIONING_AND_RELEASE_PLAN.md` (v1beta1 section)

**Future State (v2)** - **Not Yet Implemented**:
- Enhanced `priority` field (0.0-1.0 numeric score) for more granular severity
- `resources` array (multiple affected resources per observation)
- `fingerprint` field (content-based hash for deduplication)
- `adapter` field (which adapter processed this observation)
- `expiresAt` field (explicit expiration timestamp)
- **CRD Group Migration**: `zen.kube-zen.io` → neutral group (e.g., `observations.kubernetes.io` or `observations.watcher.io`)
  - **Rationale**: Remove hard-coded branding from API surface for vendor-neutrality (see ``)
  - **Migration**: New CRD group with conversion webhook or migration tooling
  - **Deprecation**: `zen.kube-zen.io` served alongside new group for 2+ release cycles
  - **Mapping**: `zen.kube-zen.io/v1` → `observations.kubernetes.io/v2` (or similar)
- **Label Prefix Migration**: `zen.io/*` → neutral prefix (e.g., `observations.io/*` or `watcher.io/*`)
  - **Rationale**: Remove branding from label conventions
  - **Migration**: Support both prefixes during transition

**Migration Strategy**: v1 and v1beta1 will be served simultaneously; v1 remains storage version for backward compatibility. v2 migration will include CRD group migration to achieve vendor-neutrality. See `docs/OBSERVATION_VERSIONING_AND_RELEASE_PLAN.md` for detailed migration plans.

**Versioning Plan**: See `docs/OBSERVATION_VERSIONING_AND_RELEASE_PLAN.md` for detailed version progression (v1alpha2 → v1beta1 → v2) and compatibility policy.

**Branding & Vendor Neutrality**: See `` for complete audit of branded elements and migration plans. Hard-coded branding in CRD group (`zen.kube-zen.io`) is treated as managed tech debt with explicit migration path in v2.

**See**: `docs/OBSERVATION_CRD_API_AUDIT.md` for detailed API analysis and future improvements

### Source Integration

**Config-Driven via CRDs**:
- `Ingester`: Defines source adapters (informer, webhook, logs, k8s-events) and processing configuration

**No Code Changes Required**: New sources can be added by creating CRDs

**See**: `docs/SOURCE_ADAPTERS.md` for extensibility guide

### Performance Characteristics

**Validated Metrics** (from `docs/STRESS_TEST_RESULTS.md`):
- **Sustained Throughput**: 16-22 observations/sec
- **Burst Capacity**: 15-18 observations/sec
- **Resource Usage**: CPU +30-55m, Memory +35-60MB under load
- **etcd Impact**: ~2.2KB per observation
- **Recovery Time**: <60 seconds after burst

**Target Metrics** (aspirational, not yet validated):
- **Sustained Throughput**: 45-50 observations/sec (requires optimization)
- **Burst Capacity**: 500 observations/30sec (validated)
- **20k Object Impact**: +5m CPU, +10MB memory (validated)

**See**: `docs/STRESS_TEST_RESULTS.md` for complete performance documentation

### Observability

**Prometheus Metrics**:
- `zen_watcher_observations_created_total{source, category, severity}`
- `zen_watcher_observations_filtered_total{source, reason}`
- `zen_watcher_observations_deduped_total`
- `zen_watcher_events_total{source, category, severity, eventType}`
- `zen_watcher_observations_live` (gauge)
- `zen_watcher_event_processing_duration_seconds` (histogram)

**Dashboards**: 6 pre-built Grafana dashboards (Executive, Operations, Security, Main, Namespace Health, Explorer)

**See**: `config/dashboards/README.md` and `pkg/metrics/definitions.go` for complete metrics documentation

### Security Model

**Zero Blast Radius Architecture**:
- Core component holds **zero secrets**
- Core component makes **zero outbound connections**
- Core component has **zero external dependencies**
- All sensitive operations (SIEM sync, alerting) handled by separate controllers

**RBAC Support**:
- Namespace-scoped Observations enable granular RBAC policies
- Example: `security-team` can view Observations in `prod-*` namespaces only

**See**: `docs/SECURITY_MODEL.md` and `docs/ARCHITECTURE.md` for complete security documentation

### Extensibility

**Vendor Neutrality**: zen-watcher is designed to work with any webhook gateway or integration. Components like zen-hook, zen-agent, and zen-alpha (in the kube-zen ecosystem) are example producers/consumers, not required dependencies.

---

## Graduation Criteria / Compatibility

### API Stability

**v1 API** (Current):
- ✅ **Stable**: No breaking changes planned
- ✅ **Backward Compatible**: All v1 fields preserved
- ✅ **Production Ready**: Used in production deployments

**v2 API** (Future):
- ⚠️ **Alpha**: Not yet implemented
- ⚠️ **Breaking Changes**: Will require migration path
- ⚠️ **Timeline**: TBD based on community feedback

### Compatibility Guarantees

1. **Backward Compatibility**: v1 API will remain supported indefinitely
2. **Forward Compatibility**: v1 clients can read v2 Observations (with field mapping)
3. **Deprecation Policy**: Minimum 2 release cycles notice before removing fields
4. **Migration Tools**: Automated migration scripts for v1 → v2 (when v2 is implemented)

### KEP Readiness Checklist

- [x] **Problem Statement**: Clear and well-defined
- [x] **Use Cases**: Documented with real-world examples
- [x] **API Design**: CRD schema defined and validated
- [x] **Performance**: Baseline metrics documented
- [x] **Security**: Zero blast radius model documented
- [x] **Observability**: Metrics and dashboards validated
- [ ] **Community Adoption**: Not yet (pre-draft stage)
- [ ] **SIG Review**: Not yet submitted
- [ ] **Implementation History**: See below

---

## Implementation History / References

### Current Implementation Status

**✅ Implemented and Stable**:
- Observation CRD (v1) with full schema validation
- Source adapters (informer, webhook, logs, configmap)
- Filtering, normalization, deduplication pipeline
- TTL management and garbage collection
- Prometheus metrics and Grafana dashboards
- Stress testing scripts and performance baselines
- Informer convergence (Phases 1-2 complete)

**⚠️ Partially Implemented**:
- v2 CRD schema (defined but not yet implemented)
- Advanced optimization features (deferred)

**📋 Future Work**:
- v1 → v2 migration path
- Cross-repo informer convergence (Phase 3)
- KEP submission to sig-observability

### Related Documentation

**Current (Canonical)**:
- `the project roadmap` - Roadmap and priorities
- `docs/ARCHITECTURE.md` - Complete architecture documentation
- `docs/STRESS_TEST_RESULTS.md` - Performance baselines
- `docs/INFORMERS_CONVERGENCE_NOTES.md` - Informer architecture evolution
- `docs/OBSERVATION_CRD_API_AUDIT.md` - API analysis and future improvements
- `CONTRIBUTING.md` - Quality bar and standards

**Reference**:
- ` - Expert analysis (late 2024/early 2025)
- ` - Stress testing analysis

### Code References

- **CRD Definition**: `deployments/crds/observation_crd.yaml`
- **Processing Pipeline**: `pkg/watcher/observation_creator.go`
- **Metrics**: `pkg/metrics/definitions.go`
- **Source Adapters**: `pkg/adapter/generic/`
- **Informer Abstraction**: `internal/informers/manager.go`

---

## Open Questions

1. **SIG Assignment**: Should this target sig-observability, sig-security, or a new SIG?
2. **API Versioning**: Is v1 → v2 migration strategy sufficient, or should we consider v1beta1 first?
3. **Community Adoption**: How do we measure community interest before formal KEP submission?
4. **Tool Integration**: Should we provide a "certified sources" program for tool vendors?

---

## Next Steps

1. **Community Feedback**: Gather input from Kubernetes SIGs and tool vendors
2. **API Hardening**: Complete v2 schema design and migration path
3. **Performance Validation**: Execute full stress tests in dedicated environment
4. **Documentation Polish**: Prepare community-facing documentation
5. **KEP Submission**: Submit to appropriate SIG when ready

---

**This is a pre-draft document. For current implementation status, see `the project roadmap`.**
