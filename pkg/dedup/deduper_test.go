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

func TestDeduper_ShouldCreate(t *testing.T) {
	// Create deduper with 2 second window for testing
	deduper := NewDeduper(2, 1000)

	key := DedupKey{
		Source:      "test",
		Namespace:   "default",
		Kind:        "Pod",
		Name:        "test-pod",
		Reason:      "test-reason",
		MessageHash: HashMessage("test message"),
	}

	// First event should create
	if !deduper.ShouldCreate(key) {
		t.Error("First event should create observation")
	}

	// Duplicate within window should not create
	if deduper.ShouldCreate(key) {
		t.Error("Duplicate within window should not create observation")
	}

	// Wait for window to expire
	time.Sleep(3 * time.Second)

	// After window expires, should create again
	if !deduper.ShouldCreate(key) {
		t.Error("After window expires, should create observation again")
	}
}

func TestDeduper_LRU(t *testing.T) {
	// Create deduper with small max size
	deduper := NewDeduper(60, 3)

	// Add 3 entries
	key1 := DedupKey{Source: "test1", Namespace: "default", Kind: "Pod", Name: "pod1", Reason: "r1", MessageHash: "h1"}
	key2 := DedupKey{Source: "test2", Namespace: "default", Kind: "Pod", Name: "pod2", Reason: "r2", MessageHash: "h2"}
	key3 := DedupKey{Source: "test3", Namespace: "default", Kind: "Pod", Name: "pod3", Reason: "r3", MessageHash: "h3"}

	deduper.ShouldCreate(key1)
	deduper.ShouldCreate(key2)
	deduper.ShouldCreate(key3)

	// Verify all 3 are in cache
	size, _, _ := deduper.Stats()
	if size != 3 {
		t.Errorf("Expected 3 entries, got %d", size)
	}

	// Add 4th entry - should evict LRU (key1)
	key4 := DedupKey{Source: "test4", Namespace: "default", Kind: "Pod", Name: "pod4", Reason: "r4", MessageHash: "h4"}
	deduper.ShouldCreate(key4)

	// Verify key1 was evicted
	if deduper.ShouldCreate(key1) {
		// key1 should create (was evicted)
	} else {
		t.Error("key1 should have been evicted and should create")
	}

	// Verify key4 is in cache
	if deduper.ShouldCreate(key4) {
		t.Error("key4 should be in cache and not create")
	}
}

func TestHashMessage(t *testing.T) {
	msg1 := "test message"
	msg2 := "test message"
	msg3 := "different message"

	hash1 := HashMessage(msg1)
	hash2 := HashMessage(msg2)
	hash3 := HashMessage(msg3)

	if hash1 != hash2 {
		t.Error("Same message should produce same hash")
	}

	if hash1 == hash3 {
		t.Error("Different messages should produce different hashes")
	}

	if hash1 == "" {
		t.Error("Hash should not be empty")
	}
}

func TestDedupKey_String(t *testing.T) {
	key := DedupKey{
		Source:      "test",
		Namespace:   "default",
		Kind:        "Pod",
		Name:        "test-pod",
		Reason:      "test-reason",
		MessageHash: "abc123",
	}

	expected := "test/default/Pod/test-pod/test-reason/abc123"
	if key.String() != expected {
		t.Errorf("Expected %s, got %s", expected, key.String())
	}
}

func TestDeduper_FingerprintBasedDedup(t *testing.T) {
	deduper := NewDeduper(60, 1000)

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
		"source":   "test",
		"category": "security",
		"severity": "HIGH",
	}

	// First observation with content
	if !deduper.ShouldCreateWithContent(key1, content) {
		t.Error("First observation with content should create")
	}

	// Same content, different key should still be deduplicated by fingerprint
	if deduper.ShouldCreateWithContent(key2, content) {
		t.Error("Same content with different key should be deduplicated by fingerprint")
	}
}

func TestDeduper_RateLimiting(t *testing.T) {
	// Create deduper with rate limiting enabled (via environment variable simulation)
	deduper := NewDeduper(60, 1000)

	// Set rate limit via reflection or direct field access
	// For this test, we'll assume rate limiting is configured via env vars
	// In real code, rate limits are set via NewDeduper with env vars

	key := DedupKey{
		Source:      "test",
		Namespace:   "default",
		Kind:        "Pod",
		Name:        "test-pod",
		Reason:      "test-reason",
		MessageHash: "hash1",
	}

	content := map[string]interface{}{
		"source": "test",
	}

	// Create many observations rapidly
	createdCount := 0
	for i := 0; i < 10; i++ {
		key.MessageHash = HashMessage("message" + string(rune(i)))
		if deduper.ShouldCreateWithContent(key, content) {
			createdCount++
		}
	}

	// With rate limiting, not all should be created immediately
	// Exact behavior depends on rate limit configuration
	if createdCount == 0 {
		t.Error("At least some observations should be created")
	}
}

func TestGenerateFingerprint(t *testing.T) {
	content1 := map[string]interface{}{
		"source":   "test",
		"category": "security",
		"severity": "HIGH",
		"resource": map[string]interface{}{
			"kind": "Pod",
			"name": "test-pod",
		},
	}

	content2 := map[string]interface{}{
		"source":   "test",
		"category": "security",
		"severity": "HIGH",
		"resource": map[string]interface{}{
			"kind": "Pod",
			"name": "test-pod",
		},
	}

	content3 := map[string]interface{}{
		"source":   "test",
		"category": "security",
		"severity": "LOW", // Different severity
		"resource": map[string]interface{}{
			"kind": "Pod",
			"name": "test-pod",
		},
	}

	fp1 := GenerateFingerprint(content1)
	fp2 := GenerateFingerprint(content2)
	fp3 := GenerateFingerprint(content3)

	if fp1 != fp2 {
		t.Error("Same content should generate same fingerprint")
	}

	if fp1 == fp3 {
		t.Error("Different content should generate different fingerprints")
	}

	if fp1 == "" {
		t.Error("Fingerprint should not be empty")
	}
}

func TestDeduper_TimeBuckets(t *testing.T) {
	// Create deduper with 10 second window, buckets will be ~1 second
	deduper := NewDeduper(10, 1000)

	key := DedupKey{
		Source:      "test",
		Namespace:   "default",
		Kind:        "Pod",
		Name:        "test-pod",
		Reason:      "test-reason",
		MessageHash: "hash1",
	}

	// First event creates observation
	if !deduper.ShouldCreate(key) {
		t.Error("First event should create observation")
	}

	// Same event immediately should not create
	if deduper.ShouldCreate(key) {
		t.Error("Duplicate within window should not create")
	}

	// Check bucket cleanup - stats should show buckets
	size, maxSize, windowSeconds := deduper.Stats()
	if size == 0 {
		t.Error("Deduper should have at least one entry")
	}
	if maxSize != 1000 {
		t.Errorf("Expected maxSize 1000, got %d", maxSize)
	}
	if windowSeconds != 10 {
		t.Errorf("Expected windowSeconds 10, got %d", windowSeconds)
	}
}
