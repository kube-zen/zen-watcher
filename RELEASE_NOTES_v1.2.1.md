# Release Notes: Zen Watcher v1.2.1

**Version**: v1.2.1  
**Release Date**: 2025-01-25  
**Status**: Final

---

## Summary

Zen Watcher v1.2.1 is a **patch release** that fixes critical Helm chart issues, improves security defaults, and updates documentation for OSS launch readiness.

**Key Highlights:**
- 🔒 **Security Hardening**: NetworkPolicy now requires explicit destinations (no silent broad CIDRs)
- 🔧 **Helm Chart Fixes**: CRDs properly shipped, critical ops controls exposed
- 📚 **Documentation Updates**: All version references updated to 1.2.1
- 🛠️ **CI Improvements**: Version fallback now reads from VERSION file

---

## Fixed

### Helm Chart

- **NetworkPolicy Security**: Fixed NetworkPolicy to require explicit `kubernetesServiceIP` and/or `kubernetesAPICIDRs[]` when `egress.enabled=true` and `allowKubernetesAPI=true`. Previously, it silently defaulted to broad CIDRs, which is unsafe for community deployments.
- **CRD Installation**: CRDs are now properly shipped and installable via Helm with `crds.enabled: true` by default. Templates now correctly read from `files/crds/*`.
- **Deployment Template**: Now properly renders critical hardening controls:
  - `extraEnv` for custom environment variables
  - Webhook auth envs (`WEBHOOK_AUTH_DISABLED`, `WEBHOOK_AUTH_TOKEN`, `WEBHOOK_ALLOWED_IPS`)
  - Retention/GC knobs (`OBSERVATION_TTL_*`, `GC_INTERVAL`, `GC_TIMEOUT`)

### Documentation

- **Version References**: Updated all documentation from 1.0.0-alpha/1.2.0 to 1.2.1:
  - `docs/DEVELOPER_GUIDE.md`
  - `docs/DEPLOYMENT_HELM.md`
  - `docs/E2E_VALIDATION_GUIDE.md`
  - `docs/IMAGE_AND_REGISTRY_GUIDE.md`
  - `docs/VERSIONING.md`
  - `examples/ingesters/README.md`
  - `examples/ingesters/TESTING.md`

### CI/CD

- **Version Fallback**: Fixed `scripts/ci-build.sh` to read from `VERSION` file instead of hardcoded fallback to `1.0.19`. Now fails loudly if VERSION file is missing or empty.

---

## Changed

### Helm Chart

- **NetworkPolicy Defaults**: Default remains `egress.enabled=false` for safer community posture. When enabled, explicit destinations are now required.

---

## Compatibility

- **zen-sdk**: zen-watcher 1.2.1 requires zen-sdk v0.2.9-alpha
- **Kubernetes**: 1.26+
- **Helm**: 3.8+

---

## Migration from v1.2.0

No breaking changes. This is a patch release with bug fixes and documentation updates.

If you're using NetworkPolicy with egress enabled, you may need to add explicit destinations:

```yaml
# values.yaml
networkPolicy:
  egress:
    enabled: true
    allowKubernetesAPI: true
    kubernetesServiceIP: "10.96.0.0/12"  # Required
    kubernetesAPICIDRs:                   # Required
      - "10.0.0.0/8"
```

---

## Known Issues

None.

---

## Credits

Thanks to all contributors and the community for feedback and testing.

---

**Full Changelog**: [CHANGELOG.md](CHANGELOG.md)

