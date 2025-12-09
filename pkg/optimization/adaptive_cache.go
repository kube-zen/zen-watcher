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

package optimization

import (
	"runtime"
	"sync"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/config"
	"github.com/kube-zen/zen-watcher/pkg/logger"
)

// AdaptiveCacheManager manages adaptive cache sizing based on memory pressure and traffic
type AdaptiveCacheManager struct {
	haConfig        *config.CacheOptimizationConfig
	currentSize     int
	targetSize      int
	lowTrafficSize  int
	highTrafficSize int
	sizeMu          sync.RWMutex
	memoryPressure  float64 // 0.0-1.0, where 1.0 is high pressure
	eventsPerSec    float64
	updateTicker    *time.Ticker
	stopChan        chan struct{}
	wg              sync.WaitGroup
	cacheHitRate    float64
	gcFrequency     float64 // garbage collection frequency per minute
}

// NewAdaptiveCacheManager creates a new adaptive cache manager
func NewAdaptiveCacheManager(haConfig *config.CacheOptimizationConfig, initialSize int) *AdaptiveCacheManager {
	if haConfig == nil || !haConfig.Enabled {
		return nil
	}

	lowSize := haConfig.LowTrafficSize
	highSize := haConfig.HighTrafficSize

	if lowSize <= 0 {
		lowSize = 5000
	}
	if highSize <= 0 {
		highSize = 50000
	}

	return &AdaptiveCacheManager{
		haConfig:        haConfig,
		currentSize:     initialSize,
		targetSize:      initialSize,
		lowTrafficSize:  lowSize,
		highTrafficSize: highSize,
		stopChan:        make(chan struct{}),
	}
}

// Start begins the adaptive cache sizing loop
func (acm *AdaptiveCacheManager) Start(updateInterval time.Duration) {
	if acm == nil {
		return
	}

	acm.updateTicker = time.NewTicker(updateInterval)
	acm.wg.Add(1)

	go func() {
		defer acm.wg.Done()
		for {
			select {
			case <-acm.updateTicker.C:
				acm.AdjustCacheSize()
			case <-acm.stopChan:
				return
			}
		}
	}()
}

// Stop stops the adaptive cache manager
func (acm *AdaptiveCacheManager) Stop() {
	if acm == nil {
		return
	}

	if acm.updateTicker != nil {
		acm.updateTicker.Stop()
	}
	close(acm.stopChan)
	acm.wg.Wait()
}

// UpdateMetrics updates metrics used for cache sizing decisions
func (acm *AdaptiveCacheManager) UpdateMetrics(eventsPerSec, cacheHitRate, gcFrequency float64) {
	if acm == nil {
		return
	}

	acm.sizeMu.Lock()
	defer acm.sizeMu.Unlock()

	acm.eventsPerSec = eventsPerSec
	acm.cacheHitRate = cacheHitRate
	acm.gcFrequency = gcFrequency
}

// AdjustCacheSize adjusts the cache size based on memory pressure and traffic
func (acm *AdaptiveCacheManager) AdjustCacheSize() {
	if acm == nil {
		return
	}

	acm.sizeMu.Lock()
	defer acm.sizeMu.Unlock()

	// Calculate memory pressure
	memoryPressure := acm.calculateMemoryPressure()

	// Determine target size based on traffic and memory
	var targetSize int

	if acm.haConfig.MemoryBasedSizing {
		// Memory-based sizing: adjust based on memory pressure
		if memoryPressure > 0.8 {
			// High memory pressure: use low traffic size
			targetSize = acm.lowTrafficSize
		} else if memoryPressure < 0.3 {
			// Low memory pressure: can use high traffic size
			targetSize = acm.highTrafficSize
		} else {
			// Medium memory pressure: interpolate
			factor := (0.8 - memoryPressure) / 0.5 // 0.0-1.0
			if factor < 0 {
				factor = 0
			}
			if factor > 1 {
				factor = 1
			}
			targetSize = int(float64(acm.lowTrafficSize) + factor*float64(acm.highTrafficSize-acm.lowTrafficSize))
		}
	} else {
		// Traffic-based sizing
		if acm.eventsPerSec < 50 {
			targetSize = acm.lowTrafficSize
		} else if acm.eventsPerSec > 500 {
			targetSize = acm.highTrafficSize
		} else {
			// Interpolate between low and high based on traffic
			factor := (acm.eventsPerSec - 50) / 450.0 // 0.0-1.0
			if factor < 0 {
				factor = 0
			}
			if factor > 1 {
				factor = 1
			}
			targetSize = int(float64(acm.lowTrafficSize) + factor*float64(acm.highTrafficSize-acm.lowTrafficSize))
		}
	}

	// Update target size
	oldSize := acm.targetSize
	acm.targetSize = targetSize
	acm.memoryPressure = memoryPressure

	// Log significant changes
	if oldSize != targetSize {
		logger.Info("Cache size adjustment",
			logger.Fields{
				Component: "cache",
				Operation: "cache_size_adjust",
				Additional: map[string]interface{}{
					"old_size":        oldSize,
					"new_size":        targetSize,
					"memory_pressure": memoryPressure,
					"events_per_sec":  acm.eventsPerSec,
					"cache_hit_rate":  acm.cacheHitRate,
				},
			})
	}
}

// GetTargetSize returns the target cache size
func (acm *AdaptiveCacheManager) GetTargetSize() int {
	if acm == nil {
		return 10000 // default
	}

	acm.sizeMu.RLock()
	defer acm.sizeMu.RUnlock()
	return acm.targetSize
}

// calculateMemoryPressure calculates current memory pressure (0.0-1.0)
func (acm *AdaptiveCacheManager) calculateMemoryPressure() float64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Calculate memory pressure based on:
	// 1. Heap allocation vs heap system
	// 2. GC frequency (higher frequency = more pressure)
	// 3. Number of GCs

	heapPressure := float64(m.Alloc) / float64(m.Sys)
	if heapPressure > 1.0 {
		heapPressure = 1.0
	}

	// Factor in GC frequency (normalize to 0-1)
	gcPressure := acm.gcFrequency / 60.0 // assume 60 GCs/min is high pressure
	if gcPressure > 1.0 {
		gcPressure = 1.0
	}

	// Combined pressure (weighted average)
	pressure := heapPressure*0.7 + gcPressure*0.3
	if pressure > 1.0 {
		pressure = 1.0
	}

	return pressure
}

// GetCacheEfficiency returns cache efficiency metrics
func (acm *AdaptiveCacheManager) GetCacheEfficiency() map[string]interface{} {
	if acm == nil {
		return nil
	}

	acm.sizeMu.RLock()
	defer acm.sizeMu.RUnlock()

	return map[string]interface{}{
		"current_size":    acm.currentSize,
		"target_size":     acm.targetSize,
		"memory_pressure": acm.memoryPressure,
		"cache_hit_rate":  acm.cacheHitRate,
		"gc_frequency":    acm.gcFrequency,
		"events_per_sec":  acm.eventsPerSec,
	}
}
