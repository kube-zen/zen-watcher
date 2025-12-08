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
