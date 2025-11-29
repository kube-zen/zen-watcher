# Dashboard & Alerting Improvements

## 📊 Dashboard Improvements

### 1. **New Panels to Add**

#### A. Error Tracking Section
```json
{
  "title": "Observation Creation Errors",
  "expr": "sum by(source, error_type)(zen_watcher_observations_create_errors_total)",
  "type": "table",
  "description": "Track failures in observation creation by source and error type"
}

{
  "title": "GC Errors",
  "expr": "sum by(operation, error_type)(zen_watcher_gc_errors_total)",
  "type": "stat",
  "thresholds": {
    "steps": [
      {"color": "green", "value": 0},
      {"color": "yellow", "value": 1},
      {"color": "red", "value": 5}
    ]
  }
}
```

#### B. Garbage Collection Metrics
```json
{
  "title": "Observations Deleted (GC)",
  "expr": "sum by(source, reason)(zen_watcher_observations_deleted_total)",
  "type": "timeseries",
  "description": "Observations cleaned up by garbage collector"
}

{
  "title": "GC Run Frequency",
  "expr": "rate(zen_watcher_gc_runs_total[1h])",
  "type": "stat",
  "unit": "runs/hour"
}

{
  "title": "GC Duration",
  "expr": "histogram_quantile(0.95, rate(zen_watcher_gc_duration_seconds_bucket[5m]))",
  "type": "timeseries",
  "unit": "s",
  "description": "95th percentile GC execution time"
}
```

#### C. Processing Pipeline Health
```json
{
  "title": "Filter Efficiency",
  "expr": "sum(zen_watcher_observations_filtered_total) / (sum(zen_watcher_observations_created_total) + sum(zen_watcher_observations_filtered_total)) * 100",
  "type": "gauge",
  "unit": "percent",
  "description": "Percentage of observations filtered vs created"
}

{
  "title": "Deduplication Rate",
  "expr": "rate(zen_watcher_observations_deduped_total[5m])",
  "type": "timeseries",
  "description": "Rate of duplicate observations detected"
}

{
  "title": "Pipeline Flow",
  "expr": [
    "sum(zen_watcher_observations_created_total) as Created",
    "sum(zen_watcher_observations_filtered_total) as Filtered",
    "zen_watcher_observations_deduped_total as Deduped"
  ],
  "type": "bargauge",
  "description": "Visual representation of observation pipeline"
}
```

#### D. Rate-Based Metrics (Better than Cumulative)
```json
{
  "title": "Events Created Rate",
  "expr": "sum(rate(zen_watcher_observations_created_total[5m])) * 60",
  "type": "timeseries",
  "unit": "obs/min",
  "description": "Current rate of observation creation"
}

{
  "title": "Events Filtered Rate",
  "expr": "sum(rate(zen_watcher_observations_filtered_total[5m])) * 60",
  "type": "timeseries",
  "unit": "obs/min"
}

{
  "title": "Critical Events Rate",
  "expr": "sum(rate(zen_watcher_events_total{severity=\"CRITICAL\"}[5m])) * 60",
  "type": "stat",
  "unit": "events/min",
  "thresholds": {
    "steps": [
      {"color": "green", "value": 0},
      {"color": "yellow", "value": 1},
      {"color": "red", "value": 5}
    ]
  }
}
```

#### E. Active Observations Count (Gauge)
```json
{
  "title": "Current Active Observations",
  "expr": "count(kube_customresource_observations) or sum(zen_watcher_observations_created_total) - sum(zen_watcher_observations_deleted_total)",
  "type": "stat",
  "description": "Approximate count of active observations (requires kube-state-metrics or calculation)"
}
```

#### F. Processing Performance
```json
{
  "title": "Event Processing Latency",
  "expr": "histogram_quantile(0.99, sum(rate(zen_watcher_event_processing_duration_seconds_bucket[5m])) by (le, source))",
  "type": "timeseries",
  "unit": "s",
  "legend": "{{source}}"
}

{
  "title": "Processing Rate by Source",
  "expr": "sum by(source)(rate(zen_watcher_event_processing_duration_seconds_count[5m]))",
  "type": "bargauge",
  "description": "Events processed per second by source"
}
```

#### G. Webhook Health Details
```json
{
  "title": "Webhook Success Rate",
  "expr": "sum(rate(zen_watcher_webhook_requests_total{status=\"200\"}[5m])) / sum(rate(zen_watcher_webhook_requests_total[5m])) * 100",
  "type": "gauge",
  "unit": "percent",
  "thresholds": {
    "steps": [
      {"color": "red", "value": 0},
      {"color": "yellow", "value": 95},
      {"color": "green", "value": 99}
    ]
  }
}

{
  "title": "Webhook Events Dropped",
  "expr": "sum(rate(zen_watcher_webhook_events_dropped_total[5m]))",
  "type": "timeseries",
  "description": "Backpressure - events dropped due to full channels"
}
```

### 2. **Dashboard Organization Improvements**

#### Suggested Row Structure:
```
Row 1: Health & Overview (Stats)
  - Health Status
  - Active Tools Count
  - Events Created Rate (NEW)
  - Critical Events Rate (NEW)
  - Error Count (NEW)

Row 2: Observation Pipeline
  - Created Rate
  - Filtered Rate
  - Deduped Rate
  - Pipeline Flow Diagram (NEW)

Row 3: Event Distribution
  - By Source (pie)
  - By Severity (pie)
  - By Category (pie)

Row 4: Time Series
  - Events Over Time (stacked)
  - Critical/High Severity Timeline
  - Processing Latency (NEW)

Row 5: Garbage Collection (NEW)
  - GC Runs
  - GC Duration
  - Observations Deleted
  - GC Errors

Row 6: Errors & Issues (NEW)
  - Creation Errors Table
  - GC Errors
  - Webhook Failures
  - Processing Errors

Row 7: Performance
  - Processing Duration (p50/p95/p99)
  - Webhook Success Rate
  - Webhook Events Dropped

Row 8: Details
  - Top 10 Critical Observations
  - Tool Status
  - Informer Cache Sync
```

### 3. **Visualization Improvements**

#### A. Replace Cumulative Counters with Rates
- **Current**: `sum(zen_watcher_events_total)` (cumulative, misleading)
- **Better**: `sum(rate(zen_watcher_observations_created_total[5m])) * 60` (events/min)

#### B. Add Heatmaps
```json
{
  "title": "Event Activity Heatmap",
  "expr": "sum by(source, severity)(rate(zen_watcher_events_total[5m]))",
  "type": "heatmap",
  "description": "Activity matrix: Source × Severity"
}
```

#### C. Add State Timeline
```json
{
  "title": "Tool Status Timeline",
  "expr": "zen_watcher_tools_active",
  "type": "state-timeline",
  "description": "Historical tool availability"
}
```

#### D. Add Stat Panels with Trends
- Use "sparkline" mode to show trends in stat panels
- Add "comparison" to show change from previous period

### 4. **Dashboard Variables (Templating)**

Add more useful variables:
```json
{
  "name": "severity",
  "type": "query",
  "query": "label_values(zen_watcher_events_total, severity)",
  "multi": true,
  "includeAll": true
}

{
  "name": "category",
  "type": "query",
  "query": "label_values(zen_watcher_events_total, category)",
  "multi": true,
  "includeAll": true
}

{
  "name": "time_range",
  "type": "interval",
  "options": ["1h", "6h", "24h", "7d"],
  "current": "6h"
}
```

---

## 🚨 Alert Improvements

### 1. **Critical Alerts (P0 - Page Immediately)**

#### A. Service Down
```yaml
- alert: ZenWatcherDown
  expr: up{job="zen-watcher"} == 0
  for: 1m
  labels:
    severity: critical
    component: availability
  annotations:
    summary: "zen-watcher is down"
    description: "zen-watcher pod is not responding. Check pod status: kubectl get pods -n zen-system -l app.kubernetes.io/name=zen-watcher"
    runbook_url: "https://github.com/kube-zen/zen-watcher/docs/TROUBLESHOOTING.md"
```

#### B. High Error Rate
```yaml
- alert: ZenWatcherHighErrorRate
  expr: |
    (
      sum(rate(zen_watcher_observations_create_errors_total[5m])) +
      sum(rate(zen_watcher_gc_errors_total[5m]))
    ) > 10
  for: 5m
  labels:
    severity: critical
    component: reliability
  annotations:
    summary: "High error rate in zen-watcher"
    description: "{{$value}} errors/sec detected. Check logs: kubectl logs -n zen-system -l app.kubernetes.io/name=zen-watcher --tail=100"
```

#### C. Critical Events Spike
```yaml
- alert: ZenWatcherCriticalEventsSpike
  expr: sum(rate(zen_watcher_events_total{severity="CRITICAL"}[5m])) * 60 > 20
  for: 2m
  labels:
    severity: critical
    component: security
  annotations:
    summary: "Critical security events spike detected"
    description: "{{$value}} CRITICAL events/min detected. Potential security incident."
```

### 2. **Warning Alerts (P1 - Notify)**

#### A. No Events Being Created
```yaml
- alert: ZenWatcherNoEvents
  expr: sum(rate(zen_watcher_observations_created_total[10m])) == 0
  for: 10m
  labels:
    severity: warning
    component: functionality
  annotations:
    summary: "No observations being created"
    description: "No observations created in last 10 minutes. Check if sources are active and filters aren't too restrictive."
```

#### B. High Filter Rate
```yaml
- alert: ZenWatcherHighFilterRate
  expr: |
    sum(rate(zen_watcher_observations_filtered_total[5m])) /
    (sum(rate(zen_watcher_observations_created_total[5m])) + sum(rate(zen_watcher_observations_filtered_total[5m]))) > 0.9
  for: 10m
  labels:
    severity: warning
    component: configuration
  annotations:
    summary: "High filter rate (>90%)"
    description: "{{$value | humanizePercentage}} of observations are being filtered. Consider reviewing filter configuration."
```

#### C. Tool Offline
```yaml
- alert: ZenWatcherToolOffline
  expr: zen_watcher_tools_active == 0
  for: 5m
  labels:
    severity: warning
    component: integration
  annotations:
    summary: "Security tool {{$labels.tool}} not detected"
    description: "Tool {{$labels.tool}} has been offline for 5+ minutes. Check if tool is installed and running."
```

#### D. Slow Processing
```yaml
- alert: ZenWatcherSlowProcessing
  expr: |
    histogram_quantile(0.95, 
      sum(rate(zen_watcher_event_processing_duration_seconds_bucket[5m])) by (le)
    ) > 5
  for: 10m
  labels:
    severity: warning
    component: performance
  annotations:
    summary: "Slow event processing (p95 > 5s)"
    description: "p95 processing latency is {{$value}}s. Check resource constraints and API server load."
```

#### E. Webhook Failures
```yaml
- alert: ZenWatcherWebhookFailing
  expr: |
    sum(rate(zen_watcher_webhook_requests_total{status!="200"}[5m])) /
    sum(rate(zen_watcher_webhook_requests_total[5m])) > 0.1
  for: 5m
  labels:
    severity: warning
    component: integration
  annotations:
    summary: "Webhook endpoint {{$labels.endpoint}} failing"
    description: "{{$value | humanizePercentage}} of requests to {{$labels.endpoint}} are failing (status: {{$labels.status}})"
```

#### F. GC Errors
```yaml
- alert: ZenWatcherGCErrors
  expr: rate(zen_watcher_gc_errors_total[5m]) > 0.1
  for: 5m
  labels:
    severity: warning
    component: gc
  annotations:
    summary: "Garbage collection errors detected"
    description: "{{$value}} GC errors/sec. Operation: {{$labels.operation}}, Error: {{$labels.error_type}}"
```

#### G. High Deduplication Rate
```yaml
- alert: ZenWatcherHighDeduplicationRate
  expr: |
    rate(zen_watcher_observations_deduped_total[5m]) /
    (rate(zen_watcher_observations_created_total[5m]) + rate(zen_watcher_observations_deduped_total[5m])) > 0.5
  for: 10m
  labels:
    severity: info
    component: deduplication
  annotations:
    summary: "High deduplication rate (>50%)"
    description: "Many duplicate observations detected. Consider adjusting dedup window or investigating source behavior."
```

### 3. **Info Alerts (P2 - Log Only)**

#### A. GC Running Frequently
```yaml
- alert: ZenWatcherGCFrequent
  expr: rate(zen_watcher_gc_runs_total[1h]) > 2
  for: 1h
  labels:
    severity: info
    component: gc
  annotations:
    summary: "GC running more frequently than expected"
    description: "GC running {{$value}} times/hour. Consider adjusting GC_INTERVAL."
```

#### B. High Observation Count
```yaml
- alert: ZenWatcherHighObservationCount
  expr: sum(zen_watcher_observations_created_total) - sum(zen_watcher_observations_deleted_total) > 10000
  for: 1h
  labels:
    severity: info
    component: capacity
  annotations:
    summary: "High number of active observations"
    description: "{{$value}} active observations. Consider reducing TTL or increasing GC frequency."
```

### 4. **Complete Alert Rules File**

```yaml
groups:
- name: zen-watcher-critical
  interval: 30s
  rules:
    - alert: ZenWatcherDown
      expr: up{job="zen-watcher"} == 0
      for: 1m
      labels:
        severity: critical
      annotations:
        summary: "zen-watcher is down"
        
    - alert: ZenWatcherHighErrorRate
      expr: |
        (
          sum(rate(zen_watcher_observations_create_errors_total[5m])) +
          sum(rate(zen_watcher_gc_errors_total[5m]))
        ) > 10
      for: 5m
      labels:
        severity: critical
      annotations:
        summary: "High error rate in zen-watcher"

    - alert: ZenWatcherCriticalEventsSpike
      expr: sum(rate(zen_watcher_events_total{severity="CRITICAL"}[5m])) * 60 > 20
      for: 2m
      labels:
        severity: critical
      annotations:
        summary: "Critical security events spike"

- name: zen-watcher-warning
  interval: 30s
  rules:
    - alert: ZenWatcherNoEvents
      expr: sum(rate(zen_watcher_observations_created_total[10m])) == 0
      for: 10m
      labels:
        severity: warning
      annotations:
        summary: "No observations being created"

    - alert: ZenWatcherHighFilterRate
      expr: |
        sum(rate(zen_watcher_observations_filtered_total[5m])) /
        (sum(rate(zen_watcher_observations_created_total[5m])) + sum(rate(zen_watcher_observations_filtered_total[5m]))) > 0.9
      for: 10m
      labels:
        severity: warning
      annotations:
        summary: "High filter rate (>90%)"

    - alert: ZenWatcherToolOffline
      expr: zen_watcher_tools_active == 0
      for: 5m
      labels:
        severity: warning
      annotations:
        summary: "Security tool {{$labels.tool}} not detected"

    - alert: ZenWatcherSlowProcessing
      expr: |
        histogram_quantile(0.95, 
          sum(rate(zen_watcher_event_processing_duration_seconds_bucket[5m])) by (le)
        ) > 5
      for: 10m
      labels:
        severity: warning
      annotations:
        summary: "Slow event processing (p95 > 5s)"

    - alert: ZenWatcherWebhookFailing
      expr: |
        sum(rate(zen_watcher_webhook_requests_total{status!="200"}[5m])) /
        sum(rate(zen_watcher_webhook_requests_total[5m])) > 0.1
      for: 5m
      labels:
        severity: warning
      annotations:
        summary: "Webhook endpoint {{$labels.endpoint}} failing"

    - alert: ZenWatcherGCErrors
      expr: rate(zen_watcher_gc_errors_total[5m]) > 0.1
      for: 5m
      labels:
        severity: warning
      annotations:
        summary: "Garbage collection errors detected"

- name: zen-watcher-info
  interval: 1m
  rules:
    - alert: ZenWatcherHighDeduplicationRate
      expr: |
        rate(zen_watcher_observations_deduped_total[5m]) /
        (rate(zen_watcher_observations_created_total[5m]) + rate(zen_watcher_observations_deduped_total[5m])) > 0.5
      for: 10m
      labels:
        severity: info
      annotations:
        summary: "High deduplication rate (>50%)"

    - alert: ZenWatcherGCFrequent
      expr: rate(zen_watcher_gc_runs_total[1h]) > 2
      for: 1h
      labels:
        severity: info
      annotations:
        summary: "GC running more frequently than expected"
```

---

## 📈 Additional Recommendations

### 1. **SLO/SLI Tracking**
Add panels for Service Level Objectives:
- **Availability**: `(up{job="zen-watcher"} == 1)`
- **Error Rate**: `< 0.1%`
- **Latency**: `p95 < 1s`
- **Throughput**: `> 10 events/min`

### 2. **Anomaly Detection**
Consider using Grafana's ML plugin or Prometheus' `predict_linear()` for:
- Unusual event rate patterns
- Processing latency anomalies
- Error rate spikes

### 3. **Multi-Cluster Support**
If running in multiple clusters:
- Add `cluster` label to all metrics
- Create federated dashboard
- Aggregate alerts across clusters

### 4. **Integration with Incident Response**
- Link alerts to runbooks
- Add playbook URLs in annotations
- Integrate with PagerDuty/Opsgenie

### 5. **Cost Optimization**
Track resource usage:
- Memory per observation
- CPU per event processed
- Storage growth rate

---

## 🎯 Priority Implementation Order

1. **Phase 1 (Critical)**:
   - Fix cumulative counters → use rates
   - Add error tracking panels
   - Implement critical alerts (service down, high errors)

2. **Phase 2 (Important)**:
   - Add GC metrics panels
   - Add processing pipeline visualization
   - Implement warning alerts

3. **Phase 3 (Nice to Have)**:
   - Add heatmaps and advanced visualizations
   - Implement SLO tracking
   - Add anomaly detection

---

## 📝 Notes

- All rate queries use `[5m]` window - adjust based on scrape interval
- Thresholds are suggestions - tune based on your environment
- Consider using recording rules for complex queries
- Test alerts in staging before production

