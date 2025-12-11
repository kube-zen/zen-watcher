# Ingester Tooling Guide

This guide covers the command-line tools available for working with Ingester CRDs.

## Tools

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
- No filters on high-rate sources (e.g., `k8s-events`)
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
  [ERROR] NO_FILTERS_HIGH_RATE: Source 'k8s-events' is known for high event rates but has no filters configured (field: spec.filters)
  [WARNING] WIDE_MATCHER: Informer has no namespace, label, or field selectors - will watch all resources (field: spec.informer)
  [INFO] NO_PRIORITY_MAPPING: No priority mapping configured - events may get default priority (field: spec.destinations[0].mapping.priority)
  Summary: 3 total (1 errors, 1 warnings, 1 infos)
```

## CI Integration

### Pre-commit Hook

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

### GitHub Actions

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
          go-version: '1.23'
      - name: Build linter
        run: |
          cd zen-watcher
          go build -o ingester-lint ./cmd/ingester-lint
      - name: Lint Ingesters
        run: |
          cd zen-watcher
          ./ingester-lint -fail-on warning examples/ingesters/*.yaml
```

### GitLab CI

```yaml
lint-ingesters:
  stage: test
  script:
    - cd zen-watcher
    - go build -o ingester-lint ./cmd/ingester-lint
    - ./ingester-lint -fail-on warning examples/ingesters/*.yaml
```

## Best Practices

1. **Lint before committing**: Use pre-commit hooks to catch issues early
2. **Lint in CI**: Ensure all Ingester specs are linted in CI pipelines
3. **Fail on warnings in CI**: Use `-fail-on warning` in CI to catch potential issues
4. **Review info-level issues**: While not blocking, info-level issues indicate missing best practices

## Related Documentation

- [INGESTER_API.md](INGESTER_API.md) - Complete Ingester CRD API reference
- [INGESTER_MIGRATION_GUIDE.md](INGESTER_MIGRATION_GUIDE.md) - Migration from v1alpha1 to v1
- [CRD_CONFORMANCE.md](CRD_CONFORMANCE.md) - CRD validation and conformance

