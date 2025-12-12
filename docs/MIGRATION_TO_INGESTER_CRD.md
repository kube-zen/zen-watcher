# Migration Plan: Consolidate to Ingester CRD

## Executive Summary

This document outlines the migration plan to consolidate all `Observation.*Config` CRDs into the unified `Ingester` CRD. The goal is to simplify the configuration model, reduce maintenance burden, and provide a single source of truth for all source configurations.

**Target State**: All source configuration should use the `Ingester` CRD exclusively.

**Timeline**: 4-6 weeks (phased approach)

---

## Current State Analysis

### Existing CRDs

1. **ObservationSourceConfig** (`zen.kube-zen.io/v1alpha1`)
   - **Purpose**: Defines source adapters (informer, webhook, logs, configmap, k8s-events)
   - **Usage**: Used by `GenericOrchestrator` to start adapters
   - **Loader**: `pkg/config/source_config_loader.go`
   - **Status**: ⚠️ **ACTIVELY USED** - Primary orchestrator dependency

2. **ObservationTypeConfig** (`zen.kube-zen.io/v1alpha1`)
   - **Purpose**: Defines normalization rules per event type
   - **Usage**: Used for type-specific normalization
   - **Loader**: `pkg/config/type_config_loader.go`
   - **Status**: ⚠️ **ACTIVELY USED** - Type normalization

3. **ObservationFilter** (`zen.kube-zen.io/v1alpha1`)
   - **Purpose**: Per-source filter rules
   - **Usage**: Merged with ConfigMap filters
   - **Loader**: `pkg/config/observationfilter_loader.go`
   - **Status**: ⚠️ **ACTIVELY USED** - Filtering system

4. **ObservationDedupConfig** (`zen.kube-zen.io/v1alpha1`)
   - **Purpose**: Per-source deduplication windows
   - **Usage**: Configures deduplication per source
   - **Loader**: `pkg/config/observationdedupconfig_loader.go`
   - **Status**: ⚠️ **ACTIVELY USED** - Deduplication system

### Ingester CRD Current State

The `Ingester` CRD (`zen.kube-zen.io/v1alpha1`) already includes:
- ✅ Source and ingester type configuration
- ✅ Filter configuration (`spec.filters`)
- ✅ Deduplication configuration (`spec.deduplication`)
- ✅ Normalization/mapping configuration (`spec.destinations[].mapping`)
- ✅ Processing/optimization configuration (`spec.optimization`)
- ✅ Informer, webhook, logs, k8s-events adapter configs

**Gap Analysis**: The Ingester CRD already covers most functionality, but:
- ⚠️ Not used by `GenericOrchestrator` (still uses `ObservationSourceConfig`)
- ⚠️ Type-specific normalization not fully covered
- ⚠️ Some advanced features may need enhancement

---

## Migration Strategy

### Phase 1: Feature Parity (Week 1-2)

**Goal**: Ensure Ingester CRD has feature parity with all Observation.*Config CRDs

#### 1.1 Enhance Ingester CRD Schema

**Tasks**:
- [ ] Add type-specific normalization support to Ingester CRD
  - Extend `spec.destinations[].mapping` to support type-specific rules
  - Add `spec.typeConfig` section for type-specific normalization
- [ ] Verify all filter features are covered
  - Compare `ObservationFilter` capabilities with `Ingester.spec.filters`
  - Add any missing filter features
- [ ] Verify all deduplication features are covered
  - Compare `ObservationDedupConfig` capabilities with `Ingester.spec.deduplication`
  - Ensure window, strategy, fields all supported
- [ ] Add backward compatibility annotations
  - Add `zen.io/migrated-from` annotation support

**Files to Modify**:
- `deployments/crds/ingester_crd.yaml`

#### 1.2 Update Ingester Loader

**Tasks**:
- [ ] Enhance `IngesterConfig` struct to include all features
- [ ] Add type-specific normalization support
- [ ] Ensure filter and dedup configs are fully parsed
- [ ] Add validation for all new fields

**Files to Modify**:
- `pkg/config/ingester_loader.go`

#### 1.3 Create Conversion Utilities

**Tasks**:
- [ ] Create `ObservationSourceConfig` → `Ingester` converter
- [ ] Create `ObservationTypeConfig` → `Ingester` converter
- [ ] Create `ObservationFilter` → `Ingester` converter
- [ ] Create `ObservationDedupConfig` → `Ingester` converter
- [ ] Add conversion tests

**New Files**:
- `pkg/config/migration/converter.go`
- `pkg/config/migration/converter_test.go`

### Phase 2: Update Orchestrator (Week 2-3)

**Goal**: Make `GenericOrchestrator` use Ingester CRD instead of ObservationSourceConfig

#### 2.1 Update GenericOrchestrator

**Tasks**:
- [ ] Change `GenericOrchestrator` to watch Ingester CRDs instead of ObservationSourceConfig
- [ ] Update `reloadAdapters()` to use Ingester loader
- [ ] Convert Ingester configs to `generic.SourceConfig` format
- [ ] Maintain backward compatibility during transition

**Files to Modify**:
- `pkg/orchestrator/generic.go`

#### 2.2 Update Observation Creator

**Tasks**:
- [ ] Update `ObservationCreator` to use Ingester loader instead of SourceConfigLoader
- [ ] Ensure all optimization features work with Ingester configs
- [ ] Update field extraction to use Ingester normalization configs

**Files to Modify**:
- `pkg/watcher/observation_creator.go`

### Phase 3: Update Filter and Dedup Systems (Week 3-4)

**Goal**: Make filter and dedup systems use Ingester CRD

#### 3.1 Update Filter System

**Tasks**:
- [ ] Update filter loader to read from Ingester CRDs
- [ ] Merge Ingester filter configs with ConfigMap filters
- [ ] Maintain ObservationFilter support during transition (deprecation period)
- [ ] Add migration path for existing ObservationFilter CRDs

**Files to Modify**:
- `pkg/config/observationfilter_loader.go` (add Ingester support)
- `pkg/filter/merger.go` (if needed)

#### 3.2 Update Dedup System

**Tasks**:
- [ ] Update dedup loader to read from Ingester CRDs
- [ ] Ensure per-source dedup windows work from Ingester
- [ ] Maintain ObservationDedupConfig support during transition
- [ ] Add migration path for existing ObservationDedupConfig CRDs

**Files to Modify**:
- `pkg/config/observationdedupconfig_loader.go` (add Ingester support)

#### 3.3 Update Type Config System

**Tasks**:
- [ ] Integrate type-specific normalization into Ingester processing
- [ ] Update type config loader to read from Ingester CRDs
- [ ] Maintain ObservationTypeConfig support during transition
- [ ] Add migration path for existing ObservationTypeConfig CRDs

**Files to Modify**:
- `pkg/config/type_config_loader.go` (add Ingester support)

### Phase 4: Migration Tooling (Week 4-5)

**Goal**: Provide tools to help users migrate

#### 4.1 Create Migration Script

**Tasks**:
- [ ] Create `scripts/migrate-to-ingester.sh` script
- [ ] Script should:
  - Read existing Observation.*Config CRDs
  - Convert to Ingester CRDs
  - Apply new Ingester CRDs
  - Validate migration
  - Optionally delete old CRDs
- [ ] Add dry-run mode
- [ ] Add rollback capability

**New Files**:
- `scripts/migrate-to-ingester.sh`
- `scripts/migrate-to-ingester.go` (if Go-based tool needed)

#### 4.2 Create Migration Documentation

**Tasks**:
- [ ] Document migration steps
- [ ] Provide example conversions
- [ ] Document breaking changes (if any)
- [ ] Create migration FAQ

**New Files**:
- `docs/MIGRATION_GUIDE.md`
- `docs/MIGRATION_EXAMPLES.md`

### Phase 5: Deprecation and Removal (Week 5-6)

**Goal**: Deprecate old CRDs and prepare for removal

#### 5.1 Add Deprecation Warnings

**Tasks**:
- [ ] Add deprecation warnings to Observation.*Config loaders
- [ ] Log warnings when old CRDs are used
- [ ] Add deprecation notices to CRD schemas
- [ ] Update documentation with deprecation notices

**Files to Modify**:
- All Observation.*Config loaders
- CRD YAML files (add deprecation annotations)

#### 5.2 Maintain Dual Support

**Tasks**:
- [ ] Keep Observation.*Config support for 2-3 release cycles
- [ ] Prioritize Ingester CRD in case of conflicts
- [ ] Document deprecation timeline

#### 5.3 Future Removal (Post-Migration)

**Tasks** (for future release):
- [ ] Remove Observation.*Config CRDs
- [ ] Remove Observation.*Config loaders
- [ ] Remove conversion utilities (or keep for reference)
- [ ] Update all documentation

---

## Detailed Migration Steps

### Step 1: Convert ObservationSourceConfig to Ingester

**Example Conversion**:

**Before (ObservationSourceConfig)**:
```yaml
apiVersion: zen.kube-zen.io/v1alpha1
kind: ObservationSourceConfig
metadata:
  name: trivy-scanner
  namespace: zen-system
spec:
  source: trivy
  ingester: informer
  informer:
    gvr:
      group: aquasecurity.github.io
      version: v1alpha1
      resource: vulnerabilityreports
  filter:
    minPriority: 0.5
  dedup:
    window: "24h"
    strategy: fingerprint
```

**After (Ingester)**:
```yaml
apiVersion: zen.kube-zen.io/v1alpha1
kind: Ingester
metadata:
  name: trivy-scanner
  namespace: zen-system
  annotations:
    zen.io/migrated-from: "ObservationSourceConfig/trivy-scanner"
spec:
  source: trivy
  ingester: informer
  informer:
    gvr:
      group: aquasecurity.github.io
      version: v1alpha1
      resource: vulnerabilityreports
  filters:
    minPriority: 0.5
  deduplication:
    enabled: true
    window: "24h"
    strategy: fingerprint
  destinations:
    - type: crd
      value: observations
```

### Step 2: Convert ObservationFilter to Ingester

**Before (ObservationFilter)**:
```yaml
apiVersion: zen.kube-zen.io/v1alpha1
kind: ObservationFilter
metadata:
  name: trivy-filter
spec:
  targetSource: trivy
  minPriority: 0.7
  excludeNamespaces:
    - kube-system
    - kube-public
```

**After (Ingester - merged into existing)**:
```yaml
apiVersion: zen.kube-zen.io/v1alpha1
kind: Ingester
metadata:
  name: trivy-scanner
spec:
  source: trivy
  # ... other fields ...
  filters:
    minPriority: 0.7
    excludeNamespaces:
      - kube-system
      - kube-public
```

### Step 3: Convert ObservationDedupConfig to Ingester

**Before (ObservationDedupConfig)**:
```yaml
apiVersion: zen.kube-zen.io/v1alpha1
kind: ObservationDedupConfig
metadata:
  name: trivy-dedup
spec:
  targetSource: trivy
  enabled: true
  windowSeconds: 86400
  strategy: fingerprint
```

**After (Ingester - merged into existing)**:
```yaml
apiVersion: zen.kube-zen.io/v1alpha1
kind: Ingester
metadata:
  name: trivy-scanner
spec:
  source: trivy
  # ... other fields ...
  deduplication:
    enabled: true
    window: "24h"  # Converted from windowSeconds
    strategy: fingerprint
```

### Step 4: Convert ObservationTypeConfig to Ingester

**Before (ObservationTypeConfig)**:
```yaml
apiVersion: zen.kube-zen.io/v1alpha1
kind: ObservationTypeConfig
metadata:
  name: trivy-vulnerability-type
spec:
  targetType: vulnerability
  normalization:
    domain: security
    priority:
      CRITICAL: 0.9
      HIGH: 0.7
```

**After (Ingester - in destinations mapping)**:
```yaml
apiVersion: zen.kube-zen.io/v1alpha1
kind: Ingester
metadata:
  name: trivy-scanner
spec:
  source: trivy
  # ... other fields ...
  destinations:
    - type: crd
      value: observations
      mapping:
        domain: security
        type: vulnerability
        priority:
          CRITICAL: 0.9
          HIGH: 0.7
```

---

## Risk Assessment

### High Risk Areas

1. **GenericOrchestrator Migration**
   - **Risk**: Breaking existing deployments
   - **Mitigation**: Support both CRDs during transition period
   - **Rollback**: Keep ObservationSourceConfig support active

2. **Filter System Changes**
   - **Risk**: Filter behavior changes
   - **Mitigation**: Comprehensive testing, maintain merge semantics
   - **Rollback**: Keep ObservationFilter support active

3. **Dedup System Changes**
   - **Risk**: Deduplication behavior changes
   - **Mitigation**: Verify window calculations, test edge cases
   - **Rollback**: Keep ObservationDedupConfig support active

### Medium Risk Areas

1. **Type Normalization**
   - **Risk**: Normalization rules may not map perfectly
   - **Mitigation**: Careful conversion, extensive testing
   - **Rollback**: Keep ObservationTypeConfig support active

2. **Configuration Conflicts**
   - **Risk**: Both old and new CRDs present
   - **Mitigation**: Prioritize Ingester, log conflicts
   - **Rollback**: Clear conflict resolution strategy

### Low Risk Areas

1. **Documentation Updates**
   - **Risk**: Outdated documentation
   - **Mitigation**: Update docs in parallel with code changes
   - **Rollback**: N/A

---

## Testing Strategy

### Unit Tests

- [ ] Test Ingester CRD schema validation
- [ ] Test conversion utilities
- [ ] Test Ingester loader with all features
- [ ] Test filter and dedup integration

### Integration Tests

- [ ] Test GenericOrchestrator with Ingester CRDs
- [ ] Test filter merge with Ingester configs
- [ ] Test dedup with Ingester configs
- [ ] Test type normalization with Ingester configs

### E2E Tests

- [ ] Deploy with Ingester CRDs only
- [ ] Deploy with mixed CRDs (old + new)
- [ ] Test migration script
- [ ] Test rollback scenario

### Backward Compatibility Tests

- [ ] Verify ObservationSourceConfig still works
- [ ] Verify ObservationFilter still works
- [ ] Verify ObservationDedupConfig still works
- [ ] Verify ObservationTypeConfig still works
- [ ] Test conflict resolution

---

## Success Criteria

### Phase 1 Success

- ✅ Ingester CRD has feature parity with all Observation.*Config CRDs
- ✅ All conversion utilities work correctly
- ✅ Unit tests pass

### Phase 2 Success

- ✅ GenericOrchestrator works with Ingester CRDs
- ✅ All adapters start correctly
- ✅ Integration tests pass

### Phase 3 Success

- ✅ Filter system works with Ingester CRDs
- ✅ Dedup system works with Ingester CRDs
- ✅ Type normalization works with Ingester CRDs
- ✅ E2E tests pass

### Phase 4 Success

- ✅ Migration script works correctly
- ✅ Documentation is complete
- ✅ Users can successfully migrate

### Phase 5 Success

- ✅ Deprecation warnings are in place
- ✅ Dual support works correctly
- ✅ No breaking changes for existing users

---

## Rollback Plan

### If Issues Detected

1. **Immediate Rollback**:
   - Revert to using ObservationSourceConfig in orchestrator
   - Keep Ingester support as optional
   - Document issues and fixes needed

2. **Partial Rollback**:
   - Keep Ingester for new deployments
   - Maintain ObservationSourceConfig for existing
   - Gradual migration

3. **Full Rollback**:
   - Revert all changes
   - Restore ObservationSourceConfig as primary
   - Re-evaluate migration strategy

---

## Timeline Summary

| Phase | Duration | Key Deliverables |
|-------|----------|------------------|
| Phase 1 | Week 1-2 | Feature parity, conversion utilities |
| Phase 2 | Week 2-3 | Orchestrator migration |
| Phase 3 | Week 3-4 | Filter/dedup/type system updates |
| Phase 4 | Week 4-5 | Migration tooling and docs |
| Phase 5 | Week 5-6 | Deprecation warnings, dual support |

**Total Duration**: 4-6 weeks

---

## Next Steps

1. **Review and Approve**: Get stakeholder approval for migration plan
2. **Start Phase 1**: Begin feature parity work
3. **Create Issues**: Break down phases into specific GitHub issues
4. **Set Up Testing**: Prepare test environments
5. **Communicate**: Notify users of upcoming migration

---

## References

- [Ingester CRD Schema](../deployments/crds/ingester_crd.yaml)
- [ObservationSourceConfig CRD](../deployments/crds/observationsourceconfig_crd.yaml)
- [ObservationFilter CRD](../deployments/crds/observationfilter_crd.yaml)
- [ObservationDedupConfig CRD](../deployments/crds/observationdedupconfig_crd.yaml)
- [ObservationTypeConfig CRD](../deployments/crds/observationtypeconfig_crd.yaml)
- [Current Orchestrator Implementation](../pkg/orchestrator/generic.go)

