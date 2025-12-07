# Docker Hub Cleanup Instructions

## Overview

This document provides instructions for cleaning up old Docker Hub images and setting up the new 1.0.0-alpha release.

## Prerequisites

1. Docker Hub account with access to `kubezen/zen-watcher` repository
2. Docker Hub CLI or web interface access
3. Authentication: `docker login`

## Steps

### 1. List Current Images

```bash
# Using Docker Hub API (requires token)
curl -H "Authorization: Bearer $DOCKERHUB_TOKEN" \
  "https://hub.docker.com/v2/repositories/kubezen/zen-watcher/tags?page_size=100" | jq '.results[].name'
```

Or use Docker Hub web interface:
- Go to https://hub.docker.com/r/kubezen/zen-watcher/tags
- View all tags

### 2. Delete Old Images

**Option A: Using Docker Hub Web Interface (Recommended)**
1. Navigate to https://hub.docker.com/r/kubezen/zen-watcher/tags
2. For each old tag (e.g., 1.0.19, 1.0.20, 1.0.22, 1.1.0, etc.):
   - Click the tag
   - Click "Delete" button
   - Confirm deletion

**Option B: Using Docker Hub API**

```bash
# Set your Docker Hub token
export DOCKERHUB_TOKEN="your-token-here"

# List of old tags to delete (update as needed)
OLD_TAGS=(
  "1.0.19"
  "1.0.20"
  "1.0.22"
  "1.1.0"
  # Add other old versions
)

# Delete each tag
for tag in "${OLD_TAGS[@]}"; do
  echo "Deleting kubezen/zen-watcher:${tag}..."
  curl -X DELETE \
    -H "Authorization: Bearer $DOCKERHUB_TOKEN" \
    "https://hub.docker.com/v2/repositories/kubezen/zen-watcher/tags/${tag}/"
done
```

**Note:** You cannot delete the `latest` tag directly if it's the only tag. First push the new version, then delete old ones.

### 3. Push New Image

```bash
# Build the image (already done)
docker build \
  --build-arg VERSION=1.0.0-alpha \
  --build-arg COMMIT=$(git rev-parse --short HEAD) \
  --build-arg BUILD_DATE=$(date -u '+%Y-%m-%dT%H:%M:%SZ') \
  -t kubezen/zen-watcher:1.0.0-alpha \
  -t kubezen/zen-watcher:latest \
  -f build/Dockerfile \
  .

# Login to Docker Hub
docker login

# Push both tags
docker push kubezen/zen-watcher:1.0.0-alpha
docker push kubezen/zen-watcher:latest
```

### 4. Verify

```bash
# Pull and verify
docker pull kubezen/zen-watcher:1.0.0-alpha
docker run --rm kubezen/zen-watcher:1.0.0-alpha --version

# Should show: 1.0.0-alpha
```

### 5. Update Latest Tag (if needed)

If `latest` doesn't point to 1.0.0-alpha:

```bash
# Tag the new image as latest
docker tag kubezen/zen-watcher:1.0.0-alpha kubezen/zen-watcher:latest
docker push kubezen/zen-watcher:latest
```

## Important Notes

1. **Backup**: Consider keeping at least one old version for rollback purposes (optional)
2. **Dependencies**: Check if any Helm charts or deployments reference old versions
3. **CI/CD**: Update any CI/CD pipelines that reference specific versions
4. **Documentation**: All documentation has been updated to reference 1.0.0-alpha

## Automated Script

A helper script is available at `scripts/docker-hub-cleanup.sh` (create if needed):

```bash
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

echo "📤 Pushing to Docker Hub..."
docker push ${IMAGE}:${VERSION}
docker push ${IMAGE}:latest

echo "✅ Done! Image ${IMAGE}:${VERSION} is now available"
echo "⚠️  Remember to delete old images via Docker Hub web interface"
```

