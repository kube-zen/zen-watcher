# GitOps Example: Multi-Ingester Deployment with ArgoCD

This example demonstrates deploying 20 Ingesters via ArgoCD, showing both the happy path and entitlement gating behavior.

## Prerequisites

- ArgoCD installed and configured
- zen-watcher CRDs installed (`crds.enabled=true` or managed separately)
- zen-platform DeliveryFlow CRDs installed (for commercial routing)

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

## Commercial Routing with Entitlement Gating

### DeliveryFlow with Entitlement Check

**File: `delivery-flows/production-routing.yaml`**
```yaml
apiVersion: routing.zen.kube-zen.io/v1alpha1
kind: DeliveryFlow
metadata:
  name: production-routing
  namespace: zen-system
spec:
  sources:
    - namespace: zen-system
      name: pod-events
      sourceName: pod-informer
  outputs:
    - name: primary-siem
      targets:
        - destinationRef:
            name: siem-destination
            namespace: zen-system
          role: primary
        - destinationRef:
            name: siem-standby
            namespace: zen-system
          role: standby
      failoverPolicy:
        switchAfter: 30s
        cooldown: 5m
        maxSwitchesPerHour: 10
status:
  conditions:
    - type: Entitled
      status: "False"  # Not paid - delivery blocked
      reason: UnpaidSubscription
      message: Tenant subscription is not active
    - type: Ready
      status: "False"
      reason: NotEntitled
  outputs:
    - name: primary-siem
      activeTarget:
        destinationRef:
          name: siem-destination
          namespace: zen-system
        role: primary
      linkHealth:
        - destinationRef:
            name: siem-destination
            namespace: zen-system
          healthy: false
          lastError: "Delivery blocked: subscription not active"
```

### Behavior

1. **Resources Apply Cleanly**: All Ingester and DeliveryFlow CRs are created successfully
2. **Entitlement Check**: DeliveryFlow controller checks tenant entitlement
3. **Status Update**: `status.conditions[].type=Entitled` set to `False` when not paid
4. **Delivery Blocked**: Events are not delivered to destinations, but CRs remain valid
5. **GitOps Safe**: No CR rejection - entitlement only affects delivery behavior

### Verifying Status

```bash
# Check Ingester status (multi-source)
kubectl get ingesters -n zen-system
kubectl describe ingester pod-events -n zen-system
# Shows: status.sources[] with name, type, state, lastError, lastSeen

# Check DeliveryFlow entitlement
kubectl get deliveryflows -n zen-system
kubectl describe deliveryflow production-routing -n zen-system
# Shows: status.conditions[].type=Entitled status=False
```

## Key Points

1. **All examples use v1alpha1**: No v1 or v2 references
2. **Multi-source support**: `spec.sources[]` array for multiple input sources
3. **Status tracking**: Per-source status in `status.sources[]`
4. **Entitlement gating**: GitOps-safe - CRs apply, delivery is gated
5. **Failover support**: Primary/standby targets with automatic failover

---

**See Also**:
- [API Stability Policy](./API_STABILITY_POLICY.md)
- [Ingester API](./INGESTER_API.md)
- [Observation API Guide](./OBSERVATION_API_PUBLIC_GUIDE.md)

