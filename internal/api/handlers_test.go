package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStatusHandler(t *testing.T) {
	store := NewStore()
	handlers := NewHandlers(store, "1.0.0")

	// Add some test data
	handlers.RegisterAgent("agent-1", "Test Agent", "192.168.1.1")
	store.rules["rule-1"] = &Rule{ID: "rule-1", Pattern: "test", Action: "block", Enabled: true}

	t.Run("returns correct status", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/status", nil)
		rec := httptest.NewRecorder()

		handlers.StatusHandler(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}

		var resp StatusResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Status != "operational" {
			t.Errorf("expected status 'operational', got %q", resp.Status)
		}
		if resp.Version != "1.0.0" {
			t.Errorf("expected version '1.0.0', got %q", resp.Version)
		}
		if resp.AgentCount != 1 {
			t.Errorf("expected agent_count 1, got %d", resp.AgentCount)
		}
		if resp.RuleCount != 1 {
			t.Errorf("expected rule_count 1, got %d", resp.RuleCount)
		}
	})

	t.Run("rejects non-GET methods", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/status", nil)
		rec := httptest.NewRecorder()

		handlers.StatusHandler(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})
}

func TestListAgentsHandler(t *testing.T) {
	store := NewStore()
	handlers := NewHandlers(store, "1.0.0")

	t.Run("returns empty list initially", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/agents", nil)
		rec := httptest.NewRecorder()

		handlers.ListAgentsHandler(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}

		var resp AgentListResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Total != 0 {
			t.Errorf("expected total 0, got %d", resp.Total)
		}
		if len(resp.Agents) != 0 {
			t.Errorf("expected empty agents list, got %d agents", len(resp.Agents))
		}
	})

	t.Run("returns agents after registration", func(t *testing.T) {
		handlers.RegisterAgent("agent-1", "Alpha", "10.0.0.1")
		handlers.RegisterAgent("agent-2", "Beta", "10.0.0.2")

		req := httptest.NewRequest(http.MethodGet, "/agents", nil)
		rec := httptest.NewRecorder()

		handlers.ListAgentsHandler(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}

		var resp AgentListResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Total != 2 {
			t.Errorf("expected total 2, got %d", resp.Total)
		}
	})

	t.Run("rejects non-GET methods", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/agents", nil)
		rec := httptest.NewRecorder()

		handlers.ListAgentsHandler(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})
}

func TestPauseAgentHandler(t *testing.T) {
	store := NewStore()
	handlers := NewHandlers(store, "1.0.0")
	handlers.RegisterAgent("agent-1", "Test Agent", "192.168.1.1")

	t.Run("pauses existing agent", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/agents/agent-1/pause", nil)
		rec := httptest.NewRecorder()

		handlers.PauseAgentHandler(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}

		var resp AgentActionResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.ID != "agent-1" {
			t.Errorf("expected id 'agent-1', got %q", resp.ID)
		}
		if resp.Status != "paused" {
			t.Errorf("expected status 'paused', got %q", resp.Status)
		}

		// Verify agent status in store
		store.mu.RLock()
		agent := store.agents["agent-1"]
		store.mu.RUnlock()

		if agent.Status != "paused" {
			t.Errorf("expected agent status 'paused', got %q", agent.Status)
		}
	})

	t.Run("returns 404 for non-existent agent", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/agents/nonexistent/pause", nil)
		rec := httptest.NewRecorder()

		handlers.PauseAgentHandler(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rec.Code)
		}
	})

	t.Run("returns 400 for missing agent ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/agents//pause", nil)
		rec := httptest.NewRecorder()

		handlers.PauseAgentHandler(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("rejects non-POST methods", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/agents/agent-1/pause", nil)
		rec := httptest.NewRecorder()

		handlers.PauseAgentHandler(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})
}

func TestKillAgentHandler(t *testing.T) {
	store := NewStore()
	handlers := NewHandlers(store, "1.0.0")
	handlers.RegisterAgent("agent-1", "Test Agent", "192.168.1.1")

	t.Run("kills existing agent", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/agents/agent-1/kill", nil)
		rec := httptest.NewRecorder()

		handlers.KillAgentHandler(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}

		var resp AgentActionResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Status != "killed" {
			t.Errorf("expected status 'killed', got %q", resp.Status)
		}

		// Verify agent status in store
		store.mu.RLock()
		agent := store.agents["agent-1"]
		store.mu.RUnlock()

		if agent.Status != "killed" {
			t.Errorf("expected agent status 'killed', got %q", agent.Status)
		}
	})

	t.Run("returns 404 for non-existent agent", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/agents/nonexistent/kill", nil)
		rec := httptest.NewRecorder()

		handlers.KillAgentHandler(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rec.Code)
		}
	})

	t.Run("rejects non-POST methods", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/agents/agent-1/kill", nil)
		rec := httptest.NewRecorder()

		handlers.KillAgentHandler(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})
}

func TestResumeAgentHandler(t *testing.T) {
	t.Run("resumes paused agent", func(t *testing.T) {
		store := NewStore()
		handlers := NewHandlers(store, "1.0.0")
		handlers.RegisterAgent("agent-1", "Test Agent", "192.168.1.1")

		// First pause the agent
		store.mu.Lock()
		store.agents["agent-1"].Status = "paused"
		store.mu.Unlock()

		req := httptest.NewRequest(http.MethodPost, "/agents/agent-1/resume", nil)
		rec := httptest.NewRecorder()

		handlers.ResumeAgentHandler(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}

		var resp AgentActionResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Status != "active" {
			t.Errorf("expected status 'active', got %q", resp.Status)
		}
	})

	t.Run("fails to resume killed agent", func(t *testing.T) {
		store := NewStore()
		handlers := NewHandlers(store, "1.0.0")
		handlers.RegisterAgent("agent-1", "Test Agent", "192.168.1.1")

		// Kill the agent
		store.mu.Lock()
		store.agents["agent-1"].Status = "killed"
		store.mu.Unlock()

		req := httptest.NewRequest(http.MethodPost, "/agents/agent-1/resume", nil)
		rec := httptest.NewRecorder()

		handlers.ResumeAgentHandler(rec, req)

		if rec.Code != http.StatusConflict {
			t.Errorf("expected status %d, got %d", http.StatusConflict, rec.Code)
		}

		var resp ErrorResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Error != "cannot resume killed agent - use agent restore instead" {
			t.Errorf("unexpected error message: %q", resp.Error)
		}
	})

	t.Run("returns 404 for non-existent agent", func(t *testing.T) {
		store := NewStore()
		handlers := NewHandlers(store, "1.0.0")

		req := httptest.NewRequest(http.MethodPost, "/agents/nonexistent/resume", nil)
		rec := httptest.NewRecorder()

		handlers.ResumeAgentHandler(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rec.Code)
		}
	})

	t.Run("rejects non-POST methods", func(t *testing.T) {
		store := NewStore()
		handlers := NewHandlers(store, "1.0.0")

		req := httptest.NewRequest(http.MethodGet, "/agents/agent-1/resume", nil)
		rec := httptest.NewRecorder()

		handlers.ResumeAgentHandler(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})
}

func TestListRulesHandler(t *testing.T) {
	store := NewStore()
	handlers := NewHandlers(store, "1.0.0")

	t.Run("returns empty list initially", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/rules", nil)
		rec := httptest.NewRecorder()

		handlers.ListRulesHandler(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}

		var resp RuleListResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Total != 0 {
			t.Errorf("expected total 0, got %d", resp.Total)
		}
	})

	t.Run("returns rules after creation", func(t *testing.T) {
		store.mu.Lock()
		store.rules["rule-1"] = &Rule{ID: "rule-1", Pattern: "rm -rf", Action: "block", Enabled: true}
		store.rules["rule-2"] = &Rule{ID: "rule-2", Domain: "evil.com", Action: "block", Enabled: true}
		store.mu.Unlock()

		req := httptest.NewRequest(http.MethodGet, "/rules", nil)
		rec := httptest.NewRecorder()

		handlers.ListRulesHandler(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}

		var resp RuleListResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Total != 2 {
			t.Errorf("expected total 2, got %d", resp.Total)
		}
	})

	t.Run("rejects non-GET methods", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/rules", nil)
		rec := httptest.NewRecorder()

		handlers.ListRulesHandler(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})
}

func TestCreateRuleHandler(t *testing.T) {
	t.Run("creates rule with pattern", func(t *testing.T) {
		store := NewStore()
		handlers := NewHandlers(store, "1.0.0")

		body := CreateRuleRequest{
			Pattern:     "rm -rf",
			Action:      "block",
			Description: "Block dangerous rm commands",
			Enabled:     true,
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/rules", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handlers.CreateRuleHandler(rec, req)

		if rec.Code != http.StatusCreated {
			t.Errorf("expected status %d, got %d", http.StatusCreated, rec.Code)
		}

		var resp CreateRuleResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Rule.Pattern != "rm -rf" {
			t.Errorf("expected pattern 'rm -rf', got %q", resp.Rule.Pattern)
		}
		if resp.Rule.Action != "block" {
			t.Errorf("expected action 'block', got %q", resp.Rule.Action)
		}
		if !resp.Rule.Enabled {
			t.Error("expected rule to be enabled")
		}
	})

	t.Run("creates rule with domain", func(t *testing.T) {
		store := NewStore()
		handlers := NewHandlers(store, "1.0.0")

		body := CreateRuleRequest{
			Domain:  "evil.com",
			Action:  "block",
			Enabled: true,
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/rules", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handlers.CreateRuleHandler(rec, req)

		if rec.Code != http.StatusCreated {
			t.Errorf("expected status %d, got %d", http.StatusCreated, rec.Code)
		}
	})

	t.Run("rejects invalid action", func(t *testing.T) {
		store := NewStore()
		handlers := NewHandlers(store, "1.0.0")

		body := CreateRuleRequest{
			Pattern: "test",
			Action:  "invalid",
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/rules", bytes.NewReader(bodyBytes))
		rec := httptest.NewRecorder()

		handlers.CreateRuleHandler(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
		}

		var resp ErrorResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Error != "action must be 'block' or 'allow'" {
			t.Errorf("unexpected error: %q", resp.Error)
		}
	})

	t.Run("rejects missing pattern and domain", func(t *testing.T) {
		store := NewStore()
		handlers := NewHandlers(store, "1.0.0")

		body := CreateRuleRequest{
			Action: "block",
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/rules", bytes.NewReader(bodyBytes))
		rec := httptest.NewRecorder()

		handlers.CreateRuleHandler(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("rejects invalid JSON", func(t *testing.T) {
		store := NewStore()
		handlers := NewHandlers(store, "1.0.0")

		req := httptest.NewRequest(http.MethodPost, "/rules", bytes.NewReader([]byte("invalid json")))
		rec := httptest.NewRecorder()

		handlers.CreateRuleHandler(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("rejects non-POST methods", func(t *testing.T) {
		store := NewStore()
		handlers := NewHandlers(store, "1.0.0")

		req := httptest.NewRequest(http.MethodGet, "/rules", nil)
		rec := httptest.NewRecorder()

		handlers.CreateRuleHandler(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})
}

func TestDeleteRuleHandler(t *testing.T) {
	t.Run("deletes existing rule", func(t *testing.T) {
		store := NewStore()
		handlers := NewHandlers(store, "1.0.0")

		store.mu.Lock()
		store.rules["rule-123"] = &Rule{ID: "rule-123", Pattern: "test", Action: "block"}
		store.mu.Unlock()

		req := httptest.NewRequest(http.MethodDelete, "/rules/rule-123", nil)
		rec := httptest.NewRecorder()

		handlers.DeleteRuleHandler(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}

		var resp DeleteRuleResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.ID != "rule-123" {
			t.Errorf("expected id 'rule-123', got %q", resp.ID)
		}

		// Verify rule is deleted from store
		store.mu.RLock()
		_, exists := store.rules["rule-123"]
		store.mu.RUnlock()

		if exists {
			t.Error("rule should be deleted from store")
		}
	})

	t.Run("returns 404 for non-existent rule", func(t *testing.T) {
		store := NewStore()
		handlers := NewHandlers(store, "1.0.0")

		req := httptest.NewRequest(http.MethodDelete, "/rules/nonexistent", nil)
		rec := httptest.NewRecorder()

		handlers.DeleteRuleHandler(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rec.Code)
		}
	})

	t.Run("returns 400 for missing rule ID", func(t *testing.T) {
		store := NewStore()
		handlers := NewHandlers(store, "1.0.0")

		req := httptest.NewRequest(http.MethodDelete, "/rules/", nil)
		rec := httptest.NewRecorder()

		handlers.DeleteRuleHandler(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("rejects non-DELETE methods", func(t *testing.T) {
		store := NewStore()
		handlers := NewHandlers(store, "1.0.0")

		req := httptest.NewRequest(http.MethodGet, "/rules/rule-123", nil)
		rec := httptest.NewRecorder()

		handlers.DeleteRuleHandler(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})
}

func TestListLogsHandler(t *testing.T) {
	store := NewStore()
	handlers := NewHandlers(store, "1.0.0")

	// Add test logs
	store.mu.Lock()
	store.logs = []LogEntry{
		{ID: "1", AgentID: "agent-1", Type: "command", Request: "ls", Action: "allowed"},
		{ID: "2", AgentID: "agent-1", Type: "command", Request: "rm", Action: "blocked"},
		{ID: "3", AgentID: "agent-2", Type: "http", Request: "GET /api", Action: "allowed"},
		{ID: "4", AgentID: "agent-1", Type: "dns", Request: "evil.com", Action: "blocked"},
		{ID: "5", AgentID: "agent-2", Type: "command", Request: "cat", Action: "allowed"},
	}
	store.mu.Unlock()

	t.Run("returns all logs with defaults", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/logs", nil)
		rec := httptest.NewRecorder()

		handlers.ListLogsHandler(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}

		var resp LogListResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Total != 5 {
			t.Errorf("expected total 5, got %d", resp.Total)
		}
		if len(resp.Logs) != 5 {
			t.Errorf("expected 5 logs, got %d", len(resp.Logs))
		}
		if resp.Limit != 100 {
			t.Errorf("expected default limit 100, got %d", resp.Limit)
		}
	})

	t.Run("respects limit and offset", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/logs?limit=2&offset=1", nil)
		rec := httptest.NewRecorder()

		handlers.ListLogsHandler(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}

		var resp LogListResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Total != 5 {
			t.Errorf("expected total 5, got %d", resp.Total)
		}
		if len(resp.Logs) != 2 {
			t.Errorf("expected 2 logs, got %d", len(resp.Logs))
		}
		if resp.Offset != 1 {
			t.Errorf("expected offset 1, got %d", resp.Offset)
		}
		if resp.Limit != 2 {
			t.Errorf("expected limit 2, got %d", resp.Limit)
		}
	})

	t.Run("filters by agent_id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/logs?agent_id=agent-1", nil)
		rec := httptest.NewRecorder()

		handlers.ListLogsHandler(rec, req)

		var resp LogListResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Total != 3 {
			t.Errorf("expected total 3 for agent-1, got %d", resp.Total)
		}
	})

	t.Run("filters by action", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/logs?action=blocked", nil)
		rec := httptest.NewRecorder()

		handlers.ListLogsHandler(rec, req)

		var resp LogListResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Total != 2 {
			t.Errorf("expected total 2 blocked logs, got %d", resp.Total)
		}
	})

	t.Run("filters by type", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/logs?type=command", nil)
		rec := httptest.NewRecorder()

		handlers.ListLogsHandler(rec, req)

		var resp LogListResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Total != 3 {
			t.Errorf("expected total 3 command logs, got %d", resp.Total)
		}
	})

	t.Run("combines multiple filters", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/logs?agent_id=agent-1&action=blocked", nil)
		rec := httptest.NewRecorder()

		handlers.ListLogsHandler(rec, req)

		var resp LogListResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Total != 2 {
			t.Errorf("expected total 2 blocked logs for agent-1, got %d", resp.Total)
		}
	})

	t.Run("rejects non-GET methods", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/logs", nil)
		rec := httptest.NewRecorder()

		handlers.ListLogsHandler(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})
}

func TestExecCheckHandler(t *testing.T) {
	t.Run("allows command with no matching rules", func(t *testing.T) {
		store := NewStore()
		handlers := NewHandlers(store, "1.0.0")
		handlers.RegisterAgent("agent-1", "Test Agent", "192.168.1.1")

		body := ExecCheckRequest{
			Command: "ls -la",
			AgentID: "agent-1",
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/exec/check", bytes.NewReader(bodyBytes))
		rec := httptest.NewRecorder()

		handlers.ExecCheckHandler(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}

		var resp ExecCheckResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if !resp.Allowed {
			t.Error("expected command to be allowed")
		}
	})

	t.Run("blocks command matching rule", func(t *testing.T) {
		store := NewStore()
		handlers := NewHandlers(store, "1.0.0")
		handlers.RegisterAgent("agent-1", "Test Agent", "192.168.1.1")

		store.mu.Lock()
		store.rules["rule-1"] = &Rule{
			ID:          "rule-1",
			Pattern:     "rm -rf",
			Action:      "block",
			Description: "Dangerous command",
			Enabled:     true,
		}
		store.mu.Unlock()

		body := ExecCheckRequest{
			Command: "rm -rf /",
			AgentID: "agent-1",
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/exec/check", bytes.NewReader(bodyBytes))
		rec := httptest.NewRecorder()

		handlers.ExecCheckHandler(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}

		var resp ExecCheckResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Allowed {
			t.Error("expected command to be blocked")
		}
		if resp.RuleID != "rule-1" {
			t.Errorf("expected rule_id 'rule-1', got %q", resp.RuleID)
		}
		if resp.Reason != "Dangerous command" {
			t.Errorf("expected reason 'Dangerous command', got %q", resp.Reason)
		}
	})

	t.Run("blocks when agent is paused", func(t *testing.T) {
		store := NewStore()
		handlers := NewHandlers(store, "1.0.0")
		handlers.RegisterAgent("agent-1", "Test Agent", "192.168.1.1")

		store.mu.Lock()
		store.agents["agent-1"].Status = "paused"
		store.mu.Unlock()

		body := ExecCheckRequest{
			Command: "ls",
			AgentID: "agent-1",
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/exec/check", bytes.NewReader(bodyBytes))
		rec := httptest.NewRecorder()

		handlers.ExecCheckHandler(rec, req)

		var resp ExecCheckResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Allowed {
			t.Error("expected command to be blocked for paused agent")
		}
		if resp.Reason != "agent is paused" {
			t.Errorf("expected reason 'agent is paused', got %q", resp.Reason)
		}
	})

	t.Run("blocks when agent is killed", func(t *testing.T) {
		store := NewStore()
		handlers := NewHandlers(store, "1.0.0")
		handlers.RegisterAgent("agent-1", "Test Agent", "192.168.1.1")

		store.mu.Lock()
		store.agents["agent-1"].Status = "killed"
		store.mu.Unlock()

		body := ExecCheckRequest{
			Command: "ls",
			AgentID: "agent-1",
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/exec/check", bytes.NewReader(bodyBytes))
		rec := httptest.NewRecorder()

		handlers.ExecCheckHandler(rec, req)

		var resp ExecCheckResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Allowed {
			t.Error("expected command to be blocked for killed agent")
		}
		if resp.Reason != "agent is killed" {
			t.Errorf("expected reason 'agent is killed', got %q", resp.Reason)
		}
	})

	t.Run("skips disabled rules", func(t *testing.T) {
		store := NewStore()
		handlers := NewHandlers(store, "1.0.0")
		handlers.RegisterAgent("agent-1", "Test Agent", "192.168.1.1")

		store.mu.Lock()
		store.rules["rule-1"] = &Rule{
			ID:      "rule-1",
			Pattern: "rm",
			Action:  "block",
			Enabled: false, // Disabled
		}
		store.mu.Unlock()

		body := ExecCheckRequest{
			Command: "rm file.txt",
			AgentID: "agent-1",
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/exec/check", bytes.NewReader(bodyBytes))
		rec := httptest.NewRecorder()

		handlers.ExecCheckHandler(rec, req)

		var resp ExecCheckResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if !resp.Allowed {
			t.Error("expected command to be allowed (rule disabled)")
		}
	})

	t.Run("increments request and blocked counters", func(t *testing.T) {
		store := NewStore()
		handlers := NewHandlers(store, "1.0.0")
		handlers.RegisterAgent("agent-1", "Test Agent", "192.168.1.1")

		store.mu.Lock()
		store.rules["rule-1"] = &Rule{ID: "rule-1", Pattern: "block-me", Action: "block", Enabled: true}
		store.mu.Unlock()

		// Allowed request
		body1, _ := json.Marshal(ExecCheckRequest{Command: "ls", AgentID: "agent-1"})
		req1 := httptest.NewRequest(http.MethodPost, "/exec/check", bytes.NewReader(body1))
		rec1 := httptest.NewRecorder()
		handlers.ExecCheckHandler(rec1, req1)

		// Blocked request
		body2, _ := json.Marshal(ExecCheckRequest{Command: "block-me", AgentID: "agent-1"})
		req2 := httptest.NewRequest(http.MethodPost, "/exec/check", bytes.NewReader(body2))
		rec2 := httptest.NewRecorder()
		handlers.ExecCheckHandler(rec2, req2)

		store.mu.RLock()
		requestsTotal := store.requestsTotal
		blockedTotal := store.blockedTotal
		store.mu.RUnlock()

		if requestsTotal != 2 {
			t.Errorf("expected requestsTotal 2, got %d", requestsTotal)
		}
		if blockedTotal != 1 {
			t.Errorf("expected blockedTotal 1, got %d", blockedTotal)
		}
	})

	t.Run("returns 400 for missing command", func(t *testing.T) {
		store := NewStore()
		handlers := NewHandlers(store, "1.0.0")

		body := ExecCheckRequest{
			AgentID: "agent-1",
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/exec/check", bytes.NewReader(bodyBytes))
		rec := httptest.NewRecorder()

		handlers.ExecCheckHandler(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("returns 400 for invalid JSON", func(t *testing.T) {
		store := NewStore()
		handlers := NewHandlers(store, "1.0.0")

		req := httptest.NewRequest(http.MethodPost, "/exec/check", bytes.NewReader([]byte("invalid")))
		rec := httptest.NewRecorder()

		handlers.ExecCheckHandler(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("rejects non-POST methods", func(t *testing.T) {
		store := NewStore()
		handlers := NewHandlers(store, "1.0.0")

		req := httptest.NewRequest(http.MethodGet, "/exec/check", nil)
		rec := httptest.NewRecorder()

		handlers.ExecCheckHandler(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})
}
