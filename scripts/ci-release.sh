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
HELM_CHART_PATH="../helm-charts/charts/zen-watcher"

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
# Update Chart.yaml in helm-charts repo if accessible
if [ -d "$HELM_CHART_PATH" ]; then
    sed -i "s/^version: .*/version: ${VERSION}/" "${HELM_CHART_PATH}/Chart.yaml"
    sed -i "s/^  tag: .*/  tag: \"${VERSION}\"/" "${HELM_CHART_PATH}/values.yaml"
    echo "  ✅ Updated helm chart version"
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
echo "If helm-charts repo is separate, also:"
echo "  cd ${HELM_CHART_PATH}/.."
echo "  git commit -am 'chore: zen-watcher ${VERSION}'"
echo "  git push origin main"
echo ""
echo "Then verify:"
echo "  - Docker Hub: https://hub.docker.com/r/kubezen/zen-watcher/tags"
echo "  - GitHub Release: Create from tag v${VERSION}"
echo "  - Update ArtifactHub (if published)"
echo ""

