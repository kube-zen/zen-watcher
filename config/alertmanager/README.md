# AlertManager Configuration for Zen Watcher

This directory contains AlertManager configuration for Zen Watcher's enterprise alerting system.

## Files

- **alertmanager.yml** - Production AlertManager configuration with:
  - Multi-channel notifications (Email, Slack, PagerDuty)
  - Intelligent routing by severity and component
  - Automated escalation policies
  - Grouping and deduplication
  - Silence management

- **email.tmpl** - Professional HTML email templates for alert notifications

- **kubernetes-manifest.yaml** - Kubernetes deployment manifest for AlertManager

## Deployment

### Deploy AlertManager

```bash
kubectl apply -f kubernetes-manifest.yaml
```

### Configure Notification Channels

1. **Update alertmanager.yml** with your notification channel credentials:
   - SMTP server for email
   - Slack webhook URL
   - PagerDuty integration key

2. **Create secrets** for sensitive credentials:
   ```bash
   kubectl create secret generic alertmanager-secrets \
     --from-literal=smtp-password='your-password' \
     --from-literal=slack-url='your-slack-webhook' \
     --from-literal=pagerduty-key='your-pagerduty-key' \
     -n monitoring
   ```

3. **Update kubernetes-manifest.yaml** to reference the secrets

## Configuration Overview

### Routing Structure

- **Critical Security Alerts** → Security On-Call (PagerDuty)
- **Critical Infrastructure** → Infrastructure On-Call (PagerDuty)
- **Warning Alerts** → Security Team (Slack + Email)
- **Info Alerts** → Engineering Team (Email)

### Escalation Policies

- **Immediate Critical**: 0-30 minutes response
- **High Priority**: 0-4 hours response
- **Warning**: 0-24 hours response
- **Info**: 1 week response

## Integration

AlertManager integrates with:
- **Prometheus** - Receives alerts from PrometheusRule resources
- **Grafana** - Alert visualization and management
- **Notification Channels** - Email, Slack, PagerDuty

## Documentation

For detailed documentation, see:
- [Alerting Integration Guide](../../docs/alerting/alerting-integration-guide.md)
- [Silence Management](../../docs/alerting/silence-management.md)
- [Security Incident Response](../../docs/alerting/SECURITY_INCIDENT_RESPONSE.md)

## Testing

See [Alert Testing Procedures](../../docs/alerting/alert-testing-procedures.md) for validation workflows.

