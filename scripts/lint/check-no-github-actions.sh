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

# check-no-github-actions.sh - Guardrail to prevent GitHub Actions workflows
#
# This script hard-fails if any file is added under .github/workflows/** in zen-watcher.

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# Calculate repo root from script location (scripts/lint/ -> repo root)
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

echo "Checking for GitHub Actions workflows..."

# Check for .github/workflows/** files
WORKFLOW_FILES=$(find "${REPO_ROOT}/.github" -type f -path "*/.github/workflows/*" 2>/dev/null || true)

if [ -n "$WORKFLOW_FILES" ]; then
  echo ""
  echo -e "${RED}❌ GitHub Actions workflows found in .github/workflows/ (prohibited)${NC}"
  echo ""
  echo -e "${RED}Found files:${NC}"
  echo "$WORKFLOW_FILES" | while read -r file; do
    echo -e "${RED}  - $file${NC}"
  done
  echo ""
  echo -e "${YELLOW}zen-watcher does not use GitHub Actions workflows.${NC}"
  echo -e "${YELLOW}Use CI entry point scripts (e.g., scripts/ci/zen-demo-validate.sh) instead.${NC}"
  echo ""
  exit 1
else
  echo -e "${GREEN}✅ No GitHub Actions workflows found${NC}"
  exit 0
fi

