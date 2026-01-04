package metrics

// UpdateWatcherQueueDepth updates the zen-watcher queue depth metric for KEDA autoscaling
// This should be called periodically (e.g., every 10 seconds) to track pending observations
func (m *Metrics) UpdateWatcherQueueDepth(depth int) {
	if m.WatcherQueueDepth != nil {
		m.WatcherQueueDepth.WithLabelValues("zen-watcher").Set(float64(depth))
	}
}

// IncrementWatcherEvents increments the zen-watcher events counter
func (m *Metrics) IncrementWatcherEvents(status string) {
	if m.WatcherEventsTotal != nil {
		m.WatcherEventsTotal.WithLabelValues("zen-watcher", status).Inc()
	}
}
