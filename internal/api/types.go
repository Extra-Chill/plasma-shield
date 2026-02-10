// Package api provides the REST API for Plasma Shield management.
package api

import (
	"time"

	"github.com/Extra-Chill/plasma-shield/internal/bastion"
)

// StatusResponse is the response for GET /status.
type StatusResponse struct {
	Status        string    `json:"status"`
	Version       string    `json:"version"`
	Uptime        string    `json:"uptime"`
	StartedAt     time.Time `json:"started_at"`
	AgentCount    int       `json:"agent_count"`
	RuleCount     int       `json:"rule_count"`
	RequestsTotal int64     `json:"requests_total"`
	BlockedTotal  int64     `json:"blocked_total"`
}

// Agent represents a registered agent.
type Agent struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	IP        string    `json:"ip"`
	Status    string    `json:"status"` // "active", "paused", "killed"
	LastSeen  time.Time `json:"last_seen"`
	CreatedAt time.Time `json:"created_at"`
}

// AgentListResponse is the response for GET /agents.
type AgentListResponse struct {
	Agents []Agent `json:"agents"`
	Total  int     `json:"total"`
}

// AgentActionResponse is the response for agent actions (pause/kill/resume).
type AgentActionResponse struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// Rule represents a filtering rule.
type Rule struct {
	ID          string    `json:"id"`
	Pattern     string    `json:"pattern,omitempty"`
	Domain      string    `json:"domain,omitempty"`
	Action      string    `json:"action"` // "block" or "allow"
	Description string    `json:"description,omitempty"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
}

// RuleListResponse is the response for GET /rules.
type RuleListResponse struct {
	Rules []Rule `json:"rules"`
	Total int    `json:"total"`
}

// CreateRuleRequest is the request body for POST /rules.
type CreateRuleRequest struct {
	Pattern     string `json:"pattern,omitempty"`
	Domain      string `json:"domain,omitempty"`
	Action      string `json:"action"`
	Description string `json:"description,omitempty"`
	Enabled     bool   `json:"enabled"`
}

// CreateRuleResponse is the response for POST /rules.
type CreateRuleResponse struct {
	Rule    Rule   `json:"rule"`
	Message string `json:"message"`
}

// DeleteRuleResponse is the response for DELETE /rules/{id}.
type DeleteRuleResponse struct {
	ID      string `json:"id"`
	Message string `json:"message"`
}

// LogEntry represents a single traffic log entry.
type LogEntry struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	AgentID   string    `json:"agent_id"`
	Type      string    `json:"type"` // "command", "http", "dns"
	Request   string    `json:"request"`
	Action    string    `json:"action"` // "allowed", "blocked"
	RuleID    string    `json:"rule_id,omitempty"`
}

// LogListResponse is the response for GET /logs.
type LogListResponse struct {
	Logs   []LogEntry `json:"logs"`
	Total  int        `json:"total"`
	Offset int        `json:"offset"`
	Limit  int        `json:"limit"`
}

// BastionSessionListResponse is the response for GET /bastion/sessions.
type BastionSessionListResponse struct {
	Sessions []bastion.SessionEvent `json:"sessions"`
	Total    int                    `json:"total"`
	Offset   int                    `json:"offset"`
	Limit    int                    `json:"limit"`
}

// ExecCheckRequest is the request body for POST /exec/check.
type ExecCheckRequest struct {
	Command string `json:"command"`
	AgentID string `json:"agent_id"`
}

// ExecCheckResponse is the response for POST /exec/check.
type ExecCheckResponse struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
	RuleID  string `json:"rule_id,omitempty"`
}

// ErrorResponse is the standard error response.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Details string `json:"details,omitempty"`
}
