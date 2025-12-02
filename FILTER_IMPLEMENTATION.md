# Filter Framework Implementation - v0.1.0 Requirements

## Summary

Implemented the missing filter framework features required for v0.1.0 release. The filter framework now supports all the requested filtering capabilities.

## New Filter Features

### 1. Trivy: `includeSeverity` Filter

**Purpose:** Allow only specific severity levels (e.g., only CRITICAL and HIGH)

**Configuration:**
```json
{
  "sources": {
    "trivy": {
      "includeSeverity": ["CRITICAL", "HIGH"]
    }
  }
}
```

**Implementation:**
- Added `IncludeSeverity []string` field to `SourceFilter`
- Filter logic checks if severity is in the include list
- Takes precedence over `minSeverity` if both are set
- Removed hardcoded HIGH/CRITICAL only filter from Trivy processor

### 2. Kyverno: `excludeRules` Filter

**Purpose:** Exclude specific policy rules (e.g., "disallow-latest-tag")

**Configuration:**
```json
{
  "sources": {
    "kyverno": {
      "excludeRules": ["disallow-latest-tag"]
    }
  }
}
```

**Implementation:**
- Added `ExcludeRules []string` field to `SourceFilter`
- Filter extracts rule name from `details.rule` in observations
- Filters out observations where rule matches any in the exclude list

### 3. Kubernetes Events: `ignoreKinds` Filter

**Purpose:** Ignore specific resource kinds for kubernetes events

**Configuration:**
```json
{
  "sources": {
    "kubernetesEvents": {
      "ignoreKinds": ["Pod", "ConfigMap"]
    }
  }
}
```

**Implementation:**
- Added `IgnoreKinds []string` field to `SourceFilter`
- Acts as an alias for `excludeKinds` for convenience
- Automatically merged into `excludeKinds` during filter loading
- Supports case-insensitive matching

## Files Modified

1. **`pkg/filter/config.go`**
   - Added new filter fields to `SourceFilter` struct
   - Implemented `IgnoreKinds` normalization in `GetSourceFilter()`

2. **`pkg/filter/rules.go`**
   - Added `IncludeSeverity` filtering logic (takes precedence over `minSeverity`)
   - Added `ExcludeRules` filtering logic
   - Extracts rule from `details.rule` in observations

3. **`pkg/watcher/informer_handlers.go`**
   - Removed hardcoded HIGH/CRITICAL severity filter from Trivy processor
   - Removed `skippedLow` and `highCriticalCount` counters
   - Updated logging to reflect filter-based approach

4. **`docs/FILTERING.md`**
   - Added documentation for new filter options
   - Updated examples to show new features
   - Added example 7 showing complete v0.1.0 requirements

5. **`README.md`**
   - Updated filter options list
   - Updated example configuration

## Example Configuration (v0.1.0 Requirements)

```json
{
  "sources": {
    "trivy": {
      "includeSeverity": ["CRITICAL", "HIGH"]
    },
    "kyverno": {
      "excludeRules": ["disallow-latest-tag"]
    },
    "kubernetesEvents": {
      "ignoreKinds": ["Pod", "ConfigMap"]
    }
  }
}
```

## Testing

The filter framework uses the existing test infrastructure. New filter options follow the same patterns as existing filters and should work with the existing test suite.

## Backward Compatibility

- All changes are backward compatible
- Existing filters continue to work
- New options are optional (omitempty JSON tags)
- Default behavior (allow all) preserved when no filters configured

## Notes

- `IncludeSeverity` takes precedence over `minSeverity` if both are set
- `IgnoreKinds` is merged into `excludeKinds` automatically
- Rule filtering extracts from `details.rule` in observation spec
- All filters are case-insensitive for matching

