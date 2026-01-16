#!/usr/bin/env bash
# H043: OSS boundary gate - prevents platform coupling in zen-watcher
# Checks for platform terminology, imports, enrollment hooks, delivery destinations

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WATCHER_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }

cd "${WATCHER_DIR}"

STRICT_MODE="${OSS_BOUNDARY_STRICT:-0}"

VIOLATIONS=0

# Check 1: Platform package imports (platform code should not be imported)
log_info "Checking for platform package imports..."
PLATFORM_IMPORTS=$(grep -r "github.com/kube-zen/zen-platform" --include="*.go" . 2>/dev/null | grep -v "test/integration\|test/e2e" | grep -v "CODEOWNERS\|oss-boundary" || true)
if [ -n "${PLATFORM_IMPORTS}" ]; then
	log_error "Found platform package imports (should not exist in OSS):"
	echo "${PLATFORM_IMPORTS}"
	VIOLATIONS=$((VIOLATIONS + 1))
fi

# Check 2: Platform terminology (enrollment, delivery destinations beyond OSS)
log_info "Checking for platform terminology..."
PLATFORM_TERMS=$(grep -riE "(enrollment|bootstrap|identity|registration|evidence artifact|delivery receipt)" --include="*.go" --include="*.md" . 2>/dev/null | grep -v "test/integration\|test/e2e\|CHANGELOG\|ARCHITECTURE" | grep -v "CODEOWNERS\|oss-boundary" || true)
if [ -n "${PLATFORM_TERMS}" ]; then
	if [ "${STRICT_MODE}" = "1" ]; then
		log_error "Found platform terminology (strict mode):"
		echo "${PLATFORM_TERMS}"
		VIOLATIONS=$((VIOLATIONS + 1))
	else
		log_warn "Found platform terminology (non-strict):"
		echo "${PLATFORM_TERMS}"
	fi
fi

# Check 3: Security hooks (HMAC key management, enrollment hooks)
log_info "Checking for platform security hooks..."
SECURITY_HOOKS=$(grep -riE "(HMAC.*key.*source|enrollment.*hook|bootstrap.*credential)" --include="*.go" . 2>/dev/null | grep -v "test/" || true)
if [ -n "${SECURITY_HOOKS}" ]; then
	log_error "Found platform security hooks (should be in zen-ingester):"
	echo "${SECURITY_HOOKS}"
	VIOLATIONS=$((VIOLATIONS + 1))
fi

# Check 4: Delivery destinations beyond OSS (SaaS endpoints, external webhooks)
log_info "Checking for non-OSS delivery destinations..."
DELIVERY_DEST=$(grep -riE "(slack.*webhook|datadog.*endpoint|pagerduty|s3.*bucket|saas.*endpoint)" --include="*.go" --include="*.yaml" . 2>/dev/null | grep -v "test/" | grep -v "CHANGELOG\|README" || true)
if [ -n "${DELIVERY_DEST}" ]; then
	log_error "Found non-OSS delivery destinations (should be in zen-ingester/zen-egress):"
	echo "${DELIVERY_DEST}"
	VIOLATIONS=$((VIOLATIONS + 1))
fi

# Summary
if [ ${VIOLATIONS} -eq 0 ]; then
	log_info "✅ OSS boundary check passed - no platform coupling detected"
	exit 0
else
	log_error "❌ OSS boundary check failed - ${VIOLATIONS} violation(s) found"
	log_error "Platform behavior must live in zen-platform, not zen-watcher (OSS)"
	exit 1
fi
