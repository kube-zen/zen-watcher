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

## 🔄 H041 — Wire Real Creator Implementation (IN PROGRESS)
**Status:** Tests scaffolded, needs allowlist config helper

**Current State:**
- Integration tests exist: `test/integration/creator_integration_test.go`
- Security tests exist: `test/integration/creator_security_test.go`
- Real `CRDCreator` and `ObservationCreator` implementations exist with allowlist enforcement
- Tests use environment variables to configure allowlist

**Remaining Work:**
- Add helper function `newTestGVRAllowlist()` for deterministic test config (not relying on global env)
- Verify tests pass against real creator code
- Ensure deny list (secrets/RBAC/webhooks/CRDs) is enforced

**Implementation Plan:**
1. Add `NewGVRAllowlistFromConfig()` in `gvr_allowlist.go` for programmatic config
2. Update integration tests to use programmatic config
3. Verify deny list enforcement in `creator_security_test.go`

## 📋 H042 — Bulletproof envtest + CRD Install (TODO)
**Status:** Needs enhancement

**Requirements:**
- Idempotent CRD install
- Version-pinned CRDs
- Fail fast with actionable errors
- Validate CRDs are installed before creating objects

**Implementation Plan:**
1. Add CRD validation helper that checks if CRD exists before proceeding
2. Add retry logic with exponential backoff for CRD discovery lag
3. Add version pinning check (ensure CRD version matches expected)
4. Enhance error messages to include actionable guidance

## 📋 H043 — Run Tests Across Repos + Failure Heatmap (TODO)
**Status:** Pending H041/H042 completion

**Requirements:**
- Run unit + integration across: zen-watcher, zen-platform, zen-admin
- Bucket failures: build/deps, logic regression, flake/timing/race, environment coupling
- Create concise failure matrix with P0/P1 ordering

**Script:** `scripts/test/run-all-repos.sh`

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

**Completed:** H040
**In Progress:** H041 (needs test helper function)
**Pending:** H042-H048 (blocked by H041 completion or can be done in parallel)

**Next Steps:**
1. Complete H041: Add `NewGVRAllowlistFromConfig()` helper
2. Complete H042: Enhance envtest CRD validation
3. Run H043: Execute test suite across repos
4. Continue with remaining tasks in sequence
