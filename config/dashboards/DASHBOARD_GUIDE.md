# Zen Watcher Grafana Dashboard Guide

## 🎨 Dashboard Overview

The Zen Watcher dashboard provides comprehensive visibility into your security and compliance event aggregation platform.

**Dashboard ID**: `zen-watcher`
**Refresh Interval**: 10 seconds (auto-refresh)
**Time Range**: Last 1 hour (configurable)

---

## 📊 Panel Breakdown

### Row 1: Health Overview (Stats)

#### 1. Health Status
**Type**: Stat with background color  
**Metric**: `up{job="zen-watcher"}`  
**Colors**: 
- 🟢 Green = Healthy (1)
- 🔴 Red = Unhealthy (0)

**What to watch**: Should always be green. Red = immediate action required.

#### 2. Events/sec
**Type**: Stat with sparkline  
**Metric**: `rate(zen_watcher_events_total[5m])`  
**Thresholds**:
- 🟢 < 100/sec = Normal
- 🟡 100-1000/sec = Moderate
- 🔴 > 1000/sec = High load

**What to watch**: Sudden spikes may indicate security incident or misconfiguration.

#### 3. Active Events
**Type**: Stat with sparkline  
**Metric**: `sum(zen_watcher_observations_live)` or `sum(zen_watcher_events_total)`  
**Thresholds**:
- 🟢 < 100 = Normal
- 🟡 100-500 = Review needed
- 🔴 > 500 = Action required

**What to watch**: Growing number indicates events aren't being resolved.

#### 4. Critical Events
**Type**: Stat with sparkline  
**Metric**: `sum(rate(zen_watcher_events_total{severity="CRITICAL"}[5m])) * 60`  
**Thresholds**:
- 🟢 < 10 = Normal
- 🟡 10-50 = Review urgently
- 🔴 > 50 = Security incident

**What to watch**: Any critical events require immediate investigation.

---

### Row 2: Event Analysis

#### 5. Events Rate Timeline
**Type**: Stacked time series  
**Metric**: `rate(zen_watcher_events_total[5m])`  
**Legend**: Shows category/source/severity combinations  

**How to use**:
- Identify patterns (time-based attacks, maintenance windows)
- Compare different event sources
- Spot anomalies

**Common patterns**:
- Spikes during deployments (normal)
- Periodic spikes (scheduled scans)
- Continuous high rate (investigate)

#### 6. Active Events by Category
**Type**: Donut chart  
**Metric**: `sum by(category)(rate(zen_watcher_events_total[5m]))`  

**Segments**:
- 🛡️ Security events (blue)
- 📋 Compliance events (green)
- 🚀 Performance events (yellow)
- 🎯 Custom events (purple)

**What to watch**: Balance between categories. If one dominates, investigate source.

#### 7. Events by Severity
**Type**: Donut chart  
**Metric**: `sum by(severity)(rate(zen_watcher_events_total[5m]))`  

**Segments**:
- 🔴 CRITICAL (red)
- 🟠 HIGH (orange)
- 🟡 MEDIUM (yellow)
- 🟢 LOW (green)
- 🔵 INFO (blue)

**What to watch**: Too many critical/high = investigate immediately.

---

### Row 3: Watcher Health

#### 8. Watcher Status
**Type**: Gauge (multiple)  
**Metric**: `zen_watcher_tools_active{tool}`  

**Watchers**:
- Trivy
- Falco
- Kyverno
- Audit
- Kube-bench

**Colors**:
- 🟢 Green (1) = Enabled
- 🔴 Red (0) = Disabled

**What to watch**: Verify expected watchers are enabled based on your configuration.

#### 9. Watcher Scrape Duration (p95)
**Type**: Time series  
**Metric**: `histogram_quantile(0.95, zen_watcher_scrape_duration_seconds_bucket)`  

**What to watch**:
- Normal: < 5 seconds
- Slow: 5-30 seconds (investigate)
- Critical: > 30 seconds (performance issue)

---

### Row 4: System Resources

#### 10. Goroutines
**Type**: Time series  
**Metric**: `zen_watcher_goroutines`  

**Normal range**: 10-100 goroutines  
**Alert threshold**: > 1000 (possible leak)

**What to watch**: Steady growth = goroutine leak (restart needed).

#### 11. Memory Usage
**Type**: Time series (dual axis)  
**Metrics**:
- RSS (total resident memory)
- Heap Alloc (Go heap memory)

**Legends show**:
- Mean
- Current (last value)

**What to watch**: 
- Steady growth = memory leak
- Spikes during high event volumes = normal
- Approaching limit = scale up

#### 12. CPU Usage
**Type**: Time series  
**Metric**: `rate(process_cpu_seconds_total[5m]) * 100`  

**Normal range**: 5-30%  
**High usage**: > 80%

**What to watch**: Sustained high usage = need more CPU or optimization.

---

### Row 5: Operations

#### 13. Watcher Errors
**Type**: Time series (stacked)  
**Metric**: `rate(zen_watcher_watcher_errors_total[5m])`  

**What to watch**: Any errors need investigation. Common types:
- `connection_failed` - Network issues
- `permission_denied` - RBAC issues
- `parse_error` - Data format issues

#### 14. CRD Operation Duration (p95)
**Type**: Time series  
**Metric**: `histogram_quantile(0.95, zen_watcher_crd_operation_duration_seconds_bucket)`  

**Operations**:
- create
- update
- delete

**Normal latency**: < 100ms  
**Slow**: > 1s (check etcd)

**What to watch**: Increasing latency may indicate etcd performance issues.

#### 15. HTTP Requests
**Type**: Time series (stacked)  
**Metric**: `rate(zen_watcher_http_requests_total[5m])`  

**Endpoints**:
- /health (liveness)
- /ready (readiness)
- /metrics (Prometheus)
- /tools/status (status API)

**What to watch**: Unusual patterns in health checks = probe issues.

#### 16. HTTP Request Duration (p95)
**Type**: Time series  
**Metric**: `histogram_quantile(0.95, zen_watcher_http_request_duration_seconds_bucket)`  

**SLO**: 95% < 100ms  

**What to watch**: Slow endpoints = application performance issue.

---

## 🎯 Dashboard Variables

### datasource
**Type**: Datasource selector  
**Options**: All Prometheus datasources  
**Usage**: Select which Prometheus to query

### cluster
**Type**: Query variable  
**Multi-select**: No  
**Include All**: Yes

**Usage**: Filter dashboard by cluster in multi-cluster setup.

---

## 🔍 How to Read the Dashboard

### Normal State

**What you should see**:
- ✅ Health Status: Green
- ✅ Events/sec: Steady, predictable rate
- ✅ Active Events: Slowly growing or stable
- ✅ Critical Events: 0 or very low
- ✅ All watchers: Green (enabled)
- ✅ Goroutines: Stable count
- ✅ Memory: Within limits
- ✅ CPU: < 50%
- ✅ No errors in error panels

### Warning Signs

**Things to investigate**:
- ⚠️ Health Status: Not green
- ⚠️ Events/sec: Sudden spike
- ⚠️ Active Events: Rapid growth
- ⚠️ Critical Events: > 10
- ⚠️ Watcher errors: Any errors showing
- ⚠️ Memory: Steady climb
- ⚠️ CPU: > 80%
- ⚠️ CRD operations: Slow or failing

### Critical Issues

**Immediate action required**:
- 🚨 Health Status: Red
- 🚨 Critical Events: > 50
- 🚨 Events/sec: > 1000/sec sustained
- 🚨 Memory: At limit
- 🚨 CRD failures: High rate
- 🚨 Goroutines: > 1000

---

## 🎨 Dashboard Customization

### Add Custom Panel

1. Click "Add panel" in Grafana
2. Select visualization type
3. Add query:
   ```promql
   ```
4. Configure legend, colors, thresholds
5. Save dashboard

### Modify Existing Panel

1. Edit panel (click title → Edit)
2. Modify query, visualization, or options
3. Save changes
4. Export updated JSON

### Create Row

1. Add → Add row
2. Drag panels into row
3. Collapse/expand for organization
4. Save dashboard

---

## 📱 Dashboard on Mobile

Dashboard is responsive and works on mobile:

- Stats show clearly
- Time series are scrollable
- Variables work
- Auto-refresh continues

**Tip**: Create a simplified mobile dashboard with key stats only.

---

## 🔗 Dashboard Links

### Add Links to Related Dashboards

```json
{
  "links": [
    {
      "title": "Kubernetes Cluster",
      "url": "/d/kubernetes-cluster/kubernetes-cluster"
    },
    {
      "title": "Trivy Dashboard",
      "url": "/d/trivy/trivy-vulnerabilities"
    }
  ]
}
```

### Add Drill-Down Links

Click panel title → More → Add link to:
- Detailed event list (Kubernetes API)
- Log view (Loki)
- Alert manager (Prometheus)

---

## 🎯 Pro Tips

1. **Use Time Range Selector**
   - Last 5 minutes for real-time
   - Last hour for recent trends
   - Last 24 hours for daily patterns
   - Last 7 days for weekly analysis

2. **Use Variables**
   - Filter by cluster in multi-cluster setups
   - Quick switching between environments

3. **Set Up Alerts**
   - Link panels to Prometheus alerts
   - Visual indicators when alerts fire

4. **Share Dashboard**
   - Use snapshot feature
   - Share direct link
   - Export as PDF

5. **Create Playlists**
   - Rotate between multiple dashboards
   - Suitable for NOC displays

---

## 📊 Example Scenarios

### Scenario 1: Security Incident

**What you'll see**:
- Spike in Events/sec panel
- Critical Events counter increases
- Events by Severity shows red segment
- Specific watcher (Falco/Trivy) shows activity

**Action**: 
1. Check which source
2. View events: `kubectl get zenevents -l severity=critical`
3. Investigate affected resources
4. Follow incident response plan

### Scenario 2: Performance Degradation

**What you'll see**:
- CRD Operation Duration increasing
- HTTP Request Duration increasing
- CPU Usage climbing
- Memory Usage growing

**Action**:
1. Check resource limits
2. Review event volume
3. Consider scaling up
4. Check etcd performance

### Scenario 3: Watcher Failure

**What you'll see**:
- Watcher Status shows red (0)
- Watcher Errors panel shows spikes
- Events from that source stop appearing

**Action**:
1. Check watcher logs
2. Verify source tool is running
3. Check RBAC permissions
4. Review NetworkPolicy

---

## 🎓 Dashboard Best Practices

1. **Keep It Simple**
   - Focus on actionable metrics
   - Remove noise
   - Clear titles and labels

2. **Use Appropriate Visualizations**
   - Stats for current state
   - Time series for trends
   - Pie charts for distribution
   - Gauges for thresholds

3. **Set Meaningful Thresholds**
   - Green = normal operation
   - Yellow = attention needed
   - Red = action required

4. **Document Panels**
   - Add descriptions
   - Explain metrics
   - Link to runbooks

5. **Test Regularly**
   - Verify data shows correctly
   - Test with load
   - Validate alerts

---

## 🛠️ Troubleshooting Dashboard

### No Data Showing

```bash
# 1. Check metrics endpoint
kubectl exec -n zen-system deployment/zen-watcher -- \
  curl -s http://localhost:8080/metrics

# 2. Check ServiceMonitor
kubectl get servicemonitor -n zen-system

# 3. Check Prometheus targets
# Prometheus UI → Status → Targets → zen-watcher

# 4. Test query in Prometheus
# Execute: zen_watcher_health_status
```

### Panels Empty

1. Check datasource is selected
2. Verify variable values
3. Test query in Explore
4. Check time range

### Wrong Data

1. Verify cluster variable
2. Check label selectors
3. Review aggregation functions
4. Validate time range

---

## 📚 Resources

- [Grafana Documentation](https://grafana.com/docs/)
- [PromQL Guide](https://prometheus.io/docs/prometheus/latest/querying/basics/)
- [Dashboard Best Practices](https://grafana.com/docs/grafana/latest/best-practices/)

---

## 🎉 Enjoy Your Dashboard!

You now have:
- ✅ Real-time visibility
- ✅ Clear visualizations
- ✅ Actionable insights
- ✅ Historical analysis
- ✅ Alert integration

**Happy monitoring!** 📊


