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
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
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

// PerKeyRateLimiter wraps zen-sdk rate limiter to provide per-key rate limiting.
// Supports per-IP (legacy) and per-endpoint rate limiting based on URL path structure.
// zen-watcher is decoupled and only cares about endpoint identifiers, not tenant/namespace concepts.
type PerKeyRateLimiter struct {
	mu                sync.Mutex
	limiters          map[string]*limiterEntry
	maxPerSec         int
	cleanupTick       *time.Ticker
	trustedProxyCIDRs []*net.IPNet
	cleanupInterval   time.Duration          // Interval between cleanup runs
	entryTTL          time.Duration          // Time-to-live for inactive entries
	webhookMetrics    *prometheus.CounterVec // Metrics for tracking webhook requests (includes 429)
	rateLimitMetrics  *prometheus.CounterVec // Metrics for tracking rate limit rejections by scope
}

// NewPerKeyRateLimiter creates a new per-key rate limiter.
// maxTokens is the maximum tokens per key, refillInterval is the refill interval.
// This converts refillInterval-based semantics to per-second rate limiting.
func NewPerKeyRateLimiter(maxTokens int, refillInterval time.Duration, trustedProxyCIDRs []*net.IPNet) *PerKeyRateLimiter {
	return NewPerKeyRateLimiterWithMetrics(maxTokens, refillInterval, trustedProxyCIDRs, nil, nil)
}

// NewPerKeyRateLimiterWithMetrics creates a new per-key rate limiter with metrics support
func NewPerKeyRateLimiterWithMetrics(maxTokens int, refillInterval time.Duration, trustedProxyCIDRs []*net.IPNet, webhookMetrics *prometheus.CounterVec, rateLimitMetrics *prometheus.CounterVec) *PerKeyRateLimiter {
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
		rateLimitMetrics:  rateLimitMetrics,
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
			serverLogger.Debug("Rate limiter cleanup completed",
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
// Supports per-IP (legacy) and per-endpoint rate limiting based on URL path structure.
// zen-watcher is decoupled and only cares about endpoint identifiers, not tenant/namespace concepts.
func (rl *PerKeyRateLimiter) RateLimitMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract endpoint identifier from path (last segment of multi-segment paths)
		// This allows per-endpoint rate limiting without knowing about tenant/namespace structure
		endpointName := getEndpointFromPath(r.URL.Path)
		
		var key string
		var rateLimitScope string
		
		// Use per-endpoint rate limiting if path has multiple segments (suggests structured routing)
		// Otherwise fall back to per-IP for backward compatibility with legacy single-segment paths
		pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
		if len(pathParts) >= 2 {
			// Multi-segment path: use endpoint identifier for rate limiting
			// This provides endpoint-level isolation without coupling to tenant concepts
			key = fmt.Sprintf("endpoint:%s", endpointName)
			rateLimitScope = "endpoint"
		} else {
			// Single-segment or legacy path: use IP address for backward compatibility
			key = getClientIP(r, rl.trustedProxyCIDRs)
			rateLimitScope = "ip"
		}
		
		if !rl.Allow(key) {
			serverLogger.Warn("Rate limit exceeded",
				sdklog.Operation("rate_limit"),
				sdklog.ErrorCode("RATE_LIMIT_EXCEEDED"),
				sdklog.String("reason", "rate_limit_exceeded"),
				sdklog.String("scope", rateLimitScope),
				sdklog.String("key", key),
				sdklog.HTTPPath(r.URL.Path))

			// Track rate limit rejection in metrics
			if rl.webhookMetrics != nil {
				rl.webhookMetrics.WithLabelValues(endpointName, "429").Inc()
			}
			
			// Track rate limit rejection with scope (endpoint vs IP)
			// This allows monitoring rate limit effectiveness per scope
			if rl.rateLimitMetrics != nil {
				rl.rateLimitMetrics.WithLabelValues(endpointName, rateLimitScope).Inc()
			}

			// Set Retry-After header (RFC 6585)
			// Default: 60 seconds (can be made configurable)
			w.Header().Set("Retry-After", "60")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			
			response := map[string]interface{}{
				"error":        "rate limit exceeded",
				"endpoint":     endpointName,
				"retry_after":  60,
			}
			
			if err := json.NewEncoder(w).Encode(response); err != nil {
				serverLogger.Warn("Failed to write rate limit response",
					sdklog.Operation("rate_limit"),
					sdklog.ErrorCode("HTTP_WRITE_ERROR"),
					sdklog.Error(err))
			}
			return
		}
		next(w, r)
	}
}
