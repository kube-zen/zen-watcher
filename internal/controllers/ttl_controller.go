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

package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/logger"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

var (
	observationGVR = schema.GroupVersionResource{
		Group:    "zen.kube-zen.io",
		Version:  "v1",
		Resource: "observations",
	}
)

// ObservationTTLController reconciles Observation CRDs to enforce TTL
type ObservationTTLController struct {
	dynamicClient dynamic.Interface
	informer      cache.SharedIndexInformer
}

// NewObservationTTLController creates a new TTL controller
func NewObservationTTLController(
	dynamicClient dynamic.Interface,
	factory dynamicinformer.DynamicSharedInformerFactory,
) *ObservationTTLController {
	controller := &ObservationTTLController{
		dynamicClient: dynamicClient,
	}

	// Create informer for observations
	controller.informer = factory.ForResource(observationGVR).Informer()

	// Add event handlers
	controller.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.handleAdd,
		UpdateFunc: controller.handleUpdate,
	})

	return controller
}

// Start starts the TTL controller
func (r *ObservationTTLController) Start(ctx context.Context) error {
	logger.Info("Starting Observation TTL Controller",
		logger.Fields{
			Component: "ttl-controller",
			Operation: "start",
		})

	// Wait for cache sync
	if !cache.WaitForCacheSync(ctx.Done(), r.informer.HasSynced) {
		return fmt.Errorf("failed to sync TTL controller cache")
	}

	// Start periodic reconciliation
	go r.reconcileLoop(ctx)

	<-ctx.Done()
	return nil
}

// reconcileLoop periodically checks all observations for expired TTLs
func (r *ObservationTTLController) reconcileLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute) // Check every minute
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.reconcileAll(ctx)
		}
	}
}

// reconcileAll checks all observations and deletes expired ones
func (r *ObservationTTLController) reconcileAll(ctx context.Context) {
	// Get all observations from cache
	objs := r.informer.GetStore().List()

	for _, obj := range objs {
		obs, ok := obj.(*unstructured.Unstructured)
		if !ok {
			continue
		}

		if err := r.reconcileObservation(ctx, obs); err != nil {
			logger.Warn("Failed to reconcile observation",
				logger.Fields{
					Component: "ttl-controller",
					Operation: "reconcile",
					Error:     err,
					Additional: map[string]interface{}{
						"name":      obs.GetName(),
						"namespace": obs.GetNamespace(),
					},
				})
		}
	}
}

// reconcileObservation checks if an observation's TTL has expired and deletes it if so
func (r *ObservationTTLController) reconcileObservation(ctx context.Context, obs *unstructured.Unstructured) error {
	// Get TTL from spec
	ttlSeconds, found, _ := unstructured.NestedInt64(obs.Object, "spec", "ttlSecondsAfterCreation")
	if !found || ttlSeconds <= 0 {
		// No TTL set, skip
		return nil
	}

	// Calculate expiration time
	creationTime := obs.GetCreationTimestamp().Time
	expirationTime := creationTime.Add(time.Duration(ttlSeconds) * time.Second)

	// Check if expired
	if time.Now().After(expirationTime) {
		// Delete expired observation
		namespace := obs.GetNamespace()
		name := obs.GetName()

		logger.Info("Deleting expired observation",
			logger.Fields{
				Component: "ttl-controller",
				Operation: "delete_expired",
				Additional: map[string]interface{}{
					"name":            name,
					"namespace":       namespace,
					"creation_time":   creationTime.Format(time.RFC3339),
					"expiration_time": expirationTime.Format(time.RFC3339),
					"ttl_seconds":     ttlSeconds,
				},
			})

		err := r.dynamicClient.Resource(observationGVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete expired observation: %w", err)
		}
	}

	return nil
}

// handleAdd handles new observation creation
func (r *ObservationTTLController) handleAdd(obj interface{}) {
	obs, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return
	}

	// Check TTL and schedule requeue if needed
	ttlSeconds, found, _ := unstructured.NestedInt64(obs.Object, "spec", "ttlSecondsAfterCreation")
	if found && ttlSeconds > 0 {
		creationTime := obs.GetCreationTimestamp().Time
		expirationTime := creationTime.Add(time.Duration(ttlSeconds) * time.Second)
		remainingTime := time.Until(expirationTime)

		logger.Debug("Observation created with TTL",
			logger.Fields{
				Component: "ttl-controller",
				Operation: "observe_ttl",
				Additional: map[string]interface{}{
					"name":            obs.GetName(),
					"namespace":       obs.GetNamespace(),
					"ttl_seconds":     ttlSeconds,
					"expiration_time": expirationTime.Format(time.RFC3339),
					"remaining_time":  remainingTime.String(),
				},
			})
	}
}

// handleUpdate handles observation updates
func (r *ObservationTTLController) handleUpdate(oldObj, newObj interface{}) {
	// TTL is immutable, so we just log if it changed
	oldObs, ok1 := oldObj.(*unstructured.Unstructured)
	newObs, ok2 := newObj.(*unstructured.Unstructured)
	if !ok1 || !ok2 {
		return
	}

	oldTTL, _, _ := unstructured.NestedInt64(oldObs.Object, "spec", "ttlSecondsAfterCreation")
	newTTL, _, _ := unstructured.NestedInt64(newObs.Object, "spec", "ttlSecondsAfterCreation")

	if oldTTL != newTTL {
		logger.Debug("Observation TTL updated",
			logger.Fields{
				Component: "ttl-controller",
				Operation: "ttl_updated",
				Additional: map[string]interface{}{
					"name":      newObs.GetName(),
					"namespace": newObs.GetNamespace(),
					"old_ttl":   oldTTL,
					"new_ttl":   newTTL,
				},
			})
	}
}
