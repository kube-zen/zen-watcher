# Zen-Watcher Use Cases & Examples

Real-world scenarios showing how to use zen-watcher in production.

## 📚 Available Use Cases

### 1. Multi-Tenant Filtering
**File:** [01-multi-tenant-filtering.yaml](01-multi-tenant-filtering.yaml)

**Scenario:** Multi-tenant Kubernetes platform with different security requirements per tenant

**What it shows:**
- Namespace-scoped Ingester CRDs
- Different severity thresholds per tenant
- Platform team sees all, tenants see filtered view
- Dynamic updates without restart

**Key Learning:** How to use filters for multi-tenancy

---

### 2. Custom CRD Integration
**File:** [02-custom-crd-integration.yaml](02-custom-crd-integration.yaml)

**Scenario:** You have an internal security scanner that creates custom CRDs

**What it shows:**
- JSONPath-based field extraction
- Severity mapping
- Zero-code integration of custom tools

**Key Learning:** How to integrate ANY CRD without writing Go code

---

### 3. Compliance Reporting & Export
**File:** [03-compliance-reporting.yaml](03-compliance-reporting.yaml)

**Scenario:** Need to export events for SOC2/ISO27001 audits

**What it shows:**
- kubectl queries for ad-hoc reports
- JSON export for audit systems
- CSV generation for spreadsheets
- Custom controller pattern for automated export

**Key Learning:** How to extract data for compliance needs

---

## 🚀 Quick Start with Examples

```bash
# Try multi-tenant filtering
kubectl apply -f examples/use-cases/01-multi-tenant-filtering.yaml

# Verify filters are active
kubectl get observationfilters -A

# Check filtered observations
kubectl get observations -n tenant-a
kubectl get observations -n tenant-b

# Try custom CRD integration (requires your CRD)
kubectl apply -f examples/use-cases/02-custom-crd-integration.yaml

# Export compliance report
kubectl get observations -A -o json | jq -r '...' > report.json
```

## 📖 More Examples Coming Soon

- Alerting integration (Grafana, PagerDuty)
- SIEM integration patterns
- Event correlation workflows
- Performance tuning for high-volume clusters

## 💡 Contributing Your Use Case

Have a great use case? Contribute it!

1. Create `examples/use-cases/NN-your-use-case.yaml`
2. Follow the template format
3. Include clear scenario, solution, and benefits
4. Update this README
5. Submit PR

---

## 🔗 Related Documentation

- [Filtering Guide](../../docs/FILTERING.md) - Complete filter documentation
- [Source Adapters](../../docs/SOURCE_ADAPTERS.md) - Writing custom adapters
- [CRD Documentation](../../docs/CRD.md) - Observation CRD schema
- [Architecture](../../docs/ARCHITECTURE.md) - System design

