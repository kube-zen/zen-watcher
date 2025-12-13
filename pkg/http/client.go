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

package http

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/logger"
	"golang.org/x/time/rate"
)

// HTTPClientConfig holds configuration for HTTP clients
type HTTPClientConfig struct {
	Timeout               time.Duration
	MaxIdleConns          int
	MaxConnsPerHost       int
	IdleConnTimeout       time.Duration
	DisableKeepAlives     bool
	TLSHandshakeTimeout   time.Duration
	ResponseHeaderTimeout time.Duration
	ExpectContinueTimeout time.Duration
	// TLS configuration
	TLSInsecureSkipVerify bool
	TLSClientConfig       *tls.Config
	// Rate limiting
	RateLimitEnabled bool
	RateLimitRPS     float64
	RateLimitBurst   int
	// Logging
	LoggingEnabled bool
	ServiceName    string
}

// DefaultHTTPClientConfig returns a default HTTP client configuration
func DefaultHTTPClientConfig() *HTTPClientConfig {
	// Get defaults from environment variables
	maxIdleConns := getEnvInt("HTTP_MAX_IDLE_CONNS", 100)
	maxConnsPerHost := getEnvInt("HTTP_MAX_CONNS_PER_HOST", 10)
	idleConnTimeout := getEnvDuration("HTTP_IDLE_CONN_TIMEOUT", 90*time.Second)
	timeout := getEnvDuration("HTTP_TIMEOUT", 30*time.Second)

	return &HTTPClientConfig{
		Timeout:               timeout,
		MaxIdleConns:          maxIdleConns,
		MaxConnsPerHost:       maxConnsPerHost,
		IdleConnTimeout:       idleConnTimeout,
		DisableKeepAlives:     false,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSInsecureSkipVerify: false,
		RateLimitEnabled:      false,
		RateLimitRPS:          10.0,
		RateLimitBurst:        10,
		LoggingEnabled:        true,
		ServiceName:           "zen-watcher",
	}
}

// HardenedHTTPClient wraps http.Client with connection pooling, retry logic, and rate limiting
type HardenedHTTPClient struct {
	client    *http.Client
	config    *HTTPClientConfig
	limiter   *rate.Limiter
	transport *http.Transport
	service   string
}

// NewHardenedHTTPClient creates a new hardened HTTP client with proper defaults
func NewHardenedHTTPClient(config *HTTPClientConfig) *HardenedHTTPClient {
	if config == nil {
		config = DefaultHTTPClientConfig()
	}

	transport := &http.Transport{
		MaxIdleConns:          config.MaxIdleConns,
		MaxConnsPerHost:       config.MaxConnsPerHost,
		IdleConnTimeout:       config.IdleConnTimeout,
		DisableKeepAlives:     config.DisableKeepAlives,
		TLSHandshakeTimeout:   config.TLSHandshakeTimeout,
		ResponseHeaderTimeout: config.ResponseHeaderTimeout,
		ExpectContinueTimeout: config.ExpectContinueTimeout,
	}

	// Configure TLS
	if config.TLSClientConfig != nil {
		transport.TLSClientConfig = config.TLSClientConfig
	} else if config.TLSInsecureSkipVerify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   config.Timeout,
	}

	var limiter *rate.Limiter
	if config.RateLimitEnabled {
		limiter = rate.NewLimiter(rate.Limit(config.RateLimitRPS), config.RateLimitBurst)
	}

	serviceName := config.ServiceName
	if serviceName == "" {
		serviceName = "zen-watcher"
	}

	return &HardenedHTTPClient{
		client:    client,
		config:    config,
		limiter:   limiter,
		transport: transport,
		service:   serviceName,
	}
}

// Do performs an HTTP request with rate limiting and logging
func (c *HardenedHTTPClient) Do(req *http.Request) (*http.Response, error) {
	// Apply rate limiting if enabled
	if c.limiter != nil {
		ctx := req.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limit wait failed: %w", err)
		}
	}

	// Log request if enabled
	if c.config.LoggingEnabled {
		logger.Debug("HTTP request",
			logger.Fields{
				Component: "http_client",
				Operation: "http_request",
				Additional: map[string]interface{}{
					"method":  req.Method,
					"url":     req.URL.String(),
					"service": c.service,
				},
			})
	}

	start := time.Now()
	resp, err := c.client.Do(req)
	duration := time.Since(start)

	if err != nil {
		if c.config.LoggingEnabled {
			logger.Warn("HTTP request failed",
				logger.Fields{
					Component: "http_client",
					Operation: "http_request",
					Error:     err,
					Additional: map[string]interface{}{
						"method":   req.Method,
						"url":      req.URL.String(),
						"duration": duration.String(),
						"service":  c.service,
					},
				})
		}
		return nil, err
	}

	if c.config.LoggingEnabled {
		logger.Debug("HTTP response",
			logger.Fields{
				Component: "http_client",
				Operation: "http_response",
				Additional: map[string]interface{}{
					"method":      req.Method,
					"url":         req.URL.String(),
					"status_code": resp.StatusCode,
					"duration":    duration.String(),
					"service":     c.service,
				},
			})
	}

	return resp, nil
}

// Get performs a GET request
func (c *HardenedHTTPClient) Get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}

// Post performs a POST request
func (c *HardenedHTTPClient) Post(ctx context.Context, url string, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	return c.Do(req)
}

// CloseIdleConnections closes all idle connections
func (c *HardenedHTTPClient) CloseIdleConnections() {
	if c.transport != nil {
		c.transport.CloseIdleConnections()
	}
}

// Helper functions

func getEnvInt(key string, defaultValue int) int {
	if val := os.Getenv(key); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			return duration
		}
	}
	return defaultValue
}
