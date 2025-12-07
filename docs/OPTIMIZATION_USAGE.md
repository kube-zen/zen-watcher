# Zen Watcher Auto-Optimization Usage Guide

## Overview

Zen Watcher includes an intelligent auto-optimization system that learns from your cluster patterns and provides actionable optimization suggestions.

## Features

- **Self-Learning**: Analyzes metrics to find optimization opportunities
- **Dynamic Processing Order**: Automatically adjusts processing order (filter_first vs dedup_first)
- **Actionable Suggestions**: Provides kubectl commands for easy application
- **Real-Time Monitoring**: Tracks effectiveness and alerts on thresholds
- **Professional Logging**: Clean, structured logs

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

## Metrics

Zen Watcher exposes optimization metrics:

- `zen_watcher_filter_pass_rate{source}` - Filter effectiveness (0.0-1.0)
- `zen_watcher_dedup_effectiveness{source}` - Dedup effectiveness (0.0-1.0)
- `zen_watcher_low_severity_percent{source}` - Low severity ratio (0.0-1.0)
- `zen_watcher_observations_per_minute{source}` - Observation rate
- `zen_watcher_suggestions_generated_total{source,type}` - Suggestions generated
- `zen_watcher_suggestions_applied_total{source,type}` - Suggestions applied
- `zen_watcher_optimization_impact{source}` - Impact tracking
- `zen_watcher_threshold_exceeded_total{source,threshold}` - Threshold alerts

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

