# Kubernetes Events API Watcher - Implementation Plan

## Overview

Add a watcher for Kubernetes Events API (`events.k8s.io/v1`) to capture operational events (pod scheduling failures, volume mounts, node pressure) and correlate them with security events.

## Why This Matters

- **Correlation**: Link operational issues (pod restarts) with security events (vulnerabilities)
- **AI Remediation**: Enable AI to suggest fixes for operational problems
- **Complete Picture**: See both security AND operational health in one place

## Implementation

### 1. Add Events API GVR

```go
// Add to main.go after other GVRs
eventsGVR := schema.GroupVersionResource{
    Group:    "events.k8s.io",
    Version:  "v1",
    Resource: "events",
}
```

### 2. Filter Important Events

Only create ZenAgentEvents for significant operational issues:

- **Pod Scheduling Failures**: `reason=FailedScheduling`
- **Volume Mount Issues**: `reason=FailedMount`, `reason=FailedAttachVolume`
- **Node Pressure**: `reason=NodeHasDiskPressure`, `reason=NodeHasMemoryPressure`
- **Image Pull Errors**: `reason=Failed`, `reason=ErrImagePull`
- **Container Crashes**: `reason=Failed`, `reason=CrashLoopBackOff`

### 3. Deduplication Key

```go
// Dedup by: namespace/kind/name/reason/message[:50]
dedupKey := fmt.Sprintf("%s/%s/%s/%s/%s",
    event.InvolvedObject.Namespace,
    event.InvolvedObject.Kind,
    event.InvolvedObject.Name,
    event.Reason,
    truncate(event.Message, 50))
```

### 4. Category Mapping

- `category: performance` - Resource pressure, scheduling delays
- `category: operations` - Pod failures, volume issues
- `category: security` - Only if related to security (e.g., image pull from untrusted registry)

### 5. Example ZenAgentEvent

```yaml
apiVersion: zen.kube-zen.io/v1
kind: ZenAgentEvent
metadata:
  generateName: k8s-event-
  namespace: default
  labels:
    source: k8s-events
    category: performance
    severity: MEDIUM
spec:
  source: k8s-events
  category: performance
  severity: MEDIUM
  eventType: pod-scheduling-failure
  detectedAt: "2025-01-27T10:00:00Z"
  resource:
    kind: Pod
    name: my-app-123
    namespace: default
  details:
    reason: FailedScheduling
    message: "0/3 nodes are available: 3 Insufficient memory"
    count: 5
    firstSeen: "2025-01-27T09:55:00Z"
    lastSeen: "2025-01-27T10:00:00Z"
```

## Configuration

### Environment Variables

```bash
# Enable K8s Events watcher (default: true)
K8S_EVENTS_ENABLED=true

# Filter by reason (comma-separated, empty = all)
K8S_EVENTS_FILTER_REASONS=FailedScheduling,FailedMount,NodeHasDiskPressure

# Minimum event count before creating ZenAgentEvent (default: 1)
K8S_EVENTS_MIN_COUNT=1

# Time window for event aggregation (default: 5m)
K8S_EVENTS_AGGREGATION_WINDOW=5m
```

## Integration with AI Remediations

Once operational events are captured, zen-brain can:

1. **Analyze**: "Pod scheduling failures due to memory pressure"
2. **Correlate**: "Same namespace has Trivy HIGH vulnerabilities"
3. **Remediate**: "Suggest resource limits, or upgrade vulnerable images"

## Benefits

- ✅ **Unified View**: Security + Operations in one place
- ✅ **AI-Powered**: AI can suggest fixes for operational issues
- ✅ **Correlation**: Link pod restarts to security events
- ✅ **Extensible**: Easy to add more event filters

## Future Enhancements

- **Pattern Detection**: Identify recurring operational patterns
- **Time-based Correlation**: Link events within time windows
- **Resource-based Correlation**: Link events affecting same resources
- **Causation Analysis**: Identify root causes (node issue → pod failures)

