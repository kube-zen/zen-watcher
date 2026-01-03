#!/bin/bash
#
# Zen Watcher - Quick Demo Setup (Lightweight)
# 
# Lightweight demo: cluster + zen-watcher only (no monitoring stack)
# Uses kind for simplicity. For full demo with Grafana/VictoriaMetrics and
# platform options (k3d/kind/minikube), use: ./scripts/demo.sh
#
# Usage: 
#   ./scripts/quick-demo.sh                                    # Interactive mode
#   ./scripts/quick-demo.sh --non-interactive --deploy-mock-data
#
# Flags:
#   --non-interactive, --yes, -y          # Non-interactive mode
#   --use-existing-cluster                 # Use existing cluster if it exists
#   --skip-mock-data                       # Skip mock data deployment
#   --deploy-mock-data                     # Deploy mock data (explicit)
#   --install-trivy                        # Install Trivy
#   --install-falco                        # Install Falco
#   --install-kyverno                      # Install Kyverno
#   --install-checkov                      # Install Checkov
#   --install-kube-bench                   # Install kube-bench
#   --with-monitoring                      # Include observability stack (Grafana/VictoriaMetrics)
#   --no-docker-login                      # Don't use docker login credentials
#   --offline                              # Skip Helm repo updates (for air-gapped environments)
#   --skip-repo-update                     # Skip Helm repo updates (repos must already exist)

set -euo pipefail

# Source common utilities
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/utils/common.sh"
source "${SCRIPT_DIR}/cluster/utils.sh" 2>/dev/null || true

# Fixed platform: kind (simplified for quick demo)
PLATFORM="kind"

# Parse arguments
NON_INTERACTIVE=false
USE_EXISTING_CLUSTER=false
SKIP_MOCK_DATA=false
DEPLOY_MOCK_DATA=false
WITH_MONITORING=false

INSTALL_ARGS=("kind")  # Always use kind
for arg in "$@"; do
    case "$arg" in
        --non-interactive|--yes|-y)
            NON_INTERACTIVE=true
            INSTALL_ARGS+=("$arg")
            ;;
        --use-existing-cluster|--use-existing)
            USE_EXISTING_CLUSTER=true
            INSTALL_ARGS+=("--use-existing")
            ;;
        --skip-mock-data)
            SKIP_MOCK_DATA=true
            ;;
        --deploy-mock-data)
            DEPLOY_MOCK_DATA=true
            ;;
        --with-monitoring|--with-observability)
            WITH_MONITORING=true
            ;;
        --install-trivy|--install-falco|--install-kyverno|--install-checkov|--install-kube-bench|--no-docker-login|--offline|--skip-repo-update)
            INSTALL_ARGS+=("$arg")
            ;;
        # Ignore platform arguments (k3d, kind, minikube) - we always use kind
        k3d|kind|minikube)
            log_info "Platform selection ignored - quick-demo.sh always uses kind"
            ;;
        *)
            log_warn "Unknown argument: $arg (ignored)"
            ;;
    esac
done

# Default: skip monitoring for lightweight quick demo
if [ "$WITH_MONITORING" != true ]; then
    INSTALL_ARGS+=("--skip-monitoring")
fi

# Configuration
CLUSTER_NAME="${ZEN_CLUSTER_NAME:-zen-demo}"
NAMESPACE="${ZEN_NAMESPACE:-zen-system}"

# Export ZEN_DEMO_MINIMAL if set
if [ "${ZEN_DEMO_MINIMAL:-0}" = "1" ] || [ "${ZEN_DEMO_MINIMAL:-}" = "true" ]; then
    export ZEN_DEMO_MINIMAL="true"
    log_info "Minimal resource mode enabled (ZEN_DEMO_MINIMAL=1)"
fi

# Validate required tools early
log_step "Validating prerequisites..."
if ! command_exists "kind"; then
    log_error "kind is not installed"
    echo "  Install: https://kind.sigs.k8s.io/docs/user/quick-start/#installation"
    exit 1
fi

if ! command_exists "kubectl"; then
    log_error "kubectl is not installed"
    echo "  Install: https://kubernetes.io/docs/tasks/tools/"
    exit 1
fi

if ! command_exists "helm"; then
    log_error "helm is not installed"
    echo "  Install: https://helm.sh/docs/intro/install/"
    exit 1
fi

if ! command_exists "docker"; then
    log_error "docker is not installed or not running"
    echo "  Install: https://docs.docker.com/get-docker/"
    exit 1
fi

# Check if cluster exists
if cluster_exists "$PLATFORM" "$CLUSTER_NAME"; then
    if [ "$USE_EXISTING_CLUSTER" = true ]; then
        log_info "Using existing cluster: ${CLUSTER_NAME}"
    else
        log_error "Cluster '${CLUSTER_NAME}' already exists"
        echo "  Use --use-existing-cluster to use it, or destroy it first:"
        echo "  ${CYAN}./scripts/cluster/destroy.sh${NC}"
        exit 1
    fi
fi

echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  Zen Watcher - Quick Demo Setup (Lightweight)${NC}"
echo -e "${BLUE}  Platform: ${CYAN}kind${NC} (fixed for simplicity)"
echo -e "${BLUE}  Cluster: ${CYAN}${CLUSTER_NAME}${NC}"
if [ "$USE_EXISTING_CLUSTER" = true ]; then
    echo -e "${BLUE}  Mode: ${CYAN}Using existing cluster${NC}"
fi
if [ "$WITH_MONITORING" = true ]; then
    echo -e "${BLUE}  Monitoring: ${CYAN}Enabled${NC}"
else
    echo -e "${BLUE}  Monitoring: ${CYAN}Disabled (lightweight mode)${NC}"
fi
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Step 1: Install (cluster + components)
log_step "Installing Zen Watcher..."
"${SCRIPT_DIR}/install.sh" "$PLATFORM" "${INSTALL_ARGS[@]}" || {
    log_error "Installation failed"
    exit 1
}

# Get kubeconfig (kind-specific)
if command_exists "get_kubeconfig_path" 2>/dev/null || type get_kubeconfig_path >/dev/null 2>&1; then
    KUBECONFIG_FILE=$(get_kubeconfig_path "kind" "$CLUSTER_NAME" 2>/dev/null || echo "${HOME}/.kube/kind-${CLUSTER_NAME}-config")
else
    # Fallback for kind
    KUBECONFIG_FILE="${HOME}/.kube/kind-${CLUSTER_NAME}-config"
fi
export KUBECONFIG="${KUBECONFIG_FILE}"

# Step 2: Deploy mock data (if requested)
# Logic: --deploy-mock-data takes precedence, then --skip-mock-data, then non-interactive defaults to deploy
if [ "$SKIP_MOCK_DATA" = true ]; then
    log_info "Skipping mock data deployment (--skip-mock-data)"
elif [ "$DEPLOY_MOCK_DATA" = true ]; then
    log_step "Deploying mock data..."
    "${SCRIPT_DIR}/data/mock-data.sh" "$NAMESPACE" || {
        log_warn "Mock data deployment had issues, continuing..."
    }
elif [ "$NON_INTERACTIVE" = true ]; then
    # Non-interactive mode: default to deploying mock data
    log_step "Deploying mock data (non-interactive mode)..."
    "${SCRIPT_DIR}/data/mock-data.sh" "$NAMESPACE" || {
        log_warn "Mock data deployment had issues, continuing..."
    }
else
    # Interactive mode: prompt user
    echo ""
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${CYAN}  Deploy Mock Data?${NC}"
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    if [ -t 0 ]; then
        read -p "$(echo -e ${YELLOW}Deploy mock data? [Y/n]${NC}) " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Nn]$ ]]; then
            log_step "Deploying mock data..."
            "${SCRIPT_DIR}/data/mock-data.sh" "$NAMESPACE" || {
                log_warn "Mock data deployment had issues, continuing..."
            }
        else
            log_info "Skipping mock data deployment"
        fi
    else
        # Not a TTY, default to deploying
        log_step "Deploying mock data..."
        "${SCRIPT_DIR}/data/mock-data.sh" "$NAMESPACE" || {
            log_warn "Mock data deployment had issues, continuing..."
        }
    fi
fi

# Summary
echo ""
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}  ✅ Quick demo environment is ready!${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

if [ "$WITH_MONITORING" = true ]; then
    log_info "For Grafana access, check the output above or run:"
    echo -e "  ${CYAN}kubectl port-forward -n grafana svc/grafana 3000:3000${NC}"
    echo ""
fi

echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  Quick Commands${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "${YELLOW}View observations:${NC}"
echo -e "  ${CYAN}kubectl get observations -n ${NAMESPACE}${NC}"
echo ""
echo -e "${YELLOW}Watch observations:${NC}"
echo -e "  ${CYAN}kubectl get observations -n ${NAMESPACE} --watch${NC}"
echo ""
echo -e "${YELLOW}Check zen-watcher status:${NC}"
echo -e "  ${CYAN}kubectl get pods -n ${NAMESPACE}${NC}"
echo ""
echo -e "${YELLOW}View zen-watcher logs:${NC}"
echo -e "  ${CYAN}kubectl logs -n ${NAMESPACE} -l app.kubernetes.io/name=zen-watcher${NC}"
echo ""
echo -e "${YELLOW}Access metrics (port-forward):${NC}"
echo -e "  ${CYAN}kubectl port-forward -n ${NAMESPACE} svc/zen-watcher 8080:8080${NC}"
echo -e "  ${CYAN}curl http://localhost:8080/metrics${NC}"
echo ""
if [ "$WITH_MONITORING" != true ]; then
    echo -e "${YELLOW}For full demo with Grafana/VictoriaMetrics:${NC}"
    echo -e "  ${CYAN}./scripts/demo.sh${NC}"
    echo ""
fi
echo -e "${YELLOW}Clean up cluster:${NC}"
echo -e "  ${CYAN}./scripts/cluster/destroy.sh${NC}"
echo ""

show_total_time

