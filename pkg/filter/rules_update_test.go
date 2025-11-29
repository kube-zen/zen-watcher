package filter

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestFilter_UpdateConfig_NilConfig(t *testing.T) {
	// Create filter
	filterInstance := NewFilter(&FilterConfig{
		Sources: map[string]SourceFilter{
			"trivy": {
				MinSeverity: "MEDIUM",
			},
		},
	})

	// Update with nil config - should not panic
	filterInstance.UpdateConfig(nil)

	// Filter should still work (nil config means allow all)
	obs := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"source":   "trivy",
				"severity": "LOW",
			},
		},
	}
	allowed, _ := filterInstance.AllowWithReason(obs)
	if !allowed {
		t.Error("Nil config should allow all observations")
	}
}

func TestFilter_UpdateConfig_NilSourcesMap(t *testing.T) {
	// Create filter
	filterInstance := NewFilter(&FilterConfig{
		Sources: map[string]SourceFilter{
			"trivy": {
				MinSeverity: "MEDIUM",
			},
		},
	})

	// Update with config that has nil Sources map
	configWithNilSources := &FilterConfig{
		Sources: nil,
	}
	filterInstance.UpdateConfig(configWithNilSources)

	// Should not panic and should allow all (no sources configured)
	obs := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"source":   "trivy",
				"severity": "LOW",
			},
		},
	}
	allowed, _ := filterInstance.AllowWithReason(obs)
	if !allowed {
		t.Error("Nil Sources map should allow all observations")
	}
}
