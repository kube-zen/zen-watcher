# Configuration Guide

This document describes all configuration options for zen-watcher, including environment variables, ConfigMap settings, and Ingester CRD configuration.

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

3. **Environment Variables**
   - Default TTL
   - GC settings
   - API group override

4. **Built-in Defaults** (lowest priority)
   - Hardcoded defaults in code
   - See `pkg/config/constants.go`

## Examples

### Custom API Group Deployment

```bash
# Deploy with custom API group
export ZEN_API_GROUP="observations.k8s.io"
helm install zen-watcher ./deployments/helm/zen-watcher \
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

## Related Documentation

- [Ingester API](INGESTER_API.md) - Ingester CRD configuration
- [Deployment Guide](DEPLOYMENT_HELM.md) - Helm deployment configuration
- [Troubleshooting](TROUBLESHOOTING.md) - Common configuration issues

