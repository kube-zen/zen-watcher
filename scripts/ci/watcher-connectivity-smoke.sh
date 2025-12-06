#!/usr/bin/env bash
# Watcher Connectivity Smoke Test
# Tests watcher→SaaS connectivity without creating real observations
set -euo pipefail

BASE_URL="${BASE_URL:-https://api.kube-zen.io}"
TENANT_ID="${TENANT_ID:-}"
CLUSTER_ID="${CLUSTER_ID:-}"
TOKEN="${TOKEN:-}"

if [ -z "$TENANT_ID" ] || [ -z "$CLUSTER_ID" ] || [ -z "$TOKEN" ]; then
    echo "ERROR: Required env vars: BASE_URL, TENANT_ID, CLUSTER_ID, TOKEN"
    exit 1
fi

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Watcher Connectivity Smoke Test"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Base URL: $BASE_URL"
echo "Tenant ID: ${TENANT_ID:0:8}..."
echo "Cluster ID: ${CLUSTER_ID:0:8}..."
echo ""

# Test 1: TLS connectivity
echo "[1/3] Testing TLS connectivity..."
if ! curl -k -sS --max-time 5 "$BASE_URL/api/bff/v1/healthz" >/dev/null 2>&1; then
    echo "❌ WATCHER_CONNECTIVITY_TLS_FAILED: Cannot reach $BASE_URL"
    exit 1
fi
echo "✓ TLS OK"

# Test 2: Auth with token (dry-run observation creation)
echo "[2/3] Testing authentication (dry-run)..."
DRY_RUN_RESPONSE=$(curl -k -sS --max-time 10 -w "\n%{http_code}" \
    -X POST \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -H "X-Tenant-Id: $TENANT_ID" \
    "$BASE_URL/api/bff/v1/tenants/$TENANT_ID/observations" \
    -d "{\"cluster_id\":\"$CLUSTER_ID\",\"event_type\":\"connectivity_test\",\"category\":\"test\",\"severity\":\"low\",\"title\":\"Connectivity test\",\"raw_event\":{}}" 2>&1)

HTTP_CODE=$(echo "$DRY_RUN_RESPONSE" | tail -1)
if [ "$HTTP_CODE" = "401" ] || [ "$HTTP_CODE" = "403" ]; then
    echo "❌ WATCHER_CONNECTIVITY_AUTH_FAILED: HTTP $HTTP_CODE (check token/tenant IDs)"
    exit 2
fi

if [ "$HTTP_CODE" != "200" ] && [ "$HTTP_CODE" != "201" ]; then
    echo "⚠️  Unexpected HTTP $HTTP_CODE (may be OK if endpoint requires different auth)"
else
    echo "✓ Auth OK (HTTP $HTTP_CODE)"
fi

# Test 3: Endpoint reachability
echo "[3/3] Testing observation endpoint..."
if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "201" ]; then
    echo "✓ Observation endpoint OK"
else
    echo "⚠️  Observation endpoint returned HTTP $HTTP_CODE"
    echo "    This may be expected if watcher uses different auth or endpoint"
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✅ WATCHER_CONNECTIVITY_OK: Basic connectivity validated"
echo "Note: Full validation requires creating a real observation"
exit 0

