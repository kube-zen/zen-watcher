#!/bin/bash
#
# Zen Watcher - Observability Setup Script
#
# Sets up VictoriaMetrics and Grafana (already installed via Helmfile)
# Imports dashboards
#
# Usage:
#   ./scripts/observability/setup.sh <namespace> <kubeconfig_file>

set -euo pipefail

# Source utilities
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../utils/common.sh"

NAMESPACE="${1:-zen-system}"
KUBECONFIG_FILE="${2:-${HOME}/.kube/config}"

export KUBECONFIG="${KUBECONFIG_FILE}"

log_step "Setting up observability..."

# Wait for Grafana to be ready
log_info "Waiting for Grafana to be ready..."
kubectl wait --for=condition=ready pod -n grafana -l app.kubernetes.io/name=grafana --timeout=120s 2>/dev/null || {
    log_warn "Grafana may not be ready yet, continuing..."
}

# Import dashboards
log_step "Importing Grafana dashboards..."
"${SCRIPT_DIR}/dashboards.sh" "$NAMESPACE" "$KUBECONFIG_FILE" || {
    log_warn "Dashboard import had issues, continuing..."
}

log_success "Observability setup complete"

