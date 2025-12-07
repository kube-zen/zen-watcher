#!/bin/bash
#
# Zen Watcher - Cluster Destruction Script
# 
# Destroys the demo cluster and cleans up resources
#
# Usage:
#   ./scripts/cluster/destroy.sh              # Uses k3d (default)
#   ./scripts/cluster/destroy.sh kind         # Uses kind
#   ./scripts/cluster/destroy.sh minikube     # Uses minikube
#   ./scripts/cluster/destroy.sh --all        # Cleanup all demo clusters
#
# Environment Variables:
#   ZEN_CLUSTER_NAME=zen-demo           # Cluster name to delete (default: zen-demo)

set -e

# Source common utilities
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../utils/common.sh"

# Parse arguments
PLATFORM="${1:-k3d}"
CLEANUP_ALL=false

if [ "$1" = "--all" ]; then
    CLEANUP_ALL=true
    PLATFORM="k3d"
fi

CLUSTER_NAME="${ZEN_CLUSTER_NAME:-zen-demo}"

echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  Zen Watcher - Cluster Cleanup${NC}"
echo -e "${BLUE}  Platform: ${CYAN}${PLATFORM}${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Function to cleanup cluster
cleanup_cluster() {
    case "$PLATFORM" in
        k3d)
            if [ "$CLEANUP_ALL" = true ]; then
                echo -e "${YELLOW}→${NC} Cleaning up all demo clusters..."
                local clusters=$(k3d cluster list 2>/dev/null | grep -E "zen-demo|zen-watcher" | awk '{print $1}' || true)
                if [ -z "$clusters" ]; then
                    echo -e "${CYAN}ℹ${NC}  No demo clusters found"
                else
                    for cluster in $clusters; do
                        log_step "Deleting cluster: ${cluster}"
                        k3d cluster delete ${cluster} 2>/dev/null || true
                    done
                    log_success "All demo clusters deleted"
                fi
            else
                if k3d cluster list 2>/dev/null | grep -q "^${CLUSTER_NAME}"; then
                    log_step "Deleting k3d cluster '${CLUSTER_NAME}'..."
                    k3d cluster delete ${CLUSTER_NAME}
                    log_success "Cluster deleted"
                else
                    log_info "Cluster '${CLUSTER_NAME}' not found"
                fi
            fi
            ;;
        kind)
            if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
                log_step "Deleting kind cluster '${CLUSTER_NAME}'..."
                kind delete cluster --name ${CLUSTER_NAME}
                log_success "Cluster deleted"
            else
                log_info "Cluster '${CLUSTER_NAME}' not found"
            fi
            ;;
        minikube)
            if minikube status -p ${CLUSTER_NAME} &>/dev/null; then
                log_step "Deleting minikube profile '${CLUSTER_NAME}'..."
                minikube delete -p ${CLUSTER_NAME}
                log_success "Profile deleted"
            else
                log_info "Profile '${CLUSTER_NAME}' not found"
            fi
            ;;
        *)
            log_error "Unknown platform: $PLATFORM"
            echo "  Supported: k3d, kind, minikube"
            exit 1
            ;;
    esac
}

# Main cleanup - delete cluster (this will also delete all namespaces and resources)
cleanup_cluster

echo ""
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}  ✅ Cleanup Complete!${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

