# Quick Start Guide

Get zen-watcher up and running in minutes. This guide covers installation, basic configuration, verification, and troubleshooting.

## Prerequisites

- **Kubernetes 1.26+** cluster
- **Helm 3.8+** (for Helm installation)
- **kubectl** configured to access your cluster

## Installation

### Step 1: Install via Helm (Recommended)

Helm automatically installs CRDs when `crds.enabled=true` (default).

```bash
# Add the Helm repository
helm repo add kube-zen https://kube-zen.github.io/helm-charts
helm repo update

# Install zen-watcher
helm install zen-watcher kube-zen/zen-watcher \
  --namespace zen-system \
  --create-namespace
```

**Alternative: Install CRDs Out-of-Band**

If you prefer to manage CRDs separately (e.g., via GitOps):

```bash
# Disable CRD installation in Helm
helm install zen-watcher kube-zen/zen-watcher \
  --namespace zen-system \
  --create-namespace \
  --set crds.enabled=false

# Install CRDs manually
kubectl apply -f https://raw.githubusercontent.com/kube-zen/zen-watcher/main/deployments/crds/observation_crd.yaml
kubectl apply -f https://raw.githubusercontent.com/kube-zen/zen-watcher/main/deployments/crds/ingester_crd.yaml
```

### Step 2: Verify Installation

```bash
# Check pods are running
kubectl get pods -n zen-system

# Expected output:
# NAME                           READY   STATUS    RESTARTS   AGE
# zen-watcher-xxxxxxxxxx-xxxxx   1/1     Running   0          30s

# Check service
kubectl get svc -n zen-system

# Check CRDs
kubectl get crds | grep zen.kube-zen.io
# Should show: observations.zen.kube-zen.io and ingesters.zen.kube-zen.io
```

### Step 3: Apply One Ingester (Mandatory)

**zen-watcher requires at least one Ingester to start collecting Observations.** Without an Ingester, the operator runs but creates no Observations.

**Option A: Kubernetes Events (Built-in, No External Tools)**

```bash
kubectl apply -f - <<EOF
apiVersion: zen.kube-zen.io/v1alpha1
kind: Ingester
metadata:
  name: kubernetes-events
  namespace: zen-system
spec:
  source: kubernetes-events
  enabled: true
EOF
```

**Option B: Trivy (If Trivy is Installed)**

```bash
kubectl apply -f - <<EOF
apiVersion: zen.kube-zen.io/v1alpha1
kind: Ingester
metadata:
  name: trivy-informer
  namespace: zen-system
spec:
  source: trivy
  enabled: true
EOF
```

**Option C: Use Example Ingesters**

```bash
# Apply example Ingesters from the repository
kubectl apply -f https://raw.githubusercontent.com/kube-zen/zen-watcher/main/examples/ingesters/trivy-informer.yaml
```

### Step 4: Verify Observations

```bash
# Check Ingesters are active
kubectl get ingesters -n zen-system

# Check Observations are being created
kubectl get observations -n zen-system

# Watch Observations in real-time
watch kubectl get observations -n zen-system

# Count Observations
kubectl get observations -n zen-system --no-headers | wc -l
```

**Expected**: Observations should start appearing within 30-60 seconds after applying an Ingester.

### Step 5: Verify Metrics Endpoint

```bash
# Port-forward to metrics endpoint
kubectl port-forward -n zen-system svc/zen-watcher 8080:8080

# In another terminal, query metrics
curl http://localhost:8080/metrics | grep zen_watcher

# Expected metrics:
# zen_watcher_observations_created_total
# zen_watcher_ingesters_active
# zen_watcher_ingesters_last_event_timestamp_seconds
```

### Step 6: Check Logs

```bash
# View logs
kubectl logs -n zen-system -l app.kubernetes.io/name=zen-watcher

# Follow logs
kubectl logs -n zen-system -l app.kubernetes.io/name=zen-watcher -f

# Expected: No errors, "Leader election started", "Informer started for source: <source>"
```

## Uninstall / Cleanup

### Uninstall zen-watcher (Preserve CRDs)

```bash
helm uninstall zen-watcher --namespace zen-system
```

**Note**: CRDs are preserved by default. Observations and Ingesters remain in the cluster.

### Remove CRDs (Complete Cleanup)

```bash
# Delete all Observations first
kubectl delete observations --all --all-namespaces

# Delete all Ingesters
kubectl delete ingesters --all --all-namespaces

# Delete CRDs
kubectl delete crd observations.zen.kube-zen.io
kubectl delete crd ingesters.zen.kube-zen.io
```

## Troubleshooting

### Top 3 Failure Modes

#### 1. NetworkPolicy → Kubernetes API Blocked

**Symptom**: Pods start but can't connect to Kubernetes API, logs show connection errors.

**Cause**: NetworkPolicy is blocking egress to Kubernetes API.

**Solution**: Configure explicit Kubernetes API destinations in `values.yaml`:

```yaml
networkPolicy:
  egress:
    enabled: true
    allowKubernetesAPI: true
    # REQUIRED: Explicit destinations (no silent defaults)
    kubernetesServiceIP: "10.96.0.0/12"  # Your cluster's service CIDR
    kubernetesAPICIDRs:                   # Your cluster's API server CIDRs
      - "10.0.0.0/8"                      # Example: adjust for your cluster
```

**Find your cluster's service CIDR**:
```bash
kubectl cluster-info dump | grep -i service-cluster-ip-range
```

**Find your cluster's API server CIDRs**:
```bash
kubectl get endpoints kubernetes -o jsonpath='{.subsets[*].addresses[*].ip}'
```

#### 2. Running but No Observations

**Symptom**: Pods are `Running`, logs show no errors, but `kubectl get observations` returns nothing.

**Cause**: No Ingester is configured or Ingester is disabled.

**Solution**:
1. Check if Ingesters exist:
   ```bash
   kubectl get ingesters -A
   ```

2. If no Ingesters, apply one (see Step 3 above).

3. If Ingesters exist but disabled, enable them:
   ```bash
   kubectl patch ingester <name> -n <namespace> --type merge -p '{"spec":{"enabled":true}}'
   ```

4. Verify Ingester status:
   ```bash
   kubectl get ingester <name> -n <namespace> -o yaml
   # Check spec.enabled is true
   ```

#### 3. Webhook Auth Disabled by Default

**Symptom**: Webhook endpoints are accessible without authentication (security risk).

**Cause**: Webhook authentication is disabled by default for easier initial setup.

**Solution**: Enable webhook authentication in `values.yaml`:

```yaml
extraEnv:
  - name: WEBHOOK_AUTH_DISABLED
    value: "false"
  - name: WEBHOOK_AUTH_TOKEN
    valueFrom:
      secretKeyRef:
        name: zen-watcher-webhook-token
        key: token
  - name: WEBHOOK_ALLOWED_IPS
    value: "10.0.0.0/8,192.168.0.0/16"  # Adjust for your network
```

**Create the secret**:
```bash
kubectl create secret generic zen-watcher-webhook-token \
  --from-literal=token=$(openssl rand -hex 32) \
  -n zen-system
```

**Upgrade with new values**:
```bash
helm upgrade zen-watcher kube-zen/zen-watcher \
  --namespace zen-system \
  -f values.yaml
```

### Other Common Issues

#### Pods Not Starting

```bash
# Check pod status
kubectl get pods -n zen-system

# Check pod events
kubectl describe pod <pod-name> -n zen-system

# Check logs
kubectl logs <pod-name> -n zen-system
```

**Common causes**:
- Image pull errors (check image repository/tag)
- Resource limits too low
- RBAC permissions missing

#### CRDs Not Installed

```bash
# Verify CRDs exist
kubectl get crds | grep zen.kube-zen.io

# If missing, install manually
kubectl apply -f https://raw.githubusercontent.com/kube-zen/zen-watcher/main/deployments/crds/observation_crd.yaml
kubectl apply -f https://raw.githubusercontent.com/kube-zen/zen-watcher/main/deployments/crds/ingester_crd.yaml
```

#### Metrics Not Available

```bash
# Check service exists
kubectl get svc zen-watcher -n zen-system

# Port-forward and test
kubectl port-forward -n zen-system svc/zen-watcher 8080:8080
curl http://localhost:8080/metrics
```

## Next Steps

- **Configure More Sources**: See [examples/ingesters/](../examples/ingesters/) for Trivy, Falco, Kyverno, and more
- **Enable Alerts**: See [docs/OPERATIONAL_EXCELLENCE.md](OPERATIONAL_EXCELLENCE.md) for PrometheusRule setup
- **Production Hardening**: See [docs/SECURITY.md](SECURITY.md) for security best practices
- **Scaling**: See [docs/SCALING.md](SCALING.md) for HA and scaling strategies

## Additional Resources

- **Full Documentation**: [docs/INDEX.md](INDEX.md)
- **Helm Deployment**: [docs/DEPLOYMENT_HELM.md](DEPLOYMENT_HELM.md)
- **Ingester API**: [docs/INGESTER_API.md](INGESTER_API.md)
- **Troubleshooting**: [docs/OPERATIONAL_EXCELLENCE.md](OPERATIONAL_EXCELLENCE.md)

