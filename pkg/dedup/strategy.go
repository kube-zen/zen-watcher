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
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// DedupStrategy defines a deduplication strategy
type DedupStrategy interface {
	// ShouldCreate determines if a new Observation should be created
	// Returns true if the event should create an Observation, false if it should be dropped
	ShouldCreate(key DedupKey, content map[string]interface{}, window time.Duration) bool
	// Name returns the strategy name
	Name() string
}

// StrategyConfig holds configuration for a dedup strategy
type StrategyConfig struct {
	Strategy          string   // "fingerprint", "key", "strict-window"
	Window            string   // Duration string (e.g., "1h")
	Fields            []string // Fields for key-based strategy
	MaxEventsPerWindow int     // Max events per window for strict-window strategy
}

// GetStrategy returns a DedupStrategy by name
func GetStrategy(config StrategyConfig) DedupStrategy {
	switch config.Strategy {
	case "key":
		return &KeyBasedStrategy{
			fields: config.Fields,
		}
	case "strict-window":
		return &StrictWindowStrategy{
			maxEventsPerWindow: config.MaxEventsPerWindow,
		}
	case "fingerprint", "":
		fallthrough
	default:
		return &FingerprintStrategy{}
	}
}

// FingerprintStrategy implements fingerprint-based deduplication (default)
type FingerprintStrategy struct{}

func (s *FingerprintStrategy) Name() string {
	return "fingerprint"
}

func (s *FingerprintStrategy) ShouldCreate(key DedupKey, content map[string]interface{}, window time.Duration) bool {
	// Use existing fingerprint logic
	// This is a wrapper around existing Deduper behavior
	return true // Actual logic handled by Deduper.ShouldCreateWithContent
}

// KeyBasedStrategy implements field-based deduplication
type KeyBasedStrategy struct {
	fields []string
}

func (s *KeyBasedStrategy) Name() string {
	return "key"
}

func (s *KeyBasedStrategy) ShouldCreate(key DedupKey, content map[string]interface{}, window time.Duration) bool {
	// Key-based dedup uses explicit fields
	// If fields are specified, use them; otherwise fall back to default key fields
	if len(s.fields) == 0 {
		// Default fields: source, namespace, kind, name, reason
		return true // Logic handled by Deduper with key
	}
	return true
}

// StrictWindowStrategy implements strict window-based deduplication for noisy sources
type StrictWindowStrategy struct {
	maxEventsPerWindow int
}

func (s *StrictWindowStrategy) Name() string {
	return "strict-window"
}

func (s *StrictWindowStrategy) ShouldCreate(key DedupKey, content map[string]interface{}, window time.Duration) bool {
	// Strict window strategy uses shorter windows and event limits
	// This is designed for high-volume, repetitive events
	return true // Logic handled by Deduper with shorter window
}

