#!/bin/bash
#
# Cluster utility functions
# Source this file: source "$(dirname "$0")/utils.sh"

# Source common utilities
# Only set SCRIPT_DIR if not already set (to avoid overwriting parent script's SCRIPT_DIR)
if [ -z "${SCRIPT_DIR:-}" ]; then
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
fi
CLUSTER_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${CLUSTER_SCRIPT_DIR}/../utils/common.sh"

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
    
    log_error "Could not find available port for $service (tried $base_port-$port)"
    exit 1
}

# Function to check if command exists
check_command() {
    if ! command -v $1 &> /dev/null; then
        log_error "$1 is not installed. Please install it first."
        echo "  Visit: $2"
        exit 1
    fi
    log_success "$1 found"
}

# Function to check if cluster exists
cluster_exists() {
    local platform="$1"
    local cluster_name="${2:-zen-demo}"
    
    case "$platform" in
        k3d)
            k3d cluster list 2>/dev/null | grep -q "^${cluster_name}" || return 1
            ;;
        kind)
            kind get clusters 2>/dev/null | grep -q "^${cluster_name}$" || return 1
            ;;
        minikube)
            minikube status -p "${cluster_name}" &>/dev/null || return 1
            ;;
        *)
            return 1
            ;;
    esac
}

# Function to get kubeconfig file path
get_kubeconfig_path() {
    local platform="$1"
    local cluster_name="${2:-zen-demo}"
    
    case "$platform" in
        k3d)
            echo "${HOME}/.kube/${cluster_name}-kubeconfig"
            ;;
        kind)
            echo "${HOME}/.kube/kind-${cluster_name}-config"
            ;;
        minikube)
            echo "${HOME}/.kube/config"
            ;;
        *)
            echo "${HOME}/.kube/config"
            ;;
    esac
}

# Function to setup kubeconfig for a cluster
setup_kubeconfig() {
    local platform="$1"
    local cluster_name="$2"
    local kubeconfig_file="$3"
    
    case "$platform" in
        k3d)
            # Wait a moment for k3d to finish setting up
            sleep 3
            sleep 5  # Give serverlb time to start
            
            # Update kubeconfig
            if ! timeout 10 k3d kubeconfig write ${cluster_name} --output ${kubeconfig_file} 2>/dev/null; then
                # Fallback: try to find the k3d config file
                K3D_CONFIG_PATH="${HOME}/.config/k3d/kubeconfig-${cluster_name}.yaml"
                if [ -f "$K3D_CONFIG_PATH" ]; then
                    cp "$K3D_CONFIG_PATH" ${kubeconfig_file} 2>/dev/null || true
                fi
            fi
            # Fix server URL to use 127.0.0.1
            if [ -f "${kubeconfig_file}" ]; then
                local api_port="${K3D_API_PORT:-6443}"
                sed -i.bak "s|0.0.0.0:${api_port}|127.0.0.1:${api_port}|g" ${kubeconfig_file} 2>/dev/null || true
                sed -i.bak "s|server: https://.*:${api_port}|server: https://127.0.0.1:${api_port}|g" ${kubeconfig_file} 2>/dev/null || true
                rm -f ${kubeconfig_file}.bak 2>/dev/null || true
                # Remove certificate authority data and add insecure skip
                kubectl config unset clusters.k3d-${cluster_name}.certificate-authority-data --kubeconfig=${kubeconfig_file} >/dev/null 2>&1 || true
                kubectl config set clusters.k3d-${cluster_name}.insecure-skip-tls-verify true --kubeconfig=${kubeconfig_file} >/dev/null 2>&1 || true
            fi
            ;;
        kind)
            kind export kubeconfig --name ${cluster_name} --kubeconfig=${kubeconfig_file} 2>/dev/null || true
            ;;
        minikube)
            minikube update-context -p ${cluster_name} 2>/dev/null || true
            cp ${HOME}/.kube/config ${kubeconfig_file} 2>/dev/null || true
            ;;
    esac
    
    if [ -f "${kubeconfig_file}" ]; then
        chmod 600 ${kubeconfig_file} 2>/dev/null || true
    fi
}

# Function to wait for cluster to be ready
wait_for_cluster() {
    local platform="$1"
    local cluster_name="$2"
    local kubeconfig_file="$3"
    local max_wait="${4:-120}"
    
    log_step "Waiting for cluster to be ready..."
    
    local waited=0
    while [ $waited -lt $max_wait ]; do
        if KUBECONFIG=${kubeconfig_file} kubectl get nodes --request-timeout=5s &>/dev/null 2>&1; then
            log_success "Cluster is ready"
            return 0
        fi
        sleep 2
        waited=$((waited + 2))
    done
    
    log_error "Cluster did not become ready within ${max_wait}s"
    return 1
}

