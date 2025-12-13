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

package dispatcher

import (
	"context"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/logger"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// DestinationBatcher batches observations for high-volume destinations
// This improves throughput by reducing API call overhead
type DestinationBatcher struct {
	batches      map[string]*ObservationBatch // destination key -> batch
	mu           sync.RWMutex
	maxBatchSize int
	maxBatchAge  time.Duration
	batchTicker  *time.Ticker
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	flushFunc    func(ctx context.Context, destinationKey string, observations []*unstructured.Unstructured) error
	enabled      bool
}

// ObservationBatch represents a batch of observations for a destination
type ObservationBatch struct {
	DestinationKey string
	Observations   []*unstructured.Unstructured
	Timestamp      time.Time
	mu             sync.Mutex
}

// NewDestinationBatcher creates a new destination batcher
func NewDestinationBatcher(
	maxBatchSize int,
	maxBatchAge time.Duration,
	flushFunc func(ctx context.Context, destinationKey string, observations []*unstructured.Unstructured) error,
) *DestinationBatcher {
	enabled := getEnvBool("EVENT_BATCHING_ENABLED", false)
	if !enabled {
		return &DestinationBatcher{enabled: false}
	}

	if maxBatchSize <= 0 {
		maxBatchSize = getEnvInt("EVENT_BATCH_SIZE", 100)
	}
	if maxBatchAge <= 0 {
		maxBatchAge = getEnvDuration("EVENT_BATCH_AGE", 5*time.Second)
	}

	ctx, cancel := context.WithCancel(context.Background())

	db := &DestinationBatcher{
		batches:      make(map[string]*ObservationBatch),
		maxBatchSize: maxBatchSize,
		maxBatchAge:  maxBatchAge,
		batchTicker:  time.NewTicker(100 * time.Millisecond), // Check every 100ms
		ctx:          ctx,
		cancel:       cancel,
		flushFunc:    flushFunc,
		enabled:      true,
	}

	// Start batch processing goroutine
	db.wg.Add(1)
	go db.processBatches()

	return db
}

// Enqueue adds an observation to the appropriate batch
func (db *DestinationBatcher) Enqueue(ctx context.Context, destinationKey string, observation *unstructured.Unstructured) error {
	if !db.enabled {
		// If batching is disabled, flush immediately
		return db.flushFunc(ctx, destinationKey, []*unstructured.Unstructured{observation})
	}

	db.mu.Lock()
	batch, exists := db.batches[destinationKey]
	if !exists {
		batch = &ObservationBatch{
			DestinationKey: destinationKey,
			Observations:   make([]*unstructured.Unstructured, 0, db.maxBatchSize),
			Timestamp:      time.Now(),
		}
		db.batches[destinationKey] = batch
	}
	db.mu.Unlock()

	batch.mu.Lock()
	batch.Observations = append(batch.Observations, observation)
	ready := len(batch.Observations) >= db.maxBatchSize || time.Since(batch.Timestamp) >= db.maxBatchAge
	batch.mu.Unlock()

	// Flush immediately if batch is full (maintains low latency for high-rate destinations)
	if ready {
		return db.flushBatch(ctx, destinationKey, batch)
	}

	return nil
}

// processBatches periodically processes ready batches
func (db *DestinationBatcher) processBatches() {
	defer db.wg.Done()

	for {
		select {
		case <-db.ctx.Done():
			// Flush remaining batches on shutdown
			db.flushAllBatches()
			return
		case <-db.batchTicker.C:
			db.flushAllBatches()
		}
	}
}

// flushAllBatches processes all ready batches
func (db *DestinationBatcher) flushAllBatches() {
	db.mu.Lock()
	defer db.mu.Unlock()

	ctx := context.Background()

	for destinationKey, batch := range db.batches {
		batch.mu.Lock()
		ready := len(batch.Observations) >= db.maxBatchSize || time.Since(batch.Timestamp) >= db.maxBatchAge
		hasObservations := len(batch.Observations) > 0
		batch.mu.Unlock()

		if ready && hasObservations {
			// Flush batch in a goroutine to avoid blocking
			go func(key string, b *ObservationBatch) {
				_ = db.flushBatch(ctx, key, b)
			}(destinationKey, batch)
		}
	}
}

// flushBatch processes a single batch of observations
func (db *DestinationBatcher) flushBatch(ctx context.Context, destinationKey string, batch *ObservationBatch) error {
	batch.mu.Lock()
	observations := make([]*unstructured.Unstructured, len(batch.Observations))
	copy(observations, batch.Observations)
	batch.Observations = batch.Observations[:0] // Clear batch
	batch.Timestamp = time.Now()
	batch.mu.Unlock()

	if len(observations) == 0 {
		return nil
	}

	// Call flush function
	if err := db.flushFunc(ctx, destinationKey, observations); err != nil {
		logger.Warn("Failed to flush observation batch",
			logger.Fields{
				Component: "dispatcher",
				Operation: "batch_flush",
				Error:     err,
				Additional: map[string]interface{}{
					"destination": destinationKey,
					"batch_size":  len(observations),
				},
			})
		return err
	}

	logger.Debug("Flushed observation batch",
		logger.Fields{
			Component: "dispatcher",
			Operation: "batch_flushed",
			Additional: map[string]interface{}{
				"destination": destinationKey,
				"batch_size":  len(observations),
			},
		})

	return nil
}

// Stop stops the batcher and processes remaining batches
func (db *DestinationBatcher) Stop() {
	if !db.enabled {
		return
	}

	db.cancel()
	db.batchTicker.Stop()
	db.wg.Wait()

	// Flush any remaining batches
	db.flushAllBatches()
}

// Helper functions for environment variable parsing
// Note: getEnvInt is defined in worker_pool.go to avoid duplication
func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}
