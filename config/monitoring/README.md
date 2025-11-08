# Zen Watcher Monitoring

Complete monitoring setup for Zen Watcher with Prometheus and Grafana.

## 📊 Components

### 1. Prometheus Metrics

**Endpoint**: `http://zen-watcher:8080/metrics`

**Categories**:
- Event metrics (collection, writes, failures)
- Watcher metrics (status, errors, duration)
- CRD operation metrics
- HTTP API metrics
- System metrics (health, resources)
- Kubernetes API metrics

### 2. ServiceMonitor

Automatically discovers and scrapes Zen Watcher metrics.

```bash
kubectl apply -f prometheus-servicemonitor.yaml
```

### 3. Prometheus Alerts

Comprehensive alerting rules for:
- Health and availability
- Event processing
- Performance degradation
- Resource usage
- SLO violations

```bash
kubectl apply -f prometheus-alerts.yaml
```

### 4. Grafana Dashboard

Beautiful, comprehensive dashboard with 16+ panels.

See `../dashboards/zen-watcher-dashboard.json`

---

## 🚀 Quick Setup

### Complete Monitoring Stack

```bash
# 1. Deploy Zen Watcher with ServiceMonitor
helm install zen-watcher ../charts/zen-watcher \
  --namespace zen-system \
  --create-namespace \
  --set global.clusterID=my-cluster \
  --set serviceMonitor.enabled=true

# 2. Deploy alerts
kubectl apply -f prometheus-alerts.yaml

# 3. Import Grafana dashboard
# Go to Grafana → Dashboards → Import → Upload zen-watcher-dashboard.json

# 4. Verify metrics
kubectl port-forward -n zen-system svc/zen-watcher 8080:8080
curl http://localhost:8080/metrics
```

---

## 📈 Key Metrics

### Event Metrics

```promql
# Event ingestion rate
rate(zen_watcher_events_total[5m])

# Events by category
sum by(category)(rate(zen_watcher_events_total[5m]))

# Active critical events
sum(zen_watcher_active_events{severity="CRITICAL"})

# Write success rate
sum(rate(zen_watcher_events_written_total[5m])) 
/ 
sum(rate(zen_watcher_events_total[5m]))
```

### Performance Metrics

```promql
# Event processing latency p95
histogram_quantile(0.95, rate(zen_watcher_event_processing_duration_seconds_bucket[5m]))

# CRD operation latency p99
histogram_quantile(0.99, rate(zen_watcher_crd_operation_duration_seconds_bucket[5m]))

# HTTP request latency p95
histogram_quantile(0.95, rate(zen_watcher_http_request_duration_seconds_bucket[5m]))
```

### Health Metrics

```promql
# Uptime
avg_over_time(zen_watcher_health_status[24h])

# Error rate
rate(zen_watcher_watcher_errors_total[5m])

# Resource usage
process_resident_memory_bytes / 1024 / 1024  # MB
rate(process_cpu_seconds_total[5m]) * 100    # CPU %
```

---

## 🚨 Alerts Reference

### Critical Alerts

| Alert | Condition | Duration | Action |
|-------|-----------|----------|--------|
| ZenWatcherDown | `health_status == 0` | 2m | Check pod status, restart if needed |
| HighCriticalEventRate | `rate > 10/s` | 5m | Investigate source, check for attack |
| TooManyCriticalEvents | `active > 100` | 10m | Review and resolve events |
| CRDOperationFailures | `failure_rate > 5%` | 5m | Check RBAC, etcd health |
| AvailabilitySLO | `uptime < 99.9%` | 5m | Investigate root cause |

### Warning Alerts

| Alert | Condition | Duration | Action |
|-------|-----------|----------|--------|
| ZenWatcherNotReady | `readiness == 0` | 5m | Check logs, verify watchers |
| HighMemoryUsage | `memory > 512MB` | 10m | Review event volume, scale up |
| HighCPUUsage | `cpu > 90%` | 10m | Tune intervals, scale up |
| WatcherErrors | `error_rate > 0.1/s` | 5m | Check watcher logs |
| SlowCRDOperations | `p95 > 5s` | 10m | Check etcd performance |

---

## 📊 Dashboard Panels

### Row 1: Overview (Stats)
1. **Health Status** - Is Zen Watcher healthy?
2. **Events/sec** - Current event ingestion rate
3. **Active Events** - Total unresolved events
4. **Critical Events** - Critical severity events

### Row 2: Event Analysis
5. **Events Rate Timeline** - Historical view of event rates
6. **Events by Category** - Distribution across categories
7. **Events by Severity** - Severity breakdown

### Row 3: Watcher Health
8. **Watcher Status** - Which watchers are enabled
9. **Watcher Scrape Duration** - Performance of each watcher

### Row 4: System Resources
10. **Goroutines** - Runtime goroutine count
11. **Memory Usage** - RSS and Heap usage
12. **CPU Usage** - CPU utilization

### Row 5: Operations
13. **Watcher Errors** - Error rates by watcher
14. **CRD Operation Duration** - CRD write performance
15. **HTTP Requests** - API usage
16. **HTTP Duration** - API latency

---

## 🎯 Troubleshooting

### Metrics Not Showing

```bash
# Check ServiceMonitor
kubectl get servicemonitor -n zen-system

# Check if Prometheus is scraping
kubectl exec -n monitoring prometheus-0 -- \
  curl -s localhost:9090/api/v1/targets | \
  jq '.data.activeTargets[] | select(.labels.job=="zen-watcher")'

# Check metrics endpoint directly
kubectl port-forward -n zen-system svc/zen-watcher 8080:8080
curl http://localhost:8080/metrics | grep zen_watcher
```

### Dashboard Not Loading

```bash
# Check Grafana logs
kubectl logs -n monitoring -l app=grafana

# Verify datasource
# Grafana UI → Configuration → Data Sources → Prometheus → Test

# Check dashboard JSON syntax
jq . ../dashboards/zen-watcher-dashboard.json
```

### Alerts Not Firing

```bash
# Check PrometheusRule
kubectl get prometheusrule -n zen-system zen-watcher-alerts

# Check Prometheus config
kubectl exec -n monitoring prometheus-0 -- \
  curl -s localhost:9090/api/v1/rules | jq '.data.groups[] | select(.name | contains("zen-watcher"))'

# Test alert query
kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090
# Go to http://localhost:9090/alerts
```

---

## 📚 Additional Resources

- [Prometheus Best Practices](https://prometheus.io/docs/practices/)
- [Grafana Dashboard Best Practices](https://grafana.com/docs/grafana/latest/best-practices/)
- [SLO Monitoring](https://sre.google/workbook/implementing-slos/)

---

## 🎉 Success Metrics

After setup, you should see:
- ✅ Metrics being scraped (Prometheus targets)
- ✅ Dashboard showing data (Grafana)
- ✅ Alerts configured (Prometheus rules)
- ✅ No firing alerts (healthy system)

**Happy Monitoring!** 📊


