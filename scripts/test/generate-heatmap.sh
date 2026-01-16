#!/usr/bin/env bash
# H043: Generate failure heatmap from cross-repo test results
# Reads failure-matrix.json and produces human-readable summary

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ARTIFACTS_DIR="${1:-$(dirname "$SCRIPT_DIR")/../../artifacts/test-run/cross-repo}"

if [ ! -f "${ARTIFACTS_DIR}"/*/failure-matrix.json ] && [ ! -f "${ARTIFACTS_DIR}/failure-matrix.json" ]; then
    echo "Error: failure-matrix.json not found in ${ARTIFACTS_DIR}"
    exit 1
fi

# Find latest failure matrix
FAILURE_MATRIX=$(find "${ARTIFACTS_DIR}" -name "failure-matrix.json" -type f | sort | tail -1)

if [ -z "${FAILURE_MATRIX}" ] || [ ! -f "${FAILURE_MATRIX}" ]; then
    echo "Error: Could not find failure-matrix.json"
    exit 1
fi

if ! command -v jq &> /dev/null; then
    echo "Error: jq is required. Install with: apt install jq / brew install jq"
    exit 1
fi

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Failure Heatmap"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Overall statistics
echo "Overall Statistics:"
jq -r '
  "Total Repos: " + (.repos | keys | length | tostring) + "\n" +
  "Total Failures: " + ([.repos[].failed_tests] | add | tostring)
' "${FAILURE_MATRIX}"
echo ""

# Per-repo summary
echo "Per-Repo Summary:"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
jq -r '.repos | to_entries[] | 
  "\(.key):\n" +
  "  Failed: \(.value.failed_tests // 0) / \(.value.total_tests // 0)\n" +
  "  Failed Packages: \(.value.failed_packages // 0) / \(.value.total_packages // 0)"
' "${FAILURE_MATRIX}"
echo ""

# Failure breakdown by category
echo "Failure Breakdown by Category:"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
jq -r '[.repos[].failures[].category] | group_by(.) | 
  map({category: .[0], count: length}) | 
  sort_by(-.count)[] | 
  "\(.category): \(.count)"' "${FAILURE_MATRIX}"
echo ""

# P0: Build/Deps failures
echo "P0: Build/Deps Failures:"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
jq -r '.repos | to_entries[] | 
  .value.failures[] | 
  select(.category == "build/deps") | 
  "  \(.package)::\(.test) (\(.type))"' "${FAILURE_MATRIX}" | sort || echo "  None"
echo ""

# P1: Logic Regression failures
echo "P1: Logic Regression Failures:"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
jq -r '.repos | to_entries[] | 
  .value.failures[] | 
  select(.category == "logic_regression") | 
  "  \(.package)::\(.test) (\(.type))"' "${FAILURE_MATRIX}" | sort || echo "  None"
echo ""

# P2: Flake/Timing failures
echo "P2: Flake/Timing/Race Failures:"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
jq -r '.repos | to_entries[] | 
  .value.failures[] | 
  select(.category == "flake/timing/race") | 
  "  \(.package)::\(.test) (\(.type))"' "${FAILURE_MATRIX}" | sort || echo "  None"
echo ""

# P3: Environment coupling failures
echo "P3: Environment Coupling Failures:"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
jq -r '.repos | to_entries[] | 
  .value.failures[] | 
  select(.category == "environment_coupling") | 
  "  \(.package)::\(.test) (\(.type))"' "${FAILURE_MATRIX}" | sort || echo "  None"
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
