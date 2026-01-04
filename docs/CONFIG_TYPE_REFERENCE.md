# Configuration Type Reference

This document provides a clear reference for all configuration types used in zen-watcher and their relationships.

## Configuration Type Hierarchy

### 1. IngesterConfig (`pkg/config/ingester_loader.go`)
**Purpose**: Compiled configuration from an Ingester CRD  
**Scope**: Complete ingester configuration including all source types  
**Fields**:
- Namespace, Name, Source, Ingester
- Informer, Webhook, Logs configs
- Normalization, Filter, Dedup, Processing, Optimization configs
- Destinations

**Usage**: Primary configuration type loaded from Ingester CRDs

---

### 2. SourceConfig (`pkg/adapter/generic/types.go`)
**Purpose**: Generic adapter configuration  
**Scope**: Single source configuration for adapters  
**Fields**:
- Source, Ingester
- Informer, Webhook, Logs configs
- Normalization, Filter, Dedup configs

**Usage**: Used by generic adapters (InformerAdapter, WebhookAdapter, LogsAdapter)

**Relationship**: Converted from `IngesterConfig` via `config.ConvertIngesterConfigToGeneric()`

---

### 3. FilterConfig (`pkg/filter/rules.go`)
**Purpose**: Filtering rules configuration  
**Scope**: Filter-specific configuration  
**Fields**:
- Expression, MinPriority
- IncludeNamespaces, ExcludeNamespaces

**Usage**: Used by `Filter` struct (wraps `zen-sdk/pkg/filter`)

**Relationship**: Extracted from `IngesterConfig.Filter`

---

### 4. DedupConfig (`pkg/config/ingester_loader.go`)
**Purpose**: Deduplication configuration  
**Scope**: Dedup-specific settings  
**Fields**:
- Enabled, Window, Strategy
- Fields, MaxEventsPerWindow

**Usage**: Used by deduplication logic

**Relationship**: Extracted from `IngesterConfig.Dedup`

---

## Configuration Flow

```
Ingester CRD (Kubernetes)
    ↓
IngesterConfig (pkg/config)
    ↓
SourceConfig (pkg/adapter/generic)
    ↓
Adapter-specific configs (InformerConfig, WebhookConfig, etc.)
```

## Type Conversion Functions

- `config.ConvertIngesterConfigToGeneric()`: Converts `IngesterConfig` → `SourceConfig`
- `config.extractFilterConfig()`: Extracts `FilterConfig` from spec
- `config.extractDedupConfig()`: Extracts `DedupConfig` from spec

## Best Practices

1. **Use IngesterConfig** when working with CRD loading and conversion
2. **Use SourceConfig** when working with generic adapters
3. **Use specific configs** (FilterConfig, DedupConfig) when working with individual components
4. **Avoid direct field access** - use conversion functions when possible

## Future Improvements

- Consider consolidating overlapping fields between `IngesterConfig` and `SourceConfig`
- Document clear boundaries between config types
- Ensure consistent naming conventions across config types

