package watcher

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/kube-zen/zen-watcher/pkg/dedup"
	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// ObservationCreator handles creation of Observations with centralized metrics increment and deduplication
type ObservationCreator struct {
	dynClient   dynamic.Interface
	eventGVR    schema.GroupVersionResource
	eventsTotal *prometheus.CounterVec
	deduper     *dedup.Deduper
}

// NewObservationCreator creates a new observation creator
func NewObservationCreator(dynClient dynamic.Interface, eventGVR schema.GroupVersionResource, eventsTotal *prometheus.CounterVec) *ObservationCreator {
	// Get dedup window from env, default 60 seconds
	windowSeconds := 60
	if windowStr := os.Getenv("DEDUP_WINDOW_SECONDS"); windowStr != "" {
		if w, err := strconv.Atoi(windowStr); err == nil && w > 0 {
			windowSeconds = w
		}
	}
	// Get max cache size from env, default 10000
	maxSize := 10000
	if sizeStr := os.Getenv("DEDUP_MAX_SIZE"); sizeStr != "" {
		if s, err := strconv.Atoi(sizeStr); err == nil && s > 0 {
			maxSize = s
		}
	}
	return &ObservationCreator{
		dynClient:   dynClient,
		eventGVR:    eventGVR,
		eventsTotal: eventsTotal,
		deduper:     dedup.NewDeduper(windowSeconds, maxSize),
	}
}

// CreateObservation creates an Observation CRD and increments metrics
// This is the centralized place where all Observations are created, ensuring metrics are always incremented
// First event always creates an observation; duplicates within window are skipped
func (oc *ObservationCreator) CreateObservation(ctx context.Context, observation *unstructured.Unstructured) error {
	// Extract dedup key from observation
	dedupKey := oc.extractDedupKey(observation)

	// Check if we should create (first event always creates, duplicates within window are skipped)
	if !oc.deduper.ShouldCreate(dedupKey) {
		log.Printf("  📋 [DEDUP] Skipping duplicate observation within window: %s", dedupKey.String())
		return nil // Skip duplicate, but don't error
	}

	// Extract namespace from observation
	namespace, found, _ := unstructured.NestedString(observation.Object, "metadata", "namespace")
	if !found || namespace == "" {
		namespace = "default"
	}

	// Create the Observation (first event always creates)
	_, err := oc.dynClient.Resource(oc.eventGVR).Namespace(namespace).Create(ctx, observation, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create Observation: %w", err)
	}

	// Extract source, category, and severity from spec for metrics
	// Use NestedFieldCopy to handle interface{} types, then convert to string
	sourceVal, sourceFound, _ := unstructured.NestedFieldCopy(observation.Object, "spec", "source")
	categoryVal, categoryFound, _ := unstructured.NestedFieldCopy(observation.Object, "spec", "category")
	severityVal, severityFound, _ := unstructured.NestedFieldCopy(observation.Object, "spec", "severity")

	// Convert to strings, handling nil and interface{} types
	source := ""
	if sourceVal != nil {
		source = fmt.Sprintf("%v", sourceVal)
	} else if !sourceFound {
		log.Printf("  ⚠️  DEBUG: source not found in spec")
	}
	category := ""
	if categoryVal != nil {
		category = fmt.Sprintf("%v", categoryVal)
	} else if !categoryFound {
		log.Printf("  ⚠️  DEBUG: category not found in spec")
	}
	severity := ""
	if severityVal != nil {
		severity = fmt.Sprintf("%v", severityVal)
	} else if !severityFound {
		log.Printf("  ⚠️  DEBUG: severity not found in spec")
	}

	// Debug logging
	if source == "" || category == "" || severity == "" {
		log.Printf("  ⚠️  DEBUG: Extracted values - source:'%s' category:'%s' severity:'%s'", source, category, severity)
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

// extractDedupKey extracts minimal metadata for deduplication from observation
func (oc *ObservationCreator) extractDedupKey(observation *unstructured.Unstructured) dedup.DedupKey {
	// Extract source
	sourceVal, _, _ := unstructured.NestedFieldCopy(observation.Object, "spec", "source")
	source := ""
	if sourceVal != nil {
		source = fmt.Sprintf("%v", sourceVal)
	}

	// Extract resource info
	resourceVal, _, _ := unstructured.NestedMap(observation.Object, "spec", "resource")
	namespace := ""
	kind := ""
	name := ""
	if resourceVal != nil {
		if ns, ok := resourceVal["namespace"].(string); ok {
			namespace = ns
		} else if ns, ok := resourceVal["namespace"].(interface{}); ok {
			namespace = fmt.Sprintf("%v", ns)
		}
		if k, ok := resourceVal["kind"].(string); ok {
			kind = k
		} else if k, ok := resourceVal["kind"].(interface{}); ok {
			kind = fmt.Sprintf("%v", k)
		}
		if n, ok := resourceVal["name"].(string); ok {
			name = n
		} else if n, ok := resourceVal["name"].(interface{}); ok {
			name = fmt.Sprintf("%v", n)
		}
	}
	// Fallback to metadata namespace if resource namespace is empty
	if namespace == "" {
		namespace, _, _ = unstructured.NestedString(observation.Object, "metadata", "namespace")
		if namespace == "" {
			namespace = "default"
		}
	}

	// Extract reason from details or eventType
	reason := ""
	detailsVal, _, _ := unstructured.NestedMap(observation.Object, "spec", "details")
	if detailsVal != nil {
		// Try common reason fields
		if r, ok := detailsVal["reason"].(string); ok {
			reason = r
		} else if r, ok := detailsVal["reason"].(interface{}); ok {
			reason = fmt.Sprintf("%v", r)
		} else if r, ok := detailsVal["rule"].(string); ok {
			reason = r
		} else if r, ok := detailsVal["rule"].(interface{}); ok {
			reason = fmt.Sprintf("%v", r)
		} else if r, ok := detailsVal["testNumber"].(string); ok {
			reason = r
		} else if r, ok := detailsVal["testNumber"].(interface{}); ok {
			reason = fmt.Sprintf("%v", r)
		} else if r, ok := detailsVal["checkId"].(string); ok {
			reason = r
		} else if r, ok := detailsVal["checkId"].(interface{}); ok {
			reason = fmt.Sprintf("%v", r)
		} else if r, ok := detailsVal["vulnerabilityID"].(string); ok {
			reason = r
		} else if r, ok := detailsVal["vulnerabilityID"].(interface{}); ok {
			reason = fmt.Sprintf("%v", r)
		} else if r, ok := detailsVal["auditID"].(string); ok {
			reason = r
		} else if r, ok := detailsVal["auditID"].(interface{}); ok {
			reason = fmt.Sprintf("%v", r)
		}
	}
	// Fallback to eventType if no reason found
	if reason == "" {
		eventTypeVal, _, _ := unstructured.NestedFieldCopy(observation.Object, "spec", "eventType")
		if eventTypeVal != nil {
			reason = fmt.Sprintf("%v", eventTypeVal)
		}
	}

	// Extract message for hashing
	message := ""
	if detailsVal != nil {
		if msg, ok := detailsVal["message"].(string); ok {
			message = msg
		} else if msg, ok := detailsVal["message"].(interface{}); ok {
			message = fmt.Sprintf("%v", msg)
		} else if msg, ok := detailsVal["output"].(string); ok {
			message = msg
		} else if msg, ok := detailsVal["output"].(interface{}); ok {
			message = fmt.Sprintf("%v", msg)
		}
	}

	// Hash message
	messageHash := dedup.HashMessage(message)

	return dedup.DedupKey{
		Source:      source,
		Namespace:   namespace,
		Kind:        kind,
		Name:        name,
		Reason:      reason,
		MessageHash: messageHash,
	}
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
