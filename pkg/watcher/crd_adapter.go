// Copyright 2024 The Zen Watcher Authors
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

package watcher

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

var (
	// ObservationMappingGVR is the GroupVersionResource for ObservationMapping CRDs
	ObservationMappingGVR = schema.GroupVersionResource{
		Group:    "zen.kube-zen.io",
		Version:  "v1alpha1",
		Resource: "observationmappings",
	}
)

// ObservationMapping represents a mapping configuration from a source CRD to Observation Events
type ObservationMapping struct {
	SourceName  string
	Group       string
	Version     string
	Kind        string
	GVR         schema.GroupVersionResource
	Mappings    FieldMappings
	SeverityMap map[string]string
	Enabled     bool
}

// FieldMappings defines how to extract fields from source CRD to Event
type FieldMappings struct {
	Severity  string            // JSONPath or static value
	Category  string            // JSONPath or static value (default: "security")
	EventType string            // JSONPath or static value (default: "custom-event")
	Message   string            // JSONPath (optional)
	Resource  ResourceMappings  // Resource reference mappings
	Details   map[string]string // Additional details mappings (JSONPath)
}

// ResourceMappings defines how to extract resource reference fields
type ResourceMappings struct {
	APIVersion string // JSONPath
	Kind       string // JSONPath
	Name       string // JSONPath
	Namespace  string // JSONPath (optional, can use metadata.namespace)
}

// CRDSourceAdapter implements SourceAdapter for generic CRD-based sources
// This adapter watches ObservationMapping CRDs and creates informers for configured source CRDs
type CRDSourceAdapter struct {
	factory         dynamicinformer.DynamicSharedInformerFactory
	mappingGVR      schema.GroupVersionResource
	mappings        map[string]*ObservationMapping // key: sourceName
	mappingsMu      sync.RWMutex
	informer        cache.SharedIndexInformer
	activeInformers map[string]cache.SharedIndexInformer // key: sourceName
	activeMu        sync.RWMutex
	stopCh          chan struct{}
	ctx             context.Context
	cancel          context.CancelFunc
	eventOut        chan<- *Event
}

// NewCRDSourceAdapter creates a new generic CRD-based source adapter
func NewCRDSourceAdapter(
	factory dynamicinformer.DynamicSharedInformerFactory,
	mappingGVR schema.GroupVersionResource,
) *CRDSourceAdapter {
	return &CRDSourceAdapter{
		factory:         factory,
		mappingGVR:      mappingGVR,
		mappings:        make(map[string]*ObservationMapping),
		activeInformers: make(map[string]cache.SharedIndexInformer),
		stopCh:          make(chan struct{}),
	}
}

func (a *CRDSourceAdapter) Name() string {
	return "crd-generic"
}

func (a *CRDSourceAdapter) Run(ctx context.Context, out chan<- *Event) error {
	a.ctx, a.cancel = context.WithCancel(ctx)
	a.eventOut = out

	// Watch ObservationMapping CRDs
	a.informer = a.factory.ForResource(a.mappingGVR).Informer()

	a.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    a.handleMappingAdd,
		UpdateFunc: a.handleMappingUpdate,
		DeleteFunc: a.handleMappingDelete,
	})

	// Start informer
	go a.informer.Run(a.stopCh)

	// Wait for cache to sync
	if !cache.WaitForCacheSync(ctx.Done(), a.informer.HasSynced) {
		return fmt.Errorf("failed to sync ObservationMapping informer")
	}

	// Keep running until context is cancelled
	<-ctx.Done()
	return ctx.Err()
}

func (a *CRDSourceAdapter) Stop() {
	if a.cancel != nil {
		a.cancel()
	}

	// Close stopCh to stop all informers
	close(a.stopCh)
}

func (a *CRDSourceAdapter) handleMappingAdd(obj interface{}) {
	mapping := a.convertToMapping(obj)
	if mapping == nil {
		return
	}

	a.mappingsMu.Lock()
	a.mappings[mapping.SourceName] = mapping
	a.mappingsMu.Unlock()

	a.startInformerForMapping(mapping)
}

func (a *CRDSourceAdapter) handleMappingUpdate(oldObj, newObj interface{}) {
	newMapping := a.convertToMapping(newObj)
	if newMapping == nil {
		return
	}

	oldMapping := a.convertToMapping(oldObj)
	if oldMapping != nil && oldMapping.SourceName == newMapping.SourceName {
		// Same source, just update mapping
		a.mappingsMu.Lock()
		a.mappings[newMapping.SourceName] = newMapping
		a.mappingsMu.Unlock()
		return
	}

	// Source changed - restart informer
	if oldMapping != nil {
		a.stopInformerForMapping(oldMapping.SourceName)
	}

	a.mappingsMu.Lock()
	a.mappings[newMapping.SourceName] = newMapping
	a.mappingsMu.Unlock()

	a.startInformerForMapping(newMapping)
}

func (a *CRDSourceAdapter) handleMappingDelete(obj interface{}) {
	mapping := a.convertToMapping(obj)
	if mapping == nil {
		return
	}

	a.stopInformerForMapping(mapping.SourceName)

	a.mappingsMu.Lock()
	delete(a.mappings, mapping.SourceName)
	a.mappingsMu.Unlock()
}

func (a *CRDSourceAdapter) convertToMapping(obj interface{}) *ObservationMapping {
	unstruct, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil
	}

	spec, found, _ := unstructured.NestedMap(unstruct.Object, "spec")
	if !found {
		return nil
	}

	// Extract required fields
	sourceName, found, _ := unstructured.NestedString(spec, "sourceName")
	if !found || sourceName == "" {
		return nil
	}

	group, found, _ := unstructured.NestedString(spec, "group")
	if !found {
		return nil
	}

	version, found, _ := unstructured.NestedString(spec, "version")
	if !found {
		return nil
	}

	kind, found, _ := unstructured.NestedString(spec, "kind")
	if !found {
		return nil
	}

	// Check if enabled (default: true)
	enabled := true
	if val, found, _ := unstructured.NestedBool(spec, "enabled"); found {
		enabled = val
	}

	if !enabled {
		return nil
	}

	// Extract mappings
	mappingsObj, _, _ := unstructured.NestedMap(spec, "mappings")
	fieldMappings := FieldMappings{
		Category:  "security",     // default
		EventType: "custom-event", // default
		Details:   make(map[string]string),
	}

	if mappingsObj != nil {
		if val, found, _ := unstructured.NestedString(mappingsObj, "severity"); found {
			fieldMappings.Severity = val
		}
		if val, found, _ := unstructured.NestedString(mappingsObj, "category"); found {
			fieldMappings.Category = val
		}
		if val, found, _ := unstructured.NestedString(mappingsObj, "eventType"); found {
			fieldMappings.EventType = val
		}
		if val, found, _ := unstructured.NestedString(mappingsObj, "message"); found {
			fieldMappings.Message = val
		}

		// Extract resource mappings
		if resourceObj, found, _ := unstructured.NestedMap(mappingsObj, "resource"); found {
			if val, found, _ := unstructured.NestedString(resourceObj, "apiVersion"); found {
				fieldMappings.Resource.APIVersion = val
			}
			if val, found, _ := unstructured.NestedString(resourceObj, "kind"); found {
				fieldMappings.Resource.Kind = val
			}
			if val, found, _ := unstructured.NestedString(resourceObj, "name"); found {
				fieldMappings.Resource.Name = val
			}
			if val, found, _ := unstructured.NestedString(resourceObj, "namespace"); found {
				fieldMappings.Resource.Namespace = val
			}
		}

		// Extract details mappings
		if detailsObj, found, _ := unstructured.NestedMap(mappingsObj, "details"); found {
			for k, v := range detailsObj {
				if strVal, ok := v.(string); ok {
					fieldMappings.Details[k] = strVal
				}
			}
		}
	}

	// Extract severity map
	severityMap := make(map[string]string)
	if severityMapObj, found, _ := unstructured.NestedMap(spec, "severityMap"); found {
		for k, v := range severityMapObj {
			if strVal, ok := v.(string); ok {
				severityMap[k] = strVal
			}
		}
	}

	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: strings.ToLower(kind) + "s", // Simple pluralization
	}

	return &ObservationMapping{
		SourceName:  sourceName,
		Group:       group,
		Version:     version,
		Kind:        kind,
		GVR:         gvr,
		Mappings:    fieldMappings,
		SeverityMap: severityMap,
		Enabled:     enabled,
	}
}

func (a *CRDSourceAdapter) startInformerForMapping(mapping *ObservationMapping) {
	if mapping == nil || !mapping.Enabled {
		return
	}

	// Check if informer already exists
	a.activeMu.RLock()
	if _, exists := a.activeInformers[mapping.SourceName]; exists {
		a.activeMu.RUnlock()
		return
	}
	a.activeMu.RUnlock()

	// Create informer for source CRD
	informer := a.factory.ForResource(mapping.GVR).Informer()

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			a.processSourceCRD(mapping, obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			a.processSourceCRD(mapping, newObj)
		},
	})

	a.activeMu.Lock()
	a.activeInformers[mapping.SourceName] = informer
	a.activeMu.Unlock()

	// Start informer in background
	go informer.Run(a.stopCh)
}

func (a *CRDSourceAdapter) stopInformerForMapping(sourceName string) {
	a.activeMu.Lock()
	defer a.activeMu.Unlock()

	if _, exists := a.activeInformers[sourceName]; exists {
		// Informer will be stopped when stopCh is closed in Stop()
		// We just remove it from tracking
		delete(a.activeInformers, sourceName)
	}
}

func (a *CRDSourceAdapter) processSourceCRD(mapping *ObservationMapping, obj interface{}) {
	unstruct, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return
	}

	event := a.mapCRDToEvent(mapping, unstruct)
	if event == nil {
		return
	}

	select {
	case a.eventOut <- event:
	case <-a.ctx.Done():
		return
	}
}

func (a *CRDSourceAdapter) mapCRDToEvent(mapping *ObservationMapping, crd *unstructured.Unstructured) *Event {
	// Extract severity
	severity := a.extractFieldString(mapping.Mappings.Severity, crd.Object, "MEDIUM")
	if mapping.Mappings.Severity != "" && !strings.HasPrefix(mapping.Mappings.Severity, ".") {
		// Static value
		severity = mapping.Mappings.Severity
	}
	// Apply severity mapping if configured
	if mapped, exists := mapping.SeverityMap[strings.ToUpper(severity)]; exists {
		severity = mapped
	}
	severity = normalizeSeverity(severity)

	// Extract category (default: "security")
	category := a.extractFieldString(mapping.Mappings.Category, crd.Object, "security")
	if mapping.Mappings.Category != "" && !strings.HasPrefix(mapping.Mappings.Category, ".") {
		category = mapping.Mappings.Category
	}

	// Extract eventType (default: "custom-event")
	eventType := a.extractFieldString(mapping.Mappings.EventType, crd.Object, "custom-event")
	if mapping.Mappings.EventType != "" && !strings.HasPrefix(mapping.Mappings.EventType, ".") {
		eventType = mapping.Mappings.EventType
	}

	// Extract resource reference
	var resourceRef *ResourceRef
	if mapping.Mappings.Resource.Kind != "" || mapping.Mappings.Resource.Name != "" {
		resourceRef = &ResourceRef{}
		if mapping.Mappings.Resource.APIVersion != "" {
			resourceRef.APIVersion = a.extractFieldString(mapping.Mappings.Resource.APIVersion, crd.Object, "")
		}
		resourceRef.Kind = a.extractFieldString(mapping.Mappings.Resource.Kind, crd.Object, "")
		resourceRef.Name = a.extractFieldString(mapping.Mappings.Resource.Name, crd.Object, "")

		// Extract namespace - try from mapping, fallback to metadata.namespace
		if mapping.Mappings.Resource.Namespace != "" {
			resourceRef.Namespace = a.extractFieldString(mapping.Mappings.Resource.Namespace, crd.Object, "")
		}
		if resourceRef.Namespace == "" {
			resourceRef.Namespace = crd.GetNamespace()
		}
	}

	// Default namespace
	namespace := crd.GetNamespace()
	if namespace == "" {
		namespace = "default"
	}

	// Extract details
	details := make(map[string]interface{})
	for key, jsonPath := range mapping.Mappings.Details {
		if value := a.extractField(jsonPath, crd.Object, nil); value != nil {
			details[key] = value
		}
	}

	// Add message to details if present
	if mapping.Mappings.Message != "" {
		if msg := a.extractField(mapping.Mappings.Message, crd.Object, nil); msg != nil {
			details["message"] = fmt.Sprintf("%v", msg)
		}
	}

	event := &Event{
		Source:     mapping.SourceName,
		Category:   category,
		Severity:   severity,
		EventType:  eventType,
		Resource:   resourceRef,
		Namespace:  namespace,
		DetectedAt: time.Now().Format(time.RFC3339),
		Details:    details,
	}

	return event
}

// extractFieldString extracts a field value as a string from an object using dot-notation path
func (a *CRDSourceAdapter) extractFieldString(path string, obj map[string]interface{}, defaultValue string) string {
	value := a.extractField(path, obj, defaultValue)
	if value == nil {
		return defaultValue
	}
	return fmt.Sprintf("%v", value)
}

// extractField extracts a field value from an object using dot-notation path (simplified JSONPath)
// Supports paths like ".report.summary.criticalCount" or static values (non-dot-prefixed)
func (a *CRDSourceAdapter) extractField(path string, obj map[string]interface{}, defaultValue interface{}) interface{} {
	// Static value (not a path)
	if path == "" || !strings.HasPrefix(path, ".") {
		if path == "" {
			return defaultValue
		}
		return path
	}

	// Remove leading dot
	path = strings.TrimPrefix(path, ".")
	parts := strings.Split(path, ".")

	current := interface{}(obj)
	for _, part := range parts {
		// Handle array indexing [0] or [*]
		if strings.Contains(part, "[") {
			// Simple array handling - take first element
			idx := strings.Index(part, "[")
			fieldName := part[:idx]
			if fieldName != "" {
				if m, ok := current.(map[string]interface{}); ok {
					current = m[fieldName]
				}
			}
			// Extract index (simplified - take first element)
			if arr, ok := current.([]interface{}); ok && len(arr) > 0 {
				current = arr[0]
				continue
			}
			return defaultValue
		}

		// Navigate nested map
		if m, ok := current.(map[string]interface{}); ok {
			current = m[part]
		} else {
			return defaultValue
		}

		if current == nil {
			return defaultValue
		}
	}

	return current
}
