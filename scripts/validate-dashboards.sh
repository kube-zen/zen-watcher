#!/bin/bash
# Validate Grafana dashboards - check all sources are available
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Source common utilities
source "${SCRIPT_DIR}/utils/common.sh"

# Configuration
NAMESPACE="${ZEN_NAMESPACE:-zen-system}"
GRAFANA_NAMESPACE="${GRAFANA_NAMESPACE:-grafana}"
GRAFANA_USER="${GRAFANA_USER:-zen}"

# Expected sources (11 total)
EXPECTED_SOURCES=(
    "trivy"
    "falco"
    "kyverno"
    "checkov"
    "kubebench"
    "audit"
    "cert-manager"
    "sealed-secrets"
    "kubernetes-events"
    "prometheus"
    "opa-gatekeeper"
)

# Expected dashboards (6 total)
EXPECTED_DASHBOARDS=(
    "zen-watcher-executive"
    "zen-watcher-operations"
    "zen-watcher-security"
    "zen-watcher-main"
    "zen-watcher-namespace-health"
    "zen-watcher-explorer"
)

log_step "Validating Grafana dashboards and sources..."

# Get Grafana password
GRAFANA_PASSWORD=""
if kubectl get secret -n "$GRAFANA_NAMESPACE" grafana -o jsonpath='{.data.admin-password}' 2>/dev/null | base64 -d 2>/dev/null > /tmp/grafana-password.txt 2>/dev/null; then
    GRAFANA_PASSWORD=$(cat /tmp/grafana-password.txt 2>/dev/null || echo "")
    rm -f /tmp/grafana-password.txt 2>/dev/null || true
fi

# Try alternative secret name
if [ -z "$GRAFANA_PASSWORD" ]; then
    GRAFANA_PASSWORD=$(kubectl get secret -n "$GRAFANA_NAMESPACE" -l app.kubernetes.io/name=grafana -o jsonpath='{.items[0].data.admin-password}' 2>/dev/null | base64 -d 2>/dev/null || echo "")
fi

# Fallback to default
if [ -z "$GRAFANA_PASSWORD" ]; then
    log_warn "Could not retrieve Grafana password, using default"
    GRAFANA_PASSWORD="admin"
fi

# Get ingress port
INGRESS_PORT="8080"
if command -v k3d >/dev/null 2>&1 && k3d cluster list 2>/dev/null | grep -q "zen-demo"; then
    LB_CONTAINER=$(docker ps -q --filter "name=k3d-zen-demo-serverlb" 2>/dev/null)
    if [ -n "$LB_CONTAINER" ]; then
        K3D_PORT=$(docker port "$LB_CONTAINER" 2>/dev/null | grep "80/tcp" | awk -F: '{print $2}' | head -1)
        if [ -n "$K3D_PORT" ] && [ "$K3D_PORT" != "0" ] && [ "$K3D_PORT" -gt 0 ] 2>/dev/null; then
            INGRESS_PORT="$K3D_PORT"
        fi
    fi
fi

GRAFANA_URL="http://localhost:${INGRESS_PORT}/grafana"

# Wait for Grafana to be ready
log_info "Waiting for Grafana to be ready..."
for i in {1..60}; do
    if curl -sL -u "${GRAFANA_USER}:${GRAFANA_PASSWORD}" "${GRAFANA_URL}/api/health" 2>/dev/null | grep -q "database\|ok"; then
        log_success "Grafana is ready"
        break
    fi
    sleep 2
done

# Get dashboard list
log_step "Fetching dashboard list..."
DASHBOARDS=$(curl -sL -u "${GRAFANA_USER}:${GRAFANA_PASSWORD}" "${GRAFANA_URL}/api/search?type=dash-db" 2>/dev/null | jq -r '.[].uid' 2>/dev/null || echo "")

if [ -z "$DASHBOARDS" ]; then
    log_error "Could not fetch dashboards from Grafana"
    exit 1
fi

# Validate dashboards exist
log_step "Validating dashboards exist..."
MISSING_DASHBOARDS=()
for dashboard in "${EXPECTED_DASHBOARDS[@]}"; do
    if echo "$DASHBOARDS" | grep -q "$dashboard"; then
        log_success "Dashboard found: $dashboard"
    else
        log_error "Dashboard missing: $dashboard"
        MISSING_DASHBOARDS+=("$dashboard")
    fi
done

# Get observations by source
log_step "Checking observations by source..."
OBSERVATIONS_BY_SOURCE=$(kubectl get observations -n "$NAMESPACE" -o jsonpath='{range .items[*]}{.spec.source}{"\n"}{end}' 2>/dev/null | sort | uniq -c | sort -rn || echo "")

# Validate sources have observations
log_step "Validating sources have observations..."
MISSING_SOURCES=()
for source in "${EXPECTED_SOURCES[@]}"; do
    count=$(echo "$OBSERVATIONS_BY_SOURCE" | grep -E "^\s*[0-9]+\s+${source}$" | awk '{print $1}' || echo "0")
    if [ "$count" -gt 0 ]; then
        log_success "Source '$source' has $count observations"
    else
        log_warn "Source '$source' has no observations"
        MISSING_SOURCES+=("$source")
    fi
done

# Validate dashboard panels (check for "No data" issues)
log_step "Validating dashboard panels..."
PANEL_ERRORS=0
for dashboard in "${EXPECTED_DASHBOARDS[@]}"; do
    DASHBOARD_UID=$(echo "$DASHBOARDS" | grep "$dashboard" | head -1)
    if [ -z "$DASHBOARD_UID" ]; then
        continue
    fi
    
    DASHBOARD_JSON=$(curl -sL -u "${GRAFANA_USER}:${GRAFANA_PASSWORD}" "${GRAFANA_URL}/api/dashboards/uid/${DASHBOARD_UID}" 2>/dev/null | jq '.dashboard' 2>/dev/null || echo "")
    
    if [ -z "$DASHBOARD_JSON" ]; then
        log_warn "Could not fetch dashboard: $dashboard"
        continue
    fi
    
    # Check for panels with "active tools" queries
    PANELS=$(echo "$DASHBOARD_JSON" | jq -r '.panels[]? | select(.targets[]?.expr | contains("tools_active") or contains("events_total")) | .title' 2>/dev/null || echo "")
    
    if [ -n "$PANELS" ]; then
        log_info "Dashboard '$dashboard' has relevant panels"
    fi
done

# Generate report
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  📊 Dashboard Validation Report"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Dashboards Found: $(echo "$DASHBOARDS" | wc -l)"
echo "Expected Dashboards: ${#EXPECTED_DASHBOARDS[@]}"
echo "Missing Dashboards: ${#MISSING_DASHBOARDS[@]}"
if [ ${#MISSING_DASHBOARDS[@]} -gt 0 ]; then
    for db in "${MISSING_DASHBOARDS[@]}"; do
        echo "  - $db"
    done
fi
echo ""
echo "Sources with Observations:"
echo "$OBSERVATIONS_BY_SOURCE"
echo ""
echo "Expected Sources: ${#EXPECTED_SOURCES[@]}"
echo "Sources Missing Observations: ${#MISSING_SOURCES[@]}"
if [ ${#MISSING_SOURCES[@]} -gt 0 ]; then
    for src in "${MISSING_SOURCES[@]}"; do
        echo "  - $src"
    done
fi
echo ""
echo "Total Observations: $(kubectl get observations -n "$NAMESPACE" --no-headers 2>/dev/null | wc -l)"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Validation result
if [ ${#MISSING_DASHBOARDS[@]} -eq 0 ] && [ ${#MISSING_SOURCES[@]} -eq 0 ]; then
    log_success "✅ All dashboards and sources validated successfully!"
    exit 0
else
    log_warn "⚠️  Some dashboards or sources are missing"
    exit 1
fi

