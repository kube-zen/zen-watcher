#!/bin/bash
#
# Generate Grafana dashboard ConfigMap from JSON files
#
# Usage: ./generate-dashboard-configmap.sh <namespace>
#

set -euo pipefail

NAMESPACE="${1:-grafana}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
DASHBOARD_DIR="${REPO_ROOT}/config/dashboards"

if [ ! -d "$DASHBOARD_DIR" ]; then
    echo "Error: Dashboard directory not found: $DASHBOARD_DIR" >&2
    exit 1
fi

cat <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: grafana-dashboards
  namespace: ${NAMESPACE}
  labels:
    app: grafana
data:
EOF

# Add each dashboard JSON file
for dashboard_file in "$DASHBOARD_DIR"/*.json; do
    if [ -f "$dashboard_file" ]; then
        dashboard_name=$(basename "$dashboard_file" .json)
        echo "  ${dashboard_name}.json: |"
        # Minify JSON to reduce ConfigMap size (saves ~50% space)
        # Then indent by 4 spaces for YAML
        if command -v jq >/dev/null 2>&1; then
            jq -c . "$dashboard_file" | sed 's/^/    /'
        else
            # Fallback: use sed to remove extra whitespace (less effective)
            sed 's/^[[:space:]]*//; s/[[:space:]]*$//' "$dashboard_file" | tr -d '\n' | sed 's/^/    /'
        fi
        echo ""
    fi
done

