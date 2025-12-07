# Zen Watcher Grafana Dashboards

Zen Watcher includes 3 pre-built Grafana dashboards for monitoring and analysis.

---

## Dashboard Suite

### 1. Executive Overview (`zen-watcher-executive.json`)

**Use cases**: High-level monitoring, status overview, quick health checks  
**Refresh**: 10s  
**Time Range**: Last 1 hour

**Panels**:
- System Status (UP/DOWN indicator)
- Tools Monitored (count of active tools)
- Observations (24h total)
- Critical Events (1h count with threshold indicators)
- Success Rate (processing efficiency)
- Live Event Stream (stacked area chart by severity)
- Events by Source (donut chart)
- Events by Category (donut chart)
- Tool Status Matrix (table with status and event counts)

---

### 2. Operations Dashboard (`zen-watcher-operations.json`)

**Use cases**: Performance monitoring, troubleshooting, SRE operations  
**Refresh**: 10s  
**Time Range**: Last 1 hour

**Sections**:
1. **Health & Availability**
   - Service Status, Success Rate, Error Rate
   - Processing Latency (p95), Throughput
   - Dedup Cache Usage, Webhook Queue Usage

2. **Performance Metrics**
   - Observation Creation Rate (by source)
   - Event Processing Latency (p50, p95, p99 percentiles)

3. **Adapter & Filter Status**
   - Adapter Run Rate (by adapter and outcome)
   - Filter Decisions (allow vs drop)

4. **Garbage Collection & Resource Management**
   - Live Observations in etcd (by source)
   - GC Duration (p95 by operation)

5. **Webhook & Integration Health**
   - Webhook Request Rate (by status code)
   - Webhook Events Dropped (backpressure indicator)

---

### 3. Security Analytics (`zen-watcher-security.json`)

**Use cases**: Security analysis, threat detection, compliance reporting  
**Refresh**: 10s  
**Time Range**: Last 6 hours

**Sections**:
1. **Security Posture Overview**
   - Critical Events (1h)
   - High Severity (1h)
   - Medium Severity (1h)
   - Total Events (24h)
   - Active Tools

2. **Security Trends & Analysis**
   - Security Event Rate (by severity over time)
   - Line chart with severity-coded colors

3. **Source Analysis**
   - Event Rate by Source Tool (time series)
   - Source Distribution (donut chart for 24h)

4. **Category Breakdown**
   - Event Rate by Category (time series)
   - Category Distribution (donut chart for 24h)

5. **Heat Maps & Correlation**
   - Security Event Heat Map (Source × Severity matrix)
   - Color-coded cells showing event intensity

---

## Quick Start

After running `./hack/quick-demo.sh`, the dashboards are automatically available:

```bash
# Run the demo
./hack/quick-demo.sh --non-interactive --deploy-mock-data

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

### Core Event Metrics
```promql
# Events created (after filtering and dedup)
zen_watcher_events_total{source, category, severity}

# Observations created successfully
zen_watcher_observations_created_total{source}

# Observations filtered out
zen_watcher_observations_filtered_total{source, reason}

# Observations deduplicated
zen_watcher_observations_deduped_total
```

### Performance Metrics
```promql
# Processing latency histogram
zen_watcher_event_processing_duration_seconds{source, processor_type}

# Throughput calculation
rate(zen_watcher_observations_created_total[1m]) * 60  # events/min

# Success rate
100 * (1 - (errors / (created + errors)))
```

### Health Metrics
```promql
# Service up/down
up{job="zen-watcher"}

# Tools active
zen_watcher_tools_active{tool}

# Informer cache synced
zen_watcher_informer_cache_synced{resource}
```

### Resource Metrics
```promql
# Cache usage
zen_watcher_dedup_cache_usage_ratio{source}

# Queue usage
zen_watcher_webhook_queue_usage_ratio{endpoint}

# Live observations in etcd
zen_watcher_observations_live{source}
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
- **Quick Demo Script**: `../../hack/quick-demo.sh`
- **Architecture**: `../../docs/ARCHITECTURE.md`
