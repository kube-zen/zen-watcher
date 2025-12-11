# obsctl CLI Guide

`obsctl` is a cluster-local CLI for querying zen-watcher Observations without any SaaS or external tools.

## Installation

Build from source:

```bash
cd zen-watcher
go build -o obsctl ./cmd/obsctl
```

## Usage

### List Observations

List Observations with optional selector:

```bash
# List all Observations in default namespace
obsctl list --namespace default --context my-cluster

# List with label selector
obsctl list --selector 'zen.io/source=trivy,zen.io/priority=high' --namespace default

# List all Observations (all namespaces)
obsctl list
```

**Output format:**
```
NAME                SOURCE    CATEGORY    SEVERITY    EVENT TYPE        AGE
trivy-vuln-abc123   trivy     security    high        vulnerability     1h
```

### Show Statistics

Show statistics grouped by source and severity:

```bash
# Stats for default namespace
obsctl stats --group-by 'source,severity' --namespace default

# Stats for all namespaces
obsctl stats --group-by 'source,severity'
```

**Output format:**
```
SOURCE      SEVERITY    COUNT
trivy       high        42
trivy       medium      15
kyverno     high        8
```

### Get Observation

Get a specific Observation:

```bash
# Get by name
obsctl get trivy-vuln-abc123 --namespace default --context my-cluster

# JSON output
obsctl get trivy-vuln-abc123 --namespace default --output json
```

## Safety & Kubernetes Operations

**Important**: `obsctl` respects Kubernetes Operations Guardrails:

- **No context manipulation**: Always use explicit `--context` flag
- **Explicit namespace**: Use `--namespace` flag or specify in command
- **Recommended**: Run from inside cluster (e.g., debug pod) or with explicit kubeconfig

### Running from Inside Cluster

```bash
# Create a debug pod
kubectl run obsctl-debug --image=your-registry/obsctl:latest --rm -it -- /bin/sh

# Run obsctl from inside pod
obsctl list --namespace default
```

### Running with Explicit Kubeconfig

```bash
# Use specific kubeconfig and context
obsctl list --kubeconfig ~/.kube/config --context my-cluster --namespace default
```

## Example Queries

### Find High Severity Security Observations

```bash
obsctl list --selector 'zen.io/category=security,zen.io/priority=high' --namespace default
```

### Count Observations by Source

```bash
obsctl stats --group-by 'source' --namespace default
```

### Get All Trivy Observations

```bash
obsctl list --selector 'zen.io/source=trivy' --namespace default
```

## Output Formats

### Table (default)

Human-readable table format for quick viewing.

### JSON

Structured JSON output for scripting:

```bash
obsctl list --output json | jq '.items[] | {name: .metadata.name, source: .spec.source}'
```

## Related Documentation

- [OBSERVABILITY.md](OBSERVABILITY.md) - Metrics and observability guide
- [TROUBLESHOOTING.md](TROUBLESHOOTING.md) - Troubleshooting guide
- [Observation API Public Guide](OBSERVATION_API_PUBLIC_GUIDE.md) - Complete Observation CRD API reference

