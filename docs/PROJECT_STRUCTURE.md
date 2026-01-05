# Zen Watcher - Project Structure

Clean, simple project organization for an event aggregator.

---

## 📁 Directory Structure

```
zen-watcher/
├── cmd/                          # Main applications
│   ├── zen-watcher/              # Zen Watcher application
│   │   └── main.go               # Application entrypoint
│   ├── ingester-lint/            # Ingester validation tool
│   │   └── main.go
│   ├── obsctl/                   # Observation CLI tool
│   │   └── main.go
│   └── schema-doc-gen/           # Schema documentation generator
│       └── main.go
│
├── pkg/                          # Library code
│   ├── adapter/                  # Source adapters
│   │   └── generic/              # Generic adapter implementations
│   ├── balancer/                 # Load balancing
│   ├── cli/                      # CLI utilities
│   ├── config/                   # Configuration loading and management
│   ├── dispatcher/               # Event dispatching and batching
│   ├── errors/                   # Error handling
│   ├── filter/                   # Event filtering logic
│   ├── gc/                       # Garbage collection
│   ├── hooks/                    # Plugin hooks system
│   ├── metrics/                  # Prometheus metrics
│   ├── models/                   # Data models
│   ├── monitoring/               # Monitoring and thresholds
│   ├── orchestrator/             # Adapter orchestration
│   ├── processor/                # Event processing pipeline
│   ├── scaling/                  # HPA coordination
│   ├── sdk/                      # Zen SDK integration
│   ├── server/                   # HTTP server and middleware
│   └── watcher/                  # Source watchers and observation creation
│
├── internal/                     # Internal packages (not for external use)
│   ├── informers/                # Kubernetes informers
│   └── kubernetes/               # Kubernetes client utilities
│
├── build/                        # Build files
│   ├── Dockerfile                # Multi-stage Dockerfile
│   └── Dockerfile.optimized      # Optimized Dockerfile
│
├── config/                       # Configuration files
│   ├── alertmanager/             # Alertmanager configurations
│   ├── dashboards/               # Grafana dashboards
│   ├── demo-manifests/           # Demo deployment manifests
│   ├── monitoring/               # Monitoring configs
│   └── prometheus/               # Prometheus rules
│       └── rules/                # Alert rules
│
├── deployments/                  # Deployment manifests
│   ├── crds/                     # CRD definitions
│   │   ├── *.yaml                # CRD YAML manifests
│   │   └── *.go                  # CRD type definitions
│   └── configmaps/               # ConfigMap examples
│
├── docs/                         # Documentation
│   ├── alerting/                 # Alerting documentation
│   ├── playbooks/                # Operational playbooks
│   └── *.md                      # Various documentation files
│
├── examples/                     # Integration examples
│   ├── adapters/                 # Adapter examples
│   ├── aggregator/               # Aggregation examples
│   ├── hooks/                    # Hook examples
│   ├── ingesters/                # Ingester examples
│   ├── observations/             # Observation examples
│   ├── use-cases/                # Use case examples
│   └── *.yaml                    # Various example configurations
│
├── fixtures/                     # Test fixtures
│   └── report/                   # Test report data
│
├── scripts/                      # Utility scripts
│   ├── benchmark/                # Benchmark scripts
│   ├── ci/                       # CI scripts
│   ├── cleanup/                  # Cleanup scripts
│   ├── cluster/                  # Cluster management
│   ├── data/                     # Data generation scripts
│   ├── hack/                     # Development utilities
│   ├── lint/                     # Linting scripts
│   ├── observability/            # Observability setup
│   ├── test/                     # Test scripts
│   ├── utils/                    # Utility scripts
│   ├── demo.sh                   # Full demo script
│   ├── quick-demo.sh             # Quick demo script
│   ├── install.sh                # Installation script
│   └── helmfile.yaml.gotmpl      # Helmfile template
│
├── test/                         # Test code
│   ├── e2e/                      # End-to-end tests
│   ├── helpers/                  # Test helpers
│   ├── integration/              # Integration tests
│   ├── pipeline/                 # Pipeline tests
│   └── validation/               # Validation tests
│
├── go.mod                        # Go module definition
├── go.sum                        # Go dependencies
├── Makefile                      # Build targets
├── .gitignore                    # Git ignore rules
├── README.md                     # Main documentation
├── LICENSE                       # Apache 2.0 license
├── CONTRIBUTING.md               # Contribution guide
├── CHANGELOG.md                  # Version history
├── QUICK_START.md                # Quick start guide
├── VULNERABILITY_DISCLOSURE.md   # Vulnerability disclosure policy (root)
├── docs/
│   ├── SECURITY.md                # Security features and model (central authoritative doc)
└── docs/PROJECT_STRUCTURE.md     # This file

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
- ✅ `deployments/crds/` - CRD definitions with other K8s resources
- ✅ Clean, simple structure
- ✅ Easy to understand and contribute to

---

## 📂 Directory Purposes

### `/cmd`
**Purpose**: Main application entry points

- `zen-watcher/` - Main controller application
- `ingester-lint/` - Ingester validation tool
- `obsctl/` - Observation CLI tool
- `schema-doc-gen/` - Schema documentation generator
- Keep minimal - just wiring
- Each subdirectory = one binary
- Logic lives in `pkg/`

### `/pkg`
**Purpose**: All reusable code

- Well-organized packages
- Business logic
- Can be imported by other projects
- **Key packages:**
  - `adapter/` - Source adapters (generic, webhook, informer, logs)
  - `config/` - Configuration loading and management
  - `processor/` - Event processing pipeline
  - `watcher/` - Source watchers and observation creation
  - `filter/` - Event filtering logic
  - `server/` - HTTP server and middleware (auth, rate limiting)
  - `metrics/` - Prometheus metrics definitions
  - `orchestrator/` - Adapter orchestration and management
  - `sdk/` - Zen SDK integration
  - `dispatcher/` - Event dispatching and batching
  - `hooks/` - Plugin hooks system
  - `gc/` - Garbage collection for observations
  - `monitoring/` - Monitoring and threshold checking
  - `scaling/` - HPA coordination

### `/internal`
**Purpose**: Internal packages not intended for external use

- `informers/` - Kubernetes informer implementations
- `kubernetes/` - Kubernetes client utilities
- Not part of the public API

### `/build`
**Purpose**: Build artifacts

- Dockerfile
- .dockerignore
- Build configs

### Helm Charts
**Note**: Helm charts are maintained in the separate [helm-charts](https://github.com/kube-zen/helm-charts) repository and published to ArtifactHub.

- Install via Helm repository: `helm install zen-watcher kube-zen/zen-watcher`
- Charts are not stored in this repository

### `/config`
**Purpose**: Configuration files

- `dashboards/` - Grafana dashboards (JSON)
- `prometheus/rules/` - Prometheus alert rules
- `alertmanager/` - Alertmanager configurations
- `monitoring/` - Monitoring configurations
- `demo-manifests/` - Demo deployment manifests
- **Not** application code

### `/deployments`
**Purpose**: Kubernetes manifests

- `crds/` - CRD definitions (YAML and Go types)
- `configmaps/` - ConfigMap examples
- Plain YAML manifests
- Direct `kubectl apply` usage

### `/docs`
**Purpose**: User documentation

- Guides and tutorials
- Best practices
- Security policies
- Operations manuals
- API documentation
- Architecture and design docs

### `/examples`
**Purpose**: Working examples

- `ingesters/` - Ingester CRD examples
- `observations/` - Observation CRD examples
- `adapters/` - Adapter implementation examples
- `hooks/` - Hook examples
- `use-cases/` - Practical use case examples
- Integration examples
- Sample queries
- Tutorial configs

### `/scripts`
**Purpose**: Utility scripts

- `demo.sh` - Full-featured demo script
- `quick-demo.sh` - Quick demo script
- `install.sh` - Installation script
- `ci/` - CI/CD scripts
- `benchmark/` - Benchmark scripts
- `lint/` - Linting and validation scripts
- `hack/` - Development utilities
- `observability/` - Observability setup scripts
- `cluster/` - Cluster management scripts

### `/test`
**Purpose**: Test code

- `e2e/` - End-to-end tests
- `integration/` - Integration tests
- `pipeline/` - Pipeline processing tests
- `validation/` - Validation tests
- `helpers/` - Test helper utilities

---

## 🔍 Finding Things

| What | Where |
|------|-------|
| Main code | `cmd/zen-watcher/main.go` |
| CRD YAML | `deployments/crds/*.yaml` |
| CRD types | `deployments/crds/*.go` |
| Processing pipeline | `pkg/processor/pipeline.go` |
| Observation creation | `pkg/watcher/observation_creator.go` |
| Source adapters | `pkg/adapter/generic/` |
| Filtering logic | `pkg/filter/` |
| HTTP server | `pkg/server/http.go` |
| Authentication | `pkg/server/auth.go` |
| Rate limiting | `pkg/server/ratelimit_wrapper.go` |
| Metrics | `pkg/metrics/definitions.go` |
| Configuration | `pkg/config/` |
| Orchestration | `pkg/orchestrator/` |
| Dockerfile | `build/Dockerfile` |
| Helm chart | `kube-zen/zen-watcher` (from ArtifactHub) |
| K8s manifests | `deployments/` |
| Monitoring | `config/prometheus/rules/` |
| Dashboards | `config/dashboards/` |
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
kubectl apply -f deployments/configmaps/
```

---

## 📦 Import Paths

```go
import (
    "github.com/kube-zen/zen-watcher/pkg/adapter/generic"  // Source adapters
    "github.com/kube-zen/zen-watcher/pkg/config"           // Configuration
    "github.com/kube-zen/zen-watcher/pkg/processor"        // Processing pipeline
    "github.com/kube-zen/zen-watcher/pkg/watcher"          // Watchers
    "github.com/kube-zen/zen-watcher/pkg/filter"           // Filtering
    "github.com/kube-zen/zen-watcher/pkg/server"           // HTTP server
    "github.com/kube-zen/zen-watcher/pkg/metrics"          // Prometheus metrics
    "github.com/kube-zen/zen-watcher/pkg/orchestrator"     // Orchestration
    "github.com/kube-zen/zen-watcher/pkg/sdk"              // SDK integration
)
```

---

## ✨ Why This Structure?

### For Zen Watcher Specifically

1. **Not an Operator Framework Project**
   - We're not using Kubebuilder or Operator SDK
   - Don't need api/ versioning structure
   - CRD types are in deployments/crds/

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
