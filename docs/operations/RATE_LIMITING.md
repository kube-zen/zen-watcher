# Rate Limiting

**Purpose:** Per-endpoint and per-IP rate limiting for webhook requests  
**Status:** ✅ Implemented  
**Last Updated:** 2026-01-23

---

## Overview

zen-watcher implements rate limiting for webhook endpoints to prevent abuse and ensure fair resource usage. The rate limiter supports two scoping strategies:

1. **Per-endpoint rate limiting** - For multi-segment URL paths (e.g., `/prefix/endpoint-name`)
2. **Per-IP rate limiting** - For single-segment or legacy paths (backward compatibility)

**Architecture Note:** zen-watcher is decoupled and only cares about endpoint identifiers extracted from URL paths. It does not have knowledge of tenant/namespace concepts.

---

## Configuration

### Environment Variables

**`WEBHOOK_RATE_LIMIT`** (optional)
- **Default:** 100 requests per minute
- **Description:** Maximum requests per minute per endpoint/IP
- **Example:** `WEBHOOK_RATE_LIMIT=200` (200 requests per minute)

---

## Rate Limiting Behavior

### Per-Endpoint Rate Limiting

**Trigger:** Multi-segment URL paths (e.g., `/prefix/endpoint-name`, `/namespace/endpoint`)

**Key Extraction:**
- Extracts the **last segment** of the URL path as the endpoint identifier
- Uses `endpoint:<endpoint-name>` as the rate limit key
- Provides endpoint-level isolation without coupling to tenant/namespace concepts

**Example:**
```
URL: /some/prefix/security-alerts
→ Endpoint identifier: "security-alerts"
→ Rate limit key: "endpoint:security-alerts"
```

### Per-IP Rate Limiting

**Trigger:** Single-segment URL paths (legacy compatibility)

**Key Extraction:**
- Uses client IP address as the rate limit key
- Maintains backward compatibility with existing deployments

**Example:**
```
URL: /falco/webhook
→ Rate limit key: "192.168.1.100" (client IP)
```

---

## Response Format

When rate limit is exceeded, the server returns:

**HTTP Status:** `429 Too Many Requests`

**Headers:**
- `Retry-After: 60` (seconds)
- `Content-Type: application/json`

**Response Body:**
```json
{
  "error": "rate limit exceeded",
  "endpoint": "security-alerts",
  "retry_after": 60
}
```

---

## Metrics

### Rate Limit Rejections

**Metric:** `zen_watcher_webhook_rate_limit_rejections_total`

**Labels:**
- `endpoint` - Endpoint identifier (extracted from URL path)
- `scope` - Rate limit scope: `"endpoint"` or `"ip"`

**Example:**
```promql
# Rate limit rejections per endpoint
rate(zen_watcher_webhook_rate_limit_rejections_total{scope="endpoint"}[5m])

# Rate limit rejections per IP (legacy)
rate(zen_watcher_webhook_rate_limit_rejections_total{scope="ip"}[5m])
```

### Webhook Requests (includes 429)

**Metric:** `zen_watcher_webhook_requests_total`

**Labels:**
- `endpoint` - Endpoint identifier
- `status` - HTTP status code (includes `"429"` for rate limited requests)

**Example:**
```promql
# Rate limit rejection rate
sum(rate(zen_watcher_webhook_requests_total{status="429"}[5m])) / 
sum(rate(zen_watcher_webhook_requests_total[5m]))
```

---

## Implementation Details

### Rate Limiter

**Package:** `zen-watcher/pkg/server`

**Implementation:**
- Uses `zen-sdk/pkg/gc/ratelimiter` for token bucket algorithm
- Wraps SDK rate limiter with per-key management
- Automatic cleanup of inactive limiters (1 hour TTL)

### Endpoint Extraction

**Function:** `getEndpointFromPath(path string) string`

**Behavior:**
- Multi-segment paths: Returns last segment (endpoint identifier)
- Single-segment paths: Returns the segment as-is
- Empty/invalid paths: Returns `"unknown"`

**Examples:**
```go
getEndpointFromPath("/falco/webhook")        // → "webhook"
getEndpointFromPath("/prefix/endpoint-name") // → "endpoint-name"
getEndpointFromPath("/endpoint")             // → "endpoint"
getEndpointFromPath("/")                     // → "unknown"
```

---

## Monitoring and Alerting

### Recommended Alerts

**High Rate Limit Rejection Rate:**
```yaml
- alert: HighRateLimitRejections
  expr: |
    rate(zen_watcher_webhook_rate_limit_rejections_total[5m]) > 10
  for: 5m
  annotations:
    summary: "High rate limit rejection rate detected"
    description: "Rate limit rejections exceed 10/sec for 5 minutes"
```

**Rate Limit Rejection Percentage:**
```yaml
- alert: ExcessiveRateLimitRejections
  expr: |
    (sum(rate(zen_watcher_webhook_requests_total{status="429"}[5m])) / 
     sum(rate(zen_watcher_webhook_requests_total[5m]))) > 0.1
  for: 5m
  annotations:
    summary: "More than 10% of requests are rate limited"
    description: "Rate limit rejection rate exceeds 10%"
```

---

## Troubleshooting

### Rate Limits Too Aggressive

**Symptom:** Legitimate requests getting 429 responses

**Solution:**
1. Increase `WEBHOOK_RATE_LIMIT` environment variable
2. Check metrics to identify which endpoints are hitting limits
3. Consider per-endpoint configuration (future enhancement)

### Rate Limits Not Working

**Symptom:** No 429 responses even with high request volume

**Check:**
1. Verify `WEBHOOK_RATE_LIMIT` is set correctly
2. Check rate limit metrics: `zen_watcher_webhook_rate_limit_rejections_total`
3. Verify rate limiter is initialized in server logs

---

## Future Enhancements

1. **Per-endpoint configuration** - Configure rate limits per endpoint via Ingester CRD
2. **Dynamic rate limit adjustment** - Auto-adjust based on load
3. **Rate limit analytics** - Track rate limit effectiveness and trends

---

**See Also:**
- [Observability Guide](OBSERVABILITY.md) - Complete metrics reference
- [Security Guide](../security/SECURITY.md) - Security hardening
- [Troubleshooting Guide](TROUBLESHOOTING.md) - Common issues
