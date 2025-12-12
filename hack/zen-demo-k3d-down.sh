#!/bin/bash
#
# zen-demo-k3d-down.sh
#
# Deletes the zen-demo k3d cluster cleanly.
# This script is idempotent (safe to re-run).
#
# Usage: ./hack/zen-demo-k3d-down.sh
#
# Environment Variables:
#   ZEN_DEMO_CLUSTER_NAME=zen-demo    # Cluster name (default: zen-demo)
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

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

echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  Deleting zen-demo k3d cluster${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Check if cluster exists
if ! k3d cluster list 2>/dev/null | grep -q "^${CLUSTER_NAME}"; then
    log_info "Cluster '${CLUSTER_NAME}' not found (already deleted)"
    exit 0
fi

# Delete cluster
log_info "Deleting k3d cluster '${CLUSTER_NAME}'..."
if k3d cluster delete "${CLUSTER_NAME}"; then
    log_success "Cluster '${CLUSTER_NAME}' deleted"
else
    log_error "Failed to delete cluster"
    exit 1
fi

# Clean up kubeconfig if it exists
KUBECONFIG_PATH="${HOME}/.config/k3d/kubeconfig-${CLUSTER_NAME}.yaml"
if [ -f "${KUBECONFIG_PATH}" ]; then
    log_info "Removing kubeconfig: ${KUBECONFIG_PATH}"
    rm -f "${KUBECONFIG_PATH}"
fi

echo ""
log_success "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
log_success "✅ Cleanup complete!"
log_success "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

