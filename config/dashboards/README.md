# Zen Watcher Dashboards

This directory contains Grafana dashboards for monitoring Zen Watcher.

## Dashboards

### 1. zen-watcher-dashboard.json

Comprehensive dashboard for monitoring Zen Watcher including:

**Overview Panels:**
- Health Status
- Events/sec Rate
- Active Events Count
- Critical Events Count

**Event Metrics:**
- Events Rate by Category/Source/Severity (time series)
- Active Events by Category (pie chart)
- Events by Severity (pie chart)

**Watcher Metrics:**
- Watcher Status (gauge)
- Watcher Errors (time series)
- Watcher Scrape Duration p95 (time series)

**Performance Metrics:**
- Goroutines
- Memory Usage (RSS, Heap)
- CPU Usage

**Operation Metrics:**
- HTTP Requests (time series)
- HTTP Request Duration p95
- CRD Operation Duration p95

## Installation

### Prerequisites

- Grafana instance
- Prometheus datasource configured
- Zen Watcher deployed with ServiceMonitor enabled

### Import Dashboard

#### Option 1: Grafana UI

1. Open Grafana
2. Go to **Dashboards** → **Import**
3. Upload `zen-watcher-dashboard.json`
4. Select your Prometheus datasource
5. Click **Import**

#### Option 2: Command Line

```bash
# Using Grafana API
curl -X POST http://admin:admin@localhost:3000/api/dashboards/db \
  -H "Content-Type: application/json" \
  -d @zen-watcher-dashboard.json
```

#### Option 3: Kubernetes ConfigMap

```bash
# Create ConfigMap from dashboard
kubectl create configmap zen-watcher-dashboard \
  --from-file=zen-watcher-dashboard.json \
  -n monitoring

# Add label for Grafana sidecar
kubectl label configmap zen-watcher-dashboard \
  grafana_dashboard=1 \
  -n monitoring
```

### With Grafana Operator

```yaml
apiVersion: grafana.integreatly.org/v1beta1
kind: GrafanaDashboard
metadata:
  name: zen-watcher
  namespace: monitoring
spec:
  json: |
    # Paste dashboard JSON here
  datasources:
    - inputName: "DS_PROMETHEUS"
      datasourceName: "Prometheus"
```

## Customization

### Variables

The dashboard includes template variables:

- **datasource**: Prometheus datasource selection
- **cluster**: Filter by cluster ID

### Add Custom Panels

Edit the dashboard JSON and add new panels:

```json
{
  "gridPos": {
    "h": 6,
    "w": 12,
    "x": 0,
    "y": 36
  },
  "id": 17,
  "targets": [
    {
      "legendFormat": "{{label}}",
      "refId": "A"
    }
  ],
  "title": "Your Custom Panel",
  "type": "timeseries"
}
```

## Metrics Reference

### Event Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `zen_watcher_events_total` | Counter | category, source, event_type, severity | Total events collected |
| `zen_watcher_events_written_total` | Counter | category, source | Successfully written events |
| `zen_watcher_events_failures_total` | Counter | category, source, reason | Failed event writes |
| `zen_watcher_active_events` | Gauge | category, severity | Currently active events |

### Watcher Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `zen_watcher_watcher_status` | Gauge | watcher | Watcher enabled status (0/1) |
| `zen_watcher_watcher_errors_total` | Counter | watcher, error_type | Watcher errors |
| `zen_watcher_scrape_duration_seconds` | Histogram | watcher | Scrape operation duration |

### CRD Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `zen_watcher_crd_operations_total` | Counter | operation, status | CRD operations |
| `zen_watcher_crd_operation_duration_seconds` | Histogram | operation | CRD operation duration |

### HTTP Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `zen_watcher_http_requests_total` | Counter | endpoint, method, status | HTTP requests |
| `zen_watcher_http_request_duration_seconds` | Histogram | endpoint, method | HTTP request duration |

### System Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `zen_watcher_health_status` | Gauge | - | Health status (0/1) |
| `zen_watcher_readiness_status` | Gauge | - | Readiness status (0/1) |
| `zen_watcher_goroutines` | Gauge | - | Number of goroutines |

## Query Examples

### PromQL Queries

```promql
# Total events per second
rate(zen_watcher_events_total[5m])

# Security events by source
sum by(source)(rate(zen_watcher_events_total{category="security"}[5m]))

# Critical events count
sum(zen_watcher_active_events{severity="CRITICAL"})

# Error rate
rate(zen_watcher_events_failures_total[5m])

# p95 CRD operation latency
histogram_quantile(0.95, sum(rate(zen_watcher_crd_operation_duration_seconds_bucket[5m])) by (le, operation))

# Watcher uptime
count(zen_watcher_watcher_status == 1)
```

## Alerting Integration

See `../examples/prometheus-servicemonitor.yaml` for alerting rules.

Example alerts based on dashboard metrics:

```yaml
- alert: HighEventRate
  expr: rate(zen_watcher_events_total[5m]) > 100
  for: 5m
  annotations:
    summary: "High event ingestion rate"

- alert: CriticalEventsAccumulating
  expr: sum(zen_watcher_active_events{severity="CRITICAL"}) > 50
  for: 10m
  annotations:
    summary: "Too many critical events"

- alert: WatcherDown
  expr: zen_watcher_watcher_status == 0
  for: 5m
  annotations:
    summary: "Watcher {{$labels.watcher}} is down"
```

## Dashboard Panels Breakdown

1. **Health Status** - Single stat showing system health
2. **Events/sec** - Rate of events being processed
3. **Active Events** - Total number of unresolved events
4. **Critical Events** - Number of critical severity events
5. **Events Rate Timeline** - Historical event rate by category/source
6. **Events by Category** - Pie chart of event distribution
7. **Events by Severity** - Pie chart of severity distribution
8. **Watcher Status** - Status of each watcher (enabled/disabled)
9. **Watcher Scrape Duration** - Performance of watchers
10. **Goroutines** - Runtime goroutine count
11. **Memory Usage** - Memory consumption metrics
12. **CPU Usage** - CPU utilization
13. **Watcher Errors** - Error rates by watcher
14. **CRD Operation Duration** - Performance of CRD operations
15. **HTTP Requests** - HTTP endpoint usage
16. **HTTP Request Duration** - API performance

## Tips

2. **Set Refresh**: Dashboard auto-refreshes every 10s
3. **Time Range**: Adjust time range for historical analysis
4. **Annotations**: Add annotations for deployments and incidents
5. **Alerts**: Link dashboard panels to alert rules

## Troubleshooting

### No Data Showing

1. Check Prometheus is scraping:
   ```bash
   kubectl get servicemonitor -n zen-system
   kubectl logs -n monitoring prometheus-0 | grep zen-watcher
   ```

2. Verify metrics endpoint:
   ```bash
   kubectl port-forward -n zen-system svc/zen-watcher 8080:8080
   curl http://localhost:8080/metrics
   ```

3. Check datasource:
   - Ensure Prometheus datasource is configured in Grafana
   - Test connection in Grafana settings

### Panels Empty

Check if metrics exist in Prometheus:
```promql
{__name__=~"zen_watcher_.*"}
```

## Community Dashboards

Share your custom dashboards with the community!

1. Create your custom dashboard
2. Export as JSON
3. Submit PR to add to this directory
4. Help others monitor their deployments!

## Support

- Issues: https://github.com/your-org/zen-watcher/issues
- Discussions: https://github.com/your-org/zen-watcher/discussions


