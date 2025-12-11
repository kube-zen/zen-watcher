# Troubleshooting Guide

This guide helps operators understand what's happening inside zen-watcher when things go wrong.

## Error Categories

zen-watcher categorizes errors to make troubleshooting easier:

- **CONFIG_ERROR**: Configuration errors (missing fields, invalid values)
- **FILTER_ERROR**: Errors in the filter stage
- **DEDUP_ERROR**: Errors in the deduplication stage
- **NORMALIZE_ERROR**: Errors in the normalization stage
- **CRD_WRITE_ERROR**: Errors writing Observation CRDs
- **PIPELINE_ERROR**: General pipeline errors

## Common Failure Signals

### High Pipeline Error Rate

**Metric**: `zen_watcher_pipeline_errors_total{stage="..."}`

**Symptoms:**
- Metric shows increasing error count
- Logs show repeated error messages

**Possible Causes:**
1. **CONFIG_ERROR**: Invalid Ingester CRD configuration
   - **Next step**: Check Ingester CRD with `kubectl get ingester <name> -n <namespace> -o yaml`
   - **Action**: Run `ingester-lint` to validate configuration

2. **NORMALIZE_ERROR**: Normalization config missing or invalid
   - **Next step**: Check `spec.destinations[].mapping` in Ingester CRD
   - **Action**: Verify normalization config has required fields

3. **CRD_WRITE_ERROR**: Cannot write Observation CRDs
   - **Next step**: Check RBAC permissions: `kubectl auth can-i create observations --namespace <namespace>`
   - **Action**: Verify ServiceAccount has permissions to create Observations

### No Observations Created

**Metric**: `zen_watcher_observations_created_total{source="..."}`

**Symptoms:**
- Metric shows zero or very low observation count
- No Observation CRDs in cluster

**Possible Causes:**
1. **Events filtered out**: Check `zen_watcher_events_filtered_total{source="..."}`
   - **If high**: Events are being filtered (may be expected)
   - **Action**: Review filter configuration in Ingester CRD

2. **Events deduplicated**: Check `zen_watcher_events_deduped_total{source="..."}`
   - **If high**: Events are being deduplicated (may be expected)
   - **Action**: Review deduplication configuration

3. **Normalization failing**: Check logs for `[NORMALIZE_ERROR]`
   - **Action**: Fix normalization configuration

4. **CRD write failing**: Check logs for `[CRD_WRITE_ERROR]`
   - **Action**: Fix RBAC permissions or CRD installation

### High Deduplication Rate

**Metric**: `zen_watcher_events_deduped_total{source="..."}`

**Symptoms:**
- Very high deduplication rate (>80%)
- May indicate duplicate events from source

**Possible Causes:**
1. **Source sending duplicates**: Expected behavior for some sources
   - **Action**: Verify with source documentation

2. **Deduplication window too large**: Check `spec.deduplication.window`
   - **Action**: Reduce window if too many events are deduplicated

3. **Deduplication strategy ineffective**: Check `spec.deduplication.strategy`
   - **Action**: Try different strategy (fingerprint, key, hybrid)

### High Filter Rate

**Metric**: `zen_watcher_events_filtered_total{source="..."}`

**Symptoms:**
- Very high filter rate (>90%)
- May indicate overly aggressive filtering

**Possible Causes:**
1. **Filter too restrictive**: Check `spec.filters.minPriority`
   - **Action**: Lower minPriority threshold

2. **Namespace exclusions too broad**: Check `spec.filters.excludeNamespaces`
   - **Action**: Review namespace exclusions

### Normalization Errors

**Metric**: `zen_watcher_pipeline_errors_total{stage="normalize"}`

**Symptoms:**
- Logs show `[NORMALIZE_ERROR]` messages
- Observations not created

**Possible Causes:**
1. **Missing normalization config**: Check `spec.destinations[].mapping`
   - **Action**: Add normalization config to Ingester CRD

2. **Invalid field mappings**: Check `spec.destinations[].mapping.fieldMapping`
   - **Action**: Verify field mappings match source event structure

3. **Invalid priority mapping**: Check `spec.destinations[].mapping.priority`
   - **Action**: Verify priority mapping has correct keys

## Log Analysis

### Log Format

zen-watcher logs include:
- **Category**: Error category (CONFIG_ERROR, FILTER_ERROR, etc.)
- **Code**: Error code for specific error type
- **Source**: Event source name
- **Ingester**: Ingester type (informer, webhook, logs, k8s-events)
- **Message**: Human-readable error message

### Example Log Entry

```
[ERROR] [NORMALIZE_ERROR] NORMALIZE_MISSING_FIELD: Missing required field 'severity' in event (source: trivy, ingester: informer): field not found
```

### Log Levels

- **ERROR**: Errors that prevent event processing
- **WARN**: Warnings that may affect behavior but don't prevent processing
- **INFO**: Informational messages about normal operation
- **DEBUG**: Detailed debugging information (disabled by default)

## Metrics to Monitor

### Pipeline Health

- `zen_watcher_pipeline_errors_total{stage="..."}`: Error count by stage
- `zen_watcher_events_processed_total{source="..."}`: Total events processed
- `zen_watcher_observations_created_total{source="..."}`: Observations created

### Filter/Dedup Health

- `zen_watcher_events_filtered_total{source="..."}`: Events filtered
- `zen_watcher_events_deduped_total{source="..."}`: Events deduplicated
- `zen_watcher_filter_pass_rate{source="..."}`: Filter pass rate
- `zen_watcher_dedup_effectiveness{source="..."}`: Deduplication effectiveness

### Performance

- `zen_watcher_pipeline_latency_seconds{source="...",stage="..."}`: Processing latency
- `zen_watcher_observations_per_minute{source="..."}`: Observations per minute

## Debugging Steps

1. **Check Ingester CRD**: `kubectl get ingester <name> -n <namespace> -o yaml`
2. **Validate configuration**: `ingester-lint <ingester.yaml>`
3. **Check logs**: `kubectl logs -n <namespace> -l app=zen-watcher --tail=100`
4. **Check metrics**: Query Prometheus for error metrics
5. **Check RBAC**: `kubectl auth can-i create observations --namespace <namespace>`
6. **Check CRD installation**: `kubectl get crd observations.zen.kube-zen.io`

## Related Documentation

- [INGESTER_API.md](INGESTER_API.md) - Complete Ingester CRD API reference
- [OBSERVABILITY.md](OBSERVABILITY.md) - Metrics and observability guide
- [PERFORMANCE_GUIDE.md](PERFORMANCE_GUIDE.md) - Performance characteristics and sizing

