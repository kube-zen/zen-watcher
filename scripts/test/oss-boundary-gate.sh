#!/bin/bash
# OSS Boundary Enforcement Gate
# Fails if SaaS-only code patterns are found in OSS repositories

set -e

REPO_ROOT="${1:-$(pwd)}"
FAILED=0

echo "Checking OSS boundary in $REPO_ROOT..."

# Check for ZEN_API_BASE_URL references
if grep -r "ZEN_API_BASE_URL" "$REPO_ROOT" --include="*.go" --include="*.sh" --include="*.yaml" --include="*.yml" 2>/dev/null | grep -v "oss-boundary-gate.sh" | grep -v "OSS_BOUNDARY.md"; then
	echo "❌ FAIL: Found ZEN_API_BASE_URL references (SaaS-only)"
	FAILED=1
fi

# Check for /v1/audit endpoint references
if grep -r "/v1/audit" "$REPO_ROOT" --include="*.go" --include="*.sh" 2>/dev/null | grep -v "oss-boundary-gate.sh"; then
	echo "❌ FAIL: Found /v1/audit endpoint references (SaaS-only)"
	FAILED=1
fi

# Check for src/saas/ imports
if grep -r "src/saas/" "$REPO_ROOT" --include="*.go" 2>/dev/null | grep -v "oss-boundary-gate.sh"; then
	echo "❌ FAIL: Found src/saas/ imports (internal-only)"
	FAILED=1
fi

# Check for tenant entitlement SaaS handlers (pattern matching)
if grep -r "tenant.*entitlement\|entitlement.*tenant" "$REPO_ROOT" --include="*.go" -i 2>/dev/null | grep -v "oss-boundary-gate.sh" | grep -v "OSS_BOUNDARY.md"; then
	echo "❌ FAIL: Found tenant/entitlement SaaS handler patterns"
	FAILED=1
fi

if [ $FAILED -eq 0 ]; then
	echo "✅ PASS: OSS boundary check passed"
	exit 0
else
	echo "❌ FAIL: OSS boundary violations detected"
	exit 1
fi

