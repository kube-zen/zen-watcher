# Zen Watcher - Deployment Scenarios

This guide covers different deployment scenarios for Zen Watcher, from quick demos to production deployments.

---

## Quick Demo

**Use cases**: Evaluation, testing, local development

```bash
./scripts/quick-demo.sh
```

**What it does**:
- Creates a local k3d cluster
- Installs Trivy, Falco, Kyverno
- Installs VictoriaMetrics & Grafana
- Configures datasources
- Imports dashboard
- Sets up port-forwards
- Provides all URLs

**Time**: ~5 minutes

**Requirements**: kubectl, helm, k3d

**Access**:
- Grafana: http://localhost:3000 (admin/admin)
- Dashboard: http://localhost:3000/d/zen-watcher
- VictoriaMetrics: http://localhost:8428/vmui

---

## 💼 Production Scenarios

### Scenario 1: Fresh Production Cluster

**Situation**: New Kubernetes cluster, no existing monitoring or security tools

```bash
# For production, use Helm charts or kubectl apply directly
# For demo/testing, use: ./scripts/quick-demo.sh
```

**Installs**:
- ✅ VictoriaMetrics (metrics storage)
- ✅ Grafana (dashboards)
- ✅ Trivy Operator (vulnerability scanning)
- ✅ Falco (runtime security)
- ✅ Kyverno (policy management)
- ✅ Zen Watcher (event aggregation)

**Result**: Complete security & compliance monitoring stack

---

### Scenario 2: Existing Prometheus & Grafana

**Situation**: You already have Prometheus and Grafana deployed

```bash
# For demo/testing, use: ./scripts/quick-demo.sh
# For production, use Helm charts or kubectl apply directly
# Example:
  --use-prometheus http://prometheus.monitoring.svc:9090 \
  --use-grafana http://grafana.monitoring.svc:3000 \
  --namespace zen-system
```

**Installs**:
- ✅ Security tools (Trivy, Falco, Kyverno)
- ✅ Zen Watcher
- ✅ Configures to use YOUR Prometheus
- ✅ Adds dashboard to YOUR Grafana

**Result**: Zen Watcher integrates with your existing monitoring

---

### Scenario 3: Already Have Security Tools

**Situation**: Trivy, Falco, Kyverno already deployed

```bash
# For demo/testing, use: ./scripts/quick-demo.sh
# For production, use Helm charts or kubectl apply directly
# Example:
  --skip-tools \
  --namespace zen-system
```

**Installs**:
- ✅ VictoriaMetrics (if no Prometheus)
- ✅ Grafana (if none exists)
- ✅ Zen Watcher only

**Result**: Zen Watcher uses your existing security tools

---

### Scenario 4: Everything Already Exists

**Situation**: Full monitoring & security stack already deployed

```bash
# For demo/testing, use: ./scripts/quick-demo.sh
# For production, use Helm charts or kubectl apply directly
# Example:
  --skip-tools \
  --skip-monitoring \
  --use-prometheus http://prometheus.monitoring.svc:9090 \
  --namespace zen-system
```

**Installs**:
- ✅ Zen Watcher only
- ✅ Configured to use ALL existing infrastructure

**Result**: Minimal footprint, maximum integration

---

### Scenario 5: Helm-Only Production

**Situation**: Production deployment with custom values

```bash
helm install zen-watcher kube-zen/zen-watcher \
  --namespace zen-system \
  --create-namespace \
  --values production-values.yaml
```

**Example `production-values.yaml`**:

```yaml
global:
  
image:
  repository: your-registry.io/zen-watcher
  tag: 1.2.2
  pullPolicy: Always

replicas: 3

resources:
  limits:
    cpu: 1000m
    memory: 1Gi
  requests:
    cpu: 500m
    memory: 512Mi

monitoring:
  prometheus:
    enabled: true
    url: http://prometheus.monitoring.svc:9090
  grafana:
    enabled: true
    url: http://grafana.monitoring.svc:3000

tools:
  trivy:
    enabled: true
    namespace: trivy-system
  falco:
    enabled: true
    namespace: falco
  kyverno:
    enabled: true
    namespace: kyverno
  audit:
    enabled: true
  kubeBench:
    enabled: true

networkPolicy:
  enabled: true
  
podSecurityStandards:
  enabled: true
  enforce: restricted

serviceMonitor:
  enabled: true

podDisruptionBudget:
  enabled: true
  minAvailable: 2

autoscaling:
  enabled: true
  minReplicas: 3
  maxReplicas: 10
  targetCPUUtilizationPercentage: 70
```

---

## 🔧 Platform-Specific Guides

### k3d (Local Development)

```bash
# Create cluster
k3d cluster create zen-dev --agents 2

# Install
./scripts/quick-demo.sh
```

**Or manually**:
```bash
helm install zen-watcher kube-zen/zen-watcher \
  --namespace zen-system \
  --create-namespace
```

---

### k3s (Edge/IoT)

```bash
# k3s already installed
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml

# Install
# For demo/testing: ./scripts/quick-demo.sh
# For production: Use Helm charts or kubectl apply
```

**Optimized for k3s**:
```bash
helm install zen-watcher kube-zen/zen-watcher \
  --namespace zen-system \
  --set resources.limits.memory=256Mi \
  --set resources.requests.memory=128Mi \
  --set replicas=1
```

---

### kind (Testing/CI)

```bash
# Create cluster
kind create cluster --name zen-test

# Install
./scripts/quick-demo.sh
```

**For CI**:
```bash
# Minimal installation for testing
helm install zen-watcher kube-zen/zen-watcher \
  --namespace zen-system \
  --create-namespace \
  --wait --timeout=5m
```

---

### minikube (Local Development)

```bash
# Start minikube
minikube start --cpus 4 --memory 8192

# Install
./scripts/quick-demo.sh
```

**Access services**:
```bash
# Grafana
minikube service grafana -n zen-system

# Zen Watcher
minikube service zen-watcher -n zen-system
```

---

### EKS (AWS)

```bash
# Cluster already exists
export KUBECONFIG=~/.kube/eks-config

# Install with production settings
helm install zen-watcher kube-zen/zen-watcher \
  --namespace zen-system \
  --create-namespace \
  --values eks-values.yaml \
```

**EKS-specific values** (`eks-values.yaml`):

```yaml
serviceAccount:
  annotations:
    eks.amazonaws.com/role-arn: arn:aws:iam::ACCOUNT:role/zen-watcher-role

persistence:
  enabled: true
  storageClass: gp3
  size: 20Gi

ingress:
  enabled: true
  className: alb
  annotations:
    alb.ingress.kubernetes.io/scheme: internal
    alb.ingress.kubernetes.io/target-type: ip
```

---

### GKE (Google Cloud)

```bash
# Cluster already exists
gcloud container clusters get-credentials prod-cluster --region us-central1

# Install
helm install zen-watcher kube-zen/zen-watcher \
  --namespace zen-system \
  --create-namespace \
  --values gke-values.yaml
```

**GKE-specific values** (`gke-values.yaml`):

```yaml
serviceAccount:
  annotations:
    iam.gke.io/gcp-service-account: zen-watcher@PROJECT.iam.gserviceaccount.com

workloadIdentity:
  enabled: true

persistence:
  enabled: true
  storageClass: standard-rwo
```

---

### AKS (Azure)

```bash
# Cluster already exists
az aks get-credentials --resource-group rg-prod --name aks-prod

# Install
helm install zen-watcher kube-zen/zen-watcher \
  --namespace zen-system \
  --create-namespace \
  --values aks-values.yaml
```

---

## 🔍 Dry Run Mode

Want to see what will be installed without making changes?

```bash
# For demo/testing: ./scripts/quick-demo.sh --non-interactive
# For production: Review Helm values and use kubectl apply --dry-run
```

**Output shows**:
- ✅ Detected existing infrastructure
- ✅ What will be installed
- ✅ What will be skipped
- ✅ Configuration details

---

## 🧪 Testing Your Deployment

### 1. Verify Installation

```bash
# Check all pods are running
kubectl get pods -n zen-system

# Check CRDs are installed
kubectl get crd zenevents.zen.kube-zen.io

# View Zen Events
kubectl get zenevents -A
```

### 2. Test Metrics

```bash
# Port-forward Zen Watcher
kubectl port-forward -n zen-system svc/zen-watcher 8080:8080

# Check metrics endpoint
curl http://localhost:8080/metrics

# Check health
curl http://localhost:8080/health
```

### 3. Test Dashboard

```bash
# Port-forward Grafana
kubectl port-forward -n zen-system svc/grafana 3000:3000

# Open browser
# http://localhost:3000/d/zen-watcher
```

### 4. Generate Test Events

```bash
# Create a test pod (will trigger Trivy scan)
kubectl run test-nginx --image=nginx:1.14

# Wait for vulnerability scan
kubectl get vulnerabilityreports

# Check if Zen Watcher captured it
kubectl get zenevents -A
```

---

## 📊 Monitoring Integration

### With Existing Prometheus

1. **Zen Watcher auto-detects Prometheus**
2. **Exposes metrics at `/metrics`**
3. **ServiceMonitor created automatically** (if Prometheus Operator exists)

**Manual scrape config**:
```yaml
scrape_configs:
  - job_name: 'zen-watcher'
    kubernetes_sd_configs:
      - role: pod
        namespaces:
          names:
            - zen-system
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_label_app_kubernetes_io_name]
        action: keep
        regex: zen-watcher
```

### With Existing Grafana

1. **Install detects existing Grafana**
2. **Adds VictoriaMetrics/Prometheus as datasource**
3. **Imports Zen Watcher dashboard**

**Manual import**:
```bash
# Copy dashboard JSON
cp config/dashboards/zen-watcher-dashboard.json /tmp/

# Import via Grafana UI
# Dashboards → Import → Upload JSON
```

### With Loki (Logs)

```bash
# Configure Promtail to ship logs
helm upgrade --install promtail grafana/promtail \
  --set config.lokiAddress=http://loki:3100/loki/api/v1/push \
  --set config.snippets.extraScrapeConfigs='
- job_name: zen-watcher
  kubernetes_sd_configs:
    - role: pod
      namespaces:
        names: [zen-system]
  relabel_configs:
    - source_labels: [__meta_kubernetes_pod_label_app_kubernetes_io_name]
      action: keep
      regex: zen-watcher
'
```

---

## 🔐 Security Best Practices

### 1. RBAC

Zen Watcher uses least-privilege RBAC:
```yaml
rules:
- apiGroups: ["zen.kube-zen.io"]
  resources: ["zenevents"]
  verbs: ["get", "list", "watch", "create", "update"]
- apiGroups: [""]
  resources: ["pods", "pods/log"]
  verbs: ["get", "list", "watch"]
```

### 2. Network Policies

```yaml
networkPolicy:
  enabled: true
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
      - namespaceSelector:
          matchLabels:
            name: monitoring
      ports:
      - protocol: TCP
        port: 8080
```

### 3. Pod Security Standards

```yaml
podSecurityContext:
  runAsNonRoot: true
  runAsUser: 1000
  fsGroup: 1000
  seccompProfile:
    type: RuntimeDefault

securityContext:
  allowPrivilegeEscalation: false
  capabilities:
    drop:
      - ALL
  readOnlyRootFilesystem: true
```

---

## 🚨 Troubleshooting

### Issue: No metrics showing

**Check**:
```bash
# Is Zen Watcher running?
kubectl get pods -n zen-system

# Are metrics exposed?
kubectl port-forward -n zen-system svc/zen-watcher 8080:8080
curl http://localhost:8080/metrics

# Is Prometheus scraping?
kubectl logs -n monitoring prometheus-server | grep zen-watcher
```

### Issue: No events captured

**Check**:
```bash
# Are security tools running?
kubectl get pods -n trivy-system
kubectl get pods -n falco
kubectl get pods -n kyverno

# Check Zen Watcher logs
kubectl logs -n zen-system -l app.kubernetes.io/name=zen-watcher

# Trigger a test event
kubectl run test --image=nginx:1.14
```

### Issue: Dashboard not showing data

**Check**:
```bash
# Is datasource configured?
curl -u admin:admin http://localhost:3000/api/datasources

# Test datasource
curl http://victoriametrics:8428/api/v1/query?query=up

# Check dashboard queries
# Grafana → Dashboard → Settings → JSON Model
```

---

## 📚 Next Steps

- **Production Checklist**: [docs/OPERATIONAL_EXCELLENCE.md](OPERATIONAL_EXCELLENCE.md)
- **Security Features**: [docs/SECURITY.md](docs/SECURITY.md) (threat model, security layers, RBAC)
- **Vulnerability Reporting**: [VULNERABILITY_DISCLOSURE.md](../VULNERABILITY_DISCLOSURE.md) (root)
- **Monitoring Guide**: [config/monitoring/README.md](../config/monitoring/README.md)
- **Dashboard Guide**: [config/dashboards/DASHBOARD_GUIDE.md](../config/dashboards/DASHBOARD_GUIDE.md)

---

**Need help?** Open an issue on GitHub or check our documentation index.

