# Observability Review & Recommendations

## Executive Summary

This document reviews the current observability/metrics implementation in zen-watcher and identifies gaps, improvements, and recommendations for better monitoring of ingesters, filters, dedup, processing order, mapping, and other critical components.

## Current Metrics Coverage

### ✅ Well-Covered Areas

1. **Core Event Processing**
   - `zen_watcher_events_total` - Events by source, category, severity, eventType, namespace, kind, strategy
   - `zen_watcher_observations_created_total` - Observations created by source
   - `zen_watcher_observations_filtered_total` - Filtered by source and reason
   - `zen_watcher_observations_deduped_total` - Deduplicated (global counter)
   - `zen_watcher_observations_create_errors_total` - Creation errors by source and type

2. **Optimization Metrics**
   - `zen_watcher_optimization_*` - Comprehensive optimization metrics
   - `zen_watcher_optimization_source_*` - Per-source optimization metrics
   - Strategy changes, confidence, adaptive adjustments tracked

3. **Dedup Metrics**
   - `zen_watcher_dedup_effectiveness_per_strategy` - Effectiveness by strategy
   - `zen_watcher_dedup_decisions_total` - Decisions by strategy and decision type
   - `zen_watcher_dedup_cache_usage_ratio` - Cache utilization
   - `zen_watcher_dedup_evictions_total` - Cache evictions

4. **GC Metrics**
   - `zen_watcher_gc_runs_total` - GC runs
   - `zen_watcher_gc_duration_seconds` - GC duration by operation
   - `zen_watcher_observations_live` - Live observations by source
   - `zen_watcher_observations_deleted_total` - Deletions by source and reason

### ⚠️ Partially Covered Areas

1. **Filter Metrics**
   - ✅ `zen_watcher_filter_decisions_total` - Decisions by source, action, reason
   - ✅ `zen_watcher_filter_reload_total` - Reloads by source and result
   - ✅ `zen_watcher_filter_last_reload_timestamp_seconds` - Last reload time
   - ✅ `zen_watcher_filter_policies_active` - Active policies by type
   - ❌ **MISSING**: Filter rule evaluation latency
   - ❌ **MISSING**: Filter rule hit/miss rates per rule
   - ❌ **MISSING**: Filter configuration validation errors
   - ❌ **MISSING**: Global namespace filter metrics

2. **Ingester Lifecycle Metrics**
   - ✅ `zen_watcher_adapter_runs_total` - Adapter runs by adapter and outcome
   - ❌ **MISSING**: Ingester CRD status (active/inactive/error)
   - ❌ **MISSING**: Ingester configuration load errors
   - ❌ **MISSING**: Ingester startup/shutdown events
   - ❌ **MISSING**: Per-ingester event processing rates
   - ❌ **MISSING**: Ingester health status (gauge)
   - ❌ **MISSING**: Ingester last successful event timestamp
   - ❌ **MISSING**: Informer cache sync duration per ingester
   - ❌ **MISSING**: Informer resync events per ingester

3. **Mapping/Normalization Metrics**
   - ❌ **MISSING**: Field mapping transformation latency
   - ❌ **MISSING**: Field mapping transformation errors
   - ❌ **MISSING**: Normalization rule application counts
   - ❌ **MISSING**: Priority mapping hit/miss rates
   - ❌ **MISSING**: Field extraction failures

### ❌ Missing Critical Metrics

1. **Ingester-Specific Metrics**
   - Ingester CRD count (active/inactive)
   - Ingester type distribution (informer/webhook/logs/k8s-events)
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

4. **Processing Order Metrics**
   - Current processing order per source
   - Strategy performance metrics

5. **Mapping/Normalization Metrics**
   - Field mapping transformation success/failure rates
   - Normalization rule match rates
   - Priority mapping accuracy
   - Field extraction success rates
   - Mapping rule evaluation latency

6. **Destination Metrics**
   - Destination delivery success/failure rates
   - Destination delivery latency
   - Destination queue depth
   - Destination retry counts
   - Destination rate limiting hits

7. **ConfigManager Metrics**
   - ConfigMap load success/failure
   - ConfigMap reload latency
   - ConfigMap merge conflicts
   - ConfigMap validation errors
   - Config update propagation time

8. **Worker Pool Metrics** (if enabled)
   - Worker pool queue depth
   - Worker pool utilization
   - Worker pool task processing latency
   - Worker pool task failures

9. **Event Batching Metrics** (if enabled)
   - Batch size distribution
   - Batch flush latency
   - Batch age at flush
   - Batch processing errors

## Recommended New Metrics

### Ingester Metrics

```go
// Ingester lifecycle
zen_watcher_ingesters_active{source, ingester_type, namespace} // Gauge
zen_watcher_ingesters_status{source, status} // Gauge (1=active, 0=inactive, -1=error)
zen_watcher_ingesters_config_load_errors_total{source, error_type} // Counter
zen_watcher_ingesters_startup_duration_seconds{source} // Histogram
zen_watcher_ingesters_last_event_timestamp_seconds{source} // Gauge

// Per-ingester processing
zen_watcher_ingester_events_processed_total{source, ingester_type} // Counter
zen_watcher_ingester_events_processed_rate{source} // Gauge (events/sec)
zen_watcher_ingester_processing_latency_seconds{source, stage} // Histogram
zen_watcher_ingester_errors_total{source, error_type, stage} // Counter

// Informer-specific
zen_watcher_informer_cache_sync_duration_seconds{source, gvr} // Histogram
zen_watcher_informer_resync_events_total{source, gvr} // Counter
zen_watcher_informer_cache_size{source, gvr} // Gauge

// Webhook-specific
zen_watcher_webhook_ingester_requests_total{source, endpoint, status} // Counter
zen_watcher_webhook_ingester_latency_seconds{source, endpoint} // Histogram

// Destination delivery
zen_watcher_destination_delivery_total{source, destination_type, status} // Counter
zen_watcher_destination_delivery_latency_seconds{source, destination_type} // Histogram
zen_watcher_destination_queue_depth{source, destination_type} // Gauge
zen_watcher_destination_retries_total{source, destination_type} // Counter
```

### Filter Enhancement Metrics

```go
// Filter rule evaluation
zen_watcher_filter_rule_evaluation_duration_seconds{source, rule_type} // Histogram
zen_watcher_filter_rule_hits_total{source, rule_id, action} // Counter
zen_watcher_filter_rule_effectiveness{source, rule_id} // Gauge (0.0-1.0)
zen_watcher_filter_chain_depth{source} // Histogram
zen_watcher_filter_config_validation_errors_total{source, error_type} // Counter

// Global namespace filter
zen_watcher_global_namespace_filter_decisions_total{action, namespace} // Counter
zen_watcher_global_namespace_filter_events_filtered_total{namespace} // Counter
```

### Dedup Enhancement Metrics

```go
// Dedup cache details
zen_watcher_dedup_cache_size{source, strategy} // Gauge
zen_watcher_dedup_cache_hit_ratio{source, strategy} // Gauge (0.0-1.0)
zen_watcher_dedup_window_size_seconds{source, strategy} // Gauge
zen_watcher_dedup_fingerprint_generation_duration_seconds{source, strategy} // Histogram
zen_watcher_dedup_strategy_performance_comparison{source, strategy, metric} // Gauge
```

### Processing Order Metrics

```go
// Optimization analysis
zen_watcher_optimization_analysis_duration_seconds{source} // Histogram
zen_watcher_optimization_suggestion_quality_score{source, suggestion_type} // Gauge (0.0-1.0)
zen_watcher_optimization_rollbacks_total{source, reason} // Counter
zen_watcher_optimization_impact_measured{source, metric, before, after} // Gauge
zen_watcher_optimization_decision_latency_seconds{source} // Histogram
```

### Mapping/Normalization Metrics

```go
// Field mapping
zen_watcher_field_mapping_transformations_total{source, mapping_rule, status} // Counter
zen_watcher_field_mapping_latency_seconds{source, mapping_rule} // Histogram
zen_watcher_field_mapping_errors_total{source, mapping_rule, error_type} // Counter

// Normalization
zen_watcher_normalization_rule_applications_total{source, rule_type} // Counter
zen_watcher_normalization_rule_match_rate{source, rule_type} // Gauge (0.0-1.0)
zen_watcher_priority_mapping_hits_total{source, priority_level} // Counter
zen_watcher_field_extraction_failures_total{source, field_path} // Counter
```

### ConfigManager Metrics

```go
zen_watcher_configmap_load_total{configmap, result} // Counter
zen_watcher_configmap_reload_duration_seconds{configmap} // Histogram
zen_watcher_configmap_merge_conflicts_total{configmap} // Counter
zen_watcher_configmap_validation_errors_total{configmap, error_type} // Counter
zen_watcher_config_update_propagation_seconds{component} // Histogram
```

### Worker Pool Metrics (if enabled)

```go
zen_watcher_worker_pool_queue_depth{pool_name} // Gauge
zen_watcher_worker_pool_utilization{pool_name} // Gauge (0.0-1.0)
zen_watcher_worker_pool_task_processing_duration_seconds{pool_name, task_type} // Histogram
zen_watcher_worker_pool_task_failures_total{pool_name, task_type, error_type} // Counter
```

### Event Batching Metrics (if enabled)

```go
zen_watcher_batch_size{source, destination} // Histogram
zen_watcher_batch_flush_duration_seconds{source, destination} // Histogram
zen_watcher_batch_age_seconds{source, destination} // Histogram
zen_watcher_batch_processing_errors_total{source, destination, error_type} // Counter
```

## Implementation Priority

### High Priority (Critical for Operations)

1. **Ingester Status & Health Metrics**
   - Ingester active/inactive status
   - Per-ingester error rates
   - Ingester last event timestamp (staleness detection)

2. **Destination Delivery Metrics**
   - Success/failure rates per destination
   - Delivery latency
   - Queue depth

3. **ConfigManager Metrics**
   - Config load success/failure
   - Config validation errors

### Medium Priority (Important for Optimization)

1. **Filter Rule Metrics**
   - Rule evaluation latency
   - Rule effectiveness
   - Rule hit rates

2. **Dedup Cache Details**
   - Cache size per source
   - Cache hit/miss ratio
   - Fingerprint generation latency

3. **Mapping/Normalization Metrics**
   - Transformation success/failure rates
   - Field extraction failures

### Low Priority (Nice to Have)

1. **Worker Pool Metrics** (only if enabled)
2. **Event Batching Metrics** (only if enabled)
3. **Optimization Quality Scores**

## Label Cardinality Considerations

⚠️ **Warning**: Some proposed metrics may have high label cardinality. Consider:

1. **Limit label values**: Use bounded sets (e.g., `error_type` should be a small enum)
2. **Aggregation**: Prefer aggregated metrics over per-rule metrics where possible
3. **Sampling**: For high-volume metrics, consider sampling
4. **Cardinality limits**: Set reasonable limits and document them

## Metric Naming Consistency

All metrics should follow the pattern:
- `zen_watcher_<component>_<metric>_<unit>`
- Use `_total` suffix for counters
- Use `_seconds`, `_bytes`, `_ratio` suffixes for appropriate units
- Use consistent label names across related metrics

## Dashboard Recommendations

Based on these metrics, create dashboards for:

1. **Ingester Health Dashboard**
   - Ingester status overview
   - Per-ingester event rates
   - Ingester error rates
   - Staleness detection

2. **Filter Performance Dashboard**
   - Filter decision rates
   - Rule effectiveness
   - Filter latency

3. **Dedup Performance Dashboard**
   - Dedup effectiveness by strategy
   - Cache utilization
   - Dedup decision rates

4. **Processing Order Dashboard**
   - Current processing order per source
   - Strategy performance comparison

5. **Destination Delivery Dashboard**
   - Delivery success rates
   - Delivery latency
   - Queue depths

6. **ConfigManager Dashboard**
   - Config load status
   - Config validation errors
   - Config update propagation

## Next Steps

1. **Review and prioritize** the recommended metrics
2. **Implement high-priority metrics** first
3. **Add metric instrumentation** to relevant code paths
4. **Create dashboards** for key operational metrics
5. **Document metric semantics** and label meanings
6. **Set up alerts** based on critical metrics

