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

package server

import (
	"net/http"
	"sync"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/logger"
)

// RateLimiter implements token bucket rate limiting
type RateLimiter struct {
	mu          sync.Mutex
	tokens      map[string]*tokenBucket
	maxTokens   int
	refillRate  time.Duration
	cleanupTick *time.Ticker
}

type tokenBucket struct {
	tokens     int
	lastRefill time.Time
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(maxTokens int, refillInterval time.Duration) *RateLimiter {
	rl := &RateLimiter{
		tokens:     make(map[string]*tokenBucket),
		maxTokens:  maxTokens,
		refillRate: refillInterval,
	}

	// Cleanup old entries periodically
	rl.cleanupTick = time.NewTicker(1 * time.Hour)
	go rl.cleanup()

	return rl
}

// Allow checks if a request from the given key should be allowed
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	bucket, exists := rl.tokens[key]
	now := time.Now()

	if !exists {
		bucket = &tokenBucket{
			tokens:     rl.maxTokens - 1,
			lastRefill: now,
		}
		rl.tokens[key] = bucket
		return true
	}

	// Refill tokens based on elapsed time
	elapsed := now.Sub(bucket.lastRefill)
	if elapsed >= rl.refillRate {
		refills := int(elapsed / rl.refillRate)
		bucket.tokens = min(rl.maxTokens, bucket.tokens+refills)
		bucket.lastRefill = now
	}

	if bucket.tokens > 0 {
		bucket.tokens--
		return true
	}

	return false
}

// cleanup removes old entries to prevent memory leaks
func (rl *RateLimiter) cleanup() {
	for range rl.cleanupTick.C {
		rl.mu.Lock()
		now := time.Now()
		for key, bucket := range rl.tokens {
			if now.Sub(bucket.lastRefill) > 24*time.Hour {
				delete(rl.tokens, key)
			}
		}
		rl.mu.Unlock()
	}
}

// getClientKey returns a key for rate limiting (IP-based)
func getClientKey(r *http.Request) string {
	// Use IP address for rate limiting
	ip := getClientIP(r)
	return ip
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// RateLimitMiddleware wraps a handler with rate limiting
func (rl *RateLimiter) RateLimitMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := getClientKey(r)
		if !rl.Allow(key) {
			logger.Warn("Rate limit exceeded",
				logger.Fields{
					Component: "server",
					Operation: "rate_limit",
					Reason:    "rate_limit_exceeded",
					Additional: map[string]interface{}{
						"client_ip": key,
					},
				})
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"rate limit exceeded"}`))
			return
		}
		next(w, r)
	}
}
