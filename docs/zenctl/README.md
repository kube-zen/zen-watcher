# zenctl - Operator-grade CLI for Zen Kubernetes Resources

`zenctl` provides a fast, operator-grade CLI for inspecting and managing Zen Kubernetes resources including DeliveryFlows, Destinations, and Ingesters.

## Installation

### Build from source

```bash
cd zen-sdk
make zenctl
```

**Note:** If your workspace Go version mismatches (e.g., `go.work` requires Go 1.25 but your system has Go 1.24), use:

```bash
make zenctl-nowork
```

This disables workspace mode (`GOWORK=off`) and builds zenctl independently.

The binary will be built at `./zenctl` (or `./bin/zenctl` depending on Makefile configuration).

### Using Go install

```bash
go install github.com/kube-zen/zen-sdk/cmd/zenctl@latest
```

## Configuration

`zenctl` uses standard Kubernetes configuration:

- In-cluster: Automatically detected when running inside a Kubernetes cluster
- `$KUBECONFIG`: Environment variable pointing to kubeconfig file
- `~/.kube/config`: Default kubeconfig location

## Global Flags

- `--kubeconfig`: Path to kubeconfig file (overrides $KUBECONFIG)
- `--context`: Kubernetes context to use
- `--namespace, -n`: Kubernetes namespace (defaults to current context namespace)
- `--all-namespaces, -A`: List resources across all namespaces

## Commands

### `zenctl status`

Summarizes DeliveryFlows, Destinations, and Ingesters across namespaces.

```bash
# Status in current namespace
zenctl status

# Status in all namespaces
zenctl status -A

# Status in specific namespace
zenctl status -n zen-apps

# JSON output
zenctl status -o json
```

**Output Format:**
- Table format (default): Human-readable tables for each resource type
- JSON/YAML: Raw resource objects

### `zenctl flows`

Lists DeliveryFlows in table format aligned with ACTIVE_TARGET_UX_GUIDE.md.

```bash
# List flows in current namespace
zenctl flows

# List flows in all namespaces
zenctl flows -A

# List flows in specific namespace
zenctl flows -n zen-apps

# JSON output
zenctl flows -o json
```

**Columns:**
- `NAMESPACE`: Kubernetes namespace
- `NAME`: DeliveryFlow name
- `ACTIVE_TARGET`: Active destination target (namespace/name format)
- `ENTITLEMENT`: Entitlement status (Entitled, Grace Period, Expired, Not Entitled, Unknown)
- `ENTITLEMENT_REASON`: Entitlement reason
- `READY`: Ready condition status (True/False/Unknown)
- `AGE`: Age since creation

### `zenctl explain flow <name>`

Prints detailed information about a specific DeliveryFlow.

```bash
# Explain flow in current namespace
zenctl explain flow my-flow

# Explain flow in specific namespace
zenctl explain flow my-flow -n zen-apps
```

**Output includes:**
- Resolved sourceKey list from `spec.sources`
- Outputs with active target per output
- Last failover timestamp/reason (if present)
- Entitlement condition + reason

**Example Output:**
```
DeliveryFlow: zen-apps/my-flow

Resolved sourceKey list:
  [1] zen-apps/ingester-primary
  [2] zen-apps/ingester-secondary/webhook-source

Outputs:
  [1] output-1:
    Active Target: zen-apps/destination-primary (role: primary)
    Last Failover:
      Reason: HealthCheckFailed
      Time: 2025-01-23T10:30:00Z (5m ago)

Entitlement Condition:
  Status: Entitled
  Last Transition: 2025-01-23T09:00:00Z (1h ago)
```

### `zenctl doctor`

Runs diagnostic checks for common misconfigurations (no network dependency required).

```bash
# Run diagnostics
zenctl doctor
```

**Checks:**
- ✅ CRDs installed: DeliveryFlow, Destination, Ingester
- ✅ Controllers present: zen-ingester, zen-watcher deployments (best-effort)
- ✅ Status subresources exist on CRDs (best-effort via discovery)

**Exit codes:**
- `0`: All checks PASS
- `1`: One or more checks FAIL

**Example Output:**
```
Doctor Diagnostics Results:
==========================

✓ [PASS] CRD: DeliveryFlow
    Found routing.zen.kube-zen.io/v1alpha1/DeliveryFlow

✓ [PASS] CRD: Destination
    Found routing.zen.kube-zen.io/v1alpha1/Destination

✓ [PASS] CRD: Ingester
    Found zen.kube-zen.io/v1alpha1/Ingester

✗ [FAIL] CRD: DeliveryFlow
    CRD not found: no matches for kind "DeliveryFlow"
    Remediation: Enable crds.enabled in Helm chart or apply DeliveryFlow CRD manually
```

## Examples

### Quick status check

```bash
zenctl status -A
```

### List all flows with active targets

```bash
zenctl flows -A
```

### Explain a flow in detail

```bash
zenctl explain flow my-production-flow -n production
```

### Run diagnostics

```bash
zenctl doctor
```

### Using with kubectl context

```bash
zenctl flows --context my-cluster -n my-namespace
```

## Output Formats

All commands support multiple output formats:

- `table` (default): Human-readable tables
- `json`: JSON output (raw Kubernetes objects)
- `yaml`: YAML output (raw Kubernetes objects)

Example:
```bash
zenctl flows -o json | jq '.[] | {name: .metadata.name, activeTarget: .status.outputs[0].activeTarget}'
```

## Error Handling

If a required CRD is missing, `zenctl` will:

1. Print a precise error message:
   ```
   DeliveryFlow CRD not installed; enable crds.enabled or apply CRDs separately
   ```

2. Exit with non-zero exit code

3. For `zenctl status`, missing CRDs are reported as warnings and other resource types are still displayed

## Security

**Important:** `zenctl` never prints secrets or sensitive data. Only status fields and non-sensitive spec fields are displayed.

## Integration with kubectl

`zenctl` is designed to complement `kubectl`, providing:

- Faster status queries (pre-formatted output)
- Cross-namespace aggregation
- Operator-focused information (active targets, entitlement status)
- Human-readable labels (Entitled, Grace Period, etc.)

Use `kubectl` for:
- Editing resources
- Creating/deleting resources
- Raw YAML/JSON inspection
- Other Kubernetes operations

Use `zenctl` for:
- Quick status overview
- Troubleshooting flows
- Checking system health (`zenctl doctor`)

## Troubleshooting

### "CRD not installed" error

If you see:
```
DeliveryFlow CRD not installed; enable crds.enabled or apply CRDs separately
```

**Solution:**
1. Enable CRDs in Helm chart: `helm upgrade ... --set crds.enabled=true`
2. Or apply CRDs manually: `kubectl apply -f path/to/crds/`

### "failed to create client" error

If you see:
```
failed to create client: failed to build config from kubeconfig
```

**Solution:**
1. Check kubeconfig path: `echo $KUBECONFIG`
2. Verify context: `kubectl config get-contexts`
3. Use `--kubeconfig` flag: `zenctl flows --kubeconfig /path/to/config`

### No resources found

If commands return empty results:

1. Check namespace: `zenctl flows -A` to list all namespaces
2. Verify CRDs are installed: `zenctl doctor`
3. Check with kubectl: `kubectl get deliveryflows -A`

## See Also

- [API Reference](../API_REFERENCE.md)
- [ACTIVE_TARGET_UX_GUIDE.md](../../zen-platform/docs/ACTIVE_TARGET_UX_GUIDE.md)

