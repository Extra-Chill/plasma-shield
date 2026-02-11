// Package proxy provides the HTTP/HTTPS proxy implementation.
package proxy

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Extra-Chill/plasma-shield/internal/fleet"
)

// ReverseHandler handles inbound requests and routes them to agents.
// This is the "inbound" half of the shield - external traffic to agents.
type ReverseHandler struct {
	fleet  *fleet.Manager
	tokens map[string]string // token -> tenant ID (for auth)
	client *http.Client
}

// NewReverseHandler creates a new reverse proxy handler.
func NewReverseHandler(fleetMgr *fleet.Manager) *ReverseHandler {
	return &ReverseHandler{
		fleet:  fleetMgr,
		tokens: make(map[string]string),
		client: &http.Client{
			Timeout: 60 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// RegisterToken registers an auth token for a tenant.
func (h *ReverseHandler) RegisterToken(token, tenantID string) {
	h.tokens[token] = tenantID
}

// ServeHTTP handles inbound requests.
// Routes: /agent/{agent-id}/* -> agent's webhook
func (h *ReverseHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Extract auth token
	token := extractBearerToken(r)
	if token == "" {
		h.jsonError(w, "Unauthorized: missing bearer token", http.StatusUnauthorized)
		return
	}

	// Validate token and get tenant
	tenantID, valid := h.tokens[token]
	if !valid {
		h.jsonError(w, "Unauthorized: invalid token", http.StatusUnauthorized)
		return
	}

	// Parse path: /agent/{agent-id}/...
	path := strings.TrimPrefix(r.URL.Path, "/")
	parts := strings.SplitN(path, "/", 3)

	if len(parts) < 2 || parts[0] != "agent" {
		h.jsonError(w, "Not found: use /agent/{agent-id}/...", http.StatusNotFound)
		return
	}

	agentID := parts[1]
	remainingPath := "/"
	if len(parts) > 2 {
		remainingPath = "/" + parts[2]
	}

	// Get tenant and check agent ownership
	tenant := h.fleet.GetTenant(tenantID)
	if tenant == nil {
		h.jsonError(w, "Forbidden: tenant not found", http.StatusForbidden)
		return
	}

	agent, exists := tenant.Agents[agentID]
	if !exists {
		h.jsonError(w, "Forbidden: agent not in your fleet", http.StatusForbidden)
		return
	}

	// Get agent's internal URL
	targetURL := agent.WebhookURL
	if targetURL == "" && agent.IP != "" {
		// Default to OpenClaw webhook port
		targetURL = "http://" + agent.IP + ":18789"
	}

	if targetURL == "" {
		h.jsonError(w, "Bad gateway: agent has no endpoint configured", http.StatusBadGateway)
		return
	}

	// Build target URL
	target, err := url.Parse(targetURL)
	if err != nil {
		h.jsonError(w, "Bad gateway: invalid agent URL", http.StatusBadGateway)
		return
	}
	target.Path = remainingPath
	target.RawQuery = r.URL.RawQuery

	// Get captain name for identity masking
	captainName := tenant.CaptainName
	if captainName == "" {
		captainName = tenantID // Fallback to tenant ID
	}

	// Log the request
	h.logRequest(tenantID, agentID, r.Method, remainingPath, "forward")

	// Forward request with identity masking
	h.forward(w, r, target.String(), captainName)
}

// forward proxies the request to the target URL with identity masking.
// The captainName is used to mask the true origin of the request.
func (h *ReverseHandler) forward(w http.ResponseWriter, r *http.Request, targetURL, captainName string) {
	// Create outgoing request
	outReq, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, r.Body)
	if err != nil {
		h.jsonError(w, "Internal error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Copy headers (except hop-by-hop, auth, and identity-revealing headers)
	for key, values := range r.Header {
		lower := strings.ToLower(key)
		// Skip hop-by-hop headers
		if lower == "authorization" || lower == "connection" ||
			lower == "keep-alive" || lower == "proxy-authenticate" ||
			lower == "proxy-authorization" || lower == "te" ||
			lower == "trailers" || lower == "transfer-encoding" ||
			lower == "upgrade" {
			continue
		}
		// Skip headers that could reveal origin identity
		if lower == "x-forwarded-for" || lower == "x-real-ip" ||
			lower == "x-originating-ip" || lower == "x-remote-ip" ||
			lower == "x-remote-addr" || lower == "x-client-ip" ||
			lower == "x-agent-id" || lower == "x-source-agent" {
			continue
		}
		for _, value := range values {
			outReq.Header.Add(key, value)
		}
	}

	// IDENTITY MASKING: Set headers that identify the request as coming from Captain
	// The agent will see this as a request from their Captain, not from another agent
	outReq.Header.Set("X-Captain", captainName)
	outReq.Header.Set("X-Forwarded-Proto", "https")
	outReq.Header.Set("X-Plasma-Shield", "true")
	// Note: We deliberately do NOT set X-Forwarded-For to hide the true origin

	// Make request
	resp, err := h.client.Do(outReq)
	if err != nil {
		h.jsonError(w, "Bad gateway: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// jsonError writes a JSON error response.
func (h *ReverseHandler) jsonError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

// logRequest logs an inbound request.
func (h *ReverseHandler) logRequest(tenantID, agentID, method, path, action string) {
	entry := map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"direction": "inbound",
		"tenant":    tenantID,
		"agent":     agentID,
		"method":    method,
		"path":      path,
		"action":    action,
	}
	data, _ := json.Marshal(entry)
	log.Println(string(data))
}

// extractBearerToken extracts the bearer token from Authorization header.
func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return ""
	}
	return parts[1]
}
