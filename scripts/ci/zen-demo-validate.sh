#!/bin/bash
#
# zen-demo-validate.sh
#
# CI-friendly entry point for zen-demo validation.
# Runs canonical cluster create, deploy, and e2e validation.
#
# Usage: ./scripts/ci/zen-demo-validate.sh [--load-test]
#
# Environment Variables:
#   ZEN_DEMO_CLUSTER_NAME=zen-demo    # Cluster name (default: zen-demo)
#   ZEN_DEMO_NAMESPACE=zen-system     # Namespace (default: zen-system)
#   ZEN_DEMO_LOAD_TEST=1              # Enable load test (default: disabled)
#   ZEN_DEMO_BACKEND=k3d              # Backend: k3d, kind, or minikube (default: k3d)
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

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

CLUSTER_NAME="${ZEN_DEMO_CLUSTER_NAME:-zen-demo}"
NAMESPACE="${ZEN_DEMO_NAMESPACE:-zen-system}"
ENABLE_LOAD_TEST="${ZEN_DEMO_LOAD_TEST:-0}"

# Parse arguments
if [[ "${1:-}" == "--load-test" ]]; then
	ENABLE_LOAD_TEST=1
fi

echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  zen-demo CI Validation${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

cd "${REPO_ROOT}"

# Step 1: Create cluster (canonical script)
log_info "Step 1: Creating zen-demo cluster (backend: ${BACKEND})..."
if ! ./scripts/cluster/create.sh "${BACKEND}" "${CLUSTER_NAME}"; then
	log_error "Cluster creation failed"
	exit 1
fi
log_success "Cluster created"

# Step 2: Build and load image
log_info "Step 2: Building and loading watcher image..."
if ! make zen-demo-build-push; then
	log_error "Image build/push failed"
	exit 1
fi
log_success "Image loaded"

# Step 3: Deploy watcher (canonical script)
log_info "Step 3: Deploying zen-watcher..."
if ! make zen-demo-deploy-watcher; then
	log_error "Watcher deployment failed"
	exit 1
fi
log_success "Watcher deployed"

# Step 4: Run e2e validation
log_info "Step 4: Running e2e validation..."
if [ "${ENABLE_LOAD_TEST}" = "1" ]; then
	log_info "  Load test enabled (60-120s burst)"
	if ! make zen-demo-validate-load; then
		log_error "E2E validation with load test failed"
		exit 1
	fi
else
	if ! make zen-demo-validate; then
		log_error "E2E validation failed"
		exit 1
	fi
fi
log_success "E2E validation passed"

# Step 5: Export artifacts (for CI)
ARTIFACTS_DIR="${ARTIFACTS_DIR:-/tmp/zen-demo-artifacts}"
mkdir -p "${ARTIFACTS_DIR}"

log_info "Step 5: Exporting validation artifacts..."
# Determine kubeconfig path based on backend
case "${BACKEND}" in
	k3d)
		KUBECONFIG="${HOME}/.config/k3d/kubeconfig-${CLUSTER_NAME}.yaml"
		CONTEXT="k3d-${CLUSTER_NAME}"
		;;
	kind)
		KUBECONFIG="${HOME}/.kube/config"
		CONTEXT="kind-${CLUSTER_NAME}"
		;;
	minikube)
		KUBECONFIG="${HOME}/.kube/config"
		CONTEXT="${CLUSTER_NAME}"
		;;
esac

# Export pod logs
kubectl --kubeconfig="${KUBECONFIG}" --context="${CONTEXT}" logs -n "${NAMESPACE}" -l app.kubernetes.io/name=zen-watcher --tail=100 > "${ARTIFACTS_DIR}/watcher-logs.txt" 2>&1 || true

# Export pod status and events (debug bundle)
kubectl --kubeconfig="${KUBECONFIG}" --context="${CONTEXT}" get pods -n "${NAMESPACE}" -o yaml > "${ARTIFACTS_DIR}/pods.yaml" 2>&1 || true
kubectl --kubeconfig="${KUBECONFIG}" --context="${CONTEXT}" get events -n "${NAMESPACE}" --sort-by='.lastTimestamp' > "${ARTIFACTS_DIR}/events.txt" 2>&1 || true

# Export Ingester CRs
kubectl --kubeconfig="${KUBECONFIG}" --context="${CONTEXT}" get ingesters -A -o yaml > "${ARTIFACTS_DIR}/ingesters.yaml" 2>&1 || true

# Export Observations
kubectl --kubeconfig="${KUBECONFIG}" --context="${CONTEXT}" get observations -A -o yaml > "${ARTIFACTS_DIR}/observations.yaml" 2>&1 || true

log_success "Artifacts exported to ${ARTIFACTS_DIR}"

echo ""
log_success "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
log_success "✅ zen-demo validation complete!"
log_success "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

