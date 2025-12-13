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

# Get Grafana admin password and username
GRAFANA_PASSWORD=$(kubectl get secret -n grafana grafana -o jsonpath='{.data.admin-password}' 2>/dev/null | base64 -d 2>/dev/null || echo "admin")
GRAFANA_USER="${GRAFANA_USER:-zen}"

# Determine Grafana URL - prefer ingress URL if available
GRAFANA_BASE_URL="${GRAFANA_BASE_URL:-}"
INGRESS_PORT="${INGRESS_PORT:-}"

if [ -z "$GRAFANA_BASE_URL" ]; then
    # Try to detect ingress port from k3d loadbalancer
    if [ -n "$INGRESS_PORT" ]; then
        GRAFANA_BASE_URL="http://localhost:${INGRESS_PORT}/grafana"
    else
        # Fallback: try to detect from k3d
        CLUSTER_NAME=$(kubectl config view --minify -o jsonpath='{.clusters[0].name}' 2>/dev/null | sed 's/k3d-//' || echo "")
        if [ -n "$CLUSTER_NAME" ]; then
            LB_CONTAINER="k3d-${CLUSTER_NAME}-serverlb"
            if docker inspect "$LB_CONTAINER" >/dev/null 2>&1; then
                DETECTED_PORT=$(docker port "$LB_CONTAINER" 2>/dev/null | grep "80/tcp" | awk -F: '{print $2}' | head -1)
                if [ -n "$DETECTED_PORT" ] && [ "$DETECTED_PORT" != "0" ] && [ "$DETECTED_PORT" -gt 0 ] 2>/dev/null; then
                    GRAFANA_BASE_URL="http://localhost:${DETECTED_PORT}/grafana"
                    log_info "Detected ingress port: ${DETECTED_PORT}"
                fi
            fi
        fi
    fi
fi

# Final fallback: use port-forward on default port
if [ -z "$GRAFANA_BASE_URL" ]; then
    GRAFANA_PORT="${GRAFANA_PORT:-8080}"
    GRAFANA_BASE_URL="http://localhost:${GRAFANA_PORT}"
    USE_EXISTING_PF=false
    
    if curl -s "${GRAFANA_BASE_URL}/api/health" >/dev/null 2>&1; then
        log_info "Using existing connection on ${GRAFANA_BASE_URL}"
        USE_EXISTING_PF=true
    else
        # Port-forward to Grafana
        log_info "Setting up port-forward to Grafana on port ${GRAFANA_PORT}..."
        kubectl port-forward -n grafana svc/grafana ${GRAFANA_PORT}:3000 >/tmp/grafana-pf.log 2>&1 &
        PF_PID=$!
        sleep 5
    fi
else
    USE_EXISTING_PF=true
    log_info "Using ingress URL: ${GRAFANA_BASE_URL}"
fi

# Wait for Grafana to be ready
GRAFANA_READY=false
for i in {1..60}; do
    if curl -sL -u "${GRAFANA_USER:-zen}:${GRAFANA_PASSWORD}" "${GRAFANA_BASE_URL}/api/health" 2>/dev/null | grep -q "database\|ok"; then
        GRAFANA_READY=true
        break
    fi
    sleep 1
done

if [ "$GRAFANA_READY" != true ]; then
    log_warn "Grafana not ready after 60s, attempting import anyway..."
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
            RESPONSE=$(curl -sL -w "\n%{http_code}" -X POST \
                -H "Content-Type: application/json" \
                -u "${GRAFANA_USER}:${GRAFANA_PASSWORD}" \
                -d "{\"dashboard\":${DASHBOARD_JSON},\"overwrite\":true}" \
                "${GRAFANA_BASE_URL}/api/dashboards/db" 2>/dev/null || echo -e "\n000")
            
            HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
            
            if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "201" ]; then
                log_success "  ✓ Imported: $dashboard_name"
                IMPORTED=$((IMPORTED + 1))
            else
                # Try alternative endpoint (older Grafana)
                RESPONSE2=$(curl -sL -w "\n%{http_code}" -X POST \
                    -H "Content-Type: application/json" \
                    -u "${GRAFANA_USER}:${GRAFANA_PASSWORD}" \
                    -d @"$dashboard" \
                    "${GRAFANA_BASE_URL}/api/dashboards/db" 2>/dev/null || echo -e "\n000")
                
                HTTP_CODE2=$(echo "$RESPONSE2" | tail -n1)
                if [ "$HTTP_CODE2" = "200" ] || [ "$HTTP_CODE2" = "201" ]; then
                    log_success "  ✓ Imported: $dashboard_name"
                    IMPORTED=$((IMPORTED + 1))
                else
                    log_warn "  ✗ Failed to import: $dashboard_name (HTTP $HTTP_CODE/$HTTP_CODE2)"
                    if [ "$HTTP_CODE" != "000" ]; then
                        echo "$RESPONSE" | head -5
                    fi
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

# Cleanup port-forward only if we created it
if [ "$USE_EXISTING_PF" != true ] && [ -n "${PF_PID:-}" ]; then
    kill $PF_PID 2>/dev/null || true
    wait $PF_PID 2>/dev/null || true
fi

