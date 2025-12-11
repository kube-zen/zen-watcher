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

# Team Instructions: Dashboard & Script Fixes

## 🎯 Overview

This document provides step-by-step instructions for implementing critical fixes to Zen Watcher's Grafana dashboards and script organization. These fixes address display issues, calculation errors, and operational inconsistencies.

## 📊 Dashboard Fixes Applied

### ✅ Issues Fixed

1. **Display Formatting Issues**
   - ✅ Replaced confusing "short" units with appropriate units
   - ✅ Fixed "100k" display problem (was showing K=1000 conversion incorrectly)
   - ✅ Added proper thousand separators for large numbers
   - ✅ Standardized percentage displays

2. **Hardcoded Values (NaN Issues)**
   - ✅ Replaced `expr: "0"` with actual error rate calculations
   - ✅ Replaced `expr: "100"` with real success rate metrics
   - ✅ All panels now show live data instead of static values

3. **Missing Dashboard Variables**
   - ✅ Added comprehensive filtering variables:
     - `${datasource}` - Data source selector
     - `${source}` - Security tool filter (Trivy, Falco, Kyverno, etc.)
     - `${category}` - Event category filter
     - `${severity}` - Severity level filter (Critical, High, Medium, Low)
     - `${eventType}` - Event type filter
     - `${namespace}` - Namespace filter for multi-tenancy

4. **Metric Standardization**
   - ✅ Consistent use of `zen_watcher_events_total` for security events
   - ✅ Consistent use of `zen_watcher_observations_created_total` for system metrics
   - ✅ Fixed calculation inconsistencies across dashboards

### 📈 Dashboard Performance Improvements

- ✅ Optimized query performance for faster loading
- ✅ Improved time window consistency
- ✅ Added proper threshold configurations

## 🔧 Script Organization Changes

### ✅ Reorganization Completed

**Scripts Moved from `./hack/` to `./scripts/`:**
- ✅ `quick-demo.sh` → `./scripts/quick-demo.sh`
- ✅ `mock-data.sh` → `./scripts/data/mock-data.sh`
- ✅ `cleanup-demo.sh` → `./scripts/cleanup-demo.sh`
- ✅ `e2e-test.sh` → `./scripts/ci/e2e-test.sh`
- ✅ `send-mock-webhooks.sh` → `./scripts/data/send-webhooks.sh`
- ✅ `helmfile.yaml.gotmpl` → `./scripts/helmfile.yaml.gotmpl`
- ✅ `benchmark/` directory → `./scripts/benchmark/`

**Documentation Updated:**
- ✅ 14 script references updated across 7 files
- ✅ All README files now point to correct script locations
- ✅ New `./hack/README.md` created for dev tools only

## 🧪 Testing Instructions

### Step 1: Verify Dashboard Functionality

```bash
# 1. Start the demo environment
./scripts/quick-demo.sh --non-interactive --deploy-mock-data

# 2. Access Grafana (credentials displayed at end)
# URL: http://localhost:3100
# Login: zen / <password-from-demo>

# 3. Verify each dashboard shows real data:
#    - Executive Overview: Should show actual event counts (not "100k")
#    - Operations: Should show live metrics (not NaN)
#    - Security Analytics: Should show real security events
```

**Expected Results:**
- ✅ No more "100k" confusion - numbers show actual values with proper formatting
- ✅ No more NaN values - all panels show real data
- ✅ Dashboard variables work - filtering by source, category, severity
- ✅ Performance metrics display correctly (percentages, rates, durations)

### Step 2: Test Dashboard Variables

In each dashboard, test the new filtering variables:

```bash
# In Grafana, use the dropdown menus at the top:
# - Security Tool: Filter by specific tools (Trivy, Falco, Kyverno)
# - Category: Filter by event categories
# - Severity: Filter by severity levels
# - Event Type: Filter by specific event types
# - Namespace: Filter by Kubernetes namespaces
```

**Expected Results:**
- ✅ Dropdown menus populate with actual values from your environment
- ✅ Filtering works correctly and updates all panels
- ✅ "All" option shows combined data from all selections

### Step 3: Verify Script Organization

```bash
# 1. Check that old paths no longer work
./hack/quick-demo.sh  # Should fail or show deprecation message

# 2. Verify new script locations work
./scripts/quick-demo.sh --help
./scripts/data/mock-data.sh --help
./scripts/benchmark/quick-bench.sh --help

# 3. Check benchmark scripts exist and work
ls -la ./scripts/benchmark/
./scripts/benchmark/load-test.sh --help
./scripts/benchmark/burst-test.sh --help
./scripts/benchmark/stress-test.sh --help
```

**Expected Results:**
- ✅ All functional scripts now in `./scripts/` directory
- ✅ Benchmark scripts accessible via `./scripts/benchmark/`
- ✅ Documentation references updated correctly

## 🔍 Troubleshooting

### Dashboard Issues

**Problem:** Still showing "100k" or similar confusing numbers
```bash
# Solution: Check unit configuration in panel settings
# 1. Open problematic panel in Grafana
# 2. Panel Edit → Field → Unit → Change to "none" or "short"
# 3. Save dashboard
```

**Problem:** Variables show "No data" or empty dropdowns
```bash
# Solution: Verify Prometheus datasource and metrics
# 1. Check Grafana datasource points to correct Prometheus
# 2. Verify zen-watcher is deployed and generating metrics
# 3. Check metrics endpoints: kubectl get endpoints -n zen-system
```

**Problem:** Panels show "No data" or NaN
```bash
# Solution: Check metric availability
# 1. Open Prometheus UI
# 2. Query: zen_watcher_events_total
# 3. If no data, check zen-watcher deployment status
```

### Script Issues

**Problem:** Script not found errors
```bash
# Solution: Update to new script locations
# Old: ./hack/quick-demo.sh
# New: ./scripts/quick-demo.sh

# Check the new location
ls -la ./scripts/quick-demo.sh
```

**Problem:** Benchmark scripts not found
```bash
# Solution: All benchmarks now in ./scripts/benchmark/
ls -la ./scripts/benchmark/
# Should show: load-test.sh, burst-test.sh, stress-test.sh, etc.
```

## 📋 Verification Checklist

### Dashboard Verification
- [ ] Executive dashboard shows real event counts (not "100k")
- [ ] Operations dashboard shows live metrics (not NaN)
- [ ] Security dashboard shows actual security events
- [ ] All dashboard variables populate with real data
- [ ] Filtering works correctly across all panels
- [ ] Performance metrics display with correct units
- [ ] No hardcoded values or static numbers

### Script Verification
- [ ] `./scripts/quick-demo.sh` works correctly
- [ ] `./scripts/benchmark/` contains all benchmark scripts
- [ ] All documentation references point to `./scripts/`
- [ ] Old `./hack/` paths show deprecation or fail appropriately
- [ ] Team can run stress tests using new script locations

## 🚀 Next Steps

### Immediate (Today)
1. **Test dashboard fixes** using the demo environment
2. **Verify script reorganization** works correctly
3. **Update any local scripts** that might reference old paths

### This Week
1. **Update any custom dashboards** using the same principles
2. **Train team** on new script organization
3. **Update any CI/CD pipelines** that use old script paths

### Future Considerations
1. **Create additional specialized dashboards** for specific use cases
2. **Implement dashboard versioning** for better change tracking
3. **Add automated dashboard testing** to CI pipeline

## 📞 Support

If you encounter any issues:

1. **Check this document** for troubleshooting steps
2. **Verify the demo environment** works correctly
3. **Check the fixed dashboard files** in `/config/dashboards/`
4. **Review script locations** in `/scripts/` directory

---

**Summary:** All dashboard display issues have been fixed, scripts reorganized to `/scripts/`, and comprehensive testing instructions provided. The fixes ensure reliable monitoring with accurate data display and improved operational consistency.