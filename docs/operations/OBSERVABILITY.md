# Observability Guide

## Overview

zen-watcher exposes comprehensive Prometheus metrics for monitoring pipeline performance, optimization decisions, and error rates. All metrics are available at the `/metrics` HTTP endpoint.

## Metrics Endpoint

**Location**: `http://<zen-watcher-pod>:8080/metrics`

**Format**: Prometheus text format

**Example**:
```bash
# From within cluster
kubectl port-forward -n zen-system deployment/zen-watcher 8080:8080
curl http://localhost:8080/metrics

# Or directly from pod
kubectl exec -n zen-system zen-watcher-0 -- curl -s http://localhost:8080/metrics
```

## Metric Categories

### Per-Source / Per-Ingester Metrics

#### Events Processed
- **`zen_watcher_optimization_source_events_processed_total{source="<source>"}`**
  - Total number of events processed per source
  - Type: Counter
  - Labels: `source`

#### Dropped Events (Filter)
- **`zen_watcher_observations_filtered_total{source="<source>",reason="<reason>"}`**
  - Total number of events filtered out
  - Type: Counter
  - Labels: `source`, `reason`

#### Deduplicated Events
- **`zen_watcher_optimization_source_events_deduped_total{source="<source>"}`**
  - Total number of events deduplicated per source
  - Type: Counter
  - Labels: `source`

#### Deduplication Strategy Metrics (W33 - v1.1)
- **`zen_watcher_dedup_effectiveness_per_strategy{strategy="<strategy>",source="<source>"}`**
  - Deduplication effectiveness per strategy (ratio of dropped to total events)
  - Type: Gauge
  - Labels: `strategy`, `source`
  - Strategies: `fingerprint`, `event-stream`, `key`
  - Example: `zen_watcher_dedup_effectiveness_per_strategy{strategy="fingerprint",source="trivy"}`

- **`zen_watcher_dedup_decisions_total{strategy="<strategy>",source="<source>",decision="<decision>"}`**
  - Total deduplication decisions (create or drop) by strategy and source
  - Type: Counter
  - Labels: `strategy`, `source`, `decision`
  - Decision values: `create`, `drop`
  - Example: `zen_watcher_dedup_decisions_total{strategy="event-stream",source="kubernetes-events",decision="drop"}`

### Processing Order Metrics

#### Current Strategy
- **`zen_watcher_optimization_current_strategy{source="<source>"}`**
  - Current processing order per source (filter_first, dedup_first)
  - Type: Gauge
  - Values: `1` = filter_first, `2` = dedup_first
  - Labels: `source`

#### Strategy Switches
- **`zen_watcher_optimization_strategy_changes_total{source="<source>",old_strategy="<old>",new_strategy="<new>"}`**
  - Total number of processing strategy changes
  - Type: Counter
  - Labels: `source`, `old_strategy`, `new_strategy`

#### Optimization Decisions
- **`zen_watcher_optimization_decisions_total{source="<source>",decision_type="<type>",strategy="<strategy>"}`**
  - Total number of optimization decisions made
  - Type: Counter
  - Labels: `source`, `decision_type`, `strategy`

#### Optimization Confidence
- **`zen_watcher_optimization_confidence{source="<source>"}`**
  - Confidence level of optimization decisions (0.0-1.0)
  - Type: Gauge
  - Labels: `source`

#### Filter Effectiveness
- **`zen_watcher_optimization_filter_effectiveness_ratio{source="<source>"}`**
  - Filter effectiveness ratio per source (0.0-1.0)
  - Type: Gauge
  - Labels: `source`

#### Deduplication Rate
- **`zen_watcher_optimization_deduplication_rate_ratio{source="<source>"}`**
  - Deduplication rate ratio per source (0.0-1.0)
  - Type: Gauge
  - Labels: `source`

#### Observations Per Minute
- **`zen_watcher_optimization_observations_per_minute{source="<source>"}`**
  - Observations created per minute per source
  - Type: Gauge
  - Labels: `source`

### Webhook Metrics

#### Webhook Requests
- **`zen_watcher_webhook_requests_total{endpoint="<endpoint>",status="<status>"}`**
  - Total number of webhook requests received by endpoint and HTTP status code
  - Type: Counter
  - Labels: `endpoint` (e.g., "falco", "audit"), `status` (HTTP status code as string)
  - Status codes tracked:
    - `200` - Success
    - `400` - Bad Request (invalid JSON, parse errors)
    - `401` - Unauthorized (authentication failures)
    - `405` - Method Not Allowed (invalid HTTP method)
    - `413` - Request Entity Too Large (request body exceeds size limit)
    - `429` - Too Many Requests (rate limit exceeded)
    - `503` - Service Unavailable (channel buffer full, backpressure)
  - Example: `zen_watcher_webhook_requests_total{endpoint="falco",status="401"}`

#### Webhook Events Dropped
- **`zen_watcher_webhook_events_dropped_total{endpoint="<endpoint>"}`**
  - Total number of webhook events dropped due to channel buffer full (backpressure)
  - Type: Counter
  - Labels: `endpoint`
  - Example: `zen_watcher_webhook_events_dropped_total{endpoint="falco"}`

#### Rate Limit Rejections
- **`zen_watcher_webhook_rate_limit_rejections_total{endpoint="<endpoint>",scope="<scope>"}`**
  - Total number of webhook requests rejected due to rate limiting
  - Type: Counter
  - Labels: `endpoint` (endpoint identifier), `scope` (rate limit scope: `"endpoint"` or `"ip"`)
  - Use this metric to monitor rate limit effectiveness per scope
  - Example: `zen_watcher_webhook_rate_limit_rejections_total{endpoint="security-alerts",scope="endpoint"}`
  - See: [Rate Limiting Guide](RATE_LIMITING.md) for details

### Pipeline Errors

#### Errors by Stage
- **`zen_watcher_pipeline_errors_total{source="<source>",stage="<stage>",error_type="<type>"}`**
  - Total number of pipeline errors by stage
  - Type: Counter
  - Labels: `source`, `stage`, `error_type`
  - Stages: `filter`, `dedup`, `normalize`, `write`

#### Observation Creation Errors
- **`zen_watcher_observations_create_errors_total{source="<source>",error_type="<type>"}`**
  - Total number of Observation CRD creation errors
  - Type: Counter
  - Labels: `source`, `error_type`

### Processing Performance

#### Processing Latency
- **`zen_watcher_optimization_source_processing_latency_seconds{source="<source>"}`**
  - Processing latency per source in seconds
  - Type: Histogram
  - Labels: `source`
  - Buckets: [0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0]

#### Event Processing Duration
- **`zen_watcher_event_processing_duration_seconds{source="<source>",processor_type="<type>"}`**
  - Time taken to process and create an Observation
  - Type: Histogram
  - Labels: `source`, `processor_type`

## Prometheus Scraping Configuration

### ServiceMonitor (if using Prometheus Operator)

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: zen-watcher
  namespace: zen-system
spec:
  selector:
    matchLabels:
      app: zen-watcher
  endpoints:
  - port: http
    path: /metrics
    interval: 30s
```

### Prometheus Config (static)

```yaml
scrape_configs:
  - job_name: 'zen-watcher'
    kubernetes_sd_configs:
      - role: pod
        namespaces:
          names:
            - zen-system
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_label_app]
        regex: zen-watcher
        action: keep
      - source_labels: [__meta_kubernetes_pod_ip]
        action: replace
        target_label: __address__
        replacement: $1:8080
      - source_labels: [__meta_kubernetes_pod_name]
        action: replace
        target_label: pod
```

## Interpreting Processing Order Metrics

### Strategy Selection

Choose `filter_first` or `dedup_first` based on:

1. **Low Severity Percentage** (`zen_watcher_low_severity_percent`)
   - If > 70% → `filter_first` (filter out noise early)
   - Metric: `zen_watcher_low_severity_percent{source="trivy"}`

2. **Deduplication Effectiveness** (`zen_watcher_optimization_deduplication_rate_ratio`)
   - If > 50% → `dedup_first` (remove duplicates early)
   - Metric: `zen_watcher_optimization_deduplication_rate_ratio{source="cert-manager"}`

### Monitoring Strategy Changes

Query for strategy changes:
```promql
rate(zen_watcher_optimization_strategy_changes_total[5m])
```

Query current strategy per source:
```promql
zen_watcher_optimization_current_strategy
```

### Optimization Health

Monitor optimization confidence:
```promql
zen_watcher_optimization_confidence
```

Values close to 1.0 indicate high confidence in optimization decisions.

## Example Queries

### Events Processed Per Source (Rate)
```promql
rate(zen_watcher_optimization_source_events_processed_total[5m])
```

### Filter Effectiveness
```promql
zen_watcher_optimization_filter_effectiveness_ratio
```

### Deduplication Rate
```promql
zen_watcher_optimization_deduplication_rate_ratio
```

### Deduplication Strategy Effectiveness (W33 - v1.1)
```promql
# Effectiveness per strategy
zen_watcher_dedup_effectiveness_per_strategy

# Compare strategies for a specific source
zen_watcher_dedup_effectiveness_per_strategy{source="trivy"}

# Total decisions per strategy
sum by (strategy) (zen_watcher_dedup_decisions_total)

# Drop rate per strategy
sum by (strategy) (zen_watcher_dedup_decisions_total{decision="drop"}) / 
sum by (strategy) (zen_watcher_dedup_decisions_total)
```

### Pipeline Error Rate
```promql
rate(zen_watcher_pipeline_errors_total[5m])
```

### Processing Latency (P95)
```promql
histogram_quantile(0.95, zen_watcher_optimization_source_processing_latency_seconds_bucket)
```

### Strategy Distribution
```promql
zen_watcher_optimization_current_strategy
```

## Alerting Recommendations

### High Error Rate
```yaml
- alert: ZenWatcherHighPipelineErrorRate
  expr: rate(zen_watcher_pipeline_errors_total[5m]) > 0.1
  for: 5m
  annotations:
    summary: "High pipeline error rate in zen-watcher"
```

### Strategy Oscillation
```yaml
- alert: ZenWatcherStrategyOscillation
  expr: rate(zen_watcher_optimization_strategy_changes_total[10m]) > 2
  for: 5m
  annotations:
    summary: "Frequent strategy changes detected"
```

### Low Optimization Confidence
```yaml
- alert: ZenWatcherLowOptimizationConfidence
  expr: zen_watcher_optimization_confidence < 0.5
  for: 10m
  annotations:
    summary: "Low confidence in optimization decisions"
```

### High Webhook Authentication Failures
```yaml
- alert: ZenWatcherHighAuthFailures
  expr: rate(zen_watcher_webhook_requests_total{status="401"}[5m]) > 1
  for: 5m
  annotations:
    summary: "High rate of webhook authentication failures"
    description: "Rate of 401 Unauthorized responses is above threshold - possible attack or misconfiguration"
```

### High Webhook Rate Limit Rejections
```yaml
- alert: ZenWatcherHighRateLimitRejections
  expr: rate(zen_watcher_webhook_requests_total{status="429"}[5m]) > 10
  for: 5m
  annotations:
    summary: "High rate of webhook rate limit rejections"
    description: "Rate of 429 Too Many Requests responses is above threshold - possible DoS attempt or misconfigured upstream"
```

## CLI Tools

**Query Observations**: Use `obsctl` CLI for querying Observations without external tools. See [TOOLING_GUIDE.md](TOOLING_GUIDE.md#obsctl) for details.

## Future Improvements

This section outlines recommended metrics enhancements for better observability. Current metrics are production-ready; these are enhancements for future releases.

### Missing Critical Metrics

1. **Ingester-Specific Metrics**
   - Ingester CRD count (active/inactive)
   - Ingester type distribution (informer/webhook/logs)
   - Per-ingester event throughput
   - Per-ingester error rates
   - Ingester configuration validation failures
   - Ingester destination delivery metrics (per destination type)

2. **Filter Enhancement Metrics**
   - Filter rule evaluation time (histogram)
   - Filter rule effectiveness (ratio of events filtered per rule)
   - Filter chain depth (how many rules evaluated)
   - Filter cache hit/miss (if caching is implemented)
   - Global namespace filter impact

3. **Dedup Enhancement Metrics**
   - Dedup window size per source (gauge)
   - Dedup cache size per source (gauge)
   - Dedup cache hit/miss ratio
   - Dedup fingerprint generation latency
   - Dedup strategy performance comparison

4. **Mapping/Normalization Metrics**
   - Field mapping transformation success/failure rates
   - Normalization rule match rates
   - Priority mapping accuracy
   - Field extraction success rates
   - Mapping rule evaluation latency

5. **Destination Metrics**
   - Destination delivery success/failure rates
   - Destination delivery latency
   - Destination queue depth
   - Destination retry counts
   - Destination rate limiting hits

### Implementation Priority

**High Priority (Critical for Operations)**
1. Ingester Status & Health Metrics
2. Destination Delivery Metrics
3. ConfigManager Metrics

**Medium Priority (Important for Optimization)**
1. Filter Rule Metrics
2. Dedup Cache Details
3. Mapping/Normalization Metrics

**Low Priority (Nice to Have)**
1. Worker Pool Metrics (only if enabled)
2. Event Batching Metrics (only if enabled)
3. Optimization Quality Scores

See the "Future Improvements" section above for detailed metric specifications and implementation guidance.

## Related Documentation

- [PERFORMANCE.md](PERFORMANCE.md) - Performance characteristics, benchmarks, and tuning
- [OPERATIONAL_EXCELLENCE.md](OPERATIONAL_EXCELLENCE.md) - Operations best practices

