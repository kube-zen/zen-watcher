// Copyright 2025 The Zen Watcher Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may Obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package generic

import (
	"time"
)

// RawEvent represents a raw event before any normalization
// All original data is preserved in RawData
type RawEvent struct {
	Source    string
	Timestamp time.Time
	RawData   map[string]interface{} // All original data preserved
	Metadata  map[string]interface{} // Type, priority from config normalization
}

// SourceConfig represents the adapter configuration from Ingester CRD
type SourceConfig struct {
	Source   string
	Ingester string // informer, webhook, logs, k8s-events (cm/configmap is NOT supported)

	// Adapter-specific configs (only one should be set based on ingester)
	Informer *InformerConfig
	Webhook  *WebhookConfig
	Logs     *LogsConfig
	// Normalization
	Normalization *NormalizationConfig

	// Thresholds (for warnings only)
	Thresholds *ThresholdsConfig

	// Rate limiting
	RateLimit *RateLimitConfig

	// Deduplication (W33 - v1.1)
	Dedup *DedupConfig

	// Processing order and optimization (W33 - v1.1)
	Processing *ProcessingConfig
}

// InformerConfig configuration for informer adapter
type InformerConfig struct {
	GVR           GVRConfig
	Namespace     string
	LabelSelector string
	FieldSelector string
	ResyncPeriod  string
}

// GVRConfig represents GroupVersionResource
type GVRConfig struct {
	Group    string
	Version  string
	Resource string
}

// WebhookConfig configuration for webhook adapter
type WebhookConfig struct {
	Path       string
	Port       int
	BufferSize int
	Auth       *AuthConfig
}

// AuthConfig for webhook authentication
type AuthConfig struct {
	Type       string // none, bearer, basic
	SecretName string
}

// LogsConfig configuration for logs adapter
type LogsConfig struct {
	PodSelector  string
	Container    string
	Patterns     []LogPattern
	SinceSeconds int
	PollInterval string
}

// LogPattern defines a regex pattern to match in logs
type LogPattern struct {
	Regex    string
	Type     string
	Priority float64
}

// NormalizationConfig for normalizing raw events
type NormalizationConfig struct {
	Domain       string
	Type         string
	Priority     map[string]float64 // Source value -> 0.0-1.0
	FieldMapping []FieldMapping
}

// FieldMapping maps fields from raw data
type FieldMapping struct {
	From string // JSONPath
	To   string // Label or field name
}

// ThresholdsConfig for threshold monitoring
type ThresholdsConfig struct {
	ObservationsPerMinute *ThresholdValues
	Custom                []CustomThreshold
}

// ThresholdValues for numeric thresholds
type ThresholdValues struct {
	Warning  int
	Critical int
}

// CustomThreshold for custom threshold checks
type CustomThreshold struct {
	Name     string
	Field    string // JSONPath
	Operator string // >, <, ==, !=, contains
	Value    interface{}
	Message  string
}

// RateLimitConfig for rate limiting
type RateLimitConfig struct {
	ObservationsPerMinute int
	Burst                 int
}

// DedupConfig holds deduplication configuration (W33 - v1.1)
type DedupConfig struct {
	Enabled            bool
	Window             string
	Strategy           string                 // fingerprint, key, event-stream
	Fields             []string               // For key-based strategy
	MaxEventsPerWindow int                    // For event-stream strategy
	Config             map[string]interface{} // Strategy-specific configuration
}

// ProcessingConfig holds processing order settings (W33 - v1.1)
// Note: Auto-optimization has been removed. Only manual order selection (filter_first, dedup_first) is supported.
type ProcessingConfig struct {
	Order string // filter_first or dedup_first
}
