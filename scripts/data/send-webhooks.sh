#!/bin/bash
#
# Zen Watcher - Send Mock Webhooks
#
# Sends mock webhooks to zen-watcher for Falco and Audit events
# Creates PolicyReports for Checkov and kube-bench
# This simulates real data from all sources to test dashboard
#

set -euo pipefail

NAMESPACE="${1:-zen-system}"
ZEN_WATCHER_URL="${ZEN_WATCHER_URL:-http://zen-watcher.${NAMESPACE}.svc.cluster.local:8080}"

# Generate a unique, DNS-safe pod name suffix
unique_suffix() {
    # Prefer nanoseconds + random to avoid collisions even in fast loops
    local ns rnd
    ns="$(date +%s%N 2>/dev/null || date +%s)"
    rnd="$RANDOM"
    # Produce a DNS-safe suffix (lowercase alnum + '-') and avoid trailing '-'
    # Keep it short to fit Kubernetes name limits when prefixed.
    echo "${ns}${rnd}" | tr -cd '0-9' | tail -c 16
}

# Wait for a pod to finish (Succeeded or Failed) instead of waiting for Ready
wait_pod_done() {
    local pod="$1"
    local ns="$2"
    local timeout="${3:-30s}"

    # kubectl wait supports condition=Complete only for Jobs, so we poll phase here
    local start now elapsed
    start="$(date +%s)"
    while true; do
        phase="$(kubectl get pod "${pod}" -n "${ns}" -o jsonpath='{.status.phase}' 2>/dev/null || echo "Unknown")"
        if [[ "${phase}" == "Succeeded" || "${phase}" == "Failed" ]]; then
            return 0
        fi
        now="$(date +%s)"
        elapsed=$((now - start))
        # crude timeout parsing: treat "30s" as seconds
        max="${timeout%s}"
        if [[ "${elapsed}" -ge "${max}" ]]; then
            return 1
        fi
        sleep 1
    done
}

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  Sending Mock Webhooks to zen-watcher${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Check if zen-watcher is accessible from within cluster
# Use a pod with PodSecurity compliance to test connectivity
cat <<EOF | kubectl apply -f - >/dev/null 2>&1
apiVersion: v1
kind: Pod
metadata:
  name: test-zen-watcher-connectivity
  namespace: ${NAMESPACE}
spec:
  securityContext:
    runAsNonRoot: true
    runAsUser: 65534
    seccompProfile:
      type: RuntimeDefault
  containers:
  - name: curl
    image: curlimages/curl:latest
    securityContext:
      allowPrivilegeEscalation: false
      capabilities:
        drop: ["ALL"]
      runAsNonRoot: true
      runAsUser: 65534
      seccompProfile:
        type: RuntimeDefault
    command: ["sh", "-c", "curl -s -f ${ZEN_WATCHER_URL}/health || exit 1"]
  restartPolicy: Never
EOF

# Wait for pod to finish
if wait_pod_done test-zen-watcher-connectivity "${NAMESPACE}" 30; then
    if kubectl logs test-zen-watcher-connectivity -n ${NAMESPACE} >/dev/null 2>&1; then
        echo -e "${GREEN}✓${NC} zen-watcher accessible at ${ZEN_WATCHER_URL} (internal service)"
    else
        echo -e "${YELLOW}⚠${NC}  zen-watcher connectivity check inconclusive, continuing..."
    fi
    kubectl delete pod test-zen-watcher-connectivity -n ${NAMESPACE} >/dev/null 2>&1 || true
else
    echo -e "${YELLOW}⚠${NC}  zen-watcher connectivity check timed out, continuing anyway..."
    kubectl delete pod test-zen-watcher-connectivity -n ${NAMESPACE} >/dev/null 2>&1 || true
fi
echo ""

# Function to send Falco webhook
send_falco_webhook() {
    local priority=$1
    local rule=$2
    local output=$3
    local pod_name=${4:-"demo-pod"}
    local pod_ns=${5:-"default"}
    
    local payload=$(cat <<EOF
{
  "output": "${output}",
  "priority": "${priority}",
  "rule": "${rule}",
  "time": "$(date -u +"%Y-%m-%dT%H:%M:%S.%3NZ")",
  "output_fields": {
    "k8s.pod.name": "${pod_name}",
    "k8s.ns.name": "${pod_ns}",
    "container.id": "1234567890abcdef",
    "container.name": "demo-container"
  },
  "source": "syscall",
  "tags": ["container", "mitre"]
}
EOF
)
    # Compact to a single line so YAML/JSON embedding is safe
    payload="$(echo "${payload}" | tr -d '\n' | sed 's/[[:space:]]\\+/ /g')"
    
    echo -e "${CYAN}  →${NC} Sending Falco webhook: ${rule} (${priority})"
    # Send webhook from within cluster using a PodSecurity-compliant pod with app label
    POD_NAME="send-falco-webhook-$(unique_suffix)"
    # Encode payload to avoid YAML quoting issues in the pod spec
    PAYLOAD_B64="$(printf '%s' "${payload}" | base64 | tr -d '\n')"
    # NOTE: don't suppress errors here; failures are important for debugging in demos
    cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: ${POD_NAME}
  namespace: ${NAMESPACE}
  labels:
    app: webhook-sender
spec:
  securityContext:
    runAsNonRoot: true
    runAsUser: 65534
    seccompProfile:
      type: RuntimeDefault
  containers:
  - name: curl
    image: curlimages/curl:latest
    securityContext:
      allowPrivilegeEscalation: false
      capabilities:
        drop: ["ALL"]
      runAsNonRoot: true
      runAsUser: 65534
      seccompProfile:
        type: RuntimeDefault
    command: ["sh", "-c", "echo '${PAYLOAD_B64}' | base64 -d | curl -s -X POST -H 'Content-Type: application/json' --data-binary @- ${ZEN_WATCHER_URL}/falco/webhook"]
  restartPolicy: Never
EOF
    if wait_pod_done "${POD_NAME}" "${NAMESPACE}" 20 >/dev/null 2>&1; then
        # consider it sent if pod succeeded; show response for debugging
        resp="$(kubectl logs "${POD_NAME}" -n "${NAMESPACE}" 2>/dev/null || true)"
        kubectl delete pod/${POD_NAME} -n ${NAMESPACE} >/dev/null 2>&1 || true
        echo -e "    ${GREEN}✓${NC} Sent"
        [[ -n "${resp}" ]] && echo "      Response: ${resp}" | head -c 2000
    else
        kubectl delete pod/${POD_NAME} -n ${NAMESPACE} >/dev/null 2>&1 || true
        echo -e "    ${RED}✗${NC} Failed"
    fi
    sleep 0.5
}

# Function to send Audit webhook
send_audit_webhook() {
    local verb=$1
    local resource=$2
    local name=$3
    local namespace=${4:-"default"}
    local event_type=${5:-"audit-event"}
    local api_group=${6:-""}
    local privileged=${7:-"false"}
    
    local payload=$(cat <<EOF
{
  "auditID": "$(uuidgen 2>/dev/null || cat /proc/sys/kernel/random/uuid 2>/dev/null || echo $(date +%s))",
  "stage": "ResponseComplete",
  "verb": "${verb}",
  "user": {
    "username": "system:serviceaccount:${namespace}:demo-sa",
    "uid": "system:serviceaccount:${namespace}:demo-sa"
  },
  "objectRef": {
    "resource": "${resource}",
    "namespace": "${namespace}",
    "name": "${name}",
    "apiVersion": "v1",
    "apiGroup": "${api_group}"
  },
  "responseStatus": {
    "code": 201
  },
  "requestObject": {
    "metadata": {
      "name": "${name}",
      "namespace": "${namespace}"
    },
    "spec": {
      "containers": [
        {
          "name": "test-container",
          "image": "nginx:latest",
          "securityContext": {
            "privileged": ${privileged}
          }
        }
      ]
    }
  }
}
EOF
)
    # Compact to a single line so YAML/JSON embedding is safe
    payload="$(echo "${payload}" | tr -d '\n' | sed 's/[[:space:]]\\+/ /g')"
    
    echo -e "${CYAN}  →${NC} Sending Audit webhook: ${verb} ${resource}/${name}"
    # Send webhook from within cluster using a PodSecurity-compliant pod
    POD_NAME="send-audit-webhook-$(unique_suffix)"
    # Encode payload to avoid YAML quoting issues in the pod spec
    PAYLOAD_B64="$(printf '%s' "${payload}" | base64 | tr -d '\n')"
    # NOTE: don't suppress errors here; failures are important for debugging in demos
    cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: ${POD_NAME}
  namespace: ${NAMESPACE}
spec:
  securityContext:
    runAsNonRoot: true
    runAsUser: 65534
    seccompProfile:
      type: RuntimeDefault
  containers:
  - name: curl
    image: curlimages/curl:latest
    securityContext:
      allowPrivilegeEscalation: false
      capabilities:
        drop: ["ALL"]
      runAsNonRoot: true
      runAsUser: 65534
      seccompProfile:
        type: RuntimeDefault
    command: ["sh", "-c", "echo '${PAYLOAD_B64}' | base64 -d | curl -s -X POST -H 'Content-Type: application/json' --data-binary @- ${ZEN_WATCHER_URL}/audit/webhook"]
  restartPolicy: Never
EOF
    if wait_pod_done "${POD_NAME}" "${NAMESPACE}" 20 >/dev/null 2>&1; then
        resp="$(kubectl logs "${POD_NAME}" -n "${NAMESPACE}" 2>/dev/null || true)"
        kubectl delete pod/${POD_NAME} -n ${NAMESPACE} >/dev/null 2>&1 || true
        echo -e "    ${GREEN}✓${NC} Sent"
        [[ -n "${resp}" ]] && echo "      Response: ${resp}" | head -c 2000
    else
        kubectl delete pod/${POD_NAME} -n ${NAMESPACE} >/dev/null 2>&1 || true
        echo -e "    ${RED}✗${NC} Failed"
    fi
    sleep 0.5
}

# Function to create ConfigMap for Checkov
create_checkov_configmap() {
    local name=$1
    local check_id=$2
    local check_name=$3
    local severity=$4
    local guideline=$5
    local resource=${6:-"Pod.default.demo-pod"}
    local namespace=${7:-"checkov"}
    
    kubectl create namespace ${namespace} 2>/dev/null || true
    
    cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: ${name}
  namespace: ${namespace}
  labels:
    app: checkov
    demo.zen.kube-zen.io/observation: "true"
data:
  results.json: |
    {
      "results": {
        "failed_checks": [
          {
            "check_id": "${check_id}",
            "check_name": "${check_name}",
            "resource": "${resource}",
            "guideline": "${guideline}",
            "severity": "${severity}"
          }
        ]
      }
    }
EOF
    echo -e "    ${GREEN}✓${NC} Created Checkov ConfigMap: ${name}"
}

# Function to create ConfigMap for kube-bench
create_kubebench_configmap() {
    local name=$1
    local test_number=$2
    local test_desc=$3
    local remediation=$4
    local section=${5:-"1"}
    local namespace=${6:-"kube-bench"}
    
    kubectl create namespace ${namespace} 2>/dev/null || true
    
    cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: ${name}
  namespace: ${namespace}
  labels:
    app: kube-bench
    demo.zen.kube-zen.io/observation: "true"
data:
  results.json: |
    {
      "Controls": [
        {
          "id": "1",
          "version": "CIS",
          "description": "Master Node Security Configuration",
          "tests": [
            {
              "section": "${section}",
              "type": "master",
              "results": [
                {
                  "test_number": "${test_number}",
                  "test_desc": "${test_desc}",
                  "remediation": "${remediation}",
                  "status": "FAIL",
                  "scored": true
                }
              ]
            }
          ]
        }
      ]
    }
EOF
    echo -e "    ${GREEN}✓${NC} Created kube-bench ConfigMap: ${name}"
}

echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  1. Sending Falco Webhooks${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

# Send Falco webhooks
# Create demo-manifests namespace if it doesn't exist (for Falco webhooks)
kubectl create namespace demo-manifests 2>/dev/null || true

# Generate unique pod names and timestamps to avoid deduplication
TIMESTAMP=$(date +%s)
send_falco_webhook "Critical" "Privileged container started" "Container running in privileged mode detected - ${TIMESTAMP}" "demo-insecure-pod-${TIMESTAMP}" "demo-manifests"
send_falco_webhook "Critical" "Write below binary dir" "File below a known binary directory opened for writing - ${TIMESTAMP}" "demo-pod-${TIMESTAMP}" "default"
send_falco_webhook "Error" "Sensitive file accessed" "File below /etc opened for reading - ${TIMESTAMP}" "demo-pod-${TIMESTAMP}" "default"
send_falco_webhook "Warning" "Unexpected network connection" "Connection to external IP 8.8.8.8 detected" "demo-pod" "default"
send_falco_webhook "Alert" "Shell spawned in container" "A shell was spawned in a container" "demo-pod" "default"

echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  2. Sending Audit Webhooks${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

# Send Audit webhooks
send_audit_webhook "delete" "pods" "demo-pod" "default" "resource-deletion" "" "false"
send_audit_webhook "create" "secrets" "demo-secret" "default" "secret-access" "" "false"
send_audit_webhook "update" "configmaps" "demo-config" "default" "secret-access" "" "false"
send_audit_webhook "create" "clusterrolebindings" "demo-binding" "" "rbac-change" "rbac.authorization.k8s.io" "false"
send_audit_webhook "create" "pods" "privileged-pod" "default" "privileged-pod-creation" "" "true"

echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  3. Creating Checkov ConfigMaps${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

# Create Checkov ConfigMaps
# Create demo-manifests namespace if it doesn't exist
kubectl create namespace demo-manifests 2>/dev/null || true

create_checkov_configmap "checkov-pod-security-1" "CKV_K8S_24" "Pod Security Policy" "high" "Ensure that the Pod Security Policy is set" "Pod.demo-manifests.demo-insecure-pod" "checkov"
create_checkov_configmap "checkov-resource-limits-1" "CKV_K8S_12" "Resource Limits" "medium" "CPU limits should be set" "Pod.demo-manifests.demo-no-security-context" "checkov"
create_checkov_configmap "checkov-secret-mount-1" "CKV_K8S_14" "Service Account Token" "high" "Ensure that the Service Account token is not mounted" "ServiceAccount.demo-manifests.demo-excessive-permissions" "checkov"
create_checkov_configmap "checkov-network-policy-1" "CKV_K8S_5" "Default Namespace" "medium" "Ensure that default namespace is not used" "Namespace.default.default" "checkov"

echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  4. Creating kube-bench ConfigMaps${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

# Create kube-bench ConfigMaps
create_kubebench_configmap "kubebench-anonymous-auth" "1.2.1" "Ensure that the --anonymous-auth argument is set to false" "Edit the API server pod specification file and set the --anonymous-auth parameter to false" "1" "kube-bench"
create_kubebench_configmap "kubebench-basic-auth" "1.2.2" "Ensure that the --basic-auth-file argument is not set" "Edit the API server pod specification file and remove the --basic-auth-file parameter" "1" "kube-bench"
create_kubebench_configmap "kubebench-token-auth" "1.2.3" "Ensure that the --token-auth-file parameter is not set" "Edit the API server pod specification file and remove the --token-auth-file parameter" "1" "kube-bench"
create_kubebench_configmap "kubebench-kubelet-auth" "4.2.1" "Ensure that the --anonymous-auth argument is set to false" "Edit the kubelet service file and set the --anonymous-auth parameter to false" "4" "kube-bench"

echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}✅ Mock data sent!${NC}"
echo ""
echo -e "${CYAN}📊 Check observations:${NC}"
echo -e "   kubectl get observations -n ${NAMESPACE} --sort-by=.metadata.creationTimestamp"
echo ""
echo -e "${CYAN}📈 Check metrics:${NC}"
echo -e "   curl -s 'http://localhost:8080/victoriametrics/api/v1/query?query=zen_watcher_events_total' | jq '.data.result[] | {source: .metric.source, severity: .metric.severity, value: .value[1]}'"
echo ""
echo -e "${CYAN}🔍 View logs:${NC}"
echo -e "   kubectl logs -n ${NAMESPACE} -l app.kubernetes.io/name=zen-watcher --tail=50 | grep -E 'Created.*Observation|Falco|Audit'"
echo ""

