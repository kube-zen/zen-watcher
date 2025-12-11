# Playbook: Integrating zen-watcher Observations with Prometheus/Alertmanager

## Overview

This playbook shows how to export zen-watcher Observations to Prometheus metrics and create Alertmanager alerts based on Observations.

**Note**: zen-watcher does not ship Prometheus integration. This is a pattern for operators to implement.

## Integration Mode

**Export via Prometheus Exporter**: Deploy a custom exporter that watches Observations and exposes Prometheus metrics.

## Observations Fields That Matter

- `spec.severity`: Maps to alert severity (critical, high, medium, low, info)
- `spec.category`: Maps to alert category label
- `spec.source`: Source label
- `spec.eventType`: Event type label
- `metadata.labels`: Additional labels for grouping

## Configuration

### 1. Deploy Observation Exporter

Create a simple exporter that watches Observations:

```yaml
# observation-exporter.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: observation-exporter
spec:
  replicas: 1
  template:
    spec:
      containers:
      - name: exporter
        image: your-registry/observation-exporter:latest
        env:
        - name: OBSERVATION_NAMESPACE
          value: "default"
```

### 2. Exporter Implementation (Example)

```go
// observation-exporter/main.go
func main() {
    // Watch Observations
    informer := watchObservations()
    
    // Expose Prometheus metrics
    http.Handle("/metrics", promhttp.Handler())
    
    // Convert Observations to metrics
    for obs := range informer.Events() {
        observationCount.WithLabelValues(
            obs.Spec.Source,
            obs.Spec.Category,
            obs.Spec.Severity,
        ).Inc()
    }
}
```

## Prometheus Metrics

### Observation Count Metric

```prometheus
# HELP zen_observations_total Total number of Observations
# TYPE zen_observations_total counter
zen_observations_total{source="trivy",category="security",severity="high"} 42
```

### Observation Age Metric

```prometheus
# HELP zen_observation_age_seconds Age of Observation in seconds
# TYPE zen_observation_age_seconds gauge
zen_observation_age_seconds{source="trivy",category="security"} 3600
```

## Alertmanager Rules

### Alert: High Severity Security Observations

```yaml
# alertmanager-rules.yaml
groups:
  - name: zen_observations
    rules:
      - alert: HighSeveritySecurityObservation
        expr: |
          rate(zen_observations_total{category="security",severity="high"}[5m]) > 0
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High severity security observation detected"
          description: "Source: {{ $labels.source }}, Type: {{ $labels.eventType }}"
```

### Alert: Critical Observations

```yaml
      - alert: CriticalObservation
        expr: |
          rate(zen_observations_total{severity="critical"}[5m]) > 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Critical observation detected"
          description: "{{ $labels.source }}: {{ $labels.eventType }}"
```

## Example Queries

### Count Observations by Source

```promql
sum by (source) (zen_observations_total)
```

### Rate of High Severity Observations

```promql
rate(zen_observations_total{severity="high"}[5m])
```

### Observations by Category

```promql
sum by (category) (zen_observations_total)
```

## Alertmanager Configuration

```yaml
# alertmanager-config.yaml
route:
  routes:
    - match:
        severity: critical
      receiver: critical-alerts
    - match:
        category: security
      receiver: security-alerts
receivers:
  - name: critical-alerts
    slack_configs:
      - channel: '#critical-alerts'
  - name: security-alerts
    slack_configs:
      - channel: '#security-alerts'
```

## Related Documentation

- [Prometheus Documentation](https://prometheus.io/docs/)
- [Alertmanager Documentation](https://prometheus.io/docs/alerting/latest/alertmanager/)
- [zen-watcher Observation API](../OBSERVATION_API_PUBLIC_GUIDE.md)

