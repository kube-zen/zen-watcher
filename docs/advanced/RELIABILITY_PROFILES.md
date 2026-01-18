# Reliability Profiles

**Status:** ✅ **Architectural Guidance** | **Version:** 1.x | **Last Updated:** 2025-01-27

## Overview

Zen Watcher operates with **two runtime profiles**:

1. **Baseline (No Redis)** - Default, no external dependencies
2. **Reliability/HA (Redis Enabled)** - Optional acceleration for edge resilience

**Key Principle:** Redis is **not part of the North Star narrative**. It is a **reliability acceleration option** that enhances system capabilities but is **never required** for core functionality.

---

## Baseline Profile (Default)

**Characteristics:**
- ✅ **Zero external dependencies** - works with Kubernetes primitives only
- ✅ In-memory caches (bounded, safe defaults)
- ✅ Kubernetes-native leader election (etcd)
- ✅ Local rate limiting (per-pod)
- ✅ Local deduplication (per-pod)
- ✅ Direct destination delivery (no spool)

**When to Use:**
- Standard deployments
- Edge deployments where operational simplicity is priority
- Environments where Redis is not available or desired
- **Default for all deployments**

**Performance:**
- Handles 10,000+ events/day (single instance)
- Handles 100,000+ events/day (sharded)
- Handles 1,000,000+ events/day (distributed with etcd leader election)

---

## Reliability/HA Profile (Optional)

**Characteristics:**
- ✅ **Optional Redis dependency** - graceful degradation if unavailable
- ✅ Shared caches across replicas
- ✅ Shared rate limiting (token bucket across pods)
- ✅ Shared deduplication cache
- ✅ Spool/buffering during intermittent connectivity
- ✅ Fast coordination (leader election, distributed locks)
- ✅ Transient routing state (endpoint availability, backoff state)

**When to Use:**
- Edge deployments requiring resilience during WAN instability
- Multi-replica deployments needing shared state
- High-volume scenarios benefiting from shared rate limiting
- **Customer-managed edge** - optional "Reliability Profile" toggle

**Performance:**
- Same baseline performance + acceleration benefits
- Shared rate limiting prevents duplicate work
- Spool enables store-and-forward during connectivity issues

---

## Use Cases by Plane

### Edge Plane (Customer-Managed)

**Default:** Baseline (no Redis)

**Optional Reliability Profile** enables:
1. **Spool / Buffering** - Store-and-forward during intermittent connectivity
2. **Rate Limiting** - Token bucket shared across replicas
3. **Dedup/Idempotency Cache** - Shared across replicas
4. **Fast Coordination** - Leader election / distributed locks (if K8s primitives insufficient)
5. **Transient Routing State** - Endpoint availability hints, backoff state

**Recommendation:**
- Default: **no Redis**, rely on in-memory + K8s + durable destinations only
- Offer Redis as optional "Reliability Profile" toggle for customers requiring edge resilience
- **Risk:** Redis becomes customer operational burden (backups, upgrades, persistence mode)
- **Constraint:** Functional contract must be identical with or without Redis

### Data Plane (Zen-Managed)

**Default:** Optional but default-installed

**Use Cases:**
1. **Global Caches** - Routing metadata, policy decisions, short-lived enrollment artifacts
2. **Rate Limiting** - Shared ingress rate limiting
3. **Idempotency/Dedup Cache** - At scale across workers
4. **Backpressure Coordination** - Across workers

**Recommendation:**
- Make it "optional" in charts for flexibility
- Operationally plan to install by default in all DP clusters (managed by Zen)
- **Treat Redis as non-authoritative** - if Redis is down, system degrades gracefully (reduced performance), not unsafe or lose integrity

---

## What NOT to Use Redis For

**Do not make Redis the source of truth for:**

❌ **Receipts/Audit Logs** - Use durable stores or customer-owned destinations  
❌ **Durable Queues** - Unless explicitly choosing Redis Streams with durability (accept tradeoffs)  
❌ **Core Identity/Enrollment State** - Use durable stores or customer-owned destinations

**These belong in durable stores or customer-owned destinations.**

---

## Implementation Approach

### Feature Flags / Config Keys

Switch implementations without changing external behavior:

```yaml
# Configuration options
reliability:
  profile: baseline | redis-enabled
  
  # Component-level toggles
  idempotencyStore: memory | redis
  rateLimiter: local | redis
  spool: none | redis | disk  # disk spool alternative to Redis at edge
```

### Health Semantics

**Redis Down Behavior:**
- ✅ **Degrade gracefully** - disable shared features, fall back to baseline
- ✅ **Emit clear status** - metrics and logs indicate Redis unavailability
- ✅ **Do not hard-fail** - unless customer explicitly requires it
- ✅ **Maintain integrity** - no data loss, no unsafe operations

**Example Health Check:**
```yaml
status:
  reliability:
    profile: redis-enabled
    redis:
      available: false
      degraded: true
      fallback: baseline
    components:
      idempotencyStore: memory  # fallback from redis
      rateLimiter: local        # fallback from redis
      spool: none              # fallback from redis
```

---

## Practical Recommendations

### Edge Use Cases

**If you share which edge use cases you expect first**, the Redis feature set can be scoped to the minimal needed subset:

- **Webhook capture burstiness** → Spool + rate limiting
- **Tunnel reliability** → Spool + coordination
- **Fan-out buffering** → Spool + routing state

### Data Plane

- Keep optional in manifests for flexibility
- Deploy by default and design as accelerator, not dependency
- Plan for graceful degradation in all code paths

---

## Migration Path

**From Baseline to Reliability Profile:**

1. Deploy Redis (optional, customer-managed at edge)
2. Enable reliability profile via config
3. System automatically uses Redis when available
4. System automatically falls back to baseline if Redis unavailable

**No code changes required** - feature flags handle the switch.

---

## Summary

| Aspect | Baseline | Reliability/HA |
|--------|----------|----------------|
| **Dependencies** | None (K8s only) | Optional Redis |
| **Default** | ✅ Yes | ❌ No (optional) |
| **Edge** | ✅ Default | ⚠️ Optional toggle |
| **Data Plane** | ✅ Supported | ✅ Default-installed |
| **Failure Mode** | N/A | Graceful degradation |
| **Source of Truth** | etcd (K8s) | etcd (K8s) - Redis is cache only |

**Key Takeaway:** Redis is an **acceleration layer**, not a **dependency**. The system is fully functional and production-ready without it.
