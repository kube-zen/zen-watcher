# Versioning Strategy

## Overview

Zen-watcher uses semantic versioning with synchronized releases across components.

## Version Sync Policy

**Starting v1.1.0:** Image and Helm chart versions are synchronized.

| Component | Version | Location |
|-----------|---------|----------|
| Docker Image | 1.0.0-alpha | `kubezen/zen-watcher:1.0.0-alpha` |
| Helm Chart | 1.1.0 | `kube-zen/helm-charts/charts/zen-watcher` |
| Git Tag | v1.1.0 | `github.com:kube-zen/zen-watcher` |

## Semantic Versioning

Format: `MAJOR.MINOR.PATCH`

### MAJOR (Breaking Changes)
- CRD schema changes that require migration
- Removed features or APIs
- Incompatible configuration changes

**Example:** 1.x → 2.0.0

### MINOR (New Features)
- New adapters or sources
- New CRDs (Ingester, ObservationMapping)
- New features (backward compatible)
- Significant enhancements

**Example:** 1.0.x → 1.1.0

### PATCH (Bug Fixes)
- Bug fixes
- Security patches
- Performance improvements
- Documentation updates

**Example:** 1.1.0 → 1.1.1

## Release Process

```bash
# 1. Decide version
VERSION=1.1.0

# 2. Run release script
./scripts/ci-release.sh $VERSION

# 3. Script will:
#    - Run all tests
#    - Build and push image
#    - Update chart version
#    - Create git tag
#    - Sign image (if tools available)

# 4. Manual steps:
git push origin main
git push origin v${VERSION}

# 5. Update helm-charts repo
cd ../helm-charts
git commit -am "chore: zen-watcher ${VERSION}"
git push origin main

# 6. Create GitHub Release from tag
# 7. Update ArtifactHub (if published)
```

## Historical Versioning (Pre-1.1.0)

**v1.0.0 - v1.0.19 (Image):**
- Image versions incremented independently
- Chart stayed at 1.0.x

**v1.0.10 (Chart):**
- Major Helm chart update with CRDs as templates
- Corresponds roughly to image 1.0.19

**v1.1.0+ (Synced):**
- Both image and chart use same version
- Easier to track and communicate

## Version Numbering for Dependent Components

### Helm Chart Values
```yaml
image:
  repository: kubezen/zen-watcher
  tag: "1.1.0"  # Synced with chart version

Chart.yaml:
  version: 1.1.0
  appVersion: "1.1.0"
```

### CI/CD
```bash
# Automated in ci-release.sh
sed -i "s/^version: .*/version: ${VERSION}/" Chart.yaml
sed -i "s/^  tag: .*/  tag: \"${VERSION}\"/" values.yaml
```

## FAQ

**Q: Why did you have different versions before?**  
A: Image was iterated quickly during development while chart was more stable. Now synced for clarity.

**Q: Do I need to upgrade both image and chart together?**  
A: Yes, starting v1.1.0, they're tested together as a unit.

**Q: What if I only want to update the chart (e.g., change replica count)?**  
A: Chart patches (1.1.0 → 1.1.1) are fine without image changes. But we'll keep the versions synced.

## Checking Versions

```bash
# Image version
kubectl get deployment zen-watcher -n zen-system -o jsonpath='{.spec.template.spec.containers[0].image}'

# Chart version
helm list -n zen-system

# Git tag
git describe --tags

# App version at runtime
kubectl logs -n zen-system deployment/zen-watcher | head -1 | grep "version"
```

## References

- [CHANGELOG.md](CHANGELOG.md) - Detailed change history
- [Semantic Versioning](https://semver.org/)
- [Helm Chart Versioning](https://helm.sh/docs/topics/charts/#charts-and-versioning)

