# Optimization Dashboard Updates

## Overview

This document outlines the Grafana dashboard updates needed to display optimization insights and metrics.

## New Panels to Add

### 1. Optimization Insights Panel

**Location**: New panel in Executive Dashboard

**Metrics**:
- `zen_watcher_suggestions_generated_total{source,type}` - Suggestions by source and type
- `zen_watcher_suggestions_applied_total{source,type}` - Applied suggestions
- `zen_watcher_optimization_impact{source}` - Impact percentage

**Visualization**: Table or Stat panel showing:
- Current optimization opportunities
- Impact of past optimizations
- Resource savings (estimated)

### 2. Source Efficiency Dashboard

**Location**: New section in Operations Dashboard

**Panels**:

#### Filter Effectiveness Panel
- **Query**: `zen_watcher_filter_pass_rate{source=~"$source"}`
- **Type**: Gauge or Time Series
- **Description**: Shows filter pass rate per source (0.0-1.0)

#### Dedup Effectiveness Panel
- **Query**: `zen_watcher_dedup_effectiveness{source=~"$source"}`
- **Type**: Gauge or Time Series
- **Description**: Shows dedup effectiveness per source (0.0-1.0)

#### Observation Rate vs Thresholds
- **Query**: 
  ```
  zen_watcher_observations_per_minute{source=~"$source"}
  ```
- **Type**: Time Series with thresholds
- **Description**: Shows observation rate with warning (100/min) and critical (200/min) thresholds

#### Low Severity Ratio
- **Query**: `zen_watcher_low_severity_percent{source=~"$source"}`
- **Type**: Gauge
- **Description**: Shows percentage of LOW severity observations

### 3. Auto-Optimization Status Panel

**Location**: New panel in Executive Dashboard

**Metrics**:
- Processing order per source (from Ingester)
- Auto-optimization enabled status
- Last optimization timestamp

**Visualization**: Table showing:
- Currently applied optimizations
- Next scheduled analysis
- Total savings (observations reduced)

### 4. Threshold Alerts Panel

**Location**: New panel in Operations Dashboard

**Metrics**:
- `zen_watcher_threshold_exceeded_total{source,threshold,severity}`

**Visualization**: Alert list or table showing:
- Sources with exceeded thresholds
- Threshold type (observation_rate, low_severity, dedup_effectiveness)
- Severity (warning, critical)

## Dashboard JSON Updates

### Executive Dashboard (`zen-watcher-executive.json`)

Add new row with:
1. **Optimization Insights Panel**
   - Current opportunities
   - Past optimizations impact
   - Resource savings

2. **Auto-Optimization Status Panel**
   - Enabled sources
   - Current processing orders
   - Last optimization time

### Operations Dashboard (`zen-watcher-operations.json`)

Add new row "Source Efficiency" with:
1. **Filter Effectiveness** (Gauge)
2. **Dedup Effectiveness** (Gauge)
3. **Observation Rate** (Time Series with thresholds)
4. **Low Severity Ratio** (Gauge)
5. **Threshold Alerts** (Alert list)

## Prometheus Queries

### Optimization Opportunities
```promql
# High low severity ratio
zen_watcher_low_severity_percent > 0.7

# Low dedup effectiveness
zen_watcher_dedup_effectiveness < 0.3

# High observation rate
zen_watcher_observations_per_minute > 100
```

### Optimization Impact
```promql
# Total observations reduced
sum(zen_watcher_optimization_impact{source=~".+"}) * 100

# Sources optimized
count(zen_watcher_suggestions_applied_total > 0)

# Average reduction
avg(zen_watcher_optimization_impact{source=~".+"})
```

### Processing Order Status
```promql
# This would require a custom metric or annotation
# Could be derived from Ingester CRD status
```

## Implementation Notes

1. **Variables**: Add `$source` variable to all optimization panels
2. **Thresholds**: Use Grafana threshold configuration for visual alerts
3. **Annotations**: Add annotations for optimization events
4. **Refresh**: Set appropriate refresh intervals (30s-1m)

## Example Panel Configuration

### Filter Effectiveness Gauge

```json
{
  "title": "Filter Effectiveness",
  "type": "gauge",
  "targets": [{
    "expr": "zen_watcher_filter_pass_rate{source=~\"$source\"}",
    "legendFormat": "{{source}}"
  }],
  "fieldConfig": {
    "defaults": {
      "min": 0,
      "max": 1,
      "thresholds": {
        "mode": "absolute",
        "steps": [
          {"value": 0, "color": "red"},
          {"value": 0.5, "color": "yellow"},
          {"value": 0.8, "color": "green"}
        ]
      },
      "unit": "percentunit"
    }
  }
}
```

## Next Steps

1. Update `zen-watcher-executive.json` with optimization insights panel
2. Update `zen-watcher-operations.json` with source efficiency section
3. Add optimization variables to dashboard templates
4. Test with real metrics data
5. Document in README

