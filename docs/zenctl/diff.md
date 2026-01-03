# zenctl diff

Compare desired YAML manifests with live cluster state to detect GitOps drift.

## Synopsis

```bash
zenctl diff -f <file|dir> [-n namespace|-A] [--ignore-status] [--ignore-annotations]
```

## Description

`zenctl diff` compares desired Kubernetes resource manifests (from file or directory) with the current live state in the cluster. It produces deterministic diff output showing any drift between desired and live state.

This command is designed for GitOps workflows where you want to verify that cluster state matches your version-controlled manifests before applying changes.

## Options

### Required Flags

- `-f, --file <path>`: Path to YAML file or directory containing manifests to compare

### Optional Flags

- `-n, --namespace <namespace>`: Kubernetes namespace (required unless using `-A`)
- `-A, --all-namespaces`: Compare resources across all namespaces
- `--ignore-status`: Ignore status field in diff (status is always removed by default, this flag is for explicit clarity)
- `--ignore-annotations`: Ignore annotations in diff comparisons

### Global Flags

- `--kubeconfig <path>`: Path to kubeconfig file (default: `$KUBECONFIG` or `~/.kube/config`)
- `--context <context>`: Kubernetes context to use

## File and Directory Behavior

### Single File

When `-f` points to a single YAML file:
- Supports multi-document YAML (multiple resources in one file separated by `---`)
- All resources in the file are compared with cluster state

Example:
```bash
zenctl diff -f manifests/flow.yaml -n zen-apps
```

### Directory

When `-f` points to a directory:
- Recursively scans for `.yaml` and `.yml` files
- All YAML files found are processed
- Multi-document YAML files are supported within each file

Example:
```bash
zenctl diff -f manifests/ -n zen-apps
```

## Normalization Rules

To enable accurate drift detection, `zenctl diff` normalizes both desired and live objects by removing runtime metadata that changes on every reconciliation:

**Removed Fields:**
- `status` (runtime state)
- `metadata.resourceVersion`
- `metadata.uid`
- `metadata.managedFields`
- `metadata.creationTimestamp`
- `metadata.generation`
- `metadata.selfLink`
- `metadata.annotations.kubectl.kubernetes.io/last-applied-configuration`

**Preserved Fields:**
- `spec` (desired configuration)
- `metadata.name`
- `metadata.namespace`
- `metadata.labels`
- `metadata.annotations` (unless `--ignore-annotations` is used)

## Secret Redaction

`zenctl diff` applies the same secret redaction patterns as `zenctl export` before generating diff output. Fields matching these patterns are redacted to `[REDACTED]`:

- `token`, `password`, `secret`
- `apiKey`, `api_key`, `accessKey`, `access_key`
- `secretKey`, `secret_key`
- `credentials`, `auth`, `authorization`

Redaction is applied recursively to nested maps and slices.

## Determinism Guarantees

`zenctl diff` uses stable sorting for maps and slices to ensure deterministic output:
- Map keys are sorted alphabetically
- Slices maintain their order (stable sort)
- YAML output is consistent across runs for identical input

This ensures that drift detection is reliable and CI/CD pipelines can use exit codes confidently.

## Exit Codes

| Exit Code | Meaning |
|-----------|---------|
| 0 | No drift detected - desired and live state match |
| 2 | Drift detected - differences found between desired and live |
| 1 | Error - missing CRD, RBAC issue, file I/O error, etc. |

## Interpreting Diff Output

The diff output uses a simple format:
- `--- desired: <resource>` - Shows desired state (from file)
- `+++ live: <resource>` - Shows live state (from cluster)
- `- <line>` - Line present in desired but different/absent in live
- `+ <line>` - Line present in live but different/absent in desired

**Note:** The current implementation uses line-by-line comparison. Enhanced diff output (using proper diff libraries) is planned for future improvements.

## Recommended Workflows

### GitOps Preflight Check

Before applying manifests, verify they match cluster state:

```bash
# Export current state
zenctl export flow my-flow -n zen-apps -f yaml > current-state.yaml

# Compare with desired state
zenctl diff -f desired-state.yaml -n zen-apps
```

If exit code is 0, desired and current state match. If exit code is 2, review the diff to understand changes.

### Cluster Drift Detection

Periodically check if cluster state has drifted from Git:

```bash
# In CI/CD pipeline
zenctl diff -f git-manifests/ -A
if [ $? -eq 2 ]; then
  echo "Drift detected - cluster state differs from Git"
  exit 1
fi
```

### Selective Comparison

Use flags to focus on specific aspects:

```bash
# Ignore annotation changes (common in GitOps tools)
zenctl diff -f manifests/ -n zen-apps --ignore-annotations

# Compare across all namespaces
zenctl diff -f manifests/ -A
```

## Examples

### Compare Single Resource

```bash
$ zenctl diff -f flow.yaml -n zen-apps
No drift detected
$ echo $?
0
```

### Detect Drift

```bash
$ zenctl diff -f flow.yaml -n zen-apps
--- desired: DeliveryFlow/zen-apps/my-flow
+++ live: DeliveryFlow/zen-apps/my-flow
-   replicas: 3
+   replicas: 5
$ echo $?
2
```

### Error Handling

```bash
$ zenctl diff -f flow.yaml -n zen-apps
ERROR: DeliveryFlow CRD not found: ...
$ echo $?
1
```

## Related Commands

- `zenctl export` - Export resources as GitOps YAML/JSON
- `zenctl validate` - Validate cluster configuration and contracts

