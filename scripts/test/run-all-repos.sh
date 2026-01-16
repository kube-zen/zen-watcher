#!/usr/bin/env bash
# H043: Run unit+integration tests across repos and create failure heatmap
# Executes tests in: zen-watcher, zen-platform, zen-admin
# Buckets failures: build/deps, logic regression, flake/timing/race, environment coupling
# Produces: artifacts/test-run/cross-repo/<timestamp>/failure-matrix.json

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WATCHER_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
ROOT_DIR="$(cd "$WATCHER_DIR/.." && pwd)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_section() {
    echo -e "${CYAN}━━━━ $1 ━━━━${NC}"
}

# Timestamp for artifact directory
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
ARTIFACTS_DIR="${ROOT_DIR}/artifacts/test-run/cross-repo/${TIMESTAMP}"
mkdir -p "${ARTIFACTS_DIR}"

# Failure matrix (JSON structure)
FAILURE_MATRIX="${ARTIFACTS_DIR}/failure-matrix.json"

# Initialize failure matrix
cat > "${FAILURE_MATRIX}" <<'EOF'
{
  "timestamp": "",
  "repos": {}
}
EOF

# Update timestamp in JSON
TIMESTAMP_ISO=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
if command -v jq &> /dev/null; then
    jq --arg ts "${TIMESTAMP_ISO}" '.timestamp = $ts' "${FAILURE_MATRIX}" > "${FAILURE_MATRIX}.tmp" && mv "${FAILURE_MATRIX}.tmp" "${FAILURE_MATRIX}"
fi

# Function to classify failure
classify_failure() {
    local output="$1"
    local test_name="$2"
    
    # Build/Deps: compilation errors, missing imports, module issues
    if echo "$output" | grep -qiE "(undefined:|cannot find package|undefined reference|build constraints|go.mod)" ; then
        echo "build/deps"
        return
    fi
    
    # Deterministic panics/crashes: nil pointer, index out of range, etc.
    if echo "$output" | grep -qiE "(panic:|nil pointer|index out of range|slice bounds|invalid memory)" ; then
        echo "logic_regression"
        return
    fi
    
    # Flaky/Timing: timeout, race condition, sleep-dependent
    if echo "$output" | grep -qiE "(timeout|deadline exceeded|race detected|data race)" ; then
        echo "flake/timing/race"
        return
    fi
    
    # Environment coupling: requires cluster, network, credentials
    if echo "$output" | grep -qiE "(connection refused|dial tcp|no such host|unauthorized|certificate|kubeconfig|cluster)" ; then
        echo "environment_coupling"
        return
    fi
    
    # Default: logic regression (deterministic assertion failures)
    echo "logic_regression"
}

# Function to run tests for a repo
run_repo_tests() {
    local repo_name="$1"
    local repo_dir="$2"
    local repo_artifacts="${ARTIFACTS_DIR}/${repo_name}"
    mkdir -p "${repo_artifacts}"
    
    log_section "Testing ${repo_name}"
    
    if [ ! -d "${repo_dir}" ]; then
        log_warn "Repository ${repo_name} not found at ${repo_dir}, skipping"
        return 1
    fi
    
    cd "${repo_dir}"
    
    # Check if Go module
    if [ ! -f "go.mod" ]; then
        log_warn "${repo_name} does not appear to be a Go module, skipping"
        return 1
    fi
    
    # Initialize repo entry in failure matrix
    local repo_json=$(cat <<JSON
{
  "total_packages": 0,
  "failed_packages": 0,
  "total_tests": 0,
  "failed_tests": 0,
  "failures": []
}
JSON
)
    
    if command -v jq &> /dev/null; then
        jq --arg repo "${repo_name}" --argjson data "${repo_json}" '.repos[$repo] = $data' "${FAILURE_MATRIX}" > "${FAILURE_MATRIX}.tmp" && mv "${FAILURE_MATRIX}.tmp" "${FAILURE_MATRIX}"
    fi
    
    # Run unit tests
    log_info "Running unit tests for ${repo_name}..."
    local unit_output="${repo_artifacts}/unit-test-output.log"
    local unit_result="${repo_artifacts}/unit-result.json"
    
    if GOMAXPROCS=${GOMAXPROCS:-1} go test -v -count=1 -short ./... 2>&1 | tee "${unit_output}" ; then
        echo "{\"result\":\"pass\",\"failures\":[]}" > "${unit_result}"
        log_info "✅ Unit tests passed for ${repo_name}"
    else
        local exit_code=$?
        log_error "❌ Unit tests failed for ${repo_name}"
        
        # Parse failures from test output
        parse_test_failures "${unit_output}" "${repo_name}" "unit" "${unit_result}"
    fi
    
    # Run integration tests (if directory exists)
    if [ -d "./test/integration" ] || [ -d "./tests/integration" ]; then
        log_info "Running integration tests for ${repo_name}..."
        local integration_output="${repo_artifacts}/integration-test-output.log"
        local integration_result="${repo_artifacts}/integration-result.json"
        
        if GOMAXPROCS=${GOMAXPROCS:-1} go test -v -count=1 -timeout 15m ./test/integration/... 2>&1 | tee "${integration_output}" ; then
            echo "{\"result\":\"pass\",\"failures\":[]}" > "${integration_result}"
            log_info "✅ Integration tests passed for ${repo_name}"
        else
            log_error "❌ Integration tests failed for ${repo_name}"
            parse_test_failures "${integration_output}" "${repo_name}" "integration" "${integration_result}"
        fi
    else
        log_info "No integration tests found for ${repo_name}"
    fi
    
    return 0
}

# Function to parse test failures from output
parse_test_failures() {
    local output_file="$1"
    local repo_name="$2"
    local test_type="$3"
    local result_file="$4"
    
    local failures_json="[]"
    local current_package=""
    local current_test=""
    local failure_output=""
    
    while IFS= read -r line; do
        # Detect package start
        if [[ "$line" =~ ^===.*RUN.*(.*/.*)$ ]]; then
            current_package=$(echo "$line" | sed -E 's/.*\(([^)]+)\)/\1/')
            current_test=""
            failure_output=""
        elif [[ "$line" =~ ^---.*FAIL.*:(.+)$ ]]; then
            current_test=$(echo "$line" | sed -E 's/.*FAIL[^:]*:(.+)$/\1/' | xargs)
            failure_output=""
        elif [[ "$line" =~ ^FAIL[[:space:]]+([^[:space:]]+)$ ]]; then
            current_package=$(echo "$line" | sed -E 's/^FAIL[[:space:]]+//' | xargs)
        elif [[ -n "$current_test" ]]; then
            failure_output+="$line"$'\n'
        fi
        
        # If we hit a new test or package, save previous failure
        if [[ "$line" =~ ^===.*RUN ]] || [[ "$line" =~ ^---.*PASS ]] || [[ "$line" =~ ^ok[[:space:]] ]] || [[ "$line" =~ ^FAIL[[:space:]] ]]; then
            if [[ -n "$current_test" && -n "$failure_output" ]]; then
                local category=$(classify_failure "$failure_output" "$current_test")
                local failure_entry=$(cat <<JSON
{
  "package": "${current_package}",
  "test": "${current_test}",
  "category": "${category}",
  "type": "${test_type}",
  "error_preview": "$(echo "$failure_output" | head -5 | jq -Rs . | tr -d '\n' | cut -c1-200)"
}
JSON
)
                if command -v jq &> /dev/null; then
                    failures_json=$(echo "$failures_json" | jq --argjson entry "${failure_entry}" '. += [$entry]')
                fi
                current_test=""
                failure_output=""
            fi
        fi
    done < "${output_file}"
    
    # Create result file
    echo "{\"result\":\"fail\",\"failures\":${failures_json}}" > "${result_file}"
    
    # Update failure matrix
    if command -v jq &> /dev/null && [ -s "${result_file}" ]; then
        local failures_count=$(echo "$failures_json" | jq 'length')
        jq --arg repo "${repo_name}" \
           --arg type "${test_type}" \
           --argjson failures "${failures_json}" \
           '.repos[$repo].failures += $failures | 
            .repos[$repo].failed_tests += '$failures_count' |
            .repos[$repo].total_tests += '$failures_count'' \
           "${FAILURE_MATRIX}" > "${FAILURE_MATRIX}.tmp" && mv "${FAILURE_MATRIX}.tmp" "${FAILURE_MATRIX}"
    fi
}

# Main execution
log_section "Cross-Repo Test Execution (H043)"

# Test zen-watcher (current repo)
run_repo_tests "zen-watcher" "${WATCHER_DIR}"

# Test zen-platform (if exists)
if [ -d "${ROOT_DIR}/zen-platform" ]; then
    run_repo_tests "zen-platform" "${ROOT_DIR}/zen-platform"
else
    log_warn "zen-platform not found at ${ROOT_DIR}/zen-platform, skipping"
fi

# Test zen-admin (if exists)
if [ -d "${ROOT_DIR}/zen-admin" ]; then
    run_repo_tests "zen-admin" "${ROOT_DIR}/zen-admin"
else
    log_warn "zen-admin not found at ${ROOT_DIR}/zen-admin, skipping"
fi

# Generate summary report
log_section "Failure Summary"

if command -v jq &> /dev/null && [ -f "${FAILURE_MATRIX}" ]; then
    echo ""
    echo "Failure Matrix: ${FAILURE_MATRIX}"
    echo ""
    jq -r '.repos | to_entries[] | "\(.key): \(.value.failed_tests // 0) failed / \(.value.total_tests // 0) total"' "${FAILURE_MATRIX}"
    echo ""
    
    # Generate priority-ordered failure list
    log_info "P0/P1 Failures (Priority Order):"
    jq -r '.repos | to_entries[] | .value.failures[] | 
           select(.category == "build/deps" or .category == "logic_regression") |
           "P0/P1|\(.package)|\(.test)|\(.category)|\(.type)"' \
           "${FAILURE_MATRIX}" | sort -u || true
    echo ""
    
    log_info "Failure categories:"
    jq -r '.repos | to_entries[] | .value.failures[].category' "${FAILURE_MATRIX}" | sort | uniq -c | sort -rn || true
else
    log_warn "jq not installed, cannot generate JSON summary. Install with: apt install jq / brew install jq"
fi

log_info "Artifacts saved to: ${ARTIFACTS_DIR}"

# Exit with error if any failures
if command -v jq &> /dev/null && [ -f "${FAILURE_MATRIX}" ]; then
    TOTAL_FAILURES=$(jq '[.repos[].failed_tests] | add' "${FAILURE_MATRIX}")
    if [ "${TOTAL_FAILURES:-0}" -gt 0 ]; then
        log_error "Total failures: ${TOTAL_FAILURES}"
        exit 1
    fi
fi

log_section "✅ Cross-repo test execution complete"
