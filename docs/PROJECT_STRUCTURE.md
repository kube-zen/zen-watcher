# Zen Watcher - Project Structure

Clean, simple project organization for an event aggregator.

---

## 📁 Directory Structure

```
zen-watcher/
├── cmd/                          # Main applications
│   └── zen-watcher/              # Zen Watcher application
│       ├── main.go               # Application entrypoint
│       └── main_test.go          # Main tests
│
├── pkg/                          # Library code
│   ├── actions/                  # Event action handlers
│   ├── adapters/                 # Tool adapters
│   ├── config/                   # Configuration
│   ├── controller/               # Controllers
│   ├── detection/                # Tool detection
│   ├── installation/             # Tool installation
│   ├── manager/                  # Watcher management
│   ├── metrics/                  # Prometheus metrics
│   ├── models/                   # Data models
│   ├── actions/                  # Action templates
│   ├── types/                    # CRD types & client
│   ├── watcher/                  # Source watchers
│   └── writer/                   # CRD writer
│
├── build/                        # Build files
│   ├── Dockerfile                # Multi-stage Dockerfile
│   └── .dockerignore             # Docker ignore rules
│
├── config/                       # Configuration files
│   ├── dashboards/               # Grafana dashboards
│   ├── monitoring/               # Monitoring configs
│   └── samples/                  # Sample configurations
│
├── deployments/                  # Deployment manifests
│   ├── crds/                     # CRD definitions
│   ├── k8s-deployment.yaml       # Kubernetes deployment
│   ├── victoriametrics.yaml      # VictoriaMetrics
│   └── grafana-deployment.yaml   # Grafana
│
├── docs/                         # Documentation
│   ├── SECURITY.md               # Security policy
│   ├── SBOM.md                   # SBOM guide
│   ├── COSIGN.md                 # Image signing
│   └── OPERATIONAL_EXCELLENCE.md # Operations guide
│
├── examples/                     # Integration examples
│   ├── query-examples.sh         # Query examples
│   ├── loki-promtail-config.yaml # Loki config
│   └── README.md                 # Examples guide
│
├── scripts/lint/                 # Lint and validation scripts
│   ├── check-branding.sh         # Check for forbidden compatibility labels
│   └── check-no-github-actions.sh # Guardrail for GitHub Actions
│
├── .github.disabled/             # Disabled GitHub workflows (per guardrails)
│   └── workflows/                # Disabled GitHub Actions workflows
│       └── security-scan.yml     # Security scanning (disabled)
│
├── go.mod                        # Go module definition
├── go.sum                        # Go dependencies
├── .gitignore                    # Git ignore rules
├── README.md                     # Main documentation
├── LICENSE                       # Apache 2.0 license
├── CONTRIBUTING.md               # Contribution guide
├── CHANGELOG.md                  # Version history
├── QUICK_START.md                # Quick start guide
├── docs/PROJECT_STRUCTURE.md      # This file
└── docs/DOCUMENTATION_INDEX.md    # Doc index

```

---

## 🎯 Design Philosophy

**Simple and Focused**: Zen Watcher is an event aggregator, not a complex operator framework.

### What We DON'T Need
- ❌ `api/` directory - Too heavy for a simple aggregator
- ❌ Multiple API versions - Not an operator with evolving APIs
- ❌ Controller-runtime scaffolding - Not using Kubebuilder/Operator SDK
- ❌ Webhook servers - Not validating/mutating resources

### What We DO Have
- ✅ `pkg/types/` - CRD definitions where they belong
- ✅ `deployments/crds/` - CRD manifests with other K8s resources
- ✅ Clean, simple structure
- ✅ Easy to understand and contribute to

---

## 📂 Directory Purposes

### `/cmd`
**Purpose**: Main application entry points

- Keep minimal - just wiring
- Each subdirectory = one binary
- Logic lives in `pkg/`

### `/pkg`
**Purpose**: All reusable code

- Well-organized packages
- Business logic
- Can be imported by other projects
- Includes CRD types in `pkg/types/`

### `/pkg/types`
**Purpose**: CRD type definitions and client

- Observation CRD types
- CRD client implementation
- Type constants
- **Note**: For simple projects like this, types belong in pkg/ not a separate api/ directory

### `/build`
**Purpose**: Build artifacts

- Dockerfile
- .dockerignore
- Build configs
- CI/CD files

### Helm Charts
**Note**: Helm charts are maintained in the separate [helm-charts](https://github.com/kube-zen/helm-charts) repository and published to ArtifactHub.

- Install via Helm repository: `helm install zen-watcher kube-zen/zen-watcher`
- Charts are not stored in this repository

### `/config`
**Purpose**: Configuration files

- Dashboards (Grafana)
- Monitoring (Prometheus alerts)
- Sample configs
- **Not** application code

### `/deployments`
**Purpose**: Kubernetes manifests

- Plain YAML manifests
- CRD definitions
- Direct `kubectl apply` usage
- **Includes** `/deployments/crds/` for CRD YAMLs

### `/docs`
**Purpose**: User documentation

- Guides and tutorials
- Best practices
- Security policies
- Operations manuals

### `/examples`
**Purpose**: Working examples

- Integration examples
- Sample queries
- Tutorial configs

### `/hack`
**Purpose**: Development utilities

- Build scripts
- Test helpers
- Development tools

---

## 🔍 Finding Things

| What | Where |
|------|-------|
| Main code | `cmd/zen-watcher/main.go` |
| CRD types | `pkg/types/types.go` |
| CRD client | `pkg/types/zen_client.go` |
| CRD YAML | `deployments/crds/zen_event_crd.yaml` |
| Business logic | `pkg/*/` subdirectories |
| Dockerfile | `build/Dockerfile` |
| Helm chart | `kube-zen/zen-watcher` (from ArtifactHub) |
| K8s manifests | `deployments/` |
| Monitoring | `config/monitoring/` |
| Dashboard | `config/dashboards/` |
| Examples | `examples/` |
| Documentation | `docs/` + root `.md` files |

---

## 🏗️ Build Commands

### Go Build
```bash
go build -o bin/zen-watcher ./cmd/zen-watcher
```

### Docker Build
```bash
docker build -f build/Dockerfile -t zen-watcher:1.0.0 .
```

### Helm Install
```bash
helm repo add kube-zen https://kube-zen.github.io/helm-charts
helm repo update
helm install zen-watcher kube-zen/zen-watcher --namespace zen-system --create-namespace
```

### kubectl Deploy
```bash
kubectl apply -f deployments/crds/
kubectl apply -f deployments/k8s-deployment.yaml
```

---

## 📦 Import Paths

```go
import (
    "github.com/kube-zen/zen-watcher/pkg/types"    // CRD types
    "github.com/kube-zen/zen-watcher/pkg/actions"  // Event handlers
    "github.com/kube-zen/zen-watcher/pkg/config"   // Configuration
    "github.com/kube-zen/zen-watcher/pkg/manager"  // Watcher manager
    "github.com/kube-zen/zen-watcher/pkg/metrics"  // Prometheus metrics
    "github.com/kube-zen/zen-watcher/pkg/writer"   // CRD writer
)
```

**Note**: No complex `api/v1` aliasing needed - simple `pkg/types` import!

---

## ✨ Why This Structure?

### For Zen Watcher Specifically

1. **Not an Operator Framework Project**
   - We're not using Kubebuilder or Operator SDK
   - Don't need api/ versioning structure
   - Simple CRD types belong in pkg/types

2. **Event Aggregator, Not Controller**
   - We watch and write, we don't reconcile
   - Simpler than full operator
   - Don't need controller-runtime complexity

3. **Community Friendly**
   - Easy to understand
   - Less intimidating for contributors
   - Clear where everything lives

4. **Apache 2 Best Practices**
   - Clean root directory
   - Logical organization
   - Standard for Go projects

---

**This is the right structure for Zen Watcher!** ✅
