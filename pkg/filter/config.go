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
	sdkfilter "github.com/kube-zen/zen-sdk/pkg/filter"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// LoadFilterConfig loads filter configuration from ConfigMap and returns zen-sdk FilterConfig
// ConfigMap name and namespace can be set via environment variables:
// - FILTER_CONFIGMAP_NAME (default: "zen-watcher-filter")
// - FILTER_CONFIGMAP_NAMESPACE (default: "zen-system")
// - FILTER_CONFIGMAP_KEY (default: "filter.json")
func LoadFilterConfig(clientSet kubernetes.Interface) (*sdkfilter.FilterConfig, error) {
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
		return &sdkfilter.FilterConfig{
			Sources: make(map[string]sdkfilter.SourceFilter),
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
		return &sdkfilter.FilterConfig{
			Sources: make(map[string]sdkfilter.SourceFilter),
		}, nil
	}

	// Parse JSON
	var config sdkfilter.FilterConfig
	if err := json.Unmarshal([]byte(filterJSON), &config); err != nil {
		return nil, fmt.Errorf("failed to parse filter config: %w", err)
	}

	// Normalize IgnoreKinds into ExcludeKinds for all sources
	for sourceName, sourceFilter := range config.Sources {
		if len(sourceFilter.IgnoreKinds) > 0 {
			// Merge IgnoreKinds into ExcludeKinds (deduplicate, preserve case)
			excludeMap := make(map[string]string) // lowercase -> original case
			// First, add existing ExcludeKinds
			for _, k := range sourceFilter.ExcludeKinds {
				lower := strings.ToLower(k)
				if _, exists := excludeMap[lower]; !exists {
					excludeMap[lower] = k
				}
			}
			// Then, add IgnoreKinds (only if not already present)
			for _, k := range sourceFilter.IgnoreKinds {
				lower := strings.ToLower(k)
				if _, exists := excludeMap[lower]; !exists {
					excludeMap[lower] = k
				}
			}
			// Rebuild ExcludeKinds list
			sourceFilter.ExcludeKinds = make([]string, 0, len(excludeMap))
			for _, v := range excludeMap {
				sourceFilter.ExcludeKinds = append(sourceFilter.ExcludeKinds, v)
			}
			config.Sources[sourceName] = sourceFilter
		}
	}

	logger := sdklog.NewLogger("zen-watcher-filter")
	logger.Info("Loaded filter configuration from ConfigMap",
		sdklog.Operation("config_load"),
		sdklog.String("namespace", configMapNamespace),
		sdklog.String("configmap_name", configMapName))
	return &config, nil
}
