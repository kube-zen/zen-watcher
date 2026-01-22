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
create_observation "demo-falco-critical-1" "falco" "security" "critical" "runtime_threat" "Pod" "demo-insecure-pod" "demo-manifests" '    rule: "Privileged container started"
    priority: "Critical"
    output: "Container running in privileged mode"'
create_observation "demo-falco-high-1" "falco" "security" "high" "runtime_threat" "Pod" "demo-insecure-pod" "demo-manifests" '    rule: "Sensitive file accessed"
    priority: "High"
    output: "Access to /etc/shadow detected"'
create_observation "demo-falco-high-2" "falco" "security" "high" "runtime_threat" "Pod" "demo-no-security-context" "demo-manifests" '    rule: "Unexpected network connection"
    priority: "High"
    output: "Connection to external IP detected"'

# Create demo observations from Kyverno (policy violations)
create_observation "demo-kyverno-medium-1" "kyverno" "security" "medium" "policy_violation" "Pod" "demo-no-security-context" "demo-manifests" '    policy: "require-security-context"
    rule: "requireSecurityContext"
    message: "Pod missing security context"'
create_observation "demo-kyverno-medium-2" "kyverno" "security" "medium" "policy_violation" "Pod" "demo-insecure-pod" "demo-manifests" '    policy: "disallow-privileged"
    rule: "disallowPrivileged"
    message: "Privileged containers not allowed"'
create_observation "demo-kyverno-low-1" "kyverno" "compliance" "low" "policy_violation" "Deployment" "demo-public-registry" "demo-manifests" '    policy: "require-resource-limits"
    rule: "requireResourceLimits"
    message: "Missing resource limits"'

# Create demo observations from Checkov (IaC scanning)
create_observation "demo-checkov-high-1" "checkov" "security" "high" "iac_scan" "Pod" "demo-insecure-pod" "demo-manifests" '    check: "CKV_K8S_1"
    guideline: "Ensure that the API Server pod specification file has permissions of 644 or more restrictive"'
create_observation "demo-checkov-medium-1" "checkov" "security" "medium" "iac_scan" "Pod" "demo-no-security-context" "demo-manifests" '    check: "CKV_K8S_24"
    guideline: "Ensure that the Pod Security Policy is set"'
create_observation "demo-checkov-medium-2" "checkov" "security" "medium" "iac_scan" "ServiceAccount" "demo-excessive-permissions" "demo-manifests" '    check: "CKV_K8S_14"
    guideline: "Ensure that the Service Account token is not mounted"'

# Create demo observations from kube-bench (CIS compliance)
create_observation "demo-kubebench-medium-1" "kube-bench" "compliance" "medium" "cis_benchmark" "Node" "demo-node" "" '    test: "1.2.1"
    description: "Ensure that the --anonymous-auth argument is set to false"'
create_observation "demo-kubebench-low-1" "kube-bench" "compliance" "low" "cis_benchmark" "Node" "demo-node" "" '    test: "1.2.2"
    description: "Ensure that the --basic-auth-file argument is not set"'

# Create demo observations from audit logs
create_observation "demo-audit-info-1" "audit" "compliance" "info" "audit_event" "ServiceAccount" "demo-excessive-permissions" "demo-manifests" '    action: "create"
    user: "system:serviceaccount:demo-manifests:demo-excessive-permissions"
    verb: "create"'
create_observation "demo-audit-info-2" "audit" "compliance" "info" "audit_event" "ClusterRoleBinding" "demo-excessive-binding" "" '    action: "create"
    user: "admin"
    verb: "create"'

# Create demo observations from cert-manager
create_observation "demo-cert-manager-warning-1" "cert-manager" "security" "medium" "cert_manager_event" "Certificate" "demo-cert" "demo-manifests" '    reason: "CertificateExpiring"
    message: "Certificate expiring in 30 days"
    issuer: "letsencrypt-prod"'
create_observation "demo-cert-manager-info-1" "cert-manager" "operations" "low" "cert_manager_event" "CertificateRequest" "demo-cert-request" "demo-manifests" '    reason: "CertificateIssued"
    message: "Certificate successfully issued"'

# Create demo observations from sealed-secrets
create_observation "demo-sealed-secrets-error-1" "sealed-secrets" "security" "high" "sealed_secrets_event" "SealedSecret" "demo-secret" "demo-manifests" '    reason: "DecryptionFailed"
    message: "Failed to decrypt sealed secret"'
create_observation "demo-sealed-secrets-info-1" "sealed-secrets" "operations" "low" "sealed_secrets_event" "SealedSecret" "demo-secret-2" "demo-manifests" '    reason: "SecretCreated"
    message: "Secret successfully created from sealed secret"'

# Create demo observations from kubernetes-events
create_observation "demo-kubernetes-events-warning-1" "kubernetes-events" "operations" "medium" "kubernetes_event" "Pod" "demo-insecure-pod" "demo-manifests" '    reason: "Failed"
    message: "Pod failed to start"
    type: "Warning"'
create_observation "demo-kubernetes-events-info-1" "kubernetes-events" "operations" "info" "kubernetes_event" "Deployment" "demo-public-registry" "demo-manifests" '    reason: "ScalingReplicaSet"
    message: "Scaled up replica set"
    type: "Normal"'

# Create demo observations from prometheus
create_observation "demo-prometheus-critical-1" "prometheus" "operations" "critical" "prometheus_alert" "Pod" "demo-insecure-pod" "demo-manifests" '    alert: "HighMemoryUsage"
    message: "Memory usage above 90%"
    severity: "critical"'
create_observation "demo-prometheus-warning-1" "prometheus" "operations" "medium" "prometheus_alert" "Deployment" "demo-public-registry" "demo-manifests" '    alert: "HighCPUUsage"
    message: "CPU usage above 80%"
    severity: "warning"'

# Create demo observations from opa-gatekeeper
create_observation "demo-opa-violation-1" "opa-gatekeeper" "security" "high" "opa_violation" "Pod" "demo-insecure-pod" "demo-manifests" '    constraint: "K8sRequiredLabels"
    message: "Pod missing required labels"
    enforcementAction: "deny"'
create_observation "demo-opa-violation-2" "opa-gatekeeper" "compliance" "medium" "opa_violation" "Deployment" "demo-public-registry" "demo-manifests" '    constraint: "K8sResourceLimits"
    message: "Container missing resource limits"
    enforcementAction: "dryrun"'

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
    app: zen-watcher-mock
    app.kubernetes.io/name: zen-watcher-mock
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
            # Emit metrics that match the *current* dashboard PromQL:
            # - zen_watcher_events_total{source,category,severity,eventType,namespace,kind,strategy}
            # - zen_watcher_observations_created_total{source}
            # - zen_watcher_tools_active{tool}
            return """# HELP zen_watcher_events_total Total number of events that resulted in Observation CRD creation
# TYPE zen_watcher_events_total counter
zen_watcher_events_total{source="falco",category="security",severity="critical",eventType="runtime_threat",namespace="demo-manifests",kind="Pod",strategy="filter_first"} 3
zen_watcher_events_total{source="falco",category="security",severity="high",eventType="runtime_threat",namespace="demo-manifests",kind="Pod",strategy="filter_first"} 5
zen_watcher_events_total{source="audit",category="security",severity="medium",eventType="audit_event",namespace="default",kind="pods",strategy="filter_first"} 4
zen_watcher_events_total{source="trivy",category="security",severity="critical",eventType="vulnerability",namespace="demo-manifests",kind="Pod",strategy="filter_first"} 2
zen_watcher_events_total{source="checkov",category="security",severity="high",eventType="iac_scan",namespace="checkov",kind="ConfigMap",strategy="filter_first"} 4
zen_watcher_events_total{source="kube-bench",category="compliance",severity="medium",eventType="cis_benchmark",namespace="zen-system",kind="ConfigMap",strategy="filter_first"} 2

# HELP zen_watcher_observations_created_total Total number of Observation CRDs successfully created
# TYPE zen_watcher_observations_created_total counter
zen_watcher_observations_created_total{source="falco"} 8
zen_watcher_observations_created_total{source="audit"} 4
zen_watcher_observations_created_total{source="trivy"} 5
zen_watcher_observations_created_total{source="checkov"} 4
zen_watcher_observations_created_total{source="kube-bench"} 2

# HELP zen_watcher_tools_active Number of security tools currently detected (1=active, 0=inactive)
# TYPE zen_watcher_tools_active gauge
zen_watcher_tools_active{tool="falco"} 1
zen_watcher_tools_active{tool="audit"} 1
zen_watcher_tools_active{tool="trivy"} 1
zen_watcher_tools_active{tool="checkov"} 1
zen_watcher_tools_active{tool="kube-bench"} 1
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
    app: zen-watcher-mock
    demo.zen.kube-zen.io/metrics: "true"
  annotations:
    prometheus.io/scrape: "true"
    prometheus.io/port: "9090"
    prometheus.io/path: "/metrics"
spec:
  ports:
  - port: 9090
    targetPort: 9090
    name: metrics
  selector:
    app: zen-watcher-mock
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
