---
category: OPERATIONS
purpose: Analysis and recommendations for reducing zen-watcher Docker image size
status: current
last_updated: 2025-12-20
---

# Zen-Watcher Image Size Optimization

## Current Image Size

**Total**: ~31MB (30.8MB measured)

| Component | Size | Status |
|-----------|------|--------|
| Compiled Binary | ~27MB | Required (Kubernetes client-go + Prometheus) |
| Timezone Data | ~2MB | **Unnecessary - can be removed** |
| Distroless Base | ~2MB | Optimal choice |
| CA Certificates | ~200KB | Required for HTTPS/TLS |

## Optimization: Remove Timezone Data

**Why**: Timezone data (`/usr/share/zoneinfo`) is **not needed**.

- Code uses `time.Now()`, `time.Parse()`, `time.Format()` - these work without timezone database
- **No `time.LoadLocation()` calls** found in codebase
- Timezone database is only needed for `time.LoadLocation()` which we don't use

**Action**: Remove this line from `build/Dockerfile`:
```dockerfile
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
```

**Result**: ~29MB image (saves ~2MB, 6% reduction)

**Risk**: None - timezone data is unused

---

## Why Binary Is 27MB

The binary size is primarily from required dependencies:

- **Kubernetes client-go** (~15-20MB): Required for CRD operations, informers, API access
- **Prometheus libraries** (~3-5MB): Required for metrics collection
- **Other dependencies** (~2-4MB): Logging, YAML parsing, HTTP clients

The binary is already optimized:
- Stripped with `-w -s` flags
- Static binary (`CGO_ENABLED=0`)
- No unnecessary dependencies

**Conclusion**: Binary size is reasonable for a Kubernetes-native application. No further optimization needed.

---

## What We're NOT Doing

### ❌ UPX Compression
- **Why not**: Adds startup overhead (~100-200ms), security scanner flags, debugging complexity
- **When to consider**: Only for edge deployments with extreme bandwidth constraints
- **OSS best practice**: Keep it simple - don't add complexity for marginal gains

### ❌ Removing Dependencies
- Kubernetes client-go and Prometheus are core functionality
- Removing them would break the application
- Not worth the size savings

---

## Recommended Change

**Remove timezone data** from Dockerfile:

```dockerfile
# Final stage - Distroless for minimal attack surface
FROM gcr.io/distroless/static:nonroot

# Copy CA certificates (required for HTTPS/TLS)
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy compiled binary
COPY --from=builder /build/zen-watcher /zen-watcher

# Note: Timezone data removed - not needed (no LoadLocation calls)
```

**Expected Result**: ~29MB image (from ~31MB)

---

## Testing

After removing timezone data, verify:
- [ ] Container starts successfully
- [ ] Time operations work (`time.Now()`, `time.Parse()`)
- [ ] Event timestamps are accurate
- [ ] No errors related to timezone operations

---

## Related Documentation

- [Dockerfile](../build/Dockerfile) - Current build configuration
- [Performance Guide](PERFORMANCE_GUIDE.md) - Resource usage benchmarks
