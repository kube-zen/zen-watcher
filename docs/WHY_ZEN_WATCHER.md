# Why Zen Watcher?

**TL;DR:** Zen Watcher is a **Kubernetes-native observation collector** that turns any signal (security, compliance, performance, operations, cost) into unified `Observation` CRDs. It's **not a replacement** for Falco, Kyverno, Prometheus, or custom controllers—it's a **unified aggregation layer** that works alongside them.

---

## What We Do Instead of Alerts

**Traditional Approach:**
- Each tool (Trivy, Falco, Kyverno) sends alerts to different destinations
- Alerts are siloed, hard to correlate, and create alert fatigue
- No unified view of what's happening in your cluster

**Zen Watcher Approach:**
- Collects events from **all tools** into unified `Observation` CRDs
- Single source of truth for all events (security, compliance, performance, operations, cost)
- You decide where to route: Use kubewatch/Robusta to send to Slack, PagerDuty, SIEMs, or build custom controllers
- **We aggregate, you route**—separation of concerns

**Example:**
```yaml
# Trivy finds a vulnerability → Observation CRD
# Falco detects suspicious activity → Observation CRD
# Kyverno policy violation → Observation CRD
# All in the same format, same namespace, queryable with kubectl
```

---

## Why CRDs Over Logs

**Logs are ephemeral, CRDs are persistent:**

| Aspect | Logs | CRDs (Observations) |
|--------|------|---------------------|
| **Persistence** | Rotate, disappear | Stored in etcd, queryable |
| **Queryability** | `kubectl logs` (limited) | `kubectl get observations` (rich queries) |
| **Filtering** | Text search only | Structured fields (severity, category, source) |
| **TTL** | Log rotation (unpredictable) | Configurable TTL per observation |
| **Integration** | Parse logs (fragile) | Watch CRDs (standard Kubernetes pattern) |
| **History** | Lost after rotation | Preserved until TTL expires |

**Real-world benefit:**
```bash
# Find all HIGH severity security events from the last hour
kubectl get observations -n zen-system \
  --field-selector spec.severity=HIGH,spec.category=security \
  -o json | jq '.items[] | select(.metadata.creationTimestamp > "2025-01-05T05:00:00Z")'
```

Try doing that with logs.

---

## Why We Don't Remediate by Default

**Zen Watcher is a collector, not a remediator.**

**Design Philosophy:**
- **Collect:** Aggregate events from all sources
- **Normalize:** Convert to unified format
- **Store:** Write Observation CRDs
- **Route:** Let downstream controllers handle remediation

**Why?**
1. **Separation of Concerns:** Collection ≠ Remediation
2. **Flexibility:** Different teams want different remediation strategies
3. **Safety:** Remediation is risky—should be explicit, not automatic
4. **Composability:** Build specialized remediation controllers (e.g., `zen-remediator`)

**What We Do:**
- Collect events from Trivy, Falco, Kyverno, etc.
- Normalize to `Observation` CRDs
- Provide structured data for downstream controllers

**What We Don't Do:**
- Automatically delete pods
- Automatically patch resources
- Automatically send to external systems
- Hold secrets or API keys

**You Build Remediation:**
```go
// Example: Custom remediation controller
informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
    AddFunc: func(obj interface{}) {
        obs := obj.(*Observation)
        if obs.Spec.Severity == "CRITICAL" && obs.Spec.Category == "security" {
            // Your remediation logic here
            deletePod(obs.Spec.Resource)
        }
    },
})
```

---

## What We Intentionally Don't Do

### 1. We Don't Send Events to External Systems

**Why:** Zen Watcher stays pure—no egress, no secrets, no external dependencies.

**What This Means:**
- No Slack webhooks
- No PagerDuty integration
- No SIEM forwarding
- No cloud API calls

**What You Do Instead:**
- Use [kubewatch](https://github.com/robusta-dev/kubewatch) or [Robusta](https://home.robusta.dev/) to watch Observations and route them
- Build custom controllers that watch Observations
- Use standard Kubernetes patterns (informers, controllers)

### 2. We Don't Hold Secrets

**Why:** Security boundary—compromising zen-watcher shouldn't expose credentials.

**What This Means:**
- No API keys in ConfigMaps
- No tokens in environment variables
- No credentials in the codebase

**What You Do Instead:**
- Use SealedSecrets or external secret managers
- Pass credentials to downstream controllers (kubewatch, Robusta)
- Use IRSA, Workload Identity, or similar for cloud credentials

### 3. We Don't Remediate

**Why:** Remediation is dangerous and should be explicit.

**What This Means:**
- No automatic pod deletion
- No automatic resource patching
- No automatic policy enforcement

**What You Do Instead:**
- Build specialized remediation controllers
- Use policy engines (Kyverno, OPA) for enforcement
- Implement approval workflows for critical actions

### 4. We Don't Store Events Forever

**Why:** etcd bloat prevention—Observations have TTL.

**What This Means:**
- Observations expire after TTL (configurable)
- No long-term event storage
- No event history beyond TTL

**What You Do Instead:**
- Use external systems (SIEMs, databases) for long-term storage
- Build controllers that forward Observations to external systems
- Configure TTL based on your retention needs

---

## Comparison with Other Tools

### vs. Falco

| Aspect | Falco | Zen Watcher |
|--------|-------|-------------|
| **Purpose** | Runtime security detection | Event aggregation |
| **Output** | Falco events (JSON) | Observation CRDs |
| **Scope** | Security events only | All event types (security, compliance, performance, operations, cost) |
| **Integration** | Falco → External systems | Falco → Zen Watcher → Observation CRDs → Your controllers |
| **Use Case** | Real-time threat detection | Unified event aggregation |

**Can I use both?** Yes! Falco detects threats, Zen Watcher aggregates them into CRDs.

### vs. Kyverno

| Aspect | Kyverno | Zen Watcher |
|--------|---------|-------------|
| **Purpose** | Policy enforcement | Event aggregation |
| **Output** | Policy violations (PolicyReports) | Observation CRDs |
| **Scope** | Policy violations only | All event types |
| **Integration** | Kyverno → PolicyReports | Kyverno → Zen Watcher → Observation CRDs |
| **Use Case** | Policy enforcement | Unified event aggregation |

**Can I use both?** Yes! Kyverno enforces policies, Zen Watcher aggregates violations into CRDs.

### vs. Robusta

| Aspect | Robusta | Zen Watcher |
|--------|---------|-------------|
| **Purpose** | Alert routing and remediation | Event aggregation |
| **Output** | Alerts to Slack, PagerDuty, etc. | Observation CRDs |
| **Scope** | Alert routing | Event collection and normalization |
| **Integration** | Robusta → External systems | Zen Watcher → Observation CRDs → Robusta → External systems |
| **Use Case** | Alert management | Unified event aggregation |

**Can I use both?** Yes! Zen Watcher aggregates events, Robusta routes them to external systems.

### vs. Prometheus Alerts

| Aspect | Prometheus Alerts | Zen Watcher |
|--------|-------------------|-------------|
| **Purpose** | Metrics-based alerting | Event aggregation |
| **Output** | Prometheus alerts | Observation CRDs |
| **Scope** | Metrics-based alerts | All event types (including Prometheus alerts) |
| **Integration** | Prometheus → Alertmanager | Prometheus → Zen Watcher → Observation CRDs |
| **Use Case** | Metrics alerting | Unified event aggregation |

**Can I use both?** Yes! Prometheus alerts on metrics, Zen Watcher aggregates alerts into CRDs.

### vs. Custom Controllers

| Aspect | Custom Controllers | Zen Watcher |
|--------|-------------------|-------------|
| **Purpose** | Custom business logic | Event aggregation |
| **Output** | Custom resources | Observation CRDs |
| **Scope** | Specific use case | All event types |
| **Integration** | Custom logic | Standard Observation CRDs |
| **Use Case** | Custom automation | Unified event aggregation |

**Can I use both?** Yes! Custom controllers can watch Observations and implement business logic.

---

## When to Use Zen Watcher

**✅ Use Zen Watcher when:**
- You have multiple security/compliance/operations tools (Trivy, Falco, Kyverno, etc.)
- You want a unified view of all events
- You want to build custom controllers on top of events
- You want Kubernetes-native event storage (CRDs)
- You want to avoid vendor lock-in (no external SaaS dependencies)

**❌ Don't use Zen Watcher when:**
- You only have one tool and don't need aggregation
- You need immediate alert routing (use kubewatch/Robusta directly)
- You need long-term event storage (use external systems)
- You need automatic remediation (build custom controllers)

---

## Architecture Comparison

### Traditional Approach (Siloed)
```
Trivy ──→ Slack
Falco ──→ PagerDuty
Kyverno ──→ SIEM
Prometheus ──→ Email
```
**Problems:** Siloed, hard to correlate, alert fatigue

### Zen Watcher Approach (Unified)
```
Trivy ──┐
Falco ──┼──→ Zen Watcher ──→ Observation CRDs ──→ kubewatch/Robusta ──→ Slack, PagerDuty, SIEM
Kyverno ──┘
Prometheus ──┘
```
**Benefits:** Unified format, single source of truth, flexible routing

---

## Summary

**Zen Watcher is not:**
- ❌ A replacement for Falco, Kyverno, or Prometheus
- ❌ An alert routing system
- ❌ A remediation engine
- ❌ A long-term storage solution

**Zen Watcher is:**
- ✅ A unified event aggregation layer
- ✅ A Kubernetes-native observation collector
- ✅ A foundation for building custom controllers
- ✅ A way to avoid vendor lock-in

**Think of it as:** The "event bus" for Kubernetes—collects events from all sources, normalizes them, and stores them as CRDs. What you do with those CRDs is up to you.

---

**Questions?** See [docs/ARCHITECTURE.md](ARCHITECTURE.md) for detailed architecture, or [GitHub Discussions](https://github.com/kube-zen/zen-watcher/discussions) for community support.

