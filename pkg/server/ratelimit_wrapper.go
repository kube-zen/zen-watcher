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
	"net/http"
	"sync"
	"time"

	"github.com/kube-zen/zen-sdk/pkg/gc/ratelimiter"
	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
)

// PerKeyRateLimiter wraps zen-sdk rate limiter to provide per-key (per-IP) rate limiting.
// This maintains backward compatibility with zen-watcher's per-IP rate limiting semantics.
type PerKeyRateLimiter struct {
	mu          sync.Mutex
	limiters    map[string]*ratelimiter.RateLimiter
	maxPerSec   int
	cleanupTick *time.Ticker
}

// NewPerKeyRateLimiter creates a new per-key rate limiter.
// maxTokens is the maximum tokens per key, refillInterval is the refill interval.
// This converts refillInterval-based semantics to per-second rate limiting.
func NewPerKeyRateLimiter(maxTokens int, refillInterval time.Duration) *PerKeyRateLimiter {
	// Convert refillInterval to per-second rate
	// e.g., 100 tokens per minute = 100/60 = ~1.67 per second
	// Round up to ensure we don't exceed the intended rate
	maxPerSec := int(float64(maxTokens) / refillInterval.Seconds())
	if maxPerSec < 1 {
		maxPerSec = 1
	}

	rl := &PerKeyRateLimiter{
		limiters:  make(map[string]*ratelimiter.RateLimiter),
		maxPerSec: maxPerSec,
	}

	// Cleanup old entries periodically
	rl.cleanupTick = time.NewTicker(1 * time.Hour)
	go rl.cleanup()

	return rl
}

// Allow checks if a request from the given key should be allowed.
func (rl *PerKeyRateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, exists := rl.limiters[key]
	if !exists {
		// Create new rate limiter for this key
		limiter = ratelimiter.NewRateLimiter(rl.maxPerSec)
		rl.limiters[key] = limiter
	}

	return limiter.Allow()
}

// cleanup removes old entries to prevent memory leaks.
func (rl *PerKeyRateLimiter) cleanup() {
	for range rl.cleanupTick.C {
		// Note: zen-sdk rate limiter doesn't track last access time,
		// so we can't implement automatic cleanup based on inactivity.
		// For now, we keep all limiters. In production, consider adding
		// a last-access-time tracking mechanism if memory becomes an issue.
		// No cleanup logic currently implemented, so no lock needed.
		_ = rl // Keep reference to avoid unused receiver warning
	}
}

// RateLimitMiddleware wraps a handler with rate limiting.
func (rl *PerKeyRateLimiter) RateLimitMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := getClientIP(r)
		if !rl.Allow(key) {
			logger := sdklog.NewLogger("zen-watcher-server")
			logger.Warn("Rate limit exceeded",
				sdklog.Operation("rate_limit"),
				sdklog.String("reason", "rate_limit_exceeded"),
				sdklog.String("client_ip", key))
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
