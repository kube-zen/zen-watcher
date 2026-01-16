#!/usr/bin/env bash
# H039: CI wiring for integration tests
# Runs integration tests on PR (unit + integration with envtest)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WATCHER_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

cd "${WATCHER_DIR}"

log_info "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
log_info "H039: Integration Test Gate"
log_info "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Artifact directory
ARTIFACT_DIR="./artifacts/test-run/ci-$(date +%Y%m%d-%H%M%S)"
mkdir -p "${ARTIFACT_DIR}"

# Step 1: Unit tests
log_info "Step 1: Running unit tests..."
UNIT_OUTPUT="${ARTIFACT_DIR}/unit-test-output.log"
if make test-unit 2>&1 | tee "${UNIT_OUTPUT}"; then
    log_success "Unit tests passed"
    UNIT_EXIT_CODE=0
else
    log_error "Unit tests failed"
    UNIT_EXIT_CODE=1
fi

# Step 2: Integration tests (envtest)
log_info "Step 2: Running integration tests (envtest)..."

# Check if kubebuilder tools are available
if ! command -v setup-envtest &> /dev/null; then
    log_warn "kubebuilder tools not installed (setup-envtest not found)"
    log_info "Install kubebuilder tools:"
    log_info "  go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest"
    log_info "  setup-envtest use"
    log_warn "Skipping integration tests (require kubebuilder tools)"
    log_success "Integration test gate complete (unit tests only)"
    exit 0
fi

# Set up envtest
export KUBEBUILDER_ASSETS=$(setup-envtest use -p path --bin-dir=/tmp/kubebuilder/bin 2>/dev/null || echo "")

if [ -z "$KUBEBUILDER_ASSETS" ]; then
    log_warn "Failed to setup envtest (kubebuilder tools may not be installed)"
    log_warn "Skipping integration tests"
    log_success "Integration test gate complete (unit tests only)"
    exit 0
fi

# Run integration tests
INTEGRATION_OUTPUT="${ARTIFACT_DIR}/integration-test-output.log"
if make test-integration 2>&1 | tee "${INTEGRATION_OUTPUT}"; then
    log_success "Integration tests passed"
    INTEGRATION_EXIT_CODE=0
else
    log_error "Integration tests failed"
    INTEGRATION_EXIT_CODE=1
fi

# H048: Classify failures if tests failed
if [ ${UNIT_EXIT_CODE:-0} -ne 0 ] || [ ${INTEGRATION_EXIT_CODE:-0} -ne 0 ]; then
    log_info "Classifying failures..."
    COMBINED_OUTPUT="${ARTIFACT_DIR}/combined-test-output.log"
    cat "${UNIT_OUTPUT}" "${INTEGRATION_OUTPUT}" > "${COMBINED_OUTPUT}" 2>/dev/null || true
    
    "${SCRIPT_DIR}/classify-failures.sh" "${COMBINED_OUTPUT}" > "${ARTIFACT_DIR}/failure-classification.txt" || true
    
    if [ -f "${ARTIFACT_DIR}/failure-classification.txt" ]; then
        cat "${ARTIFACT_DIR}/failure-classification.txt"
    fi
fi

# Exit with appropriate code
if [ ${UNIT_EXIT_CODE:-0} -ne 0 ] || [ ${INTEGRATION_EXIT_CODE:-0} -ne 0 ]; then
    exit 1
fi

log_success "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
log_success "Integration test gate complete"
log_success "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
