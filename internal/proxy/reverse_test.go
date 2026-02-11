package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Extra-Chill/plasma-shield/internal/fleet"
)

func TestReverseHandler_NoAuth(t *testing.T) {
	fleetMgr := fleet.NewManager()
	handler := NewReverseHandler(fleetMgr)

	req := httptest.NewRequest("GET", "/agent/test/hooks", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestReverseHandler_InvalidToken(t *testing.T) {
	fleetMgr := fleet.NewManager()
	handler := NewReverseHandler(fleetMgr)
	handler.RegisterToken("valid-token", "tenant1")

	req := httptest.NewRequest("GET", "/agent/test/hooks", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestReverseHandler_TenantNotFound(t *testing.T) {
	fleetMgr := fleet.NewManager()
	handler := NewReverseHandler(fleetMgr)
	handler.RegisterToken("valid-token", "nonexistent-tenant")

	req := httptest.NewRequest("GET", "/agent/test/hooks", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestReverseHandler_AgentNotInFleet(t *testing.T) {
	fleetMgr := fleet.NewManager()
	fleetMgr.CreateTenant("tenant1")
	// Don't add any agents

	handler := NewReverseHandler(fleetMgr)
	handler.RegisterToken("valid-token", "tenant1")

	req := httptest.NewRequest("GET", "/agent/unknown-agent/hooks", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestReverseHandler_ValidRequest(t *testing.T) {
	// Create a test backend server
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Plasma-Shield") != "true" {
			t.Error("expected X-Plasma-Shield header")
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer backend.Close()

	fleetMgr := fleet.NewManager()
	fleetMgr.CreateTenant("tenant1")
	fleetMgr.AddAgent("tenant1", fleet.Agent{
		ID:         "test-agent",
		Name:       "Test Agent",
		WebhookURL: backend.URL,
	})

	handler := NewReverseHandler(fleetMgr)
	handler.RegisterToken("valid-token", "tenant1")

	req := httptest.NewRequest("GET", "/agent/test-agent/status", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestReverseHandler_IdentityMasking(t *testing.T) {
	// Create a test backend server that checks for identity masking
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Should have X-Captain header
		captain := r.Header.Get("X-Captain")
		if captain != "Captain Chubes" {
			t.Errorf("expected X-Captain 'Captain Chubes', got '%s'", captain)
		}

		// Should NOT have X-Forwarded-For (identity masked)
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			t.Errorf("expected no X-Forwarded-For (identity masking), got '%s'", xff)
		}

		// Should NOT have source agent headers
		if src := r.Header.Get("X-Agent-Id"); src != "" {
			t.Errorf("expected no X-Agent-Id (identity masking), got '%s'", src)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	fleetMgr := fleet.NewManager()
	fleetMgr.CreateTenant("chubes")
	fleetMgr.SetCaptainName("chubes", "Captain Chubes")
	fleetMgr.AddAgent("chubes", fleet.Agent{
		ID:         "sarai",
		Name:       "Sarai Chinwag",
		WebhookURL: backend.URL,
	})

	handler := NewReverseHandler(fleetMgr)
	handler.RegisterToken("chubes-token", "chubes")

	// Simulate request from Fleet Command (another agent) to Sarai
	req := httptest.NewRequest("POST", "/agent/sarai/hooks", nil)
	req.Header.Set("Authorization", "Bearer chubes-token")
	req.Header.Set("X-Forwarded-For", "178.156.153.244") // Command's IP - should be stripped
	req.Header.Set("X-Agent-Id", "fleet-command")        // Should be stripped
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestReverseHandler_InvalidPath(t *testing.T) {
	fleetMgr := fleet.NewManager()
	handler := NewReverseHandler(fleetMgr)
	handler.RegisterToken("valid-token", "tenant1")

	req := httptest.NewRequest("GET", "/invalid/path", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name     string
		auth     string
		expected string
	}{
		{"valid bearer", "Bearer abc123", "abc123"},
		{"lowercase bearer", "bearer abc123", "abc123"},
		{"no auth", "", ""},
		{"basic auth", "Basic abc123", ""},
		{"just token", "abc123", ""},
		{"bearer no space", "Bearerabc123", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.auth != "" {
				req.Header.Set("Authorization", tt.auth)
			}
			got := extractBearerToken(req)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}
