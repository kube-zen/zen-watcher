#!/bin/bash
#
# Zen Watcher - Mock Data Generator
#
# Deploys a mock zen-watcher that serves Prometheus metrics
#

set -e

NAMESPACE="${1:-zen-system}"

echo "→ Deploying mock zen-watcher with metrics..."

# Deploy a simple HTTP server serving metrics
kubectl apply -f - <<'EOF'
apiVersion: v1
kind: Pod
metadata:
  name: zen-watcher-mock
  namespace: zen-system
  labels:
    app: zen-watcher
    app.kubernetes.io/name: zen-watcher
spec:
  containers:
  - name: metrics
    image: python:3.11-alpine
    command: ["/bin/sh", "-c"]
    args:
      - |
        python3 << 'PYEOF'
        from http.server import HTTPServer, BaseHTTPRequestHandler
        
        METRICS = """# HELP zen_watcher_health_status Health status
# TYPE zen_watcher_health_status gauge
zen_watcher_health_status 1

# HELP zen_watcher_readiness_status Readiness
# TYPE zen_watcher_readiness_status gauge  
zen_watcher_readiness_status 1

# HELP zen_watcher_events_total Total events
# TYPE zen_watcher_events_total counter
zen_watcher_events_total{category="security",source="trivy",event_type="vulnerability",severity="CRITICAL"} 5
zen_watcher_events_total{category="security",source="trivy",event_type="vulnerability",severity="HIGH"} 23
zen_watcher_events_total{category="security",source="falco",event_type="runtime-threat",severity="CRITICAL"} 3
zen_watcher_events_total{category="security",source="falco",event_type="runtime-threat",severity="HIGH"} 8
zen_watcher_events_total{category="security",source="kyverno",event_type="policy-violation",severity="MEDIUM"} 12
zen_watcher_events_total{category="compliance",source="audit",event_type="audit-event",severity="INFO"} 67

# HELP zen_watcher_watchers_active Active watchers
# TYPE zen_watcher_watchers_active gauge
zen_watcher_watchers_active{tool="trivy"} 1
zen_watcher_watchers_active{tool="falco"} 1
zen_watcher_watchers_active{tool="kyverno"} 1
zen_watcher_watchers_active{tool="audit"} 1
zen_watcher_watchers_active{tool="kube-bench"} 1

# HELP go_goroutines Number of goroutines
# TYPE go_goroutines gauge
go_goroutines 18

# HELP process_resident_memory_bytes Memory
# TYPE process_resident_memory_bytes gauge
process_resident_memory_bytes 45678912
"""
        
        class Handler(BaseHTTPRequestHandler):
            def do_GET(self):
                self.send_response(200)
                self.send_header("Content-Type", "text/plain; version=0.0.4")
                self.end_headers()
                self.wfile.write(METRICS.encode())
            def log_message(self, *args): pass
        
        print("Metrics server starting on :8080")
        HTTPServer(("0.0.0.0", 8080), Handler).serve_forever()
        PYEOF
    ports:
    - containerPort: 8080
      name: metrics
---
apiVersion: v1
kind: Service
metadata:
  name: zen-watcher
  namespace: zen-system
spec:
  ports:
  - port: 8080
    targetPort: 8080
  selector:
    app: zen-watcher
EOF

echo "✓ Mock deployed"
echo "→ Waiting for pod..."
kubectl wait --for=condition=ready pod/zen-watcher-mock -n ${NAMESPACE} --timeout=60s
echo "✓ Mock ready and serving metrics on :8080"
