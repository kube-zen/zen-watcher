# Versioning Strategy

## Overview

Zen-watcher uses semantic versioning with synchronized releases across components.

## Single Source of Truth

**Version is defined in the root `VERSION` file** and propagated to all components:
- Code version (main.go)
- Docker image tags
- Helm chart version and appVersion
- Git tags (with v-prefix)

## Version Sync Policy

**Starting v1.2.0:** All versions are synchronized from the `VERSION` file.

| Component | Version | Location |
|-----------|---------|----------|
| VERSION file | 1.2.0 | `VERSION` (root) |
| Docker Image | 1.2.0 | `kubezen/zen-watcher:1.2.0` |
| Helm Chart | 1.2.0 | `kube-zen/zen-watcher` (ArtifactHub) |
| Git Tag | v1.2.0 | `github.com:kube-zen/zen-watcher` |

## Versioning Contract

**v-prefix rule**: Git tags use the `v` prefix (e.g., `v1.2.0`), while all other references use the version number without prefix (e.g., `1.2.0`).

- **Git tags**: `v1.2.0`, `v1.2.1`, etc.
- **Docker images**: `kubezen/zen-watcher:1.2.0` (no v-prefix)
- **Helm charts**: `version: 1.2.0`, `appVersion: "1.2.0"` (no v-prefix)
- **Code**: `Version = "1.2.0"` (no v-prefix)

## Semantic Versioning

Format: `MAJOR.MINOR.PATCH`

### MAJOR (Breaking Changes)
- CRD schema changes that require migration
- Removed features or APIs
- Incompatible configuration changes

**Example:** 1.x → 2.0.0

### MINOR (New Features)
- New adapters or sources
- New CRDs (Ingester)
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

# 5. Update helm-charts repo (separate repository)
# Clone if needed: git clone https://github.com/kube-zen/helm-charts.git
cd helm-charts
# Update charts/zen-watcher/Chart.yaml version to ${VERSION}
git commit -am "chore: zen-watcher ${VERSION}"
git push origin main
# Chart will be available via: helm install zen-watcher kube-zen/zen-watcher

# 6. Create GitHub Release from tag
# 7. Update ArtifactHub (if published)
```

## Historical Versioning

**v1.0.0 - v1.0.19 (Pre-G010):**
- Image versions incremented independently
- Chart stayed at 1.0.x
- Version inconsistencies existed

**v1.2.0 (G010 - Version Alignment):**
- All versions synchronized from `VERSION` file
- Git tag `v1.2.0` ↔ image `1.2.0` ↔ chart `1.2.0` ↔ app `1.2.0`
- Single source of truth established

## Version Numbering for Dependent Components

### Helm Chart Values
```yaml
image:
  repository: kubezen/zen-watcher
  tag: "1.2.0"  # Synced with chart version

Chart.yaml:
  version: 1.2.0
  appVersion: "1.2.0"
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
A: Yes, starting v1.2.0, they're tested together as a unit and synchronized from the `VERSION` file.

**Q: What if I only want to update the chart (e.g., change replica count)?**  
A: Chart patches (1.2.0 → 1.2.1) are fine without image changes. But we'll keep the versions synced from `VERSION`.

**Q: How do I update the version?**  
A: Update the root `VERSION` file, then run the release script which propagates it to all components.

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

