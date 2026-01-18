# Plugins and Hooks

zen-watcher supports compile-time hooks that can extend the pipeline without modifying core code.

## Overview

Hooks are called after normalization but before Observations are written to CRDs. They allow you to:
- Add labels based on severity thresholds
- Enrich Observations with static metadata
- Apply custom transformations

**Important**: Hooks are compile-time registered, in-process, and must not perform network I/O.

## Hook Interface

```go
type ObservationHook interface {
    Process(ctx context.Context, obs *unstructured.Unstructured) error
}
```

## Creating a Hook

### 1. Implement the Interface

```go
package main

import (
    "context"
    "github.com/kube-zen/zen-watcher/pkg/hooks"
    "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type MyHook struct{}

func (h *MyHook) Process(ctx context.Context, obs *unstructured.Unstructured) error {
    // Modify Observation
    labels := obs.GetLabels()
    if labels == nil {
        labels = make(map[string]string)
    }
    labels["custom-label"] = "value"
    obs.SetLabels(labels)
    return nil
}
```

### 2. Register the Hook

```go
func init() {
    hooks.RegisterHook(&MyHook{})
}
```

### 3. Build with Your Hook

```bash
go build -o zen-watcher ./cmd/watcher
```

## Example Hooks

See `examples/hooks/` for example implementations:

- **SeverityLabelHook**: Adds labels based on severity thresholds
- **StaticMetadataHook**: Enriches Observations with static metadata from environment

## Constraints

### Performance

Hooks must be fast (<10ms per Observation). Slow hooks will impact pipeline throughput.

### No Blocking Operations

Hooks must not:
- Make network calls
- Perform file I/O (except for configuration)
- Block on external services

### Idempotent

Hooks should be idempotent (safe to re-run). If a hook is called multiple times with the same Observation, it should produce the same result.

### Error Handling

Hooks should return errors for fatal issues only. If a hook returns an error, the Observation will not be written.

## Wiring Hooks into Pipeline

Hooks are automatically executed by the pipeline after normalization. No additional configuration is required.

## Testing Hooks

Test hooks in isolation:

```go
func TestMyHook(t *testing.T) {
    hook := &MyHook{}
    obs := createTestObservation()
    
    err := hook.Process(context.Background(), obs)
    if err != nil {
        t.Fatalf("Hook failed: %v", err)
    }
    
    // Verify modifications
    labels := obs.GetLabels()
    if labels["custom-label"] != "value" {
        t.Error("Label not set correctly")
    }
}
```

## Related Documentation

- [Observation API Public Guide](OBSERVATION_API_PUBLIC_GUIDE.md) - Complete Observation CRD API reference

