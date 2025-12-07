#!/bin/bash
#
# Zen Watcher - Quick Demo Setup
# 
# Complete demo setup: cluster + installation + mock data
#
# Usage: 
#   ./scripts/quick-demo.sh              # Uses k3d (interactive)
#   ./scripts/quick-demo.sh kind           # Uses kind
#   ./scripts/quick-demo.sh minikube      # Uses minikube
#   ./scripts/quick-demo.sh --non-interactive  # Non-interactive mode
#
# Flags:
#   --non-interactive, --yes, -y          # Non-interactive mode
#   --skip-mock-data                      # Skip mock data deployment
#   --deploy-mock-data                    # Deploy mock data (explicit)
#   --install-trivy                       # Install Trivy
#   --install-falco                       # Install Falco
#   --install-kyverno                     # Install Kyverno
#   --install-checkov                     # Install Checkov
#   --install-kube-bench                  # Install kube-bench
#   --skip-monitoring                     # Skip observability stack
#   --no-docker-login                     # Don't use docker login credentials

set -euo pipefail

# Source common utilities
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/utils/common.sh"

# Parse arguments
PLATFORM="k3d"
NON_INTERACTIVE=false
SKIP_MOCK_DATA=false
DEPLOY_MOCK_DATA=false
SKIP_MONITORING=false

INSTALL_ARGS=()
for arg in "$@"; do
    case "$arg" in
        --non-interactive|--yes|-y)
            NON_INTERACTIVE=true
            INSTALL_ARGS+=("$arg")
            ;;
        --skip-mock-data)
            SKIP_MOCK_DATA=true
            ;;
        --deploy-mock-data)
            DEPLOY_MOCK_DATA=true
            ;;
        --skip-monitoring|--skip-observability)
            SKIP_MONITORING=true
            INSTALL_ARGS+=("$arg")
            ;;
        --install-trivy|--install-falco|--install-kyverno|--install-checkov|--install-kube-bench|--no-docker-login)
            INSTALL_ARGS+=("$arg")
            ;;
        k3d|kind|minikube)
            PLATFORM="$arg"
            INSTALL_ARGS+=("$arg")
            ;;
    esac
done

# Configuration
CLUSTER_NAME="${ZEN_CLUSTER_NAME:-zen-demo}"
NAMESPACE="${ZEN_NAMESPACE:-zen-system}"

echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  Zen Watcher - Quick Demo Setup${NC}"
echo -e "${BLUE}  Platform: ${CYAN}${PLATFORM}${NC}"
echo -e "${BLUE}  Cluster: ${CYAN}${CLUSTER_NAME}${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Step 1: Install (cluster + components)
log_step "Installing Zen Watcher and components..."
"${SCRIPT_DIR}/install.sh" "$PLATFORM" "${INSTALL_ARGS[@]}" || {
    log_error "Installation failed"
    exit 1
}

# Get kubeconfig
KUBECONFIG_FILE="${HOME}/.kube/${CLUSTER_NAME}-kubeconfig"
export KUBECONFIG="${KUBECONFIG_FILE}"

# Step 2: Deploy mock data (if requested)
if [ "$DEPLOY_MOCK_DATA" = true ] || ([ "$SKIP_MOCK_DATA" != true ] && [ "$NON_INTERACTIVE" != true ]); then
    if [ "$NON_INTERACTIVE" != true ] && [ "$DEPLOY_MOCK_DATA" != true ]; then
        echo ""
        echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
        echo -e "${CYAN}  Deploy Mock Data?${NC}"
        echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
        if [ -t 0 ]; then
            read -p "$(echo -e ${YELLOW}Deploy mock data? [Y/n]${NC}) " -n 1 -r
            echo
            if [[ $REPLY =~ ^[Nn]$ ]]; then
                SKIP_MOCK_DATA=true
            else
                DEPLOY_MOCK_DATA=true
            fi
        else
            DEPLOY_MOCK_DATA=true
        fi
    fi
    
    if [ "$DEPLOY_MOCK_DATA" = true ] && [ "$SKIP_MOCK_DATA" != true ]; then
        log_step "Deploying mock data..."
        "${SCRIPT_DIR}/data/mock-data.sh" "$NAMESPACE" || {
            log_warn "Mock data deployment had issues, continuing..."
        }
    fi
fi

# Summary
echo ""
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}  ✅ Demo environment is ready!${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "${CYAN}To clean up:${NC}"
echo -e "  ${CYAN}./scripts/cluster/destroy.sh${NC}"
echo ""

show_total_time

