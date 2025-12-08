# Zen Watcher Per-Source Auto-Optimization Usage Guide

## Overview

Zen Watcher includes a **per-source auto-optimization system** that continuously learns from cluster patterns and optimizes processing strategies for each source independently. This system dynamically adjusts filtering, deduplication, and processing order to maximize efficiency while maintaining data integrity.

## Features

- **Per-Source Optimization**: Each source (Trivy, Falco, Kyverno, etc.) is optimized independently
- **Dynamic Processing Order**: Selects optimal order based on metrics (filter_first, dedup_first, hybrid, adaptive)
- **Adaptive Filtering**: Adjusts filter thresholds based on event patterns
- **Adaptive Deduplication**: Optimizes deduplication windows based on effectiveness
- **Intelligent Strategy Selection**: Uses metrics-driven decision making with confidence scoring
- **Real-Time Monitoring**: Comprehensive Prometheus metrics for all optimization decisions
- **State Management**: Tracks optimization history and decision reasoning
- **Zero Blast Radius**: All optimizations are safe, tested, and reversible

## CLI Commands

### Analyze Optimization Opportunities

```bash
zen-watcher-optimize --command=analyze --source=trivy
```

Analyzes a source and shows:
- Current configuration
- Optimization suggestions
- Past optimization impact

### Apply a Suggestion

```bash
zen-watcher-optimize --command=apply --source=trivy --suggestion=1
```

Applies a specific suggestion by index.

### Enable Auto-Optimization

```bash
zen-watcher-optimize --command=auto --enable
```

Enables auto-optimization for all sources. Zen Watcher will automatically adjust processing order and filters based on metrics.

### View Optimization History

```bash
zen-watcher-optimize --command=history --source=trivy
```

Shows past optimizations and their impact:
- Number of optimizations applied
- Observations reduced
- Reduction percentage
- CPU savings
- Most effective optimization

### List All Sources

```bash
zen-watcher-optimize --command=list
```

Lists all configured sources with their optimization status:
- Processing order
- Auto-optimize setting
- Filter min priority
- Dedup window

## Configuration

### ObservationSourceConfig CRD

Configure optimization per source:

```yaml
apiVersion: zen.kube-zen.io/v1alpha1
kind: ObservationSourceConfig
metadata:
  name: trivy-config
spec:
  source: trivy
  processing:
    order: auto  # auto, filter_first, or dedup_first
    autoOptimize: true
  filter:
    minPriority: 0.5  # Filter out LOW severity events
  dedup:
    window: 1h
  thresholds:
    observationsPerMinute:
      warning: 100
      critical: 200
    lowSeverityPercent:
      warning: 0.7
      critical: 0.9
    dedupEffectiveness:
      warning: 0.3
      critical: 0.1
```

## Processing Order Logic

Zen Watcher automatically determines optimal processing order:

- **filter_first**: Used when LOW severity > 70% (many events to filter)
- **dedup_first**: Used when dedup effectiveness > 50% (many duplicates)
- **Default**: Source-specific defaults (trivy: filter_first, cert-manager: dedup_first)

## Metrics & Monitoring

### Per-Source Processing Metrics

- `zen_watcher_optimization_source_events_processed_total{source}` - Total events processed per source
- `zen_watcher_optimization_source_events_filtered_total{source}` - Total events filtered per source
- `zen_watcher_optimization_source_events_deduped_total{source}` - Total events deduplicated per source
- `zen_watcher_optimization_source_processing_latency_seconds{source}` - Processing latency histogram (p50, p95, p99)
- `zen_watcher_optimization_filter_effectiveness_ratio{source}` - Filter effectiveness (0.0-1.0)
- `zen_watcher_optimization_deduplication_rate_ratio{source}` - Deduplication rate (0.0-1.0)
- `zen_watcher_optimization_observations_per_minute{source}` - Observations per minute per source

### Optimization Decision Metrics

- `zen_watcher_optimization_decisions_total{source,decision_type,strategy}` - Total optimization decisions
- `zen_watcher_optimization_strategy_changes_total{source,old_strategy,new_strategy}` - Processing strategy changes
- `zen_watcher_optimization_adaptive_adjustments_total{source,adjustment_type}` - Adaptive adjustments applied
- `zen_watcher_optimization_confidence{source}` - Confidence level of optimization decisions (0.0-1.0)

### Legacy Metrics (Still Available)

- `zen_watcher_filter_pass_rate{source}` - Filter effectiveness (0.0-1.0)
- `zen_watcher_dedup_effectiveness{source}` - Dedup effectiveness (0.0-1.0)
- `zen_watcher_low_severity_percent{source}` - Low severity ratio (0.0-1.0)
- `zen_watcher_observations_per_minute{source}` - Observation rate
- `zen_watcher_suggestions_generated_total{source,type}` - Suggestions generated
- `zen_watcher_suggestions_applied_total{source,type}` - Suggestions applied
- `zen_watcher_optimization_impact{source}` - Impact tracking
- `zen_watcher_threshold_exceeded_total{source,threshold}` - Threshold alerts

### Grafana Dashboard

The Grafana dashboard (`config/monitoring/zen-watcher-dashboard.json`) includes optimization panels showing:
- **Events Processing Breakdown**: Processed, filtered, and deduplicated events per source
- **Filter & Dedup Effectiveness**: Real-time effectiveness ratios
- **Processing Latency**: P50, P95, P99 latencies per source
- **Observations per Minute**: Throughput metrics per source
- **Strategy Changes**: Processing strategy changes over time
- **Optimization Confidence**: Confidence levels for optimization decisions

## Alerts

Prometheus alerts are configured in `config/monitoring/optimization-alerts.yaml`:

- High observation rate (>100/min warning, >200/min critical)
- High low severity ratio (>70% warning, >90% critical)
- Low dedup effectiveness (<30% warning, <10% critical)
- Threshold exceeded alerts

## Best Practices

1. **Start with Auto-Optimization**: Enable `autoOptimize: true` and let Zen Watcher learn your patterns
2. **Review Suggestions**: Use `analyze` command to review suggestions before applying
3. **Monitor Metrics**: Watch Prometheus metrics to track optimization effectiveness
4. **Set Thresholds**: Configure thresholds in ObservationSourceConfig to get early warnings
5. **Review Weekly Reports**: Check optimization impact reports to measure success

## Examples

### Example 1: Optimize Trivy (High LOW Severity)

```bash
# Analyze
zen-watcher-optimize --command=analyze --source=trivy

# Output shows: 85% LOW severity, suggests filter.minPriority=0.5
# Apply suggestion
zen-watcher-optimize --command=apply --source=trivy --suggestion=1

# Or configure directly
kubectl patch observationsourceconfig trivy --type=merge -p '{"spec":{"filter":{"minPriority":0.5}}}'
```

### Example 2: Optimize Cert-Manager (Retry Patterns)

```bash
# Cert-manager failures retry frequently
# Zen Watcher automatically switches to dedup_first
# Or configure explicitly:
kubectl patch observationsourceconfig cert-manager --type=merge -p '{"spec":{"processing":{"order":"dedup_first"},"dedup":{"window":"24h"}}}'
```

### Example 3: Enable Auto-Optimization Globally

```bash
zen-watcher-optimize --command=auto --enable
```

This enables auto-optimization for all sources that have `autoOptimize: true` in their config.

## Architecture Components

The per-source optimization system consists of several integrated components that work together to provide intelligent, adaptive optimization:

### Core Components

- **StrategyDecider**: Determines optimal processing order (filter_first, dedup_first, hybrid, adaptive) based on metrics and thresholds
- **AdaptiveFilter**: Adaptive filtering with learning capabilities, dynamic rules, and effectiveness tracking
- **PerSourceMetricsCollector**: Per-source metrics collection with Prometheus integration, sliding window aggregation, and real-time KPIs
- **PerformanceTracker**: Performance tracking per source including latency (p50, p95, p99), throughput, filter hit rates, and resource utilization
- **SmartProcessor**: Main orchestrator that coordinates all optimization components and applies strategies
- **OptimizationStateManager**: Tracks optimization state, decision history, performance data, and active rules per source
- **OptimizationEngine**: Continuous optimization loop that runs periodic analysis, evaluates metrics, and applies optimizations
- **AdaptiveProcessor**: Per-source adaptive processing adjustments for filters, deduplication, and rate limiting
- **Optimizer**: Unified coordinator that ties all components together and manages the optimization lifecycle

### Processing Strategies

The system supports four processing strategies, automatically selected based on source characteristics:

1. **filter_first**: Filter → Normalize → Dedup → Create
   - **Best for**: High-volume sources with many LOW severity events (>70%)
   - **Benefit**: Removes noise early, reduces dedup cache pressure
   - **Use case**: Trivy with high LOW severity rate

2. **dedup_first**: Dedup → Filter → Normalize → Create
   - **Best for**: Sources with high deduplication effectiveness (>50%)
   - **Benefit**: Removes duplicates early, reduces filter processing load
   - **Use case**: cert-manager with retry patterns

3. **hybrid**: Dynamic combination based on event characteristics
   - **Best for**: Variable workload patterns, mixed event types
   - **Benefit**: Optimal balance for complex sources
   - **Use case**: Falco with varying event patterns

4. **adaptive**: Machine learning-based dynamic adaptation
   - **Best for**: Very high volume, complex workloads requiring continuous learning
   - **Benefit**: Continuous learning and optimization without manual tuning
   - **Use case**: Large-scale deployments with multiple high-volume sources

### Optimization Loop

The system runs continuous optimization cycles:

1. **Metrics Collection** (continuous)
   - Per-source events processed, filtered, deduplicated
   - Processing latency (p50, p95, p99)
   - Filter and deduplication effectiveness ratios
   - Event patterns (low severity percentage, throughput)
   - Resource usage (CPU, memory per source)

2. **Analysis** (every 5 minutes by default, configurable via `analysisInterval`)
   - Evaluate metrics against configured thresholds
   - Calculate confidence scores based on data volume and consistency
   - Identify optimization opportunities
   - Compare current performance to historical data

3. **Decision Making** (when thresholds exceeded or auto-optimize enabled)
   - Determine optimal strategy using StrategyDecider
   - Calculate confidence level (requires minimum threshold: `confidenceThreshold`)
   - Generate decision with reasoning and expected impact
   - Store decision in OptimizationStateManager

4. **Application** (if confidence > `confidenceThreshold`)
   - Apply strategy changes via SmartProcessor
   - Adjust adaptive parameters (filter thresholds, dedup windows)
   - Update rate limits based on targets
   - Record decision in state manager with timestamp

5. **Monitoring** (continuous)
   - Track optimization effectiveness
   - Measure impact metrics (latency improvement, resource savings)
   - Export comprehensive metrics to Prometheus
   - Alert on threshold exceedances

## Troubleshooting

### No Suggestions Generated

- Check that metrics are being collected
- Verify Prometheus is accessible
- Ensure source has sufficient data (at least 100 observations)

### Auto-Optimization Not Working

- Verify `autoOptimize: true` in ObservationSourceConfig
- Check that processing order is set to "auto"
- Review logs for optimization advisor messages

### High Observation Rate

- Review filter rules
- Consider increasing dedup window
- Check for source-specific issues (e.g., misconfigured scanner)

