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
#   ./hack/quick-demo.sh              # Uses k3d (interactive)
#   ./hack/quick-demo.sh kind         # Uses kind (interactive)
#   ./hack/quick-demo.sh minikube     # Uses minikube (interactive)
#   ./hack/quick-demo.sh --non-interactive  # Non-interactive mode (auto-accepts defaults)
#   ./hack/quick-demo.sh --yes        # Same as --non-interactive
#
# Flags:
#   --non-interactive, --yes, -y     # Non-interactive mode (auto-accept all prompts)
#   --use-existing-cluster           # Use existing cluster if it exists (non-interactive)
#   --delete-existing-cluster         # Delete existing cluster if it exists (non-interactive)
#   --use-existing-namespace         # Use existing namespace if it exists (non-interactive)
#   --delete-existing-namespace      # Delete existing namespace if it exists (non-interactive)
#   --skip-mock-data                 # Skip mock data deployment (explicit)
#   --deploy-mock-data               # Deploy mock data (explicit, default: prompt in interactive mode)
#   --install-trivy                  # Install Trivy Operator (vulnerability scanning)
#   --install-falco                  # Install Falco (runtime security)
#   --install-kyverno                # Install Kyverno (policy engine)
#   --install-checkov                # Install Checkov (IaC scanning job)
#   --install-kube-bench             # Install kube-bench (CIS benchmark job)
#   --no-docker-login                # Don't use docker login credentials (use public images only)
#
# Port Configuration (all configurable via environment variables):
#   GRAFANA_PORT=3100                 # Grafana service port (default: 3100, not used with ingress)
#   VICTORIA_METRICS_PORT=8528        # VictoriaMetrics service port (default: 8528, not used with ingress)
#   ZEN_WATCHER_PORT=8180             # Zen Watcher service port (default: 8180, not used with ingress)
#   K3D_API_PORT=6443                 # k3d API server port (default: 6443)
#   KIND_API_PORT=6443                # kind API server port (default: 6443)
#   MINIKUBE_API_PORT=8443            # minikube API server port (default: 8443)
#
# Cluster Configuration:
#   ZEN_CLUSTER_NAME=zen-demo         # Cluster name (default: zen-demo)
#   ZEN_NAMESPACE=zen-system         # Namespace (default: zen-system)
#
# Examples:
#   # Custom ports and cluster name (avoids conflicts with existing k3d)
#   GRAFANA_PORT=3200 VICTORIA_METRICS_PORT=8600 ZEN_CLUSTER_NAME=zen-demo-2 ./hack/quick-demo.sh
#
#   # Install all security tools for comprehensive demo
#   ./hack/quick-demo.sh --install-trivy --install-falco --install-kyverno --install-checkov --install-kube-bench
#
#   # Install only specific tools
#   ./hack/quick-demo.sh --install-falco --install-kube-bench
#
#   # Non-interactive with mock data (full demo)
#   ./hack/quick-demo.sh --non-interactive --deploy-mock-data
#
#   # Non-interactive without mock data (infrastructure only)
#   ./hack/quick-demo.sh --non-interactive --skip-mock-data
#
#   # Use existing k3d cluster (when prompted, choose option 3)
#   ./hack/quick-demo.sh
#
# Note: Script validates all ports and cluster conflicts BEFORE making any changes
#

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Timing tracking
SCRIPT_START_TIME=$(date +%s)
SECTION_START_TIME=$(date +%s)

# Function to show elapsed time for a section
show_section_time() {
    local section_name="$1"
    local end_time=$(date +%s)
    local elapsed=$((end_time - SECTION_START_TIME))
    echo -e "${CYAN}   ⏱  ${section_name} took ${elapsed} seconds${NC}"
    SECTION_START_TIME=$(date +%s)
}

# Function to show total elapsed time
show_total_time() {
    local end_time=$(date +%s)
    local total_elapsed=$((end_time - SCRIPT_START_TIME))
    local minutes=$((total_elapsed / 60))
    local seconds=$((total_elapsed % 60))
    if [ $minutes -gt 0 ]; then
        echo -e "${CYAN}⏱  Total time: ${minutes}m ${seconds}s${NC}"
    else
        echo -e "${CYAN}⏱  Total time: ${total_elapsed}s${NC}"
    fi
}

# Function to check if all controllers in a namespace are ready
# Usage: check_namespace_ready <namespace> <name>
# Checks all deployments, daemonsets, statefulsets, and ingress in the namespace
# Returns: 0 if all ready, 1 if not ready
check_namespace_ready() {
    local namespace=$1
    local name=$2
    
    # Check deployments
    local deployments=$(kubectl get deployment -n "$namespace" --no-headers 2>/dev/null | wc -l | tr -d ' ' || echo "0")
    if [ "$deployments" -gt 0 ]; then
        local ready_deployments=$(kubectl get deployment -n "$namespace" -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.readyReplicas}{"\t"}{.spec.replicas}{"\n"}{end}' 2>/dev/null | \
            awk -F'\t' '$2 == $3 && $3 > 0' | wc -l | tr -d ' ' || echo "0")
        if [ "$ready_deployments" != "$deployments" ]; then
            return 1
        fi
    fi
    
    # Check daemonsets
    local daemonsets=$(kubectl get daemonset -n "$namespace" --no-headers 2>/dev/null | wc -l | tr -d ' ' || echo "0")
    if [ "$daemonsets" -gt 0 ]; then
        local ready_daemonsets=$(kubectl get daemonset -n "$namespace" -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.numberReady}{"\t"}{.status.desiredNumberScheduled}{"\n"}{end}' 2>/dev/null | \
            awk -F'\t' '$2 == $3 && $3 > 0' | wc -l | tr -d ' ' || echo "0")
        if [ "$ready_daemonsets" != "$daemonsets" ]; then
            return 1
        fi
    fi
    
    # Check statefulsets
    local statefulsets=$(kubectl get statefulset -n "$namespace" --no-headers 2>/dev/null | wc -l | tr -d ' ' || echo "0")
    if [ "$statefulsets" -gt 0 ]; then
        local ready_statefulsets=$(kubectl get statefulset -n "$namespace" -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.readyReplicas}{"\t"}{.spec.replicas}{"\n"}{end}' 2>/dev/null | \
            awk -F'\t' '$2 == $3 && $3 > 0' | wc -l | tr -d ' ' || echo "0")
        if [ "$ready_statefulsets" != "$statefulsets" ]; then
            return 1
        fi
    fi
    
    # If namespace has controllers and all are ready, return success
    local total_controllers=$((deployments + daemonsets + statefulsets))
    if [ "$total_controllers" -gt 0 ]; then
        return 0
    fi
    
    # If no controllers found, check if namespace exists (for jobs/ingress)
    if kubectl get namespace "$namespace" >/dev/null 2>&1; then
        return 0
    fi
    
    return 1
}

# Parse arguments for flags
NON_INTERACTIVE=false
USE_EXISTING_CLUSTER_FLAG=false
DELETE_EXISTING_CLUSTER_FLAG=false
USE_EXISTING_NAMESPACE_FLAG=false
DELETE_EXISTING_NAMESPACE_FLAG=false
SKIP_MOCK_DATA=false
DEPLOY_MOCK_DATA=false
INSTALL_TRIVY=false
INSTALL_FALCO=false
INSTALL_KYVERNO=false
INSTALL_CHECKOV=false
INSTALL_KUBE_BENCH=false
SKIP_MONITORING=false
NO_DOCKER_LOGIN=false

# Parse platform and flags
PLATFORM="k3d"
for arg in "$@"; do
    case "$arg" in
        --non-interactive|--yes|-y)
            NON_INTERACTIVE=true
            ;;
        --use-existing-cluster)
            USE_EXISTING_CLUSTER_FLAG=true
            ;;
        --delete-existing-cluster)
            DELETE_EXISTING_CLUSTER_FLAG=true
            ;;
        --use-existing-namespace)
            USE_EXISTING_NAMESPACE_FLAG=true
            ;;
        --delete-existing-namespace)
            DELETE_EXISTING_NAMESPACE_FLAG=true
            ;;
        --skip-mock-data)
            SKIP_MOCK_DATA=true
            ;;
        --deploy-mock-data)
            DEPLOY_MOCK_DATA=true
            ;;
        --install-trivy)
            INSTALL_TRIVY=true
            ;;
        --install-falco)
            INSTALL_FALCO=true
            ;;
        --install-kyverno)
            INSTALL_KYVERNO=true
            ;;
        --install-checkov)
            INSTALL_CHECKOV=true
            ;;
        --install-kube-bench)
            INSTALL_KUBE_BENCH=true
            ;;
        --no-docker-login)
            NO_DOCKER_LOGIN=true
            ;;
        --skip-monitoring|--skip-observability)
            SKIP_MONITORING=true
            ;;
        k3d|kind|minikube)
            PLATFORM="$arg"
            ;;
        *)
            if [[ ! "$arg" =~ ^-- ]]; then
                # Assume it's a platform if not a flag
                PLATFORM="$arg"
            fi
            ;;
    esac
done

# Configuration
CLUSTER_NAME="${ZEN_CLUSTER_NAME:-zen-demo}"
NAMESPACE="${ZEN_NAMESPACE:-zen-system}"

# Port configuration (all configurable via environment variables)
# Cluster API ports
K3D_API_PORT="${K3D_API_PORT:-6443}"
KIND_API_PORT="${KIND_API_PORT:-6443}"
MINIKUBE_API_PORT="${MINIKUBE_API_PORT:-8443}"

# Service ports (internal cluster ports, not exposed directly)
GRAFANA_PORT="${GRAFANA_PORT:-3100}"
ZEN_WATCHER_PORT="${ZEN_WATCHER_PORT:-8180}"
VICTORIA_METRICS_PORT="${VICTORIA_METRICS_PORT:-8528}"

# Generate random password for zen user
GRAFANA_PASSWORD=$(openssl rand -base64 12 | tr -d "=+/" | cut -c1-12)

# Function to check if a port is in use
check_port() {
    local port=$1
    local service=$2
    
    # Use ss first (most reliable on modern Linux)
    if command -v ss &> /dev/null; then
        if ss -tlnp 2>/dev/null | grep -qE ":${port}[[:space:]]|:${port}$"; then
            return 1  # Port is in use
        fi
    elif command -v lsof &> /dev/null; then
        if lsof -Pi :${port} -sTCP:LISTEN -t >/dev/null 2>&1; then
            return 1  # Port is in use
        fi
    elif command -v netstat &> /dev/null; then
        if netstat -an 2>/dev/null | grep -q ":${port}.*LISTEN"; then
            return 1  # Port is in use
        fi
    fi
    
    return 0  # Port is available
}

# Function to find an available port starting from a base port
find_available_port() {
    local base_port=$1
    local service=$2
    local port=$base_port
    local max_attempts=100
    
    for i in $(seq 0 $max_attempts); do
        if check_port $port "$service"; then
            echo $port
            return 0
        fi
        port=$((base_port + i))
    done
    
    echo -e "${RED}✗${NC} Could not find available port for $service (tried $base_port-$port)"
    exit 1
}

# Function to check if ports suggest an existing demo is running
check_existing_demo_ports() {
    local default_grafana=3100
    local default_vm=8528
    local default_watcher=8180
    
    # Check if default ports are in use (suggests existing demo)
    if ! check_port ${default_grafana} "Grafana" || \
       ! check_port ${default_vm} "VictoriaMetrics" || \
       ! check_port ${default_watcher} "Zen Watcher"; then
        return 0  # Default ports are in use
    fi
    
    return 1  # Default ports are free
}

# Function to validate and auto-adjust ports
validate_ports() {
    echo -e "${YELLOW}→${NC} Checking port availability..."
    
    ports_changed=false
    default_ports_in_use=false
    original_grafana=${GRAFANA_PORT}
    original_vm=${VICTORIA_METRICS_PORT}
    original_watcher=${ZEN_WATCHER_PORT}
    
    # Check k3d loadbalancer port (8080) if using k3d
    if [ "$PLATFORM" = "k3d" ]; then
        K3D_LB_PORT=8080
        if ! check_port ${K3D_LB_PORT} "k3d LoadBalancer"; then
            echo -e "${YELLOW}⚠${NC}  Port ${K3D_LB_PORT} is in use (k3d LoadBalancer)"
            echo -e "${CYAN}   Finding alternative port...${NC}"
            K3D_LB_PORT=$(find_available_port ${K3D_LB_PORT} "k3d LoadBalancer")
            echo -e "${CYAN}   Will use port ${K3D_LB_PORT} for k3d LoadBalancer${NC}"
            ports_changed=true
        fi
        INGRESS_HTTP_PORT=${K3D_LB_PORT}
    else
        # For other platforms, use default
        INGRESS_HTTP_PORT=8080
    fi
    
    # Check if default ports are in use (might indicate existing demo)
    if check_existing_demo_ports; then
        default_ports_in_use=true
    fi
    
    # Check Grafana port
    if ! check_port ${GRAFANA_PORT} "Grafana"; then
        echo -e "${YELLOW}⚠${NC}  Port ${GRAFANA_PORT} is in use (Grafana)"
        if [ "$default_ports_in_use" = true ] && [ "${GRAFANA_PORT}" = "3100" ]; then
            echo -e "${CYAN}   This might be from an existing demo setup${NC}"
        fi
        GRAFANA_PORT=$(find_available_port ${GRAFANA_PORT} "Grafana")
        echo -e "${CYAN}   Using port ${GRAFANA_PORT} instead${NC}"
        ports_changed=true
    fi
    
    # Check VictoriaMetrics port
    if ! check_port ${VICTORIA_METRICS_PORT} "VictoriaMetrics"; then
        echo -e "${YELLOW}⚠${NC}  Port ${VICTORIA_METRICS_PORT} is in use (VictoriaMetrics)"
        if [ "$default_ports_in_use" = true ] && [ "${VICTORIA_METRICS_PORT}" = "8528" ]; then
            echo -e "${CYAN}   This might be from an existing demo setup${NC}"
        fi
        VICTORIA_METRICS_PORT=$(find_available_port ${VICTORIA_METRICS_PORT} "VictoriaMetrics")
        echo -e "${CYAN}   Using port ${VICTORIA_METRICS_PORT} instead${NC}"
        ports_changed=true
    fi
    
    # Check Zen Watcher port
    if ! check_port ${ZEN_WATCHER_PORT} "Zen Watcher"; then
        echo -e "${YELLOW}⚠${NC}  Port ${ZEN_WATCHER_PORT} is in use (Zen Watcher)"
        if [ "$default_ports_in_use" = true ] && [ "${ZEN_WATCHER_PORT}" = "8180" ]; then
            echo -e "${CYAN}   This might be from an existing demo setup${NC}"
        fi
        ZEN_WATCHER_PORT=$(find_available_port ${ZEN_WATCHER_PORT} "Zen Watcher")
        echo -e "${CYAN}   Using port ${ZEN_WATCHER_PORT} instead${NC}"
        ports_changed=true
    fi
    
    # Check platform-specific API ports
    case "$PLATFORM" in
        k3d)
            # Check if default k3d API port (6443) is in use
            if ! check_port ${K3D_API_PORT} "k3d API"; then
                echo -e "${YELLOW}⚠${NC}  Port ${K3D_API_PORT} is in use (k3d API)"
                echo -e "${CYAN}   This port is needed for k3d cluster API server${NC}"
                echo -e "${CYAN}   Checking for available port...${NC}"
                K3D_API_PORT=$(find_available_port ${K3D_API_PORT} "k3d API")
                echo -e "${CYAN}   Using port ${K3D_API_PORT} instead${NC}"
                ports_changed=true
            else
                # Even if default port is free, check if any k3d clusters exist that might conflict
                local existing_k3d=$(k3d cluster list 2>/dev/null | grep -v "^NAME" | wc -l | tr -d '[:space:]' || echo "0")
                existing_k3d=${existing_k3d:-0}
                if [ "$existing_k3d" -gt 0 ] && [ "${K3D_API_PORT}" = "6443" ]; then
                    echo -e "${CYAN}ℹ${NC}  Found ${existing_k3d} existing k3d cluster(s)"
                    echo -e "${CYAN}   k3d will auto-select ports, but using explicit port ${K3D_API_PORT} to avoid conflicts${NC}"
                fi
            fi
            ;;
        kind)
            if ! check_port ${KIND_API_PORT} "kind API"; then
                echo -e "${YELLOW}⚠${NC}  Port ${KIND_API_PORT} is in use (kind API)"
                echo -e "${CYAN}   This port is needed for kind cluster API server${NC}"
                echo -e "${CYAN}   Checking for available port...${NC}"
                KIND_API_PORT=$(find_available_port ${KIND_API_PORT} "kind API")
                echo -e "${CYAN}   Using port ${KIND_API_PORT} instead${NC}"
                ports_changed=true
            else
                # Check if any kind clusters exist that might conflict
                local existing_kind=$(kind get clusters 2>/dev/null | wc -l | tr -d '[:space:]' || echo "0")
                existing_kind=${existing_kind:-0}
                if [ "$existing_kind" -gt 0 ] && [ "${KIND_API_PORT}" = "6443" ]; then
                    echo -e "${CYAN}ℹ${NC}  Found ${existing_kind} existing kind cluster(s)"
                    echo -e "${CYAN}   Using explicit API port ${KIND_API_PORT} to avoid conflicts${NC}"
                fi
            fi
            ;;
        minikube)
            if ! check_port ${MINIKUBE_API_PORT} "minikube API"; then
                echo -e "${YELLOW}⚠${NC}  Port ${MINIKUBE_API_PORT} is in use (minikube API)"
                echo -e "${CYAN}   This port is needed for minikube cluster API server${NC}"
                echo -e "${CYAN}   Checking for available port...${NC}"
                MINIKUBE_API_PORT=$(find_available_port ${MINIKUBE_API_PORT} "minikube API")
                echo -e "${CYAN}   Using port ${MINIKUBE_API_PORT} instead${NC}"
                ports_changed=true
            else
                # Check if any minikube profiles exist that might conflict
                local existing_minikube=$(minikube profile list 2>/dev/null | grep -v "Profile" | grep -v "^-" | wc -l | tr -d '[:space:]' || echo "0")
                existing_minikube=${existing_minikube:-0}
                if [ "$existing_minikube" -gt 0 ] && [ "${MINIKUBE_API_PORT}" = "8443" ]; then
                    echo -e "${CYAN}ℹ${NC}  Found ${existing_minikube} existing minikube profile(s)"
                    echo -e "${CYAN}   Using explicit API port ${MINIKUBE_API_PORT} to avoid conflicts${NC}"
                fi
            fi
            ;;
    esac
    
    # If default ports are in use, suggest checking for existing cluster
    if [ "$default_ports_in_use" = true ] && [ "$ports_changed" = true ]; then
        echo ""
        echo -e "${CYAN}💡 Suggestion:${NC} Default demo ports are in use."
        echo -e "${CYAN}   You might have an existing demo running. Consider:${NC}"
        echo -e "   ${CYAN}1. Check existing clusters:${NC}"
        case "$PLATFORM" in
            k3d) echo -e "      ${CYAN}k3d cluster list${NC}" ;;
            kind) echo -e "      ${CYAN}kind get clusters${NC}" ;;
            minikube) echo -e "      ${CYAN}minikube profile list${NC}" ;;
        esac
        echo -e "   ${CYAN}2. Use existing cluster (will be prompted later)${NC}"
        echo -e "   ${CYAN}3. Use different ports:${NC}"
        echo -e "      ${CYAN}GRAFANA_PORT=3200 VICTORIA_METRICS_PORT=8600 ./hack/quick-demo.sh${NC}"
        echo ""
        if [ "$NON_INTERACTIVE" = true ]; then
            echo -e "${CYAN}   Non-interactive mode: continuing with adjusted ports${NC}"
        else
            read -p "$(echo -e ${YELLOW}Continue with adjusted ports? [Y/n]${NC}) " -n 1 -r
            echo
            if [[ $REPLY =~ ^[Nn]$ ]]; then
                echo -e "${CYAN}Exiting. You can:${NC}"
                echo -e "  - Use different ports via environment variables"
                echo -e "  - Check for existing demo and use that cluster"
                exit 0
            fi
        fi
    fi
    
    if [ "$ports_changed" = true ]; then
        echo -e "${YELLOW}⚠${NC}  Some ports were adjusted due to conflicts"
        echo -e "${CYAN}   Export these to reuse:${NC}"
        echo -e "   ${CYAN}export GRAFANA_PORT=${GRAFANA_PORT}${NC}"
        echo -e "   ${CYAN}export VICTORIA_METRICS_PORT=${VICTORIA_METRICS_PORT}${NC}"
        echo -e "   ${CYAN}export ZEN_WATCHER_PORT=${ZEN_WATCHER_PORT}${NC}"
    fi
    
    echo -e "${GREEN}✓${NC} Port configuration validated"
}

# Function to find available namespace name
find_available_namespace() {
    local base_name=$1
    local name=$base_name
    local counter=1
    
    while kubectl get namespace ${name} &>/dev/null; do
        name="${base_name}-${counter}"
        counter=$((counter + 1))
        if [ $counter -gt 10 ]; then
            echo -e "${RED}✗${NC} Could not find available namespace (tried ${base_name} to ${name})"
            exit 1
        fi
    done
    
    echo $name
}

# Function to validate namespace
validate_namespace() {
    echo -e "${YELLOW}→${NC} Checking namespace availability..."
    
    if kubectl get namespace ${NAMESPACE} &>/dev/null; then
        echo -e "${YELLOW}⚠${NC}  Namespace '${NAMESPACE}' already exists!"
        echo ""
        echo -e "${CYAN}This namespace may contain resources from a previous demo.${NC}"
        echo -e "${CYAN}Using the same namespace can make things messy.${NC}"
        echo ""
                echo -e "${CYAN}Options:${NC}"
                echo -e "  1. Use a different namespace: ${CYAN}ZEN_NAMESPACE=zen-system-2 ./hack/quick-demo.sh${NC}"
                echo -e "  2. Use existing namespace (may cause conflicts)"
                echo -e "  3. Delete existing namespace: ${CYAN}kubectl delete namespace ${NAMESPACE}${NC}"
                echo ""
                
                if [ "$NON_INTERACTIVE" = true ]; then
                    if [ "$DELETE_EXISTING_NAMESPACE_FLAG" = true ]; then
                        REPLY=3
                    elif [ "$USE_EXISTING_NAMESPACE_FLAG" = true ]; then
                        REPLY=2
                    else
                        REPLY=1  # Default: suggest different namespace
                        local suggested_ns=$(find_available_namespace ${NAMESPACE})
                        echo -e "${CYAN}Non-interactive mode: using suggested namespace ${suggested_ns}${NC}"
                        NAMESPACE=${suggested_ns}
                        REPLY=2  # Now use the new namespace
                    fi
                else
                    read -p "$(echo -e ${YELLOW}Choose option [1/2/3] or Ctrl+C to cancel:${NC}) " -n 1 -r
                    echo
                fi
                
                case $REPLY in
                    1)
                        local suggested_ns=$(find_available_namespace ${NAMESPACE})
                        echo -e "${CYAN}Suggested namespace: ${suggested_ns}${NC}"
                        echo -e "${CYAN}Please set ZEN_NAMESPACE=${suggested_ns} and run again${NC}"
                        exit 1
                        ;;
                    2)
                        echo -e "${YELLOW}⚠${NC}  Using existing namespace '${NAMESPACE}' (may cause conflicts)"
                        USE_EXISTING_NAMESPACE=true
                        ;;
                    3)
                        echo -e "${YELLOW}→${NC} Deleting existing namespace..."
                        kubectl delete namespace ${NAMESPACE} --wait=false || {
                            echo -e "${RED}✗${NC} Failed to delete namespace. Please delete manually."
                            exit 1
                        }
                        echo -e "${GREEN}✓${NC} Namespace deletion initiated (will be cleaned up)"
                        sleep 2
                        ;;
                    *)
                        echo -e "${RED}✗${NC} Invalid option. Exiting."
                        exit 1
                        ;;
                esac
    else
        echo -e "${GREEN}✓${NC} Namespace '${NAMESPACE}' is available"
    fi
}

# Function to validate cluster name and check for conflicts
validate_cluster() {
    echo -e "${YELLOW}→${NC} Validating cluster configuration..."
    
    case "$PLATFORM" in
        k3d)
            # Check if cluster name already exists
            if k3d cluster list 2>/dev/null | grep -q "^${CLUSTER_NAME}"; then
                echo -e "${YELLOW}⚠${NC}  k3d cluster '${CLUSTER_NAME}' already exists!"
                echo ""
                echo -e "${YELLOW}Existing k3d clusters:${NC}"
                k3d cluster list 2>/dev/null || true
                echo ""
                echo -e "${CYAN}Options:${NC}"
                echo -e "  1. Use a different cluster name: ${CYAN}ZEN_CLUSTER_NAME=zen-demo-2 ./hack/quick-demo.sh${NC}"
                echo -e "  2. Delete existing cluster: ${CYAN}k3d cluster delete ${CLUSTER_NAME}${NC}"
                echo -e "  3. Use existing cluster (will skip creation)"
                echo ""
                
                if [ "$NON_INTERACTIVE" = true ]; then
                    if [ "$DELETE_EXISTING_CLUSTER_FLAG" = true ]; then
                        REPLY=2
                    elif [ "$USE_EXISTING_CLUSTER_FLAG" = true ]; then
                        REPLY=3
                    else
                        REPLY=3  # Default: use existing
                    fi
                else
                    read -p "$(echo -e ${YELLOW}Choose option [1/2/3] or Ctrl+C to cancel:${NC}) " -n 1 -r
                    echo
                fi
                
                case $REPLY in
                    1)
                        echo -e "${CYAN}Please set ZEN_CLUSTER_NAME and run again${NC}"
                        exit 1
                        ;;
                    2)
                        echo -e "${YELLOW}→${NC} Deleting existing cluster..."
                        timeout 30 k3d cluster delete ${CLUSTER_NAME} || {
                            echo -e "${RED}✗${NC} Failed to delete cluster. Please delete manually."
                            exit 1
                        }
                        echo -e "${GREEN}✓${NC} Cluster deleted"
                        ;;
                    3)
                        echo -e "${CYAN}→${NC} Will use existing cluster"
                        USE_EXISTING_CLUSTER=true
                        ;;
                    *)
                        echo -e "${RED}✗${NC} Invalid option. Exiting."
                        exit 1
                        ;;
                esac
            else
                # Check if any k3d clusters exist (might indicate k3d is in use)
                local existing_clusters=$(k3d cluster list 2>/dev/null | grep -v "^NAME" | wc -l | tr -d '[:space:]' || echo "0")
                existing_clusters=${existing_clusters:-0}
                if [ "$existing_clusters" -gt 0 ]; then
                    echo -e "${CYAN}ℹ${NC}  Found ${existing_clusters} existing k3d cluster(s)"
                    echo -e "${CYAN}   Cluster name '${CLUSTER_NAME}' will be used (no conflicts)${NC}"
                fi
            fi
            ;;
        kind)
            if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
                echo -e "${YELLOW}⚠${NC}  kind cluster '${CLUSTER_NAME}' already exists!"
                echo ""
                echo -e "${YELLOW}Existing kind clusters:${NC}"
                kind get clusters 2>/dev/null || true
                echo ""
                echo -e "${CYAN}Options:${NC}"
                echo -e "  1. Use a different cluster name: ${CYAN}ZEN_CLUSTER_NAME=zen-demo-2 ./hack/quick-demo.sh${NC}"
                echo -e "  2. Delete existing cluster: ${CYAN}kind delete cluster --name ${CLUSTER_NAME}${NC}"
                echo ""
                read -p "$(echo -e ${YELLOW}Choose option [1/2] or Ctrl+C to cancel:${NC}) " -n 1 -r
                echo
                case $REPLY in
                    1)
                        echo -e "${CYAN}Please set ZEN_CLUSTER_NAME and run again${NC}"
                        exit 1
                        ;;
                    2)
                        echo -e "${YELLOW}→${NC} Deleting existing cluster..."
                        kind delete cluster --name ${CLUSTER_NAME} || {
                            echo -e "${RED}✗${NC} Failed to delete cluster. Please delete manually."
                            exit 1
                        }
                        echo -e "${GREEN}✓${NC} Cluster deleted"
                        ;;
                    *)
                        echo -e "${RED}✗${NC} Invalid option. Exiting."
                        exit 1
                        ;;
                esac
            fi
            ;;
        minikube)
            if minikube status -p ${CLUSTER_NAME} &>/dev/null; then
                echo -e "${YELLOW}⚠${NC}  minikube profile '${CLUSTER_NAME}' already exists!"
                echo ""
                echo -e "${CYAN}Options:${NC}"
                echo -e "  1. Use a different profile name: ${CYAN}ZEN_CLUSTER_NAME=zen-demo-2 ./hack/quick-demo.sh${NC}"
                echo -e "  2. Delete existing profile: ${CYAN}minikube delete -p ${CLUSTER_NAME}${NC}"
                echo ""
                
                if [ "$NON_INTERACTIVE" = true ]; then
                    if [ "$DELETE_EXISTING_CLUSTER_FLAG" = true ]; then
                        REPLY=2
                    else
                        REPLY=1  # Default: suggest different name
                        echo -e "${CYAN}Non-interactive mode: please set ZEN_CLUSTER_NAME and run again${NC}"
                        exit 1
                    fi
                else
                    read -p "$(echo -e ${YELLOW}Choose option [1/2] or Ctrl+C to cancel:${NC}) " -n 1 -r
                    echo
                fi
                
                case $REPLY in
                    1)
                        echo -e "${CYAN}Please set ZEN_CLUSTER_NAME and run again${NC}"
                        exit 1
                        ;;
                    2)
                        echo -e "${YELLOW}→${NC} Deleting existing profile..."
                        minikube delete -p ${CLUSTER_NAME} || {
                            echo -e "${RED}✗${NC} Failed to delete profile. Please delete manually."
                            exit 1
                        }
                        echo -e "${GREEN}✓${NC} Profile deleted"
                        ;;
                    *)
                        echo -e "${RED}✗${NC} Invalid option. Exiting."
                        exit 1
                        ;;
                esac
            fi
            ;;
    esac
    
    echo -e "${GREEN}✓${NC} Cluster configuration validated"
}

echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  Zen Watcher - Quick Demo Setup${NC}"
echo -e "${BLUE}  Platform: ${CYAN}${PLATFORM}${NC}"
echo -e "${BLUE}  Cluster: ${CYAN}${CLUSTER_NAME}${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Check prerequisites first (before validation)
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

# Validate everything upfront before making any changes
echo -e "${YELLOW}→${NC} Validating configuration..."
validate_ports
validate_cluster
echo ""
create_cluster() {
    case "$PLATFORM" in
        k3d)
            # Skip creation if we're using an existing cluster
            if [ "${USE_EXISTING_CLUSTER:-false}" = "true" ]; then
                echo -e "${CYAN}→${NC} Using existing k3d cluster '${CLUSTER_NAME}'"
                echo -e "${GREEN}✓${NC} Cluster ready"
                return
            fi
            
            # Double-check cluster doesn't exist (should have been caught in validate_cluster)
            if k3d cluster list 2>/dev/null | grep -q "^${CLUSTER_NAME}"; then
                echo -e "${RED}✗${NC} Cluster '${CLUSTER_NAME}' still exists. This should not happen."
                echo -e "${YELLOW}   Please delete it manually: k3d cluster delete ${CLUSTER_NAME}${NC}"
                exit 1
            fi
            
                echo -e "${YELLOW}→${NC} Creating k3d cluster '${CLUSTER_NAME}' (API port: ${K3D_API_PORT})..."
            
            # Build k3d command - simplified approach like zen-gamma
            # Always use explicit ports to avoid conflicts with multiple clusters
            # Use single server (no agents) for demo - more reliable
            # Disable Traefik (we'll use nginx ingress)
            # Map ingress port for LoadBalancer access
            k3d_create_args=(
                "cluster" "create" "${CLUSTER_NAME}"
                "--agents" "0"
                "--k3s-arg" "--disable=traefik@server:0"
                "--port" "${INGRESS_HTTP_PORT}:80@loadbalancer"
                "--port" "$((INGRESS_HTTP_PORT + 1)):443@loadbalancer"
            )
            
            # Determine API port: Find available port when other clusters exist
            # This prevents port conflicts when multiple k3d clusters run on same host
            local existing_clusters=$(k3d cluster list 2>/dev/null | grep -v "^NAME" | wc -l | tr -d '[:space:]' || echo "0")
            existing_clusters=${existing_clusters:-0}
            if [ "$existing_clusters" -gt 0 ] && [ "${K3D_API_PORT}" = "6443" ]; then
                # Other clusters exist - find first available port in 6550-6560 range
                local found_port=""
                for test_port in {6550..6560}; do
                    if ! ss -tlnp 2>/dev/null | grep -q ":${test_port} "; then
                        found_port=$test_port
                        break
                    fi
                done
                if [ -z "$found_port" ]; then
                    # Fallback: try higher range
                    for test_port in {6561..6570}; do
                        if ! ss -tlnp 2>/dev/null | grep -q ":${test_port} "; then
                            found_port=$test_port
                            break
                        fi
                    done
                fi
                if [ -n "$found_port" ]; then
                    k3d_create_args+=("--api-port" "${found_port}")
                    K3D_API_PORT=${found_port}
                else
                    echo -e "${RED}✗${NC} No available ports found in 6550-6570 range"
                    exit 1
                fi
            elif [ "${K3D_API_PORT}" != "6443" ]; then
                # Custom port specified - verify it's available
                if ss -tlnp 2>/dev/null | grep -q ":${K3D_API_PORT} "; then
                    echo -e "${RED}✗${NC} Port ${K3D_API_PORT} is already in use"
                    exit 1
                fi
                k3d_create_args+=("--api-port" "${K3D_API_PORT}")
                echo -e "${CYAN}   Using custom API port: ${K3D_API_PORT}${NC}"
            elif ! check_port 6443 "k3d API" 2>/dev/null; then
                # Default port is in use, find available port
                local found_port=""
                for test_port in {6550..6560}; do
                    if ! ss -tlnp 2>/dev/null | grep -q ":${test_port} "; then
                        found_port=$test_port
                        break
                    fi
                done
                if [ -n "$found_port" ]; then
                    k3d_create_args+=("--api-port" "${found_port}")
                    K3D_API_PORT=${found_port}
                else
                    echo -e "${RED}✗${NC} Port 6443 in use and no available ports found"
                    exit 1
                fi
            else
                # Default port is free, use it
                k3d_create_args+=("--api-port" "${K3D_API_PORT}")
            fi
            
            # Create cluster - use longer timeout and let it finish
            # If --no-docker-login, ensure k3d doesn't use docker login credentials
            if [ "$NO_DOCKER_LOGIN" = true ]; then
                # Unset docker config to avoid using docker login credentials
                export DOCKER_CONFIG=""
            fi
            if timeout 240 k3d "${k3d_create_args[@]}" 2>&1 | tee /tmp/k3d-create.log; then
                echo -e "${GREEN}✓${NC} Cluster creation completed"
            else
                local exit_code=$?
                # Check if cluster was actually created despite timeout
                if k3d cluster list 2>/dev/null | grep -q "^${CLUSTER_NAME}"; then
                    echo -e "${YELLOW}⚠${NC}  Cluster creation timed out, but cluster exists - continuing...${NC}"
                else
                    echo -e "${RED}✗${NC} Cluster creation failed"
                    echo -e "${YELLOW}   Check logs: cat /tmp/k3d-create.log${NC}"
                    if [ $exit_code -eq 124 ]; then
                        echo -e "${RED}   Timeout: Cluster creation took longer than 4 minutes${NC}"
                    fi
                    if grep -q -i "port.*in use\|bind.*address\|address already in use\|failed to.*port" /tmp/k3d-create.log 2>/dev/null; then
                        echo -e "${RED}   Port conflict detected!${NC}"
                    fi
                    exit 1
                fi
            fi
            ;;
        kind)
            # Double-check cluster doesn't exist
            if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
                echo -e "${RED}✗${NC} Cluster '${CLUSTER_NAME}' still exists. This should not happen."
                exit 1
            fi
            
                echo -e "${YELLOW}→${NC} Creating kind cluster '${CLUSTER_NAME}'..."
            echo -e "${CYAN}   API Port: ${KIND_API_PORT}${NC}"
            
            # Create kind config with API port (always explicit to avoid conflicts)
            cat > /tmp/kind-config-${CLUSTER_NAME}.yaml <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  apiServerPort: ${KIND_API_PORT}
EOF
            
            if timeout 180 kind create cluster --name ${CLUSTER_NAME} --config /tmp/kind-config-${CLUSTER_NAME}.yaml --wait 2m 2>&1 | tee /tmp/kind-create.log; then
                rm -f /tmp/kind-config-${CLUSTER_NAME}.yaml
                echo -e "${GREEN}✓${NC} Cluster created successfully"
            else
                local exit_code=$?
                rm -f /tmp/kind-config-${CLUSTER_NAME}.yaml
                echo -e "${RED}✗${NC} Cluster creation failed or timed out"
                echo -e "${YELLOW}   Check logs: cat /tmp/kind-create.log${NC}"
                if [ $exit_code -eq 124 ]; then
                    echo -e "${RED}   Timeout: Cluster creation took longer than 3 minutes${NC}"
                    echo -e "${YELLOW}   This might indicate port conflicts or resource issues${NC}"
                    echo -e "${CYAN}   Try using a different API port: KIND_API_PORT=7443 ./hack/quick-demo.sh kind${NC}"
                else
                    # Check if error is port-related
                    if grep -q -i "port.*in use\|bind.*address\|address already in use\|failed to.*port" /tmp/kind-create.log 2>/dev/null; then
                        echo -e "${RED}   Port conflict detected!${NC}"
                        echo -e "${CYAN}   Try using a different API port: KIND_API_PORT=7443 ./hack/quick-demo.sh kind${NC}"
                        echo -e "${CYAN}   Or check existing clusters: kind get clusters${NC}"
                    fi
                fi
                exit 1
            fi
            ;;
        minikube)
            # Double-check profile doesn't exist
            if minikube status -p ${CLUSTER_NAME} &>/dev/null; then
                echo -e "${RED}✗${NC} Profile '${CLUSTER_NAME}' still exists. This should not happen."
                exit 1
            fi
            
                echo -e "${YELLOW}→${NC} Creating minikube cluster '${CLUSTER_NAME}'..."
            echo -e "${CYAN}   API Port: ${MINIKUBE_API_PORT}${NC}"
            
            if timeout 300 minikube start -p ${CLUSTER_NAME} \
                --cpus 4 \
                --memory 8192 \
                --apiserver-port=${MINIKUBE_API_PORT} 2>&1 | tee /tmp/minikube-create.log; then
                echo -e "${GREEN}✓${NC} Cluster created successfully"
            else
                local exit_code=$?
                echo -e "${RED}✗${NC} Cluster creation failed or timed out"
                echo -e "${YELLOW}   Check logs: cat /tmp/minikube-create.log${NC}"
                if [ $exit_code -eq 124 ]; then
                    echo -e "${RED}   Timeout: Cluster creation took longer than 5 minutes${NC}"
                    echo -e "${YELLOW}   This might indicate port conflicts or resource issues${NC}"
                    echo -e "${CYAN}   Try using a different API port: MINIKUBE_API_PORT=9443 ./hack/quick-demo.sh minikube${NC}"
                else
                    # Check if error is port-related
                    if grep -q -i "port.*in use\|bind.*address\|address already in use\|failed to.*port\|apiserver.*port" /tmp/minikube-create.log 2>/dev/null; then
                        echo -e "${RED}   Port conflict detected!${NC}"
                        echo -e "${CYAN}   Try using a different API port: MINIKUBE_API_PORT=9443 ./hack/quick-demo.sh minikube${NC}"
                        echo -e "${CYAN}   Or check existing profiles: minikube profile list${NC}"
                    fi
                fi
                exit 1
            fi
            ;;
    esac
}

SECTION_START_TIME=$(date +%s)
create_cluster
show_section_time "Cluster creation"
echo -e "${YELLOW}→${NC} Setting up kubeconfig..."
SECTION_START_TIME=$(date +%s)
case "$PLATFORM" in
    k3d)
        # Wait a moment for k3d to finish setting up
        sleep 3
        
        # Detect actual port - wait a bit for serverlb to be ready, then detect
        sleep 5  # Give serverlb time to start
        
        ACTUAL_PORT=""
        # Method 1: Try to get port from docker port command (most reliable)
        for wait_attempt in {1..10}; do
            ACTUAL_PORT=$(docker port k3d-${CLUSTER_NAME}-serverlb 2>/dev/null | grep "6443/tcp" | cut -d: -f2 | tr -d ' ' | head -1 || echo "")
            if [ -n "$ACTUAL_PORT" ] && [ "$ACTUAL_PORT" != "" ]; then
                break
            fi
            sleep 1
        done
        
        # Method 2: Try docker inspect
        if [ -z "$ACTUAL_PORT" ] || [ "$ACTUAL_PORT" = "" ]; then
            ACTUAL_PORT=$(docker inspect k3d-${CLUSTER_NAME}-serverlb 2>/dev/null | jq -r '.[0].NetworkSettings.Ports."6443/tcp"[0].HostPort // empty' 2>/dev/null || echo "")
        fi
        
        # Method 3: Check listening ports (fallback - check common k3d ports)
        if [ -z "$ACTUAL_PORT" ] || [ "$ACTUAL_PORT" = "" ] || [ "$ACTUAL_PORT" = "null" ]; then
            for test_port in 6550 6551 6552 6553 6554 6555 6443 6444 6445; do
                if ss -tlnp 2>/dev/null | grep -q ":${test_port}"; then
                    # Verify it's actually k3d by checking if kubectl can connect
                    if timeout 3 kubectl --server="https://127.0.0.1:${test_port}" --insecure-skip-tls-verify get nodes --request-timeout=2s &>/dev/null 2>&1; then
                        ACTUAL_PORT=$test_port
                        break
                    fi
                fi
            done
        fi
        
        if [ -n "$ACTUAL_PORT" ] && [ "$ACTUAL_PORT" != "null" ] && [ "$ACTUAL_PORT" != "" ]; then
            K3D_API_PORT=$ACTUAL_PORT
        fi
        
        echo -e "${CYAN}   Setting up kubeconfig for port ${K3D_API_PORT}...${NC}"
        
        # Use separate kubeconfig file - don't modify default ~/.kube/config
        # Create separate kubeconfig file early to avoid touching default config
        KUBECONFIG_FILE="${HOME}/.kube/${CLUSTER_NAME}-kubeconfig"
        mkdir -p "${HOME}/.kube" 2>/dev/null || true
        
        # Write kubeconfig to separate file (don't merge into default)
        if ! timeout 10 k3d kubeconfig write ${CLUSTER_NAME} --output ${KUBECONFIG_FILE} 2>/dev/null; then
            # Fallback: write to temp file first, then copy
            timeout 10 k3d kubeconfig write ${CLUSTER_NAME} > /tmp/k3d-kubeconfig-${CLUSTER_NAME} 2>/dev/null
            if [ -f /tmp/k3d-kubeconfig-${CLUSTER_NAME} ]; then
                cp /tmp/k3d-kubeconfig-${CLUSTER_NAME} ${KUBECONFIG_FILE} 2>/dev/null || true
            fi
        fi
        
        # Use the separate kubeconfig file for all operations
        export KUBECONFIG=${KUBECONFIG_FILE}
        
        # Fix kubeconfig server URL - always use 127.0.0.1 and the port we specified/detected
        timeout 5 kubectl config set clusters.k3d-${CLUSTER_NAME}.server "https://127.0.0.1:${K3D_API_PORT}" --kubeconfig=${KUBECONFIG_FILE} 2>/dev/null || true
        
        # CRITICAL: k3d uses self-signed certificates, so we need to skip TLS verification
        # Remove certificate-authority-data if present (conflicts with insecure-skip-tls-verify)
        timeout 5 kubectl config unset clusters.k3d-${CLUSTER_NAME}.certificate-authority-data --kubeconfig=${KUBECONFIG_FILE} 2>/dev/null || true
        timeout 5 kubectl config set clusters.k3d-${CLUSTER_NAME}.insecure-skip-tls-verify true --kubeconfig=${KUBECONFIG_FILE} 2>/dev/null || true
        
        # Wait for cluster API to be accessible (don't wait for nodes - they may take longer)
        CLUSTER_READY=false
        for wait_attempt in {1..60}; do
            # Test API access directly (more reliable than waiting for nodes)
            if timeout 5 kubectl get --raw /api/v1 2>/dev/null | grep -q "kind\|versions"; then
                echo -e "${GREEN}✓${NC} Cluster API is accessible (after $((wait_attempt*2)) seconds)"
                CLUSTER_READY=true
                break
            fi
            sleep 2
        done
        
        if [ "$CLUSTER_READY" = false ]; then
            echo -e "${YELLOW}⚠${NC}  Cluster API may not be fully ready, but continuing with authentication...${NC}"
        fi
        
        # Now verify connectivity with retries
        CLUSTER_ACCESSIBLE=false
        for retry in {1..20}; do
            # Test API access (more reliable than checking nodes)
            if timeout 10 kubectl get --raw /api/v1 --request-timeout=5s 2>/dev/null | grep -q "kind\|versions"; then
                echo -e "${GREEN}✓${NC} Cluster API is accessible"
                CLUSTER_ACCESSIBLE=true
                break
            fi
            # Also try nodes as fallback
            if timeout 10 kubectl get nodes --request-timeout=5s > /dev/null 2>&1; then
                echo -e "${GREEN}✓${NC} Cluster is accessible"
                CLUSTER_ACCESSIBLE=true
                break
            fi
            
            if [ $retry -lt 20 ]; then
                # Update the separate kubeconfig file (don't touch default config)
                timeout 10 k3d kubeconfig write ${CLUSTER_NAME} --output ${KUBECONFIG_FILE} 2>&1 | grep -v "ERRO" > /dev/null || true
                # CRITICAL: Always fix 0.0.0.0 to 127.0.0.1
                timeout 5 kubectl config set clusters.k3d-${CLUSTER_NAME}.server "https://127.0.0.1:${K3D_API_PORT}" 2>&1 > /dev/null || true
                timeout 5 kubectl config set clusters.k3d-${CLUSTER_NAME}.insecure-skip-tls-verify true 2>&1 > /dev/null || true
                timeout 5 kubectl config unset clusters.k3d-${CLUSTER_NAME}.certificate-authority-data 2>&1 > /dev/null || true
                sleep 2
            fi
        done
        if [ "$CLUSTER_ACCESSIBLE" = false ]; then
            echo -e "${RED}✗${NC} Cannot access cluster after 20 retries"
            echo -e "${YELLOW}   Cluster may still be starting. Check with: kubectl get nodes${NC}"
            exit 1
        fi
        if timeout 5 kubectl get nodes --request-timeout=5s --kubeconfig=${KUBECONFIG_FILE} &>/dev/null 2>&1; then
            echo -e "${GREEN}✓${NC} Cluster connectivity verified"
        else
            echo -e "${YELLOW}⚠${NC}  Cluster connectivity check failed, but continuing...${NC}"
        fi
        ;;
    kind)
        # kind automatically updates kubeconfig, but ensure context is set
        kind export kubeconfig --name ${CLUSTER_NAME} 2>/dev/null || true
        kubectl config use-context kind-${CLUSTER_NAME} 2>/dev/null || true
        ;;
    minikube)
        # minikube automatically sets context
        minikube update-context -p ${CLUSTER_NAME} &>/dev/null || true
        eval $(minikube -p ${CLUSTER_NAME} docker-env 2>/dev/null) || true
        ;;
esac

# Helper function to retry kubectl commands
kubectl_retry() {
    local max_attempts=30
    local attempt=1
    while [ $attempt -le $max_attempts ]; do
        if kubectl "$@" --request-timeout=5s &>/dev/null 2>&1; then
            return 0
        fi
        sleep 2
        attempt=$((attempt + 1))
    done
    # Last attempt without timeout suppression to show error
    kubectl "$@" --request-timeout=5s 2>&1 || true
    return 1
}

# Wait for cluster to be ready (with retries and better error handling)
echo -e "${YELLOW}→${NC} Waiting for cluster to be ready..."
cluster_ready=false
max_wait=20
for i in $(seq 1 $max_wait); do
    # Try to check cluster readiness
    if kubectl get nodes --request-timeout=5s &>/dev/null 2>&1; then
        cluster_ready=true
        echo -e "${GREEN}✓${NC} Cluster is ready"
        show_section_time "Cluster readiness"
        break
    fi
    
    # Show progress every 5 iterations
    if [ $((i % 5)) -eq 0 ]; then
        echo -e "${CYAN}   ... still waiting ($((i*3)) seconds)${NC}"
    fi
    
    # Break after max_wait iterations
    if [ $i -eq $max_wait ]; then
        echo -e "${YELLOW}⚠${NC}  Cluster API not fully ready after $((max_wait*3)) seconds"
        echo -e "${CYAN}   Continuing anyway - operations will retry automatically...${NC}"
        show_section_time "Cluster readiness (timeout)"
        break
    fi
    
    sleep 3
done

# Use the separate kubeconfig file we created earlier (or create it now)
if [ -z "${KUBECONFIG_FILE:-}" ]; then
    KUBECONFIG_FILE="${HOME}/.kube/${CLUSTER_NAME}-kubeconfig"
fi

# Ensure kubeconfig file exists and is up to date
# Set up kubeconfig file (silently)
case "$PLATFORM" in
    k3d)
        # Update the separate kubeconfig file (don't touch default config)
        if ! timeout 10 k3d kubeconfig write ${CLUSTER_NAME} --output ${KUBECONFIG_FILE} 2>/dev/null; then
            # Fallback: try to find the k3d config file
            K3D_CONFIG_PATH="${HOME}/.config/k3d/kubeconfig-${CLUSTER_NAME}.yaml"
            if [ -f "$K3D_CONFIG_PATH" ]; then
                cp "$K3D_CONFIG_PATH" ${KUBECONFIG_FILE} 2>/dev/null || true
            fi
        fi
        # Fix server URL to use 127.0.0.1
        if [ -f "${KUBECONFIG_FILE}" ]; then
            sed -i.bak "s|0.0.0.0:${K3D_API_PORT}|127.0.0.1:${K3D_API_PORT}|g" ${KUBECONFIG_FILE} 2>/dev/null || true
            sed -i.bak "s|server: https://.*:${K3D_API_PORT}|server: https://127.0.0.1:${K3D_API_PORT}|g" ${KUBECONFIG_FILE} 2>/dev/null || true
            rm -f ${KUBECONFIG_FILE}.bak 2>/dev/null || true
            # Remove certificate authority data and add insecure skip
            kubectl config unset clusters.k3d-${CLUSTER_NAME}.certificate-authority-data --kubeconfig=${KUBECONFIG_FILE} 2>/dev/null || true
            kubectl config set clusters.k3d-${CLUSTER_NAME}.insecure-skip-tls-verify true --kubeconfig=${KUBECONFIG_FILE} 2>/dev/null || true
        fi
        ;;
    kind)
        kind export kubeconfig --name ${CLUSTER_NAME} --kubeconfig=${KUBECONFIG_FILE} 2>/dev/null || true
        ;;
    minikube)
        minikube update-context -p ${CLUSTER_NAME} 2>/dev/null || true
        cp ${HOME}/.kube/config ${KUBECONFIG_FILE} 2>/dev/null || true
        ;;
esac
if [ -f "${KUBECONFIG_FILE}" ]; then
    chmod 600 ${KUBECONFIG_FILE} 2>/dev/null || true
fi

# Final attempt to verify cluster is ready
if [ "$cluster_ready" = false ] && [ "$PLATFORM" = "k3d" ]; then
    echo -e "${CYAN}   Verifying cluster connectivity on port ${K3D_API_PORT}...${NC}"
    sleep 2
    if timeout 5 kubectl get nodes --request-timeout=5s &>/dev/null 2>&1; then
        cluster_ready=true
        echo -e "${GREEN}✓${NC} Cluster is ready"
    else
        echo -e "${YELLOW}⚠${NC}  Cluster may not be fully ready, but continuing...${NC}"
    fi
fi

# Validate namespace now that we have cluster access
echo ""
validate_namespace
echo ""

# Deploy Security Tools (default: install all for comprehensive demo)
# If no flags set, install all tools by default
if [ "$INSTALL_TRIVY" = false ] && [ "$INSTALL_FALCO" = false ] && [ "$INSTALL_KYVERNO" = false ] && [ "$INSTALL_CHECKOV" = false ] && [ "$INSTALL_KUBE_BENCH" = false ]; then
    # Default: install all tools for comprehensive demo
    INSTALL_TRIVY=true
    INSTALL_FALCO=true
    INSTALL_KYVERNO=true
    INSTALL_CHECKOV=true
    INSTALL_KUBE_BENCH=true
    echo -e "${CYAN}ℹ${NC}  No security tools specified - installing all tools for comprehensive demo"
fi

# Build component list based on flags
# Format: "namespace|name"
COMPONENTS=()

# Always install ingress and zen-watcher
COMPONENTS+=("ingress-nginx|Ingress Controller")
COMPONENTS+=("${NAMESPACE}|Zen Watcher")

# Monitoring stack (Grafana and VictoriaMetrics go together)
if [ "$SKIP_MONITORING" != true ]; then
    COMPONENTS+=("${NAMESPACE}|VictoriaMetrics")
    COMPONENTS+=("${NAMESPACE}|Grafana")
fi

# Security tools
[ "$INSTALL_TRIVY" = true ] && COMPONENTS+=("trivy-system|Trivy Operator")
[ "$INSTALL_FALCO" = true ] && COMPONENTS+=("falco|Falco")
[ "$INSTALL_KYVERNO" = true ] && COMPONENTS+=("kyverno|Kyverno")
[ "$INSTALL_CHECKOV" = true ] && COMPONENTS+=("checkov|Checkov")
[ "$INSTALL_KUBE_BENCH" = true ] && COMPONENTS+=("kube-bench|kube-bench")

echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  Installing All Components${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
SECTION_START_TIME=$(date +%s)

# Install ingress first (other components may need it)
echo -e "${YELLOW}→${NC} Installing Nginx Ingress Controller..."
# Ensure kubeconfig is properly configured (using separate file only, never touch default config)
for retry in {1..5}; do
    # Update the separate kubeconfig file if needed
    if [ -n "${KUBECONFIG_FILE:-}" ] && [ -f "${KUBECONFIG_FILE}" ]; then
        export KUBECONFIG=${KUBECONFIG_FILE}
    else
        # Create it if it doesn't exist
        KUBECONFIG_FILE="${HOME}/.kube/${CLUSTER_NAME}-kubeconfig"
        timeout 10 k3d kubeconfig write ${CLUSTER_NAME} --output ${KUBECONFIG_FILE} 2>&1 | grep -v "ERRO" > /dev/null || true
        export KUBECONFIG=${KUBECONFIG_FILE}
    fi
    timeout 5 kubectl config set clusters.k3d-${CLUSTER_NAME}.server "https://127.0.0.1:${K3D_API_PORT}" --kubeconfig=${KUBECONFIG_FILE} 2>&1 > /dev/null || true
    timeout 5 kubectl config set clusters.k3d-${CLUSTER_NAME}.insecure-skip-tls-verify true --kubeconfig=${KUBECONFIG_FILE} 2>&1 > /dev/null || true
    timeout 5 kubectl config unset clusters.k3d-${CLUSTER_NAME}.certificate-authority-data --kubeconfig=${KUBECONFIG_FILE} 2>&1 > /dev/null || true
    
    if timeout 10 kubectl get nodes --request-timeout=5s --kubeconfig=${KUBECONFIG_FILE} > /dev/null 2>&1; then
        break
    fi
    if [ $retry -lt 5 ]; then
        sleep 2
    fi
done

# Add ingress-nginx helm repo if not already added
if ! timeout 10 helm repo list 2>/dev/null | grep -q ingress-nginx; then
    timeout 30 helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx 2>&1 || true
    timeout 30 helm repo update 2>&1 || true
fi

# Install nginx ingress (non-blocking)
if ! timeout 10 helm list -n ingress-nginx 2>&1 | grep -q ingress-nginx; then
    timeout 120 helm install ingress-nginx ingress-nginx/ingress-nginx \
        --namespace ingress-nginx \
        --create-namespace \
        --set controller.service.type=LoadBalancer \
        --set controller.service.annotations."k3d\.io/loadbalancer"=true \
        --set controller.admissionWebhooks.enabled=false \
        --set controller.admissionWebhooks.patch.enabled=false \
        --set controller.podLabels.app=ingress-nginx \
        --set controller.podLabels."app\.kubernetes\.io/name"=ingress-nginx \
        >/dev/null 2>&1 &
    # Delete admission webhooks if created
    sleep 2
    kubectl delete validatingwebhookconfiguration ingress-nginx-admission 2>&1 | grep -v "not found" > /dev/null || true
    kubectl delete mutatingwebhookconfiguration ingress-nginx-admission 2>&1 | grep -v "not found" > /dev/null || true
fi

# Create ingress resources (only if monitoring is enabled)
if [ "$SKIP_MONITORING" != true ]; then
    echo -e "${YELLOW}→${NC} Creating ingress resources..."
    kubectl create namespace ${NAMESPACE} 2>/dev/null || true
    sleep 1
    
    # Create ingress with host-based routing and path rewriting
    cat <<EOF | timeout 30 kubectl apply -f - 2>&1 | grep -v "already exists\|unchanged" > /dev/null || true
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: zen-demo-grafana
  namespace: ${NAMESPACE}
  annotations:
    nginx.ingress.kubernetes.io/ssl-redirect: "false"
spec:
  ingressClassName: nginx
  rules:
  - host: localhost
    http:
      paths:
      - path: /grafana
        pathType: Prefix
        backend:
          service:
            name: grafana
            port:
              number: 3000
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: zen-demo-services
  namespace: ${NAMESPACE}
  annotations:
    nginx.ingress.kubernetes.io/ssl-redirect: "false"
    nginx.ingress.kubernetes.io/rewrite-target: /\$2
    nginx.ingress.kubernetes.io/use-regex: "true"
spec:
  ingressClassName: nginx
  rules:
  - host: localhost
    http:
      paths:
      - path: /victoriametrics(/|$)(.*)
        pathType: ImplementationSpecific
        backend:
          service:
            name: victoriametrics
            port:
              number: 8428
      - path: /zen-watcher(/|$)(.*)
        pathType: ImplementationSpecific
        backend:
          service:
            name: zen-watcher
            port:
              number: 8080
EOF
    echo -e "${GREEN}✓${NC} Ingress resources created"
fi

# Create namespace
if [ "${USE_EXISTING_NAMESPACE:-false}" != "true" ]; then
    kubectl create namespace ${NAMESPACE} 2>/dev/null || true
fi

if [ "$INSTALL_TRIVY" = true ] || [ "$INSTALL_FALCO" = true ] || [ "$INSTALL_KYVERNO" = true ] || [ "$INSTALL_CHECKOV" = true ] || [ "$INSTALL_KUBE_BENCH" = true ]; then

    # Add Helm repositories (only if needed)
    if [ "$INSTALL_TRIVY" = true ] || [ "$INSTALL_FALCO" = true ] || [ "$INSTALL_KYVERNO" = true ]; then
        [ "$INSTALL_TRIVY" = true ] && helm repo add aqua https://aquasecurity.github.io/helm-charts 2>/dev/null || true
        [ "$INSTALL_FALCO" = true ] && helm repo add falcosecurity https://falcosecurity.github.io/charts 2>/dev/null || true
        [ "$INSTALL_KYVERNO" = true ] && helm repo add kyverno https://kyverno.github.io/kyverno/ 2>/dev/null || true
        helm repo update > /dev/null 2>&1 &
    fi

    # Deploy Trivy Operator
    if [ "$INSTALL_TRIVY" = true ]; then
        echo -e "${YELLOW}→${NC} Deploying Trivy Operator..."
        helm upgrade --install trivy-operator aqua/trivy-operator \
            --namespace trivy-system \
            --create-namespace \
            --set="trivy.ignoreUnfixed=true" \
            > /dev/null 2>&1 &
    fi

    # Deploy Falco with resource limits to reduce CPU usage
    if [ "$INSTALL_FALCO" = true ]; then
        echo -e "${YELLOW}→${NC} Deploying Falco..."
        helm upgrade --install falco falcosecurity/falco \
            --namespace falco \
            --create-namespace \
            --set falcosidekick.enabled=false \
            --set falco.httpOutput.enabled=true \
            --set falco.httpOutput.url=http://zen-watcher.${NAMESPACE}.svc.cluster.local:8080/falco/webhook \
            --set driver.enabled=false \
            --set resources.requests.cpu=100m \
            --set resources.requests.memory=128Mi \
            --set resources.limits.cpu=500m \
            --set resources.limits.memory=512Mi \
            > /dev/null 2>&1 &
    fi

    # Deploy Kyverno
    if [ "$INSTALL_KYVERNO" = true ]; then
        echo -e "${YELLOW}→${NC} Deploying Kyverno..."
        helm upgrade --install kyverno kyverno/kyverno \
            --namespace kyverno \
            --create-namespace \
            --set replicaCount=1 \
            > /dev/null 2>&1 &
        
        # Create a test Kyverno policy that requires labels
        cat <<EOF | kubectl apply -f - 2>&1 | grep -v "already exists" > /dev/null || true
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: require-labels
spec:
  validationFailureAction: enforce
  rules:
  - name: check-label-app
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "The label 'app' is required."
      pattern:
        metadata:
          labels:
            app: "?*"
EOF
    fi

    # Deploy Checkov as a Kubernetes Job
    if [ "$INSTALL_CHECKOV" = true ]; then
        echo -e "${YELLOW}→${NC} Deploying Checkov scanning job..."
        kubectl create namespace checkov 2>/dev/null || true
        
        # Create ConfigMap with demo manifests for Checkov to scan
        echo -e "${CYAN}   Creating demo manifests ConfigMap...${NC}"
        kubectl create configmap demo-manifests \
            --from-file=config/demo-manifests/ \
            -n checkov 2>/dev/null || \
        kubectl create configmap demo-manifests \
            --from-file=config/demo-manifests/ \
            -n checkov --dry-run=client -o yaml | kubectl apply -f - 2>/dev/null || true
        
        cat <<EOF | kubectl apply -f - 2>/dev/null || true
apiVersion: batch/v1
kind: Job
metadata:
  name: checkov-scan-demo
  namespace: checkov
  labels:
    demo.zen.kube-zen.io/job: "true"
spec:
  template:
    metadata:
      labels:
        demo.zen.kube-zen.io/job: "true"
    spec:
      containers:
      - name: checkov
        image: bridgecrew/checkov:latest
        command: ["checkov", "-d", "/k8s", "--framework", "kubernetes", "--output", "json", "--quiet"]
        volumeMounts:
        - name: demo-manifests
          mountPath: /k8s
      volumes:
      - name: demo-manifests
        configMap:
          name: demo-manifests
      restartPolicy: Never
  backoffLimit: 1
EOF
        echo -e "${GREEN}✓${NC} Checkov job created"
        echo -e "${CYAN}   View results: kubectl logs job/checkov-scan-demo -n checkov${NC}"
        echo -e "${CYAN}   Note: Demo manifests are in config/demo-manifests/ (labeled with demo.zen.kube-zen.io)${NC}"
    fi

    # Deploy kube-bench as a Kubernetes Job
    if [ "$INSTALL_KUBE_BENCH" = true ]; then
        echo -e "${YELLOW}→${NC} Deploying kube-bench CIS benchmark job..."
        kubectl create namespace kube-bench 2>/dev/null || true
        
        # Detect cluster type for kube-bench benchmark target
        bench_target="cis-1.6"
        
        # Try to detect cluster type
        if kubectl get nodes --no-headers 2>/dev/null | head -1 | grep -q "k3d"; then
            bench_target="cis-1.6"
        elif kubectl get nodes --no-headers 2>/dev/null | head -1 | grep -q "kind"; then
            bench_target="cis-1.6"
        else
            bench_target="cis-1.6"
        fi
        
        # Create a simpler kube-bench job that runs in a pod with node access
        # Note: This requires hostPath access which may not work in all environments
        # For k3d/kind, we'll use a node selector to run on a control plane node
        cat <<EOF | kubectl apply -f - 2>/dev/null || true
apiVersion: batch/v1
kind: Job
metadata:
  name: kube-bench
  namespace: kube-bench
spec:
  template:
    spec:
      hostPID: true
      hostNetwork: true
      containers:
      - name: kube-bench
        image: aquasec/kube-bench:latest
        command: ["kube-bench", "run", "--targets", "node", "--benchmark", "${bench_target}"]
        securityContext:
          privileged: true
        volumeMounts:
        - name: var-lib-kubelet
          mountPath: /var/lib/kubelet
          readOnly: true
        - name: etc-systemd
          mountPath: /etc/systemd
          readOnly: true
        - name: etc-kubernetes
          mountPath: /etc/kubernetes
          readOnly: true
        - name: usr-bin
          mountPath: /usr/local/mount-from-host/bin
          readOnly: true
      volumes:
      - name: var-lib-kubelet
        hostPath:
          path: "/var/lib/kubelet"
      - name: etc-systemd
        hostPath:
          path: "/etc/systemd"
      - name: etc-kubernetes
        hostPath:
          path: "/etc/kubernetes"
      - name: usr-bin
        hostPath:
          path: "/usr/local/bin"
      restartPolicy: Never
      tolerations:
      - effect: NoSchedule
        operator: Exists
EOF
        echo -e "${GREEN}✓${NC} kube-bench job created"
        echo -e "${CYAN}   Note: kube-bench requires host access and may not work in all environments${NC}"
        echo -e "${CYAN}   View results: kubectl logs job/kube-bench -n kube-bench${NC}"
    fi
fi

# Deploy VictoriaMetrics (only if monitoring is enabled)
if [ "$SKIP_MONITORING" != true ]; then
    echo -e "${YELLOW}→${NC} Deploying VictoriaMetrics..."

# Create ConfigMap for VictoriaMetrics scrape configuration
cat <<EOF | kubectl apply -f - 2>&1 | grep -v "already exists\|unchanged" > /dev/null || true
apiVersion: v1
kind: ConfigMap
metadata:
  name: victoriametrics-scrape-config
  namespace: ${NAMESPACE}
data:
  scrape.yml: |
    global:
      scrape_interval: 15s
      evaluation_interval: 15s
    
    scrape_configs:
      - job_name: 'zen-watcher'
        static_configs:
          - targets: ['zen-watcher.${NAMESPACE}.svc.cluster.local:9090']
        metrics_path: /metrics
        scrape_interval: 15s
EOF

# Deploy VictoriaMetrics with path prefix and scrape config using full YAML
cat <<EOF | kubectl apply -f - 2>&1 | grep -v "already exists\|unchanged" > /dev/null || true
apiVersion: apps/v1
kind: Deployment
metadata:
  name: victoriametrics
  namespace: ${NAMESPACE}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: victoriametrics
  template:
    metadata:
      labels:
        app: victoriametrics
    spec:
      containers:
      - name: victoriametrics
        image: victoriametrics/victoria-metrics:latest
        args:
          - -http.pathPrefix=/victoriametrics
          - -promscrape.config=/etc/vm/scrape.yml
        ports:
        - containerPort: 8428
          name: http
        volumeMounts:
        - name: scrape-config
          mountPath: /etc/vm
          readOnly: true
      volumes:
      - name: scrape-config
        configMap:
          name: victoriametrics-scrape-config
EOF
# Expose VictoriaMetrics as ClusterIP (ingress will handle routing)
if kubectl get svc victoriametrics -n ${NAMESPACE} &>/dev/null; then
    kubectl delete svc victoriametrics -n ${NAMESPACE} 2>&1 | grep -v "not found" > /dev/null || true
fi
kubectl expose deployment victoriametrics \
    --port=8428 --target-port=8428 \
    --type=ClusterIP \
    --name=victoriametrics \
    -n ${NAMESPACE} 2>&1 | grep -v "already exists" > /dev/null || true

# Deploy Grafana with zen user
echo -e "${YELLOW}→${NC} Deploying Grafana..."
kubectl create deployment grafana \
    --image=grafana/grafana:latest \
    -n ${NAMESPACE} \
    --dry-run=client -o yaml 2>/dev/null | \
kubectl set env --local -f - \
    GF_SECURITY_ADMIN_USER=zen \
    GF_SECURITY_ADMIN_PASSWORD=${GRAFANA_PASSWORD} \
    GF_USERS_ALLOW_SIGN_UP=false \
    GF_USERS_DEFAULT_THEME=dark \
    GF_SERVER_ROOT_URL=http://localhost:${INGRESS_HTTP_PORT}/grafana/ \
    GF_SERVER_SERVE_FROM_SUB_PATH=true \
    GF_SERVER_DOMAIN=localhost \
    --dry-run=client -o yaml 2>/dev/null | \
kubectl apply -f - 2>&1 | grep -v "already exists" > /dev/null || true

# Update env vars if deployment exists
kubectl set env deployment/grafana \
    GF_SECURITY_ADMIN_USER=zen \
    GF_SECURITY_ADMIN_PASSWORD=${GRAFANA_PASSWORD} \
    GF_USERS_ALLOW_SIGN_UP=false \
    GF_USERS_DEFAULT_THEME=dark \
    GF_SERVER_ROOT_URL=http://localhost:${INGRESS_HTTP_PORT}/grafana/ \
    GF_SERVER_SERVE_FROM_SUB_PATH=true \
    GF_SERVER_DOMAIN=localhost \
    -n ${NAMESPACE} 2>/dev/null || true

    # Expose Grafana as ClusterIP
    if kubectl get svc grafana -n ${NAMESPACE} &>/dev/null; then
        kubectl delete svc grafana -n ${NAMESPACE} 2>&1 || true
    fi
    kubectl expose deployment grafana \
        --port=3000 --target-port=3000 \
        --type=ClusterIP \
        --name=grafana \
        -n ${NAMESPACE} 2>&1 | grep -v "already exists" > /dev/null || true
fi

# Deploy Zen Watcher using Helm chart
echo -e "${YELLOW}→${NC} Deploying Zen Watcher..."
ZEN_WATCHER_IMAGE="${ZEN_WATCHER_IMAGE:-kubezen/zen-watcher:latest}"

# Set image pull policy based on --no-docker-login flag
if [ "$NO_DOCKER_LOGIN" = true ]; then
    # Use Always to force pull from public registry without docker login
    IMAGE_PULL_POLICY="Always"
    echo -e "${CYAN}   Using public registry (no docker login credentials)${NC}"
else
    IMAGE_PULL_POLICY="IfNotPresent"
fi

# Extract image repository and tag from image string
if echo "$ZEN_WATCHER_IMAGE" | grep -q ":"; then
    IMAGE_TAG=$(echo "$ZEN_WATCHER_IMAGE" | cut -d: -f2)
    IMAGE_REPO=$(echo "$ZEN_WATCHER_IMAGE" | cut -d: -f1)
else
    IMAGE_TAG="latest"
    IMAGE_REPO="$ZEN_WATCHER_IMAGE"
fi

# Try to get latest image tag from Docker Hub or use latest
if [ "$ZEN_WATCHER_IMAGE" = "kubezen/zen-watcher:latest" ]; then
    echo -e "${CYAN}   Using image: ${ZEN_WATCHER_IMAGE}${NC}"
fi

# Deploy using Helm chart (includes CRDs, RBAC, Service, Deployment, and VMServiceScrape)
helm upgrade --install zen-watcher ./charts/zen-watcher \
    --namespace ${NAMESPACE} \
    --create-namespace \
    --set image.repository="${IMAGE_REPO}" \
    --set image.tag="${IMAGE_TAG}" \
    --set image.pullPolicy="${IMAGE_PULL_POLICY}" \
    --set config.watchNamespace="${NAMESPACE}" \
    --set config.trivyNamespace="trivy-system" \
    --set config.falcoNamespace="falco" \
    --set victoriametricsScrape.enabled=true \
    --set victoriametricsScrape.interval="15s" \
    --set service.type=ClusterIP \
    --set service.port=8080 \
    --set crd.install=true \
    --set rbac.create=true \
    --set serviceAccount.create=true \
    > /dev/null 2>&1 &

show_section_time "Installing All Components"

# Configure Grafana datasource via ingress (only if monitoring is enabled)
if [ "$SKIP_MONITORING" != true ]; then
    echo -e "${YELLOW}→${NC} Configuring VictoriaMetrics datasource..."
DATASOURCE_RESULT=$(timeout 10 curl -s -X POST \
    -H "Content-Type: application/json" \
    -H "Host: localhost" \
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
    }' \
    http://localhost:${INGRESS_HTTP_PORT}/grafana/api/datasources 2>&1 || echo "timeout or error")

if echo "$DATASOURCE_RESULT" | grep -q "Datasource added\|already exists\|success"; then
    echo -e "${GREEN}✓${NC} Datasource configured"
else
    echo -e "${YELLOW}⚠${NC}  Datasource configuration skipped (Grafana may need manual setup)"
fi

# Import dashboard via ingress (with timeout to prevent hanging)
echo -e "${YELLOW}→${NC} Importing Zen Watcher dashboard..."
if [ -f "config/dashboards/zen-watcher-dashboard.json" ]; then
    DASHBOARD_RESULT=$(timeout 10 cat config/dashboards/zen-watcher-dashboard.json | \
    jq '{dashboard: ., overwrite: true, message: "Demo Import"}' | \
    curl -s -X POST \
        -H "Content-Type: application/json" \
        -H "Host: localhost" \
        -u zen:${GRAFANA_PASSWORD} \
        -d @- \
        http://localhost:${INGRESS_HTTP_PORT}/grafana/api/dashboards/db 2>&1 || echo "timeout or error")
    
    if echo "$DASHBOARD_RESULT" | grep -q "success"; then
        echo -e "${GREEN}✓${NC} Dashboard imported successfully"
        else
            echo -e "${YELLOW}⚠${NC}  Dashboard import skipped (can be imported manually later)"
        fi
    else
        echo -e "${YELLOW}⚠${NC}  Dashboard file not found at config/dashboards/zen-watcher-dashboard.json"
    fi
fi

# Wait for all components to be ready and verify endpoints
echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  Waiting for Components to be Ready${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Initialize ready flags and shown flags for all components
declare -A COMPONENT_READY
declare -A COMPONENT_SHOWN

# Initialize all components as not ready and not shown
for comp in "${COMPONENTS[@]}"; do
    IFS='|' read -r namespace name <<< "$comp"
    COMPONENT_READY["$name"]=false
    COMPONENT_SHOWN["$name"]=false
done

# Also track ingress resources separately (only if monitoring is enabled)
INGRESS_RESOURCES_READY=false
INGRESS_RESOURCES_SHOWN=false

MAX_WAIT=120  # 2 minutes max
EXPECTED_READY=${#COMPONENTS[@]}  # Number of components in the list
[ "$SKIP_MONITORING" != true ] && EXPECTED_READY=$((EXPECTED_READY + 1))  # Add ingress resources

for i in {1..120}; do
    READY_COUNT=0
    
    # Check all components from the list
    for comp in "${COMPONENTS[@]}"; do
        IFS='|' read -r namespace name <<< "$comp"
        
        if [ "${COMPONENT_READY[$name]}" != "true" ]; then
            if check_namespace_ready "$namespace" "$name"; then
                COMPONENT_READY["$name"]=true
                if [ "${COMPONENT_SHOWN[$name]}" != "true" ]; then
                    echo -e "${GREEN}✓${NC} $name"
                    COMPONENT_SHOWN["$name"]=true
                fi
            fi
        fi
        
        if [ "${COMPONENT_READY[$name]}" = "true" ]; then
            READY_COUNT=$((READY_COUNT + 1))
        fi
    done
    
    # Check ingress resources exist (only if monitoring is enabled)
    if [ "$SKIP_MONITORING" != true ] && [ "$INGRESS_RESOURCES_READY" = false ]; then
        if kubectl get ingress zen-demo-grafana -n ${NAMESPACE} >/dev/null 2>&1 && \
           kubectl get ingress zen-demo-services -n ${NAMESPACE} >/dev/null 2>&1; then
            INGRESS_RESOURCES_READY=true
            if [ "$INGRESS_RESOURCES_SHOWN" = false ]; then
                echo -e "${GREEN}✓${NC} Ingress resources"
                INGRESS_RESOURCES_SHOWN=true
            fi
        fi
    fi
    
    if [ "$SKIP_MONITORING" != true ] && [ "$INGRESS_RESOURCES_READY" = true ]; then
        READY_COUNT=$((READY_COUNT + 1))
    fi
    
    if [ $((i % 10)) -eq 0 ]; then
        # Only show outstanding (not ready) components
        OUTSTANDING_COUNT=0
        for comp in "${COMPONENTS[@]}"; do
            IFS='|' read -r namespace name <<< "$comp"
            if [ "${COMPONENT_READY[$name]}" != "true" ]; then
                OUTSTANDING_COUNT=$((OUTSTANDING_COUNT + 1))
            fi
        done
        [ "$SKIP_MONITORING" != true ] && [ "$INGRESS_RESOURCES_READY" = false ] && OUTSTANDING_COUNT=$((OUTSTANDING_COUNT + 1))
        
        if [ "$OUTSTANDING_COUNT" -gt 0 ]; then
            echo -e "${CYAN}   Still waiting (${i}s elapsed):${NC}"
            for comp in "${COMPONENTS[@]}"; do
                IFS='|' read -r namespace name <<< "$comp"
                if [ "${COMPONENT_READY[$name]}" != "true" ]; then
                    echo -e "${YELLOW}     ⏳${NC} $name"
                fi
            done
            [ "$SKIP_MONITORING" != true ] && [ "$INGRESS_RESOURCES_READY" = false ] && echo -e "${YELLOW}     ⏳${NC} Ingress resources"
        fi
    fi
    
    if [ "$READY_COUNT" -ge "$EXPECTED_READY" ]; then
        echo -e "${GREEN}✓${NC} All components ready"
        break
    fi
    
    sleep 1
done

# Show diagnostics for any components that are still not ready
echo ""
HAS_FAILURES=false
for comp in "${COMPONENTS[@]}"; do
    IFS='|' read -r namespace name <<< "$comp"
    if [ "${COMPONENT_READY[$name]}" != "true" ]; then
        HAS_FAILURES=true
        break
    fi
done
[ "$SKIP_MONITORING" != true ] && [ "$INGRESS_RESOURCES_READY" = false ] && HAS_FAILURES=true

if [ "$HAS_FAILURES" = true ]; then
    echo -e "${YELLOW}⚠${NC}  Some components are not ready. Diagnostics:${NC}"
    echo ""
    
    for comp in "${COMPONENTS[@]}"; do
        IFS='|' read -r namespace name <<< "$comp"
        if [ "${COMPONENT_READY[$name]}" != "true" ]; then
            echo -e "${YELLOW}  $name:${NC}"
            kubectl get pods -n "$namespace" 2>&1 || true
            echo ""
        fi
    done
fi

# Test endpoints
echo ""
echo -e "${YELLOW}→${NC} Testing endpoints..."
GRAFANA_WORKING=false
VM_WORKING=false
ZW_WORKING=false

for retry in {1..20}; do
    # Test Grafana
    if [ "$GRAFANA_WORKING" = false ]; then
        HTTP_CODE=$(timeout 2 curl -s -o /dev/null -w "%{http_code}" -H "Host: localhost" http://localhost:${INGRESS_HTTP_PORT}/grafana/api/health 2>/dev/null || echo "000")
        if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "401" ] || [ "$HTTP_CODE" = "403" ]; then
            GRAFANA_WORKING=true
            echo -e "${GREEN}✓${NC} Grafana accessible (HTTP ${HTTP_CODE})"
        fi
    fi
    
    # Test VictoriaMetrics - try multiple paths
    if [ "$VM_WORKING" = false ]; then
        # Try /victoriametrics/health first
        HTTP_CODE=$(timeout 2 curl -s -o /dev/null -w "%{http_code}" -H "Host: localhost" http://localhost:${INGRESS_HTTP_PORT}/victoriametrics/health 2>/dev/null || echo "000")
        # If that fails, try root path
        if [ "$HTTP_CODE" != "200" ] && [ "$HTTP_CODE" != "204" ]; then
            HTTP_CODE=$(timeout 2 curl -s -o /dev/null -w "%{http_code}" -H "Host: localhost" http://localhost:${INGRESS_HTTP_PORT}/victoriametrics/ 2>/dev/null || echo "000")
        fi
        # Also try /victoriametrics without trailing slash
        if [ "$HTTP_CODE" != "200" ] && [ "$HTTP_CODE" != "204" ]; then
            HTTP_CODE=$(timeout 2 curl -s -o /dev/null -w "%{http_code}" -H "Host: localhost" http://localhost:${INGRESS_HTTP_PORT}/victoriametrics 2>/dev/null || echo "000")
        fi
        if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "204" ]; then
            VM_WORKING=true
            echo -e "${GREEN}✓${NC} VictoriaMetrics accessible (HTTP ${HTTP_CODE})"
        fi
    fi
    
    # Test Zen Watcher
    if [ "$ZW_WORKING" = false ]; then
        HTTP_CODE=$(timeout 2 curl -s -o /dev/null -w "%{http_code}" -H "Host: localhost" http://localhost:${INGRESS_HTTP_PORT}/zen-watcher/health 2>/dev/null || echo "000")
        if [ "$HTTP_CODE" = "200" ]; then
            ZW_WORKING=true
            echo -e "${GREEN}✓${NC} Zen Watcher accessible (HTTP ${HTTP_CODE})"
        fi
    fi
    
    if [ "$GRAFANA_WORKING" = true ] && [ "$VM_WORKING" = true ] && [ "$ZW_WORKING" = true ]; then
        break
    fi
    
    sleep 1
done

if [ "$GRAFANA_WORKING" = false ] || [ "$VM_WORKING" = false ] || [ "$ZW_WORKING" = false ]; then
    echo -e "${YELLOW}⚠${NC}  Some endpoints may need a few more seconds to become fully accessible"
fi

# Check observations
OBSERVATION_COUNT=$(kubectl get observations -A --kubeconfig=${KUBECONFIG_FILE} --no-headers 2>/dev/null | wc -l | tr -d '[:space:]' || echo "0")
if [ "$OBSERVATION_COUNT" -gt 0 ]; then
    echo -e "${GREEN}✓${NC} Observations created: ${OBSERVATION_COUNT}"
    if command -v jq >/dev/null 2>&1; then
        echo -e "${CYAN}   Observations by source:${NC}"
        kubectl get observations -A --kubeconfig=${KUBECONFIG_FILE} -o json 2>/dev/null | \
          jq -r '.items[] | .spec.source' | sort | uniq -c | awk '{printf "     %s: %d\n", $2, $1}' || true
    fi
fi

# Calculate total time
TOTAL_END_TIME=$(date +%s)
TOTAL_ELAPSED=$((TOTAL_END_TIME - SCRIPT_START_TIME))
TOTAL_MINUTES=$((TOTAL_ELAPSED / 60))
TOTAL_SECONDS=$((TOTAL_ELAPSED % 60))

echo ""
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}  🎉 Demo Environment Ready!${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
# Use ingress for all access via LoadBalancer
GRAFANA_ACCESS_PORT=${INGRESS_HTTP_PORT}
VM_ACCESS_PORT=${INGRESS_HTTP_PORT}
ZEN_WATCHER_ACCESS_PORT=${INGRESS_HTTP_PORT}

echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  📊 SERVICE ACCESS (via k3d LoadBalancer)${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "${CYAN}  VICTORIAMETRICS:${NC}"
echo -e "    ${GREEN}URL:${NC}     ${CYAN}http://localhost:${VM_ACCESS_PORT}/victoriametrics${NC}"
echo -e "    ${GREEN}Metrics API:${NC} ${CYAN}http://localhost:${VM_ACCESS_PORT}/victoriametrics/api/v1/query${NC}"
echo -e "    ${GREEN}VMUI:${NC}    ${CYAN}http://localhost:${VM_ACCESS_PORT}/victoriametrics/vmui${NC}"
echo ""
echo -e "${CYAN}  ZEN WATCHER:${NC}"
echo -e "    ${GREEN}Metrics:${NC} ${CYAN}http://localhost:${ZEN_WATCHER_ACCESS_PORT}/zen-watcher/metrics${NC}"
echo -e "    ${GREEN}Health:${NC}  ${CYAN}http://localhost:${ZEN_WATCHER_ACCESS_PORT}/zen-watcher/health${NC}"
echo ""
echo -e "${CYAN}  GRAFANA:${NC}"
echo -e "    ${GREEN}URL:${NC}     ${CYAN}http://localhost:${GRAFANA_ACCESS_PORT}/grafana${NC}"
echo -e "    ${GREEN}Username:${NC} ${CYAN}zen${NC}"
echo -e "    ${GREEN}Password:${NC} ${CYAN}${GRAFANA_PASSWORD}${NC}"
echo -e "    ${GREEN}Dashboard:${NC} ${CYAN}http://localhost:${GRAFANA_ACCESS_PORT}/grafana/d/zen-watcher${NC}"
echo ""
echo -e "${CYAN}  KUBECONFIG:${NC}"
echo -e "    ${GREEN}File:${NC}     ${CYAN}${KUBECONFIG_FILE}${NC}"
echo -e "    ${GREEN}Usage:${NC}   ${CYAN}kubectl get observations --kubeconfig ${KUBECONFIG_FILE}${NC}"
echo ""
if [ "$OBSERVATION_COUNT" -gt 0 ]; then
    echo -e "${CYAN}  OBSERVATIONS:${NC}"
    echo -e "    ${GREEN}Total:${NC}    ${CYAN}${OBSERVATION_COUNT}${NC}"
    echo -e "    ${GREEN}View:${NC}     ${CYAN}kubectl get observations -A --kubeconfig ${KUBECONFIG_FILE}${NC}"
    echo -e "    ${GREEN}By source:${NC} ${CYAN}kubectl get observations -A --kubeconfig ${KUBECONFIG_FILE} -o json | jq -r '.items[] | .spec.source' | sort | uniq -c${NC}"
    echo ""
fi

echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
if [ $TOTAL_MINUTES -gt 0 ]; then
    echo -e "  ${GREEN}⏱  Deployment Time:${NC} ${CYAN}${TOTAL_MINUTES}m ${TOTAL_SECONDS}s${NC}"
else
    echo -e "  ${GREEN}⏱  Deployment Time:${NC} ${CYAN}${TOTAL_SECONDS}s${NC}"
fi
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

echo ""
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}  ✅ Demo environment is ready and accessible!${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "${CYAN}All services are accessible via ingress LoadBalancer on port ${INGRESS_HTTP_PORT}${NC}"
echo -e "${CYAN}Endpoints will remain accessible until cluster is deleted.${NC}"
echo ""
echo -e "${BLUE}To clean up the demo:${NC}"
case "$PLATFORM" in
    k3d) echo -e "  ${CYAN}k3d cluster delete ${CLUSTER_NAME}${NC}" ;;
    kind) echo -e "  ${CYAN}kind delete cluster --name ${CLUSTER_NAME}${NC}" ;;
    minikube) echo -e "  ${CYAN}minikube delete -p ${CLUSTER_NAME}${NC}" ;;
esac
echo ""
echo -e "${CYAN}Or use the cleanup script:${NC}"
echo -e "  ${CYAN}./hack/cleanup-demo.sh${NC}"
echo ""
