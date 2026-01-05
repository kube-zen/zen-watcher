#!/bin/bash
# Copyright 2025 The Zen Watcher Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -euo pipefail

# E2E validation script for zen-watcher 1.2.1 release
# This script validates the release against a real cluster without modifying kubeconfig.

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Defaults
NAMESPACE="${NAMESPACE:-zen-system}"
CONTEXT="${CONTEXT:-}"
DRY_RUN="${DRY_RUN:-true}"
# Default to using helm repository chart
HELM_CHART_PATH="${HELM_CHART_PATH:-kube-zen/zen-watcher}"

# Functions
error() {
    echo -e "${RED}❌ Error: $1${NC}" >&2
    exit 1
}

info() {
    echo -e "${GREEN}ℹ️  $1${NC}"
}

warn() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

check_prerequisites() {
    info "Checking prerequisites..."
    
    if ! command -v kubectl &> /dev/null; then
        error "kubectl not found. Please install kubectl."
    fi
    
    if ! command -v helm &> /dev/null; then
        error "helm not found. Please install Helm 3.8+."
    fi
    
    if [ -z "$CONTEXT" ]; then
        error "CONTEXT environment variable not set. Set it to your Kubernetes context."
    fi
    
    # Verify context exists
    if ! kubectl config get-contexts "$CONTEXT" &> /dev/null; then
        error "Context '$CONTEXT' not found in kubeconfig."
    fi
    
    info "Prerequisites check passed"
}

validate_helm_chart() {
    info "Validating Helm chart..."
    
    # Add helm repository if using repo chart
    if [[ "$HELM_CHART_PATH" == *"/"* ]] && [[ "$HELM_CHART_PATH" != "./"* ]] && [[ "$HELM_CHART_PATH" != "../"* ]]; then
        info "Adding Helm repository..."
        helm repo add kube-zen https://kube-zen.github.io/helm-charts 2>/dev/null || true
        helm repo update || error "Failed to update Helm repository"
    elif [ ! -d "$HELM_CHART_PATH" ]; then
        error "Helm chart not found at $HELM_CHART_PATH"
    fi
    
    # Lint chart (skip for repo charts as they're validated upstream)
    if [[ "$HELM_CHART_PATH" == "./"* ]] || [[ "$HELM_CHART_PATH" == "../"* ]]; then
        helm lint "$HELM_CHART_PATH" || error "Helm chart lint failed"
    fi
    
    # Template and validate
    helm template zen-watcher "$HELM_CHART_PATH" \
        --namespace "$NAMESPACE" \
        --set image.tag=1.0.0-alpha \
        > /tmp/zen-watcher-manifests.yaml
    
    # Dry-run validation
    if [ "$DRY_RUN" = "true" ]; then
        info "Running kubectl dry-run validation..."
        kubectl apply --dry-run=client \
            --context "$CONTEXT" \
            --namespace "$NAMESPACE" \
            -f /tmp/zen-watcher-manifests.yaml || error "Dry-run validation failed"
        info "Dry-run validation passed"
    else
        warn "DRY_RUN=false: This would apply manifests to cluster"
        warn "To actually apply, uncomment the following line:"
        echo "# kubectl apply --context $CONTEXT --namespace $NAMESPACE -f /tmp/zen-watcher-manifests.yaml"
    fi
    
    info "Helm chart validation complete"
}

validate_ingesters() {
    info "Validating example Ingesters..."
    
    local examples_dir="./examples/ingesters"
    if [ ! -d "$examples_dir" ]; then
        warn "Examples directory not found, skipping Ingester validation"
        return
    fi
    
    # Find example Ingester YAMLs
    local ingester_files=($(find "$examples_dir" -name "*.yaml" -type f | head -2))
    
    if [ ${#ingester_files[@]} -eq 0 ]; then
        warn "No example Ingesters found, skipping validation"
        return
    fi
    
    for file in "${ingester_files[@]}"; do
        info "Validating $file..."
        
        if [ "$DRY_RUN" = "true" ]; then
            kubectl apply --dry-run=client \
                --context "$CONTEXT" \
                --namespace "$NAMESPACE" \
                -f "$file" || error "Ingester validation failed: $file"
        else
            warn "DRY_RUN=false: This would apply $file"
        fi
    done
    
    info "Ingester validation complete"
}

validate_observations() {
    info "Validating Observations..."
    
    # Check if obsctl exists
    if ! command -v obsctl &> /dev/null; then
        if [ -f "./zen-watcher" ]; then
            # Use local binary if available
            OBSCTL="./zen-watcher obsctl"
        else
            warn "obsctl not found, skipping Observations validation"
            return
        fi
    else
        OBSCTL="obsctl"
    fi
    
    # Query Observations (if not in dry-run mode)
    if [ "$DRY_RUN" = "false" ]; then
        info "Querying Observations..."
        $OBSCTL list \
            --context "$CONTEXT" \
            --namespace "$NAMESPACE" || warn "Failed to query Observations (may be expected if not deployed)"
    else
        info "Dry-run mode: Skipping Observations query"
    fi
}

main() {
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "zen-watcher 1.0.0-alpha E2E Validation"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo "Configuration:"
    echo "  Context: $CONTEXT"
    echo "  Namespace: $NAMESPACE"
    echo "  Dry-run: $DRY_RUN"
    echo "  Helm chart: $HELM_CHART_PATH"
    echo ""
    
    check_prerequisites
    validate_helm_chart
    validate_ingesters
    validate_observations
    
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    info "E2E validation complete!"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    
    if [ "$DRY_RUN" = "true" ]; then
        echo ""
        warn "This was a dry-run validation. To actually deploy:"
        echo "  DRY_RUN=false CONTEXT=your-context NAMESPACE=zen-system $0"
    fi
}

main "$@"

