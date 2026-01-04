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

package config

import (
	"context"
	"fmt"
	"sync"
	"time"

	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
	"github.com/kube-zen/zen-watcher/pkg/metrics"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// ConfigManager manages configuration from ConfigMaps
type ConfigManager struct {
	clientset kubernetes.Interface
	namespace string

	mu          sync.RWMutex
	baseConfig  map[string]interface{}
	envConfig   map[string]interface{}
	finalConfig map[string]interface{}

	// Informers for ConfigMap watching
	factory      informers.SharedInformerFactory
	baseInformer cache.SharedIndexInformer

	// Configuration callbacks
	onConfigChange []func(map[string]interface{})

	// ConfigMap names
	baseConfigName string
	envConfigName  string

	// Metrics (optional)
	metrics *metrics.Metrics
}

// NewConfigManager creates a new configuration manager
func NewConfigManager(clientset kubernetes.Interface, namespace string) *ConfigManager {
	return NewConfigManagerWithMetrics(clientset, namespace, nil)
}

// NewConfigManagerWithMetrics creates a new configuration manager with metrics
func NewConfigManagerWithMetrics(clientset kubernetes.Interface, namespace string, m *metrics.Metrics) *ConfigManager {
	// Get config names from environment or use defaults
	baseConfigName := getEnv("BASE_CONFIG_NAME", "zen-watcher-base-config")
	envConfigName := getEnv("ENV_CONFIG_NAME", "")

	cm := &ConfigManager{
		clientset:      clientset,
		namespace:      namespace,
		baseConfig:     make(map[string]interface{}),
		envConfig:      make(map[string]interface{}),
		finalConfig:    make(map[string]interface{}),
		baseConfigName: baseConfigName,
		envConfigName:  envConfigName,
		metrics:        m,
	}

	cm.setupInformers()
	return cm
}

// setupInformers configures ConfigMap informers
func (cm *ConfigManager) setupInformers() {
	cm.factory = informers.NewSharedInformerFactoryWithOptions(
		cm.clientset,
		time.Minute,
		informers.WithNamespace(cm.namespace),
	)

	// Base configuration watcher
	cm.baseInformer = cm.factory.Core().V1().ConfigMaps().Informer()
	if _, err := cm.baseInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    cm.handleConfigMapAdd,
		UpdateFunc: cm.handleConfigMapUpdate,
		DeleteFunc: cm.handleConfigMapDelete,
	}); err != nil {
		configLogger.Error(err, "Failed to add event handlers",
			sdklog.Operation("setup_informers"))
		// Note: This is called during initialization, so we log but don't return error
	}
}

// Start starts the configuration manager
func (cm *ConfigManager) Start(ctx context.Context) error {
	// Load initial configuration
	if err := cm.loadInitialConfig(); err != nil {
		configLogger.Warn("Failed to load initial config, using defaults",
			sdklog.Operation("initial_load"),
			sdklog.Error(err))
	}

	// Start informer factory
	cm.factory.Start(ctx.Done())

	// Wait for initial sync
	if !cache.WaitForCacheSync(ctx.Done(), cm.baseInformer.HasSynced) {
		return fmt.Errorf("failed to sync config informers")
	}

	configLogger.Info("ConfigManager started",
		sdklog.Operation("start"),
		sdklog.String("namespace", cm.namespace),
		sdklog.String("base_config", cm.baseConfigName),
		sdklog.String("env_config", cm.envConfigName))

	return nil
}

// loadInitialConfig loads configuration from ConfigMaps before informer is ready
func (cm *ConfigManager) loadInitialConfig() error {
	startTime := time.Now()

	// Load base config
	baseCM, err := cm.clientset.CoreV1().ConfigMaps(cm.namespace).Get(
		context.Background(),
		cm.baseConfigName,
		metav1.GetOptions{},
	)
	if err != nil {
		if errors.IsNotFound(err) {
			if cm.metrics != nil {
				cm.metrics.ConfigMapLoadTotal.WithLabelValues(cm.baseConfigName, "not_found").Inc()
			}
			configLogger.Debug("Base ConfigMap not found, using defaults",
				sdklog.Operation("load_base"),
				sdklog.String("configmap", cm.baseConfigName))
			return nil
		}
		if cm.metrics != nil {
			cm.metrics.ConfigMapLoadTotal.WithLabelValues(cm.baseConfigName, "error").Inc()
			cm.metrics.ConfigMapValidationErrors.WithLabelValues(cm.baseConfigName, "load_error").Inc()
		}
		return fmt.Errorf("failed to load base config: %w", err)
	}

	if cm.metrics != nil {
		cm.metrics.ConfigMapLoadTotal.WithLabelValues(cm.baseConfigName, "success").Inc()
		cm.metrics.ConfigMapReloadDuration.WithLabelValues(cm.baseConfigName).Observe(time.Since(startTime).Seconds())
	}

	cm.processConfigMap(baseCM)

	// Load environment config if specified
	if cm.envConfigName != "" {
		envStartTime := time.Now()
		envCM, err := cm.clientset.CoreV1().ConfigMaps(cm.namespace).Get(
			context.Background(),
			cm.envConfigName,
			metav1.GetOptions{},
		)
		if err != nil {
			if errors.IsNotFound(err) {
				if cm.metrics != nil {
					cm.metrics.ConfigMapLoadTotal.WithLabelValues(cm.envConfigName, "not_found").Inc()
				}
				configLogger.Debug("Environment ConfigMap not found, using base config only",
					sdklog.Operation("load_env"),
					sdklog.String("configmap", cm.envConfigName))
				return nil
			}
			if cm.metrics != nil {
				cm.metrics.ConfigMapLoadTotal.WithLabelValues(cm.envConfigName, "error").Inc()
				cm.metrics.ConfigMapValidationErrors.WithLabelValues(cm.envConfigName, "load_error").Inc()
			}
			return fmt.Errorf("failed to load env config: %w", err)
		}

		if cm.metrics != nil {
			cm.metrics.ConfigMapLoadTotal.WithLabelValues(cm.envConfigName, "success").Inc()
			cm.metrics.ConfigMapReloadDuration.WithLabelValues(cm.envConfigName).Observe(time.Since(envStartTime).Seconds())
		}

		cm.processConfigMap(envCM)
	}

	return nil
}

// handleConfigMapAdd handles ConfigMap addition
func (cm *ConfigManager) handleConfigMapAdd(obj interface{}) {
	cmConfig, ok := obj.(*corev1.ConfigMap)
	if !ok {
		return
	}
	cm.processConfigMap(cmConfig)
}

// handleConfigMapUpdate handles ConfigMap updates
func (cm *ConfigManager) handleConfigMapUpdate(oldObj, newObj interface{}) {
	cmConfig, ok := newObj.(*corev1.ConfigMap)
	if !ok {
		return
	}
	cm.processConfigMap(cmConfig)
}

// handleConfigMapDelete handles ConfigMap deletion
func (cm *ConfigManager) handleConfigMapDelete(obj interface{}) {
	cmConfig, ok := obj.(*corev1.ConfigMap)
	if !ok {
		// Handle DeletedFinalStateUnknown
		if deleted, ok := obj.(cache.DeletedFinalStateUnknown); ok {
			cmConfig, ok = deleted.Obj.(*corev1.ConfigMap)
			if !ok {
				return
			}
		} else {
			return
		}
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Clear config if deleted
	switch cmConfig.Name {
	case cm.baseConfigName:
		cm.baseConfig = make(map[string]interface{})
		configLogger.Info("Base ConfigMap deleted, using defaults",
			sdklog.Operation("configmap_deleted"),
			sdklog.String("configmap", cmConfig.Name))
	case cm.envConfigName:
		cm.envConfig = make(map[string]interface{})
		configLogger.Info("Environment ConfigMap deleted, using base config only",
			sdklog.Operation("configmap_deleted"),
			sdklog.String("configmap", cmConfig.Name))
	}

	// Re-merge configurations
	cm.mergeConfigurations()
	cm.notifyConfigChange()
}

// processConfigMap processes a ConfigMap
func (cm *ConfigManager) processConfigMap(cmConfig *corev1.ConfigMap) {
	startTime := time.Now()

	// Only process our config maps
	if cmConfig.Name != cm.baseConfigName && cmConfig.Name != cm.envConfigName {
		return
	}

	configData, ok := cmConfig.Data["features.yaml"]
	if !ok {
		if cm.metrics != nil {
			cm.metrics.ConfigMapValidationErrors.WithLabelValues(cmConfig.Name, "missing_features_yaml").Inc()
		}
		configLogger.Debug("ConfigMap missing features.yaml, skipping",
			sdklog.Operation("process_configmap"),
			sdklog.String("configmap", cmConfig.Name))
		return
	}

	parsedConfig, err := parseConfigYAML(configData)
	if err != nil {
		if cm.metrics != nil {
			cm.metrics.ConfigMapValidationErrors.WithLabelValues(cmConfig.Name, "parse_error").Inc()
		}
		configLogger.Warn("Failed to parse config from ConfigMap",
			sdklog.Operation("parse_config"),
			sdklog.String("configmap", cmConfig.Name),
			sdklog.Error(err))
		return
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Determine if this is base or environment config
	switch cmConfig.Name {
	case cm.baseConfigName:
		cm.baseConfig = parsedConfig
		configLogger.Debug("Base configuration updated",
			sdklog.Operation("base_config_updated"),
			sdklog.String("configmap", cmConfig.Name))
	case cm.envConfigName:
		cm.envConfig = parsedConfig
		configLogger.Debug("Environment configuration updated",
			sdklog.Operation("env_config_updated"),
			sdklog.String("configmap", cmConfig.Name))
	}

	// Merge configurations with precedence
	mergeStartTime := time.Now()
	cm.mergeConfigurations()
	if cm.metrics != nil {
		// Check for merge conflicts (simplified - could be enhanced)
		// For now, we just track merge duration
		cm.metrics.ConfigMapReloadDuration.WithLabelValues(cmConfig.Name).Observe(time.Since(startTime).Seconds())
	}

	// Notify callbacks
	notifyStartTime := time.Now()
	cm.notifyConfigChange()
	if cm.metrics != nil {
		// Track propagation time (simplified - tracks total notification time)
		cm.metrics.ConfigUpdatePropagationDuration.WithLabelValues("config_manager").Observe(time.Since(notifyStartTime).Seconds())
	}

	_ = mergeStartTime // Avoid unused variable warning
}

// mergeConfigurations merges base and environment configurations
func (cm *ConfigManager) mergeConfigurations() {
	cm.finalConfig = deepMerge(cm.baseConfig, cm.envConfig)
}

// GetConfig returns the current merged configuration
func (cm *ConfigManager) GetConfig() map[string]interface{} {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.finalConfig
}

// GetConfigWithDefaults returns configuration with defaults applied
func (cm *ConfigManager) GetConfigWithDefaults() map[string]interface{} {
	config := cm.GetConfig()
	withDefaults := applyDefaults(config)
	return withDefaults
}

// OnConfigChange registers a callback for configuration changes
func (cm *ConfigManager) OnConfigChange(callback func(map[string]interface{})) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.onConfigChange = append(cm.onConfigChange, callback)
}

// notifyConfigChange notifies all registered callbacks
func (cm *ConfigManager) notifyConfigChange() {
	config := cm.GetConfigWithDefaults()

	cm.mu.RLock()
	callbacks := make([]func(map[string]interface{}), len(cm.onConfigChange))
	copy(callbacks, cm.onConfigChange)
	cm.mu.RUnlock()

	for _, callback := range callbacks {
		callback(config)
	}
}

// parseConfigYAML parses YAML configuration
func parseConfigYAML(yamlData string) (map[string]interface{}, error) {
	var config map[string]interface{}
	err := yaml.Unmarshal([]byte(yamlData), &config)
	return config, err
}

// deepMerge merges two maps with environment config taking precedence
func deepMerge(base, override map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// Copy base config
	for k, v := range base {
		result[k] = v
	}

	// Override with env config
	for k, v := range override {
		if overrideMap, ok := v.(map[string]interface{}); ok {
			if baseMap, ok := result[k].(map[string]interface{}); ok {
				result[k] = deepMerge(baseMap, overrideMap)
				continue
			}
		}
		result[k] = v
	}

	return result
}

// applyDefaults applies default values for missing configuration
func applyDefaults(config map[string]interface{}) map[string]interface{} {
	defaults := map[string]interface{}{
		"worker_pool": map[string]interface{}{
			"enabled":    false,
			"size":       5,
			"queue_size": 1000,
		},
		"event_batching": map[string]interface{}{
			"enabled":        false,
			"batch_size":     50,
			"batch_age":      "10s",
			"flush_interval": "30s",
		},
		"namespace_filtering": map[string]interface{}{
			"enabled":             true,
			"included_namespaces": []interface{}{},
			"excluded_namespaces": []interface{}{"kube-system", "kube-public", "kube-node-lease"},
		},
	}

	return deepMerge(defaults, config)
}
