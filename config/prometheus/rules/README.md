# Prometheus Alerting Rules for Zen Watcher

This directory contains Prometheus alerting rules for Zen Watcher's enterprise alerting system.

## Files

- **security-alerts.yml** - 40+ security event alerts covering:
  - Falco runtime threats
  - Trivy vulnerability scanning
  - Kube-Bench CIS compliance
  - Checkov IaC security
  - Kubernetes Audit suspicious activity
  - Kyverno policy violations
  - Multi-source security correlation

- **performance-alerts.yml** - 25+ performance and system health alerts covering:
  - Processing latency (p95, p99)
  - Throughput monitoring
  - Resource utilization (CPU, memory, cache)
  - Predictive capacity alerts
  - Source-specific performance issues

## Deployment

### Option 1: ConfigMap (Recommended for kube-prometheus-stack)

```bash
kubectl create configmap prometheus-alerts \
  --from-file=security-alerts.yml \
  --from-file=performance-alerts.yml \
  -n monitoring

# Label for Prometheus to discover
kubectl label configmap prometheus-alerts \
  prometheus=kube-prometheus-stack \
  role=alert-rules \
  -n monitoring
```

### Option 2: PrometheusRule CRD (For Prometheus Operator)

```bash
kubectl apply -f security-alerts.yml -n zen-system
kubectl apply -f performance-alerts.yml -n zen-system
```

## Alert Severity Levels

| Severity | Response Time | Escalation |
|----------|---------------|------------|
| **Critical** | 0-30 minutes | PagerDuty critical |
| **Critical** | 0-4 hours | PagerDuty high |
| **Warning** | 0-2 hours | PagerDuty medium |
| **Warning** | 0-24 hours | Email + Slack |
| **Info** | 1 week | Jira ticket |

## Integration

These alerting rules integrate with:
- **AlertManager** - See `../alertmanager/` for configuration
- **Grafana Dashboards** - Alerts link to dashboards for investigation
- **Incident Response** - See `../../docs/alerting/` for runbooks

## Documentation

For detailed documentation, see:
- [Security Alerting Overview](../../docs/alerting/SECURITY_ALERTING_OVERVIEW.md)
- [Alerting Integration Guide](../../docs/alerting/alerting-integration-guide.md)
- [Security Incident Response](../../docs/alerting/SECURITY_ALERTING_OVERVIEW.md)

## Testing

See [Alert Testing Procedures](../../docs/alerting/alert-testing-procedures.md) for validation and testing workflows.

