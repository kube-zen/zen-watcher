#!/bin/bash
# Quick benchmark: Create 100 observations and measure performance
#
# ⚠️  USAGE WARNING:
#   This script is intended for dedicated or beefier environments.
#   For lightweight local dev runs, use minimal parameters or set ZEN_STRESS_ALLOW_LOCAL=1
#
# Usage: ./quick-bench.sh [--count N] [--namespace NAMESPACE] [--i-know-this-is-heavy]
#
# Environment Variables:
#   COUNT: Number of observations to create (default: 100, reduced to 10 if ZEN_STRESS_ALLOW_LOCAL not set)
#   NAMESPACE: Target namespace (default: zen-system)
#   ZEN_STRESS_ALLOW_LOCAL: Set to 1 to allow heavier runs on local machines (default: unset)
#
# Flags:
#   --i-know-this-is-heavy: Explicitly allow heavier runs (equivalent to ZEN_STRESS_ALLOW_LOCAL=1)

set -euo pipefail

# Source common utilities
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../utils/common.sh" 2>/dev/null || true

NAMESPACE="${NAMESPACE:-zen-system}"
ALLOW_HEAVY=false
DEFAULT_COUNT=100

# Parse arguments
for arg in "$@"; do
    case "$arg" in
        --i-know-this-is-heavy)
            ALLOW_HEAVY=true
            ;;
        --count=*)
            DEFAULT_COUNT="${arg#*=}"
            ;;
        --count)
            shift
            DEFAULT_COUNT="$1"
            ;;
        --namespace=*)
            NAMESPACE="${arg#*=}"
            ;;
        --namespace)
            shift
            NAMESPACE="$1"
            ;;
    esac
done

# Check if heavy runs are allowed
if [ "${ZEN_STRESS_ALLOW_LOCAL:-0}" = "1" ] || [ "$ALLOW_HEAVY" = true ]; then
    COUNT="${COUNT:-${DEFAULT_COUNT}}"
else
    # Default to safe, minimal run
    COUNT="${COUNT:-10}"
    if [ "${COUNT:-10}" -gt 50 ]; then
        log_warn "Heavy run detected (count=$COUNT). For safety, limiting to 50 observations."
        log_info "To allow heavier runs, set ZEN_STRESS_ALLOW_LOCAL=1 or use --i-know-this-is-heavy"
        COUNT=50
    fi
fi

log_step "Quick Benchmark: $COUNT Observations"
log_info "Namespace: $NAMESPACE"
echo ""

# Check if namespace exists
if ! kubectl get namespace "$NAMESPACE" &>/dev/null; then
    log_error "Namespace $NAMESPACE does not exist"
    exit 1
fi

# Get zen-watcher pod
POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/name=zen-watcher -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -z "$POD" ]; then
    log_error "zen-watcher pod not found in namespace $NAMESPACE"
    exit 1
fi

log_info "Using pod: $POD"
echo ""

# Check if metrics-server is available
METRICS_AVAILABLE=false
if kubectl top pod "$POD" -n "$NAMESPACE" &>/dev/null 2>&1; then
    METRICS_AVAILABLE=true
else
    log_warn "metrics-server not available - CPU/memory sampling will be skipped"
fi

# Record start time
START_TIME=$(date +%s)

# Get initial metrics
if [ "$METRICS_AVAILABLE" = true ]; then
    log_info "Collecting initial metrics..."
    INIT_CPU=$(kubectl top pod "$POD" -n "$NAMESPACE" --no-headers 2>/dev/null | awk '{print $2}' | sed 's/m//' || echo "0")
    INIT_MEM=$(kubectl top pod "$POD" -n "$NAMESPACE" --no-headers 2>/dev/null | awk '{print $3}' | sed 's/Mi//' || echo "0")
else
    INIT_CPU="N/A"
    INIT_MEM="N/A"
fi

# Create observations via script
log_step "Creating $COUNT observations..."
for i in $(seq 1 "$COUNT"); do
    cat <<EOF | kubectl apply -f - &>/dev/null || true
apiVersion: zen.kube-zen.io/v1
kind: Observation
metadata:
  generateName: benchmark-obs-
  namespace: $NAMESPACE
  labels:
    source: benchmark
    benchmark: "true"
spec:
  source: benchmark
  category: performance
  severity: LOW
  eventType: benchmark-test
  detectedAt: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
EOF
done

# Wait for processing
log_info "Waiting for processing..."
sleep 5

# Record end time
END_TIME=$(date +%s)
DURATION_SEC=$((END_TIME - START_TIME))

# Get final metrics
if [ "$METRICS_AVAILABLE" = true ]; then
    FINAL_CPU=$(kubectl top pod "$POD" -n "$NAMESPACE" --no-headers 2>/dev/null | awk '{print $2}' | sed 's/m//' || echo "0")
    FINAL_MEM=$(kubectl top pod "$POD" -n "$NAMESPACE" --no-headers 2>/dev/null | awk '{print $3}' | sed 's/Mi//' || echo "0")
else
    FINAL_CPU="N/A"
    FINAL_MEM="N/A"
fi

# Count created observations
OBS_COUNT=$(kubectl get observations -n "$NAMESPACE" -l benchmark=true --no-headers 2>/dev/null | wc -l || echo "0")

# Calculate throughput (use awk if bc not available)
if command -v bc &>/dev/null; then
    THROUGHPUT=$(echo "scale=2; $COUNT / $DURATION_SEC" | bc 2>/dev/null || echo "N/A")
else
    THROUGHPUT=$(awk "BEGIN {printf \"%.2f\", $COUNT / $DURATION_SEC}")
fi

echo ""
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}  Benchmark Results${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo "Observations created: $OBS_COUNT (expected: $COUNT)"
echo "Duration: ${DURATION_SEC}s"
echo "Throughput: $THROUGHPUT obs/sec"
if [ "$METRICS_AVAILABLE" = true ]; then
    echo "CPU: ${INIT_CPU}m → ${FINAL_CPU}m"
    echo "Memory: ${INIT_MEM}MB → ${FINAL_MEM}MB"
else
    echo "CPU/Memory: N/A (metrics-server not available)"
fi
echo ""

# Cleanup
log_step "Cleaning up test observations..."
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [ -f "${SCRIPT_DIR}/../cleanup/fast-observation-cleanup.sh" ]; then
    "${SCRIPT_DIR}/../cleanup/fast-observation-cleanup.sh" "$NAMESPACE" "benchmark=true" &>/dev/null || true
else
    kubectl delete observations -n "$NAMESPACE" -l benchmark=true --ignore-not-found=true &>/dev/null || true
fi

log_success "Benchmark complete!"

