# Deduplication

Zen Watcher uses multi-layered noise reduction to prevent alert fatigue and etcd bloat. All deduplication logic is centralized in `pkg/dedup/deduper.go` and shared across all event processors.

## Overview

Deduplication prevents duplicate Observations from being created when the same event is detected multiple times within a configurable time window. This reduces etcd churn, prevents alert fatigue, and ensures efficient resource usage.

## Deduplication Strategy

### Content-Based Fingerprinting (SHA-256)

The primary deduplication mechanism uses SHA-256 hashing of normalized event content:

- **Hash Input**: Normalized observation content including:
  - Source
  - Category
  - Severity
  - Event type
  - Resource (kind, name, namespace)
  - Critical details (normalized)

- **Algorithm**: SHA-256 hash of the normalized payload
- **Purpose**: Accurate duplicate detection based on event content, not just message text

This ensures that events with identical content are recognized as duplicates even if they arrive from different sources or at different times.

### Per-Source Token Bucket Rate Limiting

Prevents one noisy tool from overwhelming the system:

- **Algorithm**: Token bucket per source
- **Configuration**:
  - `DEDUP_MAX_RATE_PER_SOURCE`: Maximum events per second per source (default: 100)
  - `DEDUP_RATE_BURST`: Burst capacity (default: 200, 2x rate limit)
- **Purpose**: Prevents observation floods from a single misconfigured or noisy source

### Time-Bucketed Deduplication

Collapses repeating events within configurable windows:

- **Window**: Configurable via `DEDUP_WINDOW_SECONDS` (default: 60 seconds)
- **Per-Source Windows**: Configurable via `DEDUP_WINDOW_BY_SOURCE` (JSON format, e.g., `{"cert-manager": 86400, "falco": 60}`)
- **Max Size**: Configurable via `DEDUP_MAX_SIZE` (default: 10,000 entries)
- **Algorithm**: Sliding window with LRU eviction and TTL cleanup
- **Time Buckets**: Events organized into time buckets for efficient cleanup (configurable via `DEDUP_BUCKET_SIZE_SECONDS`)

### LRU Eviction

Efficient memory management:

- **Strategy**: Least Recently Used (LRU) eviction when cache reaches maximum size
- **Purpose**: Prevents unbounded memory growth while maintaining recent deduplication state

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DEDUP_WINDOW_SECONDS` | Deduplication window in seconds | `60` |
| `DEDUP_MAX_SIZE` | Maximum deduplication cache size | `10000` |
| `DEDUP_BUCKET_SIZE_SECONDS` | Time bucket size for deduplication cleanup | `10` (or 10% of window) |
| `DEDUP_MAX_RATE_PER_SOURCE` | Maximum events per second per source | `100` |
| `DEDUP_RATE_BURST` | Burst capacity for rate limiting | `200` (2x rate limit) |
| `DEDUP_ENABLE_AGGREGATION` | Enable event aggregation in rolling window | `true` |

### Example Configuration

```yaml
env:
  - name: DEDUP_WINDOW_SECONDS
    value: "120"  # 2-minute default window
  - name: DEDUP_WINDOW_BY_SOURCE
    value: '{"cert-manager": 86400, "falco": 60, "default": 120}'  # Per-source windows
  - name: DEDUP_MAX_SIZE
    value: "20000"  # Larger cache for high-volume deployments
  - name: DEDUP_MAX_RATE_PER_SOURCE
    value: "200"  # Higher rate limit for busy sources
```

## How It Works

### Processing Flow

Deduplication is part of the centralized processing pipeline:

```mermaid
graph LR
    A[Event Source] --> B[FILTER]
    B -->|if allowed| C[NORMALIZE]
    C --> D[DEDUP<br/>This step]
    D -->|if not duplicate| E[CREATE CRD]
    E --> F[METRICS & LOG]
    
    style D fill:#e3f2fd
```

**Deduplication Steps:**

1. **Event Received**: Event arrives from any source (informer, webhook, ConfigMap)
2. **Content Normalization**: Event content is normalized (severity, category, etc.) - see [NORMALIZATION.md](NORMALIZATION.md)
3. **SHA-256 Fingerprinting**: Normalized content is hashed using SHA-256
4. **Rate Limiting Check**: Per-source token bucket is checked
5. **Deduplication Check**: SHA-256 hash is checked against deduplication cache
6. **Cache Update**: If not duplicate, hash is added to cache with timestamp
7. **Observation Creation**: If not duplicate, Observation CRD is created

See [ARCHITECTURE.md](ARCHITECTURE.md#2-event-processing-pipeline) for the complete pipeline documentation.

### Thread Safety

- All deduplication logic is thread-safe
- All processors share the same deduper instance
- Mutex-protected cache operations
- Safe for concurrent access from multiple goroutines

## Metrics

Deduplication effectiveness can be monitored via Prometheus metrics:

- `zen_watcher_observations_deduped_total`: Total observations skipped due to deduplication
- `zen_watcher_observations_created_total`: Total observations created

**Deduplication Ratio**:
```
rate(zen_watcher_observations_deduped_total[5m]) / 
  (rate(zen_watcher_observations_created_total[5m]) + rate(zen_watcher_observations_deduped_total[5m]))
```

## Performance Characteristics

- **CPU Impact**: <100ms CPU spikes even under firehose conditions
- **Memory Usage**: ~8MB for 10,000 entry cache (configurable)
- **Lookup Time**: O(1) hash map lookups
- **Cleanup**: Background goroutine for efficient memory management

## Best Practices

1. **Tune Window Size**: Adjust `DEDUP_WINDOW_SECONDS` for default window, or use `DEDUP_WINDOW_BY_SOURCE` for per-source configuration
   - Short window (30-60s): For rapidly changing events (e.g., runtime security)
   - Long window (hours/days): For stable, repeating events (e.g., certificate expiration)
   - Example: `{"cert-manager": 86400}` sets 24-hour window for cert-manager to avoid flooding etcd with certificate expiration events

2. **Monitor Deduplication Ratio**: High ratio (>50%) may indicate:
   - Duplicate sources configured
   - Misconfigured tools sending duplicate events
   - Network retries causing duplicate webhooks

3. **Adjust Cache Size**: For high-volume deployments, increase `DEDUP_MAX_SIZE`
   - Monitor memory usage
   - Balance between deduplication effectiveness and memory

4. **Rate Limiting**: Adjust `DEDUP_MAX_RATE_PER_SOURCE` for noisy sources
   - Prevents one source from overwhelming the system
   - Burst capacity allows handling traffic spikes

## Troubleshooting

### High Deduplication Ratio

If deduplication ratio is consistently high (>50%):

1. Check for duplicate source configurations
2. Verify webhook endpoints aren't being called multiple times
3. Check if multiple instances are processing the same events
4. Review source tool configurations for duplicate event generation

### Memory Usage

If memory usage is high:

1. Reduce `DEDUP_MAX_SIZE` if deduplication effectiveness is acceptable
2. Reduce `DEDUP_WINDOW_SECONDS` to allow faster cache cleanup
3. Monitor cache eviction patterns

### Events Not Being Deduplicated

If duplicate events are still being created:

1. Verify deduplication is enabled (`DEDUP_ENABLE_AGGREGATION=true`)
2. Check deduplication window is appropriate for event frequency
3. Verify cache size is sufficient (`DEDUP_MAX_SIZE`)
4. Check logs for deduplication errors

## Implementation Details

### Code Location

- **Main Implementation**: `pkg/dedup/deduper.go`
- **SHA-256 Hashing**: `GenerateFingerprint()` method
- **Token Bucket**: Per-source rate limiting
- **Cache Management**: LRU eviction with time-based cleanup

### Integration Points

Deduplication is integrated into the centralized `ObservationCreator`:

- All event processors use the same deduper instance
- Consistent behavior across all sources
- Single point of configuration

## See Also

- [NORMALIZATION.md](NORMALIZATION.md) - How events are normalized before deduplication
- [Architecture](ARCHITECTURE.md) - System architecture overview
- [Performance](PERFORMANCE.md) - Performance characteristics and tuning
- [Operations](OPERATIONAL_EXCELLENCE.md) - Operational guidance and monitoring

