# Third-Party Licenses

This document lists the major third-party dependencies used by zen-watcher and their respective licenses. All dependencies are compatible with Apache 2.0.

## Direct Dependencies

### Kubernetes Client Libraries
- **k8s.io/api** v0.28.15
- **k8s.io/apimachinery** v0.28.15
- **k8s.io/client-go** v0.28.15
- **License**: Apache 2.0
- **Copyright**: 2014-2024 The Kubernetes Authors
- **Source**: https://github.com/kubernetes/kubernetes

### Prometheus Client
- **github.com/prometheus/client_golang** v1.19.0
- **License**: Apache 2.0
- **Copyright**: 2012-2024 The Prometheus Authors
- **Source**: https://github.com/prometheus/client_golang

### Structured Logging
- **go.uber.org/zap** v1.27.1
- **License**: MIT
- **Copyright**: 2016-2024 Uber Technologies, Inc.
- **Source**: https://github.com/uber-go/zap

## Indirect Dependencies

### Google Libraries
- **golang.org/x/net** v0.23.0 - BSD-3-Clause
- **golang.org/x/oauth2** v0.16.0 - BSD-3-Clause
- **golang.org/x/sys** v0.18.0 - BSD-3-Clause
- **golang.org/x/text** v0.14.0 - BSD-3-Clause
- **golang.org/x/time** v0.3.0 - BSD-3-Clause
- **google.golang.org/protobuf** v1.36.10 - BSD-3-Clause

### Kubernetes Ecosystem
- **k8s.io/klog/v2** v2.130.1 - Apache 2.0
- **k8s.io/kube-openapi** - Apache 2.0
- **k8s.io/utils** - Apache 2.0
- **sigs.k8s.io/json** - Apache 2.0
- **sigs.k8s.io/yaml** v1.6.0 - MIT / Apache 2.0

### Other Libraries
- **github.com/go-openapi/jsonpointer** v0.21.0 - Apache 2.0
- **github.com/go-openapi/jsonreference** v0.20.2 - Apache 2.0
- **github.com/go-openapi/swag** v0.23.0 - Apache 2.0
- **github.com/google/gnostic-models** v0.6.8 - Apache 2.0
- **gopkg.in/yaml.v2** v2.4.0 - Apache 2.0 / MIT
- **gopkg.in/yaml.v3** v3.0.1 - MIT / Apache 2.0

## License Compatibility

All listed dependencies use licenses that are compatible with Apache 2.0:
- **Apache 2.0**: Fully compatible, can be combined
- **MIT**: Compatible, can be combined
- **BSD-3-Clause**: Compatible, can be combined

## Verification

To verify licenses of all dependencies, run:
```bash
go list -m -f '{{.Path}} {{.Version}}' all | xargs -I {} sh -c 'echo "Checking {}"; go list -m -json {} | jq -r ".Path, .Version, .Dir"'
```

For a more detailed license check, consider using tools like:
- [go-licenses](https://github.com/google/go-licenses)
- [license-checker](https://github.com/davglass/license-checker)

## Last Updated

This document was last updated: 2024-11-29

