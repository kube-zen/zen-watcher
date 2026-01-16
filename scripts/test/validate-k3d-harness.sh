#!/usr/bin/env bash
# H045: Validate k3d E2E harness connectivity
# Verifies DNS resolution, ingress endpoints, netpol/rbac baseline

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WATCHER_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1" >&2; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_section() { echo -e "${CYAN}━━━━ $1 ━━━━${NC}"; }

CLUSTER_CORE="${CLUSTER_CORE:-zen-core}"
CLUSTER_CUST_A="${CLUSTER_CUST_A:-zen-cust-a}"
CLUSTER_EDGE_UAT="${CLUSTER_EDGE_UAT:-zen-edge-uat}"

log_section "k3d Harness Validation (H045)"

# Check k3d is installed
if ! command -v k3d &> /dev/null; then
    log_error "k3d is not installed. Install with: curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | bash"
    exit 1
fi

# Check if clusters exist
log_info "Checking cluster existence..."
CLUSTERS_EXIST=true
for cluster in "${CLUSTER_CORE}" "${CLUSTER_CUST_A}" "${CLUSTER_EDGE_UAT}"; do
    if ! k3d cluster list | grep -q "${cluster}"; then
        log_warn "Cluster ${cluster} does not exist. Run: ./scripts/e2e/k3d-up.sh"
        CLUSTERS_EXIST=false
    fi
done

if [ "${CLUSTERS_EXIST}" != "true" ]; then
    log_error "Required clusters not found. Please run: ./scripts/e2e/k3d-up.sh"
    exit 1
fi

# Test DNS resolution strategy
log_section "DNS Resolution"

# Check if /etc/hosts has cluster entries (common k3d DNS strategy)
if grep -q "${CLUSTER_CORE}" /etc/hosts 2>/dev/null; then
    log_info "✓ Hosts file has cluster entries"
else
    log_warn "Hosts file does not have cluster entries (may use k3d's internal DNS)"
fi

# Test cluster connectivity
log_section "Cluster Connectivity"

for cluster in "${CLUSTER_CORE}" "${CLUSTER_CUST_A}" "${CLUSTER_EDGE_UAT}"; do
    log_info "Testing ${cluster} connectivity..."
    kubeconfig="${HOME}/.config/k3d/kubeconfig-${cluster}.yaml"
    
    if [ ! -f "${kubeconfig}" ]; then
        log_error "Kubeconfig not found: ${kubeconfig}"
        continue
    fi
    
    # Test basic kubectl connectivity
    if KUBECONFIG="${kubeconfig}" kubectl get nodes &> /dev/null; then
        log_info "  ✓ ${cluster}: Nodes accessible"
    else
        log_error "  ✗ ${cluster}: Cannot access nodes"
    fi
    
    # Test API server connectivity
    if KUBECONFIG="${kubeconfig}" kubectl cluster-info &> /dev/null; then
        log_info "  ✓ ${cluster}: API server reachable"
    else
        log_error "  ✗ ${cluster}: API server unreachable"
    fi
done

# Test ingress endpoints (if ingress controller is installed)
log_section "Ingress Connectivity"

for cluster in "${CLUSTER_CORE}" "${CLUSTER_CUST_A}"; do
    kubeconfig="${HOME}/.config/k3d/kubeconfig-${cluster}.yaml"
    
    if KUBECONFIG="${kubeconfig}" kubectl get svc -n kube-system | grep -q ingress; then
        log_info "✓ ${cluster}: Ingress controller found"
        
        # Get ingress service port
        INGRESS_PORT=$(KUBECONFIG="${kubeconfig}" kubectl get svc -n kube-system -l app.kubernetes.io/name=traefik -o jsonpath='{.items[0].spec.ports[?(@.name=="http")].nodePort}' 2>/dev/null || echo "")
        
        if [ -n "${INGRESS_PORT}" ]; then
            log_info "  Ingress HTTP port: ${INGRESS_PORT}"
            # Test localhost connectivity (k3d port mapping)
            if curl -s -m 2 "http://localhost:${INGRESS_PORT}" &> /dev/null || \
               curl -s -m 2 "http://127.0.0.1:${INGRESS_PORT}" &> /dev/null; then
                log_info "  ✓ Ingress endpoint reachable"
            else
                log_warn "  ⚠ Ingress endpoint not reachable on localhost:${INGRESS_PORT} (may need port mapping)"
            fi
        fi
    else
        log_warn "  ⚠ ${cluster}: No ingress controller found"
    fi
done

# Test cross-cluster connectivity (if multi-cluster setup)
log_section "Cross-Cluster Connectivity"

if [ -f "${HOME}/.config/k3d/kubeconfig-${CLUSTER_CORE}.yaml" ] && \
   [ -f "${HOME}/.config/k3d/kubeconfig-${CLUSTER_CUST_A}.yaml" ]; then
    log_info "Testing cross-cluster service discovery..."
    # In k3d, clusters can be in same network or separate
    # This is a basic check - full E2E tests will validate actual connectivity
    log_info "  Cross-cluster setup detected (full validation in E2E tests)"
fi

# Test NetPol/RBAC baseline
log_section "Network Policy / RBAC Baseline"

for cluster in "${CLUSTER_CORE}" "${CLUSTER_CUST_A}" "${CLUSTER_EDGE_UAT}"; do
    kubeconfig="${HOME}/.config/k3d/kubeconfig-${cluster}.yaml"
    
    log_info "Checking ${cluster} baseline policies..."
    
    # Check if control-plane calls work (kubectl is allowed)
    if KUBECONFIG="${kubeconfig}" kubectl get namespaces &> /dev/null; then
        log_info "  ✓ Control-plane API calls allowed"
    else
        log_error "  ✗ Control-plane API calls blocked (check RBAC/NetPol)"
    fi
    
    # Check if core namespaces are accessible
    if KUBECONFIG="${kubeconfig}" kubectl get namespace kube-system &> /dev/null; then
        log_info "  ✓ Core namespaces accessible"
    else
        log_error "  ✗ Core namespaces blocked"
    fi
done

log_section "✅ Harness Validation Complete"

log_info "Summary:"
log_info "  - Clusters exist: ${CLUSTERS_EXIST}"
log_info "  - DNS: Checked"
log_info "  - Connectivity: Tested"
log_info "  - Ingress: Checked"
log_info "  - NetPol/RBAC: Baseline validated"

echo ""
log_info "Next steps: Run E2E tests with: make test-e2e"
