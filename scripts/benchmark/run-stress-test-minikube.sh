#!/usr/bin/env bash
# Run stress test on minikube cluster
# Usage: ./run-stress-test-minikube.sh [--jobs N] [--rate N] [--qps N] [--client-burst N]

set -euo pipefail

CLUSTER_NAME="${CLUSTER_NAME:-zen-watcher-stress}"
KUBECTL_CONTEXT="minikube"
NAMESPACE="${NAMESPACE:-zen-system}"

# Load stress test image into minikube if it exists locally
STRESS_IMAGE="kubezen/zen-watcher-stress-test:latest"
if docker images "$STRESS_IMAGE" --format "{{.Repository}}:{{.Tag}}" 2>/dev/null | grep -q "$STRESS_IMAGE"; then
    echo "Loading stress test image into minikube cluster..."
    eval $(minikube -p "${CLUSTER_NAME}" docker-env)
    docker pull "$STRESS_IMAGE" 2>&1 | head -2 || echo "  (Image may already be loaded)"
fi

# Create service account for stress test if it doesn't exist
kubectl --context="$KUBECTL_CONTEXT" apply -f - <<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  name: zen-watcher-stress-test
  namespace: ${NAMESPACE}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: zen-watcher-stress-test
rules:
- apiGroups: ["zen.kube-zen.io"]
  resources: ["observations"]
  verbs: ["create", "get", "list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: zen-watcher-stress-test
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: zen-watcher-stress-test
subjects:
- kind: ServiceAccount
  name: zen-watcher-stress-test
  namespace: ${NAMESPACE}
EOF

# Run the parallel stress test script with minikube context
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

KUBECTL_CONTEXT="$KUBECTL_CONTEXT" NAMESPACE="$NAMESPACE" \
    ./run-parallel-stress-test.sh "$@"

