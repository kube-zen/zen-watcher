# Alertmanager Testing Procedures

This document outlines comprehensive testing procedures for Zen Watcher's Alertmanager configuration to ensure reliable incident response and escalation alerting.

## Testing Overview

The testing procedures cover:
- Configuration validation
- Alert routing verification
- Notification channel testing
- Escalation policy validation
- Integration testing
- Performance testing
- Failure scenario testing

## Prerequisites

### Required Tools
```bash
# Install testing dependencies
kubectl
amtool (Alertmanager CLI)
curl
jq
promtool (Prometheus CLI)
```

### Access Requirements
- Alertmanager web interface access
- Prometheus access for alert testing
- Notification channel credentials (test accounts)
- Kubernetes cluster access

## Pre-Testing Setup

### 1. Environment Validation
```bash
# Check Alertmanager status
kubectl get pods -n monitoring -l app=alertmanager

# Verify Alertmanager is responding
curl -s http://alertmanager:9093/-/healthy

# Check configuration
kubectl exec -n monitoring alertmanager-pod -- amtool config show
```

### 2. Test Data Preparation
```bash
# Create test alert rules
cat << EOF > test-alerts.yaml
groups:
- name: zen-watcher-test-alerts
  rules:
  - alert: ZenWatcherTestCritical
    expr: vector(1)
    for: 0s
    labels:
      severity: critical
      component: test
      cluster: test-cluster
    annotations:
      summary: "Test critical alert"
      description: "This is a test critical alert"
      runbook_url: "https://example.com/runbook"

  - alert: ZenWatcherTestWarning
    expr: vector(1)
    for: 0s
    labels:
      severity: warning
      component: test
      cluster: test-cluster
    annotations:
      summary: "Test warning alert"
      description: "This is a test warning alert"
EOF

# Apply test rules to Prometheus
kubectl apply -f test-alerts.yaml -n monitoring
```

## Configuration Testing

### 1. Syntax Validation
```bash
# Validate alertmanager.yml syntax
kubectl exec -n monitoring alertmanager-pod -- \
  amtool config routes verify

# Check for routing conflicts
kubectl exec -n monitoring alertmanager-pod -- \
  amtool config routes tree

# Validate templates
kubectl exec -n monitoring alertmanager-pod -- \
  amtool template test
```

### 2. Routing Rules Testing
```bash
# Test routing for critical alerts
kubectl exec -n monitoring alertmanager-pod -- \
  amtool config routes test \
  --alert.labels='severity=critical,component=security,alertname=ZenWatcherCriticalEventsSpike'

# Test routing for warning alerts
kubectl exec -n monitoring alertmanager-pod -- \
  amtool config routes test \
  --alert.labels='severity=warning,component=performance,alertname=ZenWatcherSlowProcessing'

# Test routing for info alerts
kubectl exec -n monitoring alertmanager-pod -- \
  amtool config routes test \
  --alert.labels='severity=info,component=discovery,alertname=ZenWatcherNewToolDetected'
```

### 3. Expected Routing Results

#### Critical Security Alert
```
Expected Route: security-oncall
Receivers: 
- security-oncall
- security-critical
Escalation: Immediate (0s wait)
```

#### Critical Infrastructure Alert
```
Expected Route: infrastructure-oncall
Receivers:
- infrastructure-oncall
- infrastructure-critical
Escalation: Immediate (0s wait)
```

#### Warning Performance Alert
```
Expected Route: performance-team
Receivers:
- performance-team
- performance-engineering
Escalation: 5m wait, 30m interval
```

## Alert Firing Tests

### 1. Manual Alert Firing
```bash
# Fire a test critical alert
curl -X POST http://alertmanager:9093/api/v1/alerts \
  -H 'Content-Type: application/json' \
  -d '[
    {
      "labels": {
        "alertname": "ZenWatcherTestCritical",
        "severity": "critical",
        "component": "test",
        "cluster": "test-cluster",
        "instance": "test-instance"
      },
      "annotations": {
        "summary": "Test critical alert fired manually",
        "description": "This is a test to verify critical alert routing",
        "runbook_url": "https://example.com/test-runbook"
      },
      "startsAt": "'$(date -u +%Y-%m-%dT%H:%M:%S.000Z)'"
    }
  ]'

# Fire a test warning alert
curl -X POST http://alertmanager:9093/api/v1/alerts \
  -H 'Content-Type: application/json' \
  -d '[
    {
      "labels": {
        "alertname": "ZenWatcherTestWarning",
        "severity": "warning",
        "component": "test",
        "cluster": "test-cluster",
        "instance": "test-instance"
      },
      "annotations": {
        "summary": "Test warning alert fired manually",
        "description": "This is a test to verify warning alert routing"
      },
      "startsAt": "'$(date -u +%Y-%m-%dT%H:%M:%S.000Z)'"
    }
  ]'
```

### 2. Alert Verification
```bash
# Check fired alerts in Alertmanager
curl -s http://alertmanager:9093/api/v1/alerts | jq '.data[] | select(.labels.alertname == "ZenWatcherTestCritical")'

# Check active silences (should be none for test alerts)
kubectl exec -n monitoring alertmanager-pod -- amtool silence list

# Verify alert state in Prometheus
kubectl exec -n monitoring prometheus-pod -- \
  promtool query instant 'ALERTS{alertname="ZenWatcherTestCritical"}'
```

## Notification Channel Testing

### 1. Email Notification Testing

#### Test Email Configuration
```bash
# Send test email notification
curl -X POST http://alertmanager:9093/api/v1/alerts \
  -H 'Content-Type: application/json' \
  -d '[
    {
      "labels": {
        "alertname": "ZenWatcherEmailTest",
        "severity": "warning",
        "component": "test",
        "cluster": "test-cluster"
      },
      "annotations": {
        "summary": "Test email notification",
        "description": "This is a test to verify email notifications are working"
      },
      "startsAt": "'$(date -u +%Y-%m-%dT%H:%M:%S.000Z)'"
    }
  ]'
```

#### Email Verification Checklist
- [ ] Email received within expected timeframe (30s-2m)
- [ ] Subject line contains correct format: `[FIRING:1] AlertName`
- [ ] Email body contains all required fields
- [ ] HTML formatting renders correctly
- [ ] Links (runbooks) are functional
- [ ] Sender address is correct

### 2. Slack Notification Testing

#### Test Slack Integration
```bash
# Send test to Slack
curl -X POST http://alertmanager:9093/api/v1/alerts \
  -H 'Content-Type: application/json' \
  -d '[
    {
      "labels": {
        "alertname": "ZenWatcherSlackTest",
        "severity": "critical",
        "component": "test",
        "cluster": "test-cluster"
      },
      "annotations": {
        "summary": "Test Slack notification",
        "description": "This is a test to verify Slack integration"
      },
      "startsAt": "'$(date -u +%Y-%m-%dT%H:%M:%S.000Z)'"
    }
  ]'
```

#### Slack Verification Checklist
- [ ] Message appears in correct Slack channel
- [ ] Emoji indicators are appropriate (🚨 for critical, ⚠️ for warning)
- [ ] Message formatting is correct
- [ ] All alert details are included
- [ ] Mentions work for @oncall (if configured)
- [ ] Links are clickable

### 3. PagerDuty Notification Testing

#### Test PagerDuty Integration
```bash
# Send test to PagerDuty
curl -X POST http://alertmanager:9093/api/v1/alerts \
  -H 'Content-Type: application/json' \
  -d '[
    {
      "labels": {
        "alertname": "ZenWatcherPagerDutyTest",
        "severity": "critical",
        "component": "test",
        "cluster": "test-cluster"
      },
      "annotations": {
        "summary": "Test PagerDuty notification",
        "description": "This is a test to verify PagerDuty integration"
      },
      "startsAt": "'$(date -u +%Y-%m-%dT%H:%M:%S.000Z)'"
    }
  ]'
```

#### PagerDuty Verification Checklist
- [ ] Incident created in PagerDuty
- [ ] Correct severity level assigned
- [ ] Incident contains all required details
- [ ] On-call engineer receives notification
- [ ] Incident auto-resolves when alert clears

## Escalation Policy Testing

### 1. Escalation Timing Tests

#### Test Critical Alert Escalation
```bash
# Fire critical alert and monitor escalation
START_TIME=$(date +%s)

curl -X POST http://alertmanager:9093/api/v1/alerts \
  -H 'Content-Type: application/json' \
  -d '[
    {
      "labels": {
        "alertname": "ZenWatcherEscalationTest",
        "severity": "critical",
        "component": "availability",
        "cluster": "test-cluster"
      },
      "annotations": {
        "summary": "Test escalation timing",
        "description": "This alert should trigger immediate escalation"
      },
      "startsAt": "'$(date -u +%Y-%m-%dT%H:%M:%S.000Z)'"
    }
  ]'

# Monitor for escalation (should happen within 5 minutes for critical)
sleep 300

# Check incident status
END_TIME=$(date +%s)
ELAPSED=$((END_TIME - START_TIME))
echo "Escalation test completed. Elapsed time: ${ELAPSED} seconds"
```

#### Expected Escalation Behavior
- **Critical**: Immediate notification (0-5s), escalation every 1-5 minutes
- **Warning**: Notification within 30s, escalation every 15-60 minutes
- **Info**: Notification within 2-10 minutes, no escalation

### 2. Silence Handling During Escalation

```bash
# Create a silence and test escalation continues properly
SILENCE_ID=$(curl -X POST http://alertmanager:9093/api/v1/silences \
  -H 'Content-Type: application/json' \
  -d '{
    "matchers": [{"name": "alertname", "value": "ZenWatcherEscalationTest"}],
    "startsAt": "'$(date -u +%Y-%m-%dT%H:%M:%S.000Z)'",
    "endsAt": "'$(date -u -d '+10 minutes' +%Y-%m-%dT%H:%M:%S.000Z)'",
    "createdBy": "test-team",
    "comment": "Testing silence during escalation"
  }' | jq -r '.silenceID')

echo "Created silence: ${SILENCE_ID}"

# Verify alerts are silenced
curl -s http://alertmanager:9093/api/v1/silences | jq '.data[] | select(.id == "'${SILENCE_ID}'")'

# Clean up test
curl -X DELETE http://alertmanager:9093/api/v1/silences/${SILENCE_ID}
```

## Performance Testing

### 1. Alert Load Testing

#### High Volume Alert Test
```bash
# Generate multiple alerts simultaneously
for i in {1..50}; do
  curl -X POST http://alertmanager:9093/api/v1/alerts \
    -H 'Content-Type: application/json' \
    -d '[
      {
        "labels": {
          "alertname": "ZenWatcherLoadTest",
          "severity": "warning",
          "component": "test",
          "cluster": "test-cluster",
          "instance": "test-instance-'$i'"
        },
        "annotations": {
          "summary": "Load test alert '$i'",
          "description": "This is alert number '$i' in the load test"
        },
        "startsAt": "'$(date -u +%Y-%m-%dT%H:%M:%S.000Z)'"
      }
    ]' &
done

wait
echo "Generated 50 test alerts"

# Monitor Alertmanager performance
kubectl top pod -n monitoring alertmanager
kubectl logs -n monitoring alertmanager-pod --tail=100 | grep -E "(error|warning|deadline)"
```

#### Performance Metrics
- [ ] Alertmanager CPU usage < 80%
- [ ] Alertmanager memory usage < 1GB
- [ ] No timeout errors in logs
- [ ] All notifications sent within 30s

### 2. Notification Rate Testing

```bash
# Test notification rate limiting
# Fire multiple critical alerts rapidly
for i in {1..10}; do
  curl -X POST http://alertmanager:9093/api/v1/alerts \
    -H 'Content-Type: application/json' \
    -d '[
      {
        "labels": {
          "alertname": "ZenWatcherRateTest",
          "severity": "critical",
          "component": "test",
          "cluster": "test-cluster",
          "instance": "rate-test-'$i'"
        },
        "annotations": {
          "summary": "Rate test critical alert '$i'",
          "description": "Testing notification rate limiting"
        },
        "startsAt": "'$(date -u +%Y-%m-%dT%H:%M:%S.000Z)'"
      }
    ]'
done

# Check rate limiting is working
sleep 60
curl -s http://alertmanager:9093/api/v1/alerts | jq '.data | length'
```

## Failure Scenario Testing

### 1. Notification Channel Failures

#### Test Email Server Failure
```bash
# Simulate SMTP failure by using invalid server
# (This would require environment variable changes)

# Alternative: Test with valid but unresponsive server
curl -X POST http://alertmanager:9093/api/v1/alerts \
  -H 'Content-Type: application/json' \
  -d '[
    {
      "labels": {
        "alertname": "ZenWatcherEmailFailureTest",
        "severity": "warning",
        "component": "test",
        "cluster": "test-cluster"
      },
      "annotations": {
        "summary": "Test email failure handling",
        "description": "Testing behavior when email server fails"
      },
      "startsAt": "'$(date -u +%Y-%m-%dT%H:%M:%S.000Z)'"
    }
  ]'

# Check Alertmanager logs for retry attempts
kubectl logs -n monitoring alertmanager-pod | grep -E "(retry|error|failed)" | tail -20
```

#### Expected Failure Behavior
- [ ] Alertmanager logs retry attempts
- [ ] Failed notifications don't block other notifications
- [ ] System continues processing other alerts
- [ ] Dead letter queue (if configured) captures failures

### 2. Template Failure Testing

#### Test Invalid Template
```bash
# Temporarily modify template to include invalid syntax
# Then test alert firing

# Restore template and verify system recovers
```

#### Template Error Handling
- [ ] Invalid templates don't crash Alertmanager
- [ ] System falls back to default templates
- [ ] Errors are logged appropriately
- [ ] Valid templates continue to work

## Integration Testing

### 1. Prometheus Integration

```bash
# Verify Prometheus can reach Alertmanager
kubectl exec -n monitoring prometheus-pod -- \
  curl -s http://alertmanager:9093/api/v1/status | jq '.status'

# Check alert rule loading
kubectl exec -n monitoring prometheus-pod -- \
  promtool rule list | grep ZenWatcher

# Test alert firing from Prometheus
kubectl exec -n monitoring prometheus-pod -- \
  promtool alert 'vector(1)' --alert.name='ZenWatcherIntegrationTest'
```

### 2. Kubernetes Integration

```bash
# Test ConfigMap mounting
kubectl exec -n monitoring alertmanager-pod -- \
  ls -la /etc/alertmanager/

# Verify configuration reload
kubectl create configmap test-reload \
  --from-file=alertmanager.yml=/tmp/test-config.yml \
  -n monitoring
  
kubectl rollout restart deployment/alertmanager -n monitoring

# Check reload was successful
kubectl logs -n monitoring alertmanager-pod | grep reload
```

## Test Cleanup

### 1. Remove Test Alerts
```bash
# Remove all test alerts
curl -X POST http://alertmanager:9093/api/v1/alerts \
  -H 'Content-Type: application/json' \
  -d '[]'

# Verify no test alerts remain
curl -s http://alertmanager:9093/api/v1/alerts | jq '.data | length'
```

### 2. Clean Up Test Rules
```bash
# Remove test alert rules
kubectl delete prometheusrule zen-watcher-test-alerts -n monitoring

# Clean up test ConfigMaps
kubectl delete configmap test-reload -n monitoring
```

### 3. Reset Test Environment
```bash
# Reset any modified configurations
# Clear test silences
kubectl exec -n monitoring alertmanager-pod -- amtool silence expire $(kubectl exec -n monitoring alertmanager-pod -- amtool silence list -q)

# Restart Alertmanager to ensure clean state
kubectl rollout restart deployment/alertmanager -n monitoring

# Verify clean state
kubectl logs -n monitoring alertmanager-pod | tail -20
```

## Test Reporting

### Test Results Template

```markdown
# Alertmanager Test Results

## Test Date
[Date and time of testing]

## Environment
- Kubernetes cluster: [cluster-name]
- Alertmanager version: [version]
- Test environment: [dev/staging/prod]

## Configuration Tests
- [ ] Syntax validation: PASS/FAIL
- [ ] Routing rules: PASS/FAIL
- [ ] Template validation: PASS/FAIL

## Notification Tests
- [ ] Email notifications: PASS/FAIL
- [ ] Slack notifications: PASS/FAIL
- [ ] PagerDuty notifications: PASS/FAIL

## Escalation Tests
- [ ] Critical escalation: PASS/FAIL
- [ ] Warning escalation: PASS/FAIL
- [ ] Silence handling: PASS/FAIL

## Performance Tests
- [ ] High volume alerts: PASS/FAIL
- [ ] Notification rate limiting: PASS/FAIL
- [ ] Resource usage: PASS/FAIL

## Integration Tests
- [ ] Prometheus integration: PASS/FAIL
- [ ] Kubernetes integration: PASS/FAIL

## Issues Found
[List any issues discovered during testing]

## Recommendations
[List any recommendations for improvements]

## Sign-off
- Tested by: [Name]
- Reviewed by: [Name]
- Approved by: [Name]
```

## Continuous Testing

### Automated Test Suite
```bash
#!/bin/bash
# alertmanager-test-suite.sh

echo "Running Alertmanager Test Suite..."

# Configuration tests
echo "Testing configuration..."
amtool config routes verify || exit 1

# Firing test alerts
echo "Firing test alerts..."
curl -X POST http://alertmanager:9093/api/v1/alerts \
  -H 'Content-Type: application/json' \
  -d '[
    {
      "labels": {
        "alertname": "AutomatedTest",
        "severity": "warning",
        "component": "test"
      },
      "annotations": {
        "summary": "Automated test alert",
        "description": "This is an automated test"
      },
      "startsAt": "'$(date -u +%Y-%m-%dT%H:%M:%S.000Z)'"
    }
  ]'

# Wait for notification
sleep 30

# Clean up
echo "Cleaning up test alerts..."
curl -X POST http://alertmanager:9093/api/v1/alerts \
  -H 'Content-Type: application/json' \
  -d '[]'

echo "Test suite completed successfully"
```

### Monitoring Test Effectiveness
```bash
# Track test alert frequency to avoid noise
# Log test results for analysis
# Monitor false positive rates
# Regular review of test coverage
```

## Troubleshooting Common Issues

### 1. Notifications Not Working
```bash
# Check Alertmanager logs
kubectl logs -n monitoring alertmanager-pod | grep -i error

# Verify network connectivity
kubectl exec -n monitoring alertmanager-pod -- \
  nslookup smtp.company.com

# Test notification endpoints manually
curl -X POST -H 'Content-Type: application/json' \
  -d '{"text":"Test message"}' \
  $SLACK_API_URL
```

### 2. Routing Issues
```bash
# Check routing configuration
kubectl exec -n monitoring alertmanager-pod -- \
  amtool config routes tree

# Test routing with specific labels
kubectl exec -n monitoring alertmanager-pod -- \
  amtool config routes test --alert.labels='severity=critical,component=security'
```

### 3. Template Issues
```bash
# Check template syntax
kubectl exec -n monitoring alertmanager-pod -- \
  amtool template test

# Verify template files are mounted
kubectl exec -n monitoring alertmanager-pod -- \
  ls -la /etc/alertmanager/templates/
```

## Contact Information

For testing support:
- **DevOps Team**: #devops on Slack
- **On-call Engineer**: Check PagerDuty schedule
- **Documentation**: Alertmanager documentation
- **Emergency**: Follow incident response procedures