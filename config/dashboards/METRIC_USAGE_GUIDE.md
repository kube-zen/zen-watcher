# Metric Usage Guide for Zen Watcher Dashboards

## Metric Definitions

### `zen_watcher_events_total`
**Purpose**: Security event analysis and filtering  
**Labels**: `source`, `category`, `severity`, `eventType`, `namespace`, `kind`  
**Description**: Total number of events that resulted in Observation CRD creation (after filtering and deduplication)  
**Use Cases**:
- Security dashboards (filtering by severity, category)
- Event analysis (by eventType, namespace, kind)
- Threat intelligence (severity-based filtering)
- Compliance reporting (category-based analysis)

**Example Queries**:
```promql
# Critical events in last hour
sum(increase(zen_watcher_events_total{severity="CRITICAL"}[1h]))

# Events by category
sum by (category) (rate(zen_watcher_events_total[5m]))

# Security events by source
sum by (source) (increase(zen_watcher_events_total{category="security"}[24h]))
```

---

### `zen_watcher_observations_created_total`
**Purpose**: Operational metrics and system health  
**Labels**: `source`  
**Description**: Total number of Observation CRDs successfully created  
**Use Cases**:
- Throughput monitoring
- Success rate calculations
- System performance metrics
- Source-level operational stats

**Example Queries**:
```promql
# Throughput (observations per minute)
sum(rate(zen_watcher_observations_created_total[1m])) * 60

# Success rate
100 * (1 - (sum(rate(zen_watcher_observations_create_errors_total[5m])) / 
  (sum(rate(zen_watcher_observations_created_total[5m])) + 
   sum(rate(zen_watcher_observations_create_errors_total[5m])) + 0.001)))

# Observations by source (24h)
sum by (source) (increase(zen_watcher_observations_created_total[24h]))
```

---

## Dashboard Standardization Rules

### Security Dashboard (`zen-watcher-security.json`)
**Use**: `zen_watcher_events_total`  
**Rationale**: Security analysis requires filtering by severity, category, eventType, namespace, and kind

### Operations Dashboard (`zen-watcher-operations.json`)
**Use**: `zen_watcher_observations_created_total` for:
- Throughput metrics
- Success rate calculations
- Source-level operational stats

**Use**: `zen_watcher_events_total` for:
- Resource kind analysis (needs `kind` label)
- Event type breakdowns (needs `eventType` label)

### Executive Dashboard (`zen-watcher-executive.json`)
**Use**: `zen_watcher_events_total` for:
- Event counts (Critical, High, Medium)
- Event trends by severity
- Event analysis by category/eventType

**Use**: `zen_watcher_observations_created_total` for:
- Total observations count (24h)
- System health metrics
- Success rate

---

## Decision Matrix

| Dashboard Panel Type | Metric to Use | Reason |
|---------------------|---------------|--------|
| Security event counts | `zen_watcher_events_total` | Needs severity/category labels |
| Event filtering/analysis | `zen_watcher_events_total` | Rich labels for filtering |
| Throughput/rate | `zen_watcher_observations_created_total` | Operational metric |
| Success rate | `zen_watcher_observations_created_total` | Needs error metric comparison |
| Resource kind analysis | `zen_watcher_events_total` | Has `kind` label |
| Event type breakdown | `zen_watcher_events_total` | Has `eventType` label |
| Source-level ops stats | `zen_watcher_observations_created_total` | Simpler, operational focus |
| Total observations | `zen_watcher_observations_created_total` | System-level metric |

---

## Migration Notes

When updating dashboards:
1. **Security/Event Analysis** → Use `zen_watcher_events_total` with appropriate label filters
2. **Operational Metrics** → Use `zen_watcher_observations_created_total` for throughput/success rates
3. **Mixed Use Cases** → Use `zen_watcher_events_total` when you need rich labels (severity, category, eventType, namespace, kind)
4. **Simple Counts** → Use `zen_watcher_observations_created_total` for basic operational metrics

---

**Last Updated**: 2025-12-08

