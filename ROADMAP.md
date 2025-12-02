# Zen Watcher Roadmap

## Core Principles

Zen Watcher will always maintain its **pure core**:
- Only watches sources → writes Observation CRDs
- Zero egress traffic
- Zero secrets or credentials
- Zero external dependencies

## Current Status

✅ **Core Features (Complete)**
- Multi-source event aggregation (Trivy, Falco, Kyverno, Checkov, Kube-bench, Audit)
- Observation CRD creation and storage
- Prometheus metrics and Grafana dashboard
- Modular, extensible architecture
- Production-ready security (non-root, read-only filesystem)
- Structured logging with correlation IDs (zap-based, production-ready)
- Deduplication cache with LRU eviction for event deduplication

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
- **Metrics Export** - Support for additional metric backends

### Performance & Scale

**Current Status (v1.0.x):**
- ✅ **Single-replica deployment** - Recommended default (see [SCALING.md](docs/SCALING.md))
- ✅ **Namespace sharding** - Official scale-out pattern for high-volume deployments
- ✅ **Vertical scaling** - Increase resources for higher throughput

**Medium-Term (v1.1.x+):**
- 🔄 **Leader Election** - Optional leader election for singleton responsibilities:
  - Leader handles: Informer-based watchers (Kyverno, Trivy) + Garbage collection
  - All pods handle: Webhook endpoints (Falco, audit) - enables HPA for webhook traffic
  - Keeps CRD semantics intact while allowing horizontal scaling
- 🔄 **Event Batching** - Batch Observation creation for high-volume sources
- ~~**Caching**~~ ✅ **Partially Complete** - Deduplication cache with LRU eviction implemented; general-purpose caching for frequently accessed data still planned

**Design Philosophy:**
- Keep it simple and predictable
- Single-replica default with clear scaling envelope
- Sharding by namespace for scale-out (no leader election needed)
- Leader election only when real-world scale pressure demands it

See [docs/SCALING.md](docs/SCALING.md) for complete scaling strategy and recommendations.

### Developer Experience

- **Operator SDK** - Migrate to Operator SDK framework (optional)
- **Helm Chart** - Enhanced Helm chart with more configuration options
- **Kustomize** - Better Kustomize support for different environments

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

