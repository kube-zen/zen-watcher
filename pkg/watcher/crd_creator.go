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

package watcher

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// watcherLogger is shared across watcher package (defined in adapter_factory.go, reused here)

// CRDCreator handles creation of resources to any GVR (GroupVersionResource).
// This is a completely generic creator that can write to any Kubernetes resource type
// (CRDs, core resources like ConfigMaps, Secrets, etc.). It's similar to zen-egress's CRDDispatcher.
// There are no special cases - observations and ConfigMaps are just examples in documentation.
// H037: GVR writes are restricted by allowlist
type CRDCreator struct {
	dynClient dynamic.Interface
	gvr       schema.GroupVersionResource
	allowlist *GVRAllowlist // H037: GVR allowlist enforcement
}

// NewCRDCreator creates a new generic CRD creator that can write to any GVR.
// H037: GVR writes are restricted by allowlist
func NewCRDCreator(dynClient dynamic.Interface, gvr schema.GroupVersionResource, allowlist *GVRAllowlist) *CRDCreator {
	return &CRDCreator{
		dynClient: dynClient,
		gvr:       gvr,
		allowlist: allowlist,
	}
}

// CreateCRD creates a resource from an unstructured observation.
// The observation structure is converted generically to match the target GVR's format.
// The observation spec is copied to the target resource spec (or equivalent field).
// H037: Validates GVR and namespace against allowlist before creating
func (cc *CRDCreator) CreateCRD(ctx context.Context, observation *unstructured.Unstructured) error {
	// Extract namespace
	namespace, found := extractNamespace(observation)
	if !found || namespace == "" {
		namespace = "default"
	}

	// H037: Validate GVR and namespace against allowlist (hard safety rail)
	// This is the second layer of defense - even if ObservationCreator validation is bypassed
	if cc.allowlist != nil {
		if err := cc.allowlist.IsAllowed(cc.gvr, namespace); err != nil {
			// Check if this is a security policy violation
			isSecurityViolation := errors.Is(err, ErrGVRNotAllowed) ||
				errors.Is(err, ErrGVRDenied) ||
				errors.Is(err, ErrNamespaceNotAllowed) ||
				errors.Is(err, ErrClusterScopedNotAllowed)

			if isSecurityViolation {
				watcherLogger.WithContext(ctx).Warn("GVR write blocked by security policy",
					sdklog.Operation("crd_create_blocked_security"),
					sdklog.ErrorCode("SECURITY_POLICY_VIOLATION"),
					sdklog.String("gvr", cc.gvr.String()),
					sdklog.Namespace(namespace),
					sdklog.Error(err))
			} else {
				watcherLogger.WithContext(ctx).Error(err, "GVR write blocked by allowlist",
					sdklog.Operation("crd_create_blocked"),
					sdklog.ErrorCode("GVR_BLOCKED"),
					sdklog.String("gvr", cc.gvr.String()),
					sdklog.Namespace(namespace))
			}
			return fmt.Errorf("GVR write blocked: %w", err)
		}
	}

	// Convert observation to target CRD format
	crd := cc.convertToCRD(observation)

	// Create the CRD
	deliveryStartTime := time.Now()
	createdCRD, err := cc.dynClient.Resource(cc.gvr).Namespace(namespace).Create(ctx, crd, metav1.CreateOptions{})
	deliveryDuration := time.Since(deliveryStartTime)

	if err != nil {
		errorType := classifyError(err)
		watcherLogger.WithContext(ctx).Error(err, "Failed to create resource",
			sdklog.Operation("crd_create"),
			sdklog.ErrorCode("CRD_CREATE_ERROR"),
			sdklog.String("gvr", cc.gvr.String()),
			sdklog.String("group", cc.gvr.Group),
			sdklog.String("version", cc.gvr.Version),
			sdklog.String("resource", cc.gvr.Resource),
			sdklog.Namespace(namespace),
			sdklog.String("error_type", errorType))
		return fmt.Errorf("failed to create resource %s/%s/%s in namespace %s: %w",
			cc.gvr.Group, cc.gvr.Version, cc.gvr.Resource, namespace, err)
	}

	watcherLogger.WithContext(ctx).Debug("Created CRD successfully",
		sdklog.Operation("crd_create"),
		sdklog.String("gvr", cc.gvr.String()),
		sdklog.Namespace(namespace),
		sdklog.Name(createdCRD.GetName()),
		sdklog.Int64("delivery_duration_ms", deliveryDuration.Milliseconds()))

	return nil
}

// convertToCRD converts an observation to the target CRD/resource format.
// This is a completely generic conversion that works with any GVR/CRD.
// The observation spec is copied to the target resource spec.
// No special cases - works the same for observations, ConfigMaps, or any custom CRD.
func (cc *CRDCreator) convertToCRD(observation *unstructured.Unstructured) *unstructured.Unstructured {
	// Extract observation spec (this is the normalized event data)
	// This will be copied to the target resource spec
	spec, _ := extractMap(observation.Object, "spec")
	if spec == nil {
		spec = make(map[string]interface{})
	}

	// Extract namespace
	namespace, _ := extractNamespace(observation)
	if namespace == "" {
		namespace = "default"
	}

	// Determine kind and apiVersion
	kind := determineKindFromResource(cc.gvr.Resource)
	apiVersion := buildAPIVersionFromGVR(cc.gvr)

	// Extract source for name prefix
	namePrefix := extractNamePrefixFromSpec(spec)

	// Extract labels from observation metadata
	labels, _ := extractMap(observation.Object, "metadata", "labels")
	if labels == nil {
		labels = make(map[string]interface{})
	}
	source, _ := extractStringFromMap(spec, "source")
	if source != "" {
		labels["source"] = source
	}

	// Build target resource (completely generic - works for any CRD/resource)
	// The spec from the observation is copied to the target resource spec
	targetResource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]interface{}{
				"generateName": namePrefix + "-",
				"namespace":    namespace,
				"labels":       labels,
			},
			"spec": spec, // Generic: copy observation spec to target spec
		},
	}

	// Add annotations if present
	if annotations, ok := extractMap(observation.Object, "metadata", "annotations"); ok && annotations != nil {
		if err := unstructured.SetNestedMap(targetResource.Object, annotations, "metadata", "annotations"); err != nil {
			watcherLogger.Warn("Failed to set annotations",
				sdklog.Operation("create_target_resource"),
				sdklog.ErrorCode("ANNOTATION_SET_ERROR"),
				sdklog.Error(err))
		}
	}

	return targetResource
}

// extractNamespace extracts namespace from observation
func extractNamespace(observation *unstructured.Unstructured) (string, bool) {
	if ns := observation.GetNamespace(); ns != "" {
		return ns, true
	}
	// Try to extract from metadata
	if ns, ok := extractStringFromMap(observation.Object, "metadata", "namespace"); ok && ns != "" {
		return ns, true
	}
	// Try to extract from spec.resource.namespace
	if resource, ok := extractMap(observation.Object, "spec", "resource"); ok {
		if ns, ok := extractStringFromMap(resource, "namespace"); ok && ns != "" {
			return ns, true
		}
	}
	return "", false
}

// extractStringFromMap safely extracts a string value from nested maps
func extractStringFromMap(m map[string]interface{}, keys ...string) (string, bool) {
	current := m
	for i, key := range keys {
		if i == len(keys)-1 {
			// Last key - return string value
			if val, ok := current[key]; ok {
				if str, ok := val.(string); ok {
					return str, true
				}
			}
			return "", false
		}
		// Navigate nested structure
		if next, ok := current[key].(map[string]interface{}); ok {
			current = next
		} else {
			return "", false
		}
	}
	return "", false
}

// extractMap safely extracts a map value from nested maps
func extractMap(m map[string]interface{}, keys ...string) (map[string]interface{}, bool) {
	current := m
	for i, key := range keys {
		if i == len(keys)-1 {
			// Last key - return map value
			if val, ok := current[key]; ok {
				if mp, ok := val.(map[string]interface{}); ok {
					return mp, true
				}
			}
			return nil, false
		}
		// Navigate nested structure
		if next, ok := current[key].(map[string]interface{}); ok {
			current = next
		} else {
			return nil, false
		}
	}
	return nil, false
}

// classifyError classifies error types for metrics
func classifyError(err error) string {
	if err == nil {
		return ""
	}
	errMsg := strings.ToLower(err.Error())
	if strings.Contains(errMsg, "already exists") {
		return "already_exists"
	} else if strings.Contains(errMsg, "forbidden") {
		return "forbidden"
	} else if strings.Contains(errMsg, "not found") {
		return "not_found"
	}
	return "create_failed"
}

// determineKindFromResource determines kind from resource name (plural -> singular, capitalized)
func determineKindFromResource(resource string) string {
	kind := resource
	// Simple plural handling: remove trailing 's' if present
	if len(kind) > 1 && kind[len(kind)-1] == 's' {
		if len(kind) > 2 && kind[len(kind)-2] != 's' && !strings.HasSuffix(kind, "us") {
			kind = kind[:len(kind)-1] // Remove trailing 's'
		}
	}
	// Capitalize first letter
	if len(kind) > 0 {
		kind = strings.ToUpper(kind[:1]) + kind[1:]
	}
	return kind
}

// buildAPIVersionFromGVR builds apiVersion from GVR
func buildAPIVersionFromGVR(gvr schema.GroupVersionResource) string {
	apiVersion := gvr.Version
	if gvr.Group != "" {
		apiVersion = fmt.Sprintf("%s/%s", gvr.Group, gvr.Version)
	}
	return apiVersion
}

// extractNamePrefixFromSpec extracts name prefix from source in spec
func extractNamePrefixFromSpec(spec map[string]interface{}) string {
	source, _ := extractStringFromMap(spec, "source")
	if source == "" {
		source = "event"
	}
	namePrefix := source
	if len(namePrefix) > 20 {
		namePrefix = namePrefix[:20]
	}
	return namePrefix
}
