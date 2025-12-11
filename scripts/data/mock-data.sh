#!/bin/bash
#
# Zen Watcher - Mock Data Generator
#
# Creates demo Observation CRDs and a mock metrics server
# All demo resources are clearly labeled with demo.zen.kube-zen.io labels
#
# Usage:
#   ./scripts/data/mock-data.sh [namespace]
#
# Environment Variables:
#   NAMESPACE=zen-system           # Namespace for mock data (default: zen-system)

set -e

# Source common utilities
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../utils/common.sh"

NAMESPACE="${1:-${NAMESPACE:-zen-system}}"

log_step "Creating demo namespace..."
kubectl create namespace ${NAMESPACE} 2>/dev/null || true
kubectl create namespace demo-manifests 2>/dev/null || true

log_step "Creating demo Observation CRDs..."

# Helper function to create Observation CRD
create_observation() {
    local name=$1
    local source=$2
    local category=$3
    local severity=$4
    local event_type=$5
    local resource_kind=$6
    local resource_name=$7
    local resource_ns=$8
    local details_json=$9
    
    cat <<EOF | kubectl apply -f -
apiVersion: zen.kube-zen.io/v1
kind: Observation
metadata:
  name: ${name}
  namespace: ${NAMESPACE}
  labels:
    demo.zen.kube-zen.io/observation: "true"
    source: ${source}
    category: ${category}
    severity: ${severity}
spec:
  source: ${source}
  category: ${category}
  severity: ${severity}
  eventType: ${event_type}
  resource:
    apiVersion: v1
    kind: ${resource_kind}
    name: ${resource_name}
    namespace: ${resource_ns}
  detectedAt: "$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
  details:
${details_json}
status:
  synced: false
EOF
}

# Create demo observations from Trivy (vulnerabilities)
create_observation "demo-trivy-critical-1" "trivy" "security" "critical" "vulnerability" "Pod" "demo-insecure-pod" "demo-manifests" '    cve: "CVE-2024-0001"
    description: "Critical vulnerability in base image"
    package: "openssl"
    version: "1.1.1"'
create_observation "demo-trivy-critical-2" "trivy" "security" "critical" "vulnerability" "Pod" "demo-no-security-context" "demo-manifests" '    cve: "CVE-2024-0002"
    description: "Critical vulnerability in nginx"
    package: "nginx"
    version: "1.24.0"'
create_observation "demo-trivy-high-1" "trivy" "security" "high" "vulnerability" "Deployment" "demo-public-registry" "demo-manifests" '    cve: "CVE-2024-0003"
    description: "High severity vulnerability"
    package: "libc"
    version: "2.35"'
create_observation "demo-trivy-high-2" "trivy" "security" "high" "vulnerability" "Pod" "demo-insecure-pod" "demo-manifests" '    cve: "CVE-2024-0004"
    description: "High severity vulnerability"
    package: "curl"
    version: "7.85.0"'
create_observation "demo-trivy-medium-1" "trivy" "security" "medium" "vulnerability" "Deployment" "demo-public-registry" "demo-manifests" '    cve: "CVE-2024-0005"
    description: "Medium severity vulnerability"
    package: "bash"
    version: "5.1.16"'

# Create demo observations from Falco (runtime threats)
create_observation "demo-falco-critical-1" "falco" "security" "critical" "runtime-threat" "Pod" "demo-insecure-pod" "demo-manifests" '    rule: "Privileged container started"
    priority: "Critical"
    output: "Container running in privileged mode"'
create_observation "demo-falco-high-1" "falco" "security" "high" "runtime-threat" "Pod" "demo-insecure-pod" "demo-manifests" '    rule: "Sensitive file accessed"
    priority: "High"
    output: "Access to /etc/shadow detected"'
create_observation "demo-falco-high-2" "falco" "security" "high" "runtime-threat" "Pod" "demo-no-security-context" "demo-manifests" '    rule: "Unexpected network connection"
    priority: "High"
    output: "Connection to external IP detected"'

# Create demo observations from Kyverno (policy violations)
create_observation "demo-kyverno-medium-1" "kyverno" "security" "medium" "policy-violation" "Pod" "demo-no-security-context" "demo-manifests" '    policy: "require-security-context"
    rule: "requireSecurityContext"
    message: "Pod missing security context"'
create_observation "demo-kyverno-medium-2" "kyverno" "security" "medium" "policy-violation" "Pod" "demo-insecure-pod" "demo-manifests" '    policy: "disallow-privileged"
    rule: "disallowPrivileged"
    message: "Privileged containers not allowed"'
create_observation "demo-kyverno-low-1" "kyverno" "compliance" "low" "policy-violation" "Deployment" "demo-public-registry" "demo-manifests" '    policy: "require-resource-limits"
    rule: "requireResourceLimits"
    message: "Missing resource limits"'

# Create demo observations from Checkov (IaC scanning)
create_observation "demo-checkov-high-1" "checkov" "security" "high" "iac-scan" "Pod" "demo-insecure-pod" "demo-manifests" '    check: "CKV_K8S_1"
    guideline: "Ensure that the API Server pod specification file has permissions of 644 or more restrictive"'
create_observation "demo-checkov-medium-1" "checkov" "security" "medium" "iac-scan" "Pod" "demo-no-security-context" "demo-manifests" '    check: "CKV_K8S_24"
    guideline: "Ensure that the Pod Security Policy is set"'
create_observation "demo-checkov-medium-2" "checkov" "security" "medium" "iac-scan" "ServiceAccount" "demo-excessive-permissions" "demo-manifests" '    check: "CKV_K8S_14"
    guideline: "Ensure that the Service Account token is not mounted"'

# Create demo observations from kube-bench (CIS compliance)
create_observation "demo-kubebench-medium-1" "kube-bench" "compliance" "medium" "cis-benchmark" "Node" "demo-node" "" '    test: "1.2.1"
    description: "Ensure that the --anonymous-auth argument is set to false"'
create_observation "demo-kubebench-low-1" "kube-bench" "compliance" "low" "cis-benchmark" "Node" "demo-node" "" '    test: "1.2.2"
    description: "Ensure that the --basic-auth-file argument is not set"'

# Create demo observations from audit logs
create_observation "demo-audit-info-1" "audit" "compliance" "info" "audit-event" "ServiceAccount" "demo-excessive-permissions" "demo-manifests" '    action: "create"
    user: "system:serviceaccount:demo-manifests:demo-excessive-permissions"
    verb: "create"'
create_observation "demo-audit-info-2" "audit" "compliance" "info" "audit-event" "ClusterRoleBinding" "demo-excessive-binding" "" '    action: "create"
    user: "admin"
    verb: "create"'

log_success "Demo observations created"

# Deploy demo manifests for Checkov to scan
log_step "Deploying demo manifests for Checkov scanning..."
kubectl apply -f config/demo-manifests/ -n demo-manifests 2>/dev/null || echo "⚠ Demo manifests already exist or not found"

# Deploy a mock metrics server that exposes Prometheus metrics
log_step "Deploying mock metrics server..."
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: zen-watcher-mock
  namespace: ${NAMESPACE}
  labels:
    app: zen-watcher
    app.kubernetes.io/name: zen-watcher
    demo.zen.kube-zen.io/metrics: "true"
spec:
  containers:
  - name: metrics
    image: python:3.11-alpine
    command: ["/bin/sh", "-c"]
    args:
      - |
        python3 << 'PYEOF'
        from http.server import HTTPServer, BaseHTTPRequestHandler
        import time
        
        # Calculate metrics based on demo observations
        # These metrics simulate what zen-watcher would expose
        def get_metrics():
            now = int(time.time())
            return f"""# HELP zen_watcher_health_status Health status
# TYPE zen_watcher_health_status gauge
zen_watcher_health_status{{cluster_id="demo"}} 1

# HELP zen_watcher_events_total Total events collected
# TYPE zen_watcher_events_total counter
zen_watcher_events_total{{cluster_id="demo",category="security",source="trivy",event_type="vulnerability",severity="critical"}} 2
zen_watcher_events_total{{cluster_id="demo",category="security",source="trivy",event_type="vulnerability",severity="high"}} 2
zen_watcher_events_total{{cluster_id="demo",category="security",source="trivy",event_type="vulnerability",severity="medium"}} 1
zen_watcher_events_total{{cluster_id="demo",category="security",source="falco",event_type="runtime-threat",severity="critical"}} 1
zen_watcher_events_total{{cluster_id="demo",category="security",source="falco",event_type="runtime-threat",severity="high"}} 2
zen_watcher_events_total{{cluster_id="demo",category="security",source="kyverno",event_type="policy-violation",severity="medium"}} 2
zen_watcher_events_total{{cluster_id="demo",category="security",source="kyverno",event_type="policy-violation",severity="low"}} 1
zen_watcher_events_total{{cluster_id="demo",category="security",source="checkov",event_type="iac-scan",severity="high"}} 1
zen_watcher_events_total{{cluster_id="demo",category="security",source="checkov",event_type="iac-scan",severity="medium"}} 2
zen_watcher_events_total{{cluster_id="demo",category="compliance",source="kube-bench",event_type="cis-benchmark",severity="medium"}} 1
zen_watcher_events_total{{cluster_id="demo",category="compliance",source="kube-bench",event_type="cis-benchmark",severity="low"}} 1
zen_watcher_events_total{{cluster_id="demo",category="compliance",source="audit",event_type="audit-event",severity="info"}} 2

# HELP zen_watcher_active_events Currently active events
# TYPE zen_watcher_active_events gauge
zen_watcher_active_events{{cluster_id="demo",category="security",severity="critical"}} 3
zen_watcher_active_events{{cluster_id="demo",category="security",severity="high"}} 5
zen_watcher_active_events{{cluster_id="demo",category="security",severity="medium"}} 5
zen_watcher_active_events{{cluster_id="demo",category="security",severity="low"}} 1
zen_watcher_active_events{{cluster_id="demo",category="compliance",severity="medium"}} 1
zen_watcher_active_events{{cluster_id="demo",category="compliance",severity="low"}} 1
zen_watcher_active_events{{cluster_id="demo",category="compliance",severity="info"}} 2

# HELP zen_watcher_watcher_status Watcher enabled status
# TYPE zen_watcher_watcher_status gauge
zen_watcher_watcher_status{{cluster_id="demo",watcher="trivy"}} 1
zen_watcher_watcher_status{{cluster_id="demo",watcher="falco"}} 1
zen_watcher_watcher_status{{cluster_id="demo",watcher="kyverno"}} 1
zen_watcher_watcher_status{{cluster_id="demo",watcher="checkov"}} 1
zen_watcher_watcher_status{{cluster_id="demo",watcher="kube-bench"}} 1
zen_watcher_watcher_status{{cluster_id="demo",watcher="audit"}} 1
"""
        
        class Handler(BaseHTTPRequestHandler):
            def do_GET(self):
                if self.path == '/metrics':
                    self.send_response(200)
                    self.send_header("Content-Type", "text/plain; version=0.0.4")
                    self.end_headers()
                    self.wfile.write(get_metrics().encode())
                else:
                    self.send_response(404)
                    self.end_headers()
            def log_message(self, *args): pass
        
        print("Metrics server starting on :9090")
        HTTPServer(("0.0.0.0", 9090), Handler).serve_forever()
        PYEOF
    ports:
    - containerPort: 9090
      name: metrics
---
apiVersion: v1
kind: Service
metadata:
  name: zen-watcher-mock
  namespace: ${NAMESPACE}
  labels:
    app: zen-watcher
    demo.zen.kube-zen.io/metrics: "true"
spec:
  ports:
  - port: 9090
    targetPort: 9090
    name: metrics
  selector:
    app: zen-watcher
EOF

log_success "Mock metrics server deployed"
log_step "Waiting for pod..."
kubectl wait --for=condition=ready pod/zen-watcher-mock -n ${NAMESPACE} --timeout=60s 2>/dev/null || log_warn "Pod may take longer to start"
log_success "Demo data ready!"
echo ""
echo "📊 Created:"
echo "  - $(kubectl get observations -n ${NAMESPACE} --no-headers 2>/dev/null | wc -l | tr -d ' ') Observation CRDs"
echo "  - Mock metrics server on :9090"
echo "  - Demo manifests in demo-manifests namespace"
echo ""
echo "💡 View observations: kubectl get observations -n ${NAMESPACE}"
