# Per-Endpoint Rate Limiting: zen-sdk Consideration

**Date:** 2026-01-23  
**Question:** Should per-endpoint rate limiting be in zen-sdk?

---

## Analysis

### Current Implementation

**Location:** `zen-watcher/pkg/server/ratelimit_wrapper.go`

**What's Implemented:**
- Per-endpoint key extraction from URL paths
- HTTP middleware wrapper around `zen-sdk/pkg/gc/ratelimiter`
- Endpoint identifier extraction logic

### zen-sdk Rate Limiter

**Location:** `zen-sdk/pkg/gc/ratelimiter`

**What's in zen-sdk:**
- ✅ Token bucket rate limiter (`RateLimiter`)
- ✅ Low-level rate limiting primitives
- ✅ GC-focused (extracted from zen-gc)

**What's NOT in zen-sdk:**
- ❌ HTTP middleware
- ❌ URL path parsing
- ❌ Endpoint extraction logic
- ❌ Per-key management (cleanup, TTL)

---

## Recommendation: **Keep in zen-watcher**

### Rationale

1. **HTTP-Specific Logic**
   - Endpoint extraction from URL paths is HTTP-specific
   - zen-sdk should remain HTTP-agnostic
   - URL path parsing is application-specific

2. **zen-watcher-Specific Semantics**
   - Per-endpoint rate limiting is specific to zen-watcher's webhook use case
   - Other services may need different key extraction strategies
   - zen-sdk provides the primitive (`ratelimiter`), not the application logic

3. **Decoupling Maintained**
   - zen-watcher uses zen-sdk's rate limiter primitive
   - zen-watcher adds its own HTTP middleware layer
   - No circular dependencies

4. **zen-sdk Design Principle**
   - zen-sdk provides **primitives**, not **application logic**
   - HTTP middleware and URL parsing are application concerns
   - zen-sdk should remain lightweight and focused

---

## What Should Be in zen-sdk (Already There)

✅ **`zen-sdk/pkg/gc/ratelimiter`** - Token bucket rate limiter primitive
- Used by zen-watcher for per-key rate limiting
- Used by zen-gc for GC operation rate limiting
- Provides the core algorithm, not the application logic

---

## What Should Stay in zen-watcher

✅ **HTTP middleware wrapper** - `PerKeyRateLimiter`
- Manages per-key limiters (cleanup, TTL)
- Extracts keys from HTTP requests
- Provides HTTP-specific error responses

✅ **Endpoint extraction** - `getEndpointFromPath`
- URL path parsing is HTTP-specific
- zen-watcher-specific semantics (last segment = endpoint)
- Other services may need different extraction logic

---

## Alternative: Generic HTTP Rate Limiting Package

If multiple services need similar HTTP rate limiting middleware, consider:

**Option:** Create `zen-sdk/pkg/http/ratelimit` (future)
- Generic HTTP middleware for rate limiting
- Configurable key extraction function
- Still keeps endpoint extraction logic in zen-watcher

**Current Status:** Not needed yet - only zen-watcher uses this pattern

---

## Conclusion

**Keep per-endpoint rate limiting in zen-watcher.**

- zen-sdk provides the primitive (`ratelimiter`)
- zen-watcher adds HTTP-specific middleware and key extraction
- Maintains proper separation of concerns
- No architectural violations

**Future Enhancement:** If other services need similar HTTP rate limiting, consider extracting generic HTTP middleware to `zen-sdk/pkg/http/ratelimit`, but keep endpoint extraction logic service-specific.
