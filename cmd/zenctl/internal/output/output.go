package output

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Format represents the output format
type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
	FormatYAML  Format = "yaml"
)

// ParseFormat parses an output format string
func ParseFormat(s string) Format {
	switch strings.ToLower(s) {
	case "json":
		return FormatJSON
	case "yaml":
		return FormatYAML
	default:
		return FormatTable
	}
}

// Printer handles output formatting
type Printer struct {
	format Format
}

// NewPrinter creates a new printer
func NewPrinter(format Format) *Printer {
	return &Printer{format: format}
}

// Format returns the printer format
func (p *Printer) Format() Format {
	return p.format
}

// Print prints data in the configured format
func (p *Printer) Print(data interface{}) error {
	switch p.format {
	case FormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(data)
	case FormatYAML:
		enc := yaml.NewEncoder(os.Stdout)
		defer enc.Close()
		return enc.Encode(data)
	default:
		return fmt.Errorf("unsupported format: %s", p.format)
	}
}

// FormatDuration formats a duration as a human-readable string
func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// FormatAge formats a time as a relative age
func FormatAge(t time.Time) string {
	return FormatDuration(time.Since(t))
}

// FormatEntitlement formats entitlement status and reason as a human-readable label
func FormatEntitlement(status, reason string) string {
	if status == "True" && (reason == "" || reason == "<none>") {
		return "Entitled"
	}
	if status == "False" {
		switch reason {
		case "GracePeriod":
			return "Grace Period"
		case "Expired":
			return "Expired"
		case "NotEntitled":
			return "Not Entitled"
		}
		return "Not Entitled"
	}
	return "Unknown"
}

// FormatActiveTarget formats active target as namespace/name or name
func FormatActiveTarget(namespace, name string) string {
	if namespace != "" && name != "" {
		return fmt.Sprintf("%s/%s", namespace, name)
	}
	if name != "" {
		return name
	}
	return "—"
}

