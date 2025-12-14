#!/bin/bash
#
# CI Build Script - Build and push Docker image
# Usage: ./scripts/ci-build.sh [version]
#        If version not provided, uses git describe
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/utils/common.sh"

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🔨 zen-watcher CI: Build & Push"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Change to repo root
cd "$(dirname "$0")/.."

# Determine version
VERSION="${1:-}"
if [ -z "$VERSION" ]; then
    VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "1.0.19")
fi

IMAGE="kubezen/zen-watcher"

echo "📋 Build Configuration:"
echo "   Version: ${VERSION}"
echo "   Image:   ${IMAGE}"
echo ""

# Build using Makefile with resource limits
echo "🔨 Step 1: Building Docker Image"
echo "   Using resource limits: 1 CPU, best-effort I/O, nice priority"
echo "────────────────────────────────────────────────────"
run_limited make docker-build IMAGE_TAG="${VERSION}"
echo "  ✅ Image built: ${IMAGE}:${VERSION}"
echo ""

# Push image with resource limits
echo "📤 Step 2: Pushing to Docker Hub"
echo "   Using resource limits: 1 CPU, best-effort I/O, nice priority"
echo "────────────────────────────────────────────────────"
run_limited docker push "${IMAGE}:${VERSION}"
run_limited docker push "${IMAGE}:latest"
echo "  ✅ Image pushed: ${IMAGE}:${VERSION}"
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✅ Build complete!"
echo "   Image: ${IMAGE}:${VERSION}"
echo "   Latest: ${IMAGE}:latest"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Next: Run ./scripts/build-and-sign.sh ${VERSION} for security signing"

