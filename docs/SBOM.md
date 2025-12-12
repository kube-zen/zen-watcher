# Software Bill of Materials (SBOM)

## Overview

A Software Bill of Materials (SBOM) is a complete, formally structured list of components, libraries, and modules required to build software. Zen Watcher provides SBOMs for supply chain security and compliance.

## Why SBOM?

- **Transparency**: Know what's in your container
- **Security**: Identify vulnerable components
- **Compliance**: Meet regulatory requirements
- **Trust**: Verify supply chain integrity

## SBOM Generation

### Using Syft

Syft generates SBOMs for container images:

```bash
# Install Syft
curl -sSfL https://raw.githubusercontent.com/anchore/syft/main/install.sh | sh

# Generate SBOM in SPDX format
syft zubezen/zen-watcher:1.0.0 -o spdx-json > sbom.spdx.json

# Generate SBOM in CycloneDX format
syft zubezen/zen-watcher:1.0.0 -o cyclonedx-json > sbom.cyclonedx.json

# Generate SBOM in Syft native format
syft zubezen/zen-watcher:1.0.0 -o json > sbom.syft.json
```

### Using Docker SBOM

Docker Desktop includes SBOM generation:

```bash
# Generate SBOM
docker sbom zubezen/zen-watcher:1.0.0 > sbom.spdx.json
```

### During Build

Include SBOM generation in your CI/CD (invoke from your CI system or scheduled job):

```bash
# Generate SBOM during build
syft zubezen/zen-watcher:${IMAGE_TAG} -o spdx-json > sbom.spdx.json
syft zubezen/zen-watcher:${IMAGE_TAG} -o cyclonedx-json > sbom.cyclonedx.json

# Upload SBOM (adapt to your CI system's artifact upload mechanism)
# Example: Store in artifact repository, attach to release, or publish to OCI registry
```

## SBOM Formats

### SPDX (Software Package Data Exchange)

Industry standard, ISO/IEC 5962:2021:

```bash
syft zubezen/zen-watcher:1.0.0 -o spdx-json
```

### CycloneDX

OWASP standard for SBOM and VEX:

```bash
syft zubezen/zen-watcher:1.0.0 -o cyclonedx-json
```

## Vulnerability Scanning with SBOM

### Using Grype

Scan SBOM for vulnerabilities:

```bash
# Install Grype
curl -sSfL https://raw.githubusercontent.com/anchore/grype/main/install.sh | sh

# Scan SBOM
grype sbom:sbom.spdx.json

# Output formats
grype sbom:sbom.spdx.json -o table
grype sbom:sbom.spdx.json -o json
grype sbom:sbom.spdx.json -o sarif
```

### Using Trivy

```bash
# Scan with Trivy
trivy sbom sbom.spdx.json

# Scan image directly
trivy image zubezen/zen-watcher:1.0.0
```

## SBOM Attestation

### Attach SBOM to Image

```bash
# Generate SBOM
syft zubezen/zen-watcher:1.0.0 -o spdx-json > sbom.spdx.json

# Attach as attestation with Cosign
cosign attest --predicate sbom.spdx.json \
  --key cosign.key \
  zubezen/zen-watcher:1.0.0
```

### Verify SBOM Attestation

```bash
# Verify and retrieve SBOM
cosign verify-attestation \
  --key cosign.pub \
  zubezen/zen-watcher:1.0.0 | jq -r .payload | base64 -d | jq .predicate
```

## SBOM in CI/CD

### CI Integration

Invoke SBOM generation from your CI system or scheduled job:

```bash
# Generate SBOM
syft zubezen/zen-watcher:${IMAGE_TAG} -o spdx-json > sbom.spdx.json
syft zubezen/zen-watcher:${IMAGE_TAG} -o cyclonedx-json > sbom.cyclonedx.json

# Scan for vulnerabilities
grype sbom:sbom.spdx.json --fail-on critical

# Attach SBOM attestation
cosign attest --predicate sbom.spdx.json \
  --key ${COSIGN_KEY} \
  zubezen/zen-watcher:${IMAGE_TAG}

# Upload SBOM (adapt to your CI system's artifact upload mechanism)
```

### GitLab CI

```yaml
sbom:
  stage: security
  image: anchore/syft:latest
  script:
    - syft $CI_REGISTRY_IMAGE:$CI_COMMIT_TAG -o spdx-json > sbom.spdx.json
    - syft $CI_REGISTRY_IMAGE:$CI_COMMIT_TAG -o cyclonedx-json > sbom.cyclonedx.json
  artifacts:
    paths:
      - sbom.*.json
    expire_in: 1 year

scan:
  stage: security
  image: anchore/grype:latest
  dependencies:
    - sbom
  script:
    - grype sbom:sbom.spdx.json --fail-on critical
```

## SBOM Storage and Distribution

### Attach to Release

```bash
# GitHub Release
gh release create v1.0.0 \
  --title "Release v1.0.0" \
  --notes "Release notes here" \
  sbom.spdx.json \
  sbom.cyclonedx.json
```

### OCI Registry

Store SBOM in OCI registry:

```bash
# Using ORAS
oras push ghcr.io/your-org/zen-watcher-sbom:1.0.0 \
  --artifact-type application/spdx+json \
  sbom.spdx.json
```

### Dependency Track

Upload to Dependency Track for monitoring:

```bash
# Upload to Dependency Track
curl -X "POST" "https://dependency-track.example.com/api/v1/bom" \
  -H "X-Api-Key: $API_KEY" \
  -H "Content-Type: multipart/form-data" \
  -F "project=$PROJECT_UUID" \
  -F "bom=@sbom.cyclonedx.json"
```

## SBOM Compliance

### Executive Order 14028

US Executive Order on Cybersecurity requires SBOM for federal software:

- ✅ Machine-readable format (SPDX/CycloneDX)
- ✅ Comprehensive component list
- ✅ Automated generation
- ✅ Cryptographically signed

### NTIA Minimum Elements

Compliant with NTIA minimum elements:

- ✅ Supplier name
- ✅ Component name
- ✅ Version
- ✅ Unique identifier
- ✅ Dependency relationships
- ✅ Author
- ✅ Timestamp

## Example SBOM Structure

```json
{
  "SPDXID": "SPDXRef-DOCUMENT",
  "spdxVersion": "SPDX-2.3",
  "creationInfo": {
    "created": "2024-11-04T00:00:00Z",
    "creators": ["Tool: syft-0.98.0"]
  },
  "name": "zubezen/zen-watcher:1.0.0",
  "dataLicense": "CC0-1.0",
  "packages": [
    {
      "SPDXID": "SPDXRef-Package-golang",
      "name": "golang",
      "versionInfo": "1.23.0",
      "supplier": "Organization: Google",
      "downloadLocation": "https://go.dev/dl/",
      "filesAnalyzed": false,
      "licenseConcluded": "BSD-3-Clause"
    }
  ],
  "relationships": [
    {
      "spdxElementId": "SPDXRef-DOCUMENT",
      "relationshipType": "DESCRIBES",
      "relatedSpdxElement": "SPDXRef-Package-golang"
    }
  ]
}
```

## Best Practices

1. **Generate for Every Build**
   - Automate SBOM generation in CI/CD
   - Store with artifacts

2. **Sign SBOMs**
   - Cryptographically sign with Cosign
   - Verify before use

3. **Regular Scanning**
   - Scan SBOMs for vulnerabilities
   - Set up automated alerts

4. **Version Control**
   - Store SBOMs in version control
   - Track changes over time

5. **Share with Customers**
   - Provide SBOMs to users
   - Enable their security processes

## Tools

- **Generation**: Syft, Docker SBOM, Trivy
- **Scanning**: Grype, Trivy, Snyk
- **Management**: Dependency Track
- **Signing**: Cosign
- **Storage**: ORAS, OCI registries

## Resources

- [NTIA SBOM Minimum Elements](https://www.ntia.gov/files/ntia/publications/sbom_minimum_elements_report.pdf)
- [SPDX Specification](https://spdx.github.io/spdx-spec/)
- [CycloneDX Specification](https://cyclonedx.org/specification/overview/)
- [Syft Documentation](https://github.com/anchore/syft)
- [Grype Documentation](https://github.com/anchore/grype)

## Contact

For SBOM-related questions: sbom@kube-zen.com


