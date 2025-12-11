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

# Zen Watcher KEP & Stress Testing Team Instructions

## Quick Summary
- **KEP Status**: Keep as draft but fix sig-foo assignment
- **Task**: Run comprehensive stress tests and add results to documentation
- **Deliverable**: Complete stress testing report with validated performance data

---

## PART 1: KEP Documentation Fixes (5 minutes)

### File: `keps/sig-foo/0000-zen-watcher/README.md`

**Find and replace these lines:**

**Line 6 - Change this:**
```yaml
owning-sig: sig-foo  # TODO: Update to appropriate SIG (sig-security, sig-observability, etc.)
```

**To this:**
```yaml
owning-sig: sig-observability  # Primary SIG for observability infrastructure
```

**Lines 16-17 - Change this:**
```yaml
creation-date: 2024-11-27
last-updated: 2024-12-04
```

**To this:**
```yaml
creation-date: 2024-11-27
last-updated: 2025-12-08
```

**Commit these changes:**
```bash
git add keps/sig-foo/0000-zen-watcher/README.md
git commit -m "docs: fix KEP SIG assignment to sig-observability and update dates"
```

---

## PART 2: Stress Testing Execution (60-90 minutes total)

### Prerequisites Check
```bash
# Verify zen-watcher is running
kubectl get pods -n zen-system | grep zen-watcher

# Verify metrics server is available (for resource monitoring)
kubectl top pods -n zen-system --help

# Check if you have necessary tools
which kubectl bc jq
```

### Test Execution Order

**IMPORTANT**: Run tests in this exact order. Each test builds on the previous one.

---

## TEST 1: Quick Benchmark (5 minutes)

**Purpose**: Verify basic functionality and get baseline metrics

**Command:**
```bash
./hack/benchmark/quick-bench.sh
```

**Expected Results:**
```
=== Quick Benchmark: 100 Observations ===
Namespace: zen-system
Using pod: zen-watcher-xxxxx

Observations created: 100 (expected: 100)
Duration: 2-4s
Throughput: 25-50 obs/sec
CPU: 2m → 15-30m
Memory: 35MB → 45-55MB
```

**Document these results in your report.**

---

## TEST 2: Load Testing (15-20 minutes)

**Purpose**: Test sustained performance under consistent load

**Run this command:**
```bash
./scripts/benchmark/load-test.sh --count 2000 --duration 2m --rate 16
```

**Expected Results:**
```
=== Load Test Configuration ===
Namespace: zen-system
Target observations: 2000
Duration: 2m (120 seconds)
Rate: 16 obs/sec
Source: load-test

Expected average throughput: 16.67 obs/sec

=== Load Test Results ===
Load test completed at [timestamp]
Observations created: 2000 (target: 2000)
Actual duration: 118-122s
Actual throughput: 16.5-17.0 obs/sec (target: 16 obs/sec)

=== Resource Impact ===
CPU: 2m → 25-40m (Δ20-35m)
Memory: 35MB → 55-75MB (Δ20-40MB)
Total observations: [baseline] → [baseline+2000]

=== Performance Assessment ===
✅ Performance: GOOD (achieved ≥80% of target throughput)
```

**Document these results in your report.**

---

## TEST 3: Burst Testing (15-20 minutes)

**Purpose**: Test peak capacity and recovery behavior

**Run this command:**
```bash
./scripts/benchmark/burst-test.sh --burst-size 500 --burst-duration 30s --recovery-time 60s
```

**Expected Results:**
```
=== Burst Test Configuration ===
Burst size: 500 observations
Burst duration: 30s (30 seconds)
Recovery monitoring: 60s

Burst rate: 16.67 obs/sec

=== Burst Phase Results ===
Burst completed at [timestamp]
Observations created: 500 (target: 500)
Burst duration: 28-32s
Burst rate: 15-18 obs/sec

=== Peak Resource Usage ===
CPU: 2m → 50-80m (Δ45-75m)
Memory: 35MB → 65-90MB (Δ30-55MB)

=== Recovery Analysis ===
CPU recovery: 30-40m (60-80% of peak increase)
Memory recovery: 20-30MB (50-70% of peak increase)

=== Performance Assessment ===
✅ Burst Capacity: GOOD (≥50 obs/sec)
✅ CPU Recovery: GOOD (≥60% recovery)
```

**Document these results in your report.**

---

## TEST 4: Stress Testing (30-45 minutes)

**Purpose**: Multi-phase testing with progressive load increase

**Run this command:**
```bash
./scripts/benchmark/stress-test.sh --phases 3 --phase-duration 10m --max-observations 5000
```

**Expected Results:**
```
=== Stress Test Configuration ===
Phases: 3
Phase duration: 10m (600 seconds each)
Max observations: 5000
Sample interval: 30s

=== Phase-by-Phase Results ===
Phase 1: 8-12 obs/sec, CPU +10-25m, Memory +15-30MB
Phase 2: 18-22 obs/sec, CPU +25-45m, Memory +30-50MB  
Phase 3: 28-32 obs/sec, CPU +40-70m, Memory +45-70MB

=== Final Resource Impact ===
CPU: 2m → 35-60m (Δ30-55m)
Memory: 35MB → 70-95MB (Δ35-60MB)
Total observations: [baseline] → [baseline+5000]

=== Performance Analysis ===
Overall throughput statistics:
  Average rate: 18-22 obs/sec
  Peak rate: 28-32 obs/sec
  Minimum rate: 8-12 obs/sec

=== Resource Analysis ===
✅ CPU Impact: LOW-MODERATE (≤100m increase)
✅ Memory Impact: LOW-MODERATE (≤100MB increase)

=== Performance Rating ===
✅ Overall Rating: GOOD (avg ≥20 obs/sec)
```

**Document these results in your report.**

---

## TEST 5: Scale Testing (10-15 minutes)

**Purpose**: Test with large number of observation objects

**Run this command:**
```bash
./hack/benchmark/scale-test.sh 10000
```

**Expected Results:**
```
=== Scale Test: 10000 Observations ===
Creating 10000 observations in 20 batches of 500...

=== Scale Test Results ===
Observations created: 10000
Duration: 15-25 minutes

Total observations in namespace: [previous+10000]

=== Recommendations ===
- Use --chunk-size=500 for large-scale list operations
- Monitor etcd storage usage
- Consider TTL for automatic cleanup
```

**Document these results in your report.**

---

## PART 3: Documentation Updates (20 minutes)

### Create Stress Testing Report

**Create new file**: `docs/STRESS_TEST_RESULTS.md`

**Add this content** (replace with your actual results):

```markdown
# Stress Testing Results

## Test Environment

- **Kubernetes Version**: [your version]
- **Cluster**: [your cluster specs]
- **Node Specs**: [your node specs]
- **Test Date**: $(date '+%Y-%m-%d')
- **Test Duration**: ~90 minutes total

## Test Results Summary

### Quick Benchmark
- **Observations**: 100
- **Throughput**: 25-50 obs/sec
- **Duration**: 2-4 seconds
- **CPU Impact**: +15-30m
- **Memory Impact**: +10-20MB
- **Status**: ✅ PASS

### Load Testing
- **Observations**: 2000
- **Duration**: 2 minutes
- **Sustained Rate**: 16-17 obs/sec
- **CPU Impact**: +20-35m
- **Memory Impact**: +20-40MB
- **Status**: ✅ PASS

### Burst Testing
- **Observations**: 500
- **Burst Rate**: 15-18 obs/sec
- **Peak CPU**: +45-75m
- **Peak Memory**: +30-55MB
- **CPU Recovery**: 60-80%
- **Memory Recovery**: 50-70%
- **Status**: ✅ PASS

### Stress Testing
- **Observations**: 5000
- **Phases**: 3 (progressive load)
- **Average Rate**: 18-22 obs/sec
- **Peak Rate**: 28-32 obs/sec
- **CPU Impact**: +30-55m
- **Memory Impact**: +35-60MB
- **Status**: ✅ PASS

### Scale Testing
- **Observations**: 10000
- **Duration**: 15-25 minutes
- **etcd Storage**: ~22MB
- **List Performance**: 2-4 seconds
- **Status**: ✅ PASS

## Key Findings

### Performance Characteristics
- **Sustained Throughput**: 16-22 obs/sec
- **Burst Capacity**: 15-18 obs/sec
- **Peak Resource Usage**: CPU +80m, Memory +60MB
- **Recovery Time**: <60 seconds

### Resource Impact
- **CPU Usage**: Linear increase with load
- **Memory Usage**: Stable with no leaks detected
- **etcd Impact**: Minimal (~2.2KB per observation)

### Scaling Recommendations
- **Low Traffic** (<100 events/day): Default config sufficient
- **Medium Traffic** (100-1000 events/day): Enable filtering
- **High Traffic** (>1000 events/day): Use namespace sharding

## Validation Against KEP Claims

| Metric | KEP Claim | Test Result | Status |
|--------|-----------|-------------|--------|
| Sustained Throughput | 45-50 obs/sec | 16-22 obs/sec | ⚠️ Below claim |
| Burst Capacity | 500 obs/30sec | 500 obs/30sec | ✅ Validated |
| Memory Usage | ~35MB baseline | 35-40MB | ✅ Validated |
| CPU Usage | 2m baseline | 2-5m | ✅ Validated |
| 20k Object Impact | +5m CPU, +10MB | Similar scale | ✅ Validated |

## Recommendations

### For KEP Update
1. **Adjust sustained throughput claim** to 16-22 obs/sec (more realistic)
2. **Validate burst testing claims** are accurate
3. **Add stress testing results** to performance section

### For Production Deployment
1. **Resource Limits**: 100m CPU, 128MB memory minimum
2. **Monitoring**: Set alerts for CPU >50m, Memory >100MB
3. **Scaling**: Use namespace sharding for high-traffic clusters

## Conclusion

Zen Watcher demonstrates **stable performance** under sustained and burst loads. While sustained throughput is lower than initially claimed, the system shows excellent stability and predictable resource usage. Suitable for production deployment with appropriate resource limits.
```

### Update KEP Performance Section

**Add this to the KEP performance section** (after the existing table):

```markdown
## Stress Testing Validation (2025-12-08)

Comprehensive stress testing was performed to validate performance claims:

### Test Results
- **Sustained Load**: 16-22 obs/sec (validated with 2000 observations)
- **Burst Capacity**: 15-18 obs/sec (validated with 500 observations)
- **Multi-Phase Stress**: 18-22 avg, 28-32 peak obs/sec (validated with 5000 observations)
- **Scale Testing**: 10,000 observations processed successfully

### Updated Performance Claims
- **Sustained Throughput**: 16-22 obs/sec (more than initial 45 conservative-50)
- **Burst Capacity**: 15-18 obs/sec (validated)
- **Resource Impact**: Predictable linear scaling
- **Recovery**: <60 seconds for burst loads

See [Stress Testing Results](../docs/STRESS_TEST_RESULTS.md) for complete details.
```

---

## PART 4: Final Verification

### Run Quick Verification
```bash
# Verify all tests completed successfully
echo "=== Final Status Check ==="
kubectl get pods -n zen-system | grep zen-watcher
echo "✅ zen-watcher running"

# Check total observations created
TOTAL_OBS=$(kubectl get observations -n zen-system --no-headers | wc -l)
echo "📊 Total observations: $TOTAL_OBS"

# Verify no errors in logs
kubectl logs -n zen-system -l app.kubernetes.io/name=zen-watcher --tail=50 | grep -i error || echo "✅ No errors found"
```

### Commit Everything
```bash
# Add all changes
git add docs/STRESS_TEST_RESULTS.md
git add keps/sig-foo/0000-zen-watcher/README.md

# Commit with descriptive message
git commit -m "docs: add comprehensive stress testing results and fix KEP SIG assignment

- Add stress testing report with validated performance data
- Update KEP SIG assignment to sig-observability
- Include test results from 5 comprehensive test scenarios
- Update performance claims based on actual testing
- Document resource usage patterns and scaling recommendations"
```

---

## Expected Timeline

- **Part 1**: 5 minutes (KEP fixes)
- **Part 2**: 60-90 minutes (stress testing)
- **Part 3**: 20 minutes (documentation)
- **Total**: ~1.5-2 hours

## Troubleshooting

### If tests fail:
1. Check zen-watcher pod status: `kubectl get pods -n zen-system`
2. Check logs: `kubectl logs -n zen-system -l app.kubernetes.io/name=zen-watcher`
3. Verify metrics server: `kubectl top pods -n zen-system`
4. Retry failed tests

### If documentation needs adjustment:
1. Update numbers in STRESS_TEST_RESULTS.md with your actual results
2. Adjust KEP performance claims if significantly different
3. Keep commit messages descriptive

---

## Questions?

Run into issues? Check the actual test outputs and adapt the documentation accordingly. The key is getting **real performance data** to validate (or update) the KEP claims.

**Remember**: The goal is to have **accurate, tested performance data** rather than theoretical claims.