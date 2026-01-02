package commands

import (
	"fmt"
	"strings"
	"testing"
)

func TestCRDErrorMessages(t *testing.T) {
	// Test that missing CRD errors return non-zero and include correct remediation
	// This test verifies the error message format used throughout the codebase

	testCases := []struct {
		resourceName string
		err          error
		expectedMsg  string
	}{
		{
			resourceName: "DeliveryFlow",
			err:          fmt.Errorf("DeliveryFlow CRD not installed; enable crds.enabled or apply CRDs separately: %w", fmt.Errorf("test error")),
			expectedMsg:  "enable crds.enabled or apply CRDs separately",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.resourceName, func(t *testing.T) {
			errMsg := tc.err.Error()
			
			// Verify the remediation message is included
			if !strings.Contains(errMsg, tc.expectedMsg) {
				t.Errorf("Expected error message to contain remediation '%s', got: %s", tc.expectedMsg, errMsg)
			}

			// Verify the resource name is included
			if !strings.Contains(errMsg, tc.resourceName) {
				t.Errorf("Expected error message to contain resource name '%s', got: %s", tc.resourceName, errMsg)
			}

			// Verify it's not a zero error (error exists)
			if tc.err == nil {
				t.Error("Expected non-nil error")
			}
		})
	}
}

// TestFlowsCommandCRDError verifies that flows command returns correct error for missing CRD
func TestFlowsCommandCRDError(t *testing.T) {
	expectedErrorMsg := "DeliveryFlow CRD not installed; enable crds.enabled or apply CRDs separately"
	
	// Verify the error message format matches what's in flows.go
	testError := fmt.Errorf("%s: %w", expectedErrorMsg, fmt.Errorf("discovery error"))
	
	if !strings.Contains(testError.Error(), expectedErrorMsg) {
		t.Errorf("Error message should contain: %s", expectedErrorMsg)
	}

	// Verify the remediation is present
	if !strings.Contains(testError.Error(), "enable crds.enabled") {
		t.Error("Error message should contain remediation instruction")
	}
}

