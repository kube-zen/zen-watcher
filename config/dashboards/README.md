# 🎨 Zen-Watcher Grafana Dashboards

**Professional, eye-shining dashboards for the 4-minute demo!** ✨

---

## 📊 Dashboard Suite

### 1. Executive Overview (`zen-watcher-executive.json`)

**Perfect for**: Quick demos, executive reviews, first impressions  
**Refresh**: 10s  
**Time Range**: Last 1 hour

**What makes it shine** ✨:
- **Big, bold status indicators** - System health at a glance
- **Real-time event stream** - Live security events flowing in
- **Beautiful donut charts** - Event distribution by source and category
- **Tool status matrix** - See all 6 security tools and their activity
- **Color-coded severity** - Critical (red), High (orange), Medium (yellow), Low (blue)

**Key Panels**:
- 🟢 System Status (UP/DOWN with background color)
- 🛡️ Tools Monitored (count of active tools)
- 📊 Observations (24h total)
- 🚨 Critical Events (1h count with threshold colors)
- ✅ Success Rate (processing efficiency)
- 🔥 Live Event Stream (stacked area chart by severity)
- 🛡️ Events by Source (donut chart)
- 📂 Events by Category (donut chart)
- 🔍 Tool Status Matrix (table with status + event counts)

**Demo Impact**: 
> "In just 4 minutes, you see ALL 6 security tools working, events flowing in real-time, and a complete security posture overview!"

---

### 2. Operations Dashboard (`zen-watcher-operations.json`)

**Perfect for**: SRE teams, performance monitoring, troubleshooting  
**Refresh**: 10s  
**Time Range**: Last 1 hour

**What makes it shine** ✨:
- **Health metrics at the top** - Success rate, error rate, latency, throughput
- **Performance deep-dive** - Processing latency percentiles (p50, p95, p99)
- **Adapter monitoring** - See which adapters are running and their outcomes
- **Resource management** - Cache usage, queue usage, GC performance
- **Webhook health** - Request rates, dropped events, backpressure

**Key Sections**:
1. **🏥 Health & Availability**
   - Service Status, Success Rate, Error Rate
   - Processing Latency (p95), Throughput
   - Dedup Cache Usage, Webhook Queue Usage

2. **📊 Performance Metrics**
   - Observation Creation Rate (by source)
   - Event Processing Latency (p50, p95, p99 percentiles)

3. **🔧 Adapter & Filter Status**
   - Adapter Run Rate (by adapter and outcome)
   - Filter Decisions (allow vs drop)

4. **🗑️ Garbage Collection & Resource Management**
   - Live Observations in etcd (by source)
   - GC Duration (p95 by operation)

5. **🌐 Webhook & Integration Health**
   - Webhook Request Rate (by status code)
   - Webhook Events Dropped (backpressure indicator)

**SRE Value**:
> "Everything you need to keep zen-watcher healthy: latency, throughput, errors, resource usage, and integration health!"

---

### 3. Security Analytics (`zen-watcher-security.json`)

**Perfect for**: Security teams, threat analysis, compliance reporting  
**Refresh**: 10s  
**Time Range**: Last 6 hours

**What makes it shine** ✨:
- **Security posture overview** - Critical, High, Medium severity at a glance
- **Trend analysis** - See security event patterns over time
- **Source intelligence** - Which tools are detecting what
- **Category breakdown** - Vulnerabilities, policy violations, runtime threats
- **Heat map** - Source × Severity correlation matrix

**Key Sections**:
1. **🚨 Security Posture Overview**
   - 🔴 Critical Events (1h)
   - 🟠 High Severity (1h)
   - 🟡 Medium Severity (1h)
   - 📊 Total Events (24h)
   - 🛡️ Active Tools

2. **📈 Security Trends & Analysis**
   - Security Event Rate (by severity over time)
   - Beautiful line chart with severity-coded colors

3. **🔍 Source Analysis**
   - Event Rate by Source Tool (time series)
   - Source Distribution (donut chart for 24h)

4. **📂 Category Breakdown**
   - Event Rate by Category (time series)
   - Category Distribution (donut chart for 24h)

5. **🎯 Heat Maps & Correlation**
   - Security Event Heat Map (Source × Severity matrix)
   - Color-coded cells showing event intensity

**Security Team Value**:
> "Understand your security posture, identify trends, and correlate events across tools - all in one dashboard!"

---

## 🚀 Quick Start (for quick-demo.sh)

After running `./hack/quick-demo.sh`, the dashboards are automatically available!

```bash
# Run the demo
./hack/quick-demo.sh --non-interactive --deploy-mock-data

# Access Grafana (credentials shown at end of demo)
# URL: http://localhost:8080/grafana/

# Navigate to dashboards:
# - Zen Watcher - Executive Overview  (start here!)
# - Zen Watcher - Operations
# - Zen Watcher - Security Analytics
```

---

## 📸 What You'll See in 4 Minutes

### Minute 1: Installation
- k3d cluster created
- Zen-watcher deployed
- 6 security tools installed

### Minute 2-3: Data Flow
- Mock data starts flowing
- Dashboards populate with real-time data
- All 6 sources showing activity

### Minute 4: The WOW Moment! ✨
- **Executive Dashboard**: All tools active, events streaming, beautiful charts
- **Operations Dashboard**: Perfect health metrics, low latency, high throughput
- **Security Dashboard**: Security posture visible, trends clear, correlations obvious

**Demo Script**:
1. Open Executive Overview → "Look at all 6 tools working!"
2. Point to Live Event Stream → "Real-time security events"
3. Show Tool Status Matrix → "Every tool is active and reporting"
4. Switch to Security Analytics → "Deep security intelligence"
5. Show Heat Map → "Correlation across tools and severities"

**Audience Reaction**: 🤩 "This is AMAZING!"

---

## 🎨 Design Principles

### Colors
- **Critical**: Dark Red (#C4162A)
- **High**: Dark Orange (#FF7F00)
- **Medium**: Dark Yellow (#FADE2A)
- **Low**: Semi-Dark Blue (#5794F2)
- **Success**: Dark Green (#73BF69)
- **Background**: Dark theme for professional look

### Layout
- **Top Row**: Most important metrics (status, counts)
- **Middle**: Time-series charts (trends, analysis)
- **Bottom**: Detailed tables and heat maps
- **Consistent spacing**: 24-column grid

### Refresh Rates
- **Executive**: 10s (real-time feel)
- **Operations**: 10s (catch issues fast)
- **Security**: 10s (threat detection)

### Time Ranges
- **Executive**: 1h (recent activity)
- **Operations**: 1h (performance monitoring)
- **Security**: 6h (trend analysis)

---

## 📊 Metrics Reference

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

## 🎯 Dashboard Variables

All dashboards support:
- **`${datasource}`**: Prometheus/VictoriaMetrics datasource selector

Future enhancements:
- **`${namespace}`**: Filter by namespace
- **`${cluster}`**: Multi-cluster support
- **`${severity}`**: Filter by severity level

---

## 🔧 Customization Tips

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
    {"color": "yellow", "value": 10},  // Adjust these values
    {"color": "red", "value": 50}
  ]
}
```

---

## 📈 Alert Integration

Dashboards work seamlessly with Prometheus alerts (see `../monitoring/prometheus-rules.yaml`):

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

## 🎊 Pro Tips for Demos

### Before the Demo
1. Run `./hack/quick-demo.sh` 5 minutes early
2. Open all 3 dashboards in separate tabs
3. Set refresh to 5s for extra-live feel
4. Zoom to "Last 15 minutes" for dense data

### During the Demo
1. **Start with Executive** - Big picture, wow factor
2. **Zoom into Operations** - Show technical depth
3. **End with Security** - Show domain expertise
4. **Use fullscreen mode** (F11) for maximum impact

### Talking Points
- "All 6 security tools integrated out of the box"
- "Real-time event aggregation with zero lag"
- "Production-ready observability from day one"
- "Kubernetes-native, no external dependencies"
- "Beautiful dashboards that actually help"

### Common Questions
**Q**: "How do you aggregate from 6 different tools?"  
**A**: "Each tool has a dedicated adapter. Show Operations dashboard → Adapter Run Rate panel"

**Q**: "What's the performance overhead?"  
**A**: "Minimal! Show Operations dashboard → Processing Latency (p95 < 100ms)"

**Q**: "Can I add custom tools?"  
**A**: "Yes! ObservationMapping CRD. Show Security dashboard → Source Distribution"

---

## 🌟 What Makes These Dashboards Special

1. **✨ Beautiful**: Professional design, consistent colors, clean layout
2. **🚀 Fast**: 10s refresh, real-time feel, no lag
3. **📊 Informative**: Every panel tells a story, no noise
4. **🎯 Actionable**: See problems, understand cause, know what to do
5. **🎨 Persona-focused**: Executive, Operations, Security - each gets what they need
6. **🔥 Demo-ready**: 4 minutes to wow, guaranteed!

---

## 📚 Additional Resources

- **Metrics Documentation**: `../../docs/PERFORMANCE.md`
- **Alert Rules**: `../monitoring/prometheus-rules.yaml`
- **Quick Demo Script**: `../../hack/quick-demo.sh`
- **Architecture**: `../../docs/ARCHITECTURE.md`

---

**Built with ❤️ for the Kubernetes community**

*These dashboards are designed to make your eyes shine in just 4 minutes!* ✨🎉
