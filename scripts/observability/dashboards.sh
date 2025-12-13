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

# Wait for Grafana to be ready
GRAFANA_READY=false
for i in {1..30}; do
    if curl -s -u "admin:${GRAFANA_PASSWORD}" http://localhost:3100/api/health &>/dev/null; then
        GRAFANA_READY=true
        break
    fi
    sleep 1
done

if [ "$GRAFANA_READY" != true ]; then
    log_warn "Grafana not ready after 30s, attempting import anyway..."
fi

# Import dashboards
IMPORTED=0
FAILED=0
if [ -d "$DASHBOARD_DIR" ]; then
    for dashboard in "$DASHBOARD_DIR"/*.json; do
        if [ -f "$dashboard" ]; then
            dashboard_name=$(basename "$dashboard" .json)
            log_info "Importing dashboard: $dashboard_name"
            
            # Try new API endpoint first (Grafana 7+)
            DASHBOARD_JSON=$(cat "$dashboard")
            RESPONSE=$(curl -s -w "\n%{http_code}" -X POST \
                -H "Content-Type: application/json" \
                -u "admin:${GRAFANA_PASSWORD}" \
                -d "{\"dashboard\":${DASHBOARD_JSON},\"overwrite\":true}" \
                http://localhost:3100/api/dashboards/db 2>/dev/null || echo -e "\n000")
            
            HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
            
            if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "201" ]; then
                log_success "  ✓ Imported: $dashboard_name"
                IMPORTED=$((IMPORTED + 1))
            else
                # Try alternative endpoint (older Grafana)
                RESPONSE2=$(curl -s -w "\n%{http_code}" -X POST \
                    -H "Content-Type: application/json" \
                    -u "admin:${GRAFANA_PASSWORD}" \
                    -d @"$dashboard" \
                    http://localhost:3100/api/dashboards/db 2>/dev/null || echo -e "\n000")
                
                HTTP_CODE2=$(echo "$RESPONSE2" | tail -n1)
                if [ "$HTTP_CODE2" = "200" ] || [ "$HTTP_CODE2" = "201" ]; then
                    log_success "  ✓ Imported: $dashboard_name"
                    IMPORTED=$((IMPORTED + 1))
                else
                    log_warn "  ✗ Failed to import: $dashboard_name (HTTP $HTTP_CODE/$HTTP_CODE2)"
                    FAILED=$((FAILED + 1))
                fi
            fi
        fi
    done
    
    echo ""
    if [ $IMPORTED -gt 0 ]; then
        log_success "Successfully imported $IMPORTED dashboard(s)"
    fi
    if [ $FAILED -gt 0 ]; then
        log_warn "$FAILED dashboard(s) failed to import"
    fi
else
    log_warn "Dashboard directory not found: $DASHBOARD_DIR"
fi

# Cleanup port-forward
kill $PF_PID 2>/dev/null || true
wait $PF_PID 2>/dev/null || true

