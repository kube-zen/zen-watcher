# Manual Webhook Adapter Configuration

## Overview

zen-watcher supports webhooks for security tools through manual configuration. Webhooks are configured via nginx and processed by zen-watcher's built-in webhook handlers.

**Important:** nginx is a **separate deployment** from zen-watcher. zen-watcher does not include nginx as part of its deployment.

**Recommended Approach:** If you're using **nginx-ingress-controller** (common in Kubernetes clusters), use Kubernetes **Ingress resources** instead of manual nginx ConfigMaps. This is the Kubernetes-native approach and doesn't require restarting deployments when adding new webhook endpoints. See "Step 3" below for Ingress resource examples.

**Alternative:** For standalone nginx deployments or nginx on VM/host, you must manually update nginx configuration and reload/restart nginx when adding new endpoints (causes brief downtime).

## Supported Tools

### Falco Webhooks

Falco webhooks are supported via the recommended `/ingest/falco` endpoint (or legacy `/falco/webhook` for backward compatibility).

**Recommended Configuration (via Ingester CRD):**

```yaml
apiVersion: zen.kube-zen.io/v1alpha1
kind: Ingester
metadata:
  name: falco-webhook
  namespace: zen-system
spec:
  source: falco
  ingester: webhook
  webhook:
    path: /ingest/falco
    port: 8080
  destinations:
    - type: crd
      value: observations
```

**Nginx Configuration for Falco Webhook:**

```nginx
location /ingest/falco {
    proxy_pass http://zen-watcher:8080/ingest/falco;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    
    # Timeouts
    proxy_connect_timeout 60s;
    proxy_send_timeout 60s;
    proxy_read_timeout 60s;
    
    # Buffer settings
    proxy_buffering on;
    proxy_buffer_size 4k;
    proxy_buffers 8 4k;
}
```

**Falco Configuration:**

```yaml
# falco.yaml
webhook_output:
  enabled: true
  url: "http://zen-watcher:8080/ingest/falco"
  http_method: "POST"
```

### Kubernetes Audit Webhooks

Kubernetes audit webhooks are supported via the recommended `/ingest/k8s-audit` endpoint (or legacy `/audit/webhook` for backward compatibility).

**Recommended Configuration (via Ingester CRD):**

```yaml
apiVersion: zen.kube-zen.io/v1alpha1
kind: Ingester
metadata:
  name: k8s-audit-webhook
  namespace: zen-system
spec:
  source: audit
  ingester: webhook
  webhook:
    path: /ingest/k8s-audit
    port: 8080
  destinations:
    - type: crd
      value: observations
```

**Nginx Configuration for Kubernetes Audit Webhook:**

```nginx
location /ingest/k8s-audit {
    proxy_pass http://zen-watcher:8080/ingest/k8s-audit;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    
    # Timeouts
    proxy_connect_timeout 60s;
    proxy_send_timeout 60s;
    proxy_read_timeout 60s;
    
    # Buffer settings
    proxy_buffering on;
    proxy_buffer_size 4k;
    proxy_buffers 8 4k;
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

Add webhook endpoints to your nginx configuration. **Each webhook ingester requires its own location block.**

**Nginx Configuration Template:**

```nginx
server {
    listen 80;
    server_name _;

    # When you create a new Ingester CRD with webhook ingester type, 
    # you MUST add a corresponding location block here
    
    # Falco webhook endpoint
    location /ingest/falco {
        proxy_pass http://zen-watcher:8080/ingest/falco;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # Timeouts
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
        
        # Buffer settings
        proxy_buffering on;
        proxy_buffer_size 4k;
        proxy_buffers 8 4k;
    }

    # Kubernetes Audit webhook endpoint
    location /ingest/k8s-audit {
        proxy_pass http://zen-watcher:8080/ingest/k8s-audit;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # Timeouts
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
        
        # Buffer settings
        proxy_buffering on;
        proxy_buffer_size 4k;
        proxy_buffers 8 4k;
    }

    # Legacy endpoints (kept for backward compatibility)
    # Note: Use /ingest/* endpoints instead for new deployments
    location /falco/webhook {
        proxy_pass http://zen-watcher:8080/falco/webhook;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location /audit/webhook {
        proxy_pass http://zen-watcher:8080/audit/webhook;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

**Important:** When adding a new webhook ingester via Ingester CRD, you **must** add a corresponding nginx location block. See the "Adding a New Webhook Ingester" section below.

### Step 2: Configure Security Tool

Configure your security tool (Falco, Kubernetes Audit, etc.) to send webhooks to the nginx endpoint.

### Step 3: Apply Nginx Configuration Changes

Apply your nginx configuration changes based on how nginx is deployed:

**If using nginx-ingress-controller (Recommended):**

Use **one Kubernetes Ingress resource per webhook endpoint**. This is the Kubernetes-native approach, supports GitOps, and ensures adding a new ingester doesn't affect existing ones.

**Example: One Ingress per endpoint**

```yaml
# File: ingresses/falco-ingress.yaml
# Ingress for Falco webhook endpoint
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: zen-watcher-falco
  namespace: zen-system
spec:
  ingressClassName: nginx
  rules:
  - http:
      paths:
      - path: /ingest/falco
        pathType: Prefix
        backend:
          service:
            name: zen-watcher
            port:
              number: 8080
---
# File: ingresses/k8s-audit-ingress.yaml
# Ingress for Kubernetes Audit webhook endpoint
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: zen-watcher-k8s-audit
  namespace: zen-system
spec:
  ingressClassName: nginx
  rules:
  - http:
      paths:
      - path: /ingest/k8s-audit
        pathType: Prefix
        backend:
          service:
            name: zen-watcher
            port:
              number: 8080
```

**Benefits:**
- **No restarts needed** - just `kubectl apply` each Ingress resource
- **GitOps-friendly** - each endpoint managed independently as separate files
- **Isolated** - adding, modifying, or removing one endpoint doesn't affect others
- **Kubernetes-native** - declarative Ingress resources, managed like any other K8s resource
- **Easy to version control** - each Ingress in its own file, easy to track changes

**If using a standalone nginx Deployment with ConfigMap (not recommended):**

This approach causes brief downtime when adding new endpoints:

```bash
# Update the ConfigMap
kubectl create configmap nginx-config --from-file=nginx.conf=<path-to-nginx.conf> --dry-run=client -o yaml | kubectl apply -f -n <nginx-namespace>

# Restart deployment to pick up ConfigMap changes (causes brief downtime)
kubectl rollout restart deployment/nginx -n <nginx-namespace>
```

**Note:** Restarting causes downtime. For zero-downtime updates, you'd need to manually reload nginx (`kubectl exec ... nginx -s reload`), but this is not recommended for production. Use Ingress resources with nginx-ingress-controller instead.

**If nginx is deployed outside Kubernetes (on a VM/host):**

```bash
# Test nginx configuration
nginx -t

# Reload nginx configuration
sudo systemctl reload nginx
# or
sudo nginx -s reload
```

**Restart zen-watcher if needed:**

```bash
kubectl rollout restart deployment/zen-watcher -n zen-system
```

### Step 4: Test Webhook Delivery

Test webhook delivery using curl:

```bash
# Test Falco webhook (recommended /ingest/* endpoint)
curl -X POST http://zen-watcher:8080/ingest/falco \
  -H "Content-Type: application/json" \
  -d '{
    "output": "test",
    "priority": "Warning",
    "rule": "test_rule",
    "time": "2025-01-01T00:00:00Z",
    "output_fields": {}
  }'

# Test Kubernetes Audit webhook (recommended /ingest/* endpoint)
curl -X POST http://zen-watcher:8080/ingest/k8s-audit \
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

# Legacy endpoints (still supported for backward compatibility)
curl -X POST http://zen-watcher:8080/falco/webhook \
  -H "Content-Type: application/json" \
  -d '{"output": "test", "priority": "Warning", "rule": "test_rule"}'

curl -X POST http://zen-watcher:8080/audit/webhook \
  -H "Content-Type: application/json" \
  -d '{"kind": "Event", "apiVersion": "audit.k8s.io/v1"}'
```

## Adding a New Webhook Ingester

When you create a new webhook ingester via Ingester CRD, you **must** also create a corresponding Kubernetes Ingress resource (if using nginx-ingress-controller) or nginx location block (if using standalone nginx).

This ensures each webhook endpoint is managed independently, supports GitOps workflows, and prevents adding new endpoints from affecting existing ones.

### Step-by-Step Process (GitOps-Friendly)

1. **Create the Ingester CRD** with your webhook path (e.g., `/ingest/my-source`):

```yaml
apiVersion: zen.kube-zen.io/v1alpha1
kind: Ingester
metadata:
  name: my-webhook-source
  namespace: zen-system
spec:
  source: my-source
  ingester: webhook
  webhook:
    path: /ingest/my-source
    port: 8080
  destinations:
    - type: crd
      value: observations
```

2. **Create the corresponding Ingress resource** (if using nginx-ingress-controller - Recommended):

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: zen-watcher-my-source
  namespace: zen-system
  # Optional: Add annotations for rate limiting, auth, etc.
  # annotations:
  #   nginx.ingress.kubernetes.io/limit-rps: "100"
spec:
  ingressClassName: nginx
  rules:
  - http:
      paths:
      - path: /ingest/my-source
        pathType: Prefix
        backend:
          service:
            name: zen-watcher
            port:
              number: 8080
```

Apply both resources via GitOps or manually:

```bash
# Apply both Ingester and Ingress together
kubectl apply -f ingester.yaml -f ingress.yaml

# Or apply via GitOps - both resources can be in the same directory/repo
```

**Alternative: If using standalone nginx**, add the nginx location block:

```nginx
location /ingest/my-source {
    proxy_pass http://zen-watcher:8080/ingest/my-source;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    
    # Timeouts
    proxy_connect_timeout 60s;
    proxy_send_timeout 60s;
    proxy_read_timeout 60s;
    
    # Buffer settings
    proxy_buffering on;
    proxy_buffer_size 4k;
    proxy_buffers 8 4k;
    
    # Optional: Add authentication
    # auth_basic "Webhook Area";
    # auth_basic_user_file /etc/nginx/.htpasswd;
    # proxy_set_header Authorization $http_authorization;
    
    # Optional: Add rate limiting
    # limit_req zone=webhook_limit burst=10 nodelay;
}
```

3. **Apply nginx configuration** to your nginx deployment:

**If using Ingress resources (Recommended):**

```bash
# Apply both Ingester and Ingress resources
kubectl apply -f ingester-my-source.yaml
kubectl apply -f ingress-my-source.yaml

# No restart needed - nginx-ingress-controller picks up changes automatically
```

**If using standalone nginx (ConfigMap-based):**

```bash
# Update the ConfigMap with your new nginx configuration
kubectl create configmap nginx-config --from-file=nginx.conf=<path-to-nginx.conf> --dry-run=client -o yaml | kubectl apply -f -n <nginx-namespace>

# Restart the nginx deployment to pick up ConfigMap changes
kubectl rollout restart deployment/nginx -n <nginx-namespace>
```

**Note:** nginx is deployed separately from zen-watcher. When you add a new webhook ingester with standalone nginx, you must manually update the nginx ConfigMap and restart the nginx deployment.

**If nginx is outside Kubernetes (VM/host):**

```bash
# Test nginx configuration
nginx -t

# Reload nginx
sudo systemctl reload nginx
# or
sudo nginx -s reload
```

5. **Verify the endpoint** is accessible:

```bash
curl -X POST http://zen-watcher:8080/ingest/my-source \
  -H "Content-Type: application/json" \
  -d '{"test": "data"}'
```

### Important Notes

- **Path Must Match Exactly**: The `location` path in nginx must exactly match the `webhook.path` in your Ingester CRD
- **Each Endpoint Requires a Location Block**: Each webhook ingester requires its own location block in nginx
- **Order Matters**: More specific locations should be defined before less specific ones in nginx
- **Port Configuration**: Ensure the `webhook.port` in your Ingester CRD matches the port zen-watcher is listening on (default: 8080)

### Example: Multiple Webhook Ingesters (GitOps Pattern)

Each webhook endpoint gets its own Ingress resource for independent management:

```yaml
# File: ingesters/falco-ingester.yaml
apiVersion: zen.kube-zen.io/v1alpha1
kind: Ingester
metadata:
  name: falco-webhook
  namespace: zen-system
spec:
  source: falco
  ingester: webhook
  webhook:
    path: /ingest/falco
    port: 8080
  destinations:
    - type: crd
      value: observations
---
# File: ingresses/falco-ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: zen-watcher-falco
  namespace: zen-system
spec:
  ingressClassName: nginx
  rules:
  - http:
      paths:
      - path: /ingest/falco
        pathType: Prefix
        backend:
          service:
            name: zen-watcher
            port:
              number: 8080
---
# File: ingesters/k8s-audit-ingester.yaml
apiVersion: zen.kube-zen.io/v1alpha1
kind: Ingester
metadata:
  name: k8s-audit-webhook
  namespace: zen-system
spec:
  source: audit
  ingester: webhook
  webhook:
    path: /ingest/k8s-audit
    port: 8080
  destinations:
    - type: crd
      value: observations
---
# File: ingresses/k8s-audit-ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: zen-watcher-k8s-audit
  namespace: zen-system
spec:
  ingressClassName: nginx
  rules:
  - http:
      paths:
      - path: /ingest/k8s-audit
        pathType: Prefix
        backend:
          service:
            name: zen-watcher
            port:
              number: 8080
```

**GitOps Workflow:**
```bash
# Apply all ingesters and ingresses from your GitOps repo
kubectl apply -f ingesters/
kubectl apply -f ingresses/

# Or apply together if they're in the same directory
kubectl apply -f manifests/
```

Each endpoint is isolated - adding, modifying, or removing one endpoint doesn't affect others.

## Authentication

zen-watcher webhook endpoints support authentication via API keys or bearer tokens. Configure authentication in your nginx configuration:

```nginx
# Recommended: Use /ingest/* endpoints with authentication
# Each endpoint requires its own location block
location /ingest/falco {
    # Basic authentication
    auth_basic "Webhook Area";
    auth_basic_user_file /etc/nginx/.htpasswd;
    
    proxy_pass http://zen-watcher:8080/ingest/falco;
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

