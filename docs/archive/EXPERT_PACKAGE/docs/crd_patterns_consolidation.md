---
⚠️ HISTORICAL DOCUMENT - EXPERT PACKAGE ARCHIVE ⚠️

This document is from an external "Expert Package" analysis of zen-watcher/ingester.
It reflects the state of zen-watcher at a specific point in time and may be partially obsolete.

CANONICAL SOURCES (use these for current direction):
- docs/PM_AI_ROADMAP.md - Current roadmap and priorities
- CONTRIBUTING.md - Current quality bar and standards
- docs/INFORMERS_CONVERGENCE_NOTES.md - Current informer architecture
- docs/STRESS_TEST_RESULTS.md - Current performance baselines

This archive document is provided for historical context, rationale, and inspiration only.
Do NOT use this as a replacement for current documentation.

---

# Unified CRD Patterns Strategy for Zen Watcher

## Executive Summary

Zen Watcher currently operates two overlapping Custom Resource Definition (CRD) ecosystems that implement the same core concepts—sources, ingestion pipelines, and observations—using different groups, versions, schemas, and operational behaviors. In the legacy repository, the group is “zen.kube-zen.io” and the primary custom resource is the Observation, complemented by supporting types for filtering, mapping, and deduplication. In the ingester implementation, the group is “zenwatcher.kube-zen.io” and the primary resources are Source and Ingestor, each with broader configuration surfaces, richer status blocks, and a declared conversion webhook strategy.

The most visible inconsistencies are fourfold. First, the API groups diverge—“zen.kube-zen.io” versus “zenwatcher.kube-zen.io”—which complicates multi-repo operations and cross-team collaboration. Second, the Observation CRD is defined with materially different schemas in each ecosystem, including a v1 storage version with TTL fields in the legacy repository contrasted with a v2 non-storage version in the deployments folder; the ingester-side model orients around Source and Ingestor rather than Observation at all. Third, status handling is inconsistent: legacy Observation uses a minimal “processed” flag and timestamp, whereas Ingestor and Source status blocks are extensive with phases, metrics, and condition arrays designed for operator-grade observability. Fourth, validation strength and defaulting differ across the CRDs, with the legacy schemas relying primarily on required fields, patterns, enums, and preserve-unknown-fields, while the ingester schemas document conversion and scale subresources and rich additional printer columns.

These differences raise tangible risks: divergent validation can allow inconsistent configurations that are hard to debug; split informer patterns and lifecycle management increase operational complexity; and duplicate CRDs slow development velocity and complicate upgrades. A unified CRD approach can mitigate these risks by standardizing the API group and versions, adopting a single canonical Observation schema, consolidating ingestion configuration into a single Ingestor CRD, and harmonizing validation, status, and informer patterns. The key benefits are a simplified operational model, stronger correctness guarantees, easier lifecycle management, and faster feature delivery.

The recommended direction is to converge on a single group (“zen.watcher.io”), a single v1 storage version for the canonical Observation CRD, and a unified Ingestor CRD that encapsulates source behavior, filters, outputs, scheduling, and security. The Observation CRD should carry a minimal status and a TTL field for garbage collection, and the Ingestor should carry a detailed status with phases, metrics, and conditions. Informer management must be standardized via dynamic informer factories with controlled resyncs, clear shutdown semantics, and a single mapping-driven mechanism for generic CRD sources. Conversion strategy, if needed, should be explicit and centrally managed rather than embedded as disparate annotations. A phased migration plan with dual-serving versions, adoption metrics, and rollback is essential to reduce risk.

### Key Findings at a Glance

- The legacy repository defines Observation CRDs in both deployments and Helm templates with dual versions (v1 storage, v2 non-storage) and different status fields and printer columns.  
- The ingester implementation defines Source and Ingestor CRDs under a distinct API group, with richer status structures, scale subresources, and conversion webhooks.  
- Informer patterns are present in the legacy repository via dynamic factories and a CRD-driven adapter that watches mapping objects and starts per-source informers; shutdown and resync behaviors are implemented but not uniformly enforced.  
- Validation and defaults are inconsistent: the legacy schemas emphasize required fields and enums; the ingester schemas extensively document additional features but leave explicit CEL validation stubs.

### Top Consolidation Opportunities

- Unify API groups and versions to a single canonical group with v1 storage for all stable CRDs.  
- Adopt a single canonical Observation CRD schema, harmonizing required fields, status shape, TTL semantics, and printer columns.  
- Consolidate ingestion configuration into a unified Ingestor CRD, with Source behavior expressed via spec fields rather than separate CRDs.  
- Standardize informer lifecycle and conversion strategies, including explicit conversion webhooks (if needed), common resync intervals, and consistent shutdown handling.

## Scope, Inputs, and Methodology

This assessment examines CRDs and related controller/informer patterns across two ecosystems: the legacy Zen Watcher repository and the ingester implementation. The analysis focuses on how CRDs are defined and managed, the shape and semantics of status fields, validation and defaults, informer integration, lifecycle management, and opportunities for consolidation. Evidence is drawn from CRD manifests, Helm templates, and Go code that demonstrates informer setup, lifecycle hooks, and CRD-driven adapter behavior.

Two constraints are noteworthy. First, the ingester implementation uses webhook-based conversion and scale subresources; the actual webhook implementation is referenced but not provided, limiting validation of conversion behavior. Second, the existence of ObservationSourceConfig and ObservationTypeConfig CRDs is indicated but their contents are not available, which constrains the completeness of comparative mapping. The methodology is purely static analysis of the available manifests and code; no performance benchmarks or production metrics are included.

### Artifacts Reviewed

- Legacy CRDs: Observation, ObservationFilter, ObservationMapping, ObservationDedupConfig.  
- Ingester CRDs: Ingestor and Source.  
- Helm templates: an Observation CRD in the Helm chart templates.  
- Code: CRD adapter and informer management; lifecycle shutdown; dynamic informer factory setup; and filter loader patterns.

### Analytical Lens

The analysis is structured around five dimensions: definition and management, status handling, validation and defaults, informer integration, and lifecycle. For each dimension, we compare the two ecosystems and identify the deltas that drive consolidation opportunities. Where the evidence is incomplete, we note the information gaps explicitly.

## System Overview and CRD Inventories

The legacy Zen Watcher repository defines an Observation-centric model under the API group “zen.kube-zen.io” with multiple supporting CRDs that shape filtering, mapping, and deduplication. In contrast, the ingester implementation introduces a Source and Ingestor model under “zenwatcher.kube-zen.io,” designed to encapsulate ingestion behaviors and outputs with broader status and subresource features.

To illustrate the landscape, the following tables summarize the CRDs in each ecosystem.

To ground the comparison, Table 1 inventories the legacy CRDs, their versions, schema highlights, status shape, and subresources.

Table 1. Inventory of legacy CRDs (zen.kube-zen.io)

| CRD Name                      | API Group           | Version(s)               | Storage Version | Schema Highlights                                                                                                                   | Status Fields                                  | Subresources          |
|------------------------------|---------------------|--------------------------|-----------------|-------------------------------------------------------------------------------------------------------------------------------------|------------------------------------------------|-----------------------|
| Observation                  | zen.kube-zen.io     | v1 (storage), v2 (served) | v1              | Required: source, category, severity, eventType; resource object; details (preserve-unknown-fields); detectedAt; ttlSecondsAfterCreation | processed (bool), lastProcessedAt (date-time)  | status: {}            |
| ObservationFilter            | zen.kube-zen.io     | v1alpha1                 | v1alpha1        | targetSource (pattern + enum); include/exclude lists (severity, eventTypes, namespaces, kinds, categories, rules); enabled flag      | status: {}                                     | status: {}            |
| ObservationMapping           | zen.kube-zen.io     | v1alpha1                 | v1alpha1        | sourceName (pattern); group/version/kind; mappings for severity/category/eventType/message/resource; details (preserve-unknown-fields); severityMap; enabled with default | none declared                                  | none declared         |
| ObservationDedupConfig       | zen.kube-zen.io     | v1alpha1                 | v1alpha1        | targetSource (pattern); windowSeconds (min/max/default 60); enabled (default true)                                                 | status: {}                                     | status: {}            |

As shown above, the legacy model centers on Observations with optional TTL semantics for garbage collection and rich filter/mapping controls to shape upstream events. The status blocks are minimal, emphasizing processing flags and timestamps.

Table 2 summarizes the ingester CRDs under “zenwatcher.kube-zen.io.”

Table 2. Inventory of ingester CRDs (zenwatcher.kube-zen.io)

| CRD Name | API Group              | Version(s) | Storage Version | Schema Highlights                                                                                                                                                                                                                                              | Status Fields                                                                                                                                                                                                                     | Subresources                          |
|----------|------------------------|------------|-----------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|---------------------------------------|
| Ingestor | zenwatcher.kube-zen.io | v1         | v1              | Extensive spec: type (enum), enabled (default true), priority (enum), environment (enum), config (preserve-unknown-fields) with many documented fields, filters, outputs (with transformation flags), scheduling (cron/interval/jitter/timezone), healthCheck, security (encryption, rbac, compliance, vault) | phase (enum: Pending/Running/Failed/Disabled/Degraded), lastScan/nextScan (date-time), observations/errors/lastError, healthScore, performance (avgProcessingTime/throughput/errorRate), conditions array; top-level status mirrors spec with additional aggregates | status: {}; scale: specReplicasPath, statusReplicasPath, labelSelectorPath, annotationSelectorPath |
| Source   | zenwatcher.kube-zen.io | v1         | v1              | Narrower spec: type (enum subset), enabled (default true), config (preserve-unknown-fields) with provider fields (webhook/nginx/log/configmap/custom), filters, outputs (format enum: observation/raw), scheduling, healthCheck                                                     | phase (enum: Pending/Running/Failed/Disabled), lastScan/nextScan (date-time), observations/errors/lastError, conditions array                                                                                                                                                                 | status: {}                            |

The ingester model is designed for operational depth, offering detailed status and subresources. The Ingestor CRD also declares a conversion webhook strategy, enabling future evolution.

### Legacy Zen Watcher CRDs (zen.kube-zen.io)

The Observation CRD appears in multiple places with different version configurations. In deployments, both v1 (storage) and v2 (served) are defined, whereas the Helm template carries only v1 (storage). Schema differences are material: the deployments variant defines dual versions with different required fields, while the Helm variant leans into a v1-only schema with TTL and a status block that emphasizes “synced” to SaaS rather than “processed.” Supporting CRDs—ObservationFilter, ObservationMapping, and ObservationDedupConfig—are consistently served and stored as v1alpha1 with clear patterns and defaults.

### Ingester CRDs (zenwatcher.kube-zen.io)

The Ingestor and Source CRDs share a common group and version but diverge in scope. The Ingestor is a comprehensive descriptor for an ingestion pipeline with security, scheduling, health checks, and rich status. The Source is a lighter-weight abstraction focusing on upstream provider configuration and outputs. Both declare conversion webhooks with client configuration to a centralized service, and Ingestor declares scale subresources. The schemas rely on preserve-unknown-fields for extensibility, and both present condition-based status designs with phases.

### Observed Helm Template CRD

The Helm template for Observation CRD aligns with the legacy repository’s v1 storage but emphasizes a “synced” status to represent SaaS synchronization. The schema is more conservative than the deployments variant and does not carry the same level of flexibility in required fields. This variance highlights a deployment渠道-driven divergence that should be reconciled in a unified approach.

## CRD Definition and Management Patterns

A clear pattern emerges in how CRDs are authored and managed. The legacy repository distributes CRDs across deployments and Helm templates, each with a slightly different stance on versions and status. The ingester implementation centralizes CRDs as singular definitions with broader scope, explicit conversion strategies, and more comprehensive subresources.

The differences are most apparent in four areas: API group naming, version strategy, conversion, and deployment packaging.

### API Groups and Naming

Legacy CRDs use “zen.kube-zen.io,” while ingester CRDs use “zenwatcher.kube-zen.io.” This divergence creates cognitive overhead for operators and complicates client configuration, RBAC policies, and cross-repo interactions. A single canonical group reduces confusion and simplifies policy and tooling.

### Versions and Conversion

Legacy Observation CRDs appear with both v1 (storage) and v2 (served) definitions in deployments, and a v1-only definition in Helm. The v2 schema changes required fields and semantics (for example, “source,” “type,” “priority,” “title,” “description,” “detectedAt”), while the v1 schema uses “source,” “category,” “severity,” “eventType,” and ttlSecondsAfterCreation. The ingester Ingestor and Source CRDs declare conversion webhooks with explicit client configuration and allowed conversions (v1beta1 to v1), but the actual webhook implementation is not provided.

Table 3 captures version and conversion patterns.

Table 3. Version and conversion strategy by CRD

| CRD             | Served Versions         | Storage Version | Conversion Strategy                | Notes                                                                                  |
|-----------------|-------------------------|-----------------|------------------------------------|----------------------------------------------------------------------------------------|
| Observation     | v1, v2 (deployments); v1 (Helm) | v1 (deployments/Helm) | None declared                      | Dual versions in deployments differ in required fields; Helm v1 emphasizes “synced.”  |
| ObservationFilter | v1alpha1               | v1alpha1        | None declared                      | Strong enums and patterns; status subresource present but empty.                       |
| ObservationMapping | v1alpha1             | v1alpha1        | None declared                      | Mappings and preserve-unknown-fields for extensibility.                                |
| ObservationDedupConfig | v1alpha1         | v1alpha1        | None declared                      | Defaults for windowSeconds and enabled.                                                |
| Ingestor        | v1                      | v1              | Webhook strategy (v1beta1 to v1)   | Scale subresources declared; conversion service configured with CA bundle.             |
| Source          | v1                      | v1              | Webhook strategy (v1beta1 to v1)   | Lighter scope than Ingestor; conversion declared.                                      |

The lack of a unified version strategy and conversion management increases the risk of schema drift and complicates future evolution. Centralizing conversion behavior, if needed, under a single webhook service would improve control and auditability.

### Deployment Packaging

Legacy CRDs are distributed as plain manifests in deployments and via Helm templates. The ingester implementation keeps CRDs as part of implementation artifacts. Packaging inconsistencies can lead to version skew between environments and make upgrades brittle. A single packaging strategy—preferably Helm with strict linting and version pinning—will ensure consistency and enable GitOps flows.

## Status Handling Patterns

Status blocks reveal the intended operational semantics of each CRD. The legacy Observation status is intentionally minimal: a boolean “processed” flag and a timestamp, or in the Helm variant, “synced” and related fields to represent SaaS synchronization. In contrast, Ingestor and Source status blocks are rich with phases, metrics, and conditions arrays designed to reflect pipeline health and guide operator action.

Table 4 compares status field shapes.

Table 4. Status field comparison across CRDs

| CRD             | Status Fields                                                                                         | Semantics                                                                                  |
|-----------------|--------------------------------------------------------------------------------------------------------|--------------------------------------------------------------------------------------------|
| Observation (deployments) | processed (bool), lastProcessedAt (date-time)                                             | Indicates whether the observation has been processed and when.                              |
| Observation (Helm)         | synced (bool), lastSyncAttempt (date-time), syncError (string), saasEventId (string) | Indicates whether the observation has been synced to a SaaS endpoint and related details.  |
| ObservationFilter          | status subresource declared; no fields                                                            | Intended as a hook for future extension; currently empty.                                  |
| ObservationMapping         | none                                                                                                  | Mapping definitions are configuration-only.                                                |
| ObservationDedupConfig     | status subresource declared; no fields                                                            | Intended as a hook for future extension; currently empty.                                  |
| Ingestor                   | phase, lastScan, nextScan, observations, errors, lastError, healthScore, performance, conditions     | Operator-grade status reflecting pipeline health, throughput, latency, and error rates.    |
| Source                     | phase, lastScan, nextScan, observations, errors, lastError, conditions                               | Lighter status focused on scan schedule and outcome metrics.                               |

### Legacy Observation Status Semantics

In the legacy deployments, “processed” indicates completion of the ingestion or transformation pipeline, with “lastProcessedAt” capturing the timestamp. In the Helm template, “synced” and associated fields communicate external synchronization, reflecting a deployment-channel specialization. This divergence suggests the need for a unified status semantics model that can represent both processing and synchronization without conflating them.

### Ingester Status Semantics

The Ingestor status uses phases to indicate broad operational states (“Pending,” “Running,” “Failed,” “Disabled,” “Degraded”), along with metrics for last and next scan, totals for observations and errors, a health score, and performance measurements (average processing time, throughput, error rate) with a conditions array for fine-grained state. Source status is similar but lacks performance metrics and health score. This richer model is appropriate for operators managing ingestion at scale and should be adopted for the unified Ingestor CRD.

## Validation and Defaults

Validation patterns across the CRDs vary in rigor. The legacy schemas rely on OpenAPI validation with required fields, patterns, enums, minimum/maximum constraints, and preserve-unknown-fields for flexibility. DedupConfig applies defaults for windowSeconds and enabled; ObservationMapping defaults “enabled” to true and sets default strings for category and eventType. The ingester schemas document extensive enum constraints, patterns for durations and cron expressions, and preserve-unknown-fields within config blocks, but explicit CEL rules are referenced as stubs.

Table 5 summarizes validation and defaulting highlights.

Table 5. Validation and defaults by CRD

| CRD                    | Required Fields                                         | Patterns/Enums                                                                                          | Defaults                                 | x-kubernetes-preserve-unknown-fields |
|------------------------|---------------------------------------------------------|----------------------------------------------------------------------------------------------------------|-------------------------------------------|--------------------------------------|
| Observation (v1)       | source, category, severity, eventType                  | severity enum (legacy style); ttlSecondsAfterCreation minimum                                            | none explicit                             | details block                        |
| Observation (v2)       | source, type, priority, title, description, detectedAt | source/type patterns; priority min/max 0.0–1.0                                                           | none explicit                             | spec and raw block                   |
| ObservationFilter      | targetSource                    | targetSource pattern; extensive enums for severity/event types/namespaces/kinds/categories/rules         | none explicit                             | none declared                        |
| ObservationMapping     | sourceName, group, version, kind                       | sourceName pattern; severityMap enum values (CRITICAL/HIGH/MEDIUM/LOW/UNKNOWN)                           | enabled default true; category default “security”; eventType default “custom-event” | mappings.details and spec            |
| ObservationDedupConfig | targetSource                    | windowSeconds min/max                                                                                    | windowSeconds default 60; enabled default true | none declared                        |
| Ingestor               | none explicit in spec (broad object)                   | type/priority/environment enums; config patterns (durations, rate limits); cron/interval patterns        | enabled default true; priority default “normal”; environment default “production” | config, transformation, filters      |
| Source                 | none explicit in spec (broad object)                   | type enum subset; methods enum; logFormat/logLevel enums; cron/interval patterns                         | enabled default true                       | config, transformation, filters      |

### Required Fields and Enums

The legacy v1 Observation requires “source,” “category,” “severity,” and “eventType,” and the v2 Observation requires “source,” “type,” “priority,” “title,” “description,” and “detectedAt.” ObservationFilter requires “targetSource” and offers exhaustive enums to constrain inputs. ObservationMapping requires identity fields and maps source-specific values to canonical severities. DedupConfig requires “targetSource” and constrains windowSeconds.

The ingester schemas provide enum sets for “type,” “priority,” and “environment” and document extensive configuration constraints. These enums should be tightened with explicit CEL rules in the unified CRDs to improve server-side validation and reduce operator errors.

### Defaults and Extensibility

Defaults are applied sparingly in the legacy CRDs—DedupConfig defaults windowSeconds and enabled; Mapping defaults enabled, category, and eventType. The ingester CRDs set defaults for enabled, priority, environment, and health check intervals. Extensibility is primarily achieved via preserve-unknown-fields in the ingester config blocks and mapping details, which is appropriate for provider-specific options but should be balanced with strong validation to avoid silent misconfigurations.

## Informer Integration Patterns

The legacy repository demonstrates a CRD-driven adapter that watches ObservationMapping objects and dynamically provisions informers for the configured source CRDs. A dynamic informer factory is created with a default resync period, and lifecycle management uses a context and stop channel to handle shutdown. A filter loader watches ObservationFilter CRDs to dynamically reload filter configurations. This pattern is robust and extensible, enabling new sources to be added without code changes.

Table 6 summarizes informer usage by CRD and component.

Table 6. Informer usage map

| Component/CRD           | Informer Type                 | Target GVR                               | Event Handlers                        | Resync Period     | Shutdown Handling                    |
|-------------------------|-------------------------------|------------------------------------------|---------------------------------------|-------------------|--------------------------------------|
| CRDSourceAdapter        | DynamicSharedInformerFactory  | ObservationMapping (zen.kube-zen.io)     | Add/Update/Delete mapping handlers    | Factory default   | Context cancellation + stop channel  |
| Per-source informers    | SharedIndexInformer (dynamic) | Configured source CRDs via mapping GVR   | Add/Update CRD instance handlers      | Inherits factory  | Stop channel via adapter Stop()      |
| ObservationFilterLoader | DynamicSharedInformerFactory  | ObservationFilter (zen.kube-zen.io)      | Reload filters on changes             | Not specified     | Not specified                        |
| DynamicInformerFactory  | Factory setup                 | N/A                                      | N/A                                   | 30 minutes        | Stop channel from lifecycle handler  |

### CRD-Driven Adapter Pattern

The adapter extracts mapping configuration and starts an informer for each source CRD defined by the mapping. This decouples source behavior from code and allows operators to control ingestion via CRDs. Mapping changes dynamically add or remove informers, which is powerful but requires disciplined lifecycle management to avoid leaks or stale caches.

### Lifecycle and Shutdown

Shutdown is orchestrated via a context and stop channel, with cache sync checks before declaring readiness. Resync periods are set at the factory level, providing periodic reconciliation that can help deduplication and consistency. These practices should be adopted uniformly across all informers in the unified approach, with explicit resync intervals and shutdown hooks.

## CRD Lifecycle Management

Versioning and conversion strategies differ across the ecosystems. The legacy Observation CRD has dual versions in deployments but lacks explicit conversion; Helm carries a v1-only variant with different status semantics. The ingester CRDs declare conversion webhooks, but the implementation is absent from the reviewed artifacts. Subresources are used extensively in the ingester (status and scale), whereas the legacy CRDs primarily declare status subresources without fields.

Upgrade considerations are not fully documented in the reviewed artifacts. A unified lifecycle should define storage version promotion criteria, conversion pathways, and rollout controls, including dual-serving versions, conversion tests, and clear upgrade/downrunbook steps.

### Conversion Webhooks

The ingester declares a conversion strategy with client configuration pointing to a centralized service, including CA bundle and allowed conversions (v1beta1 to v1). Without the actual conversion code, we cannot validate conversion correctness or failure modes. In the unified approach, conversion must be treated as a first-class capability with tests, monitoring, and rollback plans.

### Storage Version and Status Subresources

The legacy v1 Observation uses status subresources with minimal fields, and the Helm variant uses status fields tailored for SaaS sync. The ingester Ingestor uses both status and scale subresources with detailed fields and performance metrics. Harmonizing these patterns requires choosing a canonical status shape for each CRD type and aligning subresources accordingly.

## Comparative Analysis and Gap Assessment

Bringing the observations together, the two ecosystems implement overlapping capabilities using different groups, versions, and status designs. The most significant gap is the lack of a single canonical Observation schema that can serve both the legacy and the ingester use cases. A second gap is the split between configuration-driven sources (ObservationFilter and ObservationMapping) and a dedicated Source CRD in the ingester. A third gap is validation strength: while enums and patterns exist, explicit CEL rules are not enforced, leaving room for ambiguous inputs.

Table 7 presents a consolidated delta matrix across the five analytical dimensions.

Table 7. Delta matrix: Legacy vs Ingester vs Unified Target

| Dimension                   | Legacy (zen.kube-zen.io)                                           | Ingester (zenwatcher.kube-zen.io)                                   | Unified Target (zen.watcher.io)                                                                                              |
|----------------------------|---------------------------------------------------------------------|----------------------------------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------|
| Definition & Management    | Multiple manifests; Helm template variants; dual versions (v1/v2)   | Single CRDs per type; conversion webhooks declared                   | Single group; v1 storage for stable CRDs; centralized conversion (if needed); Helm-only packaging                             |
| Status Handling            | Minimal (processed/lastProcessedAt) or SaaS sync fields             | Rich phases, metrics, conditions; scale subresources                 | Observation: minimal status + TTL; Ingestor: rich status (phases, metrics, conditions)                                        |
| Validation & Defaults      | Required fields, enums, patterns; preserve-unknown-fields in blocks | Extensive enums/patterns; preserve-unknown-fields; CEL stubs         | Strong OpenAPI + CEL; canonical enums; explicit defaults; controlled preserve-unknown-fields                                   |
| Informer Integration       | Dynamic factory; mapping-driven adapter; filter loader              | Not evidenced in reviewed code; likely central controller patterns   | Standardized dynamic factory; common resync; single lifecycle manager; mapping-driven sources                                  |
| Lifecycle                  | No explicit conversion; deployment-channel schema divergence        | Conversion declared; subresources present; webhook implementation unseen | Explicit conversion strategy; promotion criteria; rollout gates; dual-serve; explicit rollback                                 |

### Overlaps

Both ecosystems define ingestion-related schemas and status semantics. ObservationFilter and ObservationMapping constitute a configuration-driven model for sources and transformations that overlaps with the Source CRD in the ingester. The status designs both aim to represent pipeline state, but differ in depth and purpose.

### Divergences

The API groups differ, as do version strategies and conversion management. Schema differences are significant: Observation v1 uses TTL and minimal status, whereas Ingestor and Source use phase-and-metrics status. Validation and defaults vary, and informer patterns are implemented in the legacy repository with dynamic factories and lifecycle management, while the ingester implementation presents CRDs designed for a centralized controller.

## Unified CRD Approach: Design Proposal

The proposed unified approach consolidates the CRDs into a single API group (“zen.watcher.io”), a single v1 storage version for stable resources, and a clear separation of concerns: Observation as the canonical event record and Ingestor as the pipeline controller. Conversion, if needed, is centralized via an explicit webhook with tests and monitoring. Validation is strengthened with CEL rules, and informer management is standardized across controllers.

Table 8 maps the proposed unified CRDs.

Table 8. Proposed unified CRD mapping

| Name            | Group           | Version | Scope       | Purpose                                                                                  | Key Spec Fields                                                                                                                    | Key Status Fields                                                                                                    | Subresources        |
|-----------------|-----------------|---------|-------------|------------------------------------------------------------------------------------------|------------------------------------------------------------------------------------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------|---------------------|
| Observation     | zen.watcher.io  | v1      | Namespaced  | Canonical event record for security/compliance/observability                             | source, category, severity, eventType, resource, details (preserve-unknown-fields), detectedAt, ttlSecondsAfterCreation            | processed (bool), lastProcessedAt (date-time); optional “synced” extension                                           | status: {}          |
| Ingestor        | zen.watcher.io  | v1      | Namespaced  | Unified ingestion pipeline controller                                                    | type, enabled, priority, environment, config (preserve-unknown-fields), filters, outputs, scheduling, healthCheck, security        | phase, lastScan, nextScan, observations, errors, lastError, healthScore, performance, conditions                     | status: {}; scale (if needed) |
| ObservationFilter (optional) | zen.watcher.io | v1alpha1 | Namespaced | Convenience filters for sources (if needed alongside Ingestor config)                    | targetSource, include/exclude lists (severity, eventTypes, namespaces, kinds, categories, rules), enabled                         | status: {}                                                                                                            | status: {}          |

### Canonical Observation CRD

The canonical Observation CRD should combine the strongest validation elements from the legacy v1 schema with pragmatic defaults and a minimal status. Required fields: “source,” “category,” “severity,” “eventType,” “detectedAt.” Optional fields include “resource” object, “details” (with preserve-unknown-fields), and “ttlSecondsAfterCreation” for garbage collection. The status should remain minimal—“processed” and “lastProcessedAt”—with an optional “synced” extension for environments that require explicit SaaS synchronization flags. Printer columns should include Source, Category, Severity, Processed, and Age.

### Canonical Ingestor CRD

The Ingestor CRD should consolidate the ingestion pipeline configuration: type (enum), enabled, priority, environment, config (provider-specific, preserve-unknown-fields), filters, outputs, scheduling (cron/interval/jitter/timezone), healthCheck, and security (encryption, RBAC, compliance, vault). Status should be rich with phases, metrics, and conditions. If horizontal scaling is required, the scale subresource can be adopted. Defaults should align with the ingester implementation: enabled default true; priority default “normal”; environment default “production”; health check intervals default “30s” with timeout “10s” and retries “3”.

### Optional Supporting CRDs

ObservationFilter can be retained as a convenience for operators who prefer declarative filter rules separate from Ingestor specs. ObservationMapping can be deprecated in favor of config-driven transformation fields within the Ingestor’s outputs and filters, reducing the number of CRDs and centralizing ingestion logic.

### Validation and Defaults Strategy

Adopt OpenAPI plus CEL rules for server-side validation. Required fields should be enforced, and enums should be canonicalized for severity, category, and event types. Defaults should be explicit: TTL minimum of 1 second; windowSeconds default 60 for any dedup logic; severity normalization default on; and health check intervals as described above. Preserve-unknown-fields should be limited to controlled sections (e.g., provider config and transformation details) to balance flexibility with safety.

### Informer Standardization

Adopt the mapping-driven informer pattern as the single mechanism for generic CRD sources. Standardize the dynamic informer factory with a common resync interval and a single lifecycle manager that uses a context and stop channel for shutdown. Remove ad hoc informers and centralize event handling for consistency. Cache sync checks should be mandatory before readiness.

### Conversion and Rollout

If conversion is required during migration, implement a single conversion webhook service with explicit conversion review versions, CA bundles, and allowed pathways (for example, v1beta1 to v1). Use dual-serving versions during rollout, with adoption metrics and clear rollback steps. Conversion behavior must be tested and monitored.

## Migration Plan and Risk Management

A phased migration reduces risk and ensures continuity. Each phase has entry and exit criteria, validation steps, and rollback plans. During migration, dual-serving versions and parallel controllers maintain compatibility.

Table 9 outlines the migration phases.

Table 9. Migration phases, entry/exit criteria, and validation

| Phase | Scope                                      | Entry Criteria                                         | Exit Criteria                                                                                  | Validation Steps                                                                                   | Rollback Plan                                                   |
|-------|--------------------------------------------|--------------------------------------------------------|-------------------------------------------------------------------------------------------------|----------------------------------------------------------------------------------------------------|-----------------------------------------------------------------|
| 1     | Define unified CRDs (Observation/Ingestor) | Stakeholder agreement on group/versions/schema         | CRDs drafted with CEL validation and defaults; Helm packaging ready                             | Schema linting; unit tests for required fields and defaults; example manifests validated           | Revert to drafts; no cluster changes                            |
| 2     | Dual-serve v1 and legacy versions          | Conversion webhook implemented (if needed)             | Both legacy and unified CRDs served and stored; status fields reconciled                       | Conversion tests (if applicable); cross-schema compatibility checks; informer alignment verification | Disable unified CRDs; revert controllers to legacy              |
| 3     | Migrate controllers to unified CRDs        | Dual-serving stable; mapping-driven informer pattern   | Controllers operate exclusively on unified CRDs; legacy CRDs removed                            | End-to-end tests; performance and latency checks; status accuracy validation                        | Re-enable legacy controllers; restore legacy CRDs               |
| 4     | Decommission legacy CRDs                   | Controllers migrated; adoption metrics acceptable      | Legacy CRDs deleted; Helm charts updated; documentation finalized                               | Cluster drift checks; RBAC cleanup; Helm release validation                                        | Reinstate CRDs from backups; rollback Helm charts               |

### Phasing and Gates

Promotion criteria should include test coverage thresholds, schema linting success, conversion validation, and operator sign-off. Monitoring and alerting must track status accuracy, informer cache health, and pipeline performance. Rollback readiness requires backups of CRDs and a clear reinstallation procedure.

### Validation and Rollback

Validation should include CRD schema tests (required fields, enums, patterns, defaults) and end-to-end tests for ingestion, filtering, and status updates. Rollback procedures must be documented and rehearsed, including reinstating legacy CRDs and disabling unified controllers.

## Implementation Roadmap

The roadmap translates the design proposal into concrete deliverables across code, schemas, controllers, and packaging. Owners and dependencies are assigned to ensure accountability and sequence.

Table 10 details the deliverables.

Table 10. Deliverables, owners, and dependencies

| Deliverable                                      | Description                                                                                  | Owner                  | Dependencies                              | Target Milestone        |
|--------------------------------------------------|----------------------------------------------------------------------------------------------|------------------------|-------------------------------------------|-------------------------|
| Unified Observation CRD (v1)                     | Canonical schema with minimal status, TTL, and printer columns                               | Platform Engineering   | Stakeholder review                        | M1                      |
| Unified Ingestor CRD (v1)                        | Consolidated pipeline configuration with rich status, defaults, and scale (optional)         | Platform Engineering   | Observation CRD definition                | M1                      |
| CEL validation rules                              | Server-side validation for required fields, enums, patterns                                  | Platform Engineering   | Unified CRDs                              | M2                      |
| Conversion webhook (if required)                 | Centralized service for version conversion with tests and monitoring                         | Core Platform          | Unified CRDs                              | M2                      |
| Informer standardization                         | Adopt mapping-driven pattern; common resync and shutdown lifecycle                           | Core Platform          | Controller refactor                       | M3                      |
| Controller updates                               | Migrate controllers to unified CRDs; deprecate legacy CRD usage                             | Core Platform          | Informer standardization                  | M3                      |
| Helm chart updates                               | Single packaging source; strict linting and version pinning                                  | DevOps                 | Unified CRDs                              | M3                      |
| Status conventions documentation                 | Document status semantics and condition usage across CRDs                                    | Platform Engineering   | Controller updates                        | M4                      |
| Migration playbook and runbooks                  | Step-by-step migration with validation and rollback                                          | SRE                    | All above                                 | M4                      |
| Deprecation and removal plan                     | Policy and schedule for legacy CRD removal                                                   | Platform Engineering   | Adoption metrics                           | M5                      |

### Workstreams and Timelines

The workstreams align to milestones M1–M5. M1 defines the CRDs and validation. M2 implements conversion and CEL rules. M3 standardizes informers, updates controllers, and Helm packaging. M4 documents conventions and migration. M5 executes deprecation and removal based on adoption metrics and operational stability.

## Appendix: Evidence and File Map

This appendix maps the key claims in the report to the reviewed artifacts and highlights areas where evidence is incomplete. It also defines schema elements referenced throughout the analysis.

### Artifact-to-Evidence Map

Table 11 maps major claims to specific artifacts and lines of evidence. To comply with the requirement to avoid inline hyperlinks, the table references artifact labels rather than URLs.

Table 11. Claim-to-artifact mapping

| Claim                                                                                                         | Artifact Label                                             | Section Reference                         |
|---------------------------------------------------------------------------------------------------------------|------------------------------------------------------------|-------------------------------------------|
| Legacy Observation CRD has dual versions (v1 storage, v2 served) with different required fields               | Legacy deployments Observation CRD                         | CRD Inventories; Definition & Management  |
| Helm Observation CRD is v1 storage with “synced” status                                                       | Helm template Observation CRD                              | System Overview; Status Handling          |
| ObservationFilter uses extensive enums and patterns                                                           | ObservationFilter CRD                                      | Validation and Defaults                   |
| ObservationMapping includes mappings and preserve-unknown-fields                                               | ObservationMapping CRD                                     | Validation and Defaults; Informer Patterns |
| ObservationDedupConfig applies defaults for windowSeconds and enabled                                          | ObservationDedupConfig CRD                                 | Validation and Defaults                   |
| Ingestor and Source declare conversion webhooks                                                                | Ingestor and Source CRDs                                   | Definition & Management; Lifecycle        |
| Ingestor status includes phases, metrics, conditions                                                            | Ingestor CRD                                               | Status Handling Patterns                  |
| Source status includes phases, metrics, conditions (lighter than Ingestor)                                     | Source CRD                                                 | Status Handling Patterns                  |
| Dynamic informer factory with resync period and lifecycle shutdown handling                                     | Kubernetes setup and lifecycle code                        | Informer Integration Patterns             |
| CRD-driven adapter watches ObservationMapping and starts per-source informers                                  | CRD adapter code                                           | Informer Integration Patterns             |

### Schema Elements Glossary

- Observation: Canonical event record with required fields (source, category, severity, eventType), optional resource and details, detectedAt, and ttlSecondsAfterCreation. Status includes processed and lastProcessedAt, with optional synced fields.  
- ObservationFilter: Declarative filter rules for sources, severities, event types, namespaces, kinds, categories, and rules, with an enabled flag.  
- ObservationMapping: Configuration to map fields from a source CRD to Observation fields, including severity normalization and resource references, with preserve-unknown-fields for details.  
- ObservationDedupConfig: Deduplication window per source with defaults for windowSeconds and enabled.  
- Ingestor: Unified ingestion pipeline configuration including type, enabled, priority, environment, config, filters, outputs, scheduling, health checks, and security. Status includes phases, metrics, and conditions.  
- Source: Lighter ingestion descriptor focusing on upstream provider configuration, filters, outputs, scheduling, and health checks, with status focused on phases and scan metrics.

### Incomplete Evidence

- Conversion webhook implementation for Ingestor and Source is not present; only declarations are available.  
- The existence of ObservationSourceConfig and ObservationTypeConfig is indicated, but their contents are not available.  
- Full details for ObservationSourceConfig and ObservationTypeConfig are missing, limiting complete mapping of configuration patterns.  
- Informer usage for ingester-side controllers is not evidenced in the reviewed code.  
- There is no explicit performance evaluation or sizing guidance for scale subresources.

---

By converging on a single group and version, a canonical Observation schema, and a unified Ingestor CRD—with standardized validation, informer lifecycle, and conversion management—Zen Watcher can materially reduce complexity and risk. The proposed design preserves the strengths of both ecosystems while aligning them to a single operational model. The phased migration and detailed roadmap provide a pragmatic path to adoption with clear gates and rollback procedures.