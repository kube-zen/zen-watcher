# Performance Benchmark Scripts

These scripts help measure zen-watcher performance for SIG reviews and optimization.

## Quick Benchmark

Create 100 observations and measure throughput, CPU, and memory:

```bash
./scripts/benchmark/quick-bench.sh
```

**Options:**
- `NAMESPACE=zen-system` - Target namespace (default: zen-system)
- `COUNT=100` - Number of observations to create (default: 100)

## Scale Test

Create large number of observations (default: 20,000) and measure impact:

```bash
./scripts/benchmark/scale-test.sh 20000
```

**Measures:**
- etcd storage impact
- List operation performance
- Pod resource usage

## Load Test

Create observations at sustained rate:

```bash
# Create 1000 observations over 60 seconds
./scripts/benchmark/load-test.sh --count 1000 --duration 60s
```

## Profiling

Collect CPU and memory profiles:

```bash
# Port-forward metrics endpoint
kubectl port-forward -n zen-system deployment/zen-watcher 9090:9090

# Collect CPU profile (30 seconds)
curl http://localhost:9090/debug/pprof/profile?seconds=30 > cpu.prof
go tool pprof cpu.prof

# Collect memory profile
curl http://localhost:9090/debug/pprof/heap > heap.prof
go tool pprof heap.prof
```

## Requirements

- `kubectl` configured and connected to cluster
- `bc` for calculations (install: `apt-get install bc` or `brew install bc`)
- `jq` for JSON parsing (optional)

## See Also

- [Performance Documentation](../../docs/PERFORMANCE.md) - Complete performance benchmarks
- [KEP Performance Section](../../keps/sig-foo/0000-zen-watcher/README.md#performance-characteristics) - KEP performance data

