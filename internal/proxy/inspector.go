// Package proxy provides the HTTP/HTTPS forward proxy implementation.
package proxy

import (
	"log"
	"net/http"
	"strings"

	"github.com/Extra-Chill/plasma-shield/internal/mode"
	"github.com/Extra-Chill/plasma-shield/internal/rules"
)

// Inspector handles traffic inspection and rule checking.
type Inspector struct {
	engine      *rules.Engine
	modeManager *mode.Manager
}

// NewInspector creates a new traffic inspector.
func NewInspector(engine *rules.Engine, modeManager *mode.Manager) *Inspector {
	return &Inspector{
		engine:      engine,
		modeManager: modeManager,
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

// CheckRequest checks if a request should be blocked.
// Returns (shouldBlock, ruleMatched, reason).
// Respects the current mode (audit = never block, lockdown = always block).
func (i *Inspector) CheckRequest(r *http.Request) (shouldBlock bool, ruleMatched bool, reason string) {
	agentID := i.ExtractAgentToken(r)
	host := i.ExtractHost(r)

	// Check if domain matches any blocking rule
	allowed, matchedRule, ruleReason := i.engine.CheckDomain(host)
	ruleMatched = !allowed

	// Determine if we should actually block based on mode
	shouldBlock = i.modeManager.ShouldBlock(agentID, ruleMatched)

	// Build reason string
	ruleID := ""
	if matchedRule != nil {
		ruleID = matchedRule.ID
	}
	if ruleMatched {
		reason = ruleReason
		if ruleID != "" {
			reason = ruleID + ": " + reason
		}
	}

	// Log the decision
	modeStr := string(i.modeManager.AgentMode(agentID))
	if ruleMatched {
		if shouldBlock {
			log.Printf("[%s] BLOCK %s (agent=%s, rule=%s)", modeStr, host, agentID, ruleID)
		} else {
			log.Printf("[%s] AUDIT %s (agent=%s, would block: rule=%s)", modeStr, host, agentID, ruleID)
		}
	}

	return shouldBlock, ruleMatched, reason
}

// CheckDomain checks if a domain is allowed by the rule engine.
// Returns (allowed, reason).
// Note: This does not respect mode - use CheckRequest for full mode-aware checking.
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

// Mode returns the current mode for an agent.
func (i *Inspector) Mode(agentID string) mode.Mode {
	return i.modeManager.AgentMode(agentID)
}

// IsLockdown returns whether the agent is in lockdown mode.
func (i *Inspector) IsLockdown(agentID string) bool {
	return i.modeManager.AgentMode(agentID) == mode.Lockdown
}
