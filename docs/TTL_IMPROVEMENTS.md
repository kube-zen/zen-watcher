# TTL-Based Auto-Cleanup Improvements

## Problem Statement

**Root Cause**: Kubernetes API Server Throttling

- Kubernetes API server has built-in rate limiting (~5 QPS per client for writes)
- Large-scale observation deletion hits these limits hard
- etcd write amplification makes it worse (finalizers, status updates, watches)
- Sequential `kubectl delete` operations cause 12+ hour cleanup times

## Solution: TTL-Based Auto-Cleanup (Kubernetes Native)

### 1. Garbage Collector (Built-in)

**File**: `pkg/gc/collector.go`

zen-watcher includes a built-in garbage collector that automatically deletes observations when their TTL expires:

- Periodically scans Observation CRDs (default: every 1 hour)
- Checks `spec.ttlSecondsAfterCreation` field (Kubernetes native style, like Jobs)
- Deletes expired observations automatically
- Respects API server rate limits with chunking and timeouts
- Uses `spec.ttlSecondsAfterCreation` field (not annotations) - aligned with Kubernetes Job TTL pattern

**Usage**: The GC runs automatically when zen-watcher is deployed. No manual configuration needed.

**Alternative**: You can use `k8s-ttl-controller` instead if you prefer annotation-based TTL, but zen-watcher's built-in GC uses the spec field (more aligned with Kubernetes patterns like Jobs).

### 2. Enhanced Field Mapping with TTL Support

**Files**: 
- `pkg/adapter/generic/types.go` - Enhanced FieldMapping struct
- `pkg/watcher/field_mapper.go` - Field mapper implementation

**New Features**:

#### Constant Values
```yaml
fieldMapping:
  - constant: "1w"  # Static TTL value
    to: ttlSecondsAfterCreation
```

#### Static Mappings (Severity-Based TTL)
```yaml
fieldMapping:
  - from: severity
    to: ttlSecondsAfterCreation
    staticMappings:
      CRITICAL: "1814400"  # 3 weeks in seconds
      HIGH: "1209600"       # 2 weeks in seconds
      MEDIUM: "604800"      # 1 week in seconds
      LOW: "259200"         # 3 days in seconds
```

### 3. Per-Ingester TTL Configuration

**File**: `examples/ingesters/stress-test-ingester.yaml`

Ingesters can now configure TTL for observations:

```yaml
apiVersion: zen.kube-zen.io/v1alpha1
kind: Ingester
metadata:
  name: trivy-ingester
spec:
  source: trivy
  observationTemplate:
    # Option 1: Static TTL
    ttl: "1w"
    
    # Option 2: Dynamic TTL based on severity
    fieldMapping:
      - from: severity
        to: ttlSecondsAfterCreation
        staticMappings:
          CRITICAL: "3w"
          HIGH: "2w"
          MEDIUM: "1w"
          LOW: "3d"
```

### 4. Sustainable Stress Test Strategy

**File**: `scripts/benchmark/sustainable-stress-test.sh`

New stress test approach that avoids API server limits:

**Key Features**:
- Rate-limited creation: Stays under ~5 QPS API server limits
- Short TTL: Observations auto-delete during test (no manual cleanup)
- Sustained load: Tests performance over time, not peak volume
- No manual cleanup: TTL controller handles deletion

**Usage**:
```bash
./scripts/benchmark/sustainable-stress-test.sh \
  --rate-limit 100 \          # 100 observations/minute (under API limits)
  --duration 60 \             # 1 hour test
  --ttl 5m \                  # Short TTL for auto-cleanup
  --concurrent-ingesters 10   # Multiple ingester sources
```

**Benefits**:
- ✅ No 12+ hour cleanup times
- ✅ Stays under API server rate limits
- ✅ Tests realistic sustained load
- ✅ Automatic cleanup via TTL

## TTL Format Support

The field mapper supports multiple TTL formats:

- **Duration strings**: `"1w"`, `"3d"`, `"24h"`, `"5m"`, `"30s"`
- **Seconds**: `"604800"` (7 days)
- **Go duration**: `"168h"`, `"7d"` (parsed by `time.ParseDuration`)

## Implementation Approach

### Sequential Deletion (Not Recommended)
```bash
kubectl delete observations -l stress-test=true
# Result: 12+ hours for 10,000 observations
# Problem: Hits API server rate limits
```

### New Approach (TTL-Based)
```yaml
# Observations created with short TTL
spec:
  ttlSecondsAfterCreation: 300  # 5 minutes
# Result: Auto-deleted by TTL controller
# Benefit: No manual cleanup, avoids rate limits
```

## Implementation Status

✅ **Completed**:
- Built-in garbage collector (uses `spec.ttlSecondsAfterCreation` field)
- Enhanced field mapping with constant and static mappings
- Field mapper with TTL parsing
- Sustainable stress test script
- Stress test ingester examples
- Prometheus metrics for GC (deletions, errors, duration)

**Note**: zen-watcher uses `spec.ttlSecondsAfterCreation` (field-based, like Kubernetes Jobs) rather than annotations. This is more aligned with Kubernetes native patterns. If you prefer to use `k8s-ttl-controller` (annotation-based), you can disable zen-watcher's GC and use annotations instead.

## Testing

### Test TTL Controller
```bash
# Create observation with short TTL
kubectl apply -f - <<EOF
apiVersion: zen.kube-zen.io/v1
kind: Observation
metadata:
  name: test-ttl
  namespace: zen-system
spec:
  source: test
  category: operations
  severity: low
  eventType: test
  ttlSecondsAfterCreation: 60  # 1 minute
EOF

# Wait 1 minute, then check
sleep 60
kubectl get observation test-ttl -n zen-system  # Should be deleted
```

### Test Sustainable Stress Test
```bash
./scripts/benchmark/sustainable-stress-test.sh \
  --rate-limit 50 \
  --duration 10 \
  --ttl 5m
```

## Related Documentation

- [Performance Tuning Guide](PERFORMANCE_TUNING.md) - Resource allocation and TTL configuration
- [Troubleshooting Guide](TROUBLESHOOTING.md) - Common issues and solutions
- [Observation CRD API](OBSERVATION_API_PUBLIC_GUIDE.md) - CRD schema reference

