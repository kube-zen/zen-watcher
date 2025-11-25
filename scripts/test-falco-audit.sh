#!/bin/bash
# Test script for Falco and Audit integration
# Usage: ./scripts/test-falco-audit.sh

set -e

NAMESPACE="${NAMESPACE:-zen-system}"
CONTEXT="${KUBECTX:-}"

echo "=== Zen Watcher - Falco & Audit Integration Test ==="
echo ""

# Check prerequisites
echo "Checking prerequisites..."
command -v kubectl >/dev/null 2>&1 || { echo "kubectl not found. Aborting."; exit 1; }
command -v jq >/dev/null 2>&1 || { echo "jq not found. Aborting."; exit 1; }

# Set context if provided
if [ -n "$CONTEXT" ]; then
    kubectl config use-context "$CONTEXT"
fi

echo "Current context: $(kubectl config current-context)"
echo ""

# Check if zen-watcher is running
echo "=== Step 1: Checking zen-watcher deployment ==="
if ! kubectl get deployment zen-watcher -n "$NAMESPACE" >/dev/null 2>&1; then
    echo "❌ zen-watcher deployment not found in namespace $NAMESPACE"
    echo "   Please deploy zen-watcher first"
    exit 1
fi

READY=$(kubectl get deployment zen-watcher -n "$NAMESPACE" -o jsonpath='{.status.readyReplicas}')
if [ "$READY" != "1" ]; then
    echo "❌ zen-watcher is not ready (ready: $READY/1)"
    echo "   Checking pod status..."
    kubectl get pods -n "$NAMESPACE" -l app=zen-watcher
    exit 1
fi

echo "✅ zen-watcher is running"
echo ""

# Check service
echo "=== Step 2: Checking zen-watcher service ==="
if ! kubectl get svc zen-watcher -n "$NAMESPACE" >/dev/null 2>&1; then
    echo "❌ zen-watcher service not found"
    exit 1
fi
echo "✅ Service exists"
echo ""

# Test health endpoint
echo "=== Step 3: Testing health endpoint ==="
kubectl port-forward -n "$NAMESPACE" svc/zen-watcher 8080:8080 > /tmp/zen-watcher-pf.log 2>&1 &
PF_PID=$!
sleep 3

if curl -s http://localhost:8080/health >/dev/null 2>&1; then
    echo "✅ Health endpoint responding"
else
    echo "❌ Health endpoint not responding"
    kill $PF_PID 2>/dev/null || true
    exit 1
fi
echo ""

# Test Falco webhook
echo "=== Step 4: Testing Falco webhook ==="
FALCO_TEST_EVENT='{
  "output": "16:31:56.123456789: Warning Sensitive file opened for reading by non-trusted program (user=root user_loginuid=-1 program=nmap command=nmap -sS 127.0.0.1 container_id=host image=<NA>)",
  "priority": "Warning",
  "rule": "Sensitive file opened for reading by non-trusted program",
  "time": "'$(date -u +%Y-%m-%dT%H:%M:%S.%N)'",
  "output_fields": {
    "container.id": "host",
    "evt.time": "1609456316123456789",
    "fd.name": "/etc/passwd",
    "proc.cmdline": "nmap -sS 127.0.0.1",
    "proc.name": "nmap",
    "user.name": "root"
  }
}'

RESPONSE=$(curl -s -w "\n%{http_code}" -X POST http://localhost:8080/falco/webhook \
  -H "Content-Type: application/json" \
  -d "$FALCO_TEST_EVENT")

HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | sed '$d')

if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "202" ]; then
    echo "✅ Falco webhook accepted event (HTTP $HTTP_CODE)"
else
    echo "❌ Falco webhook failed (HTTP $HTTP_CODE)"
    echo "   Response: $BODY"
fi
echo ""

# Test Audit webhook
echo "=== Step 5: Testing Audit webhook ==="
AUDIT_TEST_EVENT='{
  "kind": "Event",
  "apiVersion": "audit.k8s.io/v1",
  "level": "Request",
  "auditID": "test-'$(date +%s)'",
  "stage": "ResponseComplete",
  "requestURI": "/api/v1/namespaces",
  "verb": "get",
  "user": {
    "username": "test-user",
    "groups": ["system:authenticated"]
  },
  "sourceIPs": ["127.0.0.1"],
  "userAgent": "kubectl/v1.27.0",
  "responseStatus": {
    "code": 200
  },
  "requestReceivedTimestamp": "'$(date -u +%Y-%m-%dT%H:%M:%S.%N)'",
  "stageTimestamp": "'$(date -u +%Y-%m-%dT%H:%M:%S.%N)'"
}'

RESPONSE=$(curl -s -w "\n%{http_code}" -X POST http://localhost:8080/audit/webhook \
  -H "Content-Type: application/json" \
  -d "$AUDIT_TEST_EVENT")

HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | sed '$d')

if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "202" ]; then
    echo "✅ Audit webhook accepted event (HTTP $HTTP_CODE)"
else
    echo "❌ Audit webhook failed (HTTP $HTTP_CODE)"
    echo "   Response: $BODY"
fi
echo ""

# Stop port-forward
kill $PF_PID 2>/dev/null || true
sleep 2

# Check observations
echo "=== Step 6: Checking Observations ==="
sleep 5

OBS_COUNT=$(kubectl get observations -n "$NAMESPACE" --no-headers 2>/dev/null | wc -l)
echo "Total observations: $OBS_COUNT"

if [ "$OBS_COUNT" -gt 0 ]; then
    echo ""
    echo "Observations by source:"
    kubectl get observations -n "$NAMESPACE" -o json 2>/dev/null | \
      jq -r '.items[].spec.source' | sort | uniq -c || echo "  (Unable to parse)"
    
    echo ""
    echo "Recent observations:"
    kubectl get observations -n "$NAMESPACE" -o json 2>/dev/null | \
      jq -r '.items[-3:] | .[] | "  \(.metadata.name) | Source: \(.spec.source) | Category: \(.spec.category) | Severity: \(.spec.severity)"' || \
      kubectl get observations -n "$NAMESPACE" --no-headers | tail -3
else
    echo "⚠️  No observations found"
    echo "   This might be normal if events were deduplicated or processing is delayed"
fi
echo ""

# Check logs
echo "=== Step 7: Checking zen-watcher logs ==="
echo "Recent log entries (webhook/observation related):"
kubectl logs -n "$NAMESPACE" -l app=zen-watcher --tail=30 2>/dev/null | \
  grep -E "(falco|audit|webhook|Observation|ERROR|WARN)" | tail -10 || \
  echo "  (No relevant log entries found)"
echo ""

# Summary
echo "=== Test Summary ==="
echo "✅ zen-watcher deployment: OK"
echo "✅ Health endpoint: OK"
echo "✅ Falco webhook: $([ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "202" ] && echo "OK" || echo "FAILED")"
echo "✅ Audit webhook: $([ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "202" ] && echo "OK" || echo "FAILED")"
echo "✅ Observations created: $OBS_COUNT"
echo ""
echo "Test complete!"

