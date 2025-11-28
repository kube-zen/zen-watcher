package dedup

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"
)

// DedupKey represents the minimal metadata for deduplication
type DedupKey struct {
	Source      string
	Namespace   string
	Kind        string
	Name        string
	Reason      string
	MessageHash string // SHA256 hash of message (first 16 chars for efficiency)
}

// String returns a string representation of the dedup key
func (k DedupKey) String() string {
	return fmt.Sprintf("%s/%s/%s/%s/%s/%s", k.Source, k.Namespace, k.Kind, k.Name, k.Reason, k.MessageHash)
}

// entry represents a cache entry with timestamp
type entry struct {
	key       string
	timestamp time.Time
}

// Deduper provides sliding window deduplication with LRU and TTL
type Deduper struct {
	mu            sync.RWMutex
	cache         map[string]*entry // key -> entry
	lruList       []string          // LRU list (most recent at end)
	maxSize       int               // Maximum cache size (LRU eviction)
	windowSeconds int               // Sliding window in seconds
	ttl           time.Duration     // TTL for entries
}

// NewDeduper creates a new deduper with specified configuration
func NewDeduper(windowSeconds, maxSize int) *Deduper {
	if windowSeconds <= 0 {
		windowSeconds = 60 // Default 60 seconds
	}
	if maxSize <= 0 {
		maxSize = 10000 // Default 10k entries
	}
	return &Deduper{
		cache:         make(map[string]*entry),
		lruList:       make([]string, 0, maxSize),
		maxSize:       maxSize,
		windowSeconds: windowSeconds,
		ttl:           time.Duration(windowSeconds) * time.Second,
	}
}

// HashMessage creates a short hash of the message for deduplication
func HashMessage(message string) string {
	if message == "" {
		return ""
	}
	hash := sha256.Sum256([]byte(message))
	// Use first 16 characters of hex representation for efficiency
	return fmt.Sprintf("%x", hash[:8])
}

// ShouldCreate checks if an observation should be created
// Returns true if this is the first event (should create), false if duplicate within window
func (d *Deduper) ShouldCreate(key DedupKey) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	keyStr := key.String()
	now := time.Now()

	// Clean up expired entries first
	d.cleanupExpired(now)

	// Check if entry exists and is within window
	if ent, exists := d.cache[keyStr]; exists {
		// Check if entry is still within TTL
		if now.Sub(ent.timestamp) < d.ttl {
			// Update LRU (move to end)
			d.updateLRU(keyStr)
			// Update timestamp for sliding window
			ent.timestamp = now
			return false // Duplicate within window
		}
		// Entry expired, remove it
		delete(d.cache, keyStr)
		d.removeFromLRU(keyStr)
	}

	// First event or expired entry - should create
	// Add to cache
	d.addToCache(keyStr, now)

	return true // First event, should create
}

// cleanupExpired removes expired entries (called with lock held)
func (d *Deduper) cleanupExpired(now time.Time) {
	expired := make([]string, 0)
	for keyStr, ent := range d.cache {
		if now.Sub(ent.timestamp) >= d.ttl {
			expired = append(expired, keyStr)
		}
	}
	for _, keyStr := range expired {
		delete(d.cache, keyStr)
		d.removeFromLRU(keyStr)
	}
}

// addToCache adds a new entry to cache with LRU eviction if needed
func (d *Deduper) addToCache(keyStr string, timestamp time.Time) {
	// If at capacity, evict LRU (oldest)
	if len(d.cache) >= d.maxSize && len(d.lruList) > 0 {
		// Remove oldest (first in list)
		oldest := d.lruList[0]
		delete(d.cache, oldest)
		d.lruList = d.lruList[1:]
	}

	// Add new entry
	d.cache[keyStr] = &entry{
		key:       keyStr,
		timestamp: timestamp,
	}
	d.lruList = append(d.lruList, keyStr)
}

// updateLRU moves key to end of LRU list (most recent)
func (d *Deduper) updateLRU(keyStr string) {
	// Find and remove from current position
	for i, k := range d.lruList {
		if k == keyStr {
			// Remove from current position
			d.lruList = append(d.lruList[:i], d.lruList[i+1:]...)
			break
		}
	}
	// Add to end (most recent)
	d.lruList = append(d.lruList, keyStr)
}

// removeFromLRU removes key from LRU list
func (d *Deduper) removeFromLRU(keyStr string) {
	for i, k := range d.lruList {
		if k == keyStr {
			d.lruList = append(d.lruList[:i], d.lruList[i+1:]...)
			return
		}
	}
}

// Stats returns current cache statistics
func (d *Deduper) Stats() (size int, maxSize int, windowSeconds int) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.cache), d.maxSize, d.windowSeconds
}

// Clear removes all entries from cache
func (d *Deduper) Clear() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.cache = make(map[string]*entry)
	d.lruList = make([]string, 0, d.maxSize)
}
