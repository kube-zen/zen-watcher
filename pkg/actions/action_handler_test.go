package actions

import (
	"testing"
	"time"
)

func TestTrivyActionHandler(t *testing.T) {
	handler := NewTrivyActionHandler()

	t.Run("CollectEvents", func(t *testing.T) {
		events := handler.CollectEvents()

		if events == nil {
			t.Error("Expected events slice, got nil")
		}

		t.Logf("Collected %d Trivy events", len(events))

		// Verify event structure
		for i, event := range events {
			if event.ID == "" {
				t.Errorf("Event %d: ID should not be empty", i)
			}
			if event.Source != "trivy" {
				t.Errorf("Event %d: Expected source 'trivy', got '%s'", i, event.Source)
			}
			if event.Type != "vulnerability" {
				t.Errorf("Event %d: Expected type 'vulnerability', got '%s'", i, event.Type)
			}
			if event.Timestamp.IsZero() {
				t.Errorf("Event %d: Timestamp should not be zero", i)
			}
		}
	})

	t.Run("GetName", func(t *testing.T) {
		name := handler.GetName()
		if name != "trivy" {
			t.Errorf("Expected name 'trivy', got '%s'", name)
		}
	})
}

func TestFalcoActionHandler(t *testing.T) {
	handler := NewFalcoActionHandler()

	t.Run("CollectEvents", func(t *testing.T) {
		events := handler.CollectEvents()

		if events == nil {
			t.Error("Expected events slice, got nil")
		}

		t.Logf("Collected %d Falco events", len(events))

		// Verify event structure
		for i, event := range events {
			if event.ID == "" {
				t.Errorf("Event %d: ID should not be empty", i)
			}
			if event.Source != "falco" {
				t.Errorf("Event %d: Expected source 'falco', got '%s'", i, event.Source)
			}
			if event.Type != "runtime" {
				t.Errorf("Event %d: Expected type 'runtime', got '%s'", i, event.Type)
			}
			if event.Timestamp.IsZero() {
				t.Errorf("Event %d: Timestamp should not be zero", i)
			}
		}
	})

	t.Run("GetName", func(t *testing.T) {
		name := handler.GetName()
		if name != "falco" {
			t.Errorf("Expected name 'falco', got '%s'", name)
		}
	})
}

func TestAuditActionHandler(t *testing.T) {
	handler := NewAuditActionHandler()

	t.Run("CollectEvents", func(t *testing.T) {
		events := handler.CollectEvents()

		if events == nil {
			t.Error("Expected events slice, got nil")
		}

		t.Logf("Collected %d Audit events", len(events))

		// Verify event structure
		for i, event := range events {
			if event.ID == "" {
				t.Errorf("Event %d: ID should not be empty", i)
			}
			if event.Source != "audit" {
				t.Errorf("Event %d: Expected source 'audit', got '%s'", i, event.Source)
			}
			if event.Type != "audit" {
				t.Errorf("Event %d: Expected type 'audit', got '%s'", i, event.Type)
			}
			if event.Timestamp.IsZero() {
				t.Errorf("Event %d: Timestamp should not be zero", i)
			}
		}
	})

	t.Run("GetName", func(t *testing.T) {
		name := handler.GetName()
		if name != "audit" {
			t.Errorf("Expected name 'audit', got '%s'", name)
		}
	})
}

func TestSecurityEvent(t *testing.T) {
	t.Run("CreateSecurityEvent", func(t *testing.T) {
		event := SecurityEvent{
			ID:          "test-event-1",
			Source:      "trivy",
			Type:        "vulnerability",
			Timestamp:   time.Now(),
			Severity:    "high",
			Namespace:   "default",
			Resource:    "test-pod",
			Description: "Test vulnerability",
			Details: map[string]interface{}{
				"cve":   "CVE-2023-1234",
				"score": 8.5,
			},
		}

		if event.ID != "test-event-1" {
			t.Errorf("Expected ID 'test-event-1', got '%s'", event.ID)
		}

		if event.Source != "trivy" {
			t.Errorf("Expected source 'trivy', got '%s'", event.Source)
		}

		if event.Severity != "high" {
			t.Errorf("Expected severity 'high', got '%s'", event.Severity)
		}
	})
}
