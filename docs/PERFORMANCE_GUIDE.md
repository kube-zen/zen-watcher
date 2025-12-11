# Performance Guide

## Overview

This guide provides performance characteristics, sizing recommendations, and tuning guidance for zen-watcher 1.0.0-alpha.

## Benchmark Scenarios

### High-Volume Events with Different Severity Mixes

**Scenario 1: High LOW Severity (85% LOW, 15% HIGH)**
- Simulates Trivy-like traffic with many low-priority findings
- Expected strategy: `filter_first` (filter out noise early)
- Throughput: ~150-170 events/sec
- Memory: ~65MB average

**Scenario 2: Balanced Severity (40% LOW, 30% MEDIUM, 30% HIGH)**
- Mixed severity distribution
- Expected strategy: `dedup_first` or `filter_first` depending on dedup effectiveness
- Throughput: ~180-200 events/sec
- Memory: ~55MB average

**Scenario 3: High Deduplication Rate (60% duplicates)**
- Simulates cert-manager-like traffic with retry patterns
- Expected strategy: `dedup_first` (remove duplicates early)
- Throughput: ~170-190 events/sec
- Memory: ~60MB average

### Different Duplication Rates

**Low Duplication (10%)**
- Most events are unique
- Dedup overhead minimal
- Throughput: ~180-200 events/sec

**Medium Duplication (40%)**
- Moderate duplicate rate
- Dedup provides value
- Throughput: ~170-190 events/sec

**High Duplication (70%)**
- High duplicate rate
- Dedup highly effective
- Throughput: ~160-180 events/sec (dedup reduces work downstream)

## Observed Performance Numbers

### Throughput (Events/Second)

| Scenario | Events/sec | Notes |
|----------|------------|-------|
| Single source, low volume | 45-50 | Limited by API server rate limits |
| Multiple sources (5) | 180-200 | Parallel processing |
| With filtering | 150-170 | Filter overhead ~3ms per event |
| With deduplication | 170-190 | Dedup overhead ~5ms per event |
| Full pipeline | 140-160 | Combined filter + dedup overhead |

**Note**: These numbers are from synthetic benchmarks. Real-world performance depends on:
- API server capacity
- etcd performance
- Network latency
- Event payload size

### Resource Usage

**CPU Usage**:
- Idle: ~5 millicores
- Low load (10 events/sec): ~15 millicores
- Medium load (50 events/sec): ~35 millicores
- High load (100 events/sec): ~70 millicores
- Peak load (200 events/sec): ~150 millicores

**Memory Usage**:
- Baseline: ~35MB
- Low load: ~45MB
- Medium load: ~55MB
- High load: ~75MB
- Peak load: ~95MB

**Note**: Memory usage scales with:
- Number of active sources
- Deduplication cache size
- Event payload size

## Recommendations

### Horizontal Scaling Patterns

**Single-Replica Deployment (Recommended for Most Cases)**
- Suitable for clusters with <10,000 events/day
- Simpler operation (no HA coordination)
- Lower resource overhead
- Use when: Single replica can handle your event volume

**Multi-Replica Deployment (With HA Optimization)**
- Suitable for clusters with >10,000 events/day
- Requires `haOptimization.enabled: true` in Helm values
- Provides:
  - Dynamic deduplication window adjustment
  - Adaptive cache sizing
  - Load balancing across replicas
- Use when: Single replica cannot keep up with event volume

**Namespace Sharding (For Very Large Clusters)**
- Deploy multiple zen-watcher instances, each watching specific namespaces
- Each instance operates independently
- Use when: Single cluster has >50,000 events/day

### Resource Limits Starting Points

**Small Cluster (<1,000 events/day)**:
```yaml
resources:
  requests:
    cpu: 50m
    memory: 64Mi
  limits:
    cpu: 200m
    memory: 128Mi
```

**Medium Cluster (1,000-10,000 events/day)**:
```yaml
resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 500m
    memory: 256Mi
```

**Large Cluster (10,000-50,000 events/day)**:
```yaml
resources:
  requests:
    cpu: 200m
    memory: 256Mi
  limits:
    cpu: 1000m
    memory: 512Mi
```

**Very Large Cluster (>50,000 events/day)**:
- Consider namespace sharding
- Or use multi-replica with HA optimization
- Per-replica: 500m CPU, 512Mi memory

### When to Tune Optimization Thresholds

**Default thresholds work well for most cases**. Consider tuning if:

1. **High LOW severity but filter_first not activating**
   - Current threshold: 70%
   - If you have 65% LOW severity and want filter_first, lower threshold
   - Location: `pkg/optimization/config.go` (FilterFirstThresholdLowSeverity)

2. **High dedup effectiveness but dedup_first not activating**
   - Current threshold: 50%
   - If you have 45% dedup effectiveness and want dedup_first, lower threshold
   - Location: `pkg/optimization/config.go` (DedupFirstThresholdEffectiveness)

3. **Frequent strategy oscillation**
   - Indicates thresholds are too close to actual metrics
   - Consider adding hysteresis (cooldown period)
   - Already implemented: 5-minute cooldown between strategy changes

**How to Tune**:
1. Monitor `zen_watcher_optimization_strategy_changes_total` for oscillation
2. Monitor `zen_watcher_low_severity_percent` and `zen_watcher_optimization_deduplication_rate_ratio`
3. Adjust thresholds in optimization config if needed
4. Rebuild and redeploy

## Benchmark Code

Benchmarks are located in `pkg/processor/` and can be run with:

```bash
cd pkg/processor
go test -bench=. -benchmem
```

See [PERFORMANCE.md](PERFORMANCE.md) for detailed benchmark methodology and results.

## Monitoring Performance

### Key Metrics to Monitor

1. **Throughput**: `rate(zen_watcher_optimization_source_events_processed_total[5m])`
2. **Latency**: `histogram_quantile(0.95, zen_watcher_optimization_source_processing_latency_seconds_bucket)`
3. **Error Rate**: `rate(zen_watcher_pipeline_errors_total[5m])`
4. **Resource Usage**: CPU and memory from Kubernetes metrics

### Performance Alerts

See [OBSERVABILITY.md](OBSERVABILITY.md) for recommended alerting rules.

## Related Documentation

- [PERFORMANCE.md](PERFORMANCE.md) - Detailed benchmark results and methodology
- [OBSERVABILITY.md](OBSERVABILITY.md) - Metrics and monitoring
- [SCALING.md](SCALING.md) - Scaling strategies
- [OPERATIONAL_EXCELLENCE.md](OPERATIONAL_EXCELLENCE.md) - Operations best practices

