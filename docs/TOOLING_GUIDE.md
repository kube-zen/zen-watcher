# Tooling Guide

Complete guide to all zen-watcher CLI tools for operators and developers.

---

## Ingester Tools

### ingester-migrate

Migrates Ingester CRDs from v1alpha1 to v1 format.

**See**: [INGESTER_MIGRATION_GUIDE.md](INGESTER_MIGRATION_GUIDE.md) for complete migration guide.

### ingester-lint

Lints Ingester CRD specs for common issues and best practices.

#### Installation

```bash
cd zen-watcher
go build -o ingester-lint ./cmd/ingester-lint
```

#### Usage

**Basic linting:**
```bash
# Lint single file
./ingester-lint ingester.yaml

# Lint multiple files
./ingester-lint ingester1.yaml ingester2.yaml

# Output JSON
./ingester-lint -json ingester.yaml

# Fail on warnings (not just errors)
./ingester-lint -fail-on warning ingester.yaml

# Lint directory
./ingester-lint -dir ./ingesters
```

#### Exit Codes

- **0**: Clean or only info-level issues
- **1**: Warnings or errors found (configurable via `-fail-on`)

#### Lint Checks

**ERROR level:**
- Missing required fields (`source`, `ingester`, `destinations`)
- Invalid destination type (only `crd` supported in v1)
- Missing destination value
- Invalid destination value pattern
- No filters on high-rate sources (e.g., `kubernetes-events`)
- No deduplication on duplicate-prone sources (e.g., `cert-manager`)

**WARNING level:**
- Wide matchers (no namespace/label/field selectors)

**INFO level:**
- Missing priority mapping
- Missing severity mapping
- Missing field mappings

#### Example Output

```
ingester.yaml:
  [ERROR] NO_FILTERS_HIGH_RATE: Source 'kubernetes-events' is known for high event rates but has no filters configured (field: spec.filters)
  [WARNING] WIDE_MATCHER: Informer has no namespace, label, or field selectors - will watch all resources (field: spec.informer)
  [INFO] NO_PRIORITY_MAPPING: No priority mapping configured - events may get default priority (field: spec.destinations[0].mapping.priority)
  Summary: 3 total (1 errors, 1 warnings, 1 infos)
```

#### CI Integration

**Pre-commit Hook:**

Create `.git/hooks/pre-commit`:

```bash
#!/bin/sh
# Lint Ingester YAML files before commit
ingester-lint $(git diff --cached --name-only --diff-filter=ACM | grep -E '\.(yaml|yml)$')
if [ $? -ne 0 ]; then
    echo "Ingester linting failed. Fix issues before committing."
    exit 1
fi
```

**GitHub Actions:**

```yaml
name: Lint Ingesters

on: [push, pull_request]

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.24'
      - name: Build linter
        run: |
          cd zen-watcher
          go build -o ingester-lint ./cmd/ingester-lint
      - name: Lint Ingesters
        run: |
          cd zen-watcher
          ./ingester-lint -fail-on warning examples/ingesters/*.yaml
```

**GitLab CI:**

```yaml
lint-ingesters:
  stage: test
  script:
    - cd zen-watcher
    - go build -o ingester-lint ./cmd/ingester-lint
    - ./ingester-lint -fail-on warning examples/ingesters/*.yaml
```

#### Best Practices

1. **Lint before committing**: Use pre-commit hooks to catch issues early
2. **Lint in CI**: Ensure all Ingester specs are linted in CI pipelines
3. **Fail on warnings in CI**: Use `-fail-on warning` in CI to catch potential issues
4. **Review info-level issues**: While not blocking, info-level issues indicate missing best practices

---

## Observation Tools

### obsctl

`obsctl` is a cluster-local CLI for querying zen-watcher Observations without any SaaS or external tools.

#### Installation

Build from source:

```bash
cd zen-watcher
go build -o obsctl ./cmd/obsctl
```

#### Usage

**List Observations:**

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

**Show Statistics:**

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

**Get Observation:**

Get a specific Observation:

```bash
# Get by name
obsctl get trivy-vuln-abc123 --namespace default --context my-cluster

# JSON output
obsctl get trivy-vuln-abc123 --namespace default --output json
```

#### Safety & Kubernetes Operations

**Important**: `obsctl` respects Kubernetes Operations Guardrails:

- **No context manipulation**: Always use explicit `--context` flag
- **Explicit namespace**: Use `--namespace` flag or specify in command
- **Recommended**: Run from inside cluster (e.g., debug pod) or with explicit kubeconfig

**Running from Inside Cluster:**

```bash
# Create a debug pod
kubectl run obsctl-debug --image=your-registry/obsctl:latest --rm -it -- /bin/sh

# Run obsctl from inside pod
obsctl list --namespace default
```

**Running with Explicit Kubeconfig:**

```bash
# Use specific kubeconfig and context
obsctl list --kubeconfig ~/.kube/config --context my-cluster --namespace default
```

#### Example Queries

**Find High Severity Security Observations:**

```bash
obsctl list --selector 'zen.io/category=security,zen.io/priority=high' --namespace default
```

**Count Observations by Source:**

```bash
obsctl stats --group-by 'source' --namespace default
```

**Get All Trivy Observations:**

```bash
obsctl list --selector 'zen.io/source=trivy' --namespace default
```

#### Output Formats

**Table (default):**

Human-readable table format for quick viewing.

**JSON:**

Structured JSON output for scripting:

```bash
obsctl list --output json | jq '.items[] | {name: .metadata.name, source: .spec.source}'
```

---

## Schema Tools

### schema-doc-gen

**Purpose**: Generate schema documentation from CRDs

**Usage**:
```bash
# Generate documentation
go run ./cmd/schema-doc-gen

# Outputs to docs/generated/
```

**See**: [CONTRIBUTING.md](../CONTRIBUTING.md) - Run after modifying CRDs

---

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

---

## Related Documentation

- [TOOLING_OVERVIEW.md](TOOLING_OVERVIEW.md) - Quick reference and tool selection guide
- [INGESTER_API.md](INGESTER_API.md) - Complete Ingester CRD API reference
- [INGESTER_MIGRATION_GUIDE.md](INGESTER_MIGRATION_GUIDE.md) - Migration from v1alpha1 to v1
- [GO_SDK_OVERVIEW.md](GO_SDK_OVERVIEW.md) - Go SDK for programmatic Ingester creation
- [CRD_CONFORMANCE.md](CRD_CONFORMANCE.md) - CRD validation and conformance
- [OBSERVABILITY.md](OBSERVABILITY.md) - Metrics and observability guide
- [TROUBLESHOOTING.md](TROUBLESHOOTING.md) - Troubleshooting guide
- [OBSERVATION_API_PUBLIC_GUIDE.md](OBSERVATION_API_PUBLIC_GUIDE.md) - Complete Observation CRD API reference

