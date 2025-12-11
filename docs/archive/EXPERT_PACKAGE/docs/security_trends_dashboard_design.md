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

# Security Trends Dashboard Design Document

## Executive Summary

The Security Trends Dashboard is a specialized analytical interface designed for security analysts and threat hunters within the Zen Watcher ecosystem. Unlike the existing operational security dashboard that focuses on current state monitoring, this dashboard provides deep historical analysis, threat evolution tracking, anomaly detection, and predictive analytics capabilities.

**Target Users:** Security analysts, threat hunters, incident responders, security researchers  
**Primary Use Cases:** Threat hunting, incident investigation, trend analysis, predictive threat intelligence  
**Data Sources:** Zen Watcher Observation CRDs, Prometheus metrics, historical security event data  

---

## 1. Dashboard Architecture Overview

### 1.1 Core Philosophy

The Security Trends Dashboard follows a **temporal-first** design philosophy:
- **Historical Depth:** Multi-timeframe analysis (1h, 6h, 24h, 7d, 30d, 90d, 1y)
- **Pattern Recognition:** Focus on identifying trends, anomalies, and threat evolution
- **Predictive Intelligence:** Forward-looking analytics and threat forecasting
- **Investigative Workflow:** Support for deep-dive security investigations

### 1.2 Design Principles

1. **Temporal Coherence:** All panels maintain consistent time windows for correlation
2. **Anomaly First:** Highlight deviations from baseline patterns prominently
3. **Predictive Focus:** Include forecasting and trend projection capabilities
4. **Investigation Support:** Provide drill-down paths for incident analysis
5. **Intelligence Integration:** Support for threat intelligence correlation

---

## 2. Dashboard Sections & Layout

### 2.1 Section 1: Threat Evolution Timeline (Top Row)

**Purpose:** Visualize how threats have evolved over time  
**Time Range:** 7d, 30d, 90d selectable  
**Height:** 6 grid units

#### Panel 2.1.1: Threat Volume Evolution
- **Type:** Time series graph
- **Metrics:** `zen_watcher_observations_created_total` by source over time
- **Visualization:** Multi-line chart with smoothing
- **Colors:** Source-specific color coding (Trivy=blue, Falco=red, Kyverno=orange, etc.)
- **Key Features:**
  - 7-day moving average overlay
  - Trend arrows indicating growth/decline
  - Interactive legend for source filtering

#### Panel 2.1.2: Severity Distribution Evolution  
- **Type:** Stacked area chart
- **Metrics:** Observations grouped by severity over time
- **Visualization:** Time series with severity-based stacking
- **Colors:** CRITICAL=red, HIGH=orange, MEDIUM=yellow, LOW=blue
- **Key Features:**
  - Percentage and absolute value toggle
  - Cumulative trend overlay
  - Clickable time periods for investigation

#### Panel 2.1.3: Attack Vector Heatmap
- **Type:** Heatmap
- **Metrics:** Source vs Time correlation matrix
- **Visualization:** Intensity-based heatmap with hover details
- **Key Features:**
  - Clickable cells for filtered views
  - Correlation strength indicators
  - Time-based drill-down

### 2.2 Section 2: Anomaly Detection Center (Second Row)

**Purpose:** Highlight deviations from normal patterns  
**Time Range:** 24h, 7d comparison  
**Height:** 8 grid units

#### Panel 2.2.1: Anomaly Score Dashboard
- **Type:** Gauge/Single stat
- **Metrics:** Custom anomaly score calculated from baseline deviation
- **Calculation:** Standard deviation from 30-day rolling average
- **Thresholds:**
  - Green: <1σ (normal variation)
  - Yellow: 1-2σ (minor anomaly)
  - Red: >2σ (major anomaly)
- **Key Features:**
  - Real-time scoring
  - Click to investigate source
  - Historical anomaly timeline

#### Panel 2.2.2: Baseline vs Actual Comparison
- **Type:** Dual-axis time series
- **Metrics:** Actual observations vs predicted baseline
- **Visualization:** Line chart with confidence bands
- **Key Features:**
  - Shaded confidence intervals (±1σ, ±2σ)
  - Anomaly markers with tooltips
  - Statistical significance indicators

#### Panel 2.2.3: Anomaly Source Breakdown
- **Type:** Bar chart
- **Metrics:** Anomalies grouped by source and severity
- **Visualization:** Horizontal bar chart with drill-down
- **Key Features:**
  - Sortable by impact or frequency
  - Clickable bars for detailed view
  - Anomaly severity distribution

### 2.3 Section 3: Pattern Recognition Analytics (Third Row)

**Purpose:** Identify recurring patterns and threat campaigns  
**Time Range:** 30d, 90d analysis  
**Height:** 8 grid units

#### Panel 2.3.1: Threat Pattern Clustering
- **Type:** Graph/Network visualization
- **Metrics:** Similar observation patterns grouped by characteristics
- **Algorithm:** Time-series clustering based on severity, source, namespace patterns
- **Visualization:** Network diagram with cluster nodes
- **Key Features:**
  - Pattern similarity scoring
  - Clickable clusters for investigation
  - Pattern frequency indicators

#### Panel 2.3.2: Temporal Pattern Analysis
- **Type:** Heatmap calendar
- **Metrics:** Daily/weekly/hourly observation patterns
- **Visualization:** Calendar heatmap with intensity coding
- **Key Features:**
  - Time-of-day pattern recognition
  - Weekly/monthly cycle identification
  - Holiday/incident correlation

#### Panel 2.3.3: Threat Campaign Detection
- **Type:** Table with sparklines
- **Metrics:** Suspected coordinated attacks based on pattern correlation
- **Detection Logic:** Multiple sources, similar timing, related targets
- **Visualization:** Sortable table with timeline indicators
- **Key Features:**
  - Campaign confidence scoring
  - Related events correlation
  - Attribution hints

### 2.4 Section 4: Predictive Analytics Engine (Fourth Row)

**Purpose:** Forecast future threats and trends  
**Time Range:** 7d forecast, 30d projection  
**Height:** 6 grid units

#### Panel 2.4.1: Threat Volume Forecast
- **Type:** Time series with forecasting
- **Metrics:** `zen_watcher_observations_created_total` with prediction bands
- **Algorithm:** ARIMA/Exponential smoothing for trend prediction
- **Visualization:** Line chart with forecast confidence intervals
- **Key Features:**
  - 7-day short-term forecast
  - 30-day trend projection
  - Confidence interval visualization

#### Panel 2.4.2: Risk Heat Map Prediction
- **Type:** Heatmap matrix
- **Metrics:** Predicted risk levels by namespace/source combination
- **Algorithm:** Multivariate time-series prediction
- **Visualization:** Risk matrix with predictive coloring
- **Key Features:**
  - Interactive future risk assessment
  - Clickable cells for mitigation planning
  - Risk trend indicators

#### Panel 2.4.3: Emerging Threat Indicators
- **Type:** Status list/indicators
- **Metrics:** Early warning signals for new threat types
- **Detection:** Pattern change detection, new signature emergence
- **Visualization:** Priority-ordered alert list
- **Key Features:**
  - Threat emergence scoring
  - Severity trend indicators
  - Investigation workflow integration

### 2.5 Section 5: Threat Intelligence Integration (Bottom Row)

**Purpose:** Correlate internal observations with external threat intelligence  
**Time Range:** Dynamic based on intelligence feeds  
**Height:** 6 grid units

#### Panel 2.5.1: IOC Correlation Dashboard
- **Type:** Scatter plot with tooltips
- **Metrics:** Internal observations vs external IOC matches
- **Data Sources:** Observation CRDs, threat intelligence feeds
- **Visualization:** Correlation scatter plot with enrichment data
- **Key Features:**
  - IOC match confidence scoring
  - Geographic/temporal correlation
  - External intelligence context

#### Panel 2.5.2: Threat Actor Attribution
- **Type:** Sankey diagram
- **Metrics:** Attack chain attribution to threat actors
- **Visualization:** Flow diagram showing attack progression
- **Key Features:**
  - Attribution confidence levels
  - Attack technique mapping
  - Timeline correlation

#### Panel 2.5.3: Intelligence Feed Health
- **Type:** Status indicators
- **Metrics:** Threat intelligence feed performance and coverage
- **Visualization:** Multi-status dashboard
- **Key Features:**
  - Feed freshness indicators
  - Coverage gap analysis
  - Integration health monitoring

---

## 3. Data Model & Metrics

### 3.1 Core Metrics for Historical Analysis

```promql
# Historical observation counts
zen_watcher_observations_created_total[30d]

# Anomaly detection metrics
rate(zen_watcher_observations_created_total[5m]) - 
avg_over_time(rate(zen_watcher_observations_created_total[5m])[30d])

# Pattern recognition metrics
increase(zen_watcher_observations_created_total[1h] offset 24h) - 
increase(zen_watcher_observations_created_total[1h] offset 48h)

# Predictive analytics
predict_linear(zen_watcher_observations_created_total[5m], 3600)
```

### 3.2 Custom Calculated Metrics

#### Anomaly Score Calculation
```promql
# Standard deviation from baseline
(
  rate(zen_watcher_observations_created_total[5m]) - 
  avg_over_time(rate(zen_watcher_observations_created_total[5m])[30d])
) / (
  stddev_over_time(rate(zen_watcher_observations_created_total[5m])[30d])
)
```

#### Threat Evolution Velocity
```promql
# Rate of change in threat volume
deriv(avg_over_time(zen_watcher_observations_created_total[1h])[7d])
```

#### Pattern Correlation Score
```promql
# Correlation between different sources
corr(
  avg_over_time(zen_watcher_observations_created_total{source="trivy"}[1h])[7d],
  avg_over_time(zen_watcher_observations_created_total{source="falco"}[1h])[7d]
)
```

### 3.3 Derived Security Metrics

#### Threat Density
- Observations per namespace per time period
- Normalized by cluster size and workload

#### Attack Complexity Score
- Weighted score based on multiple sources involvement
- Escalation patterns across severity levels

#### Threat Persistence
- Duration of sustained elevated threat levels
- Recurrence patterns for specific threat types

---

## 4. User Experience Design

### 4.1 Investigation Workflow

1. **Alert/Triage:** Start with anomaly detection center
2. **Timeline Analysis:** Use threat evolution to understand context
3. **Pattern Recognition:** Identify if this is part of larger campaign
4. **Predictive Assessment:** Evaluate future risk and containment needs
5. **Intelligence Correlation:** Enrich with external threat data
6. **Investigation Documentation:** Export findings for incident response

### 4.2 Interaction Patterns

#### Drill-Down Capabilities
- Click any chart element to filter related panels
- Shift+Click for multi-selection
- Right-click for context menus with investigation options

#### Cross-Panel Correlation
- Hover synchronization across related panels
- Shared time range selection affecting all panels
- Source/severity filters propagate across dashboard

#### Export & Sharing
- Dashboard state export (JSON configuration)
- PDF report generation with custom time ranges
- Grafana dashboard link sharing with preserved state

### 4.3 Responsive Design Considerations

#### Layout Adaptations
- **Desktop (1920x1080+):** Full 5-section layout as designed
- **Laptop (1366x768):** Collapse pattern recognition section, maintain others
- **Tablet (1024x768):** Stack sections vertically, reduce panel complexity
- **Mobile (375x667):** Simplified single-column view with essential panels only

#### Performance Optimization
- **Query Optimization:** Pre-calculated metrics for common time ranges
- **Data Sampling:** Automatic aggregation for large time windows
- **Lazy Loading:** Load complex visualizations on demand
- **Caching:** Browser-side caching of frequent queries

---

## 5. Technical Implementation

### 5.1 Grafana Configuration

#### Dashboard Metadata
```json
{
  "title": "Security Trends Analytics",
  "tags": ["security", "threat-hunting", "analytics", "trends"],
  "timezone": "browser",
  "refresh": "30s",
  "time": {
    "from": "now-7d",
    "to": "now"
  },
  "templating": {
    "list": [
      {
        "name": "timeRange",
        "type": "interval",
        "query": "1m,5m,15m,30m,1h,6h,12h,1d,7d,30d,90d",
        "current": {
          "value": "7d"
        }
      },
      {
        "name": "source",
        "type": "custom",
        "query": "trivy,falco,kyverno,checkov,kube-bench,audit,cert-manager,sealed-secrets,kubernetesEvents"
      },
      {
        "name": "severity",
        "type": "custom", 
        "query": "CRITICAL,HIGH,MEDIUM,LOW"
      }
    ]
  }
}
```

#### Panel Templates
Each section uses standardized panel configurations with:
- Consistent color schemes
- Standardized PromQL patterns
- Common threshold definitions
- Unified tooltip formatting

### 5.2 Data Retention Strategy

#### Time-Series Data
- **High Resolution:** 1-minute resolution for 7 days
- **Medium Resolution:** 5-minute resolution for 30 days
- **Low Resolution:** 1-hour resolution for 1 year
- **Archive:** Daily aggregation for 3 years

#### Anomaly Detection Storage
- **Anomaly Events:** Store full details for 1 year
- **Baseline Data:** Keep 30-day rolling baselines
- **Pattern Matches:** Retain for 6 months
- **Prediction History:** Archive forecast accuracy for model improvement

### 5.3 Performance Requirements

#### Query Performance
- **Dashboard Load Time:** <5 seconds for full dashboard
- **Panel Update Frequency:** 30-second refresh for real-time panels
- **Complex Query Timeout:** 10-second limit per panel
- **Concurrent Query Support:** 10 simultaneous queries

#### Resource Utilization
- **Memory Usage:** <100MB additional for dashboard processing
- **CPU Impact:** <5% increase during dashboard access
- **Network Bandwidth:** <1MB per dashboard refresh
- **Storage Growth:** <10% increase for trend data

---

## 6. Security & Compliance

### 6.1 Data Classification

#### Sensitivity Levels
- **Public:** Aggregate trend data without specific targets
- **Internal:** Observation counts and patterns by namespace
- **Confidential:** Individual observation details and investigation data
- **Restricted:** Threat intelligence correlations and attribution data

#### Access Control
- **Role-Based Access:** Different dashboard views based on user roles
- **Namespace Filtering:** Restrict visibility based on user permissions
- **Data Masking:** Anonymize sensitive fields in shared dashboards
- **Audit Logging:** Track all dashboard access and actions

### 6.2 Compliance Considerations

#### Regulatory Alignment
- **SOC 2:** Access controls and audit trails
- **GDPR:** Data retention and anonymization for EU data
- **HIPAA:** Healthcare data handling for medical environments
- **PCI DSS:** Payment data correlation and protection

#### Data Handling
- **Encryption:** All data encrypted at rest and in transit
- **Retention Policies:** Automatic cleanup based on data classification
- **Export Controls:** Secure export mechanisms for investigations
- **Incident Response:** Dashboard integration with incident workflows

---

## 7. Future Enhancements

### 7.1 Machine Learning Integration

#### Advanced Anomaly Detection
- **Isolation Forest:** For multi-dimensional anomaly detection
- **LSTM Networks:** For time-series pattern recognition
- **Autoencoders:** For unsupervised threat pattern learning
- **Ensemble Methods:** Combining multiple detection algorithms

#### Predictive Model Improvements
- **Seasonal Decomposition:** Separate trend from seasonal patterns
- **External Factor Integration:** Include business events and external threats
- **Multi-variate Prediction:** Consider source correlations in forecasts
- **Model Accuracy Tracking:** Continuous improvement of prediction accuracy

### 7.2 Threat Intelligence Expansion

#### Automated IOC Enrichment
- **VirusTotal Integration:** Automatic file hash and URL checking
- **AlienVault OTX:** Community-driven threat intelligence
- **MISP Integration:** Open-source threat intelligence platform
- **Commercial Feed Integration:** Enterprise threat intelligence services

#### Attribution Enhancement
- **Attack Pattern Mapping:** MITRE ATT&CK framework integration
- **Threat Actor Tracking:** Known actor behavior pattern correlation
- **Campaign Attribution:** Link related attacks to ongoing campaigns
- **Geographic Correlation:** Location-based threat actor tracking

### 7.3 Advanced Analytics

#### Behavioral Analysis
- **User Behavior Analytics:** Track legitimate user patterns
- **Entity Behavior Analytics:** Monitor service and application behavior
- **Peer Group Analysis:** Compare behavior across similar entities
- **Deviation Scoring:** Quantify behavior anomalies

#### Threat Hunting Automation
- **Hypothesis Generation:** Automatically suggest hunting hypotheses
- **Query Suggestions:** Proactive threat hunting query recommendations
- **Investigation Playbooks:** Guided investigation workflows
- **Knowledge Base Integration:** Built-in threat hunting expertise

---

## 8. Deployment & Maintenance

### 8.1 Installation Requirements

#### System Dependencies
- **Grafana 10.0+:** With time series and network visualization plugins
- **Prometheus:** With recording rules for complex calculations
- **Zen Watcher:** Latest version with full Observation CRD support
- **Storage:** Sufficient for extended historical data retention

#### Configuration Steps
1. Deploy updated Grafana dashboards via Zen Watcher Helm chart
2. Configure Prometheus recording rules for performance optimization
3. Set up automated data retention and cleanup policies
4. Configure user access controls and role-based permissions
5. Establish monitoring for dashboard performance and availability

### 8.2 Operational Maintenance

#### Regular Tasks
- **Weekly:** Review anomaly detection accuracy and false positive rates
- **Monthly:** Analyze prediction model performance and adjust parameters
- **Quarterly:** Review access controls and user permissions
- **Annually:** Comprehensive security audit of dashboard access

#### Performance Monitoring
- **Query Response Times:** Monitor and optimize slow queries
- **Dashboard Usage:** Track which panels are most valuable
- **User Feedback:** Regular surveys for usability improvements
- **Security Events:** Monitor for suspicious dashboard access patterns

### 8.3 Troubleshooting Guide

#### Common Issues
- **Slow Loading:** Check Prometheus query performance and optimize
- **Missing Data:** Verify Zen Watcher Observation CRD generation
- **Inconsistent Results:** Review time range settings across panels
- **Access Problems:** Check Grafana user permissions and authentication

#### Debug Mode
- **Query Inspector:** Use Grafana's query inspector for debugging
- **Prometheus Console:** Direct query testing in Prometheus
- **Dashboard JSON:** Export and validate dashboard configuration
- **Log Analysis:** Review Zen Watcher logs for data generation issues

---

## 9. Success Metrics & KPIs

### 9.1 Dashboard Adoption Metrics
- **Daily Active Users:** Number of unique users accessing dashboard
- **Session Duration:** Average time spent on dashboard per session
- **Feature Usage:** Which panels and features are most utilized
- **Investigation Rate:** Percentage of sessions leading to investigations

### 9.2 Security Effectiveness Metrics
- **Anomaly Detection Accuracy:** Ratio of true positives to false positives
- **Investigation Success Rate:** Percentage of investigations leading to findings
- **Time to Detection:** Average time from threat occurrence to detection
- **Prediction Accuracy:** Accuracy of threat forecasting models

### 9.3 Operational Efficiency Metrics
- **Dashboard Performance:** Average load times and query response times
- **User Satisfaction:** Regular surveys on dashboard usability and value
- **Maintenance Overhead:** Time required for dashboard maintenance
- **Cost Effectiveness:** Security outcomes per unit of operational cost

---

## 10. Conclusion

The Security Trends Dashboard represents a significant advancement in security observability for Zen Watcher users. By focusing on historical patterns, threat evolution, anomaly detection, and predictive analytics, it transforms security monitoring from reactive to proactive.

**Key Benefits:**
- **Enhanced Threat Detection:** Early identification of emerging threats
- **Improved Investigation Efficiency:** Streamlined workflow for security analysts
- **Predictive Capabilities:** Forward-looking security risk assessment
- **Comprehensive Visibility:** Complete threat landscape understanding

**Next Steps:**
1. Implement core dashboard structure with basic trend visualization
2. Develop anomaly detection algorithms and baseline calculations
3. Integrate threat intelligence feeds for enriched analysis
4. Deploy machine learning models for predictive analytics
5. Establish operational monitoring and maintenance procedures

The Security Trends Dashboard will establish Zen Watcher as a comprehensive security analytics platform, providing security analysts and threat hunters with the tools they need to stay ahead of evolving threats in Kubernetes environments.

---

**Document Version:** 1.0  
**Last Updated:** 2025-12-08  
**Author:** Security Analytics Team  
**Review Cycle:** Quarterly  
**Distribution:** Zen Watcher Development Team, Security Operations