---
name: New Source Adapter Request
about: Request support for a new security/compliance tool
title: '[ADAPTER] '
labels: enhancement, source-adapter
assignees: ''
---

## Tool Information
- **Tool Name**: [e.g. Wiz, Snyk, Kubescape]
- **Tool Type**: [e.g. Vulnerability Scanner, Policy Engine, Runtime Security]
- **Tool Website**: [URL]

## Integration Method
How should Zen Watcher integrate with this tool?

- [ ] **Informer (CRD-based)** - Tool emits Kubernetes CRDs
- [ ] **Webhook** - Tool can send HTTP webhooks
- [ ] **ConfigMap** - Tool writes to ConfigMaps
- [ ] **Logs** - Tool outputs structured logs
- [ ] **Other** - Describe below

## Tool Output Format
Describe the format of data the tool produces:
- CRD schema (if CRD-based)
- Webhook payload structure
- ConfigMap structure
- Log format

## Use Case
Why is this tool important for your security/compliance workflow?

## Additional Context
- **Tool Version**: [e.g. v1.2.3]
- **Kubernetes Version**: [e.g. 1.28.0]
- **Priority**: [High/Medium/Low]

## Implementation Notes (Optional)
If you're willing to contribute this adapter, or have implementation ideas, share them here.

