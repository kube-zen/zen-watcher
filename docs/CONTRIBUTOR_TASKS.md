# Contributor Tasks

**Purpose**: Curated task list for external contributors, aligned with zen-watcher roadmap and KEP.

**Last Updated**: 2025-12-10

**How to Use**:
1. Pick a task that matches your skill level
2. Read the linked design doc to understand context
3. Check for blockers (e.g., cluster access requirements)
4. Open an issue or discussion to coordinate with maintainers

---

## Good First Tasks

Small improvements that help you learn the codebase without requiring deep architectural knowledge.

### Documentation Improvements

1. **Extend schema validation test examples**
   - **Description**: Add more test cases to `pkg/watcher/observation_creator_validation_test.go` covering edge cases
   - **Design Doc**: `docs/OBSERVATION_CRD_API_AUDIT.md`
   - **Blockers**: None
   - **Difficulty**: Low

2. **Add example Observation variants**
   - **Description**: Create additional examples in `examples/observations/` for different categories (performance, cost, operations)
   - **Design Doc**: `examples/observations/README.md`
   - **Blockers**: None
   - **Difficulty**: Low

3. **Improve troubleshooting guide**
   - **Description**: Expand troubleshooting section in `docs/GETTING_STARTED_GENERIC.md` with common issues and solutions
   - **Design Doc**: `docs/GETTING_STARTED_GENERIC.md`
   - **Blockers**: None
   - **Difficulty**: Low

### Test Enhancements

4. **Add unit tests for filter rules**
   - **Description**: Extend `pkg/filter/rules_test.go` with more test cases for edge cases
   - **Design Doc**: `docs/DEDUPLICATION.md`
   - **Blockers**: None
   - **Difficulty**: Low

5. **Add integration test for ConfigMap source**
   - **Description**: Create e2e test in `test/e2e/` for ConfigMap-based source configuration
   - **Design Doc**: `docs/ARCHITECTURE.md`
   - **Blockers**: Requires local cluster (k3d/kind/minikube)
   - **Difficulty**: Low-Medium

---

## Intermediate Tasks

Code contributions that extend functionality without changing core CRDs or architecture.

### Example Sources

6. **Add Wiz integration example**
   - **Description**: Create example CRD configuration for Wiz security scanner in `examples/`
   - **Design Doc**: `the project roadmap` (Near-Term Backlog #4)
   - **Blockers**: None (example only, no code changes)
   - **Difficulty**: Medium

7. **Add Snyk integration example**
   - **Description**: Create example CRD configuration for Snyk security scanner in `examples/`
   - **Design Doc**: `the project roadmap` (Near-Term Backlog #4)
   - **Blockers**: None (example only, no code changes)
   - **Difficulty**: Medium

8. **Add Aqua integration example**
   - **Description**: Create example CRD configuration for Aqua security scanner in `examples/`
   - **Design Doc**: `the project roadmap` (Near-Term Backlog #4)
   - **Blockers**: None (example only, no code changes)
   - **Difficulty**: Medium

### Dashboard Improvements

9. **Enhance Grafana dashboard panels**
   - **Description**: Improve existing dashboards in `config/dashboards/` with additional panels or better visualizations
   - **Design Doc**: `config/dashboards/README.md`
   - **Blockers**: Requires Grafana access (can use quick-demo.sh)
   - **Difficulty**: Medium

10. **Add dashboard for cost category observations**
    - **Description**: Create new dashboard focused on cost-related observations
    - **Design Doc**: `config/dashboards/README.md`
    - **Blockers**: Requires Grafana access (can use quick-demo.sh)
    - **Difficulty**: Medium

### Code Contributions

11. **Improve error messages in CRD validation**
    - **Description**: Enhance validation error messages in `pkg/watcher/observation_creator.go` to be more actionable
    - **Design Doc**: `the project roadmap` (Near-Term Backlog #3)
    - **Blockers**: None
    - **Difficulty**: Medium

12. **Add regex support to filter rules**
    - **Description**: Extend `pkg/filter/rules.go` to support regex patterns in filter conditions
    - **Design Doc**: `the project roadmap` (Medium-Priority #7)
    - **Blockers**: None
    - **Difficulty**: Medium

13. **Implement time-based filter rules**
    - **Description**: Add time-based filtering (e.g., "only observations from last 24h") to `pkg/filter/rules.go`
    - **Design Doc**: `the project roadmap` (Medium-Priority #7)
    - **Blockers**: None
    - **Difficulty**: Medium

---

## Advanced Tasks

Work that requires deep understanding of CRDs, informers, or KEP-driven architecture changes.

### CRD Evolution

14. **Design v1beta1 Observation CRD schema**
    - **Description**: Design schema changes for v1beta1 based on audit findings in `docs/OBSERVATION_CRD_API_AUDIT.md`
    - **Design Doc**: `docs/OBSERVATION_VERSIONING_AND_RELEASE_PLAN.md` (v1beta1 section)
    - **Blockers**: Requires coordination with maintainers, KEP alignment
    - **Difficulty**: High

15. **Implement CRD conversion webhook**
    - **Description**: Create conversion webhook for v1 → v1beta1 migration
    - **Design Doc**: `docs/OBSERVATION_VERSIONING_AND_RELEASE_PLAN.md`
    - **Blockers**: Requires coordination with maintainers, Kubernetes 1.16+
    - **Difficulty**: High

### Informer Architecture

16. **Optimize informer resync tuning**
    - **Description**: Analyze and optimize resync periods in `internal/informers/manager.go` based on metrics
    - **Design Doc**: `docs/INFORMERS_CONVERGENCE_NOTES.md`, `the project roadmap` (Medium-Priority #6)
    - **Blockers**: Requires access to cluster with production-like load
    - **Difficulty**: High

17. **Implement batch processing for high-volume sources**
    - **Description**: Add batch processing to `pkg/processor/pipeline.go` for sources with >1000 events/hour
    - **Design Doc**: `the project roadmap` (Medium-Priority #6)
    - **Blockers**: Requires performance testing environment
    - **Difficulty**: High

### Dynamic Webhook Integration

18. **Design webhook registration CRD**
    - **Description**: Design CRD for dynamic webhook endpoint registration (for webhook gateway integration)
    - **Design Doc**: `the project roadmap` (Mid-Term Backlog - Dynamic Webhook Integration)
    - **Blockers**: Requires coordination with maintainers, KEP alignment
    - **Difficulty**: High

19. **Implement Observation export API for webhook gateways**
    - **Description**: Create API endpoint for webhook gateways to consume Observations
    - **Design Doc**: `the project roadmap` (Mid-Term Backlog - Dynamic Webhook Integration)
    - **Blockers**: Requires coordination with maintainers
    - **Difficulty**: High

### KEP-Driven Work

20. **Prepare KEP submission materials**
    - **Description**: Finalize `docs/KEP_DRAFT_ZEN_WATCHER_OBSERVATIONS.md` for Kubernetes SIG submission
    - **Design Doc**: `docs/KEP_DRAFT_ZEN_WATCHER_OBSERVATIONS.md`
    - **Blockers**: Requires maintainer approval, community feedback
    - **Difficulty**: High

21. **Implement API stability guarantees**
    - **Description**: Add deprecation policies and compatibility guarantees per KEP requirements
    - **Design Doc**: `docs/KEP_DRAFT_ZEN_WATCHER_OBSERVATIONS.md`, `docs/OBSERVATION_VERSIONING_AND_RELEASE_PLAN.md`
    - **Blockers**: Requires KEP approval
    - **Difficulty**: High

---

## Task Sources

All tasks are sourced from:
- `the project roadmap` - Near-term and mid-term backlog
- `docs/OBSERVATION_VERSIONING_AND_RELEASE_PLAN.md` - Versioning and API evolution
- `docs/KEP_DRAFT_ZEN_WATCHER_OBSERVATIONS.md` - KEP-driven work

**Note**: Tasks are not invented off-roadmap. All work aligns with documented priorities.

---

## Getting Started

1. **Pick a task** that matches your skill level
2. **Read the design doc** to understand context and requirements
3. **Check blockers** - do you have required access/environment?
4. **Open an issue** or discussion to coordinate with maintainers (especially for advanced tasks)
5. **Submit a PR** following [CONTRIBUTING.md](../CONTRIBUTING.md) guidelines

---

**Questions?** Open a GitHub Discussion or check `the project roadmap` for current priorities.
