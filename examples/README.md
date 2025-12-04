# Zen Watcher - Examples

This directory contains example configurations and integrations for Zen Watcher with popular observability tools.

## 📁 Files

### 1. **grafana-dashboard.json**
Grafana dashboard template for visualizing security and compliance events.

**Features**:
- Event count by category (security/compliance)
- Severity distribution pie chart
- Timeline of security events
- Critical events table

**Usage**:
1. Open Grafana
2. Go to Dashboards → Import
3. Upload `grafana-dashboard.json`
4. Configure Kubernetes API datasource
5. Select namespace: `zen-system`

### 2. **prometheus-servicemonitor.yaml**
Prometheus ServiceMonitor and alerting rules for monitoring Zen Watcher.

**Includes**:
- ServiceMonitor for metrics collection
- Alert rule for high critical events
- Alert rule for watcher downtime

**Usage**:
```bash
# Requires Prometheus Operator
kubectl apply -f prometheus-servicemonitor.yaml

# Verify ServiceMonitor
kubectl get servicemonitor -n zen-system zen-watcher

# Check alerts in Prometheus UI
# Alert: HighNumberOfCriticalEvents
# Alert: ZenWatcherDown
```

### 3. **loki-promtail-config.yaml**
Promtail configuration for collecting Zen Watcher logs into Loki.

**Features**:
- Scrapes zen-watcher pod logs
- Extracts structured log fields
- Labels for filtering (level, category, source)

**Usage**:
```bash
# Apply to your Promtail deployment
kubectl apply -f loki-promtail-config.yaml

# Query in Loki/Grafana
{namespace="zen-system", app="zen-watcher"}
{namespace="zen-system", category="security"}
{namespace="zen-system", source="trivy"}
```

### 4. **query-examples.sh**
Bash script with kubectl query examples.

**Includes**:
- Basic queries (all events, by category, by source)
- Filtering examples (severity, combined filters)
- JSON queries with jq
- Count aggregations
- Export commands

**Usage**:
```bash
# Make executable (already done)
chmod +x query-examples.sh

# Run all examples
./query-examples.sh

# Run individual commands from the script
```

---

## 🎯 Quick Start Examples

### Query Events

```bash
# All events
kubectl get zenevents -n zen-system

# Security events
kubectl get zenevents -n zen-system -l category=security

# Compliance events
kubectl get zenevents -n zen-system -l category=compliance

# Custom category events (performance, observability, etc.)
kubectl get zenevents -n zen-system -l category=performance

# Critical vulnerabilities
kubectl get zenevents -n zen-system -l severity=critical,source=trivy

# Falco runtime threats
kubectl get zenevents -n zen-system -l source=falco
```

### Export Events

```bash
# Export all events to YAML
kubectl get zenrecommendations -n zen-system -o yaml > events-backup.yaml

# Export as JSON
kubectl get zenrecommendations -n zen-system -o json > events.json

# Export specific fields
kubectl get zenrecommendations -n zen-system -o custom-columns=\
NAME:.metadata.name,\
CATEGORY:.spec.category,\
SOURCE:.spec.source,\
SEVERITY:.spec.severity,\
ISSUE:.spec.issue
```

### Watch Events in Real-Time

```bash
# Watch all events
kubectl get zenrecommendations -n zen-system --watch

# Watch critical events only
kubectl get zenrecommendations -n zen-system -l severity=critical --watch
```

### Count and Aggregate

```bash
# Count events by source
kubectl get zenrecommendations -n zen-system -o json | \
  jq -r '.items[] | .spec.source' | sort | uniq -c

# Count by category
kubectl get zenrecommendations -n zen-system -o json | \
  jq -r '.items[] | .spec.category' | sort | uniq -c

# Count by severity
kubectl get zenrecommendations -n zen-system -o json | \
  jq -r '.items[] | .spec.severity' | sort | uniq -c
```

---

## 🔌 Integration Examples

### Grafana Dashboard Query

When setting up Grafana dashboard panels:

**Datasource**: Kubernetes API

**Query**:
```
/apis/zen.kube-zen.io/v1/namespaces/zen-system/zenrecommendations
```

**With label selector**:
```
/apis/zen.kube-zen.io/v1/namespaces/zen-system/zenrecommendations?labelSelector=category=security
```

### Prometheus Queries

If using kube-state-metrics with CRD support:

```promql
# Count of recommendations
count(kube_customresource_zenrecommendation)

# By severity
count(kube_customresource_zenrecommendation{severity="critical"})

# By category
count(kube_customresource_zenrecommendation) by (category)

# By source
count(kube_customresource_zenrecommendation) by (source)
```

### Loki Queries

Query Zen Watcher logs in Loki:

```logql
# All logs
{namespace="zen-system", app="zen-watcher"}

# Security events
{namespace="zen-system", category="security"}

# Error logs
{namespace="zen-system", app="zen-watcher"} |= "ERROR"

# Critical events
{namespace="zen-system", severity="critical"}

# Specific source
{namespace="zen-system", source="trivy"}
```

---

## 📊 Dashboard Ideas

### Security Dashboard
- Total security events (stat)
- Events by severity (pie chart)
- Security timeline (time series)
- Critical vulnerabilities (table)
- Runtime threats (table)
- Policy violations (table)

### Compliance Dashboard
- Total compliance events (stat)
- Audit events timeline (time series)
- Failed benchmarks (table)
- Compliance score (gauge)
- Top violated policies (bar chart)

### Overview Dashboard
- Events by category (stat panels)
- Events by source (pie chart)
- Recent events (table)
- Event trend (time series)
- Watcher health (stat)

---

## 🛠️ Customization

### Modify Dashboard
1. Import the dashboard
2. Edit panels as needed
3. Add custom queries
4. Export and save

### Custom Alerts
Edit `prometheus-servicemonitor.yaml` to add alerts:

```yaml
- alert: YourCustomAlert
  expr: your_promql_query > threshold
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "Your alert summary"
```

### Custom Log Parsing
Edit `loki-promtail-config.yaml` pipeline stages:

```yaml
pipeline_stages:
  - json:
      expressions:
        your_field: your_field
  - labels:
      your_field:
```

---

## 💡 Tips

1. **Use Labels**: Query efficiency with label selectors
2. **Set Retention**: Configure CRD lifecycle (TTL controllers)
3. **Archive Events**: Periodic export for long-term storage
4. **Custom Dashboards**: Create team-specific views
5. **Alert Tuning**: Adjust thresholds for your environment

---

## 📚 Additional Resources

- [Grafana Documentation](https://grafana.com/docs/)
- [Prometheus Operator](https://github.com/prometheus-operator/prometheus-operator)
- [Loki Documentation](https://grafana.com/docs/loki/)
- [Kubernetes CRDs](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/)

---

## 🤝 Contributing Examples

Have a great integration example? Please contribute!

1. Add your example to this directory
2. Update this README
3. Submit a pull request

Examples we'd love to see:
- Splunk integration
- Elasticsearch integration
- Slack/Discord webhooks
- Custom exporters
- Terraform/Helm charts

