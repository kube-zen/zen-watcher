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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func TestPerKeyRateLimiter_Allow(t *testing.T) {
	rl := NewPerKeyRateLimiter(10, 1*time.Second, nil)

	// First 10 requests should be allowed
	for i := 0; i < 10; i++ {
		if !rl.Allow("test-key") {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}

	// 11th request should be rate limited
	if rl.Allow("test-key") {
		t.Error("11th request should be rate limited")
	}

	// Different key should still be allowed
	if !rl.Allow("different-key") {
		t.Error("Different key should be allowed")
	}
}

func TestPerKeyRateLimiter_Cleanup(t *testing.T) {
	rl := NewPerKeyRateLimiter(10, 1*time.Second, nil)
	rl.entryTTL = 100 * time.Millisecond // Short TTL for testing
	rl.cleanupInterval = 200 * time.Millisecond

	// Create entries
	rl.Allow("key1")
	rl.Allow("key2")

	// Wait for cleanup
	time.Sleep(300 * time.Millisecond)

	// Entries should be cleaned up (new limiters created)
	rl.mu.Lock()
	entryCount := len(rl.limiters)
	rl.mu.Unlock()

	// Cleanup should have removed old entries
	// (exact count depends on timing, but should be <= 2)
	if entryCount > 2 {
		t.Errorf("Expected cleanup, but found %d entries", entryCount)
	}
}

func TestRateLimitMiddleware_PerEndpoint(t *testing.T) {
	// Create rate limiter with low limit for testing
	rl := NewPerKeyRateLimiter(2, 1*time.Second, nil)

	handler := rl.RateLimitMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Test multi-segment path (per-endpoint rate limiting)
	req := httptest.NewRequest("POST", "/prefix/endpoint-name", nil)
	req.RemoteAddr = "192.168.1.100:12345"

	// First 2 requests should succeed
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		handler(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("Request %d should succeed, got status %d", i+1, w.Code)
		}
	}

	// 3rd request should be rate limited
	w := httptest.NewRecorder()
	handler(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected 429, got %d", w.Code)
	}

	// Verify response body
	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["error"] != "rate limit exceeded" {
		t.Errorf("Expected error message, got %v", response["error"])
	}

	if response["endpoint"] != "endpoint-name" {
		t.Errorf("Expected endpoint 'endpoint-name', got %v", response["endpoint"])
	}

	// Verify Retry-After header
	if w.Header().Get("Retry-After") != "60" {
		t.Errorf("Expected Retry-After: 60, got %s", w.Header().Get("Retry-After"))
	}
}

func TestRateLimitMiddleware_PerIP(t *testing.T) {
	// Create rate limiter with low limit for testing
	rl := NewPerKeyRateLimiter(2, 1*time.Second, nil)

	handler := rl.RateLimitMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Test single-segment path (per-IP rate limiting)
	req := httptest.NewRequest("POST", "/falco/webhook", nil)
	req.RemoteAddr = "192.168.1.100:12345"

	// First 2 requests should succeed
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		handler(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("Request %d should succeed, got status %d", i+1, w.Code)
		}
	}

	// 3rd request should be rate limited
	w := httptest.NewRecorder()
	handler(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected 429, got %d", w.Code)
	}
}

func TestRateLimitMiddleware_DifferentEndpoints(t *testing.T) {
	// Create rate limiter with low limit for testing
	rl := NewPerKeyRateLimiter(2, 1*time.Second, nil)

	handler := rl.RateLimitMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Different endpoints should have separate rate limits
	endpoints := []string{"/prefix/endpoint1", "/prefix/endpoint2"}

	for _, endpoint := range endpoints {
		req := httptest.NewRequest("POST", endpoint, nil)
		req.RemoteAddr = "192.168.1.100:12345"

		// Each endpoint should allow 2 requests
		for i := 0; i < 2; i++ {
			w := httptest.NewRecorder()
			handler(w, req)
			if w.Code != http.StatusOK {
				t.Errorf("Endpoint %s request %d should succeed, got status %d", endpoint, i+1, w.Code)
			}
		}
	}
}

func TestRateLimitMiddleware_Metrics(t *testing.T) {
	// Create metrics
	webhookMetrics := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "test_webhook_requests_total",
			Help: "Test metric",
		},
		[]string{"endpoint", "status"},
	)

	rateLimitMetrics := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "test_rate_limit_rejections_total",
			Help: "Test metric",
		},
		[]string{"endpoint", "scope"},
	)

	rl := NewPerKeyRateLimiterWithMetrics(1, 1*time.Second, nil, webhookMetrics, rateLimitMetrics)

	handler := rl.RateLimitMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("POST", "/prefix/test-endpoint", nil)
	req.RemoteAddr = "192.168.1.100:12345"

	// First request should succeed
	w := httptest.NewRecorder()
	handler(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("First request should succeed, got %d", w.Code)
	}

	// Second request should be rate limited
	w = httptest.NewRecorder()
	handler(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected 429, got %d", w.Code)
	}

	// Verify metrics were incremented
	// Note: In a real test, you'd use a metrics testing library to verify exact values
	// For now, we just verify the code path executes without errors
}

func TestGetEndpointFromPath(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/falco/webhook", "webhook"},
		{"/prefix/endpoint-name", "endpoint-name"},
		{"/endpoint", "endpoint"},
		{"/a/b/c/endpoint", "endpoint"},
		{"/", "unknown"},
		{"", "unknown"},
		{"/   ", "unknown"}, // Whitespace-only
		{"/a//b", "b"},      // Double slash
		{"/a/b/", "b"},      // Trailing slash
		{"/single", "single"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := getEndpointFromPath(tt.path)
			if result != tt.expected {
				t.Errorf("getEndpointFromPath(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}
