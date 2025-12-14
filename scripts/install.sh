#!/bin/bash
#
# Zen Watcher - Installation Script
#
# Installs Zen Watcher and all components (cluster, tools, observability)
# This is the main installation orchestrator
#
# Usage:
#   ./scripts/install.sh [platform] [options]
#
# Options:
#   --skip-monitoring          Skip observability stack
#   --install-trivy            Install Trivy
#   --install-falco            Install Falco
#   --install-kyverno          Install Kyverno
#   --install-checkov          Install Checkov
#   --install-kube-bench       Install kube-bench
#   --no-docker-login          Don't use docker login credentials

set -euo pipefail

# Source common utilities
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/utils/common.sh"

# Parse arguments
PLATFORM="k3d"
SKIP_MONITORING=false
INSTALL_TRIVY=false
INSTALL_FALCO=false
INSTALL_KYVERNO=false
INSTALL_CHECKOV=false
INSTALL_KUBE_BENCH=false
NO_DOCKER_LOGIN=false
USE_EXISTING_CLUSTER=false

for arg in "$@"; do
    case "$arg" in
        --skip-monitoring|--skip-observability)
            SKIP_MONITORING=true
            ;;
        --install-trivy)
            INSTALL_TRIVY=true
            ;;
        --install-falco)
            INSTALL_FALCO=true
            ;;
        --install-kyverno)
            INSTALL_KYVERNO=true
            ;;
        --install-checkov)
            INSTALL_CHECKOV=true
            ;;
        --install-kube-bench)
            INSTALL_KUBE_BENCH=true
            ;;
        --no-docker-login)
            NO_DOCKER_LOGIN=true
            ;;
        --use-existing|--use-existing-cluster)
            USE_EXISTING_CLUSTER=true
            ;;
        k3d|kind|minikube)
            PLATFORM="$arg"
            ;;
    esac
done

# Configuration
CLUSTER_NAME="${ZEN_CLUSTER_NAME:-zen-demo}"
NAMESPACE="${ZEN_NAMESPACE:-zen-system}"

# Default: install all tools if none specified
if [ "$INSTALL_TRIVY" = false ] && [ "$INSTALL_FALCO" = false ] && \
   [ "$INSTALL_KYVERNO" = false ] && [ "$INSTALL_CHECKOV" = false ] && \
   [ "$INSTALL_KUBE_BENCH" = false ]; then
    INSTALL_TRIVY=true
    INSTALL_FALCO=true
    INSTALL_KYVERNO=true
    INSTALL_CHECKOV=true
    INSTALL_KUBE_BENCH=true
    log_info "No security tools specified - installing all tools for comprehensive demo"
fi

echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  Zen Watcher - Installation${NC}"
echo -e "${BLUE}  Platform: ${CYAN}${PLATFORM}${NC}"
echo -e "${BLUE}  Cluster: ${CYAN}${CLUSTER_NAME}${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Check prerequisites
log_step "Checking prerequisites..."
source "${SCRIPT_DIR}/cluster/utils.sh"
check_command "kubectl" "https://kubernetes.io/docs/tasks/tools/"
check_command "helm" "https://helm.sh/docs/intro/install/"
check_command "helmfile" "https://helmfile.readthedocs.io/en/latest/#installation"
check_command "jq" "https://stedolan.github.io/jq/download/"
check_command "openssl" "https://www.openssl.org/"

case "$PLATFORM" in
    k3d)
        check_command "k3d" "https://k3d.io/#installation"
        ;;
    kind)
        check_command "kind" "https://kind.sigs.k8s.io/docs/user/quick-start/#installation"
        ;;
    minikube)
        check_command "minikube" "https://minikube.sigs.k8s.io/docs/start/"
        ;;
    *)
        log_error "Unknown platform: $PLATFORM"
        echo "  Supported: k3d, kind, minikube"
        exit 1
        ;;
esac

# Create cluster (if needed)
if ! cluster_exists "$PLATFORM" "$CLUSTER_NAME"; then
    log_step "Creating cluster..."
    CREATE_ARGS=()
    if [ "$USE_EXISTING_CLUSTER" = true ]; then
        CREATE_ARGS+=("--use-existing")
    fi
    if [ "$NO_DOCKER_LOGIN" = true ]; then
        CREATE_ARGS+=("--no-docker-login")
    fi
    "${SCRIPT_DIR}/cluster/create.sh" "$PLATFORM" "$CLUSTER_NAME" "${CREATE_ARGS[@]}" || {
        log_error "Failed to create cluster"
        exit 1
    }
    show_section_time "Cluster creation"
else
    log_info "Using existing cluster: $CLUSTER_NAME"
fi

# Setup kubeconfig
KUBECONFIG_FILE=$(get_kubeconfig_path "$PLATFORM" "$CLUSTER_NAME")
setup_kubeconfig "$PLATFORM" "$CLUSTER_NAME" "$KUBECONFIG_FILE"
export KUBECONFIG="$KUBECONFIG_FILE"

# Wait for cluster to be ready
wait_for_cluster "$PLATFORM" "$CLUSTER_NAME" "$KUBECONFIG_FILE" 120

# Install components via Helmfile
log_step "Installing components with Helmfile..."

# Export environment variables for Helmfile
export NAMESPACE=${NAMESPACE}
export ZEN_WATCHER_IMAGE="${ZEN_WATCHER_IMAGE:-kubezen/zen-watcher:latest}"
export GRAFANA_PASSWORD=$(openssl rand -base64 12 | tr -d "=+/" | cut -c1-12)
export INSTALL_TRIVY=${INSTALL_TRIVY}
export INSTALL_FALCO=${INSTALL_FALCO}
export INSTALL_KYVERNO=${INSTALL_KYVERNO}
export INSTALL_KUBE_BENCH=${INSTALL_KUBE_BENCH}
export SKIP_MONITORING=${SKIP_MONITORING}
export IMAGE_PULL_POLICY=$([ "$NO_DOCKER_LOGIN" = true ] && echo "Always" || echo "IfNotPresent")
export ZEN_DEMO_MINIMAL="${ZEN_DEMO_MINIMAL:-false}"

# Get repo root
REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || echo "$(cd "$(dirname "$0")/.." && pwd)")"
cd "$REPO_ROOT" || exit 1

# Check if helmfile.yaml.gotmpl exists
if [ ! -f "${SCRIPT_DIR}/helmfile.yaml.gotmpl" ]; then
    log_error "Helmfile configuration not found at ${SCRIPT_DIR}/helmfile.yaml.gotmpl"
    exit 1
fi

# Ensure CRDs are properly handled before helmfile sync
# Delete existing CRDs if they exist without proper Helm ownership to avoid conflicts
log_info "Preparing CRDs for Helm management..."
for crd in ingesters.zen.kube-zen.io observations.zen.kube-zen.io; do
    if kubectl get crd "$crd" >/dev/null 2>&1; then
        # Check if CRD has Helm ownership metadata
        if ! kubectl get crd "$crd" -o jsonpath='{.metadata.labels.app\.kubernetes\.io/managed-by}' 2>/dev/null | grep -q "Helm"; then
            log_info "Removing CRD $crd without Helm ownership to allow clean installation..."
            kubectl delete crd "$crd" --ignore-not-found=true >/dev/null 2>&1 || true
            sleep 1
        fi
    fi
done

# Add Helm repositories
log_info "Ensuring Helm repositories are available..."
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx 2>&1 | grep -v "already exists" > /dev/null || true
helm repo add vm https://victoriametrics.github.io/helm-charts 2>&1 | grep -v "already exists" > /dev/null || true
helm repo add grafana https://grafana.github.io/helm-charts 2>&1 | grep -v "already exists" > /dev/null || true
helm repo add aqua https://aquasecurity.github.io/helm-charts 2>&1 | grep -v "already exists" > /dev/null || true
helm repo add falcosecurity https://falcosecurity.github.io/charts 2>&1 | grep -v "already exists" > /dev/null || true
helm repo add kyverno https://kyverno.github.io/kyverno/ 2>&1 | grep -v "already exists" > /dev/null || true
helm repo add kube-zen https://kube-zen.github.io/helm-charts 2>&1 | grep -v "already exists" > /dev/null || true
helm repo update > /dev/null 2>&1 || true

# Run helmfile sync
if helmfile -f "${SCRIPT_DIR}/helmfile.yaml.gotmpl" --quiet sync 2>&1 | tee /tmp/helmfile-sync.log; then
    log_success "Helmfile sync completed"
else
    HELMFILE_EXIT=$?
    if grep -q "already exists" /tmp/helmfile-sync.log; then
        log_warn "Some repositories already exist (non-fatal)"
    else
        log_warn "Helmfile sync had errors (exit code: $HELMFILE_EXIT)"
        log_info "Check logs: cat /tmp/helmfile-sync.log"
    fi
fi

# Delete ingress admission webhooks (they cause TLS issues)
sleep 2
kubectl delete validatingwebhookconfiguration ingress-nginx-admission 2>&1 | grep -v "not found" > /dev/null || true
kubectl delete mutatingwebhookconfiguration ingress-nginx-admission 2>&1 | grep -v "not found" > /dev/null || true

# Setup observability (if not skipped)
if [ "$SKIP_MONITORING" != true ]; then
    log_step "Setting up observability..."
    
    # Wait for grafana namespace to be created by helmfile, then create dashboard ConfigMap
    log_info "Waiting for Grafana namespace..."
    for i in {1..30}; do
        if kubectl get namespace grafana >/dev/null 2>&1; then
            break
        fi
        sleep 1
    done
    
    # Create Grafana dashboard ConfigMaps for permanent provisioning
    # One ConfigMap per dashboard for better modularity and smaller size
    # Use server-side apply to avoid annotation size limits
    if kubectl get namespace grafana >/dev/null 2>&1; then
        log_info "Creating Grafana dashboard ConfigMaps (one per dashboard)..."
        if [ -f "${SCRIPT_DIR}/observability/generate-dashboard-configmap.sh" ]; then
            # Generate and apply all dashboard ConfigMaps
            # Each dashboard gets its own ConfigMap with label grafana_dashboard: "1"
            "${SCRIPT_DIR}/observability/generate-dashboard-configmap.sh" grafana | \
                kubectl apply --server-side --field-manager=helmfile --force-conflicts -f - 2>&1 | \
                grep -E "configmap/grafana-dashboard|created|serverside-applied" | \
                sed 's/^/  /' || {
                log_warn "Some dashboard ConfigMaps may have had issues, continuing..."
            }
            DASHBOARD_COUNT=$(kubectl get configmap -n grafana -l grafana_dashboard=1 --no-headers 2>/dev/null | wc -l || echo "0")
            log_success "Created ${DASHBOARD_COUNT} dashboard ConfigMap(s) (dashboards will be automatically provisioned)"
        else
            log_warn "Dashboard ConfigMap generator script not found, skipping..."
        fi
    else
        log_warn "Grafana namespace not found, will create ConfigMaps later..."
    fi
    
    "${SCRIPT_DIR}/observability/setup.sh" "$NAMESPACE" "$KUBECONFIG_FILE" || {
        log_warn "Observability setup had issues, continuing..."
    }
    show_section_time "Observability setup"
fi

log_success "Installation complete!"
show_total_time

