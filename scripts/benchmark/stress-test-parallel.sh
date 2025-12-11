#!/usr/bin/env bash
# Parallel stress test using kubectl with xargs
# Creates observations as fast as possible in parallel
#
# Usage: ./stress-test-parallel.sh [--rate N] [--duration M] [--workers W]

set -euo pipefail

NAMESPACE="${NAMESPACE:-zen-system}"
KUBECTL_CONTEXT="${KUBECTL_CONTEXT:-}"
RATE="${RATE:-100}"          # target observations per second
DURATION="${DURATION:-60}"   # duration in seconds
WORKERS="${WORKERS:-10}"     # parallel workers

# Build kubectl command with context if provided
KUBECTL_CMD="kubectl"
if [ -n "$KUBECTL_CONTEXT" ]; then
    KUBECTL_CMD="kubectl --context=$KUBECTL_CONTEXT"
fi

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --rate)
            RATE="$2"
            shift 2
            ;;
        --duration)
            DURATION="$2"
            shift 2
            ;;
        --workers)
            WORKERS="$2"
            shift 2
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

echo "=== Parallel Stress Test Configuration ==="
echo "Namespace: $NAMESPACE"
echo "Target rate: $RATE obs/sec"
echo "Duration: ${DURATION}s"
echo "Workers: $WORKERS"
echo ""

# Calculate target observations
TOTAL_OBS=$((RATE * DURATION))
OBS_PER_WORKER=$((TOTAL_OBS / WORKERS))
if [ $OBS_PER_WORKER -eq 0 ]; then
    OBS_PER_WORKER=1
fi

echo "Target: $TOTAL_OBS observations in ${DURATION}s"
echo "Observations per worker: $OBS_PER_WORKER"
echo ""

# Create observation function
create_observation() {
    local worker_id=$1
    local obs_num=$2
    local timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    local name="stress-test-${worker_id}-${obs_num}-$(date +%s%N)"
    
    cat <<EOF
apiVersion: zen.kube-zen.io/v1
kind: Observation
metadata:
  name: ${name}
  namespace: ${NAMESPACE}
  labels:
    stress-test: "true"
    worker: "${worker_id}"
spec:
  source: stress-test
  category: performance
  severity: LOW
  eventType: stress-test
  detectedAt: ${timestamp}
EOF
}

# Function to create observations as fast as possible
worker_func() {
    local worker_id=$1
    local count=0
    local start_time=$(date +%s)
    local end_time=$((start_time + DURATION))
    
    # Create observations as fast as possible until time expires
    while [ $(date +%s) -lt $end_time ]; do
        if create_observation "$worker_id" "$count" | $KUBECTL_CMD apply -f - &>/dev/null; then
            count=$((count + 1))
        fi
        # Small sleep to avoid overwhelming the system
        sleep 0.01 2>/dev/null || true
    done
    
    echo "$count"
}

# Export functions and variables for parallel execution
export -f create_observation worker_func
export NAMESPACE KUBECTL_CMD DURATION KUBECTL_CONTEXT

echo "Starting $WORKERS parallel workers (creating observations as fast as possible)..."
START_TIME=$(date +%s)

# Run workers in parallel using xargs
seq 1 $WORKERS | xargs -P $WORKERS -I {} bash -c 'worker_func {}' > /tmp/stress_results_$$.txt 2>&1

END_TIME=$(date +%s)
ELAPSED=$((END_TIME - START_TIME))

# Calculate results
TOTAL_CREATED=$(awk '{sum+=$1} END {print sum}' /tmp/stress_results_$$.txt 2>/dev/null || echo "0")
if [ "$TOTAL_CREATED" = "" ] || [ "$TOTAL_CREATED" = "0" ]; then
    echo "⚠️  No observations created. Check errors:"
    cat /tmp/stress_results_$$.txt | head -20
    rm -f /tmp/stress_results_$$.txt
    exit 1
fi

ACTUAL_RATE=$(awk "BEGIN {printf \"%.2f\", $TOTAL_CREATED / $ELAPSED}")

echo ""
echo "=== Results ==="
echo "Duration: ${ELAPSED}s"
echo "Created: $TOTAL_CREATED observations"
echo "Actual rate: ${ACTUAL_RATE} obs/sec"
echo "Target rate: $RATE obs/sec"

PERCENT=$(awk "BEGIN {printf \"%.1f\", ($ACTUAL_RATE / $RATE) * 100}")
if (( $(echo "$ACTUAL_RATE < $RATE * 0.9" | bc -l 2>/dev/null || echo "1") )); then
    echo "⚠️  Below target rate (${PERCENT}% of target)"
    echo ""
    echo "Note: Actual rate depends on Kubernetes API server performance."
    echo "To test rate limiting, ensure actual rate exceeds Ingester CRD rateLimit setting."
else
    echo "✅ Met target rate (${PERCENT}% of target)"
fi

# Cleanup
rm -f /tmp/stress_results_$$.txt
