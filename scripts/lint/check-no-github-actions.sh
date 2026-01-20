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

# check-github-workflows.sh - Guardrail to verify GitHub Actions workflows for OSS projects
#
# This script verifies that OSS projects have .github/workflows/** as required.
# For OSS projects, workflows are REQUIRED, not prohibited.

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# Calculate repo root from script location (scripts/lint/ -> repo root)
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Check repo profile
if [ -f "${REPO_ROOT}/repo.profile" ]; then
  PROFILE=$(cat "${REPO_ROOT}/repo.profile" | tr -d '[:space:]')
else
  echo -e "${YELLOW}⚠️  No repo.profile found, assuming OSS profile${NC}"
  PROFILE="oss"
fi

echo "Checking GitHub Actions workflows (profile: ${PROFILE})..."

if [ "$PROFILE" = "oss" ]; then
  # OSS projects MUST have workflows
  if [ ! -d "${REPO_ROOT}/.github/workflows" ]; then
    echo ""
    echo -e "${RED}❌ .github/workflows/ directory is REQUIRED for OSS projects${NC}"
    echo ""
    echo -e "${YELLOW}OSS projects must have .github/workflows/ with at least one workflow file.${NC}"
    echo -e "${YELLOW}Policy token: OSS_WORKFLOWS_REQUIRED${NC}"
    echo ""
    exit 1
  fi
  
  WORKFLOW_FILES=$(find "${REPO_ROOT}/.github/workflows" -type f \( -name "*.yml" -o -name "*.yaml" \) 2>/dev/null | wc -l)
  if [ "$WORKFLOW_FILES" -eq 0 ]; then
    echo ""
    echo -e "${RED}❌ .github/workflows/ must contain at least one *.yml or *.yaml file${NC}"
    echo ""
    echo -e "${YELLOW}OSS projects must have active GitHub Actions workflows.${NC}"
    echo -e "${YELLOW}Policy token: OSS_WORKFLOWS_REQUIRED${NC}"
    echo ""
    exit 1
  fi
  
  echo -e "${GREEN}✅ Found ${WORKFLOW_FILES} workflow file(s) (required for OSS)${NC}"
  exit 0
elif [ "$PROFILE" = "platform" ]; then
  # Platform projects MUST NOT have workflows
  if [ -d "${REPO_ROOT}/.github/workflows" ]; then
    echo ""
    echo -e "${RED}❌ .github/workflows/ is FORBIDDEN for platform projects${NC}"
    echo ""
    echo -e "${RED}Found workflows:${NC}"
    find "${REPO_ROOT}/.github/workflows" -type f 2>/dev/null | while read -r file; do
      echo -e "${RED}  - $file${NC}"
    done
    echo ""
    echo -e "${YELLOW}Platform projects must NOT use GitHub Actions workflows.${NC}"
    echo -e "${YELLOW}Policy token: NEVER_ENABLE_.GITHUB_WORKFLOWS${NC}"
    echo ""
    exit 1
  fi
  echo -e "${GREEN}✅ No .github/workflows/ found (required for platform)${NC}"
  exit 0
else
  echo -e "${YELLOW}⚠️  Unknown profile: ${PROFILE}${NC}"
  exit 0
fi

