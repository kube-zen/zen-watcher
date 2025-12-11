# Benchmark and Stress Test Scripts

This directory contains various scripts for testing zen-watcher performance and rate limiting.

## Stress Test Scripts

### 1. Parallel Bash Stress Test (Recommended - No Build Required)

**File:** `stress-test-parallel.sh`

A high-performance bash script that creates observations in parallel using `xargs` and `kubectl`.

**Usage:**
```bash
# Basic usage
KUBECTL_CONTEXT=par-dev-eks-1 \
NAMESPACE=zen-system \
./stress-test-parallel.sh \
  --rate 1000 \
  --duration 60 \
  --workers 20

# Options:
#   --rate: Target observations per second (default: 100)
#   --duration: Test duration in seconds (default: 60)
#   --workers: Number of parallel workers (default: 10)
```

**Example:**
```bash
# Test with 5,000 obs/sec for 3 minutes
KUBECTL_CONTEXT=par-dev-eks-1 \
./stress-test-parallel.sh --rate 5000 --duration 180 --workers 50
```

### 2. Go-based Parallel Stress Test (Advanced)

**File:** `stress-test.go`

A high-performance Go program that creates observations in parallel using the Kubernetes client. Requires Go 1.23+ to build.

**Build (using Docker):**
```bash
cd scripts/benchmark
docker run --rm -v "$(pwd):/work" -w /work \
  -v "$HOME/.kube:/root/.kube:ro" \
  kubezen/go:1.23-alpine \
  sh -c "go mod init stress-test && \
         go get k8s.io/client-go@v0.28.15 k8s.io/apimachinery@v0.28.15 && \
         go mod tidy && \
         go build -o stress-test stress-test.go"
```

**Usage:**
```bash
# Basic usage
./stress-test -rate 1000 -duration 60s -workers 20

# With kubeconfig and context
./stress-test \
  -kubeconfig ~/.kube/config \
  -context par-dev-eks-1 \
  -namespace zen-system \
  -rate 5000 \
  -duration 5m \
  -workers 50 \
  -burst 100
```

### 3. k6-based HTTP Stress Test

**File:** `stress-test-k6.js`

Uses k6 to test HTTP endpoints (if zen-watcher exposes an API for creating observations).

**Prerequisites:**
```bash
# Install k6
# macOS: brew install k6
# Linux: https://k6.io/docs/getting-started/installation/
```

**Usage:**
```bash
# Basic usage
k6 run --vus 10 --duration 60s stress-test-k6.js

# With environment variables
NAMESPACE=zen-system \
BASE_URL=http://zen-watcher:8080 \
RATE=1000 \
DURATION=60s \
k6 run --vus 100 --duration 60s stress-test-k6.js
```

**Note:** This requires zen-watcher to expose an HTTP API endpoint for creating observations. Currently, zen-watcher uses informers to watch for Observation CRDs, so this script may not be applicable unless an HTTP API is added.

### 4. Bash-based Sequential Stress Test

**File:** `stress-test.sh`

The original bash script that creates observations sequentially using `kubectl apply`. This is slower but useful for basic testing.

**Usage:**
```bash
KUBECTL_CONTEXT=par-dev-eks-1 \
NAMESPACE=zen-system \
./stress-test.sh \
  --phases 2 \
  --phase-duration 3 \
  --max-observations 1000
```

## Performance Comparison

| Method | Max Rate | Latency | Use Case |
|--------|----------|---------|----------|
| Parallel bash | 5,000+ obs/sec | Low | Fast testing without build |
| Go stress-test | 10,000+ obs/sec | Low | High-performance testing |
| k6 | Depends on API | Medium | HTTP endpoint testing |
| Sequential bash | ~1 obs/sec | High | Basic validation |

## Testing Rate Limits

To test rate limiting configured via Ingester CRD:

1. **Configure Ingester CRD:**
```yaml
apiVersion: zen.kube-zen.io/v1alpha1
kind: Ingester
metadata:
  name: stress-test-ingester
  namespace: zen-system
spec:
  source: stress-test
  rateLimit:
    enabled: true
    requestsPerSecond: 10000
    burst: 20000
```

2. **Run stress test:**
```bash
./stress-test -rate 15000 -duration 5m -workers 100
```

3. **Monitor logs:**
```bash
kubectl logs -n zen-system -l app.kubernetes.io/name=zen-watcher | grep rate
```

The rate limiter should throttle requests above the configured limit.

## Troubleshooting

- **Low throughput:** Increase `-workers` and `-burst` parameters
- **Connection errors:** Check kubeconfig and context settings
- **Rate limiting not working:** Verify Ingester CRD is configured correctly and zen-watcher has reloaded it
