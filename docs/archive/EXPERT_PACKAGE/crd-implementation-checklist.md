---
⚠️ HISTORICAL DOCUMENT - EXPERT PACKAGE ARCHIVE ⚠️

This document is from an external "Expert Package" analysis of zen-watcher/ingester.
It reflects the state of zen-watcher at a specific point in time and may be partially obsolete.

CANONICAL SOURCES (use these for current direction):
- docs/PM_AI_ROADMAP.md - Current roadmap and priorities
- CONTRIBUTING.md - Current quality bar and standards
- docs/INFORMERS_CONVERGENCE_NOTES.md - Current informer architecture
- docs/STRESS_TEST_RESULTS.md - Current performance baselines

This archive document is provided for historical context, rationale, and inspiration only.
Do NOT use this as a replacement for current documentation.

---

# CRD Implementation Checklist

## Quick Start Implementation Guide

### Phase 1: Core Infrastructure (Start Here)

#### Step 1: Create Source CRD Definition
- [ ] Create `deploy/crd/source_crd.yaml` with the CRD definition
- [ ] Apply CRD to cluster: `kubectl apply -f deploy/crd/source_crd.yaml`
- [ ] Verify CRD creation: `kubectl get crd sources.zenwatcher.kube-zen.io`

#### Step 2: Implement Generic Adapter Interface
- [ ] Create `pkg/adapter/generic_adapter.go` with SourceAdapter struct
- [ ] Implement SourceHandler interface
- [ ] Create handler registry map
- [ ] Add Start(), Stop(), GetObservations(), GetHealth() methods

#### Step 3: Build Webhook Handler First (Most Impressive Feature)
- [ ] Create `pkg/adapter/webhook_handler.go`
- [ ] Implement nginx configuration generation
- [ ] Add authentication support (bearer token, API key, basic auth)
- [ ] Implement ConfigMap-based nginx deployment
- [ ] Test webhook endpoint creation

#### Step 4: Migrate Existing Adapters to YAML
- [ ] Create Trivy YAML config example
- [ ] Create Falco YAML config example  
- [ ] Create Kyverno YAML config example
- [ ] Test YAML-based configuration vs. existing code

### Phase 2: Testing & Validation

#### Step 5: Create Example Configurations
- [ ] Create `examples/sources/` directory
- [ ] Add 10 example YAML configurations (provided in implementation files)
- [ ] Test each configuration type
- [ ] Validate CRD schema compliance

#### Step 6: Implement Admission Webhooks (Optional but Recommended)
- [ ] Create validation webhook service
- [ ] Add OpenAPI schema validation
- [ ] Implement default value injection
- [ ] Test webhook functionality

### Phase 3: Advanced Features

#### Step 7: Complete Handler Implementation
- [ ] Implement TrivyHandler with real Trivy integration
- [ ] Implement FalcoHandler with real Falco integration
- [ ] Implement KyvernoHandler with real Kyverno integration
- [ ] Implement LogHandler for log aggregation
- [ ] Implement ConfigMapHandler for ConfigMap watching

#### Step 8: Add Advanced Features
- [ ] Scheduling support (cron, interval)
- [ ] Health check configuration
- [ ] Filter expressions
- [ ] Output transformation rules
- [ ] Monitoring integration

### Phase 4: Production Deployment

#### Step 9: Performance Testing
- [ ] Benchmark generic adapter vs. specific adapters
- [ ] Test webhook endpoint creation speed
- [ ] Validate nginx configuration reliability
- [ ] Test with high-frequency webhook events

#### Step 10: Documentation & Examples
- [ ] Create comprehensive README with examples
- [ ] Add troubleshooting guide
- [ ] Create video tutorial for dynamic webhooks
- [ ] Update main documentation

## Quick Test Commands

### Test CRD Creation
```bash
kubectl apply -f deploy/crd/source_crd.yaml
kubectl get crd sources.zenwatcher.kube-zen.io
kubectl describe crd sources.zenwatcher.kube-zen.io
```

### Test Basic Source Creation
```bash
kubectl apply -f examples/sources/trivy-basic.yaml
kubectl get sources
kubectl describe source trivy-basic
```

### Test Webhook Creation
```bash
kubectl apply -f examples/sources/webhook-security.yaml
kubectl get configmaps | grep webhook
kubectl describe configmap zen-watcher-webhook-security-events
```

### Test Nginx Configuration
```bash
kubectl exec -n ingress-nginx deployment/ingress-nginx-controller -- nginx -t
kubectl logs -n ingress-nginx deployment/ingress-nginx-controller | grep webhook
```

## Implementation Files Reference

### Core Files to Create:
1. **`deploy/crd/source_crd.yaml`** - CRD definition (provided)
2. **`pkg/adapter/generic_adapter.go`** - Generic adapter implementation (provided)
3. **`pkg/adapter/webhook_handler.go`** - Webhook handler (part of generic adapter)
4. **`examples/sources/`** - Example configurations (provided)

### Modified Files:
1. **`main.go`** - Integrate generic adapter initialization
2. **`pkg/types/source.go`** - Source type definitions
3. **`go.mod`** - Add required dependencies

## Success Criteria

### Phase 1 Success:
- [ ] CRD created and functional
- [ ] Generic adapter compiles and runs
- [ ] Webhook handler creates nginx configuration
- [ ] Basic YAML source creation works

### Phase 2 Success:
- [ ] All example configurations validate
- [ ] Admission webhooks validate YAML
- [ ] Error messages are clear and helpful
- [ ] Documentation is comprehensive

### Phase 3 Success:
- [ ] All handlers implement real functionality
- [ ] Advanced features work as expected
- [ ] Performance meets or exceeds current implementation
- [ ] Security is comprehensive

### Phase 4 Success:
- [ ] Production deployment is stable
- [ ] Performance testing shows improvement
- [ ] Documentation is complete
- [ ] Community adoption is high

## Risk Mitigation

### Low Risk:
- CRD definition (tested pattern)
- Generic adapter interface (standard pattern)
- Webhook handler (proven nginx approach)

### Medium Risk:
- YAML configuration errors (mitigate with validation)
- Performance impact (mitigate with benchmarking)
- Complex configurations (mitigate with examples)

### High Risk:
- None identified - this is a proven architectural pattern

## Quick Start (30 minutes)

If you want to see the magic immediately:

1. **Apply CRD** (5 minutes):
   ```bash
   kubectl apply -f deploy/crd/source_crd.yaml
   ```

2. **Create basic webhook source** (5 minutes):
   ```bash
   kubectl apply -f examples/sources/webhook-security.yaml
   ```

3. **Check nginx configuration** (5 minutes):
   ```bash
   kubectl get configmaps | grep webhook
   ```

4. **Test webhook endpoint** (5 minutes):
   ```bash
   curl -X POST https://zen-watcher.kube-zen.io/api/security-events \
     -H "Authorization: Bearer test-token" \
     -H "Content-Type: application/json" \
     -d '{"event":"test","severity":"high"}'
   ```

5. **Verify observation creation** (10 minutes):
   ```bash
   kubectl get observations
   kubectl describe observation <observation-name>
   ```

**Result:** You'll have a production-ready webhook endpoint with SSL, authentication, and rate limiting - all generated from a simple YAML file. This demonstrates the "magic" of the new architecture.

## Next Steps

1. **Start with the checklist above**
2. **Focus on the webhook handler first** (most impressive feature)
3. **Create comprehensive examples** (provided)
4. **Test thoroughly** before production deployment
5. **Document everything** for users

**This CRD architecture will transform Zen Watcher into a truly user-friendly, self-service security platform.**
