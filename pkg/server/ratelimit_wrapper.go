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

package server

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/kube-zen/zen-sdk/pkg/gc/ratelimiter"
	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
	"github.com/prometheus/client_golang/prometheus"
)

// limiterEntry holds a rate limiter and its last-seen timestamp
type limiterEntry struct {
	limiter  *ratelimiter.RateLimiter
	lastSeen time.Time
}

// PerKeyRateLimiter wraps zen-sdk rate limiter to provide per-key (per-IP) rate limiting.
// This maintains backward compatibility with zen-watcher's per-IP rate limiting semantics.
type PerKeyRateLimiter struct {
	mu                sync.Mutex
	limiters          map[string]*limiterEntry
	maxPerSec         int
	cleanupTick       *time.Ticker
	trustedProxyCIDRs []*net.IPNet
	cleanupInterval   time.Duration // Interval between cleanup runs
	entryTTL          time.Duration // Time-to-live for inactive entries
	webhookMetrics    *prometheus.CounterVec // Metrics for tracking rate limit rejections
}

// NewPerKeyRateLimiter creates a new per-key rate limiter.
// maxTokens is the maximum tokens per key, refillInterval is the refill interval.
// This converts refillInterval-based semantics to per-second rate limiting.
func NewPerKeyRateLimiter(maxTokens int, refillInterval time.Duration, trustedProxyCIDRs []*net.IPNet) *PerKeyRateLimiter {
	return NewPerKeyRateLimiterWithMetrics(maxTokens, refillInterval, trustedProxyCIDRs, nil)
}

// NewPerKeyRateLimiterWithMetrics creates a new per-key rate limiter with metrics support
func NewPerKeyRateLimiterWithMetrics(maxTokens int, refillInterval time.Duration, trustedProxyCIDRs []*net.IPNet, webhookMetrics *prometheus.CounterVec) *PerKeyRateLimiter {
	// Convert refillInterval to per-second rate
	// e.g., 100 tokens per minute = 100/60 = ~1.67 per second
	// Round up to ensure we don't exceed the intended rate
	maxPerSec := int(float64(maxTokens) / refillInterval.Seconds())
	if maxPerSec < 1 {
		maxPerSec = 1
	}

	rl := &PerKeyRateLimiter{
		limiters:          make(map[string]*limiterEntry),
		maxPerSec:         maxPerSec,
		trustedProxyCIDRs: trustedProxyCIDRs,
		cleanupInterval:   1 * time.Hour, // Run cleanup every hour
		entryTTL:          1 * time.Hour, // Remove entries inactive for 1 hour
		webhookMetrics:    webhookMetrics,
	}

	// Start periodic cleanup
	rl.cleanupTick = time.NewTicker(rl.cleanupInterval)
	go rl.cleanup()

	return rl
}

// Allow checks if a request from the given key should be allowed.
func (rl *PerKeyRateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	entry, exists := rl.limiters[key]
	if !exists {
		// Create new rate limiter for this key
		entry = &limiterEntry{
			limiter:  ratelimiter.NewRateLimiter(rl.maxPerSec),
			lastSeen: time.Now(),
		}
		rl.limiters[key] = entry
	} else {
		// Update last-seen timestamp
		entry.lastSeen = time.Now()
	}

	return entry.limiter.Allow()
}

// cleanup removes old entries that haven't been accessed within the TTL period.
func (rl *PerKeyRateLimiter) cleanup() {
	for range rl.cleanupTick.C {
		rl.mu.Lock()
		now := time.Now()
		removed := 0
		for key, entry := range rl.limiters {
			// Remove entries that haven't been accessed within the TTL period
			if now.Sub(entry.lastSeen) > rl.entryTTL {
				delete(rl.limiters, key)
				removed++
			}
		}
		remaining := len(rl.limiters)
		rl.mu.Unlock()

		if removed > 0 {
			logger := sdklog.NewLogger("zen-watcher-server")
			logger.Debug("Rate limiter cleanup completed",
				sdklog.Operation("rate_limit_cleanup"),
				sdklog.Int("removed", removed),
				sdklog.Int("remaining", remaining))
		}
	}
}

// Stop stops the cleanup ticker (for graceful shutdown)
func (rl *PerKeyRateLimiter) Stop() {
	if rl.cleanupTick != nil {
		rl.cleanupTick.Stop()
	}
}

// RateLimitMiddleware wraps a handler with rate limiting.
func (rl *PerKeyRateLimiter) RateLimitMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := getClientIP(r, rl.trustedProxyCIDRs)
		if !rl.Allow(key) {
			logger := sdklog.NewLogger("zen-watcher-server")
			logger.Warn("Rate limit exceeded",
				sdklog.Operation("rate_limit"),
				sdklog.String("reason", "rate_limit_exceeded"),
				sdklog.String("client_ip", key))
			
			// Track rate limit rejection in metrics
			if rl.webhookMetrics != nil {
				endpoint := getEndpointFromPath(r.URL.Path)
				rl.webhookMetrics.WithLabelValues(endpoint, "429").Inc()
			}
			
			w.WriteHeader(http.StatusTooManyRequests)
			if _, err := w.Write([]byte(`{"error":"rate limit exceeded"}`)); err != nil {
				logger.Warn("Failed to write rate limit response",
					sdklog.Operation("rate_limit"),
					sdklog.Error(err))
			}
			return
		}
		next(w, r)
	}
}
