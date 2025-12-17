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
#   --offline                  Skip Helm repo updates (for air-gapped environments)
#   --skip-repo-update         Skip Helm repo updates (repos must already exist)

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
OFFLINE_MODE=false
SKIP_REPO_UPDATE=false

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
        --offline)
            OFFLINE_MODE=true
            SKIP_REPO_UPDATE=true
            ;;
        --skip-repo-update)
            SKIP_REPO_UPDATE=true
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

# Get repo root (calculate from script directory, not git - makes it work standalone)
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "$REPO_ROOT" || exit 1

# Check if helmfile.yaml.gotmpl exists
if [ ! -f "${SCRIPT_DIR}/helmfile.yaml.gotmpl" ]; then
    log_error "Helmfile configuration not found at ${SCRIPT_DIR}/helmfile.yaml.gotmpl"
    exit 1
fi

# Install CRDs manually before helmfile sync to avoid ownership conflicts
# NOTE: For quick-demo, CRDs are installed via Helm (crds.install: true in helmfile.yaml.gotmpl)
# This manual installation is skipped when using Helm to avoid conflicts
log_info "Skipping manual CRD installation - Helm will install CRDs via helmfile (crds.install: true)"
CRD_DIR="${REPO_ROOT}/deployments/crds"
# Manual CRD installation disabled - Helm installs CRDs via helmfile.yaml.gotmpl (crds.install: true)
SKIP_MANUAL_CRD_INSTALL=true

# Manual CRD installation is disabled - Helm installs CRDs via helmfile.yaml.gotmpl (crds.install: true)
# However, we need to clean up any existing CRDs that don't have Helm ownership to avoid conflicts
log_info "Cleaning up existing CRDs without Helm ownership..."
if [ -d "$CRD_DIR" ]; then
    for crd_file in "${CRD_DIR}"/*_crd.yaml; do
        if [ -f "$crd_file" ]; then
            crd_name=$(grep "^  name:" "$crd_file" | head -1 | awk '{print $2}' || grep "^name:" "$crd_file" | head -1 | awk '{print $2}')
            if [ -n "$crd_name" ] && kubectl get crd "$crd_name" >/dev/null 2>&1; then
                # Check if CRD has Helm ownership
                if ! kubectl get crd "$crd_name" -o jsonpath='{.metadata.labels.app\.kubernetes\.io/managed-by}' 2>/dev/null | grep -q "Helm"; then
                    log_info "Removing CRD $crd_name without Helm ownership (Helm will reinstall it)..."
                    kubectl delete crd "$crd_name" --ignore-not-found=true >/dev/null 2>&1 || true
                    # Wait for CRD to be fully deleted
                    for i in {1..30}; do
                        if ! kubectl get crd "$crd_name" >/dev/null 2>&1; then
                            break
                        fi
                        sleep 1
                    done
                fi
            fi
        fi
    done
fi

log_info "CRDs will be installed by Helm via helmfile (crds.install: true)"

# Add Helm repositories
if [ "$OFFLINE_MODE" = true ]; then
    log_info "Offline mode: Skipping Helm repository setup (repos must be pre-configured)"
    log_info "Required Helm repositories:"
    log_info "  - ingress-nginx: https://kubernetes.github.io/ingress-nginx"
    log_info "  - vm: https://victoriametrics.github.io/helm-charts"
    log_info "  - grafana: https://grafana.github.io/helm-charts"
    log_info "  - aqua: https://aquasecurity.github.io/helm-charts"
    log_info "  - falcosecurity: https://falcosecurity.github.io/charts"
    log_info "  - kyverno: https://kyverno.github.io/kyverno/"
    log_info "  - kube-zen: https://kube-zen.github.io/helm-charts"
else
    log_info "Ensuring Helm repositories are available..."
    helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx 2>&1 | grep -v "already exists" > /dev/null || true
    helm repo add vm https://victoriametrics.github.io/helm-charts 2>&1 | grep -v "already exists" > /dev/null || true
    helm repo add grafana https://grafana.github.io/helm-charts 2>&1 | grep -v "already exists" > /dev/null || true
    helm repo add aqua https://aquasecurity.github.io/helm-charts 2>&1 | grep -v "already exists" > /dev/null || true
    helm repo add falcosecurity https://falcosecurity.github.io/charts 2>&1 | grep -v "already exists" > /dev/null || true
    helm repo add kyverno https://kyverno.github.io/kyverno/ 2>&1 | grep -v "already exists" > /dev/null || true
    helm repo add kube-zen https://kube-zen.github.io/helm-charts 2>&1 | grep -v "already exists" > /dev/null || true
    
    # Update repos only if not skipped
    if [ "$SKIP_REPO_UPDATE" = false ]; then
        log_info "Updating Helm repositories..."
        helm repo update > /dev/null 2>&1 || {
            log_warn "Helm repo update failed (non-fatal, continuing with cached charts)"
        }
    else
        log_info "Skipping Helm repo update (--skip-repo-update)"
    fi
fi

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

