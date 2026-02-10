package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Extra-Chill/plasma-shield/internal/bastion"
)

// Store holds the in-memory state for the shield.
// In production, this would be backed by a database.
type Store struct {
	mu            sync.RWMutex
	agents        map[string]*Agent
	rules         map[string]*Rule
	logs          []LogEntry
	bastionLogs   *bastion.LogStore
	startedAt     time.Time
	requestsTotal int64
	blockedTotal  int64
}

// NewStore creates a new in-memory store.
func NewStore() *Store {
	return &Store{
		agents:      make(map[string]*Agent),
		rules:       make(map[string]*Rule),
		logs:        make([]LogEntry, 0),
		bastionLogs: bastion.NewLogStore(bastion.DefaultLogLimit),
		startedAt:   time.Now(),
	}
}

// Handlers holds all HTTP handlers and their dependencies.
type Handlers struct {
	store   *Store
	version string
}

// NewHandlers creates a new Handlers instance.
func NewHandlers(store *Store, version string) *Handlers {
	return &Handlers{
		store:   store,
		version: version,
	}
}

// StatusHandler handles GET /status.
func (h *Handlers) StatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	h.store.mu.RLock()
	defer h.store.mu.RUnlock()

	uptime := time.Since(h.store.startedAt)
	resp := StatusResponse{
		Status:        "operational",
		Version:       h.version,
		Uptime:        uptime.Round(time.Second).String(),
		StartedAt:     h.store.startedAt,
		AgentCount:    len(h.store.agents),
		RuleCount:     len(h.store.rules),
		RequestsTotal: h.store.requestsTotal,
		BlockedTotal:  h.store.blockedTotal,
	}

	writeJSON(w, http.StatusOK, resp)
}

// ListAgentsHandler handles GET /agents.
func (h *Handlers) ListAgentsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	h.store.mu.RLock()
	defer h.store.mu.RUnlock()

	agents := make([]Agent, 0, len(h.store.agents))
	for _, a := range h.store.agents {
		agents = append(agents, *a)
	}

	writeJSON(w, http.StatusOK, AgentListResponse{
		Agents: agents,
		Total:  len(agents),
	})
}

// PauseAgentHandler handles POST /agents/{id}/pause.
func (h *Handlers) PauseAgentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	agentID := extractAgentID(r.URL.Path, "/agents/", "/pause")
	if agentID == "" {
		writeError(w, http.StatusBadRequest, "missing agent ID")
		return
	}

	h.store.mu.Lock()
	defer h.store.mu.Unlock()

	agent, exists := h.store.agents[agentID]
	if !exists {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	agent.Status = "paused"

	writeJSON(w, http.StatusOK, AgentActionResponse{
		ID:      agentID,
		Status:  "paused",
		Message: "agent paused successfully - all traffic blocked",
	})
}

// KillAgentHandler handles POST /agents/{id}/kill.
func (h *Handlers) KillAgentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	agentID := extractAgentID(r.URL.Path, "/agents/", "/kill")
	if agentID == "" {
		writeError(w, http.StatusBadRequest, "missing agent ID")
		return
	}

	h.store.mu.Lock()
	defer h.store.mu.Unlock()

	agent, exists := h.store.agents[agentID]
	if !exists {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	agent.Status = "killed"

	// In production, this would trigger an alert system
	writeJSON(w, http.StatusOK, AgentActionResponse{
		ID:      agentID,
		Status:  "killed",
		Message: "agent killed - traffic blocked and alert sent",
	})
}

// ResumeAgentHandler handles POST /agents/{id}/resume.
func (h *Handlers) ResumeAgentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	agentID := extractAgentID(r.URL.Path, "/agents/", "/resume")
	if agentID == "" {
		writeError(w, http.StatusBadRequest, "missing agent ID")
		return
	}

	h.store.mu.Lock()
	defer h.store.mu.Unlock()

	agent, exists := h.store.agents[agentID]
	if !exists {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	if agent.Status == "killed" {
		writeError(w, http.StatusConflict, "cannot resume killed agent - use agent restore instead")
		return
	}

	agent.Status = "active"

	writeJSON(w, http.StatusOK, AgentActionResponse{
		ID:      agentID,
		Status:  "active",
		Message: "agent resumed - traffic flowing normally",
	})
}

// ListRulesHandler handles GET /rules.
func (h *Handlers) ListRulesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	h.store.mu.RLock()
	defer h.store.mu.RUnlock()

	rules := make([]Rule, 0, len(h.store.rules))
	for _, rule := range h.store.rules {
		rules = append(rules, *rule)
	}

	writeJSON(w, http.StatusOK, RuleListResponse{
		Rules: rules,
		Total: len(rules),
	})
}

// CreateRuleHandler handles POST /rules.
func (h *Handlers) CreateRuleHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req CreateRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Action != "block" && req.Action != "allow" {
		writeError(w, http.StatusBadRequest, "action must be 'block' or 'allow'")
		return
	}

	if req.Pattern == "" && req.Domain == "" {
		writeError(w, http.StatusBadRequest, "pattern or domain is required")
		return
	}

	h.store.mu.Lock()
	defer h.store.mu.Unlock()

	id := generateID()
	rule := &Rule{
		ID:          id,
		Pattern:     req.Pattern,
		Domain:      req.Domain,
		Action:      req.Action,
		Description: req.Description,
		Enabled:     req.Enabled,
		CreatedAt:   time.Now(),
	}

	h.store.rules[id] = rule

	writeJSON(w, http.StatusCreated, CreateRuleResponse{
		Rule:    *rule,
		Message: "rule created successfully",
	})
}

// DeleteRuleHandler handles DELETE /rules/{id}.
func (h *Handlers) DeleteRuleHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ruleID := strings.TrimPrefix(r.URL.Path, "/rules/")
	if ruleID == "" || ruleID == r.URL.Path {
		writeError(w, http.StatusBadRequest, "missing rule ID")
		return
	}

	h.store.mu.Lock()
	defer h.store.mu.Unlock()

	if _, exists := h.store.rules[ruleID]; !exists {
		writeError(w, http.StatusNotFound, "rule not found")
		return
	}

	delete(h.store.rules, ruleID)

	writeJSON(w, http.StatusOK, DeleteRuleResponse{
		ID:      ruleID,
		Message: "rule deleted successfully",
	})
}

// ListLogsHandler handles GET /logs.
func (h *Handlers) ListLogsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Parse query parameters
	query := r.URL.Query()
	limit := 100
	offset := 0

	if l := query.Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 1000 {
			limit = parsed
		}
	}

	if o := query.Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	agentFilter := query.Get("agent_id")
	actionFilter := query.Get("action")
	typeFilter := query.Get("type")

	h.store.mu.RLock()
	defer h.store.mu.RUnlock()

	// Filter logs
	filtered := make([]LogEntry, 0)
	for _, log := range h.store.logs {
		if agentFilter != "" && log.AgentID != agentFilter {
			continue
		}
		if actionFilter != "" && log.Action != actionFilter {
			continue
		}
		if typeFilter != "" && log.Type != typeFilter {
			continue
		}
		filtered = append(filtered, log)
	}

	// Apply pagination
	total := len(filtered)
	start := offset
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}

	writeJSON(w, http.StatusOK, LogListResponse{
		Logs:   filtered[start:end],
		Total:  total,
		Offset: offset,
		Limit:  limit,
	})
}

// ListBastionSessionsHandler handles GET /bastion/sessions.
func (h *Handlers) ListBastionSessionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	query := r.URL.Query()
	limit := 100
	offset := 0

	if l := query.Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 1000 {
			limit = parsed
		}
	}

	if o := query.Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	events, total := h.store.bastionLogs.List(offset, limit)

	writeJSON(w, http.StatusOK, BastionSessionListResponse{
		Sessions: events,
		Total:    total,
		Offset:   offset,
		Limit:    limit,
	})
}

// ExecCheckHandler handles POST /exec/check.
func (h *Handlers) ExecCheckHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req ExecCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Command == "" {
		writeError(w, http.StatusBadRequest, "command is required")
		return
	}

	h.store.mu.Lock()
	defer h.store.mu.Unlock()

	h.store.requestsTotal++

	// Check agent status
	if agent, exists := h.store.agents[req.AgentID]; exists {
		if agent.Status == "paused" || agent.Status == "killed" {
			h.store.blockedTotal++
			h.addLog(req.AgentID, "command", req.Command, "blocked", "agent-status")
			writeJSON(w, http.StatusOK, ExecCheckResponse{
				Allowed: false,
				Reason:  "agent is " + agent.Status,
			})
			return
		}
		agent.LastSeen = time.Now()
	}

	// Check rules
	for _, rule := range h.store.rules {
		if !rule.Enabled {
			continue
		}
		if rule.Pattern != "" && matchPattern(req.Command, rule.Pattern) {
			if rule.Action == "block" {
				h.store.blockedTotal++
				h.addLog(req.AgentID, "command", req.Command, "blocked", rule.ID)
				writeJSON(w, http.StatusOK, ExecCheckResponse{
					Allowed: false,
					Reason:  rule.Description,
					RuleID:  rule.ID,
				})
				return
			}
		}
	}

	h.addLog(req.AgentID, "command", req.Command, "allowed", "")
	writeJSON(w, http.StatusOK, ExecCheckResponse{
		Allowed: true,
	})
}

// addLog adds a log entry to the store (must be called with lock held).
func (h *Handlers) addLog(agentID, logType, request, action, ruleID string) {
	entry := LogEntry{
		ID:        generateID(),
		Timestamp: time.Now(),
		AgentID:   agentID,
		Type:      logType,
		Request:   request,
		Action:    action,
		RuleID:    ruleID,
	}
	h.store.logs = append(h.store.logs, entry)

	// Keep only last 10000 logs
	if len(h.store.logs) > 10000 {
		h.store.logs = h.store.logs[len(h.store.logs)-10000:]
	}
}

// RegisterAgent registers a new agent (for testing/setup).
func (h *Handlers) RegisterAgent(id, name, ip string) {
	h.store.mu.Lock()
	defer h.store.mu.Unlock()

	h.store.agents[id] = &Agent{
		ID:        id,
		Name:      name,
		IP:        ip,
		Status:    "active",
		LastSeen:  time.Now(),
		CreatedAt: time.Now(),
	}
}

// Helper functions

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, ErrorResponse{
		Error: message,
		Code:  status,
	})
}

func extractAgentID(path, prefix, suffix string) string {
	path = strings.TrimPrefix(path, prefix)
	path = strings.TrimSuffix(path, suffix)
	return path
}

func generateID() string {
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}

func matchPattern(command, pattern string) bool {
	// Simple substring match for now
	// Could be extended to support glob or regex
	return strings.Contains(command, pattern)
}
