#!/bin/bash
#
# Zen Watcher - Quick Demo Setup
# 
# Complete demo setup: cluster + installation + mock data
#
# Usage: 
#   ./scripts/quick-demo.sh                                    # Uses k3d (interactive)
#   ./scripts/quick-demo.sh k3d --non-interactive --deploy-mock-data
#   ./scripts/quick-demo.sh kind --non-interactive --deploy-mock-data
#   ./scripts/quick-demo.sh minikube --non-interactive --deploy-mock-data
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
#   --skip-monitoring                      # Skip observability stack
#   --no-docker-login                      # Don't use docker login credentials

set -euo pipefail

# Source common utilities
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/utils/common.sh"
source "${SCRIPT_DIR}/cluster/utils.sh" 2>/dev/null || true

# Parse arguments
PLATFORM="k3d"
NON_INTERACTIVE=false
USE_EXISTING_CLUSTER=false
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

# Export ZEN_DEMO_MINIMAL if set
if [ "${ZEN_DEMO_MINIMAL:-0}" = "1" ] || [ "${ZEN_DEMO_MINIMAL:-}" = "true" ]; then
    export ZEN_DEMO_MINIMAL="true"
    log_info "Minimal resource mode enabled (ZEN_DEMO_MINIMAL=1)"
fi

# Validate required tools early
log_step "Validating prerequisites..."
case "$PLATFORM" in
    k3d)
        if ! command_exists "k3d"; then
            log_error "k3d is not installed"
            echo "  Install: https://k3d.io/#installation"
            exit 1
        fi
        ;;
    kind)
        if ! command_exists "kind"; then
            log_error "kind is not installed"
            echo "  Install: https://kind.sigs.k8s.io/docs/user/quick-start/#installation"
            exit 1
        fi
        ;;
    minikube)
        if ! command_exists "minikube"; then
            log_error "minikube is not installed"
            echo "  Install: https://minikube.sigs.k8s.io/docs/start/"
            exit 1
        fi
        ;;
esac

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
echo -e "${BLUE}  Zen Watcher - Quick Demo Setup${NC}"
echo -e "${BLUE}  Platform: ${CYAN}${PLATFORM}${NC}"
echo -e "${BLUE}  Cluster: ${CYAN}${CLUSTER_NAME}${NC}"
if [ "$USE_EXISTING_CLUSTER" = true ]; then
    echo -e "${BLUE}  Mode: ${CYAN}Using existing cluster${NC}"
fi
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Step 1: Install (cluster + components)
log_step "Installing Zen Watcher and components..."
"${SCRIPT_DIR}/install.sh" "$PLATFORM" "${INSTALL_ARGS[@]}" || {
    log_error "Installation failed"
    exit 1
}

# Get kubeconfig
if command_exists "get_kubeconfig_path" 2>/dev/null || type get_kubeconfig_path >/dev/null 2>&1; then
    KUBECONFIG_FILE=$(get_kubeconfig_path "$PLATFORM" "$CLUSTER_NAME" 2>/dev/null || echo "${HOME}/.kube/${CLUSTER_NAME}-kubeconfig")
else
    # Fallback if function not available
    case "$PLATFORM" in
        k3d)
            KUBECONFIG_FILE="${HOME}/.kube/${CLUSTER_NAME}-kubeconfig"
            ;;
        kind)
            KUBECONFIG_FILE="${HOME}/.kube/kind-${CLUSTER_NAME}-config"
            ;;
        minikube)
            KUBECONFIG_FILE="${HOME}/.kube/config"
            ;;
        *)
            KUBECONFIG_FILE="${HOME}/.kube/${CLUSTER_NAME}-kubeconfig"
            ;;
    esac
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
echo -e "${GREEN}  ✅ Demo environment is ready!${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Show endpoints and credentials
if [ "$SKIP_MONITORING" != true ]; then
    log_step "Setting up Grafana access..."
    GRAFANA_PASSWORD=""
    GRAFANA_USER="zen"
    GRAFANA_PORT="${GRAFANA_PORT:-8080}"
    
    # Try to get password from Grafana secret (helmfile sets admin-password key)
    if kubectl get secret -n grafana grafana -o jsonpath='{.data.admin-password}' 2>/dev/null | base64 -d 2>/dev/null > /tmp/grafana-password.txt 2>/dev/null; then
        GRAFANA_PASSWORD=$(cat /tmp/grafana-password.txt 2>/dev/null || echo "")
        rm -f /tmp/grafana-password.txt 2>/dev/null || true
    fi
    
    # If password not found, try to get from helmfile values or generate one
    if [ -z "$GRAFANA_PASSWORD" ]; then
        # Try alternative secret name
        if kubectl get secret -n zen-system grafana -o jsonpath='{.data.admin-password}' 2>/dev/null | base64 -d 2>/dev/null > /tmp/grafana-password.txt 2>/dev/null; then
            GRAFANA_PASSWORD=$(cat /tmp/grafana-password.txt 2>/dev/null || echo "")
            rm -f /tmp/grafana-password.txt 2>/dev/null || true
        fi
    fi
    
    # Wait for Grafana pod to be ready
    log_info "Waiting for Grafana to be ready..."
    if kubectl wait --for=condition=ready pod -n grafana -l app.kubernetes.io/name=grafana --timeout=120s 2>/dev/null; then
        log_success "Grafana is ready"
    else
        log_warn "Grafana may not be ready yet, continuing anyway..."
    fi
    
    # Set up port-forward in background
    log_info "Setting up port-forward on port ${GRAFANA_PORT}..."
    # Kill any existing port-forward on this port
    lsof -ti:${GRAFANA_PORT} 2>/dev/null | xargs kill -9 2>/dev/null || true
    sleep 1
    
    # Start port-forward in background
    kubectl port-forward -n grafana svc/grafana ${GRAFANA_PORT}:3000 >/tmp/grafana-pf.log 2>&1 &
    GRAFANA_PF_PID=$!
    sleep 3
    
    # Wait for port-forward to be ready
    for i in {1..30}; do
        if curl -s http://localhost:${GRAFANA_PORT}/api/health >/dev/null 2>&1; then
            break
        fi
        sleep 1
    done
    
    # Import dashboards if password is available
    if [ -n "$GRAFANA_PASSWORD" ]; then
        log_info "Importing Grafana dashboards..."
        export GRAFANA_USER
        export GRAFANA_PORT
        export GRAFANA_PASSWORD
        "${SCRIPT_DIR}/observability/dashboards.sh" "$NAMESPACE" "$KUBECONFIG_FILE" || {
            log_warn "Dashboard import had issues, continuing..."
        }
    fi
    
    # Build Grafana URL
    GRAFANA_URL="http://localhost:${GRAFANA_PORT}"
    
    echo ""
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${CYAN}  📊 Grafana Dashboard${NC}"
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
    echo -e "${YELLOW}Access Information:${NC}"
    echo -e "  URL: ${CYAN}${GRAFANA_URL}${NC}"
    if [ -n "$GRAFANA_PASSWORD" ]; then
        echo -e "  Username: ${CYAN}${GRAFANA_USER}${NC}"
        echo -e "  Password: ${CYAN}${GRAFANA_PASSWORD}${NC}"
    else
        echo -e "  Username: ${CYAN}${GRAFANA_USER}${NC}"
        echo -e "  Password: ${CYAN}(check Grafana secret in cluster)${NC}"
    fi
    echo ""
    
    # Open browser automatically
    if [ -n "$GRAFANA_PASSWORD" ]; then
        log_info "Opening browser..."
        if command -v xdg-open >/dev/null 2>&1; then
            # Linux
            xdg-open "${GRAFANA_URL}" >/dev/null 2>&1 &
        elif command -v open >/dev/null 2>&1; then
            # macOS
            open "${GRAFANA_URL}" >/dev/null 2>&1 &
        elif command -v start >/dev/null 2>&1; then
            # Windows (Git Bash)
            start "${GRAFANA_URL}" >/dev/null 2>&1 &
        else
            log_warn "Could not detect browser command, please open manually: ${GRAFANA_URL}"
        fi
        echo -e "${GREEN}✓ Browser opened! Login with credentials above.${NC}"
    fi
    echo ""
    echo -e "${YELLOW}Note:${NC} Port-forward is running in background (PID: ${GRAFANA_PF_PID})"
    echo -e "  To stop it: ${CYAN}kill ${GRAFANA_PF_PID}${NC}"
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
echo -e "${YELLOW}Clean up cluster:${NC}"
echo -e "  ${CYAN}./scripts/cluster/destroy.sh${NC}"
echo ""

show_total_time

