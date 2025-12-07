#!/bin/bash
#
# Zen Watcher - End-to-End Test
#
# Complete E2E test that:
# 1. Deploys a fresh cluster with quick-demo.sh
# 2. Verifies all 6 sources create observations
# 3. Validates the dashboard shows all sources
#

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

CLUSTER_NAME="${CLUSTER_NAME:-zen-demo}"
EXPECTED_SOURCES=("trivy" "kyverno" "falco" "audit" "checkov" "kubebench")

echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  Zen Watcher - End-to-End Test${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Step 1: Clean up any existing cluster
echo -e "${YELLOW}→${NC} Cleaning up existing cluster..."
k3d cluster delete ${CLUSTER_NAME} 2>/dev/null || true
echo -e "${GREEN}✓${NC} Cleanup complete"
echo ""

# Step 2: Deploy fresh cluster
echo -e "${YELLOW}→${NC} Deploying fresh cluster with quick-demo.sh..."
./scripts/quick-demo.sh
if [ $? -ne 0 ]; then
    echo -e "${RED}✗${NC} Failed to deploy cluster"
    exit 1
fi
echo -e "${GREEN}✓${NC} Cluster deployed"
echo ""

# Set kubeconfig
export KUBECONFIG=/home/neves/.kube/zen-demo-kubeconfig

# Step 3: Wait for zen-watcher to be ready
echo -e "${YELLOW}→${NC} Waiting for zen-watcher to be ready..."
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=zen-watcher -n zen-system --timeout=120s
echo -e "${GREEN}✓${NC} zen-watcher is ready"
echo ""

# Step 4: Install missing CRDs
echo -e "${YELLOW}→${NC} Installing ObservationFilter and ObservationMapping CRDs..."
kubectl apply -f deployments/crds/observationfilter_crd.yaml >/dev/null 2>&1
kubectl apply -f deployments/crds/observationmapping_crd.yaml >/dev/null 2>&1
echo -e "${GREEN}✓${NC} CRDs installed"
echo ""

# Step 5: Restart zen-watcher to pick up CRDs
echo -e "${YELLOW}→${NC} Restarting zen-watcher to pick up new CRDs..."
kubectl delete pod -l app.kubernetes.io/name=zen-watcher -n zen-system >/dev/null 2>&1
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=zen-watcher -n zen-system --timeout=120s
sleep 10  # Give it time to initialize all adapters
echo -e "${GREEN}✓${NC} zen-watcher restarted"
echo ""

# Step 6: Send mock data for all sources
echo -e "${YELLOW}→${NC} Sending mock data for all 6 sources..."
./scripts/data/send-webhooks.sh zen-system 2>&1 | grep -E "(✓|✗|→)" || true
echo -e "${GREEN}✓${NC} Mock data sent"
echo ""

# Step 7: Wait for observations to be created
echo -e "${YELLOW}→${NC} Waiting for observations to be created (30s)..."
sleep 30
echo ""

# Step 8: Verify observations from all sources
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  Verification Results${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Get total observations
TOTAL_OBS=$(kubectl get observations -A --no-headers 2>/dev/null | wc -l)
echo -e "${CYAN}Total Observations:${NC} ${TOTAL_OBS}"
echo ""

# Get observations by source
echo -e "${CYAN}Observations by Source:${NC}"
SOURCES_FOUND=$(kubectl get observations -A -o json 2>/dev/null | jq -r '.items[] | .spec.source' | sort | uniq)

MISSING_SOURCES=()
for source in "${EXPECTED_SOURCES[@]}"; do
    COUNT=$(kubectl get observations -A -o json 2>/dev/null | jq -r ".items[] | select(.spec.source==\"${source}\") | .metadata.name" | wc -l)
    if [ "$COUNT" -gt 0 ]; then
        echo -e "  ${GREEN}✓${NC} ${source}: ${COUNT} observations"
    else
        echo -e "  ${RED}✗${NC} ${source}: 0 observations"
        MISSING_SOURCES+=("$source")
    fi
done
echo ""

# Step 9: Show severity distribution
echo -e "${CYAN}Severity Distribution:${NC}"
kubectl get observations -A -o json 2>/dev/null | jq -r '.items[] | "\(.spec.source) - \(.spec.severity)"' | sort | uniq -c | head -20
echo ""

# Step 10: Final result
if [ ${#MISSING_SOURCES[@]} -eq 0 ]; then
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${GREEN}  ✅ E2E Test PASSED!${NC}"
    echo -e "${GREEN}  All 6 sources are creating observations${NC}"
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    exit 0
else
    echo -e "${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${RED}  ✗ E2E Test FAILED!${NC}"
    echo -e "${RED}  Missing sources: ${MISSING_SOURCES[*]}${NC}"
    echo -e "${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
    echo -e "${YELLOW}Troubleshooting:${NC}"
    echo "  1. Check zen-watcher logs: kubectl logs -n zen-system -l app.kubernetes.io/name=zen-watcher"
    echo "  2. Check if source tools are running:"
    for source in "${MISSING_SOURCES[@]}"; do
        case "$source" in
            "kyverno")
                echo "     - kubectl get pods -n kyverno"
                echo "     - kubectl get policyreports -A"
                ;;
            "falco")
                echo "     - kubectl get pods -n falco"
                ;;
            "checkov")
                echo "     - kubectl get configmaps -n checkov"
                ;;
            "kubebench")
                echo "     - kubectl get configmaps -n kube-bench"
                ;;
        esac
    done
    exit 1
fi


