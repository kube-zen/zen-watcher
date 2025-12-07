#!/bin/bash
#
# Zen Watcher - Dashboard Import Script
#
# Imports Grafana dashboards
#
# Usage:
#   ./scripts/observability/dashboards.sh <namespace> <kubeconfig_file>

set -euo pipefail

# Source utilities
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../utils/common.sh"

NAMESPACE="${1:-zen-system}"
KUBECONFIG_FILE="${2:-${HOME}/.kube/config}"

export KUBECONFIG="${KUBECONFIG_FILE}"

# Get repo root
REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || echo "$(cd "$(dirname "$0")/../.." && pwd)")"
DASHBOARD_DIR="${REPO_ROOT}/config/dashboards"

log_step "Importing Grafana dashboards..."

# Wait for Grafana API
GRAFANA_POD=$(kubectl get pod -n grafana -l app.kubernetes.io/name=grafana -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -z "$GRAFANA_POD" ]; then
    log_warn "Grafana pod not found, skipping dashboard import"
    exit 0
fi

# Get Grafana admin password
GRAFANA_PASSWORD=$(kubectl get secret -n grafana grafana -o jsonpath='{.data.admin-password}' 2>/dev/null | base64 -d 2>/dev/null || echo "admin")

# Port-forward to Grafana
log_info "Setting up port-forward to Grafana..."
kubectl port-forward -n grafana svc/grafana 3100:80 >/tmp/grafana-pf.log 2>&1 &
PF_PID=$!
sleep 5

# Import dashboards
if [ -d "$DASHBOARD_DIR" ]; then
    for dashboard in "$DASHBOARD_DIR"/*.json; do
        if [ -f "$dashboard" ]; then
            dashboard_name=$(basename "$dashboard" .json)
            log_info "Importing dashboard: $dashboard_name"
            
            # Use Grafana API to import dashboard
            curl -s -X POST \
                -H "Content-Type: application/json" \
                -u "admin:${GRAFANA_PASSWORD}" \
                -d @"$dashboard" \
                http://localhost:3100/api/dashboards/db >/dev/null 2>&1 || {
                log_warn "Failed to import dashboard: $dashboard_name"
            }
        fi
    done
    log_success "Dashboards imported"
else
    log_warn "Dashboard directory not found: $DASHBOARD_DIR"
fi

# Cleanup port-forward
kill $PF_PID 2>/dev/null || true

