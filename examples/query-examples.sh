#!/bin/bash

# Zen Watcher - Query Examples
# These examples show how to query events

echo "======================================"
echo "Zen Watcher v1.0 - Query Examples"
echo "======================================"
echo ""

# 1. List all events
echo "1. List all events:"
kubectl get zenevents -n zen-system
echo ""

# 2. Filter by category
echo "2. Security events only:"
kubectl get zenevents -n zen-system -l category=security
echo ""

echo "3. Compliance events only:"
kubectl get zenevents -n zen-system -l category=compliance
echo ""

# 3. Filter by source
echo "4. Trivy vulnerability events:"
kubectl get zenrecommendations -n zen-system -l source=trivy
echo ""

echo "5. Falco runtime threat events:"
kubectl get zenrecommendations -n zen-system -l source=falco
echo ""

# 4. Filter by severity
echo "6. Critical events:"
kubectl get zenrecommendations -n zen-system -l severity=critical
echo ""

echo "7. High severity events:"
kubectl get zenrecommendations -n zen-system -l severity=high
echo ""

# 5. Combined filters
echo "8. Critical security events from Trivy:"
kubectl get zenrecommendations -n zen-system \
  -l category=security,severity=critical,source=trivy
echo ""

# 6. JSON output with jq
echo "9. Event summary (requires jq):"
kubectl get zenrecommendations -n zen-system -o json | \
  jq -r '.items[] | "\(.metadata.name): \(.spec.severity) - \(.spec.issue)"'
echo ""

# 7. Count events by source
echo "10. Count events by source:"
kubectl get zenrecommendations -n zen-system -o json | \
  jq -r '.items[] | .spec.source' | sort | uniq -c
echo ""

# 8. Count events by category
echo "11. Count events by category:"
kubectl get zenrecommendations -n zen-system -o json | \
  jq -r '.items[] | .spec.category' | sort | uniq -c
echo ""

# 9. Get event details
echo "12. Detailed view of first event:"
EVENT_NAME=$(kubectl get zenrecommendations -n zen-system -o name | head -1)
if [ ! -z "$EVENT_NAME" ]; then
  kubectl describe $EVENT_NAME -n zen-system
fi
echo ""

# 10. Watch for new events
echo "13. Watch for new events (Ctrl+C to stop):"
echo "kubectl get zenrecommendations -n zen-system --watch"
echo ""

# 11. Export events
echo "14. Export all events to file:"
echo "kubectl get zenrecommendations -n zen-system -o yaml > zen-events-backup.yaml"
echo ""

# 12. Query specific time range (requires custom timestamps)
echo "15. Events from last hour (using jq):"
ONE_HOUR_AGO=$(date -u -d '1 hour ago' +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -v-1H +%Y-%m-%dT%H:%M:%SZ)
kubectl get zenrecommendations -n zen-system -o json | \
  jq --arg time "$ONE_HOUR_AGO" '.items[] | select(.spec.timestamp > $time) | .metadata.name'
echo ""

echo "======================================"
echo "For more examples, see the documentation"
echo "======================================"

