#!/bin/bash
# Quick benchmark: Create 100 observations and measure performance

set -euo pipefail

NAMESPACE="${NAMESPACE:-zen-system}"
COUNT="${COUNT:-100}"
DURATION="${DURATION:-30}"

echo "=== Quick Benchmark: $COUNT Observations ==="
echo "Namespace: $NAMESPACE"
echo ""

# Check if namespace exists
if ! kubectl get namespace "$NAMESPACE" &>/dev/null; then
    echo "Error: Namespace $NAMESPACE does not exist"
    exit 1
fi

# Get zen-watcher pod
POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/name=zen-watcher -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -z "$POD" ]; then
    echo "Error: zen-watcher pod not found in namespace $NAMESPACE"
    exit 1
fi

echo "Using pod: $POD"
echo ""

# Record start time
START_TIME=$(date +%s)

# Get initial metrics
echo "Collecting initial metrics..."
INIT_CPU=$(kubectl top pod "$POD" -n "$NAMESPACE" --no-headers 2>/dev/null | awk '{print $2}' | sed 's/m//' || echo "0")
INIT_MEM=$(kubectl top pod "$POD" -n "$NAMESPACE" --no-headers 2>/dev/null | awk '{print $3}' | sed 's/Mi//' || echo "0")

# Create observations via script
echo "Creating $COUNT observations..."
for i in $(seq 1 "$COUNT"); do
    cat <<EOF | kubectl apply -f - &>/dev/null || true
apiVersion: zen.kube-zen.io/v1
kind: Observation
metadata:
  generateName: benchmark-obs-
  namespace: $NAMESPACE
spec:
  source: benchmark
  category: performance
  severity: LOW
  eventType: benchmark-test
  detectedAt: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
EOF
done

# Wait for processing
echo "Waiting for processing..."
sleep 5

# Record end time
END_TIME=$(date +%s)
DURATION_SEC=$((END_TIME - START_TIME))

# Get final metrics
FINAL_CPU=$(kubectl top pod "$POD" -n "$NAMESPACE" --no-headers 2>/dev/null | awk '{print $2}' | sed 's/m//' || echo "0")
FINAL_MEM=$(kubectl top pod "$POD" -n "$NAMESPACE" --no-headers 2>/dev/null | awk '{print $3}' | sed 's/Mi//' || echo "0")

# Count created observations
OBS_COUNT=$(kubectl get observations -n "$NAMESPACE" -l source=benchmark --no-headers 2>/dev/null | wc -l || echo "0")

# Calculate throughput
THROUGHPUT=$(echo "scale=2; $COUNT / $DURATION_SEC" | bc 2>/dev/null || echo "N/A")

echo ""
echo "=== Results ==="
echo "Observations created: $OBS_COUNT (expected: $COUNT)"
echo "Duration: ${DURATION_SEC}s"
echo "Throughput: $THROUGHPUT obs/sec"
echo "CPU: ${INIT_CPU}m → ${FINAL_CPU}m"
echo "Memory: ${INIT_MEM}MB → ${FINAL_MEM}MB"
echo ""

# Cleanup
echo "Cleaning up test observations..."
kubectl delete observations -n "$NAMESPACE" -l source=benchmark --ignore-not-found=true &>/dev/null || true

echo "Benchmark complete!"

