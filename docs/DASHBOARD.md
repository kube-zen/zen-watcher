# Grafana Dashboard Guide

## Overview

This guide covers all Grafana dashboards for `zen-watcher`, including dashboard review, optimization updates, and recommendations.

## Available Dashboards

1. `config/dashboards/zen-watcher-executive.json` - Executive overview dashboard
2. `config/dashboards/zen-watcher-operations.json` - Operations monitoring dashboard
3. `config/dashboards/zen-watcher-security.json` - Security analytics dashboard
4. `config/dashboards/zen-watcher-dashboard.json` - Main unified dashboard
5. `config/dashboards/zen-watcher-namespace-health.json` - Namespace health dashboard
6. `config/dashboards/zen-watcher-explorer.json` - Data exploration dashboard

## Dashboard Review

### Critical Issues Fixed

#### Severity Value Mismatches (32+ instances fixed)

**Problem**: Dashboards used uppercase severity values (`CRITICAL`, `HIGH`, `MEDIUM`, `LOW`, `FAIL`) but metrics use lowercase (`critical`, `high`, `medium`, `low`, `fail`).

**Affected Dashboards**:
- `zen-watcher-executive.json`: 8 instances
- `zen-watcher-security.json`: 8 instances
- `zen-watcher-namespace-health.json`: 7 instances
- `zen-watcher-dashboard.json`: 4 instances
- `zen-watcher-explorer.json`: 3 instances
- `zen-watcher-operations.json`: 2 instances

**Fix Applied**:
- `severity="CRITICAL"` → `severity="critical"`
- `severity="HIGH"` → `severity="high"`
- `severity="MEDIUM"` → `severity="medium"`
- `severity="LOW"` → `severity="low"`
- `severity=~"CRITICAL|HIGH"` → `severity=~"critical|high"`
- `severity=~"FAIL|CRITICAL"` → `severity=~"fail|critical"`

**Impact**: Dashboard queries now correctly match metric labels and display data.

### Gaps Identified

The following new metrics are not yet represented in dashboards:

1. **Ingester Lifecycle Metrics** (11 metrics)
   - `zen_watcher_ingesters_active`
   - `zen_watcher_ingesters_status`
   - `zen_watcher_ingesters_config_errors_total`
   - `zen_watcher_ingesters_startup_duration_seconds`
   - `zen_watcher_ingesters_last_event_timestamp_seconds`
   - `zen_watcher_ingester_events_processed_total`
   - `zen_watcher_ingester_events_processed_rate`
   - `zen_watcher_ingester_processing_latency_seconds`
   - `zen_watcher_ingester_errors_total`
   - `zen_watcher_informer_cache_sync_duration_seconds`
   - `zen_watcher_informer_resync_events_total`

2. **Destination Delivery Metrics** (4 metrics)
   - `zen_watcher_destination_delivery_total`
   - `zen_watcher_destination_delivery_latency_seconds`
   - `zen_watcher_destination_queue_depth`
   - `zen_watcher_destination_retries_total`

3. **ConfigManager Metrics** (5 metrics)
   - `zen_watcher_configmap_load_total`
   - `zen_watcher_configmap_reload_duration_seconds`
   - `zen_watcher_configmap_merge_conflicts_total`
   - `zen_watcher_configmap_validation_errors_total`
   - `zen_watcher_config_update_propagation_duration_seconds`

4. **Filter Rule Evaluation Metrics** (1 metric)
   - `zen_watcher_filter_rule_evaluation_duration_seconds`

## Optimization Dashboard Updates

### New Panels to Add

#### 1. Optimization Insights Panel

**Location**: New panel in Executive Dashboard

**Metrics**:
- `zen_watcher_suggestions_generated_total{source,type}` - Suggestions by source and type
- `zen_watcher_suggestions_applied_total{source,type}` - Applied suggestions
- `zen_watcher_optimization_impact{source}` - Impact percentage

**Visualization**: Table or Stat panel showing:
- Current optimization opportunities
- Impact of past optimizations
- Resource savings (estimated)

#### 2. Source Efficiency Dashboard

**Location**: New section in Operations Dashboard

**Panels**:

##### Filter Effectiveness Panel
- **Query**: `zen_watcher_filter_pass_rate{source=~"$source"}`
- **Type**: Gauge or Time Series
- **Description**: Shows filter pass rate per source (0.0-1.0)

##### Dedup Effectiveness Panel
- **Query**: `zen_watcher_dedup_effectiveness{source=~"$source"}`
- **Type**: Gauge or Time Series
- **Description**: Shows dedup effectiveness per source (0.0-1.0)

##### Observation Rate vs Thresholds
- **Query**: `zen_watcher_observations_per_minute{source=~"$source"}`
- **Type**: Time Series with thresholds
- **Description**: Shows observation rate with warning (100/min) and critical (200/min) thresholds

##### Low Severity Ratio
- **Query**: `zen_watcher_low_severity_percent{source=~"$source"}`
- **Type**: Gauge
- **Description**: Shows percentage of LOW severity observations

#### 3. Processing Order Status Panel

**Location**: New panel in Executive Dashboard

**Metrics**:
- Processing order per source (from Ingester)
- Current strategy (filter_first, dedup_first)

**Visualization**: Table showing:
- Current processing order per source
- Strategy performance metrics

#### 4. Threshold Alerts Panel

**Location**: New panel in Operations Dashboard

**Metrics**:
- `zen_watcher_threshold_exceeded_total{source,threshold,severity}`

**Visualization**: Alert list or table showing:
- Sources with exceeded thresholds
- Threshold type (observation_rate, low_severity, dedup_effectiveness)
- Severity (warning, critical)

## Dashboard Recommendations

### Priority 1: Add New Metrics to Operations Dashboard

Add panels for:
- **Ingester Health Section**: Status, errors, processing rate, latency
- **Destination Delivery Section**: Success/failure rates, latency, queue depth
- **ConfigManager Section**: Load failures, reload duration, validation errors

### Priority 2: Enhance Executive Dashboard

Add high-level metrics:
- Active Ingesters count
- Destination delivery success rate
- ConfigManager health status

### Priority 3: Create New Dashboard (Optional)

Consider creating a dedicated "Ingester & Destination Health" dashboard for detailed monitoring of:
- Ingester lifecycle and health
- Destination delivery performance
- ConfigManager operations

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

## Implementation Notes

1. **Variables**: Add `$source` variable to all optimization panels
2. **Thresholds**: Use Grafana threshold configuration for visual alerts
3. **Annotations**: Add annotations for optimization events
4. **Refresh**: Set appropriate refresh intervals (30s-1m)

## Testing Recommendations

1. **Validate Dashboard Queries**: Import dashboards into Grafana and verify all panels display data
2. **Check Metric Availability**: Ensure all queried metrics exist and have data
3. **Verify Label Matching**: Confirm severity filters work correctly
4. **Test Time Ranges**: Verify dashboards work across different time ranges

## Next Steps

1. ✅ **Fixed**: All severity value mismatches
2. ⏳ **Pending**: Add new metrics panels to Operations dashboard
3. ⏳ **Pending**: Add high-level metrics to Executive dashboard
4. ⏳ **Optional**: Create dedicated Ingester/Destination health dashboard

## Related Documentation

- [OBSERVABILITY.md](OBSERVABILITY.md) - Metrics and monitoring guide
- [config/dashboards/README.md](../config/dashboards/README.md) - Dashboard documentation
- [config/dashboards/METRIC_USAGE_GUIDE.md](../config/dashboards/METRIC_USAGE_GUIDE.md) - Metric usage guide
- [config/dashboards/DASHBOARD_GUIDE.md](../config/dashboards/DASHBOARD_GUIDE.md) - Dashboard guide

