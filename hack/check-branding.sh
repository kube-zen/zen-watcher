#!/bin/bash
# Copyright 2025 The Zen Watcher Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# check-branding.sh - Lint script to prevent executor-specific compatibility labels
#
# This script scans zen-watcher source and docs for forbidden phrases that claim
# compatibility with specific executors (e.g., "bridge-compatible", "portal-compatible").
#
# zen-watcher must present itself as a standalone OSS component; any cross-executor
# alignment is captured in program/common docs, not via labels inside the repo.

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

# Forbidden phrases (case-insensitive)
FORBIDDEN_PATTERNS=(
  "bridge-compatible"
  "Bridge compatible"
  "bridge compatible"
  "portal-compatible"
  "Portal compatible"
  "portal compatible"
  "hooks-compatible"
  "Hooks compatible"
  "hooks compatible"
)

# Directories to scan
SCAN_DIRS=(
  "."
)

# Files to exclude
EXCLUDE_PATTERNS=(
  ".git"
  "vendor"
  "node_modules"
  "bin"
  "build"
  "coverage.out"
  "hack/check-branding.sh"  # Exclude self
)

# Track if any matches found
FOUND=0
MATCHES=()

echo "Checking for executor-specific compatibility labels..."

# Build exclude pattern for find
EXCLUDE_ARGS=()
for pattern in "${EXCLUDE_PATTERNS[@]}"; do
  EXCLUDE_ARGS+=(-not -path "*/${pattern}/*" -not -name "${pattern}")
done

# Scan for forbidden patterns
for pattern in "${FORBIDDEN_PATTERNS[@]}"; do
  # Use grep with case-insensitive search
  while IFS= read -r file; do
    # Skip binary files
    if file "$file" | grep -q "text"; then
      # Check if file matches exclude patterns
      SKIP=0
      for exclude in "${EXCLUDE_PATTERNS[@]}"; do
        if [[ "$file" == *"$exclude"* ]]; then
          SKIP=1
          break
        fi
      done
      
      if [ $SKIP -eq 0 ]; then
        # Check for pattern (case-insensitive)
        if grep -qi "$pattern" "$file" 2>/dev/null; then
          FOUND=1
          MATCHES+=("$file: $pattern")
        fi
      fi
    fi
  done < <(find "${SCAN_DIRS[@]}" -type f "${EXCLUDE_ARGS[@]}" 2>/dev/null || true)
done

# Report results
if [ $FOUND -eq 1 ]; then
  echo ""
  echo -e "${RED}❌ Forbidden executor-specific compatibility labels found:${NC}"
  echo ""
  for match in "${MATCHES[@]}"; do
    echo -e "${RED}  - $match${NC}"
  done
  echo ""
  echo -e "${YELLOW}zen-watcher must not label features or examples as executor-compatible.${NC}"
  echo -e "${YELLOW}Cross-executor semantics belong in zen-admin/docs/common/, not in zen-watcher.${NC}"
  echo ""
  exit 1
else
  echo -e "${GREEN}✅ No forbidden compatibility labels found${NC}"
  exit 0
fi

