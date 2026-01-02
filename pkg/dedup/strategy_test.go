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

package dedup

import (
	"testing"
	"time"
)

func TestFingerprintStrategy(t *testing.T) {
	deduper := NewDeduper(60, 1000)
	defer deduper.Stop()

	strategy := &FingerprintStrategy{}

	key1 := DedupKey{
		Source:      "test",
		Namespace:   "default",
		Kind:        "Pod",
		Name:        "test-pod",
		Reason:      "test-reason",
		MessageHash: "hash1",
	}

	key2 := DedupKey{
		Source:      "test",
		Namespace:   "default",
		Kind:        "Pod",
		Name:        "test-pod",
		Reason:      "test-reason",
		MessageHash: "hash2", // Different hash
	}

	// Same content should generate same fingerprint
	content := map[string]interface{}{
		"spec": map[string]interface{}{
			"source":   "test",
			"category": "security",
			"severity": "HIGH",
		},
	}

	// First observation with content should create
	if !strategy.ShouldCreate(deduper, key1, content) {
		t.Error("First observation with content should create")
	}

	// Same content, different key should still be deduplicated by fingerprint
	if strategy.ShouldCreate(deduper, key2, content) {
		t.Error("Same content with different key should be deduplicated by fingerprint")
	}

	// Different content should create
	content2 := map[string]interface{}{
		"spec": map[string]interface{}{
			"source":   "test",
			"category": "security",
			"severity": "LOW", // Different severity
		},
	}
	if !strategy.ShouldCreate(deduper, key1, content2) {
		t.Error("Different content should create new observation")
	}
}

func TestEventStreamStrategy(t *testing.T) {
	deduper := NewDeduper(60, 1000)
	defer deduper.Stop()

	strategy := &EventStreamStrategy{
		maxEventsPerWindow: 10,
	}

	key := DedupKey{
		Source:      "test",
		Namespace:   "default",
		Kind:        "Pod",
		Name:        "test-pod",
		Reason:      "test-reason",
		MessageHash: "hash1",
	}

	content := map[string]interface{}{
		"spec": map[string]interface{}{
			"source": "test",
		},
	}

	// First event should create
	if !strategy.ShouldCreate(deduper, key, content) {
		t.Error("First event should create observation")
	}

	// Duplicate within window should not create
	if strategy.ShouldCreate(deduper, key, content) {
		t.Error("Duplicate within window should not create observation")
	}

	// Wait for window to expire
	time.Sleep(3 * time.Second)

	// After window expires, should create again
	if !strategy.ShouldCreate(deduper, key, content) {
		t.Error("After window expires, should create observation again")
	}
}

func TestKeyBasedStrategy(t *testing.T) {
	deduper := NewDeduper(60, 1000)
	defer deduper.Stop()

	strategy := &KeyBasedStrategy{
		fields: []string{"source", "kind", "name"},
	}

	key1 := DedupKey{
		Source:      "test",
		Namespace:   "default",
		Kind:        "Pod",
		Name:        "test-pod",
		Reason:      "test-reason",
		MessageHash: "hash1",
	}

	key2 := DedupKey{
		Source:      "test",
		Namespace:   "default",
		Kind:        "Pod",
		Name:        "test-pod",
		Reason:      "test-reason",
		MessageHash: "hash2", // Different hash but same key fields
	}

	content := map[string]interface{}{
		"spec": map[string]interface{}{
			"source": "test",
		},
	}

	// First event should create
	if !strategy.ShouldCreate(deduper, key1, content) {
		t.Error("First event should create observation")
	}

	// Same key fields should be deduplicated
	if strategy.ShouldCreate(deduper, key2, content) {
		t.Error("Same key fields should be deduplicated")
	}
}

func TestGetStrategy(t *testing.T) {
	// Test default strategy
	config := StrategyConfig{
		Strategy: "",
	}
	strategy := GetStrategy(config)
	if strategy.Name() != "fingerprint" {
		t.Errorf("Expected fingerprint strategy, got %s", strategy.Name())
	}

	// Test fingerprint strategy
	config.Strategy = "fingerprint"
	strategy = GetStrategy(config)
	if strategy.Name() != "fingerprint" {
		t.Errorf("Expected fingerprint strategy, got %s", strategy.Name())
	}

	// Test event-stream strategy
	config.Strategy = "event-stream"
	config.MaxEventsPerWindow = 10
	strategy = GetStrategy(config)
	if strategy.Name() != "event-stream" {
		t.Errorf("Expected event-stream strategy, got %s", strategy.Name())
	}

	// Test key strategy
	config.Strategy = "key"
	config.Fields = []string{"source", "kind"}
	strategy = GetStrategy(config)
	if strategy.Name() != "key" {
		t.Errorf("Expected key strategy, got %s", strategy.Name())
	}
}

func TestStrategyGetWindow(t *testing.T) {
	defaultWindow := 60 * time.Second

	// Fingerprint strategy should use default window
	fpStrategy := &FingerprintStrategy{}
	if fpStrategy.GetWindow(defaultWindow) != defaultWindow {
		t.Error("Fingerprint strategy should use default window")
	}

	// Event stream strategy should use shorter window
	esStrategy := &EventStreamStrategy{}
	window := esStrategy.GetWindow(defaultWindow)
	if window >= defaultWindow {
		t.Errorf("Event stream strategy should use shorter window, got %v", window)
	}

	// Key strategy should use default window
	keyStrategy := &KeyBasedStrategy{}
	if keyStrategy.GetWindow(defaultWindow) != defaultWindow {
		t.Error("Key strategy should use default window")
	}
}
