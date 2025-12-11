---
⚠️ HISTORICAL DOCUMENT - EXPERT PACKAGE ARCHIVE ⚠️

This document is from an external "Expert Package" analysis of zen-watcher/ingester.
It reflects the state of zen-watcher at a specific point in time and may be partially obsolete.

CANONICAL SOURCES (use these for current direction):
- docs/PM_AI_ROADMAP.md - Current roadmap and priorities
- CONTRIBUTING.md - Current quality bar and standards
- docs/INFORMERS_CONVERGENCE_NOTES.md - Current informer architecture
- docs/STRESS_TEST_RESULTS.md - Current performance baselines

This archive document is provided for historical context, rationale, and inspiration only.
Do NOT use this as a replacement for current documentation.

---

# Zen Watcher Dashboard & Script Analysis Report

## Executive Summary

Analysis reveals critical issues in both Grafana dashboards and script organization that need immediate attention to ensure reliable monitoring and operational consistency.

---

## 🔴 Dashboard Issues Identified

### 1. **Critical Display Problems**

#### Unit Formatting Issues
- **Problem**: Panels using `"unit": "short"` cause display confusion
- **Example**: Shows "100k" when actual value is ~850 (K=1000 conversion)
- **Impact**: Misleading executives and operations teams about actual event volumes
- **Affected Files**: All dashboard JSON files

#### Hardcoded Values (NaN Issues)
- **Problem**: Panels with static expressions like `"expr": "0"` and `"expr": "100"`
- **Impact**: Shows NaN or constant values regardless of actual metrics
- **Examples Found**:
  - `zen-watcher-operations.json:248` - Error rate showing "0"
  - `zen-watcher-executive.json:380` - Success rate showing "100"

### 2. **Calculation Inconsistencies**

#### Metric Name Confusion
- **Problem**: Mixed usage of `zen_watcher_events_total` vs `zen_watcher_observations_created_total`
- **Impact**: Inconsistent data across dashboards, confusion about what metrics represent
- **Examples**:
  - Executive dashboard uses observations for some panels, events for others
  - Security dashboard uses events for all metrics

#### Time Window Inconsistencies
- **Problem**: Different panels use different time windows (1h, 6h, 24h) without clear rationale
- **Impact**: Difficult to correlate data across time ranges

### 3. **Missing Dashboard Variables**

#### Undefined Variables
- **Problem**: Dashboards reference variables like `$source`, `$category`, `$severity`, `$eventType`, `$namespace`, `$kind`
- **Impact**: Variable-based filtering doesn't work, causing filtering panels to be non-functional
- **Affected**: zen-watcher-executive.json, zen-watcher-security.json

### 4. **Performance & Resource Issues**

#### Heavy Queries
- **Problem**: Some panels use complex PromQL queries that could impact performance
- **Impact**: Slow dashboard loading, potential timeout issues

---

## 🟠 Script Organization Issues

### 1. **Directory Duplication**

#### Overlapping Functionality
- **Problem**: Both `./hack/` and `./scripts/` contain similar functionality
- **Examples**:
  - `./hack/quick-demo.sh` vs `./scripts/quick-demo.sh`
  - `./hack/mock-data.sh` vs `./scripts/data/mock-data.sh`
  - `./hack/benchmark/` vs `./scripts/benchmark/`

#### Inconsistent Structure
- **Problem**: No clear separation between development tools and operational scripts
- **Impact**: Confusing for team members, maintenance overhead

### 2. **Benchmark Scripts Scattered**
- **Problem**: Benchmark scripts exist in both locations
- **Impact**: Inconsistent usage, potential version conflicts

### 3. **Documentation Conflicts**
- **Problem**: README files reference different paths
- **Example**: Dashboard guide references `./hack/quick-demo.sh` while actual working script is in `./scripts/`

---

## 🎯 Recommended Solutions

### Dashboard Fixes

1. **Fix Unit Formatting**
   - Replace "short" units with appropriate units (none, bytes, seconds)
   - Add thousand separators for large numbers
   - Use custom formatting for percentage displays

2. **Replace Hardcoded Values**
   - Replace static expressions with actual metric queries
   - Add proper error handling for missing metrics

3. **Standardize Metrics Usage**
   - Use `zen_watcher_events_total` for security events
   - Use `zen_watcher_observations_created_total` for system metrics
   - Ensure consistent metric names across all panels

4. **Add Dashboard Variables**
   - Define variables for filtering (source, category, severity)
   - Add namespace and cluster variables for multi-tenancy
   - Set proper default values

5. **Optimize Performance**
   - Simplify complex queries where possible
   - Use recording rules for expensive calculations
   - Add appropriate step sizes for time ranges

### Script Reorganization

1. **Consolidate to `./scripts/`**
   - Move all functional scripts from `./hack/` to `./scripts/`
   - Keep `./hack/` for development tools only
   - Update all references and documentation

2. **Standardize Benchmark Structure**
   - All benchmarks in `./scripts/benchmark/`
   - Remove duplicate benchmark scripts

3. **Update Documentation**
   - Fix all path references
   - Update README files to reflect new structure

---

## 🚀 Implementation Priority

### High Priority (Immediate)
1. Fix dashboard unit formatting and hardcoded values
2. Consolidate script directories
3. Update documentation references

### Medium Priority (This Week)
1. Add dashboard variables
2. Standardize metric usage
3. Optimize query performance

### Low Priority (Future)
1. Add new dashboard panels
2. Create additional specialized dashboards
3. Implement dashboard versioning

---

## 📋 Files Requiring Updates

### Dashboard Files (6 files)
- `config/dashboards/zen-watcher-executive.json`
- `config/dashboards/zen-watcher-operations.json`
- `config/dashboards/zen-watcher-security.json`
- `config/dashboards/zen-watcher-dashboard.json`
- `config/dashboards/zen-watcher-namespace-health.json`
- `config/dashboards/zen-watcher-explorer.json`

### Script Files
- Consolidate `./hack/` → `./scripts/` (except dev tools)
- Update README files in both directories
- Fix all internal script references

### Documentation Files
- `config/dashboards/README.md`
- `scripts/README.md`
- `hack/README.md`
- Various doc files with script references

---

**Next Steps**: Detailed implementation plan with specific fixes and team instructions to follow.