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

package filter

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// FilterConfig represents the filter configuration loaded from ConfigMap
type FilterConfig struct {
	// Expression is an optional filter expression (v1.1 feature)
	// If set, this expression is evaluated instead of list-based filters
	// Example: "(severity >= HIGH) AND (category IN [security, compliance])"
	Expression string `json:"expression,omitempty"`

	// Global namespace filtering (applies to all sources)
	// These are merged with source-specific namespace filters
	GlobalNamespaceFilter *GlobalNamespaceFilter `json:"globalNamespaceFilter,omitempty"`

	// Sources is a map of source name to SourceFilter
	// If a source is not in this map, it is allowed by default
	// Note: Sources are ignored when Expression is set
	Sources map[string]SourceFilter `json:"sources"`
}

// GlobalNamespaceFilter defines global namespace filtering rules
type GlobalNamespaceFilter struct {
	// Enabled controls whether global namespace filtering is active
	Enabled bool `json:"enabled,omitempty"`

	// IncludedNamespaces is a list of namespaces to include (if set, only these are allowed)
	// Empty list means all namespaces are allowed (unless ExcludedNamespaces is set)
	IncludedNamespaces []string `json:"includedNamespaces,omitempty"`

	// ExcludedNamespaces is a list of namespaces to exclude
	ExcludedNamespaces []string `json:"excludedNamespaces,omitempty"`
}

// SourceFilter defines filtering rules for a specific source
type SourceFilter struct {
	// MinSeverity is the minimum severity level to allow (e.g., "MEDIUM", "HIGH", "CRITICAL")
	// Severity levels: CRITICAL > HIGH > MEDIUM > LOW > UNKNOWN
	MinSeverity string `json:"minSeverity,omitempty"`

	// ExcludeEventTypes is a list of event types to exclude (e.g., ["audit", "info"])
	ExcludeEventTypes []string `json:"excludeEventTypes,omitempty"`

	// IncludeEventTypes is a list of event types to include (if set, only these are allowed)
	IncludeEventTypes []string `json:"includeEventTypes,omitempty"`

	// ExcludeNamespaces is a list of namespaces to exclude
	ExcludeNamespaces []string `json:"excludeNamespaces,omitempty"`

	// IncludeNamespaces is a list of namespaces to include (if set, only these are allowed)
	IncludeNamespaces []string `json:"includeNamespaces,omitempty"`

	// ExcludeKinds is a list of resource kinds to exclude (e.g., ["Pod", "Deployment"])
	ExcludeKinds []string `json:"excludeKinds,omitempty"`

	// IncludeKinds is a list of resource kinds to include (if set, only these are allowed)
	IncludeKinds []string `json:"includeKinds,omitempty"`

	// ExcludeCategories is a list of categories to exclude (e.g., ["compliance"])
	ExcludeCategories []string `json:"excludeCategories,omitempty"`

	// IncludeCategories is a list of categories to include (if set, only these are allowed)
	IncludeCategories []string `json:"includeCategories,omitempty"`

	// IncludeSeverity is a list of severity levels to include (if set, only these are allowed)
	// Example: ["CRITICAL", "HIGH"] - only allow CRITICAL and HIGH severity
	IncludeSeverity []string `json:"includeSeverity,omitempty"`

	// ExcludeRules is a list of rule names to exclude (e.g., ["disallow-latest-tag"])
	// Used for sources like Kyverno where observations have a rule in details.rule
	ExcludeRules []string `json:"excludeRules,omitempty"`

	// IgnoreKinds is an alias for ExcludeKinds (convenience for kubernetesEvents source)
	// If both IgnoreKinds and ExcludeKinds are set, they are merged
	IgnoreKinds []string `json:"ignoreKinds,omitempty"`

	// Enabled controls whether this source is enabled (default: true)
	Enabled *bool `json:"enabled,omitempty"`
}

// LoadFilterConfig loads filter configuration from ConfigMap
// ConfigMap name and namespace can be set via environment variables:
// - FILTER_CONFIGMAP_NAME (default: "zen-watcher-filter")
// - FILTER_CONFIGMAP_NAMESPACE (default: "zen-system")
// - FILTER_CONFIGMAP_KEY (default: "filter.json")
func LoadFilterConfig(clientSet kubernetes.Interface) (*FilterConfig, error) {
	configMapName := os.Getenv("FILTER_CONFIGMAP_NAME")
	if configMapName == "" {
		configMapName = "zen-watcher-filter"
	}

	configMapNamespace := os.Getenv("FILTER_CONFIGMAP_NAMESPACE")
	if configMapNamespace == "" {
		configMapNamespace = os.Getenv("WATCH_NAMESPACE")
		if configMapNamespace == "" {
			configMapNamespace = "zen-system"
		}
	}

	configMapKey := os.Getenv("FILTER_CONFIGMAP_KEY")
	if configMapKey == "" {
		configMapKey = "filter.json"
	}

	// Try to load ConfigMap
	cm, err := clientSet.CoreV1().ConfigMaps(configMapNamespace).Get(
		context.Background(),
		configMapName,
		metav1.GetOptions{},
	)
	if err != nil {
		// ConfigMap not found - return default (allow all) config
		logger := sdklog.NewLogger("zen-watcher-filter")
		logger.Debug("Filter ConfigMap not found, using default (allow all) filter",
			sdklog.Operation("config_load"),
			sdklog.String("namespace", configMapNamespace),
			sdklog.String("configmap_name", configMapName))
		return &FilterConfig{
			Sources: make(map[string]SourceFilter),
		}, nil
	}

	// Extract filter.json from ConfigMap
	filterJSON, found := cm.Data[configMapKey]
	if !found {
		// Key not found - return default config
		logger := sdklog.NewLogger("zen-watcher-filter")
		logger.Debug("Filter key not found in ConfigMap, using default (allow all) filter",
			sdklog.Operation("config_load"),
			sdklog.String("namespace", configMapNamespace),
			sdklog.String("configmap_name", configMapName),
			sdklog.String("key", configMapKey))
		return &FilterConfig{
			Sources: make(map[string]SourceFilter),
		}, nil
	}

	// Parse JSON
	var config FilterConfig
	if err := json.Unmarshal([]byte(filterJSON), &config); err != nil {
		return nil, fmt.Errorf("failed to parse filter config: %w", err)
	}

	logger := sdklog.NewLogger("zen-watcher-filter")
	logger.Info("Loaded filter configuration from ConfigMap",
		sdklog.Operation("config_load"),
		sdklog.String("namespace", configMapNamespace),
		sdklog.String("configmap_name", configMapName))
	return &config, nil
}

// GetSourceFilter returns the filter configuration for a specific source
// Returns nil if no filter is configured (allow all)
func (fc *FilterConfig) GetSourceFilter(source string) *SourceFilter {
	if fc == nil || fc.Sources == nil {
		return nil
	}
	filter, exists := fc.Sources[strings.ToLower(source)]
	if !exists {
		return nil
	}
	// Normalize IgnoreKinds into ExcludeKinds
	if len(filter.IgnoreKinds) > 0 {
		// Merge IgnoreKinds into ExcludeKinds (deduplicate, preserve case)
		excludeMap := make(map[string]string) // lowercase -> original case
		// First, add existing ExcludeKinds
		for _, k := range filter.ExcludeKinds {
			lower := strings.ToLower(k)
			if _, exists := excludeMap[lower]; !exists {
				excludeMap[lower] = k
			}
		}
		// Then, add IgnoreKinds (only if not already present)
		for _, k := range filter.IgnoreKinds {
			lower := strings.ToLower(k)
			if _, exists := excludeMap[lower]; !exists {
				excludeMap[lower] = k
			}
		}
		// Rebuild ExcludeKinds list
		filter.ExcludeKinds = make([]string, 0, len(excludeMap))
		for _, v := range excludeMap {
			filter.ExcludeKinds = append(filter.ExcludeKinds, v)
		}
	}
	return &filter
}

// IsSourceEnabled checks if a source is enabled (default: true)
func (sf *SourceFilter) IsSourceEnabled() bool {
	if sf == nil || sf.Enabled == nil {
		return true // Default: enabled
	}
	return *sf.Enabled
}
