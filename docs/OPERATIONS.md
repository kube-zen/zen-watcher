# Zen-Watcher Operations Guide

## Overview

This guide covers day-to-day operations, troubleshooting, and maintenance of zen-watcher in production.

## 🚀 Quick Operations Reference

### Health Checks

```bash
# Pod health
kubectl get pods -n zen-system -l app.kubernetes.io/name=zen-watcher

# Health endpoint
kubectl port-forward -n zen-system svc/zen-watcher 8080:8080 &
curl http://localhost:8080/health
curl http://localhost:8080/ready

# Check informer sync status
curl http://localhost:8080/metrics | grep zen_watcher_informer_cache_synced
```

### View Observations

```bash
# All observations
kubectl get observations -A

# By source
kubectl get observations -A -l source=trivy

# By severity
kubectl get observations -A -l severity=CRITICAL

# Watch live
kubectl get observations -A --watch

# Count by source
kubectl get observations -A -o json | jq -r '.items[] | .spec.source' | sort | uniq -c
```

### Check Metrics

```bash
# All zen-watcher metrics
kubectl port-forward -n zen-system svc/zen-watcher 8080:8080 &
curl -s http://localhost:8080/metrics | grep zen_watcher

# Key metrics to monitor
curl -s http://localhost:8080/metrics | grep -E "zen_watcher_events_total|zen_watcher_observations_filtered_total|zen_watcher_observations_deduped_total|zen_watcher_webhook_requests_total"
```

## 📊 Monitoring & Alerting

### Critical Alerts

```yaml
# High observation creation error rate
- alert: ZenWatcherObservationCreationErrors
  expr: rate(zen_watcher_observations_create_errors_total[5m]) > 1
  severity: critical
  
# Webhook drops (backpressure)
- alert: ZenWatcherWebhookDrops
  expr: rate(zen_watcher_webhook_events_dropped_total[5m]) > 10
  severity: warning
  
# Informer not synced
- alert: ZenWatcherInformerNotSynced
  expr: zen_watcher_informer_cache_synced < 1
  severity: critical

# GC errors
- alert: ZenWatcherGCErrors
  expr: zen_watcher_gc_errors_total > 0
  severity: warning
```

### Key Metrics Dashboard Queries

```promql
# Event rate by source
sum(rate(zen_watcher_events_total[5m])) by (source)

# P99 event processing latency
histogram_quantile(0.99, sum(rate(zen_watcher_event_processing_duration_seconds_bucket[5m])) by (le, source))

# Dedup effectiveness
rate(zen_watcher_observations_deduped_total[5m]) / 
  (rate(zen_watcher_observations_created_total[5m]) + rate(zen_watcher_observations_deduped_total[5m]))

# Live observation count (etcd footprint)
sum(zen_watcher_observations_live) by (source)
```

## 🔧 Common Operations

### Update Filter Configuration

**Via ConfigMap:**
```bash
kubectl edit configmap zen-watcher-filter -n zen-system
# Changes take effect within seconds (no restart needed)
```

**Via Ingester CRD:**
```bash
kubectl apply -f my-filter.yaml
# Dynamic reload, no restart needed
```

### Scale Replicas

**✅ HA Support Available**

Zen Watcher supports high availability with multiple replicas. HA optimization features (enabled via `haOptimization.enabled: true` in Helm values) provide dynamic deduplication window adjustment, adaptive cache sizing, and load balancing to ensure proper operation across replicas.

**Recommended Deployment:**
```bash
# Standard deployment (HA optimization available)
kubectl scale deployment zen-watcher -n zen-system --replicas=1
```

**Scaling Strategy:**
- **Vertical Scaling Only**: Increase CPU/memory resources for higher event volumes
- **Resource Guidance**:
  - Low traffic (<1,000 events/day): 100m CPU, 128Mi RAM
  - Medium traffic (1,000-10,000 events/day): 200m CPU, 256Mi RAM
  - High traffic (>10,000 events/day): 500m CPU, 512Mi RAM

**HA Optimization Support (v1.0.0-alpha+)**
- HA optimization features are available when `haOptimization.enabled: true` in Helm values
- Enables dynamic dedup window adjustment, adaptive cache sizing, and load balancing
- For standard deployments, single replica is sufficient. For HA, enable `haOptimization.enabled: true` with multiple replicas.
- See HA configuration documentation for multi-replica deployment guidance

See [OPERATIONAL_EXCELLENCE.md](OPERATIONAL_EXCELLENCE.md#scaling-strategy) for complete scaling guidance.

### Restart zen-watcher

```bash
# Rolling restart
kubectl rollout restart deployment/zen-watcher -n zen-system

# Force delete (faster)
kubectl delete pod -n zen-system -l app.kubernetes.io/name=zen-watcher
```

### Upgrade zen-watcher

```bash
# Via Helm (recommended)
helm upgrade zen-watcher kube-zen/zen-watcher -n zen-system --reuse-values

# Change image version
helm upgrade zen-watcher kube-zen/zen-watcher -n zen-system --set image.tag=1.0.0-alpha

# Rollback if needed
helm rollback zen-watcher -n zen-system
```

### Check Adapter Status

```bash
# View logs for all adapters
kubectl logs -n zen-system deployment/zen-watcher --tail=100 | grep -E "adapter|Started|Stopped"

# Check for adapter errors
kubectl logs -n zen-system deployment/zen-watcher | grep -i "adapter.*error"

# Verify all 6 adapters are running
kubectl logs -n zen-system deployment/zen-watcher --tail=500 | grep "adapter" | sort -u
```

## 🐛 Troubleshooting

### No Observations Being Created

**Check 1: Are source tools running?**
```bash
kubectl get pods -n trivy-system    # Trivy Operator
kubectl get pods -n falco           # Falco
kubectl get pods -n kyverno         # Kyverno
```

**Check 2: RBAC permissions correct?**
```bash
kubectl describe clusterrole zen-watcher | grep -A 50 "Rules:"
```

**Check 3: Filters too restrictive?**
```bash
kubectl get configmap zen-watcher-filter -n zen-system -o yaml
kubectl get observationfilters -A
```

**Check 4: Check metrics for filtering**
```bash
curl http://localhost:8080/metrics | grep zen_watcher_observations_filtered_total
```

### High Memory Usage

**Check dedup cache size:**
```bash
# Default: 5000 events
# Increase if needed:
kubectl set env deployment/zen-watcher -n zen-system DEDUP_CACHE_SIZE=10000
```

**Check observation backlog:**
```bash
kubectl get observations -A --no-headers | wc -l
```

**Reduce TTL if etcd pressure:**
```bash
kubectl set env deployment/zen-watcher -n zen-system OBSERVATION_TTL_SECONDS=259200  # 3 days
```

### Webhook Not Working

**Check 1: Service accessible?**
```bash
kubectl get svc zen-watcher -n zen-system
kubectl get endpoints zen-watcher -n zen-system
```

**Check 2: NetworkPolicy blocking?**
```bash
kubectl get networkpolicy -n zen-system
```

**Check 3: Webhook metrics**
```bash
curl http://localhost:8080/metrics | grep zen_watcher_webhook_requests_total
```

**Check 4: Test webhook manually**
```bash
kubectl port-forward -n zen-system svc/zen-watcher 8080:8080 &
curl -X POST http://localhost:8080/falco/webhook \
  -H 'Content-Type: application/json' \
  -d '{"priority":"Critical","rule":"Test","output":"Test alert"}'
```

### ConfigMap Sources Not Creating Observations

**Issue:** Checkov or KubeBench observations not appearing

**Check 1: ConfigMaps exist with correct labels?**
```bash
kubectl get configmap -n checkov -l app=checkov
kubectl get configmap -n kube-bench -l app=kube-bench
```

**Check 2: Informer is watching ConfigMaps?**
```bash
# Check if informer is detecting ConfigMap changes
kubectl logs -n zen-system deployment/zen-watcher | grep -E "ConfigMap|checkov|kube-bench"
```

**Check 3: Verify Ingester configuration**
```bash
# Restart zen-watcher to trigger immediate poll
kubectl delete pod -n zen-system -l app.kubernetes.io/name=zen-watcher
```

### Grafana Dashboard Shows "No Data"

**Check 1: VictoriaMetrics scraping?**
```bash
curl http://localhost:8080/victoriametrics/targets | grep zen-watcher
```

**Check 2: Service annotations correct?**
```bash
kubectl get svc zen-watcher -n zen-system -o yaml | grep prometheus.io
```

**Check 3: Query VictoriaMetrics directly**
```bash
curl -s 'http://localhost:8080/victoriametrics/api/v1/query?query=zen_watcher_events_total' | jq '.data.result[] | .metric.source' | sort -u
```

**Fix: Enable VMServiceScrape**
```bash
helm upgrade zen-watcher kube-zen/zen-watcher -n zen-system --set vmServiceScrape.enabled=true
```

## 📈 Performance Tuning

### High-Volume Clusters (>100 nodes, >1000 events/min)

```yaml
# values.yaml
env:
  - name: DEDUP_CACHE_SIZE
    value: "10000"
  - name: DEDUP_WINDOW_SECONDS
    value: "600"
  - name: GC_INTERVAL_MINUTES
    value: "30"

resources:
  requests:
    memory: 512Mi
    cpu: 500m
  limits:
    memory: 1Gi
    cpu: 1000m

# HA optimization available (see scaling strategy above)
replicas: 1
```

### Resource-Constrained Clusters

```yaml
# values.yaml
resources:
  requests:
    memory: 64Mi
    cpu: 50m
  limits:
    memory: 256Mi
    cpu: 250m

replicas: 1

# ConfigMap watching is event-driven via informers (no polling interval needed)
```

## 🔄 Backup & Recovery

### Backup Observations

```bash
# Export all observations
kubectl get observations -A -o yaml > observations-backup.yaml

# Export specific source
kubectl get observations -A -l source=trivy -o yaml > trivy-backup.yaml
```

### Restore Observations

```bash
# Note: TTL will apply from creation time
kubectl apply -f observations-backup.yaml
```

### Disaster Recovery

**Zen-watcher is stateless** - observations are stored as CRDs in etcd:
1. Backup etcd (or rely on Kubernetes cluster backup)
2. Reinstall zen-watcher with same config
3. Observations are preserved in etcd
4. Adapters resume processing from current state

## 📊 Capacity Planning

See [STABILITY.md](STABILITY.md) for detailed capacity planning guidance.

**Rule of Thumb (Vertical Scaling or HA):**
- Small cluster (<50 nodes): 1 replica, 100m CPU, 128Mi RAM
- Medium cluster (50-200 nodes): 1 replica, 200m CPU, 256Mi RAM
- Large cluster (200-1000 nodes): 1 replica, 500m CPU, 512Mi RAM
- Very large (>1000 nodes): 1 replica, 1000m CPU, 1Gi RAM, consider shorter TTL

**✅ HA Support:** Multiple replicas are supported with HA optimization enabled (`haOptimization.enabled: true`). This provides dynamic deduplication window adjustment, adaptive cache sizing, and load balancing.

## 🔐 Security Operations

### Rotate Webhook Tokens

```bash
# Update secret
kubectl create secret generic zen-watcher-webhook-token \
  --from-literal=token=$(openssl rand -base64 32) \
  -n zen-system --dry-run=client -o yaml | kubectl apply -f -

# Restart zen-watcher to pick up new token
kubectl rollout restart deployment/zen-watcher -n zen-system
```

### Audit RBAC Permissions

```bash
# Review current permissions
kubectl describe clusterrole zen-watcher

# Verify serviceaccount
kubectl get serviceaccount zen-watcher -n zen-system

# Check if permissions are being used
kubectl logs -n zen-system deployment/zen-watcher | grep -i "forbidden\|unauthorized"
```

### Review Observations for Sensitive Data

```bash
# Check if sensitive data is being captured
kubectl get observations -A -o json | jq '.items[] | select(.spec.details | tostring | test("password|token|secret"; "i"))'
```

## 🎯 Maintenance Windows

### Zero-Downtime Upgrade

**Note:** With HA enabled, rolling updates provide zero-downtime upgrades. Single replica deployments require brief downtime during upgrades.

```bash
# Rolling update (works with HA or single replica)
helm upgrade zen-watcher kube-zen/zen-watcher -n zen-system
```

### Planned Maintenance

```bash
# Scale to 0 (stops processing)
kubectl scale deployment zen-watcher -n zen-system --replicas=0

# Perform maintenance (e.g., etcd backup, cluster upgrade)

# Scale back up
kubectl scale deployment zen-watcher -n zen-system --replicas=1
```

**Downtime Impact:**
- CRD-based adapters: Events queued by Kubernetes, processed on restart
- Webhook adapters: Webhook senders will retry or fail (check source tool retry config)
- ConfigMap sources: Events queued by Kubernetes informer, processed on restart

## 📝 Logs & Debugging

### Structured Logging

All logs are JSON format with fields:
- `level`: DEBUG, INFO, WARN, ERROR, FATAL
- `timestamp`: ISO8601
- `caller`: Source file and line
- `message`: Human-readable message
- `component`: Which component (watcher, server, gc, filter)
- `operation`: Operation name
- `error`: Error details (if applicable)

### Common Log Queries

```bash
# Errors only
kubectl logs -n zen-system deployment/zen-watcher | jq 'select(.level=="error")'

# Specific component
kubectl logs -n zen-system deployment/zen-watcher | jq 'select(.component=="filter")'

# Specific operation
kubectl logs -n zen-system deployment/zen-watcher | jq 'select(.operation=="observation_create")'

# Follow live logs
kubectl logs -n zen-system deployment/zen-watcher -f | jq '.'
```

## 🔗 Related Documentation

- [STABILITY.md](STABILITY.md) - Production readiness and HA
- [SECURITY.md](SECURITY.md) - Security features and best practices
- [FILTERING.md](FILTERING.md) - Filter configuration
- [PERFORMANCE.md](PERFORMANCE.md) - Performance benchmarks

