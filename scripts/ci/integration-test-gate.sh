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

# Step 1: Unit tests
log_info "Step 1: Running unit tests..."
if make test-unit 2>&1; then
    log_success "Unit tests passed"
else
    log_error "Unit tests failed"
    exit 1
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
if make test-integration 2>&1; then
    log_success "Integration tests passed"
else
    log_error "Integration tests failed"
    exit 1
fi

log_success "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
log_success "Integration test gate complete"
log_success "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
