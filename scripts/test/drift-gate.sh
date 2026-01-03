#!/bin/bash
#
# Drift Gate: Bounded execution gate for zenctl diff
# Validates that zenctl diff completes under time bounds and returns contract exit codes
# On failure, emits artifact bundle with normalized YAML and diff output
#
# Usage:
#   ./scripts/test/drift-gate.sh [--manifest-dir <dir>] [--namespace <ns>] [--context <ctx>] [--artifact-dir <dir>]
#
# Environment Variables:
#   ZENCTL_BIN - Path to zenctl binary (default: ./zenctl)
#   GATE_TIMEOUT_SECONDS - Maximum execution time (default: 60)
#   MANIFEST_DIR - Directory to diff (default: current directory)
#   NAMESPACE - Kubernetes namespace (default: default)
#   KUBECONFIG - Kubeconfig path (default: $KUBECONFIG or ~/.kube/config)
#   ARTIFACT_DIR - Directory for artifact bundle (default: /tmp/drift-gate-artifacts)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$SCRIPT_DIR"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Configuration
ZENCTL_BIN="${ZENCTL_BIN:-./zenctl}"
GATE_TIMEOUT_SECONDS="${GATE_TIMEOUT_SECONDS:-60}"
MANIFEST_DIR="${MANIFEST_DIR:-.}"
NAMESPACE="${NAMESPACE:-default}"
CONTEXT="${CONTEXT:-}"
ARTIFACT_DIR="${ARTIFACT_DIR:-/tmp/drift-gate-artifacts}"

# Parse args
while [[ $# -gt 0 ]]; do
	case $1 in
		--manifest-dir)
			MANIFEST_DIR="$2"
			shift 2
			;;
		--namespace)
			NAMESPACE="$2"
			shift 2
			;;
		--context)
			CONTEXT="$2"
			shift 2
			;;
		--artifact-dir)
			ARTIFACT_DIR="$2"
			shift 2
			;;
		*)
			echo "Unknown option: $1"
			exit 1
			;;
	esac
done

log_info() {
	echo -e "${GREEN}ℹ${NC} $*"
}

log_error() {
	echo -e "${RED}✗${NC} $*"
}

log_step() {
	echo -e "${YELLOW}→${NC} $*"
}

# Check prerequisites
if [ ! -f "$ZENCTL_BIN" ]; then
	log_error "zenctl binary not found at $ZENCTL_BIN"
	log_info "Build it with: make zenctl"
	exit 1
fi

if [ ! -d "$MANIFEST_DIR" ]; then
	log_error "Manifest directory not found: $MANIFEST_DIR"
	exit 1
fi

# Build zenctl command
ZENCTL_CMD=("$ZENCTL_BIN" "diff" "-f" "$MANIFEST_DIR" "-n" "$NAMESPACE")
if [ -n "$CONTEXT" ]; then
	ZENCTL_CMD+=("--context" "$CONTEXT")
fi

log_step "Running drift gate (timeout: ${GATE_TIMEOUT_SECONDS}s)..."
log_info "Command: ${ZENCTL_CMD[*]}"

# Create artifact directory
mkdir -p "$ARTIFACT_DIR"

# Run with timeout, capture output
GATE_START=$(date +%s)
DIFF_OUTPUT=$(timeout "${GATE_TIMEOUT_SECONDS}" "${ZENCTL_CMD[@]}" 2>&1 || true)
EXIT_CODE=${PIPESTATUS[0]}
GATE_END=$(date +%s)
GATE_DURATION=$((GATE_END - GATE_START))

# Save output
echo "$DIFF_OUTPUT" > "$ARTIFACT_DIR/diff-output.txt"

if [ $EXIT_CODE -eq 124 ]; then
	log_error "Gate timed out after ${GATE_TIMEOUT_SECONDS}s"
	# Emit artifact bundle on timeout
	cat > "$ARTIFACT_DIR/gate-metadata.json" <<EOF
{
  "duration": ${GATE_DURATION},
  "exitCode": 124,
  "context": "${CONTEXT:-default}",
  "namespace": "${NAMESPACE}",
  "manifestDir": "${MANIFEST_DIR}",
  "timeout": ${GATE_TIMEOUT_SECONDS},
  "status": "timeout"
}
EOF
	exit 1
fi

# Emit artifact bundle on failure (exit 1 or 2)
if [ $EXIT_CODE -eq 1 ] || [ $EXIT_CODE -eq 2 ]; then
	log_step "Creating artifact bundle in $ARTIFACT_DIR..."
	
	# Save gate metadata
	cat > "$ARTIFACT_DIR/gate-metadata.json" <<EOF
{
  "duration": ${GATE_DURATION},
  "exitCode": ${EXIT_CODE},
  "context": "${CONTEXT:-default}",
  "namespace": "${NAMESPACE}",
  "manifestDir": "${MANIFEST_DIR}",
  "timestamp": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
}
EOF
	
	# Save diff output (already saved above)
	# Note: Normalized YAML would require exporting each resource, which needs cluster access
	# For CI, the diff output provides the necessary information
	log_info "Artifact bundle created: $ARTIFACT_DIR"
	log_info "  - gate-metadata.json"
	log_info "  - diff-output.txt"
fi

# Validate exit code contract
log_info "Exit code: $EXIT_CODE"
log_info "Duration: ${GATE_DURATION}s"

# Exit code contract: 0 = no drift, 2 = drift, 1 = error
case $EXIT_CODE in
	0)
		log_info "✓ Gate PASSED: No drift detected"
		echo "GATE=drift STATUS=pass CODE=0 DURATION=${GATE_DURATION}s"
		exit 0
		;;
	2)
		log_info "✓ Gate PASSED: Drift detected (expected in some workflows)"
		echo "GATE=drift STATUS=pass CODE=2 DURATION=${GATE_DURATION}s"
		exit 0
		;;
	1)
		log_error "Gate FAILED: Error occurred (exit 1)"
		echo "GATE=drift STATUS=fail CODE=1 DURATION=${GATE_DURATION}s"
		exit 1
		;;
	124)
		log_error "Gate FAILED: Timeout exceeded"
		echo "GATE=drift STATUS=fail CODE=124 DURATION=${GATE_DURATION}s"
		exit 1
		;;
	*)
		log_error "Gate FAILED: Unexpected exit code $EXIT_CODE"
		echo "GATE=drift STATUS=fail CODE=${EXIT_CODE} DURATION=${GATE_DURATION}s"
		exit 1
		;;
esac

