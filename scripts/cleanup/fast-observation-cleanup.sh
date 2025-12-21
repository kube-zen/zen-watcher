#!/bin/bash
# Fast parallel observation cleanup with batching
#
# Usage:
#   ./fast-observation-cleanup.sh [namespace] [label-selector] [batch-size] [max-parallel]
#
# Examples:
#   ./fast-observation-cleanup.sh zen-system stress-test=true
#   ./fast-observation-cleanup.sh zen-system stress-test=true 50 10

set -euo pipefail

NAMESPACE="${1:-zen-system}"
LABEL_SELECTOR="${2:-stress-test=true}"
BATCH_SIZE="${3:-50}"
MAX_PARALLEL="${4:-10}"

# Get all observations matching label
OBS_LIST=$(kubectl get observations -n "$NAMESPACE" -l "$LABEL_SELECTOR" -o name 2>/dev/null || echo "")
OBS_COUNT=$(echo "$OBS_LIST" | grep -c "observation" || echo "0")

if [ "$OBS_COUNT" -eq 0 ]; then
    echo "No observations found to delete"
    exit 0
fi

echo "Deleting $OBS_COUNT observations in batches of $BATCH_SIZE (max $MAX_PARALLEL parallel)"

# Function to delete batch
delete_batch() {
    local batch="$1"
    echo "$batch" | xargs -I {} -P "$MAX_PARALLEL" kubectl delete {} -n "$NAMESPACE" --timeout=10s --ignore-not-found=true 2>/dev/null || true
}

# Create temporary directory for batch files
TMP_DIR=$(mktemp -d)
trap "rm -rf $TMP_DIR" EXIT

# Split into batches and process
echo "$OBS_LIST" | split -l "$BATCH_SIZE" - "$TMP_DIR/obs_batch_"

BATCH_COUNT=0
for batch_file in "$TMP_DIR"/obs_batch_*; do
    if [ -f "$batch_file" ] && [ -s "$batch_file" ]; then
        BATCH_COUNT=$((BATCH_COUNT + 1))
        echo "Processing batch $BATCH_COUNT: $(wc -l < "$batch_file") observations"
        delete_batch "$(cat "$batch_file")" &
        
        # Limit concurrent batch processes
        while [ $(jobs -r | wc -l) -ge 5 ]; do
            sleep 1
        done
    fi
done

# Wait for all batches to complete
wait
echo "Cleanup complete! Deleted $OBS_COUNT observations."

