#!/bin/bash
#
# CI Release Script - Complete release flow
# Usage: ./scripts/ci-release.sh <version>
#        Example: ./scripts/ci-release.sh 1.0.20
#
set -euo pipefail

if [ $# -eq 0 ]; then
    echo "❌ Error: Version required"
    echo "Usage: ./scripts/ci-release.sh <version>"
    echo "Example: ./scripts/ci-release.sh 1.0.20"
    exit 1
fi

VERSION="$1"
IMAGE="kubezen/zen-watcher"
# Helm charts are in separate repository: https://github.com/kube-zen/helm-charts
# To update chart versions, clone the repo and set CHARTS_REPO environment variable
CHARTS_REPO="${CHARTS_REPO:-}"
HELM_CHART_PATH="${CHARTS_REPO:+${CHARTS_REPO}/charts/zen-watcher}"

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🚀 zen-watcher CI: Release ${VERSION}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Change to repo root
cd "$(dirname "$0")/.."

# Verify clean working directory
if [[ -n $(git status --porcelain) ]]; then
    echo "❌ Error: Working directory not clean"
    echo "   Commit or stash changes before release"
    git status --short
    exit 1
fi

echo "📋 Release Plan:"
echo "   Version:    ${VERSION}"
echo "   Image:      ${IMAGE}:${VERSION}"
echo "   Helm Chart: ${VERSION}"
echo ""

# Step 1: Run tests
echo "🧪 Step 1: Running Test Suite"
echo "────────────────────────────────────────────────────"
./scripts/ci-test.sh || {
    echo "❌ Tests failed - aborting release"
    exit 1
}
echo ""

# Step 2: Update version in code
echo "📝 Step 2: Updating Version References"
echo "────────────────────────────────────────────────────"
# Update Makefile VERSION if it exists
if [ -f "Makefile" ]; then
    sed -i "s/^VERSION ?= .*/VERSION ?= ${VERSION}/" Makefile 2>/dev/null || true
fi
# Update Helm chart in helm-charts repo if accessible
# Requires: CHARTS_REPO environment variable pointing to cloned helm-charts repo
# Clone with: git clone https://github.com/kube-zen/helm-charts.git
if [ -n "$CHARTS_REPO" ] && [ -d "$CHARTS_REPO" ]; then
    echo "  → Updating Helm chart using automation script..."
    if [ -f "$CHARTS_REPO/scripts/release/update-chart-version.sh" ]; then
        # Use automated script (packages chart, updates index.yaml)
        export ZEN_WATCHER_ROOT="$(dirname "$0")/.."
        cd "$CHARTS_REPO"
        if ./scripts/release/update-chart-version.sh "$VERSION"; then
            echo "  ✅ Updated helm chart (version, package, index.yaml)"
            cd - > /dev/null
        else
            echo "  ⚠️  Helm chart update failed, continuing..."
            cd - > /dev/null
        fi
    else
        # Fallback to manual update (legacy)
        if [ -d "$HELM_CHART_PATH" ]; then
            sed -i "s/^version: .*/version: ${VERSION}/" "${HELM_CHART_PATH}/Chart.yaml"
            sed -i "s/^appVersion:.*/appVersion: \"${VERSION}\"/" "${HELM_CHART_PATH}/Chart.yaml"
            sed -i "s/^  tag: .*/  tag: \"${VERSION}\"/" "${HELM_CHART_PATH}/values.yaml"
            echo "  ✅ Updated helm chart version (manual)"
        fi
    fi
elif [ -z "$CHARTS_REPO" ]; then
    echo "  ℹ️  CHARTS_REPO not set - skipping helm chart version update"
    echo "     To update chart: git clone https://github.com/kube-zen/helm-charts.git"
    echo "     Then set: export CHARTS_REPO=/path/to/helm-charts"
fi
echo "  ✅ Version references updated"
echo ""

# Step 3: Build and push image
echo "🔨 Step 3: Building Image"
echo "────────────────────────────────────────────────────"
./scripts/ci-build.sh "${VERSION}" || {
    echo "❌ Build failed - aborting release"
    exit 1
}
echo ""

# Step 4: Sign image (if tools available)
echo "🔒 Step 4: Security Signing"
echo "────────────────────────────────────────────────────"
if command -v cosign >/dev/null 2>&1 && command -v syft >/dev/null 2>&1; then
    ./scripts/build-and-sign.sh "${VERSION}" || {
        echo "⚠️  Signing failed - continuing without signature"
    }
else
    echo "  ⚠️  cosign/syft not installed, skipping signing"
    echo "     Install: https://github.com/sigstore/cosign"
fi
echo ""

# Step 5: Create git tag
echo "🏷️  Step 5: Creating Git Tag"
echo "────────────────────────────────────────────────────"
git tag -a "v${VERSION}" -m "Release v${VERSION}

See CHANGELOG.md for details."
echo "  ✅ Created tag: v${VERSION}"
echo ""

# Step 6: Update CHANGELOG
echo "📋 Step 6: Updating CHANGELOG"
echo "────────────────────────────────────────────────────"
if [ ! -f "CHANGELOG.md" ]; then
    echo "  ⚠️  CHANGELOG.md not found - please create it"
else
    echo "  ℹ️  Remember to update CHANGELOG.md with release notes"
fi
echo ""

# Step 7: Commit version bump
echo "💾 Step 7: Committing Version Bump"
echo "────────────────────────────────────────────────────"
git add -A
if [[ -n $(git status --porcelain) ]]; then
    git commit -m "chore: bump version to ${VERSION}"
    echo "  ✅ Version bump committed"
else
    echo "  ℹ️  No changes to commit"
fi
echo ""

# Step 8: Push (with confirmation)
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🎯 Release Ready"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "To complete the release, run:"
echo "  git push origin main"
echo "  git push origin v${VERSION}"
echo ""
echo "To update helm-charts repo:"
echo "  git clone https://github.com/kube-zen/helm-charts.git"
echo "  cd helm-charts"
echo "  # Update charts/zen-watcher/Chart.yaml version to ${VERSION}"
echo "  git commit -am 'chore: zen-watcher ${VERSION}'"
echo "  git push origin main"
echo ""
echo "Then verify:"
echo "  - Docker Hub: https://hub.docker.com/r/kubezen/zen-watcher/tags"
echo "  - GitHub Release: Create from tag v${VERSION}"
echo "  - Update ArtifactHub (if published)"
echo ""

