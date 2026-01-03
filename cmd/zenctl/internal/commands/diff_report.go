package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// DiffReport represents the JSON report structure for drift detection
type DiffReport struct {
	SchemaVersion  string         `json:"schemaVersion"`
	GeneratedAt    string         `json:"generatedAt"`
	ClusterContext string         `json:"clusterContext"`
	Summary        ReportSummary  `json:"summary"`
	Resources      []ResourceReport `json:"resources"`
}

// ReportSummary contains aggregate statistics
type ReportSummary struct {
	Total        int `json:"total"`
	Drifted      int `json:"drifted"`
	NoDrift      int `json:"noDrift"`
	Errors       int `json:"errors"`
	SpecDrift    int `json:"specDrift"`
	MetadataDrift int `json:"metadataDrift"`
}

// ResourceReport contains information about a single resource
type ResourceReport struct {
	Group     string    `json:"group"`
	Version   string    `json:"version"`
	Kind      string    `json:"kind"`
	Namespace string    `json:"namespace"`
	Name      string    `json:"name"`
	Status    string    `json:"status"` // no_drift, drift, error
	DriftType string    `json:"driftType"` // spec, metadata, mixed, none, unknown
	DiffStats *DiffStats `json:"diffStats,omitempty"`
	Redacted  bool      `json:"redacted"`
	Error     string    `json:"error,omitempty"` // Present only if status=error
}

// DiffStats contains line counts for diff statistics
type DiffStats struct {
	Added   int `json:"added"`
	Removed int `json:"removed"`
	Changed int `json:"changed"`
}

// buildDiffReport constructs a DiffReport from comparison results
func buildDiffReport(ctx string, resources []ResourceReport) *DiffReport {
	summary := ReportSummary{
		Total: len(resources),
	}
	
	for _, res := range resources {
		switch res.Status {
		case "drift":
			summary.Drifted++
			if res.DriftType == "spec" {
				summary.SpecDrift++
			} else if res.DriftType == "metadata" {
				summary.MetadataDrift++
			} else if res.DriftType == "mixed" {
				summary.SpecDrift++
				summary.MetadataDrift++
			}
		case "no_drift":
			summary.NoDrift++
		case "error":
			summary.Errors++
		}
	}
	
	return &DiffReport{
		SchemaVersion:  "1.0",
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
		ClusterContext: ctx,
		Summary:        summary,
		Resources:      resources,
	}
}

// writeReportFile writes the report to a file atomically (temp file → fsync → rename)
func writeReportFile(report *DiffReport, filePath string) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create report directory: %w", err)
	}
	
	// Create temp file in same directory
	tmpFile, err := os.CreateTemp(dir, filepath.Base(filePath)+".tmp.")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	
	// Write JSON
	encoder := json.NewEncoder(tmpFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to encode report: %w", err)
	}
	
	// Sync to disk
	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to sync report: %w", err)
	}
	
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to close temp file: %w", err)
	}
	
	// Atomic rename
	if err := os.Rename(tmpPath, filePath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}
	
	return nil
}

// sortResources ensures deterministic ordering of resources
func sortResources(resources []ResourceReport) []ResourceReport {
	sorted := make([]ResourceReport, len(resources))
	copy(sorted, resources)
	
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Group != sorted[j].Group {
			return sorted[i].Group < sorted[j].Group
		}
		if sorted[i].Kind != sorted[j].Kind {
			return sorted[i].Kind < sorted[j].Kind
		}
		if sorted[i].Namespace != sorted[j].Namespace {
			return sorted[i].Namespace < sorted[j].Namespace
		}
		return sorted[i].Name < sorted[j].Name
	})
	
	return sorted
}

// calculateDiffStats calculates diff statistics from diff string
func calculateDiffStats(diff string) *DiffStats {
	stats := &DiffStats{}
	lines := splitLines(diff)
	
	for _, line := range lines {
		if len(line) > 0 {
			switch line[0] {
			case '+':
				if !strings.HasPrefix(line, "+++") {
					stats.Added++
				}
			case '-':
				if !strings.HasPrefix(line, "---") {
					stats.Removed++
				}
			}
		}
	}
	
	// Changed lines are approximated as the minimum of added/removed
	stats.Changed = stats.Added
	if stats.Removed < stats.Added {
		stats.Changed = stats.Removed
	}
	
	return stats
}

// splitLines splits a string into lines (handles both \n and \r\n)
func splitLines(s string) []string {
	if s == "" {
		return []string{}
	}
	return strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
}

// isSecretResource checks if a resource is a Secret
func isSecretResource(obj *unstructured.Unstructured) bool {
	if obj == nil {
		return false
	}
	return obj.GetKind() == "Secret"
}

