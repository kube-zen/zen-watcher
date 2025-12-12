#!/bin/bash
#
# zen-demo-deploy-watcher.sh
#
# Deploys zen-watcher CRDs and deployment to the zen-demo k3d cluster.
# Uses explicit --context flags for all kubectl operations.
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

KUBECONFIG_PATH="${HOME}/.config/k3d/kubeconfig-${CLUSTER_NAME}.yaml"
KUBECTL_CMD="kubectl --kubeconfig=${KUBECONFIG_PATH} --context=k3d-${CLUSTER_NAME}"

echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  Deploying zen-watcher to zen-demo${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Check prerequisites
if [ ! -f "${KUBECONFIG_PATH}" ]; then
    log_error "kubeconfig not found: ${KUBECONFIG_PATH}"
    echo "  Run: make zen-demo-up"
    exit 1
fi

# Verify cluster is accessible
if ! ${KUBECTL_CMD} get nodes &>/dev/null; then
    log_error "Cannot access cluster '${CLUSTER_NAME}'"
    echo "  Run: make zen-demo-up"
    exit 1
fi

log_info "Cluster: ${CLUSTER_NAME}"
log_info "Namespace: ${NAMESPACE}"
log_info "Image: ${IMAGE_NAME}:${IMAGE_TAG}"
echo ""

# Create namespace
log_info "Creating namespace '${NAMESPACE}'..."
${KUBECTL_CMD} create namespace "${NAMESPACE}" --dry-run=client -o yaml | ${KUBECTL_CMD} apply -f - || true

# Install CRDs
log_info "Installing CRDs..."
CRD_DIR="${REPO_ROOT}/deployments/crds"
for crd in "${CRD_DIR}"/*.yaml; do
    if [ -f "${crd}" ] && grep -q "kind: CustomResourceDefinition" "${crd}"; then
        log_info "  Applying $(basename ${crd})..."
        ${KUBECTL_CMD} apply -f "${crd}"
    fi
done

# Wait for CRDs to be established
log_info "Waiting for CRDs to be established..."
for crd in ingesters.zen.kube-zen.io observations.zen.kube-zen.io; do
    ${KUBECTL_CMD} wait --for condition=established --timeout=60s crd/${crd} || {
        log_warn "CRD ${crd} not established within timeout (continuing anyway)"
    }
done

# Deploy watcher using Helm
log_info "Deploying zen-watcher via Helm..."
HELM_CMD="helm --kubeconfig=${KUBECONFIG_PATH} --kube-context=k3d-${CLUSTER_NAME}"

# Check if Helm chart exists
HELM_CHART="${REPO_ROOT}/deployments/helm/zen-watcher"
if [ ! -d "${HELM_CHART}" ]; then
    log_error "Helm chart not found: ${HELM_CHART}"
    exit 1
fi

# Install or upgrade
if ${HELM_CMD} list -n "${NAMESPACE}" | grep -q "zen-watcher"; then
    log_info "Upgrading existing zen-watcher deployment..."
    ${HELM_CMD} upgrade zen-watcher "${HELM_CHART}" \
        --namespace "${NAMESPACE}" \
        --set image.repository="${IMAGE_NAME}" \
        --set image.tag="${IMAGE_TAG}" \
        --set image.pullPolicy=IfNotPresent \
        --set crd.install=false \
        --wait --timeout=5m
else
    log_info "Installing zen-watcher..."
    ${HELM_CMD} install zen-watcher "${HELM_CHART}" \
        --namespace "${NAMESPACE}" \
        --create-namespace \
        --set image.repository="${IMAGE_NAME}" \
        --set image.tag="${IMAGE_TAG}" \
        --set image.pullPolicy=IfNotPresent \
        --set crd.install=false \
        --wait --timeout=5m
fi

# Wait for pod to be ready
log_info "Waiting for zen-watcher pod to be ready..."
${KUBECTL_CMD} wait --for=condition=ready pod \
    -l app.kubernetes.io/name=zen-watcher \
    -n "${NAMESPACE}" \
    --timeout=120s || {
    log_warn "Pod not ready within timeout"
    log_info "Pod status:"
    ${KUBECTL_CMD} get pods -n "${NAMESPACE}" -l app.kubernetes.io/name=zen-watcher
    log_info "Pod logs:"
    ${KUBECTL_CMD} logs -n "${NAMESPACE}" -l app.kubernetes.io/name=zen-watcher --tail=20 || true
}

echo ""
log_success "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
log_success "✅ zen-watcher deployed!"
log_success "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "To check status:"
echo "  ${KUBECTL_CMD} get pods -n ${NAMESPACE}"
echo "  ${KUBECTL_CMD} logs -n ${NAMESPACE} -l app.kubernetes.io/name=zen-watcher"
echo ""

