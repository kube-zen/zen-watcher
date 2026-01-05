# Configuration Guide

This document describes all configuration options for zen-watcher, including environment variables, ConfigMap settings, and Ingester CRD configuration.

---

## Environment Variables

### API Group Configuration

#### `ZEN_API_GROUP`

**Purpose**: Override the default API group for zen-watcher resources.

**Default**: `zen.kube-zen.io`

**Description**: 
- Controls the API group used for Observation CRDs and Ingester CRDs
- Allows using custom API groups for vendor-neutral deployments
- Affects GVR resolution when using `value` in destination configuration

**Example**:
```bash
# Use custom API group
export ZEN_API_GROUP="observations.k8s.io"

# zen-watcher will use:
# - observations.k8s.io/v1/observations (instead of zen.kube-zen.io/v1/observations)
# - observations.k8s.io/v1alpha1/ingesters (instead of zen.kube-zen.io/v1alpha1/ingesters)
```

**Note**: This is a backward-compatible change. If not set, defaults to `zen.kube-zen.io` for compatibility with existing deployments.

### Deduplication Configuration

#### `DEDUP_WINDOW_SECONDS`

**Purpose**: Default deduplication window in seconds.

**Default**: `60` (1 minute)

**Description**: Time window for deduplication when not specified in Ingester CRD.

**Example**:
```bash
export DEDUP_WINDOW_SECONDS=300  # 5 minutes
```

#### `DEDUP_MAX_SIZE`

**Purpose**: Maximum size of deduplication cache.

**Default**: `10000`

**Description**: Maximum number of entries in the deduplication cache per source.

**Example**:
```bash
export DEDUP_MAX_SIZE=50000  # Larger cache for high-volume sources
```

### TTL Configuration

#### `OBSERVATION_TTL_SECONDS`

**Purpose**: Default TTL for observations in seconds.

**Default**: `604800` (7 days)

**Description**: Time-to-live for observations when not specified per-observation or in Ingester CRD.

**Example**:
```bash
export OBSERVATION_TTL_SECONDS=2592000  # 30 days
```

**Validation**: 
- Minimum: 60 seconds (1 minute)
- Maximum: 31536000 seconds (1 year)
- Values outside this range are automatically adjusted with a warning

#### `OBSERVATION_TTL_DAYS`

**Purpose**: Default TTL for observations in days (alternative to `OBSERVATION_TTL_SECONDS`).

**Default**: `7` days

**Description**: Convenience option for specifying TTL in days. Takes precedence over `OBSERVATION_TTL_SECONDS` if both are set.

**Example**:
```bash
export OBSERVATION_TTL_DAYS=30  # 30 days
```

**Note**: If parsing fails, a warning is logged and the default TTL is used.

### Garbage Collection Configuration

#### `GC_INTERVAL`

**Purpose**: Interval between garbage collection runs.

**Default**: `1h` (1 hour)

**Description**: How often the GC collector runs to clean up expired observations.

**Example**:
```bash
export GC_INTERVAL=30m  # Every 30 minutes
```

#### `GC_TIMEOUT`

**Purpose**: Timeout for a single GC run.

**Default**: `5m` (5 minutes)

**Description**: Maximum time allowed for a single GC cycle before timing out.

**Example**:
```bash
export GC_TIMEOUT=10m  # 10 minute timeout
```

### Logging Configuration

#### `LOG_LEVEL`

**Purpose**: Set the logging level.

**Default**: `INFO`

**Valid Values**: `DEBUG`, `INFO`, `WARN`, `ERROR`, `CRITICAL`

**Example**:
```bash
export LOG_LEVEL=DEBUG  # Enable debug logging
```

### Namespace Configuration

#### `WATCH_NAMESPACE`

**Purpose**: Restrict zen-watcher to a specific namespace.

**Default**: Empty (all namespaces)

**Description**: When set, zen-watcher only processes resources in the specified namespace.

**Example**:
```bash
export WATCH_NAMESPACE=zen-system  # Only watch zen-system namespace
```

---

## ConfigMap Configuration

zen-watcher supports ConfigMap-based configuration for runtime feature management. This approach:

- ✅ **Leverages Existing Infrastructure**: Uses the existing informer system
- ✅ **Runtime Updates**: Change configuration without pod restarts
- ✅ **Hierarchical Configuration**: Base + environment-specific overrides
- ✅ **GitOps Friendly**: Configuration as code in version control

### Configuration Structure

#### Base Configuration

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
    
    namespace_filtering:
      enabled: true
      included_namespaces: []
      excluded_namespaces: ["kube-system", "kube-public", "kube-node-lease"]
```

#### Environment-Specific Overrides

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

### Feature Configuration

#### Worker Pool

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

#### Event Batching

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

#### Namespace Filtering

Filters informer resources by namespace:

```yaml
namespace_filtering:
  enabled: true
  included_namespaces: ["production", "staging"]  # Only watch these namespaces
  excluded_namespaces: ["kube-system"]            # Exclude these namespaces
```

### ConfigMap Deployment

#### 1. Create ConfigMaps

```bash
# Apply base configuration
kubectl apply -f deployments/configmaps/zen-watcher-base-config.yaml

# Apply environment-specific configuration (optional)
kubectl apply -f deployments/configmaps/zen-watcher-prod-config.yaml
```

#### 2. Configure Environment Variables

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

#### 3. Verify Configuration

Check logs for configuration updates:

```bash
kubectl logs -n zen-system deployment/zen-watcher | grep "config_update"
```

### Runtime Updates

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

---

## Configuration Precedence

Configuration is applied in the following order (highest to lowest priority):

1. **Ingester CRD** (highest priority)
   - Source-specific configuration
   - Destination GVR configuration
   - TTL per observation
   - Deduplication settings

2. **ConfigMap** (if configured)
   - Global defaults
   - Shared normalization rules
   - Feature flags (worker pool, event batching, namespace filtering)
   - Environment-specific overrides

3. **Environment Variables**
   - Default TTL
   - GC settings
   - API group override
   - Feature flags (fallback if ConfigMap not available)

4. **Built-in Defaults** (lowest priority)
   - Hardcoded defaults in code
   - See `pkg/config/constants.go`

---

## Examples

### Custom API Group Deployment

```bash
# Deploy with custom API group
export ZEN_API_GROUP="observations.k8s.io"
helm install zen-watcher kube-zen/zen-watcher \
  --namespace zen-system \
  --create-namespace \
  --set env.ZEN_API_GROUP=observations.k8s.io
```

### High-Volume Configuration

```bash
# Optimize for high-volume sources
export DEDUP_MAX_SIZE=100000
export DEDUP_WINDOW_SECONDS=300
export GC_INTERVAL=30m
export OBSERVATION_TTL_SECONDS=2592000  # 30 days
```

### Development Configuration

```bash
# Development settings
export LOG_LEVEL=DEBUG
export DEDUP_MAX_SIZE=1000
export OBSERVATION_TTL_SECONDS=3600  # 1 hour for testing
export GC_INTERVAL=5m
```

### Production ConfigMap Configuration

```yaml
# Base ConfigMap
apiVersion: v1
kind: ConfigMap
metadata:
  name: zen-watcher-base-config
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
    namespace_filtering:
      enabled: true
      excluded_namespaces: ["kube-system", "kube-public"]
```

---

## Best Practices

1. **Use Base + Environment Pattern**: Keep defaults in base ConfigMap, overrides in environment ConfigMaps
2. **Version Control**: Store ConfigMaps in Git for auditability
3. **Gradual Rollout**: Test configuration changes in staging before production
4. **Monitor Metrics**: Watch Prometheus metrics after configuration changes
5. **Documentation**: Document environment-specific configurations
6. **Precedence Awareness**: Understand configuration precedence to avoid conflicts

---

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

### Environment Variable Override

If ConfigMap is not available, environment variables are used as fallback. Check environment variables:

```bash
kubectl get deployment zen-watcher -n zen-system -o yaml | grep -A 20 env:
```

---

## Configuration Type Reference

This section provides a reference for all configuration types used in zen-watcher and their relationships.

### Configuration Type Hierarchy

#### 1. IngesterConfig (`pkg/config/ingester_loader.go`)
**Purpose**: Compiled configuration from an Ingester CRD  
**Scope**: Complete ingester configuration including all source types  
**Fields**:
- Namespace, Name, Source, Ingester
- Informer, Webhook, Logs configs
- Normalization, Filter, Dedup, Processing configs
- Destinations

**Usage**: Primary configuration type loaded from Ingester CRDs

#### 2. SourceConfig (`pkg/adapter/generic/types.go`)
**Purpose**: Generic adapter configuration  
**Scope**: Single source configuration for adapters  
**Fields**:
- Source, Ingester
- Informer, Webhook, Logs configs
- Normalization, Filter, Dedup configs

**Usage**: Used by generic adapters (InformerAdapter, WebhookAdapter, LogsAdapter)

**Relationship**: Converted from `IngesterConfig` via `config.ConvertIngesterConfigToGeneric()`

#### 3. FilterConfig (`pkg/filter/rules.go`)
**Purpose**: Filtering rules configuration  
**Scope**: Filter-specific configuration  
**Fields**:
- Expression, MinPriority
- IncludeNamespaces, ExcludeNamespaces

**Usage**: Used by `Filter` struct (wraps `zen-sdk/pkg/filter`)

**Relationship**: Extracted from `IngesterConfig.Filter`

#### 4. DedupConfig (`pkg/config/ingester_loader.go`)
**Purpose**: Deduplication configuration  
**Scope**: Dedup-specific settings  
**Fields**:
- Enabled, Window, Strategy
- Fields, MaxEventsPerWindow

**Usage**: Used by deduplication logic

**Relationship**: Extracted from `IngesterConfig.Dedup`

### Configuration Flow

```
Ingester CRD (Kubernetes)
    ↓
IngesterConfig (pkg/config)
    ↓
SourceConfig (pkg/adapter/generic)
    ↓
Adapter-specific configs (InformerConfig, WebhookConfig, etc.)
```

### Type Conversion Functions

- `config.ConvertIngesterConfigToGeneric()`: Converts `IngesterConfig` → `SourceConfig`
- `config.extractFilterConfig()`: Extracts `FilterConfig` from spec
- `config.extractDedupConfig()`: Extracts `DedupConfig` from spec

### Best Practices

1. **Use IngesterConfig** when working with CRD loading and conversion
2. **Use SourceConfig** when working with generic adapters
3. **Use specific configs** (FilterConfig, DedupConfig) when working with individual components
4. **Avoid direct field access** - use conversion functions when possible

## Related Documentation

- [Ingester API](INGESTER_API.md) - Ingester CRD configuration
- [Deployment Guide](DEPLOYMENT_HELM.md) - Helm deployment configuration
- [Troubleshooting](TROUBLESHOOTING.md) - Common configuration issues
- [Performance Tuning](PERFORMANCE.md) - Performance optimization guide
- [Scaling Guide](SCALING.md) - Horizontal scaling patterns
