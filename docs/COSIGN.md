# Image Signing with Cosign

## Overview

Zen Watcher uses Cosign for signing and verifying container images, ensuring supply chain security and image integrity.

## Why Sign Images?

- **Integrity**: Verify images haven't been tampered with
- **Authenticity**: Confirm images come from trusted sources
- **Compliance**: Meet regulatory requirements
- **Supply Chain Security**: Protect against supply chain attacks

## Prerequisites

```bash
# Install Cosign
brew install cosign  # macOS

# Or download from GitHub
wget https://github.com/sigstore/cosign/releases/latest/download/cosign-linux-amd64
chmod +x cosign-linux-amd64
sudo mv cosign-linux-amd64 /usr/local/bin/cosign
```

## Key Generation

### Generate Key Pair

```bash
# Generate key pair (you'll be prompted for a password)
cosign generate-key-pair

# This creates:
# - cosign.key (private key - keep secret!)
# - cosign.pub (public key - share this)
```

### Keyless Signing (Sigstore)

Use keyless signing with Sigstore:

```bash
# Sign without keys (uses OIDC)
cosign sign corbe/zen-watcher:1.0.0

# Verify keyless signature
cosign verify \
  --certificate-identity=user@example.com \
  --certificate-oidc-issuer=https://github.com/login/oauth \
  corbe/zen-watcher:1.0.0
```

## Signing Images

### Sign with Key

```bash
# Sign image
cosign sign --key cosign.key corbe/zen-watcher:1.0.0

# Sign with annotations
cosign sign --key cosign.key \
  -a git-sha=$(git rev-parse HEAD) \
  -a build-date=$(date -u +%Y-%m-%dT%H:%M:%SZ) \
  corbe/zen-watcher:1.0.0
```

### Sign Multiple Tags

```bash
# Sign all tags
for tag in 1.0.0 1.0 latest; do
  cosign sign --key cosign.key corbe/zen-watcher:$tag
done
```

### Sign in CI/CD

#### GitHub Actions

```yaml
name: Sign Image

on:
  push:
    tags:
      - 'v*'

jobs:
  sign:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write
      id-token: write  # For keyless signing
    
    steps:
      - name: Install Cosign
        uses: sigstore/cosign-installer@v3
      
      - name: Sign with key
        env:
          COSIGN_KEY: ${{ secrets.COSIGN_KEY }}
          COSIGN_PASSWORD: ${{ secrets.COSIGN_PASSWORD }}
        run: |
          echo "$COSIGN_KEY" > cosign.key
          cosign sign --key cosign.key \
            -a git-sha=${{ github.sha }} \
            -a build-date=$(date -u +%Y-%m-%dT%H:%M:%SZ) \
            -a version=${{ github.ref_name }} \
            corbe/zen-watcher:${{ github.ref_name }}
      
      - name: Sign keyless (Sigstore)
        run: |
          cosign sign \
            -a git-sha=${{ github.sha }} \
            corbe/zen-watcher:${{ github.ref_name }}
```

#### GitLab CI

```yaml
sign:
  stage: sign
  image: gcr.io/projectsigstore/cosign:latest
  script:
    - echo "$COSIGN_KEY" > cosign.key
    - cosign sign --key cosign.key \
        -a git-sha=$CI_COMMIT_SHA \
        -a pipeline-id=$CI_PIPELINE_ID \
        $CI_REGISTRY_IMAGE:$CI_COMMIT_TAG
  only:
    - tags
```

## Verifying Images

### Verify with Public Key

```bash
# Verify signature
cosign verify --key cosign.pub corbe/zen-watcher:1.0.0

# Verify and show annotations
cosign verify --key cosign.pub corbe/zen-watcher:1.0.0 | jq

# Verify specific annotation
cosign verify --key cosign.pub \
  -a git-sha=abc123 \
  corbe/zen-watcher:1.0.0
```

### Verify in Kubernetes

Use admission controller to verify signatures:

```yaml
# Install sigstore-policy-controller
kubectl apply -f https://github.com/sigstore/policy-controller/releases/latest/download/release.yaml

# Create ClusterImagePolicy
apiVersion: policy.sigstore.dev/v1beta1
kind: ClusterImagePolicy
metadata:
  name: zen-watcher-policy
spec:
  images:
  - glob: "corbe/zen-watcher:**"
  authorities:
  - key:
      data: |
        -----BEGIN PUBLIC KEY-----
        <your-public-key-here>
        -----END PUBLIC KEY-----
```

### Verify in Helm

```yaml
# values.yaml
image:
  verifySignature: true
  cosignPublicKey: |
    -----BEGIN PUBLIC KEY-----
    <your-public-key-here>
    -----END PUBLIC KEY-----
```

## Attestations

### Create Attestation

```bash
# Create attestation predicate
cat > predicate.json <<EOF
{
  "buildType": "https://example.com/zen-watcher/build",
  "builder": {
    "id": "https://github.com/your-org/zen-watcher"
  },
  "invocation": {
    "configSource": {
      "uri": "https://github.com/your-org/zen-watcher",
      "digest": {
        "sha256": "abc123..."
      }
    }
  }
}
EOF

# Attach attestation
cosign attest --predicate predicate.json \
  --key cosign.key \
  corbe/zen-watcher:1.0.0
```

### SBOM Attestation

```bash
# Generate SBOM
syft corbe/zen-watcher:1.0.0 -o spdx-json > sbom.spdx.json

# Attach SBOM as attestation
cosign attest --predicate sbom.spdx.json \
  --type spdx \
  --key cosign.key \
  corbe/zen-watcher:1.0.0
```

### Verify Attestation

```bash
# Verify attestation exists
cosign verify-attestation --key cosign.pub \
  corbe/zen-watcher:1.0.0

# View attestation content
cosign verify-attestation --key cosign.pub \
  corbe/zen-watcher:1.0.0 | jq -r .payload | base64 -d | jq
```

## Policy Enforcement

### OPA/Gatekeeper

```yaml
apiVersion: templates.gatekeeper.sh/v1
kind: ConstraintTemplate
metadata:
  name: verifysignature
spec:
  crd:
    spec:
      names:
        kind: VerifySignature
  targets:
    - target: admission.k8s.gatekeeper.sh
      rego: |
        package verifysignature
        violation[{"msg": msg}] {
          container := input.review.object.spec.containers[_]
          not is_signed(container.image)
          msg := sprintf("Image %v is not signed", [container.image])
        }
```

### Kyverno

```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: verify-images
spec:
  validationFailureAction: enforce
  rules:
  - name: verify-zen-watcher
    match:
      any:
      - resources:
          kinds:
          - Pod
    verifyImages:
    - imageReferences:
      - "corbe/zen-watcher:*"
      attestors:
      - count: 1
        entries:
        - keys:
            publicKeys: |-
              -----BEGIN PUBLIC KEY-----
              <your-public-key-here>
              -----END PUBLIC KEY-----
```

## Transparency Log

### Rekor (Public Transparency Log)

```bash
# Sign and upload to Rekor
cosign sign --key cosign.key corbe/zen-watcher:1.0.0

# Verify with Rekor
cosign verify --key cosign.pub \
  --rekor-url=https://rekor.sigstore.dev \
  corbe/zen-watcher:1.0.0

# Search Rekor
rekor-cli search --email user@example.com
```

## Key Management

### Store Keys Securely

```bash
# Store in Kubernetes Secret
kubectl create secret generic cosign-keys \
  --from-file=cosign.key=cosign.key \
  --from-file=cosign.pub=cosign.pub \
  -n zen-system

# Store in HashiCorp Vault
vault kv put secret/cosign \
  private-key=@cosign.key \
  public-key=@cosign.pub

# Store in AWS Secrets Manager
aws secretsmanager create-secret \
  --name cosign-private-key \
  --secret-binary fileb://cosign.key
```

### Rotate Keys

```bash
# Generate new key pair
cosign generate-key-pair -f

# Sign with both keys (transition period)
cosign sign --key cosign.key.old corbe/zen-watcher:1.0.0
cosign sign --key cosign.key corbe/zen-watcher:1.0.0

# Update public key distribution
# After transition period, revoke old key
```

## Best Practices

1. **Always Sign Images**
   - Sign every release
   - Automate in CI/CD

2. **Protect Private Keys**
   - Never commit to git
   - Use secrets management
   - Rotate regularly

3. **Verify Before Deploy**
   - Use admission controllers
   - Enforce in CI/CD
   - Document verification process

4. **Use Annotations**
   - Add build metadata
   - Include git SHA
   - Track provenance

5. **Multiple Verification Methods**
   - Key-based signing
   - Keyless signing
   - Transparency logs

## Troubleshooting

### Signature Not Found

```bash
# Check if image is signed
cosign verify --key cosign.pub corbe/zen-watcher:1.0.0

# If not found, check registry supports OCI artifacts
# Sign again if needed
```

### Wrong Key

```bash
# List all signatures
crane manifest $(cosign triangulate corbe/zen-watcher:1.0.0)

# Verify with correct key
cosign verify --key correct-cosign.pub corbe/zen-watcher:1.0.0
```

### Expired Signature

```bash
# Check expiration
cosign verify --key cosign.pub corbe/zen-watcher:1.0.0 | jq .exp

# Re-sign if expired
cosign sign --key cosign.key corbe/zen-watcher:1.0.0
```

## Resources

- [Cosign Documentation](https://docs.sigstore.dev/cosign/overview)
- [Sigstore](https://www.sigstore.dev/)
- [Supply Chain Security](https://slsa.dev/)
- [NIST SP 800-218 SSDF](https://csrc.nist.gov/publications/detail/sp/800-218/final)

## Example Workflow

Complete workflow from build to deployment:

```bash
# 1. Build image
docker build -t corbe/zen-watcher:1.0.0 .

# 2. Generate SBOM
syft corbe/zen-watcher:1.0.0 -o spdx-json > sbom.json

# 3. Scan for vulnerabilities
grype sbom:sbom.json --fail-on critical

# 4. Push image
docker push corbe/zen-watcher:1.0.0

# 5. Sign image
cosign sign --key cosign.key corbe/zen-watcher:1.0.0

# 6. Attach SBOM attestation
cosign attest --predicate sbom.json --type spdx \
  --key cosign.key corbe/zen-watcher:1.0.0

# 7. Verify before deployment
cosign verify --key cosign.pub corbe/zen-watcher:1.0.0

# 8. Deploy
helm install zen-watcher ./charts/zen-watcher \
  --set image.verifySignature=true \
  --set-file image.cosignPublicKey=cosign.pub
```

## Contact

For Cosign/signing questions: security@kube-zen.com


