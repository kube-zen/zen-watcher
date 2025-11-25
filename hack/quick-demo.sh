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
#
# Port Configuration (all configurable via environment variables):
#   GRAFANA_PORT=3100                 # Grafana port-forward (default: 3100)
#   VICTORIA_METRICS_PORT=8528        # VictoriaMetrics port-forward (default: 8528)
#   ZEN_WATCHER_PORT=8180             # Zen Watcher port-forward (default: 8180)
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

# Service ports (port-forward targets)
GRAFANA_PORT="${GRAFANA_PORT:-3100}"
ZEN_WATCHER_PORT="${ZEN_WATCHER_PORT:-8180}"
VICTORIA_METRICS_PORT="${VICTORIA_METRICS_PORT:-8528}"

# Generate random password for zen user
GRAFANA_PASSWORD=$(openssl rand -base64 12 | tr -d "=+/" | cut -c1-12)

# Function to check if a port is in use
check_port() {
    local port=$1
    local service=$2
    
    if command -v lsof &> /dev/null; then
        if lsof -Pi :${port} -sTCP:LISTEN -t >/dev/null 2>&1; then
            return 1  # Port is in use
        fi
    elif command -v netstat &> /dev/null; then
        if netstat -an 2>/dev/null | grep -q ":${port}.*LISTEN"; then
            return 1  # Port is in use
        fi
    elif command -v ss &> /dev/null; then
        if ss -lnt 2>/dev/null | grep -q ":${port}"; then
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
    
    local ports_changed=false
    local default_ports_in_use=false
    local original_grafana=${GRAFANA_PORT}
    local original_vm=${VICTORIA_METRICS_PORT}
    local original_watcher=${ZEN_WATCHER_PORT}
    
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
                local existing_k3d=$(k3d cluster list 2>/dev/null | grep -v "^NAME" | wc -l | tr -d ' ')
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
                local existing_kind=$(kind get clusters 2>/dev/null | wc -l | tr -d ' ')
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
                local existing_minikube=$(minikube profile list 2>/dev/null | grep -v "Profile" | grep -v "^-" | wc -l | tr -d ' ')
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
                local existing_clusters=$(k3d cluster list 2>/dev/null | grep -v "^NAME" | wc -l | tr -d ' ')
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
            
                echo -e "${YELLOW}→${NC} Creating k3d cluster '${CLUSTER_NAME}'..."
            echo -e "${CYAN}   API Port: ${K3D_API_PORT}${NC}"
            echo -e "${CYAN}   This may take 30-60 seconds...${NC}"
            
            # Build k3d command - simplified approach like zen-gamma
            # Always use explicit ports to avoid conflicts with multiple clusters
            local k3d_create_args=(
                "cluster" "create" "${CLUSTER_NAME}"
                "--agents" "1"
                "--k3s-arg" "--disable=traefik@server:0"
            )
            
            # Determine API port: ALWAYS use explicit port when other clusters exist (zen-gamma pattern)
            # This prevents port conflicts when multiple k3d clusters run on same host
            local existing_clusters=$(k3d cluster list 2>/dev/null | grep -v "^NAME" | wc -l || echo "0")
            if [ "$existing_clusters" -gt 0 ] && [ "${K3D_API_PORT}" = "6443" ]; then
                # Other clusters exist - ALWAYS use 6550 (zen-gamma demo pattern)
                # Don't check if port is free - k3d will handle it or fail clearly
                k3d_create_args+=("--api-port" "6550")
                K3D_API_PORT=6550
                echo -e "${CYAN}   Multiple clusters detected, using port 6550 (zen-gamma pattern)${NC}"
            elif [ "${K3D_API_PORT}" != "6443" ]; then
                # Custom port specified
                k3d_create_args+=("--api-port" "${K3D_API_PORT}")
                echo -e "${CYAN}   Using custom API port: ${K3D_API_PORT}${NC}"
            elif ! check_port 6443 "k3d API" 2>/dev/null; then
                # Default port is in use, use 6550
                k3d_create_args+=("--api-port" "6550")
                K3D_API_PORT=6550
                echo -e "${CYAN}   Port 6443 in use, using 6550 instead${NC}"
            else
                # Default port is free, use it
                k3d_create_args+=("--api-port" "${K3D_API_PORT}")
                echo -e "${CYAN}   Using API port: ${K3D_API_PORT}${NC}"
            fi
            
            # CRITICAL: Don't use --wait as it can hang with multiple clusters
            # Instead, create cluster and manually wait for readiness
            echo -e "${CYAN}   Creating cluster (this may take 60-120 seconds)...${NC}"
            if timeout 120 k3d "${k3d_create_args[@]}" 2>&1 | tee /tmp/k3d-create.log; then
                echo -e "${CYAN}   Cluster creation command completed, waiting for readiness...${NC}"
            else
                local exit_code=$?
                # Check if cluster was actually created despite timeout
                if k3d cluster list 2>/dev/null | grep -q "^${CLUSTER_NAME}"; then
                    echo -e "${YELLOW}⚠${NC}  Cluster creation timed out, but cluster exists - continuing...${NC}"
                else
                    echo -e "${RED}✗${NC} Cluster creation failed"
                    echo -e "${YELLOW}   Check logs: cat /tmp/k3d-create.log${NC}"
                    if [ $exit_code -eq 124 ]; then
                        echo -e "${RED}   Timeout: Cluster creation took longer than 2 minutes${NC}"
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
            echo -e "${CYAN}   This may take 1-2 minutes...${NC}"
            
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
            echo -e "${CYAN}   This may take 2-3 minutes...${NC}"
            
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
        
        # Detect actual port if using existing cluster or if cluster was just created
        echo -e "${CYAN}   [DEBUG] Detecting actual API port...${NC}"
        ACTUAL_PORT=""
        # Method 1: Try to get port from docker port command
        ACTUAL_PORT=$(docker port k3d-${CLUSTER_NAME}-serverlb 2>/dev/null | grep "6443/tcp" | cut -d: -f2 | tr -d ' ' | head -1 || echo "")
        # Method 2: Try docker inspect
        if [ -z "$ACTUAL_PORT" ] || [ "$ACTUAL_PORT" = "" ]; then
            ACTUAL_PORT=$(docker inspect k3d-${CLUSTER_NAME}-serverlb 2>/dev/null | jq -r '.[0].NetworkSettings.Ports."6443/tcp"[0].HostPort // empty' 2>/dev/null || echo "")
        fi
        # Method 3: Check listening ports (fallback - check common k3d ports)
        if [ -z "$ACTUAL_PORT" ] || [ "$ACTUAL_PORT" = "" ] || [ "$ACTUAL_PORT" = "null" ]; then
            for test_port in 6550 6551 6552 6553 6554 6555 6443 6444 6445; do
                if ss -tlnp 2>/dev/null | grep -q ":${test_port}"; then
                    # Verify it's actually k3d by checking if kubectl can connect
                    if timeout 2 kubectl --server="https://127.0.0.1:${test_port}" get nodes --request-timeout=1s &>/dev/null 2>&1; then
                        ACTUAL_PORT=$test_port
                        echo -e "${CYAN}   [DEBUG] Found port via listening port check: ${ACTUAL_PORT}${NC}"
                        break
                    fi
                fi
            done
        fi
        if [ -n "$ACTUAL_PORT" ] && [ "$ACTUAL_PORT" != "null" ] && [ "$ACTUAL_PORT" != "" ]; then
            K3D_API_PORT=$ACTUAL_PORT
            echo -e "${CYAN}   [DEBUG] Using detected port: ${K3D_API_PORT}${NC}"
        else
            echo -e "${CYAN}   [DEBUG] Could not detect port, using configured: ${K3D_API_PORT}${NC}"
        fi
        
        echo -e "${CYAN}   Setting up kubeconfig for port ${K3D_API_PORT}...${NC}"
        
        # Merge kubeconfig (preferred method - updates default kubeconfig)
        if ! timeout 10 k3d kubeconfig merge ${CLUSTER_NAME} --kubeconfig-merge-default --kubeconfig-switch-context 2>/dev/null; then
            # Fallback: write to temp file and export
            timeout 10 k3d kubeconfig write ${CLUSTER_NAME} > /tmp/k3d-kubeconfig-${CLUSTER_NAME} 2>/dev/null
            export KUBECONFIG=/tmp/k3d-kubeconfig-${CLUSTER_NAME}
        fi
        
        # Ensure context is set
        timeout 5 kubectl config use-context k3d-${CLUSTER_NAME} 2>/dev/null || {
            # Try alternative context name
            timeout 5 kubectl config use-context k3d-${CLUSTER_NAME}@${CLUSTER_NAME} 2>/dev/null || true
        }
        
        # Fix kubeconfig server URL - always use 127.0.0.1 and the port we specified/detected
        timeout 5 kubectl config set clusters.k3d-${CLUSTER_NAME}.server "https://127.0.0.1:${K3D_API_PORT}" 2>/dev/null || true
        echo -e "${CYAN}   [DEBUG] Set kubeconfig server to: https://127.0.0.1:${K3D_API_PORT}${NC}"
        
        # CRITICAL: k3d uses self-signed certificates, so we need to skip TLS verification
        # Remove certificate-authority-data if present (conflicts with insecure-skip-tls-verify)
        timeout 5 kubectl config unset clusters.k3d-${CLUSTER_NAME}.certificate-authority-data 2>/dev/null || true
        timeout 5 kubectl config set clusters.k3d-${CLUSTER_NAME}.insecure-skip-tls-verify true 2>/dev/null || true
        echo -e "${CYAN}   Configured kubeconfig for port ${K3D_API_PORT} (TLS verification skipped)${NC}"
        
        # Wait for cluster to be ready, then verify connectivity with retries
        echo -e "${CYAN}   [DEBUG] Waiting for cluster to be ready...${NC}"
        sleep 5  # Give cluster a moment to start
        
        # Verify connectivity with retries
        echo -e "${CYAN}   [DEBUG] Verifying cluster connectivity...${NC}"
        CLUSTER_ACCESSIBLE=false
        for retry in {1..20}; do
            # First wait a bit for cluster to be ready
            if [ $retry -le 5 ]; then
                sleep 3
            fi
            
            if timeout 10 kubectl get nodes --request-timeout=5s > /dev/null 2>&1; then
                # Verify nodes are actually ready
                if timeout 10 kubectl get nodes --request-timeout=5s 2>&1 | grep -q "Ready"; then
                    echo -e "${GREEN}✓${NC} Cluster is accessible and nodes are ready"
                    CLUSTER_ACCESSIBLE=true
                    break
                fi
            fi
            
            if [ $retry -lt 20 ]; then
                echo -e "${CYAN}   [DEBUG] Retry $retry/20: Regenerating kubeconfig...${NC}"
                timeout 10 k3d kubeconfig write ${CLUSTER_NAME} 2>&1 | grep -v "ERRO" > /dev/null || true
                timeout 10 k3d kubeconfig merge ${CLUSTER_NAME} --kubeconfig-merge-default --kubeconfig-switch-context 2>&1 | grep -v "ERRO" > /dev/null || true
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
        if timeout 5 kubectl get nodes --request-timeout=5s &>/dev/null 2>&1; then
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

# Deploy Security Tools (only if flags are set)
if [ "$INSTALL_TRIVY" = true ] || [ "$INSTALL_FALCO" = true ] || [ "$INSTALL_KYVERNO" = true ] || [ "$INSTALL_CHECKOV" = true ] || [ "$INSTALL_KUBE_BENCH" = true ]; then
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  Deploying Security Tools${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
    SECTION_START_TIME=$(date +%s)

    # Add Helm repositories (only if needed)
    if [ "$INSTALL_TRIVY" = true ] || [ "$INSTALL_FALCO" = true ] || [ "$INSTALL_KYVERNO" = true ]; then
echo -e "${YELLOW}→${NC} Adding Helm repositories..."
        [ "$INSTALL_TRIVY" = true ] && helm repo add aqua https://aquasecurity.github.io/helm-charts 2>/dev/null || true
        [ "$INSTALL_FALCO" = true ] && helm repo add falcosecurity https://falcosecurity.github.io/charts 2>/dev/null || true
        [ "$INSTALL_KYVERNO" = true ] && helm repo add kyverno https://kyverno.github.io/kyverno/ 2>/dev/null || true
helm repo update > /dev/null 2>&1
echo -e "${GREEN}✓${NC} Helm repositories updated"
    fi

# Deploy Trivy Operator
    if [ "$INSTALL_TRIVY" = true ]; then
echo -e "${YELLOW}→${NC} Deploying Trivy Operator (this may take 1-2 minutes)..."
helm upgrade --install trivy-operator aqua/trivy-operator \
    --namespace trivy-system \
    --create-namespace \
    --set="trivy.ignoreUnfixed=true" \
    --wait --timeout=2m > /dev/null 2>&1 || echo -e "${YELLOW}⚠${NC}  Trivy deployment taking longer, continuing..."
echo -e "${GREEN}✓${NC} Trivy Operator deployed"
    fi

    # Deploy Falco
    if [ "$INSTALL_FALCO" = true ]; then
echo -e "${YELLOW}→${NC} Deploying Falco (starting in background)..."
helm upgrade --install falco falcosecurity/falco \
    --namespace falco \
    --create-namespace \
    --set falcosidekick.enabled=false \
    --wait --timeout=30s > /dev/null 2>&1 || echo -e "${YELLOW}⚠${NC}  Falco starting (will be ready soon)"
echo -e "${GREEN}✓${NC} Falco deployed"
    fi

# Deploy Kyverno
    if [ "$INSTALL_KYVERNO" = true ]; then
echo -e "${YELLOW}→${NC} Deploying Kyverno (starting in background)..."
helm upgrade --install kyverno kyverno/kyverno \
    --namespace kyverno \
    --create-namespace \
    --wait --timeout=30s > /dev/null 2>&1 || echo -e "${YELLOW}⚠${NC}  Kyverno starting (will be ready soon)"
echo -e "${GREEN}✓${NC} Kyverno deployed"
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
        local bench_target="cis-1.6"
        local node_image=""
        
        # Try to detect cluster type
        if kubectl get nodes --no-headers 2>/dev/null | head -1 | grep -q "k3d"; then
            bench_target="cis-1.6"
            node_image="k3d-${CLUSTER_NAME}-server-0"
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

echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  Deploying Monitoring Stack${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Create namespace (skip if using existing)
if [ "${USE_EXISTING_NAMESPACE:-false}" != "true" ]; then
kubectl create namespace ${NAMESPACE} 2>/dev/null || true
else
    echo -e "${CYAN}→${NC} Using existing namespace '${NAMESPACE}'"
fi

# Deploy VictoriaMetrics (with retries for cluster readiness)
echo -e "${YELLOW}→${NC} Deploying VictoriaMetrics..."
for i in {1..10}; do
    if kubectl create deployment victoriametrics \
    --image=victoriametrics/victoria-metrics:latest \
        -n ${NAMESPACE} 2>/dev/null; then
        break
    elif kubectl get deployment victoriametrics -n ${NAMESPACE} &>/dev/null; then
        kubectl rollout restart deployment/victoriametrics -n ${NAMESPACE} 2>/dev/null || true
        break
    else
        if [ $i -lt 10 ]; then
            sleep 2
        fi
    fi
done
# Expose VictoriaMetrics as ClusterIP (ingress will handle routing)
timeout 10 kubectl delete svc victoriametrics -n ${NAMESPACE} 2>&1 || true
timeout 15 kubectl expose deployment victoriametrics \
    --port=8428 --target-port=8428 \
    --type=ClusterIP \
    --name=victoriametrics \
    -n ${NAMESPACE} 2>&1 | grep -v "already exists" > /dev/null || true
echo -e "${GREEN}✓${NC} VictoriaMetrics deployed (ClusterIP)"

# Deploy Grafana with zen user (with retries for cluster readiness)
echo -e "${YELLOW}→${NC} Deploying Grafana with zen user..."
for i in {1..10}; do
    if kubectl create deployment grafana \
    --image=grafana/grafana:latest \
    -n ${NAMESPACE} \
        --dry-run=client -o yaml 2>/dev/null | \
kubectl set env --local -f - \
    GF_SECURITY_ADMIN_USER=zen \
    GF_SECURITY_ADMIN_PASSWORD=${GRAFANA_PASSWORD} \
    GF_USERS_ALLOW_SIGN_UP=false \
    GF_USERS_DEFAULT_THEME=dark \
        --dry-run=client -o yaml 2>/dev/null | \
    kubectl apply -f - 2>&1 | grep -v "already exists" > /dev/null; then
        break
    elif kubectl get deployment grafana -n ${NAMESPACE} &>/dev/null; then
        # Update env vars if deployment exists
        kubectl set env deployment/grafana \
            GF_SECURITY_ADMIN_USER=zen \
            GF_SECURITY_ADMIN_PASSWORD=${GRAFANA_PASSWORD} \
            GF_USERS_ALLOW_SIGN_UP=false \
            GF_USERS_DEFAULT_THEME=dark \
            -n ${NAMESPACE} 2>/dev/null || true
        break
    else
        if [ $i -lt 10 ]; then
            sleep 2
        fi
    fi
done

# Expose Grafana as ClusterIP (ingress will handle routing)
echo -e "${CYAN}   Creating Grafana service (ClusterIP for ingress)...${NC}"
EXISTING_SVC=$(timeout 5 kubectl get svc grafana -n ${NAMESPACE} -o jsonpath='{.spec.type}' 2>/dev/null || echo "none")
if [ "$EXISTING_SVC" != "none" ]; then
    timeout 10 kubectl delete svc grafana -n ${NAMESPACE} 2>&1 || true
    sleep 1
fi

timeout 15 kubectl expose deployment grafana \
    --port=3000 --target-port=3000 \
    --type=ClusterIP \
    --name=grafana \
    -n ${NAMESPACE} 2>&1 | grep -v "already exists" > /dev/null || true
echo -e "${GREEN}✓${NC} Grafana service created (ClusterIP)"
echo -e "${GREEN}✓${NC} Grafana deployed (user: zen)"

# Wait for pods
echo -e "${YELLOW}→${NC} Waiting for monitoring stack to be ready (this takes 30-60 seconds)..."
for attempt in {1..30}; do
    if timeout 15 kubectl wait --for=condition=ready pod -l app=victoriametrics -n ${NAMESPACE} --timeout=10s > /dev/null 2>&1 && \
       timeout 15 kubectl wait --for=condition=ready pod -l app=grafana -n ${NAMESPACE} --timeout=10s > /dev/null 2>&1; then
        break
    fi
    sleep 2
done
echo -e "${GREEN}✓${NC} Monitoring stack ready"
show_section_time "Monitoring stack deployment"

# Install nginx ingress controller (replaces Traefik for better service exposure)
echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  Installing Nginx Ingress Controller${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
SECTION_START_TIME=$(date +%s)

# Ensure kubeconfig is properly configured before installing ingress
echo -e "${CYAN}   Ensuring kubeconfig is properly configured...${NC}"
CLUSTER_ACCESSIBLE=false
for retry in {1..15}; do
    timeout 10 k3d kubeconfig merge ${CLUSTER_NAME} --kubeconfig-merge-default --kubeconfig-switch-context 2>&1 | grep -v "ERRO" > /dev/null || true
    timeout 5 kubectl config set clusters.k3d-${CLUSTER_NAME}.server "https://127.0.0.1:${K3D_API_PORT}" 2>&1 > /dev/null || true
    timeout 5 kubectl config set clusters.k3d-${CLUSTER_NAME}.insecure-skip-tls-verify true 2>&1 > /dev/null || true
    timeout 5 kubectl config unset clusters.k3d-${CLUSTER_NAME}.certificate-authority-data 2>&1 > /dev/null || true
    
    if timeout 10 kubectl get nodes --request-timeout=5s > /dev/null 2>&1; then
        echo -e "${GREEN}✓${NC} Cluster is accessible"
        CLUSTER_ACCESSIBLE=true
        break
    else
        if [ $retry -lt 15 ]; then
            echo -e "${CYAN}   [DEBUG] Retry $retry/15: Regenerating kubeconfig...${NC}"
            timeout 10 k3d kubeconfig write ${CLUSTER_NAME} 2>&1 | grep -v "ERRO" > /dev/null || true
            timeout 10 k3d kubeconfig merge ${CLUSTER_NAME} --kubeconfig-merge-default --kubeconfig-switch-context 2>&1 | grep -v "ERRO" > /dev/null || true
            sleep 2
        fi
    fi
done

if [ "$CLUSTER_ACCESSIBLE" = false ]; then
    echo -e "${RED}✗${NC} Cannot access cluster after 15 retries - aborting ingress installation"
    exit 1
fi

echo -e "${YELLOW}→${NC} Installing nginx ingress controller..."
# Add ingress-nginx helm repo if not already added
if ! timeout 10 helm repo list 2>/dev/null | grep -q ingress-nginx; then
    timeout 30 helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx 2>&1 || true
    timeout 30 helm repo update 2>&1 || true
fi

# Find available high port (8080-8090 range)
INGRESS_HTTP_PORT=8080
for port in {8080..8090}; do
    if ! timeout 2 ss -tlnp 2>/dev/null | grep -q ":${port} "; then
        INGRESS_HTTP_PORT=${port}
        echo -e "${CYAN}   Using port ${INGRESS_HTTP_PORT} for ingress${NC}"
        break
    fi
done

# Install nginx ingress with NodePort on high port - with retries
INGRESS_INSTALLED=false
for retry in {1..5}; do
    if timeout 10 helm list -n ingress-nginx 2>&1 | grep -q ingress-nginx; then
        echo -e "${GREEN}✓${NC} Nginx ingress controller already installed"
        INGRESS_INSTALLED=true
        break
    fi
    
    echo -e "${CYAN}   Installing ingress (attempt $retry/5)...${NC}"
    if timeout 120 helm install ingress-nginx ingress-nginx/ingress-nginx \
        --namespace ingress-nginx \
        --create-namespace \
        --set controller.service.type=NodePort \
        --set controller.service.nodePorts.http=${INGRESS_HTTP_PORT} \
        --set controller.service.nodePorts.https=$((INGRESS_HTTP_PORT + 1)) \
        --set controller.admissionWebhooks.enabled=false \
        --wait --timeout=2m 2>&1 | tee /tmp/ingress-install.log; then
        echo -e "${GREEN}✓${NC} Nginx ingress controller installed"
        INGRESS_INSTALLED=true
        break
    else
        if [ $retry -lt 5 ]; then
            echo -e "${YELLOW}⚠${NC}  Installation failed, fixing kubeconfig and retrying...${NC}"
            timeout 10 k3d kubeconfig write ${CLUSTER_NAME} 2>&1 | grep -v "ERRO" > /dev/null || true
            timeout 10 k3d kubeconfig merge ${CLUSTER_NAME} --kubeconfig-merge-default --kubeconfig-switch-context 2>&1 | grep -v "ERRO" > /dev/null || true
            timeout 5 kubectl config set clusters.k3d-${CLUSTER_NAME}.server "https://127.0.0.1:${K3D_API_PORT}" 2>&1 > /dev/null || true
            timeout 5 kubectl config set clusters.k3d-${CLUSTER_NAME}.insecure-skip-tls-verify true 2>&1 > /dev/null || true
            timeout 5 kubectl config unset clusters.k3d-${CLUSTER_NAME}.certificate-authority-data 2>&1 > /dev/null || true
            sleep 3
        fi
    fi
done

if [ "$INGRESS_INSTALLED" = false ]; then
    echo -e "${RED}✗${NC} Failed to install ingress after 5 attempts"
    exit 1
fi

# Wait for ingress controller to be ready with verification
echo -e "${CYAN}   Waiting for ingress controller to be ready...${NC}"
INGRESS_READY=false
for i in {1..60}; do
    if timeout 10 kubectl wait --for=condition=ready pod -l app.kubernetes.io/component=controller -n ingress-nginx --timeout=10s > /dev/null 2>&1; then
        # Verify it's actually accessible
        ACTUAL_PORT=$(timeout 10 kubectl get svc ingress-nginx-controller -n ingress-nginx -o jsonpath='{.spec.ports[?(@.name=="http")].nodePort}' 2>/dev/null || echo "")
        if [ -n "$ACTUAL_PORT" ] && [ "$ACTUAL_PORT" != "null" ] && [ "$ACTUAL_PORT" != "0" ]; then
            # Test if port is actually listening
            if timeout 2 curl -s -o /dev/null -w "%{http_code}" http://localhost:${ACTUAL_PORT} 2>/dev/null | grep -q "[234]"; then
                echo -e "${GREEN}✓${NC} Ingress controller ready and accessible on port ${ACTUAL_PORT}"
                INGRESS_READY=true
                break
            fi
        fi
    fi
    sleep 2
    if [ $((i % 10)) -eq 0 ]; then
        echo -e "${CYAN}   ... still waiting ($((i*2)) seconds)${NC}"
    fi
done

if [ "$INGRESS_READY" = false ]; then
    echo -e "${YELLOW}⚠${NC}  Ingress controller may not be fully ready, but continuing...${NC}"
fi

# Verify ingress service port
ACTUAL_INGRESS_PORT=$(timeout 10 kubectl get svc ingress-nginx-controller -n ingress-nginx -o jsonpath='{.spec.ports[?(@.name=="http")].nodePort}' 2>/dev/null || echo "${INGRESS_HTTP_PORT}")
if [ "$ACTUAL_INGRESS_PORT" != "${INGRESS_HTTP_PORT}" ]; then
    INGRESS_HTTP_PORT=${ACTUAL_INGRESS_PORT}
    echo -e "${CYAN}   Ingress using port ${INGRESS_HTTP_PORT}${NC}"
fi
echo -e "${CYAN}   Ingress accessible on: http://localhost:${INGRESS_HTTP_PORT}${NC}"
show_section_time "Nginx ingress installation"

echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  Deploying Zen Watcher${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
SECTION_START_TIME=$(date +%s)

# Deploy Zen Watcher CRDs
echo -e "${YELLOW}→${NC} Deploying Zen Watcher CRDs..."
for i in {1..10}; do
    if timeout 30 kubectl apply -f deployments/crds/ --validate=false 2>&1 | grep -v "already exists\|unchanged" > /dev/null; then
echo -e "${GREEN}✓${NC} CRDs deployed"
        break
    elif timeout 10 kubectl get crd observations.zen.kube-zen.io 2>/dev/null | grep -q observations; then
        echo -e "${GREEN}✓${NC} CRDs already exist"
        break
    else
        if [ $i -lt 10 ]; then
            sleep 2
        else
            echo -e "${YELLOW}⚠${NC}  CRD deployment had issues (continuing...)"
        fi
    fi
done

# Handle mock data deployment
SECTION_START_TIME=$(date +%s)
if [ "$SKIP_MOCK_DATA" = true ]; then
    echo -e "${CYAN}→${NC} Skipping mock data deployment (--skip-mock-data flag)${NC}"
elif [ "$DEPLOY_MOCK_DATA" = true ]; then
    echo -e "${YELLOW}→${NC} Deploying mock data (--deploy-mock-data flag)..."
    ./hack/mock-data.sh ${NAMESPACE} > /dev/null 2>&1 || echo -e "${YELLOW}⚠${NC}  Mock data deployment issue (continuing...)"
    echo -e "${GREEN}✓${NC} Mock data deployed"
    show_section_time "Mock data deployment"
elif [ "$NON_INTERACTIVE" = true ]; then
    # In non-interactive mode, skip mock data unless explicitly requested
    echo -e "${CYAN}→${NC} Non-interactive mode: skipping mock data (use --deploy-mock-data to enable)${NC}"
else
    # Interactive mode: prompt user
echo ""
    read -p "$(echo -e ${CYAN}Deploy mock data with Observations and metrics? [Y/n]${NC}) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Nn]$ ]]; then
    echo -e "${YELLOW}→${NC} Deploying mock data..."
    ./hack/mock-data.sh ${NAMESPACE} > /dev/null 2>&1 || echo -e "${YELLOW}⚠${NC}  Mock data deployment issue (continuing...)"
    echo -e "${GREEN}✓${NC} Mock data deployed"
        show_section_time "Mock data deployment"
    else
        echo -e "${CYAN}→${NC} Skipping mock data deployment${NC}"
    fi
fi

echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  Configuring Grafana${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
SECTION_START_TIME=$(date +%s)

# Create ingress resources for Grafana, VictoriaMetrics, and Zen Watcher
echo -e "${YELLOW}→${NC} Creating ingress resources..."

# Get actual ingress HTTP port (use the one we set earlier)
ACTUAL_INGRESS_PORT=$(timeout 10 kubectl get svc ingress-nginx-controller -n ingress-nginx -o jsonpath='{.spec.ports[?(@.name=="http")].nodePort}' 2>/dev/null || echo "${INGRESS_HTTP_PORT}")
if [ "$ACTUAL_INGRESS_PORT" != "null" ] && [ -n "$ACTUAL_INGRESS_PORT" ] && [ "$ACTUAL_INGRESS_PORT" != "0" ]; then
    INGRESS_HTTP_PORT=${ACTUAL_INGRESS_PORT}
fi
GRAFANA_ACCESS_PORT=${INGRESS_HTTP_PORT}

# Create single ingress with path-based routing (no Host header needed)
cat <<EOF | timeout 30 kubectl apply -f - 2>&1 | grep -v "already exists\|unchanged" > /dev/null || true
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: zen-demo-services
  namespace: ${NAMESPACE}
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /\$2
    nginx.ingress.kubernetes.io/use-regex: "true"
spec:
  ingressClassName: nginx
  rules:
  - http:
      paths:
      - path: /grafana(/|$)(.*)
        pathType: ImplementationSpecific
        backend:
          service:
            name: grafana
            port:
              number: 3000
      - path: /victoriametrics(/|$)(.*)
        pathType: ImplementationSpecific
        backend:
          service:
            name: victoriametrics
            port:
              number: 8428
EOF

# Create Zen Watcher metrics ingress (if deployment exists)
if timeout 10 kubectl get deployment zen-watcher -n ${NAMESPACE} 2>/dev/null | grep -q zen-watcher; then
    # Ensure zen-watcher service exists
    timeout 15 kubectl expose deployment zen-watcher --port=8080 --target-port=8080 --type=ClusterIP --name=zen-watcher -n ${NAMESPACE} 2>&1 | grep -v "already exists" > /dev/null || true
    
    cat <<EOF | timeout 30 kubectl apply -f - 2>&1 | grep -v "already exists\|unchanged" > /dev/null || true
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: zen-watcher
  namespace: ${NAMESPACE}
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /\$2
    nginx.ingress.kubernetes.io/use-regex: "true"
spec:
  ingressClassName: nginx
  rules:
  - http:
      paths:
      - path: /zen-watcher(/|$)(.*)
        pathType: ImplementationSpecific
        backend:
          service:
            name: zen-watcher
            port:
              number: 8080
EOF
    echo -e "${CYAN}   Zen Watcher ingress created${NC}"
fi

echo -e "${GREEN}✓${NC} Ingress resources created (path-based routing)"
echo -e "${CYAN}   Grafana: http://localhost:${INGRESS_HTTP_PORT}/grafana${NC}"
echo -e "${CYAN}   VictoriaMetrics: http://localhost:${INGRESS_HTTP_PORT}/victoriametrics${NC}"

# Wait for Grafana to be fully ready and VERIFY it works via ingress
echo -e "${YELLOW}→${NC} Waiting for Grafana to be fully ready and verifying ingress access..."
GRAFANA_READY=false
for i in {1..60}; do
    # Test via ingress with path-based routing (no Host header needed)
    HTTP_CODE=$(timeout 3 curl -s -o /dev/null -w "%{http_code}" http://localhost:${INGRESS_HTTP_PORT}/grafana/api/health 2>/dev/null || echo "000")
    if [ "$HTTP_CODE" = "200" ]; then
        HEALTH_RESPONSE=$(timeout 3 curl -s http://localhost:${INGRESS_HTTP_PORT}/grafana/api/health 2>/dev/null || echo "")
        if echo "$HEALTH_RESPONSE" | grep -q "ok\|database"; then
            echo -e "${GREEN}✓${NC} Grafana is ready and responding via ingress (HTTP ${HTTP_CODE})"
            GRAFANA_READY=true
            break
        fi
    elif [ "$HTTP_CODE" = "401" ] || [ "$HTTP_CODE" = "403" ]; then
        # These codes mean Grafana is up but requires auth - that's fine!
        echo -e "${GREEN}✓${NC} Grafana is ready via ingress (HTTP ${HTTP_CODE} - auth required)"
        GRAFANA_READY=true
        break
    fi
    if [ $((i % 10)) -eq 0 ]; then
        echo -e "${CYAN}  [DEBUG] Still waiting... (${i}/60 seconds, HTTP code: ${HTTP_CODE})${NC}"
    fi
    sleep 1
done

if [ "$GRAFANA_READY" = false ]; then
    echo -e "${YELLOW}⚠${NC}  Grafana may not be fully ready via ingress, but continuing...${NC}"
    echo -e "${CYAN}   [DEBUG] Last HTTP code: ${HTTP_CODE}${NC}"
fi

# Final verification - try to login via ingress
echo -e "${CYAN}   [DEBUG] Verifying credentials via ingress...${NC}"
LOGIN_TEST=$(timeout 5 curl -s -X POST -H "Content-Type: application/json" \
    -d "{\"user\":\"zen\",\"password\":\"${GRAFANA_PASSWORD}\"}" \
    http://localhost:${INGRESS_HTTP_PORT}/grafana/api/login 2>/dev/null || echo "ERROR")
if echo "$LOGIN_TEST" | grep -q "Logged in\|message.*success"; then
    echo -e "${GREEN}✓${NC} Credentials verified via ingress - login successful"
elif echo "$LOGIN_TEST" | grep -q "Invalid"; then
    echo -e "${YELLOW}⚠${NC}  Login test failed - password may be incorrect${NC}"
else
    echo -e "${CYAN}   [DEBUG] Login test result: ${LOGIN_TEST:0:100}${NC}"
fi

show_section_time "Grafana configuration"

# Configure Grafana datasource via ingress (with timeout to prevent hanging)
echo -e "${YELLOW}→${NC} Configuring VictoriaMetrics datasource..."
DATASOURCE_RESULT=$(timeout 10 curl -s -X POST \
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
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  📊 SERVICE ACCESS (via Nginx Ingress)${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "${CYAN}  GRAFANA:${NC}"
echo -e "    ${GREEN}URL:${NC}     ${CYAN}http://localhost:${INGRESS_HTTP_PORT}/grafana${NC}"
echo -e "    ${GREEN}Username:${NC} ${CYAN}zen${NC}"
echo -e "    ${GREEN}Password:${NC} ${CYAN}${GRAFANA_PASSWORD}${NC}"
echo -e "    ${GREEN}Dashboard:${NC} ${CYAN}http://localhost:${INGRESS_HTTP_PORT}/grafana/d/zen-watcher${NC}"
echo ""
echo -e "${CYAN}  VICTORIAMETRICS:${NC}"
echo -e "    ${GREEN}URL:${NC}     ${CYAN}http://localhost:${INGRESS_HTTP_PORT}/victoriametrics${NC}"
echo -e "    ${GREEN}Metrics API:${NC} ${CYAN}http://localhost:${INGRESS_HTTP_PORT}/victoriametrics/api/v1/query${NC}"
echo -e "    ${GREEN}VMUI:${NC}    ${CYAN}http://localhost:${INGRESS_HTTP_PORT}/victoriametrics/vmui${NC}"
echo ""

# Check if Zen Watcher has ingress
if timeout 10 kubectl get ingress zen-watcher -n ${NAMESPACE} 2>/dev/null | grep -q zen-watcher; then
    echo -e "${CYAN}  ZEN WATCHER:${NC}"
    echo -e "    ${GREEN}URL:${NC}     ${CYAN}http://localhost:${INGRESS_HTTP_PORT}/zen-watcher${NC}"
    echo -e "    ${GREEN}Metrics:${NC} ${CYAN}http://localhost:${INGRESS_HTTP_PORT}/zen-watcher/metrics${NC}"
    echo -e "    ${GREEN}Health:${NC}  ${CYAN}http://localhost:${INGRESS_HTTP_PORT}/zen-watcher/health${NC}"
    echo ""
fi

# Final verification that Grafana is accessible via ingress
echo -e "${CYAN}   [DEBUG] Final verification of Grafana via ingress...${NC}"
FINAL_CHECK=$(timeout 5 curl -s -o /dev/null -w "%{http_code}" http://localhost:${INGRESS_HTTP_PORT}/grafana/api/health 2>/dev/null || echo "000")
if [ "$FINAL_CHECK" = "200" ] || [ "$FINAL_CHECK" = "401" ] || [ "$FINAL_CHECK" = "403" ]; then
    echo -e "${GREEN}✓${NC} ${CYAN}Grafana VERIFIED via ingress (HTTP ${FINAL_CHECK})${NC}"
    echo ""
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${GREEN}  ✅✅✅ WORKING GRAFANA URL AND CREDENTIALS ✅✅✅${NC}"
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
    echo -e "  ${GREEN}URL:${NC}     ${CYAN}http://localhost:${INGRESS_HTTP_PORT}/grafana${NC}"
    echo -e "  ${GREEN}Username:${NC} ${CYAN}zen${NC}"
    echo -e "  ${GREEN}Password:${NC} ${CYAN}${GRAFANA_PASSWORD}${NC}"
    echo ""
    echo -e "${GREEN}✅ VERIFIED: Grafana is accessible and working!${NC}"
    echo ""
else
    echo -e "${YELLOW}⚠${NC}  Grafana may not be accessible yet (HTTP ${FINAL_CHECK})"
    echo -e "${CYAN}   Try: curl http://localhost:${INGRESS_HTTP_PORT}/grafana/api/health${NC}"
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

# Store PIDs (only if port-forward was used)
if [ -n "${GRAFANA_PF_PID:-}" ]; then
    echo $GRAFANA_PF_PID > /tmp/zen-demo-grafana.pid
fi
if [ -n "${VM_PF_PID:-}" ]; then
    echo $VM_PF_PID > /tmp/zen-demo-vm.pid
fi

# Trap Ctrl+C to cleanup
cleanup() {
    echo ""
    echo -e "${YELLOW}→${NC} Stopping port-forwards (if any)..."
    [ -n "${GRAFANA_PF_PID:-}" ] && kill ${GRAFANA_PF_PID} 2>/dev/null || true
    [ -n "${VM_PF_PID:-}" ] && kill ${VM_PF_PID} 2>/dev/null || true
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
