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

package balancer

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/config"
)

// ReplicaInfo holds information about a replica
type ReplicaInfo struct {
	ID           string
	Load         float64
	Healthy      bool
	LastSeen     time.Time
	CPUUsage     float64
	MemoryUsage  float64
	EventsPerSec float64
}

// LoadBalancer manages load balancing across replicas
type LoadBalancer struct {
	strategy           string
	replicas           map[string]*ReplicaInfo
	replicasMu         sync.RWMutex
	currentIndex       int
	indexMu            sync.Mutex
	rebalanceThreshold float64
}

// NewLoadBalancer creates a new load balancer
func NewLoadBalancer(haConfig *config.LoadBalancingConfig) *LoadBalancer {
	if haConfig == nil {
		return nil
	}

	strategy := haConfig.Strategy
	if strategy == "" {
		strategy = "least_loaded"
	}

	return &LoadBalancer{
		strategy:           strategy,
		replicas:           make(map[string]*ReplicaInfo),
		rebalanceThreshold: haConfig.RebalanceThreshold,
	}
}

// UpdateReplica updates information about a replica
func (lb *LoadBalancer) UpdateReplica(replicaID string, load, cpuUsage, memoryUsage, eventsPerSec float64, healthy bool) {
	if lb == nil {
		return
	}

	lb.replicasMu.Lock()
	defer lb.replicasMu.Unlock()

	if lb.replicas[replicaID] == nil {
		lb.replicas[replicaID] = &ReplicaInfo{
			ID: replicaID,
		}
	}

	replica := lb.replicas[replicaID]
	replica.Load = load
	replica.CPUUsage = cpuUsage
	replica.MemoryUsage = memoryUsage
	replica.EventsPerSec = eventsPerSec
	replica.Healthy = healthy
	replica.LastSeen = time.Now()
}

// RemoveReplica removes a replica from the load balancer
func (lb *LoadBalancer) RemoveReplica(replicaID string) {
	if lb == nil {
		return
	}

	lb.replicasMu.Lock()
	defer lb.replicasMu.Unlock()
	delete(lb.replicas, replicaID)
}

// SelectReplica selects a replica based on the load balancing strategy
func (lb *LoadBalancer) SelectReplica(eventKey string) (string, error) {
	if lb == nil {
		return "", fmt.Errorf("load balancer not initialized")
	}

	lb.replicasMu.RLock()
	defer lb.replicasMu.RUnlock()

	// Filter healthy replicas
	healthyReplicas := make([]*ReplicaInfo, 0)
	for _, replica := range lb.replicas {
		if replica.Healthy && time.Since(replica.LastSeen) < 2*time.Minute {
			healthyReplicas = append(healthyReplicas, replica)
		}
	}

	if len(healthyReplicas) == 0 {
		return "", fmt.Errorf("no healthy replicas available")
	}

	switch lb.strategy {
	case "round_robin":
		return lb.selectRoundRobin(healthyReplicas)
	case "least_loaded":
		return lb.selectLeastLoaded(healthyReplicas)
	case "consistent_hash":
		return lb.selectConsistentHash(healthyReplicas, eventKey)
	default:
		return lb.selectLeastLoaded(healthyReplicas)
	}
}

// selectRoundRobin selects the next replica in rotation
func (lb *LoadBalancer) selectRoundRobin(replicas []*ReplicaInfo) (string, error) {
	lb.indexMu.Lock()
	defer lb.indexMu.Unlock()

	if len(replicas) == 0 {
		return "", fmt.Errorf("no replicas available")
	}

	selected := replicas[lb.currentIndex%len(replicas)]
	lb.currentIndex++
	return selected.ID, nil
}

// selectLeastLoaded selects the replica with the lowest load
func (lb *LoadBalancer) selectLeastLoaded(replicas []*ReplicaInfo) (string, error) {
	if len(replicas) == 0 {
		return "", fmt.Errorf("no replicas available")
	}

	leastLoaded := replicas[0]
	for _, replica := range replicas[1:] {
		if replica.Load < leastLoaded.Load {
			leastLoaded = replica
		}
	}
	return leastLoaded.ID, nil
}

// selectConsistentHash selects a replica using consistent hashing
func (lb *LoadBalancer) selectConsistentHash(replicas []*ReplicaInfo, eventKey string) (string, error) {
	if len(replicas) == 0 {
		return "", fmt.Errorf("no replicas available")
	}

	// Simple consistent hash: hash the event key and map to replica
	hash := sha256.Sum256([]byte(eventKey))
	hashValue := int(hash[0]) + int(hash[1])<<8 + int(hash[2])<<16 + int(hash[3])<<24
	if hashValue < 0 {
		hashValue = -hashValue
	}

	selected := replicas[hashValue%len(replicas)]
	return selected.ID, nil
}

// ShouldRebalance checks if rebalancing is needed
func (lb *LoadBalancer) ShouldRebalance() bool {
	if lb == nil {
		return false
	}

	lb.replicasMu.RLock()
	defer lb.replicasMu.RUnlock()

	if len(lb.replicas) < 2 {
		return false
	}

	// Find min and max load
	minLoad, maxLoad := 1.0, 0.0
	for _, replica := range lb.replicas {
		if replica.Healthy {
			if replica.Load < minLoad {
				minLoad = replica.Load
			}
			if replica.Load > maxLoad {
				maxLoad = replica.Load
			}
		}
	}

	// Rebalance if load difference exceeds threshold
	if minLoad > 0 && (maxLoad/minLoad) > lb.rebalanceThreshold {
		return true
	}

	return false
}

// GetReplicaStats returns statistics about all replicas
func (lb *LoadBalancer) GetReplicaStats() map[string]interface{} {
	if lb == nil {
		return nil
	}

	lb.replicasMu.RLock()
	defer lb.replicasMu.RUnlock()

	stats := make(map[string]interface{})
	stats["strategy"] = lb.strategy
	stats["total_replicas"] = len(lb.replicas)

	healthyCount := 0
	totalLoad := 0.0
	for _, replica := range lb.replicas {
		if replica.Healthy {
			healthyCount++
			totalLoad += replica.Load
		}
	}

	stats["healthy_replicas"] = healthyCount
	if healthyCount > 0 {
		stats["average_load"] = totalLoad / float64(healthyCount)
	}

	return stats
}
