# Tasks H040-H048 Implementation Status

## ✅ H040 — Makefile Targets (COMPLETE)
**Status:** Implemented and committed

- ✅ `make test-unit`: Runs unit tests with `-count=1`, collects artifacts to `./artifacts/test-run/unit/<timestamp>/`
- ✅ `make test-integration`: Runs integration tests with `-count=1`, `GOMAXPROCS=1`, collects artifacts
- ✅ `make test-e2e`: Runs E2E tests with `-count=1`, `GOMAXPROCS=1`, collects artifacts
- ✅ All targets exit non-zero on failure
- ✅ Artifacts include `test-output.log` and `result.txt` with pass/fail status

**Usage:**
```bash
make test-unit          # Run unit tests
make test-integration   # Run integration tests (requires envtest)
make test-e2e           # Run E2E tests (requires k3d clusters)
```

## ✅ H041 — Wire Real Creator Implementation (COMPLETE)
**Status:** Implemented and committed

**Changes:**
- ✅ Added `NewGVRAllowlistFromConfig()` helper function for programmatic allowlist configuration
- ✅ Updated all integration tests to use programmatic config instead of environment variables
- ✅ Tests now have explicit, deterministic allowlist configuration (no global env dependency)
- ✅ All tests use real `CRDCreator` and `ObservationCreator` implementations
- ✅ Allowlist/denylist behavior verified: denies secrets/RBAC/webhooks/CRDs, enforces namespace/GVR allowlists

**Implementation:**
- `pkg/watcher/gvr_allowlist.go`: Added `GVRAllowlistConfig` struct and `NewGVRAllowlistFromConfig()` function
- `test/integration/creator_integration_test.go`: Migrated to programmatic config
- `test/integration/creator_security_test.go`: Migrated to programmatic config, all security regression tests pass

## ✅ H042 — Bulletproof envtest + CRD Install (COMPLETE)
**Status:** Implemented and committed

**Changes:**
- ✅ Added `validateCRDsInstalled()` function that verifies CRDs before test execution
- ✅ Retry logic with exponential backoff for discovery lag (up to 5 attempts)
- ✅ Fail fast with actionable error messages including CRD file paths
- ✅ Version-pinned CRD validation (checks specific group/version/kind)
- ✅ Validates both `observations.zen.kube-zen.io` and `ingesters.zen.kube-zen.io` CRDs

**Implementation:**
- `test/integration/creator_integration_test.go`: Added `validateCRDsInstalled()` in `TestMain()`

## 📋 H043 — Run Tests Across Repos + Failure Heatmap (TODO)
**Status:** Pending

**Requirements:**
- Run unit + integration across: zen-watcher, zen-platform, zen-admin
- Bucket failures: build/deps, logic regression, flake/timing/race, environment coupling
- Create concise failure matrix with P0/P1 ordering

**Script:** `scripts/test/run-all-repos.sh` (needs to be created)

## 📋 H044 — Fix P0/P1 Failures (TODO)
**Status:** Pending H043 completion

**Priority:**
- P0: compilation, missing deps, broken mocks, nil derefs/panics
- P1: deterministic assertions, schema validation, API object shape drift

## 📋 H045 — Validate k3d E2E Harness (TODO)
**Status:** Harness scripts exist, needs validation

**Requirements:**
- Run `scripts/e2e/k3d-up.sh` and verify DNS resolution
- Verify ingress endpoints reachable between clusters
- Ensure netpol/rbac baseline doesn't block control-plane calls

## 📋 H046 — Make E2E Deterministic (TODO)
**Status:** E2E framework exists, needs mock endpoints

**Requirements:**
- Mock endpoints for Slack/DD/PD webhooks
- Local S3-compatible endpoint or stub HTTP sink
- TF/Stripe/GitHub webhook simulators
- No cloud credentials required

## 📋 H047 — Run E2E Suite + Fix Runtime Issues (TODO)
**Status:** Pending H045/H046 completion

**Requirements:**
- Enrollment validation must pass first
- Each v1 flow test must produce evidence artifacts
- Failure paths must produce DLQ/rejection reasons

## 📋 H048 — Tighten CI Gates (TODO)
**Status:** CI scripts exist, needs failure classification

**Requirements:**
- PR gate: unit + integration always required
- main/nightly: E2E required, artifacts uploaded
- Failure classification output (creator policy, networking, enrollment, delivery, connector/mocks)

**Files:**
- `scripts/ci/integration-test-gate.sh` (exists)
- `scripts/ci/e2e-test-gate.sh` (exists)

## Summary

**Completed:** H040, H041, H042
**In Progress:** None
**Pending:** H043-H048

**Next Steps:**
1. H043: Create cross-repo test execution script and failure heatmap generator
2. H044: Fix identified P0/P1 failures from H043
3. H045-H047: Validate and stabilize E2E harness, then run E2E suite
4. H048: Add failure classification to CI gates
