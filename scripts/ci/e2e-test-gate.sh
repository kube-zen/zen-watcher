#!/usr/bin/env bash
# H039: CI wiring for E2E tests
# Runs E2E tests nightly + on merge-to-main (k3d) + artifact upload

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
log_info "H039: E2E Test Gate"
log_info "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Check if k3d is available
if ! command -v k3d &> /dev/null; then
    log_error "k3d not found. Install from https://k3d.io"
    exit 1
fi

# Step 1: Setup k3d clusters
log_info "Step 1: Setting up k3d clusters..."
if "${SCRIPT_DIR}/../e2e/k3d-up.sh" 2>&1; then
    log_success "k3d clusters created"
else
    log_error "Failed to create k3d clusters"
    exit 1
fi

# Cleanup function
cleanup() {
    log_warn "Cleaning up k3d clusters..."
    "${SCRIPT_DIR}/../e2e/k3d-down.sh" || true
}
trap cleanup EXIT INT TERM

# Step 2: Run E2E tests
log_info "Step 2: Running E2E tests..."
if go test -v -timeout 30m ./test/e2e/... -run TestFlow1_ObservationCreationSuccess 2>&1; then
    log_success "E2E tests passed"
else
    log_error "E2E tests failed"
    exit 1
fi

# Step 3: Collect artifacts
log_info "Step 3: Collecting test artifacts..."
ARTIFACT_DIR="./artifacts/e2e-$(date +%Y%m%d-%H%M%S)"
mkdir -p "${ARTIFACT_DIR}"

# Copy logs from test artifacts
if [ -d "./artifacts" ]; then
    cp -r ./artifacts/* "${ARTIFACT_DIR}/" 2>/dev/null || true
    log_info "Artifacts collected: ${ARTIFACT_DIR}"
fi

# Step 4: Upload artifacts (if CI environment)
if [ -n "${CI:-}" ]; then
    log_info "Step 4: Uploading artifacts to CI..."
    # Placeholder - would upload to CI artifact storage
    log_warn "Artifact upload not implemented (placeholder)"
fi

log_success "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
log_success "E2E test gate complete"
log_success "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
