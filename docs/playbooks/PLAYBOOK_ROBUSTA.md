# Playbook: Integrating zen-watcher Observations with Robusta

## Overview

Robusta is a Kubernetes observability and automation platform. This playbook shows how to configure Robusta to watch zen-watcher Observations and trigger playbooks based on events.

**Note**: zen-watcher does not ship Robusta integration. This is a pattern for operators to implement.

## Integration Mode

**Direct Observations CRD Watch**: Robusta watches Observation CRDs via Kubernetes informers and triggers playbooks.

## Observations Fields That Matter

- `spec.source`: Tool identifier
- `spec.category`: Event category (security, compliance, performance, operations, cost)
- `spec.severity`: Severity level (critical, high, medium, low, info)
- `spec.eventType`: Type of event
- `spec.resource`: Affected Kubernetes resource (for tracking)
- `spec.details`: Tool-specific details (for context)

## Configuration

### 1. Install Robusta

```bash
helm repo add robusta https://robusta-charts.storage.googleapis.com
helm install robusta robusta/robusta \
  --set clusterName=my-cluster
```

### 2. Configure Observation Watcher

Create a Robusta playbook that watches Observations:

```yaml
# robusta-playbook-observations.yaml
apiVersion: actions.robusta.dev/v1
kind: Playbook
metadata:
  name: handle-observations
spec:
  triggers:
    - on_kubernetes_event:
        kind: Observation
        apiVersion: zen.kube-zen.io/v1
        labelSelector: "zen.io/priority=high"
  actions:
    - logs_enricher:
        logs: "Observation: {{ event.spec.source }} - {{ event.spec.eventType }}"
    - custom_action:
        action_name: "handle_observation"
```

## Example Robusta Policies

### Policy: Handle Critical Security Issues

```yaml
apiVersion: actions.robusta.dev/v1
kind: Playbook
metadata:
  name: auto-remediate-critical-security
spec:
  triggers:
    - on_kubernetes_event:
        kind: Observation
        apiVersion: zen.kube-zen.io/v1
        labelSelector: "zen.io/category=security,zen.io/priority=critical"
  actions:
    - pod_runner:
        image: busybox
        command: |
          echo "Critical security issue detected: {{ event.spec.eventType }}"
          # Add response action logic here
```

### Policy: Notify on Compliance Violations

```yaml
apiVersion: actions.robusta.dev/v1
kind: Playbook
metadata:
  name: notify-compliance-violations
spec:
  triggers:
    - on_kubernetes_event:
        kind: Observation
        apiVersion: zen.kube-zen.io/v1
        labelSelector: "zen.io/category=compliance"
  actions:
    - slack:
        channel: "#compliance"
        message: |
          Compliance violation detected:
          Source: {{ event.spec.source }}
          Type: {{ event.spec.eventType }}
          Resource: {{ event.spec.resource.kind }}/{{ event.spec.resource.name }}
```

## Example Queries

### Watch All Security Observations

```yaml
triggers:
  - on_kubernetes_event:
      kind: Observation
      apiVersion: zen.kube-zen.io/v1
      labelSelector: "zen.io/category=security"
```

### Watch High/Critical Severity

```yaml
triggers:
  - on_kubernetes_event:
      kind: Observation
      apiVersion: zen.kube-zen.io/v1
      labelSelector: "zen.io/priority in (high,critical)"
```

### Watch Specific Source

```yaml
triggers:
  - on_kubernetes_event:
      kind: Observation
      apiVersion: zen.kube-zen.io/v1
      labelSelector: "zen.io/source=trivy"
```

## Accessing Observation Fields

In Robusta playbooks, access Observation fields via `event.spec.*`:

```yaml
actions:
  - custom_action:
      action_name: "log_observation"
      action_params:
        source: "{{ event.spec.source }}"
        category: "{{ event.spec.category }}"
        severity: "{{ event.spec.severity }}"
        eventType: "{{ event.spec.eventType }}"
        resource: "{{ event.spec.resource.name }}"
```

## Related Documentation

- [Robusta Documentation](https://docs.robusta.dev/)
- [zen-watcher Observation API](../OBSERVATION_API_PUBLIC_GUIDE.md)

