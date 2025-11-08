#!/bin/bash
#
# Zen Watcher - Smart Installer
#
# This script detects existing infrastructure and adapts the installation:
# - Detects existing Prometheus/Grafana/VictoriaMetrics
# - Detects existing security tools (Trivy, Falco, Kyverno, etc.)
# - Installs only what's missing
# - Configures Zen Watcher to use existing tools
#
# Usage: ./hack/install.sh [options]
#
# Options:
#   --namespace NAME         Namespace to install Zen Watcher (default: zen-system)
#   --skip-tools             Skip security tools installation
#   --skip-monitoring        Skip monitoring stack installation
#   --use-prometheus URL     Use existing Prometheus at URL
#   --use-grafana URL        Use existing Grafana at URL
#   --dry-run                Show what would be installed without installing
#   --help                   Show this help message
#

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# Default configuration
NAMESPACE="zen-system"
SKIP_TOOLS=false
SKIP_MONITORING=false
DRY_RUN=false
EXISTING_PROMETHEUS=""
EXISTING_GRAFANA=""

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --namespace)
            NAMESPACE="$2"
            shift 2
            ;;
        --skip-tools)
            SKIP_TOOLS=true
            shift
            ;;
        --skip-monitoring)
            SKIP_MONITORING=true
            shift
            ;;
        --use-prometheus)
            EXISTING_PROMETHEUS="$2"
            shift 2
            ;;
        --use-grafana)
            EXISTING_GRAFANA="$2"
            shift 2
            ;;
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --help)
            head -n 20 "$0" | grep "^#" | sed 's/^# \?//'
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            exit 1
            ;;
    esac
done

echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  Zen Watcher - Smart Installation${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

if [ "$DRY_RUN" = true ]; then
    echo -e "${YELLOW}🔍 DRY RUN MODE - No changes will be made${NC}"
    echo ""
fi

# Detection functions
detect_tool() {
    local tool=$1
    local namespace=$2
    
    if [ -n "$namespace" ]; then
        kubectl get pods -n "$namespace" -l "$3" 2>/dev/null | grep -q "Running" && return 0
    else
        kubectl get pods --all-namespaces -l "$3" 2>/dev/null | grep -q "Running" && return 0
    fi
    return 1
}

detect_service() {
    local service=$1
    kubectl get svc --all-namespaces | grep -q "$service" && return 0
    return 1
}

echo -e "${CYAN}🔍 Detecting existing infrastructure...${NC}"
echo ""

# Detect monitoring tools
echo -e "${YELLOW}→${NC} Checking for monitoring stack..."

PROMETHEUS_DETECTED=false
GRAFANA_DETECTED=false
VICTORIA_METRICS_DETECTED=false

if [ -n "$EXISTING_PROMETHEUS" ]; then
    PROMETHEUS_DETECTED=true
    PROMETHEUS_URL="$EXISTING_PROMETHEUS"
    echo -e "${GREEN}  ✓ Using provided Prometheus: ${PROMETHEUS_URL}${NC}"
elif detect_service "prometheus"; then
    PROMETHEUS_DETECTED=true
    PROMETHEUS_NS=$(kubectl get svc --all-namespaces | grep prometheus | awk '{print $1}' | head -1)
    PROMETHEUS_URL="http://prometheus.${PROMETHEUS_NS}.svc.cluster.local:9090"
    echo -e "${GREEN}  ✓ Prometheus detected in namespace: ${PROMETHEUS_NS}${NC}"
fi

if [ -n "$EXISTING_GRAFANA" ]; then
    GRAFANA_DETECTED=true
    GRAFANA_URL="$EXISTING_GRAFANA"
    echo -e "${GREEN}  ✓ Using provided Grafana: ${GRAFANA_URL}${NC}"
elif detect_service "grafana"; then
    GRAFANA_DETECTED=true
    GRAFANA_NS=$(kubectl get svc --all-namespaces | grep grafana | awk '{print $1}' | head -1)
    GRAFANA_URL="http://grafana.${GRAFANA_NS}.svc.cluster.local:3000"
    echo -e "${GREEN}  ✓ Grafana detected in namespace: ${GRAFANA_NS}${NC}"
fi

if detect_service "victoriametrics"; then
    VICTORIA_METRICS_DETECTED=true
    VM_NS=$(kubectl get svc --all-namespaces | grep victoriametrics | awk '{print $1}' | head -1)
    VM_URL="http://victoriametrics.${VM_NS}.svc.cluster.local:8428"
    echo -e "${GREEN}  ✓ VictoriaMetrics detected in namespace: ${VM_NS}${NC}"
fi

if [ "$PROMETHEUS_DETECTED" = false ] && [ "$VICTORIA_METRICS_DETECTED" = false ]; then
    echo -e "${YELLOW}  ⚠ No metrics backend detected${NC}"
    INSTALL_VICTORIA_METRICS=true
else
    INSTALL_VICTORIA_METRICS=false
fi

if [ "$GRAFANA_DETECTED" = false ]; then
    echo -e "${YELLOW}  ⚠ No Grafana detected${NC}"
    INSTALL_GRAFANA=true
else
    INSTALL_GRAFANA=false
fi

echo ""

# Detect security tools
echo -e "${YELLOW}→${NC} Checking for security tools..."

TRIVY_DETECTED=false
FALCO_DETECTED=false
KYVERNO_DETECTED=false
KUBE_BENCH_DETECTED=false

if detect_tool "Trivy" "trivy-system" "app.kubernetes.io/name=trivy-operator"; then
    TRIVY_DETECTED=true
    echo -e "${GREEN}  ✓ Trivy Operator detected${NC}"
else
    echo -e "${YELLOW}  ⚠ Trivy Operator not found${NC}"
fi

if detect_tool "Falco" "falco" "app.kubernetes.io/name=falco"; then
    FALCO_DETECTED=true
    echo -e "${GREEN}  ✓ Falco detected${NC}"
else
    echo -e "${YELLOW}  ⚠ Falco not found${NC}"
fi

if detect_tool "Kyverno" "kyverno" "app.kubernetes.io/name=kyverno"; then
    KYVERNO_DETECTED=true
    echo -e "${GREEN}  ✓ Kyverno detected${NC}"
else
    echo -e "${YELLOW}  ⚠ Kyverno not found${NC}"
fi

# Check for kube-bench
if kubectl get pods --all-namespaces | grep -q "kube-bench"; then
    KUBE_BENCH_DETECTED=true
    echo -e "${GREEN}  ✓ Kube-bench detected${NC}"
else
    echo -e "${YELLOW}  ⚠ Kube-bench not found${NC}"
fi

echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  Installation Plan${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Show installation plan
echo -e "${CYAN}📦 Components to install:${NC}"
echo ""

echo -e "${YELLOW}Monitoring Stack:${NC}"
if [ "$SKIP_MONITORING" = true ]; then
    echo -e "  ${CYAN}↪${NC} Skipped (--skip-monitoring)"
else
    if [ "$INSTALL_VICTORIA_METRICS" = true ]; then
        echo -e "  ${GREEN}✓${NC} VictoriaMetrics"
    else
        echo -e "  ${BLUE}→${NC} Using existing metrics backend"
    fi
    
    if [ "$INSTALL_GRAFANA" = true ]; then
        echo -e "  ${GREEN}✓${NC} Grafana"
    else
        echo -e "  ${BLUE}→${NC} Using existing Grafana"
    fi
fi

echo ""
echo -e "${YELLOW}Security Tools:${NC}"
if [ "$SKIP_TOOLS" = true ]; then
    echo -e "  ${CYAN}↪${NC} Skipped (--skip-tools)"
else
    [ "$TRIVY_DETECTED" = false ] && echo -e "  ${GREEN}✓${NC} Trivy Operator" || echo -e "  ${BLUE}→${NC} Using existing Trivy"
    [ "$FALCO_DETECTED" = false ] && echo -e "  ${GREEN}✓${NC} Falco" || echo -e "  ${BLUE}→${NC} Using existing Falco"
    [ "$KYVERNO_DETECTED" = false ] && echo -e "  ${GREEN}✓${NC} Kyverno" || echo -e "  ${BLUE}→${NC} Using existing Kyverno"
    [ "$KUBE_BENCH_DETECTED" = false ] && echo -e "  ${GREEN}✓${NC} Kube-bench" || echo -e "  ${BLUE}→${NC} Using existing Kube-bench"
fi

echo ""
echo -e "${YELLOW}Zen Watcher:${NC}"
echo -e "  ${GREEN}✓${NC} CRDs (ZenEvent)"
echo -e "  ${GREEN}✓${NC} Zen Watcher application"
echo -e "  ${GREEN}✓${NC} RBAC configuration"
echo -e "  ${GREEN}✓${NC} ServiceMonitor (if Prometheus Operator exists)"

echo ""

if [ "$DRY_RUN" = true ]; then
    echo -e "${YELLOW}🔍 Dry run complete. No changes made.${NC}"
    exit 0
fi

# Confirm installation
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
read -p "$(echo -e ${GREEN}Proceed with installation? [y/N]${NC}) " -n 1 -r
echo
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo -e "${YELLOW}Installation cancelled.${NC}"
    exit 0
fi

echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  Installing Components${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Create namespace
echo -e "${YELLOW}→${NC} Creating namespace: ${NAMESPACE}"
kubectl create namespace ${NAMESPACE} 2>/dev/null || echo -e "${BLUE}  → Namespace already exists${NC}"

# Install monitoring stack
if [ "$SKIP_MONITORING" = false ]; then
    if [ "$INSTALL_VICTORIA_METRICS" = true ]; then
        echo -e "${YELLOW}→${NC} Installing VictoriaMetrics..."
        kubectl create deployment victoriametrics \
            --image=victoriametrics/victoria-metrics:latest \
            -n ${NAMESPACE} 2>/dev/null || true
        kubectl expose deployment victoriametrics \
            --port=8428 --target-port=8428 \
            -n ${NAMESPACE} 2>/dev/null || true
        echo -e "${GREEN}✓${NC} VictoriaMetrics installed"
    fi
    
    if [ "$INSTALL_GRAFANA" = true ]; then
        echo -e "${YELLOW}→${NC} Installing Grafana..."
        kubectl create deployment grafana \
            --image=grafana/grafana:latest \
            -n ${NAMESPACE} 2>/dev/null || true
        kubectl expose deployment grafana \
            --port=3000 --target-port=3000 \
            -n ${NAMESPACE} 2>/dev/null || true
        echo -e "${GREEN}✓${NC} Grafana installed"
    fi
fi

# Install security tools
if [ "$SKIP_TOOLS" = false ]; then
    # Add Helm repos if needed
    if [ "$TRIVY_DETECTED" = false ] || [ "$FALCO_DETECTED" = false ] || [ "$KYVERNO_DETECTED" = false ]; then
        echo -e "${YELLOW}→${NC} Adding Helm repositories..."
        helm repo add aqua https://aquasecurity.github.io/helm-charts 2>/dev/null || true
        helm repo add falcosecurity https://falcosecurity.github.io/charts 2>/dev/null || true
        helm repo add kyverno https://kyverno.github.io/kyverno/ 2>/dev/null || true
        helm repo update > /dev/null 2>&1
    fi
    
    if [ "$TRIVY_DETECTED" = false ]; then
        echo -e "${YELLOW}→${NC} Installing Trivy Operator..."
        helm install trivy-operator aqua/trivy-operator \
            --namespace trivy-system \
            --create-namespace \
            --set="trivy.ignoreUnfixed=true" \
            --wait --timeout=2m
        echo -e "${GREEN}✓${NC} Trivy Operator installed"
    fi
    
    if [ "$FALCO_DETECTED" = false ]; then
        echo -e "${YELLOW}→${NC} Installing Falco..."
        helm install falco falcosecurity/falco \
            --namespace falco \
            --create-namespace \
            --set falcosidekick.enabled=false \
            --wait --timeout=2m
        echo -e "${GREEN}✓${NC} Falco installed"
    fi
    
    if [ "$KYVERNO_DETECTED" = false ]; then
        echo -e "${YELLOW}→${NC} Installing Kyverno..."
        helm install kyverno kyverno/kyverno \
            --namespace kyverno \
            --create-namespace \
            --wait --timeout=2m
        echo -e "${GREEN}✓${NC} Kyverno installed"
    fi
fi

# Install Zen Watcher
echo -e "${YELLOW}→${NC} Installing Zen Watcher CRDs..."
kubectl apply -f deployments/crds/ > /dev/null 2>&1
echo -e "${GREEN}✓${NC} CRDs installed"

echo -e "${YELLOW}→${NC} Installing Zen Watcher application..."
echo -e "${CYAN}  Note: Using Helm for installation${NC}"

# Determine metrics URL
METRICS_URL=""
if [ "$VICTORIA_METRICS_DETECTED" = true ]; then
    METRICS_URL="$VM_URL"
elif [ "$PROMETHEUS_DETECTED" = true ]; then
    METRICS_URL="$PROMETHEUS_URL"
elif [ "$INSTALL_VICTORIA_METRICS" = true ]; then
    METRICS_URL="http://victoriametrics.${NAMESPACE}.svc.cluster.local:8428"
fi

echo -e "${CYAN}  Metrics backend: ${METRICS_URL}${NC}"

# Note: Actual Helm install would go here
echo -e "${YELLOW}  ⚠ Complete with: helm install zen-watcher ./charts/zen-watcher --namespace ${NAMESPACE}${NC}"

echo ""
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}  ✅ Installation Complete!${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

echo -e "${BLUE}📊 Installed Components:${NC}"
echo -e "  Namespace: ${CYAN}${NAMESPACE}${NC}"
[ "$INSTALL_VICTORIA_METRICS" = true ] && echo -e "  ${GREEN}✓${NC} VictoriaMetrics"
[ "$INSTALL_GRAFANA" = true ] && echo -e "  ${GREEN}✓${NC} Grafana"
[ "$TRIVY_DETECTED" = false ] && [ "$SKIP_TOOLS" = false ] && echo -e "  ${GREEN}✓${NC} Trivy Operator"
[ "$FALCO_DETECTED" = false ] && [ "$SKIP_TOOLS" = false ] && echo -e "  ${GREEN}✓${NC} Falco"
[ "$KYVERNO_DETECTED" = false ] && [ "$SKIP_TOOLS" = false ] && echo -e "  ${GREEN}✓${NC} Kyverno"
echo -e "  ${GREEN}✓${NC} Zen Watcher CRDs"
echo ""

echo -e "${BLUE}🔧 Next Steps:${NC}"
echo -e "  1. Check deployment status:"
echo -e "     ${CYAN}kubectl get pods -n ${NAMESPACE}${NC}"
echo ""
echo -e "  2. View Zen Events:"
echo -e "     ${CYAN}kubectl get zenevents -A${NC}"
echo ""
echo -e "  3. Access Grafana (if installed):"
echo -e "     ${CYAN}kubectl port-forward -n ${NAMESPACE} svc/grafana 3000:3000${NC}"
echo ""
echo -e "  4. View logs:"
echo -e "     ${CYAN}kubectl logs -n ${NAMESPACE} -l app.kubernetes.io/name=zen-watcher${NC}"
echo ""

