# PM AI Roadmap for Zen Watcher

**Purpose**: Canonical roadmap for PM AIs working on zen-watcher. This document provides vision, current state, and prioritized backlog for future work.

**Last Updated**: 2025-12-10

**How to Use This as PM AI**:
1. Read `docs/PM_HANDBOOK.md` in `zen-alpha` for general PM AI coordination rules
2. Read this roadmap to understand zen-watcher's vision and priorities
3. Read `docs/INFORMERS_CONVERGENCE_NOTES.md` for informer architecture evolution
4. Prioritize workstreams based on: community value, quality bar (KEP-level), and trunk hygiene
5. Keep commits small, scoped, and pushed to `main` frequently

---

## Vision & Role of Zen Watcher

**Zen Watcher is a Kubernetes-native observation pipeline designed as a future KEP (Kubernetes Enhancement Proposal) candidate.**

### Core Value Proposition

- **Zero Blast Radius Security**: Core component holds zero secrets, zero egress traffic, zero external dependencies
- **Kubernetes-Native**: All data stored as Observation CRDs in etcd
- **Extensible Ecosystem**: Pure core + optional sync controllers (secrets live in isolated components)
- **Community-Grade Quality**: Designed for long-term stability, backward compatibility, and multi-vendor ecosystem use

### Target Users

- **Kubernetes operators** who need unified observation aggregation
- **Security teams** requiring zero-trust compliant observation collection
- **Platform engineers** building extensible security/compliance pipelines
- **CNCF ecosystem** as a potential standard for Kubernetes observation collection

---

## Current State Snapshot

### Quick Demo Status: ✅ Production-Ready

- **Script**: `./scripts/quick-demo.sh` supports k3d, kind, minikube
- **Features**: Non-interactive mode, mock data deployment, minimal resource profile
- **Output**: Grafana dashboards, credentials, validation checklist
- **Status**: Fully functional, documented in README.md

### Stress/Benchmark Scripts Status: ✅ Implemented, Execution Parked

- **Scripts**: `scripts/benchmark/quick-bench.sh`, `scripts/benchmark/stress-test.sh`
- **Status**: Scripts are correct and ready, but full runs are parked until dedicated environment
- **Documentation**: `docs/STRESS_TEST_RESULTS.md` explains environment requirements
- **Local Constraints**: Heavy runs not recommended on laptops (resource contention)
- **Next Step**: Execute in cloud cluster (2-3 nodes, 4+ vCPUs, 8GB+ RAM) when available

### Observability/Dashboards Status: ✅ Complete

- **Dashboards**: 6 pre-built dashboards (Executive, Operations, Security, Main, Namespace Health, Explorer)
- **Metrics**: All dashboards reference actual metrics from code (`pkg/metrics/definitions.go`)
- **Documentation**: `config/dashboards/README.md`, `METRIC_USAGE_GUIDE.md`, `DASHBOARD_GUIDE.md`
- **Validation**: Dashboard import script works, metrics verified against code
- **Status**: Production-ready, validated end-to-end

### Informer Convergence Status: ✅ Phases 1-2 Complete

- **Phase 1**: Internal informer abstraction (`internal/informers/manager.go`) - ✅ Complete
- **Phase 2**: Workqueue backpressure in `InformerAdapter` - ✅ Complete
- **Client Throttling**: QPS=5, Burst=10 (aligned with zen-agent)
- **Status**: Backward compatible, RawEvent semantics preserved
- **Documentation**: `docs/INFORMERS_CONVERGENCE_NOTES.md`
- **Next**: Phase 3 (cross-repo convergence) - design only, future work

---

## Near-Term Backlog (0-3 Months)

### High-Priority Tasks

1. **Execute Stress Tests in Cloud Cluster**
   - **Goal**: Replace "expected ranges" in `docs/STRESS_TEST_RESULTS.md` with actual measured values
   - **Requirements**: Dedicated cluster (2-3 nodes, 4+ vCPUs, 8GB+ RAM)
   - **Output**: Concrete throughput, latency, CPU/memory baselines
   - **Dependencies**: Cloud cluster access

2. **Polish Quick Demo UX**
   - **Goal**: Reduce friction for first-time users
   - **Tasks**: Better error messages, progress indicators, validation feedback
   - **Metrics**: Time-to-first-observation, success rate

3. **Tighten Configuration APIs**
   - **Goal**: Improve CRD validation and error messages
   - **Focus**: `ObservationSourceConfig`, `ObservationTypeConfig` validation
   - **Output**: Clear validation errors, schema documentation

4. **Add More Example Sources**
   - **Goal**: Demonstrate extensibility with real-world tools
   - **Candidates**: Wiz, Snyk, Aqua, Prisma Cloud
   - **Format**: Example CRDs in `examples/` directory

5. **API Surface Hardening**
   - **Goal**: Prepare for KEP submission
   - **Tasks**: Version CRDs (v1alpha1 → v1beta1), deprecation policies, compatibility guarantees
   - **Documentation**: API stability contract

### Medium-Priority Tasks

6. **Performance Optimization**
   - **Goal**: Reduce resource usage, improve throughput
   - **Focus**: Informer resync tuning, queue sizing, batch processing
   - **Metrics**: CPU/memory per 1000 observations/hour

7. **Enhanced Filtering**
   - **Goal**: More expressive filter rules
   - **Tasks**: Regex support, complex boolean logic, time-based rules
   - **Use Case**: Compliance with retention policies

8. **Multi-Namespace Support**
   - **Goal**: Efficient cross-namespace observation collection
   - **Tasks**: Namespace selector, RBAC patterns, performance testing

9. **Documentation Polish**
   - **Goal**: Community-ready documentation
   - **Tasks**: Architecture diagrams, troubleshooting guides, FAQ
   - **Format**: Markdown, diagrams (Mermaid), examples

10. **Test Coverage Expansion**
    - **Goal**: Increase unit test coverage to 80%+
    - **Focus**: Adapter tests, processor tests, edge cases
    - **Tooling**: Coverage reports, CI integration

---

## Mid-Term Backlog (3-12 Months)

### Dynamic Webhook Integration

**Goal**: Prepare for zen-hook (dynamic webhook gateway) integration

- **Observation Export**: Design API for zen-hook to consume Observations
- **Webhook Registration**: CRD-based webhook endpoint registration
- **Multi-Tenancy**: Support multiple webhook consumers with isolation
- **Documentation**: Integration guide for zen-hook developers

**Related**: See `zen-alpha/docs/OBSERVATIONS_PHASE1_BACKLOG.md` for SaaS-side ingestion API work

### KEP-Prep Work

**Goal**: Prepare zen-watcher for Kubernetes Enhancement Proposal submission

- **Problem Statement**: Document the problem zen-watcher solves
- **API Surface**: Finalize CRD schemas, versioning strategy
- **Compatibility Contracts**: Backward compatibility guarantees, deprecation policies
- **Reference Implementation**: Ensure code quality meets KEP standards
- **Community Feedback**: Gather input from Kubernetes SIGs

**Quality Bar**: See `CONTRIBUTING.md` for KEP-level quality requirements

### Deeper Informer Convergence

**Goal**: Align informer patterns with zen-agent (design-level, no shared code yet)

- **Phase 3**: Cross-repo convergence design (see `docs/INFORMERS_CONVERGENCE_NOTES.md`)
- **Shared Patterns**: Common workqueue patterns, retry logic, backoff strategies
- **Client-Go Alignment**: Ensure compatible client-go versions
- **Test Utilities**: Shared test helpers (if feasible)

**Note**: No shared code yet - keep repos independent, but align patterns

### Advanced Features

- **Event Correlation**: Link related observations across sources
- **Temporal Analysis**: Time-series analysis of observation patterns
- **Cost Optimization**: Resource-aware scaling, batch processing
- **Multi-Cluster**: Support for federated observation collection

---

## Long-Term Direction (12+ Months)

### KEP-Grade API Stability

- **v1 API**: Stable, versioned CRDs with long-term support guarantees
- **Deprecation Policy**: Clear deprecation timelines, migration paths
- **Compatibility Testing**: Automated compatibility test suite
- **Release Cadence**: Predictable release schedule aligned with Kubernetes releases

### Multi-Vendor Ecosystem

- **Vendor Adoption**: Multiple vendors using zen-watcher as observation standard
- **Ecosystem Tools**: Third-party tools that consume Observation CRDs
- **Certification**: Compatibility certification program
- **Governance**: CNCF-style governance model (if open-sourced)

### Governance & Maintainer Model

- **Maintainer Onboarding**: Clear process for new maintainers
- **Decision Process**: RFC-style process for major changes
- **Community Standards**: Code of conduct, contribution guidelines
- **Release Management**: Semantic versioning, changelog generation

---

## Key Dependencies & Constraints

### Technical Constraints

- **Client-Go Version**: Must align with Kubernetes version support
- **CRD Compatibility**: Backward compatibility with existing Observation CRDs
- **Resource Limits**: Must run in resource-constrained environments

### Process Constraints

- **Trunk-Based Development**: All work on `main`, no long-lived branches
- **Quality Bar**: KEP-level quality required (see `CONTRIBUTING.md`)
- **Documentation**: All features require documentation updates

### External Dependencies

- **Kubernetes Version**: Support latest 3 minor versions
- **CNCF Ecosystem**: Compatibility with Prometheus, Grafana, etc.
- **Security Tools**: Integration with Trivy, Falco, Kyverno, etc.

---

## Success Metrics

### Quality Metrics

- **Test Coverage**: 80%+ unit test coverage
- **Documentation**: All features documented before merge
- **API Stability**: Zero breaking changes in stable APIs

### Adoption Metrics

- **Demo Success Rate**: 95%+ success rate for `quick-demo.sh`
- **Performance**: <100ms P95 latency for observation creation
- **Resource Usage**: <500MB memory, <500m CPU per instance

### Community Metrics

- **Contributors**: Growing contributor base
- **Issues**: <7 day response time for issues
- **Releases**: Quarterly releases with clear changelogs

---

## Related Documentation

- **Architecture**: `docs/ARCHITECTURE.md`
- **Informer Convergence**: `docs/INFORMERS_CONVERGENCE_NOTES.md`
- **Quality Bar**: `CONTRIBUTING.md` (Quality Bar & API Stability section)
- **Stress Tests**: `docs/STRESS_TEST_RESULTS.md`
- **Dashboards**: `config/dashboards/README.md`

## Historical Context

For historical reference and deep background on design decisions:

- **Expert Package Archive**: `docs/archive/EXPERT_PACKAGE/EXPERT_PACKAGE_SUMMARY.md` - Comprehensive expert analysis from late 2024/early 2025
- **Archive Overview**: `docs/archive/README.md` - Guidelines for using historical archives

**Note**: Archive documents are non-canonical and may be partially obsolete. Always check current documentation (`docs/PM_AI_ROADMAP.md`, `CONTRIBUTING.md`) for authoritative direction.

---

## Release Process

**Release Notes**: All releases must follow `docs/RELEASE_NOTES_TEMPLATE.md` structure. CRD/API changes must reference:
- `docs/OBSERVATION_VERSIONING_AND_RELEASE_PLAN.md` (versioning plan)
- `docs/KEP_DRAFT_ZEN_WATCHER_OBSERVATIONS.md` (if relevant)

**Version History**: See `docs/releases/` for release notes.

---

## How to Prioritize Workstreams

When choosing the next workstream, consider:

1. **Community Value**: Does this help users? Does it demonstrate zen-watcher's value?
2. **Quality Bar**: Does this meet KEP-level standards? Is it backward compatible?
3. **Trunk Hygiene**: Can this be done in small, scoped commits? Does it keep `main` clean?
4. **Dependencies**: Are blockers resolved? Do we have required resources?
5. **Risk**: Is this low-risk? Can we validate it easily?

**Example Prioritization**:
- ✅ High community value + low risk → Do first
- ✅ High quality bar requirement + clear path → Do second
- ⚠️ High risk + unclear path → Design first, implement later
- ❌ Low community value → Defer or skip

---

## Questions or Updates?

This roadmap is a living document. PM AIs should update it as priorities change or work completes.

For questions, see:
- `zen-alpha/docs/PM_HANDBOOK.md` for PM AI coordination
- `CONTRIBUTING.md` for quality standards
- GitHub issues for specific technical questions
