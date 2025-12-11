#!/usr/bin/env bash
# Setup local Kubernetes cluster using kubeadm with increased API server limits
# Usage: ./setup-kubeadm-local.sh

set -euo pipefail

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Setting up local Kubernetes cluster with kubeadm"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Check if kubeadm is installed
if ! command -v kubeadm >/dev/null 2>&1; then
    echo "❌ Error: kubeadm not found"
    echo "   Install with: sudo apt-get install -y kubeadm kubelet kubectl"
    exit 1
fi

# Check if running as root or with sudo
if [ "$EUID" -ne 0 ]; then
    echo "⚠️  This script needs root privileges for kubeadm"
    echo "   Run with: sudo ./setup-kubeadm-local.sh"
    exit 1
fi

# Get a real IP address (not localhost)
HOST_IP=$(ip route get 8.8.8.8 2>/dev/null | awk '{print $7; exit}' || echo "192.168.1.1")
echo "Using host IP: $HOST_IP"

# Create kubeadm config with increased API server limits
cat > /tmp/kubeadm-config.yaml <<EOF
apiVersion: kubeadm.k8s.io/v1beta3
kind: ClusterConfiguration
kubernetesVersion: v1.28.15
apiServer:
  extraArgs:
    max-requests-inflight: "5000"
    max-mutating-requests-inflight: "2500"
    request-timeout: "300s"
networking:
  podSubnet: "10.244.0.0/16"
---
apiVersion: kubeadm.k8s.io/v1beta3
kind: InitConfiguration
localAPIEndpoint:
  advertiseAddress: "$HOST_IP"
  bindPort: 6443
EOF

echo "Initializing kubeadm cluster with increased API server limits..."
kubeadm init --config=/tmp/kubeadm-config.yaml

echo ""
echo "Setting up kubeconfig..."
mkdir -p "$HOME/.kube"
cp -i /etc/kubernetes/admin.conf "$HOME/.kube/config"
chown "$(id -u):$(id -g)" "$HOME/.kube/config"

echo ""
echo "Installing CNI (Flannel)..."
kubectl apply -f https://github.com/flannel-io/flannel/releases/latest/download/kube-flannel.yml

echo ""
echo "Removing taint to allow scheduling on master..."
kubectl taint nodes --all node-role.kubernetes.io/control-plane-

echo ""
echo "Waiting for cluster to be ready..."
kubectl wait --for=condition=ready node --all --timeout=300s

echo ""
echo "Creating zen-system namespace..."
kubectl create namespace zen-system

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✅ Local Kubernetes cluster created"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "API Server Limits:"
echo "  - max-requests-inflight: 5000"
echo "  - max-mutating-requests-inflight: 2500"
echo "  - request-timeout: 300s"
echo ""
echo "Next steps:"
echo "  1. Install CRDs: kubectl apply -f deployments/crds/*.yaml"
echo "  2. Deploy zen-watcher"
echo "  3. Run stress tests"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

