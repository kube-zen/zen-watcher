#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/utils/common.sh"

VERSION="${1:-$(git describe --tags --always --dirty)}"
IMAGE="kubezen/zen-watcher"

echo "🔨 Building zen-watcher:${VERSION}..."
echo "   Using resource limits: 1 CPU, best-effort I/O, nice priority"

# Build multi-arch image with resource limits
run_limited docker buildx build \
  -f build/Dockerfile \
  -t "${IMAGE}:${VERSION}" \
  -t "${IMAGE}:latest" \
  --platform linux/amd64,linux/arm64 \
  --push \
  .

echo "🔒 Scanning with Trivy..."
run_limited trivy image --severity HIGH,CRITICAL "${IMAGE}:${VERSION}" || {
  echo "⚠️  Trivy scan found vulnerabilities"
  exit 1
}

echo "📝 Generating SBOM..."
run_limited syft "${IMAGE}:${VERSION}" -o cyclonedx-json > sbom.json

echo "✍️  Signing with Cosign..."
run_limited cosign sign --yes "${IMAGE}:${VERSION}"

echo "📦 Generating attestation..."
run_limited cosign attest --yes --predicate sbom.json --type cyclonedx "${IMAGE}:${VERSION}"

echo "✅ Build complete and signed!"
echo "   Image: ${IMAGE}:${VERSION}"
echo "   Verify: cosign verify ${IMAGE}:${VERSION}"

