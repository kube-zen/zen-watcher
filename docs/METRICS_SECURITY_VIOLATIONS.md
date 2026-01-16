# Security Violations Metrics

**Date**: 2025-01-27  
**Status**: ✅ **VERIFIED**

## Metric: `zen_watcher_destination_delivery_total`

**Type**: Prometheus CounterVec  
**Labels**: `["source", "destination_type", "status"]`

### Status Values

1. **`success`** - Successful resource creation
   - Incremented after successful Kubernetes API write
   - Includes latency tracking

2. **`failure`** - Regular creation failure
   - Network errors, API errors, resource conflicts
   - Includes latency tracking (if deliveryDuration > 0)

3. **`not_allowed`** - Security policy violation (NEW)
   - GVR not in allowlist (`ErrGVRNotAllowed`)
   - GVR categorically denied (`ErrGVRDenied`)
   - Namespace not in allowlist (`ErrNamespaceNotAllowed`)
   - Cluster-scoped resource not allowed (`ErrClusterScopedNotAllowed`)
   - **No latency tracking** (blocked before write attempt)

## Implementation Verification

### Code Locations

**Metric Definition**: `pkg/metrics/definitions.go:378-384`
```go
destinationDeliveryTotal := prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "zen_watcher_destination_delivery_total",
        Help: "Destination delivery attempts",
    },
    []string{"source", "destination_type", "status"},
)
```

**Success Tracking**: `pkg/watcher/observation_creator.go:411`
```go
oc.destinationMetrics.DestinationDeliveryTotal.WithLabelValues(source, "crd", "success").Inc()
```

**Security Violation Tracking**: `pkg/watcher/observation_creator.go:505`
```go
if isSecurityViolation {
    oc.destinationMetrics.DestinationDeliveryTotal.WithLabelValues(source, "crd", "not_allowed").Inc()
}
```

**Regular Failure Tracking**: `pkg/watcher/observation_creator.go:508`
```go
oc.destinationMetrics.DestinationDeliveryTotal.WithLabelValues(source, "crd", "failure").Inc()
```

### Security Violation Detection

**Location**: `pkg/watcher/observation_creator.go:489-493`
```go
isSecurityViolation := errors.Is(err, ErrGVRNotAllowed) ||
    errors.Is(err, ErrGVRDenied) ||
    errors.Is(err, ErrNamespaceNotAllowed) ||
    errors.Is(err, ErrClusterScopedNotAllowed)
```

## Prometheus Queries

### Count Security Violations
```promql
sum(rate(zen_watcher_destination_delivery_total{status="not_allowed"}[5m])) by (source, destination_type)
```

### Security Violation Rate
```promql
sum(rate(zen_watcher_destination_delivery_total{status="not_allowed"}[5m])) by (source) /
sum(rate(zen_watcher_destination_delivery_total[5m])) by (source)
```

### All Delivery Statuses
```promql
sum(rate(zen_watcher_destination_delivery_total[5m])) by (source, status)
```

## Alerting Recommendations

### Security Violation Alert
```yaml
- alert: ZenWatcherSecurityPolicyViolations
  expr: |
    sum(rate(zen_watcher_destination_delivery_total{status="not_allowed"}[5m])) by (source) > 0
  for: 1m
  labels:
    severity: warning
    component: security
  annotations:
    summary: "Security policy violations detected for {{$labels.source}}"
    description: "Source {{$labels.source}} attempted {{$value}} blocked writes in the last 5 minutes."
```

### High Security Violation Rate
```yaml
- alert: ZenWatcherHighSecurityViolationRate
  expr: |
    (
      sum(rate(zen_watcher_destination_delivery_total{status="not_allowed"}[5m])) by (source) /
      (sum(rate(zen_watcher_destination_delivery_total[5m])) by (source) + 0.001)
    ) > 0.1
  for: 5m
  labels:
    severity: critical
    component: security
  annotations:
    summary: "High security violation rate for {{$labels.source}}"
    description: "{{$value | humanizePercentage}} of delivery attempts from {{$labels.source}} are blocked by security policy."
```

## Verification Status

✅ **Metric properly defined** - CounterVec with correct labels  
✅ **Success tracking** - Incremented after successful writes  
✅ **Security violation tracking** - Incremented for policy violations  
✅ **Regular failure tracking** - Incremented for non-security failures  
✅ **Error detection** - Uses `errors.Is()` for proper error wrapping  
✅ **No latency for blocked writes** - Correctly skips latency for `not_allowed`  

## Test Coverage

All metrics paths are covered by unit tests:
- ✅ Security violations tracked as `not_allowed`
- ✅ Regular failures tracked as `failure`
- ✅ Successes tracked as `success`
