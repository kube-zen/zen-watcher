# Performance Tuning Guide

## Resource Allocation

### Small Clusters (<5 nodes)
```yaml
resources:
  limits:
    cpu: 100m
    memory: 256Mi
  requests:
    cpu: 50m
    memory: 128Mi
```

### Medium Clusters (5-20 nodes)
```yaml
resources:
  limits:
    cpu: 500m
    memory: 512Mi
  requests:
    cpu: 100m
    memory: 256Mi
```

### Large Clusters (20+ nodes)
```yaml
resources:
  limits:
    cpu: 1000m
    memory: 1Gi
  requests:
    cpu: 500m
    memory: 512Mi
```

## Observation Lifecycle Management

### Automatic Cleanup

Enable automatic cleanup via CronJob:
```bash
helm upgrade zen-watcher kube-zen/zen-watcher \
  --set lifecycle.cleanup.enabled=true \
  --set lifecycle.cleanup.schedule="0 2 * * *" \
  --set lifecycle.cleanup.ttlDays=7
```

### TTL Configuration by Use Case

- **Dev/test**: 24 hours
  ```yaml
  lifecycle:
    cleanup:
      ttlDays: 1
  ```

- **Production**: 7 days
  ```yaml
  lifecycle:
    cleanup:
      ttlDays: 7
  ```

- **Compliance**: 90 days
  ```yaml
  lifecycle:
    cleanup:
      ttlDays: 90
  ```

## Stress Testing Guidelines

### Max Observations per Test

- **Small clusters**: 1000 observations
- **Medium clusters**: 5000 observations
- **Large clusters**: 10000 observations

### Batch Cleanup for Large Tests

For stress tests with >1000 observations, use batch cleanup:
```bash
./scripts/cleanup/fast-observation-cleanup.sh zen-system stress-test=true 50 10
```

### Monitor etcd Storage During Tests

```bash
# Check etcd storage usage
kubectl top nodes
kubectl get events --all-namespaces --sort-by='.lastTimestamp' | tail -20
```

## Horizontal Scaling

### Enable Autoscaling

```bash
helm upgrade zen-watcher kube-zen/zen-watcher \
  --set autoscaling.enabled=true \
  --set autoscaling.minReplicas=2 \
  --set autoscaling.maxReplicas=10 \
  --set autoscaling.targetCPUUtilizationPercentage=80 \
  --set autoscaling.targetMemoryUtilizationPercentage=80
```

### HA Optimization

For multi-replica deployments, enable HA optimization:
```yaml
config:
  haOptimization:
    enabled: true
```

## Resource Quotas

### Enable Resource Quotas

```bash
helm upgrade zen-watcher kube-zen/zen-watcher \
  --set resourceQuota.enabled=true \
  --set resourceQuota.observationLimit=10000
```

### Recommended Limits

```yaml
resourceQuota:
  requests:
    cpu: "2"
    memory: 4Gi
  limits:
    cpu: "4"
    memory: 8Gi
  observationLimit: "10000"
```

## Performance Monitoring

### Key Metrics to Monitor

- `zen_watcher_observations_processed_total` - Total observations processed
- `zen_watcher_event_processing_duration_seconds` - Processing latency
- `zen_watcher_observations_filtered_total` - Filtered events
- `zen_watcher_observations_deduped_total` - Deduplicated events

### Check Metrics

```bash
kubectl port-forward -n zen-system svc/zen-watcher 9090:9090
curl http://localhost:9090/metrics | grep zen_watcher
```

## Optimization Tips

1. **Enable filtering** to reduce unnecessary observations
2. **Configure deduplication** to prevent duplicate events
3. **Use resource quotas** to prevent resource exhaustion
4. **Enable automatic cleanup** to manage observation lifecycle
5. **Monitor etcd storage** during high-load periods
6. **Scale horizontally** for high-throughput scenarios

