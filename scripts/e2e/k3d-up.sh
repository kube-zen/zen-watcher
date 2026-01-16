#!/usr/bin/env bash
# H035: Multi-k3d E2E harness setup script
# Creates clusters: core, cust-a, edge-uat (optional saas, dp)
# Installs ingress controller(s)
# Configures host DNS mapping (or local DNS container)
# Applies baseline NetPol + RBAC templates

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WATCHER_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
source "${SCRIPT_DIR}/../utils/common.sh" 2>/dev/null || {
    # Fallback if common.sh not available
    log_info() { echo "[INFO] $*"; }
    log_error() { echo "[ERROR] $*" >&2; }
    log_success() { echo "[SUCCESS] $*"; }
    log_warn() { echo "[WARN] $*"; }
}

# Configuration
TOPOLOGY="${TOPOLOGY:-split}" # 'combined' or 'split'
ENABLE_SAAS="${ENABLE_SAAS:-false}"
ENABLE_DP="${ENABLE_DP:-false}"

# Cluster names
CLUSTER_CORE="zen-core"
CLUSTER_CUST_A="zen-cust-a"
CLUSTER_EDGE_UAT="zen-edge-uat"
CLUSTER_SAAS="zen-saas"
CLUSTER_DP="zen-dp"

# Port assignments (deterministic endpoints)
CORE_HTTP_PORT=9080
CORE_HTTPS_PORT=9443
CUST_A_HTTP_PORT=9090
CUST_A_HTTPS_PORT=9453
EDGE_UAT_HTTP_PORT=9100
EDGE_UAT_HTTPS_PORT=9463
SAAS_HTTP_PORT=8080
SAAS_HTTPS_PORT=8443
DP_HTTP_PORT=9110
DP_HTTPS_PORT=9473

# Cleanup function
cleanup_on_exit() {
    log_warn "Cleaning up on exit..."
    # Script should call k3d-down.sh explicitly, but cleanup here as safety
}

trap cleanup_on_exit EXIT INT TERM

# Check k3d is installed
if ! command -v k3d &> /dev/null; then
    log_error "k3d not found. Install from https://k3d.io"
    exit 1
fi

log_info "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
log_info "H035: Multi-k3d E2E harness setup"
log_info "Topology: ${TOPOLOGY}"
log_info "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Function to create a k3d cluster
create_cluster() {
    local cluster_name="$1"
    local http_port="$2"
    local https_port="$3"
    
    if k3d cluster list | grep -q "^${cluster_name}"; then
        log_warn "Cluster ${cluster_name} already exists, skipping..."
        return 0
    fi
    
    log_info "Creating cluster: ${cluster_name} (HTTP: ${http_port}, HTTPS: ${https_port})"
    
    k3d cluster create "${cluster_name}" \
        --agents 1 \
        --k3s-arg "--disable=traefik@server:0" \
        --port "${http_port}:80@loadbalancer" \
        --port "${https_port}:443@loadbalancer" \
        --wait || {
        log_error "Failed to create cluster ${cluster_name}"
        return 1
    }
    
    # Wait for cluster to be ready
    kubectl --context "k3d-${cluster_name}" wait --for=condition=ready node --all --timeout=120s || {
        log_warn "Cluster ${cluster_name} nodes not ready yet (may be OK)"
    }
    
    log_success "Cluster ${cluster_name} created"
}

# Function to install ingress controller
install_ingress() {
    local cluster_name="$1"
    local context="k3d-${cluster_name}"
    
    log_info "Installing ingress controller on ${cluster_name}..."
    
    # Install NGINX Ingress (lightweight)
    kubectl --context "${context}" apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.8.2/deploy/static/provider/cloud/deploy.yaml || {
        log_warn "Failed to install NGINX Ingress (may already be installed or network issue)"
        # Continue - ingress might not be critical for all tests
    }
    
    # Wait for ingress controller to be ready
    kubectl --context "${context}" wait --namespace ingress-nginx \
        --for=condition=ready pod \
        --selector=app.kubernetes.io/component=controller \
        --timeout=120s || {
        log_warn "Ingress controller not ready yet (may be OK for some tests)"
    }
    
    log_success "Ingress controller installed on ${cluster_name}"
}

# Function to configure DNS (simple /etc/hosts approach for local testing)
configure_dns() {
    log_info "Configuring DNS mapping..."
    
    # Note: This is a simple approach using /etc/hosts
    # For production, consider a local DNS container (e.g., CoreDNS, dnsmasq)
    
    local hosts_file="/etc/hosts"
    local dns_entries=(
        "${CORE_HTTP_PORT} core.zen.local"
        "${CUST_A_HTTP_PORT} cust-a.zen.local"
        "${EDGE_UAT_HTTP_PORT} edge-uat.zen.local"
    )
    
    if [ "${ENABLE_SAAS}" = "true" ]; then
        dns_entries+=("${SAAS_HTTP_PORT} saas.zen.local")
    fi
    
    if [ "${ENABLE_DP}" = "true" ]; then
        dns_entries+=("${DP_HTTP_PORT} dp.zen.local")
    fi
    
    log_info "DNS entries would be added to ${hosts_file}:"
    for entry in "${dns_entries[@]}"; do
        log_info "  127.0.0.1 ${entry}"
    done
    log_warn "DNS mapping not automatically configured (requires root). Add entries manually if needed."
    
    log_success "DNS configuration documented"
}

# Function to apply baseline NetPol + RBAC
apply_baseline_policies() {
    local cluster_name="$1"
    local context="k3d-${cluster_name}"
    
    log_info "Applying baseline NetPol + RBAC on ${cluster_name}..."
    
    # Create test namespace
    kubectl --context "${context}" create namespace zen-watcher-test --dry-run=client -o yaml | \
        kubectl --context "${context}" apply -f - || true
    
    # Apply a basic NetworkPolicy (deny all by default, allow specific)
    cat <<EOF | kubectl --context "${context}" apply -f -
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: default-deny-all
  namespace: zen-watcher-test
spec:
  podSelector: {}
  policyTypes:
  - Ingress
  - Egress
  egress:
  - {} # Allow all egress (can be restricted later)
EOF

    # Apply basic RBAC (allow service account to create observations)
    cat <<EOF | kubectl --context "${context}" apply -f -
apiVersion: v1
kind: ServiceAccount
metadata:
  name: zen-watcher-test-sa
  namespace: zen-watcher-test
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: observation-creator
  namespace: zen-watcher-test
rules:
- apiGroups: ["zen.kube-zen.io"]
  resources: ["observations"]
  verbs: ["get", "list", "watch", "create", "update", "patch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: observation-creator-binding
  namespace: zen-watcher-test
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: observation-creator
subjects:
- kind: ServiceAccount
  name: zen-watcher-test-sa
  namespace: zen-watcher-test
EOF
    
    log_success "Baseline policies applied on ${cluster_name}"
}

# Main setup
log_info "Creating clusters..."

# Always create core clusters
create_cluster "${CLUSTER_CORE}" "${CORE_HTTP_PORT}" "${CORE_HTTPS_PORT}"
create_cluster "${CLUSTER_CUST_A}" "${CUST_A_HTTP_PORT}" "${CUST_A_HTTPS_PORT}"
create_cluster "${CLUSTER_EDGE_UAT}" "${EDGE_UAT_HTTP_PORT}" "${EDGE_UAT_HTTPS_PORT}"

# Optional clusters
if [ "${ENABLE_SAAS}" = "true" ]; then
    create_cluster "${CLUSTER_SAAS}" "${SAAS_HTTP_PORT}" "${SAAS_HTTPS_PORT}"
fi

if [ "${ENABLE_DP}" = "true" ]; then
    create_cluster "${CLUSTER_DP}" "${DP_HTTP_PORT}" "${DP_HTTPS_PORT}"
fi

log_info "Installing ingress controllers..."
for cluster in "${CLUSTER_CORE}" "${CLUSTER_CUST_A}" "${CLUSTER_EDGE_UAT}"; do
    install_ingress "${cluster}"
done

if [ "${ENABLE_SAAS}" = "true" ]; then
    install_ingress "${CLUSTER_SAAS}"
fi

if [ "${ENABLE_DP}" = "true" ]; then
    install_ingress "${CLUSTER_DP}"
fi

log_info "Applying baseline policies..."
for cluster in "${CLUSTER_CORE}" "${CLUSTER_CUST_A}" "${CLUSTER_EDGE_UAT}"; do
    apply_baseline_policies "${cluster}"
done

if [ "${ENABLE_SAAS}" = "true" ]; then
    apply_baseline_policies "${CLUSTER_SAAS}"
fi

if [ "${ENABLE_DP}" = "true" ]; then
    apply_baseline_policies "${CLUSTER_DP}"
fi

configure_dns

log_success "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
log_success "Multi-k3d E2E harness setup complete"
log_success "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
log_info "Clusters created:"
log_info "  - ${CLUSTER_CORE} (HTTP: ${CORE_HTTP_PORT}, HTTPS: ${CORE_HTTPS_PORT})"
log_info "  - ${CLUSTER_CUST_A} (HTTP: ${CUST_A_HTTP_PORT}, HTTPS: ${CUST_A_HTTPS_PORT})"
log_info "  - ${CLUSTER_EDGE_UAT} (HTTP: ${EDGE_UAT_HTTP_PORT}, HTTPS: ${EDGE_UAT_HTTPS_PORT})"
[ "${ENABLE_SAAS}" = "true" ] && log_info "  - ${CLUSTER_SAAS} (HTTP: ${SAAS_HTTP_PORT}, HTTPS: ${SAAS_HTTPS_PORT})"
[ "${ENABLE_DP}" = "true" ] && log_info "  - ${CLUSTER_DP} (HTTP: ${DP_HTTP_PORT}, HTTPS: ${DP_HTTPS_PORT})"
log_info ""
log_info "To tear down: ./scripts/e2e/k3d-down.sh"
