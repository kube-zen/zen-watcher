# Playbook: Integrating zen-watcher Observations with Kubewatch

## Overview

Kubewatch is a Kubernetes event watcher that can route events to various destinations (Slack, Teams, etc.). This playbook shows how to configure Kubewatch to watch zen-watcher Observations and route them based on severity and category.

**Note**: zen-watcher does not ship Kubewatch integration. This is a pattern for operators to implement.

## Integration Mode

**Direct Observations CRD Watch**: Kubewatch watches Observation CRDs via Kubernetes informers.

## Observations Fields That Matter

- `spec.source`: Tool identifier (e.g., "trivy", "kyverno")
- `spec.category`: Event category (security, compliance, performance, operations, cost)
- `spec.severity`: Severity level (critical, high, medium, low, info)
- `spec.eventType`: Type of event (vulnerability, policy_violation, etc.)
- `spec.resource`: Affected Kubernetes resource
- `metadata.labels`: Standard labels (`zen.io/source`, `zen.io/type`, `zen.io/priority`)

## Configuration

### 1. Install Kubewatch

```bash
helm repo add kubewatch https://charts.bitnami.com/bitnami
helm install kubewatch kubewatch/kubewatch \
  --set resourcesToWatch[0].kind=Observation \
  --set resourcesToWatch[0].apiVersion=zen.kube-zen.io/v1
```

### 2. Configure Slack Destination

```yaml
# kubewatch-config.yaml
slack:
  channel: "#security-alerts"
  token: "xoxb-your-token"
filters:
  - kind: Observation
    apiVersion: zen.kube-zen.io/v1
    labelSelector: "zen.io/priority=high,zen.io/category=security"
```

### 3. Example Label Selectors

**High-severity security events:**
```yaml
labelSelector: "zen.io/priority=high,zen.io/category=security"
```

**All critical events:**
```yaml
labelSelector: "zen.io/priority=critical"
```

**Trivy vulnerabilities only:**
```yaml
labelSelector: "zen.io/source=trivy"
```

## Example Queries

### Watch All Security Observations

```yaml
resourcesToWatch:
  - kind: Observation
    apiVersion: zen.kube-zen.io/v1
    labelSelector: "zen.io/category=security"
```

### Watch High/Critical Severity Only

```yaml
resourcesToWatch:
  - kind: Observation
    apiVersion: zen.kube-zen.io/v1
    labelSelector: "zen.io/priority in (high,critical)"
```

## Example Kubewatch Handler

```go
// Custom handler for Observations
func handleObservation(obs *Observation) {
    if obs.Spec.Severity == "critical" {
        sendToSlack("#critical-alerts", formatObservation(obs))
    } else if obs.Spec.Category == "security" {
        sendToSlack("#security-alerts", formatObservation(obs))
    }
}
```

## Routing Rules

### Route by Severity

- **Critical**: `#critical-alerts` channel
- **High**: `#security-alerts` channel
- **Medium/Low**: `#general-alerts` channel

### Route by Category

- **Security**: `#security-alerts`
- **Compliance**: `#compliance-alerts`
- **Performance**: `#performance-alerts`

## Related Documentation

- [Kubewatch Documentation](https://github.com/bitnami-labs/kubewatch)
- [zen-watcher Observation API](../OBSERVATION_API_PUBLIC_GUIDE.md)

