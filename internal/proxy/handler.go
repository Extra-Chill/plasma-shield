package proxy

import (
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"time"
)

// LogEntry represents a logged request.
type LogEntry struct {
	Timestamp  time.Time `json:"timestamp"`
	SourceIP   string    `json:"source_ip,omitempty"`
	AgentID    string    `json:"agent_id,omitempty"`
	AgentToken string    `json:"agent_token,omitempty"`
	Domain     string    `json:"domain"`
	Method     string    `json:"method"`
	Action     string    `json:"action"` // "allow" or "block"
	Reason     string    `json:"reason,omitempty"`
}

// AgentRegistry validates agent source IPs.
// Implement this interface to control which IPs can use the proxy.
type AgentRegistry interface {
	// ValidateAgentIP checks if the IP belongs to a registered agent.
	// Returns agent ID and tier if valid, empty strings if not.
	ValidateAgentIP(ip string) (agentID string, tier string, valid bool)
}

// Handler is the main proxy HTTP handler.
type Handler struct {
	inspector *Inspector
	registry  AgentRegistry
	client    *http.Client
}

// HandlerOption configures the Handler.
type HandlerOption func(*Handler)

// WithAgentRegistry sets the agent registry for IP validation.
func WithAgentRegistry(r AgentRegistry) HandlerOption {
	return func(h *Handler) {
		h.registry = r
	}
}

// NewHandler creates a new proxy handler.
func NewHandler(inspector *Inspector, opts ...HandlerOption) *Handler {
	h := &Handler{
		inspector: inspector,
		client: &http.Client{
			Timeout: 30 * time.Second,
			// Don't follow redirects - let the client handle them
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// extractClientIP extracts the real client IP from the request.
func extractClientIP(r *http.Request) string {
	// RemoteAddr is "IP:port"
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// ServeHTTP handles incoming proxy requests.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Validate source IP against agent registry
	sourceIP := extractClientIP(r)
	agentID, tier, valid := h.validateSource(sourceIP)

	if !valid {
		log.Printf(`{"timestamp":"%s","source_ip":"%s","action":"reject","reason":"unregistered agent IP"}`,
			time.Now().UTC().Format(time.RFC3339), sourceIP)
		http.Error(w, "Forbidden: unregistered agent", http.StatusForbidden)
		return
	}

	// Store agent info in context for downstream use
	r.Header.Set("X-Agent-ID", agentID)
	r.Header.Set("X-Agent-Tier", tier)

	if r.Method == http.MethodConnect {
		h.handleConnect(w, r, sourceIP, agentID)
		return
	}
	h.handleHTTP(w, r, sourceIP, agentID)
}

// validateSource checks if the source IP is a registered agent.
func (h *Handler) validateSource(ip string) (agentID, tier string, valid bool) {
	if h.registry == nil {
		// No registry configured - allow all (backwards compatibility)
		return "unknown", "unknown", true
	}
	return h.registry.ValidateAgentIP(ip)
}

// handleHTTP handles regular HTTP proxy requests.
func (h *Handler) handleHTTP(w http.ResponseWriter, r *http.Request, sourceIP, agentID string) {
	domain := h.inspector.ExtractHost(r)
	agentToken := h.inspector.ExtractAgentToken(r)

	// Check if request should be blocked (mode-aware)
	shouldBlock, ruleMatched, reason := h.inspector.CheckRequest(r)
	action := "allow"
	if shouldBlock {
		action = "block"
	} else if ruleMatched {
		action = "audit" // Would have blocked, but in audit mode
	}

	h.logRequestFull(sourceIP, agentID, agentToken, domain, r.Method, action, reason)

	if shouldBlock {
		http.Error(w, "Blocked by Plasma Shield: "+reason, http.StatusForbidden)
		return
	}

	// Create outgoing request
	outReq, err := http.NewRequestWithContext(r.Context(), r.Method, r.URL.String(), r.Body)
	if err != nil {
		http.Error(w, "Failed to create request: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Copy headers, but remove proxy-specific ones
	copyHeaders(outReq.Header, r.Header)
	outReq.Header.Del("Proxy-Connection")
	outReq.Header.Del("X-Agent-Token") // Don't leak agent token to upstream

	// Forward the request
	resp, err := h.client.Do(outReq)
	if err != nil {
		http.Error(w, "Upstream error: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	copyHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)

	// Copy response body
	io.Copy(w, resp.Body)
}

// handleConnect handles HTTPS CONNECT tunnels.
func (h *Handler) handleConnect(w http.ResponseWriter, r *http.Request, sourceIP, agentID string) {
	domain := h.inspector.ExtractHost(r)
	agentToken := h.inspector.ExtractAgentToken(r)

	// Check if request should be blocked (mode-aware)
	shouldBlock, ruleMatched, reason := h.inspector.CheckRequest(r)
	action := "allow"
	if shouldBlock {
		action = "block"
	} else if ruleMatched {
		action = "audit"
	}

	h.logRequestFull(sourceIP, agentID, agentToken, domain, "CONNECT", action, reason)

	if shouldBlock {
		http.Error(w, "Blocked by Plasma Shield: "+reason, http.StatusForbidden)
		return
	}

	// Connect to the target host
	targetHost := r.Host
	if r.URL.Host != "" {
		targetHost = r.URL.Host
	}

	targetConn, err := net.DialTimeout("tcp", targetHost, 10*time.Second)
	if err != nil {
		http.Error(w, "Failed to connect to target: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer targetConn.Close()

	// Hijack the client connection
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, "Failed to hijack connection: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer clientConn.Close()

	// Send 200 Connection Established
	clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	// Tunnel data bidirectionally
	done := make(chan struct{}, 2)

	go func() {
		io.Copy(targetConn, clientConn)
		done <- struct{}{}
	}()

	go func() {
		io.Copy(clientConn, targetConn)
		done <- struct{}{}
	}()

	// Wait for either direction to finish
	<-done
}

// logRequestFull logs a request with full context.
func (h *Handler) logRequestFull(sourceIP, agentID, agentToken, domain, method, action, reason string) {
	entry := LogEntry{
		Timestamp:  time.Now().UTC(),
		SourceIP:   sourceIP,
		AgentID:    agentID,
		AgentToken: agentToken,
		Domain:     domain,
		Method:     method,
		Action:     action,
		Reason:     reason,
	}
	data, _ := json.Marshal(entry)
	log.Println(string(data))
}

// logRequest logs a request to stdout (legacy, no source info).
func (h *Handler) logRequest(agentToken, domain, method, action, reason string) {
	h.logRequestFull("", "", agentToken, domain, method, action, reason)
}

// copyHeaders copies HTTP headers from src to dst.
func copyHeaders(dst, src http.Header) {
	for key, values := range src {
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

// ExecCheckRequest is the request body for /exec/check.
type ExecCheckRequest struct {
	Command    string `json:"command"`
	AgentToken string `json:"agent_token,omitempty"`
}

// ExecCheckResponse is the response body for /exec/check.
type ExecCheckResponse struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}

// ExecCheckHandler handles POST /exec/check requests.
type ExecCheckHandler struct {
	inspector *Inspector
}

// NewExecCheckHandler creates a new exec check handler.
func NewExecCheckHandler(inspector *Inspector) *ExecCheckHandler {
	return &ExecCheckHandler{
		inspector: inspector,
	}
}

// ServeHTTP handles exec check requests.
func (h *ExecCheckHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ExecCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	allowed, reason := h.inspector.CheckCommand(req.Command)

	action := "allow"
	if !allowed {
		action = "block"
	}

	// Log the exec check
	entry := LogEntry{
		Timestamp:  time.Now().UTC(),
		AgentToken: req.AgentToken,
		Domain:     "exec",
		Method:     "EXEC",
		Action:     action,
		Reason:     reason,
	}
	data, _ := json.Marshal(entry)
	log.Println(string(data))

	resp := ExecCheckResponse{
		Allowed: allowed,
		Reason:  reason,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
