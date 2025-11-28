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
