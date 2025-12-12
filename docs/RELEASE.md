# Release Process

This document outlines the release process for zen-watcher, including coordination between the application repository and the Helm charts repository.

## Repository Separation

- **zen-watcher** (this repo): Application source code, operator logic, CRDs
- **helm-charts**: Helm chart packaging, deployment manifests, values files

## Release Checklist

When cutting a new release of zen-watcher:

### 1. Prepare Release in zen-watcher Repository

- [ ] Update `CHANGELOG.md` with release notes
- [ ] Update version in `go.mod` (if needed)
- [ ] Update version references in documentation
- [ ] Run tests: `go test ./...`
- [ ] Build and verify Docker image locally
- [ ] Create git tag: `git tag -a v1.0.X -m "Release v1.0.X"`
- [ ] Push tag: `git push origin v1.0.X`

### 2. Build and Push Docker Image

- [ ] Build Docker image: `docker build -f build/Dockerfile -t kubezen/zen-watcher:v1.0.X .`
- [ ] Tag as latest (if appropriate): `docker tag kubezen/zen-watcher:v1.0.X kubezen/zen-watcher:latest`
- [ ] Push image: `docker push kubezen/zen-watcher:v1.0.X`
- [ ] Push latest (if appropriate): `docker push kubezen/zen-watcher:latest`
- [ ] Sign image with Cosign (if enabled): `cosign sign kubezen/zen-watcher:v1.0.X`

### 3. Update Helm Chart Repository

- [ ] Clone helm-charts repository: `git clone https://github.com/kube-zen/helm-charts`
- [ ] Navigate to chart: `cd helm-charts/charts/zen-watcher`
- [ ] **Sync CRD** (if CRD was changed): `cd ../zen-watcher && make sync-crd-to-chart`
- [ ] Update `Chart.yaml`:
  - [ ] Set `version: "1.0.X"` (chart version)
  - [ ] Set `appVersion: "1.0.X"` (application version)
- [ ] Update `values.yaml` default image tag: `image.tag: "1.0.X"`
- [ ] Update chart README.md if there are breaking changes or new features
- [ ] Test chart: `helm lint .` and `helm install --dry-run zen-watcher .`
- [ ] Package chart: `helm package .`
- [ ] Update `index.yaml` in helm-charts root: `helm repo index .`
- [ ] Commit changes: `git commit -m "Update zen-watcher chart to v1.0.X"`
- [ ] Create PR or push directly (depending on workflow)

### 4. Version Mapping

**1:1 Mapping Pattern (Recommended):**

| App Tag | Chart Version | Image Tag | Notes |
|---------|---------------|-----------|-------|
| v1.0.0  | 1.0.0         | 1.0.0     | Initial release |
| v1.0.1  | 1.0.1         | 1.0.1     | Patch release |
| v1.1.0  | 1.1.0         | 1.1.0     | Minor release |

**Decoupled Pattern (if needed):**

If chart needs updates independent of app version:
- Document in `Chart.yaml` annotations
- Document in chart README.md
- Example: Chart 0.5.2 → deploys app v0.3.0

### 5. Documentation Updates

- [ ] Update version compatibility matrix in helm-charts README
- [ ] Update version references in zen-watcher README.md
- [ ] Update any version-specific documentation

### 6. GitHub Release

- [ ] Create GitHub release in zen-watcher repository
- [ ] Attach release notes from CHANGELOG.md
- [ ] Link to Helm chart release/PR

## Versioning Guidelines

### Semantic Versioning

- **MAJOR** (X.0.0): Breaking changes
- **MINOR** (0.X.0): New features, backward compatible
- **PATCH** (0.0.X): Bug fixes, backward compatible

### Chart Version vs App Version

- **Chart version**: Reflects chart changes (templates, values, dependencies)
- **App version**: Reflects application code changes
- **1:1 mapping**: Recommended for simplicity (chart version = app version)
- **Decoupled**: Use when chart needs updates independent of app

## Automation (Future)

Consider automating with your CI system (invoke scripts/ci/zen-demo-validate.sh):

```yaml
# On tag in zen-watcher repo:
# 1. Build and push Docker image
# 2. Open PR in helm-charts repo to bump version
# 3. Update index.yaml
```

## Emergency Releases

For critical security fixes:
1. Follow same process but expedite
2. Consider patch release (e.g., 1.0.0 → 1.0.1)
3. Update both repos simultaneously
4. Announce in security advisories

## Questions?

- See [CONTRIBUTING.md](CONTRIBUTING.md) for contribution guidelines
- See helm-charts repository for chart-specific questions

