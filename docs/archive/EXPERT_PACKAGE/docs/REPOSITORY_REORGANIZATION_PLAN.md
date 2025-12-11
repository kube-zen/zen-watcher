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

# Repository Reorganization Implementation Plan

## 🎯 Objective

Split the monolithic Zen Watcher repository into focused, maintainable repositories following CNCF best practices for better community contribution, independent versioning, and deployment flexibility.

---

## 📋 Current Repository Analysis

### **Files to Separate by Category:**

```
📊 Current Configurations in /workspace/zen-watcher-main:

🔧 Source Configurations (20+ files):
├── pkg/adapter/generic/           # Source adapter code
├── config/demo-manifests/         # Demo configurations
├── examples/                      # Example deployments
└── pkg/watcher/                   # Source-specific watchers

📊 Observability Configurations (15+ files):
├── config/dashboards/             # Grafana dashboards (6 files)
├── config/monitoring/             # Prometheus rules (3 files)
├── scripts/observability/         # Monitoring setup scripts
└── docs/PERFORMANCE.md            # Performance documentation

🚀 Deployment Configurations (25+ files):
├── charts/zen-watcher/            # Helm chart
├── deployments/crds/              # CRD definitions
├── config/optimization-rules.yaml # Optimization configs
└── scripts/cluster/               # Cluster management

🛠️ Operational Scripts (30+ files):
├── scripts/benchmark/             # Performance testing
├── scripts/ci/                    # CI/CD automation
├── scripts/data/                  # Data generation
└── scripts/utils/                 # Utility functions
```

---

## 🏗️ Proposed Repository Structure

### **Primary Repositories:**

#### **1. zen-watcher (Core)**
**Purpose**: Main application code and core functionality

```
zen-watcher/
├── cmd/                          # Main applications
│   ├── zen-watcher/             # Core application
│   └── zen-watcher-optimize/    # Optimization tool
├── pkg/                          # Library code
│   ├── advisor/                 # Business logic
│   ├── config/                  # Configuration management
│   ├── dedup/                   # Deduplication logic
│   ├── filter/                  # Filtering logic
│   ├── gc/                      # Garbage collection
│   ├── metrics/                 # Metrics definitions
│   ├── models/                  # Data models
│   ├── optimization/            # Optimization engine
│   ├── orchestrator/            # Event orchestration
│   ├── processor/               # Event processing
│   └── server/                  # HTTP server
├── internal/                     # Internal utilities
├── deployments/crds/             # CRD definitions only
├── charts/zen-watcher/           # Helm chart
├── Makefile                      # Build automation
├── go.mod                        # Go dependencies
└── README.md                     # Main documentation
```

**Files to KEEP**:
- All Go source code in `cmd/` and `pkg/`
- CRD definitions in `deployments/crds/`
- Helm chart in `charts/`
- Build and CI files
- Core documentation (README, ARCHITECTURE, etc.)

**Files to MOVE**:
- All configuration files
- Observability dashboards
- Operational scripts
- Examples and demo manifests

---

#### **2. zen-watcher-configurations (NEW)**
**Purpose**: All configuration files, dashboards, and operational manifests

```
zen-watcher-configurations/
├── sources/                      # Source adapter configurations
│   ├── trivy/                   # Trivy-specific configs
│   │   ├── webhook.yaml         # Webhook configuration
│   │   ├── informer.yaml        # Informer configuration
│   │   └── examples/            # Example deployments
│   ├── falco/                   # Falco-specific configs
│   ├── kyverno/                 # Kyverno-specific configs
│   ├── checkov/                 # Checkov-specific configs
│   ├── kube-bench/              # Kube-bench configs
│   └── audit/                   # Audit log configs
├── dashboards/                   # Grafana dashboards
│   ├── zen-watcher-executive.json
│   ├── zen-watcher-operations.json
│   ├── zen-watcher-security.json
│   ├── zen-watcher-dashboard.json
│   ├── zen-watcher-namespace-health.json
│   └── zen-watcher-explorer.json
├── prometheus/                   # Prometheus configurations
│   ├── alerts/                  # Alert rules
│   │   ├── critical.yaml        # Critical alerts
│   │   ├── warning.yaml         # Warning alerts
│   │   └── optimization.yaml    # Optimization alerts
│   └── recording-rules.yaml     # Recording rules
├── helm/                         # Additional Helm values
│   ├── production/              # Production values
│   ├── development/             # Development values
│   └── minimal/                 # Minimal installation
├── examples/                     # Deployment examples
│   ├── basic/                   # Basic installation
│   ├── advanced/                # Advanced configurations
│   ├── multi-tenant/            # Multi-namespace setup
│   └── high-availability/       # HA configurations
└── templates/                    # Configuration templates
    ├── source-config.yaml       # Template for source configs
    ├── filter-config.yaml       # Template for filter configs
    └── observation-config.yaml  # Template for observation configs
```

**Files to MOVE**:
- `config/dashboards/` → `dashboards/`
- `config/monitoring/` → `prometheus/`
- `config/demo-manifests/` → `examples/basic/`
- `examples/` → `examples/`
- Source-specific configurations from `pkg/adapter/` and `pkg/watcher/`

---

#### **3. zen-watcher-scripts (NEW)**
**Purpose**: All operational scripts and automation

```
zen-watcher-scripts/
├── installation/                 # Installation scripts
│   ├── quick-demo.sh            # Quick demo setup
│   ├── install.sh               # Main installation
│   └── cluster/                 # Cluster management
├── benchmarking/                 # Performance testing
│   ├── load-test.sh             # Load testing
│   ├── burst-test.sh            # Burst testing
│   ├── stress-test.sh           # Stress testing
│   ├── quick-bench.sh           # Quick benchmarks
│   └── scale-test.sh            # Scale testing
├── data/                         # Data generation
│   ├── mock-data.sh             # Mock data generation
│   └── send-webhooks.sh         # Webhook testing
├── observability/                # Monitoring setup
│   ├── setup.sh                 # Monitoring installation
│   └── dashboards.sh            # Dashboard import
├── ci/                          # CI/CD scripts
│   ├── build.sh                 # Build automation
│   ├── test.sh                  # Test automation
│   ├── release.sh               # Release automation
│   └── e2e-test.sh              # End-to-end testing
└── utils/                       # Utility functions
    ├── common.sh                # Common utilities
    └── kubernetes.sh            # Kubernetes utilities
```

**Files to MOVE**:
- All scripts from `scripts/` directory
- `hack/benchmark/` → `benchmarking/`
- Operational automation scripts

---

## 🛠️ Implementation Strategy

### **Phase 1: Preparation (Day 1)**

#### **Step 1: Create Repository Structure**
```bash
# Create new repositories (local for now)
mkdir -p zen-watcher-configurations/{sources,dashboards,prometheus,helm,examples,templates}
mkdir -p zen-watcher-scripts/{installation,benchmarking,data,observability,ci,utils}
```

#### **Step 2: Analyze Current Files**
```bash
# Inventory current files
find zen-watcher-main -name "*.yaml" -o -name "*.json" -o -name "*.sh" | head -50

# Categorize by purpose
# - Configuration files
# - Dashboard files  
# - Script files
# - Example files
```

### **Phase 2: File Migration (Days 1-2)**

#### **Step 1: Move Configuration Files**
```bash
# Move dashboards
cp -r zen-watcher-main/config/dashboards/* zen-watcher-configurations/dashboards/

# Move monitoring configs
cp -r zen-watcher-main/config/monitoring/* zen-watcher-configurations/prometheus/

# Move demo manifests
cp -r zen-watcher-main/config/demo-manifests/* zen-watcher-configurations/examples/basic/

# Move examples
cp -r zen-watcher-main/examples/* zen-watcher-configurations/examples/
```

#### **Step 2: Move Source Configurations**
```bash
# Create source-specific directories
mkdir -p zen-watcher-configurations/sources/{trivy,falco,kyverno,checkov,kube-bench,audit}

# Move source-specific files
# (This will require analysis of which files belong to which source)
```

#### **Step 3: Move Scripts**
```bash
# Move all scripts to new structure
cp -r zen-watcher-main/scripts/* zen-watcher-scripts/

# Reorganize into new structure
mkdir -p zen-watcher-scripts/{installation,benchmarking,data,observability,ci,utils}
# (Manual reorganization required)
```

### **Phase 3: Documentation Updates (Day 2-3)**

#### **Step 1: Update Core Repository Documentation**
```markdown
# Update zen-watcher/README.md
- Remove configuration sections
- Add references to zen-watcher-configurations
- Update installation instructions
- Update examples references

# Update zen-watcher/docs/
- Update all documentation files with new paths
- Remove duplicate configuration docs
- Add cross-references
```

#### **Step 2: Create Repository Documentation**
```markdown
# Create zen-watcher-configurations/README.md
- Overview of configuration management
- Source configuration guides
- Dashboard customization
- Helm values explanation

# Create zen-watcher-scripts/README.md  
- Script organization
- Usage instructions
- Automation guides
- Troubleshooting
```

### **Phase 4: Testing & Validation (Day 3-4)**

#### **Step 1: Validate Configurations**
```bash
# Test dashboard imports
# Validate Prometheus rules
# Test Helm chart with new values structure
# Verify script functionality
```

#### **Step 2: Update CI/CD**
```bash
# Update build processes
# Modify deployment pipelines
# Update documentation builds
```

---

## 📊 Effort Estimation

### **Development Hours Breakdown:**

| Phase | Task | Effort (dev/h) | Priority |
|-------|------|----------------|----------|
| **Phase 1** | Repository setup & analysis | 2-3h | High |
| **Phase 2** | File migration & reorganization | 8-12h | High |
| **Phase 3** | Documentation updates | 4-6h | Medium |
| **Phase 4** | Testing & validation | 3-4h | High |
| **Phase 5** | CI/CD updates | 2-3h | Medium |

**Total Effort: 19-28 dev/hours**

### **Timeline Recommendation:**
- **Week 1**: Complete repository reorganization
- **Week 2**: Documentation and testing
- **Week 3**: CI/CD updates and validation

---

## 🎯 Success Criteria

### **Technical Success:**
- [ ] All configuration files properly organized
- [ ] Dashboard imports work correctly
- [ ] Prometheus rules validate successfully
- [ ] Scripts function in new locations
- [ ] Helm chart works with new values structure

### **Documentation Success:**
- [ ] All cross-references updated
- [ ] Installation guides work with new structure
- [ ] Examples are properly organized
- [ ] Troubleshooting guides updated

### **Operational Success:**
- [ ] CI/CD pipelines updated
- [ ] Build processes validated
- [ ] Release workflows tested
- [ ] Community contribution guidelines updated

---

## 🔄 Migration Checklist

### **Pre-Migration:**
- [ ] Complete file inventory
- [ ] Identify all cross-dependencies
- [ ] Create backup of current repository
- [ ] Plan rollback strategy

### **During Migration:**
- [ ] Move configuration files
- [ ] Move dashboard files
- [ ] Move script files
- [ ] Update documentation references
- [ ] Test each component

### **Post-Migration:**
- [ ] Validate all configurations
- [ ] Test installation procedures
- [ ] Update CI/CD workflows
- [ ] Create migration documentation
- [ ] Communicate changes to team

---

## 📞 Next Steps

1. **Approve the plan** and allocate development resources
2. **Set up new repositories** on GitHub (zen-watcher-configurations, zen-watcher-scripts)
3. **Begin Phase 1** with repository setup and file analysis
4. **Execute migration** following the phased approach
5. **Validate results** and update team documentation

This reorganization will significantly improve maintainability, enable faster community contributions, and provide better separation of concerns while maintaining full functionality.