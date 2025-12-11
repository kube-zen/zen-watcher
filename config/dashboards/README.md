# Zen Watcher Grafana Dashboards

Zen Watcher includes 6 pre-built Grafana dashboards for monitoring and analysis.

---

## Canonical Dashboards (Launch-Ready)

### 1. Executive Overview (`zen-watcher-executive.json`) ⭐ **PRIMARY**

**Use cases**: High-level monitoring, status overview, quick health checks  
**Refresh**: 10s  
**Time Range**: Last 1 hour

**Panels**:
- System Status (UP/DOWN indicator using `up{job="zen-watcher"}`)
- Tools Monitored (count of active tools using `zen_watcher_tools_active`)
- Observations (24h total using `zen_watcher_observations_created_total`)
- Critical Events (1h count using `zen_watcher_events_total{severity="CRITICAL"}`)
- Success Rate (processing efficiency)
- Live Event Stream (stacked area chart by severity)
- Events by Source (donut chart)
- Events by Category (donut chart)
- Tool Status Matrix (table with status and event counts)

**Key Metrics**: `zen_watcher_events_total`, `zen_watcher_observations_created_total`, `zen_watcher_tools_active`

---

### 2. Operations Dashboard (`zen-watcher-operations.json`) ⭐ **PRIMARY**

**Use cases**: Performance monitoring, troubleshooting, SRE operations  
**Refresh**: 10s  
**Time Range**: Last 1 hour

**Sections**:
1. **Health & Availability**
   - Service Status (`up{job="zen-watcher"}`), Success Rate, Error Rate
   - Processing Latency (p95), Throughput
   - Dedup Cache Usage (`zen_watcher_dedup_cache_usage_ratio`), Webhook Queue Usage (`zen_watcher_webhook_queue_usage_ratio`)

2. **Performance Metrics**
   - Observation Creation Rate (by source using `zen_watcher_observations_created_total`)
   - Event Processing Latency (p50, p95, p99 using `zen_watcher_event_processing_duration_seconds_bucket`)

3. **Adapter & Filter Status**
   - Adapter Run Rate (`zen_watcher_adapter_runs_total`), Filter Decisions (`zen_watcher_filter_decisions_total`)

4. **Garbage Collection & Resource Management**
   - Live Observations (`zen_watcher_observations_live`), GC Duration (`zen_watcher_gc_duration_seconds_bucket`)

5. **Webhook & Integration Health**
   - Webhook Request Rate (`zen_watcher_webhook_requests_total`), Events Dropped (`zen_watcher_webhook_events_dropped_total`)

**Key Metrics**: `zen_watcher_observations_created_total`, `zen_watcher_event_processing_duration_seconds_bucket`, `zen_watcher_webhook_requests_total`, `zen_watcher_dedup_cache_usage_ratio`

---

### 3. Security Analytics (`zen-watcher-security.json`) ⭐ **PRIMARY**

**Use cases**: Security analysis, threat detection, compliance reporting  
**Refresh**: 10s  
**Time Range**: Last 6 hours

**Sections**:
1. **Security Posture Overview**
   - Critical Events (1h using `zen_watcher_events_total{severity="CRITICAL"}`)
   - High/Medium Severity events
   - Total Events (24h)
   - Active Tools (`zen_watcher_tools_active`)

2. **Security Trends & Analysis**
   - Security Event Rate (by severity over time using `zen_watcher_events_total`)

3. **Source Analysis**
   - Event Rate by Source Tool (time series)
   - Source Distribution (donut chart for 24h)

4. **Category Breakdown**
   - Event Rate by Category (time series)
   - Category Distribution (donut chart for 24h)

5. **Heat Maps & Correlation**
   - Security Event Heat Map (Source × Severity matrix)

**Key Metrics**: `zen_watcher_events_total` (with severity/category/source labels), `zen_watcher_tools_active`

---

### 4. Main Dashboard (`zen-watcher-dashboard.json`)

**Use cases**: Unified overview with navigation links to other dashboards  
**Refresh**: 10s  
**Time Range**: Last 1 hour

**Key Metrics**: `zen_watcher_events_total`, `zen_watcher_observations_created_total`, `zen_watcher_observations_filtered_total`, `zen_watcher_observations_deduped_total`

---

### 5. Namespace Health (`zen-watcher-namespace-health.json`)

**Use cases**: Per-namespace health metrics and event distribution  
**Refresh**: 10s  
**Time Range**: Last 1 hour

**Key Metrics**: `zen_watcher_events_total` (with namespace label)

---

### 6. Explorer (`zen-watcher-explorer.json`)

**Use cases**: Data exploration and query builder  
**Refresh**: 10s  
**Time Range**: Last 1 hour

**Key Metrics**: `zen_watcher_events_total`, `zen_watcher_observations_created_total`, `zen_watcher_observations_filtered_total`, `zen_watcher_observations_deduped_total`

---

## Quick Start

After running `./scripts/quick-demo.sh`, the dashboards are automatically available:

```bash
# Run the demo
./scripts/quick-demo.sh --non-interactive --deploy-mock-data

# Access Grafana (credentials shown at end of demo)
# URL: http://localhost:8080/grafana/

# Navigate to dashboards:
# - Zen Watcher - Executive Overview
# - Zen Watcher - Operations
# - Zen Watcher - Security Analytics
```

---

## Design

### Colors
- **Critical**: Red (#C4162A)
- **High**: Orange (#FF7F00)
- **Medium**: Yellow (#FADE2A)
- **Low**: Blue (#5794F2)
- **Success**: Green (#73BF69)
- **Background**: Dark theme

### Layout
- **Top Row**: Key metrics (status, counts)
- **Middle**: Time-series charts (trends, analysis)
- **Bottom**: Detailed tables and heat maps
- **Grid**: 24-column layout

### Refresh Rates
- **Executive**: 10s
- **Operations**: 10s
- **Security**: 10s

### Time Ranges
- **Executive**: 1h (recent activity)
- **Operations**: 1h (performance monitoring)
- **Security**: 6h (trend analysis)

---

## Metrics Reference

All metrics are defined in `pkg/metrics/definitions.go` and exposed at `/metrics` endpoint.

### Core Event Metrics
```promql
# Events created (after filtering and dedup)
# Labels: source, category, severity, eventType, namespace, kind
zen_watcher_events_total{source="trivy", category="security", severity="CRITICAL"}

# Observations created successfully
# Labels: source
zen_watcher_observations_created_total{source="trivy"}

# Observations filtered out
# Labels: source, reason
zen_watcher_observations_filtered_total{source="trivy", reason="severity_filter"}

# Observations deduplicated (no labels)
zen_watcher_observations_deduped_total
```

### Performance Metrics
```promql
# Processing latency histogram
# Labels: source, processor_type
histogram_quantile(0.95, rate(zen_watcher_event_processing_duration_seconds_bucket[5m]))

# Throughput calculation
rate(zen_watcher_observations_created_total[1m]) * 60  # events/min

# Success rate
100 * (1 - (sum(rate(zen_watcher_observations_create_errors_total[5m])) / 
  (sum(rate(zen_watcher_observations_created_total[5m])) + 
   sum(rate(zen_watcher_observations_create_errors_total[5m])) + 0.001)))
```

### Health Metrics
```promql
# Service up/down (Prometheus standard)
up{job="zen-watcher"}

# Tools active
# Labels: tool
zen_watcher_tools_active{tool="trivy"}

# Informer cache synced
# Labels: resource
zen_watcher_informer_cache_synced{resource="vulnerabilityreports"}
```

### Resource Metrics
```promql
# Cache usage
# Labels: source
zen_watcher_dedup_cache_usage_ratio{source="trivy"}

# Queue usage
# Labels: endpoint
zen_watcher_webhook_queue_usage_ratio{endpoint="/webhook/falco"}

# Live observations in etcd
# Labels: source
zen_watcher_observations_live{source="trivy"}
```

### Webhook Metrics
```promql
# Webhook requests
# Labels: endpoint, status
zen_watcher_webhook_requests_total{endpoint="/webhook/falco", status="200"}

# Webhook events dropped (backpressure)
# Labels: endpoint
zen_watcher_webhook_events_dropped_total{endpoint="/webhook/falco"}
```

### GC Metrics
```promql
# GC duration histogram
# Labels: operation
histogram_quantile(0.95, rate(zen_watcher_gc_duration_seconds_bucket[5m]))

# GC errors
# Labels: operation, error_type
zen_watcher_gc_errors_total{operation="delete", error_type="timeout"}
```

---

## Dashboard Variables

All dashboards support:
- **`${datasource}`**: Prometheus/VictoriaMetrics datasource selector

Future enhancements:
- **`${namespace}`**: Filter by namespace
- **`${cluster}`**: Multi-cluster support
- **`${severity}`**: Filter by severity level

---

## Customization

### Change Refresh Rate
```json
"refresh": "10s"  // Change to "5s", "30s", "1m", etc.
```

### Change Time Range
```json
"time": {
  "from": "now-1h",  // Change to "now-6h", "now-24h", etc.
  "to": "now"
}
```

### Add Custom Panels
1. Open dashboard in Grafana
2. Click "Add panel"
3. Use metrics from reference above
4. Save and export JSON

### Adjust Thresholds
```json
"thresholds": {
  "steps": [
    {"color": "green", "value": null},
    {"color": "yellow", "value": 10},
    {"color": "red", "value": 50}
  ]
}
```

---

## Alert Integration

Dashboards work with Prometheus alerts (see `../monitoring/prometheus-rules.yaml`):

**Critical Alerts**:
- ZenWatcherDown
- ZenWatcherHighErrorRate
- ZenWatcherCriticalEventsSpike

**Warning Alerts**:
- ZenWatcherNoEvents
- ZenWatcherHighFilterRate
- ZenWatcherToolOffline
- ZenWatcherSlowProcessing

**Info Alerts**:
- ZenWatcherHighDeduplicationRate
- ZenWatcherGCFrequent

Alerts are visualized in dashboards via:
- Color-coded thresholds
- Threshold lines on charts
- Alert annotations

---

## Additional Resources

- **Metrics Documentation**: `../../docs/PERFORMANCE.md`
- **Deduplication**: `../../docs/DEDUPLICATION.md`
- **Alert Rules**: `../monitoring/prometheus-rules.yaml`
- **Quick Demo Script**: `../../scripts/quick-demo.sh`
- **Architecture**: `../../docs/ARCHITECTURE.md`
