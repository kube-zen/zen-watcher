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
	"net"
	"net/http"
	"os"
	"strings"

	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
)

// WebhookAuth handles authentication for webhook endpoints
type WebhookAuth struct {
	// Token-based authentication
	tokenEnabled bool
	token        string

	// IP allowlist
	ipAllowlistEnabled bool
	allowedIPs         []string

	// Trusted proxy CIDRs (for X-Forwarded-For/X-Real-IP header validation)
	trustedProxyCIDRs []*net.IPNet

	// mTLS (future)
	// nolint:unused // Kept for future use
	mtlsEnabled bool
}

// NewWebhookAuth creates a new webhook authenticator from environment variables
func NewWebhookAuth() *WebhookAuth {
	auth := &WebhookAuth{}

	// Token authentication
	if token := os.Getenv("WEBHOOK_AUTH_TOKEN"); token != "" {
		auth.tokenEnabled = true
		auth.token = token
		logger := sdklog.NewLogger("zen-watcher-server")
		logger.Info("Webhook token authentication enabled",
			sdklog.Operation("auth_init"))
	}

	// IP allowlist
	if allowlist := os.Getenv("WEBHOOK_ALLOWED_IPS"); allowlist != "" {
		auth.ipAllowlistEnabled = true
		auth.allowedIPs = strings.Split(allowlist, ",")
		for i, ip := range auth.allowedIPs {
			auth.allowedIPs[i] = strings.TrimSpace(ip)
		}
		logger := sdklog.NewLogger("zen-watcher-server")
		logger.Info("Webhook IP allowlist enabled",
			sdklog.Operation("auth_init"),
			sdklog.Strings("allowed_ips", auth.allowedIPs))
	}

	// Trusted proxy CIDRs
	if trustedCIDRsStr := os.Getenv("SERVER_TRUSTED_PROXY_CIDRS"); trustedCIDRsStr != "" {
		cidrs := strings.Split(trustedCIDRsStr, ",")
		auth.trustedProxyCIDRs = make([]*net.IPNet, 0, len(cidrs))
		for _, cidrStr := range cidrs {
			cidrStr = strings.TrimSpace(cidrStr)
			if cidrStr == "" {
				continue
			}
			_, ipNet, err := net.ParseCIDR(cidrStr)
			if err != nil {
				logger := sdklog.NewLogger("zen-watcher-server")
				logger.Warn("Invalid CIDR in SERVER_TRUSTED_PROXY_CIDRS, ignoring",
					sdklog.Operation("auth_init"),
					sdklog.String("cidr", cidrStr),
					sdklog.Error(err))
				continue
			}
			auth.trustedProxyCIDRs = append(auth.trustedProxyCIDRs, ipNet)
		}
		if len(auth.trustedProxyCIDRs) > 0 {
			logger := sdklog.NewLogger("zen-watcher-server")
			logger.Info("Trusted proxy CIDRs configured",
				sdklog.Operation("auth_init"),
				sdklog.Int("count", len(auth.trustedProxyCIDRs)))
		}
	}

	return auth
}

// GetTrustedProxyCIDRs returns the trusted proxy CIDRs
func (a *WebhookAuth) GetTrustedProxyCIDRs() []*net.IPNet {
	return a.trustedProxyCIDRs
}

// Authenticate validates the request
func (a *WebhookAuth) Authenticate(r *http.Request) bool {
	// Token authentication
	if a.tokenEnabled {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			logger := sdklog.NewLogger("zen-watcher-server")
			logger.Warn("Webhook request rejected: missing Authorization header",
				sdklog.Operation("auth_validate"),
				sdklog.String("reason", "missing_auth_header"),
				sdklog.String("remote_addr", r.RemoteAddr),
				sdklog.String("path", r.URL.Path))
			return false
		}

		// Support "Bearer <token>" or just "<token>"
		token := strings.TrimPrefix(authHeader, "Bearer ")
		token = strings.TrimSpace(token)

		if subtle.ConstantTimeCompare([]byte(token), []byte(a.token)) != 1 {
			logger := sdklog.NewLogger("zen-watcher-server")
			logger.Warn("Webhook request rejected: invalid token",
				sdklog.Operation("auth_validate"),
				sdklog.String("reason", "invalid_token"),
				sdklog.String("remote_addr", r.RemoteAddr),
				sdklog.String("path", r.URL.Path))
			return false
		}
	}

	// IP allowlist
	if a.ipAllowlistEnabled {
		clientIP := getClientIP(r, a.trustedProxyCIDRs)
		allowed := false
		for _, allowedIP := range a.allowedIPs {
			if clientIP == allowedIP || strings.HasPrefix(clientIP, allowedIP+":") {
				allowed = true
				break
			}
		}
		if !allowed {
			logger := sdklog.NewLogger("zen-watcher-server")
			logger.Warn("Webhook request rejected: unauthorized IP",
				sdklog.Operation("auth_validate"),
				sdklog.String("reason", "ip_not_allowed"),
				sdklog.String("client_ip", clientIP),
				sdklog.String("path", r.URL.Path),
				sdklog.Strings("allowed_ips", a.allowedIPs))
			return false
		}
	}

	return true
}

// getClientIP extracts the client IP from the request
// Only trusts X-Forwarded-For/X-Real-IP headers when RemoteAddr is from a trusted proxy CIDR
func getClientIP(r *http.Request, trustedProxyCIDRs []*net.IPNet) string {
	// Extract IP from RemoteAddr
	remoteAddr := r.RemoteAddr
	var remoteIP net.IP
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		remoteIP = net.ParseIP(host)
	} else {
		// Try parsing as IP without port
		remoteIP = net.ParseIP(remoteAddr)
	}

	// Check if RemoteAddr is from a trusted proxy
	isTrustedProxy := false
	if remoteIP != nil {
		for _, cidr := range trustedProxyCIDRs {
			if cidr.Contains(remoteIP) {
				isTrustedProxy = true
				break
			}
		}
	}

	// Only trust proxy headers if RemoteAddr is from a trusted proxy
	if isTrustedProxy {
		// Check X-Forwarded-For header (for proxies/load balancers)
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			ips := strings.Split(xff, ",")
			if len(ips) > 0 {
				// Take the first (original client) IP
				clientIP := strings.TrimSpace(ips[0])
				if clientIP != "" {
					return clientIP
				}
			}
		}

		// Check X-Real-IP header
		if xri := r.Header.Get("X-Real-IP"); xri != "" {
			clientIP := strings.TrimSpace(xri)
			if clientIP != "" {
				return clientIP
			}
		}
	}

	// Fall back to RemoteAddr (untrusted proxy or no proxy headers)
	if remoteIP != nil {
		return remoteIP.String()
	}
	// Fallback: return RemoteAddr as-is if parsing failed
	if idx := strings.LastIndex(remoteAddr, ":"); idx != -1 {
		return remoteAddr[:idx]
	}
	return remoteAddr
}

// RequireAuth middleware function
func (a *WebhookAuth) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !a.Authenticate(r) {
			logger := sdklog.NewLogger("zen-watcher-server")
			logger.Debug("Webhook request authentication failed",
				sdklog.Operation("auth_middleware"),
				sdklog.String("reason", "authentication_failed"),
				sdklog.String("path", r.URL.Path),
				sdklog.String("remote_addr", r.RemoteAddr))
			w.WriteHeader(http.StatusUnauthorized)
			if _, err := w.Write([]byte(`{"error":"unauthorized"}`)); err != nil {
				logger.Warn("Failed to write authentication error response",
					sdklog.Operation("auth_middleware"),
					sdklog.Error(err))
			}
			return
		}
		logger := sdklog.NewLogger("zen-watcher-server")
		logger.Debug("Webhook request authentication successful",
			sdklog.Operation("auth_middleware"),
			sdklog.String("path", r.URL.Path),
			sdklog.String("remote_addr", r.RemoteAddr))
		next(w, r)
	}
}
