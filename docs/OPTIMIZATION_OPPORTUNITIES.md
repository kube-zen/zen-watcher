# Optimization Opportunities

This document identifies optimization opportunities discovered after refactoring. These are prioritized by impact and effort.

## High Priority (High Impact, Low Effort)

### 1. FieldExtractor Cache Key Generation
**Location**: `pkg/watcher/field_extractor.go`
**Issue**: Using `fmt.Sprintf("%v", path)` for cache keys is inefficient
**Impact**: High - called in hot path for every observation
**Current Code**:
```go
cacheKey := fmt.Sprintf("%v", path)
```

**Optimization**: Use string join or bytes.Buffer for better performance
```go
// Option 1: Use strings.Join (simpler, good for small paths)
cacheKey := strings.Join(path, ":")

// Option 2: Use sync.Map with []string key directly (no conversion needed)
// Change fieldPathCache to sync.Map[string][]string
```

**Expected Improvement**: ~10-15% reduction in field extraction overhead

### 2. String Formatting in Hot Paths
**Location**: Multiple files (`observation_creator.go`, `rules.go`, `deduper.go`)
**Issue**: Excessive use of `fmt.Sprintf("%v", value)` for type conversion
**Impact**: Medium-High - creates temporary allocations
**Current Code**:
```go
source = fmt.Sprintf("%v", sourceVal)
```

**Optimization**: Use type assertions with fallback
```go
if str, ok := sourceVal.(string); ok {
    source = str
} else {
    source = fmt.Sprintf("%v", sourceVal) // Only when needed
}
```

**Expected Improvement**: ~5-10% reduction in allocations

### 3. Deduper Lock Granularity
**Location**: `pkg/dedup/deduper.go:ShouldCreateWithContent`
**Issue**: Entire function holds write lock, blocking all concurrent requests
**Impact**: High - major bottleneck for high-throughput scenarios
**Current Code**:
```go
d.mu.Lock()
defer d.mu.Unlock()
// ... entire function body ...
```

**Optimization**: Use fine-grained locking with read locks where possible
```go
// Use RLock for read-only operations
d.mu.RLock()
// ... read operations ...
d.mu.RUnlock()

// Only use Lock for write operations
d.mu.Lock()
// ... write operations ...
d.mu.Unlock()
```

**Expected Improvement**: ~30-50% improvement in concurrent throughput

### 4. Repeated Map Lookups
**Location**: `pkg/filter/rules.go:AllowWithReason`
**Issue**: Multiple lookups on same map
**Impact**: Low-Medium - small overhead but adds up
**Current Code**:
```go
sourceVal, _, _ := unstructured.NestedFieldCopy(observation.Object, "spec", "source")
// ... later ...
source, sourceFilter := f.extractSourceAndFilter(observation, config)
```

**Optimization**: Cache extracted values
```go
// Extract once, reuse
fields := f.extractObservationFields(observation)
source := fields.source
```

**Expected Improvement**: ~2-5% reduction in filter overhead

## Medium Priority (Medium Impact, Medium Effort)

### 5. String Lowercasing in Hot Path
**Location**: `pkg/filter/rules.go:AllowWithReason`
**Issue**: `strings.ToLower()` called on every filter check
**Impact**: Medium - creates new string allocation
**Current Code**:
```go
source = strings.ToLower(fmt.Sprintf("%v", sourceVal))
```

**Optimization**: Cache lowercased source strings
```go
// Use sync.Map or regular map with mutex to cache lowercased sources
var sourceCache sync.Map
if cached, ok := sourceCache.Load(source); ok {
    source = cached.(string)
} else {
    source = strings.ToLower(source)
    sourceCache.Store(source, source)
}
```

**Expected Improvement**: ~3-7% reduction in filter overhead

### 6. Observation Field Extraction Optimization
**Location**: `pkg/watcher/observation_creator.go:extractDedupKey`
**Issue**: Multiple field extractions with type assertions
**Impact**: Medium - called for every observation
**Current Code**:
```go
if ns, ok := resourceVal["namespace"].(string); ok {
    namespace = ns
} else if ns, ok := resourceVal["namespace"].(interface{}); ok {
    namespace = fmt.Sprintf("%v", ns)
}
```

**Optimization**: Use helper function to reduce code duplication
```go
func extractStringField(m map[string]interface{}, key string) string {
    if val, ok := m[key].(string); ok {
        return val
    }
    if val, ok := m[key]; ok {
        return fmt.Sprintf("%v", val)
    }
    return ""
}
```

**Expected Improvement**: ~2-4% reduction in extraction overhead

### 7. Channel Buffer Sizing
**Location**: `cmd/zen-watcher/main.go`
**Issue**: Fixed buffer sizes may not be optimal for all scenarios
**Impact**: Medium - affects backpressure handling
**Current Code**:
```go
falcoAlertsChan := make(chan map[string]interface{}, 100)
auditEventsChan := make(chan map[string]interface{}, 200)
```

**Optimization**: Make buffer sizes configurable via environment variables
```go
falcoBufferSize := getEnvInt("FALCO_BUFFER_SIZE", 100)
auditBufferSize := getEnvInt("AUDIT_BUFFER_SIZE", 200)
```

**Expected Improvement**: Better handling of traffic spikes

## Low Priority (Low Impact, Low Effort)

### 8. Logger Creation
**Location**: Multiple files
**Issue**: Creating new logger instances in hot paths
**Impact**: Low - but could be optimized
**Current Code**:
```go
logger := sdklog.NewLogger("zen-watcher")
```

**Optimization**: Reuse logger instances where possible
```go
// Create once, reuse
var logger = sdklog.NewLogger("zen-watcher")
```

**Expected Improvement**: ~1-2% reduction in allocations

### 9. Error Message Formatting
**Location**: Multiple files
**Issue**: Using `fmt.Errorf` with string formatting
**Impact**: Low - only affects error paths
**Current Code**:
```go
return fmt.Errorf("failed to create resource %s: %w", gvr.Resource, err)
```

**Optimization**: Use `errors.Wrap` or pre-format strings for common errors
**Expected Improvement**: Minimal, but cleaner code

## Performance Testing Recommendations

1. **Benchmark FieldExtractor**: Compare `fmt.Sprintf` vs `strings.Join` vs `sync.Map`
2. **Profile Deduper**: Use `pprof` to identify lock contention hotspots
3. **Load Testing**: Test with 500+ events/sec to identify bottlenecks
4. **Memory Profiling**: Identify allocation hotspots with `go tool pprof`

## Implementation Priority

1. **Immediate** (Do Now):
   - FieldExtractor cache key optimization (#1)
   - Deduper lock granularity (#3)

2. **Short Term** (Next Sprint):
   - String formatting optimization (#2)
   - String lowercasing cache (#5)

3. **Medium Term** (Future):
   - Channel buffer sizing (#7)
   - Logger reuse (#8)

## Metrics to Track

- Field extraction latency (p50, p95, p99)
- Deduper lock contention time
- Memory allocations per observation
- Throughput (observations/sec) under load

