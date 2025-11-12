# Zen Watcher Developer Guide

**Version:** 1.0.19+  
**Go Version:** 1.22+  
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

---

## Getting Started

### Prerequisites

- **Go 1.22+** installed
- **Docker** for building images
- **kubectl** for Kubernetes access
- **k3d** or similar for local testing
- Basic understanding of Kubernetes CRDs and controllers

### Development Setup

```bash
# Clone the repository
git clone https://github.com/kube-zen/zen-watcher.git
cd zen-watcher

# Install dependencies
go mod download

# Build the binary
go build -o zen-watcher ./cmd/zen-watcher

# Run locally (requires kubeconfig)
export KUBECONFIG=~/.kube/config
./zen-watcher
```

### Development Environment

```bash
# Create local k3d cluster for testing
k3d cluster create zen-dev --agents 0 --api-port 6551

# Install test tools
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
│   └── main.go                    # Main entry point (~1200 lines)
│
├── build/
│   └── Dockerfile                 # Multi-stage Docker build
│
├── deployments/
│   ├── crds/
│   │   └── zenagent_event_crd.yaml
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
├── ARCHITECTURE.md                # Architecture deep dive
├── CONTRIBUTING.md                # Contribution guidelines
├── DEVELOPER_GUIDE.md             # This file
├── LICENSE                        # Apache 2.0
└── go.mod                         # Go dependencies
```

---

## Code Architecture

### Main Entry Point (`cmd/zen-watcher/main.go`)

The entire application is contained in a single `main.go` file (~1200 lines) for simplicity and ease of understanding.

#### Code Structure

```go
// Line 1-30: Package declaration and imports
package main
import (...)

// Line 31-85: Main function setup
func main() {
    // 1. Initialize logging
    // 2. Create Kubernetes clients
    // 3. Set up signal handling
    // 4. Configure tool namespaces
    // 5. Start HTTP server (webhooks, health)
    // 6. Start main watch loop
}

// Line 86-200: HTTP Server Setup
// - /health endpoint
// - /ready endpoint
// - /falco/webhook endpoint
// - /audit/webhook endpoint

// Line 201-1200: Main Watch Loop
// - Auto-detection (line 201-300)
// - Trivy watcher (line 301-450)
// - Kyverno watcher (line 451-600)
// - Falco watcher (line 601-700)
// - Kube-bench watcher (line 701-850)
// - Checkov watcher (line 851-1000)
// - Audit watcher (line 1001-1150)
// - Metrics and cleanup (line 1151-1200)
```

#### Key Data Structures

**ToolState**: Tracks detection status
```go
type ToolState struct {
    Installed bool
    Namespace string
    LastCheck time.Time
}
```

**Event Channels**: For webhook-based tools
```go
falcoAlertsChan := make(chan map[string]interface{}, 100)
auditEventsChan := make(chan map[string]interface{}, 200)
```

**Deduplication Maps**: Prevent duplicate events
```go
existingKeys := make(map[string]bool)
```

### Watch Loop Pattern

Each watcher follows this pattern:

```go
// 1. Auto-detect tool
if toolStates["mytool"].Installed {
    
    // 2. Fetch existing events for deduplication
    existingEvents, err := dynClient.Resource(eventGVR).List(...)
    existingKeys := make(map[string]bool)
    for _, ev := range existingEvents.Items {
        key := generateDedupKey(ev)
        existingKeys[key] = true
    }
    
    // 3. Fetch new events from tool
    reports, err := fetchToolReports(...)
    
    // 4. Process each report
    for _, report := range reports {
        // Generate dedup key
        key := generateDedupKey(report)
        
        // Skip if already exists
        if existingKeys[key] { continue }
        
        // Create ZenAgentEvent
        event := createZenAgentEvent(report)
        dynClient.Resource(eventGVR).Create(ctx, event, ...)
        
        // Update dedup map
        existingKeys[key] = true
    }
}
```

---

## Adding a New Watcher

### Step 1: Add Tool to Auto-Detection

```go
// Add namespace configuration
mytoolNs := os.Getenv("MYTOOL_NAMESPACE")
if mytoolNs == "" {
    mytoolNs = "mytool-system"
}

// Add to tool states
toolStates := map[string]*ToolState{
    // ... existing tools
    "mytool": {Namespace: mytoolNs},
}
```

### Step 2: Implement Auto-Detection Logic

```go
// In auto-detection phase
log.Println("  → Listing pods in namespace 'mytool-system'...")
mytoolPods, err := clientSet.CoreV1().Pods(mytoolNs).List(ctx, metav1.ListOptions{})
if err != nil {
    log.Printf("  ⚠️  Error listing pods in namespace '%s': %v", mytoolNs, err)
    toolStates["mytool"].Installed = false
} else if len(mytoolPods.Items) > 0 {
    log.Printf("  ✅ MyTool detected in namespace '%s' (%d pods)", mytoolNs, len(mytoolPods.Items))
    toolStates["mytool"].Installed = true
    toolStates["mytool"].LastCheck = time.Now()
}
```

### Step 3: Implement Event Watcher

```go
// Check if tool is installed
if toolStates["mytool"].Installed {
    log.Println("  → Checking MyTool reports...")
    
    // Define GVR (Group/Version/Resource) for tool CRD
    mytoolGVR := schema.GroupVersionResource{
        Group:    "mytool.io",
        Version:  "v1",
        Resource: "mytoolreports",
    }
    
    // Fetch reports
    reports, err := dynClient.Resource(mytoolGVR).Namespace("").List(ctx, metav1.ListOptions{})
    if err != nil {
        log.Printf("  ⚠️  Cannot access MyTool reports: %v", err)
    } else {
        log.Printf("  ✓ Found %d MyTool reports", len(reports.Items))
        
        // Get existing ZenAgentEvents for deduplication
        existingEvents, err := dynClient.Resource(eventGVR).Namespace("").List(ctx, metav1.ListOptions{
            LabelSelector: "source=mytool,category=security",
        })
        existingKeys := make(map[string]bool)
        if err != nil {
            log.Printf("  ⚠️  Cannot load existing events for dedup: %v", err)
        } else {
            for _, ev := range existingEvents.Items {
                spec, _ := ev.Object["spec"].(map[string]interface{})
                if spec != nil {
                    details, _ := spec["details"].(map[string]interface{})
                    if details != nil {
                        // Create dedup key
                        eventID := fmt.Sprintf("%v", details["eventID"])
                        resourceName := fmt.Sprintf("%v", details["resourceName"])
                        key := fmt.Sprintf("%s/%s", eventID, resourceName)
                        existingKeys[key] = true
                    }
                }
            }
        }
        log.Printf("  📋 Dedup: %d existing events, checking for new issues...", len(existingKeys))
        
        // Process each report
        mytoolCount := 0
        for _, report := range reports.Items {
            // Extract fields from report
            spec, _ := report.Object["spec"].(map[string]interface{})
            if spec == nil { continue }
            
            eventID := fmt.Sprintf("%v", spec["eventID"])
            severity := fmt.Sprintf("%v", spec["severity"])
            description := fmt.Sprintf("%v", spec["description"])
            
            // Extract resource info
            metadata, _ := report.Object["metadata"].(map[string]interface{})
            namespace := fmt.Sprintf("%v", metadata["namespace"])
            name := fmt.Sprintf("%v", metadata["name"])
            
            // Create dedup key
            dedupKey := fmt.Sprintf("%s/%s", eventID, name)
            if existingKeys[dedupKey] {
                continue
            }
            
            // Map severity to standard levels
            mappedSeverity := "MEDIUM"
            if severity == "critical" || severity == "high" {
                mappedSeverity = "HIGH"
            } else if severity == "low" {
                mappedSeverity = "LOW"
            }
            
            // Create ZenAgentEvent
            event := &unstructured.Unstructured{
                Object: map[string]interface{}{
                    "apiVersion": "zen.kube-zen.io/v1",
                    "kind":       "ZenAgentEvent",
                    "metadata": map[string]interface{}{
                        "generateName": "mytool-",
                        "namespace":    namespace,
                        "labels": map[string]interface{}{
                            "source":   "mytool",
                            "category": "security",
                            "severity": mappedSeverity,
                        },
                    },
                    "spec": map[string]interface{}{
                        "source":     "mytool",
                        "category":   "security",
                        "severity":   mappedSeverity,
                        "eventType":  "mytool-event",
                        "detectedAt": time.Now().Format(time.RFC3339),
                        "resource": map[string]interface{}{
                            "kind":      "Pod",
                            "name":      name,
                            "namespace": namespace,
                        },
                        "details": map[string]interface{}{
                            "eventID":     eventID,
                            "description": description,
                            "severity":    severity,
                        },
                    },
                },
            }
            
            _, err := dynClient.Resource(eventGVR).Namespace(namespace).Create(ctx, event, metav1.CreateOptions{})
            if err != nil {
                log.Printf("  ⚠️  Failed to create MyTool ZenAgentEvent: %v", err)
            } else {
                mytoolCount++
                existingKeys[dedupKey] = true
                lastLoopCount++
            }
        }
        
        if mytoolCount > 0 {
            log.Printf("  ✅ Created %d NEW ZenAgentEvents from MyTool", mytoolCount)
        }
    }
}
```

### Step 4: Add RBAC Permissions

Update `deployments/rbac/clusterrole.yaml`:

```yaml
- apiGroups: ["mytool.io"]
  resources: ["mytoolreports"]
  verbs: ["get", "list", "watch"]
```

### Step 5: Add to Documentation

Update `README.md` and `ARCHITECTURE.md` to include the new tool.

---

## Testing

### Unit Testing (Future)

Currently, zen-watcher doesn't have unit tests due to its integration nature. Future testing strategy:

```go
// Example: Test deduplication logic
func TestDeduplication(t *testing.T) {
    existingKeys := make(map[string]bool)
    existingKeys["CVE-2024-001/pod-1"] = true
    
    key := "CVE-2024-001/pod-1"
    if existingKeys[key] {
        // Should skip this event
        t.Log("Correctly deduplicated")
    }
}
```

### Integration Testing

```bash
# 1. Deploy zen-watcher to test cluster
kubectl apply -f deployments/crds/
kubectl apply -f deployments/base/

# 2. Install test security tool
kubectl apply -f test-manifests/trivy-operator.yaml

# 3. Wait for events
sleep 60

# 4. Verify events created
kubectl get zenagentevents -A -l source=trivy

# 5. Check logs
kubectl logs -n zen-system deployment/zen-watcher | grep "Trivy"
```

### Manual Testing Checklist

**Before Release:**
- [ ] All 6 watchers detect their tools correctly
- [ ] Events are created for each tool
- [ ] Deduplication prevents duplicates
- [ ] Categories are correct (security/compliance)
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

### Docker Build

```bash
# Build image
docker build \
    --no-cache \
    --pull \
    -t kubezen/zen-watcher:1.0.19 \
    -f build/Dockerfile \
    .

# Push to registry
docker push kubezen/zen-watcher:1.0.19
```

**Dockerfile optimization:**
- Multi-stage build (builder + distroless)
- Uses `golang:1.22-alpine` for small builder image
- Final image based on `gcr.io/distroless/static:nonroot` (~15MB)
- No shell, no package manager in final image

### Deployment

```bash
# Standard deployment
kubectl apply -f deployments/crds/
kubectl apply -f deployments/base/

# Helm deployment
helm install zen-watcher ./charts/zen-watcher \
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
   - ARCHITECTURE.md update if design changes
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

## Debugging Tips

### Enable Verbose Logging

```go
// Temporarily add debug logs
log.Printf("DEBUG: Report spec: %+v", spec)
log.Printf("DEBUG: Dedup key: %s", dedupKey)
log.Printf("DEBUG: Existing keys: %d", len(existingKeys))
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
```

### Kubernetes Debugging

```bash
# Check RBAC
kubectl auth can-i get vulnerabilityreports --as=system:serviceaccount:zen-system:zen-watcher

# Check NetworkPolicy
kubectl describe networkpolicy zen-watcher -n zen-system

# Check events
kubectl get events -n zen-system --field-selector involvedObject.name=zen-watcher

# Check CRD status
kubectl get crd zenagentevents.zen.kube-zen.io -o yaml
```

---

## Common Patterns

### Pattern 1: Fetching CRD-Based Reports

```go
// Define GVR
gvr := schema.GroupVersionResource{
    Group:    "aquasecurity.github.io",
    Version:  "v1alpha1",
    Resource: "vulnerabilityreports",
}

// List all reports
reports, err := dynClient.Resource(gvr).Namespace("").List(ctx, metav1.ListOptions{})
```

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

### Pattern 3: Creating ZenAgentEvents

```go
event := &unstructured.Unstructured{
    Object: map[string]interface{}{
        "apiVersion": "zen.kube-zen.io/v1",
        "kind":       "ZenAgentEvent",
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

### Version Bumping

1. Update version in `main.go`:
   ```go
   log.Println("🚀 zen-watcher v1.0.20 (Go 1.22, Apache 2.0)")
   ```

2. Update `CHANGELOG.md` with changes

3. Tag release:
   ```bash
   git tag v1.0.20
   git push origin v1.0.20
   ```

### Build and Push

```bash
# Build
docker build --no-cache --pull -t kubezen/zen-watcher:1.0.20 -f build/Dockerfile .

# Push
docker push kubezen/zen-watcher:1.0.20

# Tag as latest (optional)
docker tag kubezen/zen-watcher:1.0.20 kubezen/zen-watcher:latest
docker push kubezen/zen-watcher:latest
```

### Update Helm Charts

```bash
# Update zen-agent/values.yaml
sed -i 's/tag: "1.0.19"/tag: "1.0.20"/' charts/zen-agent/values.yaml

# Commit
git add charts/zen-agent/values.yaml
git commit -m "zen-watcher v1.0.20: <summary>"
git push
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

## Resources

- **Main README**: [README.md](README.md)
- **Architecture Details**: [ARCHITECTURE.md](ARCHITECTURE.md)
- **Security Docs**: [docs/SECURITY.md](docs/SECURITY.md)
- **Deployment Guide**: [docs/DEPLOYMENT_SCENARIOS.md](docs/DEPLOYMENT_SCENARIOS.md)
- **Helm Charts**: [helm-charts repository](https://github.com/kube-zen/helm-charts)

---

**Questions?** Open an issue on GitHub or check the [documentation index](DOCUMENTATION_INDEX.md).

