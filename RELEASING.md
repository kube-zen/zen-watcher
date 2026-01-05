# Release Process

This document describes the release process for zen-watcher.

## Release Types

### Pre-release (v0.x.x)

Pre-releases are used during the development phase:
- `v0.1.0-alpha`, `v0.2.0-alpha`, etc. for alpha releases
- `v0.1.0-beta`, `v0.2.0-beta`, etc. for beta releases
- Pre-releases may have breaking changes

### Stable Release (v1.0.0+)

Stable releases follow semantic versioning:
- **Major** (v1.0.0, v2.0.0): Breaking changes
- **Minor** (v1.1.0, v1.2.0): New features, backward compatible
- **Patch** (v1.0.1, v1.0.2): Bug fixes, backward compatible

## Release Checklist

### Before Release

- [ ] All tests pass (`make test`)
- [ ] Code is linted (`make lint`)
- [ ] Security checks pass (`make security-check`)
- [ ] Documentation is up-to-date
- [ ] CHANGELOG.md is updated with release notes
- [ ] Version numbers are updated in:
  - [ ] `VERSION` file
  - [ ] `go.mod` (if needed)
  - [ ] Documentation references
  - [ ] Helm chart (in helm-charts repository)
- [ ] Docker image builds successfully
- [ ] All CI checks pass

### Release Steps

1. **Update Version**
   ```bash
   # Update VERSION file
   echo "1.2.1" > VERSION
   
   # Update CHANGELOG.md with release notes
   # Update any version references in docs
   ```

2. **Create Release Branch** (optional, for major releases)
   ```bash
   git checkout -b release/v1.2.1
   ```

3. **Run Pre-release Checks**
   ```bash
   make test
   make lint
   make security-check
   ```

4. **Create Git Tag**
   ```bash
   git tag -a v1.2.1 -m "Release v1.2.1"
   git push origin v1.2.1
   ```

5. **Build and Push Docker Image**
   ```bash
   # Build image
   docker build -f build/Dockerfile -t kubezen/zen-watcher:v1.2.1 .
   
   # Push to registry
   docker push kubezen/zen-watcher:v1.2.1
   ```

6. **Update Helm Chart** (in helm-charts repository)
   - Update chart version
   - Update image tag
   - Update changelog
   - Create PR and merge

7. **Create GitHub Release**
   - Go to GitHub Releases page
   - Create new release from tag
   - Copy release notes from CHANGELOG.md
   - Publish release

8. **Post-Release**
   - [ ] Announce release (if major/minor)
   - [ ] Update documentation if needed
   - [ ] Monitor for issues

## Version Numbering

Follow [Semantic Versioning](https://semver.org/):
- **MAJOR**: Breaking changes
- **MINOR**: New features (backward compatible)
- **PATCH**: Bug fixes (backward compatible)

## Release Notes

Release notes should be written in CHANGELOG.md following the [Keep a Changelog](https://keepachangelog.com/) format.

For detailed release coordination with Helm charts, see [docs/RELEASE.md](docs/RELEASE.md).

## Emergency Releases

For critical security fixes:
1. Create hotfix branch from latest release tag
2. Apply fix
3. Increment patch version
4. Follow normal release process
5. Cherry-pick to main branch

## Resources

- [Semantic Versioning](https://semver.org/)
- [Keep a Changelog](https://keepachangelog.com/)
- [Release Documentation](docs/RELEASE.md) - Detailed release coordination guide

