# Ecosystem Integrations

zen-watcher Observations can be consumed by various ecosystem tools. This document provides an overview of integration patterns.

## Integration Patterns

### Direct CRD Watch

Tools can watch Observation CRDs directly via Kubernetes informers:

- **Kubewatch**: Route Observations to Slack, Teams, etc.
- **Robusta**: Trigger playbooks based on Observations
- **Custom Controllers**: Build your own controller to process Observations

### Export via Agent

Deploy an agent that watches Observations and exports to external systems:

- **Prometheus Exporter**: Convert Observations to Prometheus metrics
- **Log Forwarder**: Forward Observations to SIEM/log stacks

## Integration Playbooks

Detailed playbooks for integrating zen-watcher Observations with common ecosystem tools:

- [Kubewatch Integration](playbooks/PLAYBOOK_KUBEWATCH.md) - Route Observations to Slack, Teams, etc.
- [Robusta Integration](playbooks/PLAYBOOK_ROBUSTA.md) - Trigger Robusta playbooks based on Observations
- [Prometheus/Alertmanager Integration](playbooks/PLAYBOOK_PROM_ALERTS.md) - Export Observations as Prometheus metrics and alerts
- [SIEM/Log Export](playbooks/PLAYBOOK_SIEM_EXPORT.md) - Forward Observations to SIEM/log stacks

**Note**: zen-watcher does not ship these integrations. These are patterns for operators to implement.

## Related Documentation

- [Observation API Public Guide](OBSERVATION_API_PUBLIC_GUIDE.md) - Complete Observation CRD API reference
- [Go SDK Overview](GO_SDK_OVERVIEW.md) - Go SDK for programmatic Observation handling

