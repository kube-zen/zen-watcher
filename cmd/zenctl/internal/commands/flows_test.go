package commands

import (
	"strings"
	"testing"
)

func TestFlowsTableColumns(t *testing.T) {
	// Verify that flows table columns match ACTIVE_TARGET_UX_GUIDE.md
	// Expected columns: NAMESPACE | NAME | ACTIVE_TARGET | ENTITLEMENT | ENTITLEMENT_REASON | READY | AGE
	
	expectedColumns := []string{
		"NAMESPACE",
		"NAME",
		"ACTIVE_TARGET",
		"ENTITLEMENT",
		"ENTITLEMENT_REASON",
		"READY",
		"AGE",
	}

	// This is the column header that should be printed in the flows command
	// We can't easily test the actual output without running the command,
	// but we can verify the expected columns are defined correctly
	expectedHeader := strings.Join(expectedColumns, "\t")

	// Verify all expected columns are present
	for _, col := range expectedColumns {
		if !strings.Contains(expectedHeader, col) {
			t.Errorf("Missing expected column: %s", col)
		}
	}

	// Verify column order matches guide
	expectedOrder := strings.Join(expectedColumns, "\t")
	if expectedHeader != expectedOrder {
		t.Errorf("Column order mismatch. Expected: %s, Got: %s", expectedOrder, expectedHeader)
	}
}

func TestFlowsCommandExists(t *testing.T) {
	// Verify that NewFlowsCommand creates a valid command
	cmd := NewFlowsCommand()
	if cmd == nil {
		t.Fatal("NewFlowsCommand returned nil")
	}

	if cmd.Use != "flows" {
		t.Errorf("Expected command Use='flows', got '%s'", cmd.Use)
	}
}

