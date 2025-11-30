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
			logger.Warn("Webhook request missing Authorization header",
				logger.Fields{
					Component: "server",
					Operation: "auth_validate",
					Additional: map[string]interface{}{
						"remote_addr": r.RemoteAddr,
					},
				})
			return false
		}

		// Support "Bearer <token>" or just "<token>"
		token := strings.TrimPrefix(authHeader, "Bearer ")
		token = strings.TrimSpace(token)

		if subtle.ConstantTimeCompare([]byte(token), []byte(a.token)) != 1 {
			logger.Warn("Webhook request invalid token",
				logger.Fields{
					Component: "server",
					Operation: "auth_validate",
					Additional: map[string]interface{}{
						"remote_addr": r.RemoteAddr,
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
			logger.Warn("Webhook request from unauthorized IP",
				logger.Fields{
					Component: "server",
					Operation: "auth_validate",
					Additional: map[string]interface{}{
						"client_ip": clientIP,
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
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}
		next(w, r)
	}
}
