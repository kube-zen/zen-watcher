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

package sdk

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Ingester represents an Ingester CRD (v1alpha1)
type Ingester struct {
	APIVersion string            `json:"apiVersion" yaml:"apiVersion"`
	Kind       string            `json:"kind" yaml:"kind"`
	Metadata   metav1.ObjectMeta `json:"metadata" yaml:"metadata"`
	Spec       IngesterSpec      `json:"spec" yaml:"spec"`
}


// IngesterSource represents a single source configuration in the multi-source array
type IngesterSource struct {
	// Name is the source name (DNS label)
	Name string `json:"name" yaml:"name"`
	// Type is the source type: informer, logs, or webhook
	Type string `json:"type" yaml:"type"`
	// Informer configuration (required if Type == "informer")
	Informer *InformerConfig `json:"informer,omitempty" yaml:"informer,omitempty"`
	// Logs configuration (required if Type == "logs")
	Logs *LogsConfig `json:"logs,omitempty" yaml:"logs,omitempty"`
	// Webhook configuration (required if Type == "webhook")
	Webhook *WebhookConfig `json:"webhook,omitempty" yaml:"webhook,omitempty"`
}

// IngesterSpec represents the spec section of an Ingester CRD
type IngesterSpec struct {
	// Multi-source configuration (optional). If set, creates one pipeline per source.
	// If not set, legacy Source/Ingester fields are used for backward compatibility.
	Sources []IngesterSource `json:"sources,omitempty" yaml:"sources,omitempty"`

	// Legacy fields for backward compatibility (use Sources instead)

	Source        string               `json:"source,omitempty" yaml:"source,omitempty"`
	Ingester      string               `json:"ingester,omitempty" yaml:"ingester,omitempty"`
	Destinations  []Destination        `json:"destinations" yaml:"destinations"`
	Deduplication *DeduplicationConfig `json:"deduplication,omitempty" yaml:"deduplication,omitempty"`
	Filters       *FilterConfig        `json:"filters,omitempty" yaml:"filters,omitempty"`
	Optimization  *OptimizationConfig  `json:"optimization,omitempty" yaml:"optimization,omitempty"`
	Processing    *ProcessingConfig    `json:"processing,omitempty" yaml:"processing,omitempty"`
	Informer      *InformerConfig      `json:"informer,omitempty" yaml:"informer,omitempty"`
	Webhook       *WebhookConfig       `json:"webhook,omitempty" yaml:"webhook,omitempty"`
	Logs          *LogsConfig          `json:"logs,omitempty" yaml:"logs,omitempty"`
	K8sEvents     *K8sEventsConfig     `json:"k8sEvents,omitempty" yaml:"k8sEvents,omitempty"`
}

// Destination represents a destination configuration
type Destination struct {
	Type    string                `json:"type" yaml:"type"`
	Value   string                `json:"value,omitempty" yaml:"value,omitempty"` // Used if GVR is not specified
	GVR     *GVRConfig            `json:"gvr,omitempty" yaml:"gvr,omitempty"`     // Full GVR specification (takes precedence over value)
	Mapping *NormalizationMapping `json:"mapping,omitempty" yaml:"mapping,omitempty"`
}

// NormalizationMapping represents normalization configuration
type NormalizationMapping struct {
	Domain       string                 `json:"domain,omitempty" yaml:"domain,omitempty"`
	Type         string                 `json:"type,omitempty" yaml:"type,omitempty"`
	Priority     map[string]interface{} `json:"priority,omitempty" yaml:"priority,omitempty"`
	SeverityMap  map[string]interface{} `json:"severityMap,omitempty" yaml:"severityMap,omitempty"`
	FieldMapping []FieldMapping         `json:"fieldMapping,omitempty" yaml:"fieldMapping,omitempty"`
	Resource     map[string]interface{} `json:"resource,omitempty" yaml:"resource,omitempty"`
	Templates    map[string]interface{} `json:"templates,omitempty" yaml:"templates,omitempty"`
}

// FieldMapping represents a field transformation rule
type FieldMapping struct {
	From      string `json:"from" yaml:"from"`
	To        string `json:"to" yaml:"to"`
	Transform string `json:"transform,omitempty" yaml:"transform,omitempty"`
}

// DeduplicationConfig represents deduplication configuration
type DeduplicationConfig struct {
	Enabled       *bool    `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Fields        []string `json:"fields,omitempty" yaml:"fields,omitempty"`
	LearningRate  *float64 `json:"learningRate,omitempty" yaml:"learningRate,omitempty"`
	MinChange     *float64 `json:"minChange,omitempty" yaml:"minChange,omitempty"`
	Strategy      string   `json:"strategy,omitempty" yaml:"strategy,omitempty"`
	Window        string   `json:"window,omitempty" yaml:"window,omitempty"`
	WindowSeconds *int     `json:"windowSeconds,omitempty" yaml:"windowSeconds,omitempty"`
}

// FilterConfig represents filter configuration
type FilterConfig struct {
	MinPriority       *float64 `json:"minPriority,omitempty" yaml:"minPriority,omitempty"`
	MinSeverity       string   `json:"minSeverity,omitempty" yaml:"minSeverity,omitempty"`
	IncludeNamespaces []string `json:"includeNamespaces,omitempty" yaml:"includeNamespaces,omitempty"`
	ExcludeNamespaces []string `json:"excludeNamespaces,omitempty" yaml:"excludeNamespaces,omitempty"`
}

// OptimizationConfig represents optimization configuration
type OptimizationConfig struct {
	Order      string                  `json:"order,omitempty" yaml:"order,omitempty"`
	Thresholds *OptimizationThresholds `json:"thresholds,omitempty" yaml:"thresholds,omitempty"`
}

// OptimizationThresholds holds optimization thresholds
type OptimizationThresholds struct {
	DedupEffectiveness    *ThresholdRange `json:"dedupEffectiveness,omitempty" yaml:"dedupEffectiveness,omitempty"`
	LowSeverityPercent    *ThresholdRange `json:"lowSeverityPercent,omitempty" yaml:"lowSeverityPercent,omitempty"`
	ObservationsPerMinute *ThresholdRange `json:"observationsPerMinute,omitempty" yaml:"observationsPerMinute,omitempty"`
}

// ThresholdRange holds warning and critical thresholds
type ThresholdRange struct {
	Warning  float64 `json:"warning" yaml:"warning"`
	Critical float64 `json:"critical" yaml:"critical"`
}

// ProcessingConfig represents processing configuration
type ProcessingConfig struct {
	Order string `json:"order,omitempty" yaml:"order,omitempty"` // filter_first or dedup_first
}

// InformerConfig represents informer-specific configuration
type InformerConfig struct {
	GVR           *GVRConfig `json:"gvr,omitempty" yaml:"gvr,omitempty"`
	Namespace     string     `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	LabelSelector string     `json:"labelSelector,omitempty" yaml:"labelSelector,omitempty"`
	FieldSelector string     `json:"fieldSelector,omitempty" yaml:"fieldSelector,omitempty"`
	ResyncPeriod  string     `json:"resyncPeriod,omitempty" yaml:"resyncPeriod,omitempty"`
}

// GVRConfig represents GroupVersionResource
type GVRConfig struct {
	Group    string `json:"group" yaml:"group"`
	Version  string `json:"version" yaml:"version"`
	Resource string `json:"resource" yaml:"resource"`
}

// WebhookConfig represents webhook-specific configuration
type WebhookConfig struct {
	Path      string           `json:"path,omitempty" yaml:"path,omitempty"`
	Auth      *AuthConfig      `json:"auth,omitempty" yaml:"auth,omitempty"`
	RateLimit *RateLimitConfig `json:"rateLimit,omitempty" yaml:"rateLimit,omitempty"`
}

// AuthConfig represents authentication configuration
type AuthConfig struct {
	Type      string `json:"type,omitempty" yaml:"type,omitempty"`
	SecretRef string `json:"secretRef,omitempty" yaml:"secretRef,omitempty"`
}

// RateLimitConfig represents rate limiting configuration
type RateLimitConfig struct {
	RequestsPerMinute int `json:"requestsPerMinute,omitempty" yaml:"requestsPerMinute,omitempty"`
}

// LogsConfig represents logs-specific configuration
type LogsConfig struct {
	PodSelector  string       `json:"podSelector,omitempty" yaml:"podSelector,omitempty"`
	Container    string       `json:"container,omitempty" yaml:"container,omitempty"`
	Patterns     []LogPattern `json:"patterns,omitempty" yaml:"patterns,omitempty"`
	SinceSeconds int          `json:"sinceSeconds,omitempty" yaml:"sinceSeconds,omitempty"`
	PollInterval string       `json:"pollInterval,omitempty" yaml:"pollInterval,omitempty"`
}

// LogPattern defines a regex pattern to match in logs
type LogPattern struct {
	Regex    string  `json:"regex" yaml:"regex"`
	Type     string  `json:"type,omitempty" yaml:"type,omitempty"`
	Priority float64 `json:"priority,omitempty" yaml:"priority,omitempty"`
}

// K8sEventsConfig represents Kubernetes events configuration
type K8sEventsConfig struct {
	InvolvedObjectKinds []string `json:"involvedObjectKinds,omitempty" yaml:"involvedObjectKinds,omitempty"`
}
