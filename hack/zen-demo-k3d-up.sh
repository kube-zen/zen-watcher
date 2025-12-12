#!/bin/bash
#
# zen-demo-k3d-up.sh
#
# Creates a k3d cluster named "zen-demo" for end-to-end validation of zen-watcher.
# This script is idempotent and uses explicit --context flags for all kubectl operations.
#
# Usage: ./hack/zen-demo-k3d-up.sh
#
# Environment Variables:
#   ZEN_DEMO_CLUSTER_NAME=zen-demo    # Cluster name (default: zen-demo)
#   ZEN_DEMO_API_PORT=6443            # API server port (default: 6443)
#   ZEN_DEMO_HTTP_PORT=8080          # HTTP ingress port (default: 8080)
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
API_PORT="${ZEN_DEMO_API_PORT:-6443}"
HTTP_PORT="${ZEN_DEMO_HTTP_PORT:-8080}"

echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  Creating zen-demo k3d cluster${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Check prerequisites
if ! command -v k3d &> /dev/null; then
    log_error "k3d is not installed"
    echo "  Install: https://k3d.io/v5.6.0/#installation"
    exit 1
fi

if ! command -v kubectl &> /dev/null; then
    log_error "kubectl is not installed"
    echo "  Install: https://kubernetes.io/docs/tasks/tools/"
    exit 1
fi

# Check if cluster already exists
if k3d cluster list 2>/dev/null | grep -q "^${CLUSTER_NAME}"; then
    log_info "Cluster '${CLUSTER_NAME}' already exists"
    log_info "To recreate, run: make zen-demo-down"
    exit 0
fi

# Check for k3d registry (optional, for local image push)
REGISTRY_NAME="k3d-zen-registry"
if k3d registry list 2>/dev/null | grep -q "${REGISTRY_NAME}"; then
    log_info "Using existing k3d registry: ${REGISTRY_NAME}"
    REGISTRY_USE="--registry-use ${REGISTRY_NAME}"
else
    log_info "No k3d registry found (images will be loaded via k3d image import)"
    REGISTRY_USE=""
fi

# Create cluster
log_info "Creating k3d cluster '${CLUSTER_NAME}'..."
log_info "  API Port: ${API_PORT}"
log_info "  HTTP Port: ${HTTP_PORT}"

k3d_create_args=(
    "cluster" "create" "${CLUSTER_NAME}"
    "--agents" "0"
    "--k3s-arg" "--disable=traefik@server:0"
    "--port" "${HTTP_PORT}:80@loadbalancer"
    "--port" "$((HTTP_PORT + 1)):443@loadbalancer"
    "--kubeconfig-update-default=false"
)

if [ "${API_PORT}" != "6443" ]; then
    k3d_create_args+=("--api-port" "${API_PORT}")
fi

if [ -n "${REGISTRY_USE}" ]; then
    k3d_create_args+=(${REGISTRY_USE})
fi

if ! timeout 240 k3d "${k3d_create_args[@]}"; then
    log_error "Failed to create cluster"
    exit 1
fi

log_success "Cluster '${CLUSTER_NAME}' created"

# Get kubeconfig path (k3d stores it in a specific location)
KUBECONFIG_PATH="${HOME}/.config/k3d/kubeconfig-${CLUSTER_NAME}.yaml"

# Wait for cluster to be ready
log_info "Waiting for cluster to be ready..."
max_wait=120
waited=0
while [ $waited -lt $max_wait ]; do
    if KUBECONFIG="${KUBECONFIG_PATH}" kubectl --context="k3d-${CLUSTER_NAME}" get nodes --request-timeout=5s &>/dev/null 2>&1; then
        log_success "Cluster is ready"
        break
    fi
    sleep 2
    waited=$((waited + 2))
done

if [ $waited -ge $max_wait ]; then
    log_error "Cluster did not become ready within ${max_wait}s"
    exit 1
fi

echo ""
log_success "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
log_success "✅ zen-demo cluster is ready!"
log_success "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "To use this cluster:"
echo "  export KUBECONFIG=${KUBECONFIG_PATH}"
echo "  kubectl --context=k3d-${CLUSTER_NAME} get nodes"
echo ""
echo "Or use the Makefile targets:"
echo "  make zen-demo-deploy-watcher"
echo "  make zen-demo-validate"
echo ""

