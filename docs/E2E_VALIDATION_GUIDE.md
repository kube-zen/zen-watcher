# E2E Validation Guide

This guide describes how to validate zen-watcher 1.2.1 release against a real Kubernetes cluster.

## Overview

E2E validation ensures that:
- Helm chart installs correctly
- CRDs are created
- Example Ingesters work
- Observations are created
- Metrics and logs are accessible

## Prerequisites

- Kubernetes cluster (1.26+)
- `kubectl` configured with cluster access
- `helm` 3.8+ installed
- `obsctl` CLI (optional, for querying Observations)

## Manual Validation (Operator)

### Step 1: Set Context and Namespace

```bash
# Set your Kubernetes context
export CONTEXT=your-cluster-context

# Set namespace (default: zen-system)
export NAMESPACE=zen-system

# Verify context
kubectl config current-context
kubectl config get-contexts "$CONTEXT"
```

**Important**: Always use explicit `--context` and `--namespace` flags. Never modify kubeconfig.

### Step 2: Validate Helm Chart (Dry-Run)

```bash
# Run validation script in dry-run mode
cd zen-watcher
DRY_RUN=true CONTEXT=your-context NAMESPACE=zen-system ./test/e2e/validate-release.sh
```

This validates:
- Helm chart syntax
- Manifest correctness
- CRD installation
- Example Ingesters

### Step 3: Deploy zen-watcher

```bash
# Install via Helm
helm install zen-watcher kube-zen/zen-watcher \
  --namespace "$NAMESPACE" \
  --create-namespace \
  --kube-context "$CONTEXT" \
  --set image.tag=1.2.1

# Verify deployment
kubectl get pods --namespace "$NAMESPACE" --context "$CONTEXT"
kubectl get svc --namespace "$NAMESPACE" --context "$CONTEXT"
```

### Step 4: Apply Example Ingesters

```bash
# Apply Trivy example
kubectl apply --context "$CONTEXT" --namespace "$NAMESPACE" \
  -f examples/ingesters/trivy-informer.yaml

# Apply Kyverno example
kubectl apply --context "$CONTEXT" --namespace "$NAMESPACE" \
  -f examples/ingesters/kyverno-informer.yaml

# Verify Ingesters
kubectl get ingesters --namespace "$NAMESPACE" --context "$CONTEXT"
```

### Step 5: Validate Observations

```bash
# Query Observations using obsctl
obsctl list \
  --context "$CONTEXT" \
  --namespace "$NAMESPACE"

# Or using kubectl
kubectl get observations --namespace "$NAMESPACE" --context "$CONTEXT"
```

### Step 6: Check Metrics and Logs

```bash
# Port-forward to metrics endpoint
kubectl port-forward --namespace "$NAMESPACE" --context "$CONTEXT" \
  deployment/zen-watcher 8080:8080

# Query metrics
curl http://localhost:8080/metrics

# Check logs
kubectl logs --namespace "$NAMESPACE" --context "$CONTEXT" \
  -l app.kubernetes.io/name=zen-watcher
```

## CI-Style E2E Validation (Optional)

For CI pipelines, use the validation script:

```bash
# Full validation (dry-run)
DRY_RUN=true CONTEXT=ci-cluster NAMESPACE=zen-system ./test/e2e/validate-release.sh

# Actual deployment (if cluster is disposable)
DRY_RUN=false CONTEXT=ci-cluster NAMESPACE=zen-system ./test/e2e/validate-release.sh
```

### Cluster Provisioning

**Note**: Cluster provisioning is out of scope for WATCHER. CI should:
1. Provision a disposable cluster (e.g., kind, k3d, GKE)
2. Set `CONTEXT` environment variable
3. Run validation script
4. Tear down cluster after validation

## Validation Checklist

- [ ] Helm chart installs without errors
- [ ] CRDs are created (`kubectl get crds | grep zen.kube-zen.io`)
- [ ] zen-watcher pods are running
- [ ] Example Ingesters are applied
- [ ] Observations are created
- [ ] Metrics endpoint responds (`/metrics`)
- [ ] Logs show no errors

## Troubleshooting

### Context Not Found

**Error**: `Context 'xxx' not found in kubeconfig`

**Solution**: Verify context name:
```bash
kubectl config get-contexts
```

### Namespace Issues

**Error**: `namespace "zen-system" not found`

**Solution**: Create namespace or use existing:
```bash
kubectl create namespace zen-system --context "$CONTEXT"
```

### CRDs Not Installed

**Error**: `Ingester CRD not found`

**Solution**: Verify CRD installation:
```bash
kubectl get crds --context "$CONTEXT" | grep zen.kube-zen.io
```

## Safety Guarantees

This validation harness:
- ✅ **Never modifies kubeconfig**: No `kubectl config use-context` calls
- ✅ **Explicit context/namespace**: All commands require explicit flags
- ✅ **Dry-run by default**: Validation mode uses `--dry-run=client`
- ✅ **No destructive operations**: No cluster state changes in dry-run mode

## Related Documentation

- [Helm Deployment Guide](DEPLOYMENT_HELM.md) - Complete Helm installation guide
- [TOOLING_GUIDE.md](TOOLING_GUIDE.md#obsctl) - Querying Observations with obsctl
- [Troubleshooting](TROUBLESHOOTING.md) - Common issues and solutions

# Testing Falco and Kubernetes Audit Integration

This guide explains how to test zen-watcher's integration with Falco and Kubernetes Audit logs.

## Prerequisites

- Working Kubernetes cluster (kind, minikube, k3d, or cloud cluster)
- `kubectl` configured and authenticated
- `helm` installed
- `docker` or `podman` for building images

## Step 1: Create a Kubernetes Cluster

### Option A: Using kind

```bash
# Install kind (if not already installed)
curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-amd64
chmod +x ./kind
sudo mv ./kind /usr/local/bin/kind

# Create cluster
kind create cluster --name zen-test

# Note: Use --context flag instead of changing default context
# kubectl config use-context kind-zen-test  # Don't do this
```

### Option B: Using minikube

```bash
# Start minikube
minikube start

# Note: Use --context flag instead of changing default context
# kubectl config use-context minikube  # Don't do this
```

### Option C: Using k3d

```bash
# Create cluster
k3d cluster create zen-test

# Note: Use --context flag instead of changing default context
# kubectl config use-context k3d-zen-test  # Don't do this
```

## Step 2: Build and Load zen-watcher Image

```bash
cd /path/to/zen-watcher

# Build image
docker build -t kubezen/zen-watcher:test -f build/Dockerfile .

# Load into cluster
# For kind:
kind load docker-image kubezen/zen-watcher:test --name zen-test

# For minikube:
minikube image load kubezen/zen-watcher:test

# For k3d:
k3d image import kubezen/zen-watcher:test -c zen-test
```

## Step 3: Deploy zen-watcher

```bash
# Apply CRD
kubectl apply -f deployments/crds/observation_crd.yaml

# Create namespace
kubectl create namespace zen-system

# Deploy zen-watcher
cat > /tmp/zen-watcher-deploy.yaml <<'EOF'
apiVersion: v1
kind: ServiceAccount
metadata:
  name: zen-watcher
  namespace: zen-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: zen-watcher
rules:
- apiGroups: ["*"]
  resources: ["*"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["zen.kube-zen.io"]
  resources: ["observations"]
  verbs: ["get", "list", "watch", "create", "update", "patch"]
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
  namespace: zen-system
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: zen-watcher
  namespace: zen-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: zen-watcher
  template:
    metadata:
      labels:
        app: zen-watcher
    spec:
      serviceAccountName: zen-watcher
      containers:
      - name: zen-watcher
        image: kubezen/zen-watcher:test
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 8080
          name: http
        - containerPort: 9090
          name: metrics
        env:
        - name: LOG_LEVEL
          value: "INFO"
        resources:
          requests:
            memory: "64Mi"
            cpu: "100m"
          limits:
            memory: "256Mi"
            cpu: "500m"
---
apiVersion: v1
kind: Service
metadata:
  name: zen-watcher
  namespace: zen-system
spec:
  selector:
    app: zen-watcher
  ports:
  - port: 8080
    targetPort: 8080
    name: http
  - port: 9090
    targetPort: 9090
    name: metrics
EOF

kubectl apply -f /tmp/zen-watcher-deploy.yaml

# Wait for pod to be ready
kubectl wait --for=condition=ready pod -l app=zen-watcher -n zen-system --timeout=120s

# Verify
kubectl get pods -n zen-system
```

## Step 4: Install Falco

```bash
# Create namespace
kubectl create namespace falco-system

# Add Falco Helm repo
helm repo add falcosecurity https://falcosecurity.github.io/charts
helm repo update

# Install Falco with webhook output to zen-watcher
helm install falco falcosecurity/falco \
  --namespace falco-system \
  --set falco.grpc.enabled=true \
  --set falco.httpOutput.enabled=true \
  --set falco.httpOutput.url=http://zen-watcher.zen-system.svc.cluster.local:8080/falco/webhook \
  --wait --timeout=5m

# Verify Falco is running
kubectl get pods -n falco-system
```

**Note**: On k3d, Falco may not work due to kernel module/eBPF limitations. Use minikube or kind for Falco testing.

## Step 5: Test Falco Integration

### Trigger a Falco Event

```bash
# Run nmap scan (triggers Falco rule)
kubectl run test-scan --image=alpine --rm -i --restart=Never -- sh -c "apk add --no-cache nmap && nmap -sS 127.0.0.1"
```

### Check Observations

```bash
# Wait a few seconds for processing
sleep 10

# List observations
kubectl get observations -n zen-system

# View details
# Display observations with category
kubectl get observations -n zen-system -o json | jq -r '.items[] | "\(.metadata.name) | Source: \(.spec.source) | Category: \(.spec.category) | Severity: \(.spec.severity)"'

# Filter by category: security
kubectl get observations -n zen-system -o json | jq '.items[] | select(.spec.category == "security")'

# Filter by category: compliance
kubectl get observations -n zen-system -o json | jq '.items[] | select(.spec.category == "compliance")'

# Filter by source
kubectl get observations -n zen-system -o json | jq '.items[] | select(.spec.source == "falco")'
```

### Check Logs

```bash
# Zen-watcher logs
kubectl logs -n zen-system -l app=zen-watcher --tail=50 | grep -E "(falco|webhook|Observation)"

# Falco logs
kubectl logs -n falco-system -l app.kubernetes.io/name=falco --tail=20
```

## Step 6: Test Kubernetes Audit Webhook

### Configure Audit Logging (for kind/minikube)

For kind, you need to configure audit logging at cluster creation. For minikube, audit logging can be configured via API server flags.

### Test Audit Webhook Directly

```bash
# Port-forward to zen-watcher
kubectl port-forward -n zen-system svc/zen-watcher 8080:8080 &

# Test health endpoint
curl http://localhost:8080/health

# Send test audit event
curl -X POST http://localhost:8080/audit/webhook \
  -H "Content-Type: application/json" \
  -d '{
    "kind": "Event",
    "apiVersion": "audit.k8s.io/v1",
    "level": "Request",
    "auditID": "test-123",
    "stage": "ResponseComplete",
    "requestURI": "/api/v1/namespaces",
    "verb": "get",
    "user": {
      "username": "test-user"
    },
    "sourceIPs": ["127.0.0.1"],
    "responseStatus": {
      "code": 200
    }
  }'

# Check if observation was created
sleep 5
kubectl get observations -n zen-system -o json | jq '.items[] | select(.spec.source == "audit")'
```

## Step 7: Verify Integration

### Count Observations by Source

```bash
# Total observations
kubectl get observations -n zen-system --no-headers | wc -l

# By source
kubectl get observations -n zen-system -o json | jq -r '.items[].spec.source' | sort | uniq -c
```

### Expected Results

- **Falco**: Observations with `source: falco`, `category: security`, severity based on Falco rule
- **Audit**: Observations with `source: audit`, `category: compliance`, severity based on audit level

### Check Metrics

```bash
# Port-forward metrics port
kubectl port-forward -n zen-system svc/zen-watcher 9090:9090 &

# Query metrics
curl http://localhost:9090/metrics | grep zen_watcher
```

## Troubleshooting

### Zen-watcher Pod Not Starting

```bash
# Check pod status
kubectl describe pod -n zen-system -l app=zen-watcher

# Check logs
kubectl logs -n zen-system -l app=zen-watcher

# Check events
kubectl get events -n zen-system --sort-by='.lastTimestamp'
```

### Falco Not Sending Events

```bash
# Check Falco pod status
kubectl get pods -n falco-system

# Check Falco logs
kubectl logs -n falco-system -l app.kubernetes.io/name=falco

# Verify webhook URL is correct
kubectl get configmap -n falco-system -o yaml | grep -A 5 httpOutput
```

### No Observations Created

```bash
# Check zen-watcher logs for errors
kubectl logs -n zen-system -l app=zen-watcher --tail=100 | grep -E "(ERROR|WARN|webhook)"

# Verify webhook endpoints are accessible
kubectl exec -n zen-system -it deployment/zen-watcher -- wget -qO- http://localhost:8080/health || echo "Health check failed"

# Check RBAC permissions
kubectl auth can-i create observations --as=system:serviceaccount:zen-system:zen-watcher -n zen-system
```

### Falco on k3d

Falco requires kernel module or eBPF support, which k3d's lightweight kernel may not provide. Use minikube or kind for Falco testing.

## Cleanup

```bash
# Delete Falco
helm uninstall falco -n falco-system
kubectl delete namespace falco-system

# Delete zen-watcher
kubectl delete -f /tmp/zen-watcher-deploy.yaml
kubectl delete namespace zen-system
kubectl delete crd observations.zen.kube-zen.io

# Delete cluster (if using kind/minikube/k3d)
# Recommended: Use cleanup script (works with all platforms)
#   ZEN_CLUSTER_NAME=zen-test ./scripts/cleanup-demo.sh kind
#   ZEN_CLUSTER_NAME=zen-test ./scripts/cleanup-demo.sh minikube
#   ZEN_CLUSTER_NAME=zen-test ./scripts/cleanup-demo.sh k3d
# Or manually:
#   kind delete cluster --name zen-test
#   minikube delete -p zen-test
#   k3d cluster delete zen-test
```

## Summary

This testing guide covers:
- ✅ Deploying zen-watcher to a Kubernetes cluster
- ✅ Installing Falco with webhook output to zen-watcher
- ✅ Testing Falco event generation and observation creation
- ✅ Testing Kubernetes Audit webhook integration
- ✅ Verifying observations are created correctly
- ✅ Troubleshooting common issues

The integration is working when:
1. Falco events trigger Observations with `source: falco`
2. Audit webhook calls create Observations with `source: audit`
3. All Observations are stored in the `zen-system` namespace
4. Metrics show webhook requests and event processing
