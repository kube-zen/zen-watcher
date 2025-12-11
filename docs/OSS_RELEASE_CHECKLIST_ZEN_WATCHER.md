# OSS Release Checklist: zen-watcher

**Purpose**: Checklist for cutting an OSS release of zen-watcher without rediscovering the process.

**Last Updated**: 2025-12-10

---

## Preconditions

### 1. CRDs Validated

- [ ] All CRD schemas validated against current examples
- [ ] `examples/observations/*.yaml` all validate against CRD schema
- [ ] Run validation: `kubectl apply --dry-run=client -f examples/observations/*.yaml`

### 2. Examples Current

- [ ] `examples/observations/` directory contains canonical examples
- [ ] Examples cover all categories (security, compliance, performance, operations, cost)
- [ ] Examples match current CRD schema
- [ ] `examples/observations/README.md` is up to date

### 3. Public API Guide Updated

- [ ] `docs/OBSERVATION_API_PUBLIC_GUIDE.md` reflects current API
- [ ] Breaking changes (if any) are documented
- [ ] Compatibility guarantees are clear

### 4. Branding Audit Clean

- [ ] `docs/BRANDING_DECOUPLING_AUDIT.md` reviewed
- [ ] Code-level branding scan complete (see audit doc "Code-Level Branding Scan" section)
- [ ] No user-facing log/error messages force kube-zen/zen-hook as only mental model
- [ ] Documentation neutralized (zen-hook references are examples, not requirements)

---

## Release Steps

### 1. Update Release Notes

- [ ] Copy `docs/releases/NEXT_RELEASE_NOTES.md` to `docs/releases/v<version>-RELEASE_NOTES.md`
- [ ] Update version number in release notes file
- [ ] Update release date
- [ ] Finalize all sections (Summary, Breaking Changes, New Features, etc.)
- [ ] Add links to relevant documentation (KEP, API guide, examples)

### 2. Verify Quick-Demo / Getting-Started Flows

- [ ] Run `./scripts/quick-demo.sh k3d --non-interactive --deploy-mock-data` on clean cluster
- [ ] Verify all 9 sources create observations successfully
- [ ] Test `docs/GETTING_STARTED_GENERIC.md` flow on clean cluster
- [ ] Verify Helm chart installation works: `helm install zen-watcher kube-zen/zen-watcher`
- [ ] Verify CRD installation: `kubectl get crd observations.zen.kube-zen.io`

### 3. Tag and Push

- [ ] Update version in relevant files (if version is tracked in code):
  - [ ] `go.mod` (if version module is used)
  - [ ] Helm chart version (in helm-charts repo)
- [ ] Create git tag: `git tag -a v<version> -m "Release v<version>"`
- [ ] Push tag: `git push origin v<version>`
- [ ] Push main branch: `git push origin main`

### 4. Helm Chart Release (if applicable)

- [ ] Update Helm chart version in `helm-charts` repository
- [ ] Update chart `appVersion` to match release tag
- [ ] Test chart installation: `helm install zen-watcher kube-zen/zen-watcher --version <version>`
- [ ] Push chart release to Helm repository

---

## Post-Release

### 1. Update Roadmap

- [ ] Update `docs/PM_AI_ROADMAP.md` (if exists) with release completion
- [ ] Mark completed items in roadmap
- [ ] Add any new backlog items discovered during release

### 2. Update KEP Draft (if API changes)

- [ ] If release includes API changes, update `docs/KEP_DRAFT_ZEN_WATCHER_OBSERVATIONS.md`
- [ ] Mark implemented features as complete
- [ ] Update versioning plan if needed

### 3. Update Documentation Index

- [ ] Update `DOCUMENTATION_INDEX.md` (if exists) with new release notes link
- [ ] Update any version-specific documentation references

### 4. Reset NEXT_RELEASE_NOTES.md

- [ ] Clear `docs/releases/NEXT_RELEASE_NOTES.md` for next release
- [ ] Or update with next planned release items

---

## Versioning Reference

See `docs/OBSERVATION_VERSIONING_AND_RELEASE_PLAN.md` for:
- Version numbering strategy (v1alpha2 → v1beta1 → v2)
- Compatibility policy (alpha vs beta vs stable)
- Breaking change procedures

---

## Release Notes Template

See `docs/RELEASE_NOTES_TEMPLATE.md` for standard release notes structure.

---

## Related Documentation

- **Versioning Plan**: `docs/OBSERVATION_VERSIONING_AND_RELEASE_PLAN.md`
- **KEP Draft**: `docs/KEP_DRAFT_ZEN_WATCHER_OBSERVATIONS.md`
- **Branding Audit**: `docs/BRANDING_DECOUPLING_AUDIT.md`
- **API Guide**: `docs/OBSERVATION_API_PUBLIC_GUIDE.md`
- **Examples**: `examples/observations/`

---

## Quick Reference

```bash
# 1. Validate examples
kubectl apply --dry-run=client -f examples/observations/*.yaml

# 2. Test quick-demo
./scripts/quick-demo.sh k3d --non-interactive --deploy-mock-data

# 3. Create and push tag
git tag -a v<version> -m "Release v<version>"
git push origin v<version>

# 4. Verify Helm chart (in helm-charts repo)
helm install zen-watcher kube-zen/zen-watcher --version <version>
```

---

**Note**: This checklist assumes you're working in the zen-watcher repository. For helm-charts repository releases, see that repository's release process.
