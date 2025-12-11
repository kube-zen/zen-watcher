#!/usr/bin/env bash
# Install zen-watcher in minikube cluster for stress testing
# Usage: ./install-zen-watcher-minikube.sh [cluster-name]

set -euo pipefail

CLUSTER_NAME="${1:-zen-watcher-stress}"
KUBECTL_CONTEXT="minikube"
NAMESPACE="${NAMESPACE:-zen-system}"
IMAGE="${IMAGE:-kubezen/zen-watcher:1.0.0-alpha}"

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Installing zen-watcher in minikube cluster"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Cluster: ${CLUSTER_NAME}"
echo "Context: ${KUBECTL_CONTEXT}"
echo "Namespace: ${NAMESPACE}"
echo "Image: ${IMAGE}"
echo ""

# Verify cluster exists
if ! kubectl --context="$KUBECTL_CONTEXT" cluster-info >/dev/null 2>&1; then
    echo "❌ Error: minikube cluster not found or not accessible"
    echo "   Create it first with: ./setup-minikube-high-limit.sh"
    exit 1
fi

# Load image into minikube if it exists locally
if docker images "$IMAGE" --format "{{.Repository}}:{{.Tag}}" 2>/dev/null | grep -q "$IMAGE"; then
    echo "Loading image into minikube cluster..."
    eval $(minikube -p "${CLUSTER_NAME}" docker-env)
    docker pull "$IMAGE" 2>&1 | head -3 || echo "  (Image may already be loaded)"
else
    echo "⚠️  Image ${IMAGE} not found locally. Make sure it's available in the cluster."
    echo "   You may need to: docker pull ${IMAGE}"
fi

# Install CRDs
echo ""
echo "Step 1: Installing CRDs..."
CRD_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../deployments/crds" && pwd)"
if [ -f "${CRD_DIR}/ingester_crd.yaml" ]; then
    kubectl --context="$KUBECTL_CONTEXT" apply -f "${CRD_DIR}/ingester_crd.yaml"
    echo "✓ Ingester CRD installed"
else
    echo "⚠️  CRD file not found at ${CRD_DIR}/ingester_crd.yaml"
fi

# Create service account and RBAC
echo ""
echo "Step 2: Creating service account and RBAC..."
kubectl --context="$KUBECTL_CONTEXT" apply -f - <<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  name: zen-watcher
  namespace: ${NAMESPACE}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: zen-watcher
rules:
- apiGroups: ["zen.kube-zen.io"]
  resources: ["observations", "ingesters"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: [""]
  resources: ["events"]
  verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: zen-watcher
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: zen-watcher
subjects:
- kind: ServiceAccount
  name: zen-watcher
  namespace: ${NAMESPACE}
EOF
echo "✓ RBAC created"

# Create deployment
echo ""
echo "Step 3: Creating zen-watcher deployment..."
kubectl --context="$KUBECTL_CONTEXT" apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: zen-watcher
  namespace: ${NAMESPACE}
  labels:
    app.kubernetes.io/name: zen-watcher
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: zen-watcher
  template:
    metadata:
      labels:
        app.kubernetes.io/name: zen-watcher
    spec:
      serviceAccountName: zen-watcher
      containers:
      - name: zen-watcher
        image: ${IMAGE}
        imagePullPolicy: IfNotPresent
        ports:
        - name: http
          containerPort: 8080
        - name: metrics
          containerPort: 9090
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 2000m
            memory: 2Gi
        env:
        - name: LOG_LEVEL
          value: "info"
        - name: METRICS_ENABLED
          value: "true"
EOF
echo "✓ Deployment created"

# Wait for deployment to be ready
echo ""
echo "Step 4: Waiting for zen-watcher to be ready..."
kubectl --context="$KUBECTL_CONTEXT" wait --for=condition=available --timeout=120s \
    deployment/zen-watcher -n "${NAMESPACE}"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✅ zen-watcher installed successfully"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Check status:"
echo "  kubectl --context=${KUBECTL_CONTEXT} get pods -n ${NAMESPACE}"
echo "  kubectl --context=${KUBECTL_CONTEXT} logs -n ${NAMESPACE} -l app.kubernetes.io/name=zen-watcher"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

