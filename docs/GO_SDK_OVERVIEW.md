# Go SDK Overview

The zen-watcher Go SDK provides strongly-typed Go structs and helpers for working with Ingester and Observation CRDs.

## Installation

The SDK is part of the zen-watcher module:

```go
import "github.com/kube-zen/zen-watcher/pkg/sdk"
```

## Features

- **Strongly-typed structs**: Go types matching Ingester v1 and Observation CRD schemas
- **Offline validation**: Validate specs without Kubernetes API access
- **Example builders**: Programmatically generate canonical Ingester examples
- **YAML round-trip**: Full support for YAML marshaling/unmarshaling

## Usage Examples

### Creating an Ingester

```go
import (
    "github.com/kube-zen/zen-watcher/pkg/sdk"
    "gopkg.in/yaml.v3"
)

// Create a Trivy Ingester
ingester := sdk.NewTrivyIngester("default", "trivy-informer")

// Validate it
if err := sdk.ValidateIngester(ingester); err != nil {
    log.Fatalf("Invalid ingester: %v", err)
}

// Marshal to YAML
yamlData, err := yaml.Marshal(ingester)
if err != nil {
    log.Fatalf("Failed to marshal: %v", err)
}
fmt.Println(string(yamlData))
```

### Validating an Ingester

```go
// Parse YAML
var ingester sdk.Ingester
if err := yaml.Unmarshal(yamlData, &ingester); err != nil {
    log.Fatalf("Failed to parse: %v", err)
}

// Validate
if err := sdk.ValidateIngester(&ingester); err != nil {
    if ve, ok := err.(*sdk.ValidationError); ok {
        fmt.Printf("Validation error in %s: %s\n", ve.Field, ve.Message)
    }
}
```

### Creating an Observation

```go
obs := &sdk.Observation{
    APIVersion: "zen.kube-zen.io/v1",
    Kind:       "Observation",
    Metadata: metav1.ObjectMeta{
        Name:      "test-obs",
        Namespace: "default",
    },
    Spec: sdk.ObservationSpec{
        Source:    "trivy",
        Category:  "security",
        Severity:  "high",
        EventType: "vulnerability",
        Resource: &sdk.ResourceRef{
            Kind: "Pod",
            Name: "test-pod",
        },
    },
}

if err := sdk.ValidateObservation(obs); err != nil {
    log.Fatalf("Invalid observation: %v", err)
}
```

### Example Builders

The SDK provides builders for common Ingester types:

```go
// Trivy
trivyIngester := sdk.NewTrivyIngester("default", "trivy-informer")

// Kyverno
kyvernoIngester := sdk.NewKyvernoIngester("default", "kyverno-informer")

// Kube-bench
kubeBenchIngester := sdk.NewKubeBenchIngester("default", "kube-bench-informer")

// Kubernetes Events
k8sEventsIngester := sdk.NewK8sEventsIngester("default", "k8s-events-informer")
```

## Use Cases

### Writing a CI Linter

```go
// Read Ingester YAML from file
yamlData, _ := os.ReadFile("ingester.yaml")
var ingester sdk.Ingester
yaml.Unmarshal(yamlData, &ingester)

// Validate
if err := sdk.ValidateIngester(&ingester); err != nil {
    fmt.Fprintf(os.Stderr, "Validation failed: %v\n", err)
    os.Exit(1)
}
```

### Generating Ingesters Programmatically

```go
// Generate Ingesters for multiple sources
sources := []string{"trivy", "kyverno", "kube-bench"}
for _, source := range sources {
    var ingester *sdk.Ingester
    switch source {
    case "trivy":
        ingester = sdk.NewTrivyIngester("default", source+"-informer")
    case "kyverno":
        ingester = sdk.NewKyvernoIngester("default", source+"-informer")
    case "kube-bench":
        ingester = sdk.NewKubeBenchIngester("default", source+"-informer")
    }
    
    yamlData, _ := yaml.Marshal(ingester)
    os.WriteFile(source+".yaml", yamlData, 0644)
}
```

## Validation

The SDK provides offline validation that matches CRD schema validation:

- **Required fields**: Validates all required fields are present
- **Patterns**: Validates field patterns (e.g., `source` must match `^[a-z0-9-]+$`)
- **Enums**: Validates enum values (e.g., `ingester` must be one of: informer, webhook, logs, k8s-events)
- **Ranges**: Validates numeric ranges (e.g., `minPriority` must be 0.0-1.0)

## Type Alignment

All SDK types are aligned with CRD schemas:

- **Field names**: Match CRD JSON paths
- **JSON tags**: Match CRD field names
- **Types**: Compatible with CRD OpenAPI schema

## Related Documentation

- [INGESTER_API.md](INGESTER_API.md) - Complete Ingester CRD API reference
- [INGESTER_MIGRATION_GUIDE.md](INGESTER_MIGRATION_GUIDE.md) - Migration from v1alpha1 to v1
- [INGESTER_TOOLING.md](INGESTER_TOOLING.md) - Command-line tools for Ingesters

