#!/bin/bash
set -e

VERSION="1.0.0-alpha"
IMAGE="kubezen/zen-watcher"

echo "🔨 Building ${IMAGE}:${VERSION}..."
docker build \
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
echo ""

# Push version tag
docker push ${IMAGE}:${VERSION}

# Push latest tag
docker push ${IMAGE}:latest

echo ""
echo "✅ Done! Image ${IMAGE}:${VERSION} is now available on Docker Hub"
echo "   - ${IMAGE}:${VERSION}"
echo "   - ${IMAGE}:latest"
echo ""
echo "⚠️  Next steps:"
echo "   1. Delete old images via Docker Hub web interface:"
echo "      https://hub.docker.com/r/${IMAGE}/tags"
echo "   2. See DOCKER_HUB_CLEANUP.md for detailed cleanup instructions"

