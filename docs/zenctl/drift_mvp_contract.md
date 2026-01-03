# zenctl diff MVP Contract (Beta)

**Version:** 1.0 (Beta)  
**Last Updated:** 2025-01-15  
**Status:** Beta / MVP

## Overview

This document defines the contract for `zenctl diff` in MVP/beta form. This contract is the source of truth for operators, automation, and consumers of drift reports.

## Supported Commands and Flags

### Command

```
zenctl diff -f <file|dir> [flags]
```

### Flags

| Flag | Type | Description |
|------|------|-------------|
| `-f, --file` | string (required) | Path to YAML file or directory containing desired manifests |
| `--select` | string array (repeatable) | Select resources by pattern (Kind/name, Kind/namespace/name, Group/Kind/name, Group/Kind/namespace/name) |
| `--label-selector` | string | Kubernetes label selector (e.g., `app=frontend`) |
| `--report` | string | Report format: `json` (only supported value in MVP) |
| `--report-file` | string | Path to write JSON report (atomic write; if set, JSON goes to file, human output to stdout) |
| `--quiet` | bool | Suppress human output when JSON report is enabled (JSON-only workflows) |
| `--ignore-status` | bool | Ignore `status` field in diff |
| `--ignore-annotations` | bool | Ignore annotations in diff |
| `--format` | string | Diff output format: `unified` (default) or `plain` |
| `--exclude` | string array (repeatable) | Exclude patterns (gitignore-style) |
| `-n, --namespace` | string | Kubernetes namespace (inherited from global flags) |
| `-A, --all-namespaces` | bool | All namespaces (inherited from global flags) |
| `--context` | string | Kubernetes context (inherited from global flags) |

## Output Modes

### Human Output (Default)

- Unified diff format (default) or plain format
- Drift summary header (when not in JSON-only mode)
- Resource-by-resource diff output
- Error messages to stderr
- Warnings to stderr

### JSON Report Mode

Activated with `--report json`:

- If `--report-file` is set: JSON written to file (atomic), human output to stdout (unless `--quiet`)
- If `--report-file` is not set: JSON to stdout, human output suppressed (or to stderr if `--quiet` not set)
- If `--quiet` is set: Human output completely suppressed

**JSON Report Schema:** See `zen-admin/docs/pm/integrations/drift_json_report.md`

## Exit Codes

| Code | Semantics | Notes |
|------|-----------|-------|
| `0` | No drift detected | All resources match desired state (within selected scope if filters applied) |
| `2` | Drift detected | At least one resource differs from desired state |
| `1` | Error | CRD missing, RBAC error, resource not found, or all select patterns matched no resources |

**Important:** JSON report is emitted even on exit codes 1 and 2 (with partial results if errors occurred).

## Redaction Rules

### Guaranteed Redaction

1. **Secrets (v1/Secret):**
   - `.data` and `.stringData` fields are never included in any output
   - `redacted=true` flag set in JSON report
   - Human diff shows `[REDACTED]` placeholders

2. **Sensitive Patterns:**
   - Patterns like `password`, `token`, `apikey`, `secret` are redacted in ConfigMaps (best-effort)
   - JSON report never contains raw sensitive values

### What Can Be Emitted

- Resource metadata (name, namespace, kind, API version)
- Spec fields (except Secrets)
- Status fields (unless `--ignore-status` is set)
- Diff statistics (added/removed/changed line counts)
- Error messages (remediation-oriented, no sensitive data)

## Determinism Guarantees

### Stable Output

- **Resource ordering:** Canonical sort by Group → Kind → Namespace → Name
- **Diff output:** Deterministic (normalized objects, stable sorting)
- **JSON report:** Stable ordering of `resources[]` array
- **Exit codes:** Deterministic based on resource state

### What Varies

- **`generatedAt` timestamp:** RFC3339 format, varies per run (tests normalize this)
- **Resource metadata:** Server-managed fields (resourceVersion, uid, creationTimestamp, etc.) are normalized out

### Filtering Determinism

- Select patterns are normalized and sorted before application
- Label selector is applied deterministically (offline, against desired manifests)
- Filter order: exclude → select → label-selector

## Error Handling

### Actionable Errors

All errors include remediation:

- CRD missing: `"DeliveryFlow CRD not installed; enable crds.enabled or apply CRDs separately"`
- Resource not found: `"Resource not found: <kind>/<namespace>/<name>"`
- Parse errors: `"Failed to parse select pattern: <pattern> (expected Kind/name, ...)"`
- Label selector errors: `"Invalid label selector: <error>"`

### Partial Results

- If some resources succeed and others fail, JSON report includes:
  - Successful resources with status `no_drift` or `drift`
  - Failed resources with status `error` and `error` field populated
- Exit code is 1 if any errors occurred, even if drift was also detected

## Backward Compatibility

- Schema version: `1.0` (additive-only changes)
- New JSON fields are optional (`omitempty`)
- Existing consumers ignore unknown fields
- Breaking changes require schema version bump

## Limitations (MVP/Beta)

1. **No SBOM/provenance:** Version metadata only (git SHA, build date)
2. **No advanced filtering:** Only select patterns and label selector
3. **No report versioning:** Schema v1.0 only
4. **No streaming:** Full report generated in memory
5. **No parallel processing:** Resources processed sequentially

## Acceptance Criteria

For MVP/beta, the following must hold:

1. ✅ Exit codes are deterministic (0/2/1)
2. ✅ JSON reports are deterministic (except `generatedAt`)
3. ✅ No secrets are emitted in any output
4. ✅ Errors are actionable (remediation included)
5. ✅ Filtering is deterministic (stable ordering)
6. ✅ Human output is readable (unified diff format)
7. ✅ JSON reports are parseable (valid JSON, stable schema)

## References

- JSON Report Schema: `zen-admin/docs/pm/integrations/drift_json_report.md`
- Consumer Guide: `zen-admin/docs/pm/integrations/drift_json_consumer_mvp.md`
- Implementation: `zen-watcher/cmd/zenctl/internal/commands/diff.go`

