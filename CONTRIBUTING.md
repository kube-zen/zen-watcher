# Contributing to Zen Watcher

Thank you for your interest in contributing to Zen Watcher! This document outlines best practices and guidelines for adding new watchers or improving the codebase.

## Code of Conduct

All contributors must follow our [Code of Conduct](CODE_OF_CONDUCT.md). By participating, you agree to uphold this code.

## Developer Certificate of Origin (DCO)

Zen Watcher uses the **Developer Certificate of Origin (DCO)** to certify that contributors have the right to submit their code for inclusion in the project.

### What is DCO?

The DCO is a lightweight alternative to a Contributor License Agreement (CLA). It certifies that you wrote the code or have the right to pass it on as open source.

### How to Sign

**Option 1: Sign-off in commit message**
```bash
git commit -s -m "Your commit message"
```

The `-s` flag adds a `Signed-off-by` line to your commit message:
```
Signed-off-by: Your Name <your.email@example.com>
```

**Option 2: Add sign-off manually**
Add this line to your commit message:
```
Signed-off-by: Your Name <your.email@example.com>
```

### DCO Bot

We use a DCO bot to verify that all commits are signed. If your PR has unsigned commits, the bot will guide you through signing them.

### Why DCO?

- **No legal complexity**: No need to sign separate agreements
- **Standard practice**: Used by Linux kernel, Kubernetes, and many CNCF projects
- **Apache 2.0 compatible**: Aligns with our Apache 2.0 license

## Where to Start

**New to zen-watcher?** Check out [docs/CONTRIBUTOR_TASKS.md](docs/CONTRIBUTOR_TASKS.md) for a curated list of tasks organized by difficulty:
- **Good First Tasks**: Documentation, examples, simple tests
- **Intermediate**: Example sources, dashboard improvements, code contributions
- **Advanced**: CRD evolution, informer changes, KEP-driven work

### Good First Issues

We label issues with `good first issue` to help new contributors get started. These issues are:
- Well-documented with clear acceptance criteria
- Suitable for first-time contributors
- Reviewed by maintainers before labeling

**Finding Good First Issues:**
- Filter by label: [good first issue](https://github.com/kube-zen/zen-watcher/labels/good%20first%20issue)
- Look for issues marked with the `good first issue` label
- Check [docs/CONTRIBUTOR_TASKS.md](docs/CONTRIBUTOR_TASKS.md) for curated tasks

All tasks are sourced from the roadmap and KEP, ensuring your contributions align with project priorities.

**What Good Looks Like**: See [docs/OPERATIONAL_EXCELLENCE.md](docs/OPERATIONAL_EXCELLENCE.md) for operational invariants and SLO-like targets that all changes must preserve.

## Architecture Principles

Zen Watcher follows **Kubernetes controller best practices** and uses a **modular, scalable architecture**. This design makes contributions easy:

> ⚠️ **Remember**: Zen Watcher core stays pure. All egress lives in separate controllers. See [Pure Core, Extensible Ecosystem](docs/ARCHITECTURE.md#7-pure-core-extensible-ecosystem).

**🎯 Adding a New Watcher is Trivial**
- Want to add Wiz support? Add a `wiz_processor.go` and register it in `factory.go`.
- No need to understand the entire codebase—just implement one processor interface.
- Each processor is self-contained and independently testable.

**🧪 Testing is Straightforward**
- Test `configmap_poller.go` with a mock K8s client—no cluster needed.
- Test `http.go` with `net/http/httptest`—standard Go testing tools.
- Each component can be tested in isolation, making unit tests practical.

**🚀 Future Extensions Slot Cleanly**
- New event source? Choose the right processor type and implement it.
- Need a new package? Create `pkg/sync/` or any other module—the architecture scales.
- Extensions don't require refactoring existing code.

**⚡ Maintenance is Minimal**
- You no longer maintain code—you orchestrate it.
- Each module has clear responsibilities and boundaries.
- Changes are localized, reducing risk and review time.

**Technical Architecture:**

### Event Source Types

1. **Informer-Based (CRD Sources)** - Use for tools that emit Kubernetes CRDs
   - Real-time event processing
   - Automatic reconnection on errors
   - Efficient resource usage
   - Example: Kyverno PolicyReports, Trivy VulnerabilityReports

2. **Webhook-Based (Push Sources)** - Use for tools that can send HTTP webhooks
   - Immediate event delivery
   - No polling overhead
   - Example: Falco, Kubernetes Audit Logs

3. **ConfigMap-Based (Batch Sources)** - Use for tools that write to ConfigMaps
   - Periodic polling (5-minute interval)
   - Use when CRDs or webhooks aren't available
   - Example: Kube-bench, Checkov

## Building Sink Controllers

**Zen Watcher stays pure**: It only watches sources and writes Observation CRDs. Zero egress, zero secrets.

**But you can build sink controllers** that watch Observations and forward them to external systems (Slack, PagerDuty, SIEMs, etc.).

### Sink Controller Pattern

1. **Watch Observation CRDs**
   ```go
   informer := factory.ForResource(observationGVR).Informer()
   informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
       AddFunc: func(obj interface{}) {
           obs := obj.(*unstructured.Unstructured)
           // Filter and route to sinks
       },
   })
   ```

2. **Implement Sink Interface**
   ```go
   type Sink interface {
       Send(ctx context.Context, observation *Observation) error
   }
   ```

3. **Filter by Criteria**
   - Category (security, compliance, etc.)
   - Severity (HIGH, MEDIUM, LOW)
   - Source (trivy, kyverno, falco, etc.)
   - Labels (custom filtering)

4. **Forward to External Systems**
   - Use SealedSecrets or external secret managers for credentials
   - Handle rate limiting and retries
   - Log failures without blocking

### Example: Slack Sink

```go
// pkg/sink/slack.go
type SlackSink struct {
    webhookURL string
    client     *http.Client
}

func (s *SlackSink) Send(ctx context.Context, obs *Observation) error {
    // Extract fields from Observation
    // Format Slack message
    // POST to webhook
}
```

### Deployment

- Deploy as separate, optional component
- Use RBAC to grant read access to Observations
- Store credentials in SealedSecrets or external secret manager
- Can be deployed per-namespace or cluster-wide

**See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for more details on the extensibility pattern.**

---

## Adding a New Watcher

**New:** Zen Watcher now uses a formal **Source Adapter** interface for adding new event sources. This makes it much easier to integrate new tools.

**📖 See [docs/SOURCE_ADAPTERS.md](docs/SOURCE_ADAPTERS.md) for the complete guide.**

The SourceAdapter interface provides:
- ✅ Standard Event model for normalization
- ✅ Consistent integration pattern
- ✅ Automatic filtering and deduplication
- ✅ Built-in metrics and observability

### Quick Start

1. Implement the `SourceAdapter` interface:
   ```go
   type MyToolAdapter struct { ... }
   
   func (a *MyToolAdapter) Name() string { return "mytool" }
   func (a *MyToolAdapter) Run(ctx context.Context, out chan<- *Event) error { ... }
   func (a *MyToolAdapter) Stop() { ... }
   ```

2. Normalize events to the standard `Event` format
3. Register in factory and wire in main

See examples in `examples/adapters/` directory.

---

## Adding a New Watcher (Direct Implementation)

For reference, the direct implementation approach (still supported):

### Step 1: Choose the Right Processor Type

**If your tool emits CRDs → Use Informers**
```go
// Add to main.go informer setup
informer := informerFactory.ForResource(myToolGVR).Informer()
informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
    AddFunc: func(obj interface{}) {
        eventProcessor.ProcessMyTool(ctx, obj.(*unstructured.Unstructured))
    },
})
```

**If your tool can send webhooks → Use WebhookProcessor**
```go
// Add webhook handler
http.HandleFunc("/mytool/webhook", ...)
// Process in main loop
case event := <-myToolChan:
    webhookProcessor.ProcessMyTool(ctx, event)
```

**If your tool writes ConfigMaps → Use periodic polling**
```go
// Add to configMapTicker handler
configMaps, err := clientSet.CoreV1().ConfigMaps(namespace).List(...)
```

### Step 2: Implement Processor Method

**Important:** All processors use the **centralized ObservationCreator** which handles:
- **Filtering** (ConfigMap-based, per-source rules)
- **Normalization** (severity to uppercase)
- **Deduplication** (sliding window, LRU + TTL)
- **CRD Creation** (Observation CRD)
- **Metrics** (Prometheus counters)
- **Logging** (structured logs)

**For EventProcessor (CRD sources):**
```go
func (ep *EventProcessor) ProcessMyTool(ctx context.Context, report *unstructured.Unstructured) {
    // 1. Extract data from report
    // 2. Create Observation structure
    observation := &unstructured.Unstructured{
        Object: map[string]interface{}{
            "apiVersion": "zen.kube-zen.io/v1",
            "kind":       "Observation",
            "metadata": map[string]interface{}{
                "generateName": "mytool-",
                "namespace":    report.GetNamespace(),
            },
            "spec": map[string]interface{}{
                "source":     "mytool",
                "category":   "security",
                "severity":   "HIGH",
                "eventType":  "mytool-event",
                "detectedAt": time.Now().Format(time.RFC3339),
                // ... resource and details
            },
        },
    }
    
    // 3. Use centralized observation creator
    // Flow: filter() → normalize() → dedup() → create CRD + update metrics + log
    err := ep.observationCreator.CreateObservation(ctx, observation)
    if err != nil {
        log.Printf("  ⚠️  Failed to create Observation: %v", err)
    }
}
```

**For WebhookProcessor (webhook sources):**
```go
func (wp *WebhookProcessor) ProcessMyTool(ctx context.Context, event map[string]interface{}) error {
    // 1. Extract data from webhook payload
    // 2. Create Observation structure
    observation := &unstructured.Unstructured{
        Object: map[string]interface{}{
            "apiVersion": "zen.kube-zen.io/v1",
            "kind":       "Observation",
            "spec": map[string]interface{}{
                "source":     "mytool",
                "category":   "security",
                "severity":   "HIGH",
                "eventType":  "mytool-event",
                "detectedAt": time.Now().Format(time.RFC3339),
                // ... resource and details
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
```

**Key Points:**
- **No manual deduplication** - Handled by `ObservationCreator`
- **No manual metrics** - Handled by `ObservationCreator`
- **No manual filtering** - Handled by `ObservationCreator` (ConfigMap-based)
- **All sources use the same flow** - Consistent behavior across all processors

### Step 3: Understanding the Centralized Flow

The `ObservationCreator.CreateObservation()` method implements the complete pipeline:

```go
func (oc *ObservationCreator) CreateObservation(ctx context.Context, observation *unstructured.Unstructured) error {
    // STEP 1: FILTER - Source-level filtering (ConfigMap-based)
    if oc.filter != nil && !oc.filter.Allow(observation) {
        return nil // Filtered out - no CRD, no metrics, no logs
    }
    
    // STEP 2: NORMALIZE - Severity normalization
    // (happens inline during extraction)
    
    // STEP 3: DEDUP - Sliding window deduplication
    dedupKey := oc.extractDedupKey(observation)
    if !oc.deduper.ShouldCreate(dedupKey) {
        return nil // Duplicate - skip
    }
    
    // STEP 4: CREATE - Observation CRD creation
    _, err := oc.dynClient.Resource(oc.eventGVR).Namespace(namespace).Create(ctx, observation, metav1.CreateOptions{})
    
    // STEP 5: METRICS - Increment Prometheus counters
    oc.eventsTotal.WithLabelValues(source, category, severity).Inc()
    
    // STEP 6: LOG - Structured logging
    log.Printf("  ✅ Created Observation: %s/%s/%s", source, category, severity)
    
    return nil
}
```

### Step 4: Create Observation Structure

Follow the standard event structure:

```go
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
                "kind":      "Pod",
                "name":      "example-pod",
                "namespace": "default",
            },
            "details": map[string]interface{}{
                // Tool-specific details
            },
        },
    },
}
```

Then call the centralized creator:
```go
err := ep.observationCreator.CreateObservation(ctx, observation)
```

**That's it!** Filtering, deduplication, metrics, and logging are all handled automatically.
        "apiVersion": "zen.kube-zen.io/v1",
        "kind":       "Observation",
        "metadata": map[string]interface{}{
            "generateName": "mytool-",
            "namespace":    namespace,
            "labels": map[string]interface{}{
                "source":   "mytool",
                "category": "security", // or "compliance"
                "severity": "HIGH",     // HIGH, MEDIUM, LOW
            },
        },
        "spec": map[string]interface{}{
            "source":     "mytool",
            "category":   "security",
            "severity":   "HIGH",
            "eventType":  "my-event-type",
            "detectedAt": time.Now().Format(time.RFC3339),
            "resource": map[string]interface{}{
                "kind":      "Pod",
                "name":      resourceName,
                "namespace": namespace,
            },
            "details": map[string]interface{}{
                // Tool-specific details
            },
        },
    },
}
```

### Step 5: Update Metrics

Integrate Prometheus metrics:

```go
if ep.eventsTotal != nil {
    ep.eventsTotal.WithLabelValues("mytool", "security", "HIGH").Inc()
}
```

## Code Quality Standards

### Best Practices

1. **Use Informers for CRDs**: Always prefer informers over polling
2. **Thread Safety**: Protect shared state with mutexes
3. **Error Handling**: Log errors but don't crash on individual failures
4. **Modularity**: Keep processors independent and testable
5. **Documentation**: Add comments explaining tool-specific logic

### Testing

- Test deduplication logic
- Test event creation with various inputs
- Test error handling
- Verify Prometheus metrics

### Apache 2.0 License

Zen Watcher is licensed under Apache 2.0. All contributions must:
- Be compatible with Apache 2.0
- Include appropriate license headers
- Follow the project's coding standards

---

## Vendor Neutrality

**Upstream zen-watcher MUST remain usable without kube-zen.**

zen-watcher is designed as a vendor-neutral, generic Kubernetes Observation operator. It must work with any tool, integration, or ecosystem.

### Vendor Neutrality Rules

1. **CRDs, Required Fields, and Contracts**:
   - MUST NOT hard-code product/brand names (e.g., `zen-hook`, `kube-zen`, `zen-agent`) in required fields
   - Field names must be generic (e.g., `source`, `category`, `severity` - not `zenSource`, `kubeZenCategory`)
   - CRD group names may contain branding (e.g., `zen.kube-zen.io`) but this is treated as managed tech debt with future migration plans

2. **kube-zen/zen-hook References**:
   - Belong in example sections (e.g., `examples/observations/08-webhook-originated.yaml`)
   - Belong in historical/archive docs (if applicable)
   - Belong in optional annotations (not required fields)
   - MUST be clearly marked as "one example implementation" or "kube-zen ecosystem example"

3. **Documentation**:
   - Public API docs must describe zen-watcher as "a generic Kubernetes Observation operator"
   - kube-zen components (zen-hook, zen-agent, zen-alpha) must be framed as example producers/consumers, not required dependencies
   - Integration docs must use generic language (e.g., "webhook gateway" not "zen-hook") as the primary concept

4. **Examples**:
   - Must include at least one purely generic example per category (e.g., `08-webhook-gateway.yaml` for webhooks)
   - kube-zen-specific examples (e.g., `08-webhook-originated.yaml` for zen-hook) must be explicitly marked as ecosystem examples


---

## Quality Bar & API Stability

**Zen Watcher is treated as a future KEP (Kubernetes Enhancement Proposal) candidate.** This means APIs and behaviors must be designed for long-term stability and external users.

### KEP-Level Quality Requirements

**This codebase targets community-grade, KEP-level standards.** Unlike SaaS or internal control-plane components, zen-watcher must:

1. **API Stability**: Changes to CRDs, metrics, or observable behavior must:
   - Be backward-compatible when possible, or
   - Be clearly versioned with deprecation paths (e.g., v1alpha1 → v1beta1 → v1)
   - Include migration guides for breaking changes
   - **Observation API Contract**: See `docs/OBSERVATION_API_PUBLIC_GUIDE.md` for the stable, external-facing API contract

2. **Documentation**: Tests and docs are not optional for new features:
   - **Unit tests** required for all new behavior
   - **Documentation updates** required for any new CRD fields, metrics, or dashboards
   - **Examples** required for new features (see `examples/` directory)

3. **Observability**: All features must be observable:
   - **Metrics**: New behavior must expose Prometheus metrics
   - **Logging**: Structured logging for all operations
   - **Dashboards**: Dashboard updates for new metrics (if applicable)

4. **Backward Compatibility**: Breaking changes require:
   - **Deprecation period**: Minimum 2 release cycles
   - **Migration path**: Clear upgrade instructions
   - **Versioning**: Proper API versioning (v1alpha1, v1beta1, v1)

### Quality Bar Positioning

**zen-watcher = community-grade, KEP-level target**

- **zen-watcher**: Highest quality bar - designed for long-term stability, external users, potential KEP submission
- **SaaS components**: Can tolerate more tech debt as long as it's tracked (internal use, faster iteration)
- **zen-hook (future)**: Should target a bar closer to zen-watcher than SaaS (installed in user clusters, needs stability)

### Design & Evolution Standards

**Informer Architecture**: See `docs/INFORMERS_CONVERGENCE_NOTES.md` for how we carefully design and evolve informers:
- Multi-phase convergence with zen-agent (design-level alignment)
- Backward compatibility preserved during refactoring
- Test coverage for all abstractions

## Release Preparation

**Before tagging a release or opening a release PR, run:**

```bash
# Run all checks (lint, test, security, build)
make all

# Validate Helm chart
make helm-validate

# Build and validate Docker image
make docker-build

# Optional: Run fuzz tests
make fuzz
```

**For CI/CD pipelines**, use:

```bash
# Full CI pipeline (all checks + Docker + Helm)
make ci
```

**See [ZW_1_0_0_ALPHA_RELEASE_CHECKLIST.md](docs/ZW_1_0_0_ALPHA_RELEASE_CHECKLIST.md) for complete release checklist.**

**Performance & Observability**: See `docs/STRESS_TEST_RESULTS.md` and dashboard docs as examples:
- Performance characteristics documented with real numbers
- Observability dashboards validated against actual metrics
- Quality bar demonstrated through comprehensive documentation

### Implementation Guidelines

When implementing new features:

1. **Design First**: Consider API stability, backward compatibility, and versioning
2. **Test Coverage**: Aim for 80%+ unit test coverage for new code
3. **Documentation**: Update relevant docs (README, architecture, examples)
4. **Metrics**: Add Prometheus metrics for new behavior
5. **Examples**: Provide working examples in `examples/` directory

When refactoring:

1. **Preserve Contracts**: Maintain external APIs (CRDs, metrics, observable behavior)
2. **Incremental**: Small, scoped changes with tests at each step
3. **Documentation**: Update architecture docs if patterns change
4. **Validation**: Ensure existing examples and tests still pass

### Review Criteria

All PRs are reviewed against KEP-level standards:

- ✅ **Backward Compatibility**: No breaking changes without deprecation path
- ✅ **Test Coverage**: New features include unit tests
- ✅ **Documentation**: Docs updated for all user-facing changes
- ✅ **Metrics**: New behavior exposes metrics
- ✅ **Examples**: New features include working examples
- ✅ **API Design**: Changes follow Kubernetes API conventions

**Questions about quality bar?** Open an issue for discussion.

### Historical Reference & Archive Documentation

**Archive documents are non-canonical** - they provide historical context but should not be used as current guidance.


**Use archive docs for**: Historical context, rationale, and inspiration


All archive documents include banners explaining their non-canonical status.

## Development Environment Setup

### Prerequisites

- **Go 1.25+**
- **Kubernetes cluster** (k3d, kind, or minikube) for integration tests
- **kubectl** configured to access your cluster
- **helm** (optional, for deployment)

### Setup Steps

1. **Clone the repository**:
   ```bash
   git clone https://github.com/kube-zen/zen-watcher
   cd zen-watcher
   ```

2. **Install dependencies**:
   ```bash
   go mod download
   ```

3. **Build the binary**:
   ```bash
   go build -o zen-watcher ./cmd/zen-watcher
   ```

4. **Install CRDs** (if testing locally):
   ```bash
   kubectl apply -f deployments/crds/
   ```

## Schema Documentation Generation

After modifying CRDs or SDK types, regenerate the schema documentation:

```bash
go run ./cmd/schema-doc-gen
```

This generates:
- `docs/generated/INGESTER_SCHEMA_REFERENCE.md`
- `docs/generated/OBSERVATIONS_SCHEMA_REFERENCE.md`

**Note**: These files are auto-generated. Do not edit them manually.

## How to Run Tests Locally

### End-to-End Testing with zen-demo

For local end-to-end testing, use the zen-demo k3d cluster:

```bash
# Create zen-demo cluster
make zen-demo-up

# Build and load watcher image
make zen-demo-build-push

# Deploy watcher to zen-demo
make zen-demo-deploy-watcher

# Run e2e validation tests
make zen-demo-validate

# Cleanup
make zen-demo-down
```

See `make zen-demo-up`, `make zen-demo-validate`, and `make zen-demo-down` for details.

### Prerequisites
- Go 1.25+ (tested on 1.25)
- Make (optional, but recommended)

### Running Tests

```bash
# Run all tests
make test

# Or directly with go
go test -v -race ./...

# Run tests with coverage
go test -v -race -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run specific package tests
go test -v ./pkg/watcher/...

# Run specific test
go test -v -run TestKyvernoWatcher ./pkg/watcher
```

### Running Tests in CI Mode

```bash
# Run tests as CI would (with race detector)
go test -v -race -coverprofile=coverage.out ./...
```

## Branching and Commit Message Guidelines

### Branch Naming

- `feature/` - New features (e.g., `feature/add-wiz-watcher`)
- `fix/` - Bug fixes (e.g., `fix/kyverno-type-assertion`)
- `docs/` - Documentation updates (e.g., `docs/update-contributing`)
- `refactor/` - Code refactoring (e.g., `refactor/webhook-processor`)
- `test/` - Test additions/updates (e.g., `test/add-unit-tests`)

### Commit Message Format

We follow the [Conventional Commits](https://www.conventionalcommits.org/) specification:

```
<type>(<scope>): <subject>

<body>

<footer>
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, etc.)
- `refactor`: Code refactoring
- `test`: Test additions or changes
- `chore`: Maintenance tasks

**Examples:**
```
feat(watcher): add Wiz security scanner support

Adds a new watcher for Wiz security scanner that processes
vulnerability reports and creates Observation CRDs.

Fixes #123
```

```
fix(kyverno): safe type assertion in watch loops

Replaces unsafe type assertions with safe checks to prevent
panics when unexpected object types are encountered.

Closes #456
```

## How to Prepare a PR

### Pre-PR Checklist

Before submitting a pull request, ensure:

- [ ] **Code is formatted**: Run `go fmt ./...`
- [ ] **Code passes vet**: Run `go vet ./...`
- [ ] **Code passes staticcheck**: Run `staticcheck ./...` (if installed)
- [ ] **Tests pass**: Run `go test -v -race ./...`
- [ ] **Tests added**: New features include tests
- [ ] **Documentation updated**: README, docs, or code comments updated
- [ ] **Commit messages follow guidelines**: See above
- [ ] **No merge conflicts**: Rebase on latest main branch
- [ ] **PR description is complete**: See PR template

### PR Process

1. **Fork the repository** (if external contributor)
2. **Create a feature branch** from `main`
3. **Make your changes** following the guidelines above
4. **Run tests and linters** locally
5. **Commit your changes** with descriptive messages
6. **Push to your fork** and create a PR
7. **Fill out the PR template** completely
8. **Wait for review** from maintainers

### PR Review Criteria

PRs will be reviewed for:
- Code quality and style
- Test coverage
- Documentation completeness
- Backward compatibility
- Performance implications
- Security considerations

## How to Run Linters/Staticcheck

### Using Make

```bash
# Run all linters
make lint

# Individual linters
make fmt      # Format code
make vet      # Run go vet
make staticcheck  # Run staticcheck
```

### Manual Commands

```bash
# Format code
go fmt ./...

# Run go vet
go vet ./...

# Install staticcheck
go install honnef.co/go/tools/cmd/staticcheck@latest

# Run staticcheck
staticcheck ./...

# Install golangci-lint (alternative)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run golangci-lint
golangci-lint run
```

### CI Integration

All linters run automatically in CI on every PR. Ensure your code passes before requesting review.

## Release Notes

When preparing a release:

1. **Use the template**: Follow `docs/RELEASE_NOTES_TEMPLATE.md` for structure
2. **Document CRD/API changes**: All CRD/API changes must link to:
   - `docs/OBSERVATION_VERSIONING_AND_RELEASE_PLAN.md` (versioning plan)
3. **Create release notes**: Add to `docs/releases/` directory
4. **Update version**: Update version in code and documentation

**See**: `docs/releases/` for version history and `docs/RELEASE_NOTES_TEMPLATE.md` for the standard structure.

## Questions?

Open an issue or check existing documentation in `docs/` for more details.

For maintainer contact, see [MAINTAINERS](MAINTAINERS).
