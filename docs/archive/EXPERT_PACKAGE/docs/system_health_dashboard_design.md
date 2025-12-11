---
⚠️ HISTORICAL DOCUMENT - EXPERT PACKAGE ARCHIVE ⚠️

This document is from an external "Expert Package" analysis of zen-watcher/ingester.
It reflects the state of zen-watcher at a specific point in time and may be partially obsolete.

CANONICAL SOURCES (use these for current direction):
- docs/PM_AI_ROADMAP.md - Current roadmap and priorities
- CONTRIBUTING.md - Current quality bar and standards
- docs/INFORMERS_CONVERGENCE_NOTES.md - Current informer architecture
- docs/STRESS_TEST_RESULTS.md - Current performance baselines

This archive document is provided for historical context, rationale, and inspiration only.
Do NOT use this as a replacement for current documentation.

---

# Zen Watcher System Health Dashboard Design

## Executive Summary

The Zen Watcher System Health Dashboard is designed specifically for DevOps engineers and Site Reliability Engineers (SREs) to monitor cluster-wide health, critical services status, and system performance metrics. This dashboard provides deep operational insights into the Zen Watcher platform's health, resource utilization, and performance characteristics.

## Dashboard Overview

### Target Audience
- **Primary**: DevOps Engineers, Site Reliability Engineers (SREs)
- **Secondary**: Platform Engineers, Infrastructure Teams
- **Use Cases**: Production monitoring, incident response, capacity planning, performance tuning

### Key Objectives
1. **Real-time System Health Monitoring**: Immediate visibility into cluster-wide health status
2. **Critical Services Status Tracking**: Monitor all Zen Watcher components and dependencies
3. **Performance Metrics Analysis**: Deep dive into system performance and resource utilization
4. **Capacity Planning Support**: Data-driven insights for scaling and resource allocation
5. **Incident Response Tool**: Quick identification and diagnosis of system issues

## Dashboard Architecture

### Layout Structure
- **Grid System**: 24-column layout optimized for wide monitors
- **Refresh Rate**: 5 seconds for real-time monitoring
- **Time Range**: Default 1 hour with 24h/7d views for trend analysis
- **Theme**: Dark theme optimized for 24/7 operations center environments

### Visual Hierarchy
1. **Top Row**: Critical system status indicators
2. **Upper Section**: Cluster-wide health metrics
3. **Middle Section**: Service-specific performance metrics
4. **Lower Section**: Resource utilization and capacity metrics
5. **Bottom**: Detailed logs and troubleshooting information

## Dashboard Sections

### 1. Cluster-Wide Health Overview (Top Section)

#### 1.1 System Status Matrix
- **Panel Type**: Status grid
- **Metrics**: 
  - `up{job="zen-watcher"}` - Service availability
  - `zen_watcher_tools_active{tool}` - Active monitoring tools
  - `zen_watcher_informer_cache_synced{resource}` - Cache synchronization status
- **Visual**: Traffic light indicators (Green/Yellow/Red)
- **Purpose**: Immediate identification of system-wide issues

#### 1.2 Critical Service Health
- **Panel Type**: Single stat with trend
- **Metrics**:
  - `avg(up{job=~"zen-watcher.*"})` - Overall system uptime
  - `zen_watcher_observations_created_total` - Event processing health
  - `zen_watcher_webhook_queue_usage_ratio` - Integration health
- **Thresholds**: 
  - Green: >99.5% uptime
  - Yellow: 95-99.5% uptime  
  - Red: <95% uptime

#### 1.3 Cluster Resource Overview
- **Panel Type**: Gauge charts
- **Metrics**:
  - `cluster_cpu_usage_percent` - CPU utilization across cluster
  - `cluster_memory_usage_percent` - Memory utilization across cluster
  - `cluster_disk_usage_percent` - Storage utilization across cluster
  - `cluster_network_throughput_mbps` - Network utilization
- **Purpose**: High-level cluster resource health

### 2. Service-Specific Health (Middle Section)

#### 2.1 Zen Watcher Core Services
- **Panel Type**: Time series with annotations
- **Services Monitored**:
  - Main Zen Watcher service
  - Kubernetes informer
  - Webhook processors
  - Database connections (etcd)
- **Metrics**:
  - `up{job="zen-watcher"}` - Service availability
  - `zen_watcher_service_restarts_total` - Service restart frequency
  - `zen_watcher_event_processing_duration_seconds` - Processing latency
- **Alert Integration**: Prometheus alert annotations

#### 2.2 Monitoring Tool Adapters
- **Panel Type**: Heatmap with service status
- **Tools**: Falco, Trivy, Kube-Bench, Kyverno, Audit Webhook
- **Metrics**:
  - `zen_watcher_adapter_status{adapter}` - Adapter health status
  - `zen_watcher_adapter_events_processed_total{adapter}` - Event processing volume
  - `zen_watcher_adapter_error_rate{adapter}` - Adapter error rates
- **Visual**: Color-coded heatmap showing adapter health

#### 2.3 Data Pipeline Health
- **Panel Type**: Flow diagram with metrics
- **Pipeline Stages**:
  1. Event Ingestion
  2. Filtering
  3. Deduplication
  4. Processing
  5. Storage/Observation Creation
- **Metrics**:
  - `zen_watcher_events_ingestion_rate` - Events per second
  - `zen_watcher_filtering_efficiency_percent` - Filter hit rate
  - `zen_watcher_dedup_cache_hit_rate` - Cache efficiency
  - `zen_watcher_processing_throughput` - Events processed per second
- **Purpose**: Identify bottlenecks in the data processing pipeline

### 3. Performance & Resource Metrics (Lower Section)

#### 3.1 System Performance Metrics
- **Panel Type**: Multi-panel time series
- **Metrics**:
  - `zen_watcher_event_processing_latency_seconds{p95}` - 95th percentile processing time
  - `zen_watcher_event_processing_latency_seconds{p99}` - 99th percentile processing time
  - `zen_watcher_queue_depth{queue_name}` - Queue depths across all queues
  - `zen_watcher_thread_pool_utilization_percent` - Thread pool usage
- **Alert Lines**: Warning and critical thresholds
- **Purpose**: Monitor system performance under load

#### 3.2 Resource Utilization
- **Panel Type**: Stacked area charts and single stats
- **Metrics**:
  - `process_resident_memory_bytes{job="zen-watcher"}` - Memory usage
  - `process_cpu_usage_percent{job="zen-watcher"}` - CPU usage
  - `zen_watcher_gc_duration_seconds{p95}` - Garbage collection impact
  - `zen_watcher_dedup_cache_memory_usage_bytes` - Cache memory usage
- **Purpose**: Track resource consumption patterns

#### 3.3 Storage & Database Health
- **Panel Type**: Tables and time series
- **Metrics**:
  - `etcd_db_size_bytes` - etcd storage usage
  - `etcd_wal_fsync_duration_seconds` - Write performance
  - `zen_watcher_observations_live{source}` - Live observation count
  - `zen_watcher_storage_growth_rate_bytes_per_hour` - Storage growth rate
- **Purpose**: Monitor database performance and storage trends

### 4. Advanced Analytics (Bottom Section)

#### 4.1 Capacity Planning Dashboard
- **Panel Type**: Prediction charts
- **Metrics**:
  - Storage growth trends with projections
  - CPU/Memory usage trends with capacity warnings
  - Event volume growth patterns
  - Resource utilization forecasts
- **Purpose**: Proactive capacity planning

#### 4.2 Error Analysis & Troubleshooting
- **Panel Type**: Error rate charts and log analysis
- **Metrics**:
  - `zen_watcher_error_rate_by_type` - Error categorization
  - `zen_watcher_failed_observations_total` - Failed processing attempts
  - `zen_watcher_alerts_firing` - Active alert count
- **Log Integration**: Links to detailed logs in Grafana Explore

#### 4.3 Dependency Health
- **Panel Type**: Service dependency graph
- **Dependencies**:
  - Kubernetes API Server
  - etcd database
  - Prometheus/Grafana stack
  - Monitoring tool endpoints
- **Metrics**:
  - `up{job="kubernetes-api"}` - K8s API availability
  - `etcd_cluster_health` - etcd cluster health
  - `zen_watcher_webhook_connectivity_success_rate` - External connectivity

## Metrics Reference

### Core Health Metrics
```promql
# System availability
up{job="zen-watcher"}

# Service restart detection
increase(zen_watcher_service_restarts_total[5m])

# Cache synchronization
zen_watcher_informer_cache_synced{resource}

# Tool connectivity
zen_watcher_tools_active{tool}
```

### Performance Metrics
```promql
# Processing latency percentiles
histogram_quantile(0.95, zen_watcher_event_processing_duration_seconds_bucket)
histogram_quantile(0.99, zen_watcher_event_processing_duration_seconds_bucket)

# Throughput calculation
rate(zen_watcher_observations_created_total[1m])

# Queue depth monitoring
zen_watcher_queue_depth{queue_name}

# Cache efficiency
rate(zen_watcher_dedup_cache_hits_total[5m]) / rate(zen_watcher_dedup_cache_requests_total[5m])
```

### Resource Metrics
```promql
# Memory usage
process_resident_memory_bytes{job="zen-watcher"} / 1024 / 1024 / 1024  # GB

# CPU usage
rate(process_cpu_seconds_total{job="zen-watcher"}[5m]) * 100

# GC impact
zen_watcher_gc_duration_seconds{p95}

# Storage usage
zen_watcher_observations_live{source}
```

### Capacity Planning Metrics
```promql
# Storage growth rate
rate(etcd_db_size_bytes[1h])

# Event volume trends
rate(zen_watcher_events_total[24h])

# Resource utilization trends
avg_over_time(process_cpu_usage_percent{job="zen-watcher"}[24h])

# Capacity predictions
predict_linear(etcd_db_size_bytes[24h], 24*7)  # 7-day projection
```

## Alert Integration

### Critical Alerts (Red Indicators)
- `ZenWatcherDown` - Service unavailable
- `ZenWatcherCriticalErrorRate` - Error rate >5%
- `ZenWatcherHighLatency` - p95 latency >1s
- `ZenWatcherStorageFull` - Storage usage >90%

### Warning Alerts (Yellow Indicators)
- `ZenWatcherHighResourceUsage` - CPU/Memory >80%
- `ZenWatcherToolOffline` - Monitoring tool disconnected
- `ZenWatcherSlowProcessing` - Processing queue backlog
- `ZenWatcherHighFilterRate` - Filter drop rate >50%

### Info Alerts (Blue Indicators)
- `ZenWatcherPerformanceDegradation` - Gradual performance decline
- `ZenWatcherCapacityWarning` - Approaching resource limits
- `ZenWatcherMaintenanceWindow` - Scheduled maintenance notifications

## Dashboard Variables

### Standard Variables
- `${datasource}`: Prometheus/VictoriaMetrics datasource
- `${cluster}`: Multi-cluster selector (future enhancement)
- `${namespace}`: Namespace filter for multi-tenant setups
- `${service}`: Service-specific filtering

### Advanced Variables (Phase 2)
- `${severity}`: Filter by event severity
- `${tool}`: Filter by monitoring tool
- `${time_range}`: Quick time range selection
- `${environment}`: Dev/Stage/Prod environment filter

## Customization Options

### Thresholds Configuration
```json
"thresholds": {
  "steps": [
    {"color": "green", "value": null},
    {"color": "yellow", "value": 70},
    {"color": "orange", "value": 85},
    {"color": "red", "value": 95}
  ]
}
```

### Panel Layout Modifications
- Drag-and-drop panel repositioning
- Custom panel sizing (1x1 to 12x6 grid units)
- Collapsible sections for focused monitoring
- Full-screen mode for incident response

### Integration Points
- **PagerDuty**: Direct incident creation from critical panels
- **Slack**: Real-time notifications to operations channels
- **Jira**: Automatic ticket creation for recurring issues
- **StatusPage**: Public status page updates

## Performance Considerations

### Dashboard Optimization
- **Data Retention**: 13 months of metrics data
- **Query Optimization**: Use recording rules for complex calculations
- **Caching**: 30-second dashboard query caching
- **Refresh Strategy**: Progressive loading for large datasets

### Scalability Features
- **Multi-cluster Support**: Unified view across clusters
- **Regional Aggregation**: Cluster-level rollup metrics
- **Historical Analysis**: Trend analysis with statistical overlays
- **Anomaly Detection**: ML-powered anomaly identification

## Implementation Roadmap

### Phase 1: Core Health Dashboard (Weeks 1-2)
- [ ] Cluster-wide health overview
- [ ] Critical service status monitoring
- [ ] Basic performance metrics
- [ ] Alert integration

### Phase 2: Advanced Analytics (Weeks 3-4)
- [ ] Capacity planning features
- [ ] Error analysis tools
- [ ] Dependency health mapping
- [ ] Performance trending

### Phase 3: Enhanced Integration (Weeks 5-6)
- [ ] Multi-cluster support
- [ ] Advanced alerting rules
- [ ] External system integration
- [ ] Mobile responsive design

### Phase 4: Automation & AI (Weeks 7-8)
- [ ] Automated issue detection
- [ ] Predictive capacity planning
- [ ] Intelligent alert correlation
- [ ] Self-healing recommendations

## Technical Requirements

### Grafana Configuration
- **Version**: 9.0+ with Prometheus data source
- **Plugins**: 
  - Grafana WorldMap Panel
  - Pie Chart Panel
  - Histogram Panel
  - Stat Panel
- **Performance**: 
  - Dashboard load time <3 seconds
  - Query response time <1 second
  - Memory usage <2GB per dashboard

### Prometheus Setup
- **Recording Rules**: Pre-calculated metrics for dashboard queries
- **Retention**: 15-minute resolution for 90 days
- **Remote Write**: Long-term storage integration
- **High Availability**: Dual Prometheus setup

### Monitoring Stack Integration
- **AlertManager**: Integrated alert routing
- **VictoriaMetrics**: High-performance metrics storage
- **Loki**: Centralized log aggregation
- **Tempo**: Distributed tracing integration

## Success Metrics

### Operational Metrics
- **Dashboard Adoption**: >90% of SREs use daily
- **Mean Time to Detection (MTTD)**: <2 minutes for critical issues
- **Mean Time to Resolution (MTTR)**: 30% improvement
- **Alert Noise Reduction**: 50% fewer false positives

### Technical Metrics
- **Dashboard Performance**: <3 second load time
- **Query Efficiency**: <1 second average query time
- **Data Accuracy**: 99.9% metric data integrity
- **Availability**: 99.99% dashboard uptime

## Maintenance & Support

### Regular Updates
- **Weekly**: Alert rule review and tuning
- **Monthly**: Dashboard usage analytics review
- **Quarterly**: Performance optimization assessment
- **Annually**: Complete dashboard redesign review

### Documentation
- **User Guide**: Detailed dashboard usage instructions
- **Troubleshooting Guide**: Common issues and solutions
- **Metrics Catalog**: Complete reference of all metrics
- **Alert Playbook**: Response procedures for each alert type

## Conclusion

The Zen Watcher System Health Dashboard provides DevOps engineers and SREs with comprehensive visibility into cluster health, service status, and performance metrics. By focusing on real-time monitoring, proactive alerting, and capacity planning, this dashboard enables teams to maintain high system availability and performance standards.

The phased implementation approach ensures rapid value delivery while allowing for continuous improvement based on operational feedback and evolving requirements.