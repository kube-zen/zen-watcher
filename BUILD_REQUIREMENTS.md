# Build Requirements

## Tag Push Requirement

The `zen-watcher` module depends on `github.com/kube-zen/zen-sdk v0.2.0-alpha`. For Docker builds to succeed, this tag **must be pushed to the remote repository**.

### Current Status

- ✅ `go.mod` is configured with tagged version (`v0.2.0-alpha`)
- ✅ No `replace` directive in `go.mod` (uses tagged version)
- ⚠️ Tag `v0.2.0-alpha` exists locally but needs to be pushed remotely
- ⚠️ Docker builds will fail until tag is pushed

### To Enable Docker Builds

1. Push the tag to remote:
   ```bash
   cd zen-sdk
   git push origin v0.2.0-alpha
   ```

2. After pushing, Docker builds should work without modifications.

### Local Development

For local development, use `go.work` (configured at repository root) which allows using local paths without `replace` directives in `go.mod`.

### Dockerfile Strategy

The Dockerfile in `build/Dockerfile` copies `zen-sdk` locally during build. Once the tag is pushed, this can be simplified.

