package gc

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

const (
	// DefaultTTLDays is the default TTL in days for Observations
	DefaultTTLDays = 7
	// DefaultGCInterval is the default interval between GC runs
	DefaultGCInterval = 1 * time.Hour
	// TTLAnnotationKey is the annotation key for per-Observation TTL
	TTLAnnotationKey = "zen.kube-zen.io/ttl-seconds"
)

// Collector handles garbage collection of old Observations
type Collector struct {
	dynClient           dynamic.Interface
	eventGVR            schema.GroupVersionResource
	ttlDays             int
	gcInterval          time.Duration
	observationsDeleted *prometheus.CounterVec
	gcRunsTotal         prometheus.Counter
	gcDuration          *prometheus.HistogramVec
	gcErrors            *prometheus.CounterVec
}

// NewCollector creates a new garbage collector
func NewCollector(
	dynClient dynamic.Interface,
	eventGVR schema.GroupVersionResource,
	observationsDeleted *prometheus.CounterVec,
	gcRunsTotal prometheus.Counter,
	gcDuration *prometheus.HistogramVec,
	gcErrors *prometheus.CounterVec,
) *Collector {
	// Get TTL from environment variable (in days)
	ttlDays := DefaultTTLDays
	if ttlStr := os.Getenv("OBSERVATION_TTL_DAYS"); ttlStr != "" {
		if d, err := strconv.Atoi(ttlStr); err == nil && d > 0 {
			ttlDays = d
		}
	}

	// Get GC interval from environment variable
	gcInterval := DefaultGCInterval
	if intervalStr := os.Getenv("GC_INTERVAL"); intervalStr != "" {
		if d, err := time.ParseDuration(intervalStr); err == nil && d > 0 {
			gcInterval = d
		}
	}

	return &Collector{
		dynClient:           dynClient,
		eventGVR:            eventGVR,
		ttlDays:             ttlDays,
		gcInterval:          gcInterval,
		observationsDeleted: observationsDeleted,
		gcRunsTotal:         gcRunsTotal,
		gcDuration:          gcDuration,
		gcErrors:            gcErrors,
	}
}

// Start starts the garbage collection loop
func (gc *Collector) Start(ctx context.Context) {
	log.Printf("🗑️  Starting Observation garbage collector (TTL: %d days, interval: %v)", gc.ttlDays, gc.gcInterval)

	ticker := time.NewTicker(gc.gcInterval)
	defer ticker.Stop()

	// Initial run
	gc.runGC(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Println("🛑 Observation garbage collector stopped")
			return
		case <-ticker.C:
			gc.runGC(ctx)
		}
	}
}

// runGC performs a single garbage collection cycle
func (gc *Collector) runGC(ctx context.Context) {
	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime).Seconds()
		if gc.gcDuration != nil {
			gc.gcDuration.WithLabelValues("gc_run").Observe(duration)
		}
	}()

	if gc.gcRunsTotal != nil {
		gc.gcRunsTotal.Inc()
	}

	log.Printf("🗑️  Running garbage collection (TTL: %d days)...", gc.ttlDays)

	// List all namespaces (or use watch namespace if set)
	namespaces := gc.getNamespacesToScan()

	totalDeleted := 0
	for _, ns := range namespaces {
		var deleted int
		var err error
		if ns == "" {
			// List all namespaces and collect from each
			deleted, err = gc.collectAllNamespaces(ctx)
		} else {
			deleted, err = gc.collectNamespace(ctx, ns)
		}
		if err != nil {
			log.Printf("⚠️  Failed to collect Observations in namespace %s: %v", ns, err)
			continue
		}
		totalDeleted += deleted
	}

	if totalDeleted > 0 {
		log.Printf("✅ Garbage collection completed: deleted %d Observations", totalDeleted)
	} else {
		log.Printf("✅ Garbage collection completed: no Observations to delete")
	}
}

// collectNamespace collects old Observations in a specific namespace
func (gc *Collector) collectNamespace(ctx context.Context, namespace string) (int, error) {
	// List all Observations in the namespace
	observations, err := gc.dynClient.Resource(gc.eventGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		// Track GC errors
		if gc.gcErrors != nil {
			gc.gcErrors.WithLabelValues("list", "list_failed").Inc()
		}
		return 0, fmt.Errorf("failed to list Observations: %w", err)
	}

	cutoffTime := time.Now().AddDate(0, 0, -gc.ttlDays)
	deletedCount := 0

	for _, obs := range observations.Items {
		shouldDelete, reason := gc.shouldDeleteObservation(obs, cutoffTime)
		if !shouldDelete {
			continue
		}

		// Extract source for metrics
		source := "unknown"
		if sourceVal, _, _ := unstructured.NestedFieldCopy(obs.Object, "spec", "source"); sourceVal != nil {
			source = fmt.Sprintf("%v", sourceVal)
		}

		// Delete the Observation
		name := obs.GetName()
		err := gc.dynClient.Resource(gc.eventGVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
		if err != nil {
			// Track GC errors
			if gc.gcErrors != nil {
				errorType := "delete_failed"
				errMsg := strings.ToLower(err.Error())
				if strings.Contains(errMsg, "not found") {
					errorType = "not_found"
				} else if strings.Contains(errMsg, "forbidden") {
					errorType = "forbidden"
				}
				gc.gcErrors.WithLabelValues("delete", errorType).Inc()
			}
			log.Printf("⚠️  Failed to delete Observation %s/%s: %v", namespace, name, err)
			continue
		}

		deletedCount++
		if gc.observationsDeleted != nil {
			gc.observationsDeleted.WithLabelValues(source, reason).Inc()
		}
		log.Printf("  🗑️  Deleted Observation %s/%s (reason: %s)", namespace, name, reason)
	}

	return deletedCount, nil
}

// collectAllNamespaces collects old Observations across all namespaces
func (gc *Collector) collectAllNamespaces(ctx context.Context) (int, error) {
	// List Observations across all namespaces
	observations, err := gc.dynClient.Resource(gc.eventGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		// Track GC errors
		if gc.gcErrors != nil {
			gc.gcErrors.WithLabelValues("list", "list_failed").Inc()
		}
		return 0, fmt.Errorf("failed to list Observations: %w", err)
	}

	cutoffTime := time.Now().AddDate(0, 0, -gc.ttlDays)
	deletedCount := 0

	for _, obs := range observations.Items {
		shouldDelete, reason := gc.shouldDeleteObservation(obs, cutoffTime)
		if !shouldDelete {
			continue
		}

		// Extract source for metrics
		source := "unknown"
		if sourceVal, _, _ := unstructured.NestedFieldCopy(obs.Object, "spec", "source"); sourceVal != nil {
			source = fmt.Sprintf("%v", sourceVal)
		}

		// Delete the Observation
		name := obs.GetName()
		namespace := obs.GetNamespace()
		if namespace == "" {
			namespace = "default"
		}

		err := gc.dynClient.Resource(gc.eventGVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
		if err != nil {
			log.Printf("⚠️  Failed to delete Observation %s/%s: %v", namespace, name, err)
			continue
		}

		deletedCount++
		if gc.observationsDeleted != nil {
			gc.observationsDeleted.WithLabelValues(source, reason).Inc()
		}
		log.Printf("  🗑️  Deleted Observation %s/%s (reason: %s)", namespace, name, reason)
	}

	return deletedCount, nil
}

// shouldDeleteObservation determines if an Observation should be deleted
// Returns (shouldDelete, reason)
func (gc *Collector) shouldDeleteObservation(obs unstructured.Unstructured, defaultCutoffTime time.Time) (bool, string) {
	// Check for TTL annotation (per-Observation TTL override)
	if ttlStr, found := obs.GetAnnotations()[TTLAnnotationKey]; found {
		ttlSeconds, err := strconv.ParseInt(ttlStr, 10, 64)
		if err == nil && ttlSeconds > 0 {
			createdTime := obs.GetCreationTimestamp().Time
			cutoffTime := createdTime.Add(time.Duration(ttlSeconds) * time.Second)
			if time.Now().After(cutoffTime) {
				return true, "ttl_annotation"
			}
			return false, ""
		}
	}

	// Use default TTL based on creation timestamp
	createdTime := obs.GetCreationTimestamp().Time
	if createdTime.Before(defaultCutoffTime) {
		return true, "ttl_default"
	}

	return false, ""
}

// getNamespacesToScan returns the list of namespaces to scan for Observations
func (gc *Collector) getNamespacesToScan() []string {
	watchNamespace := os.Getenv("WATCH_NAMESPACE")
	if watchNamespace != "" {
		return []string{watchNamespace}
	}

	// If no specific namespace, scan all namespaces
	// Use empty string to list across all namespaces
	return []string{""} // "" means all namespaces
}
