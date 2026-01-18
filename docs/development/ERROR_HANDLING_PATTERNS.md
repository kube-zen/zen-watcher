# Error Handling Patterns

This document standardizes error handling patterns across zen-watcher.

## Error Types

### 1. Structured Errors (`pkg/errors/errors.go`)
**Use for**: Pipeline errors that need categorization  
**Types**: `PipelineError` with categories (CONFIG_ERROR, FILTER_ERROR, etc.)

**Usage**:
```go
import "github.com/kube-zen/zen-watcher/pkg/errors"

return errors.NewConfigError(source, ingester, "INVALID_GVR", "GVR validation failed", err)
```

**When to use**:
- Errors in the processing pipeline
- Errors that need categorization for metrics/logging
- Errors that should be tracked by source/ingester

---

### 2. Standard Errors (`fmt.Errorf`)
**Use for**: General errors with context  
**Pattern**: `fmt.Errorf("operation failed: %w", err)`

**Usage**:
```go
return fmt.Errorf("failed to create resource %s: %w", gvr.Resource, err)
```

**When to use**:
- Simple errors that don't need categorization
- Wrapping external library errors
- Adding context to existing errors

---

### 3. Logged Errors (No Return)
**Use for**: Errors that are logged but don't stop execution  
**Pattern**: Log error and continue

**Usage**:
```go
if err != nil {
    logger.Error(err, "Operation failed but continuing",
        sdklog.Operation("operation_name"),
        sdklog.String("context", "value"))
    // Continue execution
}
```

**When to use**:
- Non-critical errors
- Errors in background operations
- Errors that don't affect main flow

---

## Error Handling Guidelines

### 1. Always Wrap Errors
- Use `%w` verb in `fmt.Errorf` to preserve error chain
- Example: `fmt.Errorf("failed to process: %w", err)`

### 2. Add Context
- Include operation name, source, and relevant identifiers
- Example: `fmt.Errorf("failed to create observation for source %s: %w", source, err)`

### 3. Use Structured Errors for Pipeline
- Use `pkg/errors` types for pipeline errors
- Include source, ingester, and error code

### 4. Log Before Returning
- For critical errors, log before returning
- Use appropriate log level (Error for critical, Warn for recoverable)

### 5. Don't Swallow Errors
- Always handle errors explicitly
- Use `//nolint:errcheck` only when error is intentionally ignored

---

## Error Categories

| Category | Use Case | Example |
|----------|----------|---------|
| CONFIG_ERROR | Configuration loading/validation | Invalid GVR, missing required fields |
| FILTER_ERROR | Filter processing | Expression evaluation failure |
| DEDUP_ERROR | Deduplication | Dedup key extraction failure |
| NORMALIZE_ERROR | Normalization | Field mapping failure |
| CRD_WRITE_ERROR | Resource creation | Kubernetes API errors |
| PIPELINE_ERROR | General pipeline | Unknown processing errors |

---

## Best Practices

1. **Error Wrapping**: Always use `%w` to preserve error chain
2. **Context**: Add relevant context (source, operation, identifiers)
3. **Logging**: Log errors at appropriate level before returning
4. **Structured Errors**: Use `pkg/errors` for pipeline errors
5. **Error Codes**: Use consistent error codes for similar errors
6. **Documentation**: Document error handling in function comments

---

## Migration Guide

When refactoring error handling:

1. Identify error category (if applicable)
2. Replace `fmt.Errorf` with structured error if in pipeline
3. Ensure error wrapping with `%w`
4. Add logging if missing
5. Update function documentation

---

## Examples

### Good: Structured Error
```go
if err := validateConfig(config); err != nil {
    return errors.NewConfigError(
        config.Source,
        config.Ingester,
        "VALIDATION_FAILED",
        "Config validation failed",
        err,
    )
}
```

### Good: Wrapped Error
```go
if err := createResource(resource); err != nil {
    return fmt.Errorf("failed to create resource %s/%s: %w",
        resource.Namespace, resource.Name, err)
}
```

### Good: Logged Error
```go
if err := updateMetrics(); err != nil {
    logger.Warn("Failed to update metrics, continuing",
        sdklog.Operation("update_metrics"),
        sdklog.Error(err))
    // Continue execution
}
```

### Bad: Swallowed Error
```go
createResource(resource) // Error ignored
```

### Bad: Lost Context
```go
return err // No context added
```

