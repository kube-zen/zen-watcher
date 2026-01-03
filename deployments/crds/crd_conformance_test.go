// Copyright 2025 The Zen Watcher Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package crds

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestIngesterCRD_ValidManifest tests that a valid Ingester manifest passes validation
func TestIngesterCRD_ValidManifest(t *testing.T) {
	validManifest := `
apiVersion: zen.kube-zen.io/v1
kind: Ingester
metadata:
  name: test-ingester
  namespace: default
spec:
  source: trivy
  ingester: informer
  destinations:
    - type: crd
      value: observations
  informer:
    gvr:
      group: aquasecurity.github.io
      version: v1alpha1
      resource: vulnerabilityreports
  normalization:
    domain: security
    type: vulnerability
    priority:
      HIGH: 0.8
      MEDIUM: 0.5
      LOW: 0.3
`

	err := validateManifest(t, validManifest, "ingester")
	if err != nil {
		t.Fatalf("Valid Ingester manifest should pass validation: %v", err)
	}
}

// TestIngesterCRD_InvalidSource tests that invalid source pattern is rejected
func TestIngesterCRD_InvalidSource(t *testing.T) {
	invalidManifest := `
apiVersion: zen.kube-zen.io/v1
kind: Ingester
metadata:
  name: test-ingester
  namespace: default
spec:
  source: Invalid_Source  # Invalid: uppercase and underscore
  ingester: informer
  destinations:
    - type: crd
      value: observations
`

	err := validateManifest(t, invalidManifest, "ingester")
	if err == nil {
		t.Error("Invalid source pattern should be rejected by validation")
	}
}

// TestIngesterCRD_MissingRequiredFields tests that missing required fields are rejected
func TestIngesterCRD_MissingRequiredFields(t *testing.T) {
	invalidManifest := `
apiVersion: zen.kube-zen.io/v1
kind: Ingester
metadata:
  name: test-ingester
  namespace: default
spec:
  source: trivy
  # Missing: ingester (required)
  # Missing: destinations (required)
`

	err := validateManifest(t, invalidManifest, "ingester")
	if err == nil {
		t.Error("Missing required fields should be rejected by validation")
	}
}

// TestIngesterCRD_InvalidIngesterType tests that invalid ingester type enum is rejected
func TestIngesterCRD_InvalidIngesterType(t *testing.T) {
	invalidManifest := `
apiVersion: zen.kube-zen.io/v1
kind: Ingester
metadata:
  name: test-ingester
  namespace: default
spec:
  source: trivy
  ingester: invalid-type  # Invalid: not in enum
  destinations:
    - type: crd
      value: observations
`

	err := validateManifest(t, invalidManifest, "ingester")
	if err == nil {
		t.Error("Invalid ingester type enum should be rejected by validation")
	}
}

// TestIngesterCRD_InvalidDestinationType tests that invalid destination type enum is rejected
func TestIngesterCRD_InvalidDestinationType(t *testing.T) {
	invalidManifest := `
apiVersion: zen.kube-zen.io/v1
kind: Ingester
metadata:
  name: test-ingester
  namespace: default
spec:
  source: trivy
  ingester: informer
  destinations:
    - type: invalid-destination  # Invalid: not in enum
      value: observations
`

	err := validateManifest(t, invalidManifest, "ingester")
	if err == nil {
		t.Error("Invalid destination type enum should be rejected by validation")
	}
}

// TestObservationCRD_ValidManifest tests that a valid Observation manifest passes validation
func TestObservationCRD_ValidManifest(t *testing.T) {
	validManifest := `
apiVersion: zen.kube-zen.io/v1
kind: Observation
metadata:
  name: test-observation
  namespace: default
spec:
  source: trivy
  category: security
  severity: HIGH
  eventType: vulnerability
  detectedAt: "2025-01-15T10:00:00Z"
`

	err := validateManifest(t, validManifest, "observation")
	if err != nil {
		t.Fatalf("Valid Observation manifest should pass validation: %v", err)
	}
}

// TestObservationCRD_InvalidCategory tests that invalid category enum is rejected
func TestObservationCRD_InvalidCategory(t *testing.T) {
	invalidManifest := `
apiVersion: zen.kube-zen.io/v1
kind: Observation
metadata:
  name: test-observation
  namespace: default
spec:
  source: trivy
  category: invalid-category  # Invalid: not in enum
  severity: HIGH
  eventType: vulnerability
  detectedAt: "2025-01-15T10:00:00Z"
`

	err := validateManifest(t, invalidManifest, "observation")
	if err == nil {
		t.Error("Invalid category enum should be rejected by validation")
	}
}

// TestObservationCRD_InvalidSeverity tests that invalid severity enum is rejected
func TestObservationCRD_InvalidSeverity(t *testing.T) {
	invalidManifest := `
apiVersion: zen.kube-zen.io/v1
kind: Observation
metadata:
  name: test-observation
  namespace: default
spec:
  source: trivy
  category: security
  severity: INVALID  # Invalid: not in enum
  eventType: vulnerability
  detectedAt: "2025-01-15T10:00:00Z"
`

	err := validateManifest(t, invalidManifest, "observation")
	if err == nil {
		t.Error("Invalid severity enum should be rejected by validation")
	}
}

// TestObservationCRD_InvalidEventTypePattern tests that invalid eventType pattern is rejected
func TestObservationCRD_InvalidEventTypePattern(t *testing.T) {
	invalidManifest := `
apiVersion: zen.kube-zen.io/v1
kind: Observation
metadata:
  name: test-observation
  namespace: default
spec:
  source: trivy
  category: security
  severity: HIGH
  eventType: Invalid-Event-Type  # Invalid: uppercase and hyphens
  detectedAt: "2025-01-15T10:00:00Z"
`

	err := validateManifest(t, invalidManifest, "observation")
	if err == nil {
		t.Error("Invalid eventType pattern should be rejected by validation")
	}
}

// validateManifest uses kubectl apply --dry-run=client to validate a manifest
// isCRDNotAvailableError checks if the error output indicates CRD is not available
func isCRDNotAvailableError(outputStr string) bool {
	return strings.Contains(outputStr, "no matches for kind") ||
		strings.Contains(outputStr, "the server could not find the requested resource") ||
		strings.Contains(outputStr, "resource mapping not found") ||
		strings.Contains(outputStr, "ensure CRDs are installed") ||
		strings.Contains(outputStr, "NotFound") ||
		(strings.Contains(outputStr, "error:") && (strings.Contains(outputStr, "not found") || strings.Contains(outputStr, "does not exist"))) ||
		(strings.Contains(outputStr, "context") && (strings.Contains(outputStr, "not found") || strings.Contains(outputStr, "does not exist")))
}

// isValidationError checks if the error output indicates a validation failure
func isValidationError(outputStr string) bool {
	return strings.Contains(outputStr, "validation") ||
		strings.Contains(outputStr, "invalid") ||
		strings.Contains(outputStr, "required") ||
		strings.Contains(outputStr, "must be") ||
		strings.Contains(outputStr, "field is immutable")
}

// tryClientSideValidation attempts client-side validation as a fallback
func tryClientSideValidation(t *testing.T, tmpFile string) error {
	cmd := exec.Command("kubectl", "apply", "--dry-run=client", "-f", tmpFile) //nolint:gosec // G204: kubectl is trusted test tool
	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := string(output)
		if isCRDNotAvailableError(outputStr) {
			t.Skipf("CRD not available for validation (expected in unit tests): %s", outputStr)
			return nil
		}
		if isValidationError(outputStr) {
			return err
		}
		t.Skipf("kubectl validation not available: %s", outputStr)
		return nil
	}
	return nil
}

func validateManifest(t *testing.T, manifest string, crdType string) error {
	// Write manifest to temp file
	tmpFile := filepath.Join(t.TempDir(), "manifest.yaml")
	err := os.WriteFile(tmpFile, []byte(manifest), 0644) //nolint:gosec // G306: 0644 is standard for test files
	if err != nil {
		return err
	}

	// Run kubectl apply --dry-run=server
	// Use server-side validation if available, otherwise client-side
	cmd := exec.Command("kubectl", "apply", "--dry-run=server", "-f", tmpFile) //nolint:gosec // G204: kubectl is trusted test tool
	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := string(output)
		if isCRDNotAvailableError(outputStr) {
			// Fall back to client-side validation
			return tryClientSideValidation(t, tmpFile)
		}
		if isValidationError(outputStr) {
			return err
		}
		// For any other error (CRD not found, context issues, connection problems, etc.) - skip test
		t.Skipf("kubectl validation not available (CRD may not be installed): %s", outputStr)
		return nil
	}

	return nil
}
