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
	"os"
	"strconv"
	"time"
)

// HAConfig holds HA-specific optimization configuration
type HAConfig struct {
	AutoScaling       AutoScalingConfig       `json:"auto_scaling,omitempty"`
	DedupOptimization DedupOptimizationConfig `json:"dedup_optimization,omitempty"`
	CacheOptimization CacheOptimizationConfig `json:"cache_optimization,omitempty"`
	LoadBalancing     LoadBalancingConfig     `json:"load_balancing,omitempty"`
	Enabled           bool                    `json:"enabled"`
}

// AutoScalingConfig holds auto-scaling configuration
type AutoScalingConfig struct {
	Enabled        bool   `json:"enabled"`
	MinReplicas    int    `json:"min_replicas"`
	MaxReplicas    int    `json:"max_replicas"`
	TargetCPU      int    `json:"target_cpu"` // percentage
	ScaleUpDelay   string `json:"scale_up_delay"`
	ScaleDownDelay string `json:"scale_down_delay"`
}

// DedupOptimizationConfig holds dynamic dedup window configuration
type DedupOptimizationConfig struct {
	Enabled           bool   `json:"enabled"`
	LowTrafficWindow  string `json:"low_traffic_window"`  // < 50 events/sec
	HighTrafficWindow string `json:"high_traffic_window"` // > 500 events/sec
	AdaptiveWindows   bool   `json:"adaptive_windows"`
}

// CacheOptimizationConfig holds adaptive cache sizing configuration
type CacheOptimizationConfig struct {
	Enabled           bool `json:"enabled"`
	LowTrafficSize    int  `json:"low_traffic_size"`  // events
	HighTrafficSize   int  `json:"high_traffic_size"` // events
	MemoryBasedSizing bool `json:"memory_based_sizing"`
}

// LoadBalancingConfig holds load balancing strategy configuration
type LoadBalancingConfig struct {
	Strategy            string  `json:"strategy"` // "round_robin", "least_loaded", "consistent_hash"
	HealthCheckInterval string  `json:"health_check_interval"`
	RebalanceThreshold  float64 `json:"rebalance_threshold"` // 2x load difference triggers rebalance
}

// LoadHAConfig loads HA configuration from environment variables
func LoadHAConfig() *HAConfig {
	enabled := getEnvBool("HA_OPTIMIZATION_ENABLED", false)
	if !enabled {
		return &HAConfig{Enabled: false}
	}

	return &HAConfig{
		Enabled: true,
		AutoScaling: AutoScalingConfig{
			Enabled:        getEnvBool("HA_AUTO_SCALING_ENABLED", true),
			MinReplicas:    getEnvInt("HA_MIN_REPLICAS", 2),
			MaxReplicas:    getEnvInt("HA_MAX_REPLICAS", 10),
			TargetCPU:      getEnvInt("HA_TARGET_CPU", 70),
			ScaleUpDelay:   getEnv("HA_SCALE_UP_DELAY", "2m"),
			ScaleDownDelay: getEnv("HA_SCALE_DOWN_DELAY", "10m"),
		},
		DedupOptimization: DedupOptimizationConfig{
			Enabled:           getEnvBool("HA_DEDUP_OPTIMIZATION_ENABLED", true),
			LowTrafficWindow:  getEnv("HA_LOW_TRAFFIC_WINDOW", "300s"),
			HighTrafficWindow: getEnv("HA_HIGH_TRAFFIC_WINDOW", "60s"),
			AdaptiveWindows:   getEnvBool("HA_ADAPTIVE_WINDOWS", true),
		},
		CacheOptimization: CacheOptimizationConfig{
			Enabled:           getEnvBool("HA_CACHE_OPTIMIZATION_ENABLED", true),
			LowTrafficSize:    getEnvInt("HA_LOW_TRAFFIC_SIZE", 5000),
			HighTrafficSize:   getEnvInt("HA_HIGH_TRAFFIC_SIZE", 50000),
			MemoryBasedSizing: getEnvBool("HA_MEMORY_BASED_SIZING", true),
		},
		LoadBalancing: LoadBalancingConfig{
			Strategy:            getEnv("HA_LOAD_BALANCING_STRATEGY", "least_loaded"),
			HealthCheckInterval: getEnv("HA_HEALTH_CHECK_INTERVAL", "30s"),
			RebalanceThreshold:  getEnvFloat64("HA_REBALANCE_THRESHOLD", 2.0),
		},
	}
}

// IsHAEnabled returns true if HA mode is enabled
func (c *HAConfig) IsHAEnabled() bool {
	return c != nil && c.Enabled
}

// GetDedupWindowDuration parses the dedup window duration from string
func (c *DedupOptimizationConfig) GetLowTrafficWindow() (time.Duration, error) {
	return time.ParseDuration(c.LowTrafficWindow)
}

// GetHighTrafficWindow parses the high traffic window duration
func (c *DedupOptimizationConfig) GetHighTrafficWindow() (time.Duration, error) {
	return time.ParseDuration(c.HighTrafficWindow)
}

// GetScaleUpDelayDuration parses scale up delay
func (c *AutoScalingConfig) GetScaleUpDelay() (time.Duration, error) {
	return time.ParseDuration(c.ScaleUpDelay)
}

// GetScaleDownDelayDuration parses scale down delay
func (c *AutoScalingConfig) GetScaleDownDelay() (time.Duration, error) {
	return time.ParseDuration(c.ScaleDownDelay)
}

// GetHealthCheckIntervalDuration parses health check interval
func (c *LoadBalancingConfig) GetHealthCheckInterval() (time.Duration, error) {
	return time.ParseDuration(c.HealthCheckInterval)
}

// Helper functions
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvFloat64(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseFloat(value, 64); err == nil {
			return parsed
		}
	}
	return defaultValue
}
