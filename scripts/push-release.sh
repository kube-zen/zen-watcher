#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/utils/common.sh"

VERSION="1.0.0-alpha"
IMAGE="kubezen/zen-watcher"

echo "🔨 Building ${IMAGE}:${VERSION}..."
echo "   Using resource limits: 1 CPU, best-effort I/O, nice priority"
run_limited docker build \
  --build-arg VERSION=${VERSION} \
  --build-arg COMMIT=$(git rev-parse --short HEAD) \
  --build-arg BUILD_DATE=$(date -u '+%Y-%m-%dT%H:%M:%SZ') \
  -t ${IMAGE}:${VERSION} \
  -t ${IMAGE}:latest \
  -f build/Dockerfile \
  .

echo ""
echo "📤 Pushing to Docker Hub..."
echo "   Make sure you're logged in: docker login"
echo "   Using resource limits: 1 CPU, best-effort I/O, nice priority"
echo ""

# Push version tag
run_limited docker push ${IMAGE}:${VERSION}

# Push latest tag
run_limited docker push ${IMAGE}:latest

echo ""
echo "✅ Done! Image ${IMAGE}:${VERSION} is now available on Docker Hub"
echo "   - ${IMAGE}:${VERSION}"
echo "   - ${IMAGE}:latest"
echo ""
echo "⚠️  Next steps:"
echo "   1. Delete old images via Docker Hub web interface:"
echo "      https://hub.docker.com/r/${IMAGE}/tags"
echo "   2. See DOCKER_HUB_CLEANUP.md for detailed cleanup instructions"

