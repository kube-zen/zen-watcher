# Manual Webhook Adapter Configuration

## Overview

zen-watcher supports webhooks for security tools through manual configuration. Webhooks are configured via static nginx configuration and processed by zen-watcher's built-in webhook handlers.

## Supported Tools

### Falco Webhooks

Falco webhooks are automatically supported via the `/falco/webhook` endpoint.

**Configuration Example:**

```yaml
# Nginx configuration
location /falco/webhook {
    proxy_pass http://zen-watcher:8080/falco/webhook;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

**Falco Configuration:**

```yaml
# falco.yaml
webhook_output:
  enabled: true
  url: "http://zen-watcher:8080/falco/webhook"
  http_method: "POST"
```

### Kubernetes Audit Webhooks

Kubernetes audit webhooks are automatically supported via the `/audit/webhook` endpoint.

**Configuration Example:**

```yaml
# Nginx configuration
location /audit/webhook {
    proxy_pass http://zen-watcher:8080/audit/webhook;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

**Kubernetes Audit Configuration:**

```yaml
# kube-apiserver configuration
apiVersion: v1
kind: Config
audit:
  webhook:
    batchMaxSize: 100
    batchMaxWait: 5s
    throttleQPS: 10
    throttleBurst: 15
    configFile: /etc/kubernetes/audit-webhook-config.yaml
```

```yaml
# audit-webhook-config.yaml
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: http://zen-watcher:8080/audit/webhook
  name: zen-watcher
contexts:
- context:
    cluster: zen-watcher
    user: ""
  name: default
current-context: default
preferences: {}
users: []
```

## Manual Configuration Process

### Step 1: Configure Nginx

Add webhook endpoints to your nginx configuration:

```nginx
server {
    listen 80;
    server_name _;

    # Falco webhook endpoint
    location /falco/webhook {
        proxy_pass http://zen-watcher:8080/falco/webhook;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # Audit webhook endpoint
    location /audit/webhook {
        proxy_pass http://zen-watcher:8080/audit/webhook;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

### Step 2: Configure Security Tool

Configure your security tool (Falco, Kubernetes Audit, etc.) to send webhooks to the nginx endpoint.

### Step 3: Restart Services

Restart nginx and zen-watcher to apply changes:

```bash
# Restart nginx
sudo systemctl reload nginx

# Restart zen-watcher (if needed)
kubectl rollout restart deployment/zen-watcher -n zen-system
```

### Step 4: Test Webhook Delivery

Test webhook delivery using curl:

```bash
# Test Falco webhook
curl -X POST http://zen-watcher:8080/falco/webhook \
  -H "Content-Type: application/json" \
  -d '{
    "output": "test",
    "priority": "Warning",
    "rule": "test_rule",
    "time": "2025-01-01T00:00:00Z",
    "output_fields": {}
  }'

# Test Audit webhook
curl -X POST http://zen-watcher:8080/audit/webhook \
  -H "Content-Type: application/json" \
  -d '{
    "kind": "Event",
    "apiVersion": "audit.k8s.io/v1",
    "level": "Metadata",
    "auditID": "test-id",
    "stage": "ResponseComplete",
    "requestURI": "/api/v1/namespaces/default/pods",
    "verb": "get",
    "user": {
      "username": "system:serviceaccount:default:default"
    },
    "objectRef": {
      "resource": "pods",
      "namespace": "default",
      "name": "test-pod"
    }
  }'
```

## Authentication

zen-watcher webhook endpoints support authentication via API keys or bearer tokens. Configure authentication in your nginx configuration:

```nginx
location /falco/webhook {
    # Basic authentication
    auth_basic "Webhook Area";
    auth_basic_user_file /etc/nginx/.htpasswd;
    
    proxy_pass http://zen-watcher:8080/falco/webhook;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    
    # Pass authentication header
    proxy_set_header Authorization $http_authorization;
}
```

## Rate Limiting

Rate limiting is configured per endpoint. Default rate limit is 100 requests per minute per IP address. Configure via environment variable:

```bash
WEBHOOK_RATE_LIMIT=200
```

## Troubleshooting

### Webhook Not Received

1. Check nginx logs:
   ```bash
   tail -f /var/log/nginx/error.log
   ```

2. Check zen-watcher logs:
   ```bash
   kubectl logs -f deployment/zen-watcher -n zen-system
   ```

3. Verify endpoint is accessible:
   ```bash
   curl -v http://zen-watcher:8080/falco/webhook
   ```

### Authentication Failures

1. Verify authentication headers are being passed:
   ```bash
   curl -v -H "Authorization: Bearer token" http://zen-watcher:8080/falco/webhook
   ```

2. Check nginx configuration for proper header forwarding

### Rate Limiting Issues

1. Check current rate limit setting:
   ```bash
   kubectl get deployment zen-watcher -n zen-system -o yaml | grep WEBHOOK_RATE_LIMIT
   ```

2. Adjust rate limit if needed:
   ```bash
   kubectl set env deployment/zen-watcher -n zen-system WEBHOOK_RATE_LIMIT=200
   ```

## Additional Resources

- [Falco Documentation](https://falco.org/docs/)
- [Kubernetes Audit Documentation](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/)
- [Nginx Proxy Configuration](https://nginx.org/en/docs/http/ngx_http_proxy_module.html)

