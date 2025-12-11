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

# CRD Naming Analysis

## Current Options Evaluated

### ❌ "Source" - TOO GENERIC
- Vague and non-descriptive
- Could be anything (data source, event source, etc.)
- No clear indication of function

### ❌ "Security" - TOO LIMITING  
- Immediately pigeonholes as security-only tool
- Kills multi-cloud/edge computing market potential
- Users might think it's limited to security use cases

### ✅ "Ingestor" - RECOMMENDED
**Why this wins:**
- **Clear function**: Ingests events, data, metrics from various sources
- **Future-proof**: Works for security, multi-cloud, edge computing, IoT
- **Technical precision**: DevOps teams understand "ingestor" concept
- **Market differentiation**: Most tools use "collector" or "agent"
- **API-friendly**: `apiVersion: zenwatcher.kube-zen.io/v1` + `kind: Ingestor`

### Alternative Options

#### "Collector"
- Familiar term but more generic
- Could be confused with data collection tools
- Less specific than "ingestor"

#### "Gateway" 
- Suggests it acts as a gateway for events
- Good for multi-cloud routing use cases
- Might imply network layer focus

#### "Connector"
- Suggests it connects different systems
- Good for integration use cases
- Less clear about the event ingestion function

#### "Pipeline"
- Suggests it creates data pipelines
- Great for multi-cloud data flow
- Might be confused with ETL pipelines

## Recommendation: "Ingestor"

**CRD Structure:**
```yaml
apiVersion: zenwatcher.kube-zen.io/v1
kind: Ingestor
metadata:
  name: security-events
spec:
  type: "webhook"  # or "configmap", "logs", "trivy", etc.
  config:
    # ... configuration
```

**Benefits:**
- Clear functional purpose
- Scalable across use cases
- Professional and technical
- Differentiated from existing solutions