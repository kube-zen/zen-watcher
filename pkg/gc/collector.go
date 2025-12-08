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

package gc

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/logger"
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
	// DefaultGCTimeout is the default timeout for a single GC run
	DefaultGCTimeout = 5 * time.Minute
	// GCListChunkSize is the chunk size for listing Observations (prevents memory issues with large lists)
	GCListChunkSize = 500
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
	logger.Info("Starting Observation garbage collector",
		logger.Fields{
			Component: "gc",
			Operation: "gc_start",
			Additional: map[string]interface{}{
				"ttl_days": gc.ttlDays,
				"interval": gc.gcInterval.String(),
			},
		})

	ticker := time.NewTicker(gc.gcInterval)
	defer ticker.Stop()

	// Initial run
	gc.runGC(ctx)

	for {
		select {
		case <-ctx.Done():
			logger.Info("Observation garbage collector stopped",
				logger.Fields{
					Component: "gc",
					Operation: "gc_stop",
				})
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

	logger.Debug("Running garbage collection",
		logger.Fields{
			Component: "gc",
			Operation: "gc_run",
			Additional: map[string]interface{}{
				"ttl_days": gc.ttlDays,
			},
		})

	// Add timeout for GC operations to prevent hangs
	gcTimeout := DefaultGCTimeout
	if timeoutStr := os.Getenv("GC_TIMEOUT"); timeoutStr != "" {
		if parsed, err := time.ParseDuration(timeoutStr); err == nil && parsed > 0 {
			gcTimeout = parsed
		}
	}
	gcCtx, cancel := context.WithTimeout(ctx, gcTimeout)
	defer cancel()

	// List all namespaces (or use watch namespace if set)
	namespaces := gc.getNamespacesToScan()

	totalDeleted := 0
	for _, ns := range namespaces {
		var deleted int
		var err error
		if ns == "" {
			// List all namespaces and collect from each
			deleted, err = gc.collectAllNamespaces(gcCtx)
		} else {
			deleted, err = gc.collectNamespace(gcCtx, ns)
		}
		if err != nil {
			// Check if timeout occurred
			if gcCtx.Err() == context.DeadlineExceeded {
				logger.Warn("GC run timed out",
					logger.Fields{
						Component: "gc",
						Operation: "gc_run",
						Namespace: ns,
						Error:     err,
						Additional: map[string]interface{}{
							"timeout": gcTimeout.String(),
						},
					})
				if gc.gcErrors != nil {
					gc.gcErrors.WithLabelValues("timeout", "gc_timeout").Inc()
				}
			} else {
				logger.Warn("Failed to collect Observations in namespace",
					logger.Fields{
						Component: "gc",
						Operation: "gc_run",
						Namespace: ns,
						Error:     err,
					})
			}
			continue
		}
		totalDeleted += deleted
	}

	if totalDeleted > 0 {
		logger.Info("Garbage collection completed",
			logger.Fields{
				Component: "gc",
				Operation: "gc_run",
				Count:     totalDeleted,
				Additional: map[string]interface{}{
					"deleted": totalDeleted,
				},
			})
	} else {
		logger.Debug("Garbage collection completed, no Observations to delete",
			logger.Fields{
				Component: "gc",
				Operation: "gc_run",
			})
	}
}

// collectNamespace collects old Observations in a specific namespace with chunking support
func (gc *Collector) collectNamespace(ctx context.Context, namespace string) (int, error) {
	cutoffTime := time.Now().AddDate(0, 0, -gc.ttlDays)
	deletedCount := 0

	// Use chunking for large lists (prevents memory issues with 20k+ objects)
	listOptions := metav1.ListOptions{Limit: GCListChunkSize}
	continueToken := ""

	for {
		// Check if context is done
		select {
		case <-ctx.Done():
			return deletedCount, ctx.Err()
		default:
		}

		if continueToken != "" {
			listOptions.Continue = continueToken
		}

		observations, err := gc.dynClient.Resource(gc.eventGVR).Namespace(namespace).List(ctx, listOptions)
		if err != nil {
			// Track GC errors
			if gc.gcErrors != nil {
				gc.gcErrors.WithLabelValues("list", "list_failed").Inc()
			}
			return deletedCount, fmt.Errorf("failed to list Observations: %w", err)
		}

		// Process chunk
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
				logger.Warn("Failed to delete Observation",
					logger.Fields{
						Component:    "gc",
						Operation:    "gc_delete",
						Namespace:    namespace,
						ResourceName: name,
						Source:       source,
						Reason:       reason,
						Error:        err,
					})
				continue
			}

			deletedCount++
			if gc.observationsDeleted != nil {
				gc.observationsDeleted.WithLabelValues(source, reason).Inc()
			}
			logger.Debug("Deleted Observation",
				logger.Fields{
					Component:    "gc",
					Operation:    "gc_delete",
					Namespace:    namespace,
					ResourceName: name,
					Source:       source,
					Reason:       reason,
				})
		}

		// Check for more results
		continueToken = observations.GetContinue()
		if continueToken == "" {
			break
		}
	}

	return deletedCount, nil
}

// collectAllNamespaces collects old Observations across all namespaces with chunking support
func (gc *Collector) collectAllNamespaces(ctx context.Context) (int, error) {
	cutoffTime := time.Now().AddDate(0, 0, -gc.ttlDays)
	deletedCount := 0

	// Use chunking for large lists (prevents memory issues with 20k+ objects)
	listOptions := metav1.ListOptions{Limit: GCListChunkSize}
	continueToken := ""

	for {
		// Check if context is done
		select {
		case <-ctx.Done():
			return deletedCount, ctx.Err()
		default:
		}

		if continueToken != "" {
			listOptions.Continue = continueToken
		}

		observations, err := gc.dynClient.Resource(gc.eventGVR).List(ctx, listOptions)
		if err != nil {
			// Track GC errors
			if gc.gcErrors != nil {
				gc.gcErrors.WithLabelValues("list", "list_failed").Inc()
			}
			return deletedCount, fmt.Errorf("failed to list Observations: %w", err)
		}

		// Process chunk
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
				logger.Warn("Failed to delete Observation",
					logger.Fields{
						Component:    "gc",
						Operation:    "gc_delete",
						Namespace:    namespace,
						ResourceName: name,
						Source:       source,
						Reason:       reason,
						Error:        err,
					})
				continue
			}

			deletedCount++
			if gc.observationsDeleted != nil {
				gc.observationsDeleted.WithLabelValues(source, reason).Inc()
			}
			logger.Debug("Deleted Observation",
				logger.Fields{
					Component:    "gc",
					Operation:    "gc_delete",
					Namespace:    namespace,
					ResourceName: name,
					Source:       source,
					Reason:       reason,
				})
		}

		// Check for more results
		continueToken = observations.GetContinue()
		if continueToken == "" {
			break
		}
	}

	return deletedCount, nil
}

// shouldDeleteObservation determines if an Observation should be deleted
// Priority: 1) spec.ttlSecondsAfterCreation, 2) default TTL
// Returns (shouldDelete, reason)
func (gc *Collector) shouldDeleteObservation(obs unstructured.Unstructured, defaultCutoffTime time.Time) (bool, string) {
	createdTime := obs.GetCreationTimestamp().Time
	now := time.Now()

	// 1. Check spec.ttlSecondsAfterCreation (Kubernetes native style - highest priority)
	if ttlVal, found, _ := unstructured.NestedInt64(obs.Object, "spec", "ttlSecondsAfterCreation"); found && ttlVal > 0 {
		cutoffTime := createdTime.Add(time.Duration(ttlVal) * time.Second)
		if now.After(cutoffTime) {
			return true, "ttl_spec"
		}
		return false, ""
	}

	// 2. Use default TTL based on creation timestamp (fallback)
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
