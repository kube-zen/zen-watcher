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

# Enhanced Auto-Optimization for HA - Implementation Instructions

## Overview
Implement HA-specific auto-optimization features that enhance Zen Watcher's scalability and efficiency while maintaining single-replica optimization capabilities.

## CRITICAL REQUIREMENTS

### 1. Maintain Single-Replica Compatibility
- All existing per-source optimization features must continue working
- HA features should be additive, not breaking changes
- Default behavior should remain single-replica when HA is not configured

### 2. HA-Specific Configuration Structure
Create new configuration sections in Helm values.yaml and ConfigMap configs:

```yaml
# HA-specific optimizations (complements per-source optimization)
ha_optimization:
  # Dynamic replica scaling based on load
  auto_scaling:
    enabled: true
    min_replicas: 2
    max_replicas: 10
    target_cpu: 70%
    scale_up_delay: "2m"
    scale_down_delay: "10m"
    
  # Dynamic dedup window based on traffic
  dedup_optimization:
    enabled: true
    low_traffic_window: "300s"    # < 50 events/sec
    high_traffic_window: "60s"    # > 500 events/sec
    adaptive_windows: true
    
  # Dynamic cache sizing
  cache_optimization:
    enabled: true
    low_traffic_size: 5000       # events
    high_traffic_size: 50000     # events
    memory_based_sizing: true
    
  # Load balancing strategy
  load_balancing:
    strategy: "least_loaded"     # "round_robin", "least_loaded", "consistent_hash"
    health_check_interval: "30s"
    rebalance_threshold: 2.0     # 2x load difference triggers rebalance
```

## IMPLEMENTATION PHASES

### Phase 1: HA Infrastructure Foundation (Priority 1)

#### 1.1 Environment Detection
- Add environment variable detection for HA mode
- Create HA configuration struct
- Implement HA-aware logging

**Files to Modify:**
- `pkg/config/defaults.go` - Add HA configuration struct
- `pkg/server/http.go` - Add health endpoints for HA coordination
- `cmd/zen-watcher/main.go` - Initialize HA features

**Implementation Details:**
```go
// pkg/config/ha_config.go
type HAConfig struct {
    AutoScaling       AutoScalingConfig       `json:"auto_scaling,omitempty"`
    DedupOptimization DedupOptimizationConfig `json:"dedup_optimization,omitempty"`
    CacheOptimization CacheOptimizationConfig `json:"cache_optimization,omitempty"`
    LoadBalancing     LoadBalancingConfig     `json:"load_balancing,omitempty"`
}

type AutoScalingConfig struct {
    Enabled         bool    `json:"enabled"`
    MinReplicas     int     `json:"min_replicas"`
    MaxReplicas     int     `json:"max_replicas"`
    TargetCPU       int     `json:"target_cpu"`  // percentage
    ScaleUpDelay    string  `json:"scale_up_delay"`
    ScaleDownDelay  string  `json:"scale_down_delay"`
}
```

#### 1.2 HA-Ready HTTP Server
- Add HA-aware endpoints for health checks
- Implement replica health monitoring
- Add metrics endpoint for auto-scaling decisions

**New Endpoints to Add:**
- `/ha/health` - HA health check
- `/ha/metrics` - HA-specific metrics
- `/ha/status` - Current HA status and load

#### 1.3 Configuration Loading
- Load HA config from environment variables and ConfigMaps
- Merge HA config with existing single-replica config
- Implement config hot-reloading for HA settings

### Phase 2: Dynamic Dedup Window Optimization (Priority 2)

#### 2.1 Traffic-Based Dedup Window
**Files to Create/Modify:**
- `pkg/optimization/ha_dedup_optimization.go`
- `pkg/filter/deduper.go` - Add adaptive window logic
- `pkg/metrics/traffic_analyzer.go` - New file

**Implementation Requirements:**
1. **Traffic Analysis:**
   - Monitor events per second from all sources
   - Calculate rolling averages for traffic patterns
   - Store traffic history for pattern detection

2. **Dynamic Window Adjustment:**
   - Low traffic (< 50 events/sec): 300s dedup window
   - Medium traffic (50-500 events/sec): 120s dedup window  
   - High traffic (> 500 events/sec): 60s dedup window
   - Configurable thresholds

3. **HA Integration:**
   - Coordinate window changes across replicas
   - Ensure consistent dedup behavior across instances
   - Implement gradual window transitions

**Code Structure:**
```go
type TrafficAnalyzer struct {
    eventCounter    *prometheus.Counter
    windowHistory   []TrafficSample
    currentWindow   time.Duration
    haCoordination  *HACoordinator
}

type TrafficSample struct {
    Timestamp    time.Time
    EventsPerSec float64
    WindowSize   time.Duration
}
```

#### 2.2 Per-Source Integration
- Maintain existing per-source dedup configs
- Apply HA window adjustments to all sources
- Allow per-source overrides of HA settings

### Phase 3: Auto-Scaling Implementation (Priority 3)

#### 3.1 Metrics Collection
**Files to Create:**
- `pkg/metrics/ha_metrics.go`
- `pkg/scaling/hpa_coordinator.go`

**Required Metrics:**
- CPU usage per replica
- Memory usage per replica
- Events processed per second
- Queue depth (pending events)
- Response time per event

#### 3.2 Scaling Decision Engine
**Algorithm:**
1. **Scale Up Triggers:**
   - CPU > target for 2+ minutes
   - Queue depth > threshold
   - Response time > SLA

2. **Scale Down Triggers:**
   - CPU < target for 10+ minutes
   - Low queue depth
   - High spare capacity

3. **Scaling Actions:**
   - Call Kubernetes HPA API
   - Log scaling decisions
   - Update replica health status

**Integration Points:**
- `cmd/zen-watcher/main.go` - Start scaling coordinator
- `pkg/server/http.go` - Add scaling metrics endpoint
- Helm chart values.yaml - Add HPA configuration

### Phase 4: Load Balancing Strategy (Priority 4)

#### 4.1 Load Balancing Implementation
**Files to Create:**
- `pkg/balancer/load_balancer.go`
- `pkg/balancer/strategies.go`

**Strategies to Implement:**
1. **Round Robin:** Simple rotation through replicas
2. **Least Loaded:** Route to replica with lowest current load
3. **Consistent Hash:** Hash events to specific replicas for state consistency

#### 4.2 Health Check Integration
- Monitor replica health continuously
- Remove unhealthy replicas from load balancing
- Gradually add healthy replicas back to rotation

### Phase 5: Adaptive Cache Sizing (Priority 5)

#### 5.1 Memory-Based Cache Management
**Files to Modify:**
- `pkg/gc/collector.go` - Add memory-aware garbage collection
- `pkg/dedup/deduper.go` - Implement adaptive cache sizing

**Algorithm:**
1. **Monitor Memory Usage:**
   - Track dedup cache memory consumption
   - Monitor system memory pressure
   - Calculate memory efficiency metrics

2. **Adjust Cache Size:**
   - Low memory pressure: Larger cache (50,000 events)
   - High memory pressure: Smaller cache (5,000 events)
   - Respect memory limits and quotas

3. **Cache Efficiency Metrics:**
   - Hit rate optimization
   - Memory efficiency ratio
   - Garbage collection frequency

## TECHNICAL SPECIFICATIONS

### Configuration Loading
1. **Priority Order:**
   - Environment variables (highest priority)
   - ConfigMap settings
   - Helm values.yaml (default values)

2. **Hot Reloading:**
   - Monitor ConfigMap changes for HA config
   - Graceful reconfiguration without restart
   - Validation of new config before applying

### HA Coordination
1. **Replica Communication:**
   - Use Kubernetes ConfigMap or Headless Service
   - Share scaling decisions and traffic patterns
   - Coordinate dedup window changes

2. **State Consistency:**
   - Ensure dedup behavior is consistent across replicas
   - Coordinate garbage collection schedules
   - Share optimization metrics

### Integration Points

#### With Existing Optimization
1. **Per-Source Optimization:**
   - HA optimizations apply to all sources equally
   - Per-source optimizations remain unchanged
   - HA can override per-source settings if configured

2. **Processing Pipeline:**
   - Filter → Dedup → Normalize (maintain this order)
   - HA optimizations enhance, don't replace existing logic
   - Preserve all existing optimization features

#### With Kubernetes
1. **HPA Integration:**
   - Work with native Kubernetes HPA
   - Provide custom metrics for scaling decisions
   - Respect resource limits and quotas

2. **Service Mesh Ready:**
   - Support Istio, Linkerd, or other service meshes
   - Compatible with service discovery
   - Respect network policies

## TESTING REQUIREMENTS

### Unit Tests
- HA configuration loading and validation
- Traffic analysis algorithms
- Scaling decision logic
- Load balancing strategies

### Integration Tests
- Multi-replica deployment scenarios
- Scaling up and down under load
- Failover and recovery testing
- Configuration hot-reloading

### Performance Tests
- Throughput comparison (single vs HA)
- Resource utilization optimization
- Latency impact of HA features
- Memory usage patterns

## DEPLOYMENT CONSIDERATIONS

### Helm Chart Updates
1. **values.yaml additions:**
   - HA optimization configuration section
   - HPA configuration
   - Resource limits and requests

2. **Templates updates:**
   - Add HPA resource
   - Update Service for multiple replicas
   - Add HA-specific ConfigMaps

### Documentation Updates
1. **User Guide:**
   - HA configuration examples
   - Performance tuning guidelines
   - Troubleshooting HA issues

2. **Architecture Documentation:**
   - HA optimization flow diagrams
   - Scaling decision algorithms
   - Integration patterns

## SUCCESS CRITERIA

### Functional Requirements
- [ ] HA mode detection and configuration
- [ ] Dynamic dedup window adjustment
- [ ] Auto-scaling based on load
- [ ] Load balancing across replicas
- [ ] Adaptive cache sizing
- [ ] Hot-reloading of HA configuration

### Performance Requirements
- [ ] 10x throughput improvement with HA vs single-replica
- [ ] Auto-scaling responds within 2 minutes
- [ ] Load balancing maintains <10% performance variance
- [ ] Memory usage optimized for workload patterns

### Compatibility Requirements
- [ ] All existing per-source optimization features work unchanged
- [ ] Single-replica deployments unaffected
- [ ] Backward compatibility with existing configurations
- [ ] No breaking changes to existing APIs

## IMPLEMENTATION PRIORITY

**Phase 1 (Week 1):** HA Infrastructure + Dynamic Dedup
**Phase 2 (Week 2):** Auto-scaling + Load Balancing  
**Phase 3 (Week 3):** Adaptive Cache + Testing
**Phase 4 (Week 4):** Documentation + Performance Tuning

This phased approach ensures stable, incremental improvements while maintaining the reliability of existing features.