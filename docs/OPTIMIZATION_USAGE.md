# Zen Watcher Processing Order Configuration Guide

## Overview

Zen Watcher supports configurable processing order to optimize performance based on your workload patterns. You can choose between `filter_first` or `dedup_first` modes.

**Note:** Auto-optimization has been removed. Processing order must be configured manually via the Ingester CRD.

## Processing Order Modes

Zen Watcher supports two processing order modes:

| Mode | Description | When to Use |
|------|-------------|-------------|
| **filter_first** | Filter → Normalize → Dedup → Create | High LOW severity (>70%), many events to filter |
| **dedup_first** | Dedup → Filter → Normalize → Create | High duplicate rate (>50%), retry patterns |

## Configuration

### Ingester CRD

Configure processing order per source:

```yaml
apiVersion: zen.kube-zen.io/v1alpha1
kind: Ingester
metadata:
  name: trivy-config
spec:
  source: trivy
  processing:
    order: filter_first  # filter_first or dedup_first
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

## Choosing the Right Processing Order

### Use `filter_first` when:
- LOW severity events > 70% of total
- You want to reduce noise early
- Filter operations are cheaper than dedup

**Example:** Trivy scanning with many LOW severity vulnerabilities

### Use `dedup_first` when:
- Deduplication effectiveness > 50%
- You have retry patterns (e.g., cert-manager)
- Dedup operations are cheaper than filter

**Example:** cert-manager with frequent retry patterns


## Metrics & Monitoring

### Key Metrics

Monitor these metrics to guide your processing order selection:

- `zen_watcher_filter_pass_rate{source}` - Filter effectiveness (0.0-1.0)
- `zen_watcher_dedup_effectiveness{source}` - Dedup effectiveness (0.0-1.0)
- `zen_watcher_low_severity_percent{source}` - Low severity ratio (0.0-1.0)
- `zen_watcher_observations_per_minute{source}` - Observation rate

### Decision Guidelines

1. **If `zen_watcher_low_severity_percent` > 0.7**: Use `filter_first`
2. **If `zen_watcher_dedup_effectiveness` > 0.5**: Use `dedup_first`
3. **Otherwise**: Use `filter_first` (default)

## Examples

### Example 1: High LOW Severity (Trivy)

```yaml
spec:
  source: trivy
  processing:
    order: filter_first  # Filter early to reduce noise
  filter:
    minPriority: 0.5
```

### Example 2: High Duplicate Rate (cert-manager)

```yaml
spec:
  source: cert-manager
  processing:
    order: dedup_first  # Dedupe early for retry patterns
  dedup:
    window: 24h
```


## Best Practices

1. **Monitor Metrics First**: Use Prometheus metrics to understand your workload patterns
2. **Start with filter_first**: Default safe choice for most workloads
3. **Adjust Based on Metrics**: Switch to `dedup_first` if dedup effectiveness is high
4. **Use Thresholds**: Configure thresholds to get early warnings about performance issues
5. **Review Periodically**: Re-evaluate processing order as workload patterns change

## Troubleshooting

### High Observation Rate

- Review filter rules
- Consider switching to `filter_first` if LOW severity is high
- Increase dedup window if duplicates are common

### High CPU Usage

- Check if processing order matches workload patterns
- Consider switching order based on metrics
- Review filter and dedup effectiveness

### Low Dedup Effectiveness

- Consider switching to `filter_first`
- Review dedup window size
- Check if dedup strategy matches your event patterns
