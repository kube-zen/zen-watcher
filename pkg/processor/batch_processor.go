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

package processor

import (
	"context"
	"sync"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/adapter/generic"
	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
)

// EventBatch represents a batch of events from the same source
type EventBatch struct {
	Source    string
	Events    []*generic.RawEvent
	Timestamp time.Time
	Size      int
	MaxSize   int
	MaxAge    time.Duration
	mu        sync.Mutex
}

// NewEventBatch creates a new event batch
func NewEventBatch(source string, maxSize int, maxAge time.Duration) *EventBatch {
	return &EventBatch{
		Source:    source,
		Events:    make([]*generic.RawEvent, 0, maxSize),
		Timestamp: time.Now(),
		MaxSize:   maxSize,
		MaxAge:    maxAge,
	}
}

// AddEvent adds an event to the batch
func (b *EventBatch) AddEvent(event *generic.RawEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.Events = append(b.Events, event)
	b.Size = len(b.Events)
}

// IsReadyForProcessing returns true if the batch is ready to be processed
func (b *EventBatch) IsReadyForProcessing() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.Size >= b.MaxSize || time.Since(b.Timestamp) >= b.MaxAge
}

// IsEmpty returns true if the batch has no events
func (b *EventBatch) IsEmpty() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.Events) == 0
}

// TakeEvents removes and returns all events from the batch
func (b *EventBatch) TakeEvents() []*generic.RawEvent {
	b.mu.Lock()
	defer b.mu.Unlock()
	events := b.Events
	b.Events = make([]*generic.RawEvent, 0, b.MaxSize)
	b.Size = 0
	b.Timestamp = time.Now()
	return events
}

// BatchProcessor processes events in batches for improved throughput
type BatchProcessor struct {
	processor    *Processor
	batches      map[string]*EventBatch
	mu           sync.RWMutex
	maxBatchSize int
	maxBatchAge  time.Duration
	batchTicker  *time.Ticker
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

// NewBatchProcessor creates a new batch processor
func NewBatchProcessor(
	processor *Processor,
	maxBatchSize int,
	maxBatchAge time.Duration,
) *BatchProcessor {
	ctx, cancel := context.WithCancel(context.Background())

	bp := &BatchProcessor{
		processor:    processor,
		batches:      make(map[string]*EventBatch),
		maxBatchSize: maxBatchSize,
		maxBatchAge:  maxBatchAge,
		batchTicker:  time.NewTicker(100 * time.Millisecond), // Process every 100ms
		ctx:          ctx,
		cancel:       cancel,
	}

	// Start batch processing goroutine
	bp.wg.Add(1)
	go bp.processBatches()

	return bp
}

// AddEvent adds an event to the appropriate batch
func (bp *BatchProcessor) AddEvent(ctx context.Context, raw *generic.RawEvent, config *generic.SourceConfig) error {
	bp.mu.Lock()

	source := raw.Source
	batch, exists := bp.batches[source]
	if !exists {
		batch = NewEventBatch(source, bp.maxBatchSize, bp.maxBatchAge)
		bp.batches[source] = batch
	}

	batch.AddEvent(raw)
	ready := batch.IsReadyForProcessing()

	bp.mu.Unlock()

	// Process immediately if batch is full (maintains low latency for high-rate sources)
	if ready {
		return bp.processBatch(ctx, source, batch)
	}

	return nil
}

// processBatches periodically processes ready batches
func (bp *BatchProcessor) processBatches() {
	defer bp.wg.Done()

	for {
		select {
		case <-bp.ctx.Done():
			// Process remaining batches on shutdown
			bp.processAllBatches()
			return
		case <-bp.batchTicker.C:
			bp.processAllBatches()
		}
	}
}

// processAllBatches processes all ready batches
func (bp *BatchProcessor) processAllBatches() {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	ctx := context.Background()

	for source, batch := range bp.batches {
		if batch.IsReadyForProcessing() && !batch.IsEmpty() {
			// Process batch in a goroutine to avoid blocking
			go func(s string, b *EventBatch) {
				_ = bp.processBatch(ctx, s, b)
			}(source, batch)
		}
	}
}

// processBatch processes a single batch of events
func (bp *BatchProcessor) processBatch(ctx context.Context, source string, batch *EventBatch) error {
	events := batch.TakeEvents()
	if len(events) == 0 {
		return nil
	}

	// Get config for the source
	// Note: In practice, config should be retrieved per source from SourceConfigLoader
	// For now, we pass nil and let the processor handle config lookup
	var config *generic.SourceConfig

	// Process each event in the batch
	// Note: We still process individually to maintain filter/dedup semantics
	// but batching reduces channel overhead and allows better scheduling
	for _, event := range events {
		if err := bp.processor.ProcessEvent(ctx, event, config); err != nil {
			logger := sdklog.NewLogger("zen-watcher-processor")
			logger.Warn("Batch event processing failed",
				sdklog.Operation("batch_event_process"),
				sdklog.String("source", source),
				sdklog.Error(err))
			// Continue processing other events in batch
		}
	}

	logger := sdklog.NewLogger("zen-watcher-processor")
	logger.Debug("Processed event batch",
		sdklog.Operation("batch_processed"),
		sdklog.String("source", source),
		sdklog.Int("batch_size", len(events)))

	return nil
}

// Stop stops the batch processor and processes remaining batches
func (bp *BatchProcessor) Stop() {
	bp.cancel()
	bp.batchTicker.Stop()
	bp.wg.Wait()

	// Process any remaining batches
	bp.processAllBatches()
}
