#!/bin/bash
# DEPRECATED: Use scripts/benchmark/stress-test.sh instead.
# This script is kept temporarily for reference but is no longer maintained.
#
# Burst test: Test peak capacity and recovery behavior
#
# Usage: ./burst-test.sh [--burst-size N] [--burst-duration S] [--recovery-time S]
#   --burst-size: Number of observations in burst (default: 500)
#   --burst-duration: Burst duration in seconds (default: 30)
#   --recovery-time: Recovery monitoring time in seconds (default: 60)

set -euo pipefail

NAMESPACE="${NAMESPACE:-zen-system}"
BURST_SIZE="${BURST_SIZE:-500}"
BURST_DURATION="${BURST_DURATION:-30}"
RECOVERY_TIME="${RECOVERY_TIME:-60}"

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --burst-size)
            BURST_SIZE="$2"
            shift 2
            ;;
        --burst-duration)
            BURST_DURATION="$2"
            shift 2
            ;;
        --recovery-time)
            RECOVERY_TIME="$2"
            shift 2
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

echo "=== Burst Test Configuration ==="
echo "Burst size: $BURST_SIZE observations"
echo "Burst duration: ${BURST_DURATION}s (${BURST_DURATION} seconds)"
echo "Recovery monitoring: ${RECOVERY_TIME}s"
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

# Calculate burst rate
BURST_RATE=$(echo "scale=2; $BURST_SIZE / $BURST_DURATION" | bc)
INTERVAL=$(echo "scale=3; $BURST_DURATION / $BURST_SIZE" | bc)

echo "Burst rate: $BURST_RATE obs/sec"
echo "Using pod: $POD"
echo ""

# Get initial metrics
echo "Collecting baseline metrics..."
INIT_CPU=$(kubectl top pod "$POD" -n "$NAMESPACE" --no-headers 2>/dev/null | awk '{print $2}' | sed 's/m//' || echo "0")
INIT_MEM=$(kubectl top pod "$POD" -n "$NAMESPACE" --no-headers 2>/dev/null | awk '{print $3}' | sed 's/Mi//' || echo "0")

echo "Baseline:"
echo "  CPU: ${INIT_CPU}m"
echo "  Memory: ${INIT_MEM}MB"
echo ""

# Burst phase
echo "=== Burst Phase ==="
echo "Creating $BURST_SIZE observations in ${BURST_DURATION}s..."
BURST_START=$(date +%s)

for i in $(seq 1 "$BURST_SIZE"); do
    cat <<EOF | kubectl apply -f - &>/dev/null || true
apiVersion: zen.kube-zen.io/v1
kind: Observation
metadata:
  generateName: burst-test-obs-
  namespace: $NAMESPACE
  labels:
    burst-test: "true"
spec:
  source: burst-test
  category: performance
  severity: LOW
  eventType: burst-test
  detectedAt: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
EOF
    
    # Rate limiting
    sleep "$INTERVAL"
    
    if [ $((i % 100)) -eq 0 ]; then
        echo "  Created $i/$BURST_SIZE observations..."
    fi
done

BURST_END=$(date +%s)
BURST_ACTUAL_DURATION=$((BURST_END - BURST_START))
BURST_ACTUAL_RATE=$(echo "scale=2; $BURST_SIZE / $BURST_ACTUAL_DURATION" | bc)

# Get peak metrics immediately after burst
PEAK_CPU=$(kubectl top pod "$POD" -n "$NAMESPACE" --no-headers 2>/dev/null | awk '{print $2}' | sed 's/m//' || echo "0")
PEAK_MEM=$(kubectl top pod "$POD" -n "$NAMESPACE" --no-headers 2>/dev/null | awk '{print $3}' | sed 's/Mi//' || echo "0")

echo ""
echo "Burst completed at $(date -u +"%Y-%m-%dT%H:%M:%SZ")"
echo "Observations created: $BURST_SIZE (target: $BURST_SIZE)"
echo "Burst duration: ${BURST_ACTUAL_DURATION}s"
echo "Burst rate: $BURST_ACTUAL_RATE obs/sec"
echo ""

echo "=== Peak Resource Usage ==="
CPU_PEAK_DELTA=$((PEAK_CPU - INIT_CPU))
MEM_PEAK_DELTA=$((PEAK_MEM - INIT_MEM))
echo "CPU: ${INIT_CPU}m → ${PEAK_CPU}m (Δ${CPU_PEAK_DELTA}m)"
echo "Memory: ${INIT_MEM}MB → ${PEAK_MEM}MB (Δ${MEM_PEAK_DELTA}MB)"
echo ""

# Recovery phase
echo "=== Recovery Analysis ==="
echo "Monitoring recovery for ${RECOVERY_TIME}s..."
RECOVERY_START=$(date +%s)

# Sample metrics during recovery
for i in {1..6}; do
    sleep $((RECOVERY_TIME / 6))
    
    CURRENT_CPU=$(kubectl top pod "$POD" -n "$NAMESPACE" --no-headers 2>/dev/null | awk '{print $2}' | sed 's/m//' || echo "0")
    CURRENT_MEM=$(kubectl top pod "$POD" -n "$NAMESPACE" --no-headers 2>/dev/null | awk '{print $3}' | sed 's/Mi//' || echo "0")
    
    ELAPSED=$((RECOVERY_TIME * i / 6))
    CPU_RECOVERY=$((PEAK_CPU - CURRENT_CPU))
    MEM_RECOVERY=$((PEAK_MEM - CURRENT_MEM))
    
    echo "  ${ELAPSED}s: CPU ${CURRENT_CPU}m (recovered ${CPU_RECOVERY}m), Memory ${CURRENT_MEM}MB (recovered ${MEM_RECOVERY}MB)"
done

RECOVERY_END=$(date +%s)
FINAL_CPU=$(kubectl top pod "$POD" -n "$NAMESPACE" --no-headers 2>/dev/null | awk '{print $2}' | sed 's/m//' || echo "0")
FINAL_MEM=$(kubectl top pod "$POD" -n "$NAMESPACE" --no-headers 2>/dev/null | awk '{print $3}' | sed 's/Mi//' || echo "0")

CPU_RECOVERY=$((PEAK_CPU - FINAL_CPU))
MEM_RECOVERY=$((PEAK_MEM - FINAL_MEM))
CPU_RECOVERY_PERCENT=$(echo "scale=0; ($CPU_RECOVERY * 100) / $CPU_PEAK_DELTA" | bc)
MEM_RECOVERY_PERCENT=$(echo "scale=0; ($MEM_RECOVERY * 100) / $MEM_PEAK_DELTA" | bc)

echo ""
echo "Final state after recovery:"
echo "  CPU: ${FINAL_CPU}m (${CPU_RECOVERY_PERCENT}% recovery from peak)"
echo "  Memory: ${FINAL_MEM}MB (${MEM_RECOVERY_PERCENT}% recovery from peak)"
echo ""

# Performance assessment
BURST_CAPACITY=$(echo "scale=0; $BURST_ACTUAL_RATE" | bc)
if [ "$BURST_CAPACITY" -ge 50 ]; then
    BURST_STATUS="✅ GOOD"
elif [ "$BURST_CAPACITY" -ge 30 ]; then
    BURST_STATUS="⚠️ ACCEPTABLE"
else
    BURST_STATUS="❌ BELOW TARGET"
fi

if [ "$CPU_RECOVERY_PERCENT" -ge 60 ]; then
    RECOVERY_STATUS="✅ GOOD"
elif [ "$CPU_RECOVERY_PERCENT" -ge 40 ]; then
    RECOVERY_STATUS="⚠️ ACCEPTABLE"
else
    RECOVERY_STATUS="❌ POOR"
fi

echo "=== Performance Assessment ==="
echo "$BURST_STATUS Burst Capacity: ${BURST_ACTUAL_RATE} obs/sec"
echo "$RECOVERY_STATUS CPU Recovery: ${CPU_RECOVERY_PERCENT}%"
echo ""

# Cleanup option
read -p "Delete test observations? (y/N): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "Deleting test observations..."
    kubectl delete observations -n "$NAMESPACE" -l burst-test=true --ignore-not-found=true
    echo "Cleanup complete!"
fi

