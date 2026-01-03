#!/bin/bash
#
# E2E Test Suite for zenctl diff
# Creates temporary k3d cluster, validates diff behavior, then cleans up
#
# Usage:
#   ./scripts/test/zenctl-diff-e2e.sh [--keep-cluster]
#
# Environment Variables:
#   K3D_CLUSTER_NAME - Cluster name (default: zenctl-diff-test-$(date +%s))
#   ZENCTL_BIN - Path to zenctl binary (default: ./zenctl)
#   KEEP_CLUSTER - Keep cluster after test (default: false)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$SCRIPT_DIR"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Configuration
CLUSTER_NAME="${K3D_CLUSTER_NAME:-zenctl-diff-test-$(date +%s)}"
ZENCTL_BIN="${ZENCTL_BIN:-./zenctl}"
KEEP_CLUSTER="${KEEP_CLUSTER:-false}"
TEST_NS="zen-diff-test"

# Parse args
if [ "${1:-}" = "--keep-cluster" ]; then
	KEEP_CLUSTER=true
fi

# Test results
TESTS_PASSED=0
TESTS_FAILED=0
FAILURES=()

log_info() {
	echo -e "${GREEN}ℹ${NC} $*"
}

log_error() {
	echo -e "${RED}✗${NC} $*"
}

log_step() {
	echo -e "${YELLOW}→${NC} $*"
}

# Cleanup function
cleanup() {
	if [ "$KEEP_CLUSTER" != "true" ]; then
		log_step "Cleaning up k3d cluster..."
		if k3d cluster list 2>/dev/null | grep -q "^${CLUSTER_NAME}"; then
			k3d cluster delete "${CLUSTER_NAME}" 2>/dev/null || true
			log_info "Cluster ${CLUSTER_NAME} deleted"
		fi
	fi
}

trap cleanup EXIT

# Check prerequisites
log_step "Checking prerequisites..."
if ! command -v k3d &>/dev/null; then
	log_error "k3d not found. Install: https://k3d.io/"
	exit 1
fi

if ! command -v kubectl &>/dev/null; then
	log_error "kubectl not found"
	exit 1
fi

if [ ! -f "$ZENCTL_BIN" ]; then
	log_error "zenctl binary not found at $ZENCTL_BIN"
	log_info "Build it with: make zenctl"
	exit 1
fi

# Create test namespace and temp directory
TEST_DIR=$(mktemp -d)
trap "rm -rf $TEST_DIR" EXIT

log_step "Creating k3d cluster: ${CLUSTER_NAME}..."
if ! timeout 180 k3d cluster create "${CLUSTER_NAME}" --wait --timeout 120s 2>&1; then
	log_error "Failed to create cluster"
	exit 1
fi

# Get kubeconfig for cluster
KUBECONFIG_FILE=$(mktemp)
k3d kubeconfig write "${CLUSTER_NAME}" > "$KUBECONFIG_FILE"
export KUBECONFIG="$KUBECONFIG_FILE"

log_step "Waiting for cluster to be ready..."
kubectl wait --for=condition=Ready nodes --all --timeout=60s --context="k3d-${CLUSTER_NAME}" || true

# Install CRDs if available
log_step "Installing CRDs (if available)..."
if [ -d "deployments/crds" ]; then
	kubectl apply -f deployments/crds/ --context="k3d-${CLUSTER_NAME}" --server-side --force-conflicts || {
		log_info "CRDs may already be installed or not available"
	}
	sleep 2
fi

# Create test namespace
kubectl create namespace "${TEST_NS}" --context="k3d-${CLUSTER_NAME}" || true

# Test 1: No drift (exit 0)
log_step "Test 1: No drift (exit 0)"
cat > "$TEST_DIR/flow.yaml" <<'EOF'
apiVersion: routing.zen.kube-zen.io/v1alpha1
kind: DeliveryFlow
metadata:
  name: test-flow
  namespace: zen-diff-test
spec:
  sources:
    - sourceKey: default/test-ingester/test-source
  outputs:
    - name: output1
      targetGroups:
        - rule: primary
          priority: 0
          destinations:
            - name: test-dest
        - rule: standby
          priority: 1
          destinations:
            - name: test-dest
EOF

# Apply manifest
kubectl apply -f "$TEST_DIR/flow.yaml" --context="k3d-${CLUSTER_NAME}" --namespace="${TEST_NS}" || {
	log_error "Failed to apply test manifest"
	exit 1
}

# Wait for resource to be created
sleep 2

# Run diff (should exit 0)
if "$ZENCTL_BIN" diff -f "$TEST_DIR/flow.yaml" -n "${TEST_NS}" --context="k3d-${CLUSTER_NAME}" 2>&1; then
	EXIT_CODE=$?
	if [ $EXIT_CODE -eq 0 ]; then
		log_info "✓ Test 1 PASSED: No drift detected (exit 0)"
		TESTS_PASSED=$((TESTS_PASSED + 1))
	else
		log_error "Test 1 FAILED: Expected exit 0, got $EXIT_CODE"
		TESTS_FAILED=$((TESTS_FAILED + 1))
		FAILURES+=("Test 1: Expected exit 0, got $EXIT_CODE")
	fi
else
	EXIT_CODE=$?
	log_error "Test 1 FAILED: Command failed with exit $EXIT_CODE"
	TESTS_FAILED=$((TESTS_FAILED + 1))
	FAILURES+=("Test 1: Command failed with exit $EXIT_CODE")
fi

# Test 2: Spec drift (exit 2)
log_step "Test 2: Spec drift (exit 2)"
# Mutate live object
kubectl patch deliveryflow test-flow -n "${TEST_NS}" --context="k3d-${CLUSTER_NAME}" \
	--type merge -p '{"spec":{"sources":[{"sourceKey":"default/test-ingester/modified-source"}]}}' || {
	log_error "Failed to patch DeliveryFlow"
	exit 1
}

sleep 1

# Run diff (should exit 2)
if "$ZENCTL_BIN" diff -f "$TEST_DIR/flow.yaml" -n "${TEST_NS}" --context="k3d-${CLUSTER_NAME}" 2>&1; then
	EXIT_CODE=0
else
	EXIT_CODE=$?
fi

if [ $EXIT_CODE -eq 2 ]; then
	log_info "✓ Test 2 PASSED: Spec drift detected (exit 2)"
	TESTS_PASSED=$((TESTS_PASSED + 1))
else
	log_error "Test 2 FAILED: Expected exit 2, got $EXIT_CODE"
	TESTS_FAILED=$((TESTS_FAILED + 1))
	FAILURES+=("Test 2: Expected exit 2, got $EXIT_CODE")
	# Show kubectl describe for debugging
	kubectl describe deliveryflow test-flow -n "${TEST_NS}" --context="k3d-${CLUSTER_NAME}" || true
fi

# Restore original state
kubectl apply -f "$TEST_DIR/flow.yaml" --context="k3d-${CLUSTER_NAME}" --namespace="${TEST_NS}" || true
sleep 1

# Test 3: Annotation drift with --ignore-annotations (exit 0)
log_step "Test 3: Annotation drift with --ignore-annotations (exit 0)"
# Add annotation
kubectl annotate deliveryflow test-flow -n "${TEST_NS}" --context="k3d-${CLUSTER_NAME}" \
	test-key=test-value --overwrite || {
	log_error "Failed to annotate DeliveryFlow"
	exit 1
}

sleep 1

# Run diff with --ignore-annotations (should exit 0)
if "$ZENCTL_BIN" diff -f "$TEST_DIR/flow.yaml" -n "${TEST_NS}" --context="k3d-${CLUSTER_NAME}" \
	--ignore-annotations 2>&1; then
	EXIT_CODE=$?
	if [ $EXIT_CODE -eq 0 ]; then
		log_info "✓ Test 3 PASSED: Annotation drift ignored (exit 0)"
		TESTS_PASSED=$((TESTS_PASSED + 1))
	else
		log_error "Test 3 FAILED: Expected exit 0, got $EXIT_CODE"
		TESTS_FAILED=$((TESTS_FAILED + 1))
		FAILURES+=("Test 3: Expected exit 0, got $EXIT_CODE")
	fi
else
	EXIT_CODE=$?
	log_error "Test 3 FAILED: Expected exit 0, got $EXIT_CODE"
	TESTS_FAILED=$((TESTS_FAILED + 1))
	FAILURES+=("Test 3: Expected exit 0, got $EXIT_CODE")
fi

# Test 4: Annotation drift without flag (exit 2)
log_step "Test 4: Annotation drift without flag (exit 2)"
# Run diff without --ignore-annotations (should exit 2)
if "$ZENCTL_BIN" diff -f "$TEST_DIR/flow.yaml" -n "${TEST_NS}" --context="k3d-${CLUSTER_NAME}" 2>&1; then
	EXIT_CODE=0
else
	EXIT_CODE=$?
fi

if [ $EXIT_CODE -eq 2 ]; then
	log_info "✓ Test 4 PASSED: Annotation drift detected (exit 2)"
	TESTS_PASSED=$((TESTS_PASSED + 1))
else
	log_error "Test 4 FAILED: Expected exit 2, got $EXIT_CODE"
	TESTS_FAILED=$((TESTS_FAILED + 1))
	FAILURES+=("Test 4: Expected exit 2, got $EXIT_CODE")
fi

# Restore original state
kubectl apply -f "$TEST_DIR/flow.yaml" --context="k3d-${CLUSTER_NAME}" --namespace="${TEST_NS}" || true
sleep 1

# Test 5: Secret redaction
log_step "Test 5: Secret redaction"
cat > "$TEST_DIR/secret-flow.yaml" <<'EOF'
apiVersion: routing.zen.kube-zen.io/v1alpha1
kind: DeliveryFlow
metadata:
  name: secret-flow
  namespace: zen-diff-test
spec:
  sources:
    - sourceKey: default/test-ingester/test-source
  outputs:
    - name: output1
      targetGroups:
        - rule: primary
          priority: 0
          destinations:
            - name: test-dest
              config:
                token: secret-token-12345
                password: my-secret-password
EOF

kubectl apply -f "$TEST_DIR/secret-flow.yaml" --context="k3d-${CLUSTER_NAME}" --namespace="${TEST_NS}" || {
	log_error "Failed to apply secret manifest"
	exit 1
}

sleep 2

# Export and check for redaction
EXPORT_OUTPUT=$("$ZENCTL_BIN" export flow secret-flow -n "${TEST_NS}" --context="k3d-${CLUSTER_NAME}" --format yaml 2>&1)
if echo "$EXPORT_OUTPUT" | grep -q "\[REDACTED\]" && ! echo "$EXPORT_OUTPUT" | grep -q "secret-token-12345"; then
	log_info "✓ Test 5 PASSED: Secrets redacted in export"
	TESTS_PASSED=$((TESTS_PASSED + 1))
else
	log_error "Test 5 FAILED: Secrets not properly redacted"
	TESTS_FAILED=$((TESTS_FAILED + 1))
	FAILURES+=("Test 5: Secrets not redacted")
fi

# Test 6: Multi-document YAML
log_step "Test 6: Multi-document YAML"
cat > "$TEST_DIR/multi-doc.yaml" <<'EOF'
apiVersion: routing.zen.kube-zen.io/v1alpha1
kind: DeliveryFlow
metadata:
  name: flow1
  namespace: zen-diff-test
spec:
  sources:
    - sourceKey: default/test-ingester/source1
---
apiVersion: routing.zen.kube-zen.io/v1alpha1
kind: DeliveryFlow
metadata:
  name: flow2
  namespace: zen-diff-test
spec:
  sources:
    - sourceKey: default/test-ingester/source2
EOF

kubectl apply -f "$TEST_DIR/multi-doc.yaml" --context="k3d-${CLUSTER_NAME}" --namespace="${TEST_NS}" || {
	log_error "Failed to apply multi-doc manifest"
	exit 1
}

sleep 2

# Run diff (should exit 0 for both resources)
if "$ZENCTL_BIN" diff -f "$TEST_DIR/multi-doc.yaml" -n "${TEST_NS}" --context="k3d-${CLUSTER_NAME}" 2>&1; then
	EXIT_CODE=$?
	if [ $EXIT_CODE -eq 0 ]; then
		log_info "✓ Test 6 PASSED: Multi-document YAML handled (exit 0)"
		TESTS_PASSED=$((TESTS_PASSED + 1))
	else
		log_error "Test 6 FAILED: Expected exit 0, got $EXIT_CODE"
		TESTS_FAILED=$((TESTS_FAILED + 1))
		FAILURES+=("Test 6: Expected exit 0, got $EXIT_CODE")
	fi
else
	EXIT_CODE=$?
	log_error "Test 6 FAILED: Expected exit 0, got $EXIT_CODE"
	TESTS_FAILED=$((TESTS_FAILED + 1))
	FAILURES+=("Test 6: Expected exit 0, got $EXIT_CODE")
fi

# Test 7: Directory mode
log_step "Test 7: Directory mode"
mkdir -p "$TEST_DIR/manifests"
cp "$TEST_DIR/flow.yaml" "$TEST_DIR/manifests/"

if "$ZENCTL_BIN" diff -f "$TEST_DIR/manifests" -n "${TEST_NS}" --context="k3d-${CLUSTER_NAME}" 2>&1; then
	EXIT_CODE=$?
	if [ $EXIT_CODE -eq 0 ]; then
		log_info "✓ Test 7 PASSED: Directory mode works (exit 0)"
		TESTS_PASSED=$((TESTS_PASSED + 1))
	else
		log_error "Test 7 FAILED: Expected exit 0, got $EXIT_CODE"
		TESTS_FAILED=$((TESTS_FAILED + 1))
		FAILURES+=("Test 7: Expected exit 0, got $EXIT_CODE")
	fi
else
	EXIT_CODE=$?
	log_error "Test 7 FAILED: Expected exit 0, got $EXIT_CODE"
	TESTS_FAILED=$((TESTS_FAILED + 1))
	FAILURES+=("Test 7: Expected exit 0, got $EXIT_CODE")
fi

# Test 8: Error handling (missing resource, exit 1)
log_step "Test 8: Error handling (missing resource, exit 1)"
cat > "$TEST_DIR/nonexistent.yaml" <<'EOF'
apiVersion: routing.zen.kube-zen.io/v1alpha1
kind: DeliveryFlow
metadata:
  name: nonexistent-flow
  namespace: zen-diff-test
spec:
  sources:
    - sourceKey: default/test-ingester/test-source
EOF

if "$ZENCTL_BIN" diff -f "$TEST_DIR/nonexistent.yaml" -n "${TEST_NS}" --context="k3d-${CLUSTER_NAME}" 2>&1; then
	EXIT_CODE=0
else
	EXIT_CODE=$?
fi

if [ $EXIT_CODE -eq 1 ]; then
	log_info "✓ Test 8 PASSED: Missing resource error (exit 1)"
	TESTS_PASSED=$((TESTS_PASSED + 1))
else
	log_error "Test 8 FAILED: Expected exit 1, got $EXIT_CODE"
	TESTS_FAILED=$((TESTS_FAILED + 1))
	FAILURES+=("Test 8: Expected exit 1, got $EXIT_CODE")
fi

# Summary
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "E2E Test Summary"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Tests passed: ${TESTS_PASSED}"
echo "Tests failed: ${TESTS_FAILED}"

if [ ${#FAILURES[@]} -gt 0 ]; then
	echo ""
	echo "Failures:"
	for failure in "${FAILURES[@]}"; do
		echo "  - $failure"
	done
fi

if [ "$KEEP_CLUSTER" = "true" ]; then
	echo ""
	log_info "Cluster ${CLUSTER_NAME} kept (KEEP_CLUSTER=true)"
	log_info "Kubeconfig: ${KUBECONFIG_FILE}"
fi

if [ $TESTS_FAILED -eq 0 ]; then
	log_info "All tests passed!"
	exit 0
else
	log_error "Some tests failed"
	exit 1
fi

