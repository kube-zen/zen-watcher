package watcher

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/kube-zen/zen-watcher/pkg/dedup"
	"github.com/kube-zen/zen-watcher/pkg/filter"
	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// ObservationCreator handles creation of Observations with centralized filtering, normalization, deduplication, and metrics
// Flow: filter() → normalize() → dedup() → create Observation CRD + update metrics + log
type ObservationCreator struct {
	dynClient            dynamic.Interface
	eventGVR             schema.GroupVersionResource
	eventsTotal          *prometheus.CounterVec
	observationsCreated  *prometheus.CounterVec
	observationsFiltered *prometheus.CounterVec
	observationsDeduped  prometheus.Counter
	deduper              *dedup.Deduper
	filter               *filter.Filter
}

// NewObservationCreator creates a new observation creator with optional filter and metrics
func NewObservationCreator(
	dynClient dynamic.Interface,
	eventGVR schema.GroupVersionResource,
	eventsTotal *prometheus.CounterVec,
	observationsCreated *prometheus.CounterVec,
	observationsFiltered *prometheus.CounterVec,
	observationsDeduped prometheus.Counter,
	filter *filter.Filter,
) *ObservationCreator {
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
		dynClient:            dynClient,
		eventGVR:             eventGVR,
		eventsTotal:          eventsTotal,
		observationsCreated:  observationsCreated,
		observationsFiltered: observationsFiltered,
		observationsDeduped:  observationsDeduped,
		deduper:              dedup.NewDeduper(windowSeconds, maxSize),
		filter:               filter,
	}
}

// CreateObservation creates an Observation CRD and increments metrics
// This is the centralized place where all Observations are created
// Flow: filter() → normalize() → dedup() → create Observation CRD + update metrics + log
// First event always creates an observation; duplicates within window are skipped
func (oc *ObservationCreator) CreateObservation(ctx context.Context, observation *unstructured.Unstructured) error {
	// Extract source early for metrics
	sourceVal, _, _ := unstructured.NestedFieldCopy(observation.Object, "spec", "source")
	source := ""
	if sourceVal != nil {
		source = fmt.Sprintf("%v", sourceVal)
	}
	if source == "" {
		source = "unknown"
	}

	// STEP 1: FILTER - Apply source-level filtering BEFORE normalization and deduplication
	if oc.filter != nil {
		allowed, reason := oc.filter.AllowWithReason(observation)
		if !allowed {
			// Filtered out - increment filtered metric and return early
			if oc.observationsFiltered != nil {
				oc.observationsFiltered.WithLabelValues(source, reason).Inc()
			}
			return nil
		}
	}

	// STEP 2: NORMALIZE - Normalize severity to uppercase for consistency
	// (Normalization happens inline during extraction below)

	// STEP 3: DEDUP - Check if we should create (first event always creates, duplicates within window are skipped)
	dedupKey := oc.extractDedupKey(observation)
	if !oc.deduper.ShouldCreate(dedupKey) {
		log.Printf("  📋 [DEDUP] Skipping duplicate observation within window: %s", dedupKey.String())
		// Increment deduped metric
		if oc.observationsDeduped != nil {
			oc.observationsDeduped.Inc()
		}
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

	// Increment observations created metric
	if oc.observationsCreated != nil {
		oc.observationsCreated.WithLabelValues(source).Inc()
	}

	// Extract category and severity from spec for metrics
	// Use NestedFieldCopy to handle interface{} types, then convert to string
	categoryVal, categoryFound, _ := unstructured.NestedFieldCopy(observation.Object, "spec", "category")
	severityVal, severityFound, _ := unstructured.NestedFieldCopy(observation.Object, "spec", "severity")
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

	// Increment eventsTotal metric - this tracks events by source/category/severity
	if oc.eventsTotal != nil {
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
