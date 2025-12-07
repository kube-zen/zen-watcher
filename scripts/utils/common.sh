#!/bin/bash
#
# Common utilities for Zen Watcher scripts
# Source this file in other scripts: source "$(dirname "$0")/utils/common.sh"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Timing tracking
SCRIPT_START_TIME=$(date +%s)
SECTION_START_TIME=$(date +%s)

# Function to show elapsed time for a section
show_section_time() {
    local section_name="$1"
    local end_time=$(date +%s)
    local elapsed=$((end_time - SECTION_START_TIME))
    local total_elapsed=$((end_time - SCRIPT_START_TIME))
    local total_minutes=$((total_elapsed / 60))
    local total_seconds=$((total_elapsed % 60))
    
    if [ $total_minutes -gt 0 ]; then
        echo -e "${CYAN}   ⏱  ${section_name} took ${elapsed}s (total: ${total_minutes}m ${total_seconds}s)${NC}"
    else
        echo -e "${CYAN}   ⏱  ${section_name} took ${elapsed}s (total: ${total_seconds}s)${NC}"
    fi
    SECTION_START_TIME=$(date +%s)
}

# Function to show total elapsed time
show_total_time() {
    local end_time=$(date +%s)
    local total_elapsed=$((end_time - SCRIPT_START_TIME))
    local minutes=$((total_elapsed / 60))
    local seconds=$((total_elapsed % 60))
    if [ $minutes -gt 0 ]; then
        echo -e "${CYAN}⏱  Total time: ${minutes}m ${seconds}s${NC}"
    else
        echo -e "${CYAN}⏱  Total time: ${total_elapsed}s${NC}"
    fi
}

# Function to get script directory (works with symlinks)
get_script_dir() {
    local script_path="${BASH_SOURCE[0]}"
    while [ -L "$script_path" ]; do
        script_path=$(readlink "$script_path")
    done
    dirname "$(cd "$(dirname "$script_path")" && pwd -P)"
}

# Get project root directory
get_project_root() {
    local script_dir=$(get_script_dir)
    # Go up from scripts/utils/ to project root
    echo "$(cd "$script_dir/../.." && pwd -P)"
}

# Logging functions
log_info() {
    echo -e "${BLUE}ℹ${NC}  $1"
}

log_success() {
    echo -e "${GREEN}✓${NC}  $1"
}

log_warn() {
    echo -e "${YELLOW}⚠${NC}  $1"
}

log_error() {
    echo -e "${RED}✗${NC}  $1" >&2
}

log_step() {
    echo -e "${YELLOW}→${NC} $1"
}

# Check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Check if cluster exists
cluster_exists() {
    local platform="$1"
    local cluster_name="${2:-zen-demo}"
    
    case "$platform" in
        k3d)
            k3d cluster list 2>/dev/null | grep -q "^${cluster_name}" || return 1
            ;;
        kind)
            kind get clusters 2>/dev/null | grep -q "^${cluster_name}$" || return 1
            ;;
        minikube)
            minikube status -p "${cluster_name}" &>/dev/null || return 1
            ;;
        *)
            return 1
            ;;
    esac
}

# Get kubeconfig file path
get_kubeconfig() {
    local platform="$1"
    local cluster_name="${2:-zen-demo}"
    
    case "$platform" in
        k3d)
            echo "${HOME}/.kube/${cluster_name}-kubeconfig"
            ;;
        kind)
            kind get kubeconfig --name "${cluster_name}" 2>/dev/null
            ;;
        minikube)
            minikube kubectl --profile "${cluster_name}" -- config view --flatten 2>/dev/null
            ;;
        *)
            echo ""
            ;;
    esac
}

