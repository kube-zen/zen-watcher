# Structured Logging

## Overview

Zen Watcher uses structured logging with [zap](https://github.com/uber-go/zap) for production-grade observability. All logs follow a consistent schema with structured fields for easy parsing, filtering, and correlation.

## Log Schema

### Standard Fields

All log entries include these standard fields:

- **`timestamp`**: ISO8601 timestamp (UTC)
- **`level`**: Log level (DEBUG, INFO, WARN, ERROR, FATAL)
- **`message`**: Human-readable log message
- **`caller`**: Source file and line number
- **`component`**: Component name (e.g., "server", "watcher", "gc")
- **`operation`**: Operation name (e.g., "falco_webhook", "observation_create")
- **`source`**: Event source (e.g., "falco", "audit", "trivy", "kyverno")
- **`namespace`**: Kubernetes namespace (when applicable)
- **`correlation_id`**: Request/event correlation ID for tracing
- **`error`**: Error details (when applicable)

### Optional Fields

Additional context fields are included as needed:

- **`event_type`**: Type of event (e.g., "webhook_received", "observation_created")
- **`observation_id`**: Observation CRD UID
- **`severity`**: Event severity (e.g., "HIGH", "MEDIUM", "LOW")
- **`resource_kind`**: Kubernetes resource kind
- **`resource_name`**: Kubernetes resource name
- **`reason`**: Reason code (e.g., "filtered", "deduplicated", "channel_full")
- **`duration`**: Operation duration
- **`count`**: Count of items processed
- **Additional fields**: Key-value pairs for extra context

## Log Levels

### DEBUG
Detailed debugging information, typically only enabled during development or troubleshooting.

```json
{
  "timestamp": "2025-11-29T19:30:00.000Z",
  "level": "DEBUG",
  "message": "Processing Falco alert",
  "component": "watcher",
  "operation": "process_falco",
  "source": "falco",
  "correlation_id": "falco-1234567890",
  "rule": "Write below binary dir"
}
```

### INFO
Informational messages about normal operations.

```json
{
  "timestamp": "2025-11-29T19:30:00.000Z",
  "level": "INFO",
  "message": "Falco webhook received and queued",
  "component": "server",
  "operation": "falco_webhook",
  "source": "falco",
  "event_type": "webhook_received",
  "correlation_id": "falco-1234567890"
}
```

### WARN
Warning messages for recoverable issues or unusual conditions.

```json
{
  "timestamp": "2025-11-29T19:30:00.000Z",
  "level": "WARN",
  "message": "Falco alerts channel full, dropping alert",
  "component": "server",
  "operation": "falco_webhook",
  "source": "falco",
  "event_type": "channel_full",
  "correlation_id": "falco-1234567890",
  "reason": "channel_buffer_full"
}
```

### ERROR
Error messages for failed operations that may be retried.

```json
{
  "timestamp": "2025-11-29T19:30:00.000Z",
  "level": "ERROR",
  "message": "Failed to create observation",
  "component": "watcher",
  "operation": "observation_create",
  "source": "falco",
  "namespace": "default",
  "error": "observations.zen.kube-zen.io \"obs-123\" already exists"
}
```

### FATAL
Critical errors that cause the application to exit.

```json
{
  "timestamp": "2025-11-29T19:30:00.000Z",
  "level": "FATAL",
  "message": "Failed to initialize Kubernetes clients",
  "component": "main",
  "operation": "kubernetes_init",
  "error": "unable to load in-cluster configuration"
}
```

## Configuration

### Environment Variables

- **`LOG_LEVEL`**: Log level (DEBUG, INFO, WARN, ERROR, FATAL). Default: `INFO`
- **`LOG_DEVELOPMENT`**: Enable development mode (colored output, human-readable). Default: `false`

### Example Configuration

```yaml
env:
  - name: LOG_LEVEL
    value: "INFO"
  - name: LOG_DEVELOPMENT
    value: "false"
```

## Log Examples

### Webhook Processing

```json
{
  "timestamp": "2025-11-29T19:30:00.000Z",
  "level": "INFO",
  "message": "Falco webhook received and queued",
  "component": "server",
  "operation": "falco_webhook",
  "source": "falco",
  "event_type": "webhook_received",
  "correlation_id": "falco-1732905000000000000",
  "rule": "Write below binary dir"
}
```

### Observation Creation

```json
{
  "timestamp": "2025-11-29T19:30:01.000Z",
  "level": "INFO",
  "message": "Observation created",
  "component": "watcher",
  "operation": "observation_create",
  "source": "falco",
  "namespace": "default",
  "event_type": "observation_created",
  "observation_id": "obs-123",
  "severity": "HIGH",
  "resource_kind": "Pod",
  "resource_name": "nginx-abc123"
}
```

### Filtering

```json
{
  "timestamp": "2025-11-29T19:30:02.000Z",
  "level": "DEBUG",
  "message": "Observation filtered",
  "component": "filter",
  "operation": "filter_observation",
  "source": "trivy",
  "namespace": "default",
  "reason": "severity_below_threshold",
  "severity": "LOW"
}
```

### Garbage Collection

```json
{
  "timestamp": "2025-11-29T20:00:00.000Z",
  "level": "INFO",
  "message": "Garbage collection completed",
  "component": "gc",
  "operation": "gc_run",
  "count": 42,
  "duration": "1.234s"
}
```

## Correlation IDs

Correlation IDs are automatically generated for webhook requests and can be propagated through the processing pipeline. This enables tracing a single event through all processing stages.

### Webhook Correlation IDs

- **Falco**: `falco-{timestamp}`
- **Audit**: `audit-{auditID}`

### Using Correlation IDs

```go
ctx := logger.WithCorrelationID(ctx, "my-correlation-id")
logger.Info("Processing event", logger.Fields{
    Component:     "watcher",
    CorrelationID:  logger.GetCorrelationID(ctx),
})
```

## Log Aggregation

### JSON Output (Production)

In production, logs are output as JSON for easy parsing by log aggregation systems:

```bash
kubectl logs -n zen-system deployment/zen-watcher | jq .
```

### Development Mode

In development mode, logs are human-readable with colors:

```bash
LOG_DEVELOPMENT=true ./zen-watcher
```

## Best Practices

### 1. Use Structured Fields

✅ **Good:**
```go
logger.Info("Observation created",
    logger.Fields{
        Component:      "watcher",
        Operation:      "observation_create",
        Source:         "falco",
        Namespace:      "default",
        ObservationID:  "obs-123",
        Severity:       "HIGH",
    })
```

❌ **Bad:**
```go
log.Printf("Created observation obs-123 for falco in default namespace with HIGH severity")
```

### 2. Include Correlation IDs

Always include correlation IDs for request tracing:

```go
correlationID := logger.GetCorrelationID(ctx)
logger.Info("Processing event",
    logger.Fields{
        Component:     "watcher",
        CorrelationID: correlationID,
    })
```

### 3. Use Appropriate Log Levels

- **DEBUG**: Detailed debugging (off in production)
- **INFO**: Normal operations
- **WARN**: Recoverable issues
- **ERROR**: Failed operations
- **FATAL**: Critical failures (exits)

### 4. Avoid Sensitive Data

Never log sensitive information:

❌ **Bad:**
```go
logger.Info("Processing secret", logger.Fields{
    Additional: map[string]interface{}{
        "secret_value": secretValue, // DON'T DO THIS
    },
})
```

✅ **Good:**
```go
logger.Info("Processing secret", logger.Fields{
    ResourceName: secretName,
    Namespace:    namespace,
})
```

### 5. Include Error Context

Always include error details:

```go
if err != nil {
    logger.Error("Failed to create observation",
        logger.Fields{
            Component: "watcher",
            Operation: "observation_create",
            Error:     err,
        })
}
```

## Integration with Log Aggregation

### ELK Stack (Elasticsearch, Logstash, Kibana)

Logs are automatically in JSON format, ready for ingestion:

```yaml
# Logstash config
input {
  kubernetes {
    codec => json
  }
}

filter {
  json {
    source => "message"
  }
}
```

### Loki

Loki can parse JSON logs directly:

```yaml
# Promtail config
- json:
    expressions:
      level: level
      component: component
      source: source
      correlation_id: correlation_id
```

### Splunk

Splunk can parse JSON logs:

```conf
# props.conf
[zen-watcher]
KV_MODE = json
```

## Monitoring and Alerting

### Key Log Patterns to Monitor

1. **High Error Rate:**
   ```
   level=ERROR | stats count by component, operation
   ```

2. **Channel Full Events:**
   ```
   event_type=channel_full | stats count by source
   ```

3. **Failed Observations:**
   ```
   operation=observation_create AND level=ERROR | stats count by source, namespace
   ```

4. **GC Errors:**
   ```
   component=gc AND level=ERROR | stats count
   ```

## Troubleshooting

### Enable Debug Logging

```bash
kubectl set env deployment/zen-watcher -n zen-system LOG_LEVEL=DEBUG
```

### View Logs by Component

```bash
kubectl logs -n zen-system deployment/zen-watcher | jq 'select(.component=="server")'
```

### View Logs by Correlation ID

```bash
kubectl logs -n zen-system deployment/zen-watcher | jq 'select(.correlation_id=="falco-1234567890")'
```

### View Errors Only

```bash
kubectl logs -n zen-system deployment/zen-watcher | jq 'select(.level=="ERROR")'
```

