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

// Package e2e provides mock webhook servers for E2E tests
// H046: Local mocks for Slack, Datadog, PagerDuty, etc. (no external dependencies)

package e2e

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"
)

// MockWebhookServer provides a local HTTP server that mocks external webhook endpoints
type MockWebhookServer struct {
	server       *httptest.Server
	mu           sync.RWMutex
	received     []WebhookRequest
	responseCode int
	responseBody string
	delay        time.Duration
}

// WebhookRequest represents a received webhook request
type WebhookRequest struct {
	Method  string
	Path    string
	Headers map[string]string
	Body    []byte
	Time    time.Time
}

// NewMockWebhookServer creates a new mock webhook server
func NewMockWebhookServer() *MockWebhookServer {
	m := &MockWebhookServer{
		received:     make([]WebhookRequest, 0),
		responseCode: http.StatusOK,
		responseBody: `{"status":"ok"}`,
	}

	mux := http.NewServeMux()

	// Slack webhook endpoint
	mux.HandleFunc("/slack/webhook", m.handleSlack)

	// Datadog webhook endpoint
	mux.HandleFunc("/datadog/events", m.handleDatadog)

	// PagerDuty webhook endpoint
	mux.HandleFunc("/pagerduty/events", m.handlePagerDuty)

	// Generic webhook endpoint
	mux.HandleFunc("/webhook", m.handleGeneric)

	// S3-compatible endpoint (stub)
	mux.HandleFunc("/s3/", m.handleS3)

	// Terraform webhook
	mux.HandleFunc("/terraform/webhook", m.handleTerraform)

	// Stripe webhook
	mux.HandleFunc("/stripe/webhook", m.handleStripe)

	// GitHub webhook
	mux.HandleFunc("/github/webhook", m.handleGitHub)

	m.server = httptest.NewServer(mux)
	return m
}

// URL returns the base URL of the mock server
func (m *MockWebhookServer) URL() string {
	return m.server.URL
}

// Close shuts down the mock server
func (m *MockWebhookServer) Close() {
	m.server.Close()
}

// SetResponse configures the response for all endpoints
func (m *MockWebhookServer) SetResponse(code int, body string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responseCode = code
	m.responseBody = body
}

// SetDelay sets a delay before responding (for timeout testing)
func (m *MockWebhookServer) SetDelay(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.delay = d
}

// GetReceived returns all received webhook requests
func (m *MockWebhookServer) GetReceived() []WebhookRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]WebhookRequest, len(m.received))
	copy(result, m.received)
	return result
}

// ClearReceived clears the received requests list
func (m *MockWebhookServer) ClearReceived() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.received = make([]WebhookRequest, 0)
}

// recordRequest records a received request
func (m *MockWebhookServer) recordRequest(r *http.Request, body []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()

	headers := make(map[string]string)
	for k, v := range r.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	m.received = append(m.received, WebhookRequest{
		Method:  r.Method,
		Path:    r.URL.Path,
		Headers: headers,
		Body:    body,
		Time:    time.Now(),
	})
}

// writeResponse writes the configured response
func (m *MockWebhookServer) writeResponse(w http.ResponseWriter) {
	m.mu.RLock()
	code := m.responseCode
	body := m.responseBody
	delay := m.delay
	m.mu.RUnlock()

	if delay > 0 {
		time.Sleep(delay)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	fmt.Fprintf(w, "%s", body)
}

// handleSlack handles Slack webhook requests
func (m *MockWebhookServer) handleSlack(w http.ResponseWriter, r *http.Request) {
	body, _ := readRequestBody(r)
	m.recordRequest(r, body)
	m.writeResponse(w)
}

// handleDatadog handles Datadog webhook requests
func (m *MockWebhookServer) handleDatadog(w http.ResponseWriter, r *http.Request) {
	body, _ := readRequestBody(r)
	m.recordRequest(r, body)
	m.writeResponse(w)
}

// handlePagerDuty handles PagerDuty webhook requests
func (m *MockWebhookServer) handlePagerDuty(w http.ResponseWriter, r *http.Request) {
	body, _ := readRequestBody(r)
	m.recordRequest(r, body)
	m.writeResponse(w)
}

// handleGeneric handles generic webhook requests
func (m *MockWebhookServer) handleGeneric(w http.ResponseWriter, r *http.Request) {
	body, _ := readRequestBody(r)
	m.recordRequest(r, body)
	m.writeResponse(w)
}

// handleS3 handles S3-compatible stub requests
func (m *MockWebhookServer) handleS3(w http.ResponseWriter, r *http.Request) {
	body, _ := readRequestBody(r)
	m.recordRequest(r, body)

	// S3-compatible stub response
	m.mu.RLock()
	code := m.responseCode
	m.mu.RUnlock()

	if code == http.StatusOK {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?><PutObjectResult><ETag>"mock-etag"</ETag></PutObjectResult>`)
	} else {
		m.writeResponse(w)
	}
}

// handleTerraform handles Terraform webhook requests
func (m *MockWebhookServer) handleTerraform(w http.ResponseWriter, r *http.Request) {
	body, _ := readRequestBody(r)
	m.recordRequest(r, body)
	m.writeResponse(w)
}

// handleStripe handles Stripe webhook requests
func (m *MockWebhookServer) handleStripe(w http.ResponseWriter, r *http.Request) {
	body, _ := readRequestBody(r)
	m.recordRequest(r, body)
	m.writeResponse(w)
}

// handleGitHub handles GitHub webhook requests
func (m *MockWebhookServer) handleGitHub(w http.ResponseWriter, r *http.Request) {
	body, _ := readRequestBody(r)
	m.recordRequest(r, body)
	m.writeResponse(w)
}

// readRequestBody reads the request body as bytes
func readRequestBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	defer r.Body.Close()
	return io.ReadAll(r.Body)
}

// MockS3Server provides a local S3-compatible stub server
type MockS3Server struct {
	server  *httptest.Server
	mu      sync.RWMutex
	buckets map[string][]S3Object
}

// S3Object represents a stored S3 object
type S3Object struct {
	Key      string
	Body     []byte
	Metadata map[string]string
}

// NewMockS3Server creates a new mock S3 server
func NewMockS3Server() *MockS3Server {
	m := &MockS3Server{
		buckets: make(map[string][]S3Object),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", m.handleRequest)
	m.server = httptest.NewServer(mux)
	return m
}

// URL returns the base URL of the mock S3 server
func (m *MockS3Server) URL() string {
	return m.server.URL
}

// Close shuts down the mock S3 server
func (m *MockS3Server) Close() {
	m.server.Close()
}

// handleRequest handles S3-compatible requests
func (m *MockS3Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	// Simple S3 stub - accepts PUT requests and stores objects
	if r.Method == http.MethodPut || r.Method == http.MethodPost {
		body, _ := io.ReadAll(r.Body)
		r.Body.Close()

		// Extract bucket and key from path (simplified)
		path := r.URL.Path
		parts := splitS3Path(path)

		if len(parts) >= 2 {
			bucket := parts[0]
			key := parts[1]

			m.mu.Lock()
			if m.buckets[bucket] == nil {
				m.buckets[bucket] = make([]S3Object, 0)
			}
			m.buckets[bucket] = append(m.buckets[bucket], S3Object{
				Key:  key,
				Body: body,
			})
			m.mu.Unlock()

			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?><PutObjectResult><ETag>"mock-etag-%s"</ETag></PutObjectResult>`, key)
			return
		}
	}

	w.WriteHeader(http.StatusNotImplemented)
}

// splitS3Path splits an S3 path into bucket and key
func splitS3Path(path string) []string {
	// Remove leading /
	if len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}

	if path == "" {
		return []string{}
	}

	parts := strings.Split(path, "/")
	result := make([]string, 0)
	for _, part := range parts {
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}
