# Zen Watcher - Development Tools

This directory contains **development-only tools** for Zen Watcher.

> **Note**: All operational scripts have been moved to `./scripts/`. See [scripts/README.md](../scripts/README.md) for the complete script directory.

---

## Development Tools

### Helmfile Configuration

- **`helmfile.yaml.gotmpl`** - Helmfile template for development deployments
- **`README-HELMFILE.md`** - Helmfile usage guide

---

## Operational Scripts

All operational scripts (demo, testing, benchmarking, etc.) are now in `./scripts/`:

| Task | Script Location |
|------|----------------|
| Quick demo | `./scripts/quick-demo.sh` |
| Mock data | `./scripts/data/mock-data.sh` |
| E2E tests | `./scripts/ci/e2e-test.sh` |
| Benchmarks | `./scripts/benchmark/` |
| Cluster management | `./scripts/cluster/` |
| Observability setup | `./scripts/observability/` |

See **[scripts/README.md](../scripts/README.md)** for complete documentation.

---

## Quick Reference

```bash
# Quick demo (from scripts/)
./scripts/quick-demo.sh

# Cleanup cluster
./scripts/cluster/cleanup.sh

# Run benchmarks
./scripts/benchmark/quick-bench.sh
```

---

**For all operational scripts, see `./scripts/` directory.**
