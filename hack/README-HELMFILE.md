# Using Helmfile for Quick Demo

This directory contains a `helmfile.yaml` that can be used to deploy most components via Helmfile instead of individual Helm commands.

## Benefits of Using Helmfile

1. **Simplified Management**: All Helm releases in one declarative file
2. **Dependency Handling**: Automatic ordering via `needs` directives
3. **Namespace Management**: Automatic namespace creation
4. **Conditional Releases**: Easy enable/disable via environment variables
5. **Better Error Handling**: Helmfile provides better error reporting
6. **Parallel Execution**: Helmfile handles parallel deployments safely

## Current Status

The `helmfile.yaml` handles:
- ✅ Ingress Controller (nginx)
- ✅ VictoriaMetrics Operator
- ✅ Zen Watcher
- ✅ Trivy Operator
- ✅ Falco
- ✅ Kyverno

Still deployed via kubectl (not in Helmfile):
- VictoriaMetrics (deployment + service)
- Grafana (deployment + service)
- Checkov (Job)
- kube-bench (Job)
- Ingress resources (Ingress CRDs)

## Usage

### Prerequisites

Install Helmfile:
```bash
# macOS
brew install helmfile

# Linux
curl -LO https://github.com/helmfile/helmfile/releases/latest/download/helmfile_linux_amd64
chmod +x helmfile_linux_amd64
sudo mv helmfile_linux_amd64 /usr/local/bin/helmfile
```

### Deploy with Helmfile

Set environment variables and run:
```bash
export NAMESPACE=zen-system
export INSTALL_TRIVY=true
export INSTALL_FALCO=true
export INSTALL_KYVERNO=true
export SKIP_MONITORING=false
export ZEN_WATCHER_IMAGE=kubezen/zen-watcher:latest
export IMAGE_PULL_POLICY=IfNotPresent

cd hack
helmfile sync
```

### Integration with quick-demo.sh

The `quick-demo.sh` script can be modified to use Helmfile when available. This would:
1. Check if `helmfile` is installed
2. If yes, use `helmfile sync` for Helm-managed components
3. Fall back to individual Helm commands if Helmfile is not available

## Future Improvements

1. Create Helm charts for VictoriaMetrics and Grafana (or use existing ones)
2. Move Checkov and kube-bench to Helm charts or Helmfile hooks
3. Move Ingress resources to Helmfile hooks or separate Helm chart

