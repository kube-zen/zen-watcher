package output

import (
	"testing"
	"time"
)

func TestFormatEntitlement(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		reason   string
		expected string
	}{
		{"Entitled", "True", "", "Entitled"},
		{"Entitled with none", "True", "<none>", "Entitled"},
		{"Grace Period", "False", "GracePeriod", "Grace Period"},
		{"Expired", "False", "Expired", "Expired"},
		{"Not Entitled", "False", "NotEntitled", "Not Entitled"},
		{"Unknown status", "Unknown", "", "Unknown"},
		{"False without reason", "False", "", "Not Entitled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatEntitlement(tt.status, tt.reason)
			if result != tt.expected {
				t.Errorf("FormatEntitlement(%q, %q) = %q, want %q", tt.status, tt.reason, result, tt.expected)
			}
		})
	}
}

func TestFormatActiveTarget(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		target    string
		expected  string
	}{
		{"Both present", "ns", "target", "ns/target"},
		{"Only name", "", "target", "target"},
		{"Only namespace", "ns", "", "—"},
		{"Both empty", "", "", "—"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatActiveTarget(tt.namespace, tt.target)
			if result != tt.expected {
				t.Errorf("FormatActiveTarget(%q, %q) = %q, want %q", tt.namespace, tt.target, result, tt.expected)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{"Seconds", 30 * time.Second, "30s"},
		{"Minutes", 5 * time.Minute, "5m"},
		{"Hours", 2 * time.Hour, "2h"},
		{"Days", 3 * 24 * time.Hour, "3d"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatDuration(tt.duration)
			if result != tt.expected {
				t.Errorf("FormatDuration(%v) = %q, want %q", tt.duration, result, tt.expected)
			}
		})
	}
}

func TestParseFormat(t *testing.T) {
	tests := []struct {
		input    string
		expected Format
	}{
		{"table", FormatTable},
		{"TABLE", FormatTable},
		{"json", FormatJSON},
		{"JSON", FormatJSON},
		{"yaml", FormatYAML},
		{"YAML", FormatYAML},
		{"unknown", FormatTable},
		{"", FormatTable},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParseFormat(tt.input)
			if result != tt.expected {
				t.Errorf("ParseFormat(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

