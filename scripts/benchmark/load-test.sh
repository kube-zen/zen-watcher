#!/bin/bash
# DEPRECATED: Use scripts/benchmark/stress-test.sh instead.
# This script is kept temporarily for reference but is no longer maintained.
#
# Load test: Sustained performance under consistent load
#
# Usage: ./load-test.sh [--count N] [--duration M] [--rate R]
#   --count: Total observations to create (default: 2000)
#   --duration: Test duration in minutes (default: 2)
#   --rate: Target observations per second (default: 16)

set -euo pipefail

NAMESPACE="${NAMESPACE:-zen-system}"
COUNT="${COUNT:-2000}"
DURATION="${DURATION:-2}"
RATE="${RATE:-16}"

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --count)
            COUNT="$2"
            shift 2
            ;;
        --duration)
            DURATION="$2"
            shift 2
            ;;
        --rate)
            RATE="$2"
            shift 2
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

echo "=== Load Test Configuration ==="
echo "Namespace: $NAMESPACE"
echo "Target observations: $COUNT"
echo "Duration: ${DURATION}m ($((DURATION * 60)) seconds)"
echo "Rate: $RATE obs/sec"
echo "Source: load-test"
echo ""

# Check prerequisites
if ! kubectl get namespace "$NAMESPACE" &>/dev/null; then
    echo "Error: Namespace $NAMESPACE does not exist"
    exit 1
fi

POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/name=zen-watcher -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -z "$POD" ]; then
    echo "Error: zen-watcher pod not found"
    exit 1
fi

echo "Using pod: $POD"
echo ""

# Get initial metrics
echo "Collecting initial metrics..."
INIT_CPU=$(kubectl top pod "$POD" -n "$NAMESPACE" --no-headers 2>/dev/null | awk '{print $2}' | sed 's/m//' || echo "0")
INIT_MEM=$(kubectl top pod "$POD" -n "$NAMESPACE" --no-headers 2>/dev/null | awk '{print $3}' | sed 's/Mi//' || echo "0")
INIT_OBS=$(kubectl get observations -n "$NAMESPACE" --no-headers 2>/dev/null | wc -l || echo "0")

echo "Initial state:"
echo "  CPU: ${INIT_CPU}m"
echo "  Memory: ${INIT_MEM}MB"
echo "  Observations: $INIT_OBS"
echo ""

# Calculate timing
DURATION_SEC=$((DURATION * 60))
INTERVAL=$(echo "scale=3; 1 / $RATE" | bc)
OBS_PER_INTERVAL=$(echo "scale=0; $COUNT / $DURATION_SEC" | bc)

echo "Expected average throughput: $(echo "scale=2; $COUNT / $DURATION_SEC" | bc) obs/sec"
echo "Starting load test..."
echo ""

START_TIME=$(date +%s)
OBS_CREATED=0

# Create observations at target rate
while [ $OBS_CREATED -lt $COUNT ]; do
    CURRENT_TIME=$(date +%s)
    ELAPSED=$((CURRENT_TIME - START_TIME))
    
    if [ $ELAPSED -ge $DURATION_SEC ]; then
        break
    fi
    
    # Create observation
    cat <<EOF | kubectl apply -f - &>/dev/null || true
apiVersion: zen.kube-zen.io/v1
kind: Observation
metadata:
  generateName: load-test-obs-
  namespace: $NAMESPACE
  labels:
    load-test: "true"
spec:
  source: load-test
  category: performance
  severity: LOW
  eventType: load-test
  detectedAt: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
EOF
    
    OBS_CREATED=$((OBS_CREATED + 1))
    
    # Rate limiting
    sleep "$INTERVAL"
    
    # Progress indicator
    if [ $((OBS_CREATED % 100)) -eq 0 ]; then
        echo "  Created $OBS_CREATED/$COUNT observations..."
    fi
done

END_TIME=$(date +%s)
ACTUAL_DURATION=$((END_TIME - START_TIME))

# Wait for processing
echo ""
echo "Waiting for processing to complete..."
sleep 10

# Get final metrics
FINAL_CPU=$(kubectl top pod "$POD" -n "$NAMESPACE" --no-headers 2>/dev/null | awk '{print $2}' | sed 's/m//' || echo "0")
FINAL_MEM=$(kubectl top pod "$POD" -n "$NAMESPACE" --no-headers 2>/dev/null | awk '{print $3}' | sed 's/Mi//' || echo "0")
FINAL_OBS=$(kubectl get observations -n "$NAMESPACE" --no-headers 2>/dev/null | wc -l || echo "0")

# Calculate actual throughput
ACTUAL_THROUGHPUT=$(echo "scale=2; $OBS_CREATED / $ACTUAL_DURATION" | bc)

echo ""
echo "=== Load Test Results ==="
echo "Load test completed at $(date -u +"%Y-%m-%dT%H:%M:%SZ")"
echo "Observations created: $OBS_CREATED (target: $COUNT)"
echo "Actual duration: ${ACTUAL_DURATION}s"
echo "Actual throughput: $ACTUAL_THROUGHPUT obs/sec (target: $RATE obs/sec)"
echo ""

echo "=== Resource Impact ==="
CPU_DELTA=$((FINAL_CPU - INIT_CPU))
MEM_DELTA=$((FINAL_MEM - INIT_MEM))
OBS_DELTA=$((FINAL_OBS - INIT_OBS))
echo "CPU: ${INIT_CPU}m → ${FINAL_CPU}m (Δ${CPU_DELTA}m)"
echo "Memory: ${INIT_MEM}MB → ${FINAL_MEM}MB (Δ${MEM_DELTA}MB)"
echo "Total observations: $INIT_OBS → $FINAL_OBS (Δ$OBS_DELTA)"
echo ""

# Performance assessment
THROUGHPUT_PERCENT=$(echo "scale=0; ($ACTUAL_THROUGHPUT * 100) / $RATE" | bc)
if [ "$THROUGHPUT_PERCENT" -ge 80 ]; then
    STATUS="✅ GOOD"
elif [ "$THROUGHPUT_PERCENT" -ge 60 ]; then
    STATUS="⚠️ ACCEPTABLE"
else
    STATUS="❌ BELOW TARGET"
fi

echo "=== Performance Assessment ==="
echo "$STATUS Performance: achieved ${THROUGHPUT_PERCENT}% of target throughput"
echo ""

# Cleanup option
read -p "Delete test observations? (y/N): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "Deleting test observations..."
    kubectl delete observations -n "$NAMESPACE" -l load-test=true --ignore-not-found=true
    echo "Cleanup complete!"
fi

