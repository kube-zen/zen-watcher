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

# Quick Checklist for Team - Zen Watcher Stress Testing

## ✅ Pre-Execution Checklist

- [ ] Zen Watcher running: `kubectl get pods -n zen-system | grep zen-watcher`
- [ ] Metrics server working: `kubectl top pods -n zen-system --help`
- [ ] Tools available: `which kubectl bc jq`
- [ ] Clone latest changes: `git pull && git checkout main`

## ✅ Execution Order (Don't Skip!)

### 1. KEP Fixes (5 min)
- [ ] Update `keps/sig-foo/0000-zen-watcher/README.md`
  - [ ] Change `sig-foo` to `sig-observability`
  - [ ] Update dates to 2025-12-08
- [ ] Commit: "docs: fix KEP SIG assignment"

### 2. Stress Tests (60-90 min total)
- [ ] **Quick Benchmark** (5 min): `./hack/benchmark/quick-bench.sh`
- [ ] **Load Test** (15 min): `./scripts/benchmark/load-test.sh --count 2000 --duration 2m --rate 16`
- [ ] **Burst Test** (15 min): `./scripts/benchmark/burst-test.sh --burst-size 500 --burst-duration 30s`
- [ ] **Stress Test** (30 min): `./scripts/benchmark/stress-test.sh --phases 3 --phase-duration 10m`
- [ ] **Scale Test** (10 min): `./hack/benchmark/scale-test.sh 10000`

### 3. Documentation (20 min)
- [ ] Create `docs/STRESS_TEST_RESULTS.md` (copy template from instructions)
- [ ] Update KEP performance section with actual results
- [ ] Commit: "docs: add stress testing results and update KEP"

## ✅ Expected Result Ranges

| Test | Duration | Throughput | CPU Impact | Memory Impact |
|------|----------|------------|------------|---------------|
| Quick | 2-4s | 25-50/sec | +15-30m | +10-20MB |
| Load | 2min | 16-17/sec | +20-35m | +20-40MB |
| Burst | 30s | 15-18/sec | +45-75m | +30-55MB |
| Stress | 30min | 18-22/sec avg | +30-55m | +35-60MB |
| Scale | 15min | N/A | Minimal | +22MB storage |

## ✅ Success Criteria

- [ ] All 5 tests complete without errors
- [ ] Zen Watcher remains stable throughout
- [ ] No memory leaks detected
- [ ] Performance within expected ranges
- [ ] Documentation updated with actual results
- [ ] KEP SIG assignment fixed
- [ ] All changes committed to git

## 🚨 If Something Goes Wrong

**Test fails?**
1. Check pod status: `kubectl get pods -n zen-system`
2. Check logs: `kubectl logs -n zen-system -l app.kubernetes.io/name=zen-watcher`
3. Retry the specific failed test
4. Document the failure in your report

**Results different from expected?**
1. That's fine! Use your actual results
2. Update the documentation with real numbers
3. Adjust KEP claims if significantly different

**Questions?**
1. Check the detailed instructions file
2. Verify you're running the right commands
3. Take screenshots of any issues for debugging

---

## 📋 Final Deliverables

1. **Updated KEP** with correct SIG assignment
2. **Stress Testing Report** (`docs/STRESS_TEST_RESULTS.md`)
3. **Updated Performance Claims** in KEP based on real testing
4. **Git commit** with all changes documented

**Total Time Investment**: ~1.5-2 hours
**Key Outcome**: Accurate, tested performance data for KEP submission