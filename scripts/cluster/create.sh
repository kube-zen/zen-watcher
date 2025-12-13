#!/bin/bash
#
# Zen Watcher - Cluster Creation Script
#
# Creates a Kubernetes cluster using k3d, kind, or minikube
#
# Usage:
#   ./scripts/cluster/create.sh <platform> <cluster_name> [options]
#
# Options:
#   --use-existing          Use existing cluster if it exists
#   --no-docker-login       Don't use docker login credentials

set -euo pipefail

# Source utilities
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../utils/common.sh"
source "${SCRIPT_DIR}/utils.sh" 2>/dev/null || source "${SCRIPT_DIR}/cluster/utils.sh" 2>/dev/null || true

PLATFORM="${1:-k3d}"
CLUSTER_NAME="${2:-zen-demo}"
USE_EXISTING=false
NO_DOCKER_LOGIN=false

# Parse options
shift 2 2>/dev/null || shift 1 2>/dev/null || true
for arg in "$@"; do
    case "$arg" in
        --use-existing)
            USE_EXISTING=true
            ;;
        --no-docker-login)
            NO_DOCKER_LOGIN=true
            ;;
    esac
done

# Port configuration
K3D_API_PORT="${K3D_API_PORT:-6443}"
KIND_API_PORT="${KIND_API_PORT:-6443}"
MINIKUBE_API_PORT="${MINIKUBE_API_PORT:-8443}"
INGRESS_HTTP_PORT="${INGRESS_HTTP_PORT:-8080}"

log_step "Creating ${PLATFORM} cluster '${CLUSTER_NAME}'..."

case "$PLATFORM" in
    k3d)
        if [ "$USE_EXISTING" = true ] && cluster_exists "k3d" "$CLUSTER_NAME"; then
            log_info "Using existing k3d cluster '${CLUSTER_NAME}'"
            exit 0
        fi
        
        if cluster_exists "k3d" "$CLUSTER_NAME"; then
            log_error "Cluster '${CLUSTER_NAME}' already exists"
            exit 1
        fi
        
        # Check if ingress HTTP port is available, find alternative if needed
        if ! check_port "${INGRESS_HTTP_PORT}" "k3d ingress HTTP"; then
            log_warn "Port ${INGRESS_HTTP_PORT} is already in use, finding alternative..."
            INGRESS_HTTP_PORT=$(find_available_port "${INGRESS_HTTP_PORT}" "k3d ingress HTTP")
            log_info "Using port ${INGRESS_HTTP_PORT} for ingress HTTP"
        fi
        
        # Check if ingress HTTPS port is available
        INGRESS_HTTPS_PORT=$((INGRESS_HTTP_PORT + 1))
        if ! check_port "${INGRESS_HTTPS_PORT}" "k3d ingress HTTPS"; then
            log_warn "Port ${INGRESS_HTTPS_PORT} is already in use, finding alternative..."
            INGRESS_HTTPS_PORT=$(find_available_port "${INGRESS_HTTPS_PORT}" "k3d ingress HTTPS")
            log_info "Using port ${INGRESS_HTTPS_PORT} for ingress HTTPS"
        fi
        
        # Export the ports so they're available to other scripts
        export INGRESS_HTTP_PORT
        export INGRESS_HTTPS_PORT
        
        # Build k3d command
        k3d_create_args=(
            "cluster" "create" "${CLUSTER_NAME}"
            "--agents" "0"
            "--host-pid-mode"
            "--k3s-arg" "--disable=traefik@server:0"
            "--port" "${INGRESS_HTTP_PORT}:80@loadbalancer"
            "--port" "${INGRESS_HTTPS_PORT}:443@loadbalancer"
        )
        
        # Handle API port
        if [ "${K3D_API_PORT}" != "6443" ]; then
            if ! check_port "${K3D_API_PORT}" "k3d API"; then
                log_error "Port ${K3D_API_PORT} is already in use"
                exit 1
            fi
            k3d_create_args+=("--api-port" "${K3D_API_PORT}")
        else
            # Check if default port is available
            if ! check_port 6443 "k3d API" 2>/dev/null; then
                # Find available port
                found_port=$(find_available_port 6550 "k3d API")
                k3d_create_args+=("--api-port" "${found_port}")
                K3D_API_PORT=${found_port}
            else
                k3d_create_args+=("--api-port" "${K3D_API_PORT}")
            fi
        fi
        
        # Handle docker login
        if [ "$NO_DOCKER_LOGIN" = true ]; then
            export DOCKER_CONFIG=""
        fi
        
        # Create cluster
        if timeout 240 k3d "${k3d_create_args[@]}" 2>&1 | tee /tmp/k3d-create.log; then
            log_success "Cluster created successfully"
        else
            exit_code=$?
            if cluster_exists "k3d" "$CLUSTER_NAME"; then
                log_warn "Cluster creation timed out, but cluster exists - continuing..."
            else
                log_error "Cluster creation failed"
                log_info "Check logs: cat /tmp/k3d-create.log"
                exit 1
            fi
        fi
        ;;
    kind)
        if [ "$USE_EXISTING" = true ] && cluster_exists "kind" "$CLUSTER_NAME"; then
            log_info "Using existing kind cluster '${CLUSTER_NAME}'"
            exit 0
        fi
        
        if cluster_exists "kind" "$CLUSTER_NAME"; then
            log_error "Cluster '${CLUSTER_NAME}' already exists"
            exit 1
        fi
        
        # Create kind config
        cat > /tmp/kind-config-${CLUSTER_NAME}.yaml <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  apiServerPort: ${KIND_API_PORT}
EOF
        
        if timeout 180 kind create cluster --name ${CLUSTER_NAME} --config /tmp/kind-config-${CLUSTER_NAME}.yaml --wait 2m 2>&1 | tee /tmp/kind-create.log; then
            rm -f /tmp/kind-config-${CLUSTER_NAME}.yaml
            log_success "Cluster created successfully"
        else
            exit_code=$?
            rm -f /tmp/kind-config-${CLUSTER_NAME}.yaml
            log_error "Cluster creation failed or timed out"
            log_info "Check logs: cat /tmp/kind-create.log"
            exit 1
        fi
        ;;
    minikube)
        if [ "$USE_EXISTING" = true ] && cluster_exists "minikube" "$CLUSTER_NAME"; then
            log_info "Using existing minikube profile '${CLUSTER_NAME}'"
            exit 0
        fi
        
        if cluster_exists "minikube" "$CLUSTER_NAME"; then
            log_error "Profile '${CLUSTER_NAME}' already exists"
            exit 1
        fi
        
        if timeout 300 minikube start -p ${CLUSTER_NAME} \
            --cpus 4 \
            --memory 8192 \
            --apiserver-port=${MINIKUBE_API_PORT} 2>&1 | tee /tmp/minikube-create.log; then
            log_success "Cluster created successfully"
        else
            exit_code=$?
            log_error "Cluster creation failed or timed out"
            log_info "Check logs: cat /tmp/minikube-create.log"
            exit 1
        fi
        ;;
    *)
        log_error "Unknown platform: $PLATFORM"
        echo "  Supported: k3d, kind, minikube"
        exit 1
        ;;
esac

