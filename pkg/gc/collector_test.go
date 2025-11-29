package gc

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestCollector_shouldDeleteObservation(t *testing.T) {
	now := time.Now()
	oldTime := now.AddDate(0, 0, -8)    // 8 days ago
	recentTime := now.AddDate(0, 0, -1) // 1 day ago

	tests := []struct {
		name          string
		obs           unstructured.Unstructured
		defaultCutoff time.Time
		wantDelete    bool
		wantReason    string
	}{
		{
			name: "Old observation without annotation",
			obs: unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":              "old-obs",
						"creationTimestamp": oldTime.Format(time.RFC3339),
					},
				},
			},
			defaultCutoff: now.AddDate(0, 0, -7),
			wantDelete:    true,
			wantReason:    "ttl_default",
		},
		{
			name: "Recent observation without annotation",
			obs: unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":              "recent-obs",
						"creationTimestamp": recentTime.Format(time.RFC3339),
					},
				},
			},
			defaultCutoff: now.AddDate(0, 0, -7),
			wantDelete:    false,
			wantReason:    "",
		},
		{
			name: "Observation with TTL annotation (expired)",
			obs: unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":              "ttl-obs",
						"creationTimestamp": recentTime.Format(time.RFC3339),
						"annotations": map[string]interface{}{
							TTLAnnotationKey: "3600", // 1 hour TTL
						},
					},
				},
			},
			defaultCutoff: now.AddDate(0, 0, -7),
			wantDelete:    true, // Created 1 day ago, TTL is 1 hour, so expired
			wantReason:    "ttl_annotation",
		},
		{
			name: "Observation with TTL annotation (not expired)",
			obs: unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":              "ttl-obs-recent",
						"creationTimestamp": now.Add(-30 * time.Minute).Format(time.RFC3339),
						"annotations": map[string]interface{}{
							TTLAnnotationKey: "3600", // 1 hour TTL
						},
					},
				},
			},
			defaultCutoff: now.AddDate(0, 0, -7),
			wantDelete:    false, // Created 30 min ago, TTL is 1 hour, so not expired
			wantReason:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set creation timestamp properly
			tt.obs.SetCreationTimestamp(metav1.NewTime(parseTime(t, tt.obs.Object["metadata"].(map[string]interface{})["creationTimestamp"].(string))))

			gc := &Collector{}
			gotDelete, gotReason := gc.shouldDeleteObservation(tt.obs, tt.defaultCutoff)
			if gotDelete != tt.wantDelete {
				t.Errorf("shouldDeleteObservation() gotDelete = %v, want %v", gotDelete, tt.wantDelete)
			}
			if gotReason != tt.wantReason {
				t.Errorf("shouldDeleteObservation() gotReason = %v, want %v", gotReason, tt.wantReason)
			}
		})
	}
}

func parseTime(t *testing.T, timeStr string) time.Time {
	parsed, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		t.Fatalf("Failed to parse time: %v", err)
	}
	return parsed
}

func TestCollector_getNamespacesToScan(t *testing.T) {
	tests := []struct {
		name           string
		watchNamespace string
		want           []string
	}{
		{
			name:           "With WATCH_NAMESPACE set",
			watchNamespace: "zen-system",
			want:           []string{"zen-system"},
		},
		{
			name:           "Without WATCH_NAMESPACE",
			watchNamespace: "",
			want:           []string{""}, // All namespaces
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable
			if tt.watchNamespace != "" {
				t.Setenv("WATCH_NAMESPACE", tt.watchNamespace)
			} else {
				t.Setenv("WATCH_NAMESPACE", "")
			}

			gc := &Collector{}
			got := gc.getNamespacesToScan()
			if len(got) != len(tt.want) {
				t.Errorf("getNamespacesToScan() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Note: Integration test for collectNamespace would require a real Kubernetes cluster
// or a more complex fake client setup. The logic is tested via shouldDeleteObservation.
