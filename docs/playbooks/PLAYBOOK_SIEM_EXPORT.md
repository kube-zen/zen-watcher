# Playbook: Exporting zen-watcher Observations to SIEM/Log Stacks

## Overview

This playbook shows how to export zen-watcher Observations to SIEM systems (Splunk, ELK, etc.) and log aggregation stacks.

**Note**: zen-watcher does not ship SIEM integration. This is a pattern for operators to implement.

## Integration Mode

**Export via Log Forwarder Agent**: Deploy an agent that watches Observations and forwards them to external SIEM/log stacks.

## Observations Fields That Matter

- **All fields**: Full Observation context is valuable for SIEM
- `spec.source`: Tool identifier
- `spec.category`: Event category
- `spec.severity`: Severity level
- `spec.eventType`: Type of event
- `spec.resource`: Affected Kubernetes resource
- `spec.details`: Tool-specific details (preserved for full context)
- `metadata.labels`: Additional metadata
- `spec.detectedAt`: Timestamp for correlation

## Configuration

### 1. Deploy Log Forwarder Agent

```yaml
# log-forwarder.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: observation-log-forwarder
spec:
  replicas: 1
  template:
    spec:
      containers:
      - name: forwarder
        image: your-registry/observation-forwarder:latest
        env:
        - name: SIEM_ENDPOINT
          value: "https://splunk.example.com:8088"
        - name: SIEM_TOKEN
          valueFrom:
            secretKeyRef:
              name: siem-credentials
              key: token
```

### 2. Forwarder Implementation (Example)

```go
// observation-forwarder/main.go
func main() {
    // Watch Observations
    informer := watchObservations()
    
    // Forward to SIEM
    for obs := range informer.Events() {
        event := convertToSIEMFormat(obs)
        forwarder.Send(event)
    }
}

func convertToSIEMFormat(obs *Observation) SIEMEvent {
    return SIEMEvent{
        Timestamp: obs.Spec.DetectedAt,
        Source: obs.Spec.Source,
        Category: obs.Spec.Category,
        Severity: obs.Spec.Severity,
        EventType: obs.Spec.EventType,
        Resource: obs.Spec.Resource,
        Details: obs.Spec.Details,
        Labels: obs.Metadata.Labels,
    }
}
```

## SIEM Format Examples

### Splunk Format

```json
{
  "time": "2025-12-11T12:00:00Z",
  "source": "zen-watcher",
  "sourcetype": "k8s:observation",
  "event": {
    "source": "trivy",
    "category": "security",
    "severity": "high",
    "eventType": "vulnerability",
    "resource": {
      "kind": "Pod",
      "name": "my-pod",
      "namespace": "default"
    },
    "details": {
      "cve": "CVE-2024-1234",
      "package": "openssl"
    }
  }
}
```

### ELK (Elasticsearch) Format

```json
{
  "@timestamp": "2025-12-11T12:00:00Z",
  "source": "zen-watcher",
  "observation": {
    "source": "trivy",
    "category": "security",
    "severity": "high",
    "eventType": "vulnerability",
    "resource": {
      "kind": "Pod",
      "name": "my-pod",
      "namespace": "default"
    },
    "details": {
      "cve": "CVE-2024-1234",
      "package": "openssl"
    }
  },
  "labels": {
    "zen.io/source": "trivy",
    "zen.io/category": "security",
    "zen.io/priority": "high"
  }
}
```

## Filtering and Routing

### Filter by Severity

```go
func shouldForward(obs *Observation) bool {
    return obs.Spec.Severity == "critical" || obs.Spec.Severity == "high"
}
```

### Filter by Category

```go
func shouldForward(obs *Observation) bool {
    return obs.Spec.Category == "security" || obs.Spec.Category == "compliance"
}
```

### Route to Different SIEM Endpoints

```go
func routeObservation(obs *Observation) string {
    switch obs.Spec.Category {
    case "security":
        return "https://security-siem.example.com"
    case "compliance":
        return "https://compliance-siem.example.com"
    default:
        return "https://general-siem.example.com"
    }
}
```

## Example Queries

### Splunk Query: High Severity Security Events

```
index=kubernetes sourcetype=k8s:observation category=security severity=high
| stats count by source, eventType
```

### ELK Query: Critical Observations

```json
{
  "query": {
    "bool": {
      "must": [
        { "term": { "observation.severity": "critical" } }
      ]
    }
  }
}
```

## Related Documentation

- [Splunk Documentation](https://docs.splunk.com/)
- [ELK Stack Documentation](https://www.elastic.co/guide/)
- [zen-watcher Observation API](../OBSERVATION_API_PUBLIC_GUIDE.md)

