# Stability Guarantees

**Version:** 1.2.1  
**Last Updated:** 2025-01-05

This document defines what zen-watcher **will never do**, what stability guarantees we provide, and our breaking change policy.

---

## What We Never Do

### 1. We Never Delete Your Cluster Resources

**Guarantee:** Zen Watcher **never deletes, modifies, or patches** any Kubernetes resources except:
- `Observation` CRDs (which it creates)
- Leader election Leases (for HA coordination)

**What This Means:**
- ✅ Safe to run in production
- ✅ No risk of accidental resource deletion
- ✅ No risk of configuration drift
- ✅ Read-only access to watched resources (informers)

**Exception:** If you configure TTL on Observations, they will be automatically deleted after TTL expires. This is **explicit and configurable**.

### 2. We Never Hold Secrets or Credentials

**Guarantee:** Zen Watcher **never stores, processes, or transmits** secrets, API keys, or credentials.

**What This Means:**
- ✅ No secrets in ConfigMaps
- ✅ No credentials in environment variables
- ✅ No API keys in code
- ✅ Zero-trust compliant core

**Exception:** If you configure webhook authentication (`WEBHOOK_AUTH_TOKEN`), the token is stored in a Kubernetes Secret (not in zen-watcher's code or ConfigMaps).

### 3. We Never Make External Network Calls

**Guarantee:** Zen Watcher **never initiates outbound network connections** except:
- Kubernetes API server (for informers, CRD operations, leader election)
- Health/readiness probes (inbound only)

**What This Means:**
- ✅ No external SaaS dependencies
- ✅ No cloud API calls
- ✅ No external webhooks
- ✅ Works in air-gapped environments (if Kubernetes API is accessible)

**Exception:** If you enable NetworkPolicy egress for Kubernetes API access, zen-watcher will connect to the API server. This is **explicit and configurable**.

### 4. We Never Modify Watched Resources

**Guarantee:** Zen Watcher **never modifies** the resources it watches (Trivy reports, Kyverno PolicyReports, ConfigMaps, etc.).

**What This Means:**
- ✅ Read-only access to source resources
- ✅ No risk of data corruption
- ✅ No risk of breaking source tools
- ✅ Safe to run alongside Trivy, Falco, Kyverno, etc.

### 5. We Never Crash Your Cluster

**Guarantee:** Zen Watcher **never performs operations** that could crash or destabilize your cluster.

**What This Means:**
- ✅ No etcd overload (TTL prevents bloat)
- ✅ No resource exhaustion (configurable limits)
- ✅ No infinite loops or deadlocks
- ✅ Graceful degradation on errors

---

## API Stability

### Stable APIs (v1.2.0+)

**Observation CRD (v1):**
- ✅ **Stable:** Schema will not change in v1.x releases
- ✅ **Backward Compatible:** New fields may be added, but existing fields won't be removed
- ✅ **Breaking Changes:** Only in v2.0.0+ (major version bump)

**Ingester CRD (v1alpha1):**
- ⚠️ **Alpha:** May change in minor releases (1.2.x → 1.3.x)
- ⚠️ **Breaking Changes:** Possible in minor releases
- ✅ **Migration Path:** Documented in CHANGELOG.md

**Helm Chart Values:**
- ✅ **Stable:** Core values (replicaCount, resources, image) are stable
- ⚠️ **Alpha:** Advanced values (networkPolicy, webhook auth) may change
- ✅ **Breaking Changes:** Documented in CHANGELOG.md

### Deprecated APIs

**When an API is deprecated:**
1. **Announcement:** Deprecated in CHANGELOG.md
2. **Warning Period:** Minimum 2 minor releases (e.g., deprecated in 1.2.0, removed in 1.4.0)
3. **Migration Guide:** Provided in CHANGELOG.md
4. **Removal:** Only in major version (2.0.0)

**Example:**
- Deprecated in v1.2.0: `leaderElection.mode=zenlead`
- Removed in v1.2.1: Immediate removal (security issue)
- Normal process: Deprecated in v1.2.0, removed in v1.4.0

---

## Version Stability

### Alpha (v0.x.x or v1.x.x-alpha)

**Characteristics:**
- ⚠️ **Unstable:** APIs may change without notice
- ⚠️ **Breaking Changes:** Possible in any release
- ⚠️ **Not Production Ready:** Use at your own risk
- ✅ **Experimental Features:** New features may be alpha

**Current Status:** v1.2.1 is **stable** (not alpha)

### Beta (v1.x.x-beta)

**Characteristics:**
- ⚠️ **Mostly Stable:** Core APIs stable, new features may change
- ⚠️ **Breaking Changes:** Possible in minor releases
- ⚠️ **Production Use:** Supported but not recommended for critical workloads
- ✅ **Feature Complete:** All planned features implemented

**Current Status:** Not applicable (v1.2.1 is stable)

### Stable (v1.x.x)

**Characteristics:**
- ✅ **Stable:** APIs follow semantic versioning
- ✅ **Breaking Changes:** Only in major version (2.0.0)
- ✅ **Production Ready:** Recommended for production use
- ✅ **Long-Term Support:** Security patches for 12 months

**Current Status:** v1.2.1 is **stable**

---

## Breaking Change Policy

### What Constitutes a Breaking Change?

**Breaking Changes:**
- Removing a field from Observation CRD
- Changing field types (string → int)
- Removing a Helm chart value
- Changing default behavior (e.g., webhook auth now enabled by default)
- Removing a feature or API

**Not Breaking Changes:**
- Adding new fields to Observation CRD
- Adding new Helm chart values
- Adding new features
- Performance improvements
- Bug fixes that restore intended behavior

### Breaking Change Process

**For Major Releases (2.0.0):**
1. **RFC:** Design document or GitHub Discussion
2. **Community Feedback:** Minimum 2 weeks
3. **Migration Guide:** Detailed upgrade instructions
4. **Deprecation Period:** Deprecate in v1.x.x, remove in v2.0.0

**For Minor Releases (1.2.0 → 1.3.0):**
- ⚠️ **Alpha APIs:** Breaking changes allowed (Ingester CRD)
- ✅ **Stable APIs:** No breaking changes (Observation CRD)

**For Patch Releases (1.2.0 → 1.2.1):**
- ✅ **No Breaking Changes:** Only bug fixes and security patches

---

## Upgrade Safety

### Safe Upgrades

**Patch Releases (1.2.0 → 1.2.1):**
- ✅ **Zero Downtime:** Rolling update safe
- ✅ **No Data Loss:** Observations preserved
- ✅ **Backward Compatible:** No schema changes

**Minor Releases (1.2.0 → 1.3.0):**
- ✅ **Zero Downtime:** Rolling update safe (if no breaking changes)
- ✅ **No Data Loss:** Observations preserved
- ⚠️ **Alpha APIs:** May require migration (Ingester CRD)

**Major Releases (1.2.0 → 2.0.0):**
- ⚠️ **Migration Required:** Follow migration guide
- ⚠️ **Downtime Possible:** Plan for maintenance window
- ⚠️ **Data Migration:** May require manual steps

### Upgrade Recommendations

**Production:**
- ✅ Test in staging first
- ✅ Read CHANGELOG.md for breaking changes
- ✅ Follow migration guide if upgrading major version
- ✅ Backup Observations before upgrade (if needed)

**Development:**
- ✅ Upgrade freely (patch and minor releases)
- ⚠️ Test alpha features before production use

---

## Support Policy

### Supported Versions

**Current Stable:** v1.2.1
- ✅ **Security Patches:** 12 months from release
- ✅ **Bug Fixes:** 6 months from release
- ✅ **Documentation:** Updated for current version

**Previous Stable:** v1.2.0
- ✅ **Security Patches:** 12 months from release
- ⚠️ **Bug Fixes:** Best effort (6 months)
- ⚠️ **Documentation:** May be outdated

**Unsupported:** v1.0.x, v1.1.x
- ❌ **No Support:** Upgrade to v1.2.x

### End of Life (EOL)

**EOL Announcement:**
- **Timeline:** 6 months before EOL
- **Communication:** GitHub Discussions, CHANGELOG.md
- **Migration:** Upgrade guide provided

**EOL Policy:**
- ❌ **No Security Patches:** After EOL date
- ❌ **No Bug Fixes:** After EOL date
- ⚠️ **Documentation:** May be removed

---

## Operational Guarantees

### Resource Usage

**Guaranteed Limits:**
- **CPU:** <100m average (typical load: <10m)
- **Memory:** <256Mi average (typical load: <50Mi)
- **Storage:** Configurable TTL prevents etcd bloat
- **Network:** None (local only, except Kubernetes API)

**Measured Baseline:**
- **CPU:** ~2-3m (idle)
- **Memory:** ~9-10MB (idle)
- **Storage:** ~2MB per 1000 events (with TTL)

### Performance Guarantees

**Event Processing:**
- **Latency:** <1s for webhook events, <5s for informer events
- **Throughput:** 1000 events/day (typical), 10,000 events/day (heavy load)
- **Deduplication:** 99%+ effectiveness (configurable)

**No Guarantees:**
- ❌ Real-time processing (informers have ~5-15s delay)
- ❌ Zero data loss (informer failover gap exists)
- ❌ Unlimited throughput (resource limits apply)

---

## Security Guarantees

### What We Guarantee

**Security Posture:**
- ✅ **No Secrets:** Never holds credentials
- ✅ **No Egress:** Never makes external calls
- ✅ **Read-Only:** Never modifies watched resources
- ✅ **NetworkPolicy:** Supports ingress-only (default)

**Vulnerability Response:**
- ✅ **24-Hour Response:** Security issues acknowledged within 24 hours
- ✅ **7-Day Fix:** Critical vulnerabilities patched within 7 days
- ✅ **Disclosure:** Coordinated disclosure (see [SECURITY.md](../SECURITY.md))

### What We Don't Guarantee

**Not Guaranteed:**
- ❌ Zero vulnerabilities (we patch promptly, but can't guarantee zero)
- ❌ Protection against malicious source tools (if Trivy/Falco is compromised, events may be malicious)
- ❌ Protection against compromised Kubernetes API (if API is compromised, zen-watcher is compromised)

---

## Summary

**What We Guarantee:**
- ✅ Never delete your cluster resources
- ✅ Never hold secrets or credentials
- ✅ Never make external network calls
- ✅ Never modify watched resources
- ✅ Never crash your cluster
- ✅ Stable APIs (Observation CRD v1)
- ✅ Breaking changes only in major versions
- ✅ Security patches for 12 months

**What We Don't Guarantee:**
- ❌ Zero data loss (informer failover gap exists)
- ❌ Real-time processing (informers have delay)
- ❌ Unlimited throughput (resource limits apply)
- ❌ Zero vulnerabilities (we patch promptly)

**For Production Use:**
- ✅ Safe to run in production (v1.2.1+)
- ✅ Follow upgrade recommendations
- ✅ Monitor resource usage
- ✅ Configure TTL to prevent etcd bloat
- ✅ Use NetworkPolicy for security

---

**Questions?** See [docs/ARCHITECTURE.md](ARCHITECTURE.md) for architecture details, or [GitHub Discussions](https://github.com/kube-zen/zen-watcher/discussions) for community support.

