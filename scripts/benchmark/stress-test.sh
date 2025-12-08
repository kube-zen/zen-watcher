#!/bin/bash
# Stress test: Multi-phase testing with progressive load increase
#
# Usage: ./stress-test.sh [--phases N] [--phase-duration M] [--max-observations N]
#   --phases: Number of phases (default: 3)
#   --phase-duration: Duration per phase in minutes (default: 10)
#   --max-observations: Maximum total observations (default: 5000)

set -euo pipefail

NAMESPACE="${NAMESPACE:-zen-system}"
PHASES="${PHASES:-3}"
PHASE_DURATION="${PHASE_DURATION:-10}"
MAX_OBSERVATIONS="${MAX_OBSERVATIONS:-5000}"

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --phases)
            PHASES="$2"
            shift 2
            ;;
        --phase-duration)
            PHASE_DURATION="$2"
            shift 2
            ;;
        --max-observations)
            MAX_OBSERVATIONS="$2"
            shift 2
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

echo "=== Stress Test Configuration ==="
echo "Phases: $PHASES"
echo "Phase duration: ${PHASE_DURATION}m ($((PHASE_DURATION * 60)) seconds each)"
echo "Max observations: $MAX_OBSERVATIONS"
echo "Sample interval: 30s"
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
echo "Collecting baseline metrics..."
INIT_CPU=$(kubectl top pod "$POD" -n "$NAMESPACE" --no-headers 2>/dev/null | awk '{print $2}' | sed 's/m//' || echo "0")
INIT_MEM=$(kubectl top pod "$POD" -n "$NAMESPACE" --no-headers 2>/dev/null | awk '{print $3}' | sed 's/Mi//' || echo "0")
INIT_OBS=$(kubectl get observations -n "$NAMESPACE" --no-headers 2>/dev/null | wc -l || echo "0")

echo "Baseline:"
echo "  CPU: ${INIT_CPU}m"
echo "  Memory: ${INIT_MEM}MB"
echo "  Observations: $INIT_OBS"
echo ""

PHASE_DURATION_SEC=$((PHASE_DURATION * 60))
OBS_PER_PHASE=$((MAX_OBSERVATIONS / PHASES))

# Phase-by-phase results
declare -a PHASE_RATES
declare -a PHASE_CPU
declare -a PHASE_MEM

# Run phases
for phase in $(seq 1 "$PHASES"); do
    # Calculate rate for this phase (progressive increase)
    RATE=$(echo "scale=0; 10 + ($phase * 10)" | bc)
    INTERVAL=$(echo "scale=3; 1 / $RATE" | bc)
    
    echo "=== Phase $phase/$PHASES ==="
    echo "Target rate: $RATE obs/sec"
    echo "Target observations: $OBS_PER_PHASE"
    echo ""
    
    PHASE_START=$(date +%s)
    OBS_CREATED=0
    
    # Create observations for this phase
    while [ $OBS_CREATED -lt $OBS_PER_PHASE ]; do
        CURRENT_TIME=$(date +%s)
        ELAPSED=$((CURRENT_TIME - PHASE_START))
        
        if [ $ELAPSED -ge $PHASE_DURATION_SEC ]; then
            break
        fi
        
        cat <<EOF | kubectl apply -f - &>/dev/null || true
apiVersion: zen.kube-zen.io/v1
kind: Observation
metadata:
  generateName: stress-test-obs-
  namespace: $NAMESPACE
  labels:
    stress-test: "true"
    phase: "$phase"
spec:
  source: stress-test
  category: performance
  severity: LOW
  eventType: stress-test
  detectedAt: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
EOF
        
        OBS_CREATED=$((OBS_CREATED + 1))
        sleep "$INTERVAL"
        
        if [ $((OBS_CREATED % 200)) -eq 0 ]; then
            echo "  Created $OBS_CREATED/$OBS_PER_PHASE observations..."
        fi
    done
    
    PHASE_END=$(date +%s)
    PHASE_ACTUAL_DURATION=$((PHASE_END - PHASE_START))
    PHASE_ACTUAL_RATE=$(echo "scale=2; $OBS_CREATED / $PHASE_ACTUAL_DURATION" | bc)
    
    # Sample metrics during phase
    PHASE_CPU_SAMPLE=$(kubectl top pod "$POD" -n "$NAMESPACE" --no-headers 2>/dev/null | awk '{print $2}' | sed 's/m//' || echo "0")
    PHASE_MEM_SAMPLE=$(kubectl top pod "$POD" -n "$NAMESPACE" --no-headers 2>/dev/null | awk '{print $3}' | sed 's/Mi//' || echo "0")
    
    PHASE_RATES[$phase]=$PHASE_ACTUAL_RATE
    PHASE_CPU[$phase]=$PHASE_CPU_SAMPLE
    PHASE_MEM[$phase]=$PHASE_MEM_SAMPLE
    
    echo ""
    echo "Phase $phase results:"
    echo "  Observations: $OBS_CREATED"
    echo "  Duration: ${PHASE_ACTUAL_DURATION}s"
    echo "  Rate: $PHASE_ACTUAL_RATE obs/sec"
    echo "  CPU: ${PHASE_CPU_SAMPLE}m"
    echo "  Memory: ${PHASE_MEM_SAMPLE}MB"
    echo ""
    
    # Brief pause between phases
    if [ $phase -lt $PHASES ]; then
        echo "Pausing 10s before next phase..."
        sleep 10
    fi
done

# Final metrics
FINAL_CPU=$(kubectl top pod "$POD" -n "$NAMESPACE" --no-headers 2>/dev/null | awk '{print $2}' | sed 's/m//' || echo "0")
FINAL_MEM=$(kubectl top pod "$POD" -n "$NAMESPACE" --no-headers 2>/dev/null | awk '{print $3}' | sed 's/Mi//' || echo "0")
FINAL_OBS=$(kubectl get observations -n "$NAMESPACE" --no-headers 2>/dev/null | wc -l || echo "0")

# Calculate statistics
TOTAL_OBS=$((FINAL_OBS - INIT_OBS))
CPU_DELTA=$((FINAL_CPU - INIT_CPU))
MEM_DELTA=$((FINAL_MEM - INIT_MEM))

# Calculate average and peak rates
TOTAL_RATE=0
PEAK_RATE=0
MIN_RATE=999
for rate in "${PHASE_RATES[@]}"; do
    TOTAL_RATE=$(echo "scale=2; $TOTAL_RATE + $rate" | bc)
    if (( $(echo "$rate > $PEAK_RATE" | bc -l) )); then
        PEAK_RATE=$rate
    fi
    if (( $(echo "$rate < $MIN_RATE" | bc -l) )); then
        MIN_RATE=$rate
    fi
done
AVG_RATE=$(echo "scale=2; $TOTAL_RATE / $PHASES" | bc)

echo "=== Final Resource Impact ==="
echo "CPU: ${INIT_CPU}m → ${FINAL_CPU}m (Δ${CPU_DELTA}m)"
echo "Memory: ${INIT_MEM}MB → ${FINAL_MEM}MB (Δ${MEM_DELTA}MB)"
echo "Total observations: $INIT_OBS → $FINAL_OBS (Δ$TOTAL_OBS)"
echo ""

echo "=== Performance Analysis ==="
echo "Overall throughput statistics:"
echo "  Average rate: $AVG_RATE obs/sec"
echo "  Peak rate: $PEAK_RATE obs/sec"
echo "  Minimum rate: $MIN_RATE obs/sec"
echo ""

echo "=== Resource Analysis ==="
if [ $CPU_DELTA -le 100 ]; then
    CPU_STATUS="✅ LOW-MODERATE"
else
    CPU_STATUS="⚠️ HIGH"
fi

if [ $MEM_DELTA -le 100 ]; then
    MEM_STATUS="✅ LOW-MODERATE"
else
    MEM_STATUS="⚠️ HIGH"
fi

echo "$CPU_STATUS CPU Impact: Δ${CPU_DELTA}m (≤100m increase)"
echo "$MEM_STATUS Memory Impact: Δ${MEM_DELTA}MB (≤100MB increase)"
echo ""

# Performance rating
AVG_RATE_INT=$(echo "scale=0; $AVG_RATE" | bc)
if [ "$AVG_RATE_INT" -ge 20 ]; then
    PERF_STATUS="✅ GOOD"
elif [ "$AVG_RATE_INT" -ge 15 ]; then
    PERF_STATUS="⚠️ ACCEPTABLE"
else
    PERF_STATUS="❌ BELOW TARGET"
fi

echo "=== Performance Rating ==="
echo "$PERF_STATUS Overall Rating: avg ${AVG_RATE} obs/sec"
echo ""

# Cleanup option
read -p "Delete test observations? (y/N): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "Deleting test observations..."
    kubectl delete observations -n "$NAMESPACE" -l stress-test=true --ignore-not-found=true
    echo "Cleanup complete!"
fi

