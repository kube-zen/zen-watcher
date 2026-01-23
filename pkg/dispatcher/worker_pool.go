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
	"fmt"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Package-level logger to avoid repeated allocations
var dispatcherLogger = sdklog.NewLogger("zen-watcher-dispatcher")

var (
	// WorkerPoolQueueDepth tracks the current depth of the work queue
	WorkerPoolQueueDepth = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "zen_watcher_worker_pool_queue_depth",
			Help: "Current number of items in the worker pool queue",
		},
	)

	// WorkerPoolWorkersActive tracks the number of active workers
	WorkerPoolWorkersActive = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "zen_watcher_worker_pool_workers_active",
			Help: "Current number of active workers processing items",
		},
	)

	// WorkerPoolWorkProcessed tracks the total number of work items processed
	WorkerPoolWorkProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zen_watcher_worker_pool_work_processed_total",
			Help: "Total number of work items processed by the worker pool",
		},
		[]string{"status"}, // success, error
	)

	// WorkerPoolWorkDuration tracks the duration of work processing
	WorkerPoolWorkDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "zen_watcher_worker_pool_work_duration_seconds",
			Help:    "Duration of work item processing in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"status"},
	)
)

// WorkItem represents a unit of work for the worker pool
// This interface allows any type to be processed by the worker pool
type WorkItem interface {
	Process(ctx context.Context) error
}

// WorkerPoolConfig holds configuration for the worker pool
type WorkerPoolConfig struct {
	Enabled   bool
	Size      int
	QueueSize int
}

// WorkerPool manages a pool of concurrent workers for processing work items
type WorkerPool struct {
	mu            sync.RWMutex
	config        WorkerPoolConfig
	workers       int
	workQueue     chan WorkItem
	maxQueueSize  int
	wg            sync.WaitGroup
	ctx           context.Context
	cancel        context.CancelFunc
	activeWorkers int64          // Atomic counter for active workers
	stopOnce      sync.Once      // Protects against double close
	drainWg       sync.WaitGroup // Tracks drain goroutines
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(workerCount int, maxQueueSize int) *WorkerPool {
	if workerCount <= 0 {
		workerCount = getEnvInt("WORKER_POOL_SIZE", 5)
	}
	if maxQueueSize <= 0 {
		maxQueueSize = workerCount * 2 // Default: 2x worker count
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &WorkerPool{
		config: WorkerPoolConfig{
			Enabled:   true,
			Size:      workerCount,
			QueueSize: maxQueueSize,
		},
		workers:      workerCount,
		workQueue:    make(chan WorkItem, maxQueueSize),
		maxQueueSize: maxQueueSize,
		ctx:          ctx,
		cancel:       cancel,
	}
}

// UpdateConfig updates the worker pool configuration
func (wp *WorkerPool) UpdateConfig(newConfig WorkerPoolConfig) {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	// If disabled, stop the pool
	if !newConfig.Enabled && wp.config.Enabled {
		wp.Stop()
		wp.config = newConfig
		return
	}

	// If enabled and wasn't before, start it
	if newConfig.Enabled && !wp.config.Enabled {
		wp.config = newConfig
		wp.workers = newConfig.Size
		wp.maxQueueSize = newConfig.QueueSize
		wp.workQueue = make(chan WorkItem, newConfig.QueueSize)
		wp.ctx, wp.cancel = context.WithCancel(context.Background())
		wp.Start()
		return
	}

	// Update size if changed
	if newConfig.Size != wp.workers {
		// Note: Dynamic resizing would require more complex logic
		// For now, we log a warning and keep existing workers
		dispatcherLogger.Warn("Worker pool size change requires restart",
			sdklog.Operation("worker_pool_update"),
			sdklog.ErrorCode("CONFIG_WARNING"),
			sdklog.Int("old_size", wp.workers),
			sdklog.Int("new_size", newConfig.Size))
	}

	// Update queue size if changed (requires recreation)
	if newConfig.QueueSize != wp.maxQueueSize {
		oldQueue := wp.workQueue
		wp.workQueue = make(chan WorkItem, newConfig.QueueSize)
		wp.maxQueueSize = newConfig.QueueSize

		// Drain old queue into new queue (non-blocking)
		wp.drainWg.Add(1)
		go func() {
			defer wp.drainWg.Done()
			for {
				select {
				case item := <-oldQueue:
					select {
					case wp.workQueue <- item:
					default:
						dispatcherLogger.Warn("Dropped work item during queue resize",
							sdklog.Operation("queue_resize"),
							sdklog.ErrorCode("QUEUE_FULL"))
					}
				default:
					return
				}
			}
		}()
	}

	wp.config = newConfig
}

// Start starts all workers in the pool
func (wp *WorkerPool) Start() {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	if !wp.config.Enabled {
		return
	}

	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}
}

// IsRunning returns true if the worker pool is running
func (wp *WorkerPool) IsRunning() bool {
	wp.mu.RLock()
	defer wp.mu.RUnlock()
	return wp.config.Enabled && wp.workers > 0
}

// Stop gracefully stops all workers
func (wp *WorkerPool) Stop() {
	wp.stopOnce.Do(func() {
		close(wp.workQueue)
		wp.cancel()
		wp.wg.Wait()
		wp.drainWg.Wait() // Wait for any drain goroutines to complete
	})
}

// Enqueue adds a work item to the queue
// Returns error if queue is full (non-blocking)
func (wp *WorkerPool) Enqueue(work WorkItem) error {
	select {
	case wp.workQueue <- work:
		WorkerPoolQueueDepth.Inc()
		return nil
	default:
		return fmt.Errorf("work queue full (max: %d)", wp.maxQueueSize)
	}
}

// EnqueueBlocking adds a work item to the queue, blocking if queue is full
// Accepts interface{} for compatibility with WorkerPoolInterface
func (wp *WorkerPool) EnqueueBlocking(ctx context.Context, work interface{}) error {
	workItem, ok := work.(WorkItem)
	if !ok {
		return fmt.Errorf("work item does not implement WorkItem interface")
	}
	select {
	case wp.workQueue <- workItem:
		WorkerPoolQueueDepth.Inc()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// QueueSize returns the current queue size
func (wp *WorkerPool) QueueSize() int {
	return len(wp.workQueue)
}

// ActiveWorkers returns the number of currently active workers
func (wp *WorkerPool) ActiveWorkers() int {
	return int(atomic.LoadInt64(&wp.activeWorkers))
}

// worker is the main worker loop
func (wp *WorkerPool) worker(id int) {
	defer wp.wg.Done()

	for {
		select {
		case work, ok := <-wp.workQueue:
			if !ok {
				// Queue closed, exit
				return
			}

			WorkerPoolQueueDepth.Dec()
			atomic.AddInt64(&wp.activeWorkers, 1)
			WorkerPoolWorkersActive.Inc()

			// Process the work
			startTime := time.Now()
			err := wp.processWork(work)

			// Record metrics
			duration := time.Since(startTime).Seconds()
			status := "success"
			if err != nil {
				status = "error"
			}
			WorkerPoolWorkProcessed.WithLabelValues(status).Inc()
			WorkerPoolWorkDuration.WithLabelValues(status).Observe(duration)

			atomic.AddInt64(&wp.activeWorkers, -1)
			WorkerPoolWorkersActive.Dec()

			// Handle errors
			if err != nil {
				dispatcherLogger.WithContext(wp.ctx).Warn("Work item processing failed",
					sdklog.Operation("worker_process"),
					sdklog.ErrorCode("WORK_ITEM_ERROR"),
					sdklog.Error(err),
					sdklog.Int("worker_id", id))
			}

		case <-wp.ctx.Done():
			return
		}
	}
}

// processWork processes a single work item
func (wp *WorkerPool) processWork(work WorkItem) error {
	if work == nil {
		return fmt.Errorf("work item is nil")
	}

	// Create context with timeout for this work item
	ctx, cancel := context.WithTimeout(wp.ctx, 5*time.Minute)
	defer cancel()

	return work.Process(ctx)
}

// getEnvInt gets an integer environment variable or returns default
// Note: This is a local helper, not exported to avoid conflicts
func getEnvInt(key string, defaultValue int) int {
	if val := os.Getenv(key); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return defaultValue
}
