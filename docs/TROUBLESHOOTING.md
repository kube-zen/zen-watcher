# Zen-Watcher Troubleshooting Guide

## Quick Demo Issues

### Issue: Demo fails with "cluster already exists"
**Solution**: Use `--use-existing-cluster` or run cleanup first:
```bash
./scripts/cluster/destroy.sh
```

### Issue: Grafana dashboards not showing data
**Steps to debug**:
1. Check VictoriaMetrics is scraping zen-watcher:
   ```bash
   kubectl port-forward -n vm svc/victoria-metrics 8428:8428
   curl localhost:8428/api/v1/query?query=up{job="zen-watcher"}
   ```

2. Verify zen-watcher metrics endpoint:
   ```bash
   kubectl exec -n zen-system deployment/zen-watcher -- curl localhost:9090/metrics
   ```

3. Check if zen-watcher is running:
   ```bash
   kubectl get pods -n zen-system -l app=zen-watcher
   kubectl logs -n zen-system -l app=zen-watcher --tail=100
   ```

## Stress Test Issues

### Issue: Observation cleanup takes hours
**Solution**: Use the fast cleanup script:
```bash
./scripts/cleanup/fast-observation-cleanup.sh zen-system stress-test=true
```

**For large batches** (>1000 observations):
```bash
./scripts/cleanup/fast-observation-cleanup.sh zen-system stress-test=true 50 10
```

### Issue: Stress test fails with "too many observations"
**Solution**: Enable automatic cleanup via CronJob:
```bash
helm upgrade zen-watcher ./deployments/helm/zen-watcher \
  --set lifecycle.cleanup.enabled=true \
  --set lifecycle.cleanup.ttlDays=1
```

## Component Readiness Issues

### Issue: Components not ready after deployment
**Solution**: Use readiness checks:
```bash
source scripts/utils/readiness.sh
wait_for_deployment zen-system zen-watcher
wait_for_zen_watcher zen-system
```

### Issue: Grafana API not responding
**Solution**: Wait for Grafana API before importing dashboards:
```bash
source scripts/utils/readiness.sh
wait_for_grafana_api grafana
```

## Resource Management Issues

### Issue: Pod OOM (Out of Memory) kills
**Solution**: Increase resource limits:
```bash
helm upgrade zen-watcher ./deployments/helm/zen-watcher \
  --set resources.limits.memory=1Gi \
  --set resources.requests.memory=512Mi
```

### Issue: Too many observations consuming etcd storage
**Solution**: Enable resource quotas and lifecycle cleanup:
```bash
helm upgrade zen-watcher ./deployments/helm/zen-watcher \
  --set resourceQuota.enabled=true \
  --set resourceQuota.observationLimit=5000 \
  --set lifecycle.cleanup.enabled=true
```

## Performance Issues

### Issue: Slow observation processing
**Check metrics**:
```bash
kubectl exec -n zen-system deployment/zen-watcher -- curl localhost:9090/metrics | grep zen_watcher_observations_processed
```

**Check logs**:
```bash
kubectl logs -n zen-system -l app=zen-watcher --tail=100 | grep -i "slow\|error\|timeout"
```

### Issue: High CPU usage
**Solution**: Scale horizontally or increase resources:
```bash
helm upgrade zen-watcher ./deployments/helm/zen-watcher \
  --set autoscaling.enabled=true \
  --set autoscaling.minReplicas=2 \
  --set autoscaling.maxReplicas=5
```

## Common Errors

### Error: "observation CRD not found"
**Solution**: Install CRDs first:
```bash
kubectl apply -f deployments/helm/zen-watcher/crds/
```

### Error: "permission denied" when creating observations
**Solution**: Check RBAC configuration:
```bash
kubectl get clusterrole zen-watcher -o yaml
kubectl get clusterrolebinding zen-watcher -o yaml
```

### Error: "connection refused" to metrics endpoint
**Solution**: Check service and port configuration:
```bash
kubectl get svc -n zen-system -l app=zen-watcher
kubectl get endpoints -n zen-system -l app=zen-watcher
```

## Getting Help

1. Check logs: `kubectl logs -n zen-system -l app=zen-watcher`
2. Check events: `kubectl get events -n zen-system --sort-by='.lastTimestamp'`
3. Check metrics: `kubectl port-forward -n zen-system svc/zen-watcher 9090:9090` then visit `http://localhost:9090/metrics`
4. Run E2E validation: `./scripts/ci/demo-e2e-test.sh zen-system`
