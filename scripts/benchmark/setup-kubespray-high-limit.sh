#!/usr/bin/env bash
# Create Kubernetes cluster using kubespray with increased API server limits
# Usage: ./setup-kubespray-high-limit.sh
#
# Prerequisites:
# - kubespray cloned locally or available
# - Ansible installed
# - SSH access to target nodes (or local VMs)

set -euo pipefail

KUBESPRAY_DIR="${KUBESPRAY_DIR:-${HOME}/kubespray}"
CLUSTER_NAME="${CLUSTER_NAME:-zen-watcher-stress}"
INVENTORY_DIR="${INVENTORY_DIR:-${KUBESPRAY_DIR}/inventory/${CLUSTER_NAME}}"

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Setting up Kubernetes cluster with kubespray"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Kubespray dir: ${KUBESPRAY_DIR}"
echo "Cluster name: ${CLUSTER_NAME}"
echo ""

# Check if kubespray exists
if [ ! -d "${KUBESPRAY_DIR}" ]; then
    echo "❌ Error: kubespray not found at ${KUBESPRAY_DIR}"
    echo ""
    echo "To install kubespray:"
    echo "  git clone https://github.com/kubernetes-sigs/kubespray.git ${KUBESPRAY_DIR}"
    echo "  cd ${KUBESPRAY_DIR}"
    echo "  pip install -r requirements.txt"
    exit 1
fi

# Check if ansible is installed
if ! command -v ansible-playbook >/dev/null 2>&1; then
    echo "❌ Error: ansible-playbook not found"
    echo "   Install with: pip install ansible"
    exit 1
fi

cd "${KUBESPRAY_DIR}"

# Create inventory if it doesn't exist
if [ ! -d "${INVENTORY_DIR}" ]; then
    echo "Creating inventory from sample..."
    cp -rfp inventory/sample "${INVENTORY_DIR}"
    
    # Configure inventory for local testing (single node)
    # Edit inventory/hosts.yaml to configure your nodes
    echo ""
    echo "⚠️  Inventory created at: ${INVENTORY_DIR}"
    echo "   You need to edit ${INVENTORY_DIR}/hosts.yaml to configure your nodes"
    echo "   Example for localhost:"
    echo "     all:"
    echo "       hosts:"
    echo "         node1:"
    echo "           ansible_host: 127.0.0.1"
    echo "           ip: 127.0.0.1"
    echo "           access_ip: 127.0.0.1"
    exit 1
fi

# Configure API server limits in group_vars
echo "Configuring API server limits..."
mkdir -p "${INVENTORY_DIR}/group_vars/k8s_cluster"

cat >> "${INVENTORY_DIR}/group_vars/k8s_cluster/k8s-cluster.yml" <<EOF

# Custom API server limits for stress testing
kube_apiserver_extra_args:
  max-requests-inflight: "5000"
  max-mutating-requests-inflight: "2500"
  request-timeout: "300s"
EOF

echo "✓ API server limits configured"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Configuration complete"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Next steps:"
echo "  1. Edit ${INVENTORY_DIR}/hosts.yaml to configure your nodes"
echo "  2. Run: ansible-playbook -i ${INVENTORY_DIR}/hosts.yaml cluster.yml"
echo ""
echo "API Server Limits configured:"
echo "  - max-requests-inflight: 5000"
echo "  - max-mutating-requests-inflight: 2500"
echo "  - request-timeout: 300s"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

