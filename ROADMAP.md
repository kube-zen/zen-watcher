# Zen Watcher Roadmap

## Core Principles

Zen Watcher will always maintain its **pure core**:
- Only watches sources → writes Observation CRDs
- Zero egress traffic
- Zero secrets or credentials
- Zero external dependencies

## Current Status

✅ **Core Features (Complete)**
- Multi-source event aggregation (9 sources: Trivy, Falco, Kyverno, Checkov, Kube-bench, Audit, cert-manager, sealed-secrets, Kubernetes Events)
- Observation CRD creation and storage
- Prometheus metrics and Grafana dashboards
- Modular, extensible architecture
- Production-ready security (non-root, read-only filesystem)
- Structured logging with correlation IDs (zap-based, production-ready)
- Deduplication cache with LRU eviction for event deduplication
- YAML-only source creation via Ingester CRD (no code required)

✅ **Enterprise Alerting System (Phase 2 - Complete)**
- 40+ security alerts covering all sources (Falco, Trivy, Kube-Bench, Checkov, Audit, Kyverno)
- 25+ performance alerts with predictive capacity monitoring
- AlertManager configuration with multi-channel notifications (Email, Slack, PagerDuty)
- Intelligent routing by severity and component
- Automated escalation policies with response time SLAs
- Comprehensive incident response runbooks and documentation

✅ **Production-Grade Dashboards (Phase 3 - Complete)**
- **Executive Dashboard**: Strategic KPIs, ROI metrics, predictive analytics, security posture score
- **Operations Dashboard**: Real-time health monitoring, SLA tracking, capacity planning, incident correlation
- **Security Dashboard**: Threat intelligence, behavioral analytics, attack chain visualization, investigation workflows
- **Main Dashboard**: Unified navigation hub, cross-dashboard correlation, live alert feed
- **Namespace Health**: Drill-down capabilities, tenant correlation, per-namespace compliance tracking
- **Data Explorer**: Advanced query builder, saved searches, query performance metrics
- All 6 dashboards enhanced with 28+ new production-grade panels

## Future Enhancements

### Community-Driven Sink Controllers

Add support for forwarding Observation events to external systems via optional, isolated controllers:

- 📢 **Slack** - Forward high-severity events to Slack channels
- 🚨 **PagerDuty** - Create incidents for critical security events
- 🛠️ **ServiceNow** - Create tickets for compliance violations
- 📊 **SIEM Integration** - Forward to Datadog, Splunk, or other SIEMs
- 📧 **Email** - Send email notifications for filtered events
- 🔔 **Custom Webhooks** - Generic webhook sink for any HTTP endpoint

**Note**: All sink controllers will be:
- Separate, optional components (not part of zen-watcher core)
- Deployable independently
- Using SealedSecrets or external secret managers for credentials
- Built by the community or enterprise users

**Zen Watcher core remains pure** — it only writes Observation CRDs. The ecosystem extends it.

### Additional Event Sources

- **Polaris** - Kubernetes configuration validation
- **OPA Gatekeeper** - Policy violations
- **Kubescape** - Security scanning
- **Nexus IQ** - Dependency scanning
- **Snyk** - Container and dependency scanning
- **Kubecost** - Cost optimization and anomalies

**Note:** New sources can be added easily using the formal [SourceAdapter interface](docs/SOURCE_ADAPTERS.md). The interface provides a standardized way to integrate any tool that emits events, making community contributions straightforward.

### Observability Enhancements

- **OpenTelemetry** - Distributed tracing support
- ~~**Structured Logging**~~ ✅ **Complete** - Enhanced log format with correlation IDs (implemented with zap)
- ~~**Enterprise Alerting**~~ ✅ **Complete** - Comprehensive alerting system with 65+ alerts, AlertManager integration, and incident response workflows (Phase 2)
- ~~**Production Dashboards**~~ ✅ **Complete** - All 6 dashboards enhanced with strategic KPIs, operational intelligence, and threat analysis (Phase 3)
- **Metrics Export** - Support for additional metric backends

### Performance & Scale

**Current Status (v1.0.x):**
- ✅ **Single-replica deployment** - Recommended default (see [SCALING.md](docs/SCALING.md))
- ✅ **Namespace sharding** - Official scale-out pattern for high-volume deployments
- ✅ **Vertical scaling** - Increase resources for higher throughput

**Current (v1.0.0-alpha):**
- ✅ **Leader Election** - Mandatory leader election for singleton responsibilities:
  - Leader handles: Informer-based watchers (Kyverno, Trivy) + Garbage collection
  - All pods handle: Webhook endpoints (Falco, audit) - enables HPA for webhook traffic
  - Keeps CRD semantics intact while allowing horizontal scaling
  - See [docs/LEADER_ELECTION.md](docs/LEADER_ELECTION.md) for details

**Medium-Term (v1.1.x+):**
- 🔄 **HPA Support** - Standard Kubernetes autoscaling for webhook traffic (leader election already implemented)
- 🔄 **KEDA Support** - Advanced autoscaling with custom metrics (optional, leader election already implemented)
- 🔄 **Event Batching** - Batch Observation creation for high-volume sources
- ~~**Caching**~~ ✅ **Partially Complete** - Deduplication cache with LRU eviction implemented; general-purpose caching for frequently accessed data still planned

**Design Philosophy:**
- Keep it simple and predictable
- Single-replica default with clear scaling envelope
- Sharding by namespace for scale-out (alternative to multi-replica)
- Leader election mandatory (enables HPA for webhook traffic)

See [docs/SCALING.md](docs/SCALING.md) for complete scaling strategy and recommendations.

### Developer Experience

- **Operator SDK** - Migrate to Operator SDK framework (optional)
- **Helm Chart** - Enhanced Helm chart with more configuration options
- **Kustomize** - Better Kustomize support for different environments

## What's Next: 90-Day Plan

**Next 90 Days (Q1 2025):**

### Immediate (Next 30 Days)
- ✅ **v1.2.1 Release** - OSS launch with secure defaults
- 🔄 **Community On-Ramp** - Improve contributor experience
- 🔄 **Documentation Polish** - Complete missing docs, add examples

### Short-Term (30-60 Days)
- 🔄 **v1.3.0 Release** - Leader takeover catch-up scan (reduces informer failover gap)
- 🔄 **Additional Sources** - Community-requested integrations (Polaris, OPA Gatekeeper, Kubescape)
- 🔄 **Performance Improvements** - Event batching for high-volume deployments

### Medium-Term (60-90 Days)
- 🔄 **v1.4.0 Release** - Optional active-active informer processing (eliminates failover gap)
- 🔄 **Observability Enhancements** - OpenTelemetry tracing support
- 🔄 **Ecosystem Growth** - Community sink controllers (Slack, PagerDuty, SIEM integrations)

**This roadmap is living and evolves based on community feedback.** See [GitHub Discussions](https://github.com/kube-zen/zen-watcher/discussions) to influence priorities.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines on:
- Adding new watchers
- Building sink controllers
- Contributing to the codebase

## Philosophy

Zen Watcher follows the **"core is minimal; ecosystem extends it"** pattern, similar to:
- **Prometheus** (core metrics collection, Alertmanager extends it)
- **Flux** (core GitOps, ecosystem extends it)
- **Crossplane** (core resource management, providers extend it)

This ensures:
- Core remains lean, trusted, and maintainable
- Community can extend without complicating core
- Enterprise users can build custom solutions
- Clear separation of concerns

