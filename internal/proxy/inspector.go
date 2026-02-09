// Package proxy provides the HTTP/HTTPS forward proxy implementation.
package proxy

import (
	"net/http"
	"strings"

	"github.com/Extra-Chill/plasma-shield/internal/rules"
)

// Inspector handles traffic inspection and rule checking.
type Inspector struct {
	engine *rules.Engine
}

// NewInspector creates a new traffic inspector.
func NewInspector(engine *rules.Engine) *Inspector {
	return &Inspector{
		engine: engine,
	}
}

// ExtractHost extracts the host/domain from an HTTP request.
// For CONNECT requests, it parses the host from the URL.
// For regular requests, it uses the Host header.
func (i *Inspector) ExtractHost(r *http.Request) string {
	host := r.Host
	if host == "" {
		host = r.URL.Host
	}
	// Strip port if present
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		// Check if this is an IPv6 address
		if !strings.Contains(host, "]") || strings.LastIndex(host, "]") < idx {
			host = host[:idx]
		}
	}
	return strings.ToLower(host)
}

// ExtractAgentToken extracts the agent token from request headers.
// Agents authenticate via X-Agent-Token header.
func (i *Inspector) ExtractAgentToken(r *http.Request) string {
	return r.Header.Get("X-Agent-Token")
}

// CheckDomain checks if a domain is allowed by the rule engine.
// Returns (allowed, reason).
func (i *Inspector) CheckDomain(domain string) (bool, string) {
	allowed, _, reason := i.engine.CheckDomain(domain)
	return allowed, reason
}

// CheckCommand checks if a command is allowed by the rule engine.
// Returns (allowed, reason).
func (i *Inspector) CheckCommand(command string) (bool, string) {
	allowed, _, reason := i.engine.CheckCommand(command)
	return allowed, reason
}
