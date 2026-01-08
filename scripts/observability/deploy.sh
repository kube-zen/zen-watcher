#!/bin/bash
#
# Zen Watcher - Observability Stack Deployment
#
# Deploys VictoriaMetrics and Grafana stack on any cluster
# This script can be executed independently on any sporadic cluster
#
# Usage:
#   ./scripts/observability/deploy.sh [options]
#   ./scripts/observability/deploy.sh --context k3d-zen-demo
#   KUBECTL_CONTEXT=k3d-zen-demo ./scripts/observability/deploy.sh
#
# Options:
#   --context <context>           Kubernetes context to use
#   --namespace <namespace>       Namespace for zen-watcher (default: zen-system)
#   --grafana-password <pass>     Grafana admin password (default: random)
#   --ingress-port <port>         Ingress port (default: 8080)
#   --offline                     Skip Helm repo updates (for air-gapped environments)
#   --skip-repo-update            Skip Helm repo updates (repos must already exist)

set -euo pipefail

# Source utilities
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../utils/common.sh"

# Parse arguments
KUBECTL_CONTEXT="${KUBECTL_CONTEXT:-}"
NAMESPACE="${NAMESPACE:-zen-system}"
GRAFANA_PASSWORD="${GRAFANA_PASSWORD:-}"
INGRESS_PORT="${INGRESS_PORT:-8080}"
OFFLINE_MODE=false
SKIP_REPO_UPDATE=false

while [[ $# -gt 0 ]]; do
    case "$1" in
        --context)
            KUBECTL_CONTEXT="$2"
            shift 2
            ;;
        --namespace)
            NAMESPACE="$2"
            shift 2
            ;;
        --grafana-password)
            GRAFANA_PASSWORD="$2"
            shift 2
            ;;
        --ingress-port)
            INGRESS_PORT="$2"
            shift 2
            ;;
        --offline)
            OFFLINE_MODE=true
            SKIP_REPO_UPDATE=true
            shift
            ;;
        --skip-repo-update)
            SKIP_REPO_UPDATE=true
            shift
            ;;
        *)
            log_error "Unknown option: $1"
            echo "Usage: $0 [--context <context>] [--namespace <namespace>] [--grafana-password <pass>] [--ingress-port <port>] [--offline] [--skip-repo-update]"
            exit 1
            ;;
    esac
done

# Build kubectl command with context if provided
KUBECTL_CMD="kubectl"
if [ -n "$KUBECTL_CONTEXT" ]; then
    KUBECTL_CMD="kubectl --context=${KUBECTL_CONTEXT}"
    export KUBECONFIG=""
else
    # Use current kubeconfig
    export KUBECONFIG="${KUBECONFIG:-${HOME}/.kube/config}"
fi

# Verify cluster is accessible
if ! $KUBECTL_CMD cluster-info >/dev/null 2>&1; then
    log_error "Cannot access Kubernetes cluster"
    if [ -n "$KUBECTL_CONTEXT" ]; then
        log_error "Context: $KUBECTL_CONTEXT"
    fi
    exit 1
fi

log_step "Deploying observability stack..."
if [ -n "$KUBECTL_CONTEXT" ]; then
    log_info "Context: $KUBECTL_CONTEXT"
fi
log_info "Namespace: $NAMESPACE"

# Generate Grafana password if not provided
if [ -z "$GRAFANA_PASSWORD" ]; then
    GRAFANA_PASSWORD=$(openssl rand -base64 12 | tr -d "=+/" | cut -c1-12)
    log_info "Generated Grafana password: $GRAFANA_PASSWORD"
fi

# Get repo root
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
cd "$REPO_ROOT" || exit 1

# Check if helmfile.yaml.gotmpl exists
if [ ! -f "${SCRIPT_DIR}/../helmfile.yaml.gotmpl" ]; then
    log_error "Helmfile configuration not found at ${SCRIPT_DIR}/../helmfile.yaml.gotmpl"
    exit 1
fi

# Add Helm repositories (only for observability stack)
if [ "$OFFLINE_MODE" = true ]; then
    log_info "Offline mode: Skipping Helm repository setup (repos must be pre-configured)"
    log_info "Required Helm repositories for observability:"
    log_info "  - ingress-nginx: https://kubernetes.github.io/ingress-nginx"
    log_info "  - vm: https://victoriametrics.github.io/helm-charts"
    log_info "  - grafana: https://grafana.github.io/helm-charts"
else
    log_info "Ensuring Helm repositories are available..."
    helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx 2>&1 | grep -v "already exists" > /dev/null || true
    helm repo add vm https://victoriametrics.github.io/helm-charts 2>&1 | grep -v "already exists" > /dev/null || true
    helm repo add grafana https://grafana.github.io/helm-charts 2>&1 | grep -v "already exists" > /dev/null || true
    
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

# Export environment variables for Helmfile
# Set all tool installations to false (we only want observability)
export NAMESPACE=${NAMESPACE}
export SKIP_MONITORING="false"
export INSTALL_TRIVY="false"
export INSTALL_FALCO="false"
export INSTALL_KYVERNO="false"
export INSTALL_CHECKOV="false"
export INSTALL_KUBE_BENCH="false"
export GRAFANA_PASSWORD=${GRAFANA_PASSWORD}
export INGRESS_HTTP_PORT=${INGRESS_PORT}
export ZEN_DEMO_MINIMAL="${ZEN_DEMO_MINIMAL:-false}"
# We don't need zen-watcher image for observability-only deployment
export ZEN_WATCHER_IMAGE="${ZEN_WATCHER_IMAGE:-kubezen/zen-watcher:latest}"
export IMAGE_PULL_POLICY="IfNotPresent"

# Run helmfile sync (will install only observability stack based on SKIP_MONITORING=false and tool flags=false)
log_step "Installing observability stack with Helmfile..."
if helmfile -f "${SCRIPT_DIR}/../helmfile.yaml.gotmpl" --quiet sync 2>&1 | tee /tmp/helmfile-observability-sync.log; then
    log_success "Helmfile sync completed"
else
    HELMFILE_EXIT=$?
    if grep -q "already exists" /tmp/helmfile-observability-sync.log; then
        log_warn "Some resources already exist (non-fatal)"
    else
        log_warn "Helmfile sync had errors (exit code: $HELMFILE_EXIT)"
        log_info "Check logs: cat /tmp/helmfile-observability-sync.log"
    fi
fi

# Delete ingress admission webhooks (they cause TLS issues)
sleep 2
$KUBECTL_CMD delete validatingwebhookconfiguration ingress-nginx-admission 2>&1 | grep -v "not found" > /dev/null || true
$KUBECTL_CMD delete mutatingwebhookconfiguration ingress-nginx-admission 2>&1 | grep -v "not found" > /dev/null || true

# Wait for grafana namespace to be created by helmfile, then create dashboard ConfigMap
log_info "Waiting for Grafana namespace..."
for i in {1..30}; do
    if $KUBECTL_CMD get namespace grafana >/dev/null 2>&1; then
        break
    fi
    sleep 1
done

# Create Grafana dashboard ConfigMaps for permanent provisioning
if $KUBECTL_CMD get namespace grafana >/dev/null 2>&1; then
    log_step "Creating Grafana dashboard ConfigMaps (one per dashboard)..."
    if [ -f "${SCRIPT_DIR}/generate-dashboard-configmap.sh" ]; then
        # Generate and apply all dashboard ConfigMaps
        # Each dashboard gets its own ConfigMap with label grafana_dashboard: "1"
        "${SCRIPT_DIR}/generate-dashboard-configmap.sh" grafana | \
            $KUBECTL_CMD apply --server-side --field-manager=helmfile --force-conflicts -f - 2>&1 | \
            grep -E "configmap/grafana-dashboard|created|serverside-applied" | \
            sed 's/^/  /' || {
            log_warn "Some dashboard ConfigMaps may have had issues, continuing..."
        }
        DASHBOARD_COUNT=$($KUBECTL_CMD get configmap -n grafana -l grafana_dashboard=1 --no-headers 2>/dev/null | wc -l || echo "0")
        log_success "Created ${DASHBOARD_COUNT} dashboard ConfigMap(s) (dashboards will be automatically provisioned)"
    else
        log_warn "Dashboard ConfigMap generator script not found, skipping..."
    fi
else
    log_warn "Grafana namespace not found, will create ConfigMaps later..."
fi

# Run setup script (waits for Grafana and imports dashboards)
export KUBECTL_CONTEXT="${KUBECTL_CONTEXT:-}"
"${SCRIPT_DIR}/setup.sh" "$NAMESPACE" "${KUBECONFIG:-${HOME}/.kube/config}" || {
    log_warn "Observability setup had issues, continuing..."
}

# Note: Service and pod configuration are handled by Helm/Helmfile values
# The helmfile.yaml.gotmpl has been updated to avoid invalid extraArgs flags

log_success "Observability stack deployment complete!"
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  Observability Stack Deployed${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "${YELLOW}Grafana Credentials:${NC}"
echo -e "  Username: ${CYAN}zen${NC}"
echo -e "  Password: ${CYAN}${GRAFANA_PASSWORD}${NC}"
echo ""
echo -e "${YELLOW}Access Grafana:${NC}"
if [ -n "$KUBECTL_CONTEXT" ]; then
    echo -e "  Context: ${CYAN}${KUBECTL_CONTEXT}${NC}"
fi
echo -e "  Port-forward: ${CYAN}kubectl${KUBECTL_CONTEXT:+ --context=${KUBECTL_CONTEXT}} -n grafana port-forward svc/grafana 3000:3000${NC}"
echo -e "  URL: ${CYAN}http://localhost:3000${NC}"
echo ""

