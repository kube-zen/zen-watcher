package metrics

// UpdateIngesterQueueDepth updates the ingester queue depth metric for KEDA autoscaling
// This should be called periodically (e.g., every 10 seconds) to track pending observations
func (m *Metrics) UpdateIngesterQueueDepth(depth int) {
	if m.IngesterQueueDepth != nil {
		m.IngesterQueueDepth.WithLabelValues("zen-ingester").Set(float64(depth))
	}
}

// IncrementIngesterEvents increments the ingester events counter
func (m *Metrics) IncrementIngesterEvents(status string) {
	if m.IngesterEventsTotal != nil {
		m.IngesterEventsTotal.WithLabelValues("zen-ingester", status).Inc()
	}
}

// UpdateEgressQueueDepth updates the egress queue depth metric for KEDA autoscaling
// This should be called periodically (e.g., every 10 seconds) to track pending dispatches
func (m *Metrics) UpdateEgressQueueDepth(depth int) {
	if m.EgressQueueDepth != nil {
		m.EgressQueueDepth.WithLabelValues("zen-egress").Set(float64(depth))
	}
}

// IncrementEgressEvents increments the egress events counter
func (m *Metrics) IncrementEgressEvents(status string) {
	if m.EgressEventsTotal != nil {
		m.EgressEventsTotal.WithLabelValues("zen-egress", status).Inc()
	}
}

