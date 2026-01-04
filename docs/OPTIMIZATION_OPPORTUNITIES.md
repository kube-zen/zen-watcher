# Optimization Opportunities

This document identifies optimization opportunities in zen-watcher, prioritized by impact and effort. These are opportunities for future improvements to enhance performance and reduce resource usage.

## Executive Summary

After analyzing the codebase, we've identified **15+ optimization opportunities** across 3 priority levels:
- **High Priority**: 4 opportunities (estimated 20-40% performance improvement)
- **Medium Priority**: 6 opportunities (estimated 10-20% performance improvement)
- **Low Priority**: 5+ opportunities (estimated 2-5% performance improvement)

## High Priority (High Impact, Low Effort)

### 1. Logger Reuse in Hot Paths ⚠️ CRITICAL
**Location**: Multiple files (155 instances total)
- `pkg/processor/pipeline.go`: 2 instances in `applyFilter` and `applyDedup`
- `pkg/orchestrator/generic.go`: 11 instances
- `pkg/watcher/observation_creator.go`: 9 instances
- `pkg/config/ingester_loader.go`: 13 instances

**Issue**: Creating new logger instances in hot paths causes unnecessary allocations
**Impact**: High - called for every event in the pipeline
**Current Code**:
```go
logger := sdklog.NewLogger("zen-watcher-processor")
logger.Debug("Event filtered", ...)
```

**Optimization**: Use package-level logger instances
```go
// Package-level logger
var processorLogger = sdklog.NewLogger("zen-watcher-processor")

// In functions:
processorLogger.Debug("Event filtered", ...)
```

**Expected Improvement**: ~5-10% reduction in allocations, ~2-3% overall performance gain

**Files to Update**:
- `pkg/processor/pipeline.go`
- `pkg/orchestrator/generic.go`
- `pkg/watcher/observation_creator.go` (already has `observationLogger`, but some functions still create new ones)
- `pkg/config/ingester_loader.go`

---

### 2. FieldExtractor Cache Key Generation
**Location**: `pkg/watcher/field_extractor.go`
**Issue**: Using `fmt.Sprintf("%v", path)` for cache keys is inefficient
**Impact**: High - called in hot path for every observation
**Current Code**:
```go
cacheKey := fmt.Sprintf("%v", path)
```

**Optimization**: Use string join or sync.Map
```go
// Option 1: Use strings.Join (simpler, good for small paths)
cacheKey := strings.Join(path, ":")

// Option 2: Use sync.Map with []string key directly (no conversion needed)
// Change fieldPathCache to sync.Map[string][]string
```

**Expected Improvement**: ~10-15% reduction in field extraction overhead

---

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

**Note**: This may already be implemented in zen-sdk package. Verify current implementation.

---

### 4. String Formatting in Hot Paths
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

---

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

---

### 6. Repeated Map Lookups
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

---

### 7. FieldExtractor Cache Optimization
**Location**: `pkg/watcher/field_extractor.go`

**Issue**: Cache lookup pattern could be optimized
**Impact**: Medium - called for every field extraction
**Current Code**:
```go
cacheKey := strings.Join(path, ":")
fe.mu.RLock()
cachedPath, exists := fe.fieldPathCache[cacheKey]
fe.mu.RUnlock()

if !exists {
    fe.mu.Lock()
    fe.fieldPathCache[cacheKey] = path
    fe.mu.Unlock()
    cachedPath = path
}
```

**Optimization**: Use `sync.Map` to avoid lock contention
```go
// Use sync.Map (lock-free reads)
cacheKey := strings.Join(path, ":")
if cachedPath, exists := fe.fieldPathCache.Load(cacheKey); exists {
    return cachedPath.([]string)
}
fe.fieldPathCache.Store(cacheKey, path)
return path
```

**Expected Improvement**: ~5-10% reduction in lock contention for concurrent field extractions

---

### 8. Observation Field Extraction Optimization
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

---

### 9. Channel Buffer Sizing
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

**Note**: This may already be implemented. Verify current implementation.

---

### 10. splitPath String Concatenation in Loop
**Location**: `pkg/monitoring/generic_threshold_monitor.go:204-223`

**Issue**: String concatenation in loop creates multiple allocations
**Impact**: Medium - called during threshold evaluation
**Current Code**:
```go
func splitPath(path string) []string {
    result := make([]string, 0)
    current := ""
    for _, char := range path {
        if char == '.' {
            if current != "" {
                result = append(result, current)
                current = ""
            }
        } else {
            current += string(char)  // String concatenation in loop
        }
    }
    // ...
}
```

**Optimization**: Use `strings.Split`
```go
func splitPath(path string) []string {
    if path == "" {
        return []string{}
    }
    return strings.Split(path, ".")
}
```

**Expected Improvement**: ~5-10% faster path splitting

---

## Low Priority (Low Impact, Low Effort)

### 11. String Concatenation in extractSourceName
**Location**: `pkg/orchestrator/generic.go:415`

**Issue**: String concatenation with `+` operator creates temporary allocations
**Impact**: Low-Medium - called during adapter management
**Current Code**:
```go
expectedPrefix := namespace + "/" + name + "/"
```

**Optimization**: Use `fmt.Sprintf` or `strings.Builder`
```go
expectedPrefix := fmt.Sprintf("%s/%s/", namespace, name)
```

**Expected Improvement**: ~1-2% reduction in allocations during adapter operations

---

### 12. strings.Split in Hot Paths
**Location**: 
- `pkg/orchestrator/generic.go:427` - `parseSourceIdentifier`
- `pkg/orchestrator/generic.go:463` - `updateAllIngesterStatus` (in loop)

**Issue**: `strings.Split` called repeatedly, could be cached or optimized
**Impact**: Low-Medium - called during status updates and source parsing
**Current Code**:
```go
parts := strings.Split(source, "/")
// ... later in loop ...
parts := strings.Split(key, "/")
```

**Optimization**: 
- Cache split results for frequently accessed sources
- Use manual parsing for simple cases (2-3 parts)
- Pre-allocate slice capacity when size is known

**Expected Improvement**: ~2-3% reduction in string operations overhead

---

### 13. Map/Slice Pre-allocation
**Location**: Multiple files (24 map instances, 137 slice instances)

**Issue**: Maps and slices created without capacity hints when size is known
**Impact**: Low - small performance gain from pre-allocating capacity
**Optimization**: Use `make(map[K]V, expectedSize)` and `make([]T, 0, expectedSize)` when size is known
**Expected Improvement**: ~1-2% reduction in reallocations

---

### 14. String Formatting in Report Generation
**Location**: `pkg/advisor/report.go:Format()`

**Issue**: Multiple string concatenations with `fmt.Sprintf`
**Impact**: Low-Medium - only affects report generation (not hot path)
**Current Code**:
```go
report := "\n=== Weekly Optimization Report ===\n\n"
report += fmt.Sprintf("Period: %s to %s\n\n", ...)
report += "Summary:\n"
report += fmt.Sprintf("  Total Optimizations Applied: %d\n", ...)
```

**Optimization**: Use `strings.Builder` for better performance
```go
var b strings.Builder
b.WriteString("\n=== Weekly Optimization Report ===\n\n")
fmt.Fprintf(&b, "Period: %s to %s\n\n", ...)
b.WriteString("Summary:\n")
fmt.Fprintf(&b, "  Total Optimizations Applied: %d\n", ...)
return b.String()
```

**Expected Improvement**: ~10-20% faster report generation

---

### 15. Error Message Formatting
**Location**: Multiple files using `fmt.Errorf`

**Issue**: Using `fmt.Errorf` with string formatting (acceptable, but could be optimized)
**Impact**: Low - only affects error paths
**Optimization**: Pre-format common error messages or use `errors.Wrap`
**Expected Improvement**: Minimal, but cleaner code

---

## Implementation Priority

### Phase 1 (Immediate - High Impact)
1. Logger reuse in hot paths (#1)
2. FieldExtractor cache key optimization (#2)
3. Deduper lock granularity (#3) - Verify if already implemented

### Phase 2 (Short Term - Medium Impact)
4. String formatting optimization (#4)
5. String lowercasing cache (#5)
6. FieldExtractor cache optimization (#7)
7. splitPath optimization (#10)

### Phase 3 (Medium Term - Lower Impact)
8. Channel buffer sizing (#9) - Verify if already implemented
9. String concatenation optimizations (#11, #12)
10. Map/slice pre-allocation (#13)
11. Report generation optimization (#14)

---

## Performance Testing Recommendations

1. **Benchmark Critical Paths**: 
   - `ProcessEvent` pipeline
   - `extractDedupKey` function
   - Field extraction operations

2. **Load Testing**: 
   - Test with 500+ events/sec
   - Measure throughput before/after optimizations

3. **Memory Profiling**: 
   - Use `go tool pprof` to identify allocation hotspots
   - Compare memory usage before/after

4. **Concurrency Testing**: 
   - Test with multiple concurrent event streams
   - Measure lock contention with `go tool pprof -mutex`

---

## Metrics to Track

To measure the impact of optimizations:

1. **Allocation Rate**: `go tool pprof -alloc_objects`
2. **CPU Profile**: `go tool pprof -cpu`
3. **Throughput**: Events/second under load
4. **Latency**: P50, P95, P99 processing times
5. **Memory Usage**: Peak and average memory consumption
6. **String Operations**: Count of `strings.Split`, `strings.Join`, concatenations
7. **GC Pause Time**: Impact of reduced allocations on GC

---

## Notes

- Some optimizations from earlier analysis have already been implemented:
  - ✅ FieldExtractor cache key generation (uses `strings.Join`)
  - ✅ String formatting with type assertions (in `observation_creator.go`)
  - ✅ Channel buffer sizes configurable (in `main.go`)

- The deduper lock granularity optimization may already be implemented in the zen-sdk package.

- Logger reuse has been partially implemented (package-level loggers in some files).

- Profile before optimizing - measure actual impact in production.

- Focus should be on maintaining code quality and readability.

---

## Related Documentation

- [PERFORMANCE.md](PERFORMANCE.md) - Performance benchmarks and tuning
- [OPTIMIZATION_USAGE.md](OPTIMIZATION_USAGE.md) - How to use optimization features
- [OBSERVABILITY.md](OBSERVABILITY.md) - Metrics and monitoring
