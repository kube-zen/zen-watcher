package adapters

import (
	"context"
	"errors"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/models"
)

// SecurityEventAdapter defines the interface for normalizing security events from different sources
type SecurityEventAdapter interface {
	// Normalize converts a source-specific event to a normalized SecurityEvent
	Normalize(ctx context.Context, rawEvent interface{}) (*models.SecurityEvent, error)
	// GetSource returns the adapter's source name (e.g., "falco", "trivy")
	GetSource() string
	// Validate validates that a raw event can be processed by this adapter
	Validate(rawEvent interface{}) bool
}

// EventBatcher handles batching and backpressure for security events
type EventBatcher struct {
	batchSize      int
	batchTimeout   time.Duration
	maxQueueSize   int
	eventChan      chan *models.SecurityEvent
	batchChan      chan []*models.SecurityEvent
	backpressure   bool
	droppedCount   int64
	processedCount int64
}

// BatcherConfig configures the event batcher
type BatcherConfig struct {
	BatchSize    int           // Maximum events per batch
	BatchTimeout time.Duration // Maximum time to wait before sending a batch
	MaxQueueSize int           // Maximum queue size before applying backpressure
	ChannelSize  int           // Size of internal channels
}

// DefaultBatcherConfig returns default batcher configuration
func DefaultBatcherConfig() BatcherConfig {
	return BatcherConfig{
		BatchSize:    50,
		BatchTimeout: 5 * time.Second,
		MaxQueueSize: 1000,
		ChannelSize:  100,
	}
}

// NewEventBatcher creates a new event batcher
func NewEventBatcher(config BatcherConfig) *EventBatcher {
	return &EventBatcher{
		batchSize:    config.BatchSize,
		batchTimeout: config.BatchTimeout,
		maxQueueSize: config.MaxQueueSize,
		eventChan:    make(chan *models.SecurityEvent, config.ChannelSize),
		batchChan:    make(chan []*models.SecurityEvent, config.ChannelSize),
		backpressure: false,
	}
}

// AddEvent adds an event to the batch queue with backpressure handling
func (eb *EventBatcher) AddEvent(ctx context.Context, event *models.SecurityEvent) error {
	select {
	case eb.eventChan <- event:
		eb.processedCount++
		eb.backpressure = len(eb.eventChan) >= eb.maxQueueSize
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Queue is full, drop event (backpressure)
		eb.droppedCount++
		eb.backpressure = true
		return ErrQueueFull
	}
}

// GetBatch returns a channel that receives batches of events
func (eb *EventBatcher) GetBatch() <-chan []*models.SecurityEvent {
	return eb.batchChan
}

// Start starts the batching process
func (eb *EventBatcher) Start(ctx context.Context) {
	go eb.batchLoop(ctx)
}

// batchLoop processes events and creates batches
func (eb *EventBatcher) batchLoop(ctx context.Context) {
	batch := make([]*models.SecurityEvent, 0, eb.batchSize)
	ticker := time.NewTicker(eb.batchTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Send any remaining events
			if len(batch) > 0 {
				eb.batchChan <- batch
			}
			return
		case event := <-eb.eventChan:
			batch = append(batch, event)
			if len(batch) >= eb.batchSize {
				// Send full batch
				eb.sendBatch(batch)
				batch = make([]*models.SecurityEvent, 0, eb.batchSize)
				ticker.Reset(eb.batchTimeout)
			}
		case <-ticker.C:
			// Timeout - send current batch
			if len(batch) > 0 {
				eb.sendBatch(batch)
				batch = make([]*models.SecurityEvent, 0, eb.batchSize)
			}
		}
	}
}

// sendBatch sends a batch of events
func (eb *EventBatcher) sendBatch(batch []*models.SecurityEvent) {
	select {
	case eb.batchChan <- batch:
		// Update backpressure status
		eb.backpressure = len(eb.eventChan) >= eb.maxQueueSize
	default:
		// Batch channel is full, this shouldn't happen in normal operation
		eb.droppedCount += int64(len(batch))
	}
}

// Stats returns batcher statistics
func (eb *EventBatcher) Stats() BatcherStats {
	return BatcherStats{
		ProcessedCount: eb.processedCount,
		DroppedCount:   eb.droppedCount,
		QueueSize:      len(eb.eventChan),
		Backpressure:   eb.backpressure,
	}
}

// BatcherStats contains batcher statistics
type BatcherStats struct {
	ProcessedCount int64
	DroppedCount   int64
	QueueSize      int
	Backpressure   bool
}

// Errors
var (
	ErrQueueFull = errors.New("event queue is full")
)
