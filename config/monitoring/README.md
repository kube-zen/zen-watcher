# Grafana Dashboard for zen-watcher

## Overview

The `zen-watcher-dashboard.json` provides comprehensive visibility into security event aggregation, tool status, and performance.

## Features

### Event-Focused Panels (Top Priority)

1. **Overview Stats** - Quick glance at events/min, total events, active tools
2. **Events by Source** - Pie chart showing distribution (Trivy, Kyverno, etc.)
3. **Events by Category** - Security vs Compliance breakdown
4. **Event Creation Rate** - Stacked area chart showing event velocity
5. **Live Event Stream** - Real-time event creation by severity
6. **Cumulative Growth** - Total events over time per source
7. **Event Heatmap** - Matrix view of Source × Severity
8. **Event Flow Sankey** - Flow diagram: Source → Category → Severity

### Performance Panels

9. **Security Tools Status** - Table showing active/inactive tools
10. **Event Activity Heatmap** - Last hour activity timeline
11. **Webhook Request Status** - Table of webhook endpoints and HTTP status codes
12. **Watch Loop Performance** - Latency percentiles (p50, p95, p99)

### Container Resources (Bottom)

13. **Memory Usage** - Container memory consumption
14. **CPU Usage** - Container CPU utilization

## Unique Visualizations

This dashboard uses advanced Grafana panel types:

- **Sankey Diagram** - Shows event flow from source → category → severity
- **State Timeline** - Heatmap-style activity over time
- **Heatmap** - Matrix visualization for source × severity
- **Donut Charts** - Modern pie charts with better readability
- **Smooth Line Interpolation** - Cleaner time series graphs
- **Table with Color Cells** - Status indicators with background colors

## Installation

### Via kubectl

```bash
# Create ConfigMap with dashboard
kubectl create configmap zen-watcher-dashboard \
  --from-file=dashboard.json=config/monitoring/zen-watcher-dashboard.json \
  -n monitoring

# Label for Grafana auto-discovery
kubectl label configmap zen-watcher-dashboard \
  grafana_dashboard="1" \
  -n monitoring
```

### Via Grafana UI

1. Open Grafana
2. Go to **Dashboards** → **Import**
3. Upload `zen-watcher-dashboard.json`
4. Select Prometheus datasource
5. Click **Import**

### Via Helm (Grafana Operator)

```yaml
# values.yaml
grafana:
  dashboardProviders:
    dashboardproviders.yaml:
      apiVersion: 1
      providers:
      - name: 'zen-watcher'
        folder: 'Security'
        type: file
        options:
          path: /var/lib/grafana/dashboards/zen-watcher

  dashboards:
    zen-watcher:
      zen-watcher-dashboard:
        file: config/monitoring/zen-watcher-dashboard.json
```

## Metrics Reference

### Event Metrics

```
# Total events created by source, category, and severity
zen_watcher_events_total{source="trivy",category="security",severity="HIGH"}

# Examples:
zen_watcher_events_total{source="trivy",category="security",severity="HIGH"} 150
zen_watcher_events_total{source="kyverno",category="security",severity="MEDIUM"} 45
zen_watcher_events_total{source="falco",category="security",severity="HIGH"} 12
zen_watcher_events_total{source="kube-bench",category="compliance",severity="HIGH"} 8
```

### Tool Detection

```
# Tool active status (1=active, 0=inactive)
zen_watcher_tools_active{tool="trivy"}
zen_watcher_tools_active{tool="kyverno"}
zen_watcher_tools_active{tool="falco"}
zen_watcher_tools_active{tool="kube-bench"}
zen_watcher_tools_active{tool="checkov"}
```

### Performance Metrics

```
# Watch loop duration histogram
zen_watcher_loop_duration_seconds_bucket{le="1"}
zen_watcher_loop_duration_seconds_sum
zen_watcher_loop_duration_seconds_count

# Webhook requests by endpoint and HTTP status
zen_watcher_webhook_requests_total{endpoint="falco",status="200"}
zen_watcher_webhook_requests_total{endpoint="audit",status="200"}
```

## Useful Queries

### Event Rate

```promql
# Events per minute by source
sum by (source) (rate(zen_watcher_events_total[5m])) * 60

# HIGH severity events per minute
sum(rate(zen_watcher_events_total{severity="HIGH"}[5m])) * 60
```

### Tool Health

```promql
# Number of active tools
sum(zen_watcher_tools_active)

# Inactive tools
count(zen_watcher_tools_active == 0)
```

### Performance

```promql
# Average loop duration
rate(zen_watcher_loop_duration_seconds_sum[5m]) / rate(zen_watcher_loop_duration_seconds_count[5m])

# 99th percentile latency
histogram_quantile(0.99, rate(zen_watcher_loop_duration_seconds_bucket[5m]))
```

### Webhook Health

```promql
# Webhook success rate
sum(rate(zen_watcher_webhook_requests_total{status="200"}[5m])) / sum(rate(zen_watcher_webhook_requests_total[5m]))

# Failed webhook requests
sum by (endpoint) (rate(zen_watcher_webhook_requests_total{status!="200"}[5m]))
```

## Alerting Rules

### Recommended Prometheus Alerts

```yaml
groups:
- name: zen-watcher
  interval: 30s
  rules:
  # No events being created
  - alert: ZenWatcherNoEvents
    expr: rate(zen_watcher_events_total[10m]) == 0
    for: 10m
    labels:
      severity: warning
    annotations:
      summary: "zen-watcher not creating events"
      description: "No events created in last 10 minutes"

  # Tool detection failure
  - alert: ZenWatcherToolOffline
    expr: zen_watcher_tools_active == 0
    for: 5m
    labels:
      severity: info
    annotations:
      summary: "Security tool {{$labels.tool}} not detected"
      description: "Tool may not be installed or running"

  # High event rate (potential attack)
  - alert: ZenWatcherHighEventRate
    expr: rate(zen_watcher_events_total{severity="HIGH"}[5m]) * 60 > 10
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "High severity event spike"
      description: "Creating {{$value}} HIGH severity events/min"

  # Slow watch loop
  - alert: ZenWatcherSlowLoop
    expr: histogram_quantile(0.99, rate(zen_watcher_loop_duration_seconds_bucket[5m])) > 30
    for: 10m
    labels:
      severity: warning
    annotations:
      summary: "zen-watcher loop running slow"
      description: "p99 latency is {{$value}}s (>30s threshold)"

  # Webhook failures
  - alert: ZenWatcherWebhookFailing
    expr: rate(zen_watcher_webhook_requests_total{status!="200"}[5m]) > 0.1
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "Webhook endpoint {{$labels.endpoint}} failing"
      description: "HTTP {{$labels.status}} errors detected"
```

## Dashboard Customization

### Change Time Range

Default: Last 6 hours  
To change: Click time picker in top-right corner

### Filter by Source

Add variable:
```json
{
  "name": "source",
  "type": "query",
  "query": "label_values(zen_watcher_events_total, source)",
  "multi": true,
  "includeAll": true
}
```

Then use in queries: `zen_watcher_events_total{source=~"$source"}`

### Add SLO Panels

```promql
# Event processing success rate (target: 99.9%)
sum(rate(zen_watcher_events_total[5m])) / (sum(rate(zen_watcher_events_total[5m])) + sum(rate(zen_watcher_crd_write_errors_total[5m])))
```

## Troubleshooting

### Dashboard shows "No Data"

1. **Check Prometheus scraping:**
   ```bash
   kubectl port-forward -n zen-cluster deployment/zen-watcher 8080:8080
   curl http://localhost:8080/metrics | grep zen_watcher
   ```

2. **Verify ServiceMonitor:**
   ```bash
   kubectl get servicemonitor -n zen-cluster
   ```

3. **Check Prometheus targets:**
   - Prometheus UI → Status → Targets
   - Look for `zen-watcher` endpoint

### Metrics not updating

1. **Check pod is running:**
   ```bash
   kubectl get pods -n zen-cluster -l app.kubernetes.io/name=zen-watcher
   ```

2. **Check metrics endpoint:**
   ```bash
   kubectl logs -n zen-cluster deployment/zen-watcher | grep metrics
   ```

3. **Verify scrape interval:**
   - Default: 30s
   - Check ServiceMonitor spec

---

**Version:** 1.0.21+  
**License:** Apache 2.0  
**Grafana Version:** 10.0+
