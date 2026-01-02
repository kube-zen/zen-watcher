package discovery

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestExpectedGVKs(t *testing.T) {
	// Verify expected GVKs are defined
	expected := []string{"DeliveryFlow", "Destination", "Ingester"}
	for _, name := range expected {
		if gvk, ok := ExpectedGVKs[name]; !ok {
			t.Errorf("ExpectedGVKs missing %s", name)
		} else {
			if gvk.Group == "" || gvk.Version == "" || gvk.Kind == "" {
				t.Errorf("ExpectedGVKs[%s] has empty fields: %+v", name, gvk)
			}
		}
	}
}

func TestExpectedGVKValues(t *testing.T) {
	// Verify specific GVK values match expected groups/versions
	tests := []struct {
		name         string
		expectedGroup string
		expectedVersion string
	}{
		{"DeliveryFlow", "routing.zen.kube-zen.io", "v1alpha1"},
		{"Destination", "routing.zen.kube-zen.io", "v1alpha1"},
		{"Ingester", "zen.kube-zen.io", "v1alpha1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gvk, ok := ExpectedGVKs[tt.name]
			if !ok {
				t.Fatalf("ExpectedGVKs missing %s", tt.name)
			}
			if gvk.Group != tt.expectedGroup {
				t.Errorf("ExpectedGVKs[%s].Group = %q, want %q", tt.name, gvk.Group, tt.expectedGroup)
			}
			if gvk.Version != tt.expectedVersion {
				t.Errorf("ExpectedGVKs[%s].Version = %q, want %q", tt.name, gvk.Version, tt.expectedVersion)
			}
		})
	}
}

func TestGVKString(t *testing.T) {
	// Test that GVKs can be converted to strings (for error messages)
	gvk := schema.GroupVersionKind{
		Group:   "test.group",
		Version: "v1",
		Kind:    "TestKind",
	}
	_ = gvk.String() // Should not panic
}

