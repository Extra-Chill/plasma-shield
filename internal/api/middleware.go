package api

import (
	"net/http"
	"strings"
)

// AuthConfig holds authentication configuration.
type AuthConfig struct {
	ManagementToken string
	AgentToken      string
}

// ManagementAuth middleware validates the management bearer token.
func ManagementAuth(cfg *AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractBearerToken(r)
			if token == "" {
				writeError(w, http.StatusUnauthorized, "missing authorization token")
				return
			}
			if token != cfg.ManagementToken {
				writeError(w, http.StatusForbidden, "invalid management token")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// AgentAuth middleware validates the agent bearer token.
func AgentAuth(cfg *AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractBearerToken(r)
			if token == "" {
				writeError(w, http.StatusUnauthorized, "missing authorization token")
				return
			}
			if token != cfg.AgentToken {
				writeError(w, http.StatusForbidden, "invalid agent token")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// extractBearerToken extracts the bearer token from the Authorization header.
func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return parts[1]
}

// JSONContentType middleware sets the Content-Type header to application/json.
func JSONContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

// RequestLogger middleware logs incoming requests.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Basic logging - could be enhanced with structured logging
		next.ServeHTTP(w, r)
	})
}
