# Metrics Review After Security Changes

**Date**: 2025-01-15  
**Context**: Review of metrics coverage after implementing security improvements (auth bypass fix, IP spoofing protection, rate limiter cleanup, pprof protection, request body size limits)

## Summary

After reviewing the metrics coverage for the recent security changes, here are the findings:

### ✅ Already Covered

1. **HTTP 413 (Request Entity Too Large)** - ✅ **Tracked**
   - Status code `413` is tracked via `webhookMetrics.WithLabelValues("falco", "413")` and `("audit", "413")`
   - Metric: `zen_watcher_webhook_requests_total{endpoint="falco|audit", status="413"}`
   - Implemented in: `pkg/server/http.go` (lines 348, 434)

2. **Rate Limiter Cleanup (TTL-based)** - ✅ **No metrics needed**
   - This is an internal implementation detail (memory management)
   - Cleanup is logged at DEBUG level for troubleshooting

3. **pprof Protection (localhost-only binding)** - ✅ **No metrics needed**
   - This is a security configuration change
   - pprof endpoints are separate and don't need metrics

### ⚠️ Gaps Identified

1. **HTTP 401 (Unauthorized) - Authentication Failures** - ❌ **NOT tracked**
   - **Location**: `pkg/server/auth.go:230` (`RequireAuth` middleware)
   - **Issue**: Authentication failures return HTTP 401 but don't increment metrics
   - **Impact**: Cannot monitor authentication failures in Prometheus
   - **Recommendation**: Add metrics tracking for 401 responses

2. **HTTP 429 (Too Many Requests) - Rate Limit Rejections** - ❌ **NOT tracked**
   - **Location**: `pkg/server/ratelimit_wrapper.go:136` (`RateLimitMiddleware`)
   - **Issue**: Rate limit rejections return HTTP 429 but don't increment metrics
   - **Impact**: Cannot monitor rate limit rejections in Prometheus
   - **Recommendation**: Add metrics tracking for 429 responses

3. **Generic Webhook Adapter - No Metrics** - ❌ **NOT tracked**
   - **Location**: `pkg/adapter/generic/webhook_adapter.go`
   - **Issue**: Generic webhook adapter has no metrics at all
   - **Impact**: Cannot monitor generic webhook requests, errors, or rejections
   - **Recommendation**: Add metrics tracking for generic webhook adapter (401, 413, 400, 503, 200)

## Technical Constraints

The gaps for 401 and 429 are due to middleware architecture:
- `RequireAuth` middleware runs **before** the handler, so it doesn't have access to `webhookMetrics`
- `RateLimitMiddleware` runs **before** the handler, so it doesn't have access to `webhookMetrics`
- These middlewares need access to metrics to track rejections

## Recommendations

### High Priority

1. **Add metrics tracking for 401 responses**
   - Pass `webhookMetrics` to `RequireAuth` middleware
   - Increment metric before returning 401: `webhookMetrics.WithLabelValues(endpoint, "401").Inc()`

2. **Add metrics tracking for 429 responses**
   - Pass `webhookMetrics` to `RateLimitMiddleware`
   - Increment metric before returning 429: `webhookMetrics.WithLabelValues(endpoint, "429").Inc()`

### Medium Priority

3. **Add metrics tracking for generic webhook adapter** — ✅ COMPLETED
   - Integrated with existing `zen_watcher_webhook_requests_total` metric
   - Tracks all HTTP status codes (200, 400, 401, 413, 503)
   - Uses source name as endpoint label (same metric as main server webhooks)
   - Implemented via factory pattern with metrics passed from orchestrator

## Current Metrics Reference

Existing webhook metrics:
- `zen_watcher_webhook_requests_total{endpoint, status}` - Total webhook requests by endpoint and status
- `zen_watcher_webhook_events_dropped_total{endpoint}` - Dropped events (backpressure)

Currently tracked status codes:
- `200` - Success (falco, audit, generic webhook adapter) ✅
- `400` - Bad Request (falco, audit, generic webhook adapter) ✅
- `401` - Unauthorized (falco, audit, generic webhook adapter) ✅ **FIXED**
- `405` - Method Not Allowed (falco, audit)
- `413` - Request Entity Too Large (falco, audit, generic webhook adapter) ✅ **NEW**
- `429` - Too Many Requests (falco, audit) ✅ **FIXED**
- `503` - Service Unavailable (falco, audit, generic webhook adapter) ✅

Missing status codes:
- None (all security-related status codes are now tracked) ✅

## Documentation Impact

No documentation updates needed immediately, as:
- HTTP 413 is already tracked (no documentation mentions it yet, but it's correctly implemented)
- HTTP 401 and 429 gaps are implementation issues, not documentation issues
- Generic webhook adapter metrics gap is an implementation issue

However, once metrics are added:
- Update `docs/OBSERVABILITY.md` to document new status codes
- Update `docs/SECURITY.md` to include 401/429 metrics in monitoring examples
- Update `docs/OPERATIONAL_EXCELLENCE.md` if these metrics are used in SLOs

## Conclusion

All security changes are correctly implemented, and metrics coverage is now complete:
- ✅ HTTP 413 is correctly tracked (request body size limits) - Falco, Audit, and Generic webhook adapters
- ✅ HTTP 401 (auth failures) are now tracked - Falco, Audit, and Generic webhook adapters
- ✅ HTTP 429 (rate limit rejections) are now tracked - Falco and Audit webhooks (rate limiter middleware)
- ✅ Generic webhook adapter metrics are now implemented - All HTTP status codes (200, 400, 401, 413, 503) tracked

All identified gaps have been addressed. The metrics implementation is complete and provides comprehensive observability for all webhook endpoints.

