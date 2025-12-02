// Copyright 2024 The Zen Watcher Authors
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
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
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

// timeBucket represents a time-based bucket for organizing events
type timeBucket struct {
	startTime    time.Time
	keys         map[string]time.Time // dedup key -> last seen time
	fingerprints map[string]time.Time // fingerprint -> last seen time
}

// fingerprint represents a content-based fingerprint
type fingerprint struct {
	hash      string
	timestamp time.Time
	count     int // number of times seen
}

// rateLimitTracker tracks rate limiting per source using token bucket algorithm
type rateLimitTracker struct {
	mu         sync.Mutex
	tokens     int       // current tokens available
	lastRefill time.Time // last token refill time
	maxTokens  int       // maximum tokens (burst capacity)
	refillRate float64   // tokens per second
}

// aggregatedEvent represents an aggregated event within a rolling window
type aggregatedEvent struct {
	firstSeen time.Time
	lastSeen  time.Time
	count     int
	fingerprint string
}

// Deduper provides enhanced deduplication with:
// - Time-based buckets for efficient cleanup
// - Content-based fingerprinting
// - Rate limiting per source
// - Event aggregation in rolling window
type Deduper struct {
	mu sync.RWMutex

	// Original cache for backward compatibility
	cache         map[string]*entry // key -> entry
	lruList       []string          // LRU list (most recent at end)
	maxSize       int               // Maximum cache size (LRU eviction)
	windowSeconds int               // Sliding window in seconds
	ttl           time.Duration     // TTL for entries

	// Enhanced features
	// Time-based buckets
	buckets          map[int64]*timeBucket // bucket key (unix timestamp) -> bucket
	bucketSizeSeconds int                  // size of each bucket in seconds

	// Fingerprint-based dedup
	fingerprints map[string]*fingerprint // fingerprint hash -> fingerprint metadata

	// Rate limiting per source
	rateLimits      map[string]*rateLimitTracker // source -> rate limit tracker
	maxRatePerSource int                        // maximum events per source per second
	maxRateBurst     int                        // burst capacity

	// Event aggregation
	aggregatedEvents map[string]*aggregatedEvent // fingerprint -> aggregated event
	enableAggregation bool                       // whether aggregation is enabled
}

// NewDeduper creates a new deduper with specified configuration and enhanced features
func NewDeduper(windowSeconds, maxSize int) *Deduper {
	if windowSeconds <= 0 {
		windowSeconds = 60 // Default 60 seconds
	}
	if maxSize <= 0 {
		maxSize = 10000 // Default 10k entries
	}

	// Read bucket size from env, default to 10% of window or minimum 10 seconds
	bucketSizeSeconds := windowSeconds / 10
	if bucketSizeSeconds < 10 {
		bucketSizeSeconds = 10
	}
	if bucketStr := os.Getenv("DEDUP_BUCKET_SIZE_SECONDS"); bucketStr != "" {
		if b, err := strconv.Atoi(bucketStr); err == nil && b > 0 {
			bucketSizeSeconds = b
		}
	}

	// Read rate limit from env, default 100 events/second per source
	maxRatePerSource := 100
	if rateStr := os.Getenv("DEDUP_MAX_RATE_PER_SOURCE"); rateStr != "" {
		if r, err := strconv.Atoi(rateStr); err == nil && r > 0 {
			maxRatePerSource = r
		}
	}

	// Read burst capacity from env, default 2x rate limit
	maxRateBurst := maxRatePerSource * 2
	if burstStr := os.Getenv("DEDUP_RATE_BURST"); burstStr != "" {
		if b, err := strconv.Atoi(burstStr); err == nil && b > 0 {
			maxRateBurst = b
		}
	}

	// Read aggregation enable flag from env, default true
	enableAggregation := true
	if aggStr := os.Getenv("DEDUP_ENABLE_AGGREGATION"); aggStr != "" {
		if aggStr == "false" || aggStr == "0" {
			enableAggregation = false
		}
	}

	deduper := &Deduper{
		cache:              make(map[string]*entry),
		lruList:            make([]string, 0, maxSize),
		maxSize:            maxSize,
		windowSeconds:      windowSeconds,
		ttl:                time.Duration(windowSeconds) * time.Second,
		buckets:            make(map[int64]*timeBucket),
		bucketSizeSeconds:  bucketSizeSeconds,
		fingerprints:       make(map[string]*fingerprint),
		rateLimits:         make(map[string]*rateLimitTracker),
		maxRatePerSource:   maxRatePerSource,
		maxRateBurst:       maxRateBurst,
		aggregatedEvents:   make(map[string]*aggregatedEvent),
		enableAggregation:  enableAggregation,
	}

	// Start background cleanup goroutine for enhanced features
	go deduper.cleanupLoop()

	return deduper
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

// GenerateFingerprint creates a content-based fingerprint from observation content
// Includes: source, category, severity, eventType, resource, and critical details
func GenerateFingerprint(content map[string]interface{}) string {
	normalized := make(map[string]interface{})

	// Extract key fields for fingerprinting
	if spec, ok := content["spec"].(map[string]interface{}); ok {
		// Include source, category, severity, eventType
		if source, ok := spec["source"].(string); ok {
			normalized["source"] = source
		}
		if category, ok := spec["category"].(string); ok {
			normalized["category"] = category
		}
		if severity, ok := spec["severity"].(string); ok {
			normalized["severity"] = severity
		}
		if eventType, ok := spec["eventType"].(string); ok {
			normalized["eventType"] = eventType
		}
		// Normalize resource for fingerprinting
		if resource, ok := spec["resource"].(map[string]interface{}); ok {
			normResource := make(map[string]interface{})
			if kind, ok := resource["kind"].(string); ok {
				normResource["kind"] = kind
			}
			if name, ok := resource["name"].(string); ok {
				normResource["name"] = name
			}
			if ns, ok := resource["namespace"].(string); ok {
				normResource["namespace"] = ns
			}
			normalized["resource"] = normResource
		}
		// Include critical details for fingerprinting (vulnerability ID, rule, etc.)
		if details, ok := spec["details"].(map[string]interface{}); ok {
			normDetails := make(map[string]interface{})
			// Include fields that uniquely identify the event
			for _, field := range []string{"vulnerabilityID", "rule", "policy", "reason", "auditID", "checkId"} {
				if val, ok := details[field]; ok {
					normDetails[field] = val
				}
			}
			if len(normDetails) > 0 {
				normalized["details"] = normDetails
			}
		}
	}

	// Serialize to JSON for consistent hashing
	jsonBytes, err := json.Marshal(normalized)
	if err != nil {
		// Fallback: hash the string representation
		jsonBytes = []byte(fmt.Sprintf("%v", normalized))
	}

	hash := sha256.Sum256(jsonBytes)
	return fmt.Sprintf("%x", hash[:16]) // Use first 16 bytes for fingerprint
}

// getBucketKey returns the bucket key for a given time
func (d *Deduper) getBucketKey(t time.Time) int64 {
	return t.Unix() / int64(d.bucketSizeSeconds)
}

// checkRateLimit checks if the source is within rate limits (called with lock held)
func (d *Deduper) checkRateLimit(source string, now time.Time) bool {
	tracker, exists := d.rateLimits[source]
	if !exists {
		tracker = &rateLimitTracker{
			tokens:     d.maxRateBurst,
			lastRefill: now,
			maxTokens:  d.maxRateBurst,
			refillRate: float64(d.maxRatePerSource),
		}
		d.rateLimits[source] = tracker
	}

	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	// Refill tokens based on time elapsed
	elapsed := now.Sub(tracker.lastRefill).Seconds()
	tokensToAdd := int(elapsed * tracker.refillRate)
	if tokensToAdd > 0 {
		tracker.tokens = tracker.tokens + tokensToAdd
		if tracker.tokens > tracker.maxTokens {
			tracker.tokens = tracker.maxTokens
		}
		tracker.lastRefill = now
	}

	// Check if we have tokens
	if tracker.tokens <= 0 {
		return false
	}

	// Consume one token
	tracker.tokens--
	return true
}

// isDuplicateInBucket checks if this key was seen in the current time bucket (called with lock held)
func (d *Deduper) isDuplicateInBucket(keyStr, fingerprintHash string, now time.Time) bool {
	bucketKey := d.getBucketKey(now)
	bucket, exists := d.buckets[bucketKey]
	if !exists {
		return false
	}

	// Check by key
	if lastSeen, exists := bucket.keys[keyStr]; exists {
		bucketDuration := time.Duration(d.bucketSizeSeconds) * time.Second
		if now.Sub(lastSeen) < bucketDuration {
			return true
		}
	}

	// Check by fingerprint
	if lastSeen, exists := bucket.fingerprints[fingerprintHash]; exists {
		bucketDuration := time.Duration(d.bucketSizeSeconds) * time.Second
		if now.Sub(lastSeen) < bucketDuration {
			return true
		}
	}

	return false
}

// addToBucket adds the key to the appropriate time bucket (called with lock held)
func (d *Deduper) addToBucket(keyStr, fingerprintHash string, now time.Time) {
	bucketKey := d.getBucketKey(now)
	bucket, exists := d.buckets[bucketKey]
	if !exists {
		startTime := now.Truncate(time.Duration(d.bucketSizeSeconds) * time.Second)
		bucket = &timeBucket{
			startTime:    startTime,
			keys:         make(map[string]time.Time),
			fingerprints: make(map[string]time.Time),
		}
		d.buckets[bucketKey] = bucket
	}

	bucket.keys[keyStr] = now
	bucket.fingerprints[fingerprintHash] = now
}

// isDuplicateFingerprint checks if this fingerprint was seen recently (called with lock held)
func (d *Deduper) isDuplicateFingerprint(fingerprintHash string, now time.Time) bool {
	fp, exists := d.fingerprints[fingerprintHash]
	if !exists {
		return false
	}

	// Check if fingerprint is still within window
	age := now.Sub(fp.timestamp)
	if age >= time.Duration(d.windowSeconds)*time.Second {
		return false // Expired, not a duplicate
	}

	// Update count and timestamp
	fp.count++
	fp.timestamp = now
	return true
}

// addFingerprint adds or updates a fingerprint (called with lock held)
func (d *Deduper) addFingerprint(fingerprintHash string, now time.Time) {
	// Check if we need to evict old fingerprints
	if len(d.fingerprints) >= d.maxSize {
		// Evict oldest fingerprint
		var oldestKey string
		var oldestTime time.Time = now
		for k, v := range d.fingerprints {
			if v.timestamp.Before(oldestTime) {
				oldestTime = v.timestamp
				oldestKey = k
			}
		}
		if oldestKey != "" {
			delete(d.fingerprints, oldestKey)
		}
	}

	d.fingerprints[fingerprintHash] = &fingerprint{
		hash:      fingerprintHash,
		timestamp: now,
		count:     1,
	}
}

// updateAggregation updates the aggregated event counter (called with lock held)
func (d *Deduper) updateAggregation(fingerprintHash string, now time.Time) {
	if !d.enableAggregation {
		return
	}

	agg, exists := d.aggregatedEvents[fingerprintHash]
	if !exists {
		agg = &aggregatedEvent{
			firstSeen:   now,
			lastSeen:    now,
			count:       0,
			fingerprint: fingerprintHash,
		}
		d.aggregatedEvents[fingerprintHash] = agg
	}

	agg.lastSeen = now
	agg.count++
}

// cleanupOldBuckets removes buckets that are outside the window (called with lock held)
func (d *Deduper) cleanupOldBuckets(now time.Time) {
	cutoffTime := now.Add(-time.Duration(d.windowSeconds) * time.Second)
	cutoffBucket := d.getBucketKey(cutoffTime)

	for bucketKey := range d.buckets {
		if bucketKey < cutoffBucket {
			delete(d.buckets, bucketKey)
		}
	}

	// Limit number of buckets (window/bucket size + 2 for safety)
	maxBuckets := (d.windowSeconds / d.bucketSizeSeconds) + 2
	if len(d.buckets) > maxBuckets {
		// Remove oldest buckets
		keys := make([]int64, 0, len(d.buckets))
		for k := range d.buckets {
			keys = append(keys, k)
		}
		// Sort and remove oldest
		for len(d.buckets) > maxBuckets && len(keys) > 0 {
			// Find oldest
			oldestIdx := 0
			for i, k := range keys {
				if k < keys[oldestIdx] {
					oldestIdx = i
				}
			}
			delete(d.buckets, keys[oldestIdx])
			keys = append(keys[:oldestIdx], keys[oldestIdx+1:]...)
		}
	}
}

// cleanupOldFingerprints removes fingerprints outside the window (called with lock held)
func (d *Deduper) cleanupOldFingerprints(now time.Time) {
	cutoff := now.Add(-time.Duration(d.windowSeconds) * time.Second)
	for hash, fp := range d.fingerprints {
		if fp.timestamp.Before(cutoff) {
			delete(d.fingerprints, hash)
		}
	}
}

// cleanupOldAggregations removes aggregated events outside the window (called with lock held)
func (d *Deduper) cleanupOldAggregations(now time.Time) {
	if !d.enableAggregation {
		return
	}

	cutoff := now.Add(-time.Duration(d.windowSeconds) * time.Second)
	for hash, agg := range d.aggregatedEvents {
		if agg.lastSeen.Before(cutoff) {
			delete(d.aggregatedEvents, hash)
		}
	}
}

// cleanupLoop runs periodic cleanup in background
func (d *Deduper) cleanupLoop() {
	ticker := time.NewTicker(time.Duration(d.bucketSizeSeconds) * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		d.mu.Lock()
		now := time.Now()
		d.cleanupOldBuckets(now)
		d.cleanupOldFingerprints(now)
		d.cleanupOldAggregations(now)
		d.mu.Unlock()
	}
}

// ShouldCreate checks if an observation should be created (backward compatible)
// Returns true if this is the first event (should create), false if duplicate within window
func (d *Deduper) ShouldCreate(key DedupKey) bool {
	return d.ShouldCreateWithContent(key, nil)
}

// ShouldCreateWithContent checks if an observation should be created with enhanced features
// If content is provided, uses fingerprint-based dedup and all enhanced features
// Returns true if this is the first event (should create), false if duplicate within window
func (d *Deduper) ShouldCreateWithContent(key DedupKey, content map[string]interface{}) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	keyStr := key.String()
	now := time.Now()

	// 1. Rate limiting check (only if we have a source)
	if key.Source != "" {
		if !d.checkRateLimit(key.Source, now) {
			return false // Rate limit exceeded
		}
	}

	// 2. Generate fingerprint if content is provided
	fingerprintHash := ""
	if content != nil {
		fingerprintHash = GenerateFingerprint(content)
		
		// Check fingerprint-based dedup first (more accurate)
		if d.isDuplicateFingerprint(fingerprintHash, now) {
			// Update aggregation
			d.updateAggregation(fingerprintHash, now)
			return false // Duplicate fingerprint
		}
	}

	// 3. Check time-based bucket dedup
	if d.isDuplicateInBucket(keyStr, fingerprintHash, now) {
		if fingerprintHash != "" {
			d.updateAggregation(fingerprintHash, now)
		}
		return false // Duplicate in bucket
	}

	// 4. Cleanup old buckets and fingerprints
	d.cleanupOldBuckets(now)
	d.cleanupOldFingerprints(now)
	d.cleanupOldAggregations(now)

	// 5. Original cache-based dedup (for backward compatibility)
	d.cleanupExpired(now)
	if ent, exists := d.cache[keyStr]; exists {
		if now.Sub(ent.timestamp) < d.ttl {
			d.updateLRU(keyStr)
			ent.timestamp = now
			return false // Duplicate in original cache
		}
		delete(d.cache, keyStr)
		d.removeFromLRU(keyStr)
	}

	// 6. Add to all structures
	d.addToBucket(keyStr, fingerprintHash, now)
	if fingerprintHash != "" {
		d.addFingerprint(fingerprintHash, now)
		d.updateAggregation(fingerprintHash, now)
	}
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

// EnhancedStats returns enhanced statistics including buckets, fingerprints, and aggregations
func (d *Deduper) EnhancedStats() (buckets int, fingerprints int, aggregated int, rateLimits int) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.buckets), len(d.fingerprints), len(d.aggregatedEvents), len(d.rateLimits)
}

// Clear removes all entries from cache
func (d *Deduper) Clear() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.cache = make(map[string]*entry)
	d.lruList = make([]string, 0, d.maxSize)
}
