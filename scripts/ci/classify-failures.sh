#!/usr/bin/env bash
# H048: Failure classification for CI gates
# Classifies test failures into: creator_policy, networking, enrollment, delivery_semantics, connector_mocks

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_section() { echo -e "${CYAN}━━━━ $1 ━━━━${NC}"; }

# Read test output from stdin or file
TEST_OUTPUT="${1:-/dev/stdin}"

# Classification function
classify_failure() {
    local output="$1"
    
    # Creator policy: allowlist/denylist errors
    if echo "$output" | grep -qiE "(GVR.*not.*allowlist|namespace.*not.*allowlist|GVR.*denied|ErrGVRNotAllowed|ErrNamespaceNotAllowed|ErrGVRDenied)"; then
        echo "creator_policy"
        return
    fi
    
    # Networking: connection errors, DNS, ingress issues
    if echo "$output" | grep -qiE "(connection refused|dial tcp|no such host|timeout|ingress.*not.*reachable|network.*error)"; then
        echo "networking"
        return
    fi
    
    # Enrollment: identity/bootstrap/registration issues
    if echo "$output" | grep -qiE "(enrollment.*failed|bootstrap.*failed|identity.*not.*found|registration.*error|certificate.*invalid)"; then
        echo "enrollment"
        return
    fi
    
    # Delivery semantics: DLQ, retry, event delivery issues
    if echo "$output" | grep -qiE "(dlq|dead.*letter|delivery.*failed|retry.*exceeded|event.*not.*delivered|backpressure)"; then
        echo "delivery_semantics"
        return
    fi
    
    # Connector/mocks: webhook connector, mock endpoint issues
    if echo "$output" | grep -qiE "(webhook.*connector|mock.*server|slack.*endpoint|datadog.*endpoint|pagerduty.*endpoint|s3.*endpoint)"; then
        echo "connector_mocks"
        return
    fi
    
    # Default: unknown
    echo "unknown"
}

# Parse and classify failures
if [ -f "${TEST_OUTPUT}" ]; then
    TEST_CONTENT=$(cat "${TEST_OUTPUT}")
else
    TEST_CONTENT="${TEST_OUTPUT}"
fi

log_section "Failure Classification (H048)"

CLASSIFICATIONS=$(echo "${TEST_CONTENT}" | grep -i "FAIL\|error\|panic" | while read -r line; do
    classification=$(classify_failure "$line")
    echo "${classification}"
done | sort | uniq -c | sort -rn)

if [ -n "${CLASSIFICATIONS}" ]; then
    echo "Failure Categories:"
    echo "${CLASSIFICATIONS}"
    echo ""
    
    # Count by category
    CREATOR_POLICY=$(echo "${CLASSIFICATIONS}" | grep "creator_policy" | awk '{print $1}' || echo "0")
    NETWORKING=$(echo "${CLASSIFICATIONS}" | grep "networking" | awk '{print $1}' || echo "0")
    ENROLLMENT=$(echo "${CLASSIFICATIONS}" | grep "enrollment" | awk '{print $1}' || echo "0")
    DELIVERY=$(echo "${CLASSIFICATIONS}" | grep "delivery_semantics" | awk '{print $1}' || echo "0")
    CONNECTOR=$(echo "${CLASSIFICATIONS}" | grep "connector_mocks" | awk '{print $1}' || echo "0")
    
    echo "Summary:"
    echo "  creator_policy: ${CREATOR_POLICY}"
    echo "  networking: ${NETWORKING}"
    echo "  enrollment: ${ENROLLMENT}"
    echo "  delivery_semantics: ${DELIVERY}"
    echo "  connector_mocks: ${CONNECTOR}"
else
    log_info "No failures detected"
fi
