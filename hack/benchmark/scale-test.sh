#!/bin/bash
# Scale test: Create large number of observations and measure impact

set -euo pipefail

NAMESPACE="${NAMESPACE:-zen-system}"
COUNT="${1:-20000}"

echo "=== Scale Test: $COUNT Observations ==="
echo "Namespace: $NAMESPACE"
echo ""

# Check prerequisites
if ! kubectl get namespace "$NAMESPACE" &>/dev/null; then
    echo "Error: Namespace $NAMESPACE does not exist"
    exit 1
fi

# Get initial etcd storage (approximate)
echo "Collecting initial metrics..."
INIT_OBS_COUNT=$(kubectl get observations -n "$NAMESPACE" --no-headers 2>/dev/null | wc -l || echo "0")
echo "Initial observation count: $INIT_OBS_COUNT"

# Create observations in batches
BATCH_SIZE=500
BATCHES=$((COUNT / BATCH_SIZE))

echo "Creating $COUNT observations in $BATCHES batches of $BATCH_SIZE..."
echo "This may take several minutes..."
echo ""

START_TIME=$(date +%s)

for batch in $(seq 1 "$BATCHES"); do
    echo -n "Batch $batch/$BATCHES... "
    
    # Create batch
    for i in $(seq 1 "$BATCH_SIZE"); do
        cat <<EOF | kubectl apply -f - &>/dev/null || true
apiVersion: zen.kube-zen.io/v1
kind: Observation
metadata:
  generateName: scale-test-obs-
  namespace: $NAMESPACE
  labels:
    scale-test: "true"
spec:
  source: scale-test
  category: performance
  severity: LOW
  eventType: scale-test
  detectedAt: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
EOF
    done
    
    echo "done"
    
    # Small delay between batches
    sleep 1
done

END_TIME=$(date +%s)
DURATION_SEC=$((END_TIME - START_TIME))

echo ""
echo "=== Scale Test Results ==="
echo "Observations created: $COUNT"
echo "Duration: ${DURATION_SEC}s"
echo ""

# Final observation count
FINAL_OBS_COUNT=$(kubectl get observations -n "$NAMESPACE" --no-headers 2>/dev/null | wc -l || echo "0")
echo "Total observations in namespace: $FINAL_OBS_COUNT"

# Test list performance
echo ""
echo "Testing list performance..."
LIST_START=$(date +%s.%N)
kubectl get observations -n "$NAMESPACE" --no-headers &>/dev/null || true
LIST_END=$(date +%s.%N)
LIST_DURATION=$(echo "$LIST_END - $LIST_START" | bc)
echo "List duration (no chunking): ${LIST_DURATION}s"

# Test list with chunking
LIST_START=$(date +%s.%N)
kubectl get observations -n "$NAMESPACE" --chunk-size=500 --no-headers &>/dev/null || true
LIST_END=$(date +%s.%N)
LIST_DURATION_CHUNK=$(echo "$LIST_END - $LIST_START" | bc)
echo "List duration (chunk-size=500): ${LIST_DURATION_CHUNK}s"

# Get pod metrics
POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/name=zen-watcher -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -n "$POD" ]; then
    echo ""
    echo "zen-watcher pod metrics:"
    kubectl top pod "$POD" -n "$NAMESPACE" 2>/dev/null || echo "Metrics unavailable"
fi

echo ""
echo "=== Recommendations ==="
echo "- Use --chunk-size=500 for large-scale list operations"
echo "- Monitor etcd storage usage"
echo "- Consider TTL for automatic cleanup"
echo ""

# Cleanup option
read -p "Delete test observations? (y/N): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "Deleting test observations..."
    kubectl delete observations -n "$NAMESPACE" -l scale-test=true --ignore-not-found=true
    echo "Cleanup complete!"
fi

