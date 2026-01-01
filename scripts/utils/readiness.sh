#!/bin/bash
# Comprehensive readiness checks for all components
#
# Usage:
#   source scripts/utils/readiness.sh
#   wait_for_deployment zen-system zen-watcher
#   wait_for_grafana_api zen-system
#   wait_for_zen_watcher zen-system
#
# Supports KUBECTL_CONTEXT environment variable:
#   KUBECTL_CONTEXT=k3d-zen-demo source scripts/utils/readiness.sh

set -euo pipefail

# Build kubectl command with context if provided
KUBECTL_CMD="kubectl"
if [ -n "${KUBECTL_CONTEXT:-}" ]; then
    KUBECTL_CMD="kubectl --context=${KUBECTL_CONTEXT}"
fi

wait_for_deployment() {
    local namespace="$1"
    local deployment="$2"
    local timeout="${3:-300}"
    
    echo "Waiting for deployment $deployment in namespace $namespace..."
    if $KUBECTL_CMD wait --for=condition=available deployment/"$deployment" -n "$namespace" --timeout="${timeout}s" 2>/dev/null; then
        echo "✓ Deployment $deployment is available"
        return 0
    else
        echo "✗ Timeout waiting for deployment $deployment"
        return 1
    fi
}

wait_for_grafana_api() {
    local namespace="$1"
    local timeout="${2:-300}"
    local start_time=$(date +%s)
    
    echo "Waiting for Grafana API to be ready..."
    while [ $(($(date +%s) - start_time)) -lt "$timeout" ]; do
        if $KUBECTL_CMD exec -n "$namespace" deployment/grafana -- curl -s localhost:3000/api/health >/dev/null 2>&1; then
            echo "✓ Grafana API is ready!"
            return 0
        fi
        sleep 10
    done
    
    echo "✗ Timeout waiting for Grafana API"
    return 1
}

check_ingress_readiness() {
    local namespace="$1"
    local timeout="${2:-300}"
    
    echo "Checking ingress controller readiness..."
    if $KUBECTL_CMD wait --for=condition=ready pod -n "$namespace" -l app.kubernetes.io/name=ingress-nginx --timeout="${timeout}s" 2>/dev/null; then
        echo "✓ Ingress controller is ready"
        return 0
    else
        echo "✗ Timeout waiting for ingress controller"
        return 1
    fi
}

wait_for_zen_watcher() {
    local namespace="$1"
    local timeout="${2:-300}"
    
    echo "Waiting for zen-watcher to be ready..."
    if ! $KUBECTL_CMD wait --for=condition=ready pod -n "$namespace" -l app=zen-watcher --timeout="${timeout}s" 2>/dev/null; then
        echo "✗ Timeout waiting for zen-watcher pods"
        return 1
    fi
    
    # Additional health check
    local start_time=$(date +%s)
    while [ $(($(date +%s) - start_time)) -lt "$timeout" ]; do
        local pod_name=$($KUBECTL_CMD get pod -n "$namespace" -l app=zen-watcher -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
        if [ -n "$pod_name" ]; then
            local phase=$($KUBECTL_CMD get pod -n "$namespace" "$pod_name" -o jsonpath='{.status.phase}' 2>/dev/null || echo "")
            if [ "$phase" = "Running" ]; then
                # Check if metrics endpoint is responding
                if $KUBECTL_CMD exec -n "$namespace" "$pod_name" -- curl -s localhost:9090/metrics >/dev/null 2>&1; then
                    echo "✓ Zen-watcher is fully ready!"
                    return 0
                fi
            fi
        fi
        sleep 5
    done
    
    echo "✗ Timeout waiting for zen-watcher full readiness"
    return 1
}

wait_for_crd() {
    local crd_name="$1"
    local timeout="${2:-300}"
    
    echo "Waiting for CRD $crd_name to be available..."
    if $KUBECTL_CMD wait --for=condition=established crd/"$crd_name" --timeout="${timeout}s" 2>/dev/null; then
        echo "✓ CRD $crd_name is available"
        return 0
    else
        echo "✗ Timeout waiting for CRD $crd_name"
        return 1
    fi
}

