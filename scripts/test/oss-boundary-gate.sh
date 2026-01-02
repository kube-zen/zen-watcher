#!/bin/bash
# OSS Boundary Enforcement Gate
# Fails if SaaS-only code patterns are found in OSS repositories

set -e

REPO_ROOT="${1:-$(pwd)}"
FAILED=0
VIOLATIONS=()

echo "Checking OSS boundary in $REPO_ROOT..."
echo "Scanning: cmd/, pkg/, internal/ (excluding scripts/, tests/, fixtures/, docs/, examples/, vendor/, dist/)"
echo ""

# Function to add violation with rule ID
add_violation() {
	local rule_id=$1
	local file=$2
	local line=$3
	local pattern=$4
	local hint=$5
	VIOLATIONS+=("$rule_id|$file|$line|$pattern|$hint")
	FAILED=1
}

# Find relevant Go files (cmd/, pkg/, internal/ only, exclude test files)
SCAN_DIRS=("cmd" "pkg" "internal")
GO_FILES=$(find "$REPO_ROOT" -type f -name "*.go" | \
	grep -E "/(cmd|pkg|internal)/" | \
	grep -vE "/(scripts|tests?|fixtures?|docs|examples|vendor|dist)/" | \
	grep -v "_test\.go$" || true)

if [ -z "$GO_FILES" ]; then
	echo "⚠️  No Go files found in cmd/, pkg/, internal/"
	exit 0
fi

# OSS001: ZEN_API_BASE_URL references
echo "Checking OSS001: ZEN_API_BASE_URL references..."
while IFS= read -r file; do
	if grep -n "ZEN_API_BASE_URL" "$file" 2>/dev/null | grep -v "oss-boundary-gate.sh" | grep -v "OSS_BOUNDARY.md"; then
		line=$(grep -n "ZEN_API_BASE_URL" "$file" | head -1 | cut -d: -f1)
		add_violation "OSS001" "$file" "$line" "ZEN_API_BASE_URL" "Remove SaaS API base URL; use kubeconfig for OSS operations"
	fi
done <<< "$GO_FILES"

# OSS002: SaaS API endpoint references (/v1/audit, /v1/clusters, /v1/adapters, /v1/tenants)
echo "Checking OSS002: SaaS API endpoint references..."
while IFS= read -r file; do
	if grep -nE "/v1/(audit|clusters|adapters|tenants)" "$file" 2>/dev/null; then
		line=$(grep -nE "/v1/(audit|clusters|adapters|tenants)" "$file" | head -1 | cut -d: -f1)
		pattern=$(grep -nE "/v1/(audit|clusters|adapters|tenants)" "$file" | head -1 | cut -d: -f2- | sed 's/^[[:space:]]*//')
		add_violation "OSS002" "$file" "$line" "$pattern" "Remove SaaS API endpoint; OSS CLI should use Kubernetes APIs only"
	fi
done <<< "$GO_FILES"

# OSS003: src/saas/ imports
echo "Checking OSS003: src/saas/ imports..."
while IFS= read -r file; do
	if grep -n '".*src/saas/' "$file" 2>/dev/null; then
		line=$(grep -n '".*src/saas/' "$file" | head -1 | cut -d: -f1)
		pattern=$(grep -n '".*src/saas/' "$file" | head -1 | cut -d: -f2- | sed 's/^[[:space:]]*//')
		add_violation "OSS003" "$file" "$line" "$pattern" "Remove SaaS package import; use OSS SDK packages only"
	fi
done <<< "$GO_FILES"

# OSS004: Tenant/entitlement SaaS handlers (paired pattern)
echo "Checking OSS004: Tenant/entitlement SaaS handler patterns..."
while IFS= read -r file; do
	if grep -n -iE "tenant.*entitlement|entitlement.*tenant" "$file" 2>/dev/null | grep -v "//" | grep -v "OSS_BOUNDARY.md"; then
		line=$(grep -n -iE "tenant.*entitlement|entitlement.*tenant" "$file" | head -1 | cut -d: -f1)
		pattern=$(grep -n -iE "tenant.*entitlement|entitlement.*tenant" "$file" | head -1 | cut -d: -f2- | sed 's/^[[:space:]]*//')
		add_violation "OSS004" "$file" "$line" "$pattern" "Remove tenant/entitlement SaaS handler; OSS uses K8s CRD status only"
	fi
done <<< "$GO_FILES"

# OSS005: Redis/Cockroach client usage in CLI paths
echo "Checking OSS005: Redis/Cockroach client usage in CLI..."
CLI_GO_FILES=$(echo "$GO_FILES" | grep "/cmd/" || true)
while IFS= read -r file; do
	if grep -n -iE "\bredis\b|\bcockroach\b" "$file" 2>/dev/null | grep -v "test" | grep -v "//"; then
		line=$(grep -n -iE "\bredis\b|\bcockroach\b" "$file" | head -1 | cut -d: -f1)
		pattern=$(grep -n -iE "\bredis\b|\bcockroach\b" "$file" | head -1 | cut -d: -f2- | sed 's/^[[:space:]]*//')
		add_violation "OSS005" "$file" "$line" "$pattern" "Remove Redis/Cockroach client; OSS CLI should not use external databases"
	fi
done <<< "$CLI_GO_FILES"

# Report violations
if [ $FAILED -eq 0 ]; then
	echo "✅ PASS: OSS boundary check passed"
	exit 0
else
	echo ""
	echo "❌ FAIL: OSS boundary violations detected"
	echo ""
	echo "Violations:"
	echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	for violation in "${VIOLATIONS[@]}"; do
		IFS='|' read -r rule_id file line pattern hint <<< "$violation"
		rel_file="${file#$REPO_ROOT/}"
		echo "Rule: $rule_id"
		echo "  File: $rel_file:$line"
		echo "  Pattern: $pattern"
		echo "  Hint: $hint"
		echo ""
	done
	echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	exit 1
fi
