# Branding & Decoupling Audit

**Purpose**: Audit of kube-zen/zen-hook/zen-* branding embedded in zen-watcher's public API surface, metrics, labels, and documentation.

**Last Updated**: 2025-12-10

**Status**: ✅ Complete Audit

---

## Classification

Branded elements are classified as:

- **API Surface (Hard)**: Breaking changes required to remove (CRD group/kind, required fields, required labels)
- **Metrics/Internal**: Internal metric names (acceptable if stable, `zen_watcher_*` prefix is fine)
- **Docs/Examples (Soft)**: Documentation and examples (can be neutralized without breaking changes)

---

## API Surface (Hard Branding)

### CRD Group Names

**Location**: All CRD definitions in `deployments/crds/*.yaml`

**Branded Elements**:
- **Group**: `zen.kube-zen.io` (hard-coded in all CRDs)
  - `observations.zen.kube-zen.io`
  - `observationsourceconfigs.zen.kube-zen.io`
  - `observationtypeconfigs.zen.kube-zen.io`
  - `observationmappings.zen.kube-zen.io`
  - `observationdedupconfigs.zen.kube-zen.io`
  - `observationfilters.zen.kube-zen.io`

**Impact**: Breaking change to rename (requires new CRD group, migration path)

**Future Plan**: See `docs/OBSERVATION_VERSIONING_AND_RELEASE_PLAN.md` for migration to neutral group (e.g., `observations.kubernetes.io` or `observations.watcher.io`) in v1beta1 or v2.

---

### CRD Kind Names

**Location**: All CRD definitions

**Branded Elements**:
- **Kind**: `Observation` (neutral ✅)
- **Kind**: `ObservationSourceConfig` (neutral ✅)
- **Kind**: `ObservationTypeConfig` (neutral ✅)
- **Kind**: `ObservationMapping` (neutral ✅)
- **Kind**: `ObservationDedupConfig` (neutral ✅)
- **Kind**: `ObservationFilter` (neutral ✅)

**Impact**: No branding in kind names (good!)

---

### CRD Field Names

**Location**: CRD schema definitions

**Branded Elements**:
- **Field names**: All field names are generic (no `zen*` or `kubeZen*` prefixes) ✅
  - `spec.source`, `spec.category`, `spec.severity`, `spec.eventType` (all neutral)
  - `spec.resource`, `spec.details`, `spec.detectedAt` (all neutral)

**Impact**: No branding in field names (good!)

---

### Required Labels/Annotations

**Location**: `deployments/crds/observation_crd.yaml` (line 101)

**Branded Elements**:
- **Label prefix**: `zen.io/*` (documented as standard labels in CRD schema)
  - Example: `zen.io/source`, `zen.io/type`, `zen.io/priority`
  - **Note**: These are documented conventions, not required by CRD validation
  - **Actual usage**: Labels are optional (not enforced by CRD schema)

**Impact**: Labels are optional (not required by validation), but prefix convention is branded

**Future Plan**: Consider neutral prefix (e.g., `watcher.io/*` or `observations.io/*`) in future version, but current labels are optional so not blocking.

---

## Metrics & Labels (Internal)

### Metric Names

**Location**: `pkg/metrics/definitions.go`, `pkg/metrics/ha_metrics.go`

**Branded Elements**:
- **Metric prefix**: `zen_watcher_*` (all metrics use this prefix)
  - Examples: `zen_watcher_events_total`, `zen_watcher_observations_created_total`, `zen_watcher_webhook_requests_total`
  - Total: 54+ metrics with `zen_watcher_*` prefix

**Classification**: **Metrics/Internal** ✅

**Rationale**: Internal metric names are acceptable to be branded. The `zen_watcher_*` prefix:
- Identifies the component (standard Prometheus practice)
- Is stable (not user-facing in API contracts)
- Follows Prometheus naming conventions (component prefix)

**Impact**: No change needed - internal metrics are fine to be branded.

---

### Label Names in Metrics

**Location**: Metric label definitions

**Branded Elements**:
- **Label names**: All label names are generic (no branding) ✅
  - Examples: `source`, `category`, `severity`, `eventType`, `namespace`, `kind`, `tool`, `resource`, `endpoint`, `status`
  - No `zen*` or `kubeZen*` prefixes in label names

**Impact**: No branding in metric label names (good!)

---

## Documentation & Examples (Soft Branding)

### Public API Documentation

**Location**: `docs/OBSERVATION_API_PUBLIC_GUIDE.md`

**Branded References**:
- Line 76: `zen-hook` mentioned as example source value
- Line 61: `zen.kube-zen.io` group documented (required - this is the actual API)
- **Classification**: Mixed
  - `zen.kube-zen.io` group: **API Surface (Hard)** - actual API, must document
  - `zen-hook` example: **Docs/Examples (Soft)** - can be neutralized

**Action Required**: Neutralize `zen-hook` references (treat as example, not requirement).

---

### Dynamic Webhooks Integration Doc

**Location**: `docs/DYNAMIC_WEBHOOKS_WATCHER_INTEGRATION.md`

**Branded References**:
- Title and throughout: `zen-hook` mentioned as if it's the only/primary webhook gateway
- Line 13: "zen-hook is a future dynamic webhook gateway component"
- Line 17-20: Roles section describes zen-hook as primary actor
- Line 35-42: Contract section uses "zen-hook MUST" language
- Line 56-83: Example shows `source: "zen-hook"` as if required
- Line 88-99: Required labels include `zen.io/source: "zen-hook"` as if mandatory

**Classification**: **Docs/Examples (Soft)** - Can be neutralized

**Action Required**: 
- Reframe as "webhook gateway" (generic) with zen-hook as one example
- Remove "zen-hook MUST" language, use generic "webhook producers MUST"
- Make zen-hook references clearly optional/example-only

---

### Examples Directory

**Location**: `examples/observations/08-webhook-originated.yaml`, `examples/observations/README.md`

**Branded References**:
- `08-webhook-originated.yaml`: Uses `source: "zen-hook"` and `zen.io/source: "zen-hook"` labels
- `examples/observations/README.md` (line 102-114): Describes example as "zen-hook style webhook event" and mentions "Dynamic webhook gateway (zen-hook)"

**Classification**: **Docs/Examples (Soft)** - Can be neutralized

**Action Required**:
- Rename example to generic "webhook-gateway" or "webhook-producer"
- Update labels to use generic source (e.g., `source: "webhook-gateway"`)
- Add note that zen-hook is one concrete implementation example

---

### Contributor Documentation

**Location**: `CONTRIBUTING.md`

**Branded References**:
- Line 194, 227, 296, 332: Code examples use `apiVersion: "zen.kube-zen.io/v1"` (required - actual API)
- **Classification**: Mixed
  - `zen.kube-zen.io` group: **API Surface (Hard)** - actual API, must document
  - No other branding in contributor guidance

**Action Required**: None (API group is actual API, not branding issue).

---

### PM AI Roadmap

**Location**: `docs/PM_AI_ROADMAP.md`

**Branded References**:
- No explicit kube-zen/zen-hook branding found in roadmap
- **Classification**: ✅ Neutral

**Action Required**: None.

---

### KEP Draft

**Location**: `docs/KEP_DRAFT_ZEN_WATCHER_OBSERVATIONS.md`

**Branded References**:
- Line 88, 105: `zen.kube-zen.io` group documented (required - actual API)
- Line 247-248: Mentions zen-hook as example producer
- Line 325: References DYNAMIC_WEBHOOKS_WATCHER_INTEGRATION.md (which has branding)

**Classification**: Mixed
- `zen.kube-zen.io` group: **API Surface (Hard)** - actual API
- zen-hook references: **Docs/Examples (Soft)** - can be neutralized

**Action Required**: Neutralize zen-hook references (treat as example).

---

## Summary

### API Surface (Hard Branding) - Requires Future Versioning

1. **CRD Group**: `zen.kube-zen.io` (all 6 CRDs)
   - **Impact**: Breaking change to rename
   - **Plan**: Migrate to neutral group in v1beta1 or v2 (see versioning plan)

2. **Label Prefix Convention**: `zen.io/*` (documented, but optional)
   - **Impact**: Low (labels are optional, not enforced)
   - **Plan**: Consider neutral prefix in future version, but not blocking

### Metrics/Internal - Acceptable

1. **Metric Prefix**: `zen_watcher_*` (54+ metrics)
   - **Impact**: None (internal metrics, standard practice)
   - **Plan**: No change needed

### Docs/Examples (Soft Branding) - Can Be Neutralized Now

1. **DYNAMIC_WEBHOOKS_WATCHER_INTEGRATION.md**: Treats zen-hook as required/primary
2. **examples/observations/08-webhook-originated.yaml**: Uses zen-hook as source
3. **examples/observations/README.md**: Describes zen-hook as primary webhook gateway
4. **OBSERVATION_API_PUBLIC_GUIDE.md**: Mentions zen-hook as example (minor)

**Action**: Neutralize all of these in Workstream 2 (non-breaking).

---

## Next Steps

1. **Workstream 2**: Neutralize docs/examples (non-breaking)
2. **Workstream 3**: Plan future versioning for hard-coded branding (CRD group migration)
