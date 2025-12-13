# Zen-Watcher ConfigMap Configuration

**Purpose**: Configure zen-watcher features using Kubernetes ConfigMaps

**Last Updated**: 2025-12-13

---

## Overview

zen-watcher supports ConfigMap-based configuration for runtime feature management. This approach:

- ✅ **Leverages Existing Infrastructure**: Uses the existing informer system
- ✅ **Runtime Updates**: Change configuration without pod restarts
- ✅ **Hierarchical Configuration**: Base + environment-specific overrides
- ✅ **GitOps Friendly**: Configuration as code in version control

## Configuration Structure

### Base Configuration

The base configuration (`zen-watcher-base-config`) contains default settings for all features:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: zen-watcher-base-config
  namespace: zen-system
data:
  features.yaml: |
    worker_pool:
      enabled: false
      size: 5
      queue_size: 1000
    
    event_batching:
      enabled: false
      batch_size: 50
      batch_age: 10s
      flush_interval: 30s
    
    http_client:
      timeout: 30s
      max_connections: 100
      rate_limit: 1000
    
    namespace_filtering:
      enabled: true
      included_namespaces: []
      excluded_namespaces: ["kube-system", "kube-public", "kube-node-lease"]
```

### Environment-Specific Overrides

Environment-specific ConfigMaps (e.g., `zen-watcher-prod-config`) can override base settings:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: zen-watcher-prod-config
  namespace: zen-system
data:
  features.yaml: |
    worker_pool:
      enabled: true
      size: 20
      queue_size: 5000
    
    event_batching:
      enabled: true
      batch_size: 200
      batch_age: 5s
```

## Configuration Precedence

1. **Default Values**: Built-in sensible defaults
2. **Base ConfigMap**: `zen-watcher-base-config`
3. **Environment ConfigMap**: `zen-watcher-{environment}-config` (if specified)
4. **Runtime Updates**: Changes to ConfigMaps are applied immediately

## Feature Configuration

### Worker Pool

Controls async event processing:

```yaml
worker_pool:
  enabled: true      # Enable/disable worker pool
  size: 20          # Number of worker goroutines
  queue_size: 5000  # Maximum queue depth
```

**Environment Variables** (fallback if ConfigMap not available):
- `WORKER_POOL_ENABLED=true`
- `WORKER_POOL_SIZE=20`

### Event Batching

Batches observations for high-volume destinations:

```yaml
event_batching:
  enabled: true         # Enable/disable batching
  batch_size: 200      # Maximum events per batch
  batch_age: 5s        # Maximum age before flush
  flush_interval: 30s  # Periodic flush interval
```

**Environment Variables** (fallback):
- `EVENT_BATCHING_ENABLED=true`
- `EVENT_BATCH_SIZE=200`
- `EVENT_BATCH_AGE=5s`

### HTTP Client

Configures connection pooling and rate limiting:

```yaml
http_client:
  timeout: 30s          # Request timeout
  max_connections: 100  # Maximum idle connections
  rate_limit: 1000     # Requests per second limit
```

**Environment Variables** (fallback):
- `HTTP_TIMEOUT=30s`
- `HTTP_MAX_IDLE_CONNS=100`
- `HTTP_RATE_LIMIT_RPS=1000`

### Namespace Filtering

Filters informer resources by namespace:

```yaml
namespace_filtering:
  enabled: true
  included_namespaces: ["production", "staging"]  # Only watch these namespaces
  excluded_namespaces: ["kube-system"]            # Exclude these namespaces
```

## Deployment

### 1. Create ConfigMaps

```bash
# Apply base configuration
kubectl apply -f deployments/configmaps/zen-watcher-base-config.yaml

# Apply environment-specific configuration (optional)
kubectl apply -f deployments/configmaps/zen-watcher-prod-config.yaml
```

### 2. Configure Environment Variables

Set these environment variables in your deployment:

```yaml
env:
- name: CONFIG_NAMESPACE
  value: "zen-system"
- name: BASE_CONFIG_NAME
  value: "zen-watcher-base-config"
- name: ENV_CONFIG_NAME
  value: "zen-watcher-prod-config"  # Optional
```

### 3. Verify Configuration

Check logs for configuration updates:

```bash
kubectl logs -n zen-system deployment/zen-watcher | grep "config_update"
```

## Runtime Updates

Configuration changes are applied immediately without pod restarts:

```bash
# Update worker pool size
kubectl patch configmap zen-watcher-base-config -n zen-system --type merge -p '
data:
  features.yaml: |
    worker_pool:
      enabled: true
      size: 30
      queue_size: 10000
'
```

Changes are detected within seconds and applied automatically.

## Best Practices

1. **Use Base + Environment Pattern**: Keep defaults in base, overrides in environment ConfigMaps
2. **Version Control**: Store ConfigMaps in Git for auditability
3. **Gradual Rollout**: Test configuration changes in staging before production
4. **Monitor Metrics**: Watch Prometheus metrics after configuration changes
5. **Documentation**: Document environment-specific configurations

## Troubleshooting

### Configuration Not Applied

1. Check ConfigMap exists:
   ```bash
   kubectl get configmap zen-watcher-base-config -n zen-system
   ```

2. Verify YAML syntax:
   ```bash
   kubectl get configmap zen-watcher-base-config -n zen-system -o yaml | grep -A 20 features.yaml
   ```

3. Check logs for errors:
   ```bash
   kubectl logs -n zen-system deployment/zen-watcher | grep -i config
   ```

### Configuration Conflicts

If both base and environment ConfigMaps exist, environment settings take precedence. To debug:

```bash
# Check current merged configuration
kubectl logs -n zen-system deployment/zen-watcher | grep "config_update"
```

## See Also

- [Ingester CRD Configuration](INGESTER_CRD.md) - Source-specific configuration
- [Performance Tuning](PERFORMANCE.md) - Performance optimization guide
- [Scaling Guide](SCALING.md) - Horizontal scaling patterns

