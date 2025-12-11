# Tooling Overview

zen-watcher provides a suite of CLI tools for operators and developers.

## Tools

### ingester-migrate

**Purpose**: Migrate Ingester specs from v1alpha1 to v1

**Usage**:
```bash
# Migrate a single file
ingester-migrate -input ingester.yaml -output ingester-v1.yaml

# Migrate directory
ingester-migrate -input-dir ./ingesters -output-dir ./ingesters-v1
```

**See**: [INGESTER_MIGRATION_GUIDE.md](INGESTER_MIGRATION_GUIDE.md)

### ingester-lint

**Purpose**: Validate Ingester specs for safety and correctness

**Usage**:
```bash
# Lint a single file
ingester-lint ingester.yaml

# Lint directory
ingester-lint -dir ./ingesters

# Exit code indicates severity (0=pass, 1=warnings, 2=errors)
```

**See**: [INGESTER_TOOLING.md](INGESTER_TOOLING.md)

### obsctl

**Purpose**: Query Observations from the cluster

**Usage**:
```bash
# List Observations
obsctl list --namespace zen-system --context my-cluster

# Show statistics
obsctl stats --group-by 'source,severity' --namespace zen-system

# Get specific Observation
obsctl get observation-name --namespace zen-system
```

**See**: [OBSCTL_CLI_GUIDE.md](OBSCTL_CLI_GUIDE.md)

### schema-doc-gen

**Purpose**: Generate schema documentation from CRDs

**Usage**:
```bash
# Generate documentation
go run ./cmd/schema-doc-gen

# Outputs to docs/generated/
```

**See**: [CONTRIBUTING.md](../CONTRIBUTING.md) - Run after modifying CRDs

## When to Use Which Tool

| Task | Tool | When |
|------|------|------|
| Migrate Ingester specs | `ingester-migrate` | Upgrading from v1alpha1 to v1 |
| Validate Ingester config | `ingester-lint` | Before applying Ingesters, in CI/CD |
| Query Observations | `obsctl` | Debugging, monitoring, validation |
| Generate docs | `schema-doc-gen` | After modifying CRDs or types |

## Recommended Pipeline

For authoring and deploying Ingesters:

```bash
# 1. Author spec
vim my-ingester.yaml

# 2. Lint for safety
ingester-lint my-ingester.yaml

# 3. Migrate if needed (v1alpha1 → v1)
ingester-migrate -input my-ingester.yaml -output my-ingester-v1.yaml

# 4. Apply via GitOps or kubectl
kubectl apply -f my-ingester-v1.yaml --namespace zen-system --context my-cluster

# 5. Verify Observations
obsctl list --namespace zen-system --context my-cluster
```

## CI/CD Integration

### Pre-commit Hook

```bash
#!/bin/bash
# .githooks/pre-commit

# Lint all Ingester YAMLs
find . -name "*.yaml" -path "*/ingesters/*" | xargs ingester-lint
```

### CI Pipeline

```yaml
# Example GitHub Actions
- name: Lint Ingesters
  run: |
    make install-tools
    ingester-lint -dir ./examples/ingesters

- name: Validate schema docs
  run: |
    go run ./cmd/schema-doc-gen
    git diff --exit-code docs/generated/
```

## Related Documentation

- [Ingester API](INGESTER_API.md) - Complete Ingester CRD reference
- [Ingester Migration Guide](INGESTER_MIGRATION_GUIDE.md) - v1alpha1 → v1 migration
- [Ingester Tooling](INGESTER_TOOLING.md) - Linter and tooling details
- [obsctl CLI Guide](OBSCTL_CLI_GUIDE.md) - Querying Observations

