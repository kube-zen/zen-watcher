# Bug Report: Goroutine Leak in InformerAdapter

## Severity: HIGH

## Description

The `InformerAdapter` creates an event channel in `Start()` but never closes it in `Stop()`. This causes a goroutine leak in the orchestrator's `processEvents` function, which blocks forever waiting for events on a channel that will never be closed.

## Location

- **File**: `pkg/adapter/generic/informer_adapter.go`
- **Function**: `Start()` creates channel at line 71
- **Function**: `Stop()` does NOT close the channel (line 230-240)

## Root Cause

1. `InformerAdapter.Start()` creates a local channel: `events := make(chan RawEvent, 100)` (line 71)
2. This channel is returned to the orchestrator
3. The orchestrator starts a goroutine: `go o.processEvents(source, genericConfig, events)` (line 354 in `generic.go`)
4. `processEvents` loops on `for rawEvent := range events` (lines 214, 231)
5. When adapter is stopped, `InformerAdapter.Stop()` only closes `a.stopCh`, but NOT the `events` channel
6. The `processEvents` goroutine blocks forever waiting for events, causing a leak

## Impact

- Goroutine leak for each stopped InformerAdapter
- Memory leak (goroutines accumulate)
- Potential resource exhaustion over time

## Comparison with Other Adapters

- ✅ **LogsAdapter**: Correctly closes `a.events` channel in `Stop()` (line 273)
- ❌ **InformerAdapter**: Does NOT close the events channel
- ⚠️ **WebhookAdapter**: Uses shared `a.events` channel - need to verify cleanup

## Fix

The `InformerAdapter` needs to:
1. Store a reference to the events channel in the struct
2. Close the events channel in `Stop()` method

## Suggested Fix

```go
type InformerAdapter struct {
	manager *informers.Manager
	stopCh  chan struct{}
	events  chan RawEvent  // ADD: Store channel reference
	queue   workqueue.TypedRateLimitingInterface[*RawEvent]
	mu      sync.Mutex
}

func (a *InformerAdapter) Start(ctx context.Context, config *SourceConfig) (<-chan RawEvent, error) {
	// ...
	events := make(chan RawEvent, 100)
	a.mu.Lock()
	a.events = events  // STORE: Keep reference
	a.mu.Unlock()
	// ...
	return events, nil
}

func (a *InformerAdapter) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Close stopCh
	select {
	case <-a.stopCh:
		// Already closed
	default:
		close(a.stopCh)
	}

	// FIX: Close events channel
	if a.events != nil {
		close(a.events)
		a.events = nil
	}
}
```

