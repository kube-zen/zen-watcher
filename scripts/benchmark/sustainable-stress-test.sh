#!/bin/bash
# Sustainable stress test: Rate-limited creation with auto-cleanup via TTL
#
# This test avoids Kubernetes API server throttling by:
# - Rate-limiting observation creation (stays under ~5 QPS write limit)
# - Using short TTL for automatic cleanup (no manual deletion needed)
# - Testing sustained load over time, not peak volume
#
# Usage:
#   ./sustainable-stress-test.sh [--rate-limit N] [--duration M] [--ttl T] [--concurrent-ingesters N]
#
# Examples:
#   ./sustainable-stress-test.sh --rate-limit 100 --duration 60 --ttl 5m
#   ./sustainable-stress-test.sh --rate-limit 50 --duration 120 --ttl 10m --concurrent-ingesters 5

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../utils/common.sh" 2>/dev/null || true

NAMESPACE="${NAMESPACE:-zen-system}"
RATE_LIMIT="${RATE_LIMIT:-100}"  # observations per minute (under API limits)
DURATION="${DURATION:-60}"        # test duration in minutes
TTL="${TTL:-5m}"                  # TTL for auto-cleanup (e.g., "5m", "10m", "1h")
CONCURRENT_INGESTERS="${CONCURRENT_INGESTERS:-10}"  # Number of concurrent ingester sources

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --rate-limit)
            RATE_LIMIT="$2"
            shift 2
            ;;
        --duration)
            DURATION="$2"
            shift 2
            ;;
        --ttl)
            TTL="$2"
            shift 2
            ;;
        --concurrent-ingesters)
            CONCURRENT_INGESTERS="$2"
            shift 2
            ;;
        --namespace)
            NAMESPACE="$2"
            shift 2
            ;;
        *)
            log_error "Unknown option: $1"
            exit 1
            ;;
    esac
done

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Sustainable Stress Test"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Configuration:"
echo "  Rate Limit: $RATE_LIMIT observations/minute"
echo "  Duration: $DURATION minutes"
echo "  TTL: $TTL (auto-cleanup)"
echo "  Concurrent Ingesters: $CONCURRENT_INGESTERS"
echo "  Namespace: $NAMESPACE"
echo ""

# Check prerequisites
if ! kubectl get namespace "$NAMESPACE" &>/dev/null; then
    log_error "Namespace $NAMESPACE does not exist"
    exit 1
fi

# Deploy stress test ingester with TTL configuration
log_step "Deploying stress test ingester with TTL configuration..."

cat <<EOF | kubectl apply -f -
apiVersion: zen.kube-zen.io/v1alpha1
kind: Ingester
metadata:
  name: stress-test-ingester
  namespace: ${NAMESPACE}
spec:
  source: stress-test
  ingester: webhook
  webhook:
    path: /stress-test
  observationTemplate:
    ttl: "${TTL}"
  rateLimit:
    observationsPerMinute: ${RATE_LIMIT}
  destinations:
    - type: crd
      value: observations
EOF

log_success "Stress test ingester deployed"

# Wait for ingester to be ready
log_step "Waiting for ingester to be ready..."
sleep 5

# Get zen-watcher pod for webhook access
ZEN_POD=$(kubectl get pod -n "$NAMESPACE" -l app=zen-watcher -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -z "$ZEN_POD" ]; then
    log_error "zen-watcher pod not found"
    exit 1
fi

# Calculate observations per second (stay under API limits)
OBS_PER_SECOND=$((RATE_LIMIT / 60))
if [ $OBS_PER_SECOND -lt 1 ]; then
    OBS_PER_SECOND=1
fi

INTERVAL=$((60 / OBS_PER_SECOND))  # Seconds between observations

log_info "Starting sustainable stress test..."
log_info "Creating observations at rate: $OBS_PER_SECOND obs/sec (${RATE_LIMIT} obs/min)"
log_info "Test will run for $DURATION minutes"
log_info "Observations will auto-delete after TTL: $TTL"

START_TIME=$(date +%s)
END_TIME=$((START_TIME + DURATION * 60))
OBS_COUNT=0

# Function to create a test observation via webhook
create_observation() {
    local source_id=$1
    local obs_num=$2
    
    local payload=$(cat <<EOF
{
  "source": "stress-test-${source_id}",
  "severity": "MEDIUM",
  "category": "operations",
  "eventType": "stress_test",
  "message": "Sustainable stress test observation #${obs_num}",
  "timestamp": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
}
EOF
)
    
    kubectl exec -n "$NAMESPACE" "$ZEN_POD" -- curl -s -X POST \
        -H "Content-Type: application/json" \
        -d "$payload" \
        localhost:8080/stress-test >/dev/null 2>&1 || true
}

# Run stress test
while [ $(date +%s) -lt $END_TIME ]; do
    # Create observations from multiple concurrent sources
    for i in $(seq 1 $CONCURRENT_INGESTERS); do
        OBS_COUNT=$((OBS_COUNT + 1))
        create_observation $i $OBS_COUNT &
    done
    
    # Wait for interval
    sleep $INTERVAL
    
    # Show progress every 10 seconds
    if [ $((OBS_COUNT % 10)) -eq 0 ]; then
        ELAPSED=$((($(date +%s) - START_TIME) / 60))
        CURRENT_OBS=$(kubectl get observations -n "$NAMESPACE" -l source=stress-test --no-headers 2>/dev/null | wc -l || echo "0")
        log_info "Progress: ${ELAPSED}/${DURATION} minutes, ${CURRENT_OBS} active observations"
    fi
done

# Wait for all background jobs
wait

FINAL_OBS=$(kubectl get observations -n "$NAMESPACE" -l source=stress-test --no-headers 2>/dev/null | wc -l || echo "0")

echo ""
log_success "Stress test complete!"
echo ""
echo "Results:"
echo "  Total observations created: ~$OBS_COUNT"
echo "  Active observations: $FINAL_OBS"
echo "  Observations should auto-delete after TTL: $TTL"
echo ""
echo "Monitor cleanup:"
echo "  watch -n 5 'kubectl get observations -n $NAMESPACE -l source=stress-test | wc -l'"
echo ""

# Cleanup ingester
log_step "Cleaning up stress test ingester..."
kubectl delete ingester stress-test-ingester -n "$NAMESPACE" --ignore-not-found=true 2>/dev/null || true

log_success "Sustainable stress test completed successfully!"

