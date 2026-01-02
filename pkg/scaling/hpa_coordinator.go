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

package scaling

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/config"
	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
	"github.com/kube-zen/zen-watcher/pkg/metrics"
)

// HPACoordinator manages auto-scaling decisions and coordination
type HPACoordinator struct {
	haConfig      *config.AutoScalingConfig
	haMetrics     *metrics.HAMetrics
	replicaID     string
	currentCPU    float64
	currentMemory float64
	eventsPerSec  float64
	queueDepth    int
	responseTime  float64
	metricsMu     sync.RWMutex
	stopChan      chan struct{}
	wg            sync.WaitGroup
}

// ScalingDecision represents a scaling decision
type ScalingDecision struct {
	Action       string // "scale_up", "scale_down", "no_action"
	Reason       string
	CurrentCPU   float64
	TargetCPU    int
	QueueDepth   int
	EventsPerSec float64
}

// NewHPACoordinator creates a new HPA coordinator
func NewHPACoordinator(haConfig *config.AutoScalingConfig, haMetrics *metrics.HAMetrics, replicaID string) *HPACoordinator {
	if haConfig == nil || !haConfig.Enabled {
		return nil
	}

	return &HPACoordinator{
		haConfig:  haConfig,
		haMetrics: haMetrics,
		replicaID: replicaID,
		stopChan:  make(chan struct{}),
	}
}

// Start begins the scaling decision loop
func (c *HPACoordinator) Start(ctx context.Context, interval time.Duration) {
	if c == nil {
		return
	}

	c.wg.Add(1)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	go func() {
		defer c.wg.Done()
		for {
			select {
			case <-ticker.C:
				decision := c.EvaluateScaling()
				if decision.Action != "no_action" {
					c.ExecuteScalingDecision(decision)
				}
			case <-ctx.Done():
				return
			case <-c.stopChan:
				return
			}
		}
	}()
}

// Stop stops the scaling coordinator
func (c *HPACoordinator) Stop() {
	if c == nil {
		return
	}
	close(c.stopChan)
	c.wg.Wait()
}

// UpdateMetrics updates current metrics for scaling decisions
func (c *HPACoordinator) UpdateMetrics(cpuUsage, memoryUsage, eventsPerSec float64, queueDepth int, responseTime float64) {
	if c == nil {
		return
	}

	c.metricsMu.Lock()
	defer c.metricsMu.Unlock()

	c.currentCPU = cpuUsage
	c.currentMemory = memoryUsage
	c.eventsPerSec = eventsPerSec
	c.queueDepth = queueDepth
	c.responseTime = responseTime

	// Update Prometheus metrics
	if c.haMetrics != nil {
		c.haMetrics.CPUUsagePerReplica.WithLabelValues(c.replicaID).Set(cpuUsage)
		c.haMetrics.MemoryUsagePerReplica.WithLabelValues(c.replicaID).Set(memoryUsage)
		c.haMetrics.EventsPerSecond.Set(eventsPerSec)
		c.haMetrics.QueueDepth.Set(float64(queueDepth))
		c.haMetrics.ReplicaLoad.WithLabelValues(c.replicaID).Set(c.calculateLoad())
	}
}

// EvaluateScaling evaluates whether scaling is needed
func (c *HPACoordinator) EvaluateScaling() *ScalingDecision {
	if c == nil {
		return &ScalingDecision{Action: "no_action", Reason: "coordinator_not_initialized"}
	}

	c.metricsMu.RLock()
	cpu := c.currentCPU
	queueDepth := c.queueDepth
	eventsPerSec := c.eventsPerSec
	responseTime := c.responseTime
	c.metricsMu.RUnlock()

	decision := &ScalingDecision{
		CurrentCPU:   cpu,
		TargetCPU:    c.haConfig.TargetCPU,
		QueueDepth:   queueDepth,
		EventsPerSec: eventsPerSec,
	}

	// Scale Up Triggers
	if cpu > float64(c.haConfig.TargetCPU) {
		decision.Action = "scale_up"
		decision.Reason = fmt.Sprintf("CPU usage %.1f%% exceeds target %d%%", cpu, c.haConfig.TargetCPU)
		return decision
	}

	if queueDepth > 1000 {
		decision.Action = "scale_up"
		decision.Reason = fmt.Sprintf("Queue depth %d exceeds threshold 1000", queueDepth)
		return decision
	}

	if responseTime > 5.0 {
		decision.Action = "scale_up"
		decision.Reason = fmt.Sprintf("Response time %.2fs exceeds SLA 5s", responseTime)
		return decision
	}

	// Scale Down Triggers (only if CPU is well below target for extended period)
	if cpu < float64(c.haConfig.TargetCPU-20) && queueDepth < 100 {
		decision.Action = "scale_down"
		decision.Reason = fmt.Sprintf("CPU usage %.1f%% well below target %d%% and low queue depth", cpu, c.haConfig.TargetCPU)
		return decision
	}

	decision.Action = "no_action"
	decision.Reason = "metrics within acceptable range"
	return decision
}

// ExecuteScalingDecision executes a scaling decision
func (c *HPACoordinator) ExecuteScalingDecision(decision *ScalingDecision) {
	if c == nil || decision == nil {
		return
	}

	// Log the scaling decision
	logger := sdklog.NewLogger("zen-watcher-scaling")
	logger.Info("Scaling decision made",
		sdklog.Operation("scaling_decision"),
		sdklog.String("action", decision.Action),
		sdklog.String("reason", decision.Reason),
		sdklog.Float64("current_cpu", decision.CurrentCPU),
		sdklog.Int("target_cpu", decision.TargetCPU),
		sdklog.Int("queue_depth", decision.QueueDepth),
		sdklog.Float64("events_per_sec", decision.EventsPerSec))

	// Record in metrics
	if c.haMetrics != nil {
		c.haMetrics.ScalingDecisions.WithLabelValues(decision.Action, decision.Reason).Inc()
	}

	// Note: Actual Kubernetes HPA API calls would be implemented here
	// For now, we log the decision and update metrics
	// In production, this would call the Kubernetes API to update HPA or Deployment
}

// calculateLoad calculates the current load factor (0.0-1.0)
func (c *HPACoordinator) calculateLoad() float64 {
	c.metricsMu.RLock()
	defer c.metricsMu.RUnlock()

	// Load is a combination of CPU usage, queue depth, and response time
	cpuFactor := c.currentCPU / 100.0
	queueFactor := float64(c.queueDepth) / 10000.0 // normalize to 0-1
	responseFactor := c.responseTime / 10.0        // normalize to 0-1

	// Weighted average
	load := (cpuFactor*0.5 + queueFactor*0.3 + responseFactor*0.2)
	if load > 1.0 {
		load = 1.0
	}
	return load
}
