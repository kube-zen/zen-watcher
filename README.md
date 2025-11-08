# Zen Watcher

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](https://go.dev/)

> **Kubernetes Security & Compliance Event Aggregator**

Zen Watcher is an open-source Kubernetes operator that aggregates security and compliance events from multiple tools into unified CRDs. Simple, standalone, and useful on its own.

---

## Features

### Multi-Source Event Aggregation
Collects events from popular security and compliance tools:
- 🛡️ **Trivy** - Container vulnerabilities
- 🚨 **Falco** - Runtime threat detection  
- 📋 **Kyverno** - Policy violations
- 🔍 **Kubernetes Audit Logs** - API server audit events
- ✅ **Kube-bench** - CIS benchmark compliance

### CRD-Based Storage
- All events stored as **ZenAgentEvent** Custom Resources
- Kubernetes-native (stored in etcd)
- kubectl access: `kubectl get zenagentevents`
- GitOps compatible
- No external dependencies

### Comprehensive Observability
- 📊 20+ Prometheus metrics on :9090
- 🎨 Pre-built Grafana dashboard
- 📝 Structured logging: `2025-11-08T16:30:00.000Z [INFO] zen-watcher: message`
- 🏥 Health and readiness probes

### Production-Ready
- Non-privileged containers
- Read-only filesystem
- Minimal footprint (~15MB image, <10m CPU, <50MB RAM)
- Pod Security Standards (restricted)

---

## Quick Start

### Prerequisites
- Kubernetes 1.28+
- kubectl configured
- Security tools installed (optional: Trivy, Falco, Kyverno, etc.)

### Installation

```bash
# 1. Apply CRDs
kubectl apply -f deployments/crds/zenagent_event_crd.yaml

# 2. Deploy zen-watcher
kubectl apply -f deployments/zen-watcher.yaml

# 3. Verify
kubectl get pods -n zen-system
kubectl logs -n zen-system deployment/zen-watcher

# 4. Check events
kubectl get zenagentevents -n zen-system
```

---

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `WATCH_NAMESPACE` | Namespace to watch | `zen-system` |
| `TRIVY_NAMESPACE` | Trivy operator namespace | `trivy-system` |
| `FALCO_NAMESPACE` | Falco namespace | `falco` |
| `BEHAVIOR_MODE` | Watching behavior | `all` |
| `LOG_LEVEL` | Log level (DEBUG/INFO/WARN/ERROR/CRIT) | `INFO` |
| `METRICS_PORT` | Prometheus metrics port | `9090` |

### Behavior Modes

- `all` - Watch all available tools
- `conservative` - Only confirmed security issues
- `security-only` - Skip compliance tools
- `custom` - Use tool-specific enable flags

---

## Architecture

```
┌──────────────────────────────────┐
│ Kubernetes                       │
│                                  │
│  Security Tools                  │
│    ├─ Trivy                     │
│    ├─ Falco                     │
│    ├─ Kyverno                   │
│    ├─ Audit Logs                │
│    └─ Kube-bench                │
│         ↓                        │
│  zen-watcher (watches all)       │
│         ↓                        │
│  ZenAgentEvent CRDs (etcd)       │
│         ↓                        │
│  [Your integration]              │
│    ├─ kubectl get zenagentevents│
│    ├─ Custom controller          │
│    ├─ Export to external system  │
│    └─ Analytics/dashboards       │
│                                  │
└──────────────────────────────────┘
```

**Design:**
- **Independent** - No external services required
- **Kubernetes-native** - Uses CRDs for storage
- **Extensible** - Add your own integrations
- **Observable** - Metrics and structured logs

---

## Observability

### Prometheus Metrics (:9090/metrics)

**Core Metrics:**
- `zen_watcher_up` - Watcher is running
- `zen_watcher_events_total` - Total events created
- `zen_watcher_tools_active` - Active security tools detected

**Per-Tool Metrics:**
- `zen_watcher_trivy_events_total`
- `zen_watcher_falco_events_total`
- `zen_watcher_kyverno_events_total`
- `zen_watcher_audit_events_total`
- `zen_watcher_kubebench_events_total`

**Performance:**
- `zen_watcher_crd_write_duration_seconds`
- `zen_watcher_watch_errors_total`

### Structured Logging

**Format:**
```
2025-11-08T16:30:00.000Z [INFO] zen-watcher: Trivy watcher started
2025-11-08T16:30:01.000Z [DEBUG] zen-watcher: Processing vulnerability CVE-2024-001
2025-11-08T16:30:02.000Z [WARN] zen-watcher: Falco not detected (skipping)
2025-11-08T16:30:03.000Z [ERROR] zen-watcher: Failed to create CRD (will retry)
```

**Levels:** DEBUG, INFO, WARN, ERROR, CRIT

**Configuration:**
```bash
LOG_LEVEL=INFO  # DEBUG, INFO, WARN, ERROR, CRIT
```

### Health Endpoints

```bash
# Health check
curl http://localhost:8080/health

# Readiness check  
curl http://localhost:8080/ready

# Metrics
curl http://localhost:9090/metrics
```

---

## Integration Examples

### Watch Events in Your Code

```go
// Watch ZenAgentEvent CRDs and process them
func watchEvents(ctx context.Context) {
    watch, err := k8sClient.Resource(zenAgentEventGVR).
        Namespace("zen-system").
        Watch(ctx, metav1.ListOptions{})
    
    for event := range watch.ResultChan() {
        zenEvent := event.Object.(*ZenAgentEvent)
        
        // Process event
        fmt.Printf("New event: %s (severity: %s)\n", 
            zenEvent.Spec.EventType, 
            zenEvent.Spec.Severity)
    }
}
```

### Query with kubectl

```bash
# All events
kubectl get zenagentevents -n zen-system

# High severity only
kubectl get zenagentevents -n zen-system -l severity=high

# From specific source
kubectl get zenagentevents -n zen-system -l source=trivy

# Export to JSON
kubectl get zenagentevents -n zen-system -o json > events.json
```

---

## Resource Usage

### Typical Load (1000 events/day):
- **CPU:** <10m average
- **Memory:** <50MB
- **Storage:** ~2MB in etcd
- **Network:** None (local only)

### Heavy Load (10,000 events/day):
- **CPU:** <20m average
- **Memory:** <80MB
- **Storage:** ~20MB in etcd
- **Network:** None (local only)

---

## Building

```bash
# Standard build
go build -o zen-watcher ./cmd/zen-watcher

# Optimized build
go build -ldflags="-w -s" -trimpath -o zen-watcher ./cmd/zen-watcher

# Docker image
docker build -f build/Dockerfile -t zen-watcher:latest .
```

---

## Troubleshooting

### Enable Debug Logging
```bash
kubectl set env deployment/zen-watcher LOG_LEVEL=DEBUG -n zen-system
kubectl logs -n zen-system deployment/zen-watcher -f
```

### Check CRDs
```bash
kubectl get zenagentevents -n zen-system
kubectl describe zenagentevents <name> -n zen-system
```

### View Metrics
```bash
kubectl port-forward -n zen-system deployment/zen-watcher 9090:9090
curl http://localhost:9090/metrics
```

---

## Contributing

Contributions welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

---

## License

Apache License 2.0 - See [LICENSE](LICENSE) for details.

---

**Repository:** github.com/kube-zen/zen-watcher  
**Go Version:** 1.24.0  
**Status:** ✅ Production-ready, standalone, independently useful
