# Robustness and Fuzzing

This document outlines the fuzzing strategy for zen-watcher and what kinds of bugs it guards against.

## Overview

zen-watcher uses Go's built-in fuzzing (available since Go 1.18) to test the pipeline and Ingester spec parsing against malformed, random, and extreme inputs.

## What's Fuzzed

### Pipeline Event Processing

**File**: `pkg/processor/pipeline_fuzz_test.go`

**Fuzz targets:**
1. **`FuzzProcessEvent`**: Random event payloads with malformed structures
   - Tests that the pipeline handles arbitrary JSON structures without panicking
   - Ensures filter/dedup/normalize paths behave deterministically

2. **`FuzzProcessEvent_ExtremeSizes`**: Events with extreme payload sizes
   - Tests pipeline behavior with very small (1 byte) to large (100KB) payloads
   - Guards against OOM and excessive memory usage

3. **`FuzzProcessEvent_HighCardinalityLabels`**: Events with high-cardinality label sets
   - Tests pipeline with 1 to 1000 labels per event
   - Guards against performance degradation with many labels

### Ingester Spec Parsing

**File**: `pkg/config/ingester_loader_fuzz_test.go`

**Fuzz targets:**
1. **`FuzzLoadIngesterConfig`**: Randomized/partially-corrupt Ingester specs
   - Tests that parsing errors are handled gracefully
   - Ensures no panics or infinite loops with malformed CRD specs

2. **`FuzzLoadIngesterConfig_MalformedYAML`**: Malformed YAML-like input
   - Tests that malformed YAML is handled gracefully
   - Guards against parser crashes

## Running Fuzz Tests

### Basic Fuzzing

```bash
cd zen-watcher
go test -fuzz=FuzzProcessEvent ./pkg/processor
go test -fuzz=FuzzLoadIngesterConfig ./pkg/config
```

### Fuzzing with Timeout

```bash
# Fuzz for 10 seconds
go test -fuzz=FuzzProcessEvent -fuzztime=10s ./pkg/processor
```

### Fuzzing with Corpus

The fuzz tests include seed corpus (valid examples) to guide fuzzing. The corpus is automatically expanded as fuzzing finds new interesting inputs.

## What Bugs This Guards Against

### Panics

- **Null pointer dereferences**: Fuzzing finds cases where code assumes fields exist
- **Type assertions**: Fuzzing finds cases where code assumes wrong types
- **Index out of bounds**: Fuzzing finds cases where code assumes array/slice sizes

### Infinite Loops

- **Recursive parsing**: Fuzzing finds cases where parsing enters infinite loops
- **Circular references**: Fuzzing finds cases where data structures have cycles

### Memory Issues

- **OOM**: Fuzzing with extreme sizes finds cases where memory usage explodes
- **Memory leaks**: Fuzzing over time can reveal memory leaks

### Performance Degradation

- **High-cardinality labels**: Fuzzing finds cases where many labels cause slowdowns
- **Large payloads**: Fuzzing finds cases where large payloads cause slowdowns

## Interpreting Results

### Crashes

If fuzzing finds a crash:
1. **Save the crash input**: Fuzzing automatically saves crash inputs to `testdata/fuzz/`
2. **Reproduce**: Run the test with the saved input to reproduce
3. **Fix**: Add proper error handling or input validation
4. **Add to corpus**: Add the fixed input as a seed to prevent regression

### Timeouts

If fuzzing times out:
1. **Check for infinite loops**: The code may enter an infinite loop with certain inputs
2. **Check for performance issues**: The code may be too slow with certain inputs
3. **Add timeouts**: Add explicit timeouts to prevent hangs

### Memory Issues

If fuzzing causes OOM:
1. **Limit input size**: Add size limits to fuzz targets
2. **Check for memory leaks**: Use memory profiling to find leaks
3. **Optimize**: Reduce memory usage in hot paths

## Best Practices

1. **Seed corpus**: Always include valid examples in seed corpus
2. **Size limits**: Add size limits to prevent OOM
3. **Timeouts**: Add timeouts to prevent infinite loops
4. **Error handling**: Ensure all errors are handled gracefully
5. **Determinism**: Ensure fuzzed code is deterministic (no random behavior)

## Related Documentation

- [Go Fuzzing Documentation](https://go.dev/doc/fuzz/)
- [PERFORMANCE.md](PERFORMANCE.md) - Performance characteristics and sizing
- [CRD_CONFORMANCE.md](CRD_CONFORMANCE.md) - CRD validation and conformance

