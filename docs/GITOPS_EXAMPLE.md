# GitOps Example: Multi-Ingester Deployment with ArgoCD

This example demonstrates deploying 20 Ingesters via ArgoCD, showing both the happy path and entitlement gating behavior.

## Prerequisites

- ArgoCD installed and configured
- zen-watcher CRDs installed (`crds.enabled=true` or managed separately)

## Example: 20 Ingesters via ArgoCD

### ArgoCD Application

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: zen-watcher-ingesters
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/your-org/zen-gitops
    targetRevision: main
    path: ingesters
  destination:
    server: https://kubernetes.default.svc
    namespace: zen-system
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
      - CreateNamespace=true
```

### Ingester Configurations (20 examples)

**File: `ingesters/01-pod-events.yaml`**
```yaml
apiVersion: zen.kube-zen.io/v1alpha1
kind: Ingester
metadata:
  name: pod-events
  namespace: zen-system
spec:
  sources:
    - name: pod-informer
      type: informer
      informer:
        gvr:
          group: ""
          version: v1
          resource: pods
        namespace: default
  destinations:
    - type: crd
      gvr:
        group: zen.kube-zen.io
        version: v1alpha1
        resource: observations
```

**File: `ingesters/02-security-scans.yaml`**
```yaml
apiVersion: zen.kube-zen.io/v1alpha1
kind: Ingester
metadata:
  name: security-scans
  namespace: zen-system
spec:
  sources:
    - name: trivy-webhook
      type: webhook
      webhook:
        path: /webhook/trivy
        auth:
          type: bearer
          secretRef: trivy-webhook-secret
  destinations:
    - type: crd
      gvr:
        group: zen.kube-zen.io
        version: v1alpha1
        resource: observations
```

**File: `ingesters/03-multi-source.yaml`** (Example with 2 sources)
```yaml
apiVersion: zen.kube-zen.io/v1alpha1
kind: Ingester
metadata:
  name: multi-source-ingester
  namespace: zen-system
spec:
  sources:
    - name: informer-source
      type: informer
      informer:
        gvr:
          group: apps
          version: v1
          resource: deployments
    - name: webhook-source
      type: webhook
      webhook:
        path: /webhook/events
  destinations:
    - type: crd
      gvr:
        group: zen.kube-zen.io
        version: v1alpha1
        resource: observations
```

*(Continue with 17 more ingester configurations...)*

### Verifying Status

```bash
# Check Ingester status (multi-source)
kubectl get ingesters -n zen-system
kubectl describe ingester pod-events -n zen-system
# Shows: status.sources[] with name, type, state, lastError, lastSeen
```

## Key Points

1. **All examples use v1alpha1**: No v1 or v2 references
2. **Multi-source support**: `spec.sources[]` array for multiple input sources
3. **Status tracking**: Per-source status in `status.sources[]`
4. **GitOps-friendly**: All CRs apply cleanly and can be managed via ArgoCD

---

**See Also**:
- [API Stability Policy](./API_STABILITY_POLICY.md)
- [Ingester API](./INGESTER_API.md)
- [Observation API Guide](./OBSERVATION_API_PUBLIC_GUIDE.md)

