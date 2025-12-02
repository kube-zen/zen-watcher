# Deduplication Status Check

## Current Implementation Analysis

After reviewing the dedup code in `pkg/dedup/deduper.go`, here's what exists:

### ✅ What's Currently Implemented

1. **Basic TTL-based cache** - Sliding window with configurable duration
2. **LRU eviction** - Removes oldest entries when cache is full
3. **Message hashing** - Uses SHA256 hash of message (first 8 bytes)
4. **Key-based dedup** - Uses source/namespace/kind/name/reason/messageHash

### ❌ What's Missing (Required for v0.1.0)

1. **Time-based dedup buckets** - Events should be organized into time buckets for efficient cleanup
2. **Fingerprint-based dedup** - Content-based fingerprinting (not just message hash) using normalized observation content
3. **Rate limiting** - No limiter to prevent observation flood per source
4. **Event aggregation** - No rolling window aggregation of similar events

## Required Enhancements

### 1. Time-based Dedup Buckets

**Current:** All events in single cache with TTL cleanup
**Needed:** Organize events into time buckets (e.g., 1-minute buckets)
- More efficient cleanup (remove entire buckets)
- Faster duplicate checks within buckets
- Better memory management

### 2. Fingerprint-based Dedup

**Current:** Only hashes message field
**Needed:** Content-based fingerprint from normalized observation
- Include: source, category, severity, eventType, resource, critical details
- More accurate duplicate detection
- Better than message-only hashing

### 3. Rate Limiting

**Current:** No rate limiting
**Needed:** Token bucket per source
- Prevent floods (e.g., max 100 events/second per source)
- Burst capacity (e.g., 200 events)
- Configurable limits

### 4. Event Aggregation (Rolling Window)

**Current:** No aggregation
**Needed:** Track and aggregate similar events
- Count occurrences within window
- Enable creating aggregated observations
- Reduce noise from repeated events

## Implementation Priority

All four features are required. Implementation order:
1. Time-based buckets (foundation for efficient cleanup)
2. Fingerprint-based dedup (better accuracy)
3. Rate limiting (prevent floods)
4. Event aggregation (reduce noise)

## Configuration Needed

New environment variables:
- `DEDUP_BUCKET_SIZE_SECONDS` - Bucket size (default: 10 seconds)
- `DEDUP_MAX_RATE_PER_SOURCE` - Rate limit per source (default: 100/sec)
- `DEDUP_RATE_BURST` - Burst capacity (default: 200)
- `DEDUP_ENABLE_AGGREGATION` - Enable aggregation (default: true)

## Next Steps

Enhance the existing `Deduper` struct to add these features while maintaining backward compatibility.

