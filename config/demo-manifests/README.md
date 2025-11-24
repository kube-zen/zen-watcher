# Demo Manifests

This directory contains **demo-only** Kubernetes manifests used for demonstration purposes.

**⚠️ IMPORTANT: These are DEMO artifacts only - NOT for production use!**

These manifests are intentionally configured with security issues to demonstrate:
- Checkov static analysis scanning
- Zen Watcher observation collection
- Security tool integration

All manifests are clearly labeled with:
- `demo.zen.kube-zen.io/manifest: "true"` label
- `# DEMO ONLY` comments
- Intentional misconfigurations for demonstration

## Manifests Included

- `insecure-pod.yaml` - Pod with security issues (privileged, hostNetwork, etc.)
- `missing-security-context.yaml` - Pod without security context
- `public-registry.yaml` - Deployment using public registry without image verification
- `excessive-permissions.yaml` - ServiceAccount with excessive RBAC permissions

