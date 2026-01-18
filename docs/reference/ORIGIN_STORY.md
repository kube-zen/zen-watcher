# Origin Story: Why Zen Watcher Exists

> **This project is built by operators, for operators.**  
> We built Zen Watcher because we were tired of reinventing the same event aggregation wheel in every Kubernetes cluster.

---

## The Problem That Broke Us

### The Incident

It was 2 AM on a Tuesday. A critical security vulnerability was discovered in a production cluster. The security team needed to know:

1. **Which pods are affected?** (Trivy scan results)
2. **Are there active exploits?** (Falco alerts)
3. **Did any policy violations allow this?** (Kyverno reports)
4. **What's the blast radius?** (Kubernetes Events)

**The reality:** Each tool had its own format, its own API, its own alerting system. We spent 3 hours manually correlating data from 4 different sources, copying JSON between terminals, and trying to piece together what actually happened.

**The cost:** 3 hours of incident response time, manual correlation errors, and a security team that couldn't trust the data.

### The Pattern We Saw Everywhere

This wasn't a one-time problem. Every Kubernetes cluster we managed had the same pattern:

- **Trivy** → JSON files, webhooks, or CRDs (depending on version)
- **Falco** → Webhooks or gRPC (different format)
- **Kyverno** → PolicyReport CRDs (different schema)
- **Audit logs** → JSON files or webhooks
- **Custom tools** → Logs, webhooks, or nothing

**Every tool, every format, every time.** We were building the same aggregation layer over and over.

### The Breaking Point

After the third incident where we couldn't quickly answer "what's happening in this cluster?", we realized:

> **We needed a single source of truth for all events, in a format that Kubernetes operators understand: CRDs.**

---

## The Solution We Built

### Core Insight

**Kubernetes operators already know how to work with CRDs.** They use `kubectl get`, `kubectl watch`, RBAC, and all the standard Kubernetes tooling. Why not make events first-class Kubernetes resources?

### What We Built

Zen Watcher is a **Kubernetes-native observation collector** that:

1. **Aggregates** events from any tool (security, compliance, performance, operations, cost)
2. **Normalizes** them into a unified `Observation` CRD format
3. **Stores** them in etcd (no external database)
4. **Exposes** them via standard Kubernetes APIs

**The result:** One `kubectl get observations` command shows you everything happening in your cluster.

### Why It Works

- ✅ **Operators understand CRDs** - No new APIs to learn
- ✅ **Kubernetes-native** - Works with existing RBAC, NetworkPolicy, monitoring
- ✅ **Zero external dependencies** - No databases, no external services
- ✅ **Extensible** - Add new sources via YAML (Ingester CRD), no code required

---

## The Journey

### Phase 1: The Prototype (Week 1-2)

We built a simple Go binary that:
- Watched Trivy VulnerabilityReports
- Converted them to a simple Observation CRD
- Stored them in etcd

**Result:** We could query all vulnerabilities with `kubectl get observations`. It worked.

### Phase 2: The Realization (Week 3-4)

We added Falco webhooks, then Kyverno PolicyReports, then Audit logs. Each new source was easier than the last because we had the pattern:

1. Receive event (webhook, informer, logs)
2. Normalize to Observation format
3. Create Observation CRD

**Result:** We had a unified view of all security events. The 2 AM incident would have taken 5 minutes, not 3 hours.

### Phase 3: The Open Source Decision (Month 2)

We realized this wasn't just our problem. Every Kubernetes operator faces the same challenge:

> **"How do I get a unified view of what's happening in my cluster?"**

We decided to open source it because:
- The problem is universal
- The solution is useful to others
- We want to build a community around it

---

## What We Learned

### 1. Operators Want Kubernetes-Native Solutions

We tried external databases, external APIs, external services. They all added complexity. **CRDs in etcd** is what operators understand.

### 2. YAML Configuration Beats Code

We started with hardcoded source adapters. Then we built the Ingester CRD. **YAML configuration is 10x easier** than writing code for every new source.

### 3. Zero Egress is a Feature

We initially planned to send events to external systems. Then we realized: **operators want to control where data goes**. Zen Watcher aggregates; you route. Separation of concerns.

### 4. Security Tools Aren't Just Security Tools

Trivy finds vulnerabilities, but it's also a compliance tool. Falco detects threats, but it's also an operations tool. **Events are multi-domain**, and Zen Watcher reflects that.

---

## The Future

We're building Zen Watcher for the long term. Our commitment:

- ✅ **Pure core** - Always stays minimal (aggregate → CRD, nothing more)
- ✅ **Extensible ecosystem** - Community builds sink controllers, integrations
- ✅ **Operator-first** - Built by operators, for operators
- ✅ **Production-ready** - Secure by default, zero external dependencies

**Join us:** If you've ever spent hours correlating events from multiple tools, Zen Watcher is for you.

---

## Who We Are

We're a team of Kubernetes operators who got tired of reinventing the wheel. We built Zen Watcher because we needed it, and we're open sourcing it because we think you might need it too.

**Questions?** [GitHub Discussions](https://github.com/kube-zen/zen-watcher/discussions)  
**Want to contribute?** [CONTRIBUTING.md](../CONTRIBUTING.md)  
**Found a bug?** [GitHub Issues](https://github.com/kube-zen/zen-watcher/issues)

---

*"This project is built by operators, for operators. We bleed when things break, and we built this so you don't have to."*

