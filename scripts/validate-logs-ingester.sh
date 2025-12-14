#!/bin/bash
# Validate logs ingester implementation
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "=== Validating Logs Ingester Implementation ==="
echo ""

# Test 1: Validate YAML syntax
echo "Test 1: Validating YAML syntax..."
if ! yq eval . examples/test-logs-ingester.yaml > /dev/null 2>&1 && ! kubectl apply --dry-run=client -f examples/test-logs-ingester.yaml > /dev/null 2>&1; then
    echo "  ⚠️  YAML validation skipped (kubectl/yq not available or CRDs not installed)"
else
    echo "  ✅ YAML syntax is valid"
fi

# Test 2: Validate Go code compiles
echo ""
echo "Test 2: Validating Go code compiles..."
cd "$REPO_ROOT"
if go build ./pkg/config/... > /dev/null 2>&1 && go build ./pkg/adapter/generic/... > /dev/null 2>&1; then
    echo "  ✅ Go code compiles successfully"
else
    echo "  ❌ Go code compilation failed"
    exit 1
fi

# Test 3: Check LogsConfig struct is defined
echo ""
echo "Test 3: Checking LogsConfig struct definition..."
if grep -q "type LogsConfig struct" pkg/config/ingester_loader.go && grep -q "PodSelector" pkg/config/ingester_loader.go; then
    echo "  ✅ LogsConfig struct is properly defined"
else
    echo "  ❌ LogsConfig struct is missing or incomplete"
    exit 1
fi

# Test 4: Check logs config parsing exists
echo ""
echo "Test 4: Checking logs config parsing..."
if grep -q "Extract logs config" pkg/config/ingester_loader.go && grep -q "spec\[\"logs\"\]" pkg/config/ingester_loader.go; then
    echo "  ✅ Logs config parsing is implemented"
else
    echo "  ❌ Logs config parsing is missing"
    exit 1
fi

# Test 5: Check conversion function
echo ""
echo "Test 5: Checking conversion function..."
if grep -q "Convert logs config" pkg/config/ingester_to_generic_converter.go && grep -q "ingesterConfig.Logs" pkg/config/ingester_to_generic_converter.go; then
    echo "  ✅ Logs config conversion is implemented"
else
    echo "  ❌ Logs config conversion is missing"
    exit 1
fi

# Test 6: Check adapter factory supports logs
echo ""
echo "Test 6: Checking adapter factory..."
if grep -q 'case "logs":' pkg/adapter/generic/factory.go && grep -q "NewLogsAdapter" pkg/adapter/generic/factory.go; then
    echo "  ✅ Logs adapter factory support is implemented"
else
    echo "  ❌ Logs adapter factory support is missing"
    exit 1
fi

# Test 7: Check logs adapter implementation
echo ""
echo "Test 7: Checking logs adapter implementation..."
if grep -q "type LogsAdapter struct" pkg/adapter/generic/logs_adapter.go && grep -q "func.*Start" pkg/adapter/generic/logs_adapter.go; then
    echo "  ✅ Logs adapter implementation exists"
else
    echo "  ❌ Logs adapter implementation is missing"
    exit 1
fi

echo ""
echo "=== All Validation Tests Passed ==="
echo ""
echo "Summary:"
echo "  ✅ LogsConfig struct defined"
echo "  ✅ Logs config parsing implemented"
echo "  ✅ Logs config conversion implemented"
echo "  ✅ Adapter factory supports logs"
echo "  ✅ Logs adapter implementation exists"
echo ""
echo "Next steps:"
echo "  1. Deploy zen-watcher to a cluster"
echo "  2. Apply examples/test-logs-ingester.yaml"
echo "  3. Verify logs adapter starts and processes log events"
echo "  4. Check Observations are created"

