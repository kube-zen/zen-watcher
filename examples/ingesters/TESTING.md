# Example Ingester Testing Guide

This document provides step-by-step instructions for testing each example Ingester in zen-watcher 1.0.0-alpha.

## Prerequisites

- Kubernetes cluster with zen-watcher deployed
- `kubectl` configured to access the cluster
- Appropriate RBAC permissions to create Ingester CRDs and view Observations

## General Testing Pattern

For each example Ingester:

1. **Apply the Ingester CRD**
2. **Generate test events** (source-specific)
3. **Verify Observations are created** (check CRD output)
4. **Validate Observation fields** (ensure normalization worked)

## Example 1: Trivy Informer

### Apply Ingester

```bash
kubectl apply -f trivy-informer.yaml
```

### Verify Ingester Status

```bash
kubectl get ingester trivy-informer -n default
kubectl describe ingester trivy-informer -n default
```

**Expected**: Ingester should show `status.phase: Active` (or equivalent status field).

### Generate Test Events

If Trivy is installed and scanning:

```bash
# Trivy will automatically create VulnerabilityReport CRDs when scanning
# Check existing reports:
kubectl get vulnerabilityreports -A

# If no reports exist, trigger a scan (Trivy-specific):
# kubectl create job trivy-scan --from=cronjob/trivy-scan
```

**Alternative**: If Trivy is not installed, you can manually create a test VulnerabilityReport:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: aquasecurity.github.io/v1alpha1
kind: VulnerabilityReport
metadata:
  name: test-vuln-report
  namespace: default
spec:
  # Minimal test data
  vulnerabilities:
    - id: CVE-2024-TEST
      severity: HIGH
EOF
```

### Check Observations

```bash
# List Observations created by this ingester
kubectl get observations -n default -l source=trivy

# Or view all Observations
kubectl get observations -n default

# Inspect a specific Observation
kubectl get observation <name> -n default -o yaml
```

### Validate Observation Fields

```bash
# Check that normalization worked
kubectl get observation <name> -n default -o jsonpath='{.spec.category}'  # Should be "security"
kubectl get observation <name> -n default -o jsonpath='{.spec.eventType}'  # Should be "vulnerability"
kubectl get observation <name> -n default -o jsonpath='{.spec.severity}'   # Should be normalized (HIGH/MEDIUM/LOW)
kubectl get observation <name> -n default -o jsonpath='{.spec.source}'     # Should be "trivy"
```

**Expected Fields**:
- `spec.source`: "trivy"
- `spec.category`: "security"
- `spec.eventType`: "vulnerability"
- `spec.severity`: Normalized (HIGH/MEDIUM/LOW/CRITICAL)
- `spec.priority`: Numeric value (0.0-1.0)
- `spec.resource`: Resource information (kind, name, namespace)

## Example 2: Kyverno Informer

### Apply Ingester

```bash
kubectl apply -f kyverno-informer.yaml
```

### Verify Ingester Status

```bash
kubectl get ingester kyverno-policy-violations -n default
```

### Generate Test Events

If Kyverno is installed:

```bash
# Kyverno creates PolicyReport CRDs automatically
# Check existing reports:
kubectl get policyreports -A

# Trigger a policy violation (example):
kubectl run test-pod --image=nginx --restart=Never
# If a policy blocks this, a PolicyReport will be created
```

**Alternative**: Create a test PolicyReport:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: kyverno.io/v1
kind: PolicyReport
metadata:
  name: test-policy-report
  namespace: default
results:
  - policy: test-policy
    rule: test-rule
    status: fail
    severity: high
EOF
```

### Check Observations

```bash
kubectl get observations -n default -l source=kyverno
```

### Validate Observation Fields

```bash
kubectl get observation <name> -n default -o jsonpath='{.spec.category}'  # Should be "security"
kubectl get observation <name> -n default -o jsonpath='{.spec.eventType}'  # Should be "policy_violation"
kubectl get observation <name> -n default -o jsonpath='{.spec.severity}'   # Should be "HIGH" or "MEDIUM"
```

**Expected**: 
- `spec.source`: "kyverno"
- `spec.category`: "security"
- `spec.eventType`: "policy_violation"
- Severity mapped from `fail` → `HIGH`, `warn` → `MEDIUM`

## Example 3: Kube-bench Informer

### Apply Ingester

```bash
kubectl apply -f kube-bench-informer.yaml
```

### Generate Test Events

Kube-bench typically writes results to ConfigMaps:

```bash
# Check for existing kube-bench ConfigMaps
kubectl get configmaps -A | grep kube-bench

# If kube-bench is installed, run a scan:
# kubectl create job kube-bench-scan --from=cronjob/kube-bench
```

**Alternative**: Create a test ConfigMap:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: kube-bench-report
  namespace: default
  labels:
    app: kube-bench
data:
  report: |
    [INFO] 1 Master Node Security Configuration
    [FAIL] 1.1.1 Ensure that the API server pod specification file permissions are set to 644 or more restrictive
EOF
```

### Check Observations

```bash
kubectl get observations -n default -l source=kube-bench
```

### Validate Observation Fields

```bash
kubectl get observation <name> -n default -o jsonpath='{.spec.category}'  # Should be "compliance"
```

**Expected**:
- `spec.source`: "kube-bench"
- `spec.category`: "compliance"
- `spec.eventType`: Compliance-related type

## Example 4: High-Rate Kubernetes Events

### Apply Ingester

```bash
kubectl apply -f high-rate-kubernetes-events.yaml
```

### Generate Test Events

Generate a burst of Kubernetes events:

```bash
# Create and delete pods rapidly
for i in {1..50}; do
  kubectl run test-pod-$i --image=nginx --restart=Never
  kubectl delete pod test-pod-$i --wait=false
done

# Or create events directly
kubectl run test-event-generator --image=nginx --restart=Never
kubectl delete pod test-event-generator
```

### Check Observations

```bash
# Should see multiple Observations created
kubectl get observations -n default -l source=kubernetes-events --sort-by=.metadata.creationTimestamp
```

### Validate Observation Fields

```bash
kubectl get observation <name> -n default -o jsonpath='{.spec.category}'  # Should be "operations"
```

**Expected**:
- `spec.source`: "kubernetes-events" (or configured source name)
- `spec.category`: "operations"
- Multiple Observations created for the event burst

## Common Validation Checks

### Verify Pipeline Processing

All Observations should have:

1. **Source field**: Matches `spec.source` from Ingester
2. **Category**: Matches `spec.destinations[].mapping.domain`
3. **EventType**: Matches `spec.destinations[].mapping.type`
4. **Severity**: Normalized (HIGH/MEDIUM/LOW/CRITICAL)
5. **Priority**: Numeric value (0.0-1.0)
6. **Resource**: Resource information if available

### Verify Filtering

If `spec.filters.minPriority` is set, verify that low-priority events are filtered:

```bash
# Check that only high-priority Observations exist
kubectl get observations -n default -o jsonpath='{range .items[*]}{.spec.priority}{"\n"}{end}' | sort -n
# All priorities should be >= minPriority from Ingester config
```

### Verify Deduplication

If deduplication is enabled, send duplicate events and verify only one Observation is created:

```bash
# Send the same event twice (source-specific)
# Then check:
kubectl get observations -n default -l source=<source> --sort-by=.metadata.creationTimestamp
# Should see only one Observation for duplicate events within the dedup window
```

### Verify Optimization

If `spec.optimization.order: auto` is set, monitor optimization metrics:

```bash
# Check optimization metrics (if Prometheus is available)
kubectl exec -n zen-system zen-watcher-0 -- \
  curl -s http://localhost:8080/metrics | grep zen_watcher_optimization

# Or check logs for optimization decisions
kubectl logs -n zen-system -l app=zen-watcher | grep -i optimization
```

## Troubleshooting

### No Observations Created

1. **Check Ingester status**:
   ```bash
   kubectl describe ingester <name> -n <namespace>
   ```

2. **Check zen-watcher logs**:
   ```bash
   kubectl logs -n zen-system -l app=zen-watcher --tail=100
   ```

3. **Verify source events exist**:
   ```bash
   # For informer-based ingesters, check source CRDs exist
   kubectl get <source-crd> -A
   ```

4. **Check filter settings**: Events may be filtered out if they don't meet `minPriority` or match exclusion rules.

### Observations Missing Fields

1. **Check normalization mapping**: Verify `spec.destinations[].mapping` is correctly configured.
2. **Check source data**: Verify source events contain expected fields.
3. **Check logs**: Look for normalization errors in zen-watcher logs.

### Deduplication Not Working

1. **Check dedup window**: Verify `spec.deduplication.window` is set appropriately.
2. **Check dedup strategy**: Verify `spec.deduplication.strategy` is correct.
3. **Verify events are actually duplicates**: Check that events have identical content fingerprints.

## Next Steps

After validating examples:

1. **Customize for your use case**: Modify examples to match your specific requirements.
2. **Monitor metrics**: Set up Prometheus/Grafana to monitor zen-watcher performance.
3. **Review processing order**: Configure processing order and monitor performance metrics.
4. **Scale testing**: Test with higher event volumes to validate performance.

## References

- `README.md` - Overview of examples
- `zen-watcher/docs/INGESTER_API.md` - Complete Ingester API documentation
- `zen-admin/docs/INGESTER_V1_FINAL_SHAPE.md` - v1 spec design
- `zen-watcher/docs/INTELLIGENT_EVENT_PIPELINE.md` - Pipeline architecture

