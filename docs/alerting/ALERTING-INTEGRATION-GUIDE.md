# Zen Watcher Alerting Integration Guide

## Overview

This guide provides instructions for integrating alerting capabilities with Zen Watcher's Grafana dashboards and AlertManager configuration.

**Version:** 1.0  
**Last Updated:** January 4, 2026  
**Status:** Current implementation documentation

---

## Current Implementation

### AlertManager Configuration

Zen Watcher includes AlertManager configuration with:
- Multi-channel notifications (Email, Slack, PagerDuty)
- Severity-based routing
- Component-based routing
- Automated escalation policies

**Configuration Location:**
- `config/alertmanager/alertmanager.yml` - Main AlertManager configuration
- `config/prometheus/rules/security-alerts.yml` - Security alert rules

### Grafana Dashboards

Zen Watcher includes 6 Grafana dashboards:
1. **Executive Dashboard** (`zen-watcher-executive.json`) - Strategic KPIs and security posture
2. **Operations Dashboard** (`zen-watcher-operations.json`) - SRE-focused operational monitoring
3. **Security Dashboard** (`zen-watcher-security.json`) - Security event analysis
4. **Main Dashboard** (`zen-watcher-dashboard.json`) - Primary observation hub
5. **Namespace Health Dashboard** (`zen-watcher-namespace-health.json`) - Multi-tenant analysis
6. **Explorer Dashboard** (`zen-watcher-explorer.json`) - Detailed investigation

**Dashboard Location:** `config/dashboards/`

### Prometheus Alert Rules

Security alert rules are defined in:
- `config/prometheus/rules/security-alerts.yml`

These alerts monitor:
- Security events (Falco, Kyverno, Trivy)
- Performance issues
- System health

---

## Alert Configuration

### AlertManager Setup

**Deployment:**
```bash
# Deploy AlertManager configuration
kubectl create configmap alertmanager-config \
  --from-file=alertmanager.yml=config/alertmanager/alertmanager.yml \
  -n monitoring

# Apply security alert rules
kubectl apply -f config/prometheus/rules/security-alerts.yml
```

**Configuration Overview:**
- Routes alerts based on severity and component
- Sends notifications to configured channels (Email, Slack, PagerDuty)
- Implements escalation policies for critical alerts

### Dashboard Integration

Dashboards can be imported into Grafana:
```bash
# Import dashboard
kubectl create configmap zen-watcher-dashboard \
  --from-file=dashboard.json=config/dashboards/zen-watcher-dashboard.json \
  -n monitoring

# Label for Grafana auto-discovery
kubectl label configmap zen-watcher-dashboard \
  grafana_dashboard="1" \
  -n monitoring
```

---

## Related Documentation

- [SECURITY_ALERTING_OVERVIEW.md](SECURITY_ALERTING_OVERVIEW.md) - Security alerting system overview and incident response
- [TESTING-PROCEDURES.md](TESTING-PROCEDURES.md) - AlertManager testing procedures
- [SILENCE-MANAGEMENT.md](SILENCE-MANAGEMENT.md) - Alert silence management
- [OPERATIONAL_EXCELLENCE.md](../OPERATIONAL_EXCELLENCE.md) - Operational monitoring and metrics
- [OBSERVABILITY.md](../OBSERVABILITY.md) - Metrics and observability guide

---

**Document Information:**
- **Version:** 1.0
- **Last Updated:** January 4, 2026
- **Review Schedule:** As needed
