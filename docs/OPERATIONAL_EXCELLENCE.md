# Operational Excellence Guide

## Overview

This guide covers operational best practices for running Zen Watcher in production.

## Health Checks ✅

### Liveness Probe

**Purpose**: Detect if the application is running

**Configuration**:
```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 30
  periodSeconds: 30
  timeoutSeconds: 5
  successThreshold: 1
  failureThreshold: 3
```

**Endpoint**: `GET /health`

**Response**:
```json
{
  "status": "healthy",
  "service": "zen-watcher",
  "version": "1.0.0",
  "mode": "independent",
  "timestamp": "2024-11-04T10:00:00Z"
}
```

**Health Criteria**:
- ✅ HTTP server responsive
- ✅ Application initialized
- ✅ No critical errors

### Readiness Probe

**Purpose**: Detect if the application is ready to serve traffic

**Configuration**:
```yaml
readinessProbe:
  httpGet:
    path: /ready
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 10
  timeoutSeconds: 5
  successThreshold: 1
  failureThreshold: 3
```

**Endpoint**: `GET /ready`

**Response**:
```json
{
  "status": "ready"
}
```

**Readiness Criteria**:
- ✅ Watchers initialized and started
- ✅ Kubernetes client connected
- ✅ CRD writer initialized
- ✅ HTTP server ready

### Startup Probe

For slow-starting applications (optional):

```yaml
startupProbe:
  httpGet:
    path: /ready
    port: 8080
  initialDelaySeconds: 0
  periodSeconds: 5
  timeoutSeconds: 3
  successThreshold: 1
  failureThreshold: 30  # 30 * 5 = 150 seconds max startup time
```

---

## Monitoring ✅

### Prometheus Metrics

**Metrics Endpoint**: `GET /metrics`

**Available Metrics**:

#### Event Metrics
- `zen_watcher_events_total` - Total events (counter)
- `zen_watcher_events_written_total` - Successfully written events (counter)
- `zen_watcher_events_failures_total` - Failed writes (counter)
- `zen_watcher_active_events` - Active events (gauge)

#### Watcher Metrics
- `zen_watcher_watcher_status` - Watcher enabled status (gauge)
- `zen_watcher_watcher_errors_total` - Watcher errors (counter)
- `zen_watcher_scrape_duration_seconds` - Scrape duration (histogram)

#### CRD Metrics
- `zen_watcher_crd_operations_total` - CRD operations (counter)
- `zen_watcher_crd_operation_duration_seconds` - Operation duration (histogram)

#### HTTP Metrics
- `zen_watcher_http_requests_total` - HTTP requests (counter)
- `zen_watcher_http_request_duration_seconds` - Request duration (histogram)

#### System Metrics
- `zen_watcher_health_status` - Health status (gauge)
- `zen_watcher_readiness_status` - Readiness status (gauge)
- `zen_watcher_goroutines` - Goroutine count (gauge)
- `zen_watcher_build_info` - Build information (gauge)

### ServiceMonitor

```bash
# Deploy ServiceMonitor
kubectl apply -f monitoring/prometheus-servicemonitor.yaml

# Verify
kubectl get servicemonitor -n zen-system zen-watcher
```

### Grafana Dashboard

```bash
# Import dashboard
kubectl apply -f dashboards/zen-watcher-dashboard.json

# Or import via Grafana UI
# Dashboard ID: zen-watcher
```

---

## Logging ✅

### Log Levels

Zen Watcher uses structured logging with emojis for visibility:

- 🎉 **Startup/Success**: Application lifecycle events
- ⚠️  **Warnings**: Configuration issues, fallbacks
- ❌ **Errors**: Operation failures
- 🔍 **Info**: Regular operations
- 📝 **Debug**: Detailed operation info

### Log Examples

```
🔍 Starting Zen Watcher - Independent Security & Compliance Event Aggregator...
✅ Initialized Zen CRD client
✅ Initialized CRD writer
✅ Initialized action handlers
🚀 Starting WatcherManager...
🔍 Starting Trivy watcher...
🔍 Starting Falco watcher...
✅ WatcherManager started successfully
🌐 Starting HTTP server on :8080
📝 [WATCHER] Writing events to CRDs...
✅ [WATCHER] Successfully wrote 5 events as CRDs
📝 Created security event CRD: zen-system/trivy-vulnerability-1234567890 (severity: CRITICAL)
```

### Log Collection

**With Loki**:
```bash
kubectl apply -f examples/loki-promtail-config.yaml
```

**Query Examples**:
```logql
# All logs
{namespace="zen-system", app="zen-watcher"}

# Errors only
{namespace="zen-system", app="zen-watcher"} |= "❌"

# Event writes
{namespace="zen-system", app="zen-watcher"} |= "Created"

# Critical events
{namespace="zen-system", app="zen-watcher"} |= "CRITICAL"
```

---

## Resource Management ✅

### Resource Requests and Limits

**Production Settings**:
```yaml
resources:
  requests:
    memory: "128Mi"
    cpu: "100m"
  limits:
    memory: "512Mi"  # Allow burst for large event processing
    cpu: "500m"      # Allow CPU burst
```

**Development Settings**:
```yaml
resources:
  requests:
    memory: "64Mi"
    cpu: "50m"
  limits:
    memory: "128Mi"
    cpu: "100m"
```

### Scaling Strategy

**⚠️ Important:** Zen Watcher uses a **single-replica deployment model** by default.

**Why?**
- Deduplication and filtering are in-memory per pod
- Multiple replicas would create duplicate Observations
- GC would run multiple times unnecessarily

**Recommended Deployment:**
```yaml
replicas: 1
resources:
  requests:
    memory: "128Mi"
    cpu: "100m"
  limits:
    memory: "512Mi"
    cpu: "500m"
```

**Scaling Options:**

1. **Vertical Scaling** (First choice):
   ```yaml
   resources:
     limits:
       memory: "1Gi"
       cpu: "1000m"
   ```

2. **Namespace Sharding** (For high volume):
   - Deploy multiple instances, each scoped to different namespaces
   - See [SCALING.md](SCALING.md) for details

3. **Leader Election** (Future):
   - Planned for v1.1.x+
   - Will enable HPA for webhook traffic

**✅ HA Support:** HPA is enabled by default. With HA optimization enabled, proper deduplication and load balancing are maintained across replicas.

See [docs/SCALING.md](SCALING.md) for complete scaling strategy.

---

## High Availability ✅

### Pod Disruption Budget

```yaml
podDisruptionBudget:
  enabled: true
  minAvailable: 1  # Maintains availability during disruptions (adjust for HA)
```

### Single Replica + Restart Policy

```yaml
replicas: 1
spec:
  restartPolicy: Always  # Kubernetes automatically restarts on failure
```

**Availability Strategy:**
- Kubernetes restart policies handle pod failures automatically
- PodDisruptionBudget prevents voluntary disruptions during upgrades
- No need for multiple replicas (which would create duplicates)

### Anti-Affinity

```yaml
affinity:
  podAntiAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
    - weight: 100
      podAffinityTerm:
        labelSelector:
          matchExpressions:
          - key: app.kubernetes.io/name
            operator: In
            values:
            - zen-watcher
        topologyKey: kubernetes.io/hostname
```

---

## Backup and Recovery ✅

### Backup Observation CRDs

```bash
# Export all events
kubectl get zenevents -n zen-system -o yaml > backup-zenevents-$(date +%Y%m%d).yaml

# Automated backup (cron)
0 0 * * * kubectl get zenevents -n zen-system -o yaml > /backups/zenevents-$(date +%Y%m%d).yaml
```

### Recovery

```bash
# Restore from backup
kubectl apply -f backup-zenevents-20241104.yaml

# Or selective restore
kubectl apply -f - <<EOF
$(grep -A 100 "kind: Observation" backup-zenevents-20241104.yaml | head -102)
EOF
```

### Disaster Recovery

```bash
# 1. Backup CRDs
kubectl get crd zenevents.zen.kube-zen.io -o yaml > backup-crd.yaml

# 2. Backup events
kubectl get zenevents --all-namespaces -o yaml > backup-events.yaml

# 3. Export configuration
helm get values zen-watcher -n zen-system > backup-values.yaml

# 4. Restore
kubectl apply -f backup-crd.yaml
helm install zen-watcher kube-zen/zen-watcher \
  --namespace zen-system \
  --create-namespace \
  -f backup-values.yaml
kubectl apply -f backup-events.yaml
```

---

## Performance Optimization ✅

### Tuning Watcher Intervals

```go
// In watcher_manager.go
// Adjust based on your needs:

// High-frequency (real-time) - More CPU, faster detection
ticker := time.NewTicker(15 * time.Second)

// Medium-frequency (balanced) - Default
ticker := time.NewTicker(30 * time.Second)

// Low-frequency (resource-efficient) - Less CPU, slower detection
ticker := time.NewTicker(60 * time.Second)
```

### Resource Optimization

```yaml
# For high-volume environments
resources:
  limits:
    memory: "1Gi"
    cpu: "1000m"
  requests:
    memory: "256Mi"
    cpu: "200m"

# For low-volume environments
resources:
  limits:
    memory: "128Mi"
    cpu: "100m"
  requests:
    memory: "64Mi"
    cpu: "50m"
```

---

## Security Operations ✅

### Security Scanning

**Pre-deployment**:
```bash
# Scan image
trivy image zubezen/zen-watcher:1.0.0

# Verify signature
cosign verify --key cosign.pub zubezen/zen-watcher:1.0.0

# Check SBOM
syft zubezen/zen-watcher:1.0.0 -o spdx-json | grype
```

**Post-deployment**:
```bash
# Runtime scanning with Falco
kubectl logs -n falco -l app=falco | grep zen-watcher

# Kubescape scan
kubescape scan workload deployment/zen-watcher -n zen-system

# Network policy verification
kubectl get networkpolicy -n zen-system zen-watcher -o yaml
```

### RBAC Audit

```bash
# Review permissions
kubectl describe clusterrole zen-watcher

# Check bindings
kubectl describe clusterrolebinding zen-watcher

# Audit access
kubectl auth can-i --list --as=system:serviceaccount:zen-system:zen-watcher
```

---

## Troubleshooting ✅

### Pod Not Starting

```bash
# Check pod status
kubectl get pods -n zen-system -l app=zen-watcher

# Describe pod
kubectl describe pod -n zen-system -l app=zen-watcher

# Check events
kubectl get events -n zen-system --sort-by='.lastTimestamp'

# View logs
kubectl logs -n zen-system -l app=zen-watcher --tail=100
```

### High Memory Usage

```bash
# Check current usage
kubectl top pod -n zen-system -l app=zen-watcher

# Analyze with pprof (if enabled)
kubectl port-forward -n zen-system svc/zen-watcher 6060:6060
go tool pprof http://localhost:6060/debug/pprof/heap

# Review metrics
curl http://localhost:8080/metrics | grep memory
```

### Events Not Being Created

```bash
# Check watcher status
curl http://localhost:8080/tools/status | jq

# Check CRD exists
kubectl get crd zenevents.zen.kube-zen.io

# Check RBAC permissions
kubectl auth can-i create zenevents --as=system:serviceaccount:zen-system:zen-watcher -n zen-system

# View watcher logs
kubectl logs -n zen-system -l app=zen-watcher | grep "Writing events"
```

### Network Issues

```bash
# Test DNS resolution
kubectl exec -it -n zen-system deployment/zen-watcher -- nslookup kubernetes.default

# Check NetworkPolicy
kubectl get networkpolicy -n zen-system -o yaml

# Test connectivity
kubectl run -it --rm debug --image=busybox --restart=Never -n zen-system -- \
  wget -qO- http://zen-watcher:8080/health
```

---

## Capacity Planning ✅

### Event Volume Estimation

```bash
# Estimate events per day
kubectl get zenevents -n zen-system -o json | \
  jq -r '.items[] | .spec.timestamp' | \
  wc -l
```

### Resource Sizing

| Events/Day | CPU Request | CPU Limit | Memory Request | Memory Limit | Replicas |
|------------|-------------|-----------|----------------|--------------|----------|
| < 1,000 | 50m | 100m | 64Mi | 128Mi | 1 |
| 1,000 - 10,000 | 100m | 200m | 128Mi | 256Mi | 2 |
| 10,000 - 100,000 | 200m | 500m | 256Mi | 512Mi | 3 |
| > 100,000 | 500m | 1000m | 512Mi | 1Gi | 5+ |

### Storage Requirements

```bash
# Estimate CRD storage
# Average Observation size: ~2-5KB
# 10,000 events = ~50MB in etcd

# Check etcd size
kubectl exec -it -n kube-system etcd-master -- \
  etcdctl --endpoints=https://127.0.0.1:2379 \
  --cert=/etc/kubernetes/pki/etcd/peer.crt \
  --key=/etc/kubernetes/pki/etcd/peer.key \
  --cacert=/etc/kubernetes/pki/etcd/ca.crt \
  endpoint status --write-out=table
```

---

## Maintenance ✅

### Regular Tasks

**Daily**:
- ✅ Review critical events: `kubectl get zenevents -l severity=critical`
- ✅ Check metrics dashboard
- ✅ Review error logs

**Weekly**:
- ✅ Review all active events
- ✅ Archive/cleanup old events
- ✅ Check resource usage trends
- ✅ Review alerts

**Monthly**:
- ✅ Update to latest version
- ✅ Vulnerability scanning
- ✅ RBAC audit
- ✅ Capacity planning review

### Event Cleanup

**Manual Cleanup**:
```bash
# Delete resolved events older than 7 days
kubectl get zenevents -n zen-system -o json | \
  jq -r '.items[] | select(.status.phase=="Resolved") | select(.spec.timestamp < (now - 604800 | todate)) | .metadata.name' | \
  xargs -I {} kubectl delete zenevent {} -n zen-system
```

**Automated Cleanup with CronJob**:
```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: zen-event-cleanup
  namespace: zen-system
spec:
  schedule: "0 2 * * *"  # 2 AM daily
  jobTemplate:
    spec:
      template:
        spec:
          serviceAccountName: zen-watcher
          containers:
          - name: cleanup
            image: bitnami/kubectl:latest
            command:
            - /bin/sh
            - -c
            - |
              # Delete resolved events older than 7 days
              kubectl get zenevents -n zen-system -o name | \
              while read event; do
                phase=$(kubectl get $event -n zen-system -o jsonpath='{.status.phase}')
                timestamp=$(kubectl get $event -n zen-system -o jsonpath='{.spec.timestamp}')
                if [ "$phase" = "Resolved" ]; then
                  # Check if older than 7 days (implement proper date comparison)
                  kubectl delete $event -n zen-system
                fi
              done
          restartPolicy: OnFailure
```

---

## Updates and Upgrades ✅

### Rolling Updates

```bash
# Update image
helm upgrade zen-watcher kube-zen/zen-watcher \
  --namespace zen-system \
  --reuse-values \
  --set image.tag=1.1.0 \
  --wait

# Verify rollout
kubectl rollout status deployment/zen-watcher -n zen-system

# Rollback if needed
helm rollback zen-watcher -n zen-system
```

### Zero-Downtime Updates

```yaml
# Configure rolling update strategy
strategy:
  type: RollingUpdate
  rollingUpdate:
    maxSurge: 1
    maxUnavailable: 0
```

---

## Disaster Recovery ✅

### Backup Strategy

**What to Backup**:
1. ✅ CRD definitions
2. ✅ Observation resources
3. ✅ Helm values
4. ✅ Configuration

**Backup Script**:
```bash
#!/bin/bash
BACKUP_DIR="/backups/zen-watcher/$(date +%Y%m%d)"
mkdir -p $BACKUP_DIR

# Backup CRDs
kubectl get crd zenevents.zen.kube-zen.io -o yaml > $BACKUP_DIR/crd.yaml

# Backup events
kubectl get zenevents --all-namespaces -o yaml > $BACKUP_DIR/events.yaml

# Backup Helm release
helm get values zen-watcher -n zen-system > $BACKUP_DIR/values.yaml
helm get manifest zen-watcher -n zen-system > $BACKUP_DIR/manifest.yaml

# Compress
tar -czf $BACKUP_DIR.tar.gz $BACKUP_DIR
```

### Recovery Procedure

```bash
# 1. Restore CRDs
kubectl apply -f backup/crd.yaml

# 2. Reinstall application
helm install zen-watcher kube-zen/zen-watcher \
  --namespace zen-system \
  --create-namespace \
  -f backup/values.yaml

# 3. Wait for ready
kubectl wait --for=condition=ready pod -l app=zen-watcher -n zen-system --timeout=300s

# 4. Restore events
kubectl apply -f backup/events.yaml

# 5. Verify
kubectl get zenevents -n zen-system
```

---

## Service Level Objectives (SLOs) ✅

### Availability SLO: 99.9%

**Target**: 99.9% uptime (43.2 minutes downtime/month)

**Measurement**:
```promql
avg_over_time(zen_watcher_health_status[30d]) * 100
```

### Event Processing SLO: 99% < 5s

**Target**: 99% of events processed within 5 seconds

**Measurement**:
```promql
histogram_quantile(0.99, rate(zen_watcher_event_processing_duration_seconds_bucket[5m]))
```

### API Latency SLO: 95% < 100ms

**Target**: 95% of API requests complete within 100ms

**Measurement**:
```promql
histogram_quantile(0.95, rate(zen_watcher_http_request_duration_seconds_bucket[5m]))
```

---

## CI/CD Integration ✅

### Deployment Pipeline

Invoke the CI entry point script from your CI system or scheduled job:

```bash
# CI-friendly entry point (invoked by CI / scheduled job outside GitHub Actions)
./scripts/ci/zen-demo-validate.sh

# Or use Make targets
make zen-demo-validate

# Example deployment steps (adapt to your CI system):
# 1. Scan image
trivy image zubezen/zen-watcher:${IMAGE_TAG}

# 2. Verify signature
cosign verify --key cosign.pub zubezen/zen-watcher:${IMAGE_TAG}

# 3. Deploy
helm upgrade --install zen-watcher kube-zen/zen-watcher \
  --namespace zen-system \
  --create-namespace \
  --set image.tag=${IMAGE_TAG} \
  --wait \
  --timeout=5m

# 4. Verify deployment
kubectl rollout status deployment/zen-watcher -n zen-system

# 5. Smoke test
kubectl exec -n zen-system deployment/zen-watcher -- \
  curl -f http://localhost:8080/health
```

---

## Operational Runbooks

### High Event Rate

**Symptoms**: `rate(zen_watcher_events_total) > 100`

**Investigation**:
1. Check which source is generating events
2. Review if it's a real security issue or false positives
3. Check if upstream tool misconfigured

**Response Actions**:
- Adjust watcher frequency
- Scale up resources
- Tune upstream tool sensitivity

### Pod Crash Loop

**Symptoms**: Pod continuously restarting

**Investigation**:
```bash
kubectl logs -n zen-system -l app=zen-watcher --previous
kubectl describe pod -n zen-system -l app=zen-watcher
```

**Common Causes**:
- RBAC permissions missing
- CRD not installed
- Out of memory
- Configuration error

### High Memory Usage

**Investigation**:
```bash
# Check metrics
kubectl port-forward -n zen-system svc/zen-watcher 8080:8080
curl http://localhost:8080/metrics | grep memory

# Check events count
kubectl get zenevents -n zen-system --no-headers | wc -l
```

**Response Actions**:
- Increase memory limits
- Implement event cleanup
- Reduce watcher frequency
- Scale horizontally

---

## Best Practices Checklist ✅

### Before Production

- [ ] Health probes configured
- [ ] Readiness probe configured
- [ ] Resource limits set
- [ ] NetworkPolicy enabled
- [ ] Pod Security Standards enforced
- [ ] RBAC reviewed and minimized
- [ ] Monitoring enabled (ServiceMonitor)
- [ ] Alerts configured
- [ ] Logging centralized
- [ ] Backup strategy in place
- [ ] Image signed and verified
- [ ] SBOM generated
- [ ] Vulnerability scan passed
- [ ] Load testing completed
- [ ] Disaster recovery tested
- [ ] Runbooks created
- [ ] Documentation updated

### In Production

- [ ] Monitor metrics dashboard daily
- [ ] Review critical events immediately
- [ ] Check error logs regularly
- [ ] Update regularly (monthly)
- [ ] Backup events (daily/weekly)
- [ ] Test disaster recovery (quarterly)
- [ ] Review and tune alerts
- [ ] Capacity planning (monthly)
- [ ] Security audit (quarterly)
- [ ] Performance review (monthly)

---

## Metrics Dashboard Quick Start

```bash
# 1. Enable ServiceMonitor
helm upgrade zen-watcher kube-zen/zen-watcher \
  --namespace zen-system \
  --reuse-values \
  --set serviceMonitor.enabled=true

# 2. Import Grafana dashboard
# Go to Grafana UI → Import → Upload dashboards/zen-watcher-dashboard.json

# 3. Configure alerts
kubectl apply -f monitoring/prometheus-alerts.yaml

# 4. Verify
curl http://localhost:8080/metrics
```

---

## Contact

For operational issues:
- GitHub Issues: https://github.com/your-org/zen-watcher/issues
- Email: ops@kube-zen.com

For security incidents:
- Email: security@kube-zen.com
- See docs/SECURITY.md


