package watcher

import (
	"context"
	"fmt"
	"log"

	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// ObservationCreator handles creation of Observations with centralized metrics increment
type ObservationCreator struct {
	dynClient   dynamic.Interface
	eventGVR    schema.GroupVersionResource
	eventsTotal *prometheus.CounterVec
}

// NewObservationCreator creates a new observation creator
func NewObservationCreator(dynClient dynamic.Interface, eventGVR schema.GroupVersionResource, eventsTotal *prometheus.CounterVec) *ObservationCreator {
	return &ObservationCreator{
		dynClient:   dynClient,
		eventGVR:    eventGVR,
		eventsTotal: eventsTotal,
	}
}

// CreateObservation creates an Observation CRD and increments metrics
// This is the centralized place where all Observations are created, ensuring metrics are always incremented
func (oc *ObservationCreator) CreateObservation(ctx context.Context, observation *unstructured.Unstructured) error {
	// Extract namespace from observation
	namespace, found, _ := unstructured.NestedString(observation.Object, "metadata", "namespace")
	if !found || namespace == "" {
		namespace = "default"
	}

	// Create the Observation
	_, err := oc.dynClient.Resource(oc.eventGVR).Namespace(namespace).Create(ctx, observation, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create Observation: %w", err)
	}

	// Extract source, category, and severity from spec for metrics
	// Use NestedFieldCopy to handle interface{} types, then convert to string
	sourceVal, _, _ := unstructured.NestedFieldCopy(observation.Object, "spec", "source")
	categoryVal, _, _ := unstructured.NestedFieldCopy(observation.Object, "spec", "category")
	severityVal, _, _ := unstructured.NestedFieldCopy(observation.Object, "spec", "severity")

	// Convert to strings, handling nil and interface{} types
	source := ""
	if sourceVal != nil {
		source = fmt.Sprintf("%v", sourceVal)
	}
	category := ""
	if categoryVal != nil {
		category = fmt.Sprintf("%v", categoryVal)
	}
	severity := ""
	if severityVal != nil {
		severity = fmt.Sprintf("%v", severityVal)
	}

	// Normalize severity to uppercase for consistency
	if severity != "" {
		severity = normalizeSeverity(severity)
	}

	// Increment metrics - this is the ONLY place where metrics are incremented
	if oc.eventsTotal != nil {
		if source == "" {
			log.Printf("  ⚠️  WARNING: Observation created without source label, metrics may be incomplete")
			source = "unknown"
		}
		if category == "" {
			category = "unknown"
		}
		if severity == "" {
			severity = "UNKNOWN"
		}
		oc.eventsTotal.WithLabelValues(source, category, severity).Inc()
		log.Printf("  📊 METRIC INCREMENTED: %s/%s/%s", source, category, severity)
	} else {
		log.Printf("  ⚠️  CRITICAL: eventsTotal is nil! Metrics will not be incremented!")
	}

	return nil
}

// normalizeSeverity normalizes severity values to uppercase standard format
func normalizeSeverity(severity string) string {
	switch severity {
	case "critical", "Critical", "CRITICAL":
		return "CRITICAL"
	case "high", "High", "HIGH":
		return "HIGH"
	case "medium", "Medium", "MEDIUM":
		return "MEDIUM"
	case "low", "Low", "LOW":
		return "LOW"
	default:
		return "UNKNOWN"
	}
}
