#!/usr/bin/env bash
# Build and push stress test Docker image
# Usage: ./build.sh [--push]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

cd "${SCRIPT_DIR}"

IMAGE_NAME="kubezen/zen-watcher-stress-test"
IMAGE_TAG="latest"
FULL_IMAGE="${IMAGE_NAME}:${IMAGE_TAG}"

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Building stress test image"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Image: ${FULL_IMAGE}"
echo ""

# Build the image
echo "Building Docker image..."
export DOCKER_BUILDKIT=1

if docker buildx build \
    --platform linux/amd64 \
    -f Dockerfile \
    -t "${FULL_IMAGE}" \
    --load \
    .; then
    echo ""
    echo "✓ Build successful: ${FULL_IMAGE}"
    docker images "${FULL_IMAGE}" --format "  Size: {{.Size}}"
else
    echo ""
    echo "✗ Build failed"
    exit 1
fi

# Push if requested
if [[ "${1:-}" == "--push" ]]; then
    echo ""
    echo "Pushing to Docker Hub..."
    if docker push "${FULL_IMAGE}"; then
        echo "✓ Push successful: ${FULL_IMAGE}"
    else
        echo "✗ Push failed"
        exit 1
    fi
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Usage:"
echo "  docker run --rm -v ~/.kube:/root/.kube:ro ${FULL_IMAGE} -h"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

