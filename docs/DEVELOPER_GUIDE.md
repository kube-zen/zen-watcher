# Zen Watcher Developer Guide

**Version:** 1.0.0-alpha  
**Go Version:** 1.24+ (tested on 1.24)  
**License:** Apache 2.0

---

## Table of Contents

1. [Getting Started](#getting-started)
2. [Project Structure](#project-structure)
3. [Code Architecture](#code-architecture)
4. [Adding a New Watcher](#adding-a-new-watcher)
5. [Testing](#testing)
6. [Building & Deployment](#building--deployment)
7. [Best Practices](#best-practices)
8. [Integrations](#integrations)

---

## Getting Started

### Prerequisites

- **Go 1.24+** installed (tested on 1.24)
- **Docker or Podman** for building images
- **kubectl** for Kubernetes access
- **Kubernetes cluster 1.28+** (any distribution)
- Basic understanding of Kubernetes CRDs

### Development Setup

```bash
# Clone the repository
git clone https://github.com/kube-zen/zen-watcher.git
cd zen-watcher

# Install Go dependencies
go mod download
go mod verify

# Install development tools
make install-tools

# Install git hooks
make install-hooks

# Build and test
make all

# Build the binary
go build -o zen-watcher ./cmd/zen-watcher

# Run locally (requires kubeconfig)
export KUBECONFIG=~/.kube/config
./zen-watcher
```

### Installing Development Tools

#### Go Tools

```bash
# Vulnerability scanner
go install golang.org/x/vuln/cmd/govulncheck@latest

# Security linter
go install github.com/securego/gosec/v2/cmd/gosec@latest

# Static analysis
go install honnef.co/go/tools/cmd/staticcheck@latest

# Or use Makefile
make install-tools
```

#### Container Tools

**Trivy** (vulnerability scanner):
```bash
# macOS
brew install trivy

# Linux
wget -qO - https://aquasecurity.github.io/trivy-repo/deb/public.key | sudo apt-key add -
echo "deb https://aquasecurity.github.io/trivy-repo/deb $(lsb_release -sc) main" | sudo tee /etc/apt/sources.list.d/trivy.list
sudo apt update && sudo apt install trivy
```

**Syft** (SBOM generator):
```bash
# macOS
brew install syft

# Linux
curl -sSfL https://raw.githubusercontent.com/anchore/syft/main/install.sh | sh -s -- -b /usr/local/bin
```

**Cosign** (container signing):
```bash
# macOS
brew install cosign

# Linux
go install github.com/sigstore/cosign/v2/cmd/cosign@latest
```

### Development Workflow

#### 1. Make Changes

```bash
# Create feature branch
git checkout -b feature/my-feature

# Edit code
vim cmd/zen-watcher/main.go

# Format code
gofmt -w .
```

#### 2. Run Quality Checks

```bash
# Run all checks
make all

# Or individual checks
make fmt        # Format check
make vet        # Go vet
make lint       # All linters
make test       # Run tests
make security   # Security scans
```

#### 3. Build and Test Container Image (Docker/Podman)

```bash
# Build image (uses Docker or Podman)
make docker-build

# Scan for vulnerabilities
make docker-scan

# Generate SBOM
make docker-sbom

# All-in-one
make docker-all
```

#### 4. Test Locally

```bash
# Connect to your Kubernetes cluster (any distribution)
# For local testing, you can use k3d, minikube, kind, etc.
# Example with k3d (optional):
#   k3d cluster create zen-dev

# Deploy zen-watcher
kubectl apply -f deployments/crds/
kubectl apply -f deployments/base/

# Check logs
kubectl logs -n zen-system deployment/zen-watcher -f

# Install security tools for testing
kubectl apply -f test-manifests/trivy-operator.yaml
kubectl apply -f test-manifests/kyverno.yaml

# Verify events are created
kubectl get observations -A
```

#### 5. Commit

```bash
# Stage changes
git add .

# Commit (pre-commit hooks run automatically)
git commit -m "feat: Add new feature"

# Push
git push origin feature/my-feature
```

### Pre-Commit Checks

The `.githooks/pre-commit` hook runs automatically on every commit and checks:

#### Code Quality
- ✅ `go fmt` - Code formatting
- ✅ `go vet` - Common errors
- ✅ `go mod tidy` - Dependency management
- ✅ `go build` - Compilation

#### Security
- ✅ `govulncheck` - Known vulnerabilities in dependencies
- ✅ `gosec` - Security issues in code
- ✅ `trivy image` - Container vulnerabilities (if Dockerfile changed)

#### File Validation
- ✅ YAML syntax validation
- ✅ Trailing whitespace check
- ✅ Line ending consistency

**To skip hooks** (not recommended):
```bash
git commit --no-verify -m "message"
```

### Makefile Targets

#### Build & Test
```bash
make build          # Build optimized binary
make test           # Run tests with coverage
make lint           # Run all linters
make security       # Run security scans
make all            # Run everything
```

#### Container Build (Docker/Podman)
```bash
make docker-build   # Build container image (Docker or Podman)
make docker-scan    # Scan image with Trivy
make docker-sbom    # Generate SBOM
make docker-sign    # Sign with Cosign
make docker-verify  # Verify signature
make docker-all     # Build + Scan + SBOM
```

#### CI/CD
```bash
make ci             # Full CI pipeline (all + docker-all)
```

#### Utilities
```bash
make install-tools  # Install Go dev tools
make install-hooks  # Configure git hooks
make clean          # Clean build artifacts
make help           # Show all targets
```

### Security Scanning

#### Vulnerability Scanning

**Go dependencies:**
```bash
# Check for known vulnerabilities
govulncheck ./...

# Update vulnerable dependencies
go get -u ./...
go mod tidy
```

**Docker image:**
```bash
# Scan with Trivy
trivy image --severity HIGH,CRITICAL kubezen/zen-watcher:latest

# Scan with Grype (alternative)
grype kubezen/zen-watcher:latest
```

#### Static Analysis

```bash
# Run gosec
gosec ./...

# Run staticcheck
staticcheck ./...

# Run both
make security
```

#### SBOM Generation

```bash
# Generate SBOM for Go code
syft dir:. -o json > code-sbom.json

# Generate SBOM for Docker image
syft kubezen/zen-watcher:latest -o spdx-json > image-sbom.spdx.json

# Or use Makefile
make docker-sbom
```

### Container Signing

#### Generate Key Pair

```bash
# Generate Cosign key pair (one-time setup)
cosign generate-key-pair

# This creates:
#   cosign.key (private key - keep secret!)
#   cosign.pub (public key - distribute)
```

#### Sign Image

```bash
# Sign image
cosign sign --key cosign.key kubezen/zen-watcher:1.0.19

# Or use Makefile
make docker-sign
```

#### Verify Signature

```bash
# Verify signature
cosign verify --key cosign.pub kubezen/zen-watcher:1.0.19

# Or use Makefile
make docker-verify
```

### Development Environment

```bash
# Connect to your Kubernetes cluster (any distribution)
# For local testing, you can use k3d, minikube, kind, or any Kubernetes cluster
# Example with k3d (optional, simple local option):
#   k3d cluster create zen-dev --agents 0 --api-port 6551

# Install test tools (optional, for testing integrations)
kubectl apply -f test-manifests/trivy-operator.yaml
kubectl apply -f test-manifests/kyverno.yaml

# Deploy zen-watcher
kubectl apply -f deployments/crds/
kubectl apply -f deployments/base/
```

---

## Project Structure

```
zen-watcher/
├── cmd/zen-watcher/
│   └── main.go                    # Main entry point (~143 lines, wiring only)
│
├── pkg/                           # Public library code
│   ├── server/
│   │   └── http.go               # HTTP server & webhook handlers
│   ├── watcher/
│   │   ├── informer_handlers.go   # EventProcessor (informer-based)
│   │   ├── webhook_processor.go  # WebhookProcessor (webhook-based)
│   │   ├── configmap_poller.go    # ConfigMap polling (batch-based)
│   │   └── factory.go             # Processor factory
│   └── metrics/
│       └── definitions.go         # Prometheus metric definitions
│
├── internal/                      # Internal implementation details
│   ├── kubernetes/
│   │   ├── setup.go              # K8s client initialization
│   │   └── informers.go          # Informer setup & handlers
│   └── lifecycle/
│       └── shutdown.go           # Signal handling & graceful shutdown
│
├── build/
│   └── Dockerfile                 # Multi-stage Docker build
│
├── deployments/
│   ├── crds/
│   │   └── observation_crd.yaml
│   └── base/
│       └── zen-watcher.yaml       # Deployment manifests
│
├── config/
│   ├── monitoring/
│   │   └── grafana-dashboard.json
│   └── rbac/
│       └── clusterrole.yaml
│
├── examples/
│   └── watch-events.go            # Integration examples
│
├── docs/
│   ├── SECURITY.md                # Security considerations
│   ├── DEPLOYMENT_SCENARIOS.md    # Deployment patterns
│   └── OPERATIONAL_EXCELLENCE.md  # Production operations
│
├── README.md                      # Main documentation
├── docs/ARCHITECTURE.md            # Architecture deep dive
├── CONTRIBUTING.md                 # Contribution guidelines
├── docs/DEVELOPER_GUIDE.md         # This file
├── LICENSE                        # Apache 2.0
└── go.mod                         # Go dependencies
```

---

## Code Architecture

### Why Modular Architecture Matters

The modular architecture isn't just about code organization—it fundamentally changes how you work with zen-watcher:

**🎯 Community Contributions Become Trivial**
- Want to add Wiz support? Add a `wiz_processor.go` and register it in `factory.go`.
- No need to understand the entire codebase—just implement one processor interface.
- Each processor is self-contained and independently testable.

**🧪 Testing is No Longer Scary**
- Test `configmap_poller.go` with a mock K8s client—no cluster needed.
- Test `http.go` with `net/http/httptest`—standard Go testing tools.
- Each component can be tested in isolation, making unit tests practical.

**🚀 Future Extensions Slot Cleanly**
- New event source? Choose the right processor type and implement it.
- Need a new package? Create `pkg/sync/` or any other module—the architecture scales.
- Extensions don't require refactoring existing code.

**⚡ Your Personal Bandwidth is Freed**
- You no longer maintain code—you orchestrate it.
- Each module has clear responsibilities and boundaries.
- Changes are localized, reducing risk and review time.

### Modular Architecture

Zen Watcher uses a **modular, scalable architecture** following Kubernetes informer patterns:

#### Processor Types

**1. EventProcessor (`pkg/watcher/informer_handlers.go`)**
- Handles **CRD-based sources** (Kyverno, Trivy)
- Uses **Kubernetes informers** for real-time event processing
- Thread-safe deduplication with `sync.RWMutex`
- Automatic reconnection on API server errors

**2. WebhookProcessor (`pkg/watcher/webhook_processor.go`)**
- Handles **webhook-based sources** (Falco, Audit)
- Processes events from HTTP webhooks
- Thread-safe deduplication per source
- Channel-based async processing

**3. ConfigMapPoller (`pkg/watcher/configmap_poller.go`)**
- Handles **batch sources** (Kube-bench, Checkov)
- Periodic polling (5-minute interval)
- Used when CRDs/webhooks aren't available
- Self-contained polling logic with proper error handling

### Main Entry Point (`cmd/zen-watcher/main.go`)

The main entry point is now **minimal wiring only** (~143 lines). All business logic lives in dedicated packages:

```go
func main() {
    // 1. Setup signal handling
    ctx, stopCh := lifecycle.SetupSignalHandler()
    
    // 2. Initialize components
    m := metrics.NewMetrics()
    clients := kubernetes.NewClients()
    gvrs := kubernetes.NewGVRs()
    informerFactory := kubernetes.NewInformerFactory(clients.Dynamic)
    eventProcessor, webhookProcessor := watcher.NewProcessors(...)
    
    // 3. Setup informers
    kubernetes.SetupInformers(ctx, informerFactory, gvrs, eventProcessor, stopCh)
    
    // 4. Create and start HTTP server
    httpServer := server.NewServer(...)
    httpServer.Start(ctx, &wg)
    
    // 5. Start background processors
    // 6. Wait for shutdown
    lifecycle.WaitForShutdown(ctx, &wg, cleanup)
}
```

#### Code Structure

```go
// 1. Initialization (lines 27-100)
// - Create Kubernetes clients (kubernetes, dynamic)
// - Setup Prometheus metrics
// - Configure tool namespaces

// 2. HTTP Server Setup (lines 100-275)
// - /health endpoint
// - /ready endpoint
// - /metrics endpoint (Prometheus)
// - /falco/webhook endpoint
// - /audit/webhook endpoint

// 3. Processor Initialization (lines 277-282)
// - eventProcessor := watcher.NewEventProcessor(...)
// - webhookProcessor := watcher.NewWebhookProcessor(...)

// 4. Informer Setup (lines 284-320)
// - Kyverno PolicyReport informer
// - Trivy VulnerabilityReport informer
// - Start informer factory
// - Wait for cache sync

// 5. Main Loop (lines 322-400)
// - Handle webhook channels (Falco, Audit)
// - Periodic ConfigMap checks (Kube-bench, Checkov)
// - Graceful shutdown
```

#### Key Data Structures

**Event Channels**: For webhook-based tools
```go
falcoAlertsChan := make(chan map[string]interface{}, 100)
auditEventsChan := make(chan map[string]interface{}, 200)
```

**Informer Factory**: For CRD-based tools
```go
informerFactory := dynamicinformer.NewDynamicSharedInformerFactory(
    dynClient, 
    time.Minute*30, // Resync interval
)
```

**Processors**: Modular event handlers
```go
eventProcessor := watcher.NewEventProcessor(dynClient, eventGVR, eventsTotal)
webhookProcessor := watcher.NewWebhookProcessor(dynClient, eventGVR, eventsTotal)
```

### Informer-Based Pattern (CRD Sources)

For tools that emit Kubernetes CRDs (Kyverno, Trivy):

```go
// 1. Create informer factory
informerFactory := dynamicinformer.NewDynamicSharedInformerFactory(
    dynClient, 
    time.Minute*30, // Resync every 30 minutes
)

// 2. Create informer for your tool's CRD
myToolGVR := schema.GroupVersionResource{
    Group:    "mytool.io",
    Version:  "v1",
    Resource: "mytoolreports",
}
informer := informerFactory.ForResource(myToolGVR).Informer()

// 3. Add event handlers
informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
    AddFunc: func(obj interface{}) {
        report := obj.(*unstructured.Unstructured)
        eventProcessor.ProcessMyTool(ctx, report)
    },
    UpdateFunc: func(oldObj, newObj interface{}) {
        report := newObj.(*unstructured.Unstructured)
        eventProcessor.ProcessMyTool(ctx, report)
    },
})

// 4. Start informers
informerFactory.Start(stopCh)
cache.WaitForCacheSync(ctx.Done(), informer.HasSynced)
```

### Webhook-Based Pattern (Push Sources)

For tools that can send HTTP webhooks (Falco, Audit):

```go
// 1. Create webhook handler
http.HandleFunc("/mytool/webhook", func(w http.ResponseWriter, r *http.Request) {
    var event map[string]interface{}
    json.NewDecoder(r.Body).Decode(&event)
    
    // Send to channel for async processing
    select {
    case myToolChan <- event:
        w.WriteHeader(http.StatusOK)
    default:
        w.WriteHeader(http.StatusServiceUnavailable)
    }
})

// 2. Process in main loop
select {
case event := <-myToolChan:
    webhookProcessor.ProcessMyTool(ctx, event)
default:
    // No events
}
```

---

## Extending Zen Watcher with Sink Controllers

**Important**: Zen Watcher core stays pure — it only watches sources and writes Observation CRDs. Zero egress, zero secrets, zero external dependencies.

**But the Observation CRD is a universal signal format**, and you can build lightweight "sink" controllers that:

- Watch `Observation` CRDs
- Filter by category, severity, source, labels, etc.
- Forward to external systems:
  - 📢 Slack
  - 🚨 PagerDuty
  - 🛠️ ServiceNow
  - 📊 Datadog / Splunk / SIEMs
  - 📧 Email
  - 🔔 Custom webhooks

### Why This Pattern Works

1. **Zen Watcher stays pure**
   - Only watches sources → writes Observation CRs
   - Zero outbound traffic
   - Zero secrets
   - Zero config for external systems

2. **Sink controllers are separate, optional components**
   - Deploy only if needed
   - Use SealedSecrets or external secret managers for credentials
   - Can be built by the community or enterprise users

3. **Creates an ecosystem**
   - "If you can watch a CRD, you can act on it."
   - Enterprise users can build their own sinks without waiting
   - Follows the Prometheus Alertmanager / Flux / Crossplane pattern

### Example Sink Controller Structure

```go
pkg/sink/
├── sink.go          // interface
├── slack.go         // implements Sink for Slack
├── pagerduty.go     // implements Sink for PagerDuty
└── controller.go    // watches Observations, routes to sinks
```

**See [CONTRIBUTING.md](CONTRIBUTING.md) for detailed guidelines on building sink controllers.**

---

## Adding a New Watcher

**See [CONTRIBUTING.md](CONTRIBUTING.md) for detailed guidelines.** This section provides a quick overview.

### Step 1: Choose the Right Processor Type

**If your tool emits CRDs → Use EventProcessor (Informer-based)**
- Real-time event processing
- Automatic reconnection
- Efficient resource usage
- Example: Kyverno, Trivy

**If your tool can send webhooks → Use WebhookProcessor**
- Immediate event delivery
- No polling overhead
- Example: Falco, Audit

**If your tool writes ConfigMaps → Use periodic polling**
- 5-minute interval
- Use when CRDs/webhooks aren't available
- Example: Kube-bench, Checkov

### Step 2: Implement Processor Method

**For EventProcessor (CRD sources):**

Add to `pkg/watcher/informer_handlers.go`:

```go
func (ep *EventProcessor) ProcessMyTool(ctx context.Context, report *unstructured.Unstructured) {
    // 1. Extract data from report
    spec, found, _ := unstructured.NestedMap(report.Object, "spec")
    if !found { return }
    
    // 2. Create Observation structure
    event := &unstructured.Unstructured{
        Object: map[string]interface{}{
            "apiVersion": "zen.kube-zen.io/v1",
            "kind":       "Observation",
            "metadata": map[string]interface{}{
                "generateName": "mytool-",
                "namespace":    report.GetNamespace(),
                "labels": map[string]interface{}{
                    "source":   "mytool",
                    "category": "security",
                    "severity": "HIGH",
                },
            },
            "spec": map[string]interface{}{
                "source":     "mytool",
                "category":   "security",
                "severity":   "HIGH",
                "eventType":  "mytool-event",
                "detectedAt": time.Now().Format(time.RFC3339),
                "resource": map[string]interface{}{
                    "kind":      "Pod",
                    "name":      report.GetName(),
                    "namespace": report.GetNamespace(),
                },
                "details": map[string]interface{}{
                    // Your tool-specific details
                },
            },
        },
    }
    
    // 3. Use centralized observation creator
    // Flow: filter() → normalize() → dedup() → create CRD + update metrics + log
    err := ep.observationCreator.CreateObservation(ctx, event)
    if err != nil {
        log.Printf("  ⚠️  Failed to create Observation: %v", err)
    }
}
```

**Key Points:**
- **No manual deduplication** - Handled by `ObservationCreator`
- **No manual metrics** - Handled by `ObservationCreator`
- **Filtering happens automatically** - Before deduplication and CRD creation
- **All sources use the same flow** - Consistent behavior

**For WebhookProcessor (webhook sources):**

Add to `pkg/watcher/webhook_processor.go`:

```go
func (wp *WebhookProcessor) ProcessMyTool(ctx context.Context, event map[string]interface{}) error {
    // 1. Extract data from webhook payload
    // (Basic validation only - filtering happens in ObservationCreator)
    
    // 2. Create Observation structure
    observation := &unstructured.Unstructured{
        Object: map[string]interface{}{
            "apiVersion": "zen.kube-zen.io/v1",
            "kind":       "Observation",
            "metadata": map[string]interface{}{
                "generateName": "mytool-",
                "namespace":    "default",
                "labels": map[string]interface{}{
                    "source":   "mytool",
                    "category": "security",
                    "severity": "HIGH",
                },
            },
            "spec": map[string]interface{}{
                "source":     "mytool",
                "category":   "security",
                "severity":   "HIGH",
                "eventType":  "mytool-event",
                "detectedAt": time.Now().Format(time.RFC3339),
                "resource": map[string]interface{}{
                    // Extract from webhook payload
                },
                "details": map[string]interface{}{
                    // Webhook-specific details
                },
            },
        },
    }
    
    // 3. Use centralized observation creator
    // Flow: filter() → normalize() → dedup() → create CRD + update metrics + log
    err := wp.observationCreator.CreateObservation(ctx, observation)
    if err != nil {
        return fmt.Errorf("failed to create Observation: %w", err)
    }
    
    return nil
}
    
    // 2. Generate deduplication key
    dedupKey := fmt.Sprintf("%s/%s", 
        fmt.Sprintf("%v", event["id"]),
        fmt.Sprintf("%v", event["timestamp"]))
    
    // 3. Check deduplication (thread-safe)
    wp.mu.Lock()
    if wp.dedupKeys["mytool"] == nil {
        wp.dedupKeys["mytool"] = make(map[string]bool)
    }
    if wp.dedupKeys["mytool"][dedupKey] {
        wp.mu.Unlock()
        return nil // Skip duplicate
    }
    wp.dedupKeys["mytool"][dedupKey] = true
    wp.mu.Unlock()
    
    // 4. Create Observation
    event := &unstructured.Unstructured{...}
    _, err := wp.dynClient.Resource(wp.eventGVR).Create(ctx, event, metav1.CreateOptions{})
    
    // 5. Update metrics
    if wp.eventsTotal != nil {
        wp.eventsTotal.WithLabelValues("mytool", "security", "HIGH").Inc()
    }
    
    return err
}
```

### Step 3: Register in informers.go

**For Informer-based (CRD sources):**

Add to `internal/kubernetes/informers.go` in the `SetupInformers` function:

```go
// Add to informer setup section
myToolGVR := schema.GroupVersionResource{
    Group:    "mytool.io",
    Version:  "v1",
    Resource: "mytoolreports",
}
myToolInformer := factory.ForResource(myToolGVR).Informer()
myToolInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
    AddFunc: func(obj interface{}) {
        report, ok := obj.(*unstructured.Unstructured)
        if !ok {
            log.Printf("⚠️  Invalid object type in MyTool AddFunc")
            return
        }
        eventProcessor.ProcessMyTool(ctx, report)
    },
    UpdateFunc: func(oldObj, newObj interface{}) {
        report, ok := newObj.(*unstructured.Unstructured)
        if !ok {
            log.Printf("⚠️  Invalid object type in MyTool UpdateFunc")
            return
        }
        eventProcessor.ProcessMyTool(ctx, report)
    },
})

// Add to cache sync wait:
cache.WaitForCacheSync(ctx.Done(), ..., myToolInformer.HasSynced)
```

**For Webhook-based:**

1. **Add webhook handler** in `pkg/server/http.go`:

```go
// In registerHandlers():
http.HandleFunc("/mytool/webhook", s.handleMyToolWebhook)

// Add handler method:
func (s *Server) handleMyToolWebhook(w http.ResponseWriter, r *http.Request) {
    var event map[string]interface{}
    if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }
    
    select {
    case s.myToolChan <- event:
        w.WriteHeader(http.StatusOK)
        s.webhookMetrics.WithLabelValues("mytool", "success").Inc()
    default:
        w.WriteHeader(http.StatusServiceUnavailable)
        s.webhookMetrics.WithLabelValues("mytool", "dropped").Inc()
    }
}
```

2. **Add channel and processing** in `main.go`:

```go
// Create channel
myToolChan := make(chan map[string]interface{}, 100)

// Pass to server
httpServer := server.NewServer(..., myToolChan, ...)

// Process in webhook processing goroutine:
case event := <-myToolChan:
    webhookProcessor.ProcessMyTool(ctx, event)
```
            name := fmt.Sprintf("%v", metadata["name"])
            
            // Create dedup key
### Step 4: Add RBAC Permissions

Update `deployments/rbac/clusterrole.yaml`:

```yaml
- apiGroups: ["mytool.io"]
  resources: ["mytoolreports"]
  verbs: ["get", "list", "watch"]
```

### Step 5: Add to Documentation

Update `README.md` and `docs/ARCHITECTURE.md` to include the new tool.

---

## Testing

### Architecture Overview

Zen Watcher uses a **modular, scalable architecture** with clear separation of concerns. This design delivers real benefits:

**🎯 Community Contributions Become Trivial**
- Want to add Wiz support? Add a `wiz_processor.go` and register it in `factory.go`.
- No need to understand the entire codebase—just implement one processor interface.

**🧪 Testing is No Longer Scary**
- Test `configmap_poller.go` with a mock K8s client—no cluster needed.
- Test `http.go` with `net/http/httptest`—standard Go testing tools.
- Each component can be tested in isolation.

**🚀 Future Extensions Slot Cleanly**
- New event source? Choose the right processor type and implement it.
- Need a new package? Create `pkg/sync/` or any other module—the architecture scales.

**⚡ Your Personal Bandwidth is Freed**
- You no longer maintain code—you orchestrate it.
- Each module has clear responsibilities and boundaries.

**Architecture Components:**

1. **Informer-Based (CRD Sources)**: Real-time processing via Kubernetes informers
   - Kyverno PolicyReports
   - Trivy VulnerabilityReports
   - Implemented in `pkg/watcher/informer_handlers.go`

2. **Webhook-Based (Push Sources)**: Immediate event delivery via HTTP webhooks
   - Falco alerts
   - Kubernetes audit events
   - Implemented in `pkg/watcher/webhook_processor.go`

3. **Informer-Based**: Watch any Kubernetes resource (CRDs, ConfigMaps, Pods, etc.)
   - Kube-bench reports (via ConfigMap informer)
   - Any custom CRD
   - Checkov reports
   - Implemented in `pkg/watcher/configmap_poller.go`

**See [CONTRIBUTING.md](../CONTRIBUTING.md) for detailed guidelines on adding new watchers.**

### Unit Testing (Future)

Currently, zen-watcher doesn't have unit tests due to its integration nature. Future testing strategy:

```bash
# Run tests
go test -v ./...

# With coverage
go test -v -coverprofile=coverage.out ./...

# View coverage
go tool cover -html=coverage.out
```

### Integration Testing

```bash
# 1. Connect to your Kubernetes cluster (any distribution)
#    For local testing, you can use k3d, minikube, kind, etc.
#    Example with k3d (optional):
#      k3d cluster create zen-test

# 2. Deploy zen-watcher
kubectl apply -f deployments/crds/
kubectl apply -f deployments/base/

# 3. Install test tools
kubectl apply -f test-manifests/

# 4. Wait for events
sleep 60

# 5. Verify
kubectl get observations -A

# 6. Cleanup (if using local cluster)
#    Recommended: Use cleanup script (works with k3d, kind, minikube)
#    ZEN_CLUSTER_NAME=zen-test ./scripts/cleanup-demo.sh k3d
#    Or manually: k3d cluster delete zen-test
```

### Manual Smoke Test

```bash
# Build and run locally
make build
./zen-watcher

# In another terminal:
# - Check health: curl http://localhost:8080/health
# - Check metrics: curl http://localhost:9090/metrics
# - Send test webhook: curl -X POST http://localhost:8080/falco/webhook -d '{...}'
```

### Manual Testing Checklist

**Before Release:**
- [ ] All watchers detect their tools correctly
- [ ] Events are created for each tool
- [ ] Deduplication prevents duplicates
- [ ] Categories are correct (security/compliance/performance/operations/cost)
- [ ] Severities are mapped correctly
- [ ] Webhooks respond to POST requests
- [ ] Health endpoint returns 200
- [ ] Metrics endpoint exports data
- [ ] No RBAC permission errors in logs
- [ ] NetworkPolicy allows required traffic

---

## Building & Deployment

### Local Build

```bash
# Development build (with debug symbols)
go build -o zen-watcher ./cmd/zen-watcher

# Production build (optimized)
go build \
    -ldflags="-w -s" \
    -trimpath \
    -o zen-watcher \
    ./cmd/zen-watcher
```

**Build flags explained:**
- `-ldflags="-w -s"`: Strip debug info and symbol table (reduces size by ~30%)
- `-trimpath`: Remove file system paths from binary (security best practice)
- `CGO_ENABLED=0`: Static binary, no C dependencies

### Container Build (Docker/Podman)

```bash
# Build image (using Docker or Podman)
docker build \
    --no-cache \
    --pull \
    -t kubezen/zen-watcher:1.0.0-alpha \
    -f build/Dockerfile \
    .

# Or with Podman (drop-in replacement):
# podman build --no-cache --pull -t kubezen/zen-watcher:1.0.0-alpha -f build/Dockerfile .

# Push to registry
docker push kubezen/zen-watcher:1.0.0-alpha
# Or: podman push kubezen/zen-watcher:1.0.0-alpha
```

**Dockerfile optimization:**
- Multi-stage build (builder + distroless)
- Uses `golang:1.24-alpine` for small builder image
- Final image based on `gcr.io/distroless/static:nonroot` (~29MB)
- No shell, no package manager in final image

### Deployment

```bash
# Standard deployment
kubectl apply -f deployments/crds/
kubectl apply -f deployments/base/

# Helm deployment
helm repo add kube-zen https://kube-zen.github.io/helm-charts
helm repo update
helm install zen-watcher kube-zen/zen-watcher \
    --namespace zen-system \
    --create-namespace

# Verify
kubectl get pods -n zen-system
kubectl logs -n zen-system deployment/zen-watcher -f
```

---

## Best Practices

### Code Style

1. **Keep main.go simple and linear**
   - Single file for easy navigation
   - Clear section comments for each watcher
   - Consistent error handling

2. **Logging standards**
   ```go
   log.Println("✅ Success message")
   log.Println("ℹ️  Informational message")
   log.Println("⚠️  Warning message")
   log.Printf("  → Action being taken...")
   log.Printf("  ✓ Confirmation message")
   ```

3. **Error handling**
   ```go
   if err != nil {
       log.Printf("  ⚠️  Failed to X: %v", err)
       // Continue, don't crash
   }
   ```

4. **Deduplication pattern**
   ```go
   // Always use hash maps for O(1) lookups
   existingKeys := make(map[string]bool)
   if existingKeys[key] {
       continue  // Skip duplicate
   }
   existingKeys[key] = true
   ```

### Performance Guidelines

1. **Use buffered channels** for webhooks
   ```go
   mytoolChan := make(chan map[string]interface{}, 100)
   ```

2. **Limit API calls**
   - Batch operations when possible
   - Use label selectors to filter
   - Cache tool detection results

3. **Memory management**
   - Clear maps after use
   - Don't accumulate unbounded data
   - Use streaming for large datasets

### Security Guidelines

1. **Never log sensitive data**
   ```go
   // Bad: log.Printf("Secret: %s", secret)
   // Good: log.Printf("Processing secret: %s", secretName)
   ```

2. **Validate all webhook inputs**
   ```go
   var event map[string]interface{}
   if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
       log.Printf("⚠️  Invalid webhook payload: %v", err)
       w.WriteHeader(http.StatusBadRequest)
       return
   }
   ```

3. **Use least privilege RBAC**
   - Only request permissions you need
   - Prefer read-only when possible
   - Document why each permission is required

### Documentation Standards

1. **Every new feature needs:**
   - README.md update
   - docs/ARCHITECTURE.md update if design changes
   - CHANGELOG.md entry
   - Inline code comments

2. **Code comments should explain WHY, not WHAT**
   ```go
   // Bad: Increment counter
   counter++
   
   // Good: Track total events for metrics export
   counter++
   ```

3. **Keep documentation up-to-date**
   - Update docs in same commit as code
   - Include examples for new features
   - Add troubleshooting tips

---

## Debugging

### Enable Verbose Logging

```bash
# Set log level
export LOG_LEVEL=DEBUG

# Run watcher
./zen-watcher
```

### Use Delve Debugger

```bash
# Install delve
go install github.com/go-delve/delve/cmd/dlv@latest

# Debug
dlv debug ./cmd/zen-watcher

# In dlv:
(dlv) break main.main
(dlv) continue
(dlv) print toolStates
(dlv) next
```

### Debug in Kubernetes

```bash
# Check RBAC permissions
kubectl auth can-i get vulnerabilityreports \
  --as=system:serviceaccount:zen-system:zen-watcher

# Check NetworkPolicy
kubectl describe networkpolicy zen-watcher -n zen-system

# Check events
kubectl get events -n zen-system | grep zen-watcher

# Get logs
kubectl logs -n zen-system deployment/zen-watcher --tail=100 -f
```

### Debugging Tips

#### Enable Verbose Logging in Code

```go
// Add debug logs for troubleshooting
log.Printf("DEBUG: Report spec: %+v", spec)
log.Printf("DEBUG: Dedup key: %s", dedupKey)
log.Printf("DEBUG: Existing keys: %d", len(existingKeys))
```

#### Kubernetes Debugging

```bash
# Check RBAC
kubectl auth can-i get vulnerabilityreports --as=system:serviceaccount:zen-system:zen-watcher

# Check NetworkPolicy
kubectl describe networkpolicy zen-watcher -n zen-system

# Check events
kubectl get events -n zen-system --field-selector involvedObject.name=zen-watcher

# Check CRD status
kubectl get crd observations.zen.kube-zen.io -o yaml
```

---

## Common Patterns

### Pattern 1: Using Informers for CRD-Based Reports (Recommended)

```go
// Define GVR
gvr := schema.GroupVersionResource{
    Group:    "aquasecurity.github.io",
    Version:  "v1alpha1",
    Resource: "vulnerabilityreports",
}

// Create informer (in main.go setup)
informer := informerFactory.ForResource(gvr).Informer()
informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
    AddFunc: func(obj interface{}) {
        eventProcessor.ProcessMyTool(ctx, obj.(*unstructured.Unstructured))
    },
})

// Start informer
informerFactory.Start(stopCh)
cache.WaitForCacheSync(ctx.Done(), informer.HasSynced)
```

**Note**: Prefer informers over `List()` for real-time processing. Use `List()` only for one-time operations or batch processing.

### Pattern 2: Parsing Nested JSON

```go
// Extract nested field safely
spec, ok := report.Object["spec"].(map[string]interface{})
if !ok { continue }

vulnerabilities, ok := spec["vulnerabilities"].([]interface{})
if !ok { continue }

for _, v := range vulnerabilities {
    vuln := v.(map[string]interface{})
    vulnID := fmt.Sprintf("%v", vuln["vulnerabilityID"])
}
```

### Pattern 3: Creating Observations

```go
event := &unstructured.Unstructured{
    Object: map[string]interface{}{
        "apiVersion": "zen.kube-zen.io/v1",
        "kind":       "Observation",
        "metadata": map[string]interface{}{
            "generateName": "mytool-",
            "namespace":    namespace,
            "labels": map[string]interface{}{
                "source":   "mytool",
                "category": "security",
                "severity": "HIGH",
            },
        },
        "spec": map[string]interface{}{
            "source":     "mytool",
            "category":   "security",
            "severity":   "HIGH",
            "eventType":  "mytool-event",
            "detectedAt": time.Now().Format(time.RFC3339),
            "resource": map[string]interface{}{
                "kind":      "Pod",
                "name":      name,
                "namespace": namespace,
            },
            "details": map[string]interface{}{
                "key": "value",
            },
        },
    },
}

_, err := dynClient.Resource(eventGVR).Namespace(namespace).Create(ctx, event, metav1.CreateOptions{})
```

---

## Release Process

### 1. Update Version

```bash
# Update version in main.go (or use build-time version injection)
sed -i 's/v1.0.19/v1.0.20/' cmd/zen-watcher/main.go

# Update CHANGELOG.md
vim CHANGELOG.md
```

### 2. Run Full Pipeline

```bash
# Run all checks
make ci

# This runs:
# - go fmt, go vet, staticcheck
# - go test with coverage
# - govulncheck, gosec
# - docker build
# - trivy scan
# - SBOM generation
```

### 3. Tag Release

```bash
# Commit changes
git add .
git commit -m "Release v1.0.20"

# Tag
git tag -a v1.0.20 -m "Release v1.0.20"
git push origin v1.0.20
```

### 4. Build and Push

```bash
# Build production image
make docker-build VERSION=1.0.20

# Scan
make docker-scan

# Generate SBOM
make docker-sbom

# Sign (if keys are set up)
make docker-sign

# Push
docker push kubezen/zen-watcher:1.0.20
docker tag kubezen/zen-watcher:1.0.20 kubezen/zen-watcher:latest
docker push kubezen/zen-watcher:latest
```

### 5. Update Helm Charts

Helm charts are maintained in the separate [helm-charts](https://github.com/kube-zen/helm-charts) repository. To update charts:

```bash
# Clone the helm-charts repository
git clone https://github.com/kube-zen/helm-charts.git
cd helm-charts/charts/zen-watcher

# Update values.yaml
vim values.yaml
# Change: tag: "1.0.20"

# Commit and push
git add values.yaml
git commit -m "zen-watcher v1.0.20"
git push
```

---

## Troubleshooting

### "go fmt" fails in pre-commit

```bash
# Format all files
gofmt -w .

# Check what needs formatting
gofmt -l .
```

### "govulncheck" finds vulnerabilities

```bash
# Update dependencies
go get -u ./...
go mod tidy

# Verify fix
govulncheck ./...
```

### "trivy" finds HIGH/CRITICAL vulnerabilities

```bash
# Rebuild with updated base image
docker build --no-cache --pull -f build/Dockerfile .

# Check again
trivy image kubezen/zen-watcher:latest
```

### Pre-commit hook is slow

```bash
# Skip Docker scanning for minor changes
export SKIP_DOCKER_SCAN=1
git commit -m "message"

# Or skip all hooks (not recommended)
git commit --no-verify -m "message"
```

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for detailed contribution guidelines.

**Quick Start:**
1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Test thoroughly
5. Submit a pull request

---

## Integrations

### Consuming Observations

If you want to **consume Observation CRDs** in your own controllers or services, see:

📖 **[docs/INTEGRATIONS.md](docs/INTEGRATIONS.md)** - Complete integration guide covering:

- ✅ **OpenAPI Schema** - Schema structure, required/optional fields, programmatic access
- ✅ **Schema Sync Guidance** - How CRD schema is synced across repositories
- ✅ **Kubernetes Informers** - Real-time event streaming with complete examples
- ✅ **kubewatch / Robusta Integration** - Route Observations to external webhooks/services
- ✅ **Controller Examples** - Full working examples with work queues and event handlers
- ✅ **Best Practices** - Filtering, rate limiting, monitoring

### Key Integration Points

1. **Watch Observations via Informers** (Recommended)
   - Real-time updates with automatic reconnection
   - See: [docs/INTEGRATIONS.md#consuming-observations-via-informers](docs/INTEGRATIONS.md#consuming-observations-via-informers)

2. **kubewatch / Robusta for Event Routing**
   - Route Observations to webhooks or CloudEvents endpoints
   - See: [docs/INTEGRATIONS.md#quick-start-use-kubewatch-recommended](docs/INTEGRATIONS.md#quick-start-use-kubewatch-recommended)

3. **OpenAPI Schema Reference**
   - Type-safe schema definition and validation
   - See: [docs/INTEGRATIONS.md#openapi-schema](docs/INTEGRATIONS.md#openapi-schema)

### Quick Example

```go
// Watch Observations with informer
observationGVR := schema.GroupVersionResource{
    Group:    "zen.kube-zen.io",
    Version:  "v1",
    Resource: "observations",
}

informer := factory.ForResource(observationGVR).Informer()
informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
    AddFunc: func(obj interface{}) {
        obs := obj.(*unstructured.Unstructured)
        // Process Observation
    },
})
```

For complete examples, see [docs/INTEGRATIONS.md](docs/INTEGRATIONS.md).

---

## Resources

- **Main README**: [README.md](README.md)
- **Architecture Details**: [ARCHITECTURE.md](ARCHITECTURE.md) (in docs/)
- **Security Docs**: [docs/SECURITY.md](docs/SECURITY.md)
- **Deployment Guide**: [docs/DEPLOYMENT_SCENARIOS.md](docs/DEPLOYMENT_SCENARIOS.md)
- **Integrations Guide**: [docs/INTEGRATIONS.md](docs/INTEGRATIONS.md)
- **Helm Charts**: [helm-charts repository](https://github.com/kube-zen/helm-charts)

---

**Questions?** Open an issue on GitHub or check the [documentation index](DOCUMENTATION_INDEX.md).

