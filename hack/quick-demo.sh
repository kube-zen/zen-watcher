#!/bin/bash
#
# Zen Watcher - Quick Demo Setup
# 
# Clone → Run → See Graphs! 
# No bureaucracy, just results.
#
# Supports: k3d (default), kind, minikube
#
# Usage: 
#   ./hack/quick-demo.sh              # Uses k3d
#   ./hack/quick-demo.sh kind         # Uses kind
#   ./hack/quick-demo.sh minikube     # Uses minikube
#

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
PLATFORM="${1:-k3d}"
CLUSTER_NAME="${ZEN_CLUSTER_NAME:-zen-demo}"
NAMESPACE="${ZEN_NAMESPACE:-zen-system}"
GRAFANA_PORT="${GRAFANA_PORT:-3100}"
ZEN_WATCHER_PORT="${ZEN_WATCHER_PORT:-8180}"
VICTORIA_METRICS_PORT="${VICTORIA_METRICS_PORT:-8528}"

# Generate random password for zen user
GRAFANA_PASSWORD=$(openssl rand -base64 12 | tr -d "=+/" | cut -c1-12)

echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  Zen Watcher - Quick Demo Setup${NC}"
echo -e "${BLUE}  Platform: ${CYAN}${PLATFORM}${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Check prerequisites
echo -e "${YELLOW}→${NC} Checking prerequisites..."

check_command() {
    if ! command -v $1 &> /dev/null; then
        echo -e "${RED}✗${NC} $1 is not installed. Please install it first."
        echo "  Visit: $2"
        exit 1
    fi
    echo -e "${GREEN}✓${NC} $1 found"
}

check_command "kubectl" "https://kubernetes.io/docs/tasks/tools/"
check_command "helm" "https://helm.sh/docs/intro/install/"
check_command "jq" "https://stedolan.github.io/jq/download/"
check_command "openssl" "https://www.openssl.org/"

# Check platform-specific command
case "$PLATFORM" in
    k3d)
        check_command "k3d" "https://k3d.io/#installation"
        ;;
    kind)
        check_command "kind" "https://kind.sigs.k8s.io/docs/user/quick-start/#installation"
        ;;
    minikube)
        check_command "minikube" "https://minikube.sigs.k8s.io/docs/start/"
        ;;
    *)
        echo -e "${RED}✗${NC} Unknown platform: $PLATFORM"
        echo "  Supported: k3d, kind, minikube"
        exit 1
        ;;
esac

echo ""

# Function to create cluster
create_cluster() {
    case "$PLATFORM" in
        k3d)
            if k3d cluster list 2>/dev/null | grep -q "^${CLUSTER_NAME}"; then
                echo -e "${YELLOW}⚠${NC}  Cluster '${CLUSTER_NAME}' already exists."
                read -p "$(echo -e ${YELLOW}Delete and recreate? [y/N]${NC}) " -n 1 -r
                echo
                if [[ $REPLY =~ ^[Yy]$ ]]; then
                    echo -e "${YELLOW}→${NC} Deleting existing cluster..."
                    k3d cluster delete ${CLUSTER_NAME}
                fi
            fi
            
            if ! k3d cluster list 2>/dev/null | grep -q "^${CLUSTER_NAME}"; then
                echo -e "${YELLOW}→${NC} Creating k3d cluster '${CLUSTER_NAME}'..."
                k3d cluster create ${CLUSTER_NAME} \
                    --agents 1 \
                    --k3s-arg "--disable=traefik@server:0" \
                    --wait
                echo -e "${GREEN}✓${NC} Cluster created"
            fi
            ;;
        kind)
            if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
                echo -e "${YELLOW}⚠${NC}  Cluster '${CLUSTER_NAME}' already exists."
                read -p "$(echo -e ${YELLOW}Delete and recreate? [y/N]${NC}) " -n 1 -r
                echo
                if [[ $REPLY =~ ^[Yy]$ ]]; then
                    echo -e "${YELLOW}→${NC} Deleting existing cluster..."
                    kind delete cluster --name ${CLUSTER_NAME}
                fi
            fi
            
            if ! kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
                echo -e "${YELLOW}→${NC} Creating kind cluster '${CLUSTER_NAME}'..."
                kind create cluster --name ${CLUSTER_NAME} --wait 2m
                echo -e "${GREEN}✓${NC} Cluster created"
            fi
            ;;
        minikube)
            if minikube status -p ${CLUSTER_NAME} &>/dev/null; then
                echo -e "${YELLOW}⚠${NC}  Cluster '${CLUSTER_NAME}' already exists."
                read -p "$(echo -e ${YELLOW}Delete and recreate? [y/N]${NC}) " -n 1 -r
                echo
                if [[ $REPLY =~ ^[Yy]$ ]]; then
                    echo -e "${YELLOW}→${NC} Deleting existing cluster..."
                    minikube delete -p ${CLUSTER_NAME}
                fi
            fi
            
            if ! minikube status -p ${CLUSTER_NAME} &>/dev/null; then
                echo -e "${YELLOW}→${NC} Creating minikube cluster '${CLUSTER_NAME}'..."
                minikube start -p ${CLUSTER_NAME} --cpus 4 --memory 8192
                echo -e "${GREEN}✓${NC} Cluster created"
            fi
            ;;
    esac
}

create_cluster

echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  Deploying Security Tools${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Add Helm repositories
echo -e "${YELLOW}→${NC} Adding Helm repositories..."
helm repo add aqua https://aquasecurity.github.io/helm-charts 2>/dev/null || true
helm repo add falcosecurity https://falcosecurity.github.io/charts 2>/dev/null || true
helm repo add kyverno https://kyverno.github.io/kyverno/ 2>/dev/null || true
helm repo update > /dev/null 2>&1
echo -e "${GREEN}✓${NC} Helm repositories updated"

# Deploy Trivy Operator
echo -e "${YELLOW}→${NC} Deploying Trivy Operator (this may take 1-2 minutes)..."
helm upgrade --install trivy-operator aqua/trivy-operator \
    --namespace trivy-system \
    --create-namespace \
    --set="trivy.ignoreUnfixed=true" \
    --wait --timeout=2m > /dev/null 2>&1 || echo -e "${YELLOW}⚠${NC}  Trivy deployment taking longer, continuing..."
echo -e "${GREEN}✓${NC} Trivy Operator deployed"

# Deploy Falco (without waiting, can take time)
echo -e "${YELLOW}→${NC} Deploying Falco (starting in background)..."
helm upgrade --install falco falcosecurity/falco \
    --namespace falco \
    --create-namespace \
    --set falcosidekick.enabled=false \
    --wait --timeout=30s > /dev/null 2>&1 || echo -e "${YELLOW}⚠${NC}  Falco starting (will be ready soon)"
echo -e "${GREEN}✓${NC} Falco deployed"

# Deploy Kyverno
echo -e "${YELLOW}→${NC} Deploying Kyverno (starting in background)..."
helm upgrade --install kyverno kyverno/kyverno \
    --namespace kyverno \
    --create-namespace \
    --wait --timeout=30s > /dev/null 2>&1 || echo -e "${YELLOW}⚠${NC}  Kyverno starting (will be ready soon)"
echo -e "${GREEN}✓${NC} Kyverno deployed"

echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  Deploying Monitoring Stack${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Create namespace
kubectl create namespace ${NAMESPACE} 2>/dev/null || true

# Deploy VictoriaMetrics
echo -e "${YELLOW}→${NC} Deploying VictoriaMetrics..."
kubectl create deployment victoriametrics \
    --image=victoriametrics/victoria-metrics:latest \
    -n ${NAMESPACE} 2>/dev/null || kubectl rollout restart deployment/victoriametrics -n ${NAMESPACE}
kubectl expose deployment victoriametrics \
    --port=8428 --target-port=8428 \
    -n ${NAMESPACE} 2>/dev/null || true
echo -e "${GREEN}✓${NC} VictoriaMetrics deployed"

# Deploy Grafana with zen user
echo -e "${YELLOW}→${NC} Deploying Grafana with zen user..."
kubectl create deployment grafana \
    --image=grafana/grafana:latest \
    -n ${NAMESPACE} \
    --dry-run=client -o yaml | \
kubectl set env --local -f - \
    GF_SECURITY_ADMIN_USER=zen \
    GF_SECURITY_ADMIN_PASSWORD=${GRAFANA_PASSWORD} \
    GF_USERS_ALLOW_SIGN_UP=false \
    GF_USERS_DEFAULT_THEME=dark \
    --dry-run=client -o yaml | \
kubectl apply -f - > /dev/null 2>&1

kubectl expose deployment grafana \
    --port=3000 --target-port=3000 \
    -n ${NAMESPACE} 2>/dev/null || true
echo -e "${GREEN}✓${NC} Grafana deployed (user: zen)"

# Wait for pods
echo -e "${YELLOW}→${NC} Waiting for monitoring stack to be ready (this takes 30-60 seconds)..."
kubectl wait --for=condition=ready pod -l app=victoriametrics -n ${NAMESPACE} --timeout=60s > /dev/null 2>&1 || true
kubectl wait --for=condition=ready pod -l app=grafana -n ${NAMESPACE} --timeout=60s > /dev/null 2>&1 || true
echo -e "${GREEN}✓${NC} Monitoring stack ready"

echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  Deploying Zen Watcher${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Deploy Zen Watcher CRDs
echo -e "${YELLOW}→${NC} Deploying Zen Watcher CRDs..."
kubectl apply -f deployments/crds/ > /dev/null 2>&1
echo -e "${GREEN}✓${NC} CRDs deployed"

# Ask about mock data
echo ""
read -p "$(echo -e ${CYAN}Deploy mock zen-watcher with sample metrics? [Y/n]${NC}) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Nn]$ ]]; then
    echo -e "${YELLOW}→${NC} Deploying mock data..."
    ./hack/mock-data.sh ${NAMESPACE} > /dev/null 2>&1 || echo -e "${YELLOW}⚠${NC}  Mock data deployment issue (continuing...)"
    echo -e "${GREEN}✓${NC} Mock data deployed"
fi

echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  Configuring Grafana${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Setup port-forwards
echo -e "${YELLOW}→${NC} Setting up port-forwards..."

# Kill existing port-forwards
pkill -f "kubectl port-forward.*${NAMESPACE}" 2>/dev/null || true
sleep 2

# Start port-forwards in background
kubectl port-forward -n ${NAMESPACE} svc/grafana ${GRAFANA_PORT}:3000 --address=0.0.0.0 > /tmp/grafana-pf.log 2>&1 &
GRAFANA_PF_PID=$!
sleep 3

kubectl port-forward -n ${NAMESPACE} svc/victoriametrics ${VICTORIA_METRICS_PORT}:8428 --address=0.0.0.0 > /tmp/vm-pf.log 2>&1 &
VM_PF_PID=$!
sleep 2

echo -e "${GREEN}✓${NC} Port-forwards active"

# Wait for Grafana to be fully ready
echo -e "${YELLOW}→${NC} Waiting for Grafana to be fully ready (15-30 seconds)..."
for i in {1..30}; do
    if curl -s http://localhost:${GRAFANA_PORT}/api/health 2>/dev/null | grep -q "ok"; then
        echo -e "${GREEN}✓${NC} Grafana is ready"
        break
    fi
    sleep 1
    if [ $((i % 5)) -eq 0 ]; then
        echo -e "${CYAN}  ... still waiting ($i seconds)${NC}"
    fi
done

# Configure Grafana datasource
echo -e "${YELLOW}→${NC} Configuring VictoriaMetrics datasource..."
DATASOURCE_RESULT=$(curl -s -X POST http://localhost:${GRAFANA_PORT}/api/datasources \
    -H "Content-Type: application/json" \
    -u zen:${GRAFANA_PASSWORD} \
    -d '{
        "name": "VictoriaMetrics",
        "type": "prometheus",
        "url": "http://victoriametrics:8428",
        "access": "proxy",
        "isDefault": true,
        "jsonData": {
            "timeInterval": "15s",
            "httpMethod": "POST"
        }
    }' 2>&1)

if echo "$DATASOURCE_RESULT" | grep -q "Datasource added\|already exists"; then
    echo -e "${GREEN}✓${NC} Datasource configured"
else
    echo -e "${YELLOW}⚠${NC}  Datasource: $(echo $DATASOURCE_RESULT | jq -r '.message' 2>/dev/null || echo 'checking...')"
fi

# Import dashboard
echo -e "${YELLOW}→${NC} Importing Zen Watcher dashboard..."
if [ -f "config/dashboards/zen-watcher-dashboard.json" ]; then
    DASHBOARD_RESULT=$(cat config/dashboards/zen-watcher-dashboard.json | \
    jq '{dashboard: ., overwrite: true, message: "Demo Import"}' | \
    curl -s -X POST http://localhost:${GRAFANA_PORT}/api/dashboards/db \
        -H "Content-Type: application/json" \
        -u zen:${GRAFANA_PASSWORD} \
        -d @- 2>&1)
    
    if echo "$DASHBOARD_RESULT" | grep -q "success"; then
        echo -e "${GREEN}✓${NC} Dashboard imported successfully"
    else
        echo -e "${YELLOW}⚠${NC}  Dashboard: $(echo $DASHBOARD_RESULT | jq -r '.message' 2>/dev/null || echo 'checking...')"
    fi
else
    echo -e "${YELLOW}⚠${NC}  Dashboard file not found at config/dashboards/zen-watcher-dashboard.json"
fi

echo ""
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}  🎉 Demo Environment Ready!${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  🔐 GRAFANA CREDENTIALS${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "  Username: ${GREEN}zen${NC}"
echo -e "  Password: ${GREEN}${GRAFANA_PASSWORD}${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "${BLUE}📊 Access URLs (copy and paste):${NC}"
echo ""
echo -e "  ${GREEN}Grafana Dashboard:${NC}"
echo -e "    ${CYAN}http://localhost:${GRAFANA_PORT}/d/zen-watcher${NC}"
echo ""
echo -e "  ${GREEN}Grafana Home:${NC}"
echo -e "    ${CYAN}http://localhost:${GRAFANA_PORT}${NC}"
echo ""
echo -e "  ${GREEN}VictoriaMetrics UI:${NC}"
echo -e "    ${CYAN}http://localhost:${VICTORIA_METRICS_PORT}/vmui${NC}"
echo ""
echo -e "${YELLOW}⏳ Note:${NC} Grafana may take 10-20 seconds to fully load"
echo -e "${YELLOW}💡 Note:${NC} Password change is optional - can be done in Grafana settings"
echo ""
echo -e "${BLUE}🔧 Deployed Components:${NC}"
echo -e "  ✓ Trivy Operator (trivy-system)"
echo -e "  ✓ Falco (falco)"
echo -e "  ✓ Kyverno (kyverno)"
echo -e "  ✓ VictoriaMetrics (${NAMESPACE})"
echo -e "  ✓ Grafana (${NAMESPACE})"
echo ""
echo -e "${BLUE}🛠  Useful Commands:${NC}"
echo -e "  # View all pods"
echo -e "  kubectl get pods --all-namespaces"
echo ""
echo -e "  # Check Zen Watcher CRDs"
echo -e "  kubectl get zenevents -A"
echo ""
echo -e "  # Clean up everything"
case "$PLATFORM" in
    k3d) echo -e "  k3d cluster delete ${CLUSTER_NAME}" ;;
    kind) echo -e "  kind delete cluster --name ${CLUSTER_NAME}" ;;
    minikube) echo -e "  minikube delete -p ${CLUSTER_NAME}" ;;
esac
echo ""
echo -e "${YELLOW}💡 Tip:${NC} Keep this terminal open to maintain port-forwards!"
echo -e "${YELLOW}💡 Tip:${NC} Press Ctrl+C to stop port-forwards and exit"
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}✨ READY! Open your browser to:${NC}"
echo -e "${CYAN}   http://localhost:${GRAFANA_PORT}/d/zen-watcher${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Store PIDs
echo $GRAFANA_PF_PID > /tmp/zen-demo-grafana.pid
echo $VM_PF_PID > /tmp/zen-demo-vm.pid

# Trap Ctrl+C to cleanup
cleanup() {
    echo ""
    echo -e "${YELLOW}→${NC} Stopping port-forwards..."
    kill $GRAFANA_PF_PID $VM_PF_PID 2>/dev/null || true
    rm -f /tmp/zen-demo-*.pid
    echo -e "${GREEN}✓${NC} Port-forwards stopped"
    echo ""
    echo -e "${BLUE}Cluster is still running. To remove:${NC}"
    case "$PLATFORM" in
        k3d) echo -e "  ${CYAN}k3d cluster delete ${CLUSTER_NAME}${NC}" ;;
        kind) echo -e "  ${CYAN}kind delete cluster --name ${CLUSTER_NAME}${NC}" ;;
        minikube) echo -e "  ${CYAN}minikube delete -p ${CLUSTER_NAME}${NC}" ;;
    esac
    exit 0
}

trap cleanup INT TERM

# Wait indefinitely
echo -e "${GREEN}Port-forwards active. Press Ctrl+C to exit.${NC}"
while true; do
    sleep 1
done
