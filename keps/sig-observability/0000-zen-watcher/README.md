---
title: Zen Watcher - Kubernetes Security Event Aggregator
owning-sig: sig-observability  # Primary SIG for observability infrastructure
participating-sigs:
  - sig-security
  - sig-architecture
status: draft
creation-date: 2025-12-08
last-updated: 2025-12-08
reviewers:
  - TBD
approvers:
  - TBD
---

# Zen Watcher - Kubernetes Security Event Aggregator

## Summary

Zen Watcher is a Kubernetes operator that aggregates structured signals from security, compliance, and infrastructure tools into unified `Observation` CRDs. It provides a lightweight, standalone solution for security observability with intelligent filtering, deduplication, and auto-optimization.

## Motivation

Kubernetes clusters generate security events from multiple sources (Trivy, Falco, Kyverno, etc.), but there's no unified way to:
- Aggregate events from all sources
- Normalize event formats
- Filter noise and deduplicate events
- Provide consistent observability

Zen Watcher solves this by providing a single aggregation point that:
- Collects events from 9+ sources
- Normalizes to a standard Observation CRD format
- Applies intelligent filtering and deduplication
- Provides comprehensive Prometheus metrics

## Goals

1. **Unified Event Aggregation**: Single point for all security/compliance events
2. **Zero Blast Radius**: Core component requires zero secrets, zero egress
3. **Intelligent Processing**: Auto-optimization, filtering, deduplication
4. **Production Ready**: Minimal resource footprint, comprehensive observability

## Non-Goals

- Remediation actions (handled by external controllers)
- External system integrations (handled by sync controllers)
- Long-term storage (uses TTL for cleanup)

## Design Details

### Architecture

- **Pure Core**: Only writes to etcd, no external dependencies
- **Extensible Ecosystem**: Optional sync controllers for external systems
- **Adapter Pattern**: First-class adapters for 9 sources + generic CRD adapter
- **Intelligent Pipeline**: Dynamic processing order, adaptive filtering

### Performance Characteristics

- **Baseline Resource Usage**: 2m CPU, 35MB memory
- **Sustained Throughput**: 16-22 observations/second
- **Burst Capacity**: 15-18 observations/second
- **Scale**: Tested with 10,000+ observations

### Stress Testing Validation (2025-12-08)

Comprehensive stress testing was performed to validate performance claims:

#### Test Results

- **Sustained Load**: 16-22 obs/sec (validated with 2000 observations)
- **Burst Capacity**: 15-18 obs/sec (validated with 500 observations)
- **Multi-Phase Stress**: 18-22 avg, 28-32 peak obs/sec (validated with 5000 observations)
- **Scale Testing**: 10,000 observations processed successfully

#### Updated Performance Claims

- **Sustained Throughput**: 16-22 obs/sec (validated through stress testing)
- **Burst Capacity**: 15-18 obs/sec (validated)
- **Resource Impact**: Predictable linear scaling
- **Recovery**: <60 seconds for burst loads

See [Stress Testing Results](../../../docs/STRESS_TEST_RESULTS.md) for complete details.

## Implementation History

- **2025-12-08**: Initial KEP creation
- **2025-12-08**: Stress testing validation, SIG assignment update

## Alternatives Considered

1. **Direct Integration**: Each tool writes directly to external systems
   - Rejected: Too many integration points, inconsistent formats

2. **Centralized Log Aggregation**: Use existing log aggregation tools
   - Rejected: Doesn't provide structured event model, harder to query

3. **Custom CRDs per Tool**: Each tool creates its own CRD
   - Rejected: No normalization, harder to correlate events

## References

- [Zen Watcher Documentation](../../../docs/)
- [Stress Testing Results](../../../docs/STRESS_TEST_RESULTS.md)
- [Architecture Guide](../../../docs/ARCHITECTURE.md)

