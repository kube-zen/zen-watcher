# Alertmanager Silence Management Guide

This guide explains how to manage alert silences in Zen Watcher's Alertmanager configuration.

## What are Silences?

Silences are a mechanism to temporarily mute alerts for a specific time period without modifying the alert rules or routing configuration. They're useful for:

- Planned maintenance windows
- Known issues being worked on
- Testing and development
- Suppressing duplicate alerts during incident response

## Creating Silences

### Via Web Interface

1. Navigate to Alertmanager web interface (usually port 9093)
2. Click on "Silences" in the navigation menu
3. Click "New Silence"
4. Fill in the details:
   - **Matchers**: Define which alerts to silence using label selectors
   - **Start**: When the silence should begin
   - **End**: When the silence should expire
   - **Created By**: Your name or team
   - **Comment**: Reason for the silence

### Example Silence Configurations

#### Silence All Zen Watcher Alerts
```
Matchers:
- alertname=~"ZenWatcher.*"
- severity=critical

Start: 2025-12-08 20:00:00 UTC
End: 2025-12-08 22:00:00 UTC
Created By: operations-team
Comment: Weekly maintenance window
```

#### Silence Specific Alert Type
```
Matchers:
- alertname=ZenWatcherToolOffline
- tool=falco

Start: 2025-12-08 18:00:00 UTC
End: 2025-12-08 19:00:00 UTC
Created By: security-team
Comment: Falco tool upgrade in progress
```

#### Silence by Component
```
Matchers:
- component=performance

Start: 2025-12-08 21:00:00 UTC
End: 2025-12-08 23:00:00 UTC
Created By: performance-team
Comment: Performance testing scheduled
```

### Via API

```bash
# Create a silence
curl -X POST http://alertmanager:9093/api/v1/silences \
  -H 'Content-Type: application/json' \
  -d '{
    "matchers": [
      {
        "name": "alertname",
        "value": "ZenWatcherDown",
        "isRegex": false
      }
    ],
    "startsAt": "2025-12-08T20:00:00.000Z",
    "endsAt": "2025-12-08T22:00:00.000Z",
    "createdBy": "operations-team",
    "comment": "Maintenance window"
  }'
```

## Managing Existing Silences

### List All Silences
```bash
curl http://alertmanager:9093/api/v1/silences
```

### Get Specific Silence
```bash
curl http://alertmanager:9093/api/v1/silences/{silence-id}
```

### Expire Silence Early
```bash
curl -X DELETE http://alertmanager:9093/api/v1/silences/{silence-id}
```

### Update Silence
```bash
curl -X POST http://alertmanager:9093/api/v1/silences \
  -H 'Content-Type: application/json' \
  -d '{
    "id": "{silence-id}",
    "matchers": [...],
    "startsAt": "...",
    "endsAt": "...",
    "createdBy": "...",
    "comment": "..."
  }'
```

## Predefined Maintenance Windows

### Weekly Maintenance
- **Schedule**: Every Sunday 02:00-04:00 UTC
- **Scope**: All Zen Watcher alerts
- **Matcher**: `alertname=~"ZenWatcher.*"`

### Emergency Maintenance
- **Schedule**: As needed
- **Scope**: Specific alerts based on maintenance type
- **Matcher**: Custom based on maintenance scope

### Development Window
- **Schedule**: Weekday evenings 17:00-09:00 UTC
- **Scope**: Non-critical alerts only
- **Matcher**: `severity=warning,severity=info`

## Best Practices

### 1. Use Descriptive Comments
Always include clear comments explaining:
- Why the silence is needed
- Who is responsible
- What work is being performed
- Expected completion time

### 2. Set Appropriate Time Limits
- **Maintenance**: 2-4 hours maximum
- **Known Issues**: Until issue resolution
- **Testing**: 1-2 hours maximum
- **Never**: Set silences to expire very far in the future

### 3. Scope Silences Appropriately
- **Broad**: Only for system-wide maintenance
- **Specific**: Target specific alerts or components
- **Avoid**: Over-silencing important alerts

### 4. Monitor Silence Usage
```bash
# Check active silences
kubectl exec -n monitoring alertmanager-pod -- amtool silence list

# Check silence history
kubectl logs -n monitoring alertmanager-pod | grep -i silence
```

### 5. Review and Clean Up
- Review active silences daily
- Remove expired silences promptly
- Audit silence usage monthly

## Automated Silence Management

### Kubernetes CronJob for Regular Maintenance
```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: maintenance-silences
  namespace: monitoring
spec:
  schedule: "0 2 * * 0"  # Every Sunday at 02:00 UTC
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: create-maintenance-silence
            image: curlimages/curl:latest
            command:
            - /bin/sh
            - -c
            - |
              curl -X POST http://alertmanager:9093/api/v1/silences \
                -H 'Content-Type: application/json' \
                -d '{
                  "matchers": [{"name": "alertname", "value": "ZenWatcher.*", "isRegex": true}],
                  "startsAt": "'$(date -u +%Y-%m-%dT%H:%M:%S.000Z)'",
                  "endsAt": "'$(date -u -d '+2 hours' +%Y-%m-%dT%H:%M:%S.000Z)'",
                  "createdBy": "automated-maintenance",
                  "comment": "Weekly maintenance window"
                }'
          restartPolicy: OnFailure
```

### GitOps-Based Silence Management
Store silence configurations in Git and apply them via automation:

```yaml
# maintenance-silences.yaml
silences:
  - name: "weekly-maintenance"
    schedule: "0 2 * * 0"
    matchers:
      - alertname: "ZenWatcher.*"
    duration: "2h"
    comment: "Weekly maintenance window"
```

## Troubleshooting

### Common Issues

#### 1. Silence Not Working
- Check that alert labels match silence matchers exactly
- Verify silence is within active time range
- Ensure silence hasn't been expired manually

#### 2. Alerts Still Firing
- Confirm alert is using correct label values
- Check for case sensitivity in label matching
- Verify alert is being processed by Alertmanager

#### 3. Silence Expired Unexpectedly
- Check timezone settings
- Verify time format is correct (ISO 8601)
- Ensure system clock is synchronized

### Debug Commands

```bash
# Check active silences with details
kubectl exec -n monitoring alertmanager-pod -- amtool silence list --output=json

# Test silence matching
kubectl exec -n monitoring alertmanager-pod -- \
  amtool silence query --matcher alertname=ZenWatcherDown

# Check Alertmanager logs for silence events
kubectl logs -n monitoring alertmanager-pod | grep -E "(silence|Silence)"
```

## Integration with Incident Response

### During Incidents
1. **Immediate**: Create broad silence for affected component
2. **Investigation**: Narrow silence to specific failing alerts
3. **Resolution**: Remove silence as part of incident closure
4. **Post-Mortem**: Document silence usage for learning

### Communication
- Announce silences in incident channels
- Include silence details in status updates
- Document silence decisions in incident timeline
- Share silence management responsibility

## Security Considerations

### Access Control
- Restrict silence creation to authorized personnel
- Use RBAC to limit silence management capabilities
- Audit silence creation and modification
- Require approvals for long-duration silences

### Monitoring
- Alert on silence creation for critical systems
- Track silence duration and frequency
- Monitor for potential alert fatigue
- Review silence patterns for system health insights

## Example Scenarios

### Scenario 1: Database Maintenance
```yaml
Matchers:
- alertname=ZenWatcherDown
- component=availability

Comment: "Scheduled database maintenance - Zen Watcher will be unavailable"
Duration: 3 hours
Created By: "database-team"
```

### Scenario 2: Security Tool Upgrade
```yaml
Matchers:
- alertname=ZenWatcherToolOffline
- tool=falco

Comment: "Falco security tool upgrade in progress"
Duration: 1 hour
Created By: "security-team"
```

### Scenario 3: Performance Testing
```yaml
Matchers:
- component=performance
- severity=warning

Comment: "Performance testing generating expected load"
Duration: 4 hours
Created By: "performance-team"
```

## Contact Information

For questions about silence management:
- **Community Support**: [GitHub Discussions](https://github.com/kube-zen/zen-watcher/discussions) or [GitHub Issues](https://github.com/kube-zen/zen-watcher/issues)
- **Documentation**: This guide and Alertmanager docs
- **Emergency**: Follow incident response procedures (see [INCIDENT_RESPONSE_SUMMARY.md](INCIDENT_RESPONSE_SUMMARY.md))