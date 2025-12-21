#!/bin/bash
# Comprehensive demo validation
#
# Usage:
#   ./scripts/ci/demo-e2e-test.sh [namespace] [timeout]
#
# Examples:
#   ./scripts/ci/demo-e2e-test.sh zen-system 600

set -euo pipefail

NAMESPACE="${1:-zen-system}"
TIMEOUT="${2:-600}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../utils/readiness.sh" 2>/dev/null || true

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $@"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $@"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $@"; }
log_error() { echo -e "${RED}[ERROR]${NC} $@" >&2; }

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  E2E Demo Validation"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Test 1: Component health
log_info "1. Checking component health..."
if ! kubectl get pods -n "$NAMESPACE" -o wide; then
    log_error "Failed to get pods"
    exit 1
fi

# Test 2: Metrics availability
log_info "2. Testing metrics endpoints..."
ZEN_POD=$(kubectl get pod -n "$NAMESPACE" -l app=zen-watcher -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -z "$ZEN_POD" ]; then
    log_error "zen-watcher pod not found"
    exit 1
fi

if ! kubectl exec -n "$NAMESPACE" "$ZEN_POD" -- curl -f localhost:9090/metrics 2>/dev/null | grep -q "zen_watcher"; then
    log_error "Metrics endpoint not responding correctly"
    exit 1
fi
log_success "Metrics endpoint is working"

# Test 3: Observation creation
log_info "3. Testing observation creation..."
# Create a test observation
cat <<EOF | kubectl apply -f - 2>/dev/null || true
apiVersion: zen.kube-zen.io/v1
kind: Observation
metadata:
  name: e2e-test-observation
  namespace: ${NAMESPACE}
  labels:
    e2e-test: "true"
spec:
  source: e2e-test
  category: test
  severity: LOW
  eventType: test-event
EOF

sleep 10
OBS_COUNT=$(kubectl get observations -n "$NAMESPACE" --no-headers 2>/dev/null | wc -l || echo "0")
if [ "$OBS_COUNT" -eq 0 ]; then
    log_error "No observations created"
    exit 1
fi
log_success "Observations created successfully ($OBS_COUNT found)"

# Cleanup test observation
kubectl delete observation e2e-test-observation -n "$NAMESPACE" --ignore-not-found=true 2>/dev/null || true

# Test 4: CRD availability
log_info "4. Testing CRD availability..."
if ! kubectl get crd observations.zen.kube-zen.io >/dev/null 2>&1; then
    log_error "Observation CRD not found"
    exit 1
fi
log_success "CRDs are available"

# Test 5: Service endpoints
log_info "5. Testing service endpoints..."
if ! kubectl get svc -n "$NAMESPACE" -l app=zen-watcher >/dev/null 2>&1; then
    log_warn "Service not found (may be expected in some deployments)"
else
    log_success "Service is available"
fi

echo ""
log_success "E2E validation passed!"

