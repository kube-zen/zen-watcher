# zen-watcher Makefile
# Provides common development and build tasks

.PHONY: help build test lint security docker-build docker-scan install-tools clean

# Default target
.DEFAULT_GOAL := help

# Variables
VERSION ?= 1.1.0
COMMIT ?= $(shell git rev-parse --short HEAD)
BUILD_DATE ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
IMAGE_NAME ?= kubezen/zen-watcher
IMAGE_TAG ?= $(VERSION)
# Path to helm-charts repository (relative to zen-watcher root)
CHARTS_REPO ?= ../helm-charts

# Colors for output
GREEN := \033[0;32m
YELLOW := \033[0;33m
RED := \033[0;31m
NC := \033[0m # No Color

## help: Display this help message
help:
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "zen-watcher Makefile"
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo ""
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /' | column -t -s ':'
	@echo ""

## build: Build the zen-watcher binary
build:
	@echo "$(GREEN)Building zen-watcher...$(NC)"
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
		-ldflags="-w -s -X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildDate=$(BUILD_DATE)" \
		-tags netgo \
		-trimpath \
		-o zen-watcher \
		./cmd/zen-watcher
	@ls -lh zen-watcher
	@echo "$(GREEN)✅ Build complete$(NC)"

## test: Run all tests
test:
	@echo "$(GREEN)Running tests...$(NC)"
	go test -v -race -coverprofile=coverage.out ./...
	@echo "$(GREEN)✅ Tests complete$(NC)"

## lint: Run all linters
lint: fmt vet staticcheck

## fmt: Run go fmt
fmt:
	@echo "$(GREEN)Running go fmt...$(NC)"
	@UNFORMATTED=$$(gofmt -l .); \
	if [ -n "$$UNFORMATTED" ]; then \
		echo "$(RED)❌ Code not formatted:$(NC)"; \
		echo "$$UNFORMATTED"; \
		echo "$(YELLOW)Run: gofmt -w .$(NC)"; \
		exit 1; \
	fi
	@echo "$(GREEN)✅ Code formatted$(NC)"

## vet: Run go vet
vet:
	@echo "$(GREEN)Running go vet...$(NC)"
	@go vet ./...
	@echo "$(GREEN)✅ go vet passed$(NC)"

## staticcheck: Run staticcheck linter
staticcheck:
	@echo "$(GREEN)Running staticcheck...$(NC)"
	@if ! command -v staticcheck &> /dev/null; then \
		echo "$(YELLOW)ℹ️  staticcheck not installed$(NC)"; \
		echo "   Install: go install honnef.co/go/tools/cmd/staticcheck@latest"; \
	else \
		staticcheck ./...; \
		echo "$(GREEN)✅ staticcheck passed$(NC)"; \
	fi

## security: Run security scans
security: vuln gosec

## vuln: Check for vulnerabilities with govulncheck
vuln:
	@echo "$(GREEN)Checking for vulnerabilities...$(NC)"
	@if ! command -v govulncheck &> /dev/null; then \
		echo "$(YELLOW)ℹ️  govulncheck not installed$(NC)"; \
		echo "   Install: go install golang.org/x/vuln/cmd/govulncheck@latest"; \
	else \
		govulncheck ./...; \
		echo "$(GREEN)✅ No known vulnerabilities$(NC)"; \
	fi

## gosec: Run gosec security scanner
gosec:
	@echo "$(GREEN)Running gosec security scan...$(NC)"
	@if ! command -v gosec &> /dev/null; then \
		echo "$(YELLOW)ℹ️  gosec not installed$(NC)"; \
		echo "   Install: go install github.com/securego/gosec/v2/cmd/gosec@latest"; \
	else \
		gosec -quiet ./...; \
		echo "$(GREEN)✅ gosec passed$(NC)"; \
	fi

## docker-build: Build Docker image
docker-build:
	@echo "$(GREEN)Building Docker image...$(NC)"
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t $(IMAGE_NAME):$(IMAGE_TAG) \
		-t $(IMAGE_NAME):latest \
		-f build/Dockerfile \
		.
	@echo "$(GREEN)✅ Docker image built: $(IMAGE_NAME):$(IMAGE_TAG)$(NC)"

## docker-scan: Scan Docker image for vulnerabilities
docker-scan:
	@echo "$(GREEN)Scanning Docker image...$(NC)"
	@if ! command -v trivy &> /dev/null; then \
		echo "$(YELLOW)ℹ️  Trivy not installed$(NC)"; \
		echo "   Install: brew install trivy (or apt install trivy)"; \
	else \
		echo "  → Running Trivy scan (HIGH/CRITICAL only)..."; \
		trivy image --severity HIGH,CRITICAL --exit-code 1 $(IMAGE_NAME):$(IMAGE_TAG); \
		echo "$(GREEN)✅ No HIGH/CRITICAL vulnerabilities$(NC)"; \
	fi

## docker-sbom: Generate SBOM for Docker image
docker-sbom:
	@echo "$(GREEN)Generating SBOM...$(NC)"
	@if ! command -v syft &> /dev/null; then \
		echo "$(YELLOW)ℹ️  Syft not installed$(NC)"; \
		echo "   Install: brew install syft"; \
	else \
		syft $(IMAGE_NAME):$(IMAGE_TAG) -o json > zen-watcher-sbom.json; \
		syft $(IMAGE_NAME):$(IMAGE_TAG) -o spdx-json > zen-watcher-sbom.spdx.json; \
		echo "$(GREEN)✅ SBOM generated:$(NC)"; \
		echo "   - zen-watcher-sbom.json (Syft format)"; \
		echo "   - zen-watcher-sbom.spdx.json (SPDX format)"; \
	fi

## docker-sign: Sign Docker image with Cosign
docker-sign:
	@echo "$(GREEN)Signing Docker image with Cosign...$(NC)"
	@if ! command -v cosign &> /dev/null; then \
		echo "$(YELLOW)ℹ️  Cosign not installed$(NC)"; \
		echo "   Install: brew install cosign"; \
	else \
		if [ ! -f "cosign.key" ]; then \
			echo "$(YELLOW)ℹ️  No cosign.key found$(NC)"; \
			echo "   Generate: cosign generate-key-pair"; \
		else \
			cosign sign --key cosign.key $(IMAGE_NAME):$(IMAGE_TAG); \
			echo "$(GREEN)✅ Image signed$(NC)"; \
		fi \
	fi

## docker-verify: Verify Docker image signature
docker-verify:
	@echo "$(GREEN)Verifying Docker image signature...$(NC)"
	@if ! command -v cosign &> /dev/null; then \
		echo "$(YELLOW)ℹ️  Cosign not installed$(NC)"; \
	else \
		if [ ! -f "cosign.pub" ]; then \
			echo "$(YELLOW)ℹ️  No cosign.pub found$(NC)"; \
		else \
			cosign verify --key cosign.pub $(IMAGE_NAME):$(IMAGE_TAG); \
			echo "$(GREEN)✅ Signature verified$(NC)"; \
		fi \
	fi

## docker-all: Build, scan, generate SBOM, and sign Docker image
docker-all: docker-build docker-scan docker-sbom
	@echo "$(GREEN)✅ Docker image ready for push$(NC)"

## install-tools: Install development tools
install-tools:
	@echo "$(GREEN)Installing development tools...$(NC)"
	go install golang.org/x/vuln/cmd/govulncheck@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	go install honnef.co/go/tools/cmd/staticcheck@latest
	@echo "$(GREEN)✅ Tools installed$(NC)"
	@echo ""
	@echo "Additional tools (install manually):"
	@echo "  - Trivy: https://trivy.dev/latest/getting-started/installation/"
	@echo "  - Syft: https://github.com/anchore/syft#installation"
	@echo "  - Cosign: https://docs.sigstore.dev/cosign/installation/"

## install-hooks: Install git hooks
install-hooks:
	@echo "$(GREEN)Installing git hooks...$(NC)"
	@mkdir -p .githooks
	@if [ -f ".githooks/pre-commit" ]; then \
		chmod +x .githooks/pre-commit; \
		git config core.hooksPath .githooks; \
		echo "$(GREEN)✅ Git hooks installed$(NC)"; \
	else \
		echo "$(RED)❌ .githooks/pre-commit not found$(NC)"; \
		exit 1; \
	fi

## clean: Clean build artifacts
clean:
	@echo "$(GREEN)Cleaning build artifacts...$(NC)"
	rm -f zen-watcher
	rm -f coverage.out
	rm -f zen-watcher-sbom*.json
	rm -rf dist/
	go clean -cache -testcache -modcache
	@echo "$(GREEN)✅ Clean complete$(NC)"

## all: Run all checks (lint, test, security, build)
all: lint test security build
	@echo ""
	@echo "$(GREEN)━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(NC)"
	@echo "$(GREEN)✅ All checks passed!$(NC)"
	@echo "$(GREEN)━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(NC)"

## ci: Run CI pipeline (all checks + Docker build)
ci: all docker-all
	@echo "$(GREEN)✅ CI pipeline complete$(NC)"

## sync-crd-to-chart: Sync Observation CRD to helm-charts repository
sync-crd-to-chart:
	@echo "$(GREEN)Syncing Observation CRD to helm-charts repository...$(NC)"
	@if [ ! -d "$(CHARTS_REPO)" ]; then \
		echo "$(RED)❌ Helm charts repo not found at $(CHARTS_REPO)$(NC)"; \
		echo "   Set CHARTS_REPO environment variable to the correct path"; \
		exit 1; \
	fi
	@echo "# DO NOT EDIT THIS FILE MANUALLY" > $(CHARTS_REPO)/charts/zen-watcher/templates/observation_crd.yaml
	@echo "# This file is automatically synced from the canonical source:" >> $(CHARTS_REPO)/charts/zen-watcher/templates/observation_crd.yaml
	@echo "# https://github.com/kube-zen/zen-watcher/blob/main/deployments/crds/observation_crd.yaml" >> $(CHARTS_REPO)/charts/zen-watcher/templates/observation_crd.yaml
	@echo "#" >> $(CHARTS_REPO)/charts/zen-watcher/templates/observation_crd.yaml
	@echo "# To update this CRD:" >> $(CHARTS_REPO)/charts/zen-watcher/templates/observation_crd.yaml
	@echo "# 1. Make changes in the zen-watcher repository (deployments/crds/observation_crd.yaml)" >> $(CHARTS_REPO)/charts/zen-watcher/templates/observation_crd.yaml
	@echo "# 2. Run 'make sync-crd-to-chart' from the zen-watcher repository" >> $(CHARTS_REPO)/charts/zen-watcher/templates/observation_crd.yaml
	@echo "# 3. Commit the change in this repository" >> $(CHARTS_REPO)/charts/zen-watcher/templates/observation_crd.yaml
	@echo "#" >> $(CHARTS_REPO)/charts/zen-watcher/templates/observation_crd.yaml
	@echo "# See: https://github.com/kube-zen/zen-watcher/blob/main/docs/CRD.md" >> $(CHARTS_REPO)/charts/zen-watcher/templates/observation_crd.yaml
	@echo "" >> $(CHARTS_REPO)/charts/zen-watcher/templates/observation_crd.yaml
	@cat deployments/crds/observation_crd.yaml >> $(CHARTS_REPO)/charts/zen-watcher/templates/observation_crd.yaml
	@echo "$(GREEN)✅ CRD synced to $(CHARTS_REPO)/charts/zen-watcher/templates/observation_crd.yaml$(NC)"
	@echo "$(YELLOW)⚠️  Remember to commit the change in the helm-charts repository$(NC)"

## check-crd-drift: Check if CRD in helm-charts repo differs from canonical
check-crd-drift:
	@echo "$(GREEN)Checking for CRD drift...$(NC)"
	@if [ ! -d "$(CHARTS_REPO)" ]; then \
		echo "$(RED)❌ Helm charts repo not found at $(CHARTS_REPO)$(NC)"; \
		exit 1; \
	fi
	@# Compare CRD content (ignoring header comments in helm-charts version)
	@TEMP_FILE=$$(mktemp); \
	sed '/^# DO NOT EDIT/,/^$$/d' $(CHARTS_REPO)/charts/zen-watcher/templates/observation_crd.yaml > $$TEMP_FILE; \
	if diff -q deployments/crds/observation_crd.yaml $$TEMP_FILE > /dev/null 2>&1; then \
		echo "$(GREEN)✅ CRDs are in sync$(NC)"; \
		rm -f $$TEMP_FILE; \
	else \
		echo "$(RED)❌ CRD drift detected!$(NC)"; \
		echo "   Run 'make sync-crd-to-chart' to sync"; \
		diff -u deployments/crds/observation_crd.yaml $$TEMP_FILE || true; \
		rm -f $$TEMP_FILE; \
		exit 1; \
	fi

