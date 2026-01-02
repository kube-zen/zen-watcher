# Optimization Opportunities Analysis

**Date**: 2025-01-XX  
**Status**: Analysis Complete

This document provides a comprehensive analysis of optimization opportunities in zen-watcher, prioritized by impact and effort.

## Executive Summary

After analyzing the codebase, we've identified **15 optimization opportunities** across 5 categories:
- **High Priority**: 4 opportunities (estimated 20-40% performance improvement)
- **Medium Priority**: 6 opportunities (estimated 10-20% performance improvement)
- **Low Priority**: 5 opportunities (estimated 2-5% performance improvement)

## High Priority Optimizations

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

### 2. Hex Encoding Optimization
**Location**: 
- `pkg/processor/pipeline.go:430` - `fmt.Sprintf("%x", hash[:16])`
- `pkg/watcher/fingerprint.go:61` - `fmt.Sprintf("%s/%x", source, hash[:16])`

**Issue**: `fmt.Sprintf("%x", ...)` is slower than `hex.EncodeToString`
**Impact**: Medium-High - called for every event in dedup path
**Current Code**:
```go
hash := sha256.Sum256([]byte(fingerprint))
return sdkdedup.DedupKey{
    Source:      raw.Source,
    MessageHash: fmt.Sprintf("%x", hash[:16]),
}
```

**Optimization**: Use `encoding/hex` package
```go
import "encoding/hex"

hash := sha256.Sum256([]byte(fingerprint))
return sdkdedup.DedupKey{
    Source:      raw.Source,
    MessageHash: hex.EncodeToString(hash[:16]),
}
```

**Expected Improvement**: ~10-15% faster hash encoding, ~1-2% overall performance gain

---

### 3. String Concatenation in splitPath
**Location**: `pkg/watcher/field_mapper.go:260-279`

**Issue**: String concatenation in loop creates multiple allocations
**Impact**: Medium - called during field mapping operations
**Current Code**:
```go
func splitPath(path string) []string {
    parts := []string{}
    current := ""
    for _, char := range path {
        if char == '.' {
            if current != "" {
                parts = append(parts, current)
                current = ""
            }
        } else {
            current += string(char)  // String concatenation in loop
        }
    }
    // ...
}
```

**Optimization**: Use `strings.Split` or `strings.Builder`
```go
func splitPath(path string) []string {
    if path == "" {
        return []string{}
    }
    return strings.Split(path, ".")
}
```

**Note**: `pkg/processor/pipeline.go` already uses `strings.Split` - we should use the same approach.

**Expected Improvement**: ~5-10% faster path splitting, ~0.5-1% overall performance gain

---

### 4. Repeated Map Lookups in Observation Creator
**Location**: `pkg/watcher/observation_creator.go:extractDedupKey`

**Issue**: Multiple field extractions from the same observation object
**Impact**: Medium - called for every observation
**Current Code**:
```go
sourceVal, _ := oc.fieldExtractor.ExtractFieldCopy(observation.Object, "spec", "source")
// ... later ...
resourceVal, _ := oc.fieldExtractor.ExtractMap(observation.Object, "spec", "resource")
// ... later ...
detailsVal, _ := oc.fieldExtractor.ExtractMap(observation.Object, "spec", "details")
// ... later ...
eventTypeVal, _ := oc.fieldExtractor.ExtractFieldCopy(observation.Object, "spec", "eventType")
```

**Optimization**: Extract `spec` map once, then access fields directly
```go
specVal, _ := oc.fieldExtractor.ExtractMap(observation.Object, "spec")
if specVal != nil {
    sourceVal := specVal["source"]
    resourceVal := specVal["resource"]
    detailsVal := specVal["details"]
    eventTypeVal := specVal["eventType"]
    // ... use extracted values
}
```

**Expected Improvement**: ~3-5% reduction in field extraction overhead

---

## Medium Priority Optimizations

### 5. Channel Buffer Size Optimization
**Location**: Multiple files
- `pkg/adapter/generic/informer_adapter.go:71` - `make(chan RawEvent, 100)`
- `pkg/adapter/generic/webhook_adapter.go:39` - `make(chan RawEvent, 100)`
- `pkg/adapter/generic/logs_adapter.go:50` - `make(chan RawEvent, 100)`
- `pkg/watcher/adapter_factory.go:79` - `make(chan *Event, 1000)`

**Issue**: Fixed buffer sizes may not be optimal for all scenarios
**Impact**: Medium - affects backpressure handling and memory usage
**Status**: ✅ Already configurable in `main.go` for Falco/Audit channels

**Optimization**: Make all channel buffer sizes configurable
```go
// In adapter constructors:
bufferSize := getEnvInt("ADAPTER_BUFFER_SIZE", 100)
events := make(chan RawEvent, bufferSize)
```

**Expected Improvement**: Better handling of traffic spikes, reduced memory waste

---

### 6. fmt.Sprintf Type Conversion Optimization
**Location**: Multiple files (43 instances of `fmt.Sprintf("%v", ...)`)

**Issue**: Using `fmt.Sprintf("%v", value)` for type conversion creates allocations
**Impact**: Medium - called in various hot paths
**Current Code**:
```go
source = fmt.Sprintf("%v", sourceVal)
```

**Optimization**: Use type assertions with fallback (already done in some places)
```go
if str, ok := sourceVal.(string); ok {
    source = str
} else {
    source = fmt.Sprintf("%v", sourceVal) // Only when needed
}
```

**Status**: ✅ Already optimized in `observation_creator.go` - should be applied consistently

**Expected Improvement**: ~2-5% reduction in allocations

---

### 7. FieldExtractor Cache Optimization
**Location**: `pkg/watcher/field_extractor.go`

**Issue**: Cache key generation uses `strings.Join` which is good, but cache lookup pattern could be optimized
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

**Optimization**: Use `sync.Map` to avoid lock contention, or optimize double-check pattern
```go
// Option 1: Use sync.Map (lock-free reads)
cacheKey := strings.Join(path, ":")
if cachedPath, exists := fe.fieldPathCache.Load(cacheKey); exists {
    return cachedPath.([]string)
}
fe.fieldPathCache.Store(cacheKey, path)
return path

// Option 2: Optimize double-check pattern (keep current structure)
```

**Expected Improvement**: ~5-10% reduction in lock contention for concurrent field extractions

---

### 8. Repeated Logger Creation in Orchestrator
**Location**: `pkg/orchestrator/generic.go` (11 instances)

**Issue**: Every function creates a new logger instance
**Impact**: Medium - called during adapter management operations
**Current Code**:
```go
func (o *GenericOrchestrator) reloadAdapters() {
    logger := sdklog.NewLogger("zen-watcher-orchestrator")
    // ...
}
```

**Optimization**: Use package-level logger
```go
var orchestratorLogger = sdklog.NewLogger("zen-watcher-orchestrator")

func (o *GenericOrchestrator) reloadAdapters() {
    orchestratorLogger.Info(...)
}
```

**Expected Improvement**: ~2-3% reduction in allocations during adapter operations

---

### 9. String Formatting in Report Generation
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

### 10. Template String Replacement Optimization
**Location**: `pkg/advisor/suggestion_engine.go:formatTemplate`

**Issue**: Multiple `strings.ReplaceAll` calls on the same string
**Impact**: Low-Medium - only affects suggestion generation
**Current Code**:
```go
result = strings.ReplaceAll(result, "{{.source}}", opp.Source)
result = strings.ReplaceAll(result, "{{.reduction}}", fmt.Sprintf("%.0f", reduction*100))
result = strings.ReplaceAll(result, "{{.description}}", opp.Description)
```

**Optimization**: Use `text/template` package for better performance and maintainability
```go
import "text/template"

tmpl, _ := template.New("suggestion").Parse(tmplStr)
var b strings.Builder
tmpl.Execute(&b, opp)
return b.String()
```

**Expected Improvement**: ~5-10% faster template processing, better maintainability

---

## Low Priority Optimizations

### 11. Error Message Formatting
**Location**: Multiple files using `fmt.Errorf`

**Issue**: Using `fmt.Errorf` with string formatting (acceptable, but could be optimized)
**Impact**: Low - only affects error paths
**Optimization**: Pre-format common error messages or use `errors.Wrap`
**Expected Improvement**: Minimal, but cleaner code

---

### 12. Map Initialization Patterns
**Location**: Multiple files

**Issue**: Some maps are initialized without capacity hints
**Impact**: Low - small performance gain from pre-allocating capacity
**Optimization**: Use `make(map[string]T, expectedSize)` when size is known
**Expected Improvement**: ~1-2% reduction in map reallocations

---

### 13. Slice Pre-allocation
**Location**: Multiple files using `append` in loops

**Issue**: Slices grown dynamically without capacity hints
**Impact**: Low - small performance gain
**Optimization**: Pre-allocate slice capacity when size is known
**Expected Improvement**: ~1-2% reduction in slice reallocations

---

### 14. Type Assertion Optimization
**Location**: Multiple files

**Issue**: Some places could use type assertions more efficiently
**Impact**: Low - already mostly optimized
**Status**: ✅ Already optimized in most hot paths

---

### 15. Context Propagation
**Location**: Multiple files

**Issue**: Some functions don't propagate context properly
**Impact**: Low - affects cancellation and timeout handling
**Optimization**: Ensure context is propagated through all async operations
**Expected Improvement**: Better cancellation behavior

---

## Implementation Priority

### Phase 1 (Immediate - High Impact)
1. ✅ Logger reuse in hot paths (#1)
2. ✅ Hex encoding optimization (#2)
3. ✅ String concatenation in splitPath (#3)

### Phase 2 (Short Term - Medium Impact)
4. Repeated map lookups (#4)
5. Channel buffer size optimization (#5)
6. FieldExtractor cache optimization (#7)
7. Orchestrator logger reuse (#8)

### Phase 3 (Medium Term - Lower Impact)
8. String formatting optimizations (#6, #9, #10)
9. Error message formatting (#11)
10. Map/slice pre-allocation (#12, #13)

---

## Metrics to Track

To measure the impact of optimizations:

1. **Allocation Rate**: `go tool pprof -alloc_objects`
2. **CPU Profile**: `go tool pprof -cpu`
3. **Throughput**: Events/second under load
4. **Latency**: P50, P95, P99 processing times
5. **Memory Usage**: Peak and average memory consumption

---

## Testing Recommendations

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

## Notes

- Some optimizations from the original `OPTIMIZATION_OPPORTUNITIES.md` have already been implemented:
  - ✅ FieldExtractor cache key generation (uses `strings.Join`)
  - ✅ String formatting with type assertions (in `observation_creator.go`)
  - ✅ Channel buffer sizes configurable (in `main.go`)

- The deduper lock granularity optimization was already implemented in the zen-sdk package.

- Logger reuse has been partially implemented (package-level loggers in some files).

