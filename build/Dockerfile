# Build stage
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /build
COPY . .

# Build with optimizations for minimal size
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -trimpath \
    -o zen-watcher \
    ./cmd/zen-watcher

# Final stage - distroless for minimal footprint
FROM gcr.io/distroless/static:nonroot

COPY --from=builder /build/zen-watcher /zen-watcher

USER nonroot:nonroot

EXPOSE 8080 9090

ENTRYPOINT ["/zen-watcher"]
