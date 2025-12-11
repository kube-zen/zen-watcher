# Canonical Ingester Examples (zen-watcher 1.0.0-alpha)

This directory contains canonical examples of Ingester CRDs for common use cases in zen-watcher 1.0.0-alpha.

## Examples

### trivy-informer.yaml
Trivy vulnerability scanner using informer adapter to watch VulnerabilityReport CRDs.

**Key features:**
- Informer-based ingestion
- Filter by priority (minPriority: 0.3)
- Deduplication with 1h window
- Auto-optimization enabled
- Security domain normalization

### kyverno-informer.yaml
Kyverno policy violations using informer adapter to watch PolicyReport CRDs.

**Key features:**
- Informer-based ingestion
- Filter-first optimization strategy
- 2h deduplication window
- Security domain normalization

### kube-bench-informer.yaml
Kube-bench compliance reports using informer adapter to watch ConfigMaps.

**Key features:**
- Informer-based ingestion (ConfigMaps via GVR)
- Compliance domain normalization
- 24h deduplication window
- Auto-optimization enabled

### high-rate-k8s-events.yaml
High-rate Kubernetes events using k8s-events adapter.

**Key features:**
- k8s-events ingester type
- High-rate optimization (5m analysis interval)
- Rate limiting enabled (1000 req/s)
- 30s deduplication window for high-frequency events
- Operations domain normalization

## Pipeline Configuration

All examples follow the canonical pipeline order:
```
source → (filter | dedup, ordered dynamically by optimization) → normalize → destinations[]
```

### Processing Fields

- **`spec.filters`**: Filter configuration (minPriority, namespaces, etc.)
- **`spec.deduplication`**: Deduplication configuration (window, strategy)
- **`spec.optimization`**: Auto-optimization configuration (order, autoOptimize, thresholds)
- **`spec.destinations[].mapping`**: Normalization configuration (domain, type, priority mapping)

### Optimization

The optimization engine automatically chooses the optimal order (filter_first vs dedup_first) based on:
- Traffic statistics per source
- Filter effectiveness
- Dedup effectiveness
- Low severity percentage

Set `spec.optimization.order: auto` to enable automatic optimization, or specify `filter_first` or `dedup_first` explicitly.

## Destination Policy (zen-watcher 1.0.0-alpha)

All examples use the official OSS destination:
```yaml
destinations:
  - type: crd
    value: observations
```

**Note**: zen-watcher OSS 1.0.0-alpha only supports `type: crd` with `value: observations`. For external sinks (webhooks, queues, SaaS), use external agents (kubewatch, robusta) that watch Observations CRDs, or use zen-bridge (platform component) for SaaS integration.

## How to Apply Examples

Each example can be applied directly to your cluster:

```bash
# Apply Trivy ingester
kubectl apply -f trivy-informer.yaml

# Apply Kyverno ingester
kubectl apply -f kyverno-informer.yaml

# Apply Kube-bench ingester
kubectl apply -f kube-bench-informer.yaml

# Apply high-rate K8s events ingester
kubectl apply -f high-rate-k8s-events.yaml
```

Verify the ingester is running:
```bash
kubectl get ingesters -A
kubectl describe ingester <name> -n <namespace>
```

## Performance Testing

To validate optimization engine effectiveness:

1. Deploy an ingester with `spec.optimization.order: auto`
2. Generate load (e.g., high-rate k8s-events)
3. Monitor metrics:
   - `zen_watcher_optimization_source_events_processed_total`
   - `zen_watcher_optimization_source_processing_latency_seconds`
   - `zen_watcher_optimization_strategy_changes_total`
4. Verify optimization engine chooses filter_first or dedup_first based on metrics

### Generate Load for High-Rate Events

```bash
# Apply high-rate ingester
kubectl apply -f high-rate-k8s-events.yaml

# Generate Kubernetes events (example script)
for i in {1..100}; do
  kubectl run test-pod-$i --image=nginx --restart=Never
  kubectl delete pod test-pod-$i
done
```

### Observe P95 Latency and Throughput

```bash
# Watch optimization metrics
kubectl exec -n zen-system zen-watcher-0 -- \
  curl -s http://localhost:8080/metrics | grep zen_watcher_optimization

# Query Prometheus (if available)
# rate(zen_watcher_optimization_source_events_processed_total[5m])
# histogram_quantile(0.95, zen_watcher_optimization_source_processing_latency_seconds)
```

### Optimization Behavior

The optimization engine will automatically switch between `filter_first` and `dedup_first` per source based on:
- If filter drops >70% events → `filter_first`
- If dedup effectiveness >50% → `dedup_first`
- Default: `filter_first`

Monitor `zen_watcher_optimization_strategy_changes_total` to see when the engine switches strategies.

## References

- `zen-watcher/docs/INGESTER_API.md` - Ingester API documentation
- `zen-admin/docs/CRD_INGESTER_DESTINATION_DESIGN.md` - CRD design principles
- `zen-admin/docs/INGESTER_V1_FINAL_SHAPE.md` - v1 spec design

