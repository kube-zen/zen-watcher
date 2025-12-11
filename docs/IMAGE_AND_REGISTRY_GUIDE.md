# Image and Registry Guide

This guide covers building, tagging, and pushing zen-watcher container images.

## Building Locally

### Basic Build

```bash
# Build with default version (1.0.0-alpha)
make image

# Or use docker-build directly
make docker-build
```

### Custom Version/Tag

```bash
# Build with custom version
VERSION=1.0.0-alpha make image

# Build with custom image name
IMAGE_NAME=my-registry/zen-watcher make image
```

### Build Details

The Dockerfile uses:
- **Base image**: `golang:1.23-alpine` (builder stage)
- **Final image**: `gcr.io/distroless/static:nonroot` (minimal, secure)
- **Build args**: `VERSION`, `COMMIT`, `BUILD_DATE`

**Image characteristics:**
- Static binary (CGO_ENABLED=0)
- Stripped symbols (-w -s flags)
- Non-root user (uid=65532)
- Minimal attack surface (distroless)

## Tagging

### Recommended Tags

For **1.0.0-alpha release**:
- `1.0.0-alpha` - Release tag
- `latest` - Development tag (optional, for dev builds)

### Tag Examples

```bash
# Tag for release
docker tag kubezen/zen-watcher:1.0.0-alpha kubezen/zen-watcher:1.0.0-alpha

# Tag as latest (dev only)
docker tag kubezen/zen-watcher:1.0.0-alpha kubezen/zen-watcher:latest
```

## Pushing to Registry

### Using Make Target

```bash
# Set registry and tag
make image-push REGISTRY=your-registry.io TAG=1.0.0-alpha

# Example: Docker Hub
make image-push REGISTRY=docker.io TAG=1.0.0-alpha

# Example: Custom registry
make image-push REGISTRY=registry.example.com:5000 TAG=1.0.0-alpha
```

### Manual Push

```bash
# Tag for your registry
docker tag kubezen/zen-watcher:1.0.0-alpha your-registry.io/zen-watcher:1.0.0-alpha

# Push
docker push your-registry.io/zen-watcher:1.0.0-alpha
```

### Authentication

**Docker Hub:**
```bash
docker login
make image-push REGISTRY=docker.io TAG=1.0.0-alpha
```

**Custom Registry:**
```bash
docker login your-registry.io
make image-push REGISTRY=your-registry.io TAG=1.0.0-alpha
```

## CI/CD Integration

### GitHub Actions Example

```yaml
- name: Build and push
  env:
    REGISTRY: ghcr.io
    IMAGE_NAME: ${{ github.repository }}
  run: |
    make image-push REGISTRY=$REGISTRY TAG=${{ github.ref_name }}
```

### GitLab CI Example

```yaml
build:
  script:
    - make image-push REGISTRY=$CI_REGISTRY TAG=$CI_COMMIT_TAG
```

### Key Points

- **No hard-coded registries**: Always use environment variables or Makefile variables
- **Configurable**: Registry and tag should be configurable via env/CI variables
- **Security**: Use CI secrets for registry authentication

## Image Scanning

### Security Scan

```bash
# Scan for vulnerabilities (HIGH/CRITICAL only)
make docker-scan

# Or with Trivy directly
trivy image --severity HIGH,CRITICAL kubezen/zen-watcher:1.0.0-alpha
```

### SBOM Generation

```bash
# Generate SBOM
make docker-sbom

# Outputs:
# - zen-watcher-sbom.json (Syft format)
# - zen-watcher-sbom.spdx.json (SPDX format)
```

## Image Signing (Optional)

### Sign with Cosign

```bash
# Generate key pair (first time)
cosign generate-key-pair

# Sign image
make docker-sign

# Verify signature
make docker-verify
```

## Registry Layout Guidelines

### Recommended Structure

```
your-registry.io/
  └── zen-watcher/
      ├── 1.0.0-alpha
      ├── 1.0.0-beta
      ├── 1.0.0
      └── latest (dev only)
```

### Tagging Strategy

- **Release tags**: Use semantic versioning (`1.0.0-alpha`, `1.0.0`, `1.1.0`)
- **Dev tags**: Use `latest` or commit SHA for development builds
- **No mutable tags in production**: Avoid `latest` for production deployments

## Troubleshooting

### Build Fails

**Issue**: Build fails with "go mod download" errors

**Solution**: Ensure network access and valid `go.mod`:
```bash
go mod download
go mod verify
```

### Push Fails

**Issue**: "unauthorized" or "authentication required"

**Solution**: Login to registry first:
```bash
docker login your-registry.io
```

### Image Too Large

**Issue**: Image size is larger than expected

**Solution**: Verify distroless base and stripped binary:
```bash
docker images kubezen/zen-watcher:1.0.0-alpha
```

## Related Documentation

- [Dockerfile](../build/Dockerfile) - Image build configuration
- [Makefile](../Makefile) - Build targets and automation
- [Release Checklist](ZW_1_0_0_ALPHA_RELEASE_CHECKLIST.md) - Release validation steps

