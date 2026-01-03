# zenctl diff JSON Report Implementation Plan

**Tranche:** F1 - Core JSON Drift Report  
**Date:** 2025-01-27  
**Status:** Implementation Plan (Pre-Implementation)

## Current Flow Map

### Current diff.go Flow

```
Input: --file <path>, --namespace, --context
  ↓
1. Load manifests from file/dir (loadManifests)
   - Parse YAML (multi-doc support)
   - Apply .zenignore and --exclude patterns
   ↓
2. For each desired object:
   a. Parse APIVersion → GroupVersionKind
   b. Resolve GVR via discovery client
   c. Fetch live object from cluster (Get)
   d. Normalize both (normalizeForDiff):
      - Remove status, resourceVersion, uid, managedFields, etc.
      - Redact secrets (redactSecrets)
      - Stable sort (stableSortObject)
   e. Generate diff (generateDiff):
      - Marshal to YAML
      - Compute unified/plain diff
      - Classify driftType (spec vs metadata)
   ↓
3. Collect results:
   - drifts []string (diff strings)
   - errors []string (error messages)
   - driftSummary struct (counts)
   ↓
4. Output and exit:
   - If errors: print to stderr, return error (exit 1)
   - If drifts: print summary + diffs, os.Exit(2)
   - If no drifts: print "No drift detected", return nil (exit 0)
```

## Insertion Points for Report Collection

### Point 1: ResourceReport Creation

**Location:** In the loop processing each desired object (step 2e, after generateDiff)

**Code Pattern:**
```go
// After generating diff
diff, driftType := generateDiff(...)
resourceReport := ResourceReport{
    Group:     gvk.Group,
    Version:   gvk.Version,
    Kind:      gvk.Kind,
    Namespace: objNS,
    Name:      desired.GetName(),
    Status:    "no_drift", // or "drift" or "error"
    DriftType: driftType,  // from generateDiff
    DiffStats: calculateDiffStats(diff),
    Redacted:  isSecretOrHasSecrets(desired, live),
}

// Determine status
if diff != "" {
    resourceReport.Status = "drift"
} else {
    resourceReport.Status = "no_drift"
}
```

### Point 2: Error Entry Creation

**Location:** When GVR resolution fails or Get fails (steps 2b, 2c)

**Code Pattern:**
```go
gvr, err := resolver.ResolveGVR(gvk)
if err != nil {
    resourceReport := ResourceReport{
        Group:     gvk.Group,
        Version:   gvk.Version,
        Kind:      gvk.Kind,
        Namespace: desired.GetNamespace(),
        Name:      desired.GetName(),
        Status:    "error",
        DriftType: "unknown",
        Error:     fmt.Sprintf("CRD not found: %v", err),
        Redacted:  false,
    }
    resourceReports = append(resourceReports, resourceReport)
    errors = append(errors, ...) // Keep for human output
    continue
}
```

### Point 3: DiffStats Computation

**Location:** After generateDiff returns diff string

**Implementation:** Already implemented in `diff_report.go::calculateDiffStats`

### Point 4: DriftType Classification

**Current Behavior:** `generateDiff` returns driftType as "spec" or "metadata"

**Enhancement Needed:** Detect "mixed" case (both spec and metadata differ)
- Current: simple heuristic (check spec change)
- Enhancement: Compare both spec and metadata separately
- Fallback: "metadata" if only metadata differs, "spec" if spec differs, "mixed" if both

**Pseudocode:**
```go
driftType := "none"
specDiff := compareSpec(desired, live)
metadataDiff := compareMetadata(desired, live)
if specDiff && metadataDiff {
    driftType = "mixed"
} else if specDiff {
    driftType = "spec"
} else if metadataDiff {
    driftType = "metadata"
}
```

## Error Model

### Error Classification

1. **Fatal Errors (exit 1, no report):**
   - Client creation failure
   - Discovery client failure
   - Resource resolver creation failure
   - Manifest loading failure
   - Report file write failure (if --report-file specified)

2. **Partial Errors (exit 1, partial report with error entries):**
   - CRD not found for specific resource → status=error
   - Resource not found in cluster → status=error
   - Individual resource fetch failures → status=error

3. **Drift (exit 2, full report):**
   - One or more resources have drift → status=drift, exit 2

### Error Entry Pattern

```go
ResourceReport{
    Status: "error",
    Error:  "CRD not found: <specific error>", // Remediation-oriented
    // Other fields populated where possible
}
```

**Remediation-oriented errors:**
- "CRD not found: <kind> - install with: kubectl apply -f <path>"
- "Resource not found: <kind>/<ns>/<name> - check namespace and name"
- "RBAC denied: <operation> - check ServiceAccount permissions"

## Output Model

### Stdout/Stderr Usage

**Without --report json:**
- Stdout: Human-readable diff output (summary + diffs)
- Stderr: Error messages (ERROR: prefix)

**With --report json (no --report-file):**
- Stdout: JSON report only
- Stderr: Error messages (ERROR: prefix)
- Human-readable output: Suppressed (JSON is primary)

**With --report json --report-file <path>:**
- File: JSON report (atomic write)
- Stdout: Human-readable diff output (preserved for compatibility)
- Stderr: Error messages

### Precedence Rules

1. If `--report json` is set:
   - Generate JSON report
   - If `--report-file` is set: Write JSON to file, keep human output to stdout
   - If `--report-file` is NOT set: Write JSON to stdout, suppress human output
2. If `--report` is not set:
   - Human-readable output to stdout (current behavior)
3. `--format unified|plain` only affects human output (not JSON)

### File Write Atomicity

**Implementation:** `writeReportFile` in `diff_report.go`

**Steps:**
1. Create temp file in same directory: `basename.tmp.<random>`
2. Write JSON to temp file
3. fsync() temp file
4. Close temp file
5. Rename temp → final (atomic on most filesystems)

**Error Handling:**
- Cleanup temp file on any error
- If rename fails, temp file remains (cleanup on next run or manual)

## Determinism Strategy

### Canonical Resource Ordering Key

**Sort Order:**
1. Group (ascending)
2. Kind (ascending, within group)
3. Namespace (ascending, within kind)
4. Name (ascending, within namespace)

**Implementation:** `sortResources` in `diff_report.go` (already implemented)

### Timestamp Normalization

**Strategy:**
- `generatedAt` field uses `time.Now().UTC().Format(time.RFC3339)`
- For tests: Normalize by replacing RFC3339 timestamp with fixed string
- Pattern: `"generatedAt": "2025-01-01T00:00:00Z"` → `"generatedAt": "<NORMALIZED>"`

**Test Normalization Function:**
```go
func normalizeTimestamp(jsonBytes []byte) []byte {
    // Replace generatedAt timestamp with placeholder
    pattern := regexp.MustCompile(`"generatedAt"\s*:\s*"[^"]+"`)
    return pattern.ReplaceAll(jsonBytes, []byte(`"generatedAt": "<NORMALIZED>"`))
}
```

## Redaction Strategy

### What Can and Cannot Be in JSON

**Allowed:**
- Resource metadata (group, version, kind, namespace, name)
- Status classification (no_drift, drift, error)
- DriftType (spec, metadata, mixed, none, unknown)
- DiffStats (line counts: added, removed, changed)
- Error messages (remediation-oriented strings)
- Redacted flag (boolean)

**Never Allowed:**
- Raw manifests (spec, data, stringData, etc.)
- Secret values (even if base64-encoded)
- ConfigMap data values (if they contain sensitive patterns)
- Any field values from .data or .stringData in Secrets
- Private keys, tokens, passwords, API keys (even as strings)

### When redacted=true is Set

1. **Secrets (always true):**
   - If Kind == "Secret" → redacted=true always
   - No data/stringData values in report

2. **Other Resources:**
   - If redactSecrets() function modified the object → redacted=true
   - Detection: Check if object contains fields matching secret patterns

3. **Detection Logic:**
   ```go
   func isSecretOrHasSecrets(desired, live *unstructured.Unstructured) bool {
       if desired.GetKind() == "Secret" || live.GetKind() == "Secret" {
           return true
       }
       // Check if redaction occurred (compare before/after redaction)
       // Simple: check for [REDACTED] placeholder (but this requires tracking)
       // Better: check for secret field patterns in normalized objects
       return hasSecretPatterns(desired) || hasSecretPatterns(live)
   }
   ```

**Simplified Approach for F1:**
- Secrets: Always redacted=true
- Other resources: redacted=false (redaction happens in diff generation, not tracked separately)
- Future enhancement (F2+): Track redaction during normalization

## Test Plan

### Golden Fixtures Needed

1. **no_drift.json:**
   - 2-3 resources, all status=no_drift
   - DiffStats nil or empty
   - Generated with known inputs

2. **drift_spec.json:**
   - 1-2 resources with spec drift
   - driftType="spec"
   - DiffStats populated

3. **drift_metadata.json:**
   - 1 resource with metadata drift only
   - driftType="metadata"

4. **error_case.json:**
   - 1 resource with status=error
   - Error field populated
   - Other resources may be no_drift

5. **secret_redacted.json:**
   - 1 Secret resource
   - redacted=true
   - No sensitive values in output

### Fixture Generation

**Process:**
1. Create test manifests (YAML files)
2. Run zenctl diff against test cluster (or mock)
3. Capture JSON output
4. Normalize timestamp
5. Save as golden fixture

**Location:** `cmd/zenctl/internal/commands/testdata/diff_report_*.json`

### drift-gate-fast Extension

**New Test Steps:**
1. Create temporary test manifests
2. Run: `zenctl diff --report json -f <test-manifests> -n <ns>`
3. Normalize timestamp in output
4. Compare to golden fixture (byte-for-byte)
5. Validate schema (JSON structure, required fields)
6. Validate redaction (no secret patterns in output)

**Test Structure:**
```bash
# Test JSON report generation
TEST_DIR=$(mktemp -d)
cat > "$TEST_DIR/test.yaml" <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
  namespace: default
data:
  key: value
EOF

# Run diff with JSON report
./zenctl diff --report json -f "$TEST_DIR" -n default > "$TEST_DIR/report.json"

# Normalize timestamp
sed -i 's/"generatedAt": "[^"]*"/"generatedAt": "<NORMALIZED>"/' "$TEST_DIR/report.json"

# Compare to golden fixture
diff "$TEST_DIR/report.json" testdata/diff_report_no_drift.json
```

## Implementation Pseudocode

### Modified diff.go RunE Function

```go
RunE: func(cmd *cobra.Command, args []string) error {
    // ... existing setup code ...
    
    var resourceReports []ResourceReport
    var drifts []string
    var errors []string
    
    // Process each desired object
    for _, desired := range desiredObjects {
        gvk := parseGVK(desired)
        
        // Resolve GVR
        gvr, err := resolver.ResolveGVR(gvk)
        if err != nil {
            // Create error entry
            resourceReports = append(resourceReports, ResourceReport{
                Group: gvk.Group, Version: gvk.Version, Kind: gvk.Kind,
                Namespace: desired.GetNamespace(), Name: desired.GetName(),
                Status: "error", Error: fmt.Sprintf("CRD not found: %v", err),
            })
            errors = append(errors, ...)
            continue
        }
        
        // Fetch live object
        live, err := dynClient.Resource(gvr).Namespace(objNS).Get(...)
        if err != nil {
            resourceReports = append(resourceReports, ResourceReport{
                Status: "error", Error: fmt.Sprintf("not found: %v", err),
            })
            errors = append(errors, ...)
            continue
        }
        
        // Normalize and diff
        desiredNormalized := normalizeForDiff(desired, ...)
        liveNormalized := normalizeForDiff(live, ...)
        diff, driftType := generateDiff(desiredNormalized, liveNormalized, ...)
        
        // Create resource report
        resourceReport := ResourceReport{
            Group: gvk.Group, Version: gvk.Version, Kind: gvk.Kind,
            Namespace: objNS, Name: desired.GetName(),
            Status: "no_drift", DriftType: driftType,
            DiffStats: calculateDiffStats(diff),
            Redacted: isSecret(desired) || isSecret(live),
        }
        
        if diff != "" {
            resourceReport.Status = "drift"
            drifts = append(drifts, diff)
        }
        
        resourceReports = append(resourceReports, resourceReport)
    }
    
    // Sort resources for determinism
    resourceReports = sortResources(resourceReports)
    
    // Generate JSON report if requested
    if reportFormat == "json" {
        report := buildDiffReport(opts.Context, resourceReports)
        
        if reportFile != "" {
            // Write to file (atomic)
            if err := writeReportFile(report, reportFile); err != nil {
                cmd.PrintErrln("ERROR: Failed to write report:", err)
                return fmt.Errorf("report write failed: %w", err)
            }
            // Human output to stdout (preserved)
        } else {
            // Write JSON to stdout
            encoder := json.NewEncoder(os.Stdout)
            encoder.SetIndent("", "  ")
            if err := encoder.Encode(report); err != nil {
                return fmt.Errorf("failed to encode report: %w", err)
            }
            // Human output suppressed
            // Exit based on drifts/errors
            if len(errors) > 0 {
                return fmt.Errorf("validation failed: %d error(s)", len(errors))
            }
            if len(drifts) > 0 {
                os.Exit(2)
            }
            return nil
        }
    }
    
    // Existing human output logic
    // ... exit code handling ...
}
```

## Acceptance Criteria Summary

1. ✅ Plan document exists and is precise
2. ✅ Pseudocode snippets show control flow
3. ✅ Insertion points identified
4. ✅ Error model defined
5. ✅ Output model defined
6. ✅ Determinism strategy defined
7. ✅ Redaction strategy defined
8. ✅ Test plan defined

## Next Steps

1. Implement flags (`--report json`, `--report-file`)
2. Refactor diff.go to collect ResourceReports
3. Implement report generation and output
4. Add tests and fixtures
5. Extend drift-gate-fast
6. Create Zen-Admin stub doc
7. Create evidence pack

