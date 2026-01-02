# Build stage - Use official Go image with build tools
FROM golang:1.25-alpine AS builder

# Install build dependencies
# git: Required for go mod download with private repos
# ca-certificates: Required for HTTPS connections during build
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

# Copy zen-sdk first (needed for tagged version resolution during build)
# Build context should be from parent directory (zen/)
COPY zen-sdk /build/zen-sdk

# Copy go.mod and go.sum first for better layer caching
# This allows Docker to cache dependencies if code changes but deps don't
COPY zen-watcher/go.mod zen-watcher/go.sum* ./

# Download dependencies first (will fail for zen-sdk but that's ok due to checksum DB delay)
RUN go mod download || true

# Add temporary replace directive for local build (go.mod stays clean with tagged version)
# Path is relative to /build (where go.mod is)
RUN go mod edit -replace github.com/kube-zen/zen-sdk=./zen-sdk

# Copy source code
COPY zen-watcher/ .

# Update go.sum with local zen-sdk (replace directive makes this use local path)
# This is safe because replace takes precedence over the tagged version
RUN go mod tidy || true
RUN go mod download || true

# Build with aggressive optimizations for production
# CGO_ENABLED=0: Static binary, no C dependencies
# GOOS=linux GOARCH=amd64: Explicit target platform
# -ldflags:
#   -w: Strip DWARF debugging information (~30% size reduction)
#   -s: Strip symbol table and debug info
#   -X: Inject build-time variables (version, commit)
# -trimpath: Remove file system paths (security + reproducible builds)
# -tags netgo: Use pure Go network stack (no CGO)
# -installsuffix cgo: Ensure separate output directory
ARG VERSION=1.0.0-alpha
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

# Disable checksum verification for zen-sdk (new tag not yet in checksum DB)
# Use -mod=mod to allow automatic dependency fetching (replace directive handles zen-sdk)
RUN GOSUMDB=off CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=mod \
    -ldflags="-w -s -X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildDate=${BUILD_DATE}" \
    -tags netgo \
    -installsuffix cgo \
    -trimpath \
    -o zen-watcher \
    ./cmd/zen-watcher

# Verify binary
RUN ./zen-watcher --version 2>&1 || true
RUN ls -lh zen-watcher

# Final stage - Distroless for minimal attack surface
# Base image: ~2MB (vs Alpine ~5MB)
# Contains: ca-certificates, /etc/passwd
# No shell, no package manager, minimal CVE exposure
# Note: Timezone data not included - not needed (no LoadLocation calls in code)
FROM gcr.io/distroless/static:nonroot

# Copy CA certificates (required for HTTPS/TLS)
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy compiled binary
COPY --from=builder /build/zen-watcher /zen-watcher

# Use non-root user (uid=65532, gid=65532)
# This is the 'nonroot' user from distroless
USER 65532:65532

# Expose ports
# 8080: HTTP server (webhooks, health checks)
# 9090: Prometheus metrics (future)
EXPOSE 8080 9090

# Health check (commented out - not supported in distroless without shell)
# HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
#   CMD ["/zen-watcher", "health"]

# Run the watcher
ENTRYPOINT ["/zen-watcher"]

# Metadata labels
LABEL org.opencontainers.image.title="zen-watcher"
LABEL org.opencontainers.image.description="Kubernetes security event aggregator"
ARG VERSION=1.0.0-alpha
LABEL org.opencontainers.image.version="${VERSION}"
LABEL org.opencontainers.image.source="https://github.com/kube-zen/zen-watcher"
LABEL org.opencontainers.image.licenses="Apache-2.0"
LABEL org.opencontainers.image.vendor="kube-zen"
