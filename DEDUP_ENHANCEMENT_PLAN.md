# Deduplication Enhancement Plan

## Current State Analysis

The current dedup implementation is basic:
- Simple TTL-based cache with sliding window
- LRU eviction
- Message hash for basic fingerprinting
- No time-based buckets
- No content-based fingerprinting
- No rate limiting
- No event aggregation

## Required Enhancements

### 1. Time-based Dedup Buckets
- Organize events into time buckets (e.g., 1-minute buckets)
- Check duplicates within the same bucket more efficiently
- Clean up old buckets automatically

### 2. Fingerprint-based Dedup
- Create content-based fingerprints using SHA256 of normalized observation content
- Include key fields: source, category, severity, eventType, resource, critical details
- More accurate than message-only hashing

### 3. Rate Limiting
- Per-source rate limiting (e.g., 100 events/second per source)
- Token bucket algorithm with burst capacity
- Prevent observation floods

### 4. Event Aggregation (Rolling Window)
- Aggregate similar events within a rolling window
- Track count of duplicate occurrences
- Enable creating aggregated observations when thresholds are met

## Implementation Strategy

Since the user wants "normalization" (not a rewrite), we should:

1. Enhance the existing `Deduper` struct with optional features
2. Add new methods while keeping backward compatibility
3. Use environment variables for configuration
4. Make features opt-in via feature flags if needed

## Configuration

New environment variables:
- `DEDUP_BUCKET_SIZE_SECONDS` - Size of time buckets (default: 10)
- `DEDUP_ENABLE_FINGERPRINT` - Enable fingerprint-based dedup (default: true)
- `DEDUP_ENABLE_RATE_LIMIT` - Enable rate limiting (default: true)
- `DEDUP_MAX_RATE_PER_SOURCE` - Max events per second per source (default: 100)
- `DEDUP_RATE_BURST` - Burst capacity for rate limiting (default: 200)
- `DEDUP_ENABLE_AGGREGATION` - Enable event aggregation (default: true)

## Backward Compatibility

All enhancements should:
- Default to enabled but with safe defaults
- Not break existing API contracts
- Be optional and configurable
- Maintain existing behavior when disabled

