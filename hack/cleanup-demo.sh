#!/bin/bash
#
# Zen Watcher - Demo Cleanup Script
# 
# Destroys the demo cluster and cleans up resources
#
# Usage:
#   ./hack/cleanup-demo.sh              # Uses k3d (default)
#   ./hack/cleanup-demo.sh kind         # Uses kind
#   ./hack/cleanup-demo.sh minikube     # Uses minikube
#   ./hack/cleanup-demo.sh --all        # Cleanup all demo clusters
#
# Environment Variables:
#   ZEN_CLUSTER_NAME=zen-demo           # Cluster name to delete (default: zen-demo)
#   ZEN_NAMESPACE=zen-system           # Namespace to delete (optional)

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Parse arguments
PLATFORM="${1:-k3d}"
CLEANUP_ALL=false

if [ "$1" = "--all" ]; then
    CLEANUP_ALL=true
    PLATFORM="k3d"
fi

CLUSTER_NAME="${ZEN_CLUSTER_NAME:-zen-demo}"
NAMESPACE="${ZEN_NAMESPACE:-zen-system}"

echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  Zen Watcher - Demo Cleanup${NC}"
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
                        echo -e "${YELLOW}  →${NC} Deleting cluster: ${cluster}"
                        k3d cluster delete ${cluster} 2>/dev/null || true
                    done
                    echo -e "${GREEN}✓${NC} All demo clusters deleted"
                fi
            else
                if k3d cluster list 2>/dev/null | grep -q "^${CLUSTER_NAME}"; then
                    echo -e "${YELLOW}→${NC} Deleting k3d cluster '${CLUSTER_NAME}'..."
                    k3d cluster delete ${CLUSTER_NAME}
                    echo -e "${GREEN}✓${NC} Cluster deleted"
                else
                    echo -e "${CYAN}ℹ${NC}  Cluster '${CLUSTER_NAME}' not found"
                fi
            fi
            ;;
        kind)
            if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
                echo -e "${YELLOW}→${NC} Deleting kind cluster '${CLUSTER_NAME}'..."
                kind delete cluster --name ${CLUSTER_NAME}
                echo -e "${GREEN}✓${NC} Cluster deleted"
            else
                echo -e "${CYAN}ℹ${NC}  Cluster '${CLUSTER_NAME}' not found"
            fi
            ;;
        minikube)
            if minikube status -p ${CLUSTER_NAME} &>/dev/null; then
                echo -e "${YELLOW}→${NC} Deleting minikube profile '${CLUSTER_NAME}'..."
                minikube delete -p ${CLUSTER_NAME}
                echo -e "${GREEN}✓${NC} Profile deleted"
            else
                echo -e "${CYAN}ℹ${NC}  Profile '${CLUSTER_NAME}' not found"
            fi
            ;;
        *)
            echo -e "${RED}✗${NC} Unknown platform: $PLATFORM"
            echo "  Supported: k3d, kind, minikube"
            exit 1
            ;;
    esac
}

# Function to cleanup namespace (optional)
cleanup_namespace() {
    if kubectl get namespace ${NAMESPACE} &>/dev/null 2>&1; then
        echo -e "${YELLOW}→${NC} Deleting namespace '${NAMESPACE}'..."
        kubectl delete namespace ${NAMESPACE} --wait=false 2>/dev/null || true
        echo -e "${GREEN}✓${NC} Namespace deletion initiated"
    else
        echo -e "${CYAN}ℹ${NC}  Namespace '${NAMESPACE}' not found"
    fi
}

# Optionally cleanup namespace BEFORE deleting cluster (if cluster still exists)
# Note: If cluster is deleted, namespaces are automatically deleted too
if [ "$CLEANUP_ALL" = false ]; then
    # Check if cluster exists before asking about namespace
    CLUSTER_EXISTS=false
    case "$PLATFORM" in
        k3d)
            if k3d cluster list 2>/dev/null | grep -q "^${CLUSTER_NAME}"; then
                CLUSTER_EXISTS=true
            fi
            ;;
        kind)
            if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
                CLUSTER_EXISTS=true
            fi
            ;;
        minikube)
            if minikube status -p ${CLUSTER_NAME} &>/dev/null 2>&1; then
                CLUSTER_EXISTS=true
            fi
            ;;
    esac
    
    if [ "$CLUSTER_EXISTS" = true ]; then
        echo ""
        read -p "$(echo -e ${YELLOW}Delete namespace '${NAMESPACE}' before deleting cluster? [y/N]${NC}) " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            cleanup_namespace
        fi
    else
        echo -e "${CYAN}ℹ${NC}  Cluster doesn't exist, skipping namespace deletion (it would be deleted with cluster anyway)"
    fi
else
    # In --all mode, try to cleanup common namespaces before deleting clusters
    for ns in zen-system zen-watcher zen-demo; do
        if kubectl get namespace ${ns} &>/dev/null 2>&1; then
            echo -e "${YELLOW}→${NC} Deleting namespace '${ns}'..."
            kubectl delete namespace ${ns} --wait=false 2>/dev/null || true
        fi
    done
fi

# Main cleanup - delete cluster (this will also delete all namespaces)
cleanup_cluster

echo ""
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}  ✅ Cleanup Complete!${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

