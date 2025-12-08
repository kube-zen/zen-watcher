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
	"crypto/subtle"
	"net/http"
	"os"
	"strings"

	"github.com/kube-zen/zen-watcher/pkg/logger"
)

// WebhookAuth handles authentication for webhook endpoints
type WebhookAuth struct {
	// Token-based authentication
	tokenEnabled bool
	token        string

	// IP allowlist
	ipAllowlistEnabled bool
	allowedIPs         []string

	// mTLS (future)
	mtlsEnabled bool
}

// NewWebhookAuth creates a new webhook authenticator from environment variables
func NewWebhookAuth() *WebhookAuth {
	auth := &WebhookAuth{}

	// Token authentication
	if token := os.Getenv("WEBHOOK_AUTH_TOKEN"); token != "" {
		auth.tokenEnabled = true
		auth.token = token
		logger.Info("Webhook token authentication enabled",
			logger.Fields{
				Component: "server",
				Operation: "auth_init",
			})
	}

	// IP allowlist
	if allowlist := os.Getenv("WEBHOOK_ALLOWED_IPS"); allowlist != "" {
		auth.ipAllowlistEnabled = true
		auth.allowedIPs = strings.Split(allowlist, ",")
		for i, ip := range auth.allowedIPs {
			auth.allowedIPs[i] = strings.TrimSpace(ip)
		}
		logger.Info("Webhook IP allowlist enabled",
			logger.Fields{
				Component: "server",
				Operation: "auth_init",
				Additional: map[string]interface{}{
					"allowed_ips": auth.allowedIPs,
				},
			})
	}

	return auth
}

// Authenticate validates the request
func (a *WebhookAuth) Authenticate(r *http.Request) bool {
	// Token authentication
	if a.tokenEnabled {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			logger.Warn("Webhook request rejected: missing Authorization header",
				logger.Fields{
					Component: "server",
					Operation: "auth_validate",
					Reason:    "missing_auth_header",
					Additional: map[string]interface{}{
						"remote_addr": r.RemoteAddr,
						"path":        r.URL.Path,
					},
				})
			return false
		}

		// Support "Bearer <token>" or just "<token>"
		token := strings.TrimPrefix(authHeader, "Bearer ")
		token = strings.TrimSpace(token)

		if subtle.ConstantTimeCompare([]byte(token), []byte(a.token)) != 1 {
			logger.Warn("Webhook request rejected: invalid token",
				logger.Fields{
					Component: "server",
					Operation: "auth_validate",
					Reason:    "invalid_token",
					Additional: map[string]interface{}{
						"remote_addr": r.RemoteAddr,
						"path":        r.URL.Path,
					},
				})
			return false
		}
	}

	// IP allowlist
	if a.ipAllowlistEnabled {
		clientIP := getClientIP(r)
		allowed := false
		for _, allowedIP := range a.allowedIPs {
			if clientIP == allowedIP || strings.HasPrefix(clientIP, allowedIP+":") {
				allowed = true
				break
			}
		}
		if !allowed {
			logger.Warn("Webhook request rejected: unauthorized IP",
				logger.Fields{
					Component: "server",
					Operation: "auth_validate",
					Reason:    "ip_not_allowed",
					Additional: map[string]interface{}{
						"client_ip":   clientIP,
						"path":        r.URL.Path,
						"allowed_ips": a.allowedIPs,
					},
				})
			return false
		}
	}

	return true
}

// getClientIP extracts the client IP from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (for proxies/load balancers)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

// RequireAuth middleware function
func (a *WebhookAuth) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !a.Authenticate(r) {
			logger.Debug("Webhook request authentication failed",
				logger.Fields{
					Component: "server",
					Operation: "auth_middleware",
					Reason:    "authentication_failed",
					Additional: map[string]interface{}{
						"path":        r.URL.Path,
						"remote_addr": r.RemoteAddr,
					},
				})
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}
		logger.Debug("Webhook request authentication successful",
			logger.Fields{
				Component: "server",
				Operation: "auth_middleware",
				Additional: map[string]interface{}{
					"path":        r.URL.Path,
					"remote_addr": r.RemoteAddr,
				},
			})
		next(w, r)
	}
}
