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

package metrics

import (
	"context"
	"runtime"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

// SystemMetrics tracks system resource usage for HA coordination
type SystemMetrics struct {
	lastCPUStats  *cpu.TimesStat
	lastCheckTime time.Time
	eventCount    int64
	lastEventTime time.Time
	mu            sync.RWMutex
	queueDepth    int
	eventCtx      context.Context
	eventCancel   context.CancelFunc
}

// NewSystemMetrics creates a new system metrics tracker
func NewSystemMetrics() *SystemMetrics {
	ctx, cancel := context.WithCancel(context.Background())
	return &SystemMetrics{
		lastCheckTime: time.Now(),
		lastEventTime: time.Now(),
		eventCtx:      ctx,
		eventCancel:   cancel,
	}
}

// Close cleans up resources
func (sm *SystemMetrics) Close() {
	if sm.eventCancel != nil {
		sm.eventCancel()
	}
}

// GetCPUUsagePercent returns current CPU usage percentage
func (sm *SystemMetrics) GetCPUUsagePercent() float64 {
	now := time.Now()

	// Get current CPU stats
	current, err := cpu.Times(false)
	if err != nil || len(current) == 0 {
		// Fallback: try to get per-process CPU
		percentages, err := cpu.Percent(1*time.Second, false)
		if err == nil && len(percentages) > 0 {
			return percentages[0]
		}
		return 0.0
	}

	// Calculate usage since last check
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.lastCPUStats != nil {
		timeDiff := now.Sub(sm.lastCheckTime).Seconds()
		if timeDiff > 0 {
			totalBefore := sm.lastCPUStats.User + sm.lastCPUStats.System + sm.lastCPUStats.Idle
			totalCurrent := current[0].User + current[0].System + current[0].Idle

			totalDiff := totalCurrent - totalBefore
			idleDiff := current[0].Idle - sm.lastCPUStats.Idle

			if totalDiff > 0 {
				usage := (1 - idleDiff/totalDiff) * 100
				if usage < 0 {
					usage = 0
				}
				if usage > 100 {
					usage = 100
				}
				sm.lastCPUStats = &current[0]
				sm.lastCheckTime = now
				return usage
			}
		}
	}

	// First time or error - initialize
	sm.lastCPUStats = &current[0]
	sm.lastCheckTime = now
	return 0.0
}

// RecordEvent tracks an event occurrence for rate calculation
func (sm *SystemMetrics) RecordEvent() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.eventCount++
	sm.lastEventTime = time.Now()
}

// GetEventsPerSecond returns current events per second rate
func (sm *SystemMetrics) GetEventsPerSecond() float64 {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	timeDiff := now.Sub(sm.lastEventTime).Seconds()

	// If less than 1 second has passed, use a sliding window approach
	if timeDiff < 1.0 {
		// Use a 5-second window for more stable calculation
		windowStart := now.Add(-5 * time.Second)
		if sm.lastEventTime.After(windowStart) {
			// Recent events, calculate rate
			return float64(sm.eventCount) / 5.0
		}
	}

	// Reset if too much time has passed
	if timeDiff > 60.0 {
		sm.eventCount = 0
		sm.lastEventTime = now
		return 0.0
	}

	if timeDiff > 0 && sm.eventCount > 0 {
		rate := float64(sm.eventCount) / timeDiff
		// Reset counter after calculation
		sm.eventCount = 0
		sm.lastEventTime = now
		return rate
	}

	return 0.0
}

// SetQueueDepth sets the current queue depth
func (sm *SystemMetrics) SetQueueDepth(depth int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if depth < 0 {
		depth = 0
	}
	sm.queueDepth = depth
}

// GetQueueDepth returns current queue depth
func (sm *SystemMetrics) GetQueueDepth() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.queueDepth
}

// GetMemoryUsage returns current memory usage in bytes
func (sm *SystemMetrics) GetMemoryUsage() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Alloc
}

// GetMemoryUsagePercent returns memory usage as percentage of system memory
func (sm *SystemMetrics) GetMemoryUsagePercent() float64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Get system memory info
	vmStat, err := mem.VirtualMemory()
	if err != nil {
		// Fallback: use Go runtime stats only
		return 0.0
	}

	if vmStat.Total == 0 {
		return 0.0
	}

	// Calculate percentage of system memory used by this process
	percent := (float64(m.Alloc) / float64(vmStat.Total)) * 100
	if percent > 100 {
		percent = 100
	}
	return percent
}

// GetResponseTime calculates average response time (placeholder for future implementation)
func (sm *SystemMetrics) GetResponseTime() float64 {
	// TODO: Implement actual response time tracking
	// For now, return 0.0 as placeholder
	return 0.0
}
