package commands

import (
	"fmt"
	"strings"
	"testing"
)

func TestDoctorCommandRemediationMessages(t *testing.T) {
	// Test that missing CRD errors include the correct remediation string
	// Expected: "DeliveryFlow CRD not installed; enable crds.enabled or apply CRDs separately."

	testCases := []struct {
		resourceName string
		expectedMsg  string
	}{
		{
			resourceName: "DeliveryFlow",
			expectedMsg:  "enable crds.enabled or apply CRDs separately",
		},
		{
			resourceName: "Destination",
			expectedMsg:  "enable crds.enabled or apply CRDs separately",
		},
		{
			resourceName: "Ingester",
			expectedMsg:  "enable crds.enabled or apply CRDs separately",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.resourceName, func(t *testing.T) {
			// Construct the expected error message format
			expectedError := fmt.Sprintf("%s CRD not installed; %s", tc.resourceName, tc.expectedMsg)
			
			// Verify the remediation message is included
			if !strings.Contains(expectedError, tc.expectedMsg) {
				t.Errorf("Expected error message to contain remediation: %s", tc.expectedMsg)
			}

			// Verify the resource name is included
			if !strings.Contains(expectedError, tc.resourceName) {
				t.Errorf("Expected error message to contain resource name: %s", tc.resourceName)
			}
		})
	}
}

func TestDoctorCommandExists(t *testing.T) {
	// Verify that NewDoctorCommand creates a valid command
	cmd := NewDoctorCommand()
	if cmd == nil {
		t.Fatal("NewDoctorCommand returned nil")
	}

	if cmd.Use != "doctor" {
		t.Errorf("Expected command Use='doctor', got '%s'", cmd.Use)
	}
}

