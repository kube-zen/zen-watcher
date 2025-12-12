#!/bin/bash
#
# zen-demo-deploy-watcher.sh
#
# Thin wrapper around scripts/install.sh for zen-demo cluster deployment.
# This script delegates to the canonical installation orchestrator.
#
# Usage: ./hack/zen-demo-deploy-watcher.sh [IMAGE_TAG]
#
# Environment Variables:
#   ZEN_DEMO_CLUSTER_NAME=zen-demo    # Cluster name (default: zen-demo)
#   ZEN_DEMO_NAMESPACE=zen-system     # Namespace for watcher (default: zen-system)
#   ZEN_DEMO_IMAGE_TAG=zen-demo-...   # Image tag (default: auto-detect from build)
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

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
IMAGE_NAME="kubezen/zen-watcher"
IMAGE_TAG="${1:-${ZEN_DEMO_IMAGE_TAG:-zen-demo-$(git -C "${REPO_ROOT}" rev-parse --short HEAD 2>/dev/null || echo "latest")}}"

echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  Deploying zen-watcher to zen-demo${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

log_info "Cluster: ${CLUSTER_NAME}"
log_info "Namespace: ${NAMESPACE}"
log_info "Image: ${IMAGE_NAME}:${IMAGE_TAG}"
log_info "Delegating to canonical install.sh..."
echo ""

# Delegate to canonical install.sh with minimal flags
# Skip tools and monitoring for e2e validation (faster, focused)
export ZEN_CLUSTER_NAME="${CLUSTER_NAME}"
export ZEN_NAMESPACE="${NAMESPACE}"
export ZEN_WATCHER_IMAGE="${IMAGE_NAME}:${IMAGE_TAG}"
export SKIP_MONITORING="true"
export INSTALL_TRIVY="false"
export INSTALL_FALCO="false"
export INSTALL_KYVERNO="false"
export INSTALL_CHECKOV="false"
export INSTALL_KUBE_BENCH="false"
export NO_DOCKER_LOGIN="false"
export USE_EXISTING_CLUSTER="true"

# Run canonical install script
cd "${REPO_ROOT}"
"${REPO_ROOT}/scripts/install.sh" k3d \
    --skip-monitoring \
    --use-existing-cluster

echo ""
log_success "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
log_success "✅ zen-watcher deployed!"
log_success "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
