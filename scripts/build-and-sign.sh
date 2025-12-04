#!/bin/bash
set -e

VERSION="${1:-$(git describe --tags --always --dirty)}"
IMAGE="kubezen/zen-watcher"

echo "🔨 Building zen-watcher:${VERSION}..."

# Build multi-arch image
docker buildx build \
  -f build/Dockerfile \
  -t "${IMAGE}:${VERSION}" \
  -t "${IMAGE}:latest" \
  --platform linux/amd64,linux/arm64 \
  --push \
  .

echo "🔒 Scanning with Trivy..."
trivy image --severity HIGH,CRITICAL "${IMAGE}:${VERSION}" || {
  echo "⚠️  Trivy scan found vulnerabilities"
  exit 1
}

echo "📝 Generating SBOM..."
syft "${IMAGE}:${VERSION}" -o cyclonedx-json > sbom.json

echo "✍️  Signing with Cosign..."
cosign sign --yes "${IMAGE}:${VERSION}"

echo "📦 Generating attestation..."
cosign attest --yes --predicate sbom.json --type cyclonedx "${IMAGE}:${VERSION}"

echo "✅ Build complete and signed!"
echo "   Image: ${IMAGE}:${VERSION}"
echo "   Verify: cosign verify ${IMAGE}:${VERSION}"

