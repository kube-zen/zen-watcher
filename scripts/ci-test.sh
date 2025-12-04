#!/bin/bash
#
# CI Test Script - Runs all tests, lints, and checks
# Usage: ./scripts/ci-test.sh
#
set -euo pipefail

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🧪 zen-watcher CI: Test Suite"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Change to repo root
cd "$(dirname "$0")/.."

EXIT_CODE=0

# 1. Code Formatting
echo "📝 Step 1: Code Formatting"
echo "────────────────────────────────────────────────────"
if ! gofmt -l . | grep -q .; then
    echo "  ✅ All Go files formatted"
else
    echo "  ❌ Code not formatted:"
    gofmt -l .
    EXIT_CODE=1
fi
echo ""

# 2. Go Vet
echo "🔍 Step 2: Go Vet (Static Analysis)"
echo "────────────────────────────────────────────────────"
if go vet ./...; then
    echo "  ✅ No issues found"
else
    echo "  ❌ Go vet found issues"
    EXIT_CODE=1
fi
echo ""

# 3. Unit Tests with Coverage
echo "🧪 Step 3: Unit Tests"
echo "────────────────────────────────────────────────────"
if go test ./... -coverprofile=coverage.out -covermode=atomic; then
    echo "  ✅ All tests passed"
    
    # Display coverage summary
    COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}')
    echo "  📊 Total Coverage: ${COVERAGE}"
    
    # Check coverage threshold (50% minimum for now)
    COVERAGE_NUM=$(echo "$COVERAGE" | sed 's/%//')
    if (( $(echo "$COVERAGE_NUM >= 50" | bc -l) )); then
        echo "  ✅ Coverage meets 50% threshold"
    else
        echo "  ⚠️  Coverage below 50% threshold"
        EXIT_CODE=1
    fi
else
    echo "  ❌ Tests failed"
    EXIT_CODE=1
fi
echo ""

# 4. Build Test
echo "🔨 Step 4: Build Test"
echo "────────────────────────────────────────────────────"
if go build -o /tmp/zen-watcher-test ./cmd/zen-watcher; then
    echo "  ✅ Build successful"
    rm -f /tmp/zen-watcher-test
else
    echo "  ❌ Build failed"
    EXIT_CODE=1
fi
echo ""

# 5. Shellcheck (if available)
echo "🐚 Step 5: Shell Script Linting"
echo "────────────────────────────────────────────────────"
if command -v shellcheck >/dev/null 2>&1; then
    if shellcheck hack/*.sh scripts/*.sh 2>/dev/null; then
        echo "  ✅ Shell scripts OK"
    else
        echo "  ⚠️  Shellcheck found issues (non-blocking)"
    fi
else
    echo "  ⚠️  shellcheck not installed, skipping"
fi
echo ""

# Summary
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
if [ $EXIT_CODE -eq 0 ]; then
    echo "✅ All CI checks passed!"
else
    echo "❌ CI checks failed - see errors above"
fi
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

exit $EXIT_CODE

