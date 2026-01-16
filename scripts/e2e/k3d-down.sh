#!/usr/bin/env bash
# H035: Multi-k3d E2E harness teardown script
# Deletes clusters: core, cust-a, edge-uat (optional saas, dp)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../utils/common.sh" 2>/dev/null || {
    log_info() { echo "[INFO] $*"; }
    log_error() { echo "[ERROR] $*" >&2; }
    log_success() { echo "[SUCCESS] $*"; }
    log_warn() { echo "[WARN] $*"; }
}

# Configuration
ENABLE_SAAS="${ENABLE_SAAS:-false}"
ENABLE_DP="${ENABLE_DP:-false}"

# Cluster names
CLUSTER_CORE="zen-core"
CLUSTER_CUST_A="zen-cust-a"
CLUSTER_EDGE_UAT="zen-edge-uat"
CLUSTER_SAAS="zen-saas"
CLUSTER_DP="zen-dp"

log_info "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
log_info "H035: Multi-k3d E2E harness teardown"
log_info "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Function to delete a k3d cluster
delete_cluster() {
    local cluster_name="$1"
    
    if ! k3d cluster list | grep -q "^${cluster_name}"; then
        log_warn "Cluster ${cluster_name} does not exist, skipping..."
        return 0
    fi
    
    log_info "Deleting cluster: ${cluster_name}"
    k3d cluster delete "${cluster_name}" || {
        log_error "Failed to delete cluster ${cluster_name}"
        return 1
    }
    
    log_success "Cluster ${cluster_name} deleted"
}

# Main teardown
log_info "Deleting clusters..."

# Always delete core clusters
delete_cluster "${CLUSTER_CORE}"
delete_cluster "${CLUSTER_CUST_A}"
delete_cluster "${CLUSTER_EDGE_UAT}"

# Optional clusters
if [ "${ENABLE_SAAS}" = "true" ]; then
    delete_cluster "${CLUSTER_SAAS}"
fi

if [ "${ENABLE_DP}" = "true" ]; then
    delete_cluster "${CLUSTER_DP}"
fi

log_success "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
log_success "Multi-k3d E2E harness teardown complete"
log_success "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
