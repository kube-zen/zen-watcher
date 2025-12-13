#!/bin/bash
#
# Generate Grafana dashboard ConfigMap(s) from JSON files
#
# Usage: 
#   ./generate-dashboard-configmap.sh <namespace> [dashboard-name]
#   If dashboard-name is provided, generates ConfigMap for that dashboard only
#   Otherwise, generates separate ConfigMap for each dashboard
#

set -euo pipefail

NAMESPACE="${1:-grafana}"
DASHBOARD_NAME="${2:-}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
DASHBOARD_DIR="${REPO_ROOT}/config/dashboards"

if [ ! -d "$DASHBOARD_DIR" ]; then
    echo "Error: Dashboard directory not found: $DASHBOARD_DIR" >&2
    exit 1
fi

# Function to generate a single dashboard ConfigMap
generate_dashboard_cm() {
    local dashboard_file="$1"
    local dashboard_name=$(basename "$dashboard_file" .json)
    local cm_name="grafana-dashboard-${dashboard_name}"
    
    cat <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: ${cm_name}
  namespace: ${NAMESPACE}
  labels:
    app: grafana
    grafana_dashboard: "1"
data:
  ${dashboard_name}.json: |
EOF
    
    # Minify JSON to reduce ConfigMap size (saves ~50% space)
    # Then indent by 4 spaces for YAML
    if command -v jq >/dev/null 2>&1; then
        jq -c . "$dashboard_file" | sed 's/^/    /'
    else
        # Fallback: use sed to remove extra whitespace (less effective)
        sed 's/^[[:space:]]*//; s/[[:space:]]*$//' "$dashboard_file" | tr -d '\n' | sed 's/^/    /'
    fi
    echo ""
}

# Generate ConfigMap(s)
if [ -n "$DASHBOARD_NAME" ]; then
    # Generate single dashboard ConfigMap
    dashboard_file="${DASHBOARD_DIR}/${DASHBOARD_NAME}.json"
    if [ -f "$dashboard_file" ]; then
        generate_dashboard_cm "$dashboard_file"
    else
        echo "Error: Dashboard file not found: $dashboard_file" >&2
        exit 1
    fi
else
    # Generate separate ConfigMap for each dashboard
    for dashboard_file in "$DASHBOARD_DIR"/*.json; do
        if [ -f "$dashboard_file" ]; then
            generate_dashboard_cm "$dashboard_file"
            echo "---"
        fi
    done
fi
